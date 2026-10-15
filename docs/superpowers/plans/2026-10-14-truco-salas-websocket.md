# Spec 3 — Salas, Lobby, WebSocket, Timers e Reconexão: Plano de Implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Servidor de jogo em tempo real: lobby com salas públicas e privadas, ator por sala, protocolo WebSocket tipado, timers por fase, ausência e W.O., revanche, sinais rápidos, recuperação após restart e desligamento gracioso — mais as 7 pendências herdadas da Spec 2.

**Architecture:** Cada sala é um ator (`room.Room` dirigido por `room.Actor`, caixa de entrada sem limite) e é o único escritor da sua partida no banco. O lobby (`lobby.Lobby`) é um registro protegido por mutex que indexa salas, códigos, usuários e conexões, e entrega mensagens às salas por `Actor.Push` (nunca bloqueia). `transport/ws` autentica o upgrade, mantém uma conexão por usuário e só fala com o lobby. O tempo vem de `clock.Clock` (manual nos testes).

**Tech Stack:** Go 1.27 (stdlib `net/http`, `log/slog`), `github.com/coder/websocket` v1.8.15, `golang.org/x/time/rate`, PostgreSQL 18 via `pgx/v5` + `sqlc`, `testcontainers-go`, `tygo` v0.2.21.

**Spec:** `docs/superpowers/specs/2026-10-14-truco-salas-websocket-design.md` (leia junto com este plano).

## Global Constraints

- Comentários, mensagens de erro, logs e textos ao usuário em **pt-BR**; identificadores em inglês, seguindo o código existente.
- Commits convencionais em pt-BR, **sem** linhas de atribuição (`Co-Authored-By` etc.).
- Rode Go a partir de `backend/`. Testes de integração neste host: `TESTCONTAINERS_RYUK_DISABLED=true TRUCO_REQUIRE_DB=1 go test ./...` (o Ryuk não sobe aqui).
- **Nunca** edite `backend/internal/db/dbgen/*` nem `frontend/src/protocol/*.ts` à mão: use `make gen-sql` / `make gen-types` (rodar da raiz do repositório).
- Nunca mexa no container `postgres` de outro projeto na porta 5432; o banco de dev do projeto é o compose na porta 5433 (`truco-postgres-1`). Os testes usam testcontainers e não precisam dele.
- `internal/engine` continua importando **apenas a stdlib**; `engine.RulesVersion` continua **1**.
- `internal/room` não importa `net/http`, `coder/websocket`, `match` nem `db`.
- Só `internal/transport/ws` importa `github.com/coder/websocket` (além de testes).
- Nada de push, merge ou ação fora do repositório sem confirmação do usuário.
- Ao terminar cada tarefa: `go vet ./...` e `go test ./...` verdes no pacote tocado (com as variáveis de integração quando o pacote usa banco).

## Review Focus

Cinco entradas que a spec implica mas nenhuma regra explícita cobre; cada uma tem teste na tarefa dona:

1. **Duas abas do mesmo usuário:** a aba nova substitui a antiga (fechada com 4001) e mensagens que ainda cheguem pela conexão antiga são ignoradas — teste `TestStaleConnectionIsIgnored` (Tarefa 14).
2. **Código de sala digitado em minúsculas ou com espaços:** `join_room{code:" k7qm2p "}` entra na sala — teste `TestJoinByCodeIsCaseAndSpaceInsensitive` (Tarefa 14).
3. **Jogada fora da vez:** devolve o código de regra com o `seq` do cliente, não grava nada e não muda o estado — teste `TestRuleErrorKeepsStateAndLogNothing` (Tarefa 10).
4. **Anfitrião sai da sala em espera:** o anfitrião passa ao próximo ocupante; a última saída destrói a sala e a remove da lista — testes `TestHostLeavingPassesHostToNextOccupant` (Tarefa 9) e `TestDestroyedRoomLeavesTheList` (Tarefa 14).
5. **Sessão revogada com o WebSocket aberto (logout-all / conta desativada):** a conexão fecha com 4003 na revalidação — teste `TestRevalidationClosesRevokedSession` (Tarefa 15) e `TestWebSocketLogoutAllClosesConnection` (Tarefa 17).

---

## Mapa de arquivos

| Arquivo | Responsabilidade | Tarefa |
|---|---|---|
| `backend/internal/clock/clock.go` (+ `_test`) | `Clock`, `Timer`, `Real`, `Manual` | 1 |
| `backend/internal/testclock/` | **removido** (substituído por `clock.Manual`) | 1 |
| `backend/internal/engine/{actions,events,apply}.go`, `forfeit.go`, `gamestate.go`, `state.go` | `SysForfeit`; `GameState`/`HandState` movidos para `gamestate.go` | 2 |
| `backend/internal/db/queries/matches.sql`, `backend/internal/match/store.go` | `AppendAction` transacional com `FOR SHARE` e `ErrPlyGap` | 3 |
| `backend/internal/match/{store,recover}.go` | ordem de lock em `writeStats`, `LOCK TABLE` no recompute, `Recover` resiliente | 4 |
| `backend/internal/auth/service.go` | `Service.Wait` (e-mails em voo) | 5 |
| `backend/internal/config/config.go` | `GameTiming`, `MaxRooms`, `ShutdownTimeout` | 6 |
| `backend/internal/protocol/{envelope,client,server,errors,registry}.go` | protocolo redefinido e tipado | 7 |
| `backend/internal/protocol/union.go`, `backend/cmd/gen-protocol/main.go`, `backend/tygo.yaml`, `Makefile`, `frontend/src/protocol/*.ts` (gerados) | uniões TS discriminadas | 8 |
| `backend/internal/room/{ports,msgs,room,seats}.go` | sala em espera | 9 |
| `backend/internal/room/{playing,window}.go` | início, jogadas, persistência, fim, abort, restore | 10 |
| `backend/internal/room/timers.go` | `settle`, prazos, pausa entre mãos, agendamento | 11 |
| `backend/internal/room/presence.go` | ausência, W.O., todos ausentes, remoção em espera | 12 |
| `backend/internal/room/{signals,actor}.go` | sinais rápidos, ator; teste de propriedade | 13 |
| `backend/internal/lobby/{lobby,store}.go` | registro, lista ao vivo, restore, abort, shutdown, adaptador do store | 14 |
| `backend/internal/transport/ws/{handler,client}.go` | `/ws`: autenticação, conexão, limites | 15 |
| `backend/internal/httpapi/server.go`, `backend/cmd/server/main.go`, `README.md`, `.env.example` | montagem, boot, shutdown, documentação | 16 |
| `backend/internal/httpapi/ws_e2e_test.go` | testes ponta a ponta com Postgres | 17 |

**Modelos sugeridos para a execução (subagent-driven):** haiku nas Tarefas 5 e 6 (transcrição direta); sonnet nas demais e nos revisores; revisão final da branch no modelo mais capaz.

---

### Task 1: Pacote `clock` (substitui `testclock`)

**Files:**
- Create: `backend/internal/clock/clock.go`
- Test: `backend/internal/clock/clock_test.go`
- Modify: todos os arquivos que importam `internal/testclock` (`internal/ratelimit/ratelimit_test.go`, `internal/admin/admin_test.go`, `internal/stats/reader_test.go`, `internal/httpapi/harness_test.go`, `internal/auth/main_test.go`, `internal/match/store_test.go`)
- Delete: `backend/internal/testclock/clock.go`

**Interfaces:**
- Produces:
  - `type Timer interface{ Stop() bool }`
  - `type Clock interface{ Now() time.Time; AfterFunc(d time.Duration, f func()) Timer }`
  - `type Real struct{}` (implementa `Clock` com `time`)
  - `func NewManual(t time.Time) *Manual`; `(*Manual).Now()`, `(*Manual).AfterFunc(d, f) Timer`, `(*Manual).Advance(d)`, `(*Manual).Pending() int`
  - Contrato do `Manual.Advance`: dispara, em ordem de prazo (empate: ordem de criação), cada timer com prazo ≤ alvo; durante o callback `Now()` é o prazo daquele timer; callbacks rodam sem lock e podem armar novos timers (que disparam no mesmo `Advance` se vencerem até o alvo).

- [ ] **Step 1: Escrever os testes que falham**

`backend/internal/clock/clock_test.go`:

```go
package clock_test

import (
	"slices"
	"testing"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/clock"
)

var t0 = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

func TestManualFiresInDeadlineOrder(t *testing.T) {
	c := clock.NewManual(t0)
	var got []string
	c.AfterFunc(3*time.Second, func() { got = append(got, "c") })
	c.AfterFunc(1*time.Second, func() { got = append(got, "a") })
	c.AfterFunc(2*time.Second, func() { got = append(got, "b") })
	c.Advance(2 * time.Second)
	if !slices.Equal(got, []string{"a", "b"}) {
		t.Fatalf("disparos = %v", got)
	}
	if !c.Now().Equal(t0.Add(2 * time.Second)) {
		t.Fatalf("Now = %v", c.Now())
	}
	c.Advance(time.Second)
	if !slices.Equal(got, []string{"a", "b", "c"}) {
		t.Fatalf("disparos = %v", got)
	}
	if c.Pending() != 0 {
		t.Fatalf("Pending = %d", c.Pending())
	}
}

func TestManualStopPreventsFiring(t *testing.T) {
	c := clock.NewManual(t0)
	fired := false
	timer := c.AfterFunc(time.Second, func() { fired = true })
	if !timer.Stop() {
		t.Fatal("Stop de timer armado deve devolver true")
	}
	if timer.Stop() {
		t.Fatal("segundo Stop deve devolver false")
	}
	c.Advance(time.Minute)
	if fired {
		t.Fatal("timer parado disparou")
	}
}

func TestManualCallbackSeesItsInstantAndCanArmMore(t *testing.T) {
	c := clock.NewManual(t0)
	var instants []time.Time
	c.AfterFunc(time.Second, func() {
		instants = append(instants, c.Now())
		c.AfterFunc(time.Second, func() { instants = append(instants, c.Now()) })
	})
	c.Advance(3 * time.Second)
	want := []time.Time{t0.Add(time.Second), t0.Add(2 * time.Second)}
	if !slices.EqualFunc(instants, want, time.Time.Equal) {
		t.Fatalf("instantes = %v, quer %v", instants, want)
	}
	if !c.Now().Equal(t0.Add(3 * time.Second)) {
		t.Fatalf("Now = %v", c.Now())
	}
}

func TestManualZeroDurationFiresOnAdvanceZero(t *testing.T) {
	c := clock.NewManual(t0)
	fired := false
	c.AfterFunc(0, func() { fired = true })
	c.Advance(0)
	if !fired {
		t.Fatal("timer de duração zero deve disparar em Advance(0)")
	}
}

func TestManualSameInstantKeepsCreationOrder(t *testing.T) {
	c := clock.NewManual(t0)
	var got []int
	for i := range 3 {
		c.AfterFunc(time.Second, func() { got = append(got, i) })
	}
	c.Advance(time.Second)
	if !slices.Equal(got, []int{0, 1, 2}) {
		t.Fatalf("ordem = %v", got)
	}
}

func TestRealAfterFuncFires(t *testing.T) {
	done := make(chan struct{})
	clock.Real{}.AfterFunc(time.Millisecond, func() { close(done) })
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Real.AfterFunc não disparou")
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/clock/`
Expected: FAIL — `package .../internal/clock` sem arquivos Go / `undefined: clock.NewManual`.

- [ ] **Step 3: Implementar**

`backend/internal/clock/clock.go`:

```go
// Package clock abstrai o tempo para que timers e expirações sejam determinísticos nos testes.
package clock

import (
	"sync"
	"time"
)

// Timer é um disparo agendado que pode ser cancelado.
type Timer interface {
	Stop() bool
}

// Clock fornece o instante atual e agenda funções.
type Clock interface {
	Now() time.Time
	AfterFunc(d time.Duration, f func()) Timer
}

// Real usa o relógio do sistema.
type Real struct{}

func (Real) Now() time.Time { return time.Now() }

func (Real) AfterFunc(d time.Duration, f func()) Timer { return time.AfterFunc(d, f) }

// Manual é um relógio controlado pelo teste, seguro para uso concorrente.
type Manual struct {
	mu     sync.Mutex
	now    time.Time
	seq    uint64
	timers []*manualTimer
}

type manualTimer struct {
	clock *Manual
	at    time.Time
	seq   uint64
	f     func()
	done  bool // disparado ou parado; protegido por clock.mu
}

// NewManual cria um relógio começando em t.
func NewManual(t time.Time) *Manual { return &Manual{now: t} }

func (c *Manual) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *Manual) AfterFunc(d time.Duration, f func()) Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.seq++
	t := &manualTimer{clock: c, at: c.now.Add(d), seq: c.seq, f: f}
	c.timers = append(c.timers, t)
	return t
}

func (t *manualTimer) Stop() bool {
	c := t.clock
	c.mu.Lock()
	defer c.mu.Unlock()
	if t.done {
		return false
	}
	t.done = true
	c.remove(t)
	return true
}

// Advance avança o relógio em d e dispara, em ordem de prazo, os timers vencidos.
// Os callbacks rodam na goroutine de quem chamou, sem o lock, e podem armar novos timers.
func (c *Manual) Advance(d time.Duration) {
	c.mu.Lock()
	target := c.now.Add(d)
	c.mu.Unlock()
	for {
		c.mu.Lock()
		next := c.earliestDue(target)
		if next == nil {
			c.now = target
			c.mu.Unlock()
			return
		}
		next.done = true
		c.remove(next)
		if next.at.After(c.now) {
			c.now = next.at
		}
		c.mu.Unlock()
		next.f()
	}
}

// Pending conta os timers armados e ainda não disparados.
func (c *Manual) Pending() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.timers)
}

func (c *Manual) earliestDue(target time.Time) *manualTimer {
	var best *manualTimer
	for _, t := range c.timers {
		if t.at.After(target) {
			continue
		}
		if best == nil || t.at.Before(best.at) || (t.at.Equal(best.at) && t.seq < best.seq) {
			best = t
		}
	}
	return best
}

func (c *Manual) remove(t *manualTimer) {
	for i, other := range c.timers {
		if other == t {
			c.timers = append(c.timers[:i], c.timers[i+1:]...)
			return
		}
	}
}
```

- [ ] **Step 4: Rodar e ver passar**

Run: `cd backend && go test ./internal/clock/`
Expected: PASS.

- [ ] **Step 5: Migrar os usos de `testclock` e remover o pacote**

O `Manual` tem os mesmos métodos `Now()` e `Advance(d)` do `testclock.Clock`. Troque mecanicamente:

```bash
cd backend
grep -rl 'internal/testclock' --include='*.go' . | xargs sed -i \
  -e 's#"github.com/caarlosandree/truco-multiplayer/backend/internal/testclock"#"github.com/caarlosandree/truco-multiplayer/backend/internal/clock"#' \
  -e 's/testclock\.New(/clock.NewManual(/g' \
  -e 's/\*testclock\.Clock/*clock.Manual/g'
git rm -q internal/testclock/clock.go
gofmt -l ./internal
```

Se algum arquivo já tiver uma variável local chamada `clock` (ex.: `clock := clock.NewManual(t0)` em `ratelimit_test.go`), o nome do pacote fica sombreado na mesma função: renomeie a variável local para `clk` nesse arquivo (todas as ocorrências dentro da função). Rode `go vet ./...` para achar os casos: o erro é `clock.NewManual undefined (type *clock.Manual has no field or method NewManual)`.

- [ ] **Step 6: Rodar tudo**

Run: `cd backend && go vet ./... && TESTCONTAINERS_RYUK_DISABLED=true TRUCO_REQUIRE_DB=1 go test ./...`
Expected: PASS; `grep -r testclock backend` não encontra nada.

- [ ] **Step 7: Commit**

```bash
git add -A backend/internal
git commit -m "feat(clock): relógio injetável com timers e Manual para testes, substituindo testclock"
```

---

### Task 2: `SysForfeit` no engine e separação do estado privado

**Files:**
- Modify: `backend/internal/engine/actions.go`, `backend/internal/engine/events.go`, `backend/internal/engine/apply.go`, `backend/internal/engine/state.go`
- Create: `backend/internal/engine/forfeit.go`, `backend/internal/engine/gamestate.go`
- Test: `backend/internal/engine/forfeit_test.go`, `backend/internal/stats/forfeit_test.go`

**Interfaces:**
- Produces:
  - `engine.ActSysForfeit ActionKind = "sys_forfeit"`; campo `Action.Team Team` (`json:"team,omitempty"`; a dupla 0 é o valor zero e some do JSON, o que é inofensivo porque decodifica de volta como 0).
  - `engine.EvForfeited EventKind = "forfeited"` (visível a todos, `Team` = dupla que perdeu).
  - `GameState`, `HandState`, `RoundResult`, `PlayedCard` (+ `Cutter`, `Clone`, `clone`, `MarshalJSON`) passam a viver em `gamestate.go` (a Tarefa 8 exclui esse arquivo do tygo).

- [ ] **Step 1: Escrever os testes que falham**

`backend/internal/engine/forfeit_test.go`:

```go
package engine

import "testing"

func TestSysForfeitEndsGameInAnyPhase(t *testing.T) {
	waiting := newTestGame(1)
	cut, _ := mustApply(t, waiting, Action{Kind: ActSysStartHand})
	playing := dealWithoutBurning(t, cut)
	if playing.Phase != PhasePlaying {
		t.Fatalf("fase inesperada %s", playing.Phase)
	}
	for _, before := range []GameState{waiting, cut, playing} {
		for _, loser := range []Team{0, 1} {
			next, evs := mustApply(t, before, Action{Kind: ActSysForfeit, Team: loser})
			winner := Other(loser)
			if next.Phase != PhaseGameOver || next.Winner == nil || *next.Winner != winner {
				t.Fatalf("fase %s: estado final %s vencedor %v", before.Phase, next.Phase, next.Winner)
			}
			if next.Score[winner] != WinningScore || next.Score[loser] != before.Score[loser] {
				t.Fatalf("fase %s: placar %v", before.Phase, next.Score)
			}
			if len(evs) != 2 || evs[0].Kind != EvForfeited || *evs[0].Team != loser || evs[0].Visibility.Kind != VisAll || evs[1].Kind != EvGameOver {
				t.Fatalf("fase %s: eventos %+v", before.Phase, evs)
			}
			_ = ViewFor(next, 0) // não pode entrar em pânico, nem sem mão
		}
	}
}

func TestSysForfeitRejectsGameOverAndInvalidTeam(t *testing.T) {
	g := newTestGame(1)
	_, _, err := Apply(g, Action{Kind: ActSysForfeit, Team: 2})
	wantCode(t, err, ErrInvalidChoice)
	over, _ := mustApply(t, g, Action{Kind: ActSysForfeit, Team: 0})
	_, _, err = Apply(over, Action{Kind: ActSysForfeit, Team: 1})
	wantCode(t, err, ErrGameOver)
}
```

`backend/internal/stats/forfeit_test.go`:

```go
package stats_test

import (
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/enginetest"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/stats"
)

func TestForfeitedMatchRebuildsAndDerives(t *testing.T) {
	rec, _ := enginetest.RandomRecord(5)
	rec.Actions = rec.Actions[:20]
	rec.Actions = append(rec.Actions, replay.LoggedAction{Ply: 20,
		Action: engine.Action{Kind: engine.ActSysForfeit, Team: 1}, Source: replay.SourceSystem})
	final, err := replay.Rebuild(rec)
	if err != nil {
		t.Fatal(err)
	}
	if final.Phase != engine.PhaseGameOver || *final.Winner != 0 {
		t.Fatalf("fase %s vencedor %v", final.Phase, final.Winner)
	}
	perSeat, err := stats.Derive(rec)
	if err != nil {
		t.Fatal(err)
	}
	for seat, p := range perSeat {
		if p.Won != (engine.TeamOf(engine.Seat(seat)) == 0) || p.PointsFor+p.PointsAgainst == 0 {
			t.Fatalf("assento %d: %+v", seat, p)
		}
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/engine/ ./internal/stats/ -run 'Forfeit'`
Expected: FAIL — `undefined: ActSysForfeit`, `unknown field Team`.

- [ ] **Step 3: Implementar a ação e o evento**

Em `backend/internal/engine/actions.go`, adicione a constante depois de `ActSysTimeout` e o campo `Team` ao fim de `Action`:

```go
	ActSysTimeout   ActionKind = "sys_timeout"
	// ActSysForfeit encerra a partida por W.O.; Team é a dupla que perde. Só a sala envia.
	ActSysForfeit ActionKind = "sys_forfeit"
```

```go
	Choice  VoteChoice `json:"choice,omitempty"`
	Team    Team       `json:"team,omitempty"`
```

Em `backend/internal/engine/events.go`, depois de `EvGameOver`:

```go
	EvGameOver       EventKind = "game_over"
	EvForfeited      EventKind = "forfeited"
```

Em `backend/internal/engine/apply.go`, no `switch a.Kind` de `dispatch`, antes de `case ActChooseCard:`:

```go
	case ActSysForfeit:
		return forfeit(g, a)
```

Crie `backend/internal/engine/forfeit.go`:

```go
package engine

// forfeit encerra a partida por W.O.: a dupla a.Team perde e a adversária vai a 12.
// A mão corrente fica como está; só a sala envia esta ação (nunca o cliente).
func forfeit(g *GameState, a Action) ([]Event, error) {
	if a.Team > 1 {
		return nil, ruleErr(ErrInvalidChoice, "dupla inválida")
	}
	winner := Other(a.Team)
	g.Score[winner] = WinningScore
	evs := []Event{{Kind: EvForfeited, Visibility: visAll, Team: teamPtr(a.Team)}}
	return append(evs, finishGame(g, winner)...), nil
}
```

- [ ] **Step 4: Mover o estado privado para `gamestate.go`**

Recorte de `backend/internal/engine/state.go` e cole em `backend/internal/engine/gamestate.go` (novo arquivo, `package engine`, importando `errors` e `slices`), sem alterar o código: os tipos `PlayedCard`, `GameState`, `HandState`, `RoundResult`, os métodos `Cutter`, `Clone`, `(HandState) clone`, a variável `errStateNotSerializable` e o método `MarshalJSON`. Abra o arquivo com o comentário:

```go
package engine

import (
	"errors"
	"slices"
)

// Este arquivo guarda o estado completo e privado do servidor. Fica fora do tygo
// (tygo.yaml, exclude_files): o cliente só conhece PlayerView.
```

Depois remova de `state.go` os imports que ficarem sem uso (`errors`, `slices`). `state.go` continua com `Seat`, `Team`, `TeamOf`, `Partner`, `Next`, `Other`, `teamPtr`, `seatPtr`, `clonePtr`, `Phase`, `HandKind`, `VoteChoice`, `WinningScore`, `Player` e `PendingRaise`.

- [ ] **Step 5: Rodar e ver passar**

Run: `cd backend && gofmt -l internal/engine && go vet ./internal/engine/ && go test ./internal/engine/ ./internal/stats/ ./internal/replay/`
Expected: PASS (inclusive os testes de vazamento e de partidas aleatórias já existentes).

- [ ] **Step 6: Commit**

```bash
git add backend/internal/engine backend/internal/stats/forfeit_test.go
git commit -m "feat(engine): ação sys_forfeit para W.O. e estado privado isolado em gamestate.go"
```

---

### Task 3: `AppendAction` transacional (pendência 3)

**Files:**
- Modify: `backend/internal/db/queries/matches.sql`, `backend/internal/match/store.go`
- Regenerate: `backend/internal/db/dbgen/*` (via `make gen-sql`)
- Test: `backend/internal/match/store_test.go`

**Interfaces:**
- Produces: `match.ErrPlyGap`; `(*Store).AppendAction` com a mesma assinatura, agora em transação: `SELECT … FOR SHARE` da partida → `ErrNotFound` / `ErrMatchClosed`; ply menor que o próximo → `ErrPlyConflict`; maior → `ErrPlyGap`.

- [ ] **Step 1: Escrever os testes que falham**

Acrescente a `backend/internal/match/store_test.go`:

```go
func TestAppendActionRejectsGapsAndRepeats(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	id, _ := e.persist(t, 3, 3)
	full, _ := enginetest.RandomRecord(3)
	if err := e.store.AppendAction(ctx, id, 5, full.Actions[5].Action, full.Actions[5].Source); !errors.Is(err, match.ErrPlyGap) {
		t.Fatalf("ply 5 com próximo 3: %v", err)
	}
	if err := e.store.AppendAction(ctx, id, 2, full.Actions[2].Action, full.Actions[2].Source); !errors.Is(err, match.ErrPlyConflict) {
		t.Fatalf("ply 2 repetido: %v", err)
	}
	if err := e.store.AppendAction(ctx, id, 3, full.Actions[3].Action, full.Actions[3].Source); err != nil {
		t.Fatalf("ply 3: %v", err)
	}
}

func TestAppendActionWaitsForConcurrentAbandon(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	id, _ := e.persist(t, 3, 1)
	full, _ := enginetest.RandomRecord(3)
	tx, err := e.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT 1 FROM matches WHERE id = $1 FOR UPDATE", id); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- e.store.AppendAction(ctx, id, 1, full.Actions[1].Action, full.Actions[1].Source) }()
	select {
	case err := <-done:
		t.Fatalf("AppendAction não esperou o encerramento concorrente: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
	if _, err := tx.Exec(ctx, "UPDATE matches SET status = 'abandoned', finished_at = now(), abandon_reason = 'teste' WHERE id = $1", id); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, match.ErrMatchClosed) {
		t.Fatalf("ação depois de abandono confirmado: %v", err)
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && TESTCONTAINERS_RYUK_DISABLED=true TRUCO_REQUIRE_DB=1 go test ./internal/match/ -run 'AppendAction'`
Expected: FAIL — `undefined: match.ErrPlyGap`; depois de definir a variável, `TestAppendActionWaitsForConcurrentAbandon` falha porque o `INSERT … WHERE EXISTS` atual não espera o lock.

- [ ] **Step 3: Trocar a query**

Em `backend/internal/db/queries/matches.sql`, substitua o bloco `-- name: AppendMatchAction :execrows` (as 4 linhas) por:

```sql
-- name: LockMatchStatus :one
-- FOR SHARE serializa com Abandon (UPDATE) e Finish (FOR UPDATE).
SELECT status FROM matches WHERE id = @id FOR SHARE;

-- name: NextPly :one
SELECT coalesce(max(ply) + 1, 0)::integer FROM match_actions WHERE match_id = @match_id;

-- name: InsertMatchAction :exec
INSERT INTO match_actions (match_id, ply, action, source, created_at)
VALUES (@match_id::uuid, @ply::integer, @action::jsonb, @source::text, @now::timestamptz);
```

Run (da raiz): `make gen-sql`
Expected: `dbgen` regenerado com `LockMatchStatus(ctx, id string) (string, error)`, `NextPly(ctx, matchID string) (int32, error)` e `InsertMatchAction(ctx, InsertMatchActionParams)`; `AppendMatchAction` some. Confira com `grep -n "func (q \*Queries) \(LockMatchStatus\|NextPly\|InsertMatchAction\)" backend/internal/db/dbgen/*.go`.

- [ ] **Step 4: Reescrever `AppendAction`**

Em `backend/internal/match/store.go`, acrescente a `var (...)`:

```go
	ErrPlyGap          = errors.New("match: ply fora de sequência")
```

e substitua `AppendAction` inteira por:

```go
// AppendAction grava uma ação aplicada com sucesso. A sala chama antes de enviar o estado aos jogadores.
// Contrato: a sala é o único escritor das ações da sua partida. Mesmo assim a gravação trava a partida
// em modo compartilhado e exige o ply seguinte, para que nada entre depois de um encerramento confirmado.
func (s *Store) AppendAction(ctx context.Context, id string, ply int, a engine.Action, src replay.Source) error {
	if !src.Valid() {
		return fmt.Errorf("match: origem inválida %q", src)
	}
	if a.Kind == engine.ActSysTimeout {
		return errors.New("match: grave a ação concreta de DefaultAction, não sys_timeout")
	}
	raw, err := json.Marshal(a)
	if err != nil {
		return err
	}
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		status, err := q.LockMatchStatus(ctx, id)
		if err != nil {
			return notFound(err)
		}
		if status != string(replay.StatusInProgress) {
			return ErrMatchClosed
		}
		next, err := q.NextPly(ctx, id)
		if err != nil {
			return err
		}
		switch {
		case ply < int(next):
			return ErrPlyConflict
		case ply > int(next):
			return ErrPlyGap
		}
		return q.InsertMatchAction(ctx, dbgen.InsertMatchActionParams{MatchID: id, Ply: int32(ply), Action: raw, Source: string(src), Now: s.now()})
	})
}
```

Remova `closedOrMissing` se `go vet` / o compilador indicar que ficou sem uso (`grep -n closedOrMissing internal/match/*.go`).

- [ ] **Step 5: Rodar e ver passar**

Run: `cd backend && go vet ./... && TESTCONTAINERS_RYUK_DISABLED=true TRUCO_REQUIRE_DB=1 go test ./internal/match/ ./internal/httpapi/`
Expected: PASS (inclui os testes antigos de `ErrPlyConflict`/`ErrMatchClosed`).

- [ ] **Step 6: Commit**

```bash
git add backend/internal/db backend/internal/match
git commit -m "fix(match): AppendAction trava a partida em FOR SHARE e exige ply contíguo"
```

---

### Task 4: Estatísticas sem deadlock e `Recover` resiliente (pendências 1, 4 e 5)

**Files:**
- Modify: `backend/internal/match/store.go` (`writeStats`), `backend/internal/match/recover.go`
- Test: `backend/internal/match/store_test.go`

**Interfaces:**
- Produces: `(*Store).Recover(ctx, log *slog.Logger) ([]Recovered, error)` — mesma assinatura; agora só devolve erro se a **listagem** falhar; `log == nil` usa `slog.Default()`; falhas por partida são logadas e a partida é pulada (fica `in_progress`).

- [ ] **Step 1: Escrever os testes que falham**

Acrescente a `backend/internal/match/store_test.go` (adicione `"sync"` aos imports se faltar):

