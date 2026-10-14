package engine

type PlayerInfo struct {
	Seat      Seat   `json:"seat"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	CardCount int    `json:"cardCount"`
	Voted     bool   `json:"voted"`
}

type HandSlotView struct {
	Slot   int     `json:"slot"`
	Card   *CardID `json:"card,omitempty"`
	Hidden bool    `json:"hidden"`
}

type TableCardView struct {
	Seat    Seat    `json:"seat"`
	Card    *CardID `json:"card,omitempty"`
	Covered bool    `json:"covered"`
}

type RoundView struct {
	Winner     *Team           `json:"winner"`
	WinnerSeat *Seat           `json:"winnerSeat"`
	Cards      []TableCardView `json:"cards"`
}

// OfferCardView evita []CardID, que o encoding/json serializaria como base64.
type OfferCardView struct {
	Keep int    `json:"keep"`
	Card CardID `json:"card"`
}

// PlayerView é a projeção enviada a um assento. Chaves com carta: só "card" e "vira".
type PlayerView struct {
	Seat         Seat            `json:"seat"`
	Phase        Phase           `json:"phase"`
	HandKind     HandKind        `json:"handKind,omitempty"`
	HandNumber   int             `json:"handNumber"`
	Players      [4]PlayerInfo   `json:"players"`
	Score        [2]int          `json:"score"`
	Dealer       Seat            `json:"dealer"`
	Turn         *Seat           `json:"turn,omitempty"`
	Stake        int             `json:"stake"`
	LastRaiser   *Team           `json:"lastRaiser,omitempty"`
	Pending      *PendingRaise   `json:"pending,omitempty"`
	VotingTeam   *Team           `json:"votingTeam,omitempty"`
	Vira         *CardID         `json:"vira,omitempty"`
	Manilha      *Rank           `json:"manilha,omitempty"`
	Table        []TableCardView `json:"table"`
	Rounds       []RoundView     `json:"rounds"`
	DeckCount    int             `json:"deckCount"`
	DiscardCount int             `json:"discardCount"`
	MyHand       []HandSlotView  `json:"myHand"`
	CutOffer     []OfferCardView `json:"cutOffer,omitempty"`
	PartnerHand  []HandSlotView  `json:"partnerHand,omitempty"`
	PartnerVote  *VoteChoice     `json:"partnerVote,omitempty"`
	Winner       *Team           `json:"winner,omitempty"`
}

// ViewFor é o único caminho pelo qual o estado sai do servidor.
func ViewFor(g GameState, seat Seat) PlayerView {
	v := PlayerView{
		Seat: seat, Phase: g.Phase, HandNumber: g.HandNumber, Score: g.Score, Dealer: g.Dealer,
		Winner: clonePtr(g.Winner), Table: []TableCardView{}, Rounds: []RoundView{}, MyHand: []HandSlotView{},
	}
	for s := range 4 {
		v.Players[s] = PlayerInfo{Seat: Seat(s), ID: g.Players[s].ID, Name: g.Players[s].Name}
	}
	h := g.Hand
	if h == nil {
		return v
	}
	v.HandKind = h.Kind
	v.Stake = h.Stake
	v.LastRaiser = clonePtr(h.LastRaiser)
	v.Pending = clonePtr(h.Pending)
	v.VotingTeam = clonePtr(h.VotingTeam)
	v.DeckCount = len(h.Deck)
	v.DiscardCount = len(h.Discarded)
	if g.Phase == PhasePlaying {
		v.Turn = seatPtr(h.Turn)
	}
	if h.Vira != nil {
		viraID, manilha := h.Vira.ID(), h.Manilha
		v.Vira, v.Manilha = &viraID, &manilha
	}
	for s := range 4 {
		v.Players[s].CardCount = remainingCards(h, Seat(s))
		v.Players[s].Voted = h.Votes[s] != nil
	}
	for _, played := range h.Table {
		v.Table = append(v.Table, tableCardFor(played, seat))
	}
	for _, round := range h.Rounds {
		rv := RoundView{Winner: clonePtr(round.Winner), WinnerSeat: clonePtr(round.WinnerSeat), Cards: []TableCardView{}}
		for _, played := range round.Cards {
			rv.Cards = append(rv.Cards, tableCardFor(played, seat))
		}
		v.Rounds = append(v.Rounds, rv)
	}
	v.MyHand = handSlotsFor(h, seat, h.Kind == HandIron)
	if chooser, choosing := currentChooser(g); choosing && chooser == seat {
		for keep, card := range h.CutOffer {
			v.CutOffer = append(v.CutOffer, OfferCardView{Keep: keep, Card: card.ID()})
		}
	}
	if teamIsVoting(g, seat) {
		v.PartnerHand = handSlotsFor(h, Partner(seat), false)
		v.PartnerVote = clonePtr(h.Votes[Partner(seat)])
	}
	return v
}

func remainingCards(h *HandState, s Seat) int {
	count := 0
	for slot := range h.Hands[s] {
		if !h.Played[s][slot] {
			count++
		}
	}
	return count
}

func handSlotsFor(h *HandState, s Seat, hidden bool) []HandSlotView {
	slots := []HandSlotView{}
	for slot, card := range h.Hands[s] {
		if h.Played[s][slot] {
			continue
		}
		view := HandSlotView{Slot: slot, Hidden: hidden}
		if !hidden {
			id := card.ID()
			view.Card = &id
		}
		slots = append(slots, view)
	}
	return slots
}

func tableCardFor(played PlayedCard, viewer Seat) TableCardView {
	view := TableCardView{Seat: played.Seat, Covered: played.Covered}
	if !played.Covered || played.Seat == viewer {
		id := played.Card.ID()
		view.Card = &id
	}
	return view
}

func currentChooser(g GameState) (Seat, bool) {
	switch g.Phase {
	case PhaseCutChoice:
		return g.Cutter(), true
	case PhaseDealerChoice:
		return g.Dealer, true
	}
	return 0, false
}

func teamIsVoting(g GameState, seat Seat) bool {
	h := g.Hand
	isVotingPhase := g.Phase == PhaseAwaitingRaiseResponse || g.Phase == PhaseElevenDecision
	return isVotingPhase && h.VotingTeam != nil && *h.VotingTeam == TeamOf(seat) && h.Kind != HandIron
}
