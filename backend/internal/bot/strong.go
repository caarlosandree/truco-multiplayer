package bot

import (
	"context"
	"math/rand/v2"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
)

// Strong é a política forte (spec §5.3): determinização + simulação. A cada
// decisão sorteia Samples distribuições das cartas não vistas coerentes com o
// Input (Sample, com a crença de belief.go), simula cada ação candidata até o
// fim da mão com a política de rollout e escolhe a de maior valor esperado em
// chance de vencer a PARTIDA (winprob.go), o que pesa o placar: mão de 11,
// perto de 12, aposta que decide o jogo. É busca de Monte Carlo plana: todas as
// ações candidatas veem as mesmas amostras, então a comparação é pareada.
//
// Rollout nil ou Simple{} usa o simulador rápido de sim.go (sem alocação, bit a
// bit igual ao engine; testado). Qualquer outra política de rollout roda no
// engine de verdade sobre o estado hipotético, bem mais devagar.
//
// Decide é single-thread e determinístico dada a seed de rng. Respeita ctx: com
// o prazo vencido devolve a melhor ação avaliada até ali, ou a do Simple se nada
// foi avaliado.
type Strong struct {
	Samples int
	Rollout Policy
}

var _ Policy = Strong{}

// DefaultStrong é a configuração de produção (e do benchmark).
func DefaultStrong() Strong { return Strong{Samples: 200, Rollout: Simple{}} }

// Parâmetros de comportamento, em unidades de chance de vitória (0..1).
const (
	// partnerSignalBias: bônus de disposição por sinal do parceiro (spec §5.6).
	partnerSignalBias = 0.03
	// raiseMargin: o aumento só é escolhido por valor se superar a melhor
	// alternativa por esta margem; a estimativa por amostras é otimista justo
	// onde o máximo é escolhido (medido: sem a margem a forte vence ~88% contra
	// o Simple, com ela ~95%).
	raiseMargin = 0.08
)

// Blefe. Os aumentos com valor esperado positivo (inclusive os de fold equity
// contra o modelo de rollout) já saem do Monte Carlo acima. O blefe extra NÃO é
// lucrativo pelo modelo: escolhe o aumento quando ele é até bluffTolerance pior
// que a melhor ação, com probabilidade bluffProb*(1-perda/bluffTolerance). Serve
// à imprevisibilidade (um bot que nunca blefa é lido por humanos) e custa pouco
// por construção, pois a perda esperada é limitada pela tolerância. Os valores
// foram escolhidos por varredura contra quatro adversários sintéticos (tabela
// em task-6-report.md): tolerância 0,03 com prob. 0,3 empata, dentro do ruído
// (~1 ponto), com a melhor taxa média de vitória (86,5% contra 86,6% sem blefe)
// e é o blefe mais frequente entre os empatados (~2% das mãos fracas); tolerâncias
// ou probabilidades maiores custam de 1 a 8 pontos. São variáveis só para a
// varredura de teste (TestBluffSweep); não mude em produção sem refazê-la.
var (
	bluffTolerance = 0.03
	bluffProb      = 0.3
)

// Decide devolve sempre uma ação de in.Legal (ou Decision vazia se Legal é vazio).
func (st Strong) Decide(ctx context.Context, in Input, rng *rand.Rand) Decision {
	if len(in.Legal) == 0 {
		return Decision{}
	}
	a := st.choose(ctx, in, rng)
	return Decision{Action: a, Signal: intentSignal(in, a, rng)}
}

func (st Strong) choose(ctx context.Context, in Input, rng *rand.Rand) engine.Action {
	v := in.View
	switch v.Phase {
	case engine.PhaseCutChoice, engine.PhaseDealerChoice:
		return chooseKeep(in)
	case engine.PhaseBurnFours:
		return burnAll(in)
	}
	if v.Phase == engine.PhasePlaying && v.HandKind == engine.HandIron {
		// Mão de ferro: as próprias cartas são ocultas e não há aumento nem
		// cobertura; a primeira carta legal é tão boa quanto qualquer outra.
		if a, ok := findLegal(in.Legal, func(a engine.Action) bool { return a.Kind == engine.ActPlayCard && !a.Covered }); ok {
			return a
		}
	}
	cands := candidates(in)
	if len(cands) <= 1 {
		if len(cands) == 1 {
			return cands[0]
		}
		return in.Legal[0]
	}
	if ctx.Err() != nil {
		return st.fallback(in)
	}
	vals, n := st.evaluate(ctx, in, cands, rng)
	if n == 0 {
		return st.fallback(in)
	}
	return pick(in, cands, vals, rng)
}

