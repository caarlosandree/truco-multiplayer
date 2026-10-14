package engine

type ActionKind string

const (
	ActChooseCard   ActionKind = "choose_card"
	ActBurn         ActionKind = "burn"
	ActPassBurn     ActionKind = "pass_burn"
	ActPlayCard     ActionKind = "play_card"
	ActRequestRaise ActionKind = "request_raise"
	ActVote         ActionKind = "vote"
	ActSysStartHand ActionKind = "sys_start_hand"
	ActSysTimeout   ActionKind = "sys_timeout"
)

// Action é plana para ser serializável no log de partidas.
// Seat é preenchido pela sala a partir da sessão, nunca pelo cliente.
type Action struct {
	Kind    ActionKind `json:"kind"`
	Seat    Seat       `json:"seat"`
	Keep    int        `json:"keep,omitempty"`
	Slots   []int      `json:"slots,omitempty"`
	Slot    int        `json:"slot,omitempty"`
	Covered bool       `json:"covered,omitempty"`
	Choice  VoteChoice `json:"choice,omitempty"`
}
