# Engine de Regras do Truco — Plano de Implementação

> **Para agentes:** SUB-SKILL OBRIGATÓRIA: use superpowers:subagent-driven-development (recomendado) ou superpowers:executing-plans para implementar este plano tarefa por tarefa. Os passos usam checkbox (`- [ ]`) para acompanhamento.

**Objetivo:** Implementar o engine de regras do Truco em Go puro (reducer determinístico com projeção anti-cheat) e o pacote de protocolo WebSocket com geração de tipos TypeScript.

**Arquitetura:** `Apply(state, action) → (state, events, error)` clona o estado, despacha por tipo de ação e devolve o estado original intacto em caso de erro. Toda saída para clientes passa por `ViewFor(state, seat)`, que filtra cartas proibidas. O RNG de cada mão é derivado de `(Seed, HandNumber)`, então o mesmo log de ações reproduz a partida.

**Tech Stack:** Go (toolchain local 1.27, só stdlib no engine), `math/rand/v2`, `encoding/json`, `tygo` (geração de TS).

**Spec:** `docs/superpowers/specs/2026-10-14-truco-engine-protocolo-design.md`

## Restrições Globais

- Módulo Go: `github.com/caarlosandree/truco/backend`, em `backend/`.
- `internal/engine` importa **apenas** a stdlib; sem goroutines, I/O ou `time.Now`.
- Identificadores em inglês; comentários e mensagens de erro em pt-BR.
- Assentos `0..3` em sentido horário; `Team(seat) = seat % 2`; parceiro `(s+2)%4`; quem corta `(dealer+3)%4`.
- Escada de força `[4,5,6,7,10,11,12,1,2,3]`; naipes `Diamonds(0) < Spades(1) < Hearts(2) < Clubs(3)`; `CardID = rankIndex*4 + suit`.
- Escada de aposta `[1,3,6,9,12]`; vitória com 12 pontos (placar limitado a 12).
- Códigos de erro: `invalid_phase`, `not_your_turn`, `invalid_slot`, `invalid_choice`, `illegal_raise`, `already_voted`, `not_voting_team`, `covered_not_allowed`, `burn_not_four`, `game_over`, `already_decided`, `invalid_seat`.
- Commits convencionais em pt-BR, sem linhas de atribuição.
- Testes no mesmo pacote (`package engine`) para acessar funções internas.

## Foco de Revisão

1. **Queima com slots duplicados** (`Burn{Slots:[0,0]}`): deve falhar com `invalid_slot`, nunca descartar duas vezes. Teste na Tarefa 4.
2. **Reuso de slot já jogado** em rodada posterior: deve falhar com `invalid_slot`. Teste na Tarefa 5.
3. **Assento fora de 0..3** numa Action: deve falhar com `invalid_seat` antes de indexar arrays (evita panic). Teste na Tarefa 4.
4. **Aposta no teto** (pedir com `Stake=12`, ou votar `raise` quando `ToStake=12`): `illegal_raise` / `invalid_choice`. Teste na Tarefa 6.
5. **Ações após o fim** (`GameOver`) e `SysStartHand` com mão em andamento: `game_over` / `invalid_phase`. Testes nas Tarefas 5 e 4.

---

## Estrutura de Arquivos

```
Makefile                              # test, test-short, gen-types
.gitignore
backend/go.mod
backend/tygo.yaml
backend/internal/engine/
  card.go        Suit, Rank, Card, CardID, rankLadder
  deck.go        NewDeck, Shuffle, handRNG
  state.go       Seat/Team helpers, Phase, HandKind, VoteChoice, GameState, HandState, Clone
  strength.go    ManilhaRank, Strength, RoundWinner
  scoring.go     HandWinner, playCard, finishRound, endHand, finishGame
  actions.go     ActionKind, Action
  events.go      Visibility, EventKind, Event, FilterEvents
  errors.go      ErrorCode, RuleError
  apply.go       NewGame, Apply, dispatch
  deal.go        startHand, chooseCard, dealRemaining, burn, revealVira
  betting.go     requestRaise, vote, resolveRaise, resolveEleven
  timeout.go     DefaultAction
  view.go        PlayerView e ViewFor
  *_test.go
backend/internal/protocol/
  protocol.go    ClientMessage, payloads, DecodeAction, ServerMessage
  protocol_test.go
frontend/src/protocol/engine.ts, messages.ts   (gerados)
```

---

### Task 1: Módulo, cartas e baralho

**Arquivos:**
- Criar: `Makefile`, `.gitignore`, `backend/go.mod`
- Criar: `backend/internal/engine/card.go`, `backend/internal/engine/deck.go`
- Teste: `backend/internal/engine/card_test.go`

**Interfaces:**
- Produz: `type Suit uint8` (`Diamonds, Spades, Hearts, Clubs`), `type Rank uint8`, `var rankLadder [10]Rank`, `func rankIndex(Rank) int`, `type Card struct{Rank; Suit}`, `type CardID uint8`, `func (Card) ID() CardID`, `func CardFromID(CardID) Card`, `const DeckSize = 40`, `func NewDeck() []Card`, `func Shuffle([]Card, *rand.Rand)`, `func handRNG(seed uint64, handNumber int) *rand.Rand`.

- [ ] **Passo 1: Criar o módulo e arquivos de suporte**

```bash
mkdir -p backend/internal/engine
cd backend && go mod init github.com/caarlosandree/truco/backend
```

`Makefile` (raiz):

```make
.PHONY: test test-short gen-types

test:
	cd backend && go test ./...

test-short:
	cd backend && go test -short ./...

gen-types:
	cd backend && go run github.com/gzuidhof/tygo@latest generate
```

`.gitignore` (raiz):

```gitignore
node_modules/
dist/
*.out
.env
```

- [ ] **Passo 2: Escrever o teste que falha**

`backend/internal/engine/card_test.go`:

```go
package engine

import (
	"slices"
	"testing"
)

func TestCardIDRoundTrip(t *testing.T) {
	for id := range DeckSize {
		c := CardFromID(CardID(id))
		if got := c.ID(); got != CardID(id) {
			t.Fatalf("CardFromID(%d).ID() = %d", id, got)
		}
	}
}

func TestNewDeckHas40UniqueCardsWithout8And9(t *testing.T) {
	deck := NewDeck()
	if len(deck) != DeckSize {
		t.Fatalf("len = %d, quer %d", len(deck), DeckSize)
	}
	seen := map[Card]bool{}
	for _, c := range deck {
		if c.Rank == 0 || c.Rank == 8 || c.Rank == 9 || c.Rank > 12 {
			t.Fatalf("rank inválido no baralho: %+v", c)
		}
		if seen[c] {
			t.Fatalf("carta duplicada: %+v", c)
		}
		seen[c] = true
	}
}

func TestShuffleIsDeterministicPerSeedAndHand(t *testing.T) {
	a, b, c := NewDeck(), NewDeck(), NewDeck()
	Shuffle(a, handRNG(42, 1))
	Shuffle(b, handRNG(42, 1))
	Shuffle(c, handRNG(42, 2))
	if !slices.Equal(a, b) {
		t.Fatal("mesma seed e mão deveriam gerar o mesmo embaralhamento")
	}
	if slices.Equal(a, c) {
		t.Fatal("mãos diferentes deveriam gerar embaralhamentos diferentes")
	}
}
```

- [ ] **Passo 3: Rodar e verificar que falha**

Run: `cd backend && go test ./internal/engine/`
Expected: FAIL de compilação (`undefined: DeckSize`, `CardFromID`...).

- [ ] **Passo 4: Implementar**

`backend/internal/engine/card.go`:

```go
package engine

import "fmt"

// Suit é o naipe; o valor numérico é a ordem de desempate entre manilhas.
type Suit uint8

const (
	Diamonds Suit = iota // ♦ Ouros (menor)
	Spades               // ♠ Espadas
	Hearts               // ♥ Copas
	Clubs                // ♣ Paus (maior)
)

// Rank usa valores neutros: 1..7, 10, 11, 12 (o frontend mapeia para A/J/Q/K).
type Rank uint8

// rankLadder é a escada base, da carta mais fraca (índice 0) para a mais forte.
var rankLadder = [10]Rank{4, 5, 6, 7, 10, 11, 12, 1, 2, 3}

func rankIndex(r Rank) int {
	for i, candidate := range rankLadder {
		if candidate == r {
			return i
		}
	}
	panic(fmt.Sprintf("engine: rank inválido %d", r))
}

type Card struct {
	Rank Rank `json:"rank"`
	Suit Suit `json:"suit"`
}

// CardID identifica uma carta em 0..39 (rankIndex*4 + suit).
type CardID uint8

func (c Card) ID() CardID { return CardID(rankIndex(c.Rank)*4 + int(c.Suit)) }

func CardFromID(id CardID) Card {
	return Card{Rank: rankLadder[id/4], Suit: Suit(id % 4)}
}
```

`backend/internal/engine/deck.go`:

```go
package engine

import "math/rand/v2"

const DeckSize = 40

// NewDeck devolve as 40 cartas em ordem de CardID.
func NewDeck() []Card {
	deck := make([]Card, 0, DeckSize)
	for id := range DeckSize {
		deck = append(deck, CardFromID(CardID(id)))
	}
	return deck
}

func Shuffle(deck []Card, rng *rand.Rand) {
	rng.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
}

// handRNG deriva um RNG determinístico por mão a partir da seed da partida.
func handRNG(seed uint64, handNumber int) *rand.Rand {
	return rand.New(rand.NewPCG(seed, uint64(handNumber)))
}
```

- [ ] **Passo 5: Rodar e verificar que passa**

Run: `make test`
Expected: PASS

- [ ] **Passo 6: Commit**

```bash
git add Makefile .gitignore backend
git commit -m "feat(engine): adiciona modelo de cartas e baralho determinístico"
```

---

### Task 2: Força das cartas e vencedor da rodada

**Arquivos:**
- Criar: `backend/internal/engine/state.go` (apenas helpers de assento nesta tarefa)
- Criar: `backend/internal/engine/strength.go`
- Teste: `backend/internal/engine/strength_test.go`

**Interfaces:**
- Consome: `Card`, `Rank`, `Suit`, `rankLadder`, `rankIndex` (Tarefa 1).
- Produz: `type Seat uint8`, `type Team uint8`, `TeamOf(Seat) Team`, `Partner(Seat) Seat`, `Next(Seat) Seat`, `Other(Team) Team`, `teamPtr(Team) *Team`, `seatPtr(Seat) *Seat`, `clonePtr[T](*T) *T`, `type PlayedCard struct{Seat; Card; Covered bool}`, `ManilhaRank(vira Card) Rank`, `Strength(c Card, covered bool, manilha Rank) int`, `RoundWinner(table []PlayedCard, manilha Rank) (*Team, *Seat)`.

- [ ] **Passo 1: Escrever o teste que falha**

`backend/internal/engine/strength_test.go`:

```go
package engine

import "testing"

func c(r Rank, s Suit) Card { return Card{Rank: r, Suit: s} }

func pc(seat Seat, r Rank, s Suit, covered bool) PlayedCard {
	return PlayedCard{Seat: seat, Card: c(r, s), Covered: covered}
}

func TestManilhaRankIsNextRankCyclically(t *testing.T) {
	cases := map[Rank]Rank{4: 5, 5: 6, 6: 7, 7: 10, 10: 11, 11: 12, 12: 1, 1: 2, 2: 3, 3: 4}
	for vira, want := range cases {
		if got := ManilhaRank(c(vira, Hearts)); got != want {
			t.Errorf("vira %d: manilha = %d, quer %d", vira, got, want)
		}
	}
}

func TestStrengthOrdering(t *testing.T) {
	const manilha Rank = 5
	ordered := []struct {
		name    string
		card    Card
		covered bool
	}{
		{"coberta", c(3, Clubs), true},
		{"4 comum", c(4, Clubs), false},
		{"7 comum", c(7, Hearts), false},
		{"rei", c(12, Spades), false},
		{"ás", c(1, Diamonds), false},
		{"2", c(2, Clubs), false},
		{"3", c(3, Diamonds), false},
		{"manilha ouros", c(5, Diamonds), false},
		{"manilha espadas", c(5, Spades), false},
		{"manilha copas", c(5, Hearts), false},
		{"manilha paus", c(5, Clubs), false},
	}
	for i := 1; i < len(ordered); i++ {
		weaker, stronger := ordered[i-1], ordered[i]
		ws := Strength(weaker.card, weaker.covered, manilha)
		ss := Strength(stronger.card, stronger.covered, manilha)
		if ss <= ws {
			t.Errorf("%s (%d) deveria vencer %s (%d)", stronger.name, ss, weaker.name, ws)
		}
	}
}

func TestCommonCardsOfSameRankTie(t *testing.T) {
	if Strength(c(3, Clubs), false, 5) != Strength(c(3, Diamonds), false, 5) {
		t.Fatal("comuns de mesmo rank deveriam empatar")
	}
}

func TestRoundWinner(t *testing.T) {
	const manilha Rank = 5 // vira 4
	cases := []struct {
		name     string
		table    []PlayedCard
		wantTeam *Team
		wantSeat *Seat
	}{
		{
			name:  "empate entre reis de duplas diferentes",
			table: []PlayedCard{pc(0, 12, Hearts, false), pc(1, 12, Spades, false), pc(2, 4, Diamonds, false), pc(3, 6, Clubs, false)},
		},
		{
			name:     "manilha de paus vence manilha de ouros",
			table:    []PlayedCard{pc(0, 5, Diamonds, false), pc(1, 5, Clubs, false), pc(2, 3, Hearts, false), pc(3, 4, Spades, false)},
			wantTeam: teamPtr(1), wantSeat: seatPtr(1),
		},
		{
			name:     "coberta perde para qualquer aberta",
			table:    []PlayedCard{pc(0, 3, Clubs, true), pc(1, 4, Diamonds, false), pc(2, 2, Clubs, true), pc(3, 1, Clubs, true)},
			wantTeam: teamPtr(1), wantSeat: seatPtr(1),
		},
		{
			name:  "todas cobertas empatam",
			table: []PlayedCard{pc(0, 3, Clubs, true), pc(1, 4, Diamonds, true), pc(2, 2, Clubs, true), pc(3, 1, Clubs, true)},
		},
		{
			name:     "vence o assento do parceiro com a maior",
			table:    []PlayedCard{pc(0, 7, Diamonds, false), pc(1, 4, Hearts, false), pc(2, 3, Spades, false), pc(3, 6, Hearts, false)},
			wantTeam: teamPtr(0), wantSeat: seatPtr(2),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			team, seat := RoundWinner(tc.table, manilha)
			if !equalPtr(team, tc.wantTeam) || !equalPtr(seat, tc.wantSeat) {
				t.Fatalf("RoundWinner = (%v, %v), quer (%v, %v)", deref(team), deref(seat), deref(tc.wantTeam), deref(tc.wantSeat))
			}
		})
	}
}

func equalPtr[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func deref[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}
```

