package engine

import "testing"

func TestSysForfeitEndsGameInAnyPhase(t *testing.T) {
	waiting := newTestGame(1)
	cut, _ := mustApply(t, waiting, Action{Kind: ActSysStartHand})
	playing := dealWithoutBurning(t, cut)
	if playing.Phase != PhasePlaying {
		t.Fatalf("fase inesperada %s", playing.Phase)
	}
	for _, before := range []GameState{waiting, cut, playing} {
		for _, loser := range []Team{0, 1} {
			next, evs := mustApply(t, before, Action{Kind: ActSysForfeit, Team: loser})
			winner := Other(loser)
			if next.Phase != PhaseGameOver || next.Winner == nil || *next.Winner != winner {
				t.Fatalf("fase %s: estado final %s vencedor %v", before.Phase, next.Phase, next.Winner)
			}
			if next.Score[winner] != WinningScore || next.Score[loser] != before.Score[loser] {
				t.Fatalf("fase %s: placar %v", before.Phase, next.Score)
			}
			if len(evs) != 2 || evs[0].Kind != EvForfeited || *evs[0].Team != loser || evs[0].Visibility.Kind != VisAll || evs[1].Kind != EvGameOver {
				t.Fatalf("fase %s: eventos %+v", before.Phase, evs)
			}
			_ = ViewFor(next, 0) // não pode entrar em pânico, nem sem mão
		}
	}
}

func TestSysForfeitRejectsGameOverAndInvalidTeam(t *testing.T) {
	g := newTestGame(1)
	_, _, err := Apply(g, Action{Kind: ActSysForfeit, Team: 2})
	wantCode(t, err, ErrInvalidChoice)
	over, _ := mustApply(t, g, Action{Kind: ActSysForfeit, Team: 0})
	_, _, err = Apply(over, Action{Kind: ActSysForfeit, Team: 1})
	wantCode(t, err, ErrGameOver)
}
