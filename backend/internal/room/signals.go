package room

import (
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/bot"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
)

// signalInterval é o intervalo mínimo entre sinais rápidos de um mesmo jogador.
const signalInterval = 3 * time.Second

func (r *Room) signal(seat engine.Seat, msg protocol.ClientMessage) {
	st := r.seats[seat]
	var p protocol.SignalPayload
	if r.status != protocol.RoomPlaying || protocol.Decode(msg.Payload, &p, "id") != nil || !p.ID.Valid() {
		r.reply(st.userID, protocol.CodeBadRequest, msg.Seq)
		return
	}
	now := r.now()
	if !st.lastSignal.IsZero() && now.Sub(st.lastSignal) < signalInterval {
		r.reply(st.userID, protocol.CodeRateLimited, msg.Seq)
		return
	}
	st.lastSignal = now
	r.signalsSeen = append(r.signalsSeen, bot.SignalSeen{Seat: seat, Signal: bot.Signal(p.ID), HandNumber: r.game.HandNumber})
	r.broadcast(protocol.SignalBroadcast(seat, p.ID))
}