- [ ] **Passo 2: Rodar e verificar que falha**

Run: `make test`
Expected: FAIL de compilação (`undefined: PlayedCard`, `ManilhaRank`...).

- [ ] **Passo 3: Implementar**

`backend/internal/engine/state.go`:

```go
package engine

// Seat é a posição na mesa (0..3, sentido horário).
type Seat uint8

// Team é a dupla: 0 = assentos 0 e 2; 1 = assentos 1 e 3.
type Team uint8

func TeamOf(s Seat) Team  { return Team(s % 2) }
func Partner(s Seat) Seat { return (s + 2) % 4 }
func Next(s Seat) Seat    { return (s + 1) % 4 }
func Other(t Team) Team   { return 1 - t }

func teamPtr(t Team) *Team { return &t }
func seatPtr(s Seat) *Seat { return &s }

func clonePtr[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

type PlayedCard struct {
	Seat    Seat `json:"seat"`
	Card    Card `json:"card"`
	Covered bool `json:"covered"`
}
```

`backend/internal/engine/strength.go`:

```go
package engine

// ManilhaRank é o rank imediatamente acima da vira, de forma cíclica (3 → 4).
func ManilhaRank(vira Card) Rank {
	return rankLadder[(rankIndex(vira.Rank)+1)%len(rankLadder)]
}

// Strength: coberta = 0; comum = 1..10; manilha = 11..14 (desempate por naipe).
func Strength(c Card, covered bool, manilha Rank) int {
	if covered {
		return 0
	}
	if c.Rank == manilha {
		return 11 + int(c.Suit)
	}
	return 1 + rankIndex(c.Rank)
}

// RoundWinner compara a maior carta de cada dupla; devolve nil, nil em empate.
func RoundWinner(table []PlayedCard, manilha Rank) (*Team, *Seat) {
	best := [2]int{-1, -1}
	var bestSeat [2]Seat
	for _, played := range table {
		team := TeamOf(played.Seat)
		strength := Strength(played.Card, played.Covered, manilha)
		if strength > best[team] {
			best[team] = strength
			bestSeat[team] = played.Seat
		}
	}
	if best[0] == best[1] {
		return nil, nil
	}
	winner := Team(0)
	if best[1] > best[0] {
		winner = 1
	}
	return teamPtr(winner), seatPtr(bestSeat[winner])
}
```

- [ ] **Passo 4: Rodar e verificar que passa**

Run: `make test`
Expected: PASS

- [ ] **Passo 5: Commit**

```bash
git add backend/internal/engine
git commit -m "feat(engine): adiciona força das cartas, manilhas e vencedor da rodada"
```

---

### Task 3: Resolução da mão (melar paulista)

**Arquivos:**
- Criar: `backend/internal/engine/scoring.go`
- Teste: `backend/internal/engine/scoring_test.go`

**Interfaces:**
- Consome: `Team`, `teamPtr` (Tarefa 2).
- Produz: `HandWinner(results []*Team) (winner *Team, decided bool)`. `winner == nil && decided` significa que ninguém pontua.

- [ ] **Passo 1: Escrever o teste que falha**

`backend/internal/engine/scoring_test.go`:

```go
package engine

import "testing"

// parseResults converte "AB-" em resultados de rodada (A=0, B=1, -=empate).
func parseResults(s string) []*Team {
	out := make([]*Team, 0, len(s))
	for _, r := range s {
		switch r {
		case 'A':
			out = append(out, teamPtr(0))
		case 'B':
			out = append(out, teamPtr(1))
		default:
			out = append(out, nil)
		}
	}
	return out
}

func TestHandWinner(t *testing.T) {
	cases := []struct {
		rounds  string
		winner  string // "A", "B", "" (ninguém)
		decided bool
	}{
		{"A", "", false},
		{"-", "", false},
		{"AA", "A", true},
		{"A-", "A", true},
		{"-A", "A", true},
		{"-B", "B", true},
		{"--", "", false},
		{"AB", "", false},
		{"ABA", "A", true},
		{"ABB", "B", true},
		{"AB-", "A", true},
		{"BA-", "B", true},
		{"--A", "A", true},
		{"--B", "B", true},
		{"---", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.rounds, func(t *testing.T) {
			winner, decided := HandWinner(parseResults(tc.rounds))
			if decided != tc.decided {
				t.Fatalf("decided = %v, quer %v", decided, tc.decided)
			}
			got := ""
			if winner != nil {
				got = string("AB"[*winner])
			}
			if got != tc.winner {
				t.Fatalf("winner = %q, quer %q", got, tc.winner)
			}
		})
	}
}
```

- [ ] **Passo 2: Rodar e verificar que falha**

Run: `make test`
Expected: FAIL (`undefined: HandWinner`).

- [ ] **Passo 3: Implementar**

`backend/internal/engine/scoring.go`:

```go
package engine

// HandWinner aplica a regra paulista de empates. decided=true com winner=nil
// significa que as três rodadas empataram e ninguém pontua.
func HandWinner(results []*Team) (winner *Team, decided bool) {
	var wins [2]int
	for _, r := range results {
		if r != nil {
			wins[*r]++
		}
	}
	for team := Team(0); team < 2; team++ {
		if wins[team] >= 2 {
			return teamPtr(team), true
		}
	}
	switch len(results) {
	case 2:
		first, second := results[0], results[1]
		if first != nil && second == nil {
			return clonePtr(first), true
		}
		if first == nil && second != nil {
			return clonePtr(second), true
		}
	case 3:
		// Casos restantes: (empate, empate, X) ou (X, Y, empate).
		if results[2] != nil {
			return clonePtr(results[2]), true
		}
		return clonePtr(results[0]), true
	}
	return nil, false
}
```

- [ ] **Passo 4: Rodar e verificar que passa**

Run: `make test`
Expected: PASS

- [ ] **Passo 5: Commit**

```bash
git add backend/internal/engine
git commit -m "feat(engine): adiciona resolução da mão com regra de empate paulista"
```

---

### Task 4: Estado, Apply e distribuição completa (corte, entrega, queima, vira)

**Arquivos:**
- Modificar: `backend/internal/engine/state.go` (anexar tipos de estado e Clone)
- Criar: `backend/internal/engine/actions.go`, `events.go`, `errors.go`, `apply.go`, `deal.go`
- Teste: `backend/internal/engine/helpers_test.go`, `backend/internal/engine/deal_test.go`

**Interfaces:**
- Consome: Tarefas 1–3.
- Produz:
  - Tipos `Phase` (`PhaseWaiting`, `PhaseCutChoice`, `PhaseDealerChoice`, `PhaseBurnFours`, `PhaseElevenDecision`, `PhasePlaying`, `PhaseAwaitingRaiseResponse`, `PhaseHandEnd`, `PhaseGameOver`), `HandKind` (`HandNormal`, `HandEleven`, `HandIron`), `VoteChoice` (`VoteAccept`, `VoteRun`, `VoteRaise`, `VotePlay`), `const WinningScore = 12`.
  - Structs `Player`, `GameState`, `HandState`, `RoundResult`, `PendingRaise` (campos abaixo), `func (GameState) Clone() GameState`, `func (GameState) Cutter() Seat`.
  - `Action` e `ActionKind` (`ActChooseCard`, `ActBurn`, `ActPassBurn`, `ActPlayCard`, `ActRequestRaise`, `ActVote`, `ActSysStartHand`, `ActSysTimeout`).
  - `Event`, `EventKind` (lista completa), `Visibility` + `Allows(Seat) bool`, `visAll`, `visSeat`, `visTeam`, `FilterEvents([]Event, Seat) []Event`.
  - `ErrorCode`, `RuleError`, `ruleErr`.
  - `NewGame(players [4]Player, seed uint64, firstDealer Seat) GameState`, `Apply(GameState, Action) (GameState, []Event, error)`, `dispatch(*GameState, Action) ([]Event, error)`.
  - Em `deal.go`: `startHand`, `chooseCard`, `dealRemaining`, `enterBurn`, `burn`, `resolveBurns`, `revealVira`, `beginAfterVira`, `startPlaying`, `draw`, `handKindFor`.
  - Helpers de teste: `testPlayers`, `newTestGame`, `mustApply`, `wantCode`, `allCards`, `dealWithoutBurning`.

- [ ] **Passo 1: Escrever helpers de teste e testes que falham**

`backend/internal/engine/helpers_test.go`:

```go
package engine

import (
	"errors"
	"testing"
)

var testPlayers = [4]Player{{ID: "p0", Name: "Ana"}, {ID: "p1", Name: "Bia"}, {ID: "p2", Name: "Caio"}, {ID: "p3", Name: "Duda"}}

func newTestGame(seed uint64) GameState { return NewGame(testPlayers, seed, 0) }

func mustApply(t *testing.T, g GameState, a Action) (GameState, []Event) {
	t.Helper()
	next, evs, err := Apply(g, a)
	if err != nil {
		t.Fatalf("Apply(%+v): %v", a, err)
	}
	return next, evs
}

func wantCode(t *testing.T, err error, code ErrorCode) {
	t.Helper()
	var re *RuleError
	if !errors.As(err, &re) || re.Code != code {
		t.Fatalf("erro = %v, quer código %s", err, code)
	}
}

// allCards reúne todas as cartas físicas da mão (monte, oferta, mãos, descarte, vira).
func allCards(g GameState) []Card {
	h := g.Hand
	cards := append([]Card{}, h.Deck...)
	cards = append(cards, h.CutOffer...)
	for _, hand := range h.Hands {
		cards = append(cards, hand...)
	}
	cards = append(cards, h.Discarded...)
	if h.Vira != nil {
		cards = append(cards, *h.Vira)
	}
	return cards
}

// dealWithoutBurning faz as duas escolhas do corte (keep 0) e passa a queima de todos.
func dealWithoutBurning(t *testing.T, g GameState) GameState {
	t.Helper()
	g, _ = mustApply(t, g, Action{Kind: ActChooseCard, Seat: g.Cutter(), Keep: 0})
	g, _ = mustApply(t, g, Action{Kind: ActChooseCard, Seat: g.Dealer, Keep: 0})
	for s := Seat(0); s < 4 && g.Phase == PhaseBurnFours; s++ {
		if !g.Hand.BurnDecided[s] {
			g, _ = mustApply(t, g, Action{Kind: ActPassBurn, Seat: s})
		}
	}
	return g
}

func hasEvent(evs []Event, kind EventKind) bool {
	for _, e := range evs {
		if e.Kind == kind {
			return true
		}
	}
	return false
}
```

`backend/internal/engine/deal_test.go`:

