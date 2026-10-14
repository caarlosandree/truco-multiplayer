package auth

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	h := NewHasher(TestingParams, 2)
	encoded, err := h.Hash("senha-muito-boa")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$v=19$m=1024,t=1,p=1$") {
		t.Fatalf("formato PHC inesperado: %s", encoded)
	}
	ok, rehash, err := h.Verify("senha-muito-boa", encoded)
	if err != nil || !ok || rehash {
		t.Fatalf("ok=%v rehash=%v err=%v", ok, rehash, err)
	}
	ok, _, err = h.Verify("senha-errada!!", encoded)
	if err != nil || ok {
		t.Fatalf("senha errada aceita: ok=%v err=%v", ok, err)
	}
}

func TestHashUsesRandomSalt(t *testing.T) {
	h := NewHasher(TestingParams, 2)
	a, _ := h.Hash("mesma-senha-123")
	b, _ := h.Hash("mesma-senha-123")
	if a == b {
		t.Fatal("dois hashes da mesma senha não podem ser iguais")
	}
}

func TestVerifyFlagsRehashWhenParamsChange(t *testing.T) {
	old := NewHasher(TestingParams, 2)
	encoded, _ := old.Hash("senha-antiga-123")
	stronger := TestingParams
	stronger.Time = 2
	ok, rehash, err := NewHasher(stronger, 2).Verify("senha-antiga-123", encoded)
	if err != nil || !ok || !rehash {
		t.Fatalf("ok=%v rehash=%v err=%v", ok, rehash, err)
	}
}

func TestVerifyRejectsMalformedHash(t *testing.T) {
	h := NewHasher(TestingParams, 2)
	for _, bad := range []string{"", "x", "$argon2i$v=19$m=1,t=1,p=1$c2FsdA$aGFzaA", "$argon2id$v=18$m=1024,t=1,p=1$c2FsdA$aGFzaA",
		"$argon2id$v=19$m=abc$c2FsdA$aGFzaA", "$argon2id$v=19$m=1024,t=1,p=1$***$aGFzaA"} {
		if _, _, err := h.Verify("qualquer-coisa", bad); err == nil {
			t.Errorf("hash malformado aceito: %q", bad)
		}
	}
}

func TestHasherLimitsConcurrency(t *testing.T) {
	h := NewHasher(TestingParams, 2)
	var current, peak atomic.Int32
	h.onAcquire = func() {
		n := current.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
	}
	h.onRelease = func() { current.Add(-1) }
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = h.Hash("concorrencia-123")
		}()
	}
	wg.Wait()
	if peak.Load() > 2 {
		t.Fatalf("pico de %d hashes simultâneos, limite 2", peak.Load())
	}
}
