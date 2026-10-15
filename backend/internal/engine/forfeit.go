package engine

// forfeit encerra a partida por W.O.: a dupla a.Team perde e a adversária vai a 12.
// A mão corrente fica como está; só a sala envia esta ação (nunca o cliente).
func forfeit(g *GameState, a Action) ([]Event, error) {
	if a.Team > 1 {
		return nil, ruleErr(ErrInvalidChoice, "dupla inválida")
	}
	winner := Other(a.Team)
	g.Score[winner] = WinningScore
	evs := []Event{{Kind: EvForfeited, Visibility: visAll, Team: teamPtr(a.Team)}}
	return append(evs, finishGame(g, winner)...), nil
}