```go
// persistFull grava uma partida aleatória completa (até GameOver), sem finalizá-la.
func (e env) persistFull(t *testing.T, seed uint64, players [4]replay.PlayerSeat) string {
	t.Helper()
	ctx := context.Background()
	rec, _ := enginetest.RandomRecord(seed)
	id, err := e.store.Create(ctx, rec.Seed, rec.FirstDealer, players)
	if err != nil {
		t.Fatal(err)
	}
	for _, logged := range rec.Actions {
		if err := e.store.AppendAction(ctx, id, logged.Ply, logged.Action, logged.Source); err != nil {
			t.Fatalf("ply %d: %v", logged.Ply, err)
		}
	}
	return id
}

func TestConcurrentFinishWithSameUsersDoesNotDeadlock(t *testing.T) {
	e := newEnv(t)
	reversed := [4]replay.PlayerSeat{e.players[3], e.players[2], e.players[1], e.players[0]}
	for i := range 5 {
		a := e.persistFull(t, uint64(100+i), e.players)
		b := e.persistFull(t, uint64(200+i), reversed)
		start := make(chan struct{})
		errs := make([]error, 2)
		var wg sync.WaitGroup
		for j, id := range []string{a, b} {
			wg.Go(func() {
				<-start
				errs[j] = e.store.Finish(context.Background(), id)
			})
		}
		close(start)
		wg.Wait()
		for _, err := range errs {
			if err != nil {
				t.Fatalf("iteração %d: %v", i, err)
			}
		}
	}
}

func TestRecomputeStatsConcurrentWithFinishKeepsTotals(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	for i := range 3 {
		if err := e.store.Finish(ctx, e.persistFull(t, uint64(300+i), e.players)); err != nil {
			t.Fatal(err)
		}
	}
	pending := e.persistFull(t, 400, e.players)
	var wg sync.WaitGroup
	var finishErr, recomputeErr error
	wg.Go(func() { finishErr = e.store.Finish(ctx, pending) })
	wg.Go(func() { _, recomputeErr = e.store.RecomputeStats(ctx) })
	wg.Wait()
	if finishErr != nil || recomputeErr != nil {
		t.Fatalf("finish %v, recompute %v", finishErr, recomputeErr)
	}
	var mismatched, total int
	err := e.pool.QueryRow(ctx, `SELECT count(*) FROM player_stats ps
		WHERE ps.wins + ps.losses <> (SELECT count(*) FROM match_player_stats m WHERE m.user_id = ps.user_id)`).Scan(&mismatched)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.pool.QueryRow(ctx, `SELECT coalesce(sum(wins + losses), 0) FROM player_stats`).Scan(&total); err != nil {
		t.Fatal(err)
	}
	if mismatched != 0 || total != 16 {
		t.Fatalf("divergentes %d, total %d (quer 0 e 16)", mismatched, total)
	}
}

func TestRecoverSkipsTransientFailuresAndAcceptsNilLogger(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	over := e.persistFull(t, 11, e.players) // em GameOver, ainda in_progress
	alive, _ := e.persist(t, 12, 10)
	_, err := e.pool.Exec(ctx, `CREATE FUNCTION fail_stats() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN IF NEW.match_id = '`+over+`' THEN RAISE EXCEPTION 'falha simulada'; END IF; RETURN NEW; END $$`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.pool.Exec(ctx, `CREATE TRIGGER fail_stats BEFORE INSERT ON match_player_stats
		FOR EACH ROW EXECUTE FUNCTION fail_stats()`); err != nil {
		t.Fatal(err)
	}
	got, err := e.store.Recover(ctx, nil)
	if err != nil {
		t.Fatalf("Recover não deve falhar por uma partida: %v", err)
	}
	if len(got) != 1 || got[0].Record.ID != alive {
		t.Fatalf("recuperadas = %+v", got)
	}
	var status string
	if err := e.pool.QueryRow(ctx, "SELECT status FROM matches WHERE id = $1", over).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "in_progress" {
		t.Fatalf("partida com falha transitória deve ficar in_progress, está %s", status)
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && TESTCONTAINERS_RYUK_DISABLED=true TRUCO_REQUIRE_DB=1 go test ./internal/match/ -run 'Concurrent|RecoverSkips' -count=1`
Expected: FAIL — `TestRecoverSkips…` com `Recover não deve falhar por uma partida: … falha simulada`; os concorrentes podem falhar com `deadlock detected (SQLSTATE 40P01)` ou totais divergentes (são corridas: se passarem por sorte agora, tudo bem, eles protegem a correção).

- [ ] **Step 3: Ordenar os upserts e travar o recompute**

Em `backend/internal/match/store.go`, no início de `writeStats` (adicione `"slices"` e `"strings"` aos imports):

```go
func writeStats(ctx context.Context, q *dbgen.Queries, matchID string, perSeat [4]stats.PlayerMatchStats, now time.Time) error {
	// Ordem fixa por user_id: dois Finish com os mesmos jogadores travam as linhas na mesma ordem.
	ordered := perSeat
	slices.SortFunc(ordered[:], func(a, b stats.PlayerMatchStats) int { return strings.Compare(a.UserID, b.UserID) })
	for _, p := range ordered {
```

(o resto da função continua igual, iterando `ordered`).

Em `backend/internal/match/recover.go`, logo no início do callback de `RecomputeStats` (antes de `skipped = nil`):

```go
	err = db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		// Finish concorrente espera o recompute terminar; o recompute só lê matches, sem lock de linha.
		if _, err := tx.Exec(ctx, "LOCK TABLE match_player_stats, player_stats IN EXCLUSIVE MODE"); err != nil {
			return err
		}
		skipped = nil
```

- [ ] **Step 4: Tornar `Recover` resiliente**

Substitua `Recover` e `abandonUnrecoverable` em `backend/internal/match/recover.go` por:

```go
// Recover reconstrói as partidas em andamento após um restart. Partidas irrecuperáveis são
// abandonadas, as que já tinham terminado são finalizadas e falhas transitórias ficam para o
// próximo boot. Só uma falha ao listar as partidas devolve erro: o servidor sobe mesmo assim.
func (s *Store) Recover(ctx context.Context, log *slog.Logger) ([]Recovered, error) {
	if log == nil {
		log = slog.Default()
	}
	ids, err := s.q.ListInProgressMatchIDs(ctx)
	if err != nil {
		return nil, err
	}
	var out []Recovered
	for _, id := range ids {
		if r, ok := s.recoverOne(ctx, log, id); ok {
			out = append(out, r)
		}
	}
	return out, nil
}

// recoverOne devolve ok=false quando a partida não deve ser retomada agora.
func (s *Store) recoverOne(ctx context.Context, log *slog.Logger, id string) (Recovered, bool) {
	rec, err := s.Load(ctx, id)
	var state engine.GameState
	if err == nil {
		state, err = replay.Rebuild(rec)
	}
	var corrupt *replay.CorruptLogError
	switch {
	case errors.Is(err, replay.ErrIncompatibleVersion):
		s.abandonUnrecoverable(ctx, log, id, "incompatible_version", err)
		return Recovered{}, false
	case errors.As(err, &corrupt):
		s.abandonUnrecoverable(ctx, log, id, "corrupt_log", err)
		return Recovered{}, false
	case err != nil:
		log.ErrorContext(ctx, "falha ao recuperar partida; nova tentativa no próximo boot", "match_id", id, "err", err)
		return Recovered{}, false
	}
	if state.Phase == engine.PhaseGameOver {
		if err := s.Finish(ctx, id); err != nil {
			log.ErrorContext(ctx, "falha ao finalizar partida recuperada; nova tentativa no próximo boot", "match_id", id, "err", err)
		}
		return Recovered{}, false
	}
	return Recovered{Record: rec, State: state, NextPly: len(rec.Actions)}, true
}

func (s *Store) abandonUnrecoverable(ctx context.Context, log *slog.Logger, id, reason string, cause error) {
	log.ErrorContext(ctx, "partida irrecuperável abandonada", "match_id", id, "reason", reason, "err", cause)
	if err := s.Abandon(ctx, id, reason); err != nil {
		log.ErrorContext(ctx, "falha ao abandonar partida irrecuperável", "match_id", id, "err", err)
	}
}
```

- [ ] **Step 5: Rodar e ver passar**

Run: `cd backend && go vet ./... && TESTCONTAINERS_RYUK_DISABLED=true TRUCO_REQUIRE_DB=1 go test -race ./internal/match/ -count=1`
Expected: PASS (inclusive os testes antigos de `Recover` com log corrompido e partida em GameOver).

- [ ] **Step 6: Commit**

```bash
git add backend/internal/match
git commit -m "fix(match): ordem de lock nas estatísticas, LOCK TABLE no recompute e Recover resiliente"
```

---

### Task 5: `auth.Service.Wait` para os e-mails em voo (pendência 7, parte 1)

**Files:**
- Modify: `backend/internal/auth/service.go`
- Test: `backend/internal/auth/wait_test.go`

**Interfaces:**
- Produces: `func (s *Service) Wait(ctx context.Context) error` — espera os envios disparados por `sendLater`; devolve `ctx.Err()` se o contexto terminar antes.

- [ ] **Step 1: Escrever o teste que falha**

`backend/internal/auth/wait_test.go`:

```go
package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/mail"
)

type blockingMailer struct{ release chan struct{} }

func (m blockingMailer) Send(context.Context, mail.Message) error {
	<-m.release
	return nil
}

func TestWaitBlocksUntilBackgroundSendsFinish(t *testing.T) {
	release := make(chan struct{})
	s := NewService(Options{Mailer: blockingMailer{release}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	s.sendLater(mail.Message{To: "ana@empresa.com"})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := s.Wait(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Wait com envio pendente = %v", err)
	}
	close(release)
	if err := s.Wait(context.Background()); err != nil {
		t.Fatalf("Wait depois do envio = %v", err)
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/auth/ -run TestWaitBlocks -short`
Expected: FAIL — `s.Wait undefined`.

- [ ] **Step 3: Implementar**

Em `backend/internal/auth/service.go`: adicione `"sync"` aos imports, o campo `pending sync.WaitGroup // e-mails em envio` ao fim de `type Service struct`, e substitua `sendLater`:

```go
// sendLater envia fora da requisição: o tempo de resposta não revela se houve envio.
func (s *Service) sendLater(m mail.Message) {
	s.pending.Add(1)
	s.background(func() {
		defer s.pending.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.mailer.Send(ctx, m); err != nil {
			s.log.Error("falha ao enviar e-mail", "to", m.To, "subject", m.Subject, "err", err)
		}
	})
}

// Wait aguarda os e-mails em envio até ctx terminar; usado no desligamento gracioso.
func (s *Service) Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		s.pending.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
```

- [ ] **Step 4: Rodar e ver passar**

Run: `cd backend && go vet ./internal/auth/ && go test ./internal/auth/ -run TestWaitBlocks -short -race`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/auth
git commit -m "feat(auth): Wait aguarda os e-mails em envio no desligamento"
```

---

### Task 6: Configuração das salas

**Files:**
- Modify: `backend/internal/config/config.go`, `.env.example`
- Test: `backend/internal/config/config_test.go`

**Interfaces:**
- Produces:
  ```go
  // GameTiming tem os mesmos campos, na mesma ordem, de room.Timing (conversão direta room.Timing(cfg.Game)).
  type GameTiming struct {
      Play, Vote, Choice, Burn, Absent time.Duration
      HandPause                        time.Duration
      AbsenceGrace, AbsenceBudget      time.Duration
      WaitingDisconnect                time.Duration
  }
  // Config ganha: Game GameTiming; MaxRooms int; ShutdownTimeout time.Duration
  ```

- [ ] **Step 1: Escrever os testes que falham**

Acrescente a `backend/internal/config/config_test.go` (adicione `"time"` aos imports):

```go
func TestGameTimingDefaults(t *testing.T) {
	cfg, err := Load(env(validEnv()))
	if err != nil {
		t.Fatal(err)
	}
	want := GameTiming{Play: 30 * time.Second, Vote: 20 * time.Second, Choice: 15 * time.Second, Burn: 15 * time.Second,
		Absent: 5 * time.Second, HandPause: 4 * time.Second, AbsenceGrace: 120 * time.Second,
		AbsenceBudget: 300 * time.Second, WaitingDisconnect: 60 * time.Second}
	if cfg.Game != want || cfg.MaxRooms != 50 || cfg.ShutdownTimeout != 15*time.Second {
		t.Fatalf("padrões: %+v, MaxRooms %d, Shutdown %v", cfg.Game, cfg.MaxRooms, cfg.ShutdownTimeout)
	}
}

func TestGameTimingOverridesAndLimits(t *testing.T) {
	values := validEnv()
	values["TIMER_PLAY_SECONDS"] = "45"
	values["HAND_PAUSE_SECONDS"] = "0"
	values["MAX_ROOMS"] = "7"
	cfg, err := Load(env(values))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Game.Play != 45*time.Second || cfg.Game.HandPause != 0 || cfg.MaxRooms != 7 {
		t.Fatalf("sobrescritas: %+v %d", cfg.Game, cfg.MaxRooms)
	}
	for key, bad := range map[string]string{
		"TIMER_PLAY_SECONDS": "4", "TIMER_VOTE_SECONDS": "301", "TIMER_CHOICE_SECONDS": "x",
		"TIMER_BURN_SECONDS": "0", "TIMER_ABSENT_SECONDS": "61", "HAND_PAUSE_SECONDS": "31",
		"ABSENCE_GRACE_SECONDS": "9", "WAITING_DISCONNECT_SECONDS": "4", "MAX_ROOMS": "0",
		"SHUTDOWN_TIMEOUT_SECONDS": "121",
	} {
		values := validEnv()
		values[key] = bad
		if _, err := Load(env(values)); err == nil || !strings.Contains(err.Error(), key) {
			t.Fatalf("%s=%s deveria falhar citando a variável: %v", key, bad, err)
		}
	}
}

func TestAbsenceBudgetMustCoverGrace(t *testing.T) {
	values := validEnv()
	values["ABSENCE_GRACE_SECONDS"] = "200"
	values["ABSENCE_BUDGET_SECONDS"] = "100"
	if _, err := Load(env(values)); err == nil || !strings.Contains(err.Error(), "ABSENCE_BUDGET_SECONDS") {
		t.Fatalf("orçamento menor que a carência deve falhar: %v", err)
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/config/`
Expected: FAIL — `undefined: GameTiming`.

- [ ] **Step 3: Implementar**

Em `backend/internal/config/config.go`: adicione `"time"` aos imports; declare o tipo antes de `Config`:

```go
// GameTiming reúne os tempos das salas (Spec 3 §12). Tem os mesmos campos, na mesma
// ordem, de room.Timing, para permitir a conversão direta room.Timing(cfg.Game).
type GameTiming struct {
	Play, Vote, Choice, Burn, Absent time.Duration
	HandPause                        time.Duration
	AbsenceGrace, AbsenceBudget      time.Duration
	WaitingDisconnect                time.Duration
}
```

acrescente ao fim de `Config`:

```go
	Game            GameTiming
	MaxRooms        int
	ShutdownTimeout time.Duration
```

e, em `Load`, logo antes do bloco `if err := cfg.LogLevel.UnmarshalText(...)`:

```go
	intIn := func(key string, fallback, lo, hi int) int {
		raw := get(key, strconv.Itoa(fallback))
		v, err := strconv.Atoi(raw)
		if err != nil || v < lo || v > hi {
			errs = append(errs, fmt.Errorf("%s deve ser um inteiro entre %d e %d, não %q", key, lo, hi, raw))
			return fallback
		}
		return v
	}
	seconds := func(key string, fallback, lo, hi int) time.Duration {
		return time.Duration(intIn(key, fallback, lo, hi)) * time.Second
	}
	cfg.Game = GameTiming{
		Play:              seconds("TIMER_PLAY_SECONDS", 30, 5, 300),
		Vote:              seconds("TIMER_VOTE_SECONDS", 20, 5, 300),
		Choice:            seconds("TIMER_CHOICE_SECONDS", 15, 5, 300),
		Burn:              seconds("TIMER_BURN_SECONDS", 15, 5, 300),
		Absent:            seconds("TIMER_ABSENT_SECONDS", 5, 1, 60),
		HandPause:         seconds("HAND_PAUSE_SECONDS", 4, 0, 30),
		AbsenceGrace:      seconds("ABSENCE_GRACE_SECONDS", 120, 10, 3600),
		AbsenceBudget:     seconds("ABSENCE_BUDGET_SECONDS", 300, 10, 7200),
		WaitingDisconnect: seconds("WAITING_DISCONNECT_SECONDS", 60, 5, 3600),
	}
	if cfg.Game.AbsenceBudget < cfg.Game.AbsenceGrace {
		errs = append(errs, errors.New("ABSENCE_BUDGET_SECONDS deve ser >= ABSENCE_GRACE_SECONDS"))
	}
	cfg.MaxRooms = intIn("MAX_ROOMS", 50, 1, 1000)
	cfg.ShutdownTimeout = seconds("SHUTDOWN_TIMEOUT_SECONDS", 15, 1, 120)
```

Em `.env.example` (raiz), acrescente ao fim:

```
# Salas (Spec 3). Tempos em segundos.
TIMER_PLAY_SECONDS=30
TIMER_VOTE_SECONDS=20
TIMER_CHOICE_SECONDS=15
TIMER_BURN_SECONDS=15
# Prazo de decisão de um jogador ausente (desconectado ou com 3 timeouts seguidos).
TIMER_ABSENT_SECONDS=5
HAND_PAUSE_SECONDS=4
# W.O. fica disponível após este tempo ausente seguido ou após o orçamento acumulado na partida.
ABSENCE_GRACE_SECONDS=120
ABSENCE_BUDGET_SECONDS=300
WAITING_DISCONNECT_SECONDS=60
MAX_ROOMS=50
SHUTDOWN_TIMEOUT_SECONDS=15
```

- [ ] **Step 4: Rodar e ver passar**

Run: `cd backend && go vet ./internal/config/ && go test ./internal/config/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/config .env.example
git commit -m "feat(config): tempos das salas, limite de salas e prazo de desligamento"
```

---
### Task 7: Protocolo redefinido e tipado

**Files:**
- Delete: `backend/internal/protocol/protocol.go` (conteúdo redistribuído)
- Create: `backend/internal/protocol/envelope.go`, `client.go`, `server.go`, `errors.go`, `registry.go`
- Modify: `backend/internal/protocol/protocol_test.go`
- Test: `backend/internal/protocol/messages_test.go`

**Interfaces:**
- Produces (todos em `package protocol`):
  - `const ProtocolVersion = 1`
  - Envelope: `ClientMessage{Type string; Seq int64; Payload json.RawMessage}`, `ServerMessage{Type ServerMessageType; Payload any}`, `func Message(t ServerMessageType, payload any) ServerMessage`
  - Tipos do cliente: `TypeCreateRoom`, `TypeJoinRoom`, `TypeLeaveRoom`, `TypeTakeSeat`, `TypeSetReady`, `TypeKick`, `TypeClaimForfeit`, `TypeSignal` (strings); jogo usa `string(engine.ActXxx)`.
  - Tipos do servidor (`ServerMessageType`): `MsgHello`, `MsgLobbySnapshot`, `MsgRoomUpserted`, `MsgRoomRemoved`, `MsgRoomState`, `MsgLeftRoom`, `MsgState`, `MsgEvents`, `MsgForfeitAvailable`, `MsgMatchEnded`, `MsgSignal`, `MsgError`.
  - Payloads do cliente: `CreateRoomPayload{Visibility}`, `JoinRoomPayload{RoomID *string; Code *string; Seat *engine.Seat}`, `TakeSeatPayload{Seat}`, `SetReadyPayload{Ready}`, `KickPayload{Seat}`, `SignalPayload{ID SignalID}` + os de jogo já existentes.
  - Payloads do servidor: `UserInfo`, `HelloPayload`, `SeatSummary`, `RoomSummary`, `LobbySnapshotPayload`, `RoomUpsertedPayload`, `RoomRemovedPayload`, `SeatInfo`, `RoomStatePayload`, `LeftRoomPayload`, `StatePayload`, `ForfeitAvailablePayload`, `MatchEndedPayload`, `SignalBroadcastPayload`, `ErrorPayload`.
  - Enums: `Visibility` (`VisibilityPublic`, `VisibilityPrivate`, `Valid()`), `RoomStatus` (`RoomWaiting`, `RoomPlaying`), `LeftReason` (`LeftLeft`, `LeftKicked`, `LeftDestroyed`), `EndReason` (`EndGameOver`, `EndForfeit`, `EndAborted`, `EndAbandoned`), `SignalID` (`SignalTruco`, `SignalBoa`, `SignalVamos`, `SignalCalma`, `SignalDesculpa`, `SignalValeu`, `Valid()`).
  - Códigos: `CodeBadRequest`, `CodeInternal`, `CodeAlreadyInRoom`, `CodeRoomFull`, `CodeRoomNotFound`, `CodeSeatTaken`, `CodeNotHost`, `CodeNotInRoom`, `CodeMatchInProgress`, `CodeForfeitUnavailable`, `CodeTooManyRooms`, `CodeRateLimited`.
  - Funções: `DecodeAction(msg, seat) (engine.Action, error)` (inalterada), `Decode(raw json.RawMessage, target any, required ...string) error` (antes `decodePayload`), `KnownClientType(t string) bool`, `StateMessage(g engine.GameState, seat engine.Seat, deadlines map[engine.Seat]time.Time) ServerMessage`, `EventsMessage(evs []engine.Event, seat engine.Seat) (ServerMessage, bool)` (false = nada visível), `ErrorMessage(err error, seq int64) ServerMessage`, `ErrorMsg(code string, seq int64) ServerMessage`, `Hello(UserInfo)`, `LobbySnapshot([]RoomSummary)`, `RoomUpserted(RoomSummary)`, `RoomRemoved(id string)`, `LeftRoom(LeftReason)`, `ForfeitAvailable(*engine.Team)`, `MatchEnded(EndReason, *engine.Team, [2]int)`, `SignalBroadcast(engine.Seat, SignalID)`.
  - Registro: `type MessageSpec struct{ Type string; Payload reflect.Type }`, `var ClientMessages, ServerMessages []MessageSpec`.

- [ ] **Step 1: Escrever os testes que falham**

`backend/internal/protocol/messages_test.go`:

```go
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
```

No `backend/internal/protocol/protocol_test.go` existente, ajuste às novas assinaturas:
- em `TestDecodeActionRejectsSystemAndUnknownTypes`, a lista vira `[]string{"sys_start_hand", "sys_timeout", "sys_forfeit", "hack"}`;
- em `TestStateAndEventsMessagesAreFilteredPerSeat`, troque `StateMessage(g, 0).Payload.(engine.PlayerView).CutOffer` por `StateMessage(g, 0, nil).Payload.(StatePayload).View.CutOffer`, e as duas chamadas `len(EventsMessage(evs, N).Payload.([]engine.Event))` por um helper local:

```go
	visible := func(seat engine.Seat) int {
		m, _ := EventsMessage(evs, seat)
		return len(m.Payload.([]engine.Event))
	}
	if visible(3) != 2 || visible(0) != 1 {
		t.Fatal("CutOffered só para quem corta (assento 3)")
	}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/protocol/`
Expected: FAIL — `undefined: CreateRoomPayload`, `undefined: Decode` etc.

- [ ] **Step 3: Implementar `envelope.go`**

Apague `backend/internal/protocol/protocol.go` (`git rm`) e crie `backend/internal/protocol/envelope.go`:

```go
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
	p := StatePayload{View: engine.ViewFor(g, seat), Deadlines: map[engine.Seat]int64{}, PendingSeats: []engine.Seat{}}
	for s, at := range deadlines {
		p.Deadlines[s] = at.UnixMilli()
		p.PendingSeats = append(p.PendingSeats, s)
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

func RoomUpserted(r RoomSummary) ServerMessage { return Message(MsgRoomUpserted, RoomUpsertedPayload{Room: r}) }

func RoomRemoved(id string) ServerMessage { return Message(MsgRoomRemoved, RoomRemovedPayload{RoomID: id}) }

func LeftRoom(reason LeftReason) ServerMessage { return Message(MsgLeftRoom, LeftRoomPayload{Reason: reason}) }

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
```

- [ ] **Step 4: Implementar `client.go`, `server.go`, `errors.go` e `registry.go`**

`backend/internal/protocol/client.go`:

```go
package protocol

import "github.com/caarlosandree/truco-multiplayer/backend/internal/engine"

// Tipos de mensagem de sala/social enviados pelo cliente. As de jogo usam engine.ActionKind.
const (
	TypeCreateRoom   = "create_room"
	TypeJoinRoom     = "join_room"
	TypeLeaveRoom    = "leave_room"
	TypeTakeSeat     = "take_seat"
	TypeSetReady     = "set_ready"
	TypeKick         = "kick"
	TypeClaimForfeit = "claim_forfeit"
	TypeSignal       = "signal"
)

type Visibility string

const (
	VisibilityPublic  Visibility = "public"
	VisibilityPrivate Visibility = "private"
)

func (v Visibility) Valid() bool { return v == VisibilityPublic || v == VisibilityPrivate }

type CreateRoomPayload struct {
	Visibility Visibility `json:"visibility"`
}

// JoinRoomPayload exige exatamente um de RoomID (sala pública da lista) ou Code (qualquer sala).
type JoinRoomPayload struct {
	RoomID *string      `json:"roomId,omitempty"`
	Code   *string      `json:"code,omitempty"`
	Seat   *engine.Seat `json:"seat,omitempty"`
}

type TakeSeatPayload struct {
	Seat engine.Seat `json:"seat"`
}

type SetReadyPayload struct {
	Ready bool `json:"ready"`
}

type KickPayload struct {
	Seat engine.Seat `json:"seat"`
}

type SignalID string

const (
	SignalTruco    SignalID = "truco"
	SignalBoa      SignalID = "boa"
	SignalVamos    SignalID = "vamos"
	SignalCalma    SignalID = "calma"
	SignalDesculpa SignalID = "desculpa"
	SignalValeu    SignalID = "valeu"
)

func (id SignalID) Valid() bool {
	switch id {
	case SignalTruco, SignalBoa, SignalVamos, SignalCalma, SignalDesculpa, SignalValeu:
		return true
	}
	return false
}

type SignalPayload struct {
	ID SignalID `json:"id"`
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
```

`backend/internal/protocol/server.go`:

```go
package protocol

import "github.com/caarlosandree/truco-multiplayer/backend/internal/engine"

type UserInfo struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

type HelloPayload struct {
	User            UserInfo `json:"user"`
	ProtocolVersion int      `json:"protocolVersion"`
}

type RoomStatus string

const (
	RoomWaiting RoomStatus = "waiting"
	RoomPlaying RoomStatus = "playing"
)

type SeatSummary struct {
	DisplayName string `json:"displayName"`
}

// RoomSummary é a linha da sala na lista do lobby (só salas públicas aparecem).
type RoomSummary struct {
	ID     string          `json:"id"`
	Name   string          `json:"name"`
	Status RoomStatus      `json:"status"`
	Seats  [4]*SeatSummary `json:"seats"`
	Score  *[2]int         `json:"score,omitempty"`
}

type LobbySnapshotPayload struct {
	Rooms []RoomSummary `json:"rooms"`
}

type RoomUpsertedPayload struct {
	Room RoomSummary `json:"room"`
}

type RoomRemovedPayload struct {
	RoomID string `json:"roomId"`
}

type SeatInfo struct {
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
	Ready       bool   `json:"ready"`
	Connected   bool   `json:"connected"`
	Absent      bool   `json:"absent"`
}

// RoomStatePayload é a visão da sala para um membro; o código só vai para membros.
type RoomStatePayload struct {
	ID         string       `json:"id"`
	Code       string       `json:"code"`
	Name       string       `json:"name"`
	Visibility Visibility   `json:"visibility"`
	Host       engine.Seat  `json:"host"`
	Status     RoomStatus   `json:"status"`
	Seats      [4]*SeatInfo `json:"seats"`
	MatchID    string       `json:"matchId,omitempty"`
	YourSeat   engine.Seat  `json:"yourSeat"`
}

type LeftReason string

const (
	LeftLeft      LeftReason = "left"
	LeftKicked    LeftReason = "kicked"
	LeftDestroyed LeftReason = "destroyed"
)

type LeftRoomPayload struct {
	Reason LeftReason `json:"reason"`
}

// StatePayload leva a projeção do assento e os prazos (epoch ms UTC) dos assentos pendentes.
type StatePayload struct {
	View         engine.PlayerView     `json:"view"`
	Deadlines    map[engine.Seat]int64 `json:"deadlines"`
	PendingSeats []engine.Seat         `json:"pendingSeats"`
}

// ForfeitAvailablePayload: Team é a dupla ausente contra a qual o W.O. pode ser pedido; null retira a oferta.
type ForfeitAvailablePayload struct {
	Team *engine.Team `json:"team"`
}

type EndReason string

const (
	EndGameOver  EndReason = "game_over"
	EndForfeit   EndReason = "forfeit"
	EndAborted   EndReason = "aborted"
	EndAbandoned EndReason = "abandoned"
)

type MatchEndedPayload struct {
	Reason EndReason    `json:"reason"`
	Winner *engine.Team `json:"winner"`
	Score  [2]int       `json:"score"`
}

type SignalBroadcastPayload struct {
	Seat engine.Seat `json:"seat"`
	ID   SignalID    `json:"id"`
}

type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Seq     int64  `json:"seq,omitempty"`
}
```

`backend/internal/protocol/errors.go`:

```go
package protocol

// Códigos de erro de sala/lobby. Erros de regra usam engine.ErrorCode.
const (
	CodeBadRequest         = "bad_request"
	CodeInternal           = "internal"
	CodeAlreadyInRoom      = "already_in_room"
	CodeRoomFull           = "room_full"
	CodeRoomNotFound       = "room_not_found"
	CodeSeatTaken          = "seat_taken"
	CodeNotHost            = "not_host"
	CodeNotInRoom          = "not_in_room"
	CodeMatchInProgress    = "match_in_progress"
	CodeForfeitUnavailable = "forfeit_unavailable"
	CodeTooManyRooms       = "too_many_rooms"
	CodeRateLimited        = "rate_limited"
)

var errorTexts = map[string]string{
	CodeBadRequest:         "mensagem inválida",
	CodeInternal:           "erro interno; tente de novo",
	CodeAlreadyInRoom:      "você já está em uma sala",
	CodeRoomFull:           "a sala está cheia",
	CodeRoomNotFound:       "sala não encontrada",
	CodeSeatTaken:          "assento ocupado",
	CodeNotHost:            "só o anfitrião pode fazer isso",
	CodeNotInRoom:          "você não está em uma sala",
	CodeMatchInProgress:    "a partida está em andamento",
	CodeForfeitUnavailable: "W.O. indisponível",
	CodeTooManyRooms:       "limite de salas atingido; tente mais tarde",
	CodeRateLimited:        "devagar: aguarde um pouco",
}

func errorText(code string) string {
	if text, ok := errorTexts[code]; ok {
		return text
	}
	return code
}
```

`backend/internal/protocol/registry.go`:

```go
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
```

- [ ] **Step 5: Rodar e ver passar**

Run: `cd backend && gofmt -l internal/protocol && go vet ./... && go test ./internal/protocol/`
Expected: PASS; `go vet ./...` sem erros em outros pacotes (nenhum pacote de produção usava `StateMessage`/`EventsMessage` fora de `protocol`).

- [ ] **Step 6: Commit**

```bash
git add -A backend/internal/protocol
git commit -m "feat(protocol): mensagens de lobby, sala e jogo com payloads tipados e registro"
```

---

### Task 8: Uniões TypeScript geradas e tipos privados fora do tygo (pendência 6)

**Files:**
- Create: `backend/internal/protocol/union.go`, `backend/internal/protocol/union_test.go`, `backend/internal/protocol/testdata/union.ts.golden`, `backend/cmd/gen-protocol/main.go`
- Modify: `backend/tygo.yaml`, `Makefile`
- Regenerate: `frontend/src/protocol/engine.ts`, `frontend/src/protocol/messages.ts`, `frontend/src/protocol/union.ts` (via `make gen-types`)

**Interfaces:**
- Consumes: `protocol.ClientMessages`, `protocol.ServerMessages` (Tarefa 7).
- Produces: `func RenderUnion() string` em `protocol`; `frontend/src/protocol/union.ts` com `ClientMessage` e `ServerMessage` discriminados por `type`.

- [ ] **Step 1: Escrever o teste e o golden**

`backend/internal/protocol/union_test.go`:

```go
package protocol

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "regrava os arquivos golden")

func TestRenderUnionGolden(t *testing.T) {
	got := RenderUnion()
	path := filepath.Join("testdata", "union.ts.golden")
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Fatalf("união TS mudou; revise e rode: go test ./internal/protocol -run Golden -update\n%s", got)
	}
}
```

`backend/internal/protocol/testdata/union.ts.golden` (conteúdo exato, terminando com uma quebra de linha):

```ts
// Code generated by cmd/gen-protocol. DO NOT EDIT.

import * as engine from "./engine";
import * as m from "./messages";

export type ClientMessage =
  | { type: "create_room"; seq?: number; payload: m.CreateRoomPayload }
  | { type: "join_room"; seq?: number; payload: m.JoinRoomPayload }
  | { type: "leave_room"; seq?: number }
  | { type: "take_seat"; seq?: number; payload: m.TakeSeatPayload }
  | { type: "set_ready"; seq?: number; payload: m.SetReadyPayload }
  | { type: "kick"; seq?: number; payload: m.KickPayload }
  | { type: "choose_card"; seq?: number; payload: m.ChooseCardPayload }
  | { type: "burn"; seq?: number; payload: m.BurnPayload }
  | { type: "pass_burn"; seq?: number }
  | { type: "play_card"; seq?: number; payload: m.PlayCardPayload }
  | { type: "request_raise"; seq?: number }
  | { type: "vote"; seq?: number; payload: m.VotePayload }
  | { type: "claim_forfeit"; seq?: number }
  | { type: "signal"; seq?: number; payload: m.SignalPayload };

export type ServerMessage =
  | { type: "hello"; payload: m.HelloPayload }
  | { type: "lobby_snapshot"; payload: m.LobbySnapshotPayload }
  | { type: "room_upserted"; payload: m.RoomUpsertedPayload }
  | { type: "room_removed"; payload: m.RoomRemovedPayload }
  | { type: "room_state"; payload: m.RoomStatePayload }
  | { type: "left_room"; payload: m.LeftRoomPayload }
  | { type: "state"; payload: m.StatePayload }
  | { type: "events"; payload: engine.Event[] }
  | { type: "forfeit_available"; payload: m.ForfeitAvailablePayload }
  | { type: "match_ended"; payload: m.MatchEndedPayload }
  | { type: "signal"; payload: m.SignalBroadcastPayload }
  | { type: "error"; payload: m.ErrorPayload };
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/protocol/ -run Golden`
Expected: FAIL — `undefined: RenderUnion`.

- [ ] **Step 3: Implementar o gerador**

`backend/internal/protocol/union.go`:

```go
package protocol

import (
	"fmt"
	"reflect"
	"strings"
)

// RenderUnion gera as uniões discriminadas TypeScript das mensagens (make gen-types).
// Payloads do pacote engine vêm de "./engine"; os demais, de "./messages" (ambos gerados pelo tygo).
func RenderUnion() string {
	var b strings.Builder
	b.WriteString("// Code generated by cmd/gen-protocol. DO NOT EDIT.\n\n")
	b.WriteString("import * as engine from \"./engine\";\nimport * as m from \"./messages\";\n\n")
	writeUnion(&b, "ClientMessage", ClientMessages, true)
	b.WriteString("\n")
	writeUnion(&b, "ServerMessage", ServerMessages, false)
	return b.String()
}

func writeUnion(b *strings.Builder, name string, specs []MessageSpec, withSeq bool) {
	fmt.Fprintf(b, "export type %s =\n", name)
	for i, spec := range specs {
		fields := fmt.Sprintf("type: %q", spec.Type)
		if withSeq {
			fields += "; seq?: number"
		}
		if spec.Payload != nil {
			fields += "; payload: " + tsType(spec.Payload)
		}
		end := ""
		if i == len(specs)-1 {
			end = ";"
		}
		fmt.Fprintf(b, "  | { %s }%s\n", fields, end)
	}
}

func tsType(t reflect.Type) string {
	if t.Kind() == reflect.Slice {
		return tsType(t.Elem()) + "[]"
	}
	if strings.HasSuffix(t.PkgPath(), "/internal/engine") {
		return "engine." + t.Name()
	}
	return "m." + t.Name()
}
```

`backend/cmd/gen-protocol/main.go`:

```go
// Comando gen-protocol escreve as uniões TypeScript das mensagens WebSocket.
//
// Uso (a partir de backend/): go run ./cmd/gen-protocol [-out caminho]
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
)