// fallback é a ação do Simple, sempre legal.
func (Strong) fallback(in Input) engine.Action {
	a := simpleAction(in)
	if actionIn(in.Legal, a) {
		return a
	}
	return in.Legal[0]
}

func actionIn(legal []engine.Action, a engine.Action) bool {
	for _, l := range legal {
		if sameAction(l, a) {
			return true
		}
	}
	return false
}

// chooseKeep escolhe a carta de maior rank na oferta do corte (a outra vai para
// o parceiro: para a dupla dá no mesmo, e a manilha ainda é desconhecida).
func chooseKeep(in Input) engine.Action {
	best, bestIdx := -1, -1
	for _, oc := range in.View.CutOffer {
		c := engine.CardFromID(oc.Card)
		idx := int(rankIdx[c.Rank])*4 + int(c.Suit)
		if idx > bestIdx {
			best, bestIdx = oc.Keep, idx
		}
	}
	if a, ok := findLegal(in.Legal, func(a engine.Action) bool { return a.Kind == engine.ActChooseCard && a.Keep == best }); ok && best >= 0 {
		return a
	}
	return in.Legal[0]
}

// burnAll queima todos os quatros: o quatro é a carta mais fraca (e só é
// manilha em ~10% das mãos), a carta que o substitui vale bem mais em média.
func burnAll(in Input) engine.Action {
	var best engine.Action
	found := false
	for _, a := range in.Legal {
		if a.Kind == engine.ActBurn && (!found || len(a.Slots) > len(best.Slots)) {
			best, found = a, true
		}
	}
	if found {
		return best
	}
	return in.Legal[0]
}

// candidates filtra in.Legal ao que vale avaliar: nunca pedir aumento onde é
// punido (mão de 11 e de ferro) e, nas cartas, todas descobertas mais a coberta
// só da carta mais fraca. Cartas em ordem crescente de força, para que num
// empate de valor se jogue a mais fraca; depois o aumento.
func candidates(in Input) []engine.Action {
	v := in.View
	if v.Phase != engine.PhasePlaying {
		return in.Legal
	}
	var plays, raises []engine.Action
	weakest, weakestStr := -1, 99
	for _, a := range in.Legal {
		if a.Kind == engine.ActPlayCard && !a.Covered {
			if s := slotStrength(v, a.Slot); s < weakestStr {
				weakest, weakestStr = a.Slot, s
			}
		}
	}
	for _, a := range in.Legal {
		switch {
		case a.Kind == engine.ActPlayCard && !a.Covered:
			plays = append(plays, a)
		case a.Kind == engine.ActPlayCard && a.Covered && a.Slot == weakest:
			plays = append(plays, a)
		case a.Kind == engine.ActRequestRaise && v.HandKind == engine.HandNormal:
			raises = append(raises, a)
		}
	}
	// Ordena as cartas por força (inserção; no máximo 4 itens).
	for i := 1; i < len(plays); i++ {
		for j := i; j > 0 && playLess(v, plays[j], plays[j-1]); j-- {
			plays[j], plays[j-1] = plays[j-1], plays[j]
		}
	}
	return append(plays, raises...)
}

func playLess(v engine.PlayerView, a, b engine.Action) bool {
	sa, sb := slotStrength(v, a.Slot), slotStrength(v, b.Slot)
	if sa != sb {
		return sa < sb
	}
	return !a.Covered && b.Covered
}

// evaluate devolve o valor médio de cada candidata e o número de amostras
// avaliadas (para na primeira checagem de ctx vencido, depois de ≥ 1 amostra).
func (st Strong) evaluate(ctx context.Context, in Input, cands []engine.Action, rng *rand.Rand) ([]float64, int) {
	samples := st.Samples
	if samples < 1 {
		samples = 1
	}
	b := NewBelief(in)
	team := int8(engine.TeamOf(in.View.Seat))
	sums := make([]float64, len(cands))
	custom := st.customRollout()
	n := 0
	for i := 0; i < samples; i++ {
		if i > 0 && ctx.Err() != nil {
			break
		}
		g := Sample(in, b, rng)
		if custom {
			for c, a := range cands {
				sums[c] += slowRolloutValue(ctx, st.Rollout, g, a, team, rng)
			}
			n++
			continue
		}
		base, ok := simFrom(&g)
		if !ok {
			continue
		}
		for c, a := range cands {
			s := base
			s.apply(a)
			s.rolloutSimple()
			sums[c] += utility(s.score, team)
		}
		n++
	}
	if n == 0 {
		return nil, 0
	}
	for c := range sums {
		sums[c] /= float64(n)
	}
	return sums, n
}

func (st Strong) customRollout() bool {
	if st.Rollout == nil {
		return false
	}
	_, simple := st.Rollout.(Simple)
	return !simple
}

