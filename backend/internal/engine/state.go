package engine

// Seat é a posição na mesa (0..3, sentido horário).
type Seat uint8

// Team é a dupla: 0 = assentos 0 e 2; 1 = assentos 1 e 3.
type Team uint8

func TeamOf(s Seat) Team  { return Team(s % 2) }
func Partner(s Seat) Seat { return (s + 2) % 4 }
func Next(s Seat) Seat    { return (s + 1) % 4 }
func Other(t Team) Team   { return 1 - t }

func teamPtr(t Team) *Team { return &t }
func seatPtr(s Seat) *Seat { return &s }

func clonePtr[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

type Phase string

const (
	PhaseWaiting               Phase = "waiting"
	PhaseCutChoice             Phase = "cut_choice"
	PhaseDealerChoice          Phase = "dealer_choice"
	PhaseBurnFours             Phase = "burn_fours"
	PhaseElevenDecision        Phase = "eleven_decision"
	PhasePlaying               Phase = "playing"
	PhaseAwaitingRaiseResponse Phase = "awaiting_raise_response"
	PhaseHandEnd               Phase = "hand_end"
	PhaseGameOver              Phase = "game_over"
)

type HandKind string

const (
	HandNormal HandKind = "normal"
	HandEleven HandKind = "eleven"
	HandIron   HandKind = "iron"
)

type VoteChoice string

const (
	VoteAccept VoteChoice = "accept"
	VoteRun    VoteChoice = "run"
	VoteRaise  VoteChoice = "raise"
	VotePlay   VoteChoice = "play"
)

const WinningScore = 12

type Player struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type PendingRaise struct {
	Requester Seat `json:"requester"`
	FromTeam  Team `json:"fromTeam"`
	PrevStake int  `json:"prevStake"`
	ToStake   int  `json:"toStake"`
}
