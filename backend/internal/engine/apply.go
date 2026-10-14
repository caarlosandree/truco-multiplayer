package engine

func NewGame(players [4]Player, seed uint64, firstDealer Seat) GameState {
	return GameState{Seed: seed, Players: players, Dealer: firstDealer, Phase: PhaseWaiting}
}

// Apply é o reducer puro: em erro devolve o estado original e nenhum evento.
func Apply(s GameState, a Action) (GameState, []Event, error) {
	next := s.Clone()
	evs, err := dispatch(&next, a)
	if err != nil {
		return s, nil, err
	}
	return next, evs, nil
}

func dispatch(g *GameState, a Action) ([]Event, error) {
	if a.Seat > 3 {
		return nil, ruleErr(ErrInvalidSeat, "assento fora de 0..3")
	}
	if g.Phase == PhaseGameOver {
		return nil, ruleErr(ErrGameOver, "partida encerrada")
	}
	switch a.Kind {
	case ActSysStartHand:
		return startHand(g)
	case ActChooseCard:
		return chooseCard(g, a)
	case ActBurn, ActPassBurn:
		return burn(g, a)
	case ActPlayCard:
		return playCard(g, a)
	}
	return nil, ruleErr(ErrInvalidChoice, "ação desconhecida: "+string(a.Kind))
}
