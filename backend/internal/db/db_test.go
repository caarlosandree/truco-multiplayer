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

func TestMatchesConstraints(t *testing.T) {
	ctx := context.Background()
	pool := testdb.New(t)
	userID := testdb.CreateUser(t, pool, "ana")
	insert := func(sql string) error {
		_, err := pool.Exec(ctx, "INSERT INTO matches (seed, first_dealer, engine_version, status, winner_team, score_a, score_b, finished_at) VALUES "+sql)
		return err
	}
	valid := []string{
		"(1, 0, 1, 'in_progress', NULL, NULL, NULL, NULL)",
		"(1, 0, 1, 'finished', 0, 12, 7, now())",
		"(1, 0, 1, 'abandoned', NULL, NULL, NULL, now())",
	}
	for _, v := range valid {
		if err := insert(v); err != nil {
			t.Fatalf("%s rejeitado: %v", v, err)
		}
	}
	invalid := []string{
		"(1, 0, 1, 'in_progress', NULL, NULL, NULL, now())",
		"(1, 0, 1, 'finished', NULL, 12, 7, now())",
		"(1, 0, 1, 'finished', 0, NULL, 7, now())",
		"(1, 0, 1, 'finished', 0, 12, NULL, now())",
		"(1, 0, 1, 'finished', 0, 12, 7, NULL)",
		"(1, 0, 1, 'abandoned', NULL, NULL, NULL, NULL)",
		"(1, 0, 1, 'finished', 0, 13, 7, now())",
		"(1, 0, 1, 'finished', 0, 12, -1, now())",
	}
	for _, v := range invalid {
		if err := insert(v); err == nil {
			t.Errorf("%s aceito", v)
		}
	}

	var matchID string
	if err := pool.QueryRow(ctx, "SELECT id::text FROM matches WHERE status = 'finished'").Scan(&matchID); err != nil {
		t.Fatal(err)
	}
	// Contadores têm DEFAULT 0 (spec §4.8).
	if _, err := pool.Exec(ctx, "INSERT INTO match_player_stats (match_id, user_id, team, won) VALUES ($1, $2, 0, true)", matchID, userID); err != nil {
		t.Fatalf("contadores sem DEFAULT 0: %v", err)
	}
	var hands int
	if err := pool.QueryRow(ctx, "SELECT hands_played + timeouts FROM match_player_stats WHERE match_id = $1", matchID).Scan(&hands); err != nil || hands != 0 {
		t.Fatalf("hands=%d err=%v", hands, err)
	}
}
