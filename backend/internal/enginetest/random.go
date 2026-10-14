// Package enginetest gera partidas completas com ações legais aleatórias, no
// formato em que a sala as grava. Uso exclusivo de testes.
package enginetest

import (
	"fmt"
	"math/rand/v2"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
)

var Players = [4]replay.PlayerSeat{
	{UserID: "u0", DisplayName: "Ana"}, {UserID: "u1", DisplayName: "Bia"},
	{UserID: "u2", DisplayName: "Caio"}, {UserID: "u3", DisplayName: "Duda"},
}

type candidate struct {
	action engine.Action
	source replay.Source
}

// RandomRecord joga uma partida até o fim e devolve o log e o estado final ao vivo.
func RandomRecord(seed uint64) (replay.Record, engine.GameState) {
	rng := rand.New(rand.NewPCG(seed, 7))
	rec := replay.Record{
		ID: fmt.Sprintf("seed-%d", seed), Seed: seed, FirstDealer: engine.Seat(seed % 4),
		EngineVersion: engine.RulesVersion, Status: replay.StatusInProgress, Players: Players,
	}
	players := [4]engine.Player{}
	for i, p := range Players {
		players[i] = engine.Player{ID: p.UserID, Name: p.DisplayName}
	}
	g := engine.NewGame(players, seed, rec.FirstDealer)
	for step := 0; g.Phase != engine.PhaseGameOver; step++ {
		if step > 5000 {
			panic(fmt.Sprintf("enginetest: seed %d não terminou", seed))
		}
		options := candidates(g, rng)
		rng.Shuffle(len(options), func(i, j int) { options[i], options[j] = options[j], options[i] })
		applied := false
		for _, c := range options {
			next, _, err := engine.Apply(g, c.action)
			if err != nil {
				continue
			}
			rec.Actions = append(rec.Actions, replay.LoggedAction{Ply: len(rec.Actions), Action: c.action, Source: c.source})
			g, applied = next, true
			break
		}
		if !applied {
			panic(fmt.Sprintf("enginetest: seed %d sem ação legal na fase %s", seed, g.Phase))
		}
	}
	return rec, g
}

func candidates(g engine.GameState, rng *rand.Rand) []candidate {
	if g.Phase == engine.PhaseWaiting || g.Phase == engine.PhaseHandEnd {
		return []candidate{{engine.Action{Kind: engine.ActSysStartHand}, replay.SourceSystem}}
	}
	player := func(a engine.Action) candidate { return candidate{a, replay.SourcePlayer} }
	var out []candidate
	for s := engine.Seat(0); s < 4; s++ {
		out = append(out,
			player(engine.Action{Kind: engine.ActChooseCard, Seat: s, Keep: 0}),
			player(engine.Action{Kind: engine.ActChooseCard, Seat: s, Keep: 1}),
			player(engine.Action{Kind: engine.ActPassBurn, Seat: s}),
		)
		for _, choice := range []engine.VoteChoice{engine.VoteAccept, engine.VoteRun, engine.VoteRaise, engine.VotePlay} {
			out = append(out, player(engine.Action{Kind: engine.ActVote, Seat: s, Choice: choice}))
		}
		for slot := range 3 {
			out = append(out,
				player(engine.Action{Kind: engine.ActPlayCard, Seat: s, Slot: slot}),
				player(engine.Action{Kind: engine.ActPlayCard, Seat: s, Slot: slot, Covered: true}),
			)
		}
		for slot, card := range g.Hand.Hands[s] {
			if card.Rank == 4 {
				out = append(out, player(engine.Action{Kind: engine.ActBurn, Seat: s, Slots: []int{slot}}))
			}
		}
		// Como a sala: o timeout vira a ação concreta de DefaultAction.
		if fallback, ok := engine.DefaultAction(g, s); ok && rng.IntN(10) == 0 {
			out = append(out, candidate{fallback, replay.SourceTimeout})
		}
	}
	// Truco em mão especial é punido e encerra a partida; raramente, para exercitar a punição.
	if g.Hand.Kind == engine.HandNormal || rng.IntN(200) == 0 {
		out = append(out, player(engine.Action{Kind: engine.ActRequestRaise, Seat: g.Hand.Turn}))
	}
	return out
}