```go
package engine

import (
	"slices"
	"testing"
)

func TestStartHandOffersTwoCardsToCutter(t *testing.T) {
	g, evs := mustApply(t, newTestGame(7), Action{Kind: ActSysStartHand})
	if g.Phase != PhaseCutChoice || g.Cutter() != 3 {
		t.Fatalf("fase = %s, cutter = %d", g.Phase, g.Cutter())
	}
	if len(g.Hand.CutOffer) != 2 || len(g.Hand.Deck) != 38 {
		t.Fatalf("oferta = %d, monte = %d", len(g.Hand.CutOffer), len(g.Hand.Deck))
	}
	if g.Hand.Kind != HandNormal || g.Hand.Stake != 1 || g.HandNumber != 1 {
		t.Fatalf("mão inesperada: kind=%s stake=%d n=%d", g.Hand.Kind, g.Hand.Stake, g.HandNumber)
	}
	for _, e := range evs {
		if e.Kind == EvCutOffered && !(e.Visibility.Allows(3) && !e.Visibility.Allows(0)) {
			t.Fatal("CutOffered deve ser visível só para quem corta")
		}
	}
}

func TestChooseCardRoutesCardsToPartners(t *testing.T) {
	g, _ := mustApply(t, newTestGame(7), Action{Kind: ActSysStartHand})
	offer := slices.Clone(g.Hand.CutOffer)
	g, _ = mustApply(t, g, Action{Kind: ActChooseCard, Seat: 3, Keep: 1})
	if g.Hand.Hands[3][0] != offer[1] || g.Hand.Hands[1][0] != offer[0] {
		t.Fatal("cortador fica com a escolhida e a outra vai para dealer+1")
	}
	if g.Phase != PhaseDealerChoice {
		t.Fatalf("fase = %s", g.Phase)
	}
	offer2 := slices.Clone(g.Hand.CutOffer)
	g, _ = mustApply(t, g, Action{Kind: ActChooseCard, Seat: 0, Keep: 0})
	if g.Hand.Hands[0][0] != offer2[0] || g.Hand.Hands[2][0] != offer2[1] {
		t.Fatal("dealer fica com a escolhida e a outra vai para dealer+2")
	}
	for s, hand := range g.Hand.Hands {
		if len(hand) != 3 {
			t.Fatalf("assento %d tem %d cartas", s, len(hand))
		}
	}
	assertConserved(t, g)
}

func TestChooseCardRejectsWrongSeatAndKeep(t *testing.T) {
	g, _ := mustApply(t, newTestGame(7), Action{Kind: ActSysStartHand})
	_, _, err := Apply(g, Action{Kind: ActChooseCard, Seat: 0, Keep: 0})
	wantCode(t, err, ErrNotYourTurn)
	_, _, err = Apply(g, Action{Kind: ActChooseCard, Seat: 3, Keep: 2})
	wantCode(t, err, ErrInvalidChoice)
	if g.Phase != PhaseCutChoice || len(g.Hand.CutOffer) != 2 {
		t.Fatal("estado original não pode mudar em erro")
	}
}

func TestDealRemainingStartsLeftOfDealer(t *testing.T) {
	h := &HandState{}
	for id := range 8 {
		h.Deck = append(h.Deck, CardFromID(CardID(id)))
	}
	for s := range h.Hands {
		h.Hands[s] = []Card{CardFromID(CardID(30 + s))}
	}
	dealRemaining(h, 0)
	// topo = último; ordem: 1,2,3,0,1,2,3,0
	want := map[Seat][2]CardID{1: {7, 3}, 2: {6, 2}, 3: {5, 1}, 0: {4, 0}}
	for s, ids := range want {
		if h.Hands[s][1].ID() != ids[0] || h.Hands[s][2].ID() != ids[1] {
			t.Fatalf("assento %d recebeu %v", s, h.Hands[s])
		}
	}
}

func burnFixture() [4][]Card {
	return [4][]Card{
		{c(4, Diamonds), c(7, Clubs), c(1, Hearts)},
		{c(5, Diamonds), c(6, Diamonds), c(7, Diamonds)},
		{c(5, Spades), c(6, Spades), c(7, Spades)},
		{c(5, Hearts), c(6, Hearts), c(7, Hearts)},
	}
}

func burnState(hands [4][]Card, deck []Card) GameState {
	g := GameState{Seed: 1, HandNumber: 1, Players: testPlayers, Dealer: 0}
	g.Hand = &HandState{Kind: HandNormal, Stake: 1, Hands: hands, Deck: deck}
	enterBurn(&g)
	return g
}

func TestBurnReplacesFourInSameSlotAndRevealsVira(t *testing.T) {
	g := burnState(burnFixture(), []Card{c(3, Clubs), c(2, Diamonds)})
	if g.Phase != PhaseBurnFours || !g.Hand.BurnDecided[1] || g.Hand.BurnDecided[0] {
		t.Fatalf("fase = %s, decididos = %v", g.Phase, g.Hand.BurnDecided)
	}
	g, evs := mustApply(t, g, Action{Kind: ActBurn, Seat: 0, Slots: []int{0}})
	if g.Hand.Hands[0][0] != c(2, Diamonds) || !slices.Equal(g.Hand.Discarded, []Card{c(4, Diamonds)}) {
		t.Fatalf("mão = %v, descarte = %v", g.Hand.Hands[0], g.Hand.Discarded)
	}
	// Risco tático: a vira 3 faz do 4 queimado uma manilha.
	if *g.Hand.Vira != c(3, Clubs) || g.Hand.Manilha != 4 {
		t.Fatalf("vira = %v, manilha = %d", *g.Hand.Vira, g.Hand.Manilha)
	}
	if g.Phase != PhasePlaying || g.Hand.Turn != 1 || g.Hand.RoundStarter != 1 {
		t.Fatalf("fase = %s, vez = %d", g.Phase, g.Hand.Turn)
	}
	if !hasEvent(evs, EvFoursBurned) || !hasEvent(evs, EvViraRevealed) {
		t.Fatal("faltam eventos de queima e vira")
	}
}

func TestPassBurnKeepsHand(t *testing.T) {
	g := burnState(burnFixture(), []Card{c(3, Clubs), c(2, Diamonds)})
	g, _ = mustApply(t, g, Action{Kind: ActPassBurn, Seat: 0})
	if g.Hand.Hands[0][0] != c(4, Diamonds) || *g.Hand.Vira != c(2, Diamonds) || len(g.Hand.Deck) != 1 {
		t.Fatal("passar não pode alterar a mão; vira deve ser o topo")
	}
}

func TestBurnRejectsInvalidInput(t *testing.T) {
	g := burnState(burnFixture(), []Card{c(3, Clubs), c(2, Diamonds), c(2, Spades)})
	_, _, err := Apply(g, Action{Kind: ActBurn, Seat: 0, Slots: []int{1}})
	wantCode(t, err, ErrBurnNotFour)
	_, _, err = Apply(g, Action{Kind: ActBurn, Seat: 0, Slots: []int{0, 0}})
	wantCode(t, err, ErrInvalidSlot)
	_, _, err = Apply(g, Action{Kind: ActBurn, Seat: 0, Slots: []int{5}})
	wantCode(t, err, ErrInvalidSlot)
	_, _, err = Apply(g, Action{Kind: ActBurn, Seat: 0, Slots: nil})
	wantCode(t, err, ErrInvalidSlot)
	_, _, err = Apply(g, Action{Kind: ActPassBurn, Seat: 1})
	wantCode(t, err, ErrAlreadyDecided)
}

func TestNoFoursSkipsBurnPhase(t *testing.T) {
	hands := burnFixture()
	hands[0][0] = c(3, Diamonds)
	g := burnState(hands, []Card{c(3, Clubs)})
	if g.Phase != PhasePlaying || g.Hand.Vira == nil {
		t.Fatalf("sem 4 na mesa a queima deve ser pulada; fase = %s", g.Phase)
	}
}

func TestReplacementFourCannotBeBurnedAgain(t *testing.T) {
	g := burnState(burnFixture(), []Card{c(3, Clubs), c(4, Clubs)})
	g, _ = mustApply(t, g, Action{Kind: ActBurn, Seat: 0, Slots: []int{0}})
	if g.Hand.Hands[0][0] != c(4, Clubs) || g.Phase != PhasePlaying {
		t.Fatal("o 4 comprado fica na mão e o jogo segue")
	}
}

func TestSimultaneousBurnsResolveClockwiseFromDealerLeft(t *testing.T) {
	hands := burnFixture()
	hands[2][0] = c(4, Spades)
	g := burnState(hands, []Card{c(3, Clubs), c(2, Diamonds), c(2, Spades)})
	g, _ = mustApply(t, g, Action{Kind: ActBurn, Seat: 2, Slots: []int{0}})
	if g.Phase != PhaseBurnFours {
		t.Fatal("deve aguardar a decisão do assento 0")
	}
	g, _ = mustApply(t, g, Action{Kind: ActBurn, Seat: 0, Slots: []int{0}})
	if g.Hand.Hands[2][0] != c(2, Spades) || g.Hand.Hands[0][0] != c(2, Diamonds) {
		t.Fatal("assento 2 resolve antes do assento 0 (ordem a partir de dealer+1)")
	}
	if !slices.Equal(g.Hand.Discarded, []Card{c(4, Spades), c(4, Diamonds)}) {
		t.Fatalf("descarte = %v", g.Hand.Discarded)
	}
}

func TestIronHandDealsBlindWithoutCutOrBurn(t *testing.T) {
	g := newTestGame(9)
	g.Score = [2]int{11, 11}
	g, _ = mustApply(t, g, Action{Kind: ActSysStartHand})
	if g.Hand.Kind != HandIron || g.Phase != PhasePlaying || g.Hand.Stake != 1 {
		t.Fatalf("kind=%s fase=%s stake=%d", g.Hand.Kind, g.Phase, g.Hand.Stake)
	}
	if g.Hand.CutOffer != nil || g.Hand.Vira == nil || len(g.Hand.Deck) != 27 {
		t.Fatal("ferro: sem corte, vira revelada, 13 cartas fora do monte")
	}
	assertConserved(t, g)
}

func TestElevenHandGoesToDecision(t *testing.T) {
	g := newTestGame(11)
	g.Score = [2]int{11, 5}
	g, _ = mustApply(t, g, Action{Kind: ActSysStartHand})
	g = dealWithoutBurning(t, g)
	if g.Phase != PhaseElevenDecision || g.Hand.Stake != 3 || *g.Hand.VotingTeam != 0 {
		t.Fatalf("fase=%s stake=%d", g.Phase, g.Hand.Stake)
	}
}

func TestDealerRotatesEachHand(t *testing.T) {
	g, _ := mustApply(t, newTestGame(3), Action{Kind: ActSysStartHand})
	g.Phase = PhaseHandEnd
	g, _ = mustApply(t, g, Action{Kind: ActSysStartHand})
	if g.Dealer != 1 || g.HandNumber != 2 || g.Cutter() != 0 {
		t.Fatalf("dealer=%d mão=%d", g.Dealer, g.HandNumber)
	}
}

func TestStartHandMidHandFails(t *testing.T) {
	g, _ := mustApply(t, newTestGame(3), Action{Kind: ActSysStartHand})
	_, _, err := Apply(g, Action{Kind: ActSysStartHand})
	wantCode(t, err, ErrInvalidPhase)
}

func TestApplyRejectsSeatOutOfRange(t *testing.T) {
	g, _ := mustApply(t, newTestGame(3), Action{Kind: ActSysStartHand})
	_, _, err := Apply(g, Action{Kind: ActChooseCard, Seat: 7})
	wantCode(t, err, ErrInvalidSeat)
}

func TestFilterEventsByVisibility(t *testing.T) {
	evs := []Event{
		{Kind: EvHandStarted, Visibility: visAll},
		{Kind: EvCutOffered, Visibility: visSeat(3)},
		{Kind: EvVoteCast, Visibility: visTeam(0)},
	}
	if got := FilterEvents(evs, 2); len(got) != 2 || got[1].Kind != EvVoteCast {
		t.Fatalf("assento 2 vê %v", got)
	}
	if got := FilterEvents(evs, 3); len(got) != 2 || got[1].Kind != EvCutOffered {
		t.Fatalf("assento 3 vê %v", got)
	}
}

func assertConserved(t *testing.T, g GameState) {
	t.Helper()
	cards := allCards(g)
	seen := map[Card]bool{}
	for _, card := range cards {
		if seen[card] {
			t.Fatalf("carta duplicada: %+v", card)
		}
		seen[card] = true
	}
	if len(cards) != DeckSize {
		t.Fatalf("%d cartas físicas, quer %d", len(cards), DeckSize)
	}
}
```

- [ ] **Passo 2: Rodar e verificar que falha**

Run: `make test`
Expected: FAIL de compilação (`undefined: Player`, `NewGame`, `Apply`...).

- [ ] **Passo 3: Implementar tipos de estado (anexar ao fim de `state.go`)**

Adicione `import "slices"` no topo de `state.go` e anexe:

```go
type Phase string

const (
	PhaseWaiting               Phase = "waiting"
	PhaseCutChoice             Phase = "cut_choice"
	PhaseDealerChoice          Phase = "dealer_choice"
	PhaseBurnFours             Phase = "burn_fours"
	PhaseElevenDecision        Phase = "eleven_decision"
	PhasePlaying               Phase = "playing"
	PhaseAwaitingRaiseResponse Phase = "awaiting_raise_response"
	PhaseHandEnd               Phase = "hand_end"
	PhaseGameOver              Phase = "game_over"
)

type HandKind string

const (
	HandNormal HandKind = "normal"
	HandEleven HandKind = "eleven"
	HandIron   HandKind = "iron"
)

type VoteChoice string

const (
	VoteAccept VoteChoice = "accept"
	VoteRun    VoteChoice = "run"
	VoteRaise  VoteChoice = "raise"
	VotePlay   VoteChoice = "play"
)

const WinningScore = 12

type Player struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GameState é o estado completo e privado do servidor. Nunca é enviado ao cliente.
type GameState struct {
	Seed       uint64
	HandNumber int // RNG da mão = PCG(Seed, HandNumber)
	Players    [4]Player
	Score      [2]int
	Dealer     Seat
	Phase      Phase
	Hand       *HandState
	Winner     *Team
}

type HandState struct {
	Kind         HandKind
	Deck         []Card // topo = último elemento
	Hands        [4][]Card
	Played       [4][3]bool // slot já jogado; os slots são estáveis
	Discarded    []Card
	CutOffer     []Card
	Vira         *Card
	Manilha      Rank
	Stake        int
	LastRaiser   *Team
	Pending      *PendingRaise
	VotingTeam   *Team
	Votes        [4]*VoteChoice
	BurnDecided  [4]bool
	BurnSlots    [4][]int
	Turn         Seat
	RoundStarter Seat
	Table        []PlayedCard
	Rounds       []RoundResult
}

type RoundResult struct {
	Winner     *Team
	WinnerSeat *Seat
	Starter    Seat
	Cards      []PlayedCard
}

type PendingRaise struct {
	Requester Seat `json:"requester"`
	FromTeam  Team `json:"fromTeam"`
	PrevStake int  `json:"prevStake"`
	ToStake   int  `json:"toStake"`
}

// Cutter é o jogador à direita do dealer.
func (g GameState) Cutter() Seat { return (g.Dealer + 3) % 4 }

// Clone faz cópia profunda; Apply muta apenas o clone.
func (g GameState) Clone() GameState {
	clone := g
	clone.Winner = clonePtr(g.Winner)
	if g.Hand != nil {
		hand := g.Hand.clone()
		clone.Hand = &hand
	}
	return clone
}

func (h HandState) clone() HandState {
	clone := h
	clone.Deck = slices.Clone(h.Deck)
	for s := range 4 {
		clone.Hands[s] = slices.Clone(h.Hands[s])
		clone.BurnSlots[s] = slices.Clone(h.BurnSlots[s])
		clone.Votes[s] = clonePtr(h.Votes[s])
	}
	clone.Discarded = slices.Clone(h.Discarded)
	clone.CutOffer = slices.Clone(h.CutOffer)
	clone.Vira = clonePtr(h.Vira)
	clone.LastRaiser = clonePtr(h.LastRaiser)
	clone.Pending = clonePtr(h.Pending)
	clone.VotingTeam = clonePtr(h.VotingTeam)
	clone.Table = slices.Clone(h.Table)
	clone.Rounds = make([]RoundResult, len(h.Rounds))
	for i, r := range h.Rounds {
		r.Winner = clonePtr(r.Winner)
		r.WinnerSeat = clonePtr(r.WinnerSeat)
		r.Cards = slices.Clone(r.Cards)
		clone.Rounds[i] = r
	}
	return clone
}
```

- [ ] **Passo 4: Implementar ações, eventos e erros**

`backend/internal/engine/actions.go`:

```go
package engine

type ActionKind string

const (
	ActChooseCard   ActionKind = "choose_card"
	ActBurn         ActionKind = "burn"
	ActPassBurn     ActionKind = "pass_burn"
	ActPlayCard     ActionKind = "play_card"
	ActRequestRaise ActionKind = "request_raise"
	ActVote         ActionKind = "vote"
	ActSysStartHand ActionKind = "sys_start_hand"
	ActSysTimeout   ActionKind = "sys_timeout"
)

// Action é plana para ser serializável no log de partidas.
// Seat é preenchido pela sala a partir da sessão, nunca pelo cliente.
type Action struct {
	Kind    ActionKind `json:"kind"`
	Seat    Seat       `json:"seat"`
	Keep    int        `json:"keep,omitempty"`
	Slots   []int      `json:"slots,omitempty"`
	Slot    int        `json:"slot,omitempty"`
	Covered bool       `json:"covered,omitempty"`
	Choice  VoteChoice `json:"choice,omitempty"`
}
```

`backend/internal/engine/events.go`:

```go
package engine

type VisibilityKind string

const (
	VisAll  VisibilityKind = "all"
	VisSeat VisibilityKind = "seat"
	VisTeam VisibilityKind = "team"
)

type Visibility struct {
	Kind VisibilityKind
	Seat Seat
	Team Team
}

func (v Visibility) Allows(seat Seat) bool {
	switch v.Kind {
	case VisAll:
		return true
	case VisSeat:
		return v.Seat == seat
	case VisTeam:
		return TeamOf(seat) == v.Team
	}
	return false
}

var visAll = Visibility{Kind: VisAll}

func visSeat(s Seat) Visibility { return Visibility{Kind: VisSeat, Seat: s} }
func visTeam(t Team) Visibility { return Visibility{Kind: VisTeam, Team: t} }

type EventKind string

const (
	EvHandStarted    EventKind = "hand_started"
	EvCutOffered     EventKind = "cut_offered"
	EvCardChosen     EventKind = "card_chosen"
	EvCardsDealt     EventKind = "cards_dealt"
	EvFoursBurned    EventKind = "fours_burned"
	EvViraRevealed   EventKind = "vira_revealed"
	EvElevenDecided  EventKind = "eleven_decided"
	EvCardPlayed     EventKind = "card_played"
	EvRoundEnded     EventKind = "round_ended"
	EvRaiseRequested EventKind = "raise_requested"
	EvVoteCast       EventKind = "vote_cast"
	EvRaiseResolved  EventKind = "raise_resolved"
	EvRaisePunished  EventKind = "raise_punished"
	EvHandEnded      EventKind = "hand_ended"
	EvGameOver       EventKind = "game_over"
)

// Event serve para animações; nunca carrega cartas privadas.
type Event struct {
	Kind       EventKind  `json:"kind"`
	Visibility Visibility `json:"-"`
	Seat       *Seat      `json:"seat,omitempty"`
	Team       *Team      `json:"team,omitempty"`
	Card       *CardID    `json:"card,omitempty"`
	Count      int        `json:"count,omitempty"`
	Stake      int        `json:"stake,omitempty"`
	Choice     VoteChoice `json:"choice,omitempty"`
	Score      *[2]int    `json:"score,omitempty"`
}

func FilterEvents(evs []Event, seat Seat) []Event {
	visible := []Event{}
	for _, e := range evs {
		if e.Visibility.Allows(seat) {
			visible = append(visible, e)
		}
	}
	return visible
}
```

`backend/internal/engine/errors.go`:

```go
package engine

type ErrorCode string

const (
	ErrInvalidPhase      ErrorCode = "invalid_phase"
	ErrNotYourTurn       ErrorCode = "not_your_turn"
	ErrInvalidSlot       ErrorCode = "invalid_slot"
	ErrInvalidChoice     ErrorCode = "invalid_choice"
	ErrIllegalRaise      ErrorCode = "illegal_raise"
	ErrAlreadyVoted      ErrorCode = "already_voted"
	ErrNotVotingTeam     ErrorCode = "not_voting_team"
	ErrCoveredNotAllowed ErrorCode = "covered_not_allowed"
	ErrBurnNotFour       ErrorCode = "burn_not_four"
	ErrGameOver          ErrorCode = "game_over"
	ErrAlreadyDecided    ErrorCode = "already_decided"
	ErrInvalidSeat       ErrorCode = "invalid_seat"
)

// RuleError indica uma ação ilegal; o estado permanece inalterado.
type RuleError struct {
	Code ErrorCode
	Msg  string
}

func (e *RuleError) Error() string { return string(e.Code) + ": " + e.Msg }

func ruleErr(code ErrorCode, msg string) error { return &RuleError{Code: code, Msg: msg} }
```

- [ ] **Passo 5: Implementar Apply e a distribuição**

`backend/internal/engine/apply.go`:

```go
package engine

func NewGame(players [4]Player, seed uint64, firstDealer Seat) GameState {
	return GameState{Seed: seed, Players: players, Dealer: firstDealer, Phase: PhaseWaiting}
}

// Apply é o reducer puro: em erro devolve o estado original e nenhum evento.
func Apply(s GameState, a Action) (GameState, []Event, error) {
	next := s.Clone()
	evs, err := dispatch(&next, a)
	if err != nil {
		return s, nil, err
	}
	return next, evs, nil
}

func dispatch(g *GameState, a Action) ([]Event, error) {
	if a.Seat > 3 {
		return nil, ruleErr(ErrInvalidSeat, "assento fora de 0..3")
	}
	if g.Phase == PhaseGameOver {
		return nil, ruleErr(ErrGameOver, "partida encerrada")
	}
	switch a.Kind {
	case ActSysStartHand:
		return startHand(g)
	case ActChooseCard:
		return chooseCard(g, a)
	case ActBurn, ActPassBurn:
		return burn(g, a)
	}
	return nil, ruleErr(ErrInvalidChoice, "ação desconhecida: "+string(a.Kind))
}
```

`backend/internal/engine/deal.go`:

```go
package engine

import "slices"

func startHand(g *GameState) ([]Event, error) {
	if g.Phase != PhaseWaiting && g.Phase != PhaseHandEnd {
		return nil, ruleErr(ErrInvalidPhase, "há uma mão em andamento")
	}
	if g.HandNumber > 0 {
		g.Dealer = Next(g.Dealer)
	}
	g.HandNumber++
	deck := NewDeck()
	Shuffle(deck, handRNG(g.Seed, g.HandNumber))
	kind := handKindFor(g.Score)
	hand := &HandState{Kind: kind, Deck: deck, Stake: 1}
	if kind == HandEleven {
		hand.Stake = 3
	}
	g.Hand = hand
	evs := []Event{{Kind: EvHandStarted, Visibility: visAll, Seat: seatPtr(g.Dealer)}}
	if kind == HandIron {
		dealRemaining(hand, g.Dealer)
		evs = append(evs, Event{Kind: EvCardsDealt, Visibility: visAll})
		evs = append(evs, revealVira(g)...)
		return append(evs, beginAfterVira(g)...), nil
	}
	hand.CutOffer = []Card{draw(hand), draw(hand)}
	g.Phase = PhaseCutChoice
	cutter := g.Cutter()
	return append(evs, Event{Kind: EvCutOffered, Visibility: visSeat(cutter), Seat: seatPtr(cutter)}), nil
}

func handKindFor(score [2]int) HandKind {
	teamAOnEleven := score[0] == WinningScore-1
	teamBOnEleven := score[1] == WinningScore-1
	switch {
	case teamAOnEleven && teamBOnEleven:
		return HandIron
	case teamAOnEleven || teamBOnEleven:
		return HandEleven
	}
	return HandNormal
}

func draw(h *HandState) Card {
	top := h.Deck[len(h.Deck)-1]
	h.Deck = h.Deck[:len(h.Deck)-1]
	return top
}

// dealRemaining distribui a partir da esquerda do dealer até todos terem 3 cartas.
func dealRemaining(h *HandState, dealer Seat) {
	for s := Next(dealer); !allHaveThree(h); s = Next(s) {
		if len(h.Hands[s]) < 3 {
			h.Hands[s] = append(h.Hands[s], draw(h))
		}
	}
}

func allHaveThree(h *HandState) bool {
	for _, hand := range h.Hands {
		if len(hand) < 3 {
			return false
		}
	}
	return true
}

func chooseCard(g *GameState, a Action) ([]Event, error) {
	var chooser Seat
	switch g.Phase {
	case PhaseCutChoice:
		chooser = g.Cutter()
	case PhaseDealerChoice:
		chooser = g.Dealer
	default:
		return nil, ruleErr(ErrInvalidPhase, "não há carta para escolher")
	}
	if a.Seat != chooser {
		return nil, ruleErr(ErrNotYourTurn, "não é você quem escolhe")
	}
	if a.Keep != 0 && a.Keep != 1 {
		return nil, ruleErr(ErrInvalidChoice, "keep deve ser 0 ou 1")
	}
	h := g.Hand
	kept, given := h.CutOffer[a.Keep], h.CutOffer[1-a.Keep]
	h.Hands[chooser] = append(h.Hands[chooser], kept)
	h.Hands[Partner(chooser)] = append(h.Hands[Partner(chooser)], given)
	h.CutOffer = nil
	evs := []Event{{Kind: EvCardChosen, Visibility: visAll, Seat: seatPtr(chooser)}}
	if g.Phase == PhaseCutChoice {
		h.CutOffer = []Card{draw(h), draw(h)}
		g.Phase = PhaseDealerChoice
		return append(evs, Event{Kind: EvCutOffered, Visibility: visSeat(g.Dealer), Seat: seatPtr(g.Dealer)}), nil
	}
	dealRemaining(h, g.Dealer)
	evs = append(evs, Event{Kind: EvCardsDealt, Visibility: visAll})
	return append(evs, enterBurn(g)...), nil
}

func enterBurn(g *GameState) []Event {
	h := g.Hand
	for s := range h.Hands {
		h.BurnDecided[s] = !hasFour(h.Hands[s])
	}
	g.Phase = PhaseBurnFours
	if allBurnDecided(h) {
		return resolveBurns(g)
	}
	return nil
}

func hasFour(hand []Card) bool {
	return slices.ContainsFunc(hand, func(card Card) bool { return card.Rank == 4 })
}

func allBurnDecided(h *HandState) bool {
	return !slices.Contains(h.BurnDecided[:], false)
}

// burn trata ActBurn e ActPassBurn; decisões são simultâneas.
func burn(g *GameState, a Action) ([]Event, error) {
	if g.Phase != PhaseBurnFours {
		return nil, ruleErr(ErrInvalidPhase, "não é hora de queimar")
	}
	h := g.Hand
	if h.BurnDecided[a.Seat] {
		return nil, ruleErr(ErrAlreadyDecided, "queima já decidida")
	}
	var slots []int
	if a.Kind == ActBurn {
		if len(a.Slots) == 0 {
			return nil, ruleErr(ErrInvalidSlot, "nenhum slot informado")
		}
		seen := map[int]bool{}
		for _, slot := range a.Slots {
			if slot < 0 || slot >= len(h.Hands[a.Seat]) || seen[slot] {
				return nil, ruleErr(ErrInvalidSlot, "slot inválido ou repetido")
			}
			if h.Hands[a.Seat][slot].Rank != 4 {
				return nil, ruleErr(ErrBurnNotFour, "só é possível queimar 4")
			}
			seen[slot] = true
		}
		slots = slices.Clone(a.Slots)
	}
	h.BurnSlots[a.Seat] = slots
	h.BurnDecided[a.Seat] = true
	if allBurnDecided(h) {
		return resolveBurns(g), nil
	}
	return nil, nil
}

// resolveBurns troca os 4 em ordem horária a partir de dealer+1; não há nova queima.
func resolveBurns(g *GameState) []Event {
	h := g.Hand
	var evs []Event
	for i, s := 0, Next(g.Dealer); i < 4; i, s = i+1, Next(s) {
		for _, slot := range h.BurnSlots[s] {
			h.Discarded = append(h.Discarded, h.Hands[s][slot])
			h.Hands[s][slot] = draw(h)
		}
		if burned := len(h.BurnSlots[s]); burned > 0 {
			evs = append(evs, Event{Kind: EvFoursBurned, Visibility: visAll, Seat: seatPtr(s), Count: burned})
		}
	}
	evs = append(evs, revealVira(g)...)
	return append(evs, beginAfterVira(g)...)
}

func revealVira(g *GameState) []Event {
	h := g.Hand
	vira := draw(h)
	h.Vira = &vira
	h.Manilha = ManilhaRank(vira)
	id := vira.ID()
	return []Event{{Kind: EvViraRevealed, Visibility: visAll, Card: &id}}
}

func beginAfterVira(g *GameState) []Event {
	if g.Hand.Kind == HandEleven {
		teamOnEleven := Team(0)
		if g.Score[1] == WinningScore-1 {
			teamOnEleven = 1
		}
		g.Hand.VotingTeam = teamPtr(teamOnEleven)
		g.Hand.Votes = [4]*VoteChoice{}
		g.Phase = PhaseElevenDecision
		return nil
	}
	startPlaying(g)
	return nil
}

func startPlaying(g *GameState) {
	g.Phase = PhasePlaying
	g.Hand.Turn = Next(g.Dealer)
	g.Hand.RoundStarter = g.Hand.Turn
}
```

