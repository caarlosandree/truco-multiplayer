// Package testclock fornece um relógio controlável para testes de expiração.
package testclock

import (
	"sync"
	"time"
)

// Clock é um relógio manual, seguro para uso concorrente.
type Clock struct {
	mu  sync.Mutex
	now time.Time
}

// New cria um relógio começando em t.
func New(t time.Time) *Clock { return &Clock{now: t} }

// Now devolve o instante atual do relógio.
func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Advance avança o relógio em d.
func (c *Clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}
