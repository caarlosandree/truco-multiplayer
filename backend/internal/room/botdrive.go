package room

import (
	"errors"
	"reflect"
	"slices"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/bot"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
)

// botInactiveLead é quanto antes do fim da janela curta o bot age por um humano conectado mas inativo:
// sobra ao humano o resto da janela para agir primeiro.
const botInactiveLead = 250 * time.Millisecond

// botThinkMargin é a folga que o Runner recebe antes do prazo da rede de segurança: o Think termina (inclusive
// o fallback simples, com o semáforo cheio) com tempo de a decisão ser aplicada antes da DefaultAction (§5.4).
// Deve ser >= botInactiveLead, o limite de aplicação da decisão.
const botThinkMargin = 500 * time.Millisecond

// botReq é o pedido de decisão de um assento controlado por bot na janela corrente.
type botReq struct {
	gen      uint64
	window   string
	readyAt  time.Time     // a decisão só é aplicada a partir daqui (pausa de "pensar")
	decision *bot.Decision // resultado válido já recebido, à espera de readyAt
	failed   bool          // resultado inválido: vale a rede de segurança do prazo
}

// botControlled: assento de bot, ou humano ausente, em partida em andamento.
func (r *Room) botControlled(s engine.Seat) bool {
	st := r.seats[s]
	return st != nil && r.status == protocol.RoomPlaying && (st.bot || st.isAbsent())
}

// humanBotControlledIn diz se algum assento humano da dupla t está agora nas mãos de um bot.
func (r *Room) humanBotControlledIn(t engine.Team) bool {
	for s, st := range r.seats {
		if st != nil && !st.bot && engine.TeamOf(engine.Seat(s)) == t && r.botControlled(engine.Seat(s)) {
			return true
		}
	}
	return false
}

// inputFor é o único construtor de bot.Input: só o que o assento enxergaria (ViewFor, eventos já filtrados
// para ele e sinais da mesa) mais as ações legais. Nunca entrega o GameState. As fatias são copiadas porque
// o cálculo corre fora do ator enquanto a sala continua acrescentando eventos.
func (r *Room) inputFor(s engine.Seat) bot.Input {
	return bot.Input{
		View:    engine.ViewFor(r.game, s),
		Events:  slices.Clone(r.seatEvents[s]),
		Signals: slices.Clone(r.signalsSeen),
		Legal:   engine.LegalActions(r.game, s),
	}
}

func (r *Room) botDelay() time.Duration {
	if r.opt.BotDelay == nil {
		return 0
	}
	return max(r.opt.BotDelay(), 0)
}

// obviousMove: jogada óbvia é a última carta, ou seja, as ações play_card usam um único slot (cobrir ou pedir
// aumento não tiram a obviedade; o engine sempre oferece os dois).
func obviousMove(legal []engine.Action) bool {
	slot, found := 0, false
	for _, a := range legal {
		if a.Kind != engine.ActPlayCard {
			continue
		}
		if found && a.Slot != slot {
			return false
		}
		slot, found = a.Slot, true
	}
	return found
}

// pauseFor é a pausa de "pensar": um quarto dela em jogada óbvia.
func pauseFor(obvious bool, delay time.Duration) time.Duration {
	if obvious {
		return delay / 4
	}
	return delay
}

// pruneBotReqs esquece pedidos de outra janela ou de assento que já decidiu (o humano agiu). O pedido em voo
// sobrevive à volta do humano: ele retoma na próxima decisão (spec §4.3); só agir antes descarta o do bot.
func (r *Room) pruneBotReqs() {
	for s, req := range r.botReqs {
		if _, pending := r.deadlines[s]; req.window != r.window || !pending {
			delete(r.botReqs, s)
		}
	}
}

