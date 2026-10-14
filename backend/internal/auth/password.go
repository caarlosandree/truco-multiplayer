package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Params são os parâmetros do argon2id. Memory é em KiB.
type Params struct {
	Memory  uint32
	Time    uint32
	Threads uint8
	SaltLen uint32
	KeyLen  uint32
}

var DefaultParams = Params{Memory: 64 * 1024, Time: 3, Threads: 2, SaltLen: 16, KeyLen: 32}

// TestingParams são baratos de propósito. Use somente em testes.
var TestingParams = Params{Memory: 1024, Time: 1, Threads: 1, SaltLen: 16, KeyLen: 32}

var errMalformedHash = errors.New("auth: hash de senha malformado")

// Hasher calcula e verifica hashes argon2id limitando quantos rodam ao mesmo
// tempo, porque cada um aloca Params.Memory de RAM.
type Hasher struct {
	params    Params
	sem       chan struct{}
	dummy     string
	onAcquire func() // ganchos de teste
	onRelease func()
}

func NewHasher(p Params, maxConcurrent int) *Hasher {
	h := &Hasher{params: p, sem: make(chan struct{}, maxConcurrent)}
	h.dummy, _ = h.Hash("senha-ficticia-para-igualar-tempo")
	return h
}

func (h *Hasher) acquire() {
	h.sem <- struct{}{}
	if h.onAcquire != nil {
		h.onAcquire()
	}
}

func (h *Hasher) release() {
	if h.onRelease != nil {
		h.onRelease()
	}
	<-h.sem
}

func (h *Hasher) Hash(password string) (string, error) {
	salt := make([]byte, h.params.SaltLen)
	_, _ = rand.Read(salt) // crypto/rand.Read nunca falha (Go 1.24+)
	h.acquire()
	key := argon2.IDKey([]byte(password), salt, h.params.Time, h.params.Memory, h.params.Threads, h.params.KeyLen)
	h.release()
	enc := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version,
		h.params.Memory, h.params.Time, h.params.Threads, enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

// Verify compara em tempo constante. needsRehash indica parâmetros diferentes dos atuais.
func (h *Hasher) Verify(password, encoded string) (ok, needsRehash bool, err error) {
	p, salt, key, err := decodeHash(encoded)
	if err != nil {
		return false, false, err
	}
	h.acquire()
	candidate := argon2.IDKey([]byte(password), salt, p.Time, p.Memory, p.Threads, uint32(len(key)))
	h.release()
	ok = subtle.ConstantTimeCompare(candidate, key) == 1
	needsRehash = p.Memory != h.params.Memory || p.Time != h.params.Time || p.Threads != h.params.Threads ||
		uint32(len(salt)) != h.params.SaltLen || uint32(len(key)) != h.params.KeyLen
	return ok, needsRehash, nil
}

// VerifyDummy gasta o tempo de uma verificação para usuários inexistentes.
func (h *Hasher) VerifyDummy(password string) { _, _, _ = h.Verify(password, h.dummy) }

func decodeHash(encoded string) (Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return Params{}, nil, nil, errMalformedHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return Params{}, nil, nil, errMalformedHash
	}
	var p Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.Memory, &p.Time, &p.Threads); err != nil || p.Time == 0 || p.Threads == 0 {
		return Params{}, nil, nil, errMalformedHash
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return Params{}, nil, nil, errMalformedHash
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) == 0 {
		return Params{}, nil, nil, errMalformedHash
	}
	p.SaltLen, p.KeyLen = uint32(len(salt)), uint32(len(key))
	return p, salt, key, nil
}
