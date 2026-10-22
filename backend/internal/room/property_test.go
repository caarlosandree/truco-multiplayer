package room_test

import (
	"math/rand/v2"
	"reflect"
	"testing"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/bot"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/room"
)

// Partidas com jogadas aleatórias (válidas e inválidas), quedas, voltas e passagem de tempo:
// o log gravado sempre reconstrói o estado da sala, cada assento só recebe a própria view
// e nenhum evento invisível a ele.
func TestRandomMatchesStayConsistent(t *testing.T) {
	for seed := uint64(1); seed <= 30; seed++ {
		runRandomMatch(t, seed)
	}
}

func runRandomMatch(t *testing.T, seed uint64) {
	key := [32]byte{byte(seed), byte(seed >> 8)}
	rng := rand.New(rand.NewPCG(seed, 99))
	// Os bots escolhem uma ação legal qualquer (inclusive aumentos e cartas cobertas); a pausa varia.
	thinker := &fakeThinker{auto: func(in bot.Input) bot.Decision { return bot.Decision{Action: in.Legal[rng.IntN(len(in.Legal))]} }}
	delay := time.Duration(seed%3) * 500 * time.Millisecond
	h := newHarness(t, func(o *room.Options) { o.Rand = rand.NewChaCha8(key) }, withBots(thinker, delay))
	if seed%2 == 0 { // partidas pares: o assento 3 é de um bot
		for i := range 3 {
			h.join(i)
		}
		for _, u := range users[1:3] {
			h.cmd(u, protocol.TypeSetReady, protocol.SetReadyPayload{Ready: true})
		}
		startWithBots(h, "u0")
		if h.room.Status() != protocol.RoomPlaying {
			t.Fatalf("seed %d: a partida com bot deveria ter começado", seed)
		}
	} else {
		h.startMatch()
	}
	matchID := h.store.last().ID
	online := [4]bool{true, true, true, true}
	for step := 0; h.room.Status() == protocol.RoomPlaying && !h.room.Closed(); step++ {
		if step > 20000 {
			t.Fatalf("seed %d: a partida não terminou", seed)
		}
		switch roll := rng.IntN(100); {
		case roll < 70:
			h.randomCommand(rng)
		case roll < 80:
			i := rng.IntN(4)
			if online[i] {
				h.send(room.Disconnected{UserID: users[i]})
			} else {
				h.send(room.Connected{UserID: users[i]})
			}
			online[i] = !online[i]
		default:
			h.advance(time.Duration(1+rng.IntN(20)) * time.Second)
		}
		h.checkConsistency(t, seed, matchID)
	}
}

func (h *harness) randomCommand(rng *rand.Rand) {
	g := h.room.Game()
	seat := engine.Seat(rng.IntN(4))
	options := []protocol.ClientMessage{
		clientMsg(engine.Action{Kind: engine.ActChooseCard, Seat: seat, Keep: rng.IntN(2)}),
		clientMsg(engine.Action{Kind: engine.ActPassBurn, Seat: seat}),
		clientMsg(engine.Action{Kind: engine.ActRequestRaise, Seat: seat}),
		clientMsg(engine.Action{Kind: engine.ActPlayCard, Seat: seat, Slot: rng.IntN(3), Covered: rng.IntN(4) == 0}),
		{Type: protocol.TypeClaimForfeit, Seq: 1},
	}
	for _, c := range []engine.VoteChoice{engine.VoteAccept, engine.VoteRun, engine.VoteRaise, engine.VotePlay} {
		options = append(options, clientMsg(engine.Action{Kind: engine.ActVote, Seat: seat, Choice: c}))
	}
	if g.Hand != nil {
		var fours []int
		for slot, c := range g.Hand.Hands[seat] {
			if c.Rank == 4 && !g.Hand.Played[seat][slot] {
				fours = append(fours, slot)
			}
		}
		if len(fours) > 0 {
			options = append(options, clientMsg(engine.Action{Kind: engine.ActBurn, Seat: seat, Slots: fours}))
		}
	}
	h.send(room.Command{UserID: users[seat], Message: options[rng.IntN(len(options))]})
}

func (h *harness) checkConsistency(t *testing.T, seed uint64, matchID string) {
	t.Helper()
	if h.room.Closed() || h.room.Status() != protocol.RoomPlaying {
		h.out.reset()
		return
	}
	live := h.room.Game() // já é um Clone; o estado reconstruído é clonado igualmente (Clone normaliza Rounds nil em vazio)
	rebuilt, err := replay.Rebuild(*h.store.records[matchID])
	if err != nil || !reflect.DeepEqual(rebuilt.Clone(), live) {
		t.Fatalf("seed %d: o log não reconstrói o estado da sala (%v)", seed, err)
	}
	for i, u := range users {
		seat := engine.Seat(i)
		var lastView *engine.PlayerView
		for _, m := range h.out.take(u) {
			switch m.Type {
			case protocol.MsgEvents:
				for _, e := range m.Payload.([]engine.Event) {
					if !e.Visibility.Allows(seat) {
						t.Fatalf("seed %d: assento %d recebeu evento invisível %s", seed, i, e.Kind)
					}
				}
			case protocol.MsgState:
				v := m.Payload.(protocol.StatePayload).View
				lastView = &v
			}
		}
		if lastView != nil && !reflect.DeepEqual(*lastView, engine.ViewFor(live, seat)) {
			t.Fatalf("seed %d: assento %d recebeu uma view diferente de ViewFor", seed, i)
		}
	}
}
