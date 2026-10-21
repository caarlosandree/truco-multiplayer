package protocol

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
)

func TestDecodeRoomPayloads(t *testing.T) {
	var create CreateRoomPayload
	if err := Decode(json.RawMessage(`{"visibility":"private"}`), &create, "visibility"); err != nil || create.Visibility != VisibilityPrivate {
		t.Fatalf("create_room: %v %+v", err, create)
	}
	if Visibility("secreta").Valid() {
		t.Fatal("visibilidade desconhecida não é válida")
	}
	var join JoinRoomPayload
	if err := Decode(json.RawMessage(`{"code":"K7QM2P","seat":2}`), &join); err != nil || *join.Code != "K7QM2P" || *join.Seat != 2 || join.RoomID != nil {
		t.Fatalf("join_room: %v %+v", err, join)
	}
	var seat TakeSeatPayload
	if err := Decode(json.RawMessage(`{}`), &seat, "seat"); err == nil {
		t.Fatal("take_seat sem seat deve falhar")
	}
	var signal SignalPayload
	if err := Decode(json.RawMessage(`{"id":"truco","extra":1}`), &signal, "id"); err == nil {
		t.Fatal("campo desconhecido deve falhar")
	}
	if !SignalTruco.Valid() || SignalID("xingar").Valid() {
		t.Fatal("validação de sinais errada")
	}
}

func TestKnownClientType(t *testing.T) {
	for _, spec := range ClientMessages {
		if !KnownClientType(spec.Type) {
			t.Fatalf("%s deveria ser conhecido", spec.Type)
		}
	}
	for _, typ := range []string{"sys_forfeit", "sys_start_hand", "hello", ""} {
		if KnownClientType(typ) {
			t.Fatalf("%q não é tipo de cliente", typ)
		}
	}
}

func TestRegistryTypesAreUnique(t *testing.T) {
	for name, specs := range map[string][]MessageSpec{"cliente": ClientMessages, "servidor": ServerMessages} {
		seen := map[string]bool{}
		for _, s := range specs {
			if seen[s.Type] {
				t.Fatalf("%s: tipo repetido %s", name, s.Type)
			}
			seen[s.Type] = true
		}
	}
}

func TestStateMessageCarriesDeadlinesAndPendingSeats(t *testing.T) {
	g := engine.NewGame([4]engine.Player{}, 1, 0)
	at := time.Date(2026, 10, 14, 12, 0, 30, 0, time.UTC)
	m := StateMessage(g, 2, map[engine.Seat]time.Time{3: at, 1: at.Add(time.Second)})
	p := m.Payload.(StatePayload)
	if m.Type != MsgState || p.View.Seat != 2 || p.Deadlines[3] != at.UnixMilli() || len(p.PendingSeats) != 2 || p.PendingSeats[0] != 1 {
		t.Fatalf("state = %+v", p)
	}
	raw, _ := json.Marshal(m)
	if !strings.Contains(string(raw), `"deadlines":{"1":`) || !strings.Contains(string(raw), `"pendingSeats":[1,3]`) {
		t.Fatalf("JSON = %s", raw)
	}
	empty, _ := json.Marshal(StateMessage(g, 0, nil))
	if !strings.Contains(string(empty), `"deadlines":{}`) || !strings.Contains(string(empty), `"pendingSeats":[]`) {
		t.Fatalf("sem prazos deve serializar vazio, não null: %s", empty)
	}
}

func TestEventsMessageReportsWhetherAnythingIsVisible(t *testing.T) {
	g := engine.NewGame([4]engine.Player{}, 1, 0)
	_, evs, err := engine.Apply(g, engine.Action{Kind: engine.ActSysStartHand})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := EventsMessage(evs, 0); !ok {
		t.Fatal("hand_started é público")
	}
	hidden := []engine.Event{{Kind: engine.EvCutOffered, Visibility: engine.Visibility{Kind: engine.VisSeat, Seat: 3}}}
	if _, ok := EventsMessage(hidden, 0); ok {
		t.Fatal("evento só do assento 3 não é visível ao 0")
	}
}

func TestErrorMsgHasPortugueseTextAndOmitsZeroSeq(t *testing.T) {
	p := ErrorMsg(CodeRoomFull, 0).Payload.(ErrorPayload)
	if p.Code != CodeRoomFull || p.Message != "a sala está cheia" {
		t.Fatalf("erro = %+v", p)
	}
	raw, _ := json.Marshal(ErrorMsg(CodeRoomFull, 0))
	if strings.Contains(string(raw), "seq") {
		t.Fatalf("seq zero deve ser omitido: %s", raw)
	}
}

func TestLobbySnapshotNeverNull(t *testing.T) {
	raw, _ := json.Marshal(LobbySnapshot(nil))
	if !strings.Contains(string(raw), `"rooms":[]`) {
		t.Fatalf("snapshot = %s", raw)
	}
}

func TestDecodeInvitePayloads(t *testing.T) {
	var inv InvitePayload
	if err := Decode(json.RawMessage(`{}`), &inv, "username"); err == nil {
		t.Fatal("invite sem username deveria falhar")
	}
	if err := Decode(json.RawMessage(`{"username":"bia","seat":2}`), &inv, "username"); err != nil || inv.Username != "bia" || *inv.Seat != 2 {
		t.Fatalf("invite = %+v, %v", inv, err)
	}
	var resp InviteRespondPayload
	if err := Decode(json.RawMessage(`{"inviteId":"x"}`), &resp, "inviteId", "accept"); err == nil {
		t.Fatal("invite_respond sem accept deveria falhar")
	}
	if err := Decode(json.RawMessage(`{"accept":true}`), &resp, "inviteId", "accept"); err == nil {
		t.Fatal("invite_respond sem inviteId deveria falhar")
	}
	if err := Decode(json.RawMessage(`{"inviteId":"x","accept":false}`), &resp, "inviteId", "accept"); err != nil || resp.Accept {
		t.Fatalf("invite_respond = %+v, %v", resp, err)
	}
	var tok InviteAcceptTokenPayload
	if err := Decode(json.RawMessage(`{}`), &tok, "token"); err == nil {
		t.Fatal("invite_accept_token sem token deveria falhar")
	}
	if err := Decode(json.RawMessage(`{"token":"abc"}`), &tok, "token"); err != nil || tok.Token != "abc" {
		t.Fatalf("token = %+v, %v", tok, err)
	}
	for _, typ := range []string{TypeInvite, TypeInviteRespond, TypeInviteAcceptToken} {
		if !KnownClientType(typ) {
			t.Fatalf("%s deveria ser tipo conhecido", typ)
		}
	}
}
