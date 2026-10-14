package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// NewToken gera 32 bytes aleatórios em base64url e o sha256 que vai para o banco.
func NewToken() (token string, hash []byte) {
	raw := make([]byte, 32)
	_, _ = rand.Read(raw) // crypto/rand.Read nunca falha (Go 1.24+)
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, HashToken(token)
}

// HashToken usa sha256: o token já tem 256 bits de entropia, então um hash lento não agrega.
func HashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
