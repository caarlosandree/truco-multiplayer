package room

import (
	"context"
	"sync"
)

// Actor executa uma sala numa goroutine própria. A caixa de entrada não tem limite,
// então Push nunca bloqueia: o lobby pode chamá-lo segurando o próprio mutex.
type Actor struct {
	room   *Room
	mu     sync.Mutex
	queue  []Msg
	closed bool
	signal chan struct{}
	done   chan struct{}
}

// Start cria a sala (restaurada quando restored != nil) e começa a processar a caixa de entrada.
func Start(ctx context.Context, o Options, restored *Restored) *Actor {
	a := &Actor{signal: make(chan struct{}, 1), done: make(chan struct{})}
	o.Post = func(m Msg) { a.Push(m) }
	if restored != nil {
		a.room = NewRestored(o, *restored)
	} else {
		a.room = New(o)
	}
	a.Push(Wake{})
	go a.run(ctx)
	return a
}

// Push entrega a mensagem; devolve false se o ator já terminou.
func (a *Actor) Push(m Msg) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return false
	}
	a.queue = append(a.queue, m)
	select {
	case a.signal <- struct{}{}:
	default:
	}
	return true
}

// Done fecha quando o ator termina.
func (a *Actor) Done() <-chan struct{} { return a.done }

func (a *Actor) run(ctx context.Context) {
	defer close(a.done)
	for {
		select {
		case <-ctx.Done():
			a.closeQueue()
			return
		case <-a.signal:
		}
		for {
			m, ok := a.pop()
			if !ok {
				break
			}
			a.room.Handle(ctx, m)
			if a.room.Closed() {
				for _, rest := range a.closeQueue() {
					a.room.Handle(ctx, rest) // sala fechada responde Join/Command com room_not_found
				}
				return
			}
		}
	}
}

func (a *Actor) pop() (Msg, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.queue) == 0 {
		return nil, false
	}
	m := a.queue[0]
	a.queue = a.queue[1:]
	return m, true
}

// closeQueue impede novos Push e devolve o que ficou na fila.
func (a *Actor) closeQueue() []Msg {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.closed = true
	rest := a.queue
	a.queue = nil
	return rest
}
