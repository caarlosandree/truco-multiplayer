package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestActiveUserByUsername(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	id := f.newUser(t, "bia", "senha-forte-123")
	u, err := f.svc.ActiveUserByUsername(ctx, "Bia")
	if err != nil || u.ID != id || u.Username != "bia" {
		t.Fatalf("ActiveUserByUsername = %+v, %v", u, err)
	}
	if _, err := f.svc.ActiveUserByUsername(ctx, "ninguem"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("inexistente = %v", err)
	}
	for _, ruim := range []string{"bi\x00a", "bi\xffa"} {
		if _, err := f.svc.ActiveUserByUsername(ctx, ruim); !errors.Is(err, ErrUserNotFound) {
			t.Fatalf("username %q = %v", ruim, err)
		}
	}
	if _, err := f.pool.Exec(ctx, `UPDATE users SET disabled_at = now() WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ActiveUserByUsername(ctx, "bia"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("desativado = %v", err)
	}
	cid := f.newUser(t, "caio", "senha-forte-123")
	if _, err := f.pool.Exec(ctx, `UPDATE users SET email_verified_at = NULL WHERE id = $1`, cid); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ActiveUserByUsername(ctx, "caio"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("não verificado = %v", err)
	}
}

func TestSendRoomInviteLink(t *testing.T) {
	f := newFixture(t)
	id := f.newUser(t, "bia", "senha-forte-123")
	f.svc.SendRoomInvite(id, "Ana", "Mesa do RH", "tok123")
	msg, ok := f.mail.LastTo("bia@empresa.com")
	if !ok || !strings.Contains(msg.Text, "https://truco.test/entrar-convite/tok123") || msg.Subject != "Ana te chamou pra mesa no Truco" {
		t.Fatalf("e-mail = %+v (ok=%v)", msg, ok)
	}
}
