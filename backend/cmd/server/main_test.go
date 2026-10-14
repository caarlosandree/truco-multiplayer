package main

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/db"
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
	_ = pool.QueryRow(ctx, "SELECT is_admin FROM users WHERE username = 'ana'").Scan(&isAdmin)
	if !isAdmin {
		t.Fatal("promote-admin não promoveu")
	}
	out.Reset()
	if err := run(ctx, []string{"recompute-stats"}, env, &out); err != nil || !strings.Contains(out.String(), "partidas puladas: 0") {
		t.Fatalf("recompute: %v %q", err, out.String())
	}
}

func TestServeStartsAndShutsDown(t *testing.T) {
	url := testdb.NewURL(t)
	addrCh := make(chan string, 1)
	onListen = func(addr string) { addrCh <- addr }
	t.Cleanup(func() { onListen = func(string) {} })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, nil, envFor(baseEnv(url)), &bytes.Buffer{}) }()
	var addr string
	select {
	case addr = <-addrCh:
	case err := <-done:
		t.Fatalf("servidor saiu antes de ouvir: %v", err)
	case <-time.After(30 * time.Second):
		t.Fatal("servidor não começou a ouvir")
	}
	resp, err := http.Get("http://" + addr + "/healthz")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz: %v %v", resp, err)
	}
	resp.Body.Close()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("desligamento: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("servidor não desligou")
	}
}