- [ ] **Passo 6: Rodar e verificar que passa**

Run: `make test`
Expected: PASS

- [ ] **Passo 7: Commit**

```bash
git add backend/internal/engine
git commit -m "feat(engine): adiciona reducer Apply e distribuição com corte, queima e vira"
```

---

### Task 5: Jogar cartas, fechar rodadas e pontuar a mão

**Arquivos:**
- Modificar: `backend/internal/engine/scoring.go` (anexar funções)
- Modificar: `backend/internal/engine/apply.go` (switch de `dispatch`)
- Teste: `backend/internal/engine/play_test.go`

**Interfaces:**
- Consome: `HandWinner`, `RoundWinner`, estado e eventos (Tarefas 2–4).
- Produz: `playCard(*GameState, Action) ([]Event, error)`, `finishRound(*GameState) []Event`, `endHand(g *GameState, winner *Team, points int) []Event`, `finishGame(g *GameState, winner Team) []Event`. Helpers de teste `playingState`, `fixtureBWinsTwice`, `fixtureTieThenA`, `playRound`.

- [ ] **Passo 1: Escrever o teste que falha**

`backend/internal/engine/play_test.go`:

```go
package engine

import "testing"

// vira 4♦ → manilha 5 (nenhuma nas mãos). Turno inicial: assento 1.
func playingState(hands [4][]Card, kind HandKind) GameState {
	vira := c(4, Diamonds)
	g := GameState{Seed: 1, HandNumber: 1, Players: testPlayers, Dealer: 0, Phase: PhasePlaying}
	stake := 1
	if kind == HandEleven {
		stake = 3
	}
	g.Hand = &HandState{Kind: kind, Hands: hands, Vira: &vira, Manilha: ManilhaRank(vira), Stake: stake, Turn: 1, RoundStarter: 1}
	return g
}

// Com slots 0 e depois 1, a dupla B (assento 1) vence duas rodadas.
func fixtureBWinsTwice() [4][]Card {
	return [4][]Card{
		{c(4, Clubs), c(6, Clubs), c(7, Clubs)},
		{c(3, Hearts), c(2, Hearts), c(1, Hearts)},
		{c(4, Spades), c(6, Spades), c(7, Spades)},
		{c(4, Hearts), c(6, Hearts), c(7, Hearts)},
	}
}

// Slot 0: empate (7 contra 7). Slot 1: dupla A vence com 3♣.
func fixtureTieThenA() [4][]Card {
	return [4][]Card{
		{c(7, Clubs), c(3, Clubs), c(4, Clubs)},
		{c(7, Hearts), c(2, Hearts), c(4, Hearts)},
		{c(6, Spades), c(1, Spades), c(4, Spades)},
		{c(6, Hearts), c(1, Hearts), c(7, Spades)},
	}
}

func playRound(t *testing.T, g GameState, slot int) GameState {
	t.Helper()
	for range 4 {
		g, _ = mustApply(t, g, Action{Kind: ActPlayCard, Seat: g.Hand.Turn, Slot: slot})
	}
	return g
}

func TestPlayCardRotatesTurn(t *testing.T) {
	g := playingState(fixtureBWinsTwice(), HandNormal)
	_, _, err := Apply(g, Action{Kind: ActPlayCard, Seat: 2, Slot: 0})
	wantCode(t, err, ErrNotYourTurn)
	g, evs := mustApply(t, g, Action{Kind: ActPlayCard, Seat: 1, Slot: 0})
	if g.Hand.Turn != 2 || len(g.Hand.Table) != 1 || evs[0].Card == nil {
		t.Fatalf("vez=%d mesa=%d", g.Hand.Turn, len(g.Hand.Table))
	}
}

func TestRoundWinnerStartsNextRoundAndPlayedSlotCannotBeReused(t *testing.T) {
	g := playRound(t, playingState(fixtureBWinsTwice(), HandNormal), 0)
	if g.Hand.Turn != 1 || g.Hand.RoundStarter != 1 || len(g.Hand.Rounds) != 1 {
		t.Fatalf("vez=%d rodadas=%d", g.Hand.Turn, len(g.Hand.Rounds))
	}
	_, _, err := Apply(g, Action{Kind: ActPlayCard, Seat: 1, Slot: 0})
	wantCode(t, err, ErrInvalidSlot)
	g = playRound(t, g, 1)
	if g.Score != [2]int{0, 1} || g.Phase != PhaseHandEnd {
		t.Fatalf("placar=%v fase=%s", g.Score, g.Phase)
	}
}

func TestTiedRoundRestartsWithSameStarter(t *testing.T) {
	g := playRound(t, playingState(fixtureTieThenA(), HandNormal), 0)
	if g.Hand.Rounds[0].Winner != nil || g.Hand.Turn != 1 {
		t.Fatal("rodada empatada deve recomeçar por quem a abriu")
	}
	g = playRound(t, g, 1)
	if g.Score != [2]int{1, 0} || g.Phase != PhaseHandEnd {
		t.Fatalf("empate + vitória da A: placar=%v", g.Score)
	}
}

func TestCoveredCardLosesAndHidesIdentityInEvent(t *testing.T) {
	g := playingState(fixtureBWinsTwice(), HandNormal)
	g, evs := mustApply(t, g, Action{Kind: ActPlayCard, Seat: 1, Slot: 0, Covered: true})
	if evs[0].Card != nil {
		t.Fatal("evento de carta coberta não pode trazer a carta")
	}
	g, _ = mustApply(t, g, Action{Kind: ActPlayCard, Seat: 2, Slot: 0})
	g, _ = mustApply(t, g, Action{Kind: ActPlayCard, Seat: 3, Slot: 1})
	g, _ = mustApply(t, g, Action{Kind: ActPlayCard, Seat: 0, Slot: 0})
	if *g.Hand.Rounds[0].WinnerSeat != 3 {
		t.Fatalf("vencedor = %d, quer 3 (6♥ vence 4s; 3♥ coberto vale 0)", *g.Hand.Rounds[0].WinnerSeat)
	}
}

func TestCoveredNotAllowedInIronHand(t *testing.T) {
	g := playingState(fixtureBWinsTwice(), HandIron)
	_, _, err := Apply(g, Action{Kind: ActPlayCard, Seat: 1, Slot: 0, Covered: true})
	wantCode(t, err, ErrCoveredNotAllowed)
}

func TestReachingTwelveEndsGameAndBlocksFurtherActions(t *testing.T) {
	g := playingState(fixtureBWinsTwice(), HandNormal)
	g.Score = [2]int{0, 11}
	g = playRound(t, playRound(t, g, 0), 1)
	if g.Phase != PhaseGameOver || g.Winner == nil || *g.Winner != 1 || g.Score[1] != 12 {
		t.Fatalf("fase=%s placar=%v", g.Phase, g.Score)
	}
	_, _, err := Apply(g, Action{Kind: ActSysStartHand})
	wantCode(t, err, ErrGameOver)
}
```

- [ ] **Passo 2: Rodar e verificar que falha**

Run: `make test`
Expected: FAIL (`ação desconhecida: play_card` ou `undefined: playCard`).

- [ ] **Passo 3: Implementar**

Anexe a `backend/internal/engine/scoring.go`:

```go
func playCard(g *GameState, a Action) ([]Event, error) {
	if g.Phase != PhasePlaying {
		return nil, ruleErr(ErrInvalidPhase, "não é hora de jogar carta")
	}
	h := g.Hand
	if a.Seat != h.Turn {
		return nil, ruleErr(ErrNotYourTurn, "não é sua vez")
	}
	if a.Slot < 0 || a.Slot >= len(h.Hands[a.Seat]) || h.Played[a.Seat][a.Slot] {
		return nil, ruleErr(ErrInvalidSlot, "slot inválido ou já jogado")
	}
	if a.Covered && h.Kind == HandIron {
		return nil, ruleErr(ErrCoveredNotAllowed, "não há carta coberta na mão de ferro")
	}
	card := h.Hands[a.Seat][a.Slot]
	h.Played[a.Seat][a.Slot] = true
	h.Table = append(h.Table, PlayedCard{Seat: a.Seat, Card: card, Covered: a.Covered})
	played := Event{Kind: EvCardPlayed, Visibility: visAll, Seat: seatPtr(a.Seat)}
	if !a.Covered {
		id := card.ID()
		played.Card = &id
	}
	evs := []Event{played}
	if len(h.Table) < 4 {
		h.Turn = Next(a.Seat)
		return evs, nil
	}
	return append(evs, finishRound(g)...), nil
}

func finishRound(g *GameState) []Event {
	h := g.Hand
	winner, winnerSeat := RoundWinner(h.Table, h.Manilha)
	h.Rounds = append(h.Rounds, RoundResult{Winner: winner, WinnerSeat: winnerSeat, Starter: h.RoundStarter, Cards: h.Table})
	h.Table = nil
	evs := []Event{{Kind: EvRoundEnded, Visibility: visAll, Team: clonePtr(winner), Seat: clonePtr(winnerSeat)}}
	results := make([]*Team, len(h.Rounds))
	for i, r := range h.Rounds {
		results[i] = r.Winner
	}
	if handWinner, decided := HandWinner(results); decided {
		return append(evs, endHand(g, handWinner, h.Stake)...)
	}
	nextStarter := h.RoundStarter // empate: recomeça quem abriu a rodada
	if winnerSeat != nil {
		nextStarter = *winnerSeat
	}
	h.RoundStarter, h.Turn = nextStarter, nextStarter
	return evs
}

func endHand(g *GameState, winner *Team, points int) []Event {
	if winner != nil {
		g.Score[*winner] = min(g.Score[*winner]+points, WinningScore)
	}
	score := g.Score
	evs := []Event{{Kind: EvHandEnded, Visibility: visAll, Team: clonePtr(winner), Stake: points, Score: &score}}
	g.Hand.Pending, g.Hand.VotingTeam = nil, nil
	if winner != nil && g.Score[*winner] >= WinningScore {
		return append(evs, finishGame(g, *winner)...)
	}
	g.Phase = PhaseHandEnd
	return evs
}

func finishGame(g *GameState, winner Team) []Event {
	g.Winner = teamPtr(winner)
	g.Phase = PhaseGameOver
	score := g.Score
	return []Event{{Kind: EvGameOver, Visibility: visAll, Team: teamPtr(winner), Score: &score}}
}
```

Em `apply.go`, substitua o `switch` de `dispatch` por:

```go
	switch a.Kind {
	case ActSysStartHand:
		return startHand(g)
	case ActChooseCard:
		return chooseCard(g, a)
	case ActBurn, ActPassBurn:
		return burn(g, a)
	case ActPlayCard:
		return playCard(g, a)
	}
```

- [ ] **Passo 4: Rodar e verificar que passa**

Run: `make test`
Expected: PASS

- [ ] **Passo 5: Commit**