// slowRolloutValue simula a mão no engine (sobre o estado hipotético g) com a
// política de rollout e devolve o valor do placar final para a dupla team.
func slowRolloutValue(ctx context.Context, p Policy, g engine.GameState, a engine.Action, team int8, rng *rand.Rand) float64 {
	cur, _, err := engine.Apply(g, a)
	if err != nil {
		return utility([2]int8{int8(g.Score[0]), int8(g.Score[1])}, team)
	}
	for i := 0; i < 64 && cur.Phase != engine.PhaseHandEnd && cur.Phase != engine.PhaseGameOver; i++ {
		seat, ok := pendingSeat(cur)
		if !ok {
			break
		}
		in := inputFromState(cur, seat, nil)
		act, valid := engine.DefaultAction(cur, seat)
		if len(in.Legal) > 0 {
			if d := p.Decide(ctx, in, rng); actionIn(in.Legal, d.Action) {
				act, valid = d.Action, true
			}
		}
		if !valid {
			break
		}
		next, _, err := engine.Apply(cur, act)
		if err != nil {
			break
		}
		cur = next
	}
	return utility([2]int8{int8(cur.Score[0]), int8(cur.Score[1])}, team)
}

// pendingSeat acha um assento com decisão pendente numa mão em jogo ou votação.
func pendingSeat(g engine.GameState) (engine.Seat, bool) {
	h := g.Hand
	if h == nil {
		return 0, false
	}
	switch g.Phase {
	case engine.PhasePlaying:
		return h.Turn, true
	case engine.PhaseAwaitingRaiseResponse, engine.PhaseElevenDecision:
		for s := engine.Seat(0); s < 4; s++ {
			if h.VotingTeam != nil && engine.TeamOf(s) == *h.VotingTeam && h.Votes[s] == nil {
				return s, true
			}
		}
	}
	return 0, false
}

// isRaise diz se a ação aumenta a aposta (pedir ou re-aumentar).
func isRaise(a engine.Action) bool {
	return a.Kind == engine.ActRequestRaise || (a.Kind == engine.ActVote && a.Choice == engine.VoteRaise)
}

// aggressive diz se a ação é "ir para cima" na convenção de sinais (§5.6).
func aggressive(a engine.Action) bool {
	return isRaise(a) || (a.Kind == engine.ActVote && (a.Choice == engine.VoteAccept || a.Choice == engine.VotePlay))
}

// pick escolhe a ação. O aumento só sai por valor se superar a melhor
// alternativa por raiseMargin (a estimativa por amostras é otimista justo onde
// o máximo é escolhido); fora isso, às vezes sai como blefe: só se for pior que
// a melhor ação por no máximo bluffTolerance, e com probabilidade que cai
// linearmente com essa perda (sai mais quando o blefe é quase de graça). O
// viés dos sinais do parceiro entra nas ações "para cima".
func pick(in Input, cands []engine.Action, vals []float64, rng *rand.Rand) engine.Action {
	bias := partnerBias(in)
	best, bestRaise := -1, -1
	adj := make([]float64, len(vals))
	for i, a := range cands {
		adj[i] = vals[i]
		if aggressive(a) {
			adj[i] += bias
		}
		switch {
		case isRaise(a):
			if bestRaise < 0 || adj[i] > adj[bestRaise] {
				bestRaise = i
			}
		case best < 0 || adj[i] > adj[best]+1e-9:
			best = i
		}
	}
	switch {
	case best < 0:
		return cands[bestRaise]
	case bestRaise < 0:
		return cands[best]
	}
	if adj[bestRaise] > adj[best]+raiseMargin {
		return cands[bestRaise]
	}
	if loss := adj[best] - adj[bestRaise]; loss >= 0 && loss <= bluffTolerance &&
		rng.Float64() < bluffProb*(1-loss/bluffTolerance) {
		return cands[bestRaise]
	}
	return cands[best]
}

// partnerBias soma o efeito dos sinais do parceiro nesta mão: truco e vamos
// deixam o bot mais disposto, calma menos (spec §5.6). Sinais de adversários
// entram só na crença sobre as mãos (belief.go).
func partnerBias(in Input) float64 {
	v := in.View
	var bias float64
	for _, sg := range in.Signals {
		if sg.HandNumber != v.HandNumber || sg.Seat != engine.Partner(v.Seat) {
			continue
		}
		switch sg.Signal {
		case SigTruco:
			bias += partnerSignalBias
		case SigVamos:
			bias += 0.75 * partnerSignalBias
		case SigCalma:
			bias -= partnerSignalBias
		}
	}
	return bias
}
