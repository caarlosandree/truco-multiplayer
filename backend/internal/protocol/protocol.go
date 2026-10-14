// Package protocol define as mensagens WebSocket trocadas com o frontend.
package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
)

type ClientMessage struct {
	Type    string          `json:"type"`
	Seq     int64           `json:"seq"`
	Payload json.RawMessage `json:"payload,omitempty"`
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

type ServerMessageType string

const (
	MsgState  ServerMessageType = "state"
	MsgEvents ServerMessageType = "events"
	MsgError  ServerMessageType = "error"
)

type ServerMessage struct {
	Type    ServerMessageType `json:"type"`
	Payload any               `json:"payload"`
}

type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Seq     int64  `json:"seq"`
}

var ErrUnknownType = errors.New("protocol: tipo de mensagem desconhecido")

// DecodeAction converte uma mensagem do cliente em Action.
// O assento vem da sessão; ações de sistema nunca são aceitas do cliente.
func DecodeAction(msg ClientMessage, seat engine.Seat) (engine.Action, error) {
	action := engine.Action{Kind: engine.ActionKind(msg.Type), Seat: seat}
	switch action.Kind {
	case engine.ActChooseCard:
		var p ChooseCardPayload
		if err := decodePayload(msg.Payload, &p, "keep"); err != nil {
			return engine.Action{}, err
		}
		action.Keep = p.Keep
	case engine.ActBurn:
		var p BurnPayload
		if err := decodePayload(msg.Payload, &p, "slots"); err != nil {
			return engine.Action{}, err
		}
		action.Slots = p.Slots
	case engine.ActPlayCard:
		var p PlayCardPayload
		if err := decodePayload(msg.Payload, &p, "slot"); err != nil {
			return engine.Action{}, err
		}
		action.Slot, action.Covered = p.Slot, p.Covered
	case engine.ActVote:
		var p VotePayload
		if err := decodePayload(msg.Payload, &p, "choice"); err != nil {
			return engine.Action{}, err
		}
		action.Choice = p.Choice
	case engine.ActPassBurn, engine.ActRequestRaise:
	default:
		return engine.Action{}, fmt.Errorf("%w: %q", ErrUnknownType, msg.Type)
	}
	return action, nil
}

// decodePayload exige um objeto JSON com os campos obrigatórios presentes,
// para que um payload vazio ou nulo não vire uma jogada com valores zero.
func decodePayload(raw json.RawMessage, target any, required ...string) error {
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

func StateMessage(g engine.GameState, seat engine.Seat) ServerMessage {
	return ServerMessage{Type: MsgState, Payload: engine.ViewFor(g, seat)}
}

func EventsMessage(evs []engine.Event, seat engine.Seat) ServerMessage {
	return ServerMessage{Type: MsgEvents, Payload: engine.FilterEvents(evs, seat)}
}

func ErrorMessage(err error, seq int64) ServerMessage {
	code := "bad_request"
	var ruleErr *engine.RuleError
	if errors.As(err, &ruleErr) {
		code = string(ruleErr.Code)
	}
	return ServerMessage{Type: MsgError, Payload: ErrorPayload{Code: code, Message: err.Error(), Seq: seq}}
}
