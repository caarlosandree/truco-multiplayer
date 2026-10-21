package admin_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/admin"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/auth"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/clock"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/mail"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/match"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/testdb"
)

func TestMain(m *testing.M) { testdb.Run(m) }

type recordingAborter struct {
	mu  sync.Mutex
	ids []string
}

func (r *recordingAborter) Abort(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ids = append(r.ids, id)
	return nil
}

type env struct {
	svc     *admin.Service
	auth    *auth.Service
	pool    *pgxpool.Pool
	mail    *mail.Fake
	aborter *recordingAborter
	actor   auth.Principal
}

func newEnv(t *testing.T) env {
	t.Helper()
	pool := testdb.New(t)
	clock := clock.NewManual(time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC))
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	e := env{pool: pool, mail: &mail.Fake{}, aborter: &recordingAborter{}}
	e.auth = auth.NewService(auth.Options{Pool: pool, Mailer: e.mail, Hasher: auth.NewHasher(auth.TestingParams, 4),
		Now: clock.Now, BaseURL: "https://truco.test", Logger: logger, Background: func(fn func()) { fn() }})
	if err := e.auth.BootstrapDomains(context.Background(), []string{"empresa.com"}); err != nil {
		t.Fatal(err)
	}
	e.svc = admin.NewService(admin.Options{Pool: pool, Auth: e.auth, Aborter: e.aborter, Now: clock.Now, Logger: logger})
	adminID := testdb.CreateUser(t, pool, "chefe")
	_, _ = pool.Exec(context.Background(), "UPDATE users SET is_admin = true WHERE id = $1", adminID)
	e.actor = auth.Principal{UserID: adminID, Username: "chefe", IsAdmin: true}
	return e
}

func (e env) auditCount(t *testing.T, action string) int {
	t.Helper()
	var n int
	_ = e.pool.QueryRow(context.Background(), "SELECT count(*) FROM audit_log WHERE action = $1", action).Scan(&n)
	return n
}

func (e env) sessionCount(t *testing.T, userID string) int {
	t.Helper()
	var n int
	_ = e.pool.QueryRow(context.Background(), "SELECT count(*) FROM sessions WHERE user_id = $1", userID).Scan(&n)
	return n
}

