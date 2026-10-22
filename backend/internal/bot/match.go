package bot

import (
	"context"
	"fmt"
	"math/rand/v2"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
)

// inputFromState monta o Input de um assento a partir do estado real. Só
// PlayMatch e os testes usam: nenhuma função exportada recebe GameState de
// partida real (a sala monta o Input por conta própria, só com ViewFor e
// FilterEvents). evs são todos os eventos da partida; o filtro é por assento.
func inputFromState(g engine.GameState, seat engine.Seat, evs []engine.Event) Input {
	return Input{
		View:   engine.ViewFor(g, seat),
		Events: engine.FilterEvents(evs, seat),
		Legal:  engine.LegalActions(g, seat),
	}
}

// PlayMatch joga uma partida completa, do NewGame ao game_over, com uma política
// por assento (assentos 0 e 2 são a dupla 0), e devolve a dupla vencedora. É
// determinístico para as mesmas políticas e seed. Os sinais devolvidos pelas
// políticas viram SignalSeen para todos os assentos, como na mesa real. As
// decisões continuam saindo só do Input de cada assento.
func PlayMatch(policies [4]Policy, seed uint64) engine.Team {
	players := [4]engine.Player{{ID: "p0", Name: "P0"}, {ID: "p1", Name: "P1"}, {ID: "p2", Name: "P2"}, {ID: "p3", Name: "P3"}}
	g := engine.NewGame(players, seed, engine.Seat(seed%4))
	rng := rand.New(rand.NewPCG(seed, 0x7011))
	var (
		evs  []engine.Event
		sigs []SignalSeen
	)
	const maxSteps = 20000
	for step := 0; g.Phase != engine.PhaseGameOver; step++ {
		if step >= maxSteps {
			panic(fmt.Sprintf("bot.PlayMatch(seed %d): partida não terminou em %d passos (fase %s)", seed, maxSteps, g.Phase))
		}
		a := engine.Action{Kind: engine.ActSysStartHand}
		decided := false
		for seat := engine.Seat(0); seat < 4; seat++ {
			if _, pending := engine.DefaultAction(g, seat); !pending {
				continue
			}
			p := policies[seat]
			if p == nil {
				panic(fmt.Sprintf("bot.PlayMatch(seed %d): política nula no assento %d (fase %s)", seed, seat, g.Phase))
			}
			in := inputFromState(g, seat, evs)
			in.Signals = sigs
			if len(in.Legal) == 0 {
				panic(fmt.Sprintf("bot.PlayMatch(seed %d): assento %d com decisão pendente e sem ações legais (fase %s)", seed, seat, g.Phase))
			}
			d := p.Decide(context.Background(), in, rng)
			if d.Signal != nil {
				sigs = append(sigs[:len(sigs):len(sigs)], SignalSeen{Seat: seat, Signal: *d.Signal, HandNumber: g.HandNumber})
			}
			a, decided = d.Action, true
			break
		}
		if !decided && g.Phase != engine.PhaseWaiting && g.Phase != engine.PhaseHandEnd {
			panic(fmt.Sprintf("bot.PlayMatch(seed %d): ninguém tem decisão pendente na fase %s", seed, g.Phase))
		}
		next, newEvs, err := engine.Apply(g, a)
		if err != nil {
			panic(fmt.Sprintf("bot.PlayMatch(seed %d): ação %+v rejeitada pelo engine: %v", seed, a, err))
		}
		g, evs = next, append(evs, newEvs...)
	}
	if g.Winner == nil {
		panic(fmt.Sprintf("bot.PlayMatch(seed %d): game_over sem vencedor", seed))
	}
	return *g.Winner
}
