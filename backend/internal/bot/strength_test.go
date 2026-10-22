package bot

import (
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
)

// MinGamesCI e MinWinsCI: versão reduzida da meta de §5.5 que roda no CI. 40
// vitórias em 60 partidas dão p < 0,01 contra 50%.
const (
	MinGamesCI = 60
	MinWinsCI  = 40
)

// strongVsSimple joga games partidas (seeds 1..games) contra o Simple.
func strongVsSimple(strong Policy, games int) int { return strongVsSimpleFrom(strong, 1, games) }

func strongVsSimpleFrom(strong Policy, first uint64, games int) int {
	return strongVsOpponent(strong, Simple{}, first, games)
}

// strongVsOpponent joga games partidas (seeds first..first+games-1) e devolve as
// vitórias da dupla forte. O lado da dupla forte é (seed/4)%2, desacoplado do
// dealer inicial (seed%4): em cada 8 seeds a dupla forte passa por todos os
// dealers iniciais nos dois lados.
func strongVsOpponent(strong, opp Policy, first uint64, games int) int {
	wins := 0
	for seed := first; seed < first+uint64(games); seed++ {
		var ps [4]Policy
		strongTeam := engine.Team((seed / 4) % 2)
		for s := range 4 {
			if engine.TeamOf(engine.Seat(s)) == strongTeam {
				ps[s] = strong
			} else {
				ps[s] = opp
			}
		}
		if PlayMatch(ps, seed) == strongTeam {
			wins++
		}
	}
	return wins
}

func TestStrongBeatsSimple(t *testing.T) {
	strong := DefaultStrong()
	strong.Samples = 40 // só no teste de CI; o benchmark (T7) usa DefaultStrong()
	wins := strongVsSimple(strong, MinGamesCI)
	t.Logf("Strong(40 amostras) venceu %d de %d partidas contra o Simple", wins, MinGamesCI)
	if wins < MinWinsCI {
		t.Fatalf("vitórias da forte = %d de %d; mínimo %d", wins, MinGamesCI, MinWinsCI)
	}
}
