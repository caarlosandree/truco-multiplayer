package bot

import (
	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
)

// Este arquivo é o simulador rápido de uma mão, usado só DENTRO da simulação do
// Strong: ele opera sobre estados HIPOTÉTICOS (sorteados por Sample), nunca
// sobre a partida real, e reproduz as regras do engine sem alocar. A fidelidade
// ao engine é garantida por teste (TestFastRolloutMatchesEngine).

// rankIdx é o índice do rank na escada base (4 < 5 < 6 < 7 < Q < J < K < A < 2 < 3).
var rankIdx = func() (m [14]uint8) {
	for i, r := range [...]engine.Rank{4, 5, 6, 7, 10, 11, 12, 1, 2, 3} {
		m[r] = uint8(i)
	}
	return m
}()

type simVote uint8

const (
	vNone simVote = iota
	vAccept
	vRun
	vRaise
	vPlay
)

const (
	simPlay uint8 = iota
	simRaiseVote
	simElevenVote
	simOver
)

const (
	kindNormal uint8 = iota
	kindEleven
	kindIron
)

type simCard struct {
	seat    int8
	card    uint8 // engine.CardID
	covered bool
}

// sim é o estado de uma mão. Cabe em ~150 bytes e é copiado por valor.
type sim struct {
	hand       [4][3]uint8
	size       [4]uint8
	played     [4]uint8 // máscara de slots já jogados
	manilha    uint8    // índice do rank da manilha
	kind       uint8
	stake      int8
	lastRaiser int8 // -1 = ninguém
	pReq       int8
	pFrom      int8
	pPrev      int8
	pTo        int8
	voting     int8 // dupla votando, -1 = nenhuma
	votes      [4]simVote
	phase      uint8
	dealer     int8
	turn       int8
	starter    int8
	table      [4]simCard
	nt         uint8
	results    [3]int8 // -1 empate, 0/1 dupla vencedora
	nr         uint8
	score      [2]int8
}

func teamOfSeat(s int8) int8 { return s & 1 }

func voteFrom(c engine.VoteChoice) simVote {
	switch c {
	case engine.VoteAccept:
		return vAccept
	case engine.VoteRun:
		return vRun
	case engine.VoteRaise:
		return vRaise
	case engine.VotePlay:
		return vPlay
	}
	return vNone
}

// simFrom converte um GameState hipotético em sim. ok=false se não há mão em
// andamento numa fase que o simulador entende.
func simFrom(g *engine.GameState) (s sim, ok bool) {
	h := g.Hand
	if h == nil {
		return s, false
	}
	switch g.Phase {
	case engine.PhasePlaying:
		s.phase = simPlay
	case engine.PhaseAwaitingRaiseResponse:
		s.phase = simRaiseVote
	case engine.PhaseElevenDecision:
		s.phase = simElevenVote
	default:
		return s, false
	}
	switch h.Kind {
	case engine.HandEleven:
		s.kind = kindEleven
	case engine.HandIron:
		s.kind = kindIron
	}
	s.manilha = rankIdx[h.Manilha]
	s.stake = int8(h.Stake)
	s.lastRaiser, s.voting = -1, -1
	if h.LastRaiser != nil {
		s.lastRaiser = int8(*h.LastRaiser)
	}
	if h.Pending != nil {
		p := h.Pending
		s.pReq, s.pFrom, s.pPrev, s.pTo = int8(p.Requester), int8(p.FromTeam), int8(p.PrevStake), int8(p.ToStake)
	}
	if h.VotingTeam != nil {
		s.voting = int8(*h.VotingTeam)
	}
	for seat := range 4 {
		s.size[seat] = uint8(min(len(h.Hands[seat]), 3))
		for slot := range int(s.size[seat]) {
			s.hand[seat][slot] = uint8(h.Hands[seat][slot].ID())
			if h.Played[seat][slot] {
				s.played[seat] |= 1 << slot
			}
		}
		if v := h.Votes[seat]; v != nil {
			s.votes[seat] = voteFrom(*v)
		}
	}
	s.dealer = int8(g.Dealer)
	s.turn = int8(h.Turn)
	s.starter = int8(h.RoundStarter)
	for _, t := range h.Table {
		if s.nt < 4 {
			s.table[s.nt] = simCard{seat: int8(t.Seat), card: uint8(t.Card.ID()), covered: t.Covered}
			s.nt++
		}
	}
	for _, r := range h.Rounds {
		if s.nr < 3 {
			s.results[s.nr] = -1
			if r.Winner != nil {
				s.results[s.nr] = int8(*r.Winner)
			}
			s.nr++
		}
	}
	s.score = [2]int8{int8(g.Score[0]), int8(g.Score[1])}
	return s, true
}