func main() {
	out := flag.String("out", "../frontend/src/protocol/union.ts", "arquivo de saída")
	flag.Parse()
	if err := os.WriteFile(*out, []byte(protocol.RenderUnion()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 4: Rodar e ver passar**

Run: `cd backend && go test ./internal/protocol/`
Expected: PASS. (Se falhar só por espaço/quebra de linha no fim, corrija o golden — nunca o gerador para "bater" com um golden digitado errado; o conteúdo acima é o contrato.)

- [ ] **Step 5: Ajustar tygo e Makefile e gerar**

Substitua `backend/tygo.yaml` por:

```yaml
packages:
  - path: "github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
    output_path: "../frontend/src/protocol/engine.ts"
    # Estado completo e privado do servidor: nunca vai ao cliente.
    exclude_files:
      - "gamestate.go"
  - path: "github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
    output_path: "../frontend/src/protocol/messages.ts"
    # Envelope (Payload any), registro e gerador: o contrato tipado é union.ts.
    exclude_files:
      - "envelope.go"
      - "registry.go"
      - "union.go"
    frontmatter: |
      import * as engine from "./engine";
    type_mappings:
      json.RawMessage: "unknown"
      engine.VoteChoice: "engine.VoteChoice"
      engine.Seat: "engine.Seat"
      engine.Team: "engine.Team"
      engine.PlayerView: "engine.PlayerView"
      engine.Event: "engine.Event"
```

No `Makefile` (raiz), troque a receita de `gen-types` por:

```make
gen-types:
	cd backend && go run github.com/gzuidhof/tygo@v0.2.21 generate && go run ./cmd/gen-protocol
```

Run (da raiz): `make gen-types`
Expected: os três arquivos em `frontend/src/protocol/` atualizados. Verifique:

```bash
grep -c "GameState\|HandState" frontend/src/protocol/engine.ts   # 0
grep -n "Payload: any\|ServerMessage\b" frontend/src/protocol/messages.ts   # nenhuma linha
grep -n "export interface StatePayload" -A4 frontend/src/protocol/messages.ts   # usa engine.PlayerView / engine.Seat
diff frontend/src/protocol/union.ts backend/internal/protocol/testdata/union.ts.golden   # sem diferenças
```

Se o tygo não reconhecer `exclude_files` (versão sem suporte), a saída ainda conterá `GameState`: nesse caso pare e reporte ao controlador em vez de editar os `.ts` à mão.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/protocol backend/cmd/gen-protocol backend/tygo.yaml Makefile frontend/src/protocol
git commit -m "feat(protocol): uniões TypeScript discriminadas e estado privado fora dos tipos gerados"
```

---
### Task 9: Sala em espera (`room`: portas, mensagens, assentos)

**Files:**
- Create: `backend/internal/room/ports.go`, `msgs.go`, `room.go`, `seats.go`, `timers.go`
- Test: `backend/internal/room/harness_test.go`, `backend/internal/room/seats_test.go`

**Interfaces:**
- Consumes: `clock.Clock` (Tarefa 1); `protocol.*` (Tarefa 7); `replay.PlayerSeat`, `replay.Source`.
- Produces (usado pelas Tarefas 10–14):
  ```go
  var ErrMatchClosed error
  type MatchStore interface {
      Create(ctx context.Context, seed uint64, firstDealer engine.Seat, players [4]replay.PlayerSeat) (string, error)
      AppendAction(ctx context.Context, id string, ply int, a engine.Action, src replay.Source) error
      Finish(ctx context.Context, id string) error
      Abandon(ctx context.Context, id, reason string) error
  }
  type Outbox interface{ Send(userID string, msg protocol.ServerMessage) }
  type Registry interface{ Update(s Summary); Released(roomID, userID string); Destroyed(roomID string) }
  type Summary struct{ Room protocol.RoomSummary; Public bool; Code string; MatchID string }
  type Timing struct{ Play, Vote, Choice, Burn, Absent, HandPause, AbsenceGrace, AbsenceBudget, WaitingDisconnect time.Duration }
  type Options struct{ ID, Code, Name string; Public bool; Timing Timing; Store MatchStore; Out Outbox; Registry Registry; Clock clock.Clock; Rand io.Reader; Log *slog.Logger; Post func(Msg) }
  type Msg interface{ roomMsg() }
  type Join struct{ UserID, DisplayName string; Seat *engine.Seat; Seq int64 }
  type Command struct{ UserID string; Message protocol.ClientMessage }
  type Connected struct{ UserID string }; type Disconnected struct{ UserID string }
  type Abort struct{ Reason string }; type Stop struct{}; type Wake struct{}
  func New(o Options) *Room
  func (r *Room) Handle(ctx context.Context, m Msg)
  func (r *Room) Closed() bool; func (r *Room) Status() protocol.RoomStatus; func (r *Room) Game() engine.GameState
  ```
- Contrato com o lobby: a sala chama `Registry.Released` sempre que um usuário deixa de pertencer a ela (saída, kick, entrada recusada, remoção, destruição) e `Registry.Destroyed` uma única vez, quando fecha. Depois de fechada, `Handle` só responde `Join`/`Command` com `room_not_found`.

- [ ] **Step 1: Escrever o harness de testes**

`backend/internal/room/harness_test.go`:

```go
package room_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/clock"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/room"
)

var (
	t0         = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	users      = [4]string{"u0", "u1", "u2", "u3"}
	names      = [4]string{"Ana", "Bia", "Caio", "Duda"}
	testTiming = room.Timing{Play: 30 * time.Second, Vote: 20 * time.Second, Choice: 15 * time.Second,
		Burn: 15 * time.Second, Absent: 5 * time.Second, HandPause: 4 * time.Second,
		AbsenceGrace: 2 * time.Minute, AbsenceBudget: 5 * time.Minute, WaitingDisconnect: time.Minute}
)

// fakeStore guarda os logs em memória e valida o contrato de escritor único (ply contíguo).
type fakeStore struct {
	records    map[string]*replay.Record
	order      []string
	abandoned  map[string]string
	failCreate error
	failAppend error
	failFinish error
	onAppend   func()
}

func newFakeStore() *fakeStore {
	return &fakeStore{records: map[string]*replay.Record{}, abandoned: map[string]string{}}
}

func (f *fakeStore) Create(_ context.Context, seed uint64, dealer engine.Seat, players [4]replay.PlayerSeat) (string, error) {
	if f.failCreate != nil {
		return "", f.failCreate
	}
	id := fmt.Sprintf("m%d", len(f.order)+1)
	f.records[id] = &replay.Record{ID: id, Seed: seed, FirstDealer: dealer, EngineVersion: engine.RulesVersion,
		Status: replay.StatusInProgress, Players: players}
	f.order = append(f.order, id)
	return id, nil
}

func (f *fakeStore) AppendAction(_ context.Context, id string, ply int, a engine.Action, src replay.Source) error {
	if f.onAppend != nil {
		f.onAppend()
	}
	if f.failAppend != nil {
		return f.failAppend
	}
	rec := f.records[id]
	if rec.Status != replay.StatusInProgress {
		return room.ErrMatchClosed
	}
	if ply != len(rec.Actions) {
		return fmt.Errorf("ply %d fora de sequência (próximo %d)", ply, len(rec.Actions))
	}
	rec.Actions = append(rec.Actions, replay.LoggedAction{Ply: ply, Action: a, Source: src})
	return nil
}

func (f *fakeStore) Finish(_ context.Context, id string) error {
	if f.failFinish != nil {
		return f.failFinish
	}
	rec := f.records[id]
	final, err := replay.Rebuild(*rec)
	if err != nil {
		return err
	}
	if final.Phase != engine.PhaseGameOver {
		return errors.New("partida não terminou")
	}
	rec.Status = replay.StatusFinished
	return nil
}

func (f *fakeStore) Abandon(_ context.Context, id, reason string) error {
	rec := f.records[id]
	if rec.Status != replay.StatusInProgress {
		return room.ErrMatchClosed
	}
	rec.Status = replay.StatusAbandoned
	f.abandoned[id] = reason
	return nil
}

func (f *fakeStore) last() *replay.Record { return f.records[f.order[len(f.order)-1]] }

func (f *fakeStore) lastAction() replay.LoggedAction {
	acts := f.last().Actions
	return acts[len(acts)-1]
}

// outbox grava as mensagens por usuário; é seguro para uso concorrente (testes do ator).
type outbox struct {
	mu     sync.Mutex
	byUser map[string][]protocol.ServerMessage
}

func newOutbox() *outbox { return &outbox{byUser: map[string][]protocol.ServerMessage{}} }

func (o *outbox) Send(user string, m protocol.ServerMessage) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.byUser[user] = append(o.byUser[user], m)
}

func (o *outbox) all(user string) []protocol.ServerMessage {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]protocol.ServerMessage(nil), o.byUser[user]...)
}

// take devolve e esquece as mensagens do usuário.
func (o *outbox) take(user string) []protocol.ServerMessage {
	o.mu.Lock()
	defer o.mu.Unlock()
	msgs := o.byUser[user]
	delete(o.byUser, user)
	return msgs
}

func (o *outbox) reset() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.byUser = map[string][]protocol.ServerMessage{}
}

func (o *outbox) count(user string, typ protocol.ServerMessageType) int {
	n := 0
	for _, m := range o.all(user) {
		if m.Type == typ {
			n++
		}
	}
	return n
}

func (o *outbox) errorCodes(user string) []string {
	var codes []string
	for _, m := range o.all(user) {
		if m.Type == protocol.MsgError {
			codes = append(codes, m.Payload.(protocol.ErrorPayload).Code)
		}
	}
	return codes
}

// last devolve o payload da última mensagem do tipo recebida pelo usuário.
func last[T any](t *testing.T, o *outbox, user string, typ protocol.ServerMessageType) T {
	t.Helper()
	msgs := o.all(user)
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Type == typ {
			return msgs[i].Payload.(T)
		}
	}
	t.Fatalf("%s não recebeu %s", user, typ)
	var zero T
	return zero
}

func lastCode(t *testing.T, o *outbox, user string) string {
	t.Helper()
	return last[protocol.ErrorPayload](t, o, user, protocol.MsgError).Code
}

type registry struct {
	mu        sync.Mutex
	updates   []room.Summary
	released  []string
	destroyed int
}

func (r *registry) Update(s room.Summary) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updates = append(r.updates, s)
}

func (r *registry) Released(_, user string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.released = append(r.released, user)
}

func (r *registry) Destroyed(string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.destroyed++
}

func (r *registry) wasReleased(user string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.released {
		if u == user {
			return true
		}
	}
	return false
}

func (r *registry) lastSummary() room.Summary {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.updates[len(r.updates)-1]
}

// harness dirige uma Room sem goroutines: disparos de timer vão para a fila e são processados em drain.
type harness struct {
	t     *testing.T
	clock *clock.Manual
	store *fakeStore
	out   *outbox
	reg   *registry
	opts  room.Options
	room  *room.Room
	queue []room.Msg
}

func newHarness(t *testing.T, mutate ...func(*room.Options)) *harness {
	h := &harness{t: t, clock: clock.NewManual(t0), store: newFakeStore(), out: newOutbox(), reg: &registry{}}
	o := room.Options{ID: "r1", Code: "K7QM2P", Name: "Sala de Ana", Public: true, Timing: testTiming,
		Store: h.store, Out: h.out, Registry: h.reg, Clock: h.clock, Rand: rand.NewChaCha8([32]byte{1}),
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Post: func(m room.Msg) { h.queue = append(h.queue, m) }}
	for _, m := range mutate {
		m(&o)
	}
	h.opts = o
	h.room = room.New(o)
	return h
}

func (h *harness) send(m room.Msg) {
	h.room.Handle(context.Background(), m)
	h.drain()
}

func (h *harness) drain() {
	for len(h.queue) > 0 {
		m := h.queue[0]
		h.queue = h.queue[1:]
		h.room.Handle(context.Background(), m)
	}
}

// advance avança o relógio em passos de no máximo 1s, processando os disparos a cada passo,
// para que prazos armados durante o avanço contem a partir do instante certo.
func (h *harness) advance(d time.Duration) {
	for d > 0 {
		step := min(d, time.Second)
		h.clock.Advance(step)
		h.drain()
		d -= step
	}
}

func (h *harness) cmd(user, typ string, payload any) {
	msg := protocol.ClientMessage{Type: typ, Seq: 7}
	if payload != nil {
		msg.Payload, _ = json.Marshal(payload)
	}
	h.send(room.Command{UserID: user, Message: msg})
}

func (h *harness) join(i int) { h.send(room.Join{UserID: users[i], DisplayName: names[i], Seq: 1}) }

func (h *harness) seatAll() {
	for i := range users {
		h.join(i)
	}
}

func (h *harness) readyAll() {
	for _, u := range users {
		h.cmd(u, protocol.TypeSetReady, protocol.SetReadyPayload{Ready: true})
	}
}

func (h *harness) startMatch() {
	h.t.Helper()
	h.seatAll()
	h.readyAll()
	if h.room.Status() != protocol.RoomPlaying {
		h.t.Fatal("a partida deveria ter começado")
	}
}

func clientMsg(a engine.Action) protocol.ClientMessage {
	msg := protocol.ClientMessage{Type: string(a.Kind), Seq: 3}
	var payload any
	switch a.Kind {
	case engine.ActChooseCard:
		payload = protocol.ChooseCardPayload{Keep: a.Keep}
	case engine.ActBurn:
		payload = protocol.BurnPayload{Slots: a.Slots}
	case engine.ActPlayCard:
		payload = protocol.PlayCardPayload{Slot: a.Slot, Covered: a.Covered}
	case engine.ActVote:
		payload = protocol.VotePayload{Choice: a.Choice}
	}
	if payload != nil {
		msg.Payload, _ = json.Marshal(payload)
	}
	return msg
}

func (h *harness) act(a engine.Action) {
	h.send(room.Command{UserID: users[a.Seat], Message: clientMsg(a)})
}

// nextMove é a jogada "humana" padrão: DefaultAction do menor assento pendente, com carta aberta
// (só cartas cobertas empatariam todas as rodadas e a partida não andaria).
func nextMove(g engine.GameState) (engine.Action, bool) {
	for s := engine.Seat(0); s < 4; s++ {
		if a, ok := engine.DefaultAction(g, s); ok {
			if a.Kind == engine.ActPlayCard {
				a.Covered = false
			}
			return a, true
		}
	}
	return engine.Action{}, false
}

// playUntil joga por todos até cond valer ou a partida acabar; a pausa entre mãos avança o relógio.
func (h *harness) playUntil(cond func(engine.GameState) bool) {
	h.t.Helper()
	for range 5000 {
		if h.room.Status() != protocol.RoomPlaying || h.room.Closed() {
			return
		}
		g := h.room.Game()
		if cond(g) {
			return
		}
		if g.Phase == engine.PhaseHandEnd {
			h.advance(testTiming.HandPause)
			continue
		}
		a, ok := nextMove(g)
		if !ok {
			h.t.Fatalf("nenhum assento pendente na fase %s", g.Phase)
		}
		h.act(a)
	}
	h.t.Fatal("a partida não chegou à condição")
}
```

- [ ] **Step 2: Escrever os testes da sala em espera**

`backend/internal/room/seats_test.go`:

```go
package room_test

import (
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/room"
)

func TestFirstJoinBecomesHostAndPublishes(t *testing.T) {
	h := newHarness(t)
	h.join(0)
	st := last[protocol.RoomStatePayload](t, h.out, "u0", protocol.MsgRoomState)
	if st.Host != 0 || st.YourSeat != 0 || st.Seats[0] == nil || st.Seats[0].DisplayName != "Ana" || !st.Seats[0].Connected ||
		st.Code != "K7QM2P" || st.Status != protocol.RoomWaiting || st.Visibility != protocol.VisibilityPublic || st.Name != "Sala de Ana" {
		t.Fatalf("room_state = %+v", st)
	}
	sum := h.reg.lastSummary()
	if !sum.Public || sum.Code != "K7QM2P" || sum.Room.ID != "r1" || sum.Room.Seats[0].DisplayName != "Ana" || sum.Room.Seats[1] != nil {
		t.Fatalf("resumo = %+v", sum)
	}
}

func TestJoinPicksRequestedOrFirstFreeSeat(t *testing.T) {
	h := newHarness(t)
	h.join(0)
	three := engine.Seat(3)
	h.send(room.Join{UserID: "u1", DisplayName: "Bia", Seat: &three, Seq: 4})
	h.join(2)
	st := last[protocol.RoomStatePayload](t, h.out, "u2", protocol.MsgRoomState)
	if st.YourSeat != 1 || st.Seats[3] == nil || st.Seats[3].UserID != "u1" {
		t.Fatalf("room_state = %+v", st)
	}
	h.send(room.Join{UserID: "u3", DisplayName: "Duda", Seat: &three, Seq: 5})
	if lastCode(t, h.out, "u3") != protocol.CodeSeatTaken || !h.reg.wasReleased("u3") {
		t.Fatal("assento ocupado deve recusar e liberar o usuário no lobby")
	}
}

func TestJoinFullRoomReleasesUser(t *testing.T) {
	h := newHarness(t)
	h.seatAll()
	h.send(room.Join{UserID: "u9", DisplayName: "Eva", Seq: 9})
	if lastCode(t, h.out, "u9") != protocol.CodeRoomFull || !h.reg.wasReleased("u9") {
		t.Fatal("sala cheia deve recusar e liberar o usuário")
	}
}

func TestJoinIsIdempotentForMember(t *testing.T) {
	h := newHarness(t)
	h.join(0)
	h.join(0)
	st := last[protocol.RoomStatePayload](t, h.out, "u0", protocol.MsgRoomState)
	if st.Seats[1] != nil || h.out.count("u0", protocol.MsgRoomState) != 2 {
		t.Fatalf("segunda entrada deve só reenviar o estado: %+v", st)
	}
}

func TestTakeSeatMovesAndClearsReady(t *testing.T) {
	h := newHarness(t)
	h.join(0)
	h.join(1)
	h.cmd("u1", protocol.TypeSetReady, protocol.SetReadyPayload{Ready: true})
	h.cmd("u1", protocol.TypeTakeSeat, protocol.TakeSeatPayload{Seat: 3})
	st := last[protocol.RoomStatePayload](t, h.out, "u1", protocol.MsgRoomState)
	if st.YourSeat != 3 || st.Seats[1] != nil || st.Seats[3].Ready {
		t.Fatalf("room_state = %+v", st)
	}
	h.cmd("u1", protocol.TypeTakeSeat, protocol.TakeSeatPayload{Seat: 0})
	if lastCode(t, h.out, "u1") != protocol.CodeSeatTaken {
		t.Fatal("trocar para assento ocupado deve falhar")
	}
}

func TestKickOnlyByHost(t *testing.T) {
	h := newHarness(t)
	h.join(0)
	h.join(1)
	h.cmd("u1", protocol.TypeKick, protocol.KickPayload{Seat: 0})
	if lastCode(t, h.out, "u1") != protocol.CodeNotHost {
		t.Fatal("só o anfitrião expulsa")
	}
	h.cmd("u0", protocol.TypeKick, protocol.KickPayload{Seat: 1})
	if left := last[protocol.LeftRoomPayload](t, h.out, "u1", protocol.MsgLeftRoom); left.Reason != protocol.LeftKicked {
		t.Fatalf("left_room = %+v", left)
	}
	if !h.reg.wasReleased("u1") || last[protocol.RoomStatePayload](t, h.out, "u0", protocol.MsgRoomState).Seats[1] != nil {
		t.Fatal("expulso deve sair do assento e do índice")
	}
}

func TestHostLeavingPassesHostToNextOccupant(t *testing.T) {
	h := newHarness(t)
	h.join(0)
	h.join(1)
	h.join(2)
	h.cmd("u0", protocol.TypeLeaveRoom, nil)
	if st := last[protocol.RoomStatePayload](t, h.out, "u1", protocol.MsgRoomState); st.Host != 1 || st.Seats[0] != nil {
		t.Fatalf("room_state = %+v", st)
	}
	if last[protocol.LeftRoomPayload](t, h.out, "u0", protocol.MsgLeftRoom).Reason != protocol.LeftLeft {
		t.Fatal("quem sai recebe left_room{left}")
	}
}

func TestLastLeaveDestroysRoom(t *testing.T) {
	h := newHarness(t)
	h.join(0)
	h.cmd("u0", protocol.TypeLeaveRoom, nil)
	if h.reg.destroyed != 1 || !h.room.Closed() {
		t.Fatal("sala vazia deve ser destruída")
	}
	h.send(room.Join{UserID: "u1", DisplayName: "Bia", Seq: 2})
	if lastCode(t, h.out, "u1") != protocol.CodeRoomNotFound || !h.reg.wasReleased("u1") {
		t.Fatal("sala destruída recusa entradas e libera o usuário")
	}
}

func TestCommandsAreValidated(t *testing.T) {
	h := newHarness(t)
	h.join(0)
	h.cmd("u9", protocol.TypeSetReady, protocol.SetReadyPayload{Ready: true})
	if lastCode(t, h.out, "u9") != protocol.CodeNotInRoom {
		t.Fatal("não membro recebe not_in_room")
	}
	h.cmd("u0", protocol.TypeTakeSeat, map[string]int{"seat": 9})
	if lastCode(t, h.out, "u0") != protocol.CodeBadRequest {
		t.Fatal("assento fora de 0..3 é bad_request")
	}
	h.cmd("u0", protocol.TypeSetReady, nil)
	if lastCode(t, h.out, "u0") != protocol.CodeBadRequest {
		t.Fatal("payload ausente é bad_request")
	}
}

func TestDisconnectAndReconnectUpdateSeat(t *testing.T) {
	h := newHarness(t)
	h.join(0)
	h.join(1)
	h.send(room.Disconnected{UserID: "u1"})
	if last[protocol.RoomStatePayload](t, h.out, "u0", protocol.MsgRoomState).Seats[1].Connected {
		t.Fatal("desconexão deve aparecer no room_state")
	}
	h.send(room.Connected{UserID: "u1"})
	if !last[protocol.RoomStatePayload](t, h.out, "u0", protocol.MsgRoomState).Seats[1].Connected {
		t.Fatal("reconexão deve aparecer no room_state")
	}
}
```

- [ ] **Step 3: Rodar e ver falhar**

Run: `cd backend && go test ./internal/room/`
Expected: FAIL — pacote `room` inexistente / `undefined: room.Timing`.

- [ ] **Step 4: Implementar portas e mensagens**

`backend/internal/room/ports.go`:

```go
package room

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/clock"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
)

// ErrMatchClosed indica que a partida foi encerrada fora da sala (ex.: abort do administrador).
// O adaptador do lobby traduz match.ErrMatchClosed para este erro.
var ErrMatchClosed = errors.New("room: partida encerrada fora da sala")

// MatchStore é o subconjunto de match.Store usado pela sala, que é o único escritor das ações da sua partida.
type MatchStore interface {
	Create(ctx context.Context, seed uint64, firstDealer engine.Seat, players [4]replay.PlayerSeat) (string, error)
	AppendAction(ctx context.Context, id string, ply int, a engine.Action, src replay.Source) error
	Finish(ctx context.Context, id string) error
	Abandon(ctx context.Context, id, reason string) error
}

// Outbox entrega mensagens a um usuário. Nunca bloqueia; usuário sem conexão é ignorado.
type Outbox interface {
	Send(userID string, msg protocol.ServerMessage)
}

