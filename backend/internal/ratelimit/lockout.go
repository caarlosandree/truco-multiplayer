package ratelimit

import (
	"sync"
	"time"
)

type failures struct {
	times       []time.Time
	lockedUntil time.Time
}

// Lockout bloqueia uma chave por `lock` depois de `maxFailures` falhas dentro de `window`.
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

func (l *Lockout) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	f, ok := l.entries[key]
	if !ok {
		f = &failures{}
		l.entries[key] = f
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
		if len(f.times) == 0 && !f.lockedUntil.After(now) {
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
