package ratelimit

import (
	"sync"
	"time"
)

type failures struct {
	times       []time.Time
	lockedUntil time.Time
	inFlight    int // tentativas reservadas por Begin e ainda sem desfecho
}

// inFlightRetry é o Retry-After sugerido quando a chave está cheia de tentativas em andamento.
const inFlightRetry = time.Second

// Lockout bloqueia uma chave por `lock` depois de `maxFailures` falhas dentro de `window`.
//
// Uso com reserva (resistente a rajadas paralelas): Begin antes de verificar a senha e, depois,
// exatamente um entre Fail (credencial errada), Reset (sucesso) ou Release (outro desfecho).
type Lockout struct {
	mu          sync.Mutex
	maxFailures int
	window      time.Duration
	lock        time.Duration
	now         func() time.Time
	entries     map[string]*failures
}

func NewLockout(maxFailures int, window, lock time.Duration, now func() time.Time) *Lockout {
	return &Lockout{maxFailures: maxFailures, window: window, lock: lock, now: now, entries: map[string]*failures{}}
}

func (l *Lockout) Locked(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	f, ok := l.entries[key]
	if !ok {
		return false, 0
	}
	if remaining := f.lockedUntil.Sub(l.now()); remaining > 0 {
		return true, remaining
	}
	return false, 0
}

// Begin reserva uma tentativa: falhas recentes mais tentativas em andamento nunca passam de
// maxFailures, então N requisições simultâneas não verificam mais senhas do que o limite permite.
func (l *Lockout) Begin(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	f, ok := l.entries[key]
	if !ok {
		f = &failures{}
		l.entries[key] = f
	}
	if remaining := f.lockedUntil.Sub(now); remaining > 0 {
		return false, remaining
	}
	f.times = recent(f.times, now.Add(-l.window))
	if len(f.times)+f.inFlight >= l.maxFailures {
		return false, inFlightRetry
	}
	f.inFlight++
	return true, 0
}

// Release devolve a reserva de uma tentativa que terminou sem ser falha de credencial.
func (l *Lockout) Release(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if f, ok := l.entries[key]; ok && f.inFlight > 0 {
		f.inFlight--
	}
}

// Fail registra uma falha; se havia reserva de Begin, ela é consumida.
func (l *Lockout) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	f, ok := l.entries[key]
	if !ok {
		f = &failures{}
		l.entries[key] = f
	}
	if f.inFlight > 0 {
		f.inFlight--
	}
	f.times = recent(f.times, now.Add(-l.window))
	f.times = append(f.times, now)
	if len(f.times) >= l.maxFailures {
		f.lockedUntil = now.Add(l.lock)
		f.times = nil
	}
}

func (l *Lockout) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}

func (l *Lockout) Sweep() {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	for key, f := range l.entries {
		f.times = recent(f.times, now.Add(-l.window))
		if len(f.times) == 0 && f.inFlight == 0 && !f.lockedUntil.After(now) {
			delete(l.entries, key)
		}
	}
}

func (l *Lockout) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.entries)
}

func recent(times []time.Time, cutoff time.Time) []time.Time {
	kept := times[:0]
	for _, t := range times {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	return kept
}