// str é a força da carta (coberta = 0; comum 1..10; manilha 11..14).
func (s *sim) str(c uint8, covered bool) int8 {
	if covered {
		return 0
	}
	if c>>2 == s.manilha {
		return 11 + int8(c&3)
	}
	return 1 + int8(c>>2)
}

func (s *sim) has(seat int8, slot int) bool {
	return slot < int(s.size[seat]) && s.played[seat]&(1<<slot) == 0
}

func (s *sim) hasManilha(seat int8) bool {
	for slot := range 3 {
		if s.has(seat, slot) && s.hand[seat][slot]>>2 == s.manilha {
			return true
		}
	}
	return false
}

func (s *sim) highCards(seat int8) int {
	n := 0
	for slot := range 3 {
		if s.has(seat, slot) && s.hand[seat][slot]>>2 >= 7 {
			n++
		}
	}
	return n
}

// nextStake espelha engine.nextStake.
func nextStakeOf(stake int8) (int8, bool) {
	switch stake {
	case 1:
		return 3, true
	case 3:
		return 6, true
	case 6:
		return 9, true
	case 9:
		return 12, true
	}
	return 0, false
}

// canRaise diz se o assento (na sua vez) pode pedir aumento numa mão normal.
func (s *sim) canRaise(seat int8) bool {
	if s.kind != kindNormal || s.phase != simPlay || seat != s.turn || s.lastRaiser == teamOfSeat(seat) {
		return false
	}
	_, ok := nextStakeOf(s.stake)
	return ok
}

func (s *sim) endHand(winner int8, points int8) {
	if winner >= 0 {
		s.score[winner] = min(s.score[winner]+points, 12)
	}
	s.phase = simOver
}

func (s *sim) play(seat int8, slot int, covered bool) {
	s.played[seat] |= 1 << slot
	s.table[s.nt] = simCard{seat: seat, card: s.hand[seat][slot], covered: covered}
	s.nt++
	if s.nt < 4 {
		s.turn = (seat + 1) & 3
		return
	}
	s.finishRound()
}

func (s *sim) finishRound() {
	best := [2]int8{-1, -1}
	var bestSeat [2]int8
	for i := range int(s.nt) {
		t := s.table[i]
		team := teamOfSeat(t.seat)
		if st := s.str(t.card, t.covered); st > best[team] {
			best[team] = st
			bestSeat[team] = t.seat
		}
	}
	winner, winnerSeat := int8(-1), int8(-1)
	if best[0] != best[1] {
		winner = 0
		if best[1] > best[0] {
			winner = 1
		}
		winnerSeat = bestSeat[winner]
	}
	s.results[s.nr] = winner
	s.nr++
	s.nt = 0
	if w, decided := s.handWinner(); decided {
		s.endHand(w, s.stake)
		return
	}
	if winnerSeat >= 0 {
		s.starter = winnerSeat
	}
	s.turn = s.starter
}

// handWinner espelha engine.HandWinner.
func (s *sim) handWinner() (winner int8, decided bool) {
	var wins [2]int
	for i := range int(s.nr) {
		if r := s.results[i]; r >= 0 {
			wins[r]++
		}
	}
	for team := range 2 {
		if wins[team] >= 2 {
			return int8(team), true
		}
	}
	switch s.nr {
	case 2:
		first, second := s.results[0], s.results[1]
		if first >= 0 && second < 0 {
			return first, true
		}
		if first < 0 && second >= 0 {
			return second, true
		}
	case 3:
		if s.results[2] >= 0 {
			return s.results[2], true
		}
		return s.results[0], true
	}
	return -1, false
}

func (s *sim) request(seat int8) {
	team := teamOfSeat(seat)
	if s.kind != kindNormal {
		s.score[1-team] = 12
		s.phase = simOver
		return
	}
	to, _ := nextStakeOf(s.stake)
	s.pReq, s.pFrom, s.pPrev, s.pTo = seat, team, s.stake, to
	s.openVote(1-team, simRaiseVote)
}

