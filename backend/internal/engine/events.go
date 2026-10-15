package engine

type VisibilityKind string

const (
	VisAll  VisibilityKind = "all"
	VisSeat VisibilityKind = "seat"
	VisTeam VisibilityKind = "team"
)

type Visibility struct {
	Kind VisibilityKind
	Seat Seat
	Team Team
}

func (v Visibility) Allows(seat Seat) bool {
	switch v.Kind {
	case VisAll:
		return true
	case VisSeat:
		return v.Seat == seat
	case VisTeam:
		return TeamOf(seat) == v.Team
	}
	return false
}

var visAll = Visibility{Kind: VisAll}

func visSeat(s Seat) Visibility { return Visibility{Kind: VisSeat, Seat: s} }
func visTeam(t Team) Visibility { return Visibility{Kind: VisTeam, Team: t} }

type EventKind string

const (
	EvHandStarted    EventKind = "hand_started"
	EvCutOffered     EventKind = "cut_offered"
	EvCardChosen     EventKind = "card_chosen"
	EvCardsDealt     EventKind = "cards_dealt"
	EvFoursBurned    EventKind = "fours_burned"
	EvViraRevealed   EventKind = "vira_revealed"
	EvElevenDecided  EventKind = "eleven_decided"
	EvCardPlayed     EventKind = "card_played"
	EvRoundEnded     EventKind = "round_ended"
	EvRaiseRequested EventKind = "raise_requested"
	EvVoteCast       EventKind = "vote_cast"
	EvRaiseResolved  EventKind = "raise_resolved"
	EvRaisePunished  EventKind = "raise_punished"
	EvHandEnded      EventKind = "hand_ended"
	EvGameOver       EventKind = "game_over"
	EvForfeited      EventKind = "forfeited"
)

// Event serve para animações; nunca carrega cartas privadas.
type Event struct {
	Kind       EventKind  `json:"kind"`
	Visibility Visibility `json:"-"`
	Seat       *Seat      `json:"seat,omitempty"`
	Team       *Team      `json:"team,omitempty"`
	Card       *CardID    `json:"card,omitempty"`
	Count      int        `json:"count,omitempty"`
	Stake      int        `json:"stake,omitempty"`
	Choice     VoteChoice `json:"choice,omitempty"`
	Score      *[2]int    `json:"score,omitempty"`
}

func FilterEvents(evs []Event, seat Seat) []Event {
	visible := []Event{}
	for _, e := range evs {
		if e.Visibility.Allows(seat) {
			visible = append(visible, e)
		}
	}
	return visible
}