// Registry é o lobby visto pela sala.
type Registry interface {
	Update(s Summary)               // o estado público da sala mudou
	Released(roomID, userID string) // o usuário deixou de pertencer à sala
	Destroyed(roomID string)        // a sala fechou e não aceita mais membros
}

// Summary é o que o lobby precisa saber da sala.
type Summary struct {
	Room    protocol.RoomSummary
	Public  bool
	Code    string
	MatchID string
}

// Timing tem os mesmos campos, na mesma ordem, de config.GameTiming (conversão direta).
type Timing struct {
	Play, Vote, Choice, Burn, Absent time.Duration
	HandPause                        time.Duration
	AbsenceGrace, AbsenceBudget      time.Duration
	WaitingDisconnect                time.Duration
}

type Options struct {
	ID       string
	Code     string
	Name     string
	Public   bool
	Timing   Timing
	Store    MatchStore
	Out      Outbox
	Registry Registry
	Clock    clock.Clock
	Rand     io.Reader // seed e dealer de cada partida (crypto/rand em produção)
	Log      *slog.Logger
	Post     func(Msg) // entrega mensagens (disparos de timer) na caixa de entrada do ator
}
```

`backend/internal/room/msgs.go`:

```go
package room

import (
	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
)

// Msg é uma mensagem da caixa de entrada da sala.
type Msg interface{ roomMsg() }

// Join pede um assento; o lobby já reservou o usuário para esta sala.
type Join struct {
	UserID      string
	DisplayName string
	Seat        *engine.Seat
	Seq         int64
}

// Command é uma mensagem do cliente para a sala (assentos, jogo, W.O., sinais).
type Command struct {
	UserID  string
	Message protocol.ClientMessage
}

type Connected struct{ UserID string }

type Disconnected struct{ UserID string }

// Abort encerra a partida viva por decisão do administrador.
type Abort struct{ Reason string }

// Stop encerra a sala no desligamento do servidor, sem tocar na partida (o Recover a retoma).
type Stop struct{}

// Wake só faz a sala reavaliar prazos e timers (usado ao iniciar o ator).
type Wake struct{}

type tick struct{ gen uint64 }

func (Join) roomMsg()         {}
func (Command) roomMsg()      {}
func (Connected) roomMsg()    {}
func (Disconnected) roomMsg() {}
func (Abort) roomMsg()        {}
func (Stop) roomMsg()         {}
func (Wake) roomMsg()         {}
func (tick) roomMsg()         {}
```

- [ ] **Step 5: Implementar a sala e os assentos**

`backend/internal/room/room.go`:

```go
// Package room implementa a sala: assentos, partida, timers e presença. Não conhece rede nem
// banco concreto; o ator (actor.go) garante que Handle seja chamado por uma goroutine de cada vez.
package room

import (
	"context"
	"log/slog"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/clock"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
)

type seatState struct {
	userID         string
	name           string
	ready          bool
	connected      bool
	disconnectedAt time.Time
	timeouts       int           // timeouts seguidos
	absentSince    time.Time     // zero = presente
	absentTotal    time.Duration // ausência acumulada na partida, sem o trecho corrente
	lastSignal     time.Time
}

type Room struct {
	opt    Options
	ctx    context.Context
	status protocol.RoomStatus
	host   engine.Seat
	seats  [4]*seatState
	closed bool

	matchID    string
	game       engine.GameState
	nextPly    int
	window     string                    // identifica a janela de decisão corrente
	deadlines  map[engine.Seat]time.Time // prazo de cada assento pendente
	pauseUntil time.Time                 // fim da pausa entre mãos
	offered    *engine.Team              // dupla contra a qual o W.O. está disponível

	timer clock.Timer
	gen   uint64
}

func New(o Options) *Room {
	if o.Log == nil {
		o.Log = slog.New(slog.DiscardHandler)
	}
	return &Room{opt: o, ctx: context.Background(), status: protocol.RoomWaiting}
}

func (r *Room) Closed() bool { return r.closed }

func (r *Room) Status() protocol.RoomStatus { return r.status }

// Game devolve uma cópia do estado da partida (testes e diagnóstico).
func (r *Room) Game() engine.GameState { return r.game.Clone() }

func (r *Room) now() time.Time { return r.opt.Clock.Now() }

// Handle processa uma mensagem e depois reavalia prazos e timers.
func (r *Room) Handle(ctx context.Context, m Msg) {
	if r.closed {
		r.rejectClosed(m)
		return
	}
	r.ctx = ctx
	switch m := m.(type) {
	case Join:
		r.join(m)
	case Command:
		r.command(m)
	case Connected:
		r.connected(m.UserID)
	case Disconnected:
		r.disconnected(m.UserID)
	case Stop:
		r.stop()
		return
	case tick:
		if m.gen != r.gen {
			return // disparo de um timer já substituído
		}
	case Wake:
	}
	r.settle()
}

func (r *Room) rejectClosed(m Msg) {
	switch m := m.(type) {
	case Join:
		r.opt.Out.Send(m.UserID, protocol.ErrorMsg(protocol.CodeRoomNotFound, m.Seq))
		r.opt.Registry.Released(r.opt.ID, m.UserID)
	case Command:
		r.opt.Out.Send(m.UserID, protocol.ErrorMsg(protocol.CodeRoomNotFound, m.Message.Seq))
	}
}

func (r *Room) stop() {
	r.closed = true
	r.stopTimer()
}
```

`backend/internal/room/seats.go`:

```go
package room

import (
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
)

func (r *Room) seatOf(userID string) (engine.Seat, bool) {
	for s, st := range r.seats {
		if st != nil && st.userID == userID {
			return engine.Seat(s), true
		}
	}
	return 0, false
}

func (r *Room) members() int {
	n := 0
	for _, st := range r.seats {
		if st != nil {
			n++
		}
	}
	return n
}

func (r *Room) firstOccupied() engine.Seat {
	for s, st := range r.seats {
		if st != nil {
			return engine.Seat(s)
		}
	}
	return 0
}

func (r *Room) reply(userID, code string, seq int64) {
	r.opt.Out.Send(userID, protocol.ErrorMsg(code, seq))
}

func (r *Room) broadcast(msg protocol.ServerMessage) {
	for _, st := range r.seats {
		if st != nil {
			r.opt.Out.Send(st.userID, msg)
		}
	}
}

func (r *Room) join(m Join) {
	if seat, ok := r.seatOf(m.UserID); ok {
		r.opt.Out.Send(m.UserID, r.roomState(seat))
		return
	}
	fail := func(code string) {
		r.reply(m.UserID, code, m.Seq)
		r.opt.Registry.Released(r.opt.ID, m.UserID)
	}
	if r.status != protocol.RoomWaiting {
		fail(protocol.CodeMatchInProgress)
		return
	}
	seat, code := r.pickSeat(m.Seat)
	if code != "" {
		fail(code)
		return
	}
	r.seats[seat] = &seatState{userID: m.UserID, name: m.DisplayName, connected: true}
	if r.members() == 1 {
		r.host = seat
	}
	r.publish()
}

func (r *Room) pickSeat(want *engine.Seat) (engine.Seat, string) {
	if r.members() == 4 {
		return 0, protocol.CodeRoomFull
	}
	if want != nil {
		if *want > 3 {
			return 0, protocol.CodeBadRequest
		}
		if r.seats[*want] != nil {
			return 0, protocol.CodeSeatTaken
		}
		return *want, ""
	}
	return r.freeSeat(), ""
}

func (r *Room) freeSeat() engine.Seat {
	for s, st := range r.seats {
		if st == nil {
			return engine.Seat(s)
		}
	}
	return 0
}

func (r *Room) command(m Command) {
	seat, ok := r.seatOf(m.UserID)
	if !ok {
		r.reply(m.UserID, protocol.CodeNotInRoom, m.Message.Seq)
		return
	}
	switch m.Message.Type {
	case protocol.TypeLeaveRoom:
		r.leave(seat, m.Message)
	case protocol.TypeTakeSeat:
		r.takeSeat(seat, m.Message)
	case protocol.TypeSetReady:
		r.setReady(seat, m.Message)
	case protocol.TypeKick:
		r.kick(seat, m.Message)
	default:
		r.reply(m.UserID, protocol.CodeBadRequest, m.Message.Seq)
	}
}

// requireWaiting responde match_in_progress quando a sala está em jogo.
func (r *Room) requireWaiting(seat engine.Seat, seq int64) bool {
	if r.status == protocol.RoomWaiting {
		return true
	}
	r.reply(r.seats[seat].userID, protocol.CodeMatchInProgress, seq)
	return false
}

func (r *Room) leave(seat engine.Seat, msg protocol.ClientMessage) {
	if r.requireWaiting(seat, msg.Seq) {
		r.removeSeat(seat, protocol.LeftLeft)
	}
}

func (r *Room) takeSeat(seat engine.Seat, msg protocol.ClientMessage) {
	if !r.requireWaiting(seat, msg.Seq) {
		return
	}
	st := r.seats[seat]
	var p protocol.TakeSeatPayload
	if err := protocol.Decode(msg.Payload, &p, "seat"); err != nil || p.Seat > 3 {
		r.reply(st.userID, protocol.CodeBadRequest, msg.Seq)
		return
	}
	if p.Seat == seat {
		r.opt.Out.Send(st.userID, r.roomState(seat))
		return
	}
	if r.seats[p.Seat] != nil {
		r.reply(st.userID, protocol.CodeSeatTaken, msg.Seq)
		return
	}
	r.seats[p.Seat], r.seats[seat] = st, nil
	if r.host == seat {
		r.host = p.Seat
	}
	st.ready = false
	r.publish()
}

func (r *Room) setReady(seat engine.Seat, msg protocol.ClientMessage) {
	if !r.requireWaiting(seat, msg.Seq) {
		return
	}
	st := r.seats[seat]
	var p protocol.SetReadyPayload
	if err := protocol.Decode(msg.Payload, &p, "ready"); err != nil {
		r.reply(st.userID, protocol.CodeBadRequest, msg.Seq)
		return
	}
	st.ready = p.Ready
	r.publish()
}

func (r *Room) kick(seat engine.Seat, msg protocol.ClientMessage) {
	if !r.requireWaiting(seat, msg.Seq) {
		return
	}
	user := r.seats[seat].userID
	if seat != r.host {
		r.reply(user, protocol.CodeNotHost, msg.Seq)
		return
	}
	var p protocol.KickPayload
	if err := protocol.Decode(msg.Payload, &p, "seat"); err != nil || p.Seat > 3 || p.Seat == seat || r.seats[p.Seat] == nil {
		r.reply(user, protocol.CodeBadRequest, msg.Seq)
		return
	}
	r.removeSeat(p.Seat, protocol.LeftKicked)
}

// removeSeat tira o ocupante; a sala vazia é destruída.
func (r *Room) removeSeat(seat engine.Seat, reason protocol.LeftReason) {
	st := r.seats[seat]
	r.seats[seat] = nil
	r.opt.Out.Send(st.userID, protocol.LeftRoom(reason))
	r.opt.Registry.Released(r.opt.ID, st.userID)
	if r.members() == 0 {
		r.destroy()
		return
	}
	if r.host == seat {
		r.host = r.firstOccupied()
	}
	r.publish()
}

func (r *Room) destroy() {
	r.closed = true
	r.stopTimer()
	r.opt.Registry.Destroyed(r.opt.ID)
}

func (r *Room) connected(userID string) {
	seat, ok := r.seatOf(userID)
	if !ok {
		return
	}
	st := r.seats[seat]
	st.connected, st.disconnectedAt, st.timeouts = true, time.Time{}, 0
	r.publish()
}

func (r *Room) disconnected(userID string) {
	seat, ok := r.seatOf(userID)
	if !ok {
		return
	}
	st := r.seats[seat]
	st.connected, st.disconnectedAt = false, r.now()
	r.publish()
}

// publish envia o room_state a cada membro e o resumo ao lobby.
func (r *Room) publish() {
	for s, st := range r.seats {
		if st != nil {
			r.opt.Out.Send(st.userID, r.roomState(engine.Seat(s)))
		}
	}
	r.opt.Registry.Update(r.summary())
}

func (r *Room) visibility() protocol.Visibility {
	if r.opt.Public {
		return protocol.VisibilityPublic
	}
	return protocol.VisibilityPrivate
}

func (r *Room) roomState(seat engine.Seat) protocol.ServerMessage {
	p := protocol.RoomStatePayload{ID: r.opt.ID, Code: r.opt.Code, Name: r.opt.Name, Visibility: r.visibility(),
		Host: r.host, Status: r.status, MatchID: r.matchID, YourSeat: seat}
	for s, st := range r.seats {
		if st != nil {
			p.Seats[s] = &protocol.SeatInfo{UserID: st.userID, DisplayName: st.name, Ready: st.ready,
				Connected: st.connected, Absent: r.status == protocol.RoomPlaying && !st.absentSince.IsZero()}
		}
	}
	return protocol.Message(protocol.MsgRoomState, p)
}

func (r *Room) summary() Summary {
	rs := protocol.RoomSummary{ID: r.opt.ID, Name: r.opt.Name, Status: r.status}
	for s, st := range r.seats {
		if st != nil {
			rs.Seats[s] = &protocol.SeatSummary{DisplayName: st.name}
		}
	}
	if r.status == protocol.RoomPlaying {
		score := r.game.Score
		rs.Score = &score
	}
	return Summary{Room: rs, Public: r.opt.Public, Code: r.opt.Code, MatchID: r.matchID}
}
```

`backend/internal/room/timers.go` (a Tarefa 11 substitui `settle` pela versão completa):

```go
package room

// settle reavalia prazos e timers depois de cada mensagem.
func (r *Room) settle() {}

func (r *Room) stopTimer() {
	if r.timer != nil {
		r.timer.Stop()
		r.timer = nil
	}
}
```

- [ ] **Step 6: Rodar e ver passar**

Run: `cd backend && gofmt -l internal/room && go vet ./internal/room/ && go test ./internal/room/`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/room
git commit -m "feat(room): sala em espera com assentos, anfitrião, pronto, kick e destruição"
```

---

### Task 10: Partida na sala — início, jogadas, persistência, fim, abort e restore

**Files:**
- Create: `backend/internal/room/playing.go`, `backend/internal/room/window.go`
- Modify: `backend/internal/room/room.go` (caso `Abort`), `backend/internal/room/seats.go` (`command`, `setReady`, `connected`)
- Test: `backend/internal/room/playing_test.go`

**Interfaces:**
- Consumes: Tarefa 9.
- Produces:
  - `type Restored struct{ MatchID string; Players [4]replay.PlayerSeat; State engine.GameState; NextPly int }`
  - `func NewRestored(o Options, rs Restored) *Room` (sala em `playing`, todos desconectados, anfitrião no assento 0)
  - internos usados pelas Tarefas 11–13: `apply(a engine.Action, src replay.Source) error` (erros: `*engine.RuleError`, `errPersist`, `errMatchGone`), `syncWindow(now)`, `decisionTime(seat)`, `pendingSeats(g)`, `(*seatState).isAbsent()`, `stateFor(seat)`, `broadcastState()`, `endMatch`, `resetToWaiting`, `storeCtx()`, `retryDelay`.
- Contrato (Spec 2 §7.2): `Apply` → `AppendAction` **antes** de enviar `events`/`state`; falha de gravação descarta o novo estado.

- [ ] **Step 1: Escrever os testes que falham**

`backend/internal/room/playing_test.go`:

```go
package room_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/enginetest"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/room"
)

func TestAllReadyStartsMatch(t *testing.T) {
	h := newHarness(t)
	h.startMatch()
	rec := h.store.last()
	if len(rec.Actions) != 1 || rec.Actions[0].Action.Kind != engine.ActSysStartHand || rec.Actions[0].Source != replay.SourceSystem {
		t.Fatalf("log inicial = %+v", rec.Actions)
	}
	for i, u := range users {
		if rec.Players[i] != (replay.PlayerSeat{UserID: u, DisplayName: names[i]}) {
			t.Fatalf("jogador %d = %+v", i, rec.Players[i])
		}
		st := last[protocol.RoomStatePayload](t, h.out, u, protocol.MsgRoomState)
		if st.Status != protocol.RoomPlaying || st.MatchID != rec.ID {
			t.Fatalf("%s: room_state = %+v", u, st)
		}
		state := last[protocol.StatePayload](t, h.out, u, protocol.MsgState)
		if state.View.Seat != engine.Seat(i) || state.View.Phase != engine.PhaseCutChoice || len(state.PendingSeats) != 1 {
			t.Fatalf("%s: state = %+v", u, state)
		}
	}
	if sum := h.reg.lastSummary(); sum.MatchID != rec.ID || sum.Room.Status != protocol.RoomPlaying || sum.Room.Score == nil {
		t.Fatalf("resumo = %+v", sum)
	}
}

func TestPlayerActionIsPersistedBeforeBroadcast(t *testing.T) {
	h := newHarness(t)
	h.startMatch()
	before := h.out.count("u0", protocol.MsgState)
	seenAtAppend := -1
	h.store.onAppend = func() { seenAtAppend = h.out.count("u0", protocol.MsgState) }
	a, _ := nextMove(h.room.Game())
	h.act(a)
	if seenAtAppend != before || h.out.count("u0", protocol.MsgState) != before+1 {
		t.Fatalf("estado enviado antes de gravar: na gravação %d, antes %d", seenAtAppend, before)
	}
	if got := h.store.lastAction(); got.Action.Kind != a.Kind || got.Source != replay.SourcePlayer || got.Ply != 1 {
		t.Fatalf("ação gravada = %+v", got)
	}
}

func TestRuleErrorKeepsStateAndLogNothing(t *testing.T) {
	h := newHarness(t)
	h.startMatch()
	g := h.room.Game()
	intruder := engine.Next(g.Cutter())
	illegal := engine.Action{Kind: engine.ActChooseCard, Seat: intruder, Keep: 0}
	_, _, want := engine.Apply(g, illegal)
	var ruleErr *engine.RuleError
	if !errors.As(want, &ruleErr) {
		t.Fatalf("a ação deveria ser ilegal: %v", want)
	}
	h.act(illegal)
	p := last[protocol.ErrorPayload](t, h.out, users[intruder], protocol.MsgError)
	if p.Code != string(ruleErr.Code) || p.Seq != 3 {
		t.Fatalf("erro = %+v", p)
	}
	if len(h.store.last().Actions) != 1 || !reflect.DeepEqual(h.room.Game(), g) {
		t.Fatal("jogada ilegal não pode gravar nem mudar o estado")
	}
}

func TestAppendFailureDiscardsState(t *testing.T) {
	h := newHarness(t)
	h.startMatch()
	g := h.room.Game()
	a, _ := nextMove(g)
	h.store.failAppend = errors.New("banco fora")
	h.act(a)
	if lastCode(t, h.out, users[a.Seat]) != protocol.CodeInternal || !reflect.DeepEqual(h.room.Game(), g) {
		t.Fatal("falha de gravação deve descartar o estado e responder internal")
	}
	h.store.failAppend = nil
	h.act(a)
	if len(h.store.last().Actions) != 2 {
		t.Fatal("a mesma jogada deve funcionar depois que o banco volta")
	}
}

func TestClosedMatchEndsAsAborted(t *testing.T) {
	h := newHarness(t)
	h.startMatch()
	h.store.last().Status = replay.StatusAbandoned
	a, _ := nextMove(h.room.Game())
	h.act(a)
	for _, u := range users {
		if e := last[protocol.MatchEndedPayload](t, h.out, u, protocol.MsgMatchEnded); e.Reason != protocol.EndAborted || e.Winner != nil {
			t.Fatalf("%s: match_ended = %+v", u, e)
		}
	}
	st := last[protocol.RoomStatePayload](t, h.out, "u0", protocol.MsgRoomState)
	if h.room.Status() != protocol.RoomWaiting || st.MatchID != "" {
		t.Fatalf("sala deve voltar a esperar: %+v", st)
	}
	for _, seat := range st.Seats {
		if seat.Ready {
			t.Fatal("pronto deve ser desmarcado")
		}
	}
}

func TestAbortMessageEndsMatch(t *testing.T) {
	h := newHarness(t)
	h.startMatch()
	h.send(room.Abort{Reason: "admin"})
	if e := last[protocol.MatchEndedPayload](t, h.out, "u2", protocol.MsgMatchEnded); e.Reason != protocol.EndAborted {
		t.Fatalf("match_ended = %+v", e)
	}
	if h.room.Status() != protocol.RoomWaiting {
		t.Fatal("abort volta a sala para a espera")
	}
}

func TestCreateFailureReturnsToWaiting(t *testing.T) {
	h := newHarness(t)
	h.store.failCreate = errors.New("banco fora")
	h.seatAll()
	h.readyAll()
	if h.room.Status() != protocol.RoomWaiting || lastCode(t, h.out, "u3") != protocol.CodeInternal {
		t.Fatal("falha ao criar a partida volta a esperar com erro interno")
	}
	for _, seat := range last[protocol.RoomStatePayload](t, h.out, "u0", protocol.MsgRoomState).Seats {
		if seat.Ready {
			t.Fatal("pronto deve ser desmarcado")
		}
	}
}

func TestStartHandFailureAbandonsCreatedMatch(t *testing.T) {
	h := newHarness(t)
	h.store.failAppend = errors.New("banco fora")
	h.seatAll()
	h.readyAll()
	id := h.store.last().ID
	if h.room.Status() != protocol.RoomWaiting || h.store.abandoned[id] != "start_failed" {
		t.Fatalf("status %s, abandono %q", h.room.Status(), h.store.abandoned[id])
	}
}

func TestRestoredRoomResumesWithSeatsOffline(t *testing.T) {
	full, _ := enginetest.RandomRecord(9)
	rec := full
	var state engine.GameState
	plies := 15
	for ; ; plies++ { // corta o log num ponto em que alguém precisa decidir
		rec.Actions = full.Actions[:plies]
		var err error
		if state, err = replay.Rebuild(rec); err != nil {
			t.Fatal(err)
		}
		if _, ok := nextMove(state); ok {
			break
		}
	}
	h := newHarness(t)
	h.store.records[rec.ID] = &rec
	h.store.order = append(h.store.order, rec.ID)
	h.room = room.NewRestored(h.opts, room.Restored{MatchID: rec.ID, Players: rec.Players, State: state, NextPly: plies})
	h.send(room.Wake{})
	if h.room.Status() != protocol.RoomPlaying {
		t.Fatal("sala restaurada começa em jogo")
	}
	h.send(room.Connected{UserID: "u2"})
	st := last[protocol.RoomStatePayload](t, h.out, "u2", protocol.MsgRoomState)
	if st.MatchID != rec.ID || !st.Seats[2].Connected || st.Seats[0].Connected || st.Host != 0 {
		t.Fatalf("room_state = %+v", st)
	}
	view := last[protocol.StatePayload](t, h.out, "u2", protocol.MsgState).View
	if !reflect.DeepEqual(view, engine.ViewFor(state, 2)) {
		t.Fatal("reconexão deve receber a view do estado restaurado")
	}
	a, _ := nextMove(h.room.Game())
	h.send(room.Connected{UserID: users[a.Seat]})
	h.act(a)
	if got := h.store.lastAction(); got.Ply != plies {
		t.Fatalf("próxima ação gravada no ply %d, quer %d", got.Ply, plies)
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/room/`
Expected: FAIL — `undefined: room.NewRestored`; os demais falham porque `set_ready` não inicia a partida.

- [ ] **Step 3: Implementar a janela de decisão**

`backend/internal/room/window.go`:

```go
package room

import (
	"fmt"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
)

// absentAfterTimeouts timeouts seguidos tornam um jogador conectado ausente.
const absentAfterTimeouts = 3

func (st *seatState) isAbsent() bool { return !st.connected || st.timeouts >= absentAfterTimeouts }

// pendingSeats são os assentos com decisão pendente no estado.
func pendingSeats(g engine.GameState) []engine.Seat {
	var out []engine.Seat
	for s := engine.Seat(0); s < 4; s++ {
		if _, ok := engine.DefaultAction(g, s); ok {
			out = append(out, s)
		}
	}
	return out
}

// windowKey identifica a janela de decisão: muda com a vez, a rodada, a carta na mesa,
// o pedido de aumento e a fase, mas não quando um votante ou um queimador simultâneo decide.
func windowKey(g engine.GameState) string {
	h := g.Hand
	if h == nil {
		return string(g.Phase)
	}
	stake, voting := 0, -1
	if h.Pending != nil {
		stake = h.Pending.ToStake
	}
	if h.VotingTeam != nil {
		voting = int(*h.VotingTeam)
	}
	return fmt.Sprintf("%d|%s|%d|%d|%d|%d|%d", g.HandNumber, g.Phase, h.Turn, len(h.Rounds), len(h.Table), stake, voting)
}

func phaseDuration(t Timing, p engine.Phase) time.Duration {
	switch p {
	case engine.PhasePlaying:
		return t.Play
	case engine.PhaseAwaitingRaiseResponse, engine.PhaseElevenDecision:
		return t.Vote
	case engine.PhaseCutChoice, engine.PhaseDealerChoice:
		return t.Choice
	case engine.PhaseBurnFours:
		return t.Burn
	}
	return 0
}

// decisionTime é o prazo do assento na fase corrente; ausente decide em TIMER_ABSENT.
func (r *Room) decisionTime(s engine.Seat) time.Duration {
	d := phaseDuration(r.opt.Timing, r.game.Phase)
	if st := r.seats[s]; st != nil && st.isAbsent() && r.opt.Timing.Absent < d {
		d = r.opt.Timing.Absent
	}
	return d
}

// syncWindow rearma os prazos quando a janela muda; dentro da mesma janela só esquece
// quem já decidiu. Ao entrar em hand_end, marca o fim da pausa entre mãos.
func (r *Room) syncWindow(now time.Time) {
	key := windowKey(r.game)
	if key != r.window {
		r.window = key
		r.deadlines = map[engine.Seat]time.Time{}
		for _, s := range pendingSeats(r.game) {
			r.deadlines[s] = now.Add(r.decisionTime(s))
		}
		if r.game.Phase == engine.PhaseHandEnd {
			r.pauseUntil = now.Add(r.opt.Timing.HandPause)
		}
		return
	}
	pending := map[engine.Seat]bool{}
	for _, s := range pendingSeats(r.game) {
		pending[s] = true
	}
	for s := range r.deadlines {
		if !pending[s] {
			delete(r.deadlines, s)
		}
	}
}
```

- [ ] **Step 4: Implementar o ciclo da partida**

`backend/internal/room/playing.go`:

```go
package room

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
)

var (
	errPersist   = errors.New("room: falha ao gravar a ação")
	errMatchGone = errors.New("room: partida encerrada fora da sala")
)

const (
	storeTimeout = 5 * time.Second
	retryDelay   = time.Second // nova tentativa de ação automática depois de falha de gravação
)

// Restored é uma partida em andamento reconstruída pelo Recover.
type Restored struct {
	MatchID string
	Players [4]replay.PlayerSeat
	State   engine.GameState
	NextPly int
}

// NewRestored recria a sala de uma partida em andamento: em jogo, todos desconectados
// (os prazos de ausência recomeçam agora) e o anfitrião no assento 0.
func NewRestored(o Options, rs Restored) *Room {
	r := New(o)
	now := r.now()
	r.status, r.matchID, r.game, r.nextPly = protocol.RoomPlaying, rs.MatchID, rs.State, rs.NextPly
	for s, p := range rs.Players {
		r.seats[s] = &seatState{userID: p.UserID, name: p.DisplayName, disconnectedAt: now}
	}
	r.syncWindow(now)
	return r
}

func (r *Room) storeCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.ctx, storeTimeout)
}

func (r *Room) maybeStart() {
	if r.status != protocol.RoomWaiting || r.members() < 4 {
		return
	}
	for _, st := range r.seats {
		if !st.ready {
			return
		}
	}
	r.startMatch()
}

func (r *Room) startMatch() {
	var buf [9]byte
	if _, err := io.ReadFull(r.opt.Rand, buf[:]); err != nil {
		r.opt.Log.Error("falha ao sortear a partida", "err", err)
		r.failStart("")
		return
	}
	seed, dealer := binary.LittleEndian.Uint64(buf[:8]), engine.Seat(buf[8]%4)
	var players [4]replay.PlayerSeat
	var enginePlayers [4]engine.Player
	for s, st := range r.seats {
		players[s] = replay.PlayerSeat{UserID: st.userID, DisplayName: st.name}
		enginePlayers[s] = engine.Player{ID: st.userID, Name: st.name}
	}
	ctx, cancel := r.storeCtx()
	id, err := r.opt.Store.Create(ctx, seed, dealer, players)
	cancel()
	if err != nil {
		r.opt.Log.Error("falha ao criar a partida", "err", err)
		r.failStart("")
		return
	}
	r.status, r.matchID, r.game, r.nextPly = protocol.RoomPlaying, id, engine.NewGame(enginePlayers, seed, dealer), 0
	for _, st := range r.seats {
		st.timeouts, st.absentSince, st.absentTotal = 0, time.Time{}, 0
	}
	r.opt.Log.Info("partida iniciada", "match_id", id)
	r.publish()
	if err := r.apply(engine.Action{Kind: engine.ActSysStartHand}, replay.SourceSystem); err != nil && !errors.Is(err, errMatchGone) {
		r.failStart(id)
	}
}

// failStart desfaz um início malsucedido: abandona a partida já criada e volta a esperar.
func (r *Room) failStart(matchID string) {
	if matchID != "" {
		ctx, cancel := r.storeCtx()
		if err := r.opt.Store.Abandon(ctx, matchID, "start_failed"); err != nil {
			r.opt.Log.Error("falha ao abandonar partida que não começou", "match_id", matchID, "err", err)
		}
		cancel()
	}
	r.resetToWaiting()
	r.broadcast(protocol.ErrorMsg(protocol.CodeInternal, 0))
}

// apply aplica, grava e só então transmite. Em erro, o estado não muda.
func (r *Room) apply(a engine.Action, src replay.Source) error {
	next, evs, err := engine.Apply(r.game, a)
	if err != nil {
		return err
	}
	ctx, cancel := r.storeCtx()
	err = r.opt.Store.AppendAction(ctx, r.matchID, r.nextPly, a, src)
	cancel()
	if errors.Is(err, ErrMatchClosed) {
		r.opt.Log.Warn("partida encerrada fora da sala", "match_id", r.matchID)
		r.endMatch(protocol.EndAborted, nil)
		return errMatchGone
	}
	if err != nil {
		r.opt.Log.Error("falha ao gravar ação", "match_id", r.matchID, "ply", r.nextPly, "err", err)
		return errPersist
	}
	scoreChanged := next.Score != r.game.Score
	r.game, r.nextPly = next, r.nextPly+1
	r.syncWindow(r.now())
	r.broadcastGame(evs)
	if scoreChanged {
		r.opt.Registry.Update(r.summary())
	}
	if next.Phase == engine.PhaseGameOver {
		r.finish(a)
	}
	return nil
}

func (r *Room) stateFor(seat engine.Seat) protocol.ServerMessage {
	return protocol.StateMessage(r.game, seat, r.deadlines)
}

func (r *Room) broadcastGame(evs []engine.Event) {
	for s, st := range r.seats {
		if st == nil {
			continue
		}
		seat := engine.Seat(s)
		if msg, ok := protocol.EventsMessage(evs, seat); ok {
			r.opt.Out.Send(st.userID, msg)
		}
		r.opt.Out.Send(st.userID, r.stateFor(seat))
	}
}

func (r *Room) broadcastState() {
	for s, st := range r.seats {
		if st != nil {
			r.opt.Out.Send(st.userID, r.stateFor(engine.Seat(s)))
		}
	}
}

// finish grava o resultado. Falha de Finish não trava a sala: o Recover conclui no próximo boot.
func (r *Room) finish(last engine.Action) {
	ctx, cancel := r.storeCtx()
	if err := r.opt.Store.Finish(ctx, r.matchID); err != nil {
		r.opt.Log.Error("falha ao finalizar a partida; o Recover conclui no próximo boot", "match_id", r.matchID, "err", err)
	}
	cancel()
	reason := protocol.EndGameOver
	if last.Kind == engine.ActSysForfeit {
		reason = protocol.EndForfeit
	}
	r.endMatch(reason, r.game.Winner)
}

func (r *Room) endMatch(reason protocol.EndReason, winner *engine.Team) {
	r.opt.Log.Info("partida encerrada", "match_id", r.matchID, "reason", reason)
	r.broadcast(protocol.MatchEnded(reason, winner, r.game.Score))
	r.resetToWaiting()
}

// resetToWaiting volta a sala para a espera com os mesmos ocupantes (revanche).
func (r *Room) resetToWaiting() {
	now := r.now()
	r.status, r.matchID, r.game, r.nextPly = protocol.RoomWaiting, "", engine.GameState{}, 0
	r.window, r.deadlines, r.pauseUntil, r.offered = "", nil, time.Time{}, nil
	for _, st := range r.seats {
		if st == nil {
			continue
		}
		st.ready, st.timeouts, st.absentSince, st.absentTotal = false, 0, time.Time{}, 0
		if !st.connected {
			st.disconnectedAt = now
		}
	}
	r.publish()
}

func (r *Room) abort(reason string) {
	if r.status != protocol.RoomPlaying {
		return
	}
	r.opt.Log.Info("partida abortada", "match_id", r.matchID, "reason", reason)
	r.endMatch(protocol.EndAborted, nil)
}

func (r *Room) playerAction(seat engine.Seat, msg protocol.ClientMessage) {
	user := r.seats[seat].userID
	if r.status != protocol.RoomPlaying {
		r.reply(user, protocol.CodeBadRequest, msg.Seq)
		return
	}
	a, err := protocol.DecodeAction(msg, seat)
	if err != nil {
		r.reply(user, protocol.CodeBadRequest, msg.Seq)
		return
	}
	err = r.apply(a, replay.SourcePlayer)
	switch {
	case err == nil:
		if st := r.seats[seat]; st != nil {
			st.timeouts = 0
		}
	case errors.Is(err, errPersist):
		r.reply(user, protocol.CodeInternal, msg.Seq)
	case errors.Is(err, errMatchGone):
	default:
		r.opt.Out.Send(user, protocol.ErrorMessage(err, msg.Seq))
	}
}
```

- [ ] **Step 5: Ligar à sala**

Em `backend/internal/room/room.go`, no `switch` de `Handle`, antes de `case Stop:`:

```go
	case Abort:
		r.abort(m.Reason)
```

Em `backend/internal/room/seats.go`:
- no `switch` de `command`, troque o `default` por `default: r.playerAction(seat, m.Message)`;
- em `setReady`, depois de `r.publish()`, acrescente `r.maybeStart()`;
- substitua `connected` por:

```go
// connected marca o assento como conectado (nova conexão ou outra aba) e reenvia o que o cliente precisa.
func (r *Room) connected(userID string) {
	seat, ok := r.seatOf(userID)
	if !ok {
		return
	}
	st := r.seats[seat]
	st.connected, st.disconnectedAt, st.timeouts = true, time.Time{}, 0
	r.publish()
	if r.status != protocol.RoomPlaying {
		return
	}
	r.opt.Out.Send(userID, r.stateFor(seat))
	if r.offered != nil && engine.TeamOf(seat) != *r.offered {
		r.opt.Out.Send(userID, protocol.ForfeitAvailable(r.offered))
	}
}
```

- [ ] **Step 6: Rodar e ver passar**

Run: `cd backend && gofmt -l internal/room && go vet ./internal/room/ && go test ./internal/room/`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/room
git commit -m "feat(room): início da partida, jogadas gravadas antes do envio, fim, abort e restauração"
```

---

### Task 11: Prazos de decisão, timeouts e pausa entre mãos

**Files:**
- Modify: `backend/internal/room/timers.go`
- Test: `backend/internal/room/timers_test.go`

**Interfaces:**
- Consumes: `apply`, `syncWindow`, `deadlines`, `pauseUntil`, `retryDelay` (Tarefa 10); `Options.Post`, `Options.Clock`.
- Produces: `settle()` completo; `runDue(now) bool` (aplica **uma** ação vencida; true = algo mudou); `reschedule(now)`; `wakeups(consider func(time.Time))` (a Tarefa 12 a estende). Um único timer por sala, com geração; disparos antigos são ignorados em `Handle`.

- [ ] **Step 1: Escrever os testes que falham**

`backend/internal/room/timers_test.go`:

```go
package room_test

import (
	"errors"
	"testing"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
)

func TestTimeoutAppliesDefaultActionWithTimeoutSource(t *testing.T) {
	h := newHarness(t)
	h.startMatch()
	cutter := h.room.Game().Cutter()
	state := last[protocol.StatePayload](t, h.out, "u0", protocol.MsgState)
	if state.Deadlines[cutter] != t0.Add(testTiming.Choice).UnixMilli() {
		t.Fatalf("prazo = %d", state.Deadlines[cutter])
	}
	h.advance(testTiming.Choice - time.Second)
	if len(h.store.last().Actions) != 1 {
		t.Fatal("disparou antes do prazo")
	}
	h.advance(time.Second)
	got := h.store.lastAction()
	if got.Source != replay.SourceTimeout || got.Action.Kind != engine.ActChooseCard || got.Action.Seat != cutter || got.Action.Keep != 0 {
		t.Fatalf("timeout gravou %+v", got)
	}
}

func TestSimultaneousVoteWindowDoesNotRearm(t *testing.T) {
	h := newHarness(t)
	h.startMatch()
	h.playUntil(func(g engine.GameState) bool { return g.Phase == engine.PhasePlaying && g.Hand.Kind == engine.HandNormal })
	turn := h.room.Game().Hand.Turn
	h.act(engine.Action{Kind: engine.ActRequestRaise, Seat: turn})
	first, second := engine.Next(turn), engine.Partner(engine.Next(turn))
	opened := h.clock.Now()
	h.advance(10 * time.Second)
	h.act(engine.Action{Kind: engine.ActVote, Seat: first, Choice: engine.VoteAccept})
	state := last[protocol.StatePayload](t, h.out, users[second], protocol.MsgState)
	if state.Deadlines[second] != opened.Add(testTiming.Vote).UnixMilli() {
		t.Fatal("o voto do parceiro não pode rearmar o prazo do outro votante")
	}
	h.advance(10 * time.Second)
	got := h.store.lastAction()
	if got.Source != replay.SourceTimeout || got.Action.Seat != second || got.Action.Choice != engine.VoteRun {
		t.Fatalf("timeout do segundo votante = %+v", got)
	}
}

func TestHandPauseStartsNextHand(t *testing.T) {
	h := newHarness(t)
	h.startMatch()
	h.playUntil(func(g engine.GameState) bool { return g.Phase == engine.PhaseHandEnd })
	hand := h.room.Game().HandNumber
	h.advance(testTiming.HandPause - time.Second)
	if h.room.Game().HandNumber != hand {
		t.Fatal("nova mão antes do fim da pausa")
	}
	h.advance(time.Second)
	got := h.store.lastAction()
	if h.room.Game().HandNumber != hand+1 || got.Action.Kind != engine.ActSysStartHand || got.Source != replay.SourceSystem {
		t.Fatalf("mão %d, última ação %+v", h.room.Game().HandNumber, got)
	}
}

func TestFullMatchFinishesAndAllowsRematch(t *testing.T) {
	h := newHarness(t)
	h.startMatch()
	first := h.store.last().ID
	h.playUntil(func(engine.GameState) bool { return false })
	if h.store.records[first].Status != replay.StatusFinished {
		t.Fatalf("status = %s", h.store.records[first].Status)
	}
	for _, u := range users {
		if e := last[protocol.MatchEndedPayload](t, h.out, u, protocol.MsgMatchEnded); e.Reason != protocol.EndGameOver || e.Winner == nil {
			t.Fatalf("%s: match_ended = %+v", u, e)
		}
	}
	if h.room.Status() != protocol.RoomWaiting {
		t.Fatal("fim da partida volta a sala para a espera")
	}
	h.readyAll()
	if h.room.Status() != protocol.RoomPlaying || h.store.last().ID == first {
		t.Fatal("revanche deve criar uma partida nova")
	}
}

func TestTimeoutPersistFailureRetries(t *testing.T) {
	h := newHarness(t)
	h.startMatch()
	h.store.failAppend = errors.New("banco fora")
	h.advance(testTiming.Choice)
	if len(h.store.last().Actions) != 1 {
		t.Fatal("nada deveria ser gravado com o banco fora")
	}
	h.store.failAppend = nil
	h.advance(time.Second)
	if len(h.store.last().Actions) != 2 {
		t.Fatal("a ação automática deve ser tentada de novo após 1s")
	}
}

