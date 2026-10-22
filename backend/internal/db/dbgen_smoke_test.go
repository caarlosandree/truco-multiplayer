package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/db/dbgen"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/testdb"
)

func TestGeneratedQueriesRoundTripUUIDAndTime(t *testing.T) {
	ctx := context.Background()
	q := dbgen.New(testdb.New(t))
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	email, hash := "ana@empresa.com", "h"
	user, err := q.CreateUser(ctx, dbgen.CreateUserParams{Username: "ana", Email: &email, DisplayName: "Ana", PasswordHash: &hash, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	got, err := q.GetUserByID(ctx, user.ID)
	if err != nil || got.Username != "ana" || !got.CreatedAt.Equal(now) || got.EmailVerifiedAt != nil {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	sessionID, err := q.CreateSession(ctx, dbgen.CreateSessionParams{UserID: user.ID, TokenHash: []byte{1}, Now: now,
		ExpiresAt: now.Add(time.Hour), AbsoluteExpiresAt: now.Add(2 * time.Hour), UserAgent: "teste", Ip: "203.0.113.9"})
	if err != nil || sessionID == "" {
		t.Fatalf("sessão: %q %v", sessionID, err)
	}
	row, err := q.GetSessionPrincipal(ctx, dbgen.GetSessionPrincipalParams{TokenHash: []byte{1}, Now: now})
	if err != nil || row.UserID != user.ID || row.SessionID != sessionID {
		t.Fatalf("row=%+v err=%v", row, err)
	}
}
