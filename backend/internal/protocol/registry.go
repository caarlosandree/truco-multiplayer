package protocol

import (
	"reflect"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
)

// MessageSpec associa um tipo de mensagem ao seu payload (nil = sem payload).
// É a fonte das uniões TypeScript geradas por cmd/gen-protocol.
type MessageSpec struct {
	Type    string
	Payload reflect.Type
}

var ClientMessages = []MessageSpec{
	{TypeCreateRoom, reflect.TypeFor[CreateRoomPayload]()},
	{TypeJoinRoom, reflect.TypeFor[JoinRoomPayload]()},
	{TypeLeaveRoom, nil},
	{TypeTakeSeat, reflect.TypeFor[TakeSeatPayload]()},
	{TypeSetReady, reflect.TypeFor[SetReadyPayload]()},
	{TypeKick, reflect.TypeFor[KickPayload]()},
	{string(engine.ActChooseCard), reflect.TypeFor[ChooseCardPayload]()},
	{string(engine.ActBurn), reflect.TypeFor[BurnPayload]()},
	{string(engine.ActPassBurn), nil},
	{string(engine.ActPlayCard), reflect.TypeFor[PlayCardPayload]()},
	{string(engine.ActRequestRaise), nil},
	{string(engine.ActVote), reflect.TypeFor[VotePayload]()},
	{TypeClaimForfeit, nil},
	{TypeSignal, reflect.TypeFor[SignalPayload]()},
}

var ServerMessages = []MessageSpec{
	{string(MsgHello), reflect.TypeFor[HelloPayload]()},
	{string(MsgLobbySnapshot), reflect.TypeFor[LobbySnapshotPayload]()},
	{string(MsgRoomUpserted), reflect.TypeFor[RoomUpsertedPayload]()},
	{string(MsgRoomRemoved), reflect.TypeFor[RoomRemovedPayload]()},
	{string(MsgRoomState), reflect.TypeFor[RoomStatePayload]()},
	{string(MsgLeftRoom), reflect.TypeFor[LeftRoomPayload]()},
	{string(MsgState), reflect.TypeFor[StatePayload]()},
	{string(MsgEvents), reflect.TypeFor[[]engine.Event]()},
	{string(MsgForfeitAvailable), reflect.TypeFor[ForfeitAvailablePayload]()},
	{string(MsgMatchEnded), reflect.TypeFor[MatchEndedPayload]()},
	{string(MsgSignal), reflect.TypeFor[SignalBroadcastPayload]()},
	{string(MsgPresenceSnapshot), reflect.TypeFor[PresenceSnapshotPayload]()},
	{string(MsgPresenceUpdated), reflect.TypeFor[PresenceUpdatedPayload]()},
	{string(MsgPresenceRemoved), reflect.TypeFor[PresenceRemovedPayload]()},
	{string(MsgError), reflect.TypeFor[ErrorPayload]()},
}

// KnownClientType diz se o tipo é uma mensagem que o cliente pode enviar.
func KnownClientType(t string) bool {
	for _, spec := range ClientMessages {
		if spec.Type == t {
			return true
		}
	}
	return false
}
