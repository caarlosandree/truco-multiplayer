package bot

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"time"
)

// RunnerOptions configura o Runner. Seed sorteia a semente do rng de cada
// decisão (os testes injetam uma função determinística).
type RunnerOptions struct {
	Strong        Policy
	Fallback      Policy
	Budget        time.Duration
	MaxConcurrent int
	Seed          func() uint64
}

// Runner executa as decisões dos bots fora do ator da sala, com orçamento de
// tempo por decisão e um semáforo global de cálculos simultâneos (§5.4).
type Runner struct {
	strong, fallback Policy
	budget           time.Duration
	sem              chan struct{}
	seed             func() uint64
}

// NewRunner cria o Runner. MaxConcurrent < 1 vira 1.
func NewRunner(o RunnerOptions) *Runner {
	if o.MaxConcurrent < 1 {
		o.MaxConcurrent = 1
	}
	if o.Seed == nil {
		o.Seed = rand.Uint64
	}
	return &Runner{
		strong: o.Strong, fallback: o.Fallback, budget: o.Budget,
		sem: make(chan struct{}, o.MaxConcurrent), seed: o.Seed,
	}
}

// Think calcula a decisão numa goroutine própria e nunca bloqueia o chamador.
// Espera uma vaga no semáforo; se a espera passar de deadline, usa o Fallback.
// Senão chama a política forte com um ctx de min(Budget, até deadline). Se a
// forte entrar em pânico, recupera, registra e usa o Fallback. done é chamado
// exatamente uma vez, de uma goroutine do Runner.
func (r *Runner) Think(in Input, deadline time.Time, done func(Decision)) {
	go func() {
		rng := rand.New(rand.NewPCG(r.seed(), 0xb07))
		wait := time.NewTimer(time.Until(deadline))
		defer wait.Stop()
		select {
		case r.sem <- struct{}{}:
		case <-wait.C:
			done(r.safeDecide(r.fallback, context.Background(), in, rng))
			return
		}
		d, ok := r.runStrong(in, deadline, rng)
		<-r.sem
		if !ok {
			d = r.safeDecide(r.fallback, context.Background(), in, rng)
		}
		done(d)
	}()
}

// runStrong roda a política forte com orçamento; ok=false se ela entrou em pânico.
func (r *Runner) runStrong(in Input, deadline time.Time, rng *rand.Rand) (d Decision, ok bool) {
	budget := r.budget
	if left := time.Until(deadline); left < budget {
		budget = left
	}
	if budget < 0 {
		budget = 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	defer func() {
		if p := recover(); p != nil {
			slog.Error("bot: política forte entrou em pânico; usando a simples", "panic", p)
			d, ok = Decision{}, false
		}
	}()
	return r.strong.Decide(ctx, in, rng), true
}

// safeDecide chama a política de reserva; se ela também entrar em pânico,
// registra e devolve Decision zero (a sala valida a ação contra a janela e a
// descarta, caindo na rede de segurança do prazo).
func (r *Runner) safeDecide(p Policy, ctx context.Context, in Input, rng *rand.Rand) (d Decision) {
	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("bot: política de reserva entrou em pânico", "panic", rec)
			d = Decision{}
		}
	}()
	return p.Decide(ctx, in, rng)
}
