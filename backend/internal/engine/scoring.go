package engine

// HandWinner aplica a regra paulista de empates. decided=true com winner=nil
// significa que as três rodadas empataram e ninguém pontua.
func HandWinner(results []*Team) (winner *Team, decided bool) {
	var wins [2]int
	for _, r := range results {
		if r != nil {
			wins[*r]++
		}
	}
	for team := Team(0); team < 2; team++ {
		if wins[team] >= 2 {
			return teamPtr(team), true
		}
	}
	switch len(results) {
	case 2:
		first, second := results[0], results[1]
		if first != nil && second == nil {
			return clonePtr(first), true
		}
		if first == nil && second != nil {
			return clonePtr(second), true
		}
	case 3:
		// Casos restantes: (empate, empate, X) ou (X, Y, empate).
		if results[2] != nil {
			return clonePtr(results[2]), true
		}
		return clonePtr(results[0]), true
	}
	return nil, false
}
