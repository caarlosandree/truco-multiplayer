package db_test

import (
	"context"
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/db"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/testdb"
)

func TestMain(m *testing.M) { testdb.Run(m) }

func TestMigrationsUpDownUp(t *testing.T) {
	ctx := context.Background()
	pool := testdb.New(t)
	exists := func(table string) bool {
		var ok bool
		if err := pool.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", table).Scan(&ok); err != nil {
			t.Fatal(err)
		}
		return ok
	}
	for _, table := range []string{"users", "sessions", "user_tokens", "allowed_email_domains", "matches",
		"match_players", "match_actions", "match_player_stats", "player_stats", "audit_log"} {
		if !exists(table) {
			t.Fatalf("tabela %s ausente após migrar", table)
		}
	}
	if err := db.MigrateDown(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if exists("users") || exists("audit_log") {
		t.Fatal("down não removeu as tabelas")
	}
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if !exists("users") {
		t.Fatal("up após down falhou")
	}
}

func TestUUIDv7DefaultAndUniqueViolation(t *testing.T) {
	ctx := context.Background()
	pool := testdb.New(t)
	id := testdb.CreateUser(t, pool, "ana")
	if len(id) != 36 || id[14] != '7' {
		t.Fatalf("id %q não é um UUID v7", id)
	}
	_, err := pool.Exec(ctx, `INSERT INTO users (username, email, display_name, password_hash) VALUES ('ana', 'outra@x.com', 'A', 'x')`)
	if !db.IsUniqueViolation(err, "users_username_key") {
		t.Fatalf("esperava violação de users_username_key, veio %v", err)
	}
	if db.IsUniqueViolation(err, "users_email_key") {
		t.Fatal("constraint errada reconhecida")
	}
}
