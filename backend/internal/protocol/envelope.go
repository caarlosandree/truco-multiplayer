// Package protocol define as mensagens WebSocket trocadas com o frontend.
// O envelope (este arquivo), o registro e o gerador de uniões ficam fora do tygo;
// os payloads (client.go, server.go) viram tipos TypeScript.
package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
)

// ProtocolVersion muda quando o contrato de mensagens quebra compatibilidade.
const ProtocolVersion = 1

type ClientMessage struct {
	Type    string          `json:"type"`
	Seq     int64           `json:"seq"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type ServerMessageType string

const (
	MsgHello            ServerMessageType = "hello"
	MsgLobbySnapshot    ServerMessageType = "lobby_snapshot"
	MsgRoomUpserted     ServerMessageType = "room_upserted"
	MsgRoomRemoved      ServerMessageType = "room_removed"
	MsgRoomState        ServerMessageType = "room_state"
	MsgLeftRoom         ServerMessageType = "left_room"
	MsgState            ServerMessageType = "state"
	MsgEvents           ServerMessageType = "events"
	MsgForfeitAvailable ServerMessageType = "forfeit_available"
	MsgMatchEnded       ServerMessageType = "match_ended"
	MsgSignal           ServerMessageType = "signal"
	MsgError            ServerMessageType = "error"
)

// ServerMessage é o envelope enviado ao cliente; o tipo do payload é dado por Type (ver registry.go).
type ServerMessage struct {
	Type    ServerMessageType `json:"type"`
	Payload any               `json:"payload"`
}

func Message(t ServerMessageType, payload any) ServerMessage {
	return ServerMessage{Type: t, Payload: payload}
}

var ErrUnknownType = errors.New("protocol: tipo de mensagem desconhecido")

// DecodeAction converte uma mensagem de jogo do cliente em Action.
// O assento vem da sessão; ações de sistema nunca são aceitas do cliente.
func DecodeAction(msg ClientMessage, seat engine.Seat) (engine.Action, error) {
	action := engine.Action{Kind: engine.ActionKind(msg.Type), Seat: seat}
	switch action.Kind {
	case engine.ActChooseCard:
		var p ChooseCardPayload
		if err := Decode(msg.Payload, &p, "keep"); err != nil {
			return engine.Action{}, err
		}
		action.Keep = p.Keep
	case engine.ActBurn:
		var p BurnPayload
		if err := Decode(msg.Payload, &p, "slots"); err != nil {
			return engine.Action{}, err
		}
		action.Slots = p.Slots
	case engine.ActPlayCard:
		var p PlayCardPayload
		if err := Decode(msg.Payload, &p, "slot"); err != nil {
			return engine.Action{}, err
		}
		action.Slot, action.Covered = p.Slot, p.Covered
	case engine.ActVote:
		var p VotePayload
		if err := Decode(msg.Payload, &p, "choice"); err != nil {
			return engine.Action{}, err
		}
		action.Choice = p.Choice
	case engine.ActPassBurn, engine.ActRequestRaise:
	default:
		return engine.Action{}, fmt.Errorf("%w: %q", ErrUnknownType, msg.Type)
	}
	return action, nil
}

// Decode exige um objeto JSON com os campos obrigatórios presentes e sem campos
// desconhecidos, para que um payload vazio ou nulo não vire um comando com valores zero.
func Decode(raw json.RawMessage, target any, required ...string) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return errors.New("protocol: payload ausente ou inválido")
	}
	for _, name := range required {
		if _, present := fields[name]; !present {
			return fmt.Errorf("protocol: campo obrigatório ausente: %s", name)
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

// StateMessage projeta o estado para o assento e anexa os prazos dos assentos pendentes.
func StateMessage(g engine.GameState, seat engine.Seat, deadlines map[engine.Seat]time.Time) ServerMessage {
	p := StatePayload{View: engine.ViewFor(g, seat), Deadlines: map[engine.Seat]int64{}, PendingSeats: []int{}}
	for s, at := range deadlines {
		p.Deadlines[s] = at.UnixMilli()
		p.PendingSeats = append(p.PendingSeats, int(s))
	}
	slices.Sort(p.PendingSeats)
	return Message(MsgState, p)
}

// EventsMessage filtra os eventos para o assento; ok=false quando nada é visível a ele.
func EventsMessage(evs []engine.Event, seat engine.Seat) (ServerMessage, bool) {
	visible := engine.FilterEvents(evs, seat)
	return Message(MsgEvents, visible), len(visible) > 0
}

// ErrorMessage converte o erro de uma jogada: regra do engine vira seu código; o resto, bad_request.
func ErrorMessage(err error, seq int64) ServerMessage {
	var ruleErr *engine.RuleError
	if errors.As(err, &ruleErr) {
		return Message(MsgError, ErrorPayload{Code: string(ruleErr.Code), Message: ruleErr.Msg, Seq: seq})
	}
	return Message(MsgError, ErrorPayload{Code: CodeBadRequest, Message: err.Error(), Seq: seq})
}

func ErrorMsg(code string, seq int64) ServerMessage {
	return Message(MsgError, ErrorPayload{Code: code, Message: errorText(code), Seq: seq})
}

func Hello(u UserInfo) ServerMessage {
	return Message(MsgHello, HelloPayload{User: u, ProtocolVersion: ProtocolVersion})
}

func LobbySnapshot(rooms []RoomSummary) ServerMessage {
	if rooms == nil {
		rooms = []RoomSummary{}
	}
	return Message(MsgLobbySnapshot, LobbySnapshotPayload{Rooms: rooms})
}

func RoomUpserted(r RoomSummary) ServerMessage {
	return Message(MsgRoomUpserted, RoomUpsertedPayload{Room: r})
}

func RoomRemoved(id string) ServerMessage {
	return Message(MsgRoomRemoved, RoomRemovedPayload{RoomID: id})
}

func LeftRoom(reason LeftReason) ServerMessage {
	return Message(MsgLeftRoom, LeftRoomPayload{Reason: reason})
}

func ForfeitAvailable(team *engine.Team) ServerMessage {
	var copied *engine.Team
	if team != nil {
		t := *team
		copied = &t
	}
	return Message(MsgForfeitAvailable, ForfeitAvailablePayload{Team: copied})
}

func MatchEnded(reason EndReason, winner *engine.Team, score [2]int) ServerMessage {
	var copied *engine.Team
	if winner != nil {
		w := *winner
		copied = &w
	}
	return Message(MsgMatchEnded, MatchEndedPayload{Reason: reason, Winner: copied, Score: score})
}

func SignalBroadcast(seat engine.Seat, id SignalID) ServerMessage {
	return Message(MsgSignal, SignalBroadcastPayload{Seat: seat, ID: id})
}
