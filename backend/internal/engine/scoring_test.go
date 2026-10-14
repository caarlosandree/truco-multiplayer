package engine

import "testing"

// parseResults converte "AB-" em resultados de rodada (A=0, B=1, -=empate).
func parseResults(s string) []*Team {
	out := make([]*Team, 0, len(s))
	for _, r := range s {
		switch r {
		case 'A':
			out = append(out, teamPtr(0))
		case 'B':
			out = append(out, teamPtr(1))
		default:
			out = append(out, nil)
		}
	}
	return out
}

func TestHandWinner(t *testing.T) {
	cases := []struct {
		rounds  string
		winner  string // "A", "B", "" (ninguém)
		decided bool
	}{
		{"A", "", false},
		{"-", "", false},
		{"AA", "A", true},
		{"A-", "A", true},
		{"-A", "A", true},
		{"-B", "B", true},
		{"--", "", false},
		{"AB", "", false},
		{"ABA", "A", true},
		{"ABB", "B", true},
		{"AB-", "A", true},
		{"BA-", "B", true},
		{"--A", "A", true},
		{"--B", "B", true},
		{"---", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.rounds, func(t *testing.T) {
			winner, decided := HandWinner(parseResults(tc.rounds))
			if decided != tc.decided {
				t.Fatalf("decided = %v, quer %v", decided, tc.decided)
			}
			got := ""
			if winner != nil {
				got = string("AB"[*winner])
			}
			if got != tc.winner {
				t.Fatalf("winner = %q, quer %q", got, tc.winner)
			}
		})
	}
}
