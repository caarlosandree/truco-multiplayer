package engine

import (
	"errors"
	"slices"
)

// Este arquivo guarda o estado completo e privado do servidor. Fica fora do tygo
// (tygo.yaml, exclude_files): o cliente só conhece PlayerView.

type PlayedCard struct {
	Seat    Seat `json:"seat"`
	Card    Card `json:"card"`
	Covered bool `json:"covered"`
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
