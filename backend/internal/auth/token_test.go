package auth

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestNewToken(t *testing.T) {
	token, hash := NewToken()
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != 32 {
		t.Fatalf("token %q: %d bytes, err %v", token, len(raw), err)
	}
	if !bytes.Equal(hash, HashToken(token)) || len(hash) != 32 {
		t.Fatal("hash deve ser sha256 do token")
	}
	other, _ := NewToken()
	if other == token {
		t.Fatal("tokens repetidos")
	}
}
