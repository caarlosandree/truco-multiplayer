package ratelimit

import (
	"testing"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/testclock"
)

var t0 = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

func TestLimiterBurstThenRefill(t *testing.T) {
	clock := testclock.New(t0)
	l := NewLimiter(10, time.Minute, clock.Now)
	for i := range 10 {
		if ok, _ := l.Allow("1.2.3.4"); !ok {
			t.Fatalf("requisição %d bloqueada dentro do burst", i)
		}
	}
	ok, retry := l.Allow("1.2.3.4")
	if ok || retry <= 0 || retry > 6*time.Second {
		t.Fatalf("11ª: ok=%v retry=%v (quer ~6s)", ok, retry)
	}
	if ok, _ := l.Allow("5.6.7.8"); !ok {
		t.Fatal("chaves devem ser independentes")
	}
	clock.Advance(6 * time.Second)
	if ok, _ := l.Allow("1.2.3.4"); !ok {
		t.Fatal("ficha deveria ter sido reposta")
	}
}

func TestLimiterRejectionDoesNotConsume(t *testing.T) {
	clock := testclock.New(t0)
	l := NewLimiter(1, time.Hour, clock.Now)
	l.Allow("k")
	for range 5 {
		l.Allow("k") // rejeitadas não podem empurrar a próxima ficha para frente
	}
	clock.Advance(time.Hour)
	if ok, _ := l.Allow("k"); !ok {
		t.Fatal("após 1h deve liberar")
	}
}

func TestLimiterSweepRemovesIdleKeys(t *testing.T) {
	clock := testclock.New(t0)
	l := NewLimiter(5, time.Minute, clock.Now)
	l.Allow("a")
	clock.Advance(31 * time.Minute)
	l.Allow("b")
	l.Sweep()
	if l.Len() != 1 {
		t.Fatalf("Len = %d, quer 1", l.Len())
	}
	if _, ok := l.entries["b"]; !ok {
		t.Fatal("a chave restante deveria ser \"b\"")
	}
}

func TestLimiterSweepKeepsExhaustedKeyOfHourlyLimit(t *testing.T) {
	clock := testclock.New(t0)
	l := NewLimiter(5, time.Hour, clock.Now)
	for range 5 {
		l.Allow("k")
	}
	clock.Advance(31 * time.Minute)
	l.Sweep()
	if l.Len() != 1 {
		t.Fatalf("Len = %d após Sweep, quer 1 (chave ociosa de 31 min não pode sair em limite de 1h)", l.Len())
	}
	// Em 31 min o balde recompõe 31/12 ≈ 2,58 fichas: exatamente 2 requisições passam.
	allowed := 0
	for range 5 {
		if ok, _ := l.Allow("k"); ok {
			allowed++
		}
	}
	if allowed != 2 {
		t.Fatalf("permitidas = %d, quer 2 (balde não pode voltar cheio)", allowed)
	}
}

func TestLockoutBlocksAfterMaxFailuresInWindow(t *testing.T) {
	clock := testclock.New(t0)
	l := NewLockout(5, 15*time.Minute, 15*time.Minute, clock.Now)
	for range 4 {
		l.Fail("ana")
	}
	if locked, _ := l.Locked("ana"); locked {
		t.Fatal("4 falhas não bloqueiam")
	}
	l.Fail("ana")
	locked, retry := l.Locked("ana")
	if !locked || retry != 15*time.Minute {
		t.Fatalf("locked=%v retry=%v", locked, retry)
	}
	clock.Advance(15 * time.Minute)
	if locked, _ := l.Locked("ana"); locked {
		t.Fatal("bloqueio deveria ter expirado")
	}
}

func TestLockoutForgetsOldFailures(t *testing.T) {
	clock := testclock.New(t0)
	l := NewLockout(5, 15*time.Minute, 15*time.Minute, clock.Now)
	for range 4 {
		l.Fail("ana")
	}
	clock.Advance(16 * time.Minute)
	l.Fail("ana")
	if locked, _ := l.Locked("ana"); locked {
		t.Fatal("falhas fora da janela não contam")
	}
}

func TestLockoutResetAndSweep(t *testing.T) {
	clock := testclock.New(t0)
	l := NewLockout(2, time.Minute, time.Minute, clock.Now)
	l.Fail("ana")
	l.Reset("ana")
	l.Fail("ana")
	if locked, _ := l.Locked("ana"); locked {
		t.Fatal("Reset deveria zerar as falhas")
	}
	l.Fail("bia")
	clock.Advance(2 * time.Minute)
	l.Sweep()
	if l.Len() != 0 {
		t.Fatalf("Len = %d após Sweep", l.Len())
	}
}

func TestLockoutBeginReservesAttemptsInFlight(t *testing.T) {
	clock := testclock.New(t0)
	l := NewLockout(3, 15*time.Minute, 15*time.Minute, clock.Now)
	for i := range 3 {
		if ok, _ := l.Begin("ana"); !ok {
			t.Fatalf("tentativa %d recusada antes do limite", i)
		}
	}
	ok, retry := l.Begin("ana")
	if ok || retry <= 0 {
		t.Fatalf("4ª tentativa simultânea: ok=%v retry=%v", ok, retry)
	}
	if locked, _ := l.Locked("ana"); locked {
		t.Fatal("tentativas em andamento não são bloqueio")
	}
	l.Release("ana") // terminou sem ser falha de credencial: libera a vaga
	if ok, _ := l.Begin("ana"); !ok {
		t.Fatal("Release deveria liberar uma vaga")
	}
	for range 3 {
		l.Fail("ana")
	}
	locked, retry := l.Locked("ana")
	if !locked || retry != 15*time.Minute {
		t.Fatalf("locked=%v retry=%v", locked, retry)
	}
	if ok, retry := l.Begin("ana"); ok || retry != 15*time.Minute {
		t.Fatalf("Begin durante bloqueio: ok=%v retry=%v", ok, retry)
	}
}

func TestLockoutBeginThenResetAndSweepKeepsInFlight(t *testing.T) {
	clock := testclock.New(t0)
	l := NewLockout(2, time.Minute, time.Minute, clock.Now)
	l.Begin("ana")
	l.Reset("ana")
	if ok, _ := l.Begin("ana"); !ok {
		t.Fatal("Reset deveria zerar as reservas")
	}
	clock.Advance(2 * time.Minute)
	l.Sweep()
	if l.Len() != 1 {
		t.Fatal("Sweep não pode descartar chave com tentativa em andamento")
	}
	l.Release("ana")
	l.Release("ana") // Release a mais não pode deixar contagem negativa
	l.Sweep()
	if l.Len() != 0 {
		t.Fatalf("Len = %d após liberar e varrer", l.Len())
	}
	l.Begin("bia")
	l.Begin("bia")
	if ok, _ := l.Begin("bia"); ok {
		t.Fatal("Release extra em outra chave não pode ter liberado vaga aqui")
	}
}
