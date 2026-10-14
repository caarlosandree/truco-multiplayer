// Package ratelimit implementa limites em memória por chave (IP, e-mail, login).
// Basta porque o servidor roda em instância única.
package ratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// minIdleTTL é o tempo ocioso mínimo antes de uma chave ser descartada pelo Sweep.
const minIdleTTL = 30 * time.Minute

type limiterEntry struct {
	lim      *rate.Limiter
	lastSeen time.Time
}

type Limiter struct {
	mu      sync.Mutex
	limit   rate.Limit
	burst   int
	idleTTL time.Duration
	now     func() time.Time
	entries map[string]*limiterEntry
}

// NewLimiter permite `events` eventos por `per`, com rajada de `events`.
// Uma chave só é descartada pelo Sweep depois de ficar ociosa por max(30min, per):
// assim um balde esgotado não volta cheio antes de ter tido tempo de se recompor.
func NewLimiter(events int, per time.Duration, now func() time.Time) *Limiter {
	idleTTL := max(minIdleTTL, per)
	return &Limiter{limit: rate.Every(per / time.Duration(events)), burst: events, idleTTL: idleTTL, now: now, entries: map[string]*limiterEntry{}}
}

func (l *Limiter) Allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	e, ok := l.entries[key]
	if !ok {
		e = &limiterEntry{lim: rate.NewLimiter(l.limit, l.burst)}
		l.entries[key] = e
	}
	e.lastSeen = now
	reservation := e.lim.ReserveN(now, 1)
	if delay := reservation.DelayFrom(now); delay > 0 {
		reservation.CancelAt(now)
		return false, delay
	}
	return true, 0
}

func (l *Limiter) Sweep() {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := l.now().Add(-l.idleTTL)
	for key, e := range l.entries {
		if e.lastSeen.Before(cutoff) {
			delete(l.entries, key)
		}
	}
}

func (l *Limiter) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.entries)
}
