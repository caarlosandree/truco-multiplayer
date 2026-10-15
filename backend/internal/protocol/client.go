package protocol

import "github.com/caarlosandree/truco-multiplayer/backend/internal/engine"

// Tipos de mensagem de sala/social enviados pelo cliente. As de jogo usam engine.ActionKind.
const (
	TypeCreateRoom   = "create_room"
	TypeJoinRoom     = "join_room"
	TypeLeaveRoom    = "leave_room"
	TypeTakeSeat     = "take_seat"
	TypeSetReady     = "set_ready"
	TypeKick         = "kick"
	TypeClaimForfeit = "claim_forfeit"
	TypeSignal       = "signal"
)

type Visibility string

const (
	VisibilityPublic  Visibility = "public"
	VisibilityPrivate Visibility = "private"
)

func (v Visibility) Valid() bool { return v == VisibilityPublic || v == VisibilityPrivate }

type CreateRoomPayload struct {
	Visibility Visibility `json:"visibility"`
}

// JoinRoomPayload exige exatamente um de RoomID (sala pública da lista) ou Code (qualquer sala).
type JoinRoomPayload struct {
	RoomID *string      `json:"roomId,omitempty"`
	Code   *string      `json:"code,omitempty"`
	Seat   *engine.Seat `json:"seat,omitempty"`
}

type TakeSeatPayload struct {
	Seat engine.Seat `json:"seat"`
}

type SetReadyPayload struct {
	Ready bool `json:"ready"`
}

type KickPayload struct {
	Seat engine.Seat `json:"seat"`
}

type SignalID string

const (
	SignalTruco    SignalID = "truco"
	SignalBoa      SignalID = "boa"
	SignalVamos    SignalID = "vamos"
	SignalCalma    SignalID = "calma"
	SignalDesculpa SignalID = "desculpa"
	SignalValeu    SignalID = "valeu"
)

func (id SignalID) Valid() bool {
	switch id {
	case SignalTruco, SignalBoa, SignalVamos, SignalCalma, SignalDesculpa, SignalValeu:
		return true
	}
	return false
}

type SignalPayload struct {
	ID SignalID `json:"id"`
}

type ChooseCardPayload struct {
	Keep int `json:"keep"`
}

type BurnPayload struct {
	Slots []int `json:"slots"`
}

type PlayCardPayload struct {
	Slot    int  `json:"slot"`
	Covered bool `json:"covered"`
}

type VotePayload struct {
	Choice engine.VoteChoice `json:"choice"`
}