func (s *sim) openVote(team int8, phase uint8) {
	s.voting = team
	s.votes = [4]simVote{}
	s.phase = phase
}

func boldness(v simVote) int {
	if v == vRun {
		return 0
	}
	if v == vAccept {
		return 1
	}
	return 2
}

func (s *sim) vote(seat int8, c simVote) {
	s.votes[seat] = c
	pv := s.votes[(seat+2)&3]
	if pv == vNone {
		return
	}
	final := c
	if boldness(pv) > boldness(c) {
		final = pv
	}
	if s.phase == simElevenVote {
		s.resolveEleven(final)
		return
	}
	s.resolveRaise(final)
}

func (s *sim) resolveEleven(c simVote) {
	team := s.voting
	s.voting = -1
	s.votes = [4]simVote{}
	if c == vRun {
		s.endHand(1-team, 1)
		return
	}
	s.phase = simPlay
	s.turn = (s.dealer + 1) & 3
	s.starter = s.turn
}

func (s *sim) resolveRaise(c simVote) {
	voting := s.voting
	switch c {
	case vAccept:
		s.stake = s.pTo
		s.lastRaiser = s.pFrom
		s.voting = -1
		s.votes = [4]simVote{}
		s.turn = s.pReq
		s.phase = simPlay
	case vRun:
		s.voting = -1
		s.endHand(s.pFrom, s.pPrev)
	default: // raise
		to, _ := nextStakeOf(s.pTo)
		s.pPrev, s.pTo, s.pFrom = s.pTo, to, voting
		s.lastRaiser = voting
		s.openVote(1-voting, simRaiseVote)
	}
}

// apply aplica uma ação (de in.Legal) do assento sobre o estado hipotético.
func (s *sim) apply(a engine.Action) {
	seat := int8(a.Seat)
	switch a.Kind {
	case engine.ActPlayCard:
		s.play(seat, a.Slot, a.Covered)
	case engine.ActRequestRaise:
		s.request(seat)
	case engine.ActVote:
		s.vote(seat, voteFrom(a.Choice))
	}
}

// simpleVote e simplePlay reproduzem a política Simple (simple.go) sobre o
// estado hipotético, sem montar Input.
func (s *sim) simpleVote(seat int8) simVote {
	if s.phase == simElevenVote {
		if s.hasManilha(seat) {
			return vPlay
		}
		return vRun
	}
	if s.hasManilha(seat) || s.highCards(seat) >= 2 {
		return vAccept
	}
	return vRun
}

func (s *sim) simpleStep() {
	switch s.phase {
	case simRaiseVote, simElevenVote:
		for seat := int8(0); seat < 4; seat++ {
			if teamOfSeat(seat) == s.voting && s.votes[seat] == vNone {
				s.vote(seat, s.simpleVote(seat))
				return
			}
		}
		// Sem votante pendente (estado incoerente): encerra para não travar.
		s.phase = simOver
	case simPlay:
		s.simplePlay()
	}
}

func (s *sim) simplePlay() {
	seat := s.turn
	if s.kind == kindNormal && s.canRaise(seat) && s.hasManilha(seat) {
		s.request(seat)
		return
	}
	// Maior força jogada por adversários nesta rodada (-1 se nenhum jogou).
	target := int8(-1)
	for i := range int(s.nt) {
		t := s.table[i]
		if teamOfSeat(t.seat) == teamOfSeat(seat) {
			continue
		}
		target = max(target, s.str(t.card, t.covered))
	}
	bestWin, bestAny := -1, -1
	var bestWinStr, bestAnyStr int8
	for slot := range 3 {
		if !s.has(seat, slot) {
			continue
		}
		var st int8
		if s.kind != kindIron {
			st = s.str(s.hand[seat][slot], false)
		}
		if bestAny < 0 || st < bestAnyStr {
			bestAny, bestAnyStr = slot, st
		}
		if st > target && (bestWin < 0 || st < bestWinStr) {
			bestWin, bestWinStr = slot, st
		}
	}
	slot := bestWin
	if slot < 0 {
		slot = bestAny
	}
	if slot < 0 { // sem cartas (incoerente): encerra
		s.phase = simOver
		return
	}
	s.play(seat, slot, false)
}

// rolloutSimple joga a mão até o fim com a política Simple para todos.
func (s *sim) rolloutSimple() {
	for i := 0; s.phase != simOver && i < 64; i++ {
		s.simpleStep()
	}
}
