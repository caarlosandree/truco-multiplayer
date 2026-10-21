package replay_test

import (
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
)

// Os motivos persistidos precisam coincidir com os do protocolo (match_ended).
func TestEndReasonMatchesProtocol(t *testing.T) {
	pairs := []struct {
		got  replay.EndReason
		want protocol.EndReason
	}{
		{replay.EndGameOver, protocol.EndGameOver},
		{replay.EndForfeit, protocol.EndForfeit},
		{replay.EndAborted, protocol.EndAborted},
		{replay.EndAbandoned, protocol.EndAbandoned},
	}
	for _, p := range pairs {
		if string(p.got) != string(p.want) {
			t.Errorf("replay %q != protocol %q", p.got, p.want)
		}
	}
}
