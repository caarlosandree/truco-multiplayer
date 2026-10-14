package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/db/dbgen"
)

const goodPassword = "cavalo-bateria-grampo"

func (f fixture) register(t *testing.T, username string) string {
	t.Helper()
	email := username + "@empresa.com"
	err := f.svc.Register(context.Background(), RegisterInput{Username: username, Email: email, DisplayName: "Nome " + username, Password: goodPassword})
	if err != nil {
		t.Fatal(err)
	}
	return email
}

func (f fixture) registerVerified(t *testing.T, username string) string {
	t.Helper()
	email := f.register(t, username)
	if err := f.svc.VerifyEmail(context.Background(), f.mail.LastToken(email)); err != nil {
		t.Fatal(err)
	}
	return email
}

func (f fixture) countUsers(t *testing.T) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(context.Background(), "SELECT count(*) FROM users").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestRegisterVerifyLogin(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	email := f.register(t, "ana")
	if msg, ok := f.mail.LastTo(email); !ok || !strings.Contains(msg.Text, "https://truco.test/verificar-email?token=") {
		t.Fatalf("e-mail de verificação = %+v", msg)
	}
	if _, err := f.svc.Login(ctx, "ana", goodPassword, SessionMeta{}); !errors.Is(err, ErrEmailNotVerified) {
		t.Fatalf("login antes de verificar: %v", err)
	}
	token := f.mail.LastToken(email)
	if err := f.svc.VerifyEmail(ctx, token); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.VerifyEmail(ctx, token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("segundo clique no link: %v", err)
	}
	res, err := f.svc.Login(ctx, "ana", goodPassword, SessionMeta{})
	if err != nil || res.Token == "" || res.User.Username != "ana" || res.User.DisplayName != "Nome ana" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if _, err := f.svc.Authenticate(ctx, res.Token); err != nil {
		t.Fatal(err)
	}
}

func TestRegisterValidation(t *testing.T) {
	f := newFixture(t)
	err := f.svc.Register(context.Background(), RegisterInput{Username: "a!", Email: "nao-e-email", DisplayName: " ", Password: "curta"})
	var fields FieldErrors
	if !errors.As(err, &fields) {
		t.Fatalf("err = %v", err)
	}
	for _, key := range []string{"username", "email", "displayName", "password"} {
		if fields[key] == "" {
			t.Errorf("faltou erro em %s: %v", key, fields)
		}
	}
}

func TestRegisterRejectsDomain(t *testing.T) {
	f := newFixture(t)
	err := f.svc.Register(context.Background(), RegisterInput{Username: "ana", Email: "ana@gmail.com", DisplayName: "Ana", Password: goodPassword})
	if !errors.Is(err, ErrEmailDomainNotAllowed) {
		t.Fatalf("err = %v", err)
	}
}

func TestRegisterExistingEmailIsSilentAndCaseInsensitive(t *testing.T) {
	f := newFixture(t)
	f.registerVerified(t, "ana")
	before := len(f.mail.Sent())
	err := f.svc.Register(context.Background(), RegisterInput{Username: "outra", Email: "  ANA@Empresa.COM ", DisplayName: "X", Password: goodPassword})
	if err != nil {
		t.Fatalf("e-mail existente deve responder como sucesso: %v", err)
	}
	if f.countUsers(t) != 1 {
		t.Fatal("não pode criar segunda conta para o mesmo e-mail")
	}
	sent := f.mail.Sent()
	if len(sent) != before+1 || sent[len(sent)-1].Subject != "Você já tem uma conta no Truco" {
		t.Fatalf("esperava aviso de conta existente, enviados: %+v", sent[before:])
	}
}

func TestRegisterUsernameTaken(t *testing.T) {
	f := newFixture(t)
	f.register(t, "ana")
	err := f.svc.Register(context.Background(), RegisterInput{Username: "ANA", Email: "outra@empresa.com", DisplayName: "X", Password: goodPassword})
	if !errors.Is(err, ErrUsernameTaken) {
		t.Fatalf("err = %v", err)
	}
}

func TestVerificationLinkExpiresAndResendInvalidatesOld(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	email := f.register(t, "ana")
	first := f.mail.LastToken(email)
	if err := f.svc.ResendVerification(ctx, " ANA@empresa.com"); err != nil {
		t.Fatal(err)
	}
	second := f.mail.LastToken(email)
	if second == first {
		t.Fatal("reenvio deve gerar token novo")
	}
	if err := f.svc.VerifyEmail(ctx, first); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("token antigo: %v", err)
	}
	f.clock.Advance(24*time.Hour + time.Second)
	if err := f.svc.VerifyEmail(ctx, second); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("token expirado: %v", err)
	}
	before := len(f.mail.Sent())
	_ = f.svc.ResendVerification(ctx, "ninguem@empresa.com")
	if len(f.mail.Sent()) != before {
		t.Fatal("e-mail inexistente não recebe nada")
	}
}

