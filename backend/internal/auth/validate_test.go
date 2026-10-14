package auth

import (
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	if got := NormalizeUsername("  Ana.Silva "); got != "ana.silva" {
		t.Fatalf("username = %q", got)
	}
	if got := NormalizeEmail(" Ana@Empresa.COM\t"); got != "ana@empresa.com" {
		t.Fatalf("email = %q", got)
	}
}

func TestValidateUsername(t *testing.T) {
	for _, ok := range []string{"ana", "ana.silva", "a_1", "abcdefghijklmnopqrst"} {
		if msg := ValidateUsername(ok); msg != "" {
			t.Errorf("%q rejeitado: %s", ok, msg)
		}
	}
	for _, bad := range []string{"", "ab", "abcdefghijklmnopqrstu", "Ana", "ana silva", "ana-silva", "ána"} {
		if ValidateUsername(bad) == "" {
			t.Errorf("%q aceito", bad)
		}
	}
}

func TestValidateEmail(t *testing.T) {
	domain, msg := ValidateEmail("ana@empresa.com.br")
	if msg != "" || domain != "empresa.com.br" {
		t.Fatalf("domain=%q msg=%q", domain, msg)
	}
	for _, bad := range []string{"", "ana", "ana@", "@empresa.com", "Ana <ana@empresa.com>", "ana@empresa.com, bia@empresa.com"} {
		if _, msg := ValidateEmail(bad); msg == "" {
			t.Errorf("%q aceito", bad)
		}
	}
}

func TestValidateDisplayName(t *testing.T) {
	name, msg := ValidateDisplayName("  Ana 🃏 ")
	if msg != "" || name != "Ana 🃏" {
		t.Fatalf("name=%q msg=%q", name, msg)
	}
	if _, msg := ValidateDisplayName(strings.Repeat("é", 30)); msg != "" {
		t.Fatal("30 runes devem ser aceitas, mesmo com mais de 30 bytes")
	}
	for _, bad := range []string{"", "   ", strings.Repeat("a", 31), "ana\x00", "ana\nbia"} {
		if _, msg := ValidateDisplayName(bad); msg == "" {
			t.Errorf("%q aceito", bad)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	if msg := ValidatePassword("cavalo-bateria-grampo", "ana", "ana@empresa.com"); msg != "" {
		t.Fatalf("senha boa rejeitada: %s", msg)
	}
	cases := map[string]string{
		"curta":                        "curta1234",
		"longa":                        strings.Repeat("x", 129),
		"contém username":              "minhaANA-senha-1",
		"contém parte local do e-mail": "anasilva-2026!",
	}
	for name, pw := range cases {
		email := "ana@empresa.com"
		if name == "contém parte local do e-mail" {
			email = "anasilva@empresa.com"
		}
		if ValidatePassword(pw, "ana", email) == "" {
			t.Errorf("%s: %q aceita", name, pw)
		}
	}
}

func TestFieldErrorsMessage(t *testing.T) {
	err := FieldErrors{"username": "inválido", "email": "inválido"}
	if !strings.Contains(err.Error(), "email") || !strings.Contains(err.Error(), "username") {
		t.Fatalf("Error() = %q", err.Error())
	}
}
