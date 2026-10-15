// Package clock abstrai o tempo para que timers e expirações sejam determinísticos nos testes.
package clock

import (
	"sync"
	"time"
)

// Timer é um disparo agendado que pode ser cancelado.
type Timer interface {
	Stop() bool
}

// Clock fornece o instante atual e agenda funções.
type Clock interface {
	Now() time.Time
	AfterFunc(d time.Duration, f func()) Timer
}

// Real usa o relógio do sistema.
type Real struct{}

func (Real) Now() time.Time { return time.Now() }

func (Real) AfterFunc(d time.Duration, f func()) Timer { return time.AfterFunc(d, f) }

// Manual é um relógio controlado pelo teste, seguro para uso concorrente.
type Manual struct {
	mu     sync.Mutex
	now    time.Time
	seq    uint64
	timers []*manualTimer
}

type manualTimer struct {
	clock *Manual
	at    time.Time
	seq   uint64
	f     func()
	done  bool // disparado ou parado; protegido por clock.mu
}

// NewManual cria um relógio começando em t.
func NewManual(t time.Time) *Manual { return &Manual{now: t} }

func (c *Manual) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *Manual) AfterFunc(d time.Duration, f func()) Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.seq++
	t := &manualTimer{clock: c, at: c.now.Add(d), seq: c.seq, f: f}
	c.timers = append(c.timers, t)
	return t
}

func (t *manualTimer) Stop() bool {
	c := t.clock
	c.mu.Lock()
	defer c.mu.Unlock()
	if t.done {
		return false
	}
	t.done = true
	c.remove(t)
	return true
}

// Advance avança o relógio em d e dispara, em ordem de prazo, os timers vencidos.
// Os callbacks rodam na goroutine de quem chamou, sem o lock, e podem armar novos timers.
func (c *Manual) Advance(d time.Duration) {
	c.mu.Lock()
	target := c.now.Add(d)
	c.mu.Unlock()
	for {
		c.mu.Lock()
		next := c.earliestDue(target)
		if next == nil {
			c.now = target
			c.mu.Unlock()
			return
		}
		next.done = true
		c.remove(next)
		if next.at.After(c.now) {
			c.now = next.at
		}
		c.mu.Unlock()
		next.f()
	}
}

// Pending conta os timers armados e ainda não disparados.
func (c *Manual) Pending() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.timers)
}

func (c *Manual) earliestDue(target time.Time) *manualTimer {
	var best *manualTimer
	for _, t := range c.timers {
		if t.at.After(target) {
			continue
		}
		if best == nil || t.at.Before(best.at) || (t.at.Equal(best.at) && t.seq < best.seq) {
			best = t
		}
	}
	return best
}

func (c *Manual) remove(t *manualTimer) {
	for i, other := range c.timers {
		if other == t {
			c.timers = append(c.timers[:i], c.timers[i+1:]...)
			return
		}
	}
}