func (e env) addSession(t *testing.T, userID string) {
	t.Helper()
	_, err := e.pool.Exec(context.Background(), `INSERT INTO sessions (user_id, token_hash, last_seen_at, expires_at, absolute_expires_at)
		VALUES ($1, decode(md5(random()::text), 'hex'), now(), now() + interval '1 day', now() + interval '2 days')`, userID)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDisableAndEnableUser(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	target := testdb.CreateUser(t, e.pool, "ana")
	e.addSession(t, target)
	if err := e.svc.DisableUser(ctx, e.actor, target); err != nil {
		t.Fatal(err)
	}
	if e.sessionCount(t, target) != 0 || e.auditCount(t, "user.disable") != 1 {
		t.Fatal("desativar deve revogar sessões e auditar")
	}
	if err := e.svc.EnableUser(ctx, e.actor, target); err != nil || e.auditCount(t, "user.enable") != 1 {
		t.Fatalf("enable: %v", err)
	}
	if err := e.svc.DisableUser(ctx, e.actor, e.actor.UserID); !errors.Is(err, admin.ErrSelfAction) {
		t.Fatalf("auto-desativação: %v", err)
	}
	for _, missing := range []string{"00000000-0000-7000-8000-000000000000", "nao-e-uuid"} {
		if err := e.svc.DisableUser(ctx, e.actor, missing); !errors.Is(err, admin.ErrNotFound) {
			t.Fatalf("%q: %v", missing, err)
		}
	}
}

func TestForcePasswordReset(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	target := testdb.CreateUser(t, e.pool, "ana")
	e.addSession(t, target)
	if err := e.svc.ForcePasswordReset(ctx, e.actor, target); err != nil {
		t.Fatal(err)
	}
	var must bool
	_ = e.pool.QueryRow(ctx, "SELECT must_reset_password FROM users WHERE id = $1", target).Scan(&must)
	msg, sent := e.mail.LastTo("ana@empresa.com")
	if !must || e.sessionCount(t, target) != 0 || !sent || msg.Subject != "Você precisa redefinir sua senha do Truco" || e.auditCount(t, "user.force_password_reset") != 1 {
		t.Fatalf("must=%v sessões=%d e-mail=%v", must, e.sessionCount(t, target), msg.Subject)
	}
}

func TestForcePasswordResetIgnoresPostCommitSendFailure(t *testing.T) {
	cases := map[string]func(e env, t *testing.T){
		"mailer falha": func(e env, _ *testing.T) { e.mail.Err = errors.New("smtp fora do ar") },
		"emissão do token falha": func(e env, t *testing.T) {
			_, err := e.pool.Exec(context.Background(), `
				CREATE FUNCTION falha_token() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'token indisponível'; END $$;
				CREATE TRIGGER falha_token BEFORE INSERT ON user_tokens FOR EACH ROW EXECUTE FUNCTION falha_token()`)
			if err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, breakSend := range cases {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			ctx := context.Background()
			target := testdb.CreateUser(t, e.pool, "ana")
			e.addSession(t, target)
			breakSend(e, t)
			if err := e.svc.ForcePasswordReset(ctx, e.actor, target); err != nil {
				t.Fatalf("falha pós-commit no envio não pode virar erro: %v", err)
			}
			var must bool
			_ = e.pool.QueryRow(ctx, "SELECT must_reset_password FROM users WHERE id = $1", target).Scan(&must)
			if !must || e.sessionCount(t, target) != 0 || e.auditCount(t, "user.force_password_reset") != 1 {
				t.Fatalf("estado não aplicado: must=%v sessões=%d", must, e.sessionCount(t, target))
			}
		})
	}
}

func TestSetAdminProtectsLastAdmin(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	if err := e.svc.SetAdmin(ctx, e.actor, e.actor.UserID, false); !errors.Is(err, admin.ErrLastAdmin) {
		t.Fatalf("remover o último admin: %v", err)
	}
	if e.auditCount(t, "user.set_admin") != 0 {
		t.Fatal("ação recusada não pode ser auditada")
	}
	other := testdb.CreateUser(t, e.pool, "bia")
	if err := e.svc.SetAdmin(ctx, e.actor, other, true); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.SetAdmin(ctx, e.actor, e.actor.UserID, false); err != nil {
		t.Fatalf("com dois admins, pode sair: %v", err)
	}
	if e.auditCount(t, "user.set_admin") != 2 {
		t.Fatal("auditoria ausente")
	}
}

func TestDomains(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	for _, bad := range []string{"", "sem-ponto", "a@b.com", "espaço.com", "-x.com"} {
		if err := e.svc.AddDomain(ctx, e.actor, bad); !errors.Is(err, admin.ErrInvalidDomain) {
			t.Errorf("%q: %v", bad, err)
		}
	}
	if err := e.svc.AddDomain(ctx, e.actor, " @Filial.Com.BR "); err != nil {
		t.Fatal(err)
	}
	domains, _ := e.svc.ListDomains(ctx)
	if len(domains) != 2 || domains[1].Domain != "filial.com.br" || domains[1].CreatedBy == nil || *domains[1].CreatedBy != e.actor.UserID {
		t.Fatalf("domínios = %+v", domains)
	}
	if err := e.svc.RemoveDomain(ctx, e.actor, "empresa.com"); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.RemoveDomain(ctx, e.actor, "filial.com.br"); !errors.Is(err, admin.ErrLastDomain) {
		t.Fatalf("último domínio: %v", err)
	}
	if err := e.svc.RemoveDomain(ctx, e.actor, "nao.existe"); !errors.Is(err, admin.ErrNotFound) {
		t.Fatalf("inexistente: %v", err)
	}
	if e.auditCount(t, "domain.add") != 1 || e.auditCount(t, "domain.remove") != 1 {
		t.Fatal("auditoria de domínios incorreta")
	}
}

func TestAbortMatch(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	var players [4]replay.PlayerSeat
	for i, name := range []string{"ana", "bia", "caio", "duda"} {
		players[i] = replay.PlayerSeat{UserID: testdb.CreateUser(t, e.pool, name), DisplayName: name}
	}
	store := match.NewStore(e.pool, nil)
	id, _ := store.Create(ctx, 1, 0, players)
	if err := e.svc.AbortMatch(ctx, e.actor, id, "mesa travada"); err != nil {
		t.Fatal(err)
	}
	summary, _ := store.Summary(ctx, id)
	if summary.Status != replay.StatusAbandoned || *summary.AbandonReason != "mesa travada" {
		t.Fatalf("summary = %+v", summary)
	}
	if summary.EndReason == nil || *summary.EndReason != replay.EndAborted {
		t.Fatalf("end_reason = %v", summary.EndReason)
	}
	if len(e.aborter.ids) != 1 || e.aborter.ids[0] != id || e.auditCount(t, "match.abort") != 1 {
		t.Fatalf("aborter=%v", e.aborter.ids)
	}
	if err := e.svc.AbortMatch(ctx, e.actor, id, "de novo"); !errors.Is(err, match.ErrMatchClosed) {
		t.Fatalf("abortar duas vezes: %v", err)
	}
}

func TestListUsersSearchAndPagination(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	for i := range 51 {
		testdb.CreateUser(t, e.pool, fmt.Sprintf("user%02d", i))
	}
	page, err := e.svc.ListUsers(ctx, "", "")
	if err != nil || len(page.Items) != 50 || page.NextCursor == "" {
		t.Fatalf("página 1: %d itens, cursor %q, err %v", len(page.Items), page.NextCursor, err)
	}
	page2, _ := e.svc.ListUsers(ctx, "", page.NextCursor)
	if len(page2.Items) != 2 || page2.NextCursor != "" {
		t.Fatalf("página 2: %+v", page2)
	}
	found, _ := e.svc.ListUsers(ctx, "USER0", "")
	if len(found.Items) != 10 {
		t.Fatalf("busca USER0: %d", len(found.Items))
	}
	if none, _ := e.svc.ListUsers(ctx, "%", ""); len(none.Items) != 0 {
		t.Fatal("% deve ser literal na busca")
	}
}

func TestListAuditNewestFirst(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	_ = e.svc.AddDomain(ctx, e.actor, "a.com")
	_ = e.svc.AddDomain(ctx, e.actor, "b.com")
	page, err := e.svc.ListAudit(ctx, "")
	if err != nil || len(page.Items) != 2 || page.Items[0].TargetID != "b.com" || page.Items[0].ActorUsername != "chefe" {
		t.Fatalf("auditoria = %+v err %v", page, err)
	}
}

func TestPromoteAdmin(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	id := testdb.CreateUser(t, e.pool, "ana")
	if err := admin.PromoteAdmin(ctx, e.pool, " ANA@empresa.com ", time.Now()); err != nil {
		t.Fatal(err)
	}
	var isAdmin bool
	_ = e.pool.QueryRow(ctx, "SELECT is_admin FROM users WHERE id = $1", id).Scan(&isAdmin)
	if !isAdmin || e.auditCount(t, "user.promote_admin") != 1 {
		t.Fatal("promoção não aplicada ou não auditada")
	}
	if err := admin.PromoteAdmin(ctx, e.pool, "bia", time.Now()); err == nil {
		t.Fatal("usuário inexistente deve falhar")
	}
}