func TestFinishFailureStillReturnsToWaiting(t *testing.T) {
	h := newHarness(t)
	h.store.failFinish = errors.New("banco fora")
	h.startMatch()
	id := h.store.last().ID
	h.playUntil(func(engine.GameState) bool { return false })
	if h.room.Status() != protocol.RoomWaiting || h.store.records[id].Status != replay.StatusInProgress {
		t.Fatal("falha de Finish não trava a sala; a partida fica para o Recover")
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/room/ -run 'Timeout|Simultaneous|HandPause|FullMatch|FinishFailure'`
Expected: FAIL — nenhum timeout é aplicado (o `settle` ainda é vazio) e `playUntil` estoura em `hand_end`.

- [ ] **Step 3: Implementar**

Substitua `backend/internal/room/timers.go` por:

```go
package room

import (
	"errors"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
)

// settle executa o que já venceu (uma ação por volta, reavaliando a cada uma) e arma
// um único timer para o próximo vencimento futuro.
func (r *Room) settle() {
	for !r.closed {
		now := r.now()
		if r.status == protocol.RoomPlaying && r.runDue(now) {
			continue
		}
		r.reschedule(now)
		return
	}
}

// runDue aplica a próxima ação automática vencida: início da mão após a pausa ou
// DefaultAction de um assento cujo prazo acabou. Devolve true se aplicou algo.
func (r *Room) runDue(now time.Time) bool {
	if r.game.Phase == engine.PhaseHandEnd {
		if now.Before(r.pauseUntil) {
			return false
		}
		err := r.apply(engine.Action{Kind: engine.ActSysStartHand}, replay.SourceSystem)
		if errors.Is(err, errPersist) {
			r.pauseUntil = now.Add(retryDelay)
		}
		return err == nil
	}
	for s := engine.Seat(0); s < 4; s++ {
		deadline, ok := r.deadlines[s]
		if !ok || now.Before(deadline) {
			continue
		}
		a, ok := engine.DefaultAction(r.game, s)
		if !ok {
			delete(r.deadlines, s)
			continue
		}
		r.opt.Log.Warn("timeout aplicado", "match_id", r.matchID, "seat", s, "action", a.Kind)
		err := r.apply(a, replay.SourceTimeout)
		switch {
		case errors.Is(err, errPersist):
			r.deadlines[s] = now.Add(retryDelay)
			return false
		case errors.Is(err, errMatchGone):
			return false
		case err != nil:
			r.opt.Log.Error("ação padrão rejeitada pelo engine", "match_id", r.matchID, "seat", s, "err", err)
			delete(r.deadlines, s)
			return false
		}
		if st := r.seats[s]; st != nil && r.status == protocol.RoomPlaying {
			st.timeouts++
		}
		return true
	}
	return false
}

func (r *Room) reschedule(now time.Time) {
	r.stopTimer()
	at, ok := r.nextWakeup(now)
	if !ok {
		return
	}
	r.gen++
	gen := r.gen
	r.timer = r.opt.Clock.AfterFunc(at.Sub(now), func() { r.opt.Post(tick{gen: gen}) })
}

// nextWakeup é o vencimento futuro mais próximo; o que já venceu foi tratado em settle.
func (r *Room) nextWakeup(now time.Time) (time.Time, bool) {
	var best time.Time
	found := false
	r.wakeups(func(t time.Time) {
		if t.After(now) && (!found || t.Before(best)) {
			best, found = t, true
		}
	})
	return best, found
}

// wakeups lista os instantes em que algo da sala vence.
func (r *Room) wakeups(consider func(time.Time)) {
	if r.status != protocol.RoomPlaying {
		return
	}
	if r.game.Phase == engine.PhaseHandEnd {
		consider(r.pauseUntil)
	}
	for _, d := range r.deadlines {
		consider(d)
	}
}

func (r *Room) stopTimer() {
	if r.timer != nil {
		r.timer.Stop()
		r.timer = nil
	}
}
```

- [ ] **Step 4: Rodar e ver passar**

Run: `cd backend && go vet ./internal/room/ && go test ./internal/room/`
Expected: PASS (todos os testes das Tarefas 9–11).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/room
git commit -m "feat(room): prazos por fase, timeouts gravados como ação concreta e pausa entre mãos"
```

---

### Task 12: Presença, ausência, W.O. e abandono

**Files:**
- Create: `backend/internal/room/presence.go`
- Modify: `backend/internal/room/timers.go` (`settle`, `wakeups`), `backend/internal/room/seats.go` (`command`)
- Test: `backend/internal/room/presence_test.go`

**Interfaces:**
- Consumes: Tarefas 9–11.
- Produces: `updateAbsence(now)`, `forfeitAt(st)`, `forfeitTarget(now)`, `offerForfeit(now)`, `claimForfeit(seat, seq)`, `allAbsentAt() (time.Time, bool)`, `abandonAllAbsent()`, `removeStaleDisconnected(now) bool`. Mensagem `claim_forfeit` tratada em `command`.

- [ ] **Step 1: Escrever os testes que falham**

`backend/internal/room/presence_test.go`:

```go
package room_test

import (
	"slices"
	"testing"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/room"
)

// shortAbsence encurta carência e orçamento para os testes de W.O.
func shortAbsence(o *room.Options) {
	o.Timing.AbsenceGrace = 20 * time.Second
	o.Timing.AbsenceBudget = 30 * time.Second
}

// runFor simula d de jogo: assentos online jogam na hora; os da lista offline só andam por timeout.
func (h *harness) runFor(d time.Duration, offline ...engine.Seat) {
	end := h.clock.Now().Add(d)
	for h.clock.Now().Before(end) && h.room.Status() == protocol.RoomPlaying && !h.room.Closed() {
		if a, ok := nextMove(h.room.Game()); ok && !slices.Contains(offline, a.Seat) {
			h.act(a)
			continue
		}
		h.advance(time.Second)
	}
}

func (h *harness) seatTimeouts(seat engine.Seat) int {
	n := 0
	for _, l := range h.store.last().Actions {
		if l.Source == replay.SourceTimeout && l.Action.Seat == seat {
			n++
		}
	}
	return n
}

func TestThreeTimeoutsMarkSeatAbsentUntilItActs(t *testing.T) {
	h := newHarness(t)
	h.startMatch()
	for i := 0; h.seatTimeouts(1) < 3; i++ {
		if i > 2000 {
			t.Fatal("assento 1 não acumulou 3 timeouts")
		}
		h.runFor(time.Second, 1)
	}
	if !last[protocol.RoomStatePayload](t, h.out, "u0", protocol.MsgRoomState).Seats[1].Absent {
		t.Fatal("3 timeouts seguidos marcam ausência")
	}
	h.playUntil(func(g engine.GameState) bool { _, ok := engine.DefaultAction(g, 1); return ok })
	a, _ := engine.DefaultAction(h.room.Game(), 1)
	a.Covered = false
	h.act(a)
	if last[protocol.RoomStatePayload](t, h.out, "u0", protocol.MsgRoomState).Seats[1].Absent {
		t.Fatal("uma jogada própria encerra a ausência")
	}
}

func TestDisconnectShortensPendingDeadline(t *testing.T) {
	h := newHarness(t)
	h.startMatch()
	cutter := h.room.Game().Cutter()
	h.advance(2 * time.Second)
	h.send(room.Disconnected{UserID: users[cutter]})
	state := last[protocol.StatePayload](t, h.out, users[engine.Next(cutter)], protocol.MsgState)
	if state.Deadlines[cutter] != h.clock.Now().Add(testTiming.Absent).UnixMilli() {
		t.Fatal("o prazo de quem cai deve encurtar para TIMER_ABSENT")
	}
	h.advance(testTiming.Absent)
	if got := h.store.lastAction(); got.Source != replay.SourceTimeout || got.Action.Seat != cutter {
		t.Fatalf("última ação = %+v", got)
	}
}

func TestForfeitOfferedAfterGraceAndClaimEndsMatch(t *testing.T) {
	h := newHarness(t, shortAbsence)
	h.startMatch()
	h.send(room.Disconnected{UserID: "u1"})
	h.runFor(19*time.Second, 1)
	if h.out.count("u0", protocol.MsgForfeitAvailable) != 0 {
		t.Fatal("W.O. antes da carência")
	}
	h.runFor(time.Second, 1)
	if offer := last[protocol.ForfeitAvailablePayload](t, h.out, "u0", protocol.MsgForfeitAvailable); offer.Team == nil || *offer.Team != 1 {
		t.Fatalf("oferta = %+v", offer)
	}
	if h.out.count("u2", protocol.MsgForfeitAvailable) != 1 || h.out.count("u3", protocol.MsgForfeitAvailable) != 0 {
		t.Fatal("só a dupla adversária do ausente recebe a oferta")
	}
	h.cmd("u2", protocol.TypeClaimForfeit, nil)
	got := h.store.lastAction()
	if got.Action.Kind != engine.ActSysForfeit || got.Action.Team != 1 || got.Source != replay.SourceSystem {
		t.Fatalf("ação gravada = %+v", got)
	}
	if h.store.last().Status != replay.StatusFinished {
		t.Fatal("W.O. finaliza a partida")
	}
	if e := last[protocol.MatchEndedPayload](t, h.out, "u0", protocol.MsgMatchEnded); e.Reason != protocol.EndForfeit || *e.Winner != 0 {
		t.Fatalf("match_ended = %+v", e)
	}
	h.cmd("u0", protocol.TypeClaimForfeit, nil)
	if lastCode(t, h.out, "u0") != protocol.CodeForfeitUnavailable {
		t.Fatal("segundo pedido de W.O. deve ser recusado")
	}
}

func TestForfeitWithdrawnWhenAbsentReturns(t *testing.T) {
	h := newHarness(t, shortAbsence)
	h.startMatch()
	h.send(room.Disconnected{UserID: "u1"})
	h.runFor(20*time.Second, 1)
	h.send(room.Connected{UserID: "u1"})
	if offer := last[protocol.ForfeitAvailablePayload](t, h.out, "u0", protocol.MsgForfeitAvailable); offer.Team != nil {
		t.Fatal("a volta do ausente retira a oferta")
	}
	h.cmd("u0", protocol.TypeClaimForfeit, nil)
	if lastCode(t, h.out, "u0") != protocol.CodeForfeitUnavailable {
		t.Fatal("W.O. retirado não pode ser pedido")
	}
}

func TestForfeitBudgetAccumulatesAcrossDisconnections(t *testing.T) {
	h := newHarness(t, shortAbsence) // carência 20s, orçamento 30s
	h.startMatch()
	h.send(room.Disconnected{UserID: "u1"})
	h.runFor(15*time.Second, 1)
	h.send(room.Connected{UserID: "u1"})
	h.runFor(5 * time.Second)
	h.send(room.Disconnected{UserID: "u1"})
	h.runFor(14*time.Second, 1)
	if h.out.count("u0", protocol.MsgForfeitAvailable) != 0 {
		t.Fatal("29s acumulados ainda não estouram o orçamento de 30s")
	}
	h.runFor(time.Second, 1)
	if offer := last[protocol.ForfeitAvailablePayload](t, h.out, "u0", protocol.MsgForfeitAvailable); offer.Team == nil || *offer.Team != 1 {
		t.Fatal("o orçamento acumulado libera o W.O. antes da carência contínua")
	}
}

func TestNoForfeitWhenBothTeamsAreAbsent(t *testing.T) {
	h := newHarness(t, shortAbsence)
	h.startMatch()
	h.send(room.Disconnected{UserID: "u1"})
	h.send(room.Disconnected{UserID: "u0"})
	h.runFor(25*time.Second, 0, 1)
	if h.out.count("u2", protocol.MsgForfeitAvailable)+h.out.count("u3", protocol.MsgForfeitAvailable) != 0 {
		t.Fatal("com ausentes nas duas duplas não há W.O.")
	}
	h.cmd("u2", protocol.TypeClaimForfeit, nil)
	if lastCode(t, h.out, "u2") != protocol.CodeForfeitUnavailable {
		t.Fatal("pedido sem oferta é recusado")
	}
}

func TestReconnectResendsForfeitOffer(t *testing.T) {
	h := newHarness(t, shortAbsence)
	h.startMatch()
	h.send(room.Disconnected{UserID: "u1"})
	h.runFor(20*time.Second, 1)
	h.out.reset()
	h.send(room.Connected{UserID: "u0"}) // outra aba do mesmo usuário
	if offer := last[protocol.ForfeitAvailablePayload](t, h.out, "u0", protocol.MsgForfeitAvailable); offer.Team == nil || *offer.Team != 1 {
		t.Fatal("nova conexão deve receber a oferta vigente")
	}
}

func TestAllAbsentAbandonsAndDestroysRoom(t *testing.T) {
	h := newHarness(t, shortAbsence)
	h.startMatch()
	for _, u := range users {
		h.send(room.Disconnected{UserID: u})
	}
	h.advance(29 * time.Second)
	if h.room.Closed() {
		t.Fatal("abandono antes do orçamento")
	}
	h.advance(time.Second)
	id := h.store.last().ID
	if h.store.abandoned[id] != "all_absent" || h.reg.destroyed != 1 || !h.room.Closed() {
		t.Fatalf("abandono %q, destruída %d", h.store.abandoned[id], h.reg.destroyed)
	}
	for _, u := range users {
		if !h.reg.wasReleased(u) {
			t.Fatalf("%s deveria ser liberado", u)
		}
	}
}

func TestWaitingDisconnectRemovesSeatAfterGrace(t *testing.T) {
	h := newHarness(t)
	h.join(0)
	h.join(1)
	h.send(room.Disconnected{UserID: "u1"})
	h.advance(59 * time.Second)
	if last[protocol.RoomStatePayload](t, h.out, "u0", protocol.MsgRoomState).Seats[1] == nil {
		t.Fatal("removido antes do prazo")
	}
	h.advance(time.Second)
	if last[protocol.RoomStatePayload](t, h.out, "u0", protocol.MsgRoomState).Seats[1] != nil || !h.reg.wasReleased("u1") {
		t.Fatal("desconectado em espera é removido após WAITING_DISCONNECT")
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/room/ -run 'Absent|Forfeit|Shortens|Waiting'`
Expected: FAIL — ausência nunca é marcada e `claim_forfeit` responde `bad_request`.

- [ ] **Step 3: Implementar a presença**

`backend/internal/room/presence.go`:

```go
package room

import (
	"errors"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
)

// updateAbsence registra as transições de ausência e encurta o prazo de quem acabou de sumir.
func (r *Room) updateAbsence(now time.Time) {
	if r.status != protocol.RoomPlaying {
		return
	}
	changed, shortened := false, false
	for s, st := range r.seats {
		if st == nil {
			continue
		}
		seat := engine.Seat(s)
		was := !st.absentSince.IsZero()
		switch is := st.isAbsent(); {
		case is && !was:
			st.absentSince, changed = now, true
			if d, ok := r.deadlines[seat]; ok {
				if limit := now.Add(r.opt.Timing.Absent); limit.Before(d) {
					r.deadlines[seat], shortened = limit, true
				}
			}
		case !is && was:
			st.absentTotal += now.Sub(st.absentSince)
			st.absentSince, changed = time.Time{}, true
		}
	}
	if changed {
		r.publish()
	}
	if shortened {
		r.broadcastState()
	}
}

// forfeitAt é quando a ausência corrente do assento libera o W.O.: carência contínua
// ou orçamento acumulado na partida, o que vier antes.
func (r *Room) forfeitAt(st *seatState) time.Time {
	byGrace := st.absentSince.Add(r.opt.Timing.AbsenceGrace)
	byBudget := st.absentSince.Add(r.opt.Timing.AbsenceBudget - st.absentTotal)
	if byBudget.Before(byGrace) {
		return byBudget
	}
	return byGrace
}

// forfeitTarget é a dupla contra a qual o W.O. está disponível agora, se houver.
func (r *Room) forfeitTarget(now time.Time) *engine.Team {
	if r.status != protocol.RoomPlaying {
		return nil
	}
	var absentTeam [2]bool
	for s, st := range r.seats {
		if st != nil && !st.absentSince.IsZero() {
			absentTeam[engine.TeamOf(engine.Seat(s))] = true
		}
	}
	for s, st := range r.seats {
		if st == nil || st.absentSince.IsZero() {
			continue
		}
		team := engine.TeamOf(engine.Seat(s))
		if absentTeam[engine.Other(team)] {
			continue
		}
		if !now.Before(r.forfeitAt(st)) {
			return &team
		}
	}
	return nil
}

func sameTeam(a, b *engine.Team) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func (r *Room) sendToTeam(team engine.Team, msg protocol.ServerMessage) {
	for s, st := range r.seats {
		if st != nil && engine.TeamOf(engine.Seat(s)) == team {
			r.opt.Out.Send(st.userID, msg)
		}
	}
}

// offerForfeit avisa a dupla adversária quando o W.O. passa a valer ou deixa de valer.
func (r *Room) offerForfeit(now time.Time) {
	target := r.forfeitTarget(now)
	if sameTeam(target, r.offered) {
		return
	}
	previous := r.offered
	r.offered = target
	if previous != nil {
		r.sendToTeam(engine.Other(*previous), protocol.ForfeitAvailable(nil))
	}
	if target != nil {
		r.opt.Log.Info("W.O. disponível", "match_id", r.matchID, "team", *target)
		r.sendToTeam(engine.Other(*target), protocol.ForfeitAvailable(target))
	}
}

func (r *Room) claimForfeit(seat engine.Seat, seq int64) {
	user := r.seats[seat].userID
	if r.status != protocol.RoomPlaying || r.offered == nil || engine.TeamOf(seat) == *r.offered {
		r.reply(user, protocol.CodeForfeitUnavailable, seq)
		return
	}
	loser := *r.offered
	r.opt.Log.Info("W.O. pedido", "match_id", r.matchID, "loser", loser, "seat", seat)
	err := r.apply(engine.Action{Kind: engine.ActSysForfeit, Team: loser}, replay.SourceSystem)
	switch {
	case err == nil, errors.Is(err, errMatchGone):
	case errors.Is(err, errPersist):
		r.reply(user, protocol.CodeInternal, seq)
	default:
		r.opt.Out.Send(user, protocol.ErrorMessage(err, seq))
	}
}

// allAbsentAt é quando a partida deve ser abandonada por estarem todos ausentes.
func (r *Room) allAbsentAt() (time.Time, bool) {
	if r.status != protocol.RoomPlaying {
		return time.Time{}, false
	}
	var latest time.Time
	for _, st := range r.seats {
		if st == nil || st.absentSince.IsZero() {
			return time.Time{}, false
		}
		if st.absentSince.After(latest) {
			latest = st.absentSince
		}
	}
	return latest.Add(r.opt.Timing.AbsenceBudget), true
}

func (r *Room) abandonAllAbsent() {
	ctx, cancel := r.storeCtx()
	if err := r.opt.Store.Abandon(ctx, r.matchID, "all_absent"); err != nil && !errors.Is(err, ErrMatchClosed) {
		r.opt.Log.Error("falha ao abandonar partida sem jogadores", "match_id", r.matchID, "err", err)
	}
	cancel()
	r.opt.Log.Info("partida abandonada: todos ausentes", "match_id", r.matchID)
	ended := protocol.MatchEnded(protocol.EndAbandoned, nil, r.game.Score)
	for s, st := range r.seats {
		if st == nil {
			continue
		}
		r.opt.Out.Send(st.userID, ended)
		r.opt.Out.Send(st.userID, protocol.LeftRoom(protocol.LeftDestroyed))
		r.opt.Registry.Released(r.opt.ID, st.userID)
		r.seats[s] = nil
	}
	r.destroy()
}

// removeStaleDisconnected tira, em espera, quem está desconectado há WAITING_DISCONNECT.
func (r *Room) removeStaleDisconnected(now time.Time) bool {
	for s, st := range r.seats {
		if st != nil && !st.connected && !now.Before(st.disconnectedAt.Add(r.opt.Timing.WaitingDisconnect)) {
			r.removeSeat(engine.Seat(s), protocol.LeftLeft)
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Ligar ao ciclo de timers e aos comandos**

Em `backend/internal/room/timers.go`, substitua `settle` e `wakeups` por:

```go
// settle registra ausências, executa o que já venceu (uma ação por volta, reavaliando a cada uma),
// atualiza a oferta de W.O. e arma um único timer para o próximo vencimento futuro.
func (r *Room) settle() {
	for !r.closed {
		now := r.now()
		r.updateAbsence(now)
		switch r.status {
		case protocol.RoomWaiting:
			if r.removeStaleDisconnected(now) {
				continue
			}
		case protocol.RoomPlaying:
			if at, all := r.allAbsentAt(); all && !now.Before(at) {
				r.abandonAllAbsent()
				return
			}
			if r.runDue(now) {
				continue
			}
		}
		r.offerForfeit(now)
		r.reschedule(now)
		return
	}
}
```

```go
// wakeups lista os instantes em que algo da sala vence.
func (r *Room) wakeups(consider func(time.Time)) {
	switch r.status {
	case protocol.RoomWaiting:
		for _, st := range r.seats {
			if st != nil && !st.connected {
				consider(st.disconnectedAt.Add(r.opt.Timing.WaitingDisconnect))
			}
		}
	case protocol.RoomPlaying:
		if r.game.Phase == engine.PhaseHandEnd {
			consider(r.pauseUntil)
		}
		for _, d := range r.deadlines {
			consider(d)
		}
		for _, st := range r.seats {
			if st != nil && !st.absentSince.IsZero() {
				consider(r.forfeitAt(st))
			}
		}
		if at, all := r.allAbsentAt(); all {
			consider(at)
		}
	}
}
```

Em `backend/internal/room/seats.go`, no `switch` de `command`, antes do `default`:

```go
	case protocol.TypeClaimForfeit:
		r.claimForfeit(seat, m.Message.Seq)
```

- [ ] **Step 5: Rodar e ver passar**

Run: `cd backend && gofmt -l internal/room && go vet ./internal/room/ && go test ./internal/room/`
Expected: PASS (Tarefas 9–12).

- [ ] **Step 6: Commit**

```bash
git add backend/internal/room
git commit -m "feat(room): ausência por queda ou timeouts, W.O. sob demanda e abandono quando todos somem"
```

---

### Task 13: Sinais rápidos, ator e teste de propriedade

**Files:**
- Create: `backend/internal/room/signals.go`, `backend/internal/room/actor.go`
- Modify: `backend/internal/room/seats.go` (`command`)
- Test: `backend/internal/room/signals_test.go`, `backend/internal/room/actor_test.go`, `backend/internal/room/property_test.go`

**Interfaces:**
- Produces:
  - `type Actor struct{...}`; `func Start(ctx context.Context, o Options, restored *Restored) *Actor` (define `o.Post`, cria a sala — restaurada se `restored != nil` — e empurra `Wake`); `func (a *Actor) Push(m Msg) bool` (nunca bloqueia; false depois que o ator fechou); `func (a *Actor) Done() <-chan struct{}`.
  - Ao fechar a sala (destruída ou `Stop`), o ator fecha a caixa, responde o que restou (`Join` → `room_not_found` + `Released`) e termina.

- [ ] **Step 1: Escrever os testes que falham**

`backend/internal/room/signals_test.go`:

```go
package room_test

import (
	"testing"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
)

func TestSignalsBroadcastAndRateLimit(t *testing.T) {
	h := newHarness(t)
	h.startMatch()
	h.cmd("u1", protocol.TypeSignal, protocol.SignalPayload{ID: protocol.SignalTruco})
	for _, u := range users {
		if s := last[protocol.SignalBroadcastPayload](t, h.out, u, protocol.MsgSignal); s.Seat != 1 || s.ID != protocol.SignalTruco {
			t.Fatalf("%s: sinal = %+v", u, s)
		}
	}
	h.cmd("u1", protocol.TypeSignal, protocol.SignalPayload{ID: protocol.SignalBoa})
	if lastCode(t, h.out, "u1") != protocol.CodeRateLimited {
		t.Fatal("dois sinais em menos de 3s")
	}
	h.advance(3 * time.Second)
	h.cmd("u1", protocol.TypeSignal, protocol.SignalPayload{ID: protocol.SignalBoa})
	if h.out.count("u0", protocol.MsgSignal) != 2 {
		t.Fatal("depois de 3s o sinal volta a valer")
	}
	h.cmd("u1", protocol.TypeSignal, map[string]string{"id": "xingar"})
	if lastCode(t, h.out, "u1") != protocol.CodeBadRequest {
		t.Fatal("sinal fora da lista é bad_request")
	}
}

func TestSignalOutsideMatchIsBadRequest(t *testing.T) {
	h := newHarness(t)
	h.join(0)
	h.cmd("u0", protocol.TypeSignal, protocol.SignalPayload{ID: protocol.SignalBoa})
	if lastCode(t, h.out, "u0") != protocol.CodeBadRequest {
		t.Fatal("sinais só durante a partida")
	}
}
```

`backend/internal/room/actor_test.go`:

```go
package room_test

import (
	"context"
	"io"
	"log/slog"
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/clock"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/room"
)

func actorOptions(out room.Outbox, reg *registry) room.Options {
	return room.Options{ID: "r1", Code: "K7QM2P", Name: "Sala de Ana", Public: true, Timing: testTiming,
		Store: newFakeStore(), Out: out, Registry: reg, Clock: clock.NewManual(t0), Rand: rand.NewChaCha8([32]byte{1}),
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func waitDone(t *testing.T, a *room.Actor) {
	t.Helper()
	select {
	case <-a.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("o ator não terminou")
	}
}

func TestActorProcessesInOrderAndStops(t *testing.T) {
	out, reg := newOutbox(), &registry{}
	a := room.Start(context.Background(), actorOptions(out, reg), nil)
	a.Push(room.Join{UserID: "u0", DisplayName: "Ana"})
	a.Push(room.Join{UserID: "u1", DisplayName: "Bia"})
	a.Push(room.Stop{})
	waitDone(t, a)
	if out.count("u1", protocol.MsgRoomState) == 0 {
		t.Fatal("mensagens anteriores ao Stop devem ser processadas")
	}
	if a.Push(room.Wake{}) {
		t.Fatal("Push depois do fim deve devolver false")
	}
	if reg.destroyed != 0 {
		t.Fatal("Stop não destrói a sala (o lobby cuida do desligamento)")
	}
}

func TestActorAnswersJoinsQueuedAfterDestruction(t *testing.T) {
	out, reg := newOutbox(), &registry{}
	gate := &gatedOutbox{inner: out, gate: make(chan struct{})}
	a := room.Start(context.Background(), actorOptions(gate, reg), nil)
	// O primeiro envio (room_state do u0) segura o ator até as outras mensagens estarem na fila.
	a.Push(room.Join{UserID: "u0", DisplayName: "Ana"})
	a.Push(room.Command{UserID: "u0", Message: protocol.ClientMessage{Type: protocol.TypeLeaveRoom, Seq: 1}})
	if !a.Push(room.Join{UserID: "u1", DisplayName: "Bia", Seq: 2}) {
		t.Fatal("o ator ainda não pode ter fechado")
	}
	close(gate.gate)
	waitDone(t, a)
	codes := out.errorCodes("u1")
	if len(codes) != 1 || codes[0] != protocol.CodeRoomNotFound || !reg.wasReleased("u1") || reg.destroyed != 1 {
		t.Fatalf("códigos %v, destruída %d", codes, reg.destroyed)
	}
}

// gatedOutbox bloqueia o primeiro envio até gate fechar.
type gatedOutbox struct {
	inner *outbox
	gate  chan struct{}
	once  sync.Once
}

func (g *gatedOutbox) Send(user string, m protocol.ServerMessage) {
	g.once.Do(func() { <-g.gate })
	g.inner.Send(user, m)
}
```

`backend/internal/room/property_test.go`:

```go
package room_test

import (
	"math/rand/v2"
	"reflect"
	"testing"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/room"
)

// Partidas com jogadas aleatórias (válidas e inválidas), quedas, voltas e passagem de tempo:
// o log gravado sempre reconstrói o estado da sala, cada assento só recebe a própria view
// e nenhum evento invisível a ele.
func TestRandomMatchesStayConsistent(t *testing.T) {
	for seed := uint64(1); seed <= 30; seed++ {
		runRandomMatch(t, seed)
	}
}

func runRandomMatch(t *testing.T, seed uint64) {
	key := [32]byte{byte(seed), byte(seed >> 8)}
	h := newHarness(t, func(o *room.Options) { o.Rand = rand.NewChaCha8(key) })
	rng := rand.New(rand.NewPCG(seed, 99))
	h.startMatch()
	matchID := h.store.last().ID
	online := [4]bool{true, true, true, true}
	for step := 0; h.room.Status() == protocol.RoomPlaying && !h.room.Closed(); step++ {
		if step > 20000 {
			t.Fatalf("seed %d: a partida não terminou", seed)
		}
		switch roll := rng.IntN(100); {
		case roll < 70:
			h.randomCommand(rng)
		case roll < 80:
			i := rng.IntN(4)
			if online[i] {
				h.send(room.Disconnected{UserID: users[i]})
			} else {
				h.send(room.Connected{UserID: users[i]})
			}
			online[i] = !online[i]
		default:
			h.advance(time.Duration(1+rng.IntN(20)) * time.Second)
		}
		h.checkConsistency(t, seed, matchID)
	}
}

func (h *harness) randomCommand(rng *rand.Rand) {
	g := h.room.Game()
	seat := engine.Seat(rng.IntN(4))
	options := []protocol.ClientMessage{
		clientMsg(engine.Action{Kind: engine.ActChooseCard, Seat: seat, Keep: rng.IntN(2)}),
		clientMsg(engine.Action{Kind: engine.ActPassBurn, Seat: seat}),
		clientMsg(engine.Action{Kind: engine.ActRequestRaise, Seat: seat}),
		clientMsg(engine.Action{Kind: engine.ActPlayCard, Seat: seat, Slot: rng.IntN(3), Covered: rng.IntN(4) == 0}),
		{Type: protocol.TypeClaimForfeit, Seq: 1},
	}
	for _, c := range []engine.VoteChoice{engine.VoteAccept, engine.VoteRun, engine.VoteRaise, engine.VotePlay} {
		options = append(options, clientMsg(engine.Action{Kind: engine.ActVote, Seat: seat, Choice: c}))
	}
	if g.Hand != nil {
		var fours []int
		for slot, c := range g.Hand.Hands[seat] {
			if c.Rank == 4 && !g.Hand.Played[seat][slot] {
				fours = append(fours, slot)
			}
		}
		if len(fours) > 0 {
			options = append(options, clientMsg(engine.Action{Kind: engine.ActBurn, Seat: seat, Slots: fours}))
		}
	}
	h.send(room.Command{UserID: users[seat], Message: options[rng.IntN(len(options))]})
}

func (h *harness) checkConsistency(t *testing.T, seed uint64, matchID string) {
	t.Helper()
	if h.room.Closed() || h.room.Status() != protocol.RoomPlaying {
		h.out.reset()
		return
	}
	live := h.room.Game()
	rebuilt, err := replay.Rebuild(*h.store.records[matchID])
	if err != nil || !reflect.DeepEqual(rebuilt, live) {
		t.Fatalf("seed %d: o log não reconstrói o estado da sala (%v)", seed, err)
	}
	for i, u := range users {
		seat := engine.Seat(i)
		var lastView *engine.PlayerView
		for _, m := range h.out.take(u) {
			switch m.Type {
			case protocol.MsgEvents:
				for _, e := range m.Payload.([]engine.Event) {
					if !e.Visibility.Allows(seat) {
						t.Fatalf("seed %d: assento %d recebeu evento invisível %s", seed, i, e.Kind)
					}
				}
			case protocol.MsgState:
				v := m.Payload.(protocol.StatePayload).View
				lastView = &v
			}
		}
		if lastView != nil && !reflect.DeepEqual(*lastView, engine.ViewFor(live, seat)) {
			t.Fatalf("seed %d: assento %d recebeu uma view diferente de ViewFor", seed, i)
		}
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/room/ -run 'Signal|Actor|RandomMatches'`
Expected: FAIL — `undefined: room.Start`; sinais respondem `bad_request` (caem em `playerAction`).

- [ ] **Step 3: Implementar sinais e ator**

`backend/internal/room/signals.go`:

```go
package room

import (
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
)

// signalInterval é o intervalo mínimo entre sinais rápidos de um mesmo jogador.
const signalInterval = 3 * time.Second

func (r *Room) signal(seat engine.Seat, msg protocol.ClientMessage) {
	st := r.seats[seat]
	var p protocol.SignalPayload
	if r.status != protocol.RoomPlaying || protocol.Decode(msg.Payload, &p, "id") != nil || !p.ID.Valid() {
		r.reply(st.userID, protocol.CodeBadRequest, msg.Seq)
		return
	}
	now := r.now()
	if !st.lastSignal.IsZero() && now.Sub(st.lastSignal) < signalInterval {
		r.reply(st.userID, protocol.CodeRateLimited, msg.Seq)
		return
	}
	st.lastSignal = now
	r.broadcast(protocol.SignalBroadcast(seat, p.ID))
}
```

Em `backend/internal/room/seats.go`, no `switch` de `command`, antes do `default`:

```go
	case protocol.TypeSignal:
		r.signal(seat, m.Message)
```

`backend/internal/room/actor.go`:

```go
package room

import (
	"context"
	"sync"
)

// Actor executa uma sala numa goroutine própria. A caixa de entrada não tem limite,
// então Push nunca bloqueia: o lobby pode chamá-lo segurando o próprio mutex.
type Actor struct {
	room   *Room
	mu     sync.Mutex
	queue  []Msg
	closed bool
	signal chan struct{}
	done   chan struct{}
}

// Start cria a sala (restaurada quando restored != nil) e começa a processar a caixa de entrada.
func Start(ctx context.Context, o Options, restored *Restored) *Actor {
	a := &Actor{signal: make(chan struct{}, 1), done: make(chan struct{})}
	o.Post = func(m Msg) { a.Push(m) }
	if restored != nil {
		a.room = NewRestored(o, *restored)
	} else {
		a.room = New(o)
	}
	a.Push(Wake{})
	go a.run(ctx)
	return a
}

// Push entrega a mensagem; devolve false se o ator já terminou.
func (a *Actor) Push(m Msg) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return false
	}
	a.queue = append(a.queue, m)
	select {
	case a.signal <- struct{}{}:
	default:
	}
	return true
}

// Done fecha quando o ator termina.
func (a *Actor) Done() <-chan struct{} { return a.done }

func (a *Actor) run(ctx context.Context) {
	defer close(a.done)
	for {
		select {
		case <-ctx.Done():
			a.closeQueue()
			return
		case <-a.signal:
		}
		for {
			m, ok := a.pop()
			if !ok {
				break
			}
			a.room.Handle(ctx, m)
			if a.room.Closed() {
				for _, rest := range a.closeQueue() {
					a.room.Handle(ctx, rest) // sala fechada responde Join/Command com room_not_found
				}
				return
			}
		}
	}
}

func (a *Actor) pop() (Msg, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.queue) == 0 {
		return nil, false
	}
	m := a.queue[0]
	a.queue = a.queue[1:]
	return m, true
}

// closeQueue impede novos Push e devolve o que ficou na fila.
func (a *Actor) closeQueue() []Msg {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.closed = true
	rest := a.queue
	a.queue = nil
	return rest
}
```

- [ ] **Step 4: Rodar e ver passar**

Run: `cd backend && gofmt -l internal/room && go vet ./internal/room/ && go test -race ./internal/room/`
Expected: PASS. Se o teste de propriedade falhar com "não terminou" para algum seed, investigue (não aumente o limite sem entender: a combinação de jogadas, timeouts e W.O. sempre termina a partida).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/room
git commit -m "feat(room): sinais rápidos, ator com caixa sem limite e teste de propriedade da sala"
```

---
### Task 14: Lobby — registro, lista ao vivo, restore, abort e desligamento

**Files:**
- Create: `backend/internal/lobby/lobby.go`, `backend/internal/lobby/store.go`
- Test: `backend/internal/lobby/lobby_test.go`

**Interfaces:**
- Consumes: `room.Start`, `room.Actor`, `room.Options`, `room.Summary`, `room.Restored`, `room.ErrMatchClosed` (Tarefas 9–13); `match.Recovered`, `match.Store`, `match.ErrMatchClosed`; `protocol.*`.
- Produces:
  ```go
  type CloseReason int // CloseReplaced, CloseShutdown
  type Client interface{ UserID() string; Send(protocol.ServerMessage); Close(CloseReason) } // Send/Close nunca bloqueiam
  type Options struct{ Timing room.Timing; MaxRooms int; Store room.MatchStore; Clock clock.Clock; Rand io.Reader; Log *slog.Logger }
  func New(ctx context.Context, o Options) *Lobby
  func (l *Lobby) Connect(c Client, info protocol.UserInfo) bool   // false durante o desligamento
  func (l *Lobby) Disconnect(c Client)
  func (l *Lobby) Dispatch(c Client, msg protocol.ClientMessage)
  func (l *Lobby) Restore(recovered []match.Recovered) error
  func (l *Lobby) Abort(ctx context.Context, matchID string) error // implementa admin.MatchAborter
  func (l *Lobby) Shutdown(ctx context.Context) error
  // room.Outbox e room.Registry: Send, Update, Released, Destroyed
  func NewMatchStore(s *match.Store) room.MatchStore
  ```
- Regras de concorrência: todo estado sob `l.mu`; envios a clientes e `Actor.Push` acontecem sob o mutex (nenhum dos dois bloqueia), o que preserva a ordem das mensagens por cliente; mensagens de uma conexão substituída são ignoradas.

- [ ] **Step 1: Escrever os testes que falham**

`backend/internal/lobby/lobby_test.go`:

```go
package lobby_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/clock"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/enginetest"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/lobby"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/match"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/room"
)

var timing = room.Timing{Play: 30 * time.Second, Vote: 20 * time.Second, Choice: 15 * time.Second, Burn: 15 * time.Second,
	Absent: 5 * time.Second, HandPause: 4 * time.Second, AbsenceGrace: 2 * time.Minute, AbsenceBudget: 5 * time.Minute,
	WaitingDisconnect: time.Minute}

type fakeClient struct {
	id     string
	mu     sync.Mutex
	msgs   []protocol.ServerMessage
	closed lobby.CloseReason
}

func (c *fakeClient) UserID() string { return c.id }

func (c *fakeClient) Send(m protocol.ServerMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.msgs = append(c.msgs, m)
}

func (c *fakeClient) Close(r lobby.CloseReason) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = r
}

func (c *fakeClient) messages() []protocol.ServerMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]protocol.ServerMessage(nil), c.msgs...)
}

func (c *fakeClient) closeReason() lobby.CloseReason {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

func (c *fakeClient) count(typ protocol.ServerMessageType) int {
	n := 0
	for _, m := range c.messages() {
		if m.Type == typ {
			n++
		}
	}
	return n
}

// waitFor espera (até 2s) uma mensagem do tipo que satisfaça match (nil = qualquer) e a devolve.
func (c *fakeClient) waitFor(t *testing.T, typ protocol.ServerMessageType, match func(protocol.ServerMessage) bool) protocol.ServerMessage {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, m := range c.messages() {
			if m.Type == typ && (match == nil || match(m)) {
				return m
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("%s não recebeu %s", c.id, typ)
	return protocol.ServerMessage{}
}

func errorCode(code string) func(protocol.ServerMessage) bool {
	return func(m protocol.ServerMessage) bool { return m.Payload.(protocol.ErrorPayload).Code == code }
}

func roomState(t *testing.T, c *fakeClient) protocol.RoomStatePayload {
	t.Helper()
	return c.waitFor(t, protocol.MsgRoomState, nil).Payload.(protocol.RoomStatePayload)
}

type nopStore struct {
	mu sync.Mutex
	n  int
}

func (s *nopStore) Create(context.Context, uint64, engine.Seat, [4]replay.PlayerSeat) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.n++
	return fmt.Sprintf("m%d", s.n), nil
}
func (s *nopStore) AppendAction(context.Context, string, int, engine.Action, replay.Source) error { return nil }
func (s *nopStore) Finish(context.Context, string) error                                         { return nil }
func (s *nopStore) Abandon(context.Context, string, string) error                                { return nil }

func newLobby(t *testing.T, maxRooms int) *lobby.Lobby {
	l := lobby.New(context.Background(), lobby.Options{Timing: timing, MaxRooms: maxRooms, Store: &nopStore{},
		Clock: clock.NewManual(time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)), Rand: rand.NewChaCha8([32]byte{2}),
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = l.Shutdown(ctx)
	})
	return l
}

func connect(l *lobby.Lobby, id, name string) *fakeClient {
	c := &fakeClient{id: id}
	l.Connect(c, protocol.UserInfo{ID: id, Username: strings.ToLower(name), DisplayName: name})
	return c
}

func dispatch(l *lobby.Lobby, c *fakeClient, typ string, payload any) {
	msg := protocol.ClientMessage{Type: typ, Seq: 1}
	if payload != nil {
		msg.Payload, _ = json.Marshal(payload)
	}
	l.Dispatch(c, msg)
}

func TestConnectSendsHelloAndSnapshot(t *testing.T) {
	l := newLobby(t, 3)
	c := connect(l, "u0", "Ana")
	msgs := c.messages()
	if len(msgs) != 2 || msgs[0].Type != protocol.MsgHello || msgs[1].Type != protocol.MsgLobbySnapshot {
		t.Fatalf("mensagens = %+v", msgs)
	}
	if hello := msgs[0].Payload.(protocol.HelloPayload); hello.User.ID != "u0" || hello.ProtocolVersion != protocol.ProtocolVersion {
		t.Fatalf("hello = %+v", hello)
	}
}

func TestPublicRoomIsListedAndPrivateIsNot(t *testing.T) {
	l := newLobby(t, 3)
	ana, bia, caio := connect(l, "u0", "Ana"), connect(l, "u1", "Bia"), connect(l, "u2", "Caio")
	dispatch(l, ana, protocol.TypeCreateRoom, protocol.CreateRoomPayload{Visibility: protocol.VisibilityPublic})
	up := bia.waitFor(t, protocol.MsgRoomUpserted, nil).Payload.(protocol.RoomUpsertedPayload)
	if up.Room.Name != "Sala de Ana" || up.Room.Seats[0] == nil || up.Room.Seats[0].DisplayName != "Ana" {
		t.Fatalf("room_upserted = %+v", up)
	}
	dispatch(l, caio, protocol.TypeCreateRoom, protocol.CreateRoomPayload{Visibility: protocol.VisibilityPrivate})
	roomState(t, caio)
	late := connect(l, "u3", "Duda")
	snap := late.waitFor(t, protocol.MsgLobbySnapshot, nil).Payload.(protocol.LobbySnapshotPayload)
	if len(snap.Rooms) != 1 || snap.Rooms[0].ID != up.Room.ID {
		t.Fatalf("snapshot deve listar só a sala pública: %+v", snap.Rooms)
	}
}

func TestJoinByCodeIsCaseAndSpaceInsensitive(t *testing.T) {
	l := newLobby(t, 3)
	ana, bia := connect(l, "u0", "Ana"), connect(l, "u1", "Bia")
	dispatch(l, ana, protocol.TypeCreateRoom, protocol.CreateRoomPayload{Visibility: protocol.VisibilityPrivate})
	code := roomState(t, ana).Code
	typed := " " + strings.ToLower(code) + " "
	dispatch(l, bia, protocol.TypeJoinRoom, protocol.JoinRoomPayload{Code: &typed})
	if st := roomState(t, bia); st.YourSeat != 1 || st.Code != code {
		t.Fatalf("room_state = %+v", st)
	}
}

func TestPrivateRoomIsNotJoinableByID(t *testing.T) {
	l := newLobby(t, 3)
	ana, bia := connect(l, "u0", "Ana"), connect(l, "u1", "Bia")
	dispatch(l, ana, protocol.TypeCreateRoom, protocol.CreateRoomPayload{Visibility: protocol.VisibilityPrivate})
	id := roomState(t, ana).ID
	dispatch(l, bia, protocol.TypeJoinRoom, protocol.JoinRoomPayload{RoomID: &id})
	bia.waitFor(t, protocol.MsgError, errorCode(protocol.CodeRoomNotFound))
}

func TestOneRoomPerUserAndMaxRooms(t *testing.T) {
	l := newLobby(t, 2)
	ana, bia, caio := connect(l, "u0", "Ana"), connect(l, "u1", "Bia"), connect(l, "u2", "Caio")
	public := protocol.CreateRoomPayload{Visibility: protocol.VisibilityPublic}
	dispatch(l, ana, protocol.TypeCreateRoom, public)
	roomState(t, ana)
	dispatch(l, ana, protocol.TypeCreateRoom, public)
	ana.waitFor(t, protocol.MsgError, errorCode(protocol.CodeAlreadyInRoom))
	dispatch(l, bia, protocol.TypeCreateRoom, public)
	roomState(t, bia)
	dispatch(l, caio, protocol.TypeCreateRoom, public)
	caio.waitFor(t, protocol.MsgError, errorCode(protocol.CodeTooManyRooms))
}

func TestLeavingRoomReturnsToLobby(t *testing.T) {
	l := newLobby(t, 3)
	ana, bia := connect(l, "u0", "Ana"), connect(l, "u1", "Bia")
	dispatch(l, ana, protocol.TypeCreateRoom, protocol.CreateRoomPayload{Visibility: protocol.VisibilityPublic})
	id := roomState(t, ana).ID
	dispatch(l, bia, protocol.TypeJoinRoom, protocol.JoinRoomPayload{RoomID: &id})
	roomState(t, bia)
	before := bia.count(protocol.MsgLobbySnapshot)
	dispatch(l, bia, protocol.TypeLeaveRoom, nil)
	bia.waitFor(t, protocol.MsgLobbySnapshot, func(protocol.ServerMessage) bool { return bia.count(protocol.MsgLobbySnapshot) > before })
	msgs := bia.messages()
	leftAt, snapAt := -1, -1
	for i, m := range msgs {
		if m.Type == protocol.MsgLeftRoom {
			leftAt = i
		}
		if m.Type == protocol.MsgLobbySnapshot {
			snapAt = i
		}
	}
	if leftAt < 0 || snapAt < leftAt {
		t.Fatalf("left_room deve vir antes do novo lobby_snapshot: %d, %d", leftAt, snapAt)
	}
}

func TestDestroyedRoomLeavesTheList(t *testing.T) {
	l := newLobby(t, 3)
	ana, bia := connect(l, "u0", "Ana"), connect(l, "u1", "Bia")
	dispatch(l, ana, protocol.TypeCreateRoom, protocol.CreateRoomPayload{Visibility: protocol.VisibilityPublic})
	id := roomState(t, ana).ID
	bia.waitFor(t, protocol.MsgRoomUpserted, nil)
	dispatch(l, ana, protocol.TypeLeaveRoom, nil)
	bia.waitFor(t, protocol.MsgRoomRemoved, func(m protocol.ServerMessage) bool {
		return m.Payload.(protocol.RoomRemovedPayload).RoomID == id
	})
	dispatch(l, bia, protocol.TypeJoinRoom, protocol.JoinRoomPayload{RoomID: &id})
	bia.waitFor(t, protocol.MsgError, errorCode(protocol.CodeRoomNotFound))
}

func TestStaleConnectionIsIgnored(t *testing.T) {
	l := newLobby(t, 3)
	old := connect(l, "u0", "Ana")
	fresh := connect(l, "u0", "Ana")
	if old.closeReason() != lobby.CloseReplaced {
		t.Fatal("a conexão antiga deve ser fechada como substituída")
	}
	fresh.waitFor(t, protocol.MsgHello, nil)
	dispatch(l, old, protocol.TypeCreateRoom, protocol.CreateRoomPayload{Visibility: protocol.VisibilityPublic})
	dispatch(l, fresh, protocol.TypeCreateRoom, protocol.CreateRoomPayload{Visibility: protocol.VisibilityPublic})
	if st := roomState(t, fresh); st.YourSeat != 0 {
		t.Fatalf("room_state = %+v", st)
	}
	if fresh.count(protocol.MsgError) != 0 {
		t.Fatal("a mensagem da conexão antiga não pode ter criado sala")
	}
}

func TestUnknownTypeAndCommandOutsideRoom(t *testing.T) {
	l := newLobby(t, 3)
	ana := connect(l, "u0", "Ana")
	dispatch(l, ana, "hack", nil)
	ana.waitFor(t, protocol.MsgError, errorCode(protocol.CodeBadRequest))
	dispatch(l, ana, protocol.TypeSetReady, protocol.SetReadyPayload{Ready: true})
	ana.waitFor(t, protocol.MsgError, errorCode(protocol.CodeNotInRoom))
}

func TestRestoreIndexesPlayersAndAbortEndsMatch(t *testing.T) {
	l := newLobby(t, 3)
	rec, _ := enginetest.RandomRecord(4)
	rec.Actions = rec.Actions[:12]
	state, err := replay.Rebuild(rec)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Restore([]match.Recovered{{Record: rec, State: state, NextPly: 12}}); err != nil {
		t.Fatal(err)
	}
	ana := connect(l, "u0", "Ana")
	if st := roomState(t, ana); st.Status != protocol.RoomPlaying || st.MatchID != rec.ID {
		t.Fatalf("room_state = %+v", st)
	}
	ana.waitFor(t, protocol.MsgState, nil)
	if ana.count(protocol.MsgLobbySnapshot) != 0 {
		t.Fatal("jogador de partida restaurada vai direto para a sala")
	}
	if err := l.Abort(context.Background(), rec.ID); err != nil {
		t.Fatal(err)
	}
	ended := ana.waitFor(t, protocol.MsgMatchEnded, nil).Payload.(protocol.MatchEndedPayload)
	if ended.Reason != protocol.EndAborted {
		t.Fatalf("match_ended = %+v", ended)
	}
	if err := l.Abort(context.Background(), "inexistente"); err != nil {
		t.Fatal("abort de partida sem sala não é erro")
	}
}

func TestShutdownClosesClientsAndRejectsConnect(t *testing.T) {
	l := newLobby(t, 3)
	ana := connect(l, "u0", "Ana")
	dispatch(l, ana, protocol.TypeCreateRoom, protocol.CreateRoomPayload{Visibility: protocol.VisibilityPublic})
	roomState(t, ana)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := l.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if ana.closeReason() != lobby.CloseShutdown {
		t.Fatal("clientes fecham no desligamento")
	}
	if l.Connect(&fakeClient{id: "u9"}, protocol.UserInfo{ID: "u9"}) {
		t.Fatal("Connect deve recusar durante o desligamento")
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/lobby/`
Expected: FAIL — pacote `lobby` inexistente.

- [ ] **Step 3: Implementar o adaptador do store**

`backend/internal/lobby/store.go`:

```go
package lobby

import (
	"context"
	"errors"
	"fmt"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/match"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/room"
)

// NewMatchStore adapta *match.Store à porta da sala, traduzindo match.ErrMatchClosed.
func NewMatchStore(s *match.Store) room.MatchStore { return storeAdapter{s} }

type storeAdapter struct{ s *match.Store }

func (a storeAdapter) Create(ctx context.Context, seed uint64, dealer engine.Seat, players [4]replay.PlayerSeat) (string, error) {
	return a.s.Create(ctx, seed, dealer, players)
}

func (a storeAdapter) AppendAction(ctx context.Context, id string, ply int, act engine.Action, src replay.Source) error {
	return closed(a.s.AppendAction(ctx, id, ply, act, src))
}

func (a storeAdapter) Finish(ctx context.Context, id string) error { return closed(a.s.Finish(ctx, id)) }

func (a storeAdapter) Abandon(ctx context.Context, id, reason string) error {
	return closed(a.s.Abandon(ctx, id, reason))
}

func closed(err error) error {
	if errors.Is(err, match.ErrMatchClosed) {
		return fmt.Errorf("%w: %w", room.ErrMatchClosed, err)
	}
	return err
}
```

- [ ] **Step 4: Implementar o lobby**

`backend/internal/lobby/lobby.go`:

```go
// Package lobby é o registro de salas e conexões: lista ao vivo, códigos, índice usuário→sala,
// restauração após restart, abort pelo administrador e desligamento. Todo o estado fica sob um
// mutex; nada aqui bloqueia, porque Actor.Push e Client.Send/Close nunca esperam.
package lobby

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
	"sync"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/clock"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/match"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/room"
)

type CloseReason int

const (
	CloseReplaced CloseReason = iota + 1 // o mesmo usuário abriu outra conexão
	CloseShutdown                        // o servidor está desligando
)

// Client é a conexão de um usuário. Send e Close nunca bloqueiam.
type Client interface {
	UserID() string
	Send(msg protocol.ServerMessage)
	Close(reason CloseReason)
}

type Options struct {
	Timing   room.Timing
	MaxRooms int
	Store    room.MatchStore
	Clock    clock.Clock
	Rand     io.Reader
	Log      *slog.Logger
}

type member struct {
	client Client
	info   protocol.UserInfo
}

type entry struct {
	actor   *room.Actor
	summary room.Summary
}

type Lobby struct {
	opt    Options
	ctx    context.Context
	cancel context.CancelFunc
	rand   io.Reader

	mu      sync.Mutex
	closing bool
	clients map[string]*member // usuário → conexão ativa
	rooms   map[string]*entry  // sala → ator e resumo
	codes   map[string]string  // código → sala
	index   map[string]string  // usuário → sala (uma sala por usuário)
}

func New(ctx context.Context, o Options) *Lobby {
	if o.Log == nil {
		o.Log = slog.Default()
	}
	ctx, cancel := context.WithCancel(ctx)
	return &Lobby{opt: o, ctx: ctx, cancel: cancel, rand: &lockedReader{r: o.Rand},
		clients: map[string]*member{}, rooms: map[string]*entry{}, codes: map[string]string{}, index: map[string]string{}}
}

// lockedReader serializa o gerador compartilhado pelas salas.
type lockedReader struct {
	mu sync.Mutex
	r  io.Reader
}

func (l *lockedReader) Read(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.r.Read(p)
}

// Connect registra a conexão (substituindo a anterior do mesmo usuário) e manda hello e,
// para quem está fora de sala, a lista; quem está em sala recebe o estado pela própria sala.
func (l *Lobby) Connect(c Client, info protocol.UserInfo) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closing {
		return false
	}
	if old := l.clients[info.ID]; old != nil {
		old.client.Close(CloseReplaced)
	}
	l.clients[info.ID] = &member{client: c, info: info}
	c.Send(protocol.Hello(info))
	if e := l.roomOfLocked(info.ID); e != nil {
		e.actor.Push(room.Connected{UserID: info.ID})
		return true
	}
	c.Send(l.snapshotLocked())
	return true
}

// Disconnect esquece a conexão, se ainda for a ativa do usuário, e avisa a sala.
func (l *Lobby) Disconnect(c Client) {
	l.mu.Lock()
	defer l.mu.Unlock()
	id := c.UserID()
	if m := l.clients[id]; m == nil || m.client != c {
		return
	}
	delete(l.clients, id)
	if e := l.roomOfLocked(id); e != nil {
		e.actor.Push(room.Disconnected{UserID: id})
	}
}

// Dispatch trata create_room/join_room e repassa o resto para a sala do usuário.
func (l *Lobby) Dispatch(c Client, msg protocol.ClientMessage) {
	l.mu.Lock()
	defer l.mu.Unlock()
	id := c.UserID()
	m := l.clients[id]
	if l.closing || m == nil || m.client != c {
		return // conexão substituída ou servidor desligando
	}
	switch msg.Type {
	case protocol.TypeCreateRoom:
		l.createRoomLocked(m, msg)
	case protocol.TypeJoinRoom:
		l.joinRoomLocked(m, msg)
	default:
		if !protocol.KnownClientType(msg.Type) {
			c.Send(protocol.ErrorMsg(protocol.CodeBadRequest, msg.Seq))
			return
		}
		e := l.roomOfLocked(id)
		if e == nil {
			c.Send(protocol.ErrorMsg(protocol.CodeNotInRoom, msg.Seq))
			return
		}
		if !e.actor.Push(room.Command{UserID: id, Message: msg}) {
			c.Send(protocol.ErrorMsg(protocol.CodeRoomNotFound, msg.Seq))
		}
	}
}

func (l *Lobby) createRoomLocked(m *member, msg protocol.ClientMessage) {
	var p protocol.CreateRoomPayload
	if err := protocol.Decode(msg.Payload, &p, "visibility"); err != nil || !p.Visibility.Valid() {
		m.client.Send(protocol.ErrorMsg(protocol.CodeBadRequest, msg.Seq))
		return
	}
	if _, in := l.index[m.info.ID]; in {
		m.client.Send(protocol.ErrorMsg(protocol.CodeAlreadyInRoom, msg.Seq))
		return
	}
	if len(l.rooms) >= l.opt.MaxRooms {
		m.client.Send(protocol.ErrorMsg(protocol.CodeTooManyRooms, msg.Seq))
		return
	}
	id, code, err := l.newIdentityLocked()
	if err != nil {
		l.opt.Log.Error("falha ao gerar identificação da sala", "err", err)
		m.client.Send(protocol.ErrorMsg(protocol.CodeInternal, msg.Seq))
		return
	}
	name := "Sala de " + m.info.DisplayName
	public := p.Visibility == protocol.VisibilityPublic
	actor := room.Start(l.ctx, l.roomOptions(id, code, name, public), nil)
	l.rooms[id] = &entry{actor: actor, summary: room.Summary{
		Room: protocol.RoomSummary{ID: id, Name: name, Status: protocol.RoomWaiting}, Public: public, Code: code}}
	l.codes[code] = id
	l.index[m.info.ID] = id
	l.opt.Log.Info("sala criada", "room_id", id, "public", public, "user_id", m.info.ID)
	actor.Push(room.Join{UserID: m.info.ID, DisplayName: m.info.DisplayName, Seq: msg.Seq})
}

func (l *Lobby) joinRoomLocked(m *member, msg protocol.ClientMessage) {
	var p protocol.JoinRoomPayload
	if err := protocol.Decode(msg.Payload, &p); err != nil || (p.RoomID == nil) == (p.Code == nil) {
		m.client.Send(protocol.ErrorMsg(protocol.CodeBadRequest, msg.Seq))
		return
	}
	if _, in := l.index[m.info.ID]; in {
		m.client.Send(protocol.ErrorMsg(protocol.CodeAlreadyInRoom, msg.Seq))
		return
	}
	var e *entry
	if p.Code != nil {
		e = l.rooms[l.codes[strings.ToUpper(strings.TrimSpace(*p.Code))]]
	} else if candidate := l.rooms[*p.RoomID]; candidate != nil && candidate.summary.Public {
		e = candidate
	}
	if e == nil {
		m.client.Send(protocol.ErrorMsg(protocol.CodeRoomNotFound, msg.Seq))
		return
	}
	roomID := e.summary.Room.ID
	l.index[m.info.ID] = roomID // reserva; a sala confirma ou chama Released
	if !e.actor.Push(room.Join{UserID: m.info.ID, DisplayName: m.info.DisplayName, Seat: p.Seat, Seq: msg.Seq}) {
		delete(l.index, m.info.ID)
		m.client.Send(protocol.ErrorMsg(protocol.CodeRoomNotFound, msg.Seq))
	}
}

// Send implementa room.Outbox.
func (l *Lobby) Send(userID string, msg protocol.ServerMessage) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if m := l.clients[userID]; m != nil {
		m.client.Send(msg)
	}
}

// Update implementa room.Registry: guarda o resumo e avisa quem está no lobby.
func (l *Lobby) Update(s room.Summary) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.rooms[s.Room.ID]
	if e == nil {
		return
	}
	e.summary = s
	if s.Public {
		l.broadcastLobbyLocked(protocol.RoomUpserted(s.Room))
	}
}

// Released implementa room.Registry: o usuário volta ao lobby e recebe a lista.
func (l *Lobby) Released(roomID, userID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index[userID] != roomID {
		return
	}
	delete(l.index, userID)
	if m := l.clients[userID]; m != nil && !l.closing {
		m.client.Send(l.snapshotLocked())
	}
}

// Destroyed implementa room.Registry: remove a sala, o código e a linha da lista.
func (l *Lobby) Destroyed(roomID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.rooms[roomID]
	if e == nil {
		return
	}
	delete(l.rooms, roomID)
	delete(l.codes, e.summary.Code)
	for user, id := range l.index {
		if id == roomID {
			delete(l.index, user)
		}
	}
	l.opt.Log.Info("sala destruída", "room_id", roomID)
	if e.summary.Public {
		l.broadcastLobbyLocked(protocol.RoomRemoved(roomID))
	}
}

// Restore recria as salas das partidas recuperadas no boot: públicas, código novo, todos desconectados.
func (l *Lobby) Restore(recovered []match.Recovered) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, rc := range recovered {
		id, code, err := l.newIdentityLocked()
		if err != nil {
			return err
		}
		name := "Sala de " + rc.Record.Players[0].DisplayName
		actor := room.Start(l.ctx, l.roomOptions(id, code, name, true),
			&room.Restored{MatchID: rc.Record.ID, Players: rc.Record.Players, State: rc.State, NextPly: rc.NextPly})
		l.rooms[id] = &entry{actor: actor, summary: room.Summary{
			Room:   protocol.RoomSummary{ID: id, Name: name, Status: protocol.RoomPlaying},
			Public: true, Code: code, MatchID: rc.Record.ID}}
		l.codes[code] = id
		for _, p := range rc.Record.Players {
			l.index[p.UserID] = id
		}
		l.opt.Log.Info("sala restaurada", "room_id", id, "match_id", rc.Record.ID)
	}
	return nil
}

// Abort implementa admin.MatchAborter: encerra a sala viva da partida, se houver.
func (l *Lobby) Abort(_ context.Context, matchID string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, e := range l.rooms {
		if e.summary.MatchID == matchID {
			e.actor.Push(room.Abort{Reason: "admin"})
		}
	}
	return nil
}

// Shutdown recusa novidades, para as salas (as partidas ficam para o Recover) e fecha as conexões.
func (l *Lobby) Shutdown(ctx context.Context) error {
	l.mu.Lock()
	l.closing = true
	actors := make([]*room.Actor, 0, len(l.rooms))
	for _, e := range l.rooms {
		actors = append(actors, e.actor)
		e.actor.Push(room.Stop{})
	}
	clients := make([]Client, 0, len(l.clients))
	for _, m := range l.clients {
		clients = append(clients, m.client)
	}
	l.mu.Unlock()
	var err error
	for _, a := range actors {
		select {
		case <-a.Done():
		case <-ctx.Done():
			err = ctx.Err()
		}
		if err != nil {
			break
		}
	}
	for _, c := range clients {
		c.Close(CloseShutdown)
	}
	l.cancel()
	return err
}

func (l *Lobby) roomOptions(id, code, name string, public bool) room.Options {
	return room.Options{ID: id, Code: code, Name: name, Public: public, Timing: l.opt.Timing, Store: l.opt.Store,
		Out: l, Registry: l, Clock: l.opt.Clock, Rand: l.rand, Log: l.opt.Log.With("room_id", id)}
}

func (l *Lobby) roomOfLocked(userID string) *entry {
	if id, ok := l.index[userID]; ok {
		return l.rooms[id]
	}
	return nil
}

func (l *Lobby) snapshotLocked() protocol.ServerMessage {
	var rooms []protocol.RoomSummary
	for _, e := range l.rooms {
		if e.summary.Public {
			rooms = append(rooms, e.summary.Room)
		}
	}
	slices.SortFunc(rooms, func(a, b protocol.RoomSummary) int { return strings.Compare(a.ID, b.ID) })
	return protocol.LobbySnapshot(rooms)
}

// broadcastLobbyLocked envia a quem está conectado e fora de sala.
func (l *Lobby) broadcastLobbyLocked(msg protocol.ServerMessage) {
	for id, m := range l.clients {
		if _, in := l.index[id]; !in {
			m.client.Send(msg)
		}
	}
}

const codeAlphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"

// newIdentityLocked sorteia o id (UUID v4) e um código de 6 caracteres livre.
func (l *Lobby) newIdentityLocked() (id, code string, err error) {
	var b [16]byte
	if _, err := io.ReadFull(l.rand, b[:]); err != nil {
		return "", "", err
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	id = fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
	for {
		var raw [6]byte
		if _, err := io.ReadFull(l.rand, raw[:]); err != nil {
			return "", "", err
		}
		buf := make([]byte, len(raw))
		for i, v := range raw {
			buf[i] = codeAlphabet[int(v)%len(codeAlphabet)]
		}
		if _, taken := l.codes[string(buf)]; !taken {
			return id, string(buf), nil
		}
	}
}
```

- [ ] **Step 5: Rodar e ver passar**

Run: `cd backend && gofmt -l internal/lobby && go vet ./internal/lobby/ && go test -race ./internal/lobby/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/lobby
git commit -m "feat(lobby): registro de salas com lista ao vivo, códigos, restauração, abort e desligamento"
```

---

### Task 15: Transporte WebSocket (`/ws`)

**Files:**
- Create: `backend/internal/transport/ws/handler.go`, `backend/internal/transport/ws/client.go`
- Modify: `backend/go.mod`, `backend/go.sum` (`github.com/coder/websocket v1.8.15`)
- Test: `backend/internal/transport/ws/handler_test.go`

**Interfaces:**
- Consumes: `lobby.Lobby` (`Connect`, `Disconnect`, `Dispatch`), `lobby.CloseReason`; `auth.Principal`, `auth.ErrUnauthenticated`, `auth.CheckOrigin`, `auth.CookieConfig`.
- Produces:
  ```go
  type Authenticator interface {
      SessionFromRequest(r *http.Request) (auth.Principal, error)
      Authenticate(ctx context.Context, token string) (auth.Principal, error)
      Cookies() auth.CookieConfig
  } // *auth.Service satisfaz
  type Limits struct{ MaxMessage int; RatePerSec float64; Burst, Queue int; PingEvery, PongWait, WriteWait, Revalidate time.Duration }
  var DefaultLimits Limits // 4096 B, 20/s burst 40, fila 64, ping 20s, pong 10s, escrita 5s, revalidação 5min
  type Handler struct{ Auth Authenticator; Lobby *lobby.Lobby; Origin string; Limits Limits; Log *slog.Logger } // http.Handler
  ```
- Códigos de fechamento: `4001 replaced`, `4003 session_invalid`, `4008` (tamanho/taxa), `1001 going_away`, `1013` (fila cheia).

- [ ] **Step 1: Adicionar a dependência**

Run: `cd backend && go get github.com/coder/websocket@v1.8.15`
Expected: `go.mod` com `github.com/coder/websocket v1.8.15`.

- [ ] **Step 2: Escrever os testes que falham**

`backend/internal/transport/ws/handler_test.go`:

```go
package ws_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/auth"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/clock"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/lobby"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/room"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/transport/ws"
)

const origin = "http://truco.test"

type fakeAuth struct {
	mu       sync.Mutex
	sessions map[string]auth.Principal
}

func (f *fakeAuth) Cookies() auth.CookieConfig { return auth.CookieConfig{Secure: false} }

func (f *fakeAuth) Authenticate(_ context.Context, token string) (auth.Principal, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.sessions[token]
	if !ok {
		return auth.Principal{}, auth.ErrUnauthenticated
	}
	return p, nil
}

func (f *fakeAuth) SessionFromRequest(r *http.Request) (auth.Principal, error) {
	return f.Authenticate(r.Context(), f.Cookies().Read(r))
}

func (f *fakeAuth) revoke(token string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.sessions, token)
}

type nopStore struct{}

func (nopStore) Create(context.Context, uint64, engine.Seat, [4]replay.PlayerSeat) (string, error) {
	return "m1", nil
}
func (nopStore) AppendAction(context.Context, string, int, engine.Action, replay.Source) error { return nil }
func (nopStore) Finish(context.Context, string) error                                         { return nil }
func (nopStore) Abandon(context.Context, string, string) error                                { return nil }

type env struct {
	srv  *httptest.Server
	auth *fakeAuth
}

func newEnv(t *testing.T, tweak ...func(*ws.Limits)) env {
	a := &fakeAuth{sessions: map[string]auth.Principal{
		"tok-ana": {UserID: "u0", Username: "ana", DisplayName: "Ana"},
		"tok-bia": {UserID: "u1", Username: "bia", DisplayName: "Bia"},
	}}
	discard := slog.New(slog.NewTextHandler(io.Discard, nil))
	l := lobby.New(context.Background(), lobby.Options{Timing: room.Timing{Play: time.Minute, Vote: time.Minute,
		Choice: time.Minute, Burn: time.Minute, Absent: time.Minute, AbsenceGrace: time.Hour, AbsenceBudget: time.Hour,
		WaitingDisconnect: time.Hour}, MaxRooms: 5, Store: nopStore{}, Clock: clock.NewManual(time.Now()),
		Rand: rand.NewChaCha8([32]byte{3}), Log: discard})
	limits := ws.DefaultLimits
	for _, f := range tweak {
		f(&limits)
	}
	srv := httptest.NewServer(&ws.Handler{Auth: a, Lobby: l, Origin: origin, Limits: limits, Log: discard})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = l.Shutdown(ctx)
		srv.Close()
	})
	return env{srv: srv, auth: a}
}

func (e env) dial(token, from string) (*websocket.Conn, *http.Response, error) {
	header := http.Header{}
	if from != "" {
		header.Set("Origin", from)
	}
	if token != "" {
		header.Set("Cookie", "truco_session="+token)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return websocket.Dial(ctx, "ws"+strings.TrimPrefix(e.srv.URL, "http"), &websocket.DialOptions{HTTPHeader: header})
}

func (e env) connect(t *testing.T, token string) *websocket.Conn {
	t.Helper()
	c, _, err := e.dial(token, origin)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = c.CloseNow() })
	return c
}

type inbound struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

func readType(t *testing.T, c *websocket.Conn, typ string) inbound {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			t.Fatalf("esperando %s: %v", typ, err)
		}
		var m inbound
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatal(err)
		}
		if m.Type == typ {
			return m
		}
	}
}

func write(t *testing.T, c *websocket.Conn, data string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Write(ctx, websocket.MessageText, []byte(data)); err != nil {
		t.Fatal(err)
	}
}

func closeStatus(t *testing.T, c *websocket.Conn) websocket.StatusCode {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for {
		if _, _, err := c.Read(ctx); err != nil {
			return websocket.CloseStatus(err)
		}
	}
}

func TestRejectsMissingSessionAndForeignOrigin(t *testing.T) {
	e := newEnv(t)
	if _, resp, err := e.dial("", origin); err == nil || resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("sem sessão: %v %v", resp, err)
	}
	if _, resp, err := e.dial("tok-ana", "http://evil.test"); err == nil || resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("origem estranha: %v %v", resp, err)
	}
}

func TestHelloOnConnect(t *testing.T) {
	e := newEnv(t)
	c := e.connect(t, "tok-ana")
	var hello protocol.HelloPayload
	if err := json.Unmarshal(readType(t, c, "hello").Payload, &hello); err != nil || hello.User.ID != "u0" || hello.User.DisplayName != "Ana" {
		t.Fatalf("hello = %+v (%v)", hello, err)
	}
	readType(t, c, "lobby_snapshot")
}

func TestSecondConnectionClosesFirstWith4001(t *testing.T) {
	e := newEnv(t)
	first := e.connect(t, "tok-ana")
	readType(t, first, "hello")
	second := e.connect(t, "tok-ana")
	readType(t, second, "hello")
	if code := closeStatus(t, first); code != 4001 {
		t.Fatalf("código = %d", code)
	}
}

func TestOversizedMessageClosesWith4008(t *testing.T) {
	e := newEnv(t)
	c := e.connect(t, "tok-ana")
	readType(t, c, "hello")
	write(t, c, `{"type":"leave_room","payload":"`+string(bytes.Repeat([]byte("a"), 5000))+`"}`)
	if code := closeStatus(t, c); code != 4008 {
		t.Fatalf("código = %d", code)
	}
}

func TestFloodClosesWith4008(t *testing.T) {
	e := newEnv(t, func(l *ws.Limits) { l.RatePerSec, l.Burst = 1, 2 })
	c := e.connect(t, "tok-ana")
	readType(t, c, "hello")
	for i := range 4 {
		write(t, c, fmt.Sprintf(`{"type":"leave_room","seq":%d}`, i))
	}
	if code := closeStatus(t, c); code != 4008 {
		t.Fatalf("código = %d", code)
	}
}

func TestMalformedMessageGetsBadRequestAndKeepsConnection(t *testing.T) {
	e := newEnv(t)
	c := e.connect(t, "tok-ana")
	readType(t, c, "hello")
	write(t, c, `{`)
	var p protocol.ErrorPayload
	if err := json.Unmarshal(readType(t, c, "error").Payload, &p); err != nil || p.Code != protocol.CodeBadRequest {
		t.Fatalf("erro = %+v", p)
	}
	write(t, c, `{"type":"create_room","seq":2,"payload":{"visibility":"public"}}`)
	readType(t, c, "room_state")
}

func TestRevalidationClosesRevokedSession(t *testing.T) {
	e := newEnv(t, func(l *ws.Limits) { l.Revalidate = 50 * time.Millisecond })
	c := e.connect(t, "tok-ana")
	readType(t, c, "hello")
	e.auth.revoke("tok-ana")
	if code := closeStatus(t, c); code != 4003 {
		t.Fatalf("código = %d", code)
	}
}
```

- [ ] **Step 3: Rodar e ver falhar**

Run: `cd backend && go test ./internal/transport/ws/`
Expected: FAIL — pacote `ws` inexistente.

- [ ] **Step 4: Implementar o handler**

`backend/internal/transport/ws/handler.go`:

```go
// Package ws expõe o lobby por WebSocket em GET /ws: autentica o upgrade pelo cookie de sessão,
// mantém uma conexão por usuário e aplica limites de tamanho, taxa e validade da sessão.
package ws

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/auth"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/lobby"
)

// Authenticator é o que o handler usa de auth.Service.
type Authenticator interface {
	SessionFromRequest(r *http.Request) (auth.Principal, error)
	Authenticate(ctx context.Context, token string) (auth.Principal, error)
	Cookies() auth.CookieConfig
}

type Limits struct {
	MaxMessage int     // bytes; acima disso fecha com 4008
	RatePerSec float64 // mensagens por segundo
	Burst      int
	Queue      int // fila de saída; cheia derruba o cliente lento
	PingEvery  time.Duration
	PongWait   time.Duration
	WriteWait  time.Duration
	Revalidate time.Duration // intervalo de revalidação da sessão
}

var DefaultLimits = Limits{MaxMessage: 4096, RatePerSec: 20, Burst: 40, Queue: 64,
	PingEvery: 20 * time.Second, PongWait: 10 * time.Second, WriteWait: 5 * time.Second, Revalidate: 5 * time.Minute}

// hardReadLimit é o teto da biblioteca: acima dele ela mesma fecha com 1009.
const hardReadLimit = 64 << 10

const (
	codeReplaced       websocket.StatusCode = 4001
	codeSessionInvalid websocket.StatusCode = 4003
	codePolicy         websocket.StatusCode = 4008
)

type Handler struct {
	Auth   Authenticator
	Lobby  *lobby.Lobby
	Origin string
	Limits Limits
	Log    *slog.Logger
}

func (h *Handler) log() *slog.Logger {
	if h.Log == nil {
		return slog.Default()
	}
	return h.Log
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p, err := h.Auth.SessionFromRequest(r)
	if errors.Is(err, auth.ErrUnauthenticated) {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "faça login para jogar")
		return
	}
	if err != nil {
		h.log().ErrorContext(r.Context(), "falha ao autenticar o WebSocket", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "erro interno")
		return
	}
	if !auth.CheckOrigin(r, h.Origin) {
		writeError(w, http.StatusForbidden, "forbidden_origin", "origem não permitida")
		return
	}
	// Os timeouts do http.Server valem para a API; a conexão longa usa os próprios limites.
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Time{})
	_ = rc.SetWriteDeadline(time.Time{})
	// A origem já foi conferida contra APP_ORIGIN, o que também funciona atrás de proxy.
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	newClient(h, conn, p, h.Auth.Cookies().Read(r)).serve()
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": message}})
}
```

`backend/internal/transport/ws/client.go`:

```go
package ws

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/coder/websocket"
	"golang.org/x/time/rate"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/auth"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/lobby"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
)

// client é uma conexão de usuário: a leitura roda numa goroutine, a escrita em outra
// (a de serve), e o fim é sinalizado por done com o código de fechamento escolhido.
type client struct {
	h       *Handler
	conn    *websocket.Conn
	info    protocol.UserInfo
	token   string
	out     chan protocol.ServerMessage
	limiter *rate.Limiter
	done    chan struct{}
	once    sync.Once
	code    websocket.StatusCode // 0 = o par sumiu; fecha sem handshake
	reason  string
}

func newClient(h *Handler, conn *websocket.Conn, p auth.Principal, token string) *client {
	return &client{h: h, conn: conn, token: token,
		info:    protocol.UserInfo{ID: p.UserID, Username: p.Username, DisplayName: p.DisplayName},
		out:     make(chan protocol.ServerMessage, h.Limits.Queue),
		limiter: rate.NewLimiter(rate.Limit(h.Limits.RatePerSec), h.Limits.Burst),
		done:    make(chan struct{})}
}

func (c *client) UserID() string { return c.info.ID }

// Send enfileira sem bloquear; fila cheia derruba o cliente lento, que reconecta e recebe o snapshot.
func (c *client) Send(m protocol.ServerMessage) {
	select {
	case <-c.done:
	case c.out <- m:
	default:
		c.closeWith(websocket.StatusTryAgainLater, "fila de saída cheia")
	}
}

func (c *client) Close(reason lobby.CloseReason) {
	if reason == lobby.CloseReplaced {
		c.closeWith(codeReplaced, "replaced")
		return
	}
	c.closeWith(websocket.StatusGoingAway, "going_away")
}

func (c *client) closeWith(code websocket.StatusCode, reason string) {
	c.once.Do(func() {
		c.code, c.reason = code, reason
		close(c.done)
	})
}

func (c *client) serve() {
	c.conn.SetReadLimit(hardReadLimit)
	if !c.h.Lobby.Connect(c, c.info) {
		_ = c.conn.Close(websocket.StatusGoingAway, "going_away")
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.h.log().Debug("conexão WebSocket aberta", "user_id", c.info.ID)
	go c.readLoop(ctx)
	c.writeLoop(ctx)
	c.h.Lobby.Disconnect(c)
	c.h.log().Debug("conexão WebSocket fechada", "user_id", c.info.ID, "code", int(c.code))
}

func (c *client) readLoop(ctx context.Context) {
	for {
		_, data, err := c.conn.Read(ctx)
		if err != nil {
			c.closeWith(0, "")
			return
		}
		if len(data) > c.h.Limits.MaxMessage {
			c.policyClose("mensagem grande demais")
			return
		}
		if !c.limiter.Allow() {
			c.policyClose("mensagens demais")
			return
		}
		var msg protocol.ClientMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			c.Send(protocol.ErrorMsg(protocol.CodeBadRequest, 0))
			continue
		}
		c.h.Lobby.Dispatch(c, msg)
	}
}

func (c *client) policyClose(reason string) {
	c.h.log().Warn("conexão fechada por política", "user_id", c.info.ID, "reason", reason)
	c.closeWith(codePolicy, reason)
}

func (c *client) writeLoop(ctx context.Context) {
	ping := time.NewTicker(c.h.Limits.PingEvery)
	defer ping.Stop()
	revalidate := time.NewTicker(c.h.Limits.Revalidate)
	defer revalidate.Stop()
	for {
		select {
		case <-c.done:
			if c.code == 0 {
				_ = c.conn.CloseNow()
			} else {
				_ = c.conn.Close(c.code, c.reason)
			}
			return
		case m := <-c.out:
			if err := c.write(ctx, m); err != nil {
				c.closeWith(0, "")
			}
		case <-ping.C:
			pctx, cancel := context.WithTimeout(ctx, c.h.Limits.PongWait)
			err := c.conn.Ping(pctx)
			cancel()
			if err != nil {
				c.closeWith(0, "")
			}
		case <-revalidate.C:
			if _, err := c.h.Auth.Authenticate(ctx, c.token); errors.Is(err, auth.ErrUnauthenticated) {
				c.closeWith(codeSessionInvalid, "session_invalid")
			}
		}
	}
}

func (c *client) write(ctx context.Context, m protocol.ServerMessage) error {
	data, err := json.Marshal(m)
	if err != nil {
		c.h.log().Error("falha ao serializar mensagem", "type", m.Type, "err", err)
		return nil
	}
	wctx, cancel := context.WithTimeout(ctx, c.h.Limits.WriteWait)
	defer cancel()
	return c.conn.Write(wctx, websocket.MessageText, data)
}
```

- [ ] **Step 5: Rodar e ver passar**

Run: `cd backend && gofmt -l internal/transport && go vet ./internal/transport/... && go test -race ./internal/transport/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/go.mod backend/go.sum backend/internal/transport
git commit -m "feat(ws): handler /ws autenticado com uma conexão por usuário, limites e revalidação de sessão"
```

---

### Task 16: Montagem no servidor, boot com `Recover`, shutdown ordenado e documentação

**Files:**
- Modify: `backend/internal/httpapi/server.go`, `backend/cmd/server/main.go`, `README.md`
- Test: `backend/internal/httpapi/server_test.go`, `backend/cmd/server/main_test.go`

**Interfaces:**
- Consumes: `lobby.New/Restore/Shutdown/NewMatchStore`, `ws.Handler`, `ws.DefaultLimits`, `room.Timing`, `config.Config.Game/MaxRooms/ShutdownTimeout`, `auth.Service.Wait`, `match.Store.Recover`.
- Produces: `httpapi.Deps.WS http.Handler` (montado em `GET /ws` quando não nil); `serve` com a ordem de boot e de desligamento da Spec 3 §9.

- [ ] **Step 1: Escrever os testes que falham**

Em `backend/internal/httpapi/server_test.go`, acrescente:

```go
func TestWebSocketRouteIsMountedWhenConfigured(t *testing.T) {
	h := newHarness(t, func(d *httpapi.Deps) {
		d.WS = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTeapot) })
	})
	if r := h.client().get("/ws"); r.status != http.StatusTeapot {
		t.Fatalf("GET /ws = %d", r.status)
	}
}
```

Em `backend/cmd/server/main_test.go`, extraia o arranque do teste existente para um helper e acrescente o teste de retomada (adicione os imports `io`, `github.com/caarlosandree/truco-multiplayer/backend/internal/enginetest`, `.../match`, `.../replay`):

```go
// startServer sobe "serve" e devolve o endereço e uma função que desliga e devolve o erro de run.
func startServer(t *testing.T, url string, out io.Writer) (string, func() error) {
	t.Helper()
	addrCh := make(chan string, 1)
	onListen = func(addr string) { addrCh <- addr }
	t.Cleanup(func() { onListen = func(string) {} })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, nil, envFor(baseEnv(url)), out) }()
	select {
	case addr := <-addrCh:
		return addr, func() error {
			cancel()
			select {
			case err := <-done:
				return err
			case <-time.After(20 * time.Second):
				t.Fatal("servidor não desligou")
				return nil
			}
		}
	case err := <-done:
		t.Fatalf("servidor saiu antes de ouvir: %v", err)
	case <-time.After(30 * time.Second):
		t.Fatal("servidor não começou a ouvir")
	}
	cancel()
	return "", nil
}

func TestServeResumesInProgressMatchesAndMountsWebSocket(t *testing.T) {
	url := testdb.NewURL(t)
	ctx := context.Background()
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var players [4]replay.PlayerSeat
	for i, name := range []string{"ana", "bia", "caio", "duda"} {
		players[i] = replay.PlayerSeat{UserID: testdb.CreateUser(t, pool, name), DisplayName: name}
	}
	rec, _ := enginetest.RandomRecord(21)
	store := match.NewStore(pool, nil)
	id, err := store.Create(ctx, rec.Seed, rec.FirstDealer, players)
	if err != nil {
		t.Fatal(err)
	}
	for _, logged := range rec.Actions[:10] {
		if err := store.AppendAction(ctx, id, logged.Ply, logged.Action, logged.Source); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	addr, stop := startServer(t, url, &out)
	resp, err := http.Get("http://" + addr + "/ws")
	if err != nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("GET /ws sem sessão: %v %v", resp, err)
	}
	resp.Body.Close()
	if err := stop(); err != nil {
		t.Fatalf("desligamento: %v", err)
	}
	if !strings.Contains(out.String(), "partidas retomadas") || !strings.Contains(out.String(), "count=1") {
		t.Fatalf("log do boot não registra a partida retomada:\n%s", out.String())
	}
}
```

E reescreva `TestServeStartsAndShutsDown` com o helper:

```go
func TestServeStartsAndShutsDown(t *testing.T) {
	addr, stop := startServer(t, testdb.NewURL(t), &bytes.Buffer{})
	resp, err := http.Get("http://" + addr + "/healthz")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz: %v %v", resp, err)
	}
	resp.Body.Close()
	if err := stop(); err != nil {
		t.Fatalf("desligamento: %v", err)
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && TESTCONTAINERS_RYUK_DISABLED=true TRUCO_REQUIRE_DB=1 go test ./internal/httpapi/ -run WebSocketRoute ./cmd/server/`
Expected: FAIL — `unknown field WS in struct literal of type httpapi.Deps`.

- [ ] **Step 3: Montar a rota**

Em `backend/internal/httpapi/server.go`, acrescente ao fim de `Deps`:

```go
	WS      http.Handler // GET /ws (Spec 3); nil desliga o WebSocket
```

e, em `New`, logo depois de `s.mux.HandleFunc("GET /healthz", s.handleHealth)`:

```go
	if d.WS != nil {
		s.mux.Handle("GET /ws", d.WS)
	}
```

- [ ] **Step 4: Reescrever `serve`**

Em `backend/cmd/server/main.go`: acrescente os imports `"crypto/rand"`, `"sync"`, `.../internal/clock`, `.../internal/lobby`, `.../internal/room`, `.../internal/transport/ws`; substitua `serve` por:

```go
func serve(ctx context.Context, cfg config.Config, pool *pgxpool.Pool, logger *slog.Logger) error {
	authSvc := auth.NewService(auth.Options{
		Pool: pool, Mailer: newMailer(cfg, logger), Hasher: auth.NewHasher(auth.DefaultParams, 4),
		BaseURL: cfg.AppBaseURL, Cookies: auth.CookieConfig{Secure: cfg.CookieSecure}, Logger: logger,
	})
	if err := authSvc.BootstrapDomains(ctx, cfg.AllowedEmailDomains); err != nil {
		return err
	}
	store := match.NewStore(pool, nil)
	recovered, err := store.Recover(ctx, logger)
	if err != nil {
		return fmt.Errorf("recuperar partidas: %w", err)
	}
	lob := lobby.New(context.Background(), lobby.Options{Timing: room.Timing(cfg.Game), MaxRooms: cfg.MaxRooms,
		Store: lobby.NewMatchStore(store), Clock: clock.Real{}, Rand: rand.Reader, Log: logger})
	if err := lob.Restore(recovered); err != nil {
		return fmt.Errorf("restaurar salas: %w", err)
	}
	logger.Info("partidas retomadas", "count", len(recovered))
	api := httpapi.New(httpapi.Deps{
		Config: cfg, Auth: authSvc, Stats: stats.NewReader(pool, cfg.RankingMinMatches), Matches: store,
		Admin: admin.NewService(admin.Options{Pool: pool, Auth: authSvc, Aborter: lob, Logger: logger}),
		WS:    &ws.Handler{Auth: authSvc, Lobby: lob, Origin: cfg.AppOrigin, Limits: ws.DefaultLimits, Log: logger},
		Pool:  pool, Logger: logger,
	})
	listener, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("ouvir em %s: %w", cfg.HTTPAddr, err)
	}
	// Os timeouts valem para a API; o handler /ws zera os deadlines da conexão longa.
	srv := &http.Server{
		Handler: api.Handler(), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second,
		WriteTimeout: 60 * time.Second, IdleTimeout: 120 * time.Second,
		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}
	maintCtx, stopMaint := context.WithCancel(context.Background())
	var maint sync.WaitGroup
	maint.Go(func() { api.RunMaintenance(maintCtx) })
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(listener) }()
	logger.Info("servidor ouvindo", "addr", listener.Addr().String())
	onListen(listener.Addr().String())

	select {
	case err := <-errCh:
		stopMaint()
		maint.Wait()
		return err
	case <-ctx.Done():
	}
	logger.Info("desligando o servidor")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	return shutdown(shutdownCtx, logger, lob, srv, errCh, stopMaint, &maint, authSvc)
}

// shutdown segue a Spec 3 §9.2: salas e conexões, HTTP, manutenção e e-mails em envio.
func shutdown(ctx context.Context, logger *slog.Logger, lob *lobby.Lobby, srv *http.Server, errCh <-chan error,
	stopMaint context.CancelFunc, maint *sync.WaitGroup, authSvc *auth.Service) error {
	var errs []error
	if err := lob.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("salas: %w", err))
	}
	if err := srv.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("http: %w", err))
	}
	if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
		errs = append(errs, err)
	}
	stopMaint()
	maint.Wait()
	if err := authSvc.Wait(ctx); err != nil {
		logger.Warn("e-mails ainda em envio no desligamento", "err", err)
	}
	return errors.Join(errs...)
}
```

- [ ] **Step 5: Rodar e ver passar**

Run: `cd backend && go vet ./... && TESTCONTAINERS_RYUK_DISABLED=true TRUCO_REQUIRE_DB=1 go test ./internal/httpapi/ ./cmd/server/ -count=1`
Expected: PASS.

- [ ] **Step 6: Atualizar o README**

Em `README.md`:

1. Na tabela de **Status e roadmap**, troque a linha do sub-projeto 3 por:
   `| 3 | **Servidor de salas** | Lobby ao vivo, salas públicas e privadas, ator por sala, WebSocket, timers, W.O., reconexão e retomada após restart | ✅ **Concluído** |`
2. Substitua o parágrafo `> Ainda não dá para jogar pelo navegador: …` por:
   `> O servidor já joga partidas completas por WebSocket (lobby, salas, timers, W.O. e retomada após restart). Falta a interface (Spec 4).`
3. Em **Decisões já tomadas para as próximas etapas**, remova o item `WebSocket (Spec 3)`.
4. Substitua a seção inteira `## 🔌 Protocolo WebSocket` (até antes de `## 🔐 API HTTP`) por:

````markdown
## 🔌 Protocolo WebSocket

Conexão em `GET /ws`, autenticada pelo mesmo cookie de sessão da API e com `Origin` igual a `APP_ORIGIN`. Cada usuário tem **uma** conexão: abrir outra aba fecha a anterior. Todas as mensagens são JSON com envelope `{ "type", "seq"?, "payload"? }`; o `seq` do cliente volta nos erros.

**Cliente → servidor**

| `type` | `payload` | Quando |
|---|---|---|
| `create_room` | `{ "visibility": "public" \| "private" }` | fora de sala |
| `join_room` | `{ "roomId" \| "code", "seat"? }` | fora de sala; sala privada só por código |
| `leave_room`, `take_seat {seat}`, `set_ready {ready}`, `kick {seat}` | — | sala em espera (`kick` só o anfitrião) |
| `choose_card`, `burn`, `pass_burn`, `play_card`, `request_raise`, `vote` | como no engine | durante a partida |
| `claim_forfeit` | — | quando o W.O. está disponível para a sua dupla |
| `signal` | `{ "id": "truco" \| "boa" \| "vamos" \| "calma" \| "desculpa" \| "valeu" }` | durante a partida, 1 a cada 3 s |

**Servidor → cliente:** `hello`, `lobby_snapshot`, `room_upserted`, `room_removed`, `room_state`, `left_room`, `state` (`PlayerView` + `deadlines` + `pendingSeats`), `events`, `forfeit_available`, `match_ended`, `signal` e `error`. Os tipos TypeScript são **gerados**: payloads em [`messages.ts`](frontend/src/protocol/messages.ts) e uniões discriminadas `ClientMessage`/`ServerMessage` em [`union.ts`](frontend/src/protocol/union.ts).

**Regras da partida em tempo real**

- Cada decisão tem prazo (jogar 30 s, votar 20 s, corte 15 s, queima 15 s); ao estourar, o servidor joga a ação padrão e a grava como `timeout`.
- Desconectado **ou** com 3 timeouts seguidos = ausente: o prazo dele cai para 5 s.
- Ausente por 2 min seguidos (ou 5 min somados na partida) libera o **W.O.** para a dupla adversária, que decide se pede. A partida conta no ranking. Todos ausentes por 5 min = partida abandonada.
- Depois do fim, a sala volta a esperar com os mesmos assentos (revanche).
- Restart do servidor: as partidas em andamento voltam como salas públicas; ao reconectar, cada jogador cai direto na mesa.

**Fechamentos:** `4001` conexão substituída, `4003` sessão inválida, `4008` mensagem grande (> 4 KB) ou mensagens demais, `1001` servidor desligando, `1013` cliente lento.
````

5. Na tabela de **Variáveis de ambiente**, acrescente ao fim:

```markdown
| `TIMER_PLAY_SECONDS` / `TIMER_VOTE_SECONDS` | `30` / `20` | Prazo para jogar e para votar |
| `TIMER_CHOICE_SECONDS` / `TIMER_BURN_SECONDS` | `15` / `15` | Prazo do corte e da queima |
| `TIMER_ABSENT_SECONDS` | `5` | Prazo de quem está ausente |
| `HAND_PAUSE_SECONDS` | `4` | Pausa entre mãos |
| `ABSENCE_GRACE_SECONDS` / `ABSENCE_BUDGET_SECONDS` | `120` / `300` | Ausência que libera o W.O. (seguida / somada) |
| `WAITING_DISCONNECT_SECONDS` | `60` | Quem cai numa sala em espera perde o assento depois disso |
| `MAX_ROOMS` | `50` | Salas simultâneas |
| `SHUTDOWN_TIMEOUT_SECONDS` | `15` | Orçamento do desligamento gracioso |
```

6. Na **Estrutura do repositório**, troque `│       │   ├── state.go               # GameState, HandState, fases, Clone` por duas linhas:
   `│       │   ├── state.go               # Seat, Team, fases, tipos públicos` e `│       │   ├── gamestate.go           # GameState e HandState (privados, fora do tygo)`;
   troque `│       ├── protocol/                  # envelopes WebSocket e decodificação segura` por `│       ├── protocol/                  # mensagens WebSocket tipadas, registro e gerador de uniões TS`; e acrescente, depois da linha de `pagination/`:

```
│       ├── clock/                     # relógio injetável (Real e Manual para testes)
│       ├── room/                      # ator da sala: assentos, partida, timers, presença, W.O.
│       ├── lobby/                     # registro de salas e conexões, lista ao vivo, restore
│       └── transport/ws/              # handler /ws: autenticação, conexão, limites
```

   (a linha de `pagination/` passa a começar com `├──`).

- [ ] **Step 7: Commit**

```bash
git add backend/internal/httpapi backend/cmd/server README.md
git commit -m "feat(server): WebSocket montado, Recover no boot, lobby como MatchAborter e desligamento ordenado"
```

---

### Task 17: Testes ponta a ponta com Postgres e verificação final

**Files:**
- Create: `backend/internal/httpapi/ws_e2e_test.go`

**Interfaces:**
- Consumes: harness de `httpapi_test` (`newHarness`, `signup`, `makeAdmin`, `client.post`, `h.clock` — agora `*clock.Manual`, `h.store`, `h.pool`); `lobby`, `ws`, `room.Timing`, `admin.NewService`.

- [ ] **Step 1: Escrever os testes**

`backend/internal/httpapi/ws_e2e_test.go`:

```go
package httpapi_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/admin"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/auth"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/httpapi"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/lobby"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/room"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/transport/ws"
)

// Sem pausa entre mãos e sem timeouts (o relógio manual só anda quando o teste manda).
var e2eTiming = room.Timing{Play: 30 * time.Second, Vote: 20 * time.Second, Choice: 15 * time.Second,
	Burn: 15 * time.Second, Absent: 5 * time.Second, HandPause: 0, AbsenceGrace: 20 * time.Second,
	AbsenceBudget: 50 * time.Second, WaitingDisconnect: time.Minute}

// swapHandler permite trocar o lobby atrás do /ws (teste de restart).
type swapHandler struct{ current atomic.Pointer[ws.Handler] }

func (s *swapHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.current.Load().ServeHTTP(w, r) }

type game struct {
	h      *harness
	auth   *auth.Service
	ws     *swapHandler
	lobby  *lobby.Lobby
	limits ws.Limits
}

func newGame(t *testing.T, tweak ...func(*ws.Limits)) *game {
	g := &game{ws: &swapHandler{}, limits: ws.DefaultLimits}
	for _, f := range tweak {
		f(&g.limits)
	}
	g.h = newHarness(t, func(d *httpapi.Deps) {
		g.auth = d.Auth
		d.WS = g.ws
		d.Admin = admin.NewService(admin.Options{Pool: d.Pool, Auth: d.Auth, Aborter: g, Now: d.Now, Logger: d.Logger})
	})
	g.startLobby()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = g.lobby.Shutdown(ctx)
	})
	return g
}

// Abort delega ao lobby corrente.
func (g *game) Abort(ctx context.Context, id string) error { return g.lobby.Abort(ctx, id) }

func (g *game) startLobby() {
	discard := slog.New(slog.NewTextHandler(io.Discard, nil))
	g.lobby = lobby.New(context.Background(), lobby.Options{Timing: e2eTiming, MaxRooms: 10,
		Store: lobby.NewMatchStore(g.h.store), Clock: g.h.clock, Rand: rand.NewChaCha8([32]byte{7}), Log: discard})
	g.ws.current.Store(&ws.Handler{Auth: g.auth, Lobby: g.lobby, Origin: origin, Limits: g.limits, Log: discard})
}

type inbound struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type player struct {
	t    *testing.T
	conn *websocket.Conn
	in   chan inbound
	err  error // motivo do fim da leitura; válido depois que in fecha
}

func (g *game) connect(c *client) *player {
	g.h.t.Helper()
	u, _ := url.Parse(g.h.srv.URL)
	header := http.Header{"Origin": {origin}}
	for _, ck := range c.http.Jar.Cookies(u) {
		header.Add("Cookie", ck.Name+"="+ck.Value)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(g.h.srv.URL, "http")+"/ws", &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		g.h.t.Fatalf("dial: %v", err)
	}
	p := &player{t: g.h.t, conn: conn, in: make(chan inbound, 1024)}
	go func() {
		defer close(p.in)
		for {
			_, data, err := conn.Read(context.Background())
			if err != nil {
				p.err = err
				return
			}
			var m inbound
			if json.Unmarshal(data, &m) == nil {
				p.in <- m
			}
		}
	}()
	g.h.t.Cleanup(func() { _ = conn.CloseNow() })
	return p
}

func (p *player) send(typ string, payload any) {
	msg := map[string]any{"type": typ, "seq": 1}
	if payload != nil {
		msg["payload"] = payload
	}
	data, _ := json.Marshal(msg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.conn.Write(ctx, websocket.MessageText, data); err != nil {
		p.t.Fatalf("enviar %s: %v", typ, err)
	}
}

// expect descarta mensagens até chegar uma do tipo e decodifica o payload em into (se não nil).
func (p *player) expect(typ string, into any) {
	p.t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case m, ok := <-p.in:
			if !ok {
				p.t.Fatalf("conexão fechou esperando %s: %v", typ, p.err)
			}
			if m.Type != typ {
				continue
			}
			if into != nil {
				if err := json.Unmarshal(m.Payload, into); err != nil {
					p.t.Fatal(err)
				}
			}
			return
		case <-timeout:
			p.t.Fatalf("não chegou %s", typ)
		}
	}
}

func (p *player) closedWith() websocket.StatusCode {
	p.t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case _, ok := <-p.in:
			if !ok {
				return websocket.CloseStatus(p.err)
			}
		case <-timeout:
			p.t.Fatal("a conexão não fechou")
			return 0
		}
	}
}

// startMatch cadastra quatro usuários, forma uma sala pública e espera a partida começar.
func (g *game) startMatch() ([]*player, []*client, protocol.RoomStatePayload) {
	g.h.t.Helper()
	var players []*player
	var clients []*client
	for _, name := range []string{"ana", "bia", "caio", "duda"} {
		c := g.h.signup(name)
		p := g.connect(c)
		p.expect("hello", nil)
		clients, players = append(clients, c), append(players, p)
	}
	players[0].send("create_room", map[string]any{"visibility": "public"})
	var created protocol.RoomStatePayload
	players[0].expect("room_state", &created)
	for _, p := range players[1:] {
		p.send("join_room", map[string]any{"roomId": created.ID})
		p.expect("room_state", nil)
	}
	for _, p := range players {
		p.send("set_ready", map[string]any{"ready": true})
	}
	var playing protocol.RoomStatePayload
	for playing.Status != protocol.RoomPlaying {
		players[0].expect("room_state", &playing)
	}
	return players, clients, playing
}

func legalMove(v engine.PlayerView) (string, any) {
	switch v.Phase {
	case engine.PhaseCutChoice, engine.PhaseDealerChoice:
		return "choose_card", map[string]int{"keep": 0}
	case engine.PhaseBurnFours:
		return "pass_burn", nil
	case engine.PhaseElevenDecision:
		return "vote", map[string]string{"choice": "play"}
	case engine.PhaseAwaitingRaiseResponse:
		return "vote", map[string]string{"choice": "accept"}
	}
	return "play_card", map[string]any{"slot": v.MyHand[0].Slot, "covered": false}
}

// playToEnd responde a cada state em que o jogador está pendente até todos receberem match_ended.
func (g *game) playToEnd(players []*player) protocol.MatchEndedPayload {
	g.h.t.Helper()
	type tagged struct {
		i int
		m inbound
	}
	merged := make(chan tagged, 4096)
	stop := make(chan struct{})
	defer close(stop)
	for i, p := range players {
		go func() {
			for m := range p.in {
				select {
				case merged <- tagged{i, m}:
				case <-stop:
					return
				}
			}
		}()
	}
	var result protocol.MatchEndedPayload
	timeout := time.After(60 * time.Second)
	for ended := 0; ended < len(players); {
		select {
		case x := <-merged:
			switch x.m.Type {
			case "state":
				var st protocol.StatePayload
				if err := json.Unmarshal(x.m.Payload, &st); err != nil {
					g.h.t.Fatal(err)
				}
				if slices.Contains(st.PendingSeats, st.View.Seat) {
					typ, payload := legalMove(st.View)
					players[x.i].send(typ, payload)
				}
			case "match_ended":
				ended++
				_ = json.Unmarshal(x.m.Payload, &result)
			}
		case <-timeout:
			g.h.t.Fatal("a partida travou")
		}
	}
	return result
}

func (g *game) matchRow(id string) (status string, winner *int16) {
	g.h.t.Helper()
	if err := g.h.pool.QueryRow(context.Background(), "SELECT status, winner_team FROM matches WHERE id = $1", id).Scan(&status, &winner); err != nil {
		g.h.t.Fatal(err)
	}
	return status, winner
}

func TestWebSocketFullMatchIsPersisted(t *testing.T) {
	g := newGame(t)
	players, _, started := g.startMatch()
	result := g.playToEnd(players)
	if result.Reason != protocol.EndGameOver || result.Winner == nil {
		t.Fatalf("match_ended = %+v", result)
	}
	status, winner := g.matchRow(started.MatchID)
	if status != "finished" || winner == nil || int(*winner) != int(*result.Winner) {
		t.Fatalf("partida no banco: %s %v", status, winner)
	}
}

func TestWebSocketReconnectReceivesSnapshot(t *testing.T) {
	g := newGame(t)
	players, clients, started := g.startMatch()
	_ = players[1].conn.CloseNow()
	again := g.connect(clients[1])
	again.expect("hello", nil)
	var st protocol.RoomStatePayload
	again.expect("room_state", &st)
	if st.Status != protocol.RoomPlaying || st.YourSeat != 1 || st.MatchID != started.MatchID {
		t.Fatalf("room_state = %+v", st)
	}
	var state protocol.StatePayload
	again.expect("state", &state)
	if state.View.Seat != 1 {
		t.Fatalf("state = %+v", state.View)
	}
}

func TestWebSocketForfeit(t *testing.T) {
	g := newGame(t)
	players, _, started := g.startMatch()
	_ = players[1].conn.CloseNow()
	for {
		var st protocol.RoomStatePayload
		players[0].expect("room_state", &st)
		if !st.Seats[1].Connected {
			break
		}
	}
	for range 25 {
		g.h.clock.Advance(time.Second)
	}
	var offer protocol.ForfeitAvailablePayload
	players[0].expect("forfeit_available", &offer)
	if offer.Team == nil || *offer.Team != 1 {
		t.Fatalf("oferta = %+v", offer)
	}
	players[0].send("claim_forfeit", nil)
	var ended protocol.MatchEndedPayload
	players[0].expect("match_ended", &ended)
	if ended.Reason != protocol.EndForfeit || *ended.Winner != 0 {
		t.Fatalf("match_ended = %+v", ended)
	}
	if status, winner := g.matchRow(started.MatchID); status != "finished" || *winner != 0 {
		t.Fatalf("partida no banco: %s %v", status, winner)
	}
}

func TestWebSocketAdminAbort(t *testing.T) {
	g := newGame(t)
	players, clients, started := g.startMatch()
	g.h.makeAdmin("ana")
	if r := clients[0].post("/api/admin/matches/"+started.MatchID+"/abort", map[string]string{"reason": "teste"}); r.status != http.StatusNoContent {
		t.Fatalf("abort = %d %s", r.status, r.body)
	}
	for _, p := range players {
		var ended protocol.MatchEndedPayload
		p.expect("match_ended", &ended)
		if ended.Reason != protocol.EndAborted {
			t.Fatalf("match_ended = %+v", ended)
		}
	}
}

func TestWebSocketRestartResumesMatch(t *testing.T) {
	g := newGame(t)
	players, clients, started := g.startMatch()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := g.lobby.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	for _, p := range players {
		if code := p.closedWith(); code != websocket.StatusGoingAway {
			t.Fatalf("fechamento no desligamento = %d", code)
		}
	}
	recovered, err := g.h.store.Recover(ctx, nil)
	if err != nil || len(recovered) != 1 || recovered[0].Record.ID != started.MatchID {
		t.Fatalf("recuperadas = %+v (%v)", recovered, err)
	}
	g.startLobby()
	if err := g.lobby.Restore(recovered); err != nil {
		t.Fatal(err)
	}
	var again []*player
	for i, c := range clients {
		p := g.connect(c)
		p.expect("hello", nil)
		var st protocol.RoomStatePayload
		p.expect("room_state", &st)
		if st.Status != protocol.RoomPlaying || int(st.YourSeat) != i || st.MatchID != started.MatchID {
			t.Fatalf("room_state = %+v", st)
		}
		again = append(again, p)
	}
	if result := g.playToEnd(again); result.Reason != protocol.EndGameOver {
		t.Fatalf("match_ended = %+v", result)
	}
	if status, _ := g.matchRow(started.MatchID); status != "finished" {
		t.Fatalf("status = %s", status)
	}
}

func TestWebSocketLogoutAllClosesConnection(t *testing.T) {
	g := newGame(t, func(l *ws.Limits) { l.Revalidate = 50 * time.Millisecond })
	c := g.h.signup("ana")
	p := g.connect(c)
	p.expect("hello", nil)
	if r := c.post("/api/auth/logout-all", nil); r.status != http.StatusNoContent {
		t.Fatalf("logout-all = %d %s", r.status, r.body)
	}
	if code := p.closedWith(); code != 4003 {
		t.Fatalf("fechamento = %d", code)
	}
}
```

- [ ] **Step 2: Rodar os testes ponta a ponta**

Run: `cd backend && TESTCONTAINERS_RYUK_DISABLED=true TRUCO_REQUIRE_DB=1 go test -race ./internal/httpapi/ -run WebSocket -count=1`
Expected: PASS. Se algum falhar, investigue com superpowers:systematic-debugging antes de mexer em qualquer código (não aumente timeouts para "fazer passar").

- [ ] **Step 3: Verificação final da branch**

Run (da raiz):

```bash
cd backend && go vet ./... && TESTCONTAINERS_RYUK_DISABLED=true TRUCO_REQUIRE_DB=1 go test -race ./... -count=1 && cd ..
make gen-sql && make gen-types && git diff --exit-code
grep -rn '"github.com/coder/websocket"' backend/internal --include='*.go' | grep -v '_test.go' | grep -v 'internal/transport/ws/'
go -C backend list -deps ./internal/engine | grep -v '^[a-z/]*$' | grep caarlosandree | grep -v internal/engine
```

Expected: testes verdes (integração executada, não pulada); `git diff --exit-code` sem diferenças nos arquivos gerados; os dois `grep` sem saída (só `transport/ws` importa a biblioteca; `engine` não importa nada do projeto).

- [ ] **Step 4: Commit**

```bash
git add backend/internal/httpapi/ws_e2e_test.go
git commit -m "test(ws): ponta a ponta com partida completa, reconexão, W.O., abort, restart e revogação de sessão"
```
