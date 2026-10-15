package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
)

func msg(t *testing.T, raw string) ClientMessage {
	t.Helper()
	var m ClientMessage
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestDecodeActionUsesSessionSeat(t *testing.T) {
	cases := []struct {
		raw  string
		want engine.Action
	}{
		{`{"type":"choose_card","seq":1,"payload":{"keep":1}}`, engine.Action{Kind: engine.ActChooseCard, Seat: 2, Keep: 1}},
		{`{"type":"burn","seq":2,"payload":{"slots":[0,2]}}`, engine.Action{Kind: engine.ActBurn, Seat: 2, Slots: []int{0, 2}}},
		{`{"type":"pass_burn","seq":3}`, engine.Action{Kind: engine.ActPassBurn, Seat: 2}},
		{`{"type":"play_card","seq":4,"payload":{"slot":1,"covered":true}}`, engine.Action{Kind: engine.ActPlayCard, Seat: 2, Slot: 1, Covered: true}},
		{`{"type":"request_raise","seq":5}`, engine.Action{Kind: engine.ActRequestRaise, Seat: 2}},
		{`{"type":"vote","seq":6,"payload":{"choice":"accept"}}`, engine.Action{Kind: engine.ActVote, Seat: 2, Choice: engine.VoteAccept}},
	}
	for _, tc := range cases {
		got, err := DecodeAction(msg(t, tc.raw), 2)
		if err != nil {
			t.Fatalf("%s: %v", tc.raw, err)
		}
		if fmt.Sprintf("%+v", got) != fmt.Sprintf("%+v", tc.want) {
			t.Fatalf("%s: got %+v, quer %+v", tc.raw, got, tc.want)
		}
	}
}

func TestDecodeActionRejectsSystemAndUnknownTypes(t *testing.T) {
	for _, typ := range []string{"sys_start_hand", "sys_timeout", "sys_forfeit", "hack"} {
		_, err := DecodeAction(msg(t, `{"type":"`+typ+`","seq":1}`), 0)
		if !errors.Is(err, ErrUnknownType) {
			t.Fatalf("%s: erro = %v", typ, err)
		}
	}
}

func TestDecodeActionRejectsSeatSmuggledInPayload(t *testing.T) {
	_, err := DecodeAction(msg(t, `{"type":"play_card","seq":1,"payload":{"slot":0,"seat":3}}`), 0)
	if err == nil {
		t.Fatal("campo desconhecido no payload deve ser rejeitado")
	}
	_, err = DecodeAction(msg(t, `{"type":"play_card","seq":1}`), 0)
	if err == nil {
		t.Fatal("payload ausente deve ser rejeitado")
	}
}

func TestErrorMessageMapsRuleErrorCode(t *testing.T) {
	_, _, ruleErr := engine.Apply(engine.NewGame([4]engine.Player{}, 1, 0), engine.Action{Kind: engine.ActPlayCard})
	m := ErrorMessage(ruleErr, 9)
	p := m.Payload.(ErrorPayload)
	if m.Type != MsgError || p.Code != "invalid_phase" || p.Seq != 9 {
		t.Fatalf("mensagem = %+v", m)
	}
	if ErrorMessage(errors.New("json ruim"), 1).Payload.(ErrorPayload).Code != "bad_request" {
		t.Fatal("erros de decodificação viram bad_request")
	}
}

func TestStateAndEventsMessagesAreFilteredPerSeat(t *testing.T) {
	g := engine.NewGame([4]engine.Player{}, 1, 0)
	g, evs, err := engine.Apply(g, engine.Action{Kind: engine.ActSysStartHand})
	if err != nil {
		t.Fatal(err)
	}
	if StateMessage(g, 0, nil).Payload.(StatePayload).View.CutOffer != nil {
		t.Fatal("assento 0 não corta nesta mão")
	}
	visible := func(seat engine.Seat) int {
		m, _ := EventsMessage(evs, seat)
		return len(m.Payload.([]engine.Event))
	}
	if visible(3) != 2 || visible(0) != 1 {
		t.Fatal("CutOffered só para quem corta (assento 3)")
	}
}

func TestDecodeActionRejectsEmptyOrNullPayload(t *testing.T) {
	for _, raw := range []string{
		`{"type":"play_card","seq":1,"payload":null}`,
		`{"type":"play_card","seq":1,"payload":{}}`,
		`{"type":"choose_card","seq":1,"payload":{}}`,
		`{"type":"burn","seq":1,"payload":{}}`,
		`{"type":"vote","seq":1,"payload":{}}`,
	} {
		if _, err := DecodeAction(msg(t, raw), 0); err == nil {
			t.Fatalf("%s: payload sem campos obrigatórios deve ser rejeitado", raw)
		}
	}
}
