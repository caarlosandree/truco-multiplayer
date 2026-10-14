// Package pagination codifica cursores opacos (instante + id) para paginação por keyset.
package pagination

import (
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidCursor = errors.New("pagination: cursor inválido")

func Encode(t time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(t.UTC().Format(time.RFC3339Nano) + "|" + id))
}

func Decode(cursor string) (*time.Time, *string, error) {
	if cursor == "" {
		return nil, nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, nil, ErrInvalidCursor
	}
	stamp, id, ok := strings.Cut(string(raw), "|")
	if !ok {
		return nil, nil, ErrInvalidCursor
	}
	u, err := uuid.Parse(id)
	if err != nil {
		return nil, nil, ErrInvalidCursor
	}
	at, err := time.Parse(time.RFC3339Nano, stamp)
	if err != nil {
		return nil, nil, ErrInvalidCursor
	}
	// uuid.Parse aceita formas que o Postgres rejeita (urn:uuid:, chaves); devolve a canônica.
	canonical := u.String()
	return &at, &canonical, nil
}
