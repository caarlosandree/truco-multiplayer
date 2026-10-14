package engine

// ManilhaRank é o rank imediatamente acima da vira, de forma cíclica (3 → 4).
func ManilhaRank(vira Card) Rank {
	return rankLadder[(rankIndex(vira.Rank)+1)%len(rankLadder)]
}

// Strength: coberta = 0; comum = 1..10; manilha = 11..14 (desempate por naipe).
func Strength(c Card, covered bool, manilha Rank) int {
	if covered {
		return 0
	}
	if c.Rank == manilha {
		return 11 + int(c.Suit)
	}
	return 1 + rankIndex(c.Rank)
}

// RoundWinner compara a maior carta de cada dupla; devolve nil, nil em empate.
func RoundWinner(table []PlayedCard, manilha Rank) (*Team, *Seat) {
	best := [2]int{-1, -1}
	var bestSeat [2]Seat
	for _, played := range table {
		team := TeamOf(played.Seat)
		strength := Strength(played.Card, played.Covered, manilha)
		if strength > best[team] {
			best[team] = strength
			bestSeat[team] = played.Seat
		}
	}
	if best[0] == best[1] {
		return nil, nil
	}
	winner := Team(0)
	if best[1] > best[0] {
		winner = 1
	}
	return teamPtr(winner), seatPtr(bestSeat[winner])
}