// driveBots dispara o cálculo (fora do ator) para cada assento controlado por bot com decisão pendente
// na janela corrente e ainda sem pedido.
func (r *Room) driveBots(now time.Time) {
	for s := engine.Seat(0); s < 4; s++ {
		st := r.seats[s]
		if st == nil || !r.botControlled(s) {
			continue
		}
		if !st.bot && !r.everBotControlled[s] {
			r.everBotControlled[s] = true
			r.opt.Log.Info("bot assumiu o assento de um humano", "match_id", r.matchID, "seat", s)
		}
		deadline, pending := r.deadlines[s]
		if !pending || r.botReqs[s] != nil || r.opt.Thinker == nil {
			continue
		}
		in := r.inputFor(s)
		if len(in.Legal) == 0 {
			continue
		}
		readyAt := now.Add(pauseFor(obviousMove(in.Legal), r.botDelay()))
		if st.connected && !st.bot { // conectado e inativo: o humano tem a janela curta para agir primeiro
			readyAt = deadline.Add(-botInactiveLead)
		}
		if limit := deadline.Add(-botInactiveLead); readyAt.After(limit) {
			readyAt = limit // a pausa de "pensar" nunca passa do prazo: o bot ainda age antes da DefaultAction
		}
		thinkBy := deadline.Add(-botThinkMargin)
		if thinkBy.Before(now) {
			thinkBy = now
		}
		r.botSeq++
		req := &botReq{gen: r.botSeq, window: r.window, readyAt: readyAt}
		r.botReqs[s] = req
		post, window, seat, gen := r.opt.Post, r.window, s, req.gen
		r.opt.Thinker.Think(in, thinkBy, func(d bot.Decision) {
			post(botDecided{gen: gen, window: window, seat: seat, d: d})
		})
	}
}

// botResult recebe o cálculo de um bot. Resultado de janela velha, de pedido substituído, de assento que
// já decidiu ou de ação que já não é legal é descartado em silêncio: nada é aplicado nem gravado.
func (r *Room) botResult(m botDecided) {
	req := r.botReqs[m.seat]
	if r.status != protocol.RoomPlaying || req == nil || req.gen != m.gen || m.window != r.window ||
		req.window != r.window || req.decision != nil || req.failed {
		return
	}
	legal := slices.ContainsFunc(engine.LegalActions(r.game, m.seat), func(a engine.Action) bool {
		return reflect.DeepEqual(a, m.d.Action)
	})
	if !legal {
		r.opt.Log.Warn("decisão de bot inválida descartada", "match_id", r.matchID, "seat", m.seat, "action", m.d.Action.Kind)
		req.failed = true
		return
	}
	d := m.d
	req.decision = &d
}

// runBotDue aplica a decisão pronta de um bot cuja pausa terminou. Devolve true se aplicou algo.
func (r *Room) runBotDue(now time.Time) bool {
	for s := engine.Seat(0); s < 4; s++ {
		req := r.botReqs[s]
		if req == nil || req.decision == nil || now.Before(req.readyAt) {
			continue
		}
		d := *req.decision
		hand := r.game.HandNumber
		err := r.apply(d.Action, replay.SourceBot)
		switch {
		case errors.Is(err, errPersist):
			req.readyAt = now.Add(retryDelay)
			return false
		case errors.Is(err, errMatchGone):
			return false
		case err != nil:
			r.opt.Log.Error("ação de bot rejeitada pelo engine", "match_id", r.matchID, "seat", s, "err", err)
			delete(r.botReqs, s)
			return false
		}
		if d.Signal != nil && r.status == protocol.RoomPlaying {
			r.emitSignal(s, protocol.SignalID(*d.Signal), hand)
		}
		return true
	}
	return false
}

// emitSignal transmite o sinal de um bot, respeitando o mesmo intervalo mínimo dos humanos (sem resposta de erro).
func (r *Room) emitSignal(seat engine.Seat, id protocol.SignalID, hand int) {
	st := r.seats[seat]
	now := r.now()
	if st == nil || !id.Valid() || (!st.lastSignal.IsZero() && now.Sub(st.lastSignal) < signalInterval) {
		return
	}
	st.lastSignal = now
	r.signalsSeen = append(r.signalsSeen, bot.SignalSeen{Seat: seat, Signal: bot.Signal(id), HandNumber: hand})
	r.broadcast(protocol.SignalBroadcast(seat, id))
}
