package replay_test

import (
	"reflect"
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/enginetest"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
)

func TestRebuildMatchesLiveStateForRandomGames(t *testing.T) {
	games := 300
	if testing.Short() {
		games = 50
	}
	for seed := range uint64(games) {
		rec, live := enginetest.RandomRecord(seed)
		rebuilt, err := replay.Rebuild(rec)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		if !reflect.DeepEqual(rebuilt, live) {
			t.Fatalf("seed %d: estado reconstruído diverge do estado ao vivo", seed)
		}
	}
}
