package bot

import (
	"math/rand/v2"
	"slices"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
)

// cardPool é o conjunto das cartas ainda não vistas, sem alocação.
type cardPool struct {
	ids [engine.DeckSize]engine.CardID
	n   int
}

func newPool(known *[engine.DeckSize]bool) cardPool {
	var p cardPool
	for id := range engine.DeckSize {
		if !known[id] {
			p.ids[p.n] = engine.CardID(id)
			p.n++
		}
	}
	return p
}

func (p *cardPool) take(i int) engine.CardID {
	id := p.ids[i]
	p.n--
	p.ids[i] = p.ids[p.n]
	return id
}

// uniform sorteia uma carta com igual probabilidade. Com o conjunto vazio
// (entrada incoerente) devolve a carta 0, sem pânico.
func (p *cardPool) uniform(rng *rand.Rand) engine.CardID {
	if p.n == 0 {
		return 0
	}
	return p.take(rng.IntN(p.n))
}

// weighted sorteia proporcionalmente a w; pesos todos zero → uniforme.
func (p *cardPool) weighted(w *[engine.DeckSize]float64, rng *rand.Rand) engine.CardID {
	var total float64
	for _, id := range p.ids[:p.n] {
		total += w[id]
	}
	if total <= 0 {
		return p.uniform(rng)
	}
	r := rng.Float64() * total
	for i, id := range p.ids[:p.n] {
		r -= w[id]
		if r < 0 {
			return p.take(i)
		}
	}
	return p.take(p.n - 1)
}

// four sorteia uniformemente um quatro do conjunto (cartas queimadas são sempre
// quatros). Sem quatro disponível, cai para qualquer carta.
func (p *cardPool) four(rng *rand.Rand) engine.CardID {
	var idx [4]int
	n := 0
	for i, id := range p.ids[:p.n] {
		if engine.CardFromID(id).Rank == 4 && n < len(idx) {
			idx[n] = i
			n++
		}
	}
	if n == 0 {
		return p.uniform(rng)
	}
	return p.take(idx[rng.IntN(n)])
}

// knownCards marca as cartas que o assento enxerga na visão: própria mão (menos
// na mão de ferro), mão do parceiro quando a dupla vota, oferta do corte, vira
// e as cartas abertas da mesa e das rodadas. Cartas cobertas alheias não entram.
func knownCards(v engine.PlayerView) [engine.DeckSize]bool {
	var k [engine.DeckSize]bool
	mark := func(id *engine.CardID) {
		if id != nil && int(*id) < engine.DeckSize {
			k[*id] = true
		}
	}
	for _, hs := range v.MyHand {
		mark(hs.Card)
	}
	for _, hs := range v.PartnerHand {
		mark(hs.Card)
	}
	for _, oc := range v.CutOffer {
		mark(&oc.Card)
	}
	mark(v.Vira)
	for _, t := range v.Table {
		mark(t.Card)
	}
	for _, r := range v.Rounds {
		for _, t := range r.Cards {
			mark(t.Card)
		}
	}
	return k
}

// slotRef localiza uma carta jogada coberta e desconhecida: na rodada ri (ou na
// mesa, ri = -1) posição ci, vinda do slot da mão do assento.
type slotRef struct {
	seat   engine.Seat
	slot   int
	ri, ci int
}

