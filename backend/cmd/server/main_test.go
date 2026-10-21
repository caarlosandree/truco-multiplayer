package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/auth"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/config"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/db"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/enginetest"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/match"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/testdb"
)

func TestMain(m *testing.M) { testdb.Run(m) }

func envFor(values map[string]string) func(string) string {
	return func(k string) string { return values[k] }
}

func baseEnv(databaseURL string) map[string]string {
	return map[string]string{
		"DATABASE_URL": databaseURL, "APP_BASE_URL": "http://localhost:5173", "ALLOWED_EMAIL_DOMAINS": "empresa.com",
		"MAIL_DRIVER": "log", "HTTP_ADDR": "127.0.0.1:0", "LOG_FORMAT": "text",
	}
}

func TestRunRejectsUnknownSubcommandBeforeTouchingDatabase(t *testing.T) {
	err := run(context.Background(), []string{"voar"}, envFor(map[string]string{}), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "subcomando desconhecido") {
		t.Fatalf("err = %v", err)
	}
}

func TestRunReportsConfigErrors(t *testing.T) {
	err := run(context.Background(), []string{"migrate"}, envFor(map[string]string{}), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("err = %v", err)
	}
}

func TestMigratePromoteAndRecompute(t *testing.T) {
	ctx := context.Background()
	url := testdb.NewURL(t)
	env := envFor(baseEnv(url))
	var out bytes.Buffer
	if err := run(ctx, []string{"migrate"}, env, &out); err != nil {
		t.Fatal(err)
	}
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	testdb.CreateUser(t, pool, "ana")
	if err := run(ctx, []string{"promote-admin"}, env, &out); err == nil || !strings.Contains(err.Error(), "uso:") {
		t.Fatalf("sem argumento: %v", err)
	}
	if err := run(ctx, []string{"promote-admin", "ana"}, env, &out); err != nil {
		t.Fatal(err)
	}
	var isAdmin bool
	if err := pool.QueryRow(ctx, "SELECT is_admin FROM users WHERE username = 'ana'").Scan(&isAdmin); err != nil {
		t.Fatal(err)
	}
	if !isAdmin {
		t.Fatal("promote-admin não promoveu")
	}
	out.Reset()
	if err := run(ctx, []string{"demote-admin", "ana"}, env, &out); err == nil || !strings.Contains(err.Error(), "ao menos um administrador") {
		t.Fatalf("rebaixar o único admin: %v", err)
	}
	if err := run(ctx, []string{"demote-admin"}, env, &out); err == nil || !strings.Contains(err.Error(), "uso:") {
		t.Fatalf("sem argumento: %v", err)
	}
	testdb.CreateUser(t, pool, "bia")
	if err := run(ctx, []string{"promote-admin", "bia"}, env, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := run(ctx, []string{"demote-admin", "ana"}, env, &out); err != nil || !strings.Contains(out.String(), "ana não é mais administrador") {
		t.Fatalf("demote: %v %q", err, out.String())
	}
	out.Reset()
	if err := run(ctx, []string{"demote-admin", "ana"}, env, &out); err != nil || !strings.Contains(out.String(), "ana não é administrador") {
		t.Fatalf("demote de quem não é admin: %v %q", err, out.String())
	}
	out.Reset()
	if err := run(ctx, []string{"recompute-stats"}, env, &out); err != nil || !strings.Contains(out.String(), "partidas puladas: 0") {
		t.Fatalf("recompute: %v %q", err, out.String())
	}
}

// startServer sobe "serve" e devolve o endereço e uma função que desliga e devolve o erro de run.
func startServer(t *testing.T, url string, out io.Writer) (string, func() error) {
	t.Helper()
	addrCh := make(chan string, 1)
	onListen = func(addr string) { addrCh <- addr }
	t.Cleanup(func() { onListen = func(string) {} })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, nil, envFor(baseEnv(url)), out) }()
	select {
	case addr := <-addrCh:
		return addr, func() error {
			cancel()
			select {
			case err := <-done:
				return err
			case <-time.After(20 * time.Second):
				t.Fatal("servidor não desligou")
				return nil
			}
		}
	case err := <-done:
		t.Fatalf("servidor saiu antes de ouvir: %v", err)
	case <-time.After(30 * time.Second):
		t.Fatal("servidor não começou a ouvir")
	}
	cancel()
	return "", nil
}

func TestServeStartsAndShutsDown(t *testing.T) {
	addr, stop := startServer(t, testdb.NewURL(t), &bytes.Buffer{})
	resp, err := http.Get("http://" + addr + "/healthz")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz: %v %v", resp, err)
	}
	resp.Body.Close()
	if err := stop(); err != nil {
		t.Fatalf("desligamento: %v", err)
	}
}

func TestServeResumesInProgressMatchesAndMountsWebSocket(t *testing.T) {
	url := testdb.NewURL(t)
	ctx := context.Background()
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var players [4]replay.PlayerSeat
	for i, name := range []string{"ana", "bia", "caio", "duda"} {
		players[i] = replay.PlayerSeat{UserID: testdb.CreateUser(t, pool, name), DisplayName: name}
	}
	rec, _ := enginetest.RandomRecord(21)
	store := match.NewStore(pool, nil)
	id, err := store.Create(ctx, rec.Seed, rec.FirstDealer, players)
	if err != nil {
		t.Fatal(err)
	}
	for _, logged := range rec.Actions[:10] {
		if err := store.AppendAction(ctx, id, logged.Ply, logged.Action, logged.Source); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	addr, stop := startServer(t, url, &out)
	resp, err := http.Get("http://" + addr + "/ws")
	if err != nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("GET /ws sem sessão: %v %v", resp, err)
	}
	resp.Body.Close()
	if err := stop(); err != nil {
		t.Fatalf("desligamento: %v", err)
	}
	if !strings.Contains(out.String(), "partidas retomadas") || !strings.Contains(out.String(), "count=1") {
		t.Fatalf("log do boot não registra a partida retomada:\n%s", out.String())
	}
}

// Sem People o lobby usa a implementação nula e ninguém consegue ser convidado: falha se a ligação com as contas sumir.
func TestLobbyOptionsWirePeople(t *testing.T) {
	opts := lobbyOptions(config.Config{}, nil, auth.NewService(auth.Options{}), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if got := fmt.Sprintf("%T", opts.People); got != "lobby.authPeople" {
		t.Fatalf("People = %s, queria lobby.authPeople (cadastro de contas ligado)", got)
	}
}