```bash
git add backend/internal/engine
git commit -m "feat(engine): adiciona jogadas, fechamento de rodadas e pontuação da mão"
```

---

### Task 6: Apostas (Truco/6/9/12), voto da dupla, punição e Mão de 11

**Arquivos:**
- Criar: `backend/internal/engine/betting.go`
- Modificar: `backend/internal/engine/apply.go` (switch de `dispatch`)
- Teste: `backend/internal/engine/betting_test.go`

**Interfaces:**
- Consome: `endHand`, `finishGame`, `startPlaying`, `playingState`, `fixtureBWinsTwice`, `dealWithoutBurning` (Tarefas 4–5).
- Produz: `stakeLadder`, `nextStake(int) (int, bool)`, `requestRaise`, `openVote`, `vote`, `validChoice`, `boldest(a, b VoteChoice) VoteChoice`, `resolveRaise`, `resolveEleven`.

- [ ] **Passo 1: Escrever o teste que falha**

`backend/internal/engine/betting_test.go`:

```go
package engine

import "testing"

func act(kind ActionKind, seat Seat) Action { return Action{Kind: kind, Seat: seat} }
func voteAs(seat Seat, choice VoteChoice) Action {
	return Action{Kind: ActVote, Seat: seat, Choice: choice}
}

func TestRaiseAcceptedReturnsTurnToRequester(t *testing.T) {
	g := playingState(fixtureBWinsTwice(), HandNormal)
	g, _ = mustApply(t, g, act(ActRequestRaise, 1))
	if g.Phase != PhaseAwaitingRaiseResponse || *g.Hand.VotingTeam != 0 {
		t.Fatalf("fase=%s", g.Phase)
	}
	if *g.Hand.Pending != (PendingRaise{Requester: 1, FromTeam: 1, PrevStake: 1, ToStake: 3}) {
		t.Fatalf("pending = %+v", *g.Hand.Pending)
	}
	_, _, err := Apply(g, voteAs(1, VoteAccept))
	wantCode(t, err, ErrNotVotingTeam)
	g, _ = mustApply(t, g, voteAs(0, VoteAccept))
	if g.Phase != PhaseAwaitingRaiseResponse {
		t.Fatal("aguarda o voto do parceiro")
	}
	_, _, err = Apply(g, voteAs(0, VoteRun))
	wantCode(t, err, ErrAlreadyVoted)
	g, _ = mustApply(t, g, voteAs(2, VoteRun)) // mais ousado prevalece: aceitar
	if g.Phase != PhasePlaying || g.Hand.Stake != 3 || g.Hand.Turn != 1 || *g.Hand.LastRaiser != 1 {
		t.Fatalf("fase=%s stake=%d vez=%d", g.Phase, g.Hand.Stake, g.Hand.Turn)
	}
	_, _, err = Apply(g, act(ActRequestRaise, 1))
	wantCode(t, err, ErrIllegalRaise) // mesma dupla não aumenta duas vezes seguidas
}

func TestRaiseRunAwardsPreviousStake(t *testing.T) {
	g := playingState(fixtureBWinsTwice(), HandNormal)
	g, _ = mustApply(t, g, act(ActRequestRaise, 1))
	g, _ = mustApply(t, g, voteAs(0, VoteRun))
	g, _ = mustApply(t, g, voteAs(2, VoteRun))
	if g.Score != [2]int{0, 1} || g.Phase != PhaseHandEnd {
		t.Fatalf("placar=%v fase=%s", g.Score, g.Phase)
	}
}

func TestCounterRaiseFlipsVotingTeam(t *testing.T) {
	g := playingState(fixtureBWinsTwice(), HandNormal)
	g, _ = mustApply(t, g, act(ActRequestRaise, 1))
	g, _ = mustApply(t, g, voteAs(0, VoteRaise))
	g, _ = mustApply(t, g, voteAs(2, VoteRun))
	if *g.Hand.Pending != (PendingRaise{Requester: 1, FromTeam: 0, PrevStake: 3, ToStake: 6}) || *g.Hand.VotingTeam != 1 {
		t.Fatalf("pending = %+v", *g.Hand.Pending)
	}
	g, _ = mustApply(t, g, voteAs(1, VoteAccept))
	g, _ = mustApply(t, g, voteAs(3, VoteAccept))
	if g.Hand.Stake != 6 || g.Hand.Turn != 1 || *g.Hand.LastRaiser != 0 || g.Phase != PhasePlaying {
		t.Fatalf("stake=%d vez=%d", g.Hand.Stake, g.Hand.Turn)
	}
	g, _ = mustApply(t, g, act(ActRequestRaise, 1)) // B pode pedir 9: o último aumento foi da A
	if g.Hand.Pending.ToStake != 9 {
		t.Fatalf("toStake = %d", g.Hand.Pending.ToStake)
	}
}

func TestRunAfterCounterRaiseAwardsAcceptedStake(t *testing.T) {
	g := playingState(fixtureBWinsTwice(), HandNormal)
	g, _ = mustApply(t, g, act(ActRequestRaise, 1))
	g, _ = mustApply(t, g, voteAs(0, VoteRaise))
	g, _ = mustApply(t, g, voteAs(2, VoteRaise))
	g, _ = mustApply(t, g, voteAs(1, VoteRun))
	g, _ = mustApply(t, g, voteAs(3, VoteRun))
	if g.Score != [2]int{3, 0} {
		t.Fatalf("placar = %v, quer A=3", g.Score)
	}
}

func TestRaiseCeilingAtTwelve(t *testing.T) {
	g := playingState(fixtureBWinsTwice(), HandNormal)
	g.Hand.Stake = 12
	_, _, err := Apply(g, act(ActRequestRaise, 1))
	wantCode(t, err, ErrIllegalRaise)

	g.Hand.Stake = 9
	g, _ = mustApply(t, g, act(ActRequestRaise, 1))
	_, _, err = Apply(g, voteAs(0, VoteRaise))
	wantCode(t, err, ErrInvalidChoice)
}

func TestRaiseOnlyOnOwnTurn(t *testing.T) {
	g := playingState(fixtureBWinsTwice(), HandNormal)
	_, _, err := Apply(g, act(ActRequestRaise, 2))
	wantCode(t, err, ErrNotYourTurn)
}

func TestRaiseInElevenOrIronHandLosesTheGame(t *testing.T) {
	for _, kind := range []HandKind{HandEleven, HandIron} {
		g := playingState(fixtureBWinsTwice(), kind)
		g.Score = [2]int{11, 4}
		g, _ = mustApply(t, g, act(ActRequestRaise, 1))
		if g.Phase != PhaseGameOver || *g.Winner != 0 || g.Score[0] != 12 {
			t.Fatalf("%s: fase=%s placar=%v", kind, g.Phase, g.Score)
		}
	}
}

func TestElevenDecisionPlayOrRun(t *testing.T) {
	start := newTestGame(11)
	start.Score = [2]int{11, 3}
	start, _ = mustApply(t, start, Action{Kind: ActSysStartHand})
	start = dealWithoutBurning(t, start)

	_, _, err := Apply(start, voteAs(1, VotePlay))
	wantCode(t, err, ErrNotVotingTeam)
	_, _, err = Apply(start, voteAs(0, VoteAccept))
	wantCode(t, err, ErrInvalidChoice)

	played, _ := mustApply(t, start, voteAs(0, VotePlay))
	played, _ = mustApply(t, played, voteAs(2, VoteRun))
	if played.Phase != PhasePlaying || played.Hand.Stake != 3 {
		t.Fatalf("jogar prevalece: fase=%s stake=%d", played.Phase, played.Hand.Stake)
	}

	ran, _ := mustApply(t, start, voteAs(0, VoteRun))
	ran, _ = mustApply(t, ran, voteAs(2, VoteRun))
	if ran.Score != [2]int{11, 4} || ran.Phase != PhaseHandEnd {
		t.Fatalf("correr da mão de 11: placar=%v", ran.Score)
	}
}
```

- [ ] **Passo 2: Rodar e verificar que falha**

Run: `make test`
Expected: FAIL (`ação desconhecida: request_raise`).

- [ ] **Passo 3: Implementar**

`backend/internal/engine/betting.go`:

```go
package engine

import "slices"

var stakeLadder = []int{1, 3, 6, 9, 12}

func nextStake(stake int) (int, bool) {
	i := slices.Index(stakeLadder, stake)
	if i < 0 || i == len(stakeLadder)-1 {
		return 0, false
	}
	return stakeLadder[i+1], true
}

func requestRaise(g *GameState, a Action) ([]Event, error) {
	if g.Phase != PhasePlaying {
		return nil, ruleErr(ErrInvalidPhase, "não é hora de pedir truco")
	}
	h := g.Hand
	if a.Seat != h.Turn {
		return nil, ruleErr(ErrNotYourTurn, "só se pede truco na própria vez")
	}
	team := TeamOf(a.Seat)
	if h.Kind == HandEleven || h.Kind == HandIron {
		opponent := Other(team)
		g.Score[opponent] = WinningScore
		evs := []Event{{Kind: EvRaisePunished, Visibility: visAll, Seat: seatPtr(a.Seat), Team: teamPtr(team)}}
		return append(evs, finishGame(g, opponent)...), nil
	}
	if h.LastRaiser != nil && *h.LastRaiser == team {
		return nil, ruleErr(ErrIllegalRaise, "sua dupla aumentou por último")
	}
	to, ok := nextStake(h.Stake)
	if !ok {
		return nil, ruleErr(ErrIllegalRaise, "aposta já está no máximo")
	}
	h.Pending = &PendingRaise{Requester: a.Seat, FromTeam: team, PrevStake: h.Stake, ToStake: to}
	openVote(g, Other(team), PhaseAwaitingRaiseResponse)
	return []Event{{Kind: EvRaiseRequested, Visibility: visAll, Seat: seatPtr(a.Seat), Team: teamPtr(team), Stake: to}}, nil
}

func openVote(g *GameState, team Team, phase Phase) {
	g.Hand.VotingTeam = teamPtr(team)
	g.Hand.Votes = [4]*VoteChoice{}
	g.Phase = phase
}

func vote(g *GameState, a Action) ([]Event, error) {
	if g.Phase != PhaseAwaitingRaiseResponse && g.Phase != PhaseElevenDecision {
		return nil, ruleErr(ErrInvalidPhase, "não há votação aberta")
	}
	h := g.Hand
	if TeamOf(a.Seat) != *h.VotingTeam {
		return nil, ruleErr(ErrNotVotingTeam, "sua dupla não está votando")
	}
	if h.Votes[a.Seat] != nil {
		return nil, ruleErr(ErrAlreadyVoted, "voto já registrado")
	}
	if !validChoice(g, a.Choice) {
		return nil, ruleErr(ErrInvalidChoice, "opção de voto inválida: "+string(a.Choice))
	}
	choice := a.Choice
	h.Votes[a.Seat] = &choice
	evs := []Event{{Kind: EvVoteCast, Visibility: visTeam(*h.VotingTeam), Seat: seatPtr(a.Seat), Choice: choice}}
	partnerVote := h.Votes[Partner(a.Seat)]
	if partnerVote == nil {
		return evs, nil
	}
	final := boldest(choice, *partnerVote)
	if g.Phase == PhaseElevenDecision {
		return append(evs, resolveEleven(g, final)...), nil
	}
	return append(evs, resolveRaise(g, final)...), nil
}

func validChoice(g *GameState, choice VoteChoice) bool {
	if g.Phase == PhaseElevenDecision {
		return choice == VotePlay || choice == VoteRun
	}
	switch choice {
	case VoteAccept, VoteRun:
		return true
	case VoteRaise:
		_, canRaise := nextStake(g.Hand.Pending.ToStake)
		return canRaise
	}
	return false
}

// boldness ordena os votos: na divergência prevalece o mais ousado.
var boldness = map[VoteChoice]int{VoteRun: 0, VoteAccept: 1, VotePlay: 2, VoteRaise: 2}

func boldest(a, b VoteChoice) VoteChoice {
	if boldness[b] > boldness[a] {
		return b
	}
	return a
}

func resolveRaise(g *GameState, choice VoteChoice) []Event {
	h := g.Hand
	pending := *h.Pending
	voting := *h.VotingTeam
	switch choice {
	case VoteAccept:
		h.Stake = pending.ToStake
		h.LastRaiser = teamPtr(pending.FromTeam)
		h.Pending, h.VotingTeam = nil, nil
		h.Votes = [4]*VoteChoice{}
		h.Turn = pending.Requester
		g.Phase = PhasePlaying
		return []Event{{Kind: EvRaiseResolved, Visibility: visAll, Team: teamPtr(voting), Choice: VoteAccept, Stake: h.Stake}}
	case VoteRun:
		evs := []Event{{Kind: EvRaiseResolved, Visibility: visAll, Team: teamPtr(voting), Choice: VoteRun, Stake: pending.PrevStake}}
		return append(evs, endHand(g, teamPtr(pending.FromTeam), pending.PrevStake)...)
	}
	to, _ := nextStake(pending.ToStake) // validChoice já garantiu o próximo degrau
	h.Pending = &PendingRaise{Requester: pending.Requester, FromTeam: voting, PrevStake: pending.ToStake, ToStake: to}
	h.LastRaiser = teamPtr(voting)
	openVote(g, Other(voting), PhaseAwaitingRaiseResponse)
	return []Event{{Kind: EvRaiseRequested, Visibility: visAll, Team: teamPtr(voting), Stake: to}}
}

func resolveEleven(g *GameState, choice VoteChoice) []Event {
	team := *g.Hand.VotingTeam
	decided := Event{Kind: EvElevenDecided, Visibility: visAll, Team: teamPtr(team), Choice: choice}
	g.Hand.VotingTeam = nil
	g.Hand.Votes = [4]*VoteChoice{}
	if choice == VoteRun {
		return append([]Event{decided}, endHand(g, teamPtr(Other(team)), 1)...)
	}
	startPlaying(g)
	return []Event{decided}
}
```

