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
		_, err := pool.Exec(ctx, "INSERT INTO matches (seed, first_dealer, engine_version, status, winner_team, score_a, score_b, finished_at, end_reason) VALUES "+sql)
		return err
	}
	valid := []string{
		"(1, 0, 1, 'in_progress', NULL, NULL, NULL, NULL, NULL)",
		"(1, 0, 1, 'finished', 0, 12, 7, now(), 'game_over')",
		"(1, 0, 1, 'abandoned', NULL, NULL, NULL, now(), 'abandoned')",
	}
	for _, v := range valid {
		if err := insert(v); err != nil {
			t.Fatalf("%s rejeitado: %v", v, err)
		}
	}
	invalid := []string{
		"(1, 0, 1, 'in_progress', NULL, NULL, NULL, now(), NULL)",
		"(1, 0, 1, 'finished', NULL, 12, 7, now(), 'game_over')",
		"(1, 0, 1, 'finished', 0, NULL, 7, now(), 'game_over')",
		"(1, 0, 1, 'finished', 0, 12, NULL, now(), 'game_over')",
		"(1, 0, 1, 'finished', 0, 12, 7, NULL, 'game_over')",
		"(1, 0, 1, 'abandoned', NULL, NULL, NULL, NULL, 'abandoned')",
		"(1, 0, 1, 'finished', 0, 13, 7, now(), 'game_over')",
		"(1, 0, 1, 'finished', 0, 12, -1, now(), 'game_over')",
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

func TestEndReasonBackfill(t *testing.T) {
	ctx := context.Background()
	pool := testdb.New(t)
	if err := db.MigrateDownTo(ctx, pool, 3); err != nil {
		t.Fatal(err)
	}
	actor := testdb.CreateUser(t, pool, "ana")
	insert := func(status, reason string) string {
		t.Helper()
		var id string
		var err error
		switch status {
		case "finished":
			err = pool.QueryRow(ctx, `INSERT INTO matches (seed, first_dealer, engine_version, status, winner_team, score_a, score_b, finished_at)
				VALUES (1, 0, 1, 'finished', 0, 12, 7, now()) RETURNING id::text`).Scan(&id)
		case "abandoned":
			err = pool.QueryRow(ctx, `INSERT INTO matches (seed, first_dealer, engine_version, status, finished_at, abandon_reason)
				VALUES (1, 0, 1, 'abandoned', now(), $1) RETURNING id::text`, reason).Scan(&id)
		default:
			err = pool.QueryRow(ctx, `INSERT INTO matches (seed, first_dealer, engine_version) VALUES (1, 0, 1) RETURNING id::text`).Scan(&id)
		}
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	a := insert("finished", "")
	if _, err := pool.Exec(ctx, `INSERT INTO match_actions (match_id, ply, action, source) VALUES ($1, 0, '{"kind":"sys_forfeit","seat":0,"team":1}', 'system')`, a); err != nil {
		t.Fatal(err)
	}
	b := insert("finished", "")
	c := insert("abandoned", "qualquer texto")
	if _, err := pool.Exec(ctx, `INSERT INTO audit_log (actor_id, action, target_type, target_id) VALUES ($1, 'match.abort', 'match', $2)`, actor, c); err != nil {
		t.Fatal(err)
	}
	d := insert("abandoned", "abortada pelo administrador") // sem auditoria: o texto livre não conta
	e := insert("in_progress", "")

	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	ptr := func(s string) *string { return &s }
	want := map[string]*string{a: ptr("forfeit"), b: ptr("game_over"), c: ptr("aborted"), d: ptr("abandoned"), e: nil}
	for id, w := range want {
		var got *string
		if err := pool.QueryRow(ctx, "SELECT end_reason FROM matches WHERE id = $1", id).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if (got == nil) != (w == nil) || (got != nil && *got != *w) {
			t.Errorf("partida %s: end_reason = %v, quer %v", id, got, w)
		}
	}
	// finished sem end_reason viola o CHECK novo.
	_, err := pool.Exec(ctx, `INSERT INTO matches (seed, first_dealer, engine_version, status, winner_team, score_a, score_b, finished_at)
		VALUES (1, 0, 1, 'finished', 0, 12, 7, now())`)
	if err == nil {
		t.Error("finished com end_reason NULL deveria violar o CHECK")
	}
	// valor fora do domínio também.
	_, err = pool.Exec(ctx, `INSERT INTO matches (seed, first_dealer, engine_version, status, winner_team, score_a, score_b, finished_at, end_reason)
		VALUES (1, 0, 1, 'finished', 0, 12, 7, now(), 'xyz')`)
	if err == nil {
		t.Error("end_reason fora do domínio deveria ser rejeitado")
	}
}