func TestLoginVariants(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.registerVerified(t, "ana")
	for _, login := range []string{"ana", "  ANA ", "ana@empresa.com", " Ana@Empresa.COM "} {
		if _, err := f.svc.Login(ctx, login, goodPassword, SessionMeta{}); err != nil {
			t.Errorf("login %q: %v", login, err)
		}
	}
	if _, err := f.svc.Login(ctx, "ana", "senha-errada-123", SessionMeta{}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("senha errada: %v", err)
	}
	if _, err := f.svc.Login(ctx, "ninguem", goodPassword, SessionMeta{}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("usuário inexistente: %v", err)
	}
}

func TestLoginBlockedStatesOnlyAfterCorrectPassword(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.registerVerified(t, "ana")
	_, _ = f.pool.Exec(ctx, "UPDATE users SET disabled_at = now() WHERE username = 'ana'")
	if _, err := f.svc.Login(ctx, "ana", "senha-errada-123", SessionMeta{}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("senha errada em conta desativada não pode revelar o estado: %v", err)
	}
	if _, err := f.svc.Login(ctx, "ana", goodPassword, SessionMeta{}); !errors.Is(err, ErrAccountDisabled) {
		t.Fatalf("desativada: %v", err)
	}
	_, _ = f.pool.Exec(ctx, "UPDATE users SET disabled_at = NULL, must_reset_password = true WHERE username = 'ana'")
	if _, err := f.svc.Login(ctx, "ana", goodPassword, SessionMeta{}); !errors.Is(err, ErrPasswordResetRequired) {
		t.Fatalf("reset obrigatório: %v", err)
	}
}

func TestLoginRehashesWhenParamsChange(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.registerVerified(t, "ana")
	stronger := TestingParams
	stronger.Time = 2
	f.svc.hasher = NewHasher(stronger, 4)
	if _, err := f.svc.Login(ctx, "ana", goodPassword, SessionMeta{}); err != nil {
		t.Fatal(err)
	}
	var hash string
	_ = f.pool.QueryRow(ctx, "SELECT password_hash FROM users WHERE username = 'ana'").Scan(&hash)
	if !strings.Contains(hash, "t=2") {
		t.Fatalf("hash não foi atualizado: %s", hash)
	}
}

func TestForgotAndResetPassword(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	email := f.registerVerified(t, "ana")
	old, _ := f.svc.Login(ctx, "ana", goodPassword, SessionMeta{})
	if err := f.svc.ForgotPassword(ctx, " ANA@empresa.com "); err != nil {
		t.Fatal(err)
	}
	first := f.mail.LastToken(email)
	_ = f.svc.ForgotPassword(ctx, email)
	second := f.mail.LastToken(email)
	if err := f.svc.ResetPassword(ctx, first, "nova-senha-segura-1"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("link antigo de reset: %v", err)
	}
	var fields FieldErrors
	if err := f.svc.ResetPassword(ctx, second, "curta"); !errors.As(err, &fields) {
		t.Fatalf("senha inválida: %v", err)
	}
	if err := f.svc.ResetPassword(ctx, second, "nova-senha-segura-1"); err != nil {
		t.Fatalf("o token deve sobreviver a uma senha inválida: %v", err)
	}
	if err := f.svc.ResetPassword(ctx, second, "outra-senha-segura-2"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("token reutilizado: %v", err)
	}
	if _, err := f.svc.Authenticate(ctx, old.Token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("reset deve derrubar todas as sessões")
	}
	if _, err := f.svc.Login(ctx, "ana", goodPassword, SessionMeta{}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatal("senha antiga ainda funciona")
	}
	if _, err := f.svc.Login(ctx, "ana", "nova-senha-segura-1", SessionMeta{}); err != nil {
		t.Fatal(err)
	}
}

func TestResetLinkExpiresIn30Minutes(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	email := f.registerVerified(t, "ana")
	_ = f.svc.ForgotPassword(ctx, email)
	f.clock.Advance(31 * time.Minute)
	if err := f.svc.ResetPassword(ctx, f.mail.LastToken(email), "nova-senha-segura-1"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("err = %v", err)
	}
}

func TestForgotForUnverifiedResendsVerification(t *testing.T) {
	f := newFixture(t)
	email := f.register(t, "ana")
	_ = f.svc.ForgotPassword(context.Background(), email)
	msg, _ := f.mail.LastTo(email)
	if msg.Subject != "Confirme seu e-mail no Truco" {
		t.Fatalf("assunto = %q", msg.Subject)
	}
}

func TestResetClearsMustResetAndVerifiesEmail(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	email := f.registerVerified(t, "ana")
	_, _ = f.pool.Exec(ctx, "UPDATE users SET must_reset_password = true WHERE username = 'ana'")
	var userID string
	_ = f.pool.QueryRow(ctx, "SELECT id::text FROM users WHERE username = 'ana'").Scan(&userID)
	if err := f.svc.SendPasswordReset(ctx, userID, true); err != nil {
		t.Fatal(err)
	}
	msg, _ := f.mail.LastTo(email)
	if msg.Subject != "Você precisa redefinir sua senha do Truco" {
		t.Fatalf("assunto = %q", msg.Subject)
	}
	if err := f.svc.ResetPassword(ctx, f.mail.LastToken(email), "nova-senha-segura-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Login(ctx, "ana", "nova-senha-segura-1", SessionMeta{}); err != nil {
		t.Fatalf("login após reset forçado: %v", err)
	}
}