Em `apply.go`, substitua o `switch` de `dispatch` por:

```go
	switch a.Kind {
	case ActSysStartHand:
		return startHand(g)
	case ActChooseCard:
		return chooseCard(g, a)
	case ActBurn, ActPassBurn:
		return burn(g, a)
	case ActPlayCard:
		return playCard(g, a)
	case ActRequestRaise:
		return requestRaise(g, a)
	case ActVote:
		return vote(g, a)
	}
```

- [ ] **Passo 4: Rodar e verificar que passa**

Run: `make test`
Expected: PASS

- [ ] **Passo 5: Commit**

```bash
git add backend/internal/engine
git commit -m "feat(engine): adiciona apostas com voto da dupla, punição e mão de 11"
```

---

### Task 7: Ação padrão por fase e `SysTimeout`

**Arquivos:**
- Criar: `backend/internal/engine/timeout.go`
- Modificar: `backend/internal/engine/apply.go` (switch de `dispatch`)
- Teste: `backend/internal/engine/timeout_test.go`

**Interfaces:**
- Consome: todo o reducer (Tarefas 4–6).
- Produz: `DefaultAction(g GameState, seat Seat) (Action, bool)`; `ActSysTimeout` despacha a ação padrão.

- [ ] **Passo 1: Escrever o teste que falha**

`backend/internal/engine/timeout_test.go`:

```go
package engine

import "testing"

func timeout(seat Seat) Action { return Action{Kind: ActSysTimeout, Seat: seat} }

func TestTimeoutInCutChoiceKeepsFirstCard(t *testing.T) {
	g, _ := mustApply(t, newTestGame(5), Action{Kind: ActSysStartHand})
	first := g.Hand.CutOffer[0]
	_, _, err := Apply(g, timeout(0))
	wantCode(t, err, ErrInvalidPhase) // o assento 0 não tem nada pendente
	g, _ = mustApply(t, g, timeout(3))
	if g.Phase != PhaseDealerChoice || g.Hand.Hands[3][0] != first {
		t.Fatal("timeout do corte mantém a primeira carta")
	}
}

func TestTimeoutInBurnPasses(t *testing.T) {
	g := burnState(burnFixture(), []Card{c(3, Clubs), c(2, Diamonds)})
	g, _ = mustApply(t, g, timeout(0))
	if g.Phase != PhasePlaying || g.Hand.Hands[0][0] != c(4, Diamonds) {
		t.Fatal("timeout na queima equivale a passar")
	}
}

func TestTimeoutInPlayPlaysLowestSlotCovered(t *testing.T) {
	g, _ := mustApply(t, playingState(fixtureBWinsTwice(), HandNormal), timeout(1))
	if pc := g.Hand.Table[0]; pc.Seat != 1 || !pc.Covered || pc.Card != c(3, Hearts) {
		t.Fatalf("mesa = %+v", g.Hand.Table)
	}
	iron, _ := mustApply(t, playingState(fixtureBWinsTwice(), HandIron), timeout(1))
	if iron.Hand.Table[0].Covered {
		t.Fatal("na mão de ferro o timeout joga aberta")
	}
}

func TestTimeoutInVoteRuns(t *testing.T) {
	g := playingState(fixtureBWinsTwice(), HandNormal)
	g, _ = mustApply(t, g, Action{Kind: ActRequestRaise, Seat: 1})
	g, _ = mustApply(t, g, timeout(0))
	g, _ = mustApply(t, g, timeout(2))
	if g.Score != [2]int{0, 1} || g.Phase != PhaseHandEnd {
		t.Fatalf("placar=%v", g.Score)
	}
}
```

- [ ] **Passo 2: Rodar e verificar que falha**

Run: `make test`
Expected: FAIL (`ação desconhecida: sys_timeout`).

- [ ] **Passo 3: Implementar**

`backend/internal/engine/timeout.go`:

```go
package engine

// DefaultAction é o que acontece quando o tempo do jogador acaba.
// ok=false significa que o assento não tem decisão pendente.
func DefaultAction(g GameState, seat Seat) (Action, bool) {
	h := g.Hand
	switch g.Phase {
	case PhaseCutChoice:
		if seat == g.Cutter() {
			return Action{Kind: ActChooseCard, Seat: seat, Keep: 0}, true
		}
	case PhaseDealerChoice:
		if seat == g.Dealer {
			return Action{Kind: ActChooseCard, Seat: seat, Keep: 0}, true
		}
	case PhaseBurnFours:
		if !h.BurnDecided[seat] {
			return Action{Kind: ActPassBurn, Seat: seat}, true
		}
	case PhaseElevenDecision, PhaseAwaitingRaiseResponse:
		if TeamOf(seat) == *h.VotingTeam && h.Votes[seat] == nil {
			return Action{Kind: ActVote, Seat: seat, Choice: VoteRun}, true
		}
	case PhasePlaying:
		if seat != h.Turn {
			break
		}
		for slot := range h.Hands[seat] {
			if !h.Played[seat][slot] {
				return Action{Kind: ActPlayCard, Seat: seat, Slot: slot, Covered: h.Kind != HandIron}, true
			}
		}
	}
	return Action{}, false
}
```

Em `apply.go`, substitua o `switch` de `dispatch` pela versão final:

```go
	switch a.Kind {
	case ActSysStartHand:
		return startHand(g)
	case ActSysTimeout:
		fallback, ok := DefaultAction(*g, a.Seat)
		if !ok {
			return nil, ruleErr(ErrInvalidPhase, "nenhuma decisão pendente para este assento")
		}
		return dispatch(g, fallback)
	case ActChooseCard:
		return chooseCard(g, a)
	case ActBurn, ActPassBurn:
		return burn(g, a)
	case ActPlayCard:
		return playCard(g, a)
	case ActRequestRaise:
		return requestRaise(g, a)
	case ActVote:
		return vote(g, a)
	}
```

- [ ] **Passo 4: Rodar e verificar que passa**

Run: `make test`
Expected: PASS

- [ ] **Passo 5: Commit**

```bash
git add backend/internal/engine
git commit -m "feat(engine): adiciona ação padrão por fase para timeouts"
```

---

### Task 8: Projeção anti-cheat (`ViewFor`)

**Arquivos:**
- Criar: `backend/internal/engine/view.go`
- Teste: `backend/internal/engine/view_test.go`

**Interfaces:**
- Consome: estado completo (Tarefas 4–7).
- Produz: `PlayerInfo`, `HandSlotView`, `TableCardView`, `RoundView`, `OfferCardView`, `PlayerView`, `ViewFor(g GameState, seat Seat) PlayerView`, `currentChooser(GameState) (Seat, bool)`, `teamIsVoting(GameState, Seat) bool`. Chaves JSON que carregam carta: **somente** `card` e `vira` (o teste de vazamento da Tarefa 9 depende disso).

- [ ] **Passo 1: Escrever o teste que falha**

`backend/internal/engine/view_test.go`:

```go
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
```

- [ ] **Passo 2: Rodar e verificar que falha**

Run: `make test`
Expected: FAIL (`undefined: ViewFor`).

- [ ] **Passo 3: Implementar**

`backend/internal/engine/view.go`:

```go
package engine

type PlayerInfo struct {
	Seat      Seat   `json:"seat"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	CardCount int    `json:"cardCount"`
	Voted     bool   `json:"voted"`
}

type HandSlotView struct {
	Slot   int     `json:"slot"`
	Card   *CardID `json:"card,omitempty"`
	Hidden bool    `json:"hidden"`
}

type TableCardView struct {
	Seat    Seat    `json:"seat"`
	Card    *CardID `json:"card,omitempty"`
	Covered bool    `json:"covered"`
}

type RoundView struct {
	Winner     *Team           `json:"winner"`
	WinnerSeat *Seat           `json:"winnerSeat"`
	Cards      []TableCardView `json:"cards"`
}

// OfferCardView evita []CardID, que o encoding/json serializaria como base64.
type OfferCardView struct {
	Keep int    `json:"keep"`
	Card CardID `json:"card"`
}

// PlayerView é a projeção enviada a um assento. Chaves com carta: só "card" e "vira".
type PlayerView struct {
	Seat         Seat            `json:"seat"`
	Phase        Phase           `json:"phase"`
	HandKind     HandKind        `json:"handKind,omitempty"`
	HandNumber   int             `json:"handNumber"`
	Players      [4]PlayerInfo   `json:"players"`
	Score        [2]int          `json:"score"`
	Dealer       Seat            `json:"dealer"`
	Turn         *Seat           `json:"turn,omitempty"`
	Stake        int             `json:"stake"`
	LastRaiser   *Team           `json:"lastRaiser,omitempty"`
	Pending      *PendingRaise   `json:"pending,omitempty"`
	VotingTeam   *Team           `json:"votingTeam,omitempty"`
	Vira         *CardID         `json:"vira,omitempty"`
	Manilha      *Rank           `json:"manilha,omitempty"`
	Table        []TableCardView `json:"table"`
	Rounds       []RoundView     `json:"rounds"`
	DeckCount    int             `json:"deckCount"`
	DiscardCount int             `json:"discardCount"`
	MyHand       []HandSlotView  `json:"myHand"`
	CutOffer     []OfferCardView `json:"cutOffer,omitempty"`
	PartnerHand  []HandSlotView  `json:"partnerHand,omitempty"`
	PartnerVote  *VoteChoice     `json:"partnerVote,omitempty"`
	Winner       *Team           `json:"winner,omitempty"`
}

// ViewFor é o único caminho pelo qual o estado sai do servidor.
func ViewFor(g GameState, seat Seat) PlayerView {
	v := PlayerView{
		Seat: seat, Phase: g.Phase, HandNumber: g.HandNumber, Score: g.Score, Dealer: g.Dealer,
		Winner: clonePtr(g.Winner), Table: []TableCardView{}, Rounds: []RoundView{}, MyHand: []HandSlotView{},
	}
	for s := range 4 {
		v.Players[s] = PlayerInfo{Seat: Seat(s), ID: g.Players[s].ID, Name: g.Players[s].Name}
	}
	h := g.Hand
	if h == nil {
		return v
	}
	v.HandKind = h.Kind
	v.Stake = h.Stake
	v.LastRaiser = clonePtr(h.LastRaiser)
	v.Pending = clonePtr(h.Pending)
	v.VotingTeam = clonePtr(h.VotingTeam)
	v.DeckCount = len(h.Deck)
	v.DiscardCount = len(h.Discarded)
	if g.Phase == PhasePlaying {
		v.Turn = seatPtr(h.Turn)
	}
	if h.Vira != nil {
		viraID, manilha := h.Vira.ID(), h.Manilha
		v.Vira, v.Manilha = &viraID, &manilha
	}
	for s := range 4 {
		v.Players[s].CardCount = remainingCards(h, Seat(s))
		v.Players[s].Voted = h.Votes[s] != nil
	}
	for _, played := range h.Table {
		v.Table = append(v.Table, tableCardFor(played, seat))
	}
	for _, round := range h.Rounds {
		rv := RoundView{Winner: clonePtr(round.Winner), WinnerSeat: clonePtr(round.WinnerSeat), Cards: []TableCardView{}}
		for _, played := range round.Cards {
			rv.Cards = append(rv.Cards, tableCardFor(played, seat))
		}
		v.Rounds = append(v.Rounds, rv)
	}
	v.MyHand = handSlotsFor(h, seat, h.Kind == HandIron)
	if chooser, choosing := currentChooser(g); choosing && chooser == seat {
		for keep, card := range h.CutOffer {
			v.CutOffer = append(v.CutOffer, OfferCardView{Keep: keep, Card: card.ID()})
		}
	}
	if teamIsVoting(g, seat) {
		v.PartnerHand = handSlotsFor(h, Partner(seat), false)
		v.PartnerVote = clonePtr(h.Votes[Partner(seat)])
	}
	return v
}

func remainingCards(h *HandState, s Seat) int {
	count := 0
	for slot := range h.Hands[s] {
		if !h.Played[s][slot] {
			count++
		}
	}
	return count
}

func handSlotsFor(h *HandState, s Seat, hidden bool) []HandSlotView {
	slots := []HandSlotView{}
	for slot, card := range h.Hands[s] {
		if h.Played[s][slot] {
			continue
		}
		view := HandSlotView{Slot: slot, Hidden: hidden}
		if !hidden {
			id := card.ID()
			view.Card = &id
		}
		slots = append(slots, view)
	}
	return slots
}

func tableCardFor(played PlayedCard, viewer Seat) TableCardView {
	view := TableCardView{Seat: played.Seat, Covered: played.Covered}
	if !played.Covered || played.Seat == viewer {
		id := played.Card.ID()
		view.Card = &id
	}
	return view
}