// Sample monta um GameState completo hipotético, coerente com in.View:
// engine.ViewFor(Sample(in, b, rng), in.View.Seat) é igual a in.View. A mão do
// próprio assento é exata (exceto na mão de ferro, em que ela é sorteada), as
// mãos alheias e as cartas cobertas alheias saem da crença b, as queimadas são
// quatros não vistos e o resto vai para o Deck. Usa só o Input: nunca o estado
// real. O resultado é determinístico para o mesmo (in, b, rng).
//
// Com uma Belief zerada ou impossível o sorteio é uniforme sobre as cartas não
// vistas. A amostragem é sequencial sem reposição (assentos em ordem aleatória),
// que é barata e respeita os pesos de b.
func Sample(in Input, b Belief, rng *rand.Rand) engine.GameState {
	v := in.View
	g := engine.GameState{
		Seed: rng.Uint64(), HandNumber: v.HandNumber, Score: v.Score, Dealer: v.Dealer, Phase: v.Phase,
	}
	if v.Winner != nil {
		w := *v.Winner
		g.Winner = &w
	}
	for s := range 4 {
		g.Players[s] = engine.Player{ID: v.Players[s].ID, Name: v.Players[s].Name}
	}
	if v.HandKind == "" || v.Seat > 3 {
		return g // sem mão em andamento
	}

	me, partner := v.Seat, engine.Partner(v.Seat)
	h := &engine.HandState{Kind: v.HandKind, Stake: v.Stake}
	g.Hand = h
	if v.LastRaiser != nil {
		t := *v.LastRaiser
		h.LastRaiser = &t
	}
	if v.Pending != nil {
		p := *v.Pending
		h.Pending = &p
	}
	if v.VotingTeam != nil {
		t := *v.VotingTeam
		h.VotingTeam = &t
	}
	if v.Vira != nil {
		vira := engine.CardFromID(*v.Vira)
		h.Vira = &vira
		if v.Manilha != nil {
			h.Manilha = *v.Manilha
		}
	}

	// Mesa e rodadas, com as cartas abertas; as cobertas alheias ficam por sortear.
	var playedBy [4]int
	var unknown []slotRef
	table := make([]engine.PlayedCard, len(v.Table))
	for i, t := range v.Table {
		table[i] = engine.PlayedCard{Seat: t.Seat, Covered: t.Covered}
		if t.Card != nil {
			table[i].Card = engine.CardFromID(*t.Card)
		}
		if t.Seat < 4 {
			playedBy[t.Seat]++
		}
	}
	rounds := make([]engine.RoundResult, len(v.Rounds))
	for i, r := range v.Rounds {
		rr := engine.RoundResult{Winner: r.Winner, WinnerSeat: r.WinnerSeat, Cards: make([]engine.PlayedCard, len(r.Cards))}
		for j, t := range r.Cards {
			rr.Cards[j] = engine.PlayedCard{Seat: t.Seat, Covered: t.Covered}
			if t.Card != nil {
				rr.Cards[j].Card = engine.CardFromID(*t.Card)
			}
			if t.Seat < 4 {
				playedBy[t.Seat]++
			}
		}
		if len(rr.Cards) > 0 {
			rr.Starter = rr.Cards[0].Seat
		}
		rounds[i] = rr
	}

	// Slots: os não jogados vêm da visão (própria mão e mão do parceiro) ou ficam
	// depois dos jogados; os jogados ocupam os demais, em ordem cronológica.
	var (
		size       [4]int
		unplayed   [4][3]int
		nUnplayed  [4]int
		hidden     [4][3]bool
		backing    = make([]engine.Card, 12)
		playedSlot [4][3]int
	)
	for s := range 4 {
		seat := engine.Seat(s)
		size[s] = min(v.Players[s].CardCount+playedBy[s], 3)
		var src []engine.HandSlotView
		switch {
		case seat == me:
			src = v.MyHand
		case seat == partner && v.PartnerHand != nil:
			src = v.PartnerHand
		}
		if src != nil || seat == me {
			for _, hs := range src {
				if nUnplayed[s] < 3 && hs.Slot >= 0 && hs.Slot < size[s] {
					unplayed[s][nUnplayed[s]] = hs.Slot
					hidden[s][nUnplayed[s]] = hs.Card == nil
					if hs.Card != nil {
						backing[s*3+hs.Slot] = engine.CardFromID(*hs.Card)
					}
					nUnplayed[s]++
				}
			}
		} else {
			cnt := min(v.Players[s].CardCount, size[s])
			for slot := size[s] - cnt; slot < size[s]; slot++ {
				unplayed[s][nUnplayed[s]] = slot
				hidden[s][nUnplayed[s]] = true
				nUnplayed[s]++
			}
		}
		h.Hands[s] = backing[s*3 : s*3+size[s] : s*3+3]
		np := 0
		for slot := range size[s] {
			if !slices.Contains(unplayed[s][:nUnplayed[s]], slot) {
				playedSlot[s][np] = slot
				h.Played[s][slot] = true
				np++
			}
		}
	}

	// Cartas jogadas: ocupam os slots jogados, em ordem cronológica por assento.
	var cursor [4]int
	place := func(t *engine.PlayedCard, ri, ci int) {
		if t.Seat > 3 || cursor[t.Seat] >= 3 {
			return
		}
		s := t.Seat
		slot := playedSlot[s][cursor[s]]
		cursor[s]++
		if t.Covered && s != me && !knownInView(v, ri, ci) {
			unknown = append(unknown, slotRef{seat: s, slot: slot, ri: ri, ci: ci})
			return
		}
		if slot < len(h.Hands[s]) {
			h.Hands[s][slot] = t.Card
		}
	}
	for ri := range rounds {
		for ci := range rounds[ri].Cards {
			place(&rounds[ri].Cards[ci], ri, ci)
		}
	}
	for ci := range table {
		place(&table[ci], -1, ci)
	}

	// Sorteio.
	known := knownCards(v)
	pool := newPool(&known)

	if v.DiscardCount > 0 {
		h.Discarded = make([]engine.Card, v.DiscardCount)
		for i := range h.Discarded {
			h.Discarded[i] = engine.CardFromID(pool.four(rng))
		}
	}
	for _, u := range unknown {
		c := engine.CardFromID(pool.uniform(rng))
		if u.slot < len(h.Hands[u.seat]) {
			h.Hands[u.seat][u.slot] = c
		}
		if u.ri >= 0 {
			rounds[u.ri].Cards[u.ci].Card = c
		} else {
			table[u.ci].Card = c
		}
	}
	order := [4]engine.Seat{0, 1, 2, 3}
	rng.Shuffle(4, func(i, j int) { order[i], order[j] = order[j], order[i] })
	for _, seat := range order {
		for i := range nUnplayed[seat] {
			if hidden[seat][i] {
				h.Hands[seat][unplayed[seat][i]] = engine.CardFromID(pool.weighted(&b.w[seat], rng))
			}
		}
	}
	switch {
	case len(v.CutOffer) > 0:
		h.CutOffer = make([]engine.Card, len(v.CutOffer))
		for _, oc := range v.CutOffer {
			if oc.Keep >= 0 && oc.Keep < len(h.CutOffer) {
				h.CutOffer[oc.Keep] = engine.CardFromID(oc.Card)
			}
		}
	case v.Phase == engine.PhaseCutChoice || v.Phase == engine.PhaseDealerChoice:
		h.CutOffer = []engine.Card{engine.CardFromID(pool.uniform(rng)), engine.CardFromID(pool.uniform(rng))}
	}
	h.Deck = make([]engine.Card, pool.n)
	for i, id := range pool.ids[:pool.n] {
		h.Deck[i] = engine.CardFromID(id)
	}
	rng.Shuffle(len(h.Deck), func(i, j int) { h.Deck[i], h.Deck[j] = h.Deck[j], h.Deck[i] })

	h.Table, h.Rounds = table, rounds
	if len(table) == 0 {
		h.Table = nil
	}
	if len(rounds) == 0 {
		h.Rounds = nil
	}

	// Vez, abridor da rodada, votos e queima.
	switch {
	case v.Turn != nil:
		h.Turn = *v.Turn
	case v.Pending != nil:
		h.Turn = v.Pending.Requester
	default:
		h.Turn = engine.Next(v.Dealer)
	}
	h.RoundStarter = h.Turn
	if len(table) > 0 {
		h.RoundStarter = table[0].Seat
	}
	for s := range 4 {
		if !v.Players[s].Voted {
			continue
		}
		choice := engine.VoteAccept
		if v.Phase == engine.PhaseElevenDecision {
			choice = engine.VotePlay
		}
		switch {
		case engine.Seat(s) == partner && v.PartnerVote != nil:
			choice = *v.PartnerVote
		case engine.Seat(s) == me:
			if c, ok := lastVoteBy(in.Events, me); ok {
				choice = c
			}
		}
		h.Votes[s] = &choice
	}
	if v.Phase == engine.PhaseBurnFours {
		for s := range 4 {
			h.BurnDecided[s] = !slices.ContainsFunc(h.Hands[s], func(c engine.Card) bool { return c.Rank == 4 })
		}
	}
	return g
}

// knownInView diz se a carta da mesa/rodada é visível na visão (aberta ou do
// próprio assento). Cobertas do próprio assento aparecem com Card != nil.
func knownInView(v engine.PlayerView, ri, ci int) bool {
	if ri >= 0 {
		return v.Rounds[ri].Cards[ci].Card != nil
	}
	return v.Table[ci].Card != nil
}

func lastVoteBy(evs []engine.Event, seat engine.Seat) (engine.VoteChoice, bool) {
	for i := len(evs) - 1; i >= 0; i-- {
		e := evs[i]
		if e.Kind == engine.EvHandStarted {
			break
		}
		if e.Kind == engine.EvVoteCast && e.Seat != nil && *e.Seat == seat {
			return e.Choice, true
		}
	}
	return "", false
}
