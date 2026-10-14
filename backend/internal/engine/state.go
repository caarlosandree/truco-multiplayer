package engine

import (
	"errors"
	"slices"
)

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

type PlayedCard struct {
	Seat    Seat `json:"seat"`
	Card    Card `json:"card"`
	Covered bool `json:"covered"`
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

// GameState é o estado completo e privado do servidor. Nunca é enviado ao cliente.
type GameState struct {
	Seed       uint64
	HandNumber int // RNG da mão = PCG(Seed, HandNumber)
	Players    [4]Player
	Score      [2]int
	Dealer     Seat
	Phase      Phase
	Hand       *HandState
	Winner     *Team
}

type HandState struct {
	Kind         HandKind
	Deck         []Card // topo = último elemento
	Hands        [4][]Card
	Played       [4][3]bool // slot já jogado; os slots são estáveis
	Discarded    []Card
	CutOffer     []Card
	Vira         *Card
	Manilha      Rank
	Stake        int
	LastRaiser   *Team
	Pending      *PendingRaise
	VotingTeam   *Team
	Votes        [4]*VoteChoice
	BurnDecided  [4]bool
	BurnSlots    [4][]int
	Turn         Seat
	RoundStarter Seat
	Table        []PlayedCard
	Rounds       []RoundResult
}

type RoundResult struct {
	Winner     *Team
	WinnerSeat *Seat
	Starter    Seat
	Cards      []PlayedCard
}

type PendingRaise struct {
	Requester Seat `json:"requester"`
	FromTeam  Team `json:"fromTeam"`
	PrevStake int  `json:"prevStake"`
	ToStake   int  `json:"toStake"`
}

// Cutter é o jogador à direita do dealer.
func (g GameState) Cutter() Seat { return (g.Dealer + 3) % 4 }

// Clone faz cópia profunda; Apply muta apenas o clone.
func (g GameState) Clone() GameState {
	clone := g
	clone.Winner = clonePtr(g.Winner)
	if g.Hand != nil {
		hand := g.Hand.clone()
		clone.Hand = &hand
	}
	return clone
}

func (h HandState) clone() HandState {
	clone := h
	clone.Deck = slices.Clone(h.Deck)
	for s := range 4 {
		clone.Hands[s] = slices.Clone(h.Hands[s])
		clone.BurnSlots[s] = slices.Clone(h.BurnSlots[s])
		clone.Votes[s] = clonePtr(h.Votes[s])
	}
	clone.Discarded = slices.Clone(h.Discarded)
	clone.CutOffer = slices.Clone(h.CutOffer)
	clone.Vira = clonePtr(h.Vira)
	clone.LastRaiser = clonePtr(h.LastRaiser)
	clone.Pending = clonePtr(h.Pending)
	clone.VotingTeam = clonePtr(h.VotingTeam)
	clone.Table = slices.Clone(h.Table)
	clone.Rounds = make([]RoundResult, len(h.Rounds))
	for i, r := range h.Rounds {
		r.Winner = clonePtr(r.Winner)
		r.WinnerSeat = clonePtr(r.WinnerSeat)
		r.Cards = slices.Clone(r.Cards)
		clone.Rounds[i] = r
	}
	return clone
}

var errStateNotSerializable = errors.New("engine: GameState é privado; use ViewFor")

// MarshalJSON impede que o estado completo vaze por um json.Marshal acidental.
// O estado só sai do servidor via ViewFor; o banco guarda ações, não estado.
func (GameState) MarshalJSON() ([]byte, error) { return nil, errStateNotSerializable }