func currentChooser(g GameState) (Seat, bool) {
	switch g.Phase {
	case PhaseCutChoice:
		return g.Cutter(), true
	case PhaseDealerChoice:
		return g.Dealer, true
	}
	return 0, false
}

func teamIsVoting(g GameState, seat Seat) bool {
	h := g.Hand
	isVotingPhase := g.Phase == PhaseAwaitingRaiseResponse || g.Phase == PhaseElevenDecision
	return isVotingPhase && h.VotingTeam != nil && *h.VotingTeam == TeamOf(seat) && h.Kind != HandIron
}
```

- [ ] **Passo 4: Rodar e verificar que passa**

Run: `make test`
Expected: PASS

- [ ] **Passo 5: Commit**

```bash
git add backend/internal/engine
git commit -m "feat(engine): adiciona projeção anti-cheat por assento"
```

---

### Task 9: Partidas aleatórias com invariantes e teste de vazamento

**Arquivos:**
- Teste: `backend/internal/engine/random_test.go`

**Interfaces:**
- Consome: `Apply`, `ViewFor`, `FilterEvents`, `allCards`, `testPlayers` e o restante do engine.
- Produz: apenas testes. O cálculo de cartas proibidas é **independente** de `view.go` (não reutiliza `teamIsVoting` nem `currentChooser`), para não validar a projeção com ela mesma.

- [ ] **Passo 1: Escrever o teste**

`backend/internal/engine/random_test.go`:

```go
package engine

import (
	"encoding/json"
	"errors"
	"math/rand/v2"
	"testing"
)

func TestRandomGamesKeepInvariantsAndNeverLeak(t *testing.T) {
	games := 2000
	if testing.Short() {
		games = 200
	}
	for seed := range uint64(games) {
		playRandomGame(t, seed)
	}
}

func playRandomGame(t *testing.T, seed uint64) {
	t.Helper()
	rng := rand.New(rand.NewPCG(seed, 99))
	g := NewGame(testPlayers, seed, Seat(seed%4))
	for step := 0; g.Phase != PhaseGameOver; step++ {
		if step > 5000 {
			t.Fatalf("seed %d: partida não terminou", seed)
		}
		prevScore := g.Score
		var evs []Event
		g, evs = applyRandomLegal(t, g, rng, seed)
		checkInvariants(t, g, prevScore, seed)
		for s := Seat(0); s < 4; s++ {
			assertNoLeak(t, g, s, ViewFor(g, s), seed)
			assertNoLeak(t, g, s, FilterEvents(evs, s), seed)
		}
	}
}

func applyRandomLegal(t *testing.T, g GameState, rng *rand.Rand, seed uint64) (GameState, []Event) {
	t.Helper()
	candidates := candidateActions(g)
	rng.Shuffle(len(candidates), func(i, j int) { candidates[i], candidates[j] = candidates[j], candidates[i] })
	for _, a := range candidates {
		next, evs, err := Apply(g, a)
		if err == nil {
			return next, evs
		}
		var re *RuleError
		if !errors.As(err, &re) {
			t.Fatalf("seed %d: erro que não é RuleError: %v", seed, err)
		}
	}
	t.Fatalf("seed %d: nenhuma ação legal na fase %s", seed, g.Phase)
	return g, nil
}

func candidateActions(g GameState) []Action {
	if g.Phase == PhaseWaiting || g.Phase == PhaseHandEnd {
		return []Action{{Kind: ActSysStartHand}}
	}
	var out []Action
	for s := Seat(0); s < 4; s++ {
		out = append(out,
			Action{Kind: ActChooseCard, Seat: s, Keep: 0},
			Action{Kind: ActChooseCard, Seat: s, Keep: 1},
			Action{Kind: ActPassBurn, Seat: s},
			Action{Kind: ActSysTimeout, Seat: s},
		)
		for _, choice := range []VoteChoice{VoteAccept, VoteRun, VoteRaise, VotePlay} {
			out = append(out, Action{Kind: ActVote, Seat: s, Choice: choice})
		}
		for slot := range 3 {
			out = append(out,
				Action{Kind: ActPlayCard, Seat: s, Slot: slot},
				Action{Kind: ActPlayCard, Seat: s, Slot: slot, Covered: true},
			)
		}
		for slot, card := range g.Hand.Hands[s] {
			if card.Rank == 4 {
				out = append(out, Action{Kind: ActBurn, Seat: s, Slots: []int{slot}})
			}
		}
	}
	// Truco só em mãos normais: a punição tem teste próprio e encerraria as partidas cedo demais.
	if g.Hand.Kind == HandNormal {
		out = append(out, Action{Kind: ActRequestRaise, Seat: g.Hand.Turn})
	}
	return out
}

func checkInvariants(t *testing.T, g GameState, prevScore [2]int, seed uint64) {
	t.Helper()
	for team := range 2 {
		if g.Score[team] < prevScore[team] || g.Score[team] > WinningScore {
			t.Fatalf("seed %d: placar inválido %v (antes %v)", seed, g.Score, prevScore)
		}
	}
	if g.Hand == nil {
		return
	}
	cards := allCards(g)
	seen := map[Card]bool{}
	for _, card := range cards {
		if seen[card] {
			t.Fatalf("seed %d: carta duplicada %+v", seed, card)
		}
		seen[card] = true
	}
	if len(cards) != DeckSize {
		t.Fatalf("seed %d: %d cartas físicas", seed, len(cards))
	}
}

// forbiddenCards lista o que o assento NÃO pode ver, sem usar a lógica de view.go.
func forbiddenCards(g GameState, viewer Seat) map[CardID]bool {
	forbidden := map[CardID]bool{}
	h := g.Hand
	if h == nil {
		return forbidden
	}
	add := func(card Card) { forbidden[card.ID()] = true }
	for _, card := range h.Deck {
		add(card)
	}
	for _, card := range h.Discarded {
		add(card)
	}
	viewerIsChoosing := (g.Phase == PhaseCutChoice && viewer == (g.Dealer+3)%4) ||
		(g.Phase == PhaseDealerChoice && viewer == g.Dealer)
	if !viewerIsChoosing {
		for _, card := range h.CutOffer {
			add(card)
		}
	}
	viewerTeamVoting := (g.Phase == PhaseAwaitingRaiseResponse || g.Phase == PhaseElevenDecision) &&
		h.VotingTeam != nil && *h.VotingTeam == TeamOf(viewer)
	for s := Seat(0); s < 4; s++ {
		for slot, card := range h.Hands[s] {
			if h.Played[s][slot] {
				continue // cartas jogadas são tratadas pela mesa
			}
			ownVisible := s == viewer && h.Kind != HandIron
			partnerVisible := s == Partner(viewer) && viewerTeamVoting
			if !ownVisible && !partnerVisible {
				add(card)
			}
		}
	}
	played := append([]PlayedCard{}, h.Table...)
	for _, round := range h.Rounds {
		played = append(played, round.Cards...)
	}
	for _, p := range played {
		if p.Covered && p.Seat != viewer {
			add(p.Card)
		}
	}
	return forbidden
}

// cardIDsInJSON coleta todo número sob as chaves "card" e "vira".
func cardIDsInJSON(t *testing.T, payload any) []CardID {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	var ids []CardID
	var walk func(key string, node any)
	walk = func(key string, node any) {
		switch n := node.(type) {
		case map[string]any:
			for k, child := range n {
				walk(k, child)
			}
		case []any:
			for _, child := range n {
				walk(key, child)
			}
		case float64:
			if key == "card" || key == "vira" {
				ids = append(ids, CardID(n))
			}
		}
	}
	walk("", doc)
	return ids
}

func assertNoLeak(t *testing.T, g GameState, viewer Seat, payload any, seed uint64) {
	t.Helper()
	forbidden := forbiddenCards(g, viewer)
	for _, id := range cardIDsInJSON(t, payload) {
		if forbidden[id] {
			t.Fatalf("seed %d, fase %s: assento %d recebeu carta proibida %+v", seed, g.Phase, viewer, CardFromID(id))
		}
	}
}
```

- [ ] **Passo 2: Rodar o teste**

Run: `cd backend && go test ./internal/engine/ -run TestRandomGames -v`
Expected: PASS. Se falhar, a mensagem traz seed, fase e assento: corrija o código de produção (não o teste) e reexecute.

- [ ] **Passo 3: Rodar a suíte inteira com race detector e cobertura**

Run: `cd backend && go test -race -cover ./...`
Expected: PASS, cobertura de `internal/engine` ≥ 90%.

- [ ] **Passo 4: Commit**

```bash
git add backend/internal/engine
git commit -m "test(engine): adiciona partidas aleatórias com invariantes e detecção de vazamento"
```

---

### Task 10: Pacote de protocolo e geração de tipos TypeScript

**Arquivos:**
- Criar: `backend/internal/protocol/protocol.go`, `backend/tygo.yaml`
- Teste: `backend/internal/protocol/protocol_test.go`
- Gerados: `frontend/src/protocol/engine.ts`, `frontend/src/protocol/messages.ts`

**Interfaces:**
- Consome: `engine.Action`, `engine.ActionKind` e constantes, `engine.ViewFor`, `engine.FilterEvents`, `engine.RuleError`, `engine.VoteChoice`, `engine.Seat`, `engine.GameState`, `engine.Event`.
- Produz: `ClientMessage`, `ChooseCardPayload`, `BurnPayload`, `PlayCardPayload`, `VotePayload`, `ServerMessageType` (`MsgState`, `MsgEvents`, `MsgError`), `ServerMessage`, `ErrorPayload`, `ErrUnknownType`, `DecodeAction(ClientMessage, engine.Seat) (engine.Action, error)`, `StateMessage`, `EventsMessage`, `ErrorMessage`.

- [ ] **Passo 1: Escrever o teste que falha**

`backend/internal/protocol/protocol_test.go`:

```go
package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/caarlosandree/truco/backend/internal/engine"
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
	for _, typ := range []string{"sys_start_hand", "sys_timeout", "hack"} {
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
	if StateMessage(g, 0).Payload.(engine.PlayerView).CutOffer != nil {
		t.Fatal("assento 0 não corta nesta mão")
	}
	if len(EventsMessage(evs, 3).Payload.([]engine.Event)) != 2 || len(EventsMessage(evs, 0).Payload.([]engine.Event)) != 1 {
		t.Fatal("CutOffered só para quem corta (assento 3)")
	}
}
```

- [ ] **Passo 2: Rodar e verificar que falha**

Run: `make test`
Expected: FAIL (`undefined: ClientMessage`...).

- [ ] **Passo 3: Implementar**

`backend/internal/protocol/protocol.go`:

```go
// Package protocol define as mensagens WebSocket trocadas com o frontend.
package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/caarlosandree/truco/backend/internal/engine"
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
		if err := decodePayload(msg.Payload, &p); err != nil {
			return engine.Action{}, err
		}
		action.Keep = p.Keep
	case engine.ActBurn:
		var p BurnPayload
		if err := decodePayload(msg.Payload, &p); err != nil {
			return engine.Action{}, err
		}
		action.Slots = p.Slots
	case engine.ActPlayCard:
		var p PlayCardPayload
		if err := decodePayload(msg.Payload, &p); err != nil {
			return engine.Action{}, err
		}
		action.Slot, action.Covered = p.Slot, p.Covered
	case engine.ActVote:
		var p VotePayload
		if err := decodePayload(msg.Payload, &p); err != nil {
			return engine.Action{}, err
		}
		action.Choice = p.Choice
	case engine.ActPassBurn, engine.ActRequestRaise:
	default:
		return engine.Action{}, fmt.Errorf("%w: %q", ErrUnknownType, msg.Type)
	}
	return action, nil
}

func decodePayload(raw json.RawMessage, target any) error {
	if len(raw) == 0 {
		return errors.New("protocol: payload ausente")
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
```

- [ ] **Passo 4: Rodar e verificar que passa**

Run: `make test`
Expected: PASS

- [ ] **Passo 5: Configurar e gerar os tipos TypeScript**

`backend/tygo.yaml`:

```yaml
packages:
  - path: "github.com/caarlosandree/truco/backend/internal/engine"
    output_path: "../frontend/src/protocol/engine.ts"
  - path: "github.com/caarlosandree/truco/backend/internal/protocol"
    output_path: "../frontend/src/protocol/messages.ts"
    frontmatter: |
      import * as engine from "./engine";
    type_mappings:
      json.RawMessage: "unknown"
```

Run: `mkdir -p frontend/src/protocol && make gen-types && grep -c "export" frontend/src/protocol/engine.ts frontend/src/protocol/messages.ts && grep -n "interface PlayerView\|interface ClientMessage" frontend/src/protocol/*.ts`
Expected: os dois arquivos existem, com `export interface PlayerView` em `engine.ts` e `export interface ClientMessage` em `messages.ts`. Se o tygo referenciar tipos do engine sem o prefixo `engine.` em `messages.ts`, ajuste o `frontmatter` para `import type { PlayerView, Event, VoteChoice } from "./engine";` e gere de novo.

- [ ] **Passo 6: Rodar a suíte completa**

Run: `make test`
Expected: PASS

- [ ] **Passo 7: Commit**

```bash
git add backend frontend/src/protocol
git commit -m "feat(protocol): adiciona mensagens WebSocket e geração de tipos TypeScript"
```
