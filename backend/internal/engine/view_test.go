package engine

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestViewShowsOwnHandAndOnlyCountsForOthers(t *testing.T) {
	g := playingState(fixtureBWinsTwice(), HandNormal)
	v := ViewFor(g, 0)
	if len(v.MyHand) != 3 || v.MyHand[0].Card == nil || *v.MyHand[0].Card != c(4, Clubs).ID() {
		t.Fatalf("mão = %+v", v.MyHand)
	}
	if v.Players[1].CardCount != 3 || v.PartnerHand != nil || v.CutOffer != nil {
		t.Fatal("outros só aparecem como contagem")
	}
	if v.Turn == nil || *v.Turn != 1 || v.Vira == nil {
		t.Fatal("vez e vira são públicas")
	}
}

func TestViewIronHandHidesOwnCards(t *testing.T) {
	v := ViewFor(playingState(fixtureBWinsTwice(), HandIron), 0)
	for _, slot := range v.MyHand {
		if !slot.Hidden || slot.Card != nil {
			t.Fatalf("slot visível na mão de ferro: %+v", slot)
		}
	}
	if len(v.MyHand) != 3 {
		t.Fatal("os slots continuam presentes para escolha às cegas")
	}
}

func TestViewCoveredCardOnlyVisibleToOwner(t *testing.T) {
	g := playingState(fixtureBWinsTwice(), HandNormal)
	g, _ = mustApply(t, g, Action{Kind: ActPlayCard, Seat: 1, Slot: 0, Covered: true})
	if ViewFor(g, 1).Table[0].Card == nil {
		t.Fatal("o dono vê a própria coberta")
	}
	if other := ViewFor(g, 0).Table[0]; other.Card != nil || !other.Covered {
		t.Fatal("adversário não pode ver a coberta")
	}
	if len(ViewFor(g, 1).MyHand) != 2 {
		t.Fatal("slot jogado sai da mão")
	}
}

func TestViewCutOfferOnlyForChooser(t *testing.T) {
	g, _ := mustApply(t, newTestGame(7), Action{Kind: ActSysStartHand})
	if len(ViewFor(g, 3).CutOffer) != 2 {
		t.Fatal("quem corta vê as 2 cartas")
	}
	for s := Seat(0); s < 3; s++ {
		if ViewFor(g, s).CutOffer != nil {
			t.Fatalf("assento %d não pode ver a oferta", s)
		}
	}
}

func TestViewPartnerHandAndVoteDuringRaise(t *testing.T) {
	g := playingState(fixtureBWinsTwice(), HandNormal)
	g, _ = mustApply(t, g, Action{Kind: ActRequestRaise, Seat: 1})
	if len(ViewFor(g, 0).PartnerHand) != 3 || ViewFor(g, 1).PartnerHand != nil {
		t.Fatal("só a dupla desafiada vê as cartas do parceiro")
	}
	g, _ = mustApply(t, g, Action{Kind: ActVote, Seat: 0, Choice: VoteAccept})
	if pv := ViewFor(g, 2).PartnerVote; pv == nil || *pv != VoteAccept {
		t.Fatal("o parceiro vê o voto")
	}
	opp := ViewFor(g, 1)
	if opp.PartnerVote != nil || !opp.Players[0].Voted {
		t.Fatal("adversário vê só que votou")
	}
}

func TestViewJSONEncodesCutOfferAsArray(t *testing.T) {
	g, _ := mustApply(t, newTestGame(7), Action{Kind: ActSysStartHand})
	raw, err := json.Marshal(ViewFor(g, 3))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"cutOffer":[{"keep":0,"card":`) {
		t.Fatalf("JSON inesperado: %s", raw)
	}
}

func TestExportedSeatAPIsDoNotPanicOnInvalidSeat(t *testing.T) {
	g := playingState(fixtureBWinsTwice(), HandNormal)
	g, _ = mustApply(t, g, Action{Kind: ActRequestRaise, Seat: 1})
	v := ViewFor(g, 9)
	if len(v.MyHand) != 0 || v.PartnerHand != nil || v.CutOffer != nil {
		t.Fatalf("assento inválido não recebe dados privados: %+v", v)
	}
	if _, ok := DefaultAction(g, 9); ok {
		t.Fatal("assento inválido não tem ação padrão")
	}
}
