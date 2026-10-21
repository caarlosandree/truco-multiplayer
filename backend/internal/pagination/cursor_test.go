package pagination

import (
	"encoding/base64"
	"errors"
	"testing"
	"time"
)

func TestCursorRoundTrip(t *testing.T) {
	at := time.Date(2026, 10, 14, 12, 0, 0, 123456000, time.FixedZone("BRT", -3*3600))
	gotAt, gotID, err := Decode(Encode(at, "0199e7a4-0000-7000-8000-000000000001"))
	if err != nil || !gotAt.Equal(at) || *gotID != "0199e7a4-0000-7000-8000-000000000001" {
		t.Fatalf("at=%v id=%v err=%v", gotAt, gotID, err)
	}
}

func TestDecodeEmptyAndInvalid(t *testing.T) {
	if at, id, err := Decode(""); at != nil || id != nil || err != nil {
		t.Fatal("cursor vazio deve ser a primeira página")
	}
	forged := base64.RawURLEncoding.EncodeToString([]byte("2026-10-14T12:00:00Z|abc"))
	for _, bad := range []string{"%%%", "c2VtLXNlcGFyYWRvcg", "bm8tdGltZXxhYmM", forged} {
		if _, _, err := Decode(bad); !errors.Is(err, ErrInvalidCursor) {
			t.Errorf("%q: err = %v", bad, err)
		}
	}
}

// Formas alternativas aceitas por uuid.Parse (urn:uuid:) são normalizadas para a canônica,
// a única que o Postgres sempre aceita.
func TestDecodeNormalizesUUIDForm(t *testing.T) {
	const id = "0199e7a4-0000-7000-8000-000000000001"
	raw := base64.RawURLEncoding.EncodeToString([]byte("2026-10-14T12:00:00Z|urn:uuid:" + id))
	_, got, err := Decode(raw)
	if err != nil || got == nil || *got != id {
		t.Fatalf("id = %v err = %v", got, err)
	}
}

func TestKeyCursorRoundTrip(t *testing.T) {
	got, err := DecodeKey(EncodeKey("ana.b"))
	if err != nil || got != "ana.b" {
		t.Fatalf("got %q err %v", got, err)
	}
	if got, err := DecodeKey(""); err != nil || got != "" {
		t.Fatalf("vazio: %q %v", got, err)
	}
	for _, bad := range []string{
		base64.RawURLEncoding.EncodeToString([]byte("a\x00b")),
		base64.RawURLEncoding.EncodeToString([]byte{0xff, 0xfe}),
	} {
		if _, err := DecodeKey(bad); !errors.Is(err, ErrInvalidCursor) {
			t.Fatalf("cursor %q aceito: %v", bad, err)
		}
	}
	if _, err := DecodeKey("lixo!"); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("err = %v", err)
	}
}