func TestChangePasswordRotatesSession(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	email := f.registerVerified(t, "ana")
	a, _ := f.svc.Login(ctx, "ana", goodPassword, SessionMeta{})
	b, _ := f.svc.Login(ctx, "ana", goodPassword, SessionMeta{})
	p, _ := f.svc.Authenticate(ctx, a.Token)
	if _, err := f.svc.ChangePassword(ctx, p, "errada-errada-1", "nova-senha-segura-1", SessionMeta{}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("senha atual errada: %v", err)
	}
	var fields FieldErrors
	if _, err := f.svc.ChangePassword(ctx, p, goodPassword, "ana-nova-senha-1", SessionMeta{}); !errors.As(err, &fields) || fields["newPassword"] == "" {
		t.Fatalf("nova senha com username: %v", err)
	}
	newToken, err := f.svc.ChangePassword(ctx, p, goodPassword, "nova-senha-segura-1", SessionMeta{})
	if err != nil || newToken == "" {
		t.Fatal(err)
	}
	for _, old := range []string{a.Token, b.Token} {
		if _, err := f.svc.Authenticate(ctx, old); !errors.Is(err, ErrUnauthenticated) {
			t.Fatal("sessões antigas devem cair")
		}
	}
	if _, err := f.svc.Authenticate(ctx, newToken); err != nil {
		t.Fatal("sessão rotacionada deve valer")
	}
	if msg, _ := f.mail.LastTo(email); msg.Subject != "Sua senha do Truco foi alterada" {
		t.Fatalf("aviso não enviado: %q", msg.Subject)
	}
}

func TestUpdateDisplayName(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.registerVerified(t, "ana")
	res, _ := f.svc.Login(ctx, "ana", goodPassword, SessionMeta{})
	u, err := f.svc.UpdateDisplayName(ctx, res.User.ID, "  Ana Clara ")
	if err != nil || u.DisplayName != "Ana Clara" {
		t.Fatalf("u=%+v err=%v", u, err)
	}
	var fields FieldErrors
	if _, err := f.svc.UpdateDisplayName(ctx, res.User.ID, ""); !errors.As(err, &fields) {
		t.Fatalf("nome vazio: %v", err)
	}
}

// Username é público: ocupado precisa dar o mesmo erro exista ou não o e-mail informado.
func TestRegisterUsernameTakenRegardlessOfEmail(t *testing.T) {
	f := newFixture(t)
	f.register(t, "ana")
	f.register(t, "bia")
	before := len(f.mail.Sent())
	for _, email := range []string{"bia@empresa.com", "ana@empresa.com", "nova@empresa.com"} {
		err := f.svc.Register(context.Background(), RegisterInput{Username: "ana", Email: email, DisplayName: "X", Password: goodPassword})
		if !errors.Is(err, ErrUsernameTaken) {
			t.Fatalf("e-mail %s: err = %v", email, err)
		}
	}
	if len(f.mail.Sent()) != before {
		t.Fatal("username ocupado não pode disparar e-mail")
	}
}

func TestChangePasswordInvalidatesPendingResetLinks(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	email := f.registerVerified(t, "ana")
	if err := f.svc.ForgotPassword(ctx, email); err != nil {
		t.Fatal(err)
	}
	pending := f.mail.LastToken(email)
	res, _ := f.svc.Login(ctx, "ana", goodPassword, SessionMeta{})
	p, _ := f.svc.Authenticate(ctx, res.Token)
	if _, err := f.svc.ChangePassword(ctx, p, goodPassword, "nova-senha-segura-1", SessionMeta{}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ResetPassword(ctx, pending, "outra-senha-segura-2"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("link de reset emitido antes da troca ainda vale: %v", err)
	}
}

func TestResetPasswordInvalidatesOtherPendingResetLinks(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	email := f.registerVerified(t, "ana")
	if err := f.svc.ForgotPassword(ctx, email); err != nil {
		t.Fatal(err)
	}
	used := f.mail.LastToken(email)
	var userID string
	if err := f.pool.QueryRow(ctx, "SELECT id::text FROM users WHERE username = 'ana'").Scan(&userID); err != nil {
		t.Fatal(err)
	}
	// Um segundo link pendente (o fluxo normal já invalida o anterior ao emitir; aqui é a defesa extra).
	extra, extraHash := NewToken()
	if err := f.svc.q.CreateUserToken(ctx, dbgen.CreateUserTokenParams{UserID: userID, Purpose: purposeReset, TokenHash: extraHash, Now: f.clock.Now(), ExpiresAt: f.clock.Now().Add(resetTTL)}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ResetPassword(ctx, used, "nova-senha-segura-1"); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ResetPassword(ctx, extra, "outra-senha-segura-2"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("outro link pendente ainda vale após o reset: %v", err)
	}
}
