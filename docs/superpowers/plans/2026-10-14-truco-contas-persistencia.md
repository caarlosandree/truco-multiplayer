# Contas e Persistência do Truco — Plano de Implementação

> **Para agentes:** SUB-SKILL OBRIGATÓRIA: use superpowers:subagent-driven-development (recomendado) ou superpowers:executing-plans para implementar este plano tarefa por tarefa. Os passos usam checkbox (`- [ ]`) para acompanhamento.

**Objetivo:** Implementar contas (cadastro com verificação de e-mail, login, sessão revogável, recuperação e troca de senha), administração, persistência de partidas por seed + log de ações, reconstrução/recuperação de partidas, estatísticas, ranking e a API HTTP JSON correspondente.

**Arquitetura:** Pacotes puros (`replay`, `stats.Derive`, validação e senha em `auth`) são testados sem banco. A persistência usa `pgx/v5` com SQL escrito à mão e código gerado pelo `sqlc`; migrations `goose` embutidas no binário. Serviços (`auth.Service`, `match.Store`, `stats.Reader`, `admin.Service`) recebem relógio injetável e são testados contra Postgres real via testcontainers. `httpapi` só traduz HTTP ↔ serviços e é testado ponta a ponta com `httptest.Server`.

**Tech Stack:** Go 1.27 (`net/http`, `log/slog`), PostgreSQL 18, `github.com/jackc/pgx/v5` v5.11.0, `github.com/pressly/goose/v3` v3.29.0, `sqlc` v1.31.1 (via `go run`, exige `gcc`), `golang.org/x/crypto` v0.58.0 (argon2), `golang.org/x/time` v0.16.0 (rate), `github.com/testcontainers/testcontainers-go` v0.44.0 (+ módulo `postgres`), Resend (HTTP direto).

**Spec:** `docs/superpowers/specs/2026-10-14-truco-contas-persistencia-design.md`

## Restrições Globais

- Módulo Go: `github.com/caarlosandree/truco-multiplayer/backend`, em `backend/`.
- `internal/engine` continua importando **apenas** a stdlib.
- Identificadores em inglês; comentários, mensagens de erro, e-mails e mensagens da API em pt-BR.
- Branch de trabalho: `feat/contas-persistencia` (já existe, saiu de `staging`). Commits convencionais em pt-BR, **sem linhas de atribuição**.
- Nunca edite `backend/internal/db/dbgen/*` nem `frontend/src/protocol/*.ts` à mão: rode `make gen-sql` / `make gen-types`.
- Sessão: idle `30d`, teto absoluto `90d`, renovação no máximo a cada `24h`; cookie `__Host-truco_session` (ou `truco_session` com `COOKIE_SECURE=false`), `HttpOnly; Secure; SameSite=Lax; Path=/; Max-Age=7776000`.
- Tokens: 32 bytes de `crypto/rand` em base64url sem padding; banco guarda `sha256`. Verificação de e-mail vale `24h`; reset de senha `30min`.
- argon2id `m=65536 KiB, t=3, p=2`, salt 16, chave 32, no máximo 4 hashes simultâneos.
- Username `^[a-z0-9_.]{3,20}$` (minúsculas após normalizar); e-mail normalizado com trim + minúsculas; nome de exibição 1..30 runes; senha 10..128 caracteres, sem conter username nem parte local do e-mail.
- Rate limits: login 10/min por IP; 5 falhas em 15 min bloqueiam o identificador por 15 min; cadastro 5/h por IP; forgot/resend 5/h por IP e 3/h por e-mail; verify/reset 20/h por IP.
- Formato de erro HTTP: `{"error":{"code":"...","message":"...","fields":{...}}}`. Códigos da spec mais `invalid_request` (JSON malformado ou `Content-Type` errado — esclarecimento desta implementação).
- Testes de integração usam `internal/testdb`; sem Docker eles são pulados, exceto com `TRUCO_REQUIRE_DB=1` (CI), quando falham.
- Rodar testes sempre a partir de `backend/`: `go test ./...` (ou o pacote indicado).

## Foco de Revisão

1. **Identificador com maiúsculas/espaços** (`"  Ana@Empresa.COM "` no login, cadastro ou forgot): deve casar com a conta existente, nunca criar duplicata nem falhar. Teste na Tarefa 13.
2. **Link antigo ou reutilizado** (clicar duas vezes na verificação; usar o primeiro link de reset depois de pedir outro): `invalid_token`. Teste na Tarefa 13.
3. **Duas gravações concorrentes do mesmo `ply`**: exatamente uma passa, a outra recebe `ErrPlyConflict`. Teste na Tarefa 14.
4. **Conta desativada com sessão viva** (inclusive desativada direto no banco, sem apagar sessões): a próxima requisição é `401`. Teste na Tarefa 12.
5. **Replay de partida abandonada por log corrompido**: `409 conflict`, nunca `500`. Teste na Tarefa 19.

---

## Estrutura de Arquivos

```
docker-compose.yml                         # postgres:18 para desenvolvimento
.env.example                               # variáveis documentadas
Makefile                                   # + dev-db, run, migrate, gen-sql
.github/workflows/ci.yml                   # vet, test -race, diff de gerados
backend/sqlc.yaml
backend/cmd/server/main.go                 # subcomandos serve, migrate, promote-admin, recompute-stats
backend/internal/engine/version.go         # RulesVersion
backend/internal/engine/state.go           # + GameState.MarshalJSON (erro)
backend/internal/replay/replay.go          # Record, LoggedAction, Source, Status, Walk, Rebuild, erros
backend/internal/replay/frames.go          # Frame, BuildFrames (replay para o frontend)
backend/internal/enginetest/random.go      # RandomRecord: partidas aleatórias como a sala gravaria
backend/internal/stats/derive.go           # PlayerMatchStats, Derive, accumulator
backend/internal/stats/reader.go           # Reader: ranking, perfil, histórico
backend/internal/testclock/clock.go        # relógio falso para testes
backend/internal/config/config.go          # Load(getenv)
backend/internal/db/db.go                  # Open, Migrate, MigrateDown, WithTx
backend/internal/db/migrations/0001_contas.sql
backend/internal/db/migrations/0002_partidas.sql
backend/internal/db/migrations/0003_auditoria.sql
backend/internal/db/queries/{users,sessions,tokens,domains,matches,stats,admin}.sql
backend/internal/db/dbgen/                 # GERADO
backend/internal/testdb/testdb.go          # Run (TestMain), New, NewURL, CreateUser
backend/internal/auth/password.go          # Hasher argon2id
backend/internal/auth/token.go             # NewToken, HashToken
backend/internal/auth/validate.go          # normalização e validação, FieldErrors
backend/internal/auth/errors.go            # erros sentinela
backend/internal/auth/service.go           # Service, Options, User, BootstrapDomains
backend/internal/auth/session.go           # sessão, cookie, SessionFromRequest, CheckOrigin
backend/internal/auth/flows.go             # cadastro, verificação, login, reset, troca de senha
backend/internal/ratelimit/limiter.go      # Limiter (token bucket por chave)
backend/internal/ratelimit/lockout.go      # Lockout (falhas por janela)
backend/internal/mail/mail.go              # Message, Mailer, LogMailer, Fake
backend/internal/mail/resend.go            # ResendMailer
backend/internal/mail/templates.go         # e-mails em pt-BR
backend/internal/match/store.go            # Store: Create, AppendAction, Load, Abandon, Finish, Summary
backend/internal/match/recover.go          # Recover, RecomputeStats
backend/internal/admin/admin.go            # Service, MatchAborter, PromoteAdmin
backend/internal/httpapi/server.go         # Deps, Server, rotas, manutenção
backend/internal/httpapi/respond.go        # JSON, erros, decodeJSON
backend/internal/httpapi/middleware.go     # request id, recover, log, headers, sessão, Origin, auth
backend/internal/httpapi/auth_handlers.go
backend/internal/httpapi/read_handlers.go
backend/internal/httpapi/admin_handlers.go
backend/internal/httpapi/static.go         # SPA
```

Dependências entre pacotes: `engine` ← `replay` ← `enginetest`, `stats` ← `match` ← `admin` ← `httpapi` ← `cmd/server`; `auth` usa `db`, `mail`; `httpapi` usa `auth`, `ratelimit`, `stats`, `match`, `admin`, `replay`, `config`.

---

### Tarefa 1: Engine — versão de regras e proteção contra `json.Marshal`

**Files:**
- Create: `backend/internal/engine/version.go`
- Modify: `backend/internal/engine/state.go` (imports e fim do arquivo)
- Test: `backend/internal/engine/state_marshal_test.go`

**Interfaces:**
- Produces: `engine.RulesVersion` (const int = 1); `func (GameState) MarshalJSON() ([]byte, error)` sempre com erro.

- [ ] **Step 1: Escrever o teste que falha**

```go
package engine

import (
	"encoding/json"
	"testing"
)

func TestGameStateIsNotSerializable(t *testing.T) {
	g := newTestGame(1)
	if _, err := json.Marshal(g); err == nil {
		t.Fatal("json.Marshal(GameState) deveria falhar")
	}
	if _, err := json.Marshal(&g); err == nil {
		t.Fatal("json.Marshal(*GameState) deveria falhar")
	}
	g, _ = mustApply(t, g, Action{Kind: ActSysStartHand})
	if _, err := json.Marshal(ViewFor(g, 0)); err != nil {
		t.Fatalf("PlayerView deve continuar serializável: %v", err)
	}
}

func TestRulesVersionIsPositive(t *testing.T) {
	if RulesVersion < 1 {
		t.Fatalf("RulesVersion = %d", RulesVersion)
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/engine/ -run 'TestGameStateIsNotSerializable|TestRulesVersion' -v`
Expected: FAIL de compilação (`undefined: RulesVersion`).

- [ ] **Step 3: Implementar**

`backend/internal/engine/version.go`:

```go
package engine

// RulesVersion identifica a versão das regras. Incremente sempre que uma mudança
// fizer Apply produzir resultado diferente para um log de ações já gravado.
const RulesVersion = 1
```

Em `backend/internal/engine/state.go`, troque `import "slices"` por:

```go
import (
	"errors"
	"slices"
)
```

e acrescente ao fim do arquivo:

```go
var errStateNotSerializable = errors.New("engine: GameState é privado; use ViewFor")

// MarshalJSON impede que o estado completo vaze por um json.Marshal acidental.
// O estado só sai do servidor via ViewFor; o banco guarda ações, não estado.
func (GameState) MarshalJSON() ([]byte, error) { return nil, errStateNotSerializable }
```

- [ ] **Step 4: Rodar a suíte do engine e conferir tipos gerados**

Run: `cd backend && go test ./internal/engine/ ./internal/protocol/ && cd .. && make gen-types && git diff --stat frontend/`
Expected: testes PASS; nenhuma mudança em `frontend/` (o método não altera os tipos gerados).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/engine/version.go backend/internal/engine/state.go backend/internal/engine/state_marshal_test.go
git commit -m "feat(engine): adiciona RulesVersion e impede serialização de GameState"
```

---

### Tarefa 2: Pacote `replay` — log de ações, reconstrução e frames

**Files:**
- Create: `backend/internal/replay/replay.go`
- Create: `backend/internal/replay/frames.go`
- Test: `backend/internal/replay/replay_test.go`

**Interfaces:**
- Consumes: `engine.NewGame`, `engine.Apply`, `engine.ViewFor`, `engine.RulesVersion`.
- Produces:
  - `type Source string` com `SourcePlayer="player"`, `SourceTimeout="timeout"`, `SourceSystem="system"`; `func (Source) Valid() bool`.
  - `type Status string` com `StatusInProgress="in_progress"`, `StatusFinished="finished"`, `StatusAbandoned="abandoned"`.
  - `type PlayerSeat struct{ UserID, DisplayName string }`
  - `type LoggedAction struct{ Ply int; Action engine.Action; Source Source }`
  - `type Record struct{ ID string; Seed uint64; FirstDealer engine.Seat; EngineVersion int; Status Status; Players [4]PlayerSeat; Actions []LoggedAction }`
  - `type Step struct{ Logged LoggedAction; Before, After engine.GameState; Events []engine.Event }`
  - `var ErrIncompatibleVersion error`; `type CorruptLogError struct{ Ply int; Err error }` (com `Unwrap`).
  - `func Walk(rec Record, fn func(Step) error) (engine.GameState, error)`; `func Rebuild(rec Record) (engine.GameState, error)`.
  - `type Frame struct{ Ply int; Source Source; Action engine.Action; Events []engine.Event; Views [4]engine.PlayerView }` (tags JSON `ply, source, action, events, views`); `func BuildFrames(rec Record) ([]Frame, error)`.

- [ ] **Step 1: Escrever os testes que falham**

`backend/internal/replay/replay_test.go`:

```go
package replay

import (
	"errors"
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
)

var testSeats = [4]PlayerSeat{{"u0", "Ana"}, {"u1", "Bia"}, {"u2", "Caio"}, {"u3", "Duda"}}

func newRecord(actions ...LoggedAction) Record {
	return Record{ID: "m1", Seed: 42, FirstDealer: 0, EngineVersion: engine.RulesVersion,
		Status: StatusInProgress, Players: testSeats, Actions: actions}
}

func start(ply int) LoggedAction {
	return LoggedAction{Ply: ply, Action: engine.Action{Kind: engine.ActSysStartHand}, Source: SourceSystem}
}

func TestRebuildAppliesActionsInOrder(t *testing.T) {
	g, err := Rebuild(newRecord(start(0)))
	if err != nil {
		t.Fatal(err)
	}
	if g.Phase != engine.PhaseCutChoice {
		t.Fatalf("fase = %s, quer cut_choice", g.Phase)
	}
	if g.Players[2].ID != "u2" || g.Players[2].Name != "Caio" {
		t.Fatalf("jogador 2 = %+v", g.Players[2])
	}
}

func TestRebuildEmptyLogIsNewGame(t *testing.T) {
	g, err := Rebuild(newRecord())
	if err != nil || g.Phase != engine.PhaseWaiting || g.Seed != 42 {
		t.Fatalf("g=%+v err=%v", g.Phase, err)
	}
}

func TestRebuildRejectsIncompatibleVersion(t *testing.T) {
	rec := newRecord(start(0))
	rec.EngineVersion = engine.RulesVersion + 1
	if _, err := Rebuild(rec); !errors.Is(err, ErrIncompatibleVersion) {
		t.Fatalf("err = %v", err)
	}
}

func TestRebuildReportsIllegalActionPly(t *testing.T) {
	illegal := LoggedAction{Ply: 1, Action: engine.Action{Kind: engine.ActPlayCard, Seat: 0, Slot: 0}, Source: SourcePlayer}
	_, err := Rebuild(newRecord(start(0), illegal))
	var corrupt *CorruptLogError
	if !errors.As(err, &corrupt) || corrupt.Ply != 1 {
		t.Fatalf("err = %v", err)
	}
	var rule *engine.RuleError
	if !errors.As(err, &rule) {
		t.Fatalf("CorruptLogError deve embrulhar o RuleError: %v", err)
	}
}

func TestRebuildRejectsPlyGap(t *testing.T) {
	_, err := Rebuild(newRecord(start(0), start(2)))
	var corrupt *CorruptLogError
	if !errors.As(err, &corrupt) || corrupt.Ply != 1 {
		t.Fatalf("err = %v", err)
	}
}

func TestRebuildRejectsLoggedSysTimeout(t *testing.T) {
	timeout := LoggedAction{Ply: 1, Action: engine.Action{Kind: engine.ActSysTimeout, Seat: 3}, Source: SourceTimeout}
	_, err := Rebuild(newRecord(start(0), timeout))
	var corrupt *CorruptLogError
	if !errors.As(err, &corrupt) || corrupt.Ply != 1 {
		t.Fatalf("sys_timeout nunca é gravado; err = %v", err)
	}
}

func TestRebuildRejectsInvalidDealer(t *testing.T) {
	rec := newRecord()
	rec.FirstDealer = 4
	var corrupt *CorruptLogError
	if _, err := Rebuild(rec); !errors.As(err, &corrupt) {
		t.Fatalf("err = %v", err)
	}
}

func TestWalkReportsBeforeAfterAndEvents(t *testing.T) {
	var steps []Step
	_, err := Walk(newRecord(start(0)), func(s Step) error {
		steps = append(steps, s)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 1 {
		t.Fatalf("%d passos", len(steps))
	}
	s := steps[0]
	if s.Before.Phase != engine.PhaseWaiting || s.After.Phase != engine.PhaseCutChoice || len(s.Events) == 0 {
		t.Fatalf("passo = before %s, after %s, %d eventos", s.Before.Phase, s.After.Phase, len(s.Events))
	}
}

func TestWalkStopsOnCallbackError(t *testing.T) {
	stop := errors.New("pare")
	_, err := Walk(newRecord(start(0)), func(Step) error { return stop })
	if !errors.Is(err, stop) {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildFramesHasOneViewPerSeat(t *testing.T) {
	frames, err := BuildFrames(newRecord(start(0)))
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 1 || frames[0].Ply != 0 || frames[0].Source != SourceSystem {
		t.Fatalf("frames = %+v", frames)
	}
	for seat, view := range frames[0].Views {
		if view.Seat != engine.Seat(seat) || view.Phase != engine.PhaseCutChoice {
			t.Fatalf("view %d = seat %d fase %s", seat, view.Seat, view.Phase)
		}
	}
}

func TestSourceValid(t *testing.T) {
	for _, s := range []Source{SourcePlayer, SourceTimeout, SourceSystem} {
		if !s.Valid() {
			t.Fatalf("%q deveria ser válido", s)
		}
	}
	if Source("bot").Valid() {
		t.Fatal("bot não é uma origem válida")
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/replay/ -v`
Expected: FAIL de compilação (pacote sem código).

- [ ] **Step 3: Implementar**

`backend/internal/replay/replay.go`:

```go
// Package replay reconstrói partidas a partir de seed + log de ações. É puro:
// não faz I/O e depende só do engine.
package replay

import (
	"errors"
	"fmt"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
)

// Source indica quem originou uma ação gravada.
type Source string

const (
	SourcePlayer  Source = "player"
	SourceTimeout Source = "timeout" // ação padrão resolvida pela sala quando o tempo acaba
	SourceSystem  Source = "system"  // ações de sistema, como sys_start_hand
)

func (s Source) Valid() bool {
	return s == SourcePlayer || s == SourceTimeout || s == SourceSystem
}

// Status é o ciclo de vida de uma partida persistida.
type Status string

const (
	StatusInProgress Status = "in_progress"
	StatusFinished   Status = "finished"
	StatusAbandoned  Status = "abandoned"
)

type PlayerSeat struct {
	UserID      string
	DisplayName string
}

type LoggedAction struct {
	Ply    int
	Action engine.Action
	Source Source
}

// Record é tudo o que é preciso para reproduzir uma partida.
type Record struct {
	ID            string
	Seed          uint64
	FirstDealer   engine.Seat
	EngineVersion int
	Status        Status
	Players       [4]PlayerSeat
	Actions       []LoggedAction // ordenadas por Ply, a partir de 0
}

// Step é um passo da reconstrução.
type Step struct {
	Logged LoggedAction
	Before engine.GameState
	After  engine.GameState
	Events []engine.Event
}

var ErrIncompatibleVersion = errors.New("replay: versão de regras incompatível")

// CorruptLogError indica um log que não reproduz uma partida válida.
// Ply = -1 quando o problema está no cabeçalho do registro.
type CorruptLogError struct {
	Ply int
	Err error
}

func (e *CorruptLogError) Error() string {
	return fmt.Sprintf("replay: log corrompido no ply %d: %v", e.Ply, e.Err)
}

func (e *CorruptLogError) Unwrap() error { return e.Err }

func (r Record) enginePlayers() [4]engine.Player {
	var players [4]engine.Player
	for i, p := range r.Players {
		players[i] = engine.Player{ID: p.UserID, Name: p.DisplayName}
	}
	return players
}

// Walk reaplica o log no engine chamando fn a cada ação (fn pode ser nil).
// Devolve o estado após a última ação aplicada com sucesso.
func Walk(rec Record, fn func(Step) error) (engine.GameState, error) {
	if rec.EngineVersion != engine.RulesVersion {
		return engine.GameState{}, fmt.Errorf("%w: log v%d, engine v%d", ErrIncompatibleVersion, rec.EngineVersion, engine.RulesVersion)
	}
	if rec.FirstDealer > 3 {
		return engine.GameState{}, &CorruptLogError{Ply: -1, Err: fmt.Errorf("dealer inicial inválido: %d", rec.FirstDealer)}
	}
	g := engine.NewGame(rec.enginePlayers(), rec.Seed, rec.FirstDealer)
	for i, logged := range rec.Actions {
		if logged.Ply != i {
			return g, &CorruptLogError{Ply: i, Err: fmt.Errorf("ply esperado %d, encontrado %d", i, logged.Ply)}
		}
		if logged.Action.Kind == engine.ActSysTimeout {
			return g, &CorruptLogError{Ply: i, Err: errors.New("sys_timeout não deve ser gravado; grave a ação concreta")}
		}
		next, evs, err := engine.Apply(g, logged.Action)
		if err != nil {
			return g, &CorruptLogError{Ply: i, Err: err}
		}
		if fn != nil {
			if err := fn(Step{Logged: logged, Before: g, After: next, Events: evs}); err != nil {
				return next, err
			}
		}
		g = next
	}
	return g, nil
}

// Rebuild reconstrói o estado final do log.
func Rebuild(rec Record) (engine.GameState, error) { return Walk(rec, nil) }
```

`backend/internal/replay/frames.go`:

```go
package replay

import "github.com/caarlosandree/truco-multiplayer/backend/internal/engine"

// Frame é um passo do replay entregue ao frontend: a ação, todos os eventos e a
// visão de cada assento depois dela. Só é montado para partidas encerradas.
type Frame struct {
	Ply     int                  `json:"ply"`
	Source  Source               `json:"source"`
	Action  engine.Action        `json:"action"`
	Events  []engine.Event       `json:"events"`
	Views   [4]engine.PlayerView `json:"views"`
}

func BuildFrames(rec Record) ([]Frame, error) {
	frames := make([]Frame, 0, len(rec.Actions))
	_, err := Walk(rec, func(s Step) error {
		frame := Frame{Ply: s.Logged.Ply, Source: s.Logged.Source, Action: s.Logged.Action, Events: s.Events}
		for seat := engine.Seat(0); seat < 4; seat++ {
			frame.Views[seat] = engine.ViewFor(s.After, seat)
		}
		frames = append(frames, frame)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return frames, nil
}
```

- [ ] **Step 4: Rodar e ver passar**

Run: `cd backend && go test ./internal/replay/ -v && go vet ./internal/replay/`
Expected: PASS em todos.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/replay
git commit -m "feat(replay): adiciona reconstrução de partidas a partir do log de ações"
```

---

### Tarefa 3: Pacote `enginetest` — partidas aleatórias como a sala gravaria

**Files:**
- Create: `backend/internal/enginetest/random.go`
- Test: `backend/internal/replay/roundtrip_test.go` (pacote externo `replay_test`)
- Test: `backend/internal/enginetest/random_test.go`

**Interfaces:**
- Consumes: `replay.Record`, `replay.LoggedAction`, `replay.Source*`, `replay.Rebuild`, `engine.DefaultAction`.
- Produces: `var enginetest.Players [4]replay.PlayerSeat`; `func enginetest.RandomRecord(seed uint64) (replay.Record, engine.GameState)` — log completo até `GameOver` (timeouts resolvidos em ação concreta com `SourceTimeout`, `sys_start_hand` com `SourceSystem`) e o estado final **ao vivo**. `Record.Status = StatusInProgress`, `ID = "seed-<n>"`, `FirstDealer = seed % 4`.

> Este pacote é código de apoio a testes (não é `_test.go` porque é importado por vários pacotes). Ele nunca é importado pelo servidor.

- [ ] **Step 1: Escrever os testes que falham**

`backend/internal/enginetest/random_test.go`:

```go
package enginetest

import (
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
)

func TestRandomRecordPlaysToGameOver(t *testing.T) {
	sawTimeout, sawSpecialHand := false, false
	for seed := range uint64(150) {
		rec, final := RandomRecord(seed)
		if final.Phase != engine.PhaseGameOver {
			t.Fatalf("seed %d: fase final %s", seed, final.Phase)
		}
		if len(rec.Actions) == 0 || rec.Actions[0].Action.Kind != engine.ActSysStartHand || rec.Actions[0].Source != replay.SourceSystem {
			t.Fatalf("seed %d: primeira ação deve ser sys_start_hand do sistema", seed)
		}
		for i, logged := range rec.Actions {
			if logged.Ply != i || !logged.Source.Valid() || logged.Action.Kind == engine.ActSysTimeout {
				t.Fatalf("seed %d ply %d: %+v", seed, i, logged)
			}
			if logged.Source == replay.SourceTimeout {
				sawTimeout = true
			}
		}
		_, _ = replay.Walk(rec, func(s replay.Step) error {
			if s.Logged.Action.Kind == engine.ActSysStartHand && s.After.Hand != nil && s.After.Hand.Kind != engine.HandNormal {
				sawSpecialHand = true
			}
			return nil
		})
	}
	if !sawTimeout {
		t.Fatal("nenhuma partida gerou timeout; o gerador deve exercitá-los")
	}
	if !sawSpecialHand {
		t.Fatal("nenhuma partida chegou a uma Mão de 11 ou de Ferro")
	}
}

func TestRandomRecordIsDeterministic(t *testing.T) {
	a, _ := RandomRecord(7)
	b, _ := RandomRecord(7)
	if len(a.Actions) != len(b.Actions) {
		t.Fatal("mesma seed deve gerar o mesmo log")
	}
}
```

`backend/internal/replay/roundtrip_test.go`:

```go
package replay_test

import (
	"reflect"
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/enginetest"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
)

func TestRebuildMatchesLiveStateForRandomGames(t *testing.T) {
	games := 300
	if testing.Short() {
		games = 50
	}
	for seed := range uint64(games) {
		rec, live := enginetest.RandomRecord(seed)
		rebuilt, err := replay.Rebuild(rec)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		if !reflect.DeepEqual(rebuilt, live) {
			t.Fatalf("seed %d: estado reconstruído diverge do estado ao vivo", seed)
		}
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/enginetest/ ./internal/replay/`
Expected: FAIL de compilação (`undefined: RandomRecord`).

- [ ] **Step 3: Implementar**

`backend/internal/enginetest/random.go`:

```go
// Package enginetest gera partidas completas com ações legais aleatórias, no
// formato em que a sala as grava. Uso exclusivo de testes.
package enginetest

import (
	"fmt"
	"math/rand/v2"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
)

var Players = [4]replay.PlayerSeat{
	{UserID: "u0", DisplayName: "Ana"}, {UserID: "u1", DisplayName: "Bia"},
	{UserID: "u2", DisplayName: "Caio"}, {UserID: "u3", DisplayName: "Duda"},
}

type candidate struct {
	action engine.Action
	source replay.Source
}

// RandomRecord joga uma partida até o fim e devolve o log e o estado final ao vivo.
func RandomRecord(seed uint64) (replay.Record, engine.GameState) {
	rng := rand.New(rand.NewPCG(seed, 7))
	rec := replay.Record{
		ID: fmt.Sprintf("seed-%d", seed), Seed: seed, FirstDealer: engine.Seat(seed % 4),
		EngineVersion: engine.RulesVersion, Status: replay.StatusInProgress, Players: Players,
	}
	players := [4]engine.Player{}
	for i, p := range Players {
		players[i] = engine.Player{ID: p.UserID, Name: p.DisplayName}
	}
	g := engine.NewGame(players, seed, rec.FirstDealer)
	for step := 0; g.Phase != engine.PhaseGameOver; step++ {
		if step > 5000 {
			panic(fmt.Sprintf("enginetest: seed %d não terminou", seed))
		}
		options := candidates(g, rng)
		rng.Shuffle(len(options), func(i, j int) { options[i], options[j] = options[j], options[i] })
		applied := false
		for _, c := range options {
			next, _, err := engine.Apply(g, c.action)
			if err != nil {
				continue
			}
			rec.Actions = append(rec.Actions, replay.LoggedAction{Ply: len(rec.Actions), Action: c.action, Source: c.source})
			g, applied = next, true
			break
		}
		if !applied {
			panic(fmt.Sprintf("enginetest: seed %d sem ação legal na fase %s", seed, g.Phase))
		}
	}
	return rec, g
}

func candidates(g engine.GameState, rng *rand.Rand) []candidate {
	if g.Phase == engine.PhaseWaiting || g.Phase == engine.PhaseHandEnd {
		return []candidate{{engine.Action{Kind: engine.ActSysStartHand}, replay.SourceSystem}}
	}
	player := func(a engine.Action) candidate { return candidate{a, replay.SourcePlayer} }
	var out []candidate
	for s := engine.Seat(0); s < 4; s++ {
		out = append(out,
			player(engine.Action{Kind: engine.ActChooseCard, Seat: s, Keep: 0}),
			player(engine.Action{Kind: engine.ActChooseCard, Seat: s, Keep: 1}),
			player(engine.Action{Kind: engine.ActPassBurn, Seat: s}),
		)
		for _, choice := range []engine.VoteChoice{engine.VoteAccept, engine.VoteRun, engine.VoteRaise, engine.VotePlay} {
			out = append(out, player(engine.Action{Kind: engine.ActVote, Seat: s, Choice: choice}))
		}
		for slot := range 3 {
			out = append(out,
				player(engine.Action{Kind: engine.ActPlayCard, Seat: s, Slot: slot}),
				player(engine.Action{Kind: engine.ActPlayCard, Seat: s, Slot: slot, Covered: true}),
			)
		}
		for slot, card := range g.Hand.Hands[s] {
			if card.Rank == 4 {
				out = append(out, player(engine.Action{Kind: engine.ActBurn, Seat: s, Slots: []int{slot}}))
			}
		}
		// Como a sala: o timeout vira a ação concreta de DefaultAction.
		if fallback, ok := engine.DefaultAction(g, s); ok && rng.IntN(10) == 0 {
			out = append(out, candidate{fallback, replay.SourceTimeout})
		}
	}
	// Truco em mão especial é punido e encerra a partida; raramente, para exercitar a punição.
	if g.Hand.Kind == engine.HandNormal || rng.IntN(200) == 0 {
		out = append(out, player(engine.Action{Kind: engine.ActRequestRaise, Seat: g.Hand.Turn}))
	}
	return out
}
```

- [ ] **Step 4: Rodar e ver passar**

Run: `cd backend && go test ./internal/enginetest/ ./internal/replay/ -v -run 'Random|Rebuild'`
Expected: PASS. Se `TestRandomRecordPlaysToGameOver` reclamar de Mão de 11 ausente, aumente o número de seeds do teste para 300 (não altere o gerador).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/enginetest backend/internal/replay/roundtrip_test.go
git commit -m "test(replay): verifica reconstrução bit a bit com partidas aleatórias"
```

---

### Tarefa 4: `stats.Derive` — estatísticas puras por partida

**Files:**
- Create: `backend/internal/stats/derive.go`
- Test: `backend/internal/stats/derive_test.go` (pacote interno `stats`)
- Test: `backend/internal/stats/derive_random_test.go` (pacote externo `stats_test`)

**Interfaces:**
- Consumes: `replay.Walk`, `replay.Record`, `replay.Step`, `replay.SourceTimeout`.
- Produces:

```go
type PlayerMatchStats struct {
	UserID                                       string
	Team                                         engine.Team
	Won                                          bool
	PointsFor, PointsAgainst                     int
	HandsPlayed, HandsWon                        int
	RaisesCalled, RaisesFolded, RaisesWonByFold  int
	ElevenHands, ElevenRuns, ElevenWon           int
	IronHands, IronWon                           int
	Punished, Timeouts                           int
}
var ErrNotFinished error
func Derive(rec replay.Record) ([4]PlayerMatchStats, error)
```

Definições (spec §7.4): contadores de dupla (`HandsWon`, `RaisesFolded`, `RaisesWonByFold`, `Eleven*`, `Iron*`) são iguais para os dois parceiros; `RaisesCalled`, `Punished` e `Timeouts` são individuais; `HandsPlayed` conta eventos `hand_ended` (mão encerrada por punição não emite `hand_ended` e não conta).

- [ ] **Step 1: Escrever os testes unitários do acumulador**

`backend/internal/stats/derive_test.go`:

```go
package stats

import (
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
)

func teamPtr(t engine.Team) *engine.Team { return &t }
func seatPtr(s engine.Seat) *engine.Seat { return &s }

func act(kind engine.ActionKind, seat engine.Seat) replay.LoggedAction {
	return replay.LoggedAction{Action: engine.Action{Kind: kind, Seat: seat}, Source: replay.SourcePlayer}
}

func startHandStep(kind engine.HandKind, score [2]int) replay.Step {
	return replay.Step{
		Logged: replay.LoggedAction{Action: engine.Action{Kind: engine.ActSysStartHand}, Source: replay.SourceSystem},
		After:  engine.GameState{Score: score, Hand: &engine.HandState{Kind: kind}},
	}
}

func eventStep(evs ...engine.Event) replay.Step {
	return replay.Step{Logged: act(engine.ActPlayCard, 0), Events: evs}
}

func TestAccumulatorElevenHandWonByTeamWithEleven(t *testing.T) {
	var a accumulator
	a.observe(startHandStep(engine.HandEleven, [2]int{11, 5}))
	a.observe(eventStep(engine.Event{Kind: engine.EvHandEnded, Team: teamPtr(0)}))
	if a.team[0].elevenHands != 1 || a.team[1].elevenHands != 0 {
		t.Fatalf("elevenHands = %d/%d", a.team[0].elevenHands, a.team[1].elevenHands)
	}
	if a.team[0].elevenWon != 1 || a.team[0].handsWon != 1 || a.handsPlayed != 1 {
		t.Fatalf("contadores = %+v, handsPlayed %d", a.team[0], a.handsPlayed)
	}
}

func TestAccumulatorElevenRunGivesHandToOpponent(t *testing.T) {
	var a accumulator
	a.observe(startHandStep(engine.HandEleven, [2]int{3, 11}))
	a.observe(eventStep(
		engine.Event{Kind: engine.EvElevenDecided, Team: teamPtr(1), Choice: engine.VoteRun},
		engine.Event{Kind: engine.EvHandEnded, Team: teamPtr(0), Stake: 1},
	))
	if a.team[1].elevenRuns != 1 || a.team[1].elevenWon != 0 || a.team[0].handsWon != 1 {
		t.Fatalf("time0 %+v time1 %+v", a.team[0], a.team[1])
	}
}

func TestAccumulatorIronHand(t *testing.T) {
	var a accumulator
	a.observe(startHandStep(engine.HandIron, [2]int{11, 11}))
	a.observe(eventStep(engine.Event{Kind: engine.EvHandEnded, Team: teamPtr(1)}))
	if a.team[0].ironHands != 1 || a.team[1].ironHands != 1 || a.team[1].ironWon != 1 || a.team[0].ironWon != 0 {
		t.Fatalf("time0 %+v time1 %+v", a.team[0], a.team[1])
	}
}

func TestAccumulatorRaiseFold(t *testing.T) {
	var a accumulator
	a.observe(eventStep(engine.Event{Kind: engine.EvRaiseResolved, Team: teamPtr(1), Choice: engine.VoteRun}))
	a.observe(eventStep(engine.Event{Kind: engine.EvRaiseResolved, Team: teamPtr(0), Choice: engine.VoteAccept}))
	if a.team[1].raisesFolded != 1 || a.team[0].raisesWonByFold != 1 || a.team[0].raisesFolded != 0 {
		t.Fatalf("time0 %+v time1 %+v", a.team[0], a.team[1])
	}
}

func TestAccumulatorIndividualCounters(t *testing.T) {
	var a accumulator
	a.observe(replay.Step{Logged: act(engine.ActRequestRaise, 1)})
	a.observe(replay.Step{Logged: replay.LoggedAction{Action: engine.Action{Kind: engine.ActVote, Seat: 2, Choice: engine.VoteRaise}, Source: replay.SourcePlayer}})
	a.observe(replay.Step{Logged: replay.LoggedAction{Action: engine.Action{Kind: engine.ActVote, Seat: 2, Choice: engine.VoteAccept}, Source: replay.SourcePlayer}})
	a.observe(replay.Step{Logged: replay.LoggedAction{Action: engine.Action{Kind: engine.ActPassBurn, Seat: 3}, Source: replay.SourceTimeout}})
	a.observe(eventStep(engine.Event{Kind: engine.EvRaisePunished, Seat: seatPtr(0), Team: teamPtr(0)}))
	if a.seat[1].raisesCalled != 1 || a.seat[2].raisesCalled != 1 || a.seat[3].timeouts != 1 || a.seat[0].punished != 1 {
		t.Fatalf("assentos = %+v", a.seat)
	}
}

func TestAccumulatorTiedHandCountsOnlyAsPlayed(t *testing.T) {
	var a accumulator
	a.observe(eventStep(engine.Event{Kind: engine.EvHandEnded}))
	if a.handsPlayed != 1 || a.team[0].handsWon != 0 || a.team[1].handsWon != 0 {
		t.Fatalf("handsPlayed %d, vitórias %d/%d", a.handsPlayed, a.team[0].handsWon, a.team[1].handsWon)
	}
}
```

- [ ] **Step 2: Escrever os testes com partidas aleatórias**

`backend/internal/stats/derive_random_test.go`:

```go
package stats_test

import (
	"errors"
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/enginetest"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/stats"
)

func TestDeriveInvariantsOnRandomGames(t *testing.T) {
	for seed := range uint64(200) {
		rec, final := enginetest.RandomRecord(seed)
		got, err := stats.Derive(rec)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		winners, timeouts, raises := 0, 0, 0
		for _, logged := range rec.Actions {
			if logged.Source == replay.SourceTimeout {
				timeouts++
			}
			a := logged.Action
			if a.Kind == engine.ActRequestRaise || (a.Kind == engine.ActVote && a.Choice == engine.VoteRaise) {
				raises++
			}
		}
		sumTimeouts, sumRaises := 0, 0
		for seat, s := range got {
			team := engine.TeamOf(engine.Seat(seat))
			if s.Team != team || s.UserID != rec.Players[seat].UserID {
				t.Fatalf("seed %d assento %d: %+v", seed, seat, s)
			}
			if s.PointsFor != final.Score[team] || s.PointsAgainst != final.Score[engine.Other(team)] {
				t.Fatalf("seed %d: pontos %d/%d, placar %v", seed, s.PointsFor, s.PointsAgainst, final.Score)
			}
			if s.Won {
				winners++
			}
			partner := got[(seat+2)%4]
			if s.HandsWon != partner.HandsWon || s.RaisesFolded != partner.RaisesFolded || s.ElevenWon != partner.ElevenWon || s.Won != partner.Won {
				t.Fatalf("seed %d: parceiros com contadores de dupla diferentes", seed)
			}
			sumTimeouts += s.Timeouts
			sumRaises += s.RaisesCalled
		}
		if winners != 2 {
			t.Fatalf("seed %d: %d vencedores (quer a dupla inteira)", seed, winners)
		}
		if got[0].HandsWon+got[1].HandsWon > got[0].HandsPlayed {
			t.Fatalf("seed %d: mãos vencidas > jogadas", seed)
		}
		if sumTimeouts != timeouts || sumRaises != raises {
			t.Fatalf("seed %d: timeouts %d/%d, aumentos %d/%d", seed, sumTimeouts, timeouts, sumRaises, raises)
		}
	}
}

func TestDeriveRejectsUnfinishedGame(t *testing.T) {
	rec, _ := enginetest.RandomRecord(3)
	rec.Actions = rec.Actions[:len(rec.Actions)/2]
	if _, err := stats.Derive(rec); !errors.Is(err, stats.ErrNotFinished) {
		t.Fatalf("err = %v", err)
	}
}

func TestDerivePropagatesCorruptLog(t *testing.T) {
	rec, _ := enginetest.RandomRecord(3)
	rec.Actions[1].Action = engine.Action{Kind: engine.ActRequestRaise, Seat: 0}
	var corrupt *replay.CorruptLogError
	if _, err := stats.Derive(rec); !errors.As(err, &corrupt) {
		t.Fatalf("err = %v", err)
	}
}
```

- [ ] **Step 3: Rodar e ver falhar**

Run: `cd backend && go test ./internal/stats/`
Expected: FAIL de compilação.

- [ ] **Step 4: Implementar**

`backend/internal/stats/derive.go`:

```go
// Package stats deriva estatísticas de partidas a partir do log e consulta o ranking.
package stats

import (
	"errors"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
)

type PlayerMatchStats struct {
	UserID          string
	Team            engine.Team
	Won             bool
	PointsFor       int
	PointsAgainst   int
	HandsPlayed     int
	HandsWon        int
	RaisesCalled    int
	RaisesFolded    int
	RaisesWonByFold int
	ElevenHands     int
	ElevenRuns      int
	ElevenWon       int
	IronHands       int
	IronWon         int
	Punished        int
	Timeouts        int
}

var ErrNotFinished = errors.New("stats: partida não terminou")

// Derive percorre o log e conta, por assento, as estatísticas da partida.
func Derive(rec replay.Record) ([4]PlayerMatchStats, error) {
	var acc accumulator
	final, err := replay.Walk(rec, func(s replay.Step) error {
		acc.observe(s)
		return nil
	})
	if err != nil {
		return [4]PlayerMatchStats{}, err
	}
	if final.Phase != engine.PhaseGameOver || final.Winner == nil {
		return [4]PlayerMatchStats{}, ErrNotFinished
	}
	return acc.result(rec, final), nil
}

type seatCounters struct {
	raisesCalled, punished, timeouts int
}

type teamCounters struct {
	handsWon, raisesFolded, raisesWonByFold int
	elevenHands, elevenRuns, elevenWon      int
	ironHands, ironWon                      int
}

// accumulator guarda a mão corrente para atribuir resultados de Mão de 11/Ferro.
type accumulator struct {
	seat        [4]seatCounters
	team        [2]teamCounters
	handsPlayed int
	handKind    engine.HandKind
	elevenTeam  *engine.Team
}

func (a *accumulator) observe(s replay.Step) {
	action := s.Logged.Action
	if s.Logged.Source == replay.SourceTimeout {
		a.seat[action.Seat].timeouts++
	}
	isRaise := action.Kind == engine.ActRequestRaise || (action.Kind == engine.ActVote && action.Choice == engine.VoteRaise)
	if isRaise {
		a.seat[action.Seat].raisesCalled++
	}
	if action.Kind == engine.ActSysStartHand && s.After.Hand != nil {
		a.startHand(s.After)
	}
	for _, e := range s.Events {
		a.event(e)
	}
}

func (a *accumulator) startHand(g engine.GameState) {
	a.handKind, a.elevenTeam = g.Hand.Kind, nil
	switch g.Hand.Kind {
	case engine.HandEleven:
		for t := engine.Team(0); t < 2; t++ {
			if g.Score[t] == engine.WinningScore-1 {
				team := t
				a.elevenTeam = &team
				a.team[t].elevenHands++
			}
		}
	case engine.HandIron:
		a.team[0].ironHands++
		a.team[1].ironHands++
	}
}

func (a *accumulator) event(e engine.Event) {
	switch e.Kind {
	case engine.EvHandEnded:
		a.handsPlayed++
		if e.Team == nil {
			return
		}
		winner := *e.Team
		a.team[winner].handsWon++
		if a.handKind == engine.HandEleven && a.elevenTeam != nil && *a.elevenTeam == winner {
			a.team[winner].elevenWon++
		}
		if a.handKind == engine.HandIron {
			a.team[winner].ironWon++
		}
	case engine.EvRaiseResolved:
		if e.Choice == engine.VoteRun && e.Team != nil {
			a.team[*e.Team].raisesFolded++
			a.team[engine.Other(*e.Team)].raisesWonByFold++
		}
	case engine.EvElevenDecided:
		if e.Choice == engine.VoteRun && e.Team != nil {
			a.team[*e.Team].elevenRuns++
		}
	case engine.EvRaisePunished:
		if e.Seat != nil {
			a.seat[*e.Seat].punished++
		}
	}
}

func (a *accumulator) result(rec replay.Record, final engine.GameState) [4]PlayerMatchStats {
	var out [4]PlayerMatchStats
	for seat := range 4 {
		team := engine.TeamOf(engine.Seat(seat))
		tc, sc := a.team[team], a.seat[seat]
		out[seat] = PlayerMatchStats{
			UserID: rec.Players[seat].UserID, Team: team, Won: *final.Winner == team,
			PointsFor: final.Score[team], PointsAgainst: final.Score[engine.Other(team)],
			HandsPlayed: a.handsPlayed, HandsWon: tc.handsWon,
			RaisesCalled: sc.raisesCalled, RaisesFolded: tc.raisesFolded, RaisesWonByFold: tc.raisesWonByFold,
			ElevenHands: tc.elevenHands, ElevenRuns: tc.elevenRuns, ElevenWon: tc.elevenWon,
			IronHands: tc.ironHands, IronWon: tc.ironWon,
			Punished: sc.punished, Timeouts: sc.timeouts,
		}
	}
	return out
}
```

- [ ] **Step 5: Rodar e ver passar**

Run: `cd backend && go test ./internal/stats/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/stats
git commit -m "feat(stats): deriva estatísticas por jogador a partir do log da partida"
```

---
### Tarefa 5: Relógio de teste, configuração e ambiente de desenvolvimento

**Files:**
- Create: `backend/internal/testclock/clock.go`
- Create: `backend/internal/config/config.go`
- Test: `backend/internal/config/config_test.go`
- Create: `docker-compose.yml`, `.env.example`
- Modify: `Makefile`

**Interfaces:**
- Produces:
  - `testclock.New(t time.Time) *testclock.Clock`; métodos `Now() time.Time`, `Advance(d time.Duration)` (seguro para goroutines).
  - `type config.MailDriver string` (`MailResend="resend"`, `MailLog="log"`).
  - `type config.Config struct{ DatabaseURL, HTTPAddr, AppBaseURL, AppOrigin string; CookieSecure, TrustProxy bool; AllowedEmailDomains []string; MailDriver MailDriver; ResendAPIKey, MailFrom string; RankingMinMatches int; MigrateOnStart bool; LogLevel slog.Level; LogFormat string; StaticDir string }`
  - `func config.Load(getenv func(string) string) (Config, error)` — todos os problemas juntos via `errors.Join`.

- [ ] **Step 1: Criar o relógio de teste**

`backend/internal/testclock/clock.go`:

```go
// Package testclock fornece um relógio controlável para testes de expiração.
package testclock

import (
	"sync"
	"time"
)

type Clock struct {
	mu  sync.Mutex
	now time.Time
}

func New(t time.Time) *Clock { return &Clock{now: t} }

func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *Clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}
```

- [ ] **Step 2: Escrever os testes de configuração que falham**

`backend/internal/config/config_test.go`:

```go
package config

import (
	"log/slog"
	"strings"
	"testing"
)

func env(values map[string]string) func(string) string {
	return func(k string) string { return values[k] }
}

func validEnv() map[string]string {
	return map[string]string{
		"DATABASE_URL":          "postgres://truco:truco@localhost:5432/truco?sslmode=disable",
		"APP_BASE_URL":          "https://truco.empresa.com.br/",
		"ALLOWED_EMAIL_DOMAINS": " Empresa.com.br, @filial.com.br ,,",
		"RESEND_API_KEY":        "re_123",
		"MAIL_FROM":             "Truco <truco@empresa.com.br>",
	}
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(env(validEnv()))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != ":8080" || !cfg.CookieSecure || cfg.TrustProxy || !cfg.MigrateOnStart {
		t.Fatalf("padrões errados: %+v", cfg)
	}
	if cfg.AppBaseURL != "https://truco.empresa.com.br" || cfg.AppOrigin != "https://truco.empresa.com.br" {
		t.Fatalf("base %q origin %q", cfg.AppBaseURL, cfg.AppOrigin)
	}
	if strings.Join(cfg.AllowedEmailDomains, ",") != "empresa.com.br,filial.com.br" {
		t.Fatalf("domínios = %v", cfg.AllowedEmailDomains)
	}
	if cfg.MailDriver != MailResend || cfg.RankingMinMatches != 5 || cfg.LogLevel != slog.LevelInfo || cfg.LogFormat != "json" {
		t.Fatalf("padrões errados: %+v", cfg)
	}
}

func TestLoadOverrides(t *testing.T) {
	values := validEnv()
	values["HTTP_ADDR"] = "127.0.0.1:9000"
	values["COOKIE_SECURE"] = "false"
	values["TRUST_PROXY"] = "true"
	values["MAIL_DRIVER"] = "log"
	values["RESEND_API_KEY"] = ""
	values["MAIL_FROM"] = ""
	values["RANKING_MIN_MATCHES"] = "1"
	values["MIGRATE_ON_START"] = "false"
	values["LOG_LEVEL"] = "debug"
	values["LOG_FORMAT"] = "text"
	values["STATIC_DIR"] = "../frontend/dist"
	values["APP_BASE_URL"] = "http://localhost:5173"
	cfg, err := Load(env(values))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CookieSecure || !cfg.TrustProxy || cfg.MailDriver != MailLog || cfg.RankingMinMatches != 1 ||
		cfg.MigrateOnStart || cfg.LogLevel != slog.LevelDebug || cfg.LogFormat != "text" ||
		cfg.StaticDir != "../frontend/dist" || cfg.AppOrigin != "http://localhost:5173" {
		t.Fatalf("overrides não aplicados: %+v", cfg)
	}
}

func TestLoadReportsAllProblems(t *testing.T) {
	values := map[string]string{
		"APP_BASE_URL":        "ftp://x",
		"COOKIE_SECURE":       "talvez",
		"MAIL_DRIVER":         "smtp",
		"RANKING_MIN_MATCHES": "0",
		"LOG_FORMAT":          "xml",
		"LOG_LEVEL":           "barulhento",
	}
	_, err := Load(env(values))
	if err == nil {
		t.Fatal("esperava erro")
	}
	for _, want := range []string{"DATABASE_URL", "APP_BASE_URL", "COOKIE_SECURE", "MAIL_DRIVER", "RANKING_MIN_MATCHES", "LOG_FORMAT", "LOG_LEVEL"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("erro não menciona %s: %v", want, err)
		}
	}
}

func TestLoadRequiresResendSettings(t *testing.T) {
	values := validEnv()
	values["RESEND_API_KEY"] = ""
	values["MAIL_FROM"] = ""
	_, err := Load(env(values))
	if err == nil || !strings.Contains(err.Error(), "RESEND_API_KEY") || !strings.Contains(err.Error(), "MAIL_FROM") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadRejectsBaseURLWithPath(t *testing.T) {
	values := validEnv()
	values["APP_BASE_URL"] = "https://empresa.com.br/truco"
	if _, err := Load(env(values)); err == nil || !strings.Contains(err.Error(), "APP_BASE_URL") {
		t.Fatalf("err = %v", err)
	}
}
```

- [ ] **Step 3: Rodar e ver falhar**

Run: `cd backend && go test ./internal/config/`
Expected: FAIL de compilação.

- [ ] **Step 4: Implementar**

`backend/internal/config/config.go`:

```go
// Package config carrega a configuração do servidor a partir de variáveis de ambiente.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
)

type MailDriver string

const (
	MailResend MailDriver = "resend"
	MailLog    MailDriver = "log"
)

type Config struct {
	DatabaseURL         string
	HTTPAddr            string
	AppBaseURL          string // sem barra final
	AppOrigin           string // esquema + host, usado na checagem de Origin
	CookieSecure        bool
	TrustProxy          bool
	AllowedEmailDomains []string
	MailDriver          MailDriver
	ResendAPIKey        string
	MailFrom            string
	RankingMinMatches   int
	MigrateOnStart      bool
	LogLevel            slog.Level
	LogFormat           string
	StaticDir           string
}

// Load lê e valida a configuração; devolve todos os problemas de uma vez.
func Load(getenv func(string) string) (Config, error) {
	var errs []error
	get := func(key, fallback string) string {
		if v := strings.TrimSpace(getenv(key)); v != "" {
			return v
		}
		return fallback
	}
	parseBool := func(key string, fallback bool) bool {
		raw := get(key, "")
		if raw == "" {
			return fallback
		}
		v, err := strconv.ParseBool(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: valor booleano inválido %q", key, raw))
			return fallback
		}
		return v
	}

	cfg := Config{
		DatabaseURL:    get("DATABASE_URL", ""),
		HTTPAddr:       get("HTTP_ADDR", ":8080"),
		CookieSecure:   parseBool("COOKIE_SECURE", true),
		TrustProxy:     parseBool("TRUST_PROXY", false),
		MailDriver:     MailDriver(get("MAIL_DRIVER", string(MailResend))),
		ResendAPIKey:   get("RESEND_API_KEY", ""),
		MailFrom:       get("MAIL_FROM", ""),
		MigrateOnStart: parseBool("MIGRATE_ON_START", true),
		LogFormat:      get("LOG_FORMAT", "json"),
		StaticDir:      get("STATIC_DIR", ""),
	}
	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL é obrigatória"))
	}

	base, origin, err := parseBaseURL(get("APP_BASE_URL", ""))
	if err != nil {
		errs = append(errs, err)
	}
	cfg.AppBaseURL, cfg.AppOrigin = base, origin

	for _, d := range strings.Split(get("ALLOWED_EMAIL_DOMAINS", ""), ",") {
		d = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(d)), "@")
		if d != "" {
			cfg.AllowedEmailDomains = append(cfg.AllowedEmailDomains, d)
		}
	}

	switch cfg.MailDriver {
	case MailResend:
		if cfg.ResendAPIKey == "" {
			errs = append(errs, errors.New("RESEND_API_KEY é obrigatória com MAIL_DRIVER=resend"))
		}
		if cfg.MailFrom == "" {
			errs = append(errs, errors.New("MAIL_FROM é obrigatória com MAIL_DRIVER=resend"))
		}
	case MailLog:
	default:
		errs = append(errs, fmt.Errorf("MAIL_DRIVER: use resend ou log, não %q", cfg.MailDriver))
	}

	minMatches, err := strconv.Atoi(get("RANKING_MIN_MATCHES", "5"))
	if err != nil || minMatches < 1 {
		errs = append(errs, errors.New("RANKING_MIN_MATCHES deve ser um inteiro >= 1"))
	}
	cfg.RankingMinMatches = minMatches

	if err := cfg.LogLevel.UnmarshalText([]byte(get("LOG_LEVEL", "info"))); err != nil {
		errs = append(errs, fmt.Errorf("LOG_LEVEL inválido: %v", err))
	}
	if cfg.LogFormat != "json" && cfg.LogFormat != "text" {
		errs = append(errs, fmt.Errorf("LOG_FORMAT: use json ou text, não %q", cfg.LogFormat))
	}
	return cfg, errors.Join(errs...)
}

func parseBaseURL(raw string) (base, origin string, err error) {
	if raw == "" {
		return "", "", errors.New("APP_BASE_URL é obrigatória")
	}
	u, parseErr := url.Parse(raw)
	if parseErr != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", "", fmt.Errorf("APP_BASE_URL deve ser uma URL http(s) absoluta, não %q", raw)
	}
	if strings.Trim(u.Path, "/") != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", "", fmt.Errorf("APP_BASE_URL não pode ter caminho, query ou fragmento: %q", raw)
	}
	origin = u.Scheme + "://" + u.Host
	return origin, origin, nil
}
```

- [ ] **Step 5: Rodar e ver passar**

Run: `cd backend && go test ./internal/config/ -v`
Expected: PASS.

- [ ] **Step 6: Ambiente de desenvolvimento**

`docker-compose.yml` (raiz do repositório):

```yaml
services:
  postgres:
    image: postgres:18
    environment:
      POSTGRES_USER: truco
      POSTGRES_PASSWORD: truco
      POSTGRES_DB: truco
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U truco -d truco"]
      interval: 2s
      timeout: 3s
      retries: 30

volumes:
  pgdata:
```

`.env.example` (raiz):

```dotenv
# Copie para .env e ajuste. O .env não é versionado.
DATABASE_URL=postgres://truco:truco@localhost:5432/truco?sslmode=disable
HTTP_ADDR=:8080
# Origem pública do app (sem caminho). No dev, a do Vite: http://localhost:5173
APP_BASE_URL=http://localhost:5173
COOKIE_SECURE=true
TRUST_PROXY=false
# Lista separada por vírgula; só popula a tabela de domínios se ela estiver vazia.
ALLOWED_EMAIL_DOMAINS=empresa.com.br
# resend (padrão) ou log (escreve os e-mails no log, para quem não tem chave)
MAIL_DRIVER=resend
RESEND_API_KEY=
MAIL_FROM=Truco <truco@empresa.com.br>
RANKING_MIN_MATCHES=5
MIGRATE_ON_START=true
LOG_LEVEL=info
LOG_FORMAT=text
STATIC_DIR=
```

No `Makefile`, troque a linha `.PHONY` e acrescente o alvo `dev-db`:

```make
.PHONY: test test-short gen-types dev-db

dev-db:
	docker compose up -d --wait postgres
```

Run: `make dev-db && docker compose ps`
Expected: serviço `postgres` com status `healthy`.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/testclock backend/internal/config docker-compose.yml .env.example Makefile
git commit -m "feat(config): adiciona configuração por ambiente e Postgres no Docker Compose"
```

---

### Tarefa 6: Banco — migrations, conexão e harness de testes com testcontainers

**Files:**
- Create: `backend/internal/db/migrations/0001_contas.sql`
- Create: `backend/internal/db/migrations/0002_partidas.sql`
- Create: `backend/internal/db/migrations/0003_auditoria.sql`
- Create: `backend/internal/db/db.go`
- Create: `backend/internal/testdb/testdb.go`
- Test: `backend/internal/db/db_test.go`
- Modify: `backend/go.mod`, `backend/go.sum` (via `go get`)

**Interfaces:**
- Produces:
  - `func db.Open(ctx context.Context, url string) (*pgxpool.Pool, error)`
  - `func db.Migrate(ctx context.Context, pool *pgxpool.Pool) error`
  - `func db.MigrateDown(ctx context.Context, pool *pgxpool.Pool) error`
  - `func db.WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error`
  - `func db.IsUniqueViolation(err error, constraint string) bool` (constraint vazio = qualquer)
  - `func db.IsInvalidInput(err error) bool` (SQLSTATE `22P02`, ex.: UUID malformado vindo da URL)
  - `func testdb.Run(m *testing.M)` — para `TestMain`.
  - `func testdb.New(t testing.TB) *pgxpool.Pool` — banco novo, migrado, removido no `Cleanup`.
  - `func testdb.NewURL(t testing.TB) string` — idem, devolve a URL (para testar o binário).
  - `func testdb.CreateUser(t testing.TB, pool *pgxpool.Pool, username string) string` — usuário verificado `<username>@empresa.com`, senha inválida (`x`), devolve o id.

- [ ] **Step 1: Adicionar dependências**

Run:

```bash
cd backend && go get github.com/jackc/pgx/v5@v5.11.0 github.com/pressly/goose/v3@v3.29.0 \
  github.com/testcontainers/testcontainers-go@v0.44.0 \
  github.com/testcontainers/testcontainers-go/modules/postgres@v0.44.0
```

Expected: `go.mod` passa a listar as quatro dependências.

- [ ] **Step 2: Escrever as migrations**

`backend/internal/db/migrations/0001_contas.sql`:

```sql
-- +goose Up
CREATE TABLE users (
    id                  uuid PRIMARY KEY DEFAULT uuidv7(),
    username            text NOT NULL UNIQUE CHECK (username ~ '^[a-z0-9_.]{3,20}$'),
    email               text NOT NULL UNIQUE CHECK (email = lower(email) AND position('@' IN email) > 1),
    display_name        text NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 30),
    password_hash       text NOT NULL,
    is_admin            boolean NOT NULL DEFAULT false,
    email_verified_at   timestamptz,
    disabled_at         timestamptz,
    must_reset_password boolean NOT NULL DEFAULT false,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    id                  uuid PRIMARY KEY DEFAULT uuidv7(),
    user_id             uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash          bytea NOT NULL UNIQUE,
    created_at          timestamptz NOT NULL DEFAULT now(),
    last_seen_at        timestamptz NOT NULL,
    expires_at          timestamptz NOT NULL,
    absolute_expires_at timestamptz NOT NULL,
    user_agent          text NOT NULL DEFAULT '',
    ip                  inet
);
CREATE INDEX sessions_user_id_idx ON sessions (user_id);
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);

CREATE TABLE user_tokens (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    purpose    text NOT NULL CHECK (purpose IN ('verify_email', 'password_reset')),
    token_hash bytea NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    used_at    timestamptz
);
CREATE INDEX user_tokens_user_purpose_idx ON user_tokens (user_id, purpose);

CREATE TABLE allowed_email_domains (
    domain     text PRIMARY KEY CHECK (domain = lower(domain) AND position('@' IN domain) = 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    created_by uuid REFERENCES users (id) ON DELETE SET NULL
);

-- +goose Down
DROP TABLE allowed_email_domains;
DROP TABLE user_tokens;
DROP TABLE sessions;
DROP TABLE users;
```

`backend/internal/db/migrations/0002_partidas.sql`:

```sql
-- +goose Up
CREATE TABLE matches (
    id             uuid PRIMARY KEY DEFAULT uuidv7(),
    seed           bigint NOT NULL,
    first_dealer   smallint NOT NULL CHECK (first_dealer BETWEEN 0 AND 3),
    engine_version integer NOT NULL,
    status         text NOT NULL DEFAULT 'in_progress' CHECK (status IN ('in_progress', 'finished', 'abandoned')),
    winner_team    smallint CHECK (winner_team IN (0, 1)),
    score_a        smallint,
    score_b        smallint,
    created_at     timestamptz NOT NULL DEFAULT now(),
    finished_at    timestamptz,
    abandon_reason text
);
CREATE INDEX matches_in_progress_idx ON matches (created_at) WHERE status = 'in_progress';
CREATE INDEX matches_closed_idx ON matches (finished_at DESC, id DESC) WHERE status <> 'in_progress';

CREATE TABLE match_players (
    match_id     uuid NOT NULL REFERENCES matches (id) ON DELETE CASCADE,
    seat         smallint NOT NULL CHECK (seat BETWEEN 0 AND 3),
    user_id      uuid NOT NULL REFERENCES users (id),
    display_name text NOT NULL,
    PRIMARY KEY (match_id, seat),
    UNIQUE (match_id, user_id)
);
CREATE INDEX match_players_user_idx ON match_players (user_id);

CREATE TABLE match_actions (
    match_id   uuid NOT NULL REFERENCES matches (id) ON DELETE CASCADE,
    ply        integer NOT NULL CHECK (ply >= 0),
    action     jsonb NOT NULL,
    source     text NOT NULL CHECK (source IN ('player', 'timeout', 'system')),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (match_id, ply)
);

CREATE TABLE match_player_stats (
    match_id           uuid NOT NULL REFERENCES matches (id) ON DELETE CASCADE,
    user_id            uuid NOT NULL REFERENCES users (id),
    team               smallint NOT NULL CHECK (team IN (0, 1)),
    won                boolean NOT NULL,
    points_for         integer NOT NULL,
    points_against     integer NOT NULL,
    hands_played       integer NOT NULL,
    hands_won          integer NOT NULL,
    raises_called      integer NOT NULL,
    raises_folded      integer NOT NULL,
    raises_won_by_fold integer NOT NULL,
    eleven_hands       integer NOT NULL,
    eleven_runs        integer NOT NULL,
    eleven_won         integer NOT NULL,
    iron_hands         integer NOT NULL,
    iron_won           integer NOT NULL,
    punished           integer NOT NULL,
    timeouts           integer NOT NULL,
    PRIMARY KEY (match_id, user_id)
);

CREATE TABLE player_stats (
    user_id            uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    matches_played     integer NOT NULL DEFAULT 0,
    wins               integer NOT NULL DEFAULT 0,
    losses             integer NOT NULL DEFAULT 0,
    points_for         integer NOT NULL DEFAULT 0,
    points_against     integer NOT NULL DEFAULT 0,
    hands_played       integer NOT NULL DEFAULT 0,
    hands_won          integer NOT NULL DEFAULT 0,
    raises_called      integer NOT NULL DEFAULT 0,
    raises_folded      integer NOT NULL DEFAULT 0,
    raises_won_by_fold integer NOT NULL DEFAULT 0,
    eleven_hands       integer NOT NULL DEFAULT 0,
    eleven_runs        integer NOT NULL DEFAULT 0,
    eleven_won         integer NOT NULL DEFAULT 0,
    iron_hands         integer NOT NULL DEFAULT 0,
    iron_won           integer NOT NULL DEFAULT 0,
    punished           integer NOT NULL DEFAULT 0,
    timeouts           integer NOT NULL DEFAULT 0,
    updated_at         timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE player_stats;
DROP TABLE match_player_stats;
DROP TABLE match_actions;
DROP TABLE match_players;
DROP TABLE matches;
```

`backend/internal/db/migrations/0003_auditoria.sql`:

```sql
-- +goose Up
CREATE TABLE audit_log (
    id          uuid PRIMARY KEY DEFAULT uuidv7(),
    actor_id    uuid NOT NULL REFERENCES users (id),
    action      text NOT NULL,
    target_type text NOT NULL,
    target_id   text NOT NULL,
    details     jsonb NOT NULL DEFAULT '{}',
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_log_created_idx ON audit_log (created_at DESC, id DESC);

-- +goose Down
DROP TABLE audit_log;
```

- [ ] **Step 3: Implementar o pacote `db`**

`backend/internal/db/db.go`:

```go
// Package db abre o pool do Postgres, aplica migrations e oferece transações.
package db

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("db: DATABASE_URL inválida: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: banco inacessível: %w", err)
	}
	return pool, nil
}

func withProvider(ctx context.Context, pool *pgxpool.Pool, fn func(*goose.Provider) error) error {
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()
	migrations, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return err
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations)
	if err != nil {
		return fmt.Errorf("db: migrations: %w", err)
	}
	return fn(provider)
}

// Migrate aplica todas as migrations pendentes.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	return withProvider(ctx, pool, func(p *goose.Provider) error {
		_, err := p.Up(ctx)
		return err
	})
}

// MigrateDown desfaz todas as migrations (usado nos testes de migração).
func MigrateDown(ctx context.Context, pool *pgxpool.Pool) error {
	return withProvider(ctx, pool, func(p *goose.Provider) error {
		_, err := p.DownTo(ctx, 0)
		return err
	})
}

// WithTx executa fn numa transação: commit se fn devolver nil, rollback caso contrário.
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	return pgx.BeginFunc(ctx, pool, fn)
}

// IsUniqueViolation informa se err é violação de unicidade (da constraint indicada, se não vazia).
func IsUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return false
	}
	return constraint == "" || pgErr.ConstraintName == constraint
}

// IsInvalidInput informa se err é entrada com sintaxe inválida (ex.: UUID malformado vindo da URL).
func IsInvalidInput(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "22P02"
}
```

- [ ] **Step 4: Implementar o harness `testdb`**

`backend/internal/testdb/testdb.go`:

```go
// Package testdb sobe um Postgres efêmero (testcontainers) por pacote de teste e
// entrega a cada teste um banco próprio clonado de um template já migrado.
package testdb

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/db"
)

const templateName = "truco_template"

type harness struct {
	container *postgres.PostgresContainer
	admin     *pgxpool.Pool
	baseURL   *url.URL
	counter   atomic.Int64
}

var (
	shared   *harness
	setupErr error
)

// Run deve ser chamado pelo TestMain dos pacotes com testes de integração.
func Run(m *testing.M) {
	flag.Parse()
	os.Exit(run(m))
}

func run(m *testing.M) int {
	if testing.Short() {
		return m.Run()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	h, err := start(ctx)
	cancel()
	if err != nil {
		setupErr = err
		return m.Run()
	}
	shared = h
	defer h.stop()
	return m.Run()
}

func start(ctx context.Context) (*harness, error) {
	container, err := postgres.Run(ctx, "postgres:18",
		postgres.WithDatabase("postgres"),
		postgres.WithUsername("truco"),
		postgres.WithPassword("truco"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		return nil, fmt.Errorf("subir contêiner: %w", err)
	}
	h := &harness{container: container}
	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		h.stop()
		return nil, err
	}
	if h.baseURL, err = url.Parse(connStr); err != nil {
		h.stop()
		return nil, err
	}
	if h.admin, err = db.Open(ctx, connStr); err != nil {
		h.stop()
		return nil, err
	}
	if _, err := h.admin.Exec(ctx, "CREATE DATABASE "+templateName); err != nil {
		h.stop()
		return nil, err
	}
	template, err := db.Open(ctx, h.urlFor(templateName))
	if err != nil {
		h.stop()
		return nil, err
	}
	err = db.Migrate(ctx, template)
	template.Close() // o template não pode ter conexões abertas ao ser clonado
	if err != nil {
		h.stop()
		return nil, fmt.Errorf("migrar template: %w", err)
	}
	return h, nil
}

func (h *harness) urlFor(name string) string {
	u := *h.baseURL
	u.Path = "/" + name
	return u.String()
}

func (h *harness) stop() {
	if h.admin != nil {
		h.admin.Close()
	}
	if h.container != nil {
		_ = testcontainers.TerminateContainer(h.container)
	}
}

// NewURL cria um banco migrado exclusivo do teste e devolve sua URL.
func NewURL(t testing.TB) string {
	t.Helper()
	if testing.Short() {
		t.Skip("teste de integração: pulado com -short")
	}
	if shared == nil {
		msg := fmt.Sprintf("teste de integração: Postgres indisponível (%v). O Docker está rodando?", setupErr)
		if os.Getenv("TRUCO_REQUIRE_DB") == "1" {
			t.Fatal(msg)
		}
		t.Skip(msg)
	}
	name := fmt.Sprintf("t_%d", shared.counter.Add(1))
	ctx := context.Background()
	if _, err := shared.admin.Exec(ctx, "CREATE DATABASE "+name+" TEMPLATE "+templateName); err != nil {
		t.Fatalf("criar banco de teste: %v", err)
	}
	t.Cleanup(func() {
		_, _ = shared.admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
	})
	return shared.urlFor(name)
}

// New cria um banco migrado exclusivo do teste e devolve um pool para ele.
func New(t testing.TB) *pgxpool.Pool {
	t.Helper()
	pool, err := db.Open(context.Background(), NewURL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// CreateUser insere um usuário verificado e devolve seu id.
func CreateUser(t testing.TB, pool *pgxpool.Pool, username string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(),
		`INSERT INTO users (username, email, display_name, password_hash, email_verified_at)
		 VALUES ($1, $1 || '@empresa.com', $1, 'x', now()) RETURNING id::text`, username).Scan(&id)
	if err != nil {
		t.Fatalf("criar usuário %s: %v", username, err)
	}
	return id
}
```

> Ordem do `t.Cleanup`: em `New`, o `pool.Close` é registrado depois do `DROP DATABASE`, então roda **antes** dele (Cleanup é LIFO). Não inverta.

- [ ] **Step 5: Escrever o teste de migrations**

`backend/internal/db/db_test.go`:

```go
package db_test

import (
	"context"
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/db"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/testdb"
)

func TestMain(m *testing.M) { testdb.Run(m) }

func TestMigrationsUpDownUp(t *testing.T) {
	ctx := context.Background()
	pool := testdb.New(t)
	exists := func(table string) bool {
		var ok bool
		if err := pool.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", table).Scan(&ok); err != nil {
			t.Fatal(err)
		}
		return ok
	}
	for _, table := range []string{"users", "sessions", "user_tokens", "allowed_email_domains", "matches",
		"match_players", "match_actions", "match_player_stats", "player_stats", "audit_log"} {
		if !exists(table) {
			t.Fatalf("tabela %s ausente após migrar", table)
		}
	}
	if err := db.MigrateDown(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if exists("users") || exists("audit_log") {
		t.Fatal("down não removeu as tabelas")
	}
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if !exists("users") {
		t.Fatal("up após down falhou")
	}
}

func TestUUIDv7DefaultAndUniqueViolation(t *testing.T) {
	ctx := context.Background()
	pool := testdb.New(t)
	id := testdb.CreateUser(t, pool, "ana")
	if len(id) != 36 || id[14] != '7' {
		t.Fatalf("id %q não é um UUID v7", id)
	}
	_, err := pool.Exec(ctx, `INSERT INTO users (username, email, display_name, password_hash) VALUES ('ana', 'outra@x.com', 'A', 'x')`)
	if !db.IsUniqueViolation(err, "users_username_key") {
		t.Fatalf("esperava violação de users_username_key, veio %v", err)
	}
	if db.IsUniqueViolation(err, "users_email_key") {
		t.Fatal("constraint errada reconhecida")
	}
}
```

- [ ] **Step 6: Rodar**

Run: `cd backend && go mod tidy && go test ./internal/db/ -v`
Expected: PASS nos dois testes (o primeiro `go test` baixa `postgres:18`; pode levar um minuto). Sem Docker: `SKIP` com a mensagem do harness.

Run: `cd backend && TRUCO_REQUIRE_DB=1 go test ./internal/db/ -short`
Expected: os testes são pulados por `-short` (o `TRUCO_REQUIRE_DB` só vale quando o contêiner falha).

- [ ] **Step 7: Commit**

```bash
git add backend/go.mod backend/go.sum backend/internal/db backend/internal/testdb
git commit -m "feat(db): adiciona schema, migrations goose e harness de testes com Postgres real"
```

---

### Tarefa 7: sqlc e queries de contas

**Files:**
- Create: `backend/sqlc.yaml`
- Create: `backend/internal/db/queries/users.sql`, `sessions.sql`, `tokens.sql`, `domains.sql`
- Create (gerado): `backend/internal/db/dbgen/*.go`
- Modify: `Makefile`

**Interfaces:**
- Produces (gerado por `sqlc`, pacote `dbgen`, todos métodos de `*dbgen.Queries`; `dbgen.New(pool)` e `q.WithTx(tx)`):
  - Modelo `dbgen.User{ID, Username, Email, DisplayName, PasswordHash string; IsAdmin bool; EmailVerifiedAt, DisabledAt *time.Time; MustResetPassword bool; CreatedAt, UpdatedAt time.Time}`.
  - `CreateUser(ctx, CreateUserParams{Username, Email, DisplayName, PasswordHash string; Now time.Time}) (User, error)`
  - `GetUserByID(ctx, id string) (User, error)`, `GetUserByEmail(ctx, email string) (User, error)`, `GetUserByUsername(ctx, username string) (User, error)`
  - `MarkEmailVerified(ctx, MarkEmailVerifiedParams{Now time.Time; ID string}) error`
  - `SetPasswordHash(ctx, SetPasswordHashParams{PasswordHash string; Now time.Time; ID string}) error`
  - `CompletePasswordReset(ctx, CompletePasswordResetParams{PasswordHash string; Now time.Time; ID string}) error`
  - `UpdateDisplayName(ctx, UpdateDisplayNameParams{DisplayName string; Now time.Time; ID string}) (User, error)`
  - `CreateSession(ctx, CreateSessionParams{UserID string; TokenHash []byte; Now, ExpiresAt, AbsoluteExpiresAt time.Time; UserAgent, Ip string}) (string, error)`
  - `GetSessionPrincipal(ctx, GetSessionPrincipalParams{TokenHash []byte; Now time.Time}) (GetSessionPrincipalRow, error)` com `Row{SessionID string; LastSeenAt time.Time; UserID, Username, Email, DisplayName string; IsAdmin bool}`
  - `TouchSession(ctx, TouchSessionParams{Now, ExpiresAt time.Time; ID string}) error`
  - `DeleteSessionByTokenHash(ctx, tokenHash []byte) error`, `DeleteUserSessions(ctx, userID string) error`, `DeleteExpiredSessions(ctx, now time.Time) (int64, error)`
  - `DeleteUnusedTokens(ctx, DeleteUnusedTokensParams{UserID, Purpose string}) error`
  - `CreateUserToken(ctx, CreateUserTokenParams{UserID, Purpose string; TokenHash []byte; Now, ExpiresAt time.Time}) error`
  - `GetValidUserToken(ctx, GetValidUserTokenParams{TokenHash []byte; Purpose string; Now time.Time}) (string, error)`
  - `ConsumeUserToken(ctx, ConsumeUserTokenParams{Now time.Time; TokenHash []byte; Purpose string}) (string, error)`
  - `DeleteStaleTokens(ctx, DeleteStaleTokensParams{Now, UsedBefore time.Time}) (int64, error)`
  - `DomainAllowed(ctx, domain string) (bool, error)`, `CountDomains(ctx) (int64, error)`, `InsertDomain(ctx, InsertDomainParams{Domain string; Now time.Time; CreatedBy *string}) error`, `ListDomains(ctx) ([]ListDomainsRow, error)` com `Row{Domain string; CreatedAt time.Time; CreatedBy *string}`, `DeleteDomain(ctx, domain string) (int64, error)`, `LockDomains(ctx) error`

> Os nomes de campos acima são os que o sqlc 1.31 gera com esta configuração. Se algum divergir na geração (por exemplo, a ordem dos campos), **ajuste as chamadas nas tarefas seguintes, não o SQL**.

- [ ] **Step 1: Configurar o sqlc**

`backend/sqlc.yaml`:

```yaml
version: "2"
sql:
  - engine: postgresql
    schema: internal/db/migrations
    queries: internal/db/queries
    gen:
      go:
        package: dbgen
        out: internal/db/dbgen
        sql_package: pgx/v5
        emit_pointers_for_null_types: true
        overrides:
          - db_type: uuid
            go_type: string
          - db_type: uuid
            nullable: true
            go_type:
              type: string
              pointer: true
          - db_type: timestamptz
            go_type: time.Time
          - db_type: timestamptz
            nullable: true
            go_type:
              import: time
              type: Time
              pointer: true
```

No `Makefile`, acrescente `gen-sql` ao `.PHONY` e o alvo:

```make
SQLC_VERSION := v1.31.1

gen-sql:
	cd backend && go run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION) generate
```

- [ ] **Step 2: Escrever as queries**

`backend/internal/db/queries/users.sql`:

```sql
-- name: CreateUser :one
INSERT INTO users (username, email, display_name, password_hash, created_at, updated_at)
VALUES (@username, @email, @display_name, @password_hash, @now, @now)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = @id;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = @email;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = @username;

-- name: MarkEmailVerified :exec
UPDATE users SET email_verified_at = COALESCE(email_verified_at, @now::timestamptz), updated_at = @now
WHERE id = @id;

-- name: SetPasswordHash :exec
UPDATE users SET password_hash = @password_hash, updated_at = @now WHERE id = @id;

-- name: CompletePasswordReset :exec
UPDATE users
SET password_hash = @password_hash,
    must_reset_password = false,
    email_verified_at = COALESCE(email_verified_at, @now::timestamptz),
    updated_at = @now
WHERE id = @id;

-- name: UpdateDisplayName :one
UPDATE users SET display_name = @display_name, updated_at = @now WHERE id = @id
RETURNING *;
```

`backend/internal/db/queries/sessions.sql`:

```sql
-- name: CreateSession :one
INSERT INTO sessions (user_id, token_hash, created_at, last_seen_at, expires_at, absolute_expires_at, user_agent, ip)
VALUES (@user_id, @token_hash, @now, @now, @expires_at, @absolute_expires_at, @user_agent, NULLIF(@ip::text, '')::inet)
RETURNING id;

-- name: GetSessionPrincipal :one
SELECT s.id AS session_id, s.last_seen_at, u.id AS user_id, u.username, u.email, u.display_name, u.is_admin
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token_hash = @token_hash
  AND s.expires_at > @now
  AND s.absolute_expires_at > @now
  AND u.disabled_at IS NULL;

-- name: TouchSession :exec
UPDATE sessions
SET last_seen_at = @now, expires_at = LEAST(@expires_at::timestamptz, absolute_expires_at)
WHERE id = @id;

-- name: DeleteSessionByTokenHash :exec
DELETE FROM sessions WHERE token_hash = @token_hash;

-- name: DeleteUserSessions :exec
DELETE FROM sessions WHERE user_id = @user_id;

-- name: DeleteExpiredSessions :execrows
DELETE FROM sessions WHERE expires_at <= @now OR absolute_expires_at <= @now;
```

`backend/internal/db/queries/tokens.sql`:

```sql
-- name: DeleteUnusedTokens :exec
DELETE FROM user_tokens WHERE user_id = @user_id AND purpose = @purpose AND used_at IS NULL;

-- name: CreateUserToken :exec
INSERT INTO user_tokens (user_id, purpose, token_hash, created_at, expires_at)
VALUES (@user_id, @purpose, @token_hash, @now, @expires_at);

-- name: GetValidUserToken :one
SELECT user_id FROM user_tokens
WHERE token_hash = @token_hash AND purpose = @purpose AND used_at IS NULL AND expires_at > @now;

-- name: ConsumeUserToken :one
UPDATE user_tokens SET used_at = @now
WHERE token_hash = @token_hash AND purpose = @purpose AND used_at IS NULL AND expires_at > @now
RETURNING user_id;

-- name: DeleteStaleTokens :execrows
DELETE FROM user_tokens WHERE expires_at <= @now OR used_at <= @used_before::timestamptz;
```

`backend/internal/db/queries/domains.sql`:

```sql
-- name: DomainAllowed :one
SELECT EXISTS (SELECT 1 FROM allowed_email_domains WHERE domain = @domain);

-- name: CountDomains :one
SELECT count(*) FROM allowed_email_domains;

-- name: InsertDomain :exec
INSERT INTO allowed_email_domains (domain, created_at, created_by)
VALUES (@domain, @now, sqlc.narg(created_by))
ON CONFLICT (domain) DO NOTHING;

-- name: ListDomains :many
SELECT domain, created_at, created_by FROM allowed_email_domains ORDER BY domain;

-- name: DeleteDomain :execrows
DELETE FROM allowed_email_domains WHERE domain = @domain;

-- name: LockDomains :exec
SELECT domain FROM allowed_email_domains FOR UPDATE;
```

- [ ] **Step 3: Gerar e compilar**

Run: `make gen-sql && cd backend && go build ./... && go vet ./internal/db/...`
Expected: `backend/internal/db/dbgen/` contém `db.go`, `models.go`, `users.sql.go`, `sessions.sql.go`, `tokens.sql.go`, `domains.sql.go`; build sem erros. Confira que os tipos batem com o bloco **Interfaces** (`grep -n "func (q \*Queries)" backend/internal/db/dbgen/*.go`).

- [ ] **Step 4: Teste de fumaça das queries com Postgres real**

`backend/internal/db/dbgen_smoke_test.go` (pacote `db_test`, mesmo `TestMain` do `db_test.go`):

```go
package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/db/dbgen"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/testdb"
)

func TestGeneratedQueriesRoundTripUUIDAndTime(t *testing.T) {
	ctx := context.Background()
	q := dbgen.New(testdb.New(t))
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	user, err := q.CreateUser(ctx, dbgen.CreateUserParams{Username: "ana", Email: "ana@empresa.com", DisplayName: "Ana", PasswordHash: "h", Now: now})
	if err != nil {
		t.Fatal(err)
	}
	got, err := q.GetUserByID(ctx, user.ID)
	if err != nil || got.Username != "ana" || !got.CreatedAt.Equal(now) || got.EmailVerifiedAt != nil {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	sessionID, err := q.CreateSession(ctx, dbgen.CreateSessionParams{UserID: user.ID, TokenHash: []byte{1}, Now: now,
		ExpiresAt: now.Add(time.Hour), AbsoluteExpiresAt: now.Add(2 * time.Hour), UserAgent: "teste", Ip: "203.0.113.9"})
	if err != nil || sessionID == "" {
		t.Fatalf("sessão: %q %v", sessionID, err)
	}
	row, err := q.GetSessionPrincipal(ctx, dbgen.GetSessionPrincipalParams{TokenHash: []byte{1}, Now: now})
	if err != nil || row.UserID != user.ID || row.SessionID != sessionID {
		t.Fatalf("row=%+v err=%v", row, err)
	}
}
```

Run: `cd backend && go test ./internal/db/ -v`
Expected: PASS (confirma que `uuid ↔ string` e `timestamptz ↔ time.Time` funcionam com o pgx).

- [ ] **Step 5: Commit**

```bash
git add backend/sqlc.yaml backend/internal/db/queries backend/internal/db/dbgen backend/internal/db/dbgen_smoke_test.go Makefile
git commit -m "feat(db): adiciona queries de contas e sessões geradas com sqlc"
```

---
### Tarefa 8: `auth` — hash de senha argon2id

**Files:**
- Create: `backend/internal/auth/password.go`
- Test: `backend/internal/auth/password_test.go`

**Interfaces:**
- Produces:
  - `type auth.Params struct{ Memory uint32 /* KiB */; Time uint32; Threads uint8; SaltLen, KeyLen uint32 }`
  - `var auth.DefaultParams = Params{Memory: 64 * 1024, Time: 3, Threads: 2, SaltLen: 16, KeyLen: 32}`
  - `var auth.TestingParams = Params{Memory: 1024, Time: 1, Threads: 1, SaltLen: 16, KeyLen: 32}` (somente testes)
  - `func auth.NewHasher(p Params, maxConcurrent int) *Hasher`
  - `func (*Hasher) Hash(password string) (string, error)`
  - `func (*Hasher) Verify(password, encoded string) (ok, needsRehash bool, err error)`
  - `func (*Hasher) VerifyDummy(password string)` — gasta o mesmo tempo de uma verificação real.

- [ ] **Step 1: Adicionar a dependência**

Run: `cd backend && go get golang.org/x/crypto@v0.58.0`

- [ ] **Step 2: Escrever os testes que falham**

`backend/internal/auth/password_test.go`:

```go
package auth

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	h := NewHasher(TestingParams, 2)
	encoded, err := h.Hash("senha-muito-boa")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$v=19$m=1024,t=1,p=1$") {
		t.Fatalf("formato PHC inesperado: %s", encoded)
	}
	ok, rehash, err := h.Verify("senha-muito-boa", encoded)
	if err != nil || !ok || rehash {
		t.Fatalf("ok=%v rehash=%v err=%v", ok, rehash, err)
	}
	ok, _, err = h.Verify("senha-errada!!", encoded)
	if err != nil || ok {
		t.Fatalf("senha errada aceita: ok=%v err=%v", ok, err)
	}
}

func TestHashUsesRandomSalt(t *testing.T) {
	h := NewHasher(TestingParams, 2)
	a, _ := h.Hash("mesma-senha-123")
	b, _ := h.Hash("mesma-senha-123")
	if a == b {
		t.Fatal("dois hashes da mesma senha não podem ser iguais")
	}
}

func TestVerifyFlagsRehashWhenParamsChange(t *testing.T) {
	old := NewHasher(TestingParams, 2)
	encoded, _ := old.Hash("senha-antiga-123")
	stronger := TestingParams
	stronger.Time = 2
	ok, rehash, err := NewHasher(stronger, 2).Verify("senha-antiga-123", encoded)
	if err != nil || !ok || !rehash {
		t.Fatalf("ok=%v rehash=%v err=%v", ok, rehash, err)
	}
}

func TestVerifyRejectsMalformedHash(t *testing.T) {
	h := NewHasher(TestingParams, 2)
	for _, bad := range []string{"", "x", "$argon2i$v=19$m=1,t=1,p=1$c2FsdA$aGFzaA", "$argon2id$v=18$m=1024,t=1,p=1$c2FsdA$aGFzaA",
		"$argon2id$v=19$m=abc$c2FsdA$aGFzaA", "$argon2id$v=19$m=1024,t=1,p=1$***$aGFzaA"} {
		if _, _, err := h.Verify("qualquer-coisa", bad); err == nil {
			t.Errorf("hash malformado aceito: %q", bad)
		}
	}
}

func TestHasherLimitsConcurrency(t *testing.T) {
	h := NewHasher(TestingParams, 2)
	var current, peak atomic.Int32
	h.onAcquire = func() {
		n := current.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
	}
	h.onRelease = func() { current.Add(-1) }
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = h.Hash("concorrencia-123")
		}()
	}
	wg.Wait()
	if peak.Load() > 2 {
		t.Fatalf("pico de %d hashes simultâneos, limite 2", peak.Load())
	}
}
```

- [ ] **Step 3: Rodar e ver falhar**

Run: `cd backend && go test ./internal/auth/ -run 'Hash|Verify' -v`
Expected: FAIL de compilação.

- [ ] **Step 4: Implementar**

`backend/internal/auth/password.go`:

```go
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Params são os parâmetros do argon2id. Memory é em KiB.
type Params struct {
	Memory  uint32
	Time    uint32
	Threads uint8
	SaltLen uint32
	KeyLen  uint32
}

var DefaultParams = Params{Memory: 64 * 1024, Time: 3, Threads: 2, SaltLen: 16, KeyLen: 32}

// TestingParams são baratos de propósito. Use somente em testes.
var TestingParams = Params{Memory: 1024, Time: 1, Threads: 1, SaltLen: 16, KeyLen: 32}

var errMalformedHash = errors.New("auth: hash de senha malformado")

// Hasher calcula e verifica hashes argon2id limitando quantos rodam ao mesmo
// tempo, porque cada um aloca Params.Memory de RAM.
type Hasher struct {
	params    Params
	sem       chan struct{}
	dummy     string
	onAcquire func() // ganchos de teste
	onRelease func()
}

func NewHasher(p Params, maxConcurrent int) *Hasher {
	h := &Hasher{params: p, sem: make(chan struct{}, maxConcurrent)}
	h.dummy, _ = h.Hash("senha-ficticia-para-igualar-tempo")
	return h
}

func (h *Hasher) acquire() {
	h.sem <- struct{}{}
	if h.onAcquire != nil {
		h.onAcquire()
	}
}

func (h *Hasher) release() {
	if h.onRelease != nil {
		h.onRelease()
	}
	<-h.sem
}

func (h *Hasher) Hash(password string) (string, error) {
	salt := make([]byte, h.params.SaltLen)
	_, _ = rand.Read(salt) // crypto/rand.Read nunca falha (Go 1.24+)
	h.acquire()
	key := argon2.IDKey([]byte(password), salt, h.params.Time, h.params.Memory, h.params.Threads, h.params.KeyLen)
	h.release()
	enc := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version,
		h.params.Memory, h.params.Time, h.params.Threads, enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

// Verify compara em tempo constante. needsRehash indica parâmetros diferentes dos atuais.
func (h *Hasher) Verify(password, encoded string) (ok, needsRehash bool, err error) {
	p, salt, key, err := decodeHash(encoded)
	if err != nil {
		return false, false, err
	}
	h.acquire()
	candidate := argon2.IDKey([]byte(password), salt, p.Time, p.Memory, p.Threads, uint32(len(key)))
	h.release()
	ok = subtle.ConstantTimeCompare(candidate, key) == 1
	needsRehash = p.Memory != h.params.Memory || p.Time != h.params.Time || p.Threads != h.params.Threads ||
		uint32(len(salt)) != h.params.SaltLen || uint32(len(key)) != h.params.KeyLen
	return ok, needsRehash, nil
}

// VerifyDummy gasta o tempo de uma verificação para usuários inexistentes.
func (h *Hasher) VerifyDummy(password string) { _, _, _ = h.Verify(password, h.dummy) }

func decodeHash(encoded string) (Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return Params{}, nil, nil, errMalformedHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return Params{}, nil, nil, errMalformedHash
	}
	var p Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.Memory, &p.Time, &p.Threads); err != nil || p.Time == 0 || p.Threads == 0 {
		return Params{}, nil, nil, errMalformedHash
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return Params{}, nil, nil, errMalformedHash
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) == 0 {
		return Params{}, nil, nil, errMalformedHash
	}
	p.SaltLen, p.KeyLen = uint32(len(salt)), uint32(len(key))
	return p, salt, key, nil
}
```

- [ ] **Step 5: Rodar e ver passar**

Run: `cd backend && go test ./internal/auth/ -run 'Hash|Verify' -race -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/go.mod backend/go.sum backend/internal/auth/password.go backend/internal/auth/password_test.go
git commit -m "feat(auth): adiciona hash de senha argon2id com limite de concorrência"
```

---

### Tarefa 9: `auth` — tokens, normalização, validação e erros

**Files:**
- Create: `backend/internal/auth/token.go`, `backend/internal/auth/validate.go`, `backend/internal/auth/errors.go`
- Test: `backend/internal/auth/token_test.go`, `backend/internal/auth/validate_test.go`

**Interfaces:**
- Produces:
  - `func auth.NewToken() (token string, hash []byte)`; `func auth.HashToken(token string) []byte`
  - `func auth.NormalizeUsername(s string) string`; `func auth.NormalizeEmail(s string) string`
  - `type auth.FieldErrors map[string]string` (implementa `error`)
  - `func auth.ValidateUsername(u string) string` ("" = válido); `func auth.ValidateEmail(e string) (domain, msg string)`; `func auth.ValidateDisplayName(n string) (normalized, msg string)`; `func auth.ValidatePassword(pw, username, email string) string`
  - Erros: `auth.ErrInvalidCredentials`, `ErrEmailNotVerified`, `ErrAccountDisabled`, `ErrPasswordResetRequired`, `ErrUsernameTaken`, `ErrEmailDomainNotAllowed`, `ErrInvalidToken`, `ErrUnauthenticated`, `ErrUserNotFound`

- [ ] **Step 1: Escrever os testes que falham**

`backend/internal/auth/token_test.go`:

```go
package auth

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestNewToken(t *testing.T) {
	token, hash := NewToken()
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != 32 {
		t.Fatalf("token %q: %d bytes, err %v", token, len(raw), err)
	}
	if !bytes.Equal(hash, HashToken(token)) || len(hash) != 32 {
		t.Fatal("hash deve ser sha256 do token")
	}
	other, _ := NewToken()
	if other == token {
		t.Fatal("tokens repetidos")
	}
}
```

`backend/internal/auth/validate_test.go`:

```go
package auth

import (
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	if got := NormalizeUsername("  Ana.Silva "); got != "ana.silva" {
		t.Fatalf("username = %q", got)
	}
	if got := NormalizeEmail(" Ana@Empresa.COM\t"); got != "ana@empresa.com" {
		t.Fatalf("email = %q", got)
	}
}

func TestValidateUsername(t *testing.T) {
	for _, ok := range []string{"ana", "ana.silva", "a_1", "abcdefghijklmnopqrst"} {
		if msg := ValidateUsername(ok); msg != "" {
			t.Errorf("%q rejeitado: %s", ok, msg)
		}
	}
	for _, bad := range []string{"", "ab", "abcdefghijklmnopqrstu", "Ana", "ana silva", "ana-silva", "ána"} {
		if ValidateUsername(bad) == "" {
			t.Errorf("%q aceito", bad)
		}
	}
}

func TestValidateEmail(t *testing.T) {
	domain, msg := ValidateEmail("ana@empresa.com.br")
	if msg != "" || domain != "empresa.com.br" {
		t.Fatalf("domain=%q msg=%q", domain, msg)
	}
	for _, bad := range []string{"", "ana", "ana@", "@empresa.com", "Ana <ana@empresa.com>", "ana@empresa.com, bia@empresa.com"} {
		if _, msg := ValidateEmail(bad); msg == "" {
			t.Errorf("%q aceito", bad)
		}
	}
}

func TestValidateDisplayName(t *testing.T) {
	name, msg := ValidateDisplayName("  Ana 🃏 ")
	if msg != "" || name != "Ana 🃏" {
		t.Fatalf("name=%q msg=%q", name, msg)
	}
	if _, msg := ValidateDisplayName(strings.Repeat("é", 30)); msg != "" {
		t.Fatal("30 runes devem ser aceitas, mesmo com mais de 30 bytes")
	}
	for _, bad := range []string{"", "   ", strings.Repeat("a", 31), "ana\x00", "ana\nbia"} {
		if _, msg := ValidateDisplayName(bad); msg == "" {
			t.Errorf("%q aceito", bad)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	if msg := ValidatePassword("cavalo-bateria-grampo", "ana", "ana@empresa.com"); msg != "" {
		t.Fatalf("senha boa rejeitada: %s", msg)
	}
	cases := map[string]string{
		"curta":                         "curta1234",
		"longa":                         strings.Repeat("x", 129),
		"contém username":               "minhaANA-senha-1",
		"contém parte local do e-mail":  "anasilva-2026!",
	}
	for name, pw := range cases {
		email := "ana@empresa.com"
		if name == "contém parte local do e-mail" {
			email = "anasilva@empresa.com"
		}
		if ValidatePassword(pw, "ana", email) == "" {
			t.Errorf("%s: %q aceita", name, pw)
		}
	}
}

func TestFieldErrorsMessage(t *testing.T) {
	err := FieldErrors{"username": "inválido", "email": "inválido"}
	if !strings.Contains(err.Error(), "email") || !strings.Contains(err.Error(), "username") {
		t.Fatalf("Error() = %q", err.Error())
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/auth/ -run 'Token|Normalize|Validate|FieldErrors'`
Expected: FAIL de compilação.

- [ ] **Step 3: Implementar**

`backend/internal/auth/token.go`:

```go
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// NewToken gera 32 bytes aleatórios em base64url e o sha256 que vai para o banco.
func NewToken() (token string, hash []byte) {
	raw := make([]byte, 32)
	_, _ = rand.Read(raw) // crypto/rand.Read nunca falha (Go 1.24+)
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, HashToken(token)
}

// HashToken usa sha256: o token já tem 256 bits de entropia, então um hash lento não agrega.
func HashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
```

`backend/internal/auth/validate.go`:

```go
package auth

import (
	netmail "net/mail"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9_.]{3,20}$`)

func NormalizeUsername(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
func NormalizeEmail(s string) string    { return strings.ToLower(strings.TrimSpace(s)) }

// FieldErrors mapeia campo → motivo e é devolvido como erro de validação.
type FieldErrors map[string]string

func (f FieldErrors) Error() string {
	keys := make([]string, 0, len(f))
	for k := range f {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + ": " + f[k]
	}
	return "auth: dados inválidos (" + strings.Join(parts, "; ") + ")"
}

func ValidateUsername(u string) string {
	if !usernamePattern.MatchString(u) {
		return "use de 3 a 20 caracteres: letras minúsculas, números, ponto ou sublinhado"
	}
	return ""
}

// ValidateEmail espera o e-mail já normalizado e devolve seu domínio.
func ValidateEmail(e string) (domain, msg string) {
	addr, err := netmail.ParseAddress(e)
	if err != nil || addr.Name != "" || addr.Address != e {
		return "", "e-mail inválido"
	}
	at := strings.LastIndex(e, "@")
	if at < 1 || at == len(e)-1 {
		return "", "e-mail inválido"
	}
	return e[at+1:], ""
}

func ValidateDisplayName(n string) (string, string) {
	n = strings.TrimSpace(n)
	count := utf8.RuneCountInString(n)
	if count < 1 || count > 30 {
		return "", "o nome de exibição deve ter de 1 a 30 caracteres"
	}
	if strings.ContainsFunc(n, unicode.IsControl) {
		return "", "o nome de exibição não pode ter caracteres de controle"
	}
	return n, ""
}

func ValidatePassword(pw, username, email string) string {
	length := utf8.RuneCountInString(pw)
	if length < 10 || length > 128 {
		return "a senha deve ter de 10 a 128 caracteres"
	}
	lower := strings.ToLower(pw)
	if len(username) >= 3 && strings.Contains(lower, strings.ToLower(username)) {
		return "a senha não pode conter o nome de usuário"
	}
	if at := strings.Index(email, "@"); at >= 3 && strings.Contains(lower, strings.ToLower(email[:at])) {
		return "a senha não pode conter o seu e-mail"
	}
	return ""
}
```

`backend/internal/auth/errors.go`:

```go
package auth

import "errors"

var (
	ErrInvalidCredentials    = errors.New("auth: usuário ou senha incorretos")
	ErrEmailNotVerified      = errors.New("auth: e-mail não confirmado")
	ErrAccountDisabled       = errors.New("auth: conta desativada")
	ErrPasswordResetRequired = errors.New("auth: redefinição de senha obrigatória")
	ErrUsernameTaken         = errors.New("auth: nome de usuário em uso")
	ErrEmailDomainNotAllowed = errors.New("auth: domínio de e-mail não permitido")
	ErrInvalidToken          = errors.New("auth: link inválido ou expirado")
	ErrUnauthenticated       = errors.New("auth: não autenticado")
	ErrUserNotFound          = errors.New("auth: usuário não encontrado")
)
```

- [ ] **Step 4: Rodar e ver passar**

Run: `cd backend && go test ./internal/auth/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/auth/token.go backend/internal/auth/validate.go backend/internal/auth/errors.go backend/internal/auth/token_test.go backend/internal/auth/validate_test.go
git commit -m "feat(auth): adiciona tokens, normalização e validação de cadastro"
```

---

### Tarefa 10: `ratelimit` — token bucket por chave e bloqueio por falhas

**Files:**
- Create: `backend/internal/ratelimit/limiter.go`, `backend/internal/ratelimit/lockout.go`
- Test: `backend/internal/ratelimit/ratelimit_test.go`

**Interfaces:**
- Consumes: `testclock.Clock` (nos testes).
- Produces:
  - `func ratelimit.NewLimiter(events int, per time.Duration, now func() time.Time) *Limiter` (burst = events)
  - `func (*Limiter) Allow(key string) (ok bool, retryAfter time.Duration)`; `func (*Limiter) Sweep()` (remove chaves ociosas há mais de 30 min); `func (*Limiter) Len() int`
  - `func ratelimit.NewLockout(maxFailures int, window, lock time.Duration, now func() time.Time) *Lockout`
  - `func (*Lockout) Locked(key string) (bool, time.Duration)`; `Fail(key string)`; `Reset(key string)`; `Sweep()`; `Len() int`

- [ ] **Step 1: Adicionar a dependência**

Run: `cd backend && go get golang.org/x/time@v0.16.0`

- [ ] **Step 2: Escrever os testes que falham**

`backend/internal/ratelimit/ratelimit_test.go`:

```go
package ratelimit

import (
	"testing"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/testclock"
)

var t0 = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

func TestLimiterBurstThenRefill(t *testing.T) {
	clock := testclock.New(t0)
	l := NewLimiter(10, time.Minute, clock.Now)
	for i := range 10 {
		if ok, _ := l.Allow("1.2.3.4"); !ok {
			t.Fatalf("requisição %d bloqueada dentro do burst", i)
		}
	}
	ok, retry := l.Allow("1.2.3.4")
	if ok || retry <= 0 || retry > 6*time.Second {
		t.Fatalf("11ª: ok=%v retry=%v (quer ~6s)", ok, retry)
	}
	if ok, _ := l.Allow("5.6.7.8"); !ok {
		t.Fatal("chaves devem ser independentes")
	}
	clock.Advance(6 * time.Second)
	if ok, _ := l.Allow("1.2.3.4"); !ok {
		t.Fatal("ficha deveria ter sido reposta")
	}
}

func TestLimiterRejectionDoesNotConsume(t *testing.T) {
	clock := testclock.New(t0)
	l := NewLimiter(1, time.Hour, clock.Now)
	l.Allow("k")
	for range 5 {
		l.Allow("k") // rejeitadas não podem empurrar a próxima ficha para frente
	}
	clock.Advance(time.Hour)
	if ok, _ := l.Allow("k"); !ok {
		t.Fatal("após 1h deve liberar")
	}
}

func TestLimiterSweepRemovesIdleKeys(t *testing.T) {
	clock := testclock.New(t0)
	l := NewLimiter(5, time.Hour, clock.Now)
	l.Allow("a")
	clock.Advance(31 * time.Minute)
	l.Allow("b")
	l.Sweep()
	if l.Len() != 1 {
		t.Fatalf("Len = %d, quer 1", l.Len())
	}
}

func TestLockoutBlocksAfterMaxFailuresInWindow(t *testing.T) {
	clock := testclock.New(t0)
	l := NewLockout(5, 15*time.Minute, 15*time.Minute, clock.Now)
	for range 4 {
		l.Fail("ana")
	}
	if locked, _ := l.Locked("ana"); locked {
		t.Fatal("4 falhas não bloqueiam")
	}
	l.Fail("ana")
	locked, retry := l.Locked("ana")
	if !locked || retry != 15*time.Minute {
		t.Fatalf("locked=%v retry=%v", locked, retry)
	}
	clock.Advance(15 * time.Minute)
	if locked, _ := l.Locked("ana"); locked {
		t.Fatal("bloqueio deveria ter expirado")
	}
}

func TestLockoutForgetsOldFailures(t *testing.T) {
	clock := testclock.New(t0)
	l := NewLockout(5, 15*time.Minute, 15*time.Minute, clock.Now)
	for range 4 {
		l.Fail("ana")
	}
	clock.Advance(16 * time.Minute)
	l.Fail("ana")
	if locked, _ := l.Locked("ana"); locked {
		t.Fatal("falhas fora da janela não contam")
	}
}

func TestLockoutResetAndSweep(t *testing.T) {
	clock := testclock.New(t0)
	l := NewLockout(2, time.Minute, time.Minute, clock.Now)
	l.Fail("ana")
	l.Reset("ana")
	l.Fail("ana")
	if locked, _ := l.Locked("ana"); locked {
		t.Fatal("Reset deveria zerar as falhas")
	}
	l.Fail("bia")
	clock.Advance(2 * time.Minute)
	l.Sweep()
	if l.Len() != 0 {
		t.Fatalf("Len = %d após Sweep", l.Len())
	}
}
```

- [ ] **Step 3: Rodar e ver falhar**

Run: `cd backend && go test ./internal/ratelimit/`
Expected: FAIL de compilação.

- [ ] **Step 4: Implementar**

`backend/internal/ratelimit/limiter.go`:

```go
// Package ratelimit implementa limites em memória por chave (IP, e-mail, login).
// Basta porque o servidor roda em instância única.
package ratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const idleTTL = 30 * time.Minute

type limiterEntry struct {
	lim      *rate.Limiter
	lastSeen time.Time
}

type Limiter struct {
	mu      sync.Mutex
	limit   rate.Limit
	burst   int
	now     func() time.Time
	entries map[string]*limiterEntry
}

// NewLimiter permite `events` eventos por `per`, com rajada de `events`.
func NewLimiter(events int, per time.Duration, now func() time.Time) *Limiter {
	return &Limiter{limit: rate.Every(per / time.Duration(events)), burst: events, now: now, entries: map[string]*limiterEntry{}}
}

func (l *Limiter) Allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	e, ok := l.entries[key]
	if !ok {
		e = &limiterEntry{lim: rate.NewLimiter(l.limit, l.burst)}
		l.entries[key] = e
	}
	e.lastSeen = now
	reservation := e.lim.ReserveN(now, 1)
	if delay := reservation.DelayFrom(now); delay > 0 {
		reservation.CancelAt(now)
		return false, delay
	}
	return true, 0
}

func (l *Limiter) Sweep() {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := l.now().Add(-idleTTL)
	for key, e := range l.entries {
		if e.lastSeen.Before(cutoff) {
			delete(l.entries, key)
		}
	}
}

func (l *Limiter) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.entries)
}
```

`backend/internal/ratelimit/lockout.go`:

```go
package ratelimit

import (
	"sync"
	"time"
)

type failures struct {
	times       []time.Time
	lockedUntil time.Time
}

// Lockout bloqueia uma chave por `lock` depois de `maxFailures` falhas dentro de `window`.
type Lockout struct {
	mu          sync.Mutex
	maxFailures int
	window      time.Duration
	lock        time.Duration
	now         func() time.Time
	entries     map[string]*failures
}

func NewLockout(maxFailures int, window, lock time.Duration, now func() time.Time) *Lockout {
	return &Lockout{maxFailures: maxFailures, window: window, lock: lock, now: now, entries: map[string]*failures{}}
}

func (l *Lockout) Locked(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	f, ok := l.entries[key]
	if !ok {
		return false, 0
	}
	if remaining := f.lockedUntil.Sub(l.now()); remaining > 0 {
		return true, remaining
	}
	return false, 0
}

func (l *Lockout) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	f, ok := l.entries[key]
	if !ok {
		f = &failures{}
		l.entries[key] = f
	}
	f.times = recent(f.times, now.Add(-l.window))
	f.times = append(f.times, now)
	if len(f.times) >= l.maxFailures {
		f.lockedUntil = now.Add(l.lock)
		f.times = nil
	}
}

func (l *Lockout) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}

func (l *Lockout) Sweep() {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	for key, f := range l.entries {
		f.times = recent(f.times, now.Add(-l.window))
		if len(f.times) == 0 && !f.lockedUntil.After(now) {
			delete(l.entries, key)
		}
	}
}

func (l *Lockout) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.entries)
}

func recent(times []time.Time, cutoff time.Time) []time.Time {
	kept := times[:0]
	for _, t := range times {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	return kept
}
```

- [ ] **Step 5: Rodar e ver passar**

Run: `cd backend && go test ./internal/ratelimit/ -race -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/go.mod backend/go.sum backend/internal/ratelimit
git commit -m "feat(ratelimit): adiciona limite por chave e bloqueio por falhas de login"
```

---

### Tarefa 11: `mail` — Mailer, Resend, driver de log, fake e templates

**Files:**
- Create: `backend/internal/mail/mail.go`, `backend/internal/mail/resend.go`, `backend/internal/mail/templates.go`
- Test: `backend/internal/mail/mail_test.go`

**Interfaces:**
- Produces:
  - `type mail.Message struct{ To, Subject, Text, HTML string }`
  - `type mail.Mailer interface{ Send(ctx context.Context, m Message) error }`
  - `func mail.NewResend(apiKey, from string) *ResendMailer`
  - `func mail.NewLog(logger *slog.Logger) *LogMailer`
  - `type mail.Fake struct{ Err error }` com `Send`, `Sent() []Message`, `LastTo(to string) (Message, bool)`, `LastToken(to string) string` (extrai `token=...` do último e-mail para `to`)
  - Templates: `mail.VerifyEmail(to, name, link string) Message`, `mail.AccountExists(to, name, loginLink, resetLink string) Message`, `mail.PasswordReset(to, name, link string) Message`, `mail.ForcedPasswordReset(to, name, link string) Message`, `mail.PasswordChanged(to, name string) Message`

- [ ] **Step 1: Escrever os testes que falham**

`backend/internal/mail/mail_test.go`:

```go
package mail

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func newTestResend(t *testing.T, handler http.HandlerFunc) *ResendMailer {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	m := NewResend("re_teste", "Truco <truco@empresa.com>")
	m.endpoint, m.backoff = srv.URL, time.Millisecond
	return m
}

func TestResendSendsExpectedRequest(t *testing.T) {
	var got map[string]any
	m := newTestResend(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer re_teste" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("requisição inesperada: %s %v", r.Method, r.Header)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"abc"}`))
	})
	err := m.Send(context.Background(), Message{To: "ana@empresa.com", Subject: "Oi", Text: "texto", HTML: "<p>html</p>"})
	if err != nil {
		t.Fatal(err)
	}
	to, _ := got["to"].([]any)
	if got["from"] != "Truco <truco@empresa.com>" || len(to) != 1 || to[0] != "ana@empresa.com" || got["subject"] != "Oi" || got["text"] != "texto" || got["html"] != "<p>html</p>" {
		t.Fatalf("corpo = %v", got)
	}
}

func TestResendRetriesServerErrors(t *testing.T) {
	var calls atomic.Int32
	m := newTestResend(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	if err := m.Send(context.Background(), Message{To: "a@b.com"}); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 3 {
		t.Fatalf("%d chamadas, quer 3", calls.Load())
	}
}

func TestResendDoesNotRetryClientErrors(t *testing.T) {
	var calls atomic.Int32
	m := newTestResend(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"from inválido"}`))
	})
	err := m.Send(context.Background(), Message{To: "a@b.com"})
	if err == nil || !strings.Contains(err.Error(), "422") || !strings.Contains(err.Error(), "from inválido") {
		t.Fatalf("err = %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("%d chamadas, quer 1", calls.Load())
	}
}

func TestResendGivesUpAfterThreeAttempts(t *testing.T) {
	var calls atomic.Int32
	m := newTestResend(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	if err := m.Send(context.Background(), Message{To: "a@b.com"}); err == nil {
		t.Fatal("esperava erro")
	}
	if calls.Load() != 3 {
		t.Fatalf("%d chamadas, quer 3", calls.Load())
	}
}

func TestFakeCapturesAndExtractsToken(t *testing.T) {
	f := &Fake{}
	_ = f.Send(context.Background(), VerifyEmail("ana@empresa.com", "Ana", "https://truco.test/verificar-email?token=AbC_-123"))
	if got := f.LastToken("ana@empresa.com"); got != "AbC_-123" {
		t.Fatalf("token = %q", got)
	}
	if _, ok := f.LastTo("bia@empresa.com"); ok {
		t.Fatal("não houve e-mail para bia")
	}
	f.Err = errors.New("falhou")
	if err := f.Send(context.Background(), Message{To: "x"}); err == nil {
		t.Fatal("Fake.Err deve ser devolvido")
	}
}

func TestTemplatesRenderLinksAndEscapeHTML(t *testing.T) {
	msgs := []Message{
		VerifyEmail("a@b.com", "<script>", "https://t/verificar-email?token=x"),
		AccountExists("a@b.com", "<script>", "https://t/entrar", "https://t/esqueci-senha"),
		PasswordReset("a@b.com", "<script>", "https://t/redefinir-senha?token=x"),
		ForcedPasswordReset("a@b.com", "<script>", "https://t/redefinir-senha?token=x"),
		PasswordChanged("a@b.com", "<script>"),
	}
	for _, m := range msgs {
		if m.To != "a@b.com" || m.Subject == "" || m.Text == "" || m.HTML == "" {
			t.Fatalf("mensagem incompleta: %+v", m)
		}
		if strings.Contains(m.HTML, "<script>") {
			t.Fatalf("HTML sem escape: %s", m.HTML)
		}
	}
	if !strings.Contains(msgs[0].Text, "https://t/verificar-email?token=x") || !strings.Contains(msgs[0].HTML, "https://t/verificar-email?token=x") {
		t.Fatal("link ausente na verificação")
	}
	if !strings.Contains(msgs[1].Text, "https://t/esqueci-senha") {
		t.Fatal("link de reset ausente em AccountExists")
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/mail/`
Expected: FAIL de compilação.

- [ ] **Step 3: Implementar**

`backend/internal/mail/mail.go`:

```go
// Package mail envia e-mails transacionais (verificação, reset de senha, avisos).
package mail

import (
	"context"
	"log/slog"
	"regexp"
	"slices"
	"sync"
)

type Message struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

type Mailer interface {
	Send(ctx context.Context, m Message) error
}

// LogMailer escreve o e-mail no log. Útil para quem roda sem chave do Resend.
type LogMailer struct{ log *slog.Logger }

func NewLog(logger *slog.Logger) *LogMailer { return &LogMailer{log: logger} }

func (l *LogMailer) Send(ctx context.Context, m Message) error {
	l.log.InfoContext(ctx, "e-mail (MAIL_DRIVER=log)", "to", m.To, "subject", m.Subject, "text", m.Text)
	return nil
}

// Fake guarda as mensagens em memória para os testes.
type Fake struct {
	mu   sync.Mutex
	sent []Message
	Err  error
}

func (f *Fake) Send(_ context.Context, m Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Err != nil {
		return f.Err
	}
	f.sent = append(f.sent, m)
	return nil
}

func (f *Fake) Sent() []Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.sent)
}

func (f *Fake) LastTo(to string) (Message, bool) {
	sent := f.Sent()
	for i := len(sent) - 1; i >= 0; i-- {
		if sent[i].To == to {
			return sent[i], true
		}
	}
	return Message{}, false
}

var tokenPattern = regexp.MustCompile(`token=([A-Za-z0-9_-]+)`)

// LastToken extrai o token do último e-mail enviado para `to` ("" se não houver).
func (f *Fake) LastToken(to string) string {
	m, ok := f.LastTo(to)
	if !ok {
		return ""
	}
	match := tokenPattern.FindStringSubmatch(m.Text)
	if match == nil {
		return ""
	}
	return match[1]
}
```

`backend/internal/mail/resend.go`:

```go
package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ResendMailer chama a API HTTP do Resend diretamente (um único POST, sem SDK).
type ResendMailer struct {
	apiKey   string
	from     string
	endpoint string
	client   *http.Client
	backoff  time.Duration
}

func NewResend(apiKey, from string) *ResendMailer {
	return &ResendMailer{apiKey: apiKey, from: from, endpoint: "https://api.resend.com/emails",
		client: &http.Client{Timeout: 10 * time.Second}, backoff: 500 * time.Millisecond}
}

// Send tenta até 3 vezes em erro de rede, 429 ou 5xx.
func (r *ResendMailer) Send(ctx context.Context, m Message) error {
	body, err := json.Marshal(map[string]any{"from": r.from, "to": []string{m.To}, "subject": m.Subject, "text": m.Text, "html": m.HTML})
	if err != nil {
		return err
	}
	var lastErr error
	for attempt := range 3 {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(r.backoff << (attempt - 1)):
			}
		}
		retry, err := r.post(ctx, body)
		if err == nil {
			return nil
		}
		lastErr = err
		if !retry {
			return err
		}
	}
	return lastErr
}

func (r *ResendMailer) post(ctx context.Context, body []byte) (retry bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint, bytes.NewReader(body))
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.client.Do(req)
	if err != nil {
		return true, fmt.Errorf("mail: falha de rede ao chamar o Resend: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 300 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return false, nil
	}
	detail, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	err = fmt.Errorf("mail: Resend respondeu %d: %s", resp.StatusCode, strings.TrimSpace(string(detail)))
	return resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500, err
}
```

`backend/internal/mail/templates.go`:

```go
package mail

import (
	htmltemplate "html/template"
	"strings"
	texttemplate "text/template"
)

type templateData struct {
	Name, Link, LoginLink, ResetLink string
}

type template struct {
	subject string
	text    *texttemplate.Template
	html    *htmltemplate.Template
}

func newTemplate(subject, text, html string) template {
	return template{
		subject: subject,
		text:    texttemplate.Must(texttemplate.New("text").Parse(text)),
		html:    htmltemplate.Must(htmltemplate.New("html").Parse(html)),
	}
}

func (t template) render(to string, data templateData) Message {
	var text, html strings.Builder
	// Os templates são fixos e testados; erro aqui é bug de programação.
	if err := t.text.Execute(&text, data); err != nil {
		panic(err)
	}
	if err := t.html.Execute(&html, data); err != nil {
		panic(err)
	}
	return Message{To: to, Subject: t.subject, Text: text.String(), HTML: html.String()}
}

var (
	verifyEmailTemplate = newTemplate("Confirme seu e-mail no Truco",
		"Olá, {{.Name}}!\n\nConfirme seu e-mail para ativar sua conta no Truco:\n{{.Link}}\n\nO link vale por 24 horas. Se você não pediu este cadastro, ignore esta mensagem.\n",
		`<p>Olá, {{.Name}}!</p><p>Confirme seu e-mail para ativar sua conta no Truco:</p><p><a href="{{.Link}}">Confirmar e-mail</a></p><p>O link vale por 24 horas. Se você não pediu este cadastro, ignore esta mensagem.</p>`)

	accountExistsTemplate = newTemplate("Você já tem uma conta no Truco",
		"Olá, {{.Name}}!\n\nAlguém tentou criar uma conta no Truco com este e-mail, mas ele já está cadastrado.\nPara entrar: {{.LoginLink}}\nEsqueceu a senha? {{.ResetLink}}\n\nSe não foi você, ignore esta mensagem.\n",
		`<p>Olá, {{.Name}}!</p><p>Alguém tentou criar uma conta no Truco com este e-mail, mas ele já está cadastrado.</p><p><a href="{{.LoginLink}}">Entrar</a> · <a href="{{.ResetLink}}">Esqueci minha senha</a></p><p>Se não foi você, ignore esta mensagem.</p>`)

	passwordResetTemplate = newTemplate("Redefinição de senha do Truco",
		"Olá, {{.Name}}!\n\nPara criar uma nova senha, acesse:\n{{.Link}}\n\nO link vale por 30 minutos. Se você não pediu, ignore esta mensagem: sua senha continua a mesma.\n",
		`<p>Olá, {{.Name}}!</p><p>Para criar uma nova senha, acesse:</p><p><a href="{{.Link}}">Redefinir senha</a></p><p>O link vale por 30 minutos. Se você não pediu, ignore esta mensagem: sua senha continua a mesma.</p>`)

	forcedResetTemplate = newTemplate("Você precisa redefinir sua senha do Truco",
		"Olá, {{.Name}}!\n\nUm administrador pediu que você redefina sua senha. Suas sessões foram encerradas.\nCrie uma nova senha em:\n{{.Link}}\n\nO link vale por 30 minutos; depois disso, use \"Esqueci minha senha\".\n",
		`<p>Olá, {{.Name}}!</p><p>Um administrador pediu que você redefina sua senha. Suas sessões foram encerradas.</p><p><a href="{{.Link}}">Criar nova senha</a></p><p>O link vale por 30 minutos; depois disso, use "Esqueci minha senha".</p>`)

	passwordChangedTemplate = newTemplate("Sua senha do Truco foi alterada",
		"Olá, {{.Name}}!\n\nA senha da sua conta no Truco acabou de ser alterada e as outras sessões foram encerradas.\nSe não foi você, use \"Esqueci minha senha\" imediatamente.\n",
		`<p>Olá, {{.Name}}!</p><p>A senha da sua conta no Truco acabou de ser alterada e as outras sessões foram encerradas.</p><p>Se não foi você, use "Esqueci minha senha" imediatamente.</p>`)
)

func VerifyEmail(to, name, link string) Message {
	return verifyEmailTemplate.render(to, templateData{Name: name, Link: link})
}

func AccountExists(to, name, loginLink, resetLink string) Message {
	return accountExistsTemplate.render(to, templateData{Name: name, LoginLink: loginLink, ResetLink: resetLink})
}

func PasswordReset(to, name, link string) Message {
	return passwordResetTemplate.render(to, templateData{Name: name, Link: link})
}

func ForcedPasswordReset(to, name, link string) Message {
	return forcedResetTemplate.render(to, templateData{Name: name, Link: link})
}

func PasswordChanged(to, name string) Message {
	return passwordChangedTemplate.render(to, templateData{Name: name})
}
```

- [ ] **Step 4: Rodar e ver passar**

Run: `cd backend && go test ./internal/mail/ -race -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/mail
git commit -m "feat(mail): adiciona envio via Resend, driver de log e templates em pt-BR"
```

---
### Tarefa 12: `auth.Service` — núcleo, sessões, cookie e Origin

**Files:**
- Create: `backend/internal/auth/service.go`, `backend/internal/auth/session.go`
- Test: `backend/internal/auth/main_test.go`, `backend/internal/auth/session_test.go`

**Interfaces:**
- Consumes: `dbgen` (Tarefa 7), `db.WithTx`, `mail.Mailer`, `Hasher`, `NewToken`/`HashToken`.
- Produces:

```go
type Options struct {
	Pool       *pgxpool.Pool
	Mailer     mail.Mailer
	Hasher     *Hasher
	Now        func() time.Time // padrão time.Now
	BaseURL    string           // ex.: https://truco.empresa.com.br (links dos e-mails)
	Cookies    CookieConfig
	Logger     *slog.Logger     // padrão slog.Default()
	Background func(func())     // padrão: nova goroutine; testes passam execução síncrona
}
func NewService(o Options) *Service

type User struct{ ID, Username, Email, DisplayName string; IsAdmin bool; CreatedAt time.Time }
type Principal struct{ UserID, SessionID, Username, Email, DisplayName string; IsAdmin bool }
type SessionMeta struct{ UserAgent, IP string }

const SessionIdleTTL = 30 * 24 * time.Hour
const SessionAbsoluteTTL = 90 * 24 * time.Hour
const SessionTouchInterval = 24 * time.Hour

func (s *Service) GetUser(ctx, userID string) (User, error)            // ErrUserNotFound
func (s *Service) BootstrapDomains(ctx, domains []string) error
func (s *Service) Authenticate(ctx, token string) (Principal, error)   // ErrUnauthenticated
func (s *Service) Logout(ctx, token string) error
func (s *Service) LogoutAll(ctx, userID string) error
func (s *Service) CleanupExpired(ctx) (sessions, tokens int64, err error)
func (s *Service) SessionFromRequest(r *http.Request) (Principal, error)
func (s *Service) Cookies() CookieConfig

type CookieConfig struct{ Secure bool }
func (c CookieConfig) Name() string                 // "__Host-truco_session" ou "truco_session"
func (c CookieConfig) Set(w http.ResponseWriter, token string)
func (c CookieConfig) Clear(w http.ResponseWriter)
func (c CookieConfig) Read(r *http.Request) string

func CheckOrigin(r *http.Request, allowedOrigin string) bool
```

Internos usados pela Tarefa 13: `s.q *dbgen.Queries`, `s.pool`, `s.hasher`, `s.now()`, `s.link(path) string`, `s.sendLater(mail.Message)`, `s.createSession(ctx, q *dbgen.Queries, userID string, meta SessionMeta) (string, error)`, `userFromRow(dbgen.User) User`.

- [ ] **Step 1: Escrever os testes que falham**

`backend/internal/auth/main_test.go`:

```go
package auth

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/mail"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/testclock"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/testdb"
)

func TestMain(m *testing.M) { testdb.Run(m) }

var t0 = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

type fixture struct {
	svc   *Service
	pool  *pgxpool.Pool
	mail  *mail.Fake
	clock *testclock.Clock
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	pool := testdb.New(t)
	f := fixture{pool: pool, mail: &mail.Fake{}, clock: testclock.New(t0)}
	f.svc = NewService(Options{
		Pool: pool, Mailer: f.mail, Hasher: NewHasher(TestingParams, 4), Now: f.clock.Now,
		BaseURL: "https://truco.test", Cookies: CookieConfig{Secure: true},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Background: func(fn func()) { fn() },
	})
	if err := f.svc.BootstrapDomains(context.Background(), []string{"empresa.com"}); err != nil {
		t.Fatal(err)
	}
	return f
}

// newUser cria um usuário verificado com a senha informada (sem passar pelo fluxo de cadastro).
func (f fixture) newUser(t *testing.T, username, password string) string {
	t.Helper()
	hash, err := f.svc.hasher.Hash(password)
	if err != nil {
		t.Fatal(err)
	}
	var id string
	err = f.pool.QueryRow(context.Background(),
		`INSERT INTO users (username, email, display_name, password_hash, email_verified_at)
		 VALUES ($1, $1 || '@empresa.com', initcap($1), $2, now()) RETURNING id::text`, username, hash).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
```

`backend/internal/auth/session_test.go`:

```go
package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAuthenticateSlidingWindowAndAbsoluteCap(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	userID := f.newUser(t, "ana", "senha-da-ana-123")
	token, err := f.svc.createSession(ctx, f.svc.q, userID, SessionMeta{UserAgent: "teste", IP: "203.0.113.7"})
	if err != nil {
		t.Fatal(err)
	}
	for day := 29; day <= 87; day += 29 { // usa a cada 29 dias: a janela de 30 dias desliza
		f.clock.Advance(29 * 24 * time.Hour)
		p, err := f.svc.Authenticate(ctx, token)
		if err != nil || p.UserID != userID || p.Username != "ana" {
			t.Fatalf("dia %d: p=%+v err=%v", day, p, err)
		}
	}
	f.clock.Advance(4 * 24 * time.Hour) // dia 91: passou do teto absoluto de 90 dias
	if _, err := f.svc.Authenticate(ctx, token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("após 90 dias: err = %v", err)
	}
}

func TestAuthenticateExpiresAfterIdleWindow(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	token, _ := f.svc.createSession(ctx, f.svc.q, f.newUser(t, "ana", "senha-da-ana-123"), SessionMeta{})
	f.clock.Advance(30*24*time.Hour + time.Second)
	if _, err := f.svc.Authenticate(ctx, token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("err = %v", err)
	}
}

func TestAuthenticateTouchesAtMostDaily(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	token, _ := f.svc.createSession(ctx, f.svc.q, f.newUser(t, "ana", "senha-da-ana-123"), SessionMeta{})
	lastSeen := func() time.Time {
		var ts time.Time
		if err := f.pool.QueryRow(ctx, "SELECT last_seen_at FROM sessions").Scan(&ts); err != nil {
			t.Fatal(err)
		}
		return ts.UTC()
	}
	f.clock.Advance(23 * time.Hour)
	_, _ = f.svc.Authenticate(ctx, token)
	if !lastSeen().Equal(t0) {
		t.Fatalf("tocou antes de 24h: %v", lastSeen())
	}
	f.clock.Advance(2 * time.Hour)
	_, _ = f.svc.Authenticate(ctx, token)
	if !lastSeen().Equal(t0.Add(25 * time.Hour)) {
		t.Fatalf("não tocou após 24h: %v", lastSeen())
	}
}

func TestAuthenticateRejectsDisabledUserEvenWithLiveSession(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	userID := f.newUser(t, "ana", "senha-da-ana-123")
	token, _ := f.svc.createSession(ctx, f.svc.q, userID, SessionMeta{})
	if _, err := f.pool.Exec(ctx, "UPDATE users SET disabled_at = now() WHERE id = $1", userID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Authenticate(ctx, token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("err = %v", err)
	}
}

func TestLogoutAndLogoutAll(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	userID := f.newUser(t, "ana", "senha-da-ana-123")
	a, _ := f.svc.createSession(ctx, f.svc.q, userID, SessionMeta{})
	b, _ := f.svc.createSession(ctx, f.svc.q, userID, SessionMeta{})
	if err := f.svc.Logout(ctx, a); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Authenticate(ctx, a); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("logout não revogou a sessão")
	}
	if _, err := f.svc.Authenticate(ctx, b); err != nil {
		t.Fatal("logout revogou a sessão errada")
	}
	if err := f.svc.LogoutAll(ctx, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Authenticate(ctx, b); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("logout-all não revogou")
	}
	if _, err := f.svc.Authenticate(ctx, ""); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("token vazio deve ser não autenticado")
	}
}

func TestCleanupExpired(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	userID := f.newUser(t, "ana", "senha-da-ana-123")
	_, _ = f.svc.createSession(ctx, f.svc.q, userID, SessionMeta{})
	f.clock.Advance(31 * 24 * time.Hour)
	_, _ = f.svc.createSession(ctx, f.svc.q, userID, SessionMeta{})
	sessions, _, err := f.svc.CleanupExpired(ctx)
	if err != nil || sessions != 1 {
		t.Fatalf("removidas %d, err %v", sessions, err)
	}
}

func TestCookieConfig(t *testing.T) {
	rec := httptest.NewRecorder()
	CookieConfig{Secure: true}.Set(rec, "tok")
	c := rec.Result().Cookies()[0]
	if c.Name != "__Host-truco_session" || c.Value != "tok" || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.MaxAge != 7776000 {
		t.Fatalf("cookie = %+v", c)
	}
	if (CookieConfig{Secure: false}).Name() != "truco_session" {
		t.Fatal("nome sem Secure")
	}
	rec = httptest.NewRecorder()
	CookieConfig{Secure: true}.Clear(rec)
	if c := rec.Result().Cookies()[0]; c.MaxAge >= 0 || c.Value != "" {
		t.Fatalf("Clear = %+v", c)
	}
}

func TestSessionFromRequest(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	token, _ := f.svc.createSession(ctx, f.svc.q, f.newUser(t, "ana", "senha-da-ana-123"), SessionMeta{})
	r := httptest.NewRequest(http.MethodGet, "/ws", nil)
	r.AddCookie(&http.Cookie{Name: "__Host-truco_session", Value: token})
	p, err := f.svc.SessionFromRequest(r)
	if err != nil || p.Username != "ana" {
		t.Fatalf("p=%+v err=%v", p, err)
	}
	if _, err := f.svc.SessionFromRequest(httptest.NewRequest(http.MethodGet, "/ws", nil)); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("sem cookie: err = %v", err)
	}
}

func TestCheckOrigin(t *testing.T) {
	const allowed = "https://truco.test"
	cases := []struct {
		origin, fetchSite string
		want              bool
	}{
		{"https://truco.test", "", true},
		{"https://evil.test", "same-origin", false},
		{"null", "", false},
		{"", "same-origin", true},
		{"", "cross-site", false},
		{"", "", false},
	}
	for _, c := range cases {
		r := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
		if c.origin != "" {
			r.Header.Set("Origin", c.origin)
		}
		if c.fetchSite != "" {
			r.Header.Set("Sec-Fetch-Site", c.fetchSite)
		}
		if got := CheckOrigin(r, allowed); got != c.want {
			t.Errorf("origin=%q site=%q: %v, quer %v", c.origin, c.fetchSite, got, c.want)
		}
	}
}

func TestBootstrapDomains(t *testing.T) {
	f := newFixture(t) // já populou "empresa.com"
	ctx := context.Background()
	if err := f.svc.BootstrapDomains(ctx, []string{"outra.com"}); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = f.pool.QueryRow(ctx, "SELECT count(*) FROM allowed_email_domains WHERE domain = 'outra.com'").Scan(&n)
	if n != 0 {
		t.Fatal("bootstrap só pode popular tabela vazia")
	}
	_, _ = f.pool.Exec(ctx, "DELETE FROM allowed_email_domains")
	if err := f.svc.BootstrapDomains(ctx, nil); err == nil {
		t.Fatal("tabela vazia sem ALLOWED_EMAIL_DOMAINS deve falhar")
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/auth/ -run 'Authenticate|Logout|Cleanup|Cookie|SessionFrom|CheckOrigin|Bootstrap'`
Expected: FAIL de compilação.

- [ ] **Step 3: Implementar `service.go`**

```go
package auth

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/db"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/db/dbgen"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/mail"
)

type Options struct {
	Pool       *pgxpool.Pool
	Mailer     mail.Mailer
	Hasher     *Hasher
	Now        func() time.Time
	BaseURL    string
	Cookies    CookieConfig
	Logger     *slog.Logger
	Background func(func())
}

type Service struct {
	pool       *pgxpool.Pool
	q          *dbgen.Queries
	mailer     mail.Mailer
	hasher     *Hasher
	now        func() time.Time
	baseURL    string
	cookies    CookieConfig
	log        *slog.Logger
	background func(func())
}

func NewService(o Options) *Service {
	s := &Service{pool: o.Pool, q: dbgen.New(o.Pool), mailer: o.Mailer, hasher: o.Hasher, now: o.Now,
		baseURL: o.BaseURL, cookies: o.Cookies, log: o.Logger, background: o.Background}
	if s.now == nil {
		s.now = time.Now
	}
	if s.log == nil {
		s.log = slog.Default()
	}
	if s.background == nil {
		s.background = func(fn func()) { go fn() }
	}
	return s
}

type User struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	Email       string    `json:"email"`
	DisplayName string    `json:"displayName"`
	IsAdmin     bool      `json:"isAdmin"`
	CreatedAt   time.Time `json:"createdAt"`
}

func userFromRow(u dbgen.User) User {
	return User{ID: u.ID, Username: u.Username, Email: u.Email, DisplayName: u.DisplayName, IsAdmin: u.IsAdmin, CreatedAt: u.CreatedAt}
}

func (s *Service) Cookies() CookieConfig { return s.cookies }

func (s *Service) GetUser(ctx context.Context, userID string) (User, error) {
	row, err := s.q.GetUserByID(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, err
	}
	return userFromRow(row), nil
}

// BootstrapDomains popula allowed_email_domains a partir da configuração, só se a tabela estiver vazia.
func (s *Service) BootstrapDomains(ctx context.Context, domains []string) error {
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		if err := q.LockDomains(ctx); err != nil {
			return err
		}
		count, err := q.CountDomains(ctx)
		if err != nil || count > 0 {
			return err
		}
		if len(domains) == 0 {
			return errors.New("auth: nenhum domínio de e-mail permitido; defina ALLOWED_EMAIL_DOMAINS")
		}
		for _, d := range domains {
			if err := q.InsertDomain(ctx, dbgen.InsertDomainParams{Domain: d, Now: s.now()}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Service) link(path string) string { return s.baseURL + path }

// sendLater envia fora da requisição: o tempo de resposta não revela se houve envio.
func (s *Service) sendLater(m mail.Message) {
	s.background(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.mailer.Send(ctx, m); err != nil {
			s.log.Error("falha ao enviar e-mail", "to", m.To, "subject", m.Subject, "err", err)
		}
	})
}
```

> `LockDomains` (`SELECT … FOR UPDATE`) numa tabela vazia não trava nada; a corrida de dois bootstraps simultâneos termina em `ON CONFLICT DO NOTHING`, então o resultado é o mesmo.

- [ ] **Step 4: Implementar `session.go`**

```go
package auth

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/db/dbgen"
)

const (
	SessionIdleTTL       = 30 * 24 * time.Hour
	SessionAbsoluteTTL   = 90 * 24 * time.Hour
	SessionTouchInterval = 24 * time.Hour
	maxUserAgent         = 256
)

type Principal struct {
	UserID      string
	SessionID   string
	Username    string
	Email       string
	DisplayName string
	IsAdmin     bool
}

type SessionMeta struct {
	UserAgent string
	IP        string
}

func (s *Service) createSession(ctx context.Context, q *dbgen.Queries, userID string, meta SessionMeta) (string, error) {
	now := s.now()
	token, hash := NewToken()
	userAgent := meta.UserAgent
	if len(userAgent) > maxUserAgent {
		userAgent = userAgent[:maxUserAgent]
	}
	_, err := q.CreateSession(ctx, dbgen.CreateSessionParams{
		UserID: userID, TokenHash: hash, Now: now, ExpiresAt: now.Add(SessionIdleTTL),
		AbsoluteExpiresAt: now.Add(SessionAbsoluteTTL), UserAgent: userAgent, Ip: meta.IP,
	})
	if err != nil {
		return "", err
	}
	return token, nil
}

// Authenticate valida o token e renova a janela deslizante no máximo uma vez a cada 24h.
func (s *Service) Authenticate(ctx context.Context, token string) (Principal, error) {
	if token == "" {
		return Principal{}, ErrUnauthenticated
	}
	now := s.now()
	row, err := s.q.GetSessionPrincipal(ctx, dbgen.GetSessionPrincipalParams{TokenHash: HashToken(token), Now: now})
	if errors.Is(err, pgx.ErrNoRows) {
		return Principal{}, ErrUnauthenticated
	}
	if err != nil {
		return Principal{}, err
	}
	if now.Sub(row.LastSeenAt) > SessionTouchInterval {
		if err := s.q.TouchSession(ctx, dbgen.TouchSessionParams{Now: now, ExpiresAt: now.Add(SessionIdleTTL), ID: row.SessionID}); err != nil {
			return Principal{}, err
		}
	}
	return Principal{UserID: row.UserID, SessionID: row.SessionID, Username: row.Username, Email: row.Email,
		DisplayName: row.DisplayName, IsAdmin: row.IsAdmin}, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.q.DeleteSessionByTokenHash(ctx, HashToken(token))
}

func (s *Service) LogoutAll(ctx context.Context, userID string) error {
	return s.q.DeleteUserSessions(ctx, userID)
}

// CleanupExpired remove sessões expiradas e tokens vencidos ou usados há mais de 7 dias.
func (s *Service) CleanupExpired(ctx context.Context) (sessions, tokens int64, err error) {
	now := s.now()
	if sessions, err = s.q.DeleteExpiredSessions(ctx, now); err != nil {
		return 0, 0, err
	}
	tokens, err = s.q.DeleteStaleTokens(ctx, dbgen.DeleteStaleTokensParams{Now: now, UsedBefore: now.Add(-7 * 24 * time.Hour)})
	return sessions, tokens, err
}

// SessionFromRequest autentica pelo cookie. Também é usado no handshake do WebSocket (Spec 3).
func (s *Service) SessionFromRequest(r *http.Request) (Principal, error) {
	return s.Authenticate(r.Context(), s.cookies.Read(r))
}

type CookieConfig struct {
	Secure bool
}

func (c CookieConfig) Name() string {
	if c.Secure {
		return "__Host-truco_session"
	}
	return "truco_session"
}

func (c CookieConfig) Set(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{Name: c.Name(), Value: token, Path: "/", MaxAge: int(SessionAbsoluteTTL.Seconds()),
		HttpOnly: true, Secure: c.Secure, SameSite: http.SameSiteLaxMode})
}

func (c CookieConfig) Clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: c.Name(), Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: c.Secure, SameSite: http.SameSiteLaxMode})
}

func (c CookieConfig) Read(r *http.Request) string {
	cookie, err := r.Cookie(c.Name())
	if err != nil {
		return ""
	}
	return cookie.Value
}

// CheckOrigin aceita a requisição se o Origin for o do app ou, sem Origin,
// se o navegador declarar Sec-Fetch-Site: same-origin.
func CheckOrigin(r *http.Request, allowedOrigin string) bool {
	if origin := r.Header.Get("Origin"); origin != "" {
		return origin == allowedOrigin
	}
	return r.Header.Get("Sec-Fetch-Site") == "same-origin"
}
```

- [ ] **Step 5: Rodar e ver passar**

Run: `cd backend && go test ./internal/auth/ -race -v`
Expected: PASS (testes de integração rodam com Docker; unitários das tarefas 8–9 continuam verdes).

- [ ] **Step 6: Commit**

```bash
git add backend/internal/auth/service.go backend/internal/auth/session.go backend/internal/auth/main_test.go backend/internal/auth/session_test.go
git commit -m "feat(auth): adiciona sessões revogáveis com janela deslizante e cookie seguro"
```

---

### Tarefa 13: `auth.Service` — cadastro, verificação, login, recuperação e troca de senha

**Files:**
- Create: `backend/internal/auth/flows.go`
- Test: `backend/internal/auth/flows_test.go`

**Interfaces:**
- Consumes: tudo da Tarefa 12; templates de `mail` (Tarefa 11); validação (Tarefa 9).
- Produces:

```go
type RegisterInput struct{ Username, Email, DisplayName, Password string }
type LoginResult struct{ Token string; User User }
func (s *Service) Register(ctx, in RegisterInput) error                 // FieldErrors, ErrEmailDomainNotAllowed, ErrUsernameTaken
func (s *Service) VerifyEmail(ctx, token string) error                  // ErrInvalidToken
func (s *Service) ResendVerification(ctx, email string) error
func (s *Service) Login(ctx, login, password string, meta SessionMeta) (LoginResult, error)
func (s *Service) ForgotPassword(ctx, email string) error
func (s *Service) ResetPassword(ctx, token, password string) error      // ErrInvalidToken, FieldErrors
func (s *Service) ChangePassword(ctx, p Principal, current, next string, meta SessionMeta) (string, error)
func (s *Service) UpdateDisplayName(ctx, userID, name string) (User, error)
func (s *Service) SendPasswordReset(ctx, userID string, forced bool) error   // usado pelo admin
```

Links: `/verificar-email?token=…`, `/redefinir-senha?token=…`, `/entrar`, `/esqueci-senha` (prefixados por `BaseURL`).

- [ ] **Step 1: Escrever os testes que falham**

`backend/internal/auth/flows_test.go`:

```go
package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

const goodPassword = "cavalo-bateria-grampo"

func (f fixture) register(t *testing.T, username string) string {
	t.Helper()
	email := username + "@empresa.com"
	err := f.svc.Register(context.Background(), RegisterInput{Username: username, Email: email, DisplayName: "Nome " + username, Password: goodPassword})
	if err != nil {
		t.Fatal(err)
	}
	return email
}

func (f fixture) registerVerified(t *testing.T, username string) string {
	t.Helper()
	email := f.register(t, username)
	if err := f.svc.VerifyEmail(context.Background(), f.mail.LastToken(email)); err != nil {
		t.Fatal(err)
	}
	return email
}

func (f fixture) countUsers(t *testing.T) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(context.Background(), "SELECT count(*) FROM users").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestRegisterVerifyLogin(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	email := f.register(t, "ana")
	if msg, ok := f.mail.LastTo(email); !ok || !strings.Contains(msg.Text, "https://truco.test/verificar-email?token=") {
		t.Fatalf("e-mail de verificação = %+v", msg)
	}
	if _, err := f.svc.Login(ctx, "ana", goodPassword, SessionMeta{}); !errors.Is(err, ErrEmailNotVerified) {
		t.Fatalf("login antes de verificar: %v", err)
	}
	token := f.mail.LastToken(email)
	if err := f.svc.VerifyEmail(ctx, token); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.VerifyEmail(ctx, token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("segundo clique no link: %v", err)
	}
	res, err := f.svc.Login(ctx, "ana", goodPassword, SessionMeta{})
	if err != nil || res.Token == "" || res.User.Username != "ana" || res.User.DisplayName != "Nome ana" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if _, err := f.svc.Authenticate(ctx, res.Token); err != nil {
		t.Fatal(err)
	}
}

func TestRegisterValidation(t *testing.T) {
	f := newFixture(t)
	err := f.svc.Register(context.Background(), RegisterInput{Username: "a!", Email: "nao-e-email", DisplayName: " ", Password: "curta"})
	var fields FieldErrors
	if !errors.As(err, &fields) {
		t.Fatalf("err = %v", err)
	}
	for _, key := range []string{"username", "email", "displayName", "password"} {
		if fields[key] == "" {
			t.Errorf("faltou erro em %s: %v", key, fields)
		}
	}
}

func TestRegisterRejectsDomain(t *testing.T) {
	f := newFixture(t)
	err := f.svc.Register(context.Background(), RegisterInput{Username: "ana", Email: "ana@gmail.com", DisplayName: "Ana", Password: goodPassword})
	if !errors.Is(err, ErrEmailDomainNotAllowed) {
		t.Fatalf("err = %v", err)
	}
}

func TestRegisterExistingEmailIsSilentAndCaseInsensitive(t *testing.T) {
	f := newFixture(t)
	f.registerVerified(t, "ana")
	before := len(f.mail.Sent())
	err := f.svc.Register(context.Background(), RegisterInput{Username: "outra", Email: "  ANA@Empresa.COM ", DisplayName: "X", Password: goodPassword})
	if err != nil {
		t.Fatalf("e-mail existente deve responder como sucesso: %v", err)
	}
	if f.countUsers(t) != 1 {
		t.Fatal("não pode criar segunda conta para o mesmo e-mail")
	}
	sent := f.mail.Sent()
	if len(sent) != before+1 || sent[len(sent)-1].Subject != "Você já tem uma conta no Truco" {
		t.Fatalf("esperava aviso de conta existente, enviados: %+v", sent[before:])
	}
}

func TestRegisterUsernameTaken(t *testing.T) {
	f := newFixture(t)
	f.register(t, "ana")
	err := f.svc.Register(context.Background(), RegisterInput{Username: "ANA", Email: "outra@empresa.com", DisplayName: "X", Password: goodPassword})
	if !errors.Is(err, ErrUsernameTaken) {
		t.Fatalf("err = %v", err)
	}
}

func TestVerificationLinkExpiresAndResendInvalidatesOld(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	email := f.register(t, "ana")
	first := f.mail.LastToken(email)
	if err := f.svc.ResendVerification(ctx, " ANA@empresa.com"); err != nil {
		t.Fatal(err)
	}
	second := f.mail.LastToken(email)
	if second == first {
		t.Fatal("reenvio deve gerar token novo")
	}
	if err := f.svc.VerifyEmail(ctx, first); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("token antigo: %v", err)
	}
	f.clock.Advance(24*time.Hour + time.Second)
	if err := f.svc.VerifyEmail(ctx, second); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("token expirado: %v", err)
	}
	before := len(f.mail.Sent())
	_ = f.svc.ResendVerification(ctx, "ninguem@empresa.com")
	if len(f.mail.Sent()) != before {
		t.Fatal("e-mail inexistente não recebe nada")
	}
}

func TestLoginVariants(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.registerVerified(t, "ana")
	for _, login := range []string{"ana", "  ANA ", "ana@empresa.com", " Ana@Empresa.COM "} {
		if _, err := f.svc.Login(ctx, login, goodPassword, SessionMeta{}); err != nil {
			t.Errorf("login %q: %v", login, err)
		}
	}
	if _, err := f.svc.Login(ctx, "ana", "senha-errada-123", SessionMeta{}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("senha errada: %v", err)
	}
	if _, err := f.svc.Login(ctx, "ninguem", goodPassword, SessionMeta{}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("usuário inexistente: %v", err)
	}
}

func TestLoginBlockedStatesOnlyAfterCorrectPassword(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.registerVerified(t, "ana")
	_, _ = f.pool.Exec(ctx, "UPDATE users SET disabled_at = now() WHERE username = 'ana'")
	if _, err := f.svc.Login(ctx, "ana", "senha-errada-123", SessionMeta{}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("senha errada em conta desativada não pode revelar o estado: %v", err)
	}
	if _, err := f.svc.Login(ctx, "ana", goodPassword, SessionMeta{}); !errors.Is(err, ErrAccountDisabled) {
		t.Fatalf("desativada: %v", err)
	}
	_, _ = f.pool.Exec(ctx, "UPDATE users SET disabled_at = NULL, must_reset_password = true WHERE username = 'ana'")
	if _, err := f.svc.Login(ctx, "ana", goodPassword, SessionMeta{}); !errors.Is(err, ErrPasswordResetRequired) {
		t.Fatalf("reset obrigatório: %v", err)
	}
}

func TestLoginRehashesWhenParamsChange(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.registerVerified(t, "ana")
	stronger := TestingParams
	stronger.Time = 2
	f.svc.hasher = NewHasher(stronger, 4)
	if _, err := f.svc.Login(ctx, "ana", goodPassword, SessionMeta{}); err != nil {
		t.Fatal(err)
	}
	var hash string
	_ = f.pool.QueryRow(ctx, "SELECT password_hash FROM users WHERE username = 'ana'").Scan(&hash)
	if !strings.Contains(hash, "t=2") {
		t.Fatalf("hash não foi atualizado: %s", hash)
	}
}

func TestForgotAndResetPassword(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	email := f.registerVerified(t, "ana")
	old, _ := f.svc.Login(ctx, "ana", goodPassword, SessionMeta{})
	if err := f.svc.ForgotPassword(ctx, " ANA@empresa.com "); err != nil {
		t.Fatal(err)
	}
	first := f.mail.LastToken(email)
	_ = f.svc.ForgotPassword(ctx, email)
	second := f.mail.LastToken(email)
	if err := f.svc.ResetPassword(ctx, first, "nova-senha-segura-1"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("link antigo de reset: %v", err)
	}
	var fields FieldErrors
	if err := f.svc.ResetPassword(ctx, second, "curta"); !errors.As(err, &fields) {
		t.Fatalf("senha inválida: %v", err)
	}
	if err := f.svc.ResetPassword(ctx, second, "nova-senha-segura-1"); err != nil {
		t.Fatalf("o token deve sobreviver a uma senha inválida: %v", err)
	}
	if err := f.svc.ResetPassword(ctx, second, "outra-senha-segura-2"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("token reutilizado: %v", err)
	}
	if _, err := f.svc.Authenticate(ctx, old.Token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("reset deve derrubar todas as sessões")
	}
	if _, err := f.svc.Login(ctx, "ana", goodPassword, SessionMeta{}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatal("senha antiga ainda funciona")
	}
	if _, err := f.svc.Login(ctx, "ana", "nova-senha-segura-1", SessionMeta{}); err != nil {
		t.Fatal(err)
	}
}

func TestResetLinkExpiresIn30Minutes(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	email := f.registerVerified(t, "ana")
	_ = f.svc.ForgotPassword(ctx, email)
	f.clock.Advance(31 * time.Minute)
	if err := f.svc.ResetPassword(ctx, f.mail.LastToken(email), "nova-senha-segura-1"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("err = %v", err)
	}
}

func TestForgotForUnverifiedResendsVerification(t *testing.T) {
	f := newFixture(t)
	email := f.register(t, "ana")
	_ = f.svc.ForgotPassword(context.Background(), email)
	msg, _ := f.mail.LastTo(email)
	if msg.Subject != "Confirme seu e-mail no Truco" {
		t.Fatalf("assunto = %q", msg.Subject)
	}
}

func TestResetClearsMustResetAndVerifiesEmail(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	email := f.registerVerified(t, "ana")
	_, _ = f.pool.Exec(ctx, "UPDATE users SET must_reset_password = true WHERE username = 'ana'")
	var userID string
	_ = f.pool.QueryRow(ctx, "SELECT id::text FROM users WHERE username = 'ana'").Scan(&userID)
	if err := f.svc.SendPasswordReset(ctx, userID, true); err != nil {
		t.Fatal(err)
	}
	msg, _ := f.mail.LastTo(email)
	if msg.Subject != "Você precisa redefinir sua senha do Truco" {
		t.Fatalf("assunto = %q", msg.Subject)
	}
	if err := f.svc.ResetPassword(ctx, f.mail.LastToken(email), "nova-senha-segura-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Login(ctx, "ana", "nova-senha-segura-1", SessionMeta{}); err != nil {
		t.Fatalf("login após reset forçado: %v", err)
	}
}

func TestChangePasswordRotatesSession(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	email := f.registerVerified(t, "ana")
	a, _ := f.svc.Login(ctx, "ana", goodPassword, SessionMeta{})
	b, _ := f.svc.Login(ctx, "ana", goodPassword, SessionMeta{})
	p, _ := f.svc.Authenticate(ctx, a.Token)
	if _, err := f.svc.ChangePassword(ctx, p, "errada-errada-1", "nova-senha-segura-1", SessionMeta{}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("senha atual errada: %v", err)
	}
	var fields FieldErrors
	if _, err := f.svc.ChangePassword(ctx, p, goodPassword, "ana-nova-senha-1", SessionMeta{}); !errors.As(err, &fields) || fields["newPassword"] == "" {
		t.Fatalf("nova senha com username: %v", err)
	}
	newToken, err := f.svc.ChangePassword(ctx, p, goodPassword, "nova-senha-segura-1", SessionMeta{})
	if err != nil || newToken == "" {
		t.Fatal(err)
	}
	for _, old := range []string{a.Token, b.Token} {
		if _, err := f.svc.Authenticate(ctx, old); !errors.Is(err, ErrUnauthenticated) {
			t.Fatal("sessões antigas devem cair")
		}
	}
	if _, err := f.svc.Authenticate(ctx, newToken); err != nil {
		t.Fatal("sessão rotacionada deve valer")
	}
	if msg, _ := f.mail.LastTo(email); msg.Subject != "Sua senha do Truco foi alterada" {
		t.Fatalf("aviso não enviado: %q", msg.Subject)
	}
}

func TestUpdateDisplayName(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.registerVerified(t, "ana")
	res, _ := f.svc.Login(ctx, "ana", goodPassword, SessionMeta{})
	u, err := f.svc.UpdateDisplayName(ctx, res.User.ID, "  Ana Clara ")
	if err != nil || u.DisplayName != "Ana Clara" {
		t.Fatalf("u=%+v err=%v", u, err)
	}
	var fields FieldErrors
	if _, err := f.svc.UpdateDisplayName(ctx, res.User.ID, ""); !errors.As(err, &fields) {
		t.Fatalf("nome vazio: %v", err)
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/auth/ -run 'Register|Verification|Login|Forgot|Reset|Change|UpdateDisplay'`
Expected: FAIL de compilação.

- [ ] **Step 3: Implementar**

`backend/internal/auth/flows.go`:

```go
package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/db"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/db/dbgen"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/mail"
)

const (
	purposeVerify = "verify_email"
	purposeReset  = "password_reset"
	verifyTTL     = 24 * time.Hour
	resetTTL      = 30 * time.Minute
)

var errEmailTaken = errors.New("auth: e-mail já cadastrado")

type RegisterInput struct {
	Username, Email, DisplayName, Password string
}

type LoginResult struct {
	Token string
	User  User
}

func issueToken(ctx context.Context, q *dbgen.Queries, userID, purpose string, now time.Time, ttl time.Duration) (string, error) {
	if err := q.DeleteUnusedTokens(ctx, dbgen.DeleteUnusedTokensParams{UserID: userID, Purpose: purpose}); err != nil {
		return "", err
	}
	token, hash := NewToken()
	err := q.CreateUserToken(ctx, dbgen.CreateUserTokenParams{UserID: userID, Purpose: purpose, TokenHash: hash, Now: now, ExpiresAt: now.Add(ttl)})
	return token, err
}

func mapUserInsertError(err error) error {
	switch {
	case db.IsUniqueViolation(err, "users_username_key"):
		return ErrUsernameTaken
	case db.IsUniqueViolation(err, "users_email_key"):
		return errEmailTaken
	}
	return err
}

func (s *Service) Register(ctx context.Context, in RegisterInput) error {
	username, email := NormalizeUsername(in.Username), NormalizeEmail(in.Email)
	fields := FieldErrors{}
	if msg := ValidateUsername(username); msg != "" {
		fields["username"] = msg
	}
	domain, msg := ValidateEmail(email)
	if msg != "" {
		fields["email"] = msg
	}
	displayName, msg := ValidateDisplayName(in.DisplayName)
	if msg != "" {
		fields["displayName"] = msg
	}
	if msg := ValidatePassword(in.Password, username, email); msg != "" {
		fields["password"] = msg
	}
	if len(fields) > 0 {
		return fields
	}
	allowed, err := s.q.DomainAllowed(ctx, domain)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrEmailDomainNotAllowed
	}
	// O hash vem antes da checagem de e-mail existente para os dois caminhos levarem o mesmo tempo.
	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return err
	}
	if existing, err := s.q.GetUserByEmail(ctx, email); err == nil {
		s.sendLater(mail.AccountExists(existing.Email, existing.DisplayName, s.link("/entrar"), s.link("/esqueci-senha")))
		return nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	now := s.now()
	var token string
	err = db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		user, err := q.CreateUser(ctx, dbgen.CreateUserParams{Username: username, Email: email, DisplayName: displayName, PasswordHash: hash, Now: now})
		if err != nil {
			return mapUserInsertError(err)
		}
		token, err = issueToken(ctx, q, user.ID, purposeVerify, now, verifyTTL)
		return err
	})
	if errors.Is(err, errEmailTaken) {
		return nil // corrida com outro cadastro do mesmo e-mail: mesma resposta de sucesso
	}
	if err != nil {
		return err
	}
	s.sendLater(mail.VerifyEmail(email, displayName, s.link("/verificar-email?token="+token)))
	return nil
}

func (s *Service) VerifyEmail(ctx context.Context, token string) error {
	if token == "" {
		return ErrInvalidToken
	}
	now := s.now()
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		userID, err := q.ConsumeUserToken(ctx, dbgen.ConsumeUserTokenParams{Now: now, TokenHash: HashToken(token), Purpose: purposeVerify})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidToken
		}
		if err != nil {
			return err
		}
		return q.MarkEmailVerified(ctx, dbgen.MarkEmailVerifiedParams{Now: now, ID: userID})
	})
}

func (s *Service) ResendVerification(ctx context.Context, email string) error {
	user, err := s.q.GetUserByEmail(ctx, NormalizeEmail(email))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if user.EmailVerifiedAt != nil || user.DisabledAt != nil {
		return nil
	}
	return s.sendVerification(ctx, user)
}

func (s *Service) sendVerification(ctx context.Context, user dbgen.User) error {
	token, err := issueToken(ctx, s.q, user.ID, purposeVerify, s.now(), verifyTTL)
	if err != nil {
		return err
	}
	s.sendLater(mail.VerifyEmail(user.Email, user.DisplayName, s.link("/verificar-email?token="+token)))
	return nil
}

func (s *Service) Login(ctx context.Context, login, password string, meta SessionMeta) (LoginResult, error) {
	login = strings.ToLower(strings.TrimSpace(login))
	var row dbgen.User
	var err error
	if strings.Contains(login, "@") {
		row, err = s.q.GetUserByEmail(ctx, login)
	} else {
		row, err = s.q.GetUserByUsername(ctx, login)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		s.hasher.VerifyDummy(password)
		return LoginResult{}, ErrInvalidCredentials
	}
	if err != nil {
		return LoginResult{}, err
	}
	ok, needsRehash, err := s.hasher.Verify(password, row.PasswordHash)
	if err != nil {
		return LoginResult{}, err
	}
	if !ok {
		return LoginResult{}, ErrInvalidCredentials
	}
	switch {
	case row.DisabledAt != nil:
		return LoginResult{}, ErrAccountDisabled
	case row.EmailVerifiedAt == nil:
		return LoginResult{}, ErrEmailNotVerified
	case row.MustResetPassword:
		return LoginResult{}, ErrPasswordResetRequired
	}
	if needsRehash {
		s.rehash(ctx, row.ID, password)
	}
	token, err := s.createSession(ctx, s.q, row.ID, meta)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{Token: token, User: userFromRow(row)}, nil
}

// rehash é melhor esforço: falhar aqui não impede o login.
func (s *Service) rehash(ctx context.Context, userID, password string) {
	hash, err := s.hasher.Hash(password)
	if err == nil {
		err = s.q.SetPasswordHash(ctx, dbgen.SetPasswordHashParams{PasswordHash: hash, Now: s.now(), ID: userID})
	}
	if err != nil {
		s.log.Error("falha ao atualizar hash de senha", "user_id", userID, "err", err)
	}
}

func (s *Service) ForgotPassword(ctx context.Context, email string) error {
	user, err := s.q.GetUserByEmail(ctx, NormalizeEmail(email))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	switch {
	case user.DisabledAt != nil:
		return nil
	case user.EmailVerifiedAt == nil:
		return s.sendVerification(ctx, user)
	}
	return s.SendPasswordReset(ctx, user.ID, false)
}

// SendPasswordReset emite um link de reset; forced=true usa o texto do reset pedido pelo admin.
func (s *Service) SendPasswordReset(ctx context.Context, userID string, forced bool) error {
	user, err := s.q.GetUserByID(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrUserNotFound
	}
	if err != nil {
		return err
	}
	token, err := issueToken(ctx, s.q, user.ID, purposeReset, s.now(), resetTTL)
	if err != nil {
		return err
	}
	link := s.link("/redefinir-senha?token=" + token)
	if forced {
		s.sendLater(mail.ForcedPasswordReset(user.Email, user.DisplayName, link))
	} else {
		s.sendLater(mail.PasswordReset(user.Email, user.DisplayName, link))
	}
	return nil
}

func (s *Service) ResetPassword(ctx context.Context, token, password string) error {
	if token == "" {
		return ErrInvalidToken
	}
	now, hash := s.now(), HashToken(token)
	userID, err := s.q.GetValidUserToken(ctx, dbgen.GetValidUserTokenParams{TokenHash: hash, Purpose: purposeReset, Now: now})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidToken
	}
	if err != nil {
		return err
	}
	user, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	// Valida antes de consumir: um erro de digitação não queima o link.
	if msg := ValidatePassword(password, user.Username, user.Email); msg != "" {
		return FieldErrors{"password": msg}
	}
	passwordHash, err := s.hasher.Hash(password)
	if err != nil {
		return err
	}
	err = db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		if _, err := q.ConsumeUserToken(ctx, dbgen.ConsumeUserTokenParams{Now: now, TokenHash: hash, Purpose: purposeReset}); errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidToken
		} else if err != nil {
			return err
		}
		if err := q.CompletePasswordReset(ctx, dbgen.CompletePasswordResetParams{PasswordHash: passwordHash, Now: now, ID: user.ID}); err != nil {
			return err
		}
		return q.DeleteUserSessions(ctx, user.ID)
	})
	if err != nil {
		return err
	}
	s.sendLater(mail.PasswordChanged(user.Email, user.DisplayName))
	return nil
}

// ChangePassword troca a senha, derruba todas as sessões e devolve o token de uma sessão nova.
func (s *Service) ChangePassword(ctx context.Context, p Principal, current, next string, meta SessionMeta) (string, error) {
	user, err := s.q.GetUserByID(ctx, p.UserID)
	if err != nil {
		return "", err
	}
	ok, _, err := s.hasher.Verify(current, user.PasswordHash)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrInvalidCredentials
	}
	if msg := ValidatePassword(next, user.Username, user.Email); msg != "" {
		return "", FieldErrors{"newPassword": msg}
	}
	passwordHash, err := s.hasher.Hash(next)
	if err != nil {
		return "", err
	}
	var token string
	err = db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		if err := q.SetPasswordHash(ctx, dbgen.SetPasswordHashParams{PasswordHash: passwordHash, Now: s.now(), ID: user.ID}); err != nil {
			return err
		}
		if err := q.DeleteUserSessions(ctx, user.ID); err != nil {
			return err
		}
		token, err = s.createSession(ctx, q, user.ID, meta)
		return err
	})
	if err != nil {
		return "", err
	}
	s.sendLater(mail.PasswordChanged(user.Email, user.DisplayName))
	return token, nil
}

func (s *Service) UpdateDisplayName(ctx context.Context, userID, name string) (User, error) {
	name, msg := ValidateDisplayName(name)
	if msg != "" {
		return User{}, FieldErrors{"displayName": msg}
	}
	row, err := s.q.UpdateDisplayName(ctx, dbgen.UpdateDisplayNameParams{DisplayName: name, Now: s.now(), ID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, err
	}
	return userFromRow(row), nil
}
```

- [ ] **Step 4: Rodar e ver passar**

Run: `cd backend && go test ./internal/auth/ -race -v`
Expected: PASS em todos os testes do pacote.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/auth/flows.go backend/internal/auth/flows_test.go
git commit -m "feat(auth): adiciona cadastro com verificação, login e recuperação de senha"
```

---
### Tarefa 14: `match.Store` — persistência, finalização, recuperação e recálculo

**Files:**
- Create: `backend/internal/db/queries/matches.sql`, `backend/internal/db/queries/stats.sql`
- Regenerate: `backend/internal/db/dbgen/`
- Create: `backend/internal/match/store.go`, `backend/internal/match/recover.go`
- Test: `backend/internal/match/store_test.go`

**Interfaces:**
- Consumes: `replay` (Tarefa 2), `stats.Derive`/`stats.ErrNotFinished` (Tarefa 4), `db.WithTx`/`db.IsUniqueViolation`/`db.IsInvalidInput` (Tarefa 6), `enginetest.RandomRecord` e `testdb` nos testes.
- Produces (gerado): `CreateMatch(ctx, CreateMatchParams{Seed int64; FirstDealer int16; EngineVersion int32; Now time.Time}) (string, error)`, `AddMatchPlayer(ctx, AddMatchPlayerParams{MatchID string; Seat int16; UserID, DisplayName string}) error`, `AppendMatchAction(ctx, AppendMatchActionParams{MatchID string; Ply int32; Action []byte; Source string; Now time.Time}) (int64, error)`, `GetMatch(ctx, id string) (Match, error)`, `GetMatchForUpdate(ctx, id string) (Match, error)`, `ListMatchPlayers(ctx, matchID string) ([]ListMatchPlayersRow, error)` com `Row{Seat int16; UserID, DisplayName, Username string}`, `ListMatchActions(ctx, matchID string) ([]ListMatchActionsRow, error)` com `Row{Ply int32; Action []byte; Source string}`, `ListInProgressMatchIDs(ctx) ([]string, error)`, `ListFinishedMatchIDs(ctx) ([]string, error)`, `FinishMatch(ctx, FinishMatchParams{WinnerTeam, ScoreA, ScoreB int16; Now time.Time; ID string}) error`, `AbandonMatch(ctx, AbandonMatchParams{Now time.Time; Reason, ID string}) (int64, error)`, `InsertMatchPlayerStats(ctx, InsertMatchPlayerStatsParams{...}) error`, `AddPlayerStats(ctx, AddPlayerStatsParams{...}) error`, `DeleteAllMatchPlayerStats(ctx) error`, `DeleteAllPlayerStats(ctx) error`. Modelo `dbgen.Match{ID string; Seed int64; FirstDealer int16; EngineVersion int32; Status string; WinnerTeam, ScoreA, ScoreB *int16; CreatedAt time.Time; FinishedAt *time.Time; AbandonReason *string}`.
- Produces (pacote `match`):

```go
var ErrNotFound, ErrMatchClosed, ErrPlyConflict, ErrNotGameOver, ErrDuplicatePlayer error
func NewStore(pool *pgxpool.Pool, now func() time.Time) *Store
func (s *Store) Create(ctx, seed uint64, firstDealer engine.Seat, players [4]replay.PlayerSeat) (string, error)
func (s *Store) AppendAction(ctx, id string, ply int, a engine.Action, src replay.Source) error
func (s *Store) Load(ctx, id string) (replay.Record, error)
func (s *Store) Finish(ctx, id string) error
func (s *Store) Abandon(ctx, id, reason string) error
func (s *Store) Summary(ctx, id string) (Summary, error)
func AbandonWith(ctx, q *dbgen.Queries, id, reason string, now time.Time) error   // usado pelo admin na mesma transação
type Recovered struct{ Record replay.Record; State engine.GameState; NextPly int }
func (s *Store) Recover(ctx, log *slog.Logger) ([]Recovered, error)
func (s *Store) RecomputeStats(ctx) (skipped []string, err error)
type PlayerSummary struct{ Seat int; Username, DisplayName string }            // json: seat, username, displayName
type Summary struct{ ID string; Status replay.Status; Players []PlayerSummary; WinnerTeam *int; Score *[2]int;
	CreatedAt time.Time; FinishedAt *time.Time; AbandonReason *string }        // json camelCase
```

- [ ] **Step 1: Escrever as queries**

`backend/internal/db/queries/matches.sql`:

```sql
-- name: CreateMatch :one
INSERT INTO matches (seed, first_dealer, engine_version, created_at)
VALUES (@seed, @first_dealer, @engine_version, @now)
RETURNING id;

-- name: AddMatchPlayer :exec
INSERT INTO match_players (match_id, seat, user_id, display_name)
VALUES (@match_id, @seat, @user_id, @display_name);

-- name: AppendMatchAction :execrows
INSERT INTO match_actions (match_id, ply, action, source, created_at)
SELECT @match_id::uuid, @ply::integer, @action::jsonb, @source::text, @now::timestamptz
WHERE EXISTS (SELECT 1 FROM matches WHERE id = @match_id::uuid AND status = 'in_progress');

-- name: GetMatch :one
SELECT * FROM matches WHERE id = @id;

-- name: GetMatchForUpdate :one
SELECT * FROM matches WHERE id = @id FOR UPDATE;

-- name: ListMatchPlayers :many
SELECT mp.seat, mp.user_id, mp.display_name, u.username
FROM match_players mp
JOIN users u ON u.id = mp.user_id
WHERE mp.match_id = @match_id
ORDER BY mp.seat;

-- name: ListMatchActions :many
SELECT ply, action, source FROM match_actions WHERE match_id = @match_id ORDER BY ply;

-- name: ListInProgressMatchIDs :many
SELECT id FROM matches WHERE status = 'in_progress' ORDER BY created_at, id;

-- name: ListFinishedMatchIDs :many
SELECT id FROM matches WHERE status = 'finished' ORDER BY finished_at, id;

-- name: FinishMatch :exec
UPDATE matches
SET status = 'finished', winner_team = @winner_team::smallint, score_a = @score_a::smallint,
    score_b = @score_b::smallint, finished_at = @now::timestamptz
WHERE id = @id;

-- name: AbandonMatch :execrows
UPDATE matches
SET status = 'abandoned', finished_at = @now::timestamptz, abandon_reason = @reason::text
WHERE id = @id AND status = 'in_progress';
```

`backend/internal/db/queries/stats.sql`:

```sql
-- name: InsertMatchPlayerStats :exec
INSERT INTO match_player_stats (
    match_id, user_id, team, won, points_for, points_against, hands_played, hands_won,
    raises_called, raises_folded, raises_won_by_fold, eleven_hands, eleven_runs, eleven_won,
    iron_hands, iron_won, punished, timeouts
) VALUES (
    @match_id, @user_id, @team, @won, @points_for, @points_against, @hands_played, @hands_won,
    @raises_called, @raises_folded, @raises_won_by_fold, @eleven_hands, @eleven_runs, @eleven_won,
    @iron_hands, @iron_won, @punished, @timeouts
);

-- name: AddPlayerStats :exec
INSERT INTO player_stats AS p (
    user_id, matches_played, wins, losses, points_for, points_against, hands_played, hands_won,
    raises_called, raises_folded, raises_won_by_fold, eleven_hands, eleven_runs, eleven_won,
    iron_hands, iron_won, punished, timeouts, updated_at
) VALUES (
    @user_id, 1, @wins, @losses, @points_for, @points_against, @hands_played, @hands_won,
    @raises_called, @raises_folded, @raises_won_by_fold, @eleven_hands, @eleven_runs, @eleven_won,
    @iron_hands, @iron_won, @punished, @timeouts, @now
)
ON CONFLICT (user_id) DO UPDATE SET
    matches_played     = p.matches_played + 1,
    wins               = p.wins + EXCLUDED.wins,
    losses             = p.losses + EXCLUDED.losses,
    points_for         = p.points_for + EXCLUDED.points_for,
    points_against     = p.points_against + EXCLUDED.points_against,
    hands_played       = p.hands_played + EXCLUDED.hands_played,
    hands_won          = p.hands_won + EXCLUDED.hands_won,
    raises_called      = p.raises_called + EXCLUDED.raises_called,
    raises_folded      = p.raises_folded + EXCLUDED.raises_folded,
    raises_won_by_fold = p.raises_won_by_fold + EXCLUDED.raises_won_by_fold,
    eleven_hands       = p.eleven_hands + EXCLUDED.eleven_hands,
    eleven_runs        = p.eleven_runs + EXCLUDED.eleven_runs,
    eleven_won         = p.eleven_won + EXCLUDED.eleven_won,
    iron_hands         = p.iron_hands + EXCLUDED.iron_hands,
    iron_won           = p.iron_won + EXCLUDED.iron_won,
    punished           = p.punished + EXCLUDED.punished,
    timeouts           = p.timeouts + EXCLUDED.timeouts,
    updated_at         = EXCLUDED.updated_at;

-- name: DeleteAllMatchPlayerStats :exec
DELETE FROM match_player_stats;

-- name: DeleteAllPlayerStats :exec
DELETE FROM player_stats;
```

Run: `make gen-sql && cd backend && go build ./...`
Expected: gera `matches.sql.go` e `stats.sql.go` sem erros.

- [ ] **Step 2: Escrever os testes que falham**

`backend/internal/match/store_test.go`:

```go
package match_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/enginetest"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/match"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/stats"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/testclock"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/testdb"
)

func TestMain(m *testing.M) { testdb.Run(m) }

type env struct {
	store   *match.Store
	pool    *pgxpool.Pool
	clock   *testclock.Clock
	players [4]replay.PlayerSeat
}

func newEnv(t *testing.T) env {
	t.Helper()
	pool := testdb.New(t)
	clock := testclock.New(time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC))
	e := env{store: match.NewStore(pool, clock.Now), pool: pool, clock: clock}
	for i, name := range []string{"ana", "bia", "caio", "duda"} {
		e.players[i] = replay.PlayerSeat{UserID: testdb.CreateUser(t, pool, name), DisplayName: name}
	}
	return e
}

// persist grava os primeiros n passos (n < 0 = todos) de uma partida aleatória.
func (e env) persist(t *testing.T, seed uint64, n int) (string, replay.Record) {
	t.Helper()
	ctx := context.Background()
	rec, _ := enginetest.RandomRecord(seed)
	rec.Players = e.players
	if n >= 0 {
		rec.Actions = rec.Actions[:n]
	}
	id, err := e.store.Create(ctx, rec.Seed, rec.FirstDealer, e.players)
	if err != nil {
		t.Fatal(err)
	}
	for _, logged := range rec.Actions {
		if err := e.store.AppendAction(ctx, id, logged.Ply, logged.Action, logged.Source); err != nil {
			t.Fatalf("ply %d: %v", logged.Ply, err)
		}
	}
	rec.ID = id
	return id, rec
}

func TestCreateAndLoad(t *testing.T) {
	e := newEnv(t)
	id, want := e.persist(t, 1, 10)
	got, err := e.store.Load(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != id || got.Seed != want.Seed || got.FirstDealer != want.FirstDealer || got.EngineVersion != engine.RulesVersion ||
		got.Status != replay.StatusInProgress || got.Players != e.players || !reflect.DeepEqual(got.Actions, want.Actions) {
		t.Fatalf("Load divergente:\n got %+v\nwant %+v", got, want)
	}
}

func TestSeedUsesFullUint64Range(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	const seed = uint64(1<<64 - 2)
	id, err := e.store.Create(ctx, seed, 3, e.players)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := e.store.Load(ctx, id)
	if got.Seed != seed {
		t.Fatalf("seed = %d", got.Seed)
	}
}

func TestCreateRejectsDuplicatePlayer(t *testing.T) {
	e := newEnv(t)
	players := e.players
	players[3] = players[0]
	if _, err := e.store.Create(context.Background(), 1, 0, players); !errors.Is(err, match.ErrDuplicatePlayer) {
		t.Fatalf("err = %v", err)
	}
}

func TestAppendActionErrors(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	id, rec := e.persist(t, 1, 3)
	first := rec.Actions[0]
	if err := e.store.AppendAction(ctx, id, 0, first.Action, first.Source); !errors.Is(err, match.ErrPlyConflict) {
		t.Fatalf("ply repetido: %v", err)
	}
	for _, missing := range []string{"00000000-0000-7000-8000-000000000000", "nao-e-uuid"} {
		if err := e.store.AppendAction(ctx, missing, 0, first.Action, first.Source); !errors.Is(err, match.ErrNotFound) {
			t.Fatalf("partida %q: %v", missing, err)
		}
	}
	if err := e.store.AppendAction(ctx, id, 3, engine.Action{Kind: engine.ActSysTimeout, Seat: 1}, replay.SourceTimeout); err == nil {
		t.Fatal("sys_timeout não pode ser gravado")
	}
	if err := e.store.AppendAction(ctx, id, 3, first.Action, replay.Source("bot")); err == nil {
		t.Fatal("origem inválida aceita")
	}
	if err := e.store.Abandon(ctx, id, "teste"); err != nil {
		t.Fatal(err)
	}
	if err := e.store.AppendAction(ctx, id, 3, first.Action, first.Source); !errors.Is(err, match.ErrMatchClosed) {
		t.Fatalf("após abandono: %v", err)
	}
}

func TestConcurrentAppendOfSamePlyHasSingleWinner(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	id, err := e.store.Create(ctx, 1, 0, e.players)
	if err != nil {
		t.Fatal(err)
	}
	start := engine.Action{Kind: engine.ActSysStartHand}
	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = e.store.AppendAction(ctx, id, 0, start, replay.SourceSystem)
		}()
	}
	wg.Wait()
	ok, conflicts := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, match.ErrPlyConflict):
			conflicts++
		default:
			t.Fatalf("erro inesperado: %v", err)
		}
	}
	if ok != 1 || conflicts != 7 {
		t.Fatalf("ok=%d conflitos=%d", ok, conflicts)
	}
}

func TestFinishWritesResultAndStats(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	id, rec := e.persist(t, 5, -1)
	if err := e.store.Finish(ctx, id); err != nil {
		t.Fatal(err)
	}
	final, _ := replay.Rebuild(rec)
	summary, err := e.store.Summary(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Status != replay.StatusFinished || summary.WinnerTeam == nil || *summary.WinnerTeam != int(*final.Winner) ||
		summary.Score == nil || *summary.Score != final.Score || summary.FinishedAt == nil {
		t.Fatalf("summary = %+v, placar ao vivo %v", summary, final.Score)
	}
	derived, _ := stats.Derive(rec)
	for _, p := range derived {
		var played, wins, pointsFor, hands int
		err := e.pool.QueryRow(ctx, `SELECT matches_played, wins, points_for, hands_played FROM player_stats WHERE user_id = $1`, p.UserID).
			Scan(&played, &wins, &pointsFor, &hands)
		if err != nil {
			t.Fatal(err)
		}
		wantWins := 0
		if p.Won {
			wantWins = 1
		}
		if played != 1 || wins != wantWins || pointsFor != p.PointsFor || hands != p.HandsPlayed {
			t.Fatalf("player_stats de %s: %d %d %d %d", p.UserID, played, wins, pointsFor, hands)
		}
	}
	var rows int
	_ = e.pool.QueryRow(ctx, "SELECT count(*) FROM match_player_stats WHERE match_id = $1", id).Scan(&rows)
	if rows != 4 {
		t.Fatalf("%d linhas em match_player_stats", rows)
	}
	if err := e.store.Finish(ctx, id); !errors.Is(err, match.ErrMatchClosed) {
		t.Fatalf("Finish duplicado: %v", err)
	}
}

func TestFinishRejectsUnfinishedMatch(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	id, _ := e.persist(t, 5, 20)
	if err := e.store.Finish(ctx, id); !errors.Is(err, match.ErrNotGameOver) {
		t.Fatalf("err = %v", err)
	}
	summary, _ := e.store.Summary(ctx, id)
	if summary.Status != replay.StatusInProgress {
		t.Fatalf("status = %s", summary.Status)
	}
}

func TestPlayerStatsAccumulateAcrossMatches(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	for _, seed := range []uint64{11, 12} {
		id, _ := e.persist(t, seed, -1)
		if err := e.store.Finish(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range e.players {
		var played, wins, losses int
		_ = e.pool.QueryRow(ctx, `SELECT matches_played, wins, losses FROM player_stats WHERE user_id = $1`, p.UserID).Scan(&played, &wins, &losses)
		if played != 2 || wins+losses != 2 {
			t.Fatalf("%s: jogou %d, %d vitórias, %d derrotas", p.DisplayName, played, wins, losses)
		}
	}
}

func TestAbandon(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	id, _ := e.persist(t, 1, 5)
	if err := e.store.Abandon(ctx, id, "sala vazia"); err != nil {
		t.Fatal(err)
	}
	summary, _ := e.store.Summary(ctx, id)
	if summary.Status != replay.StatusAbandoned || summary.AbandonReason == nil || *summary.AbandonReason != "sala vazia" || summary.FinishedAt == nil {
		t.Fatalf("summary = %+v", summary)
	}
	if err := e.store.Abandon(ctx, id, "de novo"); !errors.Is(err, match.ErrMatchClosed) {
		t.Fatalf("abandono duplicado: %v", err)
	}
	if err := e.store.Finish(ctx, id); !errors.Is(err, match.ErrMatchClosed) {
		t.Fatalf("Finish de abandonada: %v", err)
	}
	if err := e.store.Abandon(ctx, "00000000-0000-7000-8000-000000000000", "x"); !errors.Is(err, match.ErrNotFound) {
		t.Fatalf("inexistente: %v", err)
	}
}

func TestSummaryListsPlayers(t *testing.T) {
	e := newEnv(t)
	id, _ := e.persist(t, 1, 0)
	summary, err := e.store.Summary(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.Players) != 4 || summary.Players[2].Username != "caio" || summary.Players[2].Seat != 2 || summary.WinnerTeam != nil {
		t.Fatalf("summary = %+v", summary)
	}
	if _, err := e.store.Summary(context.Background(), "nao-e-uuid"); !errors.Is(err, match.ErrNotFound) {
		t.Fatalf("id inválido: %v", err)
	}
}

func TestRecover(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	partialID, partial := e.persist(t, 21, 30)

	corruptID, _ := e.persist(t, 22, 1)
	// ply 1 ilegal: ninguém pode jogar carta durante o corte.
	if err := e.store.AppendAction(ctx, corruptID, 1, engine.Action{Kind: engine.ActPlayCard, Seat: 0}, replay.SourcePlayer); err != nil {
		t.Fatal(err)
	}

	doneID, _ := e.persist(t, 23, -1) // terminou, mas o servidor caiu antes do Finish

	oldID, _ := e.persist(t, 24, 3)
	if _, err := e.pool.Exec(ctx, "UPDATE matches SET engine_version = 999 WHERE id = $1", oldID); err != nil {
		t.Fatal(err)
	}

	recovered, err := e.store.Recover(ctx, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if len(recovered) != 1 || recovered[0].Record.ID != partialID || recovered[0].NextPly != 30 {
		t.Fatalf("recuperadas = %+v", recovered)
	}
	want, _ := replay.Rebuild(partial)
	if !reflect.DeepEqual(recovered[0].State, want) {
		t.Fatal("estado recuperado diverge do log")
	}
	status := func(id string) (string, *string) {
		var s string
		var reason *string
		_ = e.pool.QueryRow(ctx, "SELECT status, abandon_reason FROM matches WHERE id = $1", id).Scan(&s, &reason)
		return s, reason
	}
	if s, r := status(corruptID); s != "abandoned" || r == nil || *r != "corrupt_log" {
		t.Fatalf("corrompida: %s %v", s, r)
	}
	if s, _ := status(doneID); s != "finished" {
		t.Fatalf("terminada sem Finish: %s", s)
	}
	if s, r := status(oldID); s != "abandoned" || r == nil || *r != "incompatible_version" {
		t.Fatalf("versão antiga: %s %v", s, r)
	}
}

func TestRecomputeStatsRestoresAggregates(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	for _, seed := range []uint64{31, 32} {
		id, _ := e.persist(t, seed, -1)
		if err := e.store.Finish(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	snapshot := func() map[string][3]int {
		out := map[string][3]int{}
		rows, _ := e.pool.Query(ctx, "SELECT user_id::text, matches_played, wins, points_for FROM player_stats")
		defer rows.Close()
		for rows.Next() {
			var id string
			var v [3]int
			_ = rows.Scan(&id, &v[0], &v[1], &v[2])
			out[id] = v
		}
		return out
	}
	before := snapshot()
	_, _ = e.pool.Exec(ctx, "UPDATE player_stats SET wins = 999")
	skipped, err := e.store.RecomputeStats(ctx)
	if err != nil || len(skipped) != 0 {
		t.Fatalf("skipped=%v err=%v", skipped, err)
	}
	if after := snapshot(); !reflect.DeepEqual(before, after) {
		t.Fatalf("recálculo divergente:\nantes %v\ndepois %v", before, after)
	}
}
```

- [ ] **Step 3: Rodar e ver falhar**

Run: `cd backend && go test ./internal/match/`
Expected: FAIL de compilação.

- [ ] **Step 4: Implementar `store.go`**

```go
// Package match persiste partidas como seed + log de ações e deriva resultados do log.
package match

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/db"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/db/dbgen"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/stats"
)

var (
	ErrNotFound        = errors.New("match: partida não encontrada")
	ErrMatchClosed     = errors.New("match: partida não está em andamento")
	ErrPlyConflict     = errors.New("match: ply já gravado")
	ErrNotGameOver     = errors.New("match: partida ainda não terminou")
	ErrDuplicatePlayer = errors.New("match: jogador repetido na partida")
)

type Store struct {
	pool *pgxpool.Pool
	q    *dbgen.Queries
	now  func() time.Time
}

func NewStore(pool *pgxpool.Pool, now func() time.Time) *Store {
	if now == nil {
		now = time.Now
	}
	return &Store{pool: pool, q: dbgen.New(pool), now: now}
}

// notFound converte "sem linhas" e UUID malformado em ErrNotFound.
func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) || db.IsInvalidInput(err) {
		return ErrNotFound
	}
	return err
}

func (s *Store) Create(ctx context.Context, seed uint64, firstDealer engine.Seat, players [4]replay.PlayerSeat) (string, error) {
	if firstDealer > 3 {
		return "", fmt.Errorf("match: dealer inicial inválido: %d", firstDealer)
	}
	var id string
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		var err error
		id, err = q.CreateMatch(ctx, dbgen.CreateMatchParams{Seed: int64(seed), FirstDealer: int16(firstDealer),
			EngineVersion: int32(engine.RulesVersion), Now: s.now()})
		if err != nil {
			return err
		}
		for seat, p := range players {
			err := q.AddMatchPlayer(ctx, dbgen.AddMatchPlayerParams{MatchID: id, Seat: int16(seat), UserID: p.UserID, DisplayName: p.DisplayName})
			if db.IsUniqueViolation(err, "match_players_match_id_user_id_key") {
				return ErrDuplicatePlayer
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
	return id, err
}

// AppendAction grava uma ação aplicada com sucesso. A sala chama antes de enviar o estado aos jogadores.
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
	n, err := s.q.AppendMatchAction(ctx, dbgen.AppendMatchActionParams{MatchID: id, Ply: int32(ply), Action: raw, Source: string(src), Now: s.now()})
	if db.IsUniqueViolation(err, "match_actions_pkey") {
		return ErrPlyConflict
	}
	if err != nil {
		return notFound(err)
	}
	if n == 0 {
		return s.closedOrMissing(ctx, s.q, id)
	}
	return nil
}

func (s *Store) closedOrMissing(ctx context.Context, q *dbgen.Queries, id string) error {
	if _, err := q.GetMatch(ctx, id); err != nil {
		return notFound(err)
	}
	return ErrMatchClosed
}

func (s *Store) Load(ctx context.Context, id string) (replay.Record, error) {
	m, err := s.q.GetMatch(ctx, id)
	if err != nil {
		return replay.Record{}, notFound(err)
	}
	return loadRecord(ctx, s.q, m)
}

func loadRecord(ctx context.Context, q *dbgen.Queries, m dbgen.Match) (replay.Record, error) {
	rec := replay.Record{ID: m.ID, Seed: uint64(m.Seed), FirstDealer: engine.Seat(m.FirstDealer),
		EngineVersion: int(m.EngineVersion), Status: replay.Status(m.Status)}
	players, err := q.ListMatchPlayers(ctx, m.ID)
	if err != nil {
		return rec, err
	}
	if len(players) != 4 {
		return rec, &replay.CorruptLogError{Ply: -1, Err: fmt.Errorf("partida com %d jogadores", len(players))}
	}
	for _, p := range players {
		rec.Players[p.Seat] = replay.PlayerSeat{UserID: p.UserID, DisplayName: p.DisplayName}
	}
	actions, err := q.ListMatchActions(ctx, m.ID)
	if err != nil {
		return rec, err
	}
	for _, row := range actions {
		var a engine.Action
		if err := json.Unmarshal(row.Action, &a); err != nil {
			return rec, &replay.CorruptLogError{Ply: int(row.Ply), Err: err}
		}
		rec.Actions = append(rec.Actions, replay.LoggedAction{Ply: int(row.Ply), Action: a, Source: replay.Source(row.Source)})
	}
	return rec, nil
}

// Finish encerra a partida a partir do próprio log: reconstrói, confere o fim e grava estatísticas.
func (s *Store) Finish(ctx context.Context, id string) error {
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		m, err := q.GetMatchForUpdate(ctx, id)
		if err != nil {
			return notFound(err)
		}
		if m.Status != string(replay.StatusInProgress) {
			return ErrMatchClosed
		}
		rec, err := loadRecord(ctx, q, m)
		if err != nil {
			return err
		}
		perSeat, err := stats.Derive(rec)
		if errors.Is(err, stats.ErrNotFinished) {
			return ErrNotGameOver
		}
		if err != nil {
			return err
		}
		winner := perSeat[0].Team // assento 0 é sempre da dupla 0
		if !perSeat[0].Won {
			winner = engine.Other(winner)
		}
		now := s.now()
		err = q.FinishMatch(ctx, dbgen.FinishMatchParams{WinnerTeam: int16(winner), ScoreA: int16(perSeat[0].PointsFor),
			ScoreB: int16(perSeat[0].PointsAgainst), Now: now, ID: id})
		if err != nil {
			return err
		}
		return writeStats(ctx, q, id, perSeat, now)
	})
}

func writeStats(ctx context.Context, q *dbgen.Queries, matchID string, perSeat [4]stats.PlayerMatchStats, now time.Time) error {
	for _, p := range perSeat {
		err := q.InsertMatchPlayerStats(ctx, dbgen.InsertMatchPlayerStatsParams{
			MatchID: matchID, UserID: p.UserID, Team: int16(p.Team), Won: p.Won,
			PointsFor: int32(p.PointsFor), PointsAgainst: int32(p.PointsAgainst),
			HandsPlayed: int32(p.HandsPlayed), HandsWon: int32(p.HandsWon),
			RaisesCalled: int32(p.RaisesCalled), RaisesFolded: int32(p.RaisesFolded), RaisesWonByFold: int32(p.RaisesWonByFold),
			ElevenHands: int32(p.ElevenHands), ElevenRuns: int32(p.ElevenRuns), ElevenWon: int32(p.ElevenWon),
			IronHands: int32(p.IronHands), IronWon: int32(p.IronWon), Punished: int32(p.Punished), Timeouts: int32(p.Timeouts),
		})
		if err != nil {
			return err
		}
		var wins int32
		if p.Won {
			wins = 1
		}
		err = q.AddPlayerStats(ctx, dbgen.AddPlayerStatsParams{
			UserID: p.UserID, Wins: wins, Losses: 1 - wins,
			PointsFor: int32(p.PointsFor), PointsAgainst: int32(p.PointsAgainst),
			HandsPlayed: int32(p.HandsPlayed), HandsWon: int32(p.HandsWon),
			RaisesCalled: int32(p.RaisesCalled), RaisesFolded: int32(p.RaisesFolded), RaisesWonByFold: int32(p.RaisesWonByFold),
			ElevenHands: int32(p.ElevenHands), ElevenRuns: int32(p.ElevenRuns), ElevenWon: int32(p.ElevenWon),
			IronHands: int32(p.IronHands), IronWon: int32(p.IronWon), Punished: int32(p.Punished), Timeouts: int32(p.Timeouts),
			Now: now,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Abandon(ctx context.Context, id, reason string) error {
	return AbandonWith(ctx, s.q, id, reason, s.now())
}

// AbandonWith marca a partida como abandonada usando q (que pode estar numa transação).
func AbandonWith(ctx context.Context, q *dbgen.Queries, id, reason string, now time.Time) error {
	n, err := q.AbandonMatch(ctx, dbgen.AbandonMatchParams{Now: now, Reason: reason, ID: id})
	if err != nil {
		return notFound(err)
	}
	if n == 0 {
		if _, err := q.GetMatch(ctx, id); err != nil {
			return notFound(err)
		}
		return ErrMatchClosed
	}
	return nil
}

type PlayerSummary struct {
	Seat        int    `json:"seat"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

type Summary struct {
	ID            string          `json:"id"`
	Status        replay.Status   `json:"status"`
	Players       []PlayerSummary `json:"players"`
	WinnerTeam    *int            `json:"winnerTeam"`
	Score         *[2]int         `json:"score"`
	CreatedAt     time.Time       `json:"createdAt"`
	FinishedAt    *time.Time      `json:"finishedAt"`
	AbandonReason *string         `json:"abandonReason,omitempty"`
}

func (s *Store) Summary(ctx context.Context, id string) (Summary, error) {
	m, err := s.q.GetMatch(ctx, id)
	if err != nil {
		return Summary{}, notFound(err)
	}
	players, err := s.q.ListMatchPlayers(ctx, id)
	if err != nil {
		return Summary{}, err
	}
	out := Summary{ID: m.ID, Status: replay.Status(m.Status), CreatedAt: m.CreatedAt, FinishedAt: m.FinishedAt, AbandonReason: m.AbandonReason}
	for _, p := range players {
		out.Players = append(out.Players, PlayerSummary{Seat: int(p.Seat), Username: p.Username, DisplayName: p.DisplayName})
	}
	if m.WinnerTeam != nil {
		winner := int(*m.WinnerTeam)
		out.WinnerTeam = &winner
	}
	if m.ScoreA != nil && m.ScoreB != nil {
		out.Score = &[2]int{int(*m.ScoreA), int(*m.ScoreB)}
	}
	return out, nil
}
```

- [ ] **Step 5: Implementar `recover.go`**

```go
package match

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/db"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/stats"
)

// Recovered é uma partida em andamento reconstruída para a sala (Spec 3) retomar.
type Recovered struct {
	Record  replay.Record
	State   engine.GameState
	NextPly int
}

// Recover reconstrói as partidas em andamento após um restart. Partidas irrecuperáveis
// são abandonadas e as que já tinham terminado são finalizadas; o servidor sobe mesmo assim.
func (s *Store) Recover(ctx context.Context, log *slog.Logger) ([]Recovered, error) {
	ids, err := s.q.ListInProgressMatchIDs(ctx)
	if err != nil {
		return nil, err
	}
	var out []Recovered
	for _, id := range ids {
		rec, err := s.Load(ctx, id)
		var state engine.GameState
		if err == nil {
			state, err = replay.Rebuild(rec)
		}
		var corrupt *replay.CorruptLogError
		switch {
		case errors.Is(err, replay.ErrIncompatibleVersion):
			if err := s.abandonUnrecoverable(ctx, log, id, "incompatible_version", err); err != nil {
				return nil, err
			}
			continue
		case errors.As(err, &corrupt):
			if err := s.abandonUnrecoverable(ctx, log, id, "corrupt_log", err); err != nil {
				return nil, err
			}
			continue
		case err != nil:
			return nil, err
		}
		if state.Phase == engine.PhaseGameOver {
			if err := s.Finish(ctx, id); err != nil {
				return nil, err
			}
			continue
		}
		out = append(out, Recovered{Record: rec, State: state, NextPly: len(rec.Actions)})
	}
	return out, nil
}

func (s *Store) abandonUnrecoverable(ctx context.Context, log *slog.Logger, id, reason string, cause error) error {
	log.ErrorContext(ctx, "partida irrecuperável abandonada", "match_id", id, "reason", reason, "err", cause)
	return s.Abandon(ctx, id, reason)
}

// RecomputeStats refaz match_player_stats e player_stats a partir dos logs de todas as partidas finalizadas.
func (s *Store) RecomputeStats(ctx context.Context) (skipped []string, err error) {
	err = db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		skipped = nil
		q := s.q.WithTx(tx)
		if err := q.DeleteAllMatchPlayerStats(ctx); err != nil {
			return err
		}
		if err := q.DeleteAllPlayerStats(ctx); err != nil {
			return err
		}
		ids, err := q.ListFinishedMatchIDs(ctx)
		if err != nil {
			return err
		}
		now := s.now()
		for _, id := range ids {
			m, err := q.GetMatch(ctx, id)
			if err != nil {
				return err
			}
			rec, err := loadRecord(ctx, q, m)
			if err != nil {
				skipped = append(skipped, id)
				continue
			}
			perSeat, err := stats.Derive(rec)
			if err != nil {
				skipped = append(skipped, id)
				continue
			}
			if err := writeStats(ctx, q, id, perSeat, now); err != nil {
				return err
			}
		}
		return nil
	})
	return skipped, err
}
```

- [ ] **Step 6: Rodar e ver passar**

Run: `cd backend && go test ./internal/match/ ./internal/db/ -race -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/db/queries backend/internal/db/dbgen backend/internal/match
git commit -m "feat(match): persiste partidas por log de ações com finalização e recuperação"
```

---

### Tarefa 15: Paginação por cursor e `stats.Reader` (ranking, perfil, histórico)

**Files:**
- Create: `backend/internal/pagination/cursor.go`
- Test: `backend/internal/pagination/cursor_test.go`
- Create: `backend/internal/db/queries/ranking.sql`; regenerate `dbgen`
- Create: `backend/internal/stats/reader.go`
- Test: `backend/internal/stats/reader_test.go` (pacote `stats_test`)

**Interfaces:**
- Produces:
  - `func pagination.Encode(t time.Time, id string) string`; `func pagination.Decode(cursor string) (at *time.Time, id *string, err error)` — cursor vazio devolve `nil, nil, nil`; inválido devolve `pagination.ErrInvalidCursor`.
  - (gerado) `Ranking(ctx, RankingParams{MinMatches, Lim int32}) ([]RankingRow, error)` com `Row{Username, DisplayName string; MatchesPlayed, Wins, Losses int32}`; `GetPlayerStats(ctx, userID string) (PlayerStat, error)`; `ListUserMatches(ctx, ListUserMatchesParams{UserID string; CursorFinishedAt *time.Time; CursorID *string; Lim int32}) ([]ListUserMatchesRow, error)` com `Row{ID, Status string; WinnerTeam, ScoreA, ScoreB *int16; CreatedAt time.Time; FinishedAt *time.Time; Seat int16}`.
  - Pacote `stats`:

```go
var ErrUserNotFound error
func NewReader(pool *pgxpool.Pool, minMatches int) *Reader
type RankingEntry struct{ Position int; Username, DisplayName string; MatchesPlayed, Wins, Losses int; WinRate float64 }
type Totals struct{ MatchesPlayed, Wins, Losses, PointsFor, PointsAgainst, HandsPlayed, HandsWon, RaisesCalled, RaisesFolded,
	RaisesWonByFold, ElevenHands, ElevenRuns, ElevenWon, IronHands, IronWon, Punished, Timeouts int; WinRate float64 }
type Profile struct{ Username, DisplayName string; CreatedAt time.Time; Stats Totals }
type HistoryItem struct{ ID string; Status replay.Status; Team int; Won bool; WinnerTeam *int; Score *[2]int; CreatedAt time.Time; FinishedAt *time.Time }
type HistoryPage struct{ Items []HistoryItem; NextCursor string }
func (r *Reader) Ranking(ctx, limit int) ([]RankingEntry, error)            // limit <= 0 → 50; máximo 100
func (r *Reader) Profile(ctx, username string) (Profile, error)
func (r *Reader) History(ctx, username, cursor string, limit int) (HistoryPage, error) // limit <= 0 → 20; máximo 50
```
(Todos com tags JSON camelCase.)

- [ ] **Step 1: Cursor — testes e implementação**

`backend/internal/pagination/cursor_test.go`:

```go
package pagination

import (
	"errors"
	"testing"
	"time"
)

func TestCursorRoundTrip(t *testing.T) {
	at := time.Date(2026, 10, 14, 12, 0, 0, 123456000, time.FixedZone("BRT", -3*3600))
	gotAt, gotID, err := Decode(Encode(at, "0199e7a4-0000-7000-8000-000000000001"))
	if err != nil || !gotAt.Equal(at) || *gotID != "0199e7a4-0000-7000-8000-000000000001" {
		t.Fatalf("at=%v id=%v err=%v", gotAt, gotID, err)
	}
}

func TestDecodeEmptyAndInvalid(t *testing.T) {
	if at, id, err := Decode(""); at != nil || id != nil || err != nil {
		t.Fatal("cursor vazio deve ser a primeira página")
	}
	for _, bad := range []string{"%%%", "c2VtLXNlcGFyYWRvcg", "bm8tdGltZXxhYmM"} {
		if _, _, err := Decode(bad); !errors.Is(err, ErrInvalidCursor) {
			t.Errorf("%q: err = %v", bad, err)
		}
	}
}
```

`backend/internal/pagination/cursor.go`:

```go
// Package pagination codifica cursores opacos (instante + id) para paginação por keyset.
package pagination

import (
	"encoding/base64"
	"errors"
	"strings"
	"time"
)

var ErrInvalidCursor = errors.New("pagination: cursor inválido")

func Encode(t time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(t.UTC().Format(time.RFC3339Nano) + "|" + id))
}

func Decode(cursor string) (*time.Time, *string, error) {
	if cursor == "" {
		return nil, nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, nil, ErrInvalidCursor
	}
	stamp, id, ok := strings.Cut(string(raw), "|")
	if !ok || id == "" {
		return nil, nil, ErrInvalidCursor
	}
	at, err := time.Parse(time.RFC3339Nano, stamp)
	if err != nil {
		return nil, nil, ErrInvalidCursor
	}
	return &at, &id, nil
}
```

Run: `cd backend && go test ./internal/pagination/ -v`
Expected: PASS.

- [ ] **Step 2: Queries de leitura**

`backend/internal/db/queries/ranking.sql`:

```sql
-- name: Ranking :many
SELECT u.username, u.display_name, p.matches_played, p.wins, p.losses
FROM player_stats p
JOIN users u ON u.id = p.user_id
WHERE p.matches_played >= @min_matches::integer AND u.disabled_at IS NULL
ORDER BY p.wins DESC, (p.wins::float8 / p.matches_played) DESC, p.matches_played DESC, u.username
LIMIT @lim::integer;

-- name: GetPlayerStats :one
SELECT * FROM player_stats WHERE user_id = @user_id;

-- name: ListUserMatches :many
SELECT m.id, m.status, m.winner_team, m.score_a, m.score_b, m.created_at, m.finished_at, mp.seat
FROM match_players mp
JOIN matches m ON m.id = mp.match_id
WHERE mp.user_id = @user_id
  AND m.status <> 'in_progress'
  AND (sqlc.narg(cursor_finished_at)::timestamptz IS NULL
       OR (m.finished_at, m.id) < (sqlc.narg(cursor_finished_at)::timestamptz, sqlc.narg(cursor_id)::uuid))
ORDER BY m.finished_at DESC, m.id DESC
LIMIT @lim::integer;
```

Run: `make gen-sql && cd backend && go build ./...`
Expected: sem erros.

- [ ] **Step 3: Escrever os testes do Reader que falham**

`backend/internal/stats/reader_test.go`:

```go
package stats_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/enginetest"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/match"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/stats"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/testclock"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/testdb"
)

func TestMain(m *testing.M) { testdb.Run(m) }

type world struct {
	pool    *pgxpool.Pool
	store   *match.Store
	clock   *testclock.Clock
	players [4]replay.PlayerSeat
}

func newWorld(t *testing.T) world {
	t.Helper()
	pool := testdb.New(t)
	clock := testclock.New(time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC))
	w := world{pool: pool, store: match.NewStore(pool, clock.Now), clock: clock}
	for i, name := range []string{"ana", "bia", "caio", "duda"} {
		w.players[i] = replay.PlayerSeat{UserID: testdb.CreateUser(t, pool, name), DisplayName: name}
	}
	return w
}

// play grava e finaliza uma partida completa; complete=false deixa em andamento.
func (w world) play(t *testing.T, seed uint64, complete bool) string {
	t.Helper()
	ctx := context.Background()
	rec, _ := enginetest.RandomRecord(seed)
	id, err := w.store.Create(ctx, rec.Seed, rec.FirstDealer, w.players)
	if err != nil {
		t.Fatal(err)
	}
	actions := rec.Actions
	if !complete {
		actions = actions[:5]
	}
	for _, a := range actions {
		if err := w.store.AppendAction(ctx, id, a.Ply, a.Action, a.Source); err != nil {
			t.Fatal(err)
		}
	}
	w.clock.Advance(time.Minute)
	if complete {
		if err := w.store.Finish(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	return id
}

func TestRankingOrderAndMinimum(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	w.play(t, 1, true)
	w.play(t, 2, true)
	entries, err := stats.NewReader(w.pool, 1).Ranking(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 {
		t.Fatalf("%d entradas", len(entries))
	}
	for i, e := range entries {
		if e.Position != i+1 || e.MatchesPlayed != 2 || e.Wins+e.Losses != 2 || e.WinRate != float64(e.Wins)/2 {
			t.Fatalf("entrada %d = %+v", i, e)
		}
		if i > 0 {
			prev := entries[i-1]
			if prev.Wins < e.Wins || (prev.Wins == e.Wins && prev.Username > e.Username) {
				t.Fatalf("ordem errada: %+v antes de %+v", prev, e)
			}
		}
	}
	if entries, _ := stats.NewReader(w.pool, 3).Ranking(ctx, 0); len(entries) != 0 {
		t.Fatalf("mínimo de partidas ignorado: %+v", entries)
	}
	_, _ = w.pool.Exec(ctx, "UPDATE users SET disabled_at = now() WHERE username = 'ana'")
	if entries, _ := stats.NewReader(w.pool, 1).Ranking(ctx, 0); len(entries) != 3 {
		t.Fatal("conta desativada não aparece no ranking")
	}
}

func TestProfile(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	reader := stats.NewReader(w.pool, 1)
	p, err := reader.Profile(ctx, "ana")
	if err != nil || p.Username != "ana" || p.Stats.MatchesPlayed != 0 || p.Stats.WinRate != 0 {
		t.Fatalf("perfil sem partidas: %+v %v", p, err)
	}
	w.play(t, 3, true)
	p, _ = reader.Profile(ctx, "ANA")
	if p.Stats.MatchesPlayed != 1 || p.Stats.HandsPlayed == 0 {
		t.Fatalf("perfil após partida: %+v", p)
	}
	if _, err := reader.Profile(ctx, "ninguem"); !errors.Is(err, stats.ErrUserNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestHistoryPagination(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	first := w.play(t, 4, true)
	w.play(t, 5, true)
	last := w.play(t, 6, true)
	w.play(t, 7, false) // em andamento: fora do histórico
	reader := stats.NewReader(w.pool, 1)
	page, err := reader.History(ctx, "bia", "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 || page.Items[0].ID != last || page.NextCursor == "" {
		t.Fatalf("página 1 = %+v", page)
	}
	if page.Items[0].Team != 1 {
		t.Fatalf("bia senta no assento 1 (dupla 1): %+v", page.Items[0])
	}
	summary, _ := w.store.Summary(ctx, last)
	if page.Items[0].Won != (*summary.WinnerTeam == 1) || *page.Items[0].Score != *summary.Score {
		t.Fatalf("item %+v vs resumo %+v", page.Items[0], summary)
	}
	page2, err := reader.History(ctx, "bia", page.NextCursor, 2)
	if err != nil || len(page2.Items) != 1 || page2.Items[0].ID != first || page2.NextCursor != "" {
		t.Fatalf("página 2 = %+v err %v", page2, err)
	}
	if _, err := reader.History(ctx, "bia", "lixo!", 2); err == nil {
		t.Fatal("cursor inválido aceito")
	}
	if _, err := reader.History(ctx, "ninguem", "", 2); !errors.Is(err, stats.ErrUserNotFound) {
		t.Fatalf("err = %v", err)
	}
}
```

- [ ] **Step 4: Rodar e ver falhar**

Run: `cd backend && go test ./internal/stats/ -run 'Ranking|Profile|History'`
Expected: FAIL de compilação.

- [ ] **Step 5: Implementar `reader.go`**

```go
package stats

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/db/dbgen"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/pagination"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
)

var ErrUserNotFound = errors.New("stats: usuário não encontrado")

type Reader struct {
	q          *dbgen.Queries
	minMatches int
}

func NewReader(pool *pgxpool.Pool, minMatches int) *Reader {
	return &Reader{q: dbgen.New(pool), minMatches: minMatches}
}

type RankingEntry struct {
	Position      int     `json:"position"`
	Username      string  `json:"username"`
	DisplayName   string  `json:"displayName"`
	MatchesPlayed int     `json:"matchesPlayed"`
	Wins          int     `json:"wins"`
	Losses        int     `json:"losses"`
	WinRate       float64 `json:"winRate"`
}

type Totals struct {
	MatchesPlayed   int     `json:"matchesPlayed"`
	Wins            int     `json:"wins"`
	Losses          int     `json:"losses"`
	WinRate         float64 `json:"winRate"`
	PointsFor       int     `json:"pointsFor"`
	PointsAgainst   int     `json:"pointsAgainst"`
	HandsPlayed     int     `json:"handsPlayed"`
	HandsWon        int     `json:"handsWon"`
	RaisesCalled    int     `json:"raisesCalled"`
	RaisesFolded    int     `json:"raisesFolded"`
	RaisesWonByFold int     `json:"raisesWonByFold"`
	ElevenHands     int     `json:"elevenHands"`
	ElevenRuns      int     `json:"elevenRuns"`
	ElevenWon       int     `json:"elevenWon"`
	IronHands       int     `json:"ironHands"`
	IronWon         int     `json:"ironWon"`
	Punished        int     `json:"punished"`
	Timeouts        int     `json:"timeouts"`
}

type Profile struct {
	Username    string    `json:"username"`
	DisplayName string    `json:"displayName"`
	CreatedAt   time.Time `json:"createdAt"`
	Stats       Totals    `json:"stats"`
}

type HistoryItem struct {
	ID         string        `json:"id"`
	Status     replay.Status `json:"status"`
	Team       int           `json:"team"`
	Won        bool          `json:"won"`
	WinnerTeam *int          `json:"winnerTeam"`
	Score      *[2]int       `json:"score"`
	CreatedAt  time.Time     `json:"createdAt"`
	FinishedAt *time.Time    `json:"finishedAt"`
}

type HistoryPage struct {
	Items      []HistoryItem `json:"items"`
	NextCursor string        `json:"nextCursor,omitempty"`
}

func clamp(v, fallback, maximum int) int {
	if v <= 0 {
		return fallback
	}
	return min(v, maximum)
}

func winRate(wins, played int) float64 {
	if played == 0 {
		return 0
	}
	return float64(wins) / float64(played)
}

func (r *Reader) Ranking(ctx context.Context, limit int) ([]RankingEntry, error) {
	rows, err := r.q.Ranking(ctx, dbgen.RankingParams{MinMatches: int32(r.minMatches), Lim: int32(clamp(limit, 50, 100))})
	if err != nil {
		return nil, err
	}
	out := make([]RankingEntry, len(rows))
	for i, row := range rows {
		out[i] = RankingEntry{Position: i + 1, Username: row.Username, DisplayName: row.DisplayName,
			MatchesPlayed: int(row.MatchesPlayed), Wins: int(row.Wins), Losses: int(row.Losses),
			WinRate: winRate(int(row.Wins), int(row.MatchesPlayed))}
	}
	return out, nil
}

func (r *Reader) user(ctx context.Context, username string) (dbgen.User, error) {
	u, err := r.q.GetUserByUsername(ctx, strings.ToLower(strings.TrimSpace(username)))
	if errors.Is(err, pgx.ErrNoRows) {
		return dbgen.User{}, ErrUserNotFound
	}
	return u, err
}

func (r *Reader) Profile(ctx context.Context, username string) (Profile, error) {
	u, err := r.user(ctx, username)
	if err != nil {
		return Profile{}, err
	}
	out := Profile{Username: u.Username, DisplayName: u.DisplayName, CreatedAt: u.CreatedAt}
	p, err := r.q.GetPlayerStats(ctx, u.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return Profile{}, err
	}
	out.Stats = Totals{
		MatchesPlayed: int(p.MatchesPlayed), Wins: int(p.Wins), Losses: int(p.Losses),
		WinRate:   winRate(int(p.Wins), int(p.MatchesPlayed)),
		PointsFor: int(p.PointsFor), PointsAgainst: int(p.PointsAgainst),
		HandsPlayed: int(p.HandsPlayed), HandsWon: int(p.HandsWon),
		RaisesCalled: int(p.RaisesCalled), RaisesFolded: int(p.RaisesFolded), RaisesWonByFold: int(p.RaisesWonByFold),
		ElevenHands: int(p.ElevenHands), ElevenRuns: int(p.ElevenRuns), ElevenWon: int(p.ElevenWon),
		IronHands: int(p.IronHands), IronWon: int(p.IronWon), Punished: int(p.Punished), Timeouts: int(p.Timeouts),
	}
	return out, nil
}

func (r *Reader) History(ctx context.Context, username, cursor string, limit int) (HistoryPage, error) {
	at, id, err := pagination.Decode(cursor)
	if err != nil {
		return HistoryPage{}, err
	}
	u, err := r.user(ctx, username)
	if err != nil {
		return HistoryPage{}, err
	}
	limit = clamp(limit, 20, 50)
	rows, err := r.q.ListUserMatches(ctx, dbgen.ListUserMatchesParams{UserID: u.ID, CursorFinishedAt: at, CursorID: id, Lim: int32(limit + 1)})
	if err != nil {
		return HistoryPage{}, err
	}
	page := HistoryPage{Items: []HistoryItem{}}
	for i, row := range rows {
		if i == limit {
			last := page.Items[limit-1]
			page.NextCursor = pagination.Encode(*last.FinishedAt, last.ID)
			break
		}
		team := int(row.Seat) % 2
		item := HistoryItem{ID: row.ID, Status: replay.Status(row.Status), Team: team, CreatedAt: row.CreatedAt, FinishedAt: row.FinishedAt}
		if row.WinnerTeam != nil {
			winner := int(*row.WinnerTeam)
			item.WinnerTeam, item.Won = &winner, winner == team
		}
		if row.ScoreA != nil && row.ScoreB != nil {
			item.Score = &[2]int{int(*row.ScoreA), int(*row.ScoreB)}
		}
		page.Items = append(page.Items, item)
	}
	return page, nil
}
```

- [ ] **Step 6: Rodar e ver passar**

Run: `cd backend && go test ./internal/stats/ ./internal/pagination/ -race -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/pagination backend/internal/db backend/internal/stats
git commit -m "feat(stats): adiciona ranking, perfil e histórico paginado de partidas"
```

---

### Tarefa 16: `admin.Service` — gestão de usuários, domínios, partidas e auditoria

**Files:**
- Create: `backend/internal/db/queries/admin.sql`; regenerate `dbgen`
- Create: `backend/internal/admin/admin.go`
- Test: `backend/internal/admin/admin_test.go`

**Interfaces:**
- Consumes: `auth.Service.SendPasswordReset`, `auth.Principal`; `match.AbandonWith`, `match.ErrNotFound`, `match.ErrMatchClosed`; `pagination`; `db`.
- Produces (gerado): `InsertAudit(ctx, InsertAuditParams{ActorID, Action, TargetType, TargetID string; Details []byte; Now time.Time}) error`; `ListAudit(ctx, ListAuditParams{CursorCreatedAt *time.Time; CursorID *string; Lim int32}) ([]ListAuditRow, error)`; `SearchUsers(ctx, SearchUsersParams{Query string; CursorCreatedAt *time.Time; CursorID *string; Lim int32}) ([]User, error)`; `SetUserDisabled(ctx, SetUserDisabledParams{DisabledAt *time.Time; Now time.Time; ID string}) (int64, error)`; `SetUserAdmin(ctx, SetUserAdminParams{IsAdmin bool; Now time.Time; ID string}) (int64, error)`; `SetMustResetPassword(ctx, SetMustResetPasswordParams{Now time.Time; ID string}) (int64, error)`; `CountActiveAdmins(ctx) (int64, error)`; `LockAdmins(ctx) error`.
- Produces (pacote `admin`):

```go
type MatchAborter interface{ Abort(ctx context.Context, matchID string) error }
type NoopAborter struct{}
var ErrNotFound, ErrSelfAction, ErrLastAdmin, ErrLastDomain, ErrInvalidDomain error
type Options struct{ Pool *pgxpool.Pool; Auth *auth.Service; Aborter MatchAborter; Now func() time.Time; Logger *slog.Logger }
func NewService(o Options) *Service
type UserSummary struct{ ID, Username, Email, DisplayName string; IsAdmin, EmailVerified, Disabled, MustResetPassword bool; CreatedAt time.Time }
type UserPage struct{ Items []UserSummary; NextCursor string }
type Domain struct{ Domain string; CreatedAt time.Time; CreatedBy *string }
type AuditEntry struct{ ID, ActorID, ActorUsername, Action, TargetType, TargetID string; Details json.RawMessage; CreatedAt time.Time }
type AuditPage struct{ Items []AuditEntry; NextCursor string }
func (s *Service) ListUsers(ctx, query, cursor string) (UserPage, error)          // 50 por página
func (s *Service) DisableUser(ctx, actor auth.Principal, userID string) error
func (s *Service) EnableUser(ctx, actor auth.Principal, userID string) error
func (s *Service) ForcePasswordReset(ctx, actor auth.Principal, userID string) error
func (s *Service) SetAdmin(ctx, actor auth.Principal, userID string, isAdmin bool) error
func (s *Service) ListDomains(ctx) ([]Domain, error)
func (s *Service) AddDomain(ctx, actor auth.Principal, domain string) error
func (s *Service) RemoveDomain(ctx, actor auth.Principal, domain string) error
func (s *Service) AbortMatch(ctx, actor auth.Principal, matchID, reason string) error
func (s *Service) ListAudit(ctx, cursor string) (AuditPage, error)               // 50 por página
func PromoteAdmin(ctx, pool *pgxpool.Pool, login string, now time.Time) error
```

Ações de auditoria: `user.disable`, `user.enable`, `user.force_password_reset`, `user.set_admin`, `user.promote_admin`, `domain.add`, `domain.remove`, `match.abort`.

- [ ] **Step 1: Queries**

`backend/internal/db/queries/admin.sql`:

```sql
-- name: InsertAudit :exec
INSERT INTO audit_log (actor_id, action, target_type, target_id, details, created_at)
VALUES (@actor_id, @action, @target_type, @target_id, @details, @now);

-- name: ListAudit :many
SELECT a.id, a.actor_id, u.username AS actor_username, a.action, a.target_type, a.target_id, a.details, a.created_at
FROM audit_log a
JOIN users u ON u.id = a.actor_id
WHERE sqlc.narg(cursor_created_at)::timestamptz IS NULL
   OR (a.created_at, a.id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::uuid)
ORDER BY a.created_at DESC, a.id DESC
LIMIT @lim::integer;

-- name: SearchUsers :many
SELECT * FROM users
WHERE (@query::text = '' OR username LIKE '%' || @query::text || '%' ESCAPE '\'
                         OR email LIKE '%' || @query::text || '%' ESCAPE '\')
  AND (sqlc.narg(cursor_created_at)::timestamptz IS NULL
       OR (created_at, id) > (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::uuid))
ORDER BY created_at, id
LIMIT @lim::integer;

-- name: SetUserDisabled :execrows
UPDATE users SET disabled_at = sqlc.narg(disabled_at), updated_at = @now WHERE id = @id;

-- name: SetUserAdmin :execrows
UPDATE users SET is_admin = @is_admin, updated_at = @now WHERE id = @id;

-- name: SetMustResetPassword :execrows
UPDATE users SET must_reset_password = true, updated_at = @now WHERE id = @id;

-- name: CountActiveAdmins :one
SELECT count(*) FROM users WHERE is_admin AND disabled_at IS NULL;

-- name: LockAdmins :exec
SELECT id FROM users WHERE is_admin FOR UPDATE;
```

Run: `make gen-sql && cd backend && go build ./...`

- [ ] **Step 2: Escrever os testes que falham**

`backend/internal/admin/admin_test.go`:

```go
package admin_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/admin"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/auth"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/mail"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/match"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/testclock"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/testdb"
)

func TestMain(m *testing.M) { testdb.Run(m) }

type recordingAborter struct {
	mu  sync.Mutex
	ids []string
}

func (r *recordingAborter) Abort(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ids = append(r.ids, id)
	return nil
}

type env struct {
	svc     *admin.Service
	auth    *auth.Service
	pool    *pgxpool.Pool
	mail    *mail.Fake
	aborter *recordingAborter
	actor   auth.Principal
}

func newEnv(t *testing.T) env {
	t.Helper()
	pool := testdb.New(t)
	clock := testclock.New(time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC))
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	e := env{pool: pool, mail: &mail.Fake{}, aborter: &recordingAborter{}}
	e.auth = auth.NewService(auth.Options{Pool: pool, Mailer: e.mail, Hasher: auth.NewHasher(auth.TestingParams, 4),
		Now: clock.Now, BaseURL: "https://truco.test", Logger: logger, Background: func(fn func()) { fn() }})
	if err := e.auth.BootstrapDomains(context.Background(), []string{"empresa.com"}); err != nil {
		t.Fatal(err)
	}
	e.svc = admin.NewService(admin.Options{Pool: pool, Auth: e.auth, Aborter: e.aborter, Now: clock.Now, Logger: logger})
	adminID := testdb.CreateUser(t, pool, "chefe")
	_, _ = pool.Exec(context.Background(), "UPDATE users SET is_admin = true WHERE id = $1", adminID)
	e.actor = auth.Principal{UserID: adminID, Username: "chefe", IsAdmin: true}
	return e
}

func (e env) auditCount(t *testing.T, action string) int {
	t.Helper()
	var n int
	_ = e.pool.QueryRow(context.Background(), "SELECT count(*) FROM audit_log WHERE action = $1", action).Scan(&n)
	return n
}

func (e env) sessionCount(t *testing.T, userID string) int {
	t.Helper()
	var n int
	_ = e.pool.QueryRow(context.Background(), "SELECT count(*) FROM sessions WHERE user_id = $1", userID).Scan(&n)
	return n
}

func (e env) addSession(t *testing.T, userID string) {
	t.Helper()
	_, err := e.pool.Exec(context.Background(), `INSERT INTO sessions (user_id, token_hash, last_seen_at, expires_at, absolute_expires_at)
		VALUES ($1, decode(md5(random()::text), 'hex'), now(), now() + interval '1 day', now() + interval '2 days')`, userID)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDisableAndEnableUser(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	target := testdb.CreateUser(t, e.pool, "ana")
	e.addSession(t, target)
	if err := e.svc.DisableUser(ctx, e.actor, target); err != nil {
		t.Fatal(err)
	}
	if e.sessionCount(t, target) != 0 || e.auditCount(t, "user.disable") != 1 {
		t.Fatal("desativar deve revogar sessões e auditar")
	}
	if err := e.svc.EnableUser(ctx, e.actor, target); err != nil || e.auditCount(t, "user.enable") != 1 {
		t.Fatalf("enable: %v", err)
	}
	if err := e.svc.DisableUser(ctx, e.actor, e.actor.UserID); !errors.Is(err, admin.ErrSelfAction) {
		t.Fatalf("auto-desativação: %v", err)
	}
	for _, missing := range []string{"00000000-0000-7000-8000-000000000000", "nao-e-uuid"} {
		if err := e.svc.DisableUser(ctx, e.actor, missing); !errors.Is(err, admin.ErrNotFound) {
			t.Fatalf("%q: %v", missing, err)
		}
	}
}

func TestForcePasswordReset(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	target := testdb.CreateUser(t, e.pool, "ana")
	e.addSession(t, target)
	if err := e.svc.ForcePasswordReset(ctx, e.actor, target); err != nil {
		t.Fatal(err)
	}
	var must bool
	_ = e.pool.QueryRow(ctx, "SELECT must_reset_password FROM users WHERE id = $1", target).Scan(&must)
	msg, sent := e.mail.LastTo("ana@empresa.com")
	if !must || e.sessionCount(t, target) != 0 || !sent || msg.Subject != "Você precisa redefinir sua senha do Truco" || e.auditCount(t, "user.force_password_reset") != 1 {
		t.Fatalf("must=%v sessões=%d e-mail=%v", must, e.sessionCount(t, target), msg.Subject)
	}
}

func TestSetAdminProtectsLastAdmin(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	if err := e.svc.SetAdmin(ctx, e.actor, e.actor.UserID, false); !errors.Is(err, admin.ErrLastAdmin) {
		t.Fatalf("remover o último admin: %v", err)
	}
	if e.auditCount(t, "user.set_admin") != 0 {
		t.Fatal("ação recusada não pode ser auditada")
	}
	other := testdb.CreateUser(t, e.pool, "bia")
	if err := e.svc.SetAdmin(ctx, e.actor, other, true); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.SetAdmin(ctx, e.actor, e.actor.UserID, false); err != nil {
		t.Fatalf("com dois admins, pode sair: %v", err)
	}
	if e.auditCount(t, "user.set_admin") != 2 {
		t.Fatal("auditoria ausente")
	}
}

func TestDomains(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	for _, bad := range []string{"", "sem-ponto", "a@b.com", "espaço.com", "-x.com"} {
		if err := e.svc.AddDomain(ctx, e.actor, bad); !errors.Is(err, admin.ErrInvalidDomain) {
			t.Errorf("%q: %v", bad, err)
		}
	}
	if err := e.svc.AddDomain(ctx, e.actor, " @Filial.Com.BR "); err != nil {
		t.Fatal(err)
	}
	domains, _ := e.svc.ListDomains(ctx)
	if len(domains) != 2 || domains[1].Domain != "filial.com.br" || domains[1].CreatedBy == nil || *domains[1].CreatedBy != e.actor.UserID {
		t.Fatalf("domínios = %+v", domains)
	}
	if err := e.svc.RemoveDomain(ctx, e.actor, "empresa.com"); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.RemoveDomain(ctx, e.actor, "filial.com.br"); !errors.Is(err, admin.ErrLastDomain) {
		t.Fatalf("último domínio: %v", err)
	}
	if err := e.svc.RemoveDomain(ctx, e.actor, "nao.existe"); !errors.Is(err, admin.ErrNotFound) {
		t.Fatalf("inexistente: %v", err)
	}
	if e.auditCount(t, "domain.add") != 1 || e.auditCount(t, "domain.remove") != 1 {
		t.Fatal("auditoria de domínios incorreta")
	}
}

func TestAbortMatch(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	var players [4]replay.PlayerSeat
	for i, name := range []string{"ana", "bia", "caio", "duda"} {
		players[i] = replay.PlayerSeat{UserID: testdb.CreateUser(t, e.pool, name), DisplayName: name}
	}
	store := match.NewStore(e.pool, nil)
	id, _ := store.Create(ctx, 1, 0, players)
	if err := e.svc.AbortMatch(ctx, e.actor, id, "mesa travada"); err != nil {
		t.Fatal(err)
	}
	summary, _ := store.Summary(ctx, id)
	if summary.Status != replay.StatusAbandoned || *summary.AbandonReason != "mesa travada" {
		t.Fatalf("summary = %+v", summary)
	}
	if len(e.aborter.ids) != 1 || e.aborter.ids[0] != id || e.auditCount(t, "match.abort") != 1 {
		t.Fatalf("aborter=%v", e.aborter.ids)
	}
	if err := e.svc.AbortMatch(ctx, e.actor, id, "de novo"); !errors.Is(err, match.ErrMatchClosed) {
		t.Fatalf("abortar duas vezes: %v", err)
	}
}

func TestListUsersSearchAndPagination(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	for i := range 51 {
		testdb.CreateUser(t, e.pool, fmt.Sprintf("user%02d", i))
	}
	page, err := e.svc.ListUsers(ctx, "", "")
	if err != nil || len(page.Items) != 50 || page.NextCursor == "" {
		t.Fatalf("página 1: %d itens, cursor %q, err %v", len(page.Items), page.NextCursor, err)
	}
	page2, _ := e.svc.ListUsers(ctx, "", page.NextCursor)
	if len(page2.Items) != 2 || page2.NextCursor != "" {
		t.Fatalf("página 2: %+v", page2)
	}
	found, _ := e.svc.ListUsers(ctx, "USER0", "")
	if len(found.Items) != 10 {
		t.Fatalf("busca USER0: %d", len(found.Items))
	}
	if none, _ := e.svc.ListUsers(ctx, "%", ""); len(none.Items) != 0 {
		t.Fatal("% deve ser literal na busca")
	}
}

func TestListAuditNewestFirst(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	_ = e.svc.AddDomain(ctx, e.actor, "a.com")
	_ = e.svc.AddDomain(ctx, e.actor, "b.com")
	page, err := e.svc.ListAudit(ctx, "")
	if err != nil || len(page.Items) != 2 || page.Items[0].TargetID != "b.com" || page.Items[0].ActorUsername != "chefe" {
		t.Fatalf("auditoria = %+v err %v", page, err)
	}
}

func TestPromoteAdmin(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	id := testdb.CreateUser(t, e.pool, "ana")
	if err := admin.PromoteAdmin(ctx, e.pool, " ANA@empresa.com ", time.Now()); err != nil {
		t.Fatal(err)
	}
	var isAdmin bool
	_ = e.pool.QueryRow(ctx, "SELECT is_admin FROM users WHERE id = $1", id).Scan(&isAdmin)
	if !isAdmin || e.auditCount(t, "user.promote_admin") != 1 {
		t.Fatal("promoção não aplicada ou não auditada")
	}
	if err := admin.PromoteAdmin(ctx, e.pool, "bia", time.Now()); err == nil {
		t.Fatal("usuário inexistente deve falhar")
	}
}
```

- [ ] **Step 3: Rodar e ver falhar**

Run: `cd backend && go test ./internal/admin/`
Expected: FAIL de compilação.

- [ ] **Step 4: Implementar**

`backend/internal/admin/admin.go`:

```go
// Package admin implementa as operações administrativas, sempre auditadas na mesma transação.
package admin

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/auth"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/db"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/db/dbgen"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/match"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/pagination"
)

// MatchAborter encerra a sala viva de uma partida abortada. A Spec 3 implementa.
type MatchAborter interface {
	Abort(ctx context.Context, matchID string) error
}

type NoopAborter struct{}

func (NoopAborter) Abort(context.Context, string) error { return nil }

var (
	ErrNotFound      = errors.New("admin: não encontrado")
	ErrSelfAction    = errors.New("admin: não é possível aplicar esta ação a si mesmo")
	ErrLastAdmin     = errors.New("admin: é preciso manter ao menos um administrador ativo")
	ErrLastDomain    = errors.New("admin: é preciso manter ao menos um domínio permitido")
	ErrInvalidDomain = errors.New("admin: domínio inválido")
)

const pageSize = 50

var domainPattern = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$`)

type Options struct {
	Pool    *pgxpool.Pool
	Auth    *auth.Service
	Aborter MatchAborter
	Now     func() time.Time
	Logger  *slog.Logger
}

type Service struct {
	pool    *pgxpool.Pool
	q       *dbgen.Queries
	auth    *auth.Service
	aborter MatchAborter
	now     func() time.Time
	log     *slog.Logger
}

func NewService(o Options) *Service {
	s := &Service{pool: o.Pool, q: dbgen.New(o.Pool), auth: o.Auth, aborter: o.Aborter, now: o.Now, log: o.Logger}
	if s.aborter == nil {
		s.aborter = NoopAborter{}
	}
	if s.now == nil {
		s.now = time.Now
	}
	if s.log == nil {
		s.log = slog.Default()
	}
	return s
}

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) || db.IsInvalidInput(err) {
		return ErrNotFound
	}
	return err
}

func writeAudit(ctx context.Context, q *dbgen.Queries, actorID, action, targetType, targetID string, details map[string]any, now time.Time) error {
	if details == nil {
		details = map[string]any{}
	}
	raw, err := json.Marshal(details)
	if err != nil {
		return err
	}
	return q.InsertAudit(ctx, dbgen.InsertAuditParams{ActorID: actorID, Action: action, TargetType: targetType, TargetID: targetID, Details: raw, Now: now})
}

// audited executa fn e grava a auditoria na mesma transação; se fn falhar, nada é gravado.
func (s *Service) audited(ctx context.Context, actor auth.Principal, action, targetType, targetID string, details map[string]any, fn func(q *dbgen.Queries, now time.Time) error) error {
	now := s.now()
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		if err := fn(q, now); err != nil {
			return err
		}
		return writeAudit(ctx, q, actor.UserID, action, targetType, targetID, details, now)
	})
}

func requireRow(n int64, err error) error {
	if err != nil {
		return notFound(err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) DisableUser(ctx context.Context, actor auth.Principal, userID string) error {
	if userID == actor.UserID {
		return ErrSelfAction
	}
	return s.audited(ctx, actor, "user.disable", "user", userID, nil, func(q *dbgen.Queries, now time.Time) error {
		if err := requireRow(q.SetUserDisabled(ctx, dbgen.SetUserDisabledParams{DisabledAt: &now, Now: now, ID: userID})); err != nil {
			return err
		}
		return q.DeleteUserSessions(ctx, userID)
	})
}

func (s *Service) EnableUser(ctx context.Context, actor auth.Principal, userID string) error {
	return s.audited(ctx, actor, "user.enable", "user", userID, nil, func(q *dbgen.Queries, now time.Time) error {
		return requireRow(q.SetUserDisabled(ctx, dbgen.SetUserDisabledParams{DisabledAt: nil, Now: now, ID: userID}))
	})
}

func (s *Service) ForcePasswordReset(ctx context.Context, actor auth.Principal, userID string) error {
	err := s.audited(ctx, actor, "user.force_password_reset", "user", userID, nil, func(q *dbgen.Queries, now time.Time) error {
		if err := requireRow(q.SetMustResetPassword(ctx, dbgen.SetMustResetPasswordParams{Now: now, ID: userID})); err != nil {
			return err
		}
		return q.DeleteUserSessions(ctx, userID)
	})
	if err != nil {
		return err
	}
	return s.auth.SendPasswordReset(ctx, userID, true)
}

func (s *Service) SetAdmin(ctx context.Context, actor auth.Principal, userID string, isAdmin bool) error {
	details := map[string]any{"isAdmin": isAdmin}
	return s.audited(ctx, actor, "user.set_admin", "user", userID, details, func(q *dbgen.Queries, now time.Time) error {
		if err := q.LockAdmins(ctx); err != nil {
			return err
		}
		if !isAdmin {
			target, err := q.GetUserByID(ctx, userID)
			if err != nil {
				return notFound(err)
			}
			if target.IsAdmin && target.DisabledAt == nil {
				count, err := q.CountActiveAdmins(ctx)
				if err != nil {
					return err
				}
				if count <= 1 {
					return ErrLastAdmin
				}
			}
		}
		return requireRow(q.SetUserAdmin(ctx, dbgen.SetUserAdminParams{IsAdmin: isAdmin, Now: now, ID: userID}))
	})
}

type Domain struct {
	Domain    string    `json:"domain"`
	CreatedAt time.Time `json:"createdAt"`
	CreatedBy *string   `json:"createdBy"`
}

func (s *Service) ListDomains(ctx context.Context) ([]Domain, error) {
	rows, err := s.q.ListDomains(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Domain, len(rows))
	for i, r := range rows {
		out[i] = Domain{Domain: r.Domain, CreatedAt: r.CreatedAt, CreatedBy: r.CreatedBy}
	}
	return out, nil
}

func normalizeDomain(d string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(d)), "@")
}

func (s *Service) AddDomain(ctx context.Context, actor auth.Principal, domain string) error {
	domain = normalizeDomain(domain)
	if !domainPattern.MatchString(domain) {
		return ErrInvalidDomain
	}
	return s.audited(ctx, actor, "domain.add", "domain", domain, nil, func(q *dbgen.Queries, now time.Time) error {
		creator := actor.UserID
		return q.InsertDomain(ctx, dbgen.InsertDomainParams{Domain: domain, Now: now, CreatedBy: &creator})
	})
}

func (s *Service) RemoveDomain(ctx context.Context, actor auth.Principal, domain string) error {
	domain = normalizeDomain(domain)
	return s.audited(ctx, actor, "domain.remove", "domain", domain, nil, func(q *dbgen.Queries, now time.Time) error {
		if err := q.LockDomains(ctx); err != nil {
			return err
		}
		if err := requireRow(q.DeleteDomain(ctx, domain)); err != nil {
			return err
		}
		remaining, err := q.CountDomains(ctx)
		if err != nil {
			return err
		}
		if remaining == 0 {
			return ErrLastDomain // desfaz a remoção junto com a transação
		}
		return nil
	})
}

func (s *Service) AbortMatch(ctx context.Context, actor auth.Principal, matchID, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "abortada pelo administrador"
	}
	err := s.audited(ctx, actor, "match.abort", "match", matchID, map[string]any{"reason": reason}, func(q *dbgen.Queries, now time.Time) error {
		return match.AbandonWith(ctx, q, matchID, reason, now)
	})
	if err != nil {
		return err
	}
	if err := s.aborter.Abort(ctx, matchID); err != nil {
		s.log.ErrorContext(ctx, "falha ao encerrar a sala da partida abortada", "match_id", matchID, "err", err)
	}
	return nil
}

type UserSummary struct {
	ID                string    `json:"id"`
	Username          string    `json:"username"`
	Email             string    `json:"email"`
	DisplayName       string    `json:"displayName"`
	IsAdmin           bool      `json:"isAdmin"`
	EmailVerified     bool      `json:"emailVerified"`
	Disabled          bool      `json:"disabled"`
	MustResetPassword bool      `json:"mustResetPassword"`
	CreatedAt         time.Time `json:"createdAt"`
}

type UserPage struct {
	Items      []UserSummary `json:"items"`
	NextCursor string        `json:"nextCursor,omitempty"`
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func (s *Service) ListUsers(ctx context.Context, query, cursor string) (UserPage, error) {
	at, id, err := pagination.Decode(cursor)
	if err != nil {
		return UserPage{}, err
	}
	rows, err := s.q.SearchUsers(ctx, dbgen.SearchUsersParams{Query: likeEscaper.Replace(strings.ToLower(strings.TrimSpace(query))),
		CursorCreatedAt: at, CursorID: id, Lim: pageSize + 1})
	if err != nil {
		return UserPage{}, err
	}
	page := UserPage{Items: []UserSummary{}}
	for i, u := range rows {
		if i == pageSize {
			last := page.Items[pageSize-1]
			page.NextCursor = pagination.Encode(last.CreatedAt, last.ID)
			break
		}
		page.Items = append(page.Items, UserSummary{ID: u.ID, Username: u.Username, Email: u.Email, DisplayName: u.DisplayName,
			IsAdmin: u.IsAdmin, EmailVerified: u.EmailVerifiedAt != nil, Disabled: u.DisabledAt != nil,
			MustResetPassword: u.MustResetPassword, CreatedAt: u.CreatedAt})
	}
	return page, nil
}

type AuditEntry struct {
	ID            string          `json:"id"`
	ActorID       string          `json:"actorId"`
	ActorUsername string          `json:"actorUsername"`
	Action        string          `json:"action"`
	TargetType    string          `json:"targetType"`
	TargetID      string          `json:"targetId"`
	Details       json.RawMessage `json:"details"`
	CreatedAt     time.Time       `json:"createdAt"`
}

type AuditPage struct {
	Items      []AuditEntry `json:"items"`
	NextCursor string       `json:"nextCursor,omitempty"`
}

func (s *Service) ListAudit(ctx context.Context, cursor string) (AuditPage, error) {
	at, id, err := pagination.Decode(cursor)
	if err != nil {
		return AuditPage{}, err
	}
	rows, err := s.q.ListAudit(ctx, dbgen.ListAuditParams{CursorCreatedAt: at, CursorID: id, Lim: pageSize + 1})
	if err != nil {
		return AuditPage{}, err
	}
	page := AuditPage{Items: []AuditEntry{}}
	for i, r := range rows {
		if i == pageSize {
			last := page.Items[pageSize-1]
			page.NextCursor = pagination.Encode(last.CreatedAt, last.ID)
			break
		}
		page.Items = append(page.Items, AuditEntry{ID: r.ID, ActorID: r.ActorID, ActorUsername: r.ActorUsername, Action: r.Action,
			TargetType: r.TargetType, TargetID: r.TargetID, Details: r.Details, CreatedAt: r.CreatedAt})
	}
	return page, nil
}

// PromoteAdmin é usado pelo subcomando `server promote-admin <email|username>` para criar o primeiro admin.
func PromoteAdmin(ctx context.Context, pool *pgxpool.Pool, login string, now time.Time) error {
	q := dbgen.New(pool)
	login = strings.ToLower(strings.TrimSpace(login))
	var user dbgen.User
	var err error
	if strings.Contains(login, "@") {
		user, err = q.GetUserByEmail(ctx, login)
	} else {
		user, err = q.GetUserByUsername(ctx, login)
	}
	if err != nil {
		return notFound(err)
	}
	return db.WithTx(ctx, pool, func(tx pgx.Tx) error {
		qt := q.WithTx(tx)
		if _, err := qt.SetUserAdmin(ctx, dbgen.SetUserAdminParams{IsAdmin: true, Now: now, ID: user.ID}); err != nil {
			return err
		}
		return writeAudit(ctx, qt, user.ID, "user.promote_admin", "user", user.ID, map[string]any{"via": "cli"}, now)
	})
}
```

- [ ] **Step 5: Rodar e ver passar**

Run: `cd backend && go test ./internal/admin/ -race -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/db backend/internal/admin
git commit -m "feat(admin): adiciona gestão de usuários, domínios e partidas com auditoria"
```

---
### Tarefa 17: `httpapi` — servidor, respostas, middlewares e estáticos

**Files:**
- Create: `backend/internal/httpapi/server.go`, `respond.go`, `middleware.go`, `static.go`
- Test: `backend/internal/httpapi/internal_test.go` (pacote `httpapi`)
- Test: `backend/internal/httpapi/harness_test.go`, `backend/internal/httpapi/server_test.go` (pacote `httpapi_test`)

**Interfaces:**
- Consumes: `config.Config`, `auth.Service` (`Authenticate`, `Cookies`, `CleanupExpired`), `auth.CheckOrigin`, `ratelimit`, sentinelas de `auth`, `stats`, `match`, `admin`, `pagination`, `replay`.
- Produces:

```go
type Deps struct {
	Config  config.Config
	Auth    *auth.Service
	Stats   *stats.Reader
	Matches *match.Store
	Admin   *admin.Service
	Pool    *pgxpool.Pool
	Logger  *slog.Logger
	Now     func() time.Time // relógio dos rate limits; padrão time.Now
}
func New(d Deps) *Server
func (s *Server) Handler() http.Handler
func (s *Server) RunMaintenance(ctx context.Context) // varre limites a cada 5 min e limpa sessões/tokens a cada hora
```

Internos usados pelas Tarefas 18–20: `type authedHandler func(http.ResponseWriter, *http.Request, auth.Principal)`; `s.requireAuth(authedHandler) http.Handler`; `s.requireAdmin(authedHandler) http.Handler`; `decodeJSON(w, r, dst any) error`; `writeJSON(w, status int, v any)`; `s.writeError(w, r, err error)`; `s.allow(w, r, l *ratelimit.Limiter, key string) bool`; `s.writeTooMany(w, r, retry time.Duration)`; `s.clientIP(r) string`; `s.sessionMeta(r) auth.SessionMeta`; erros `errInvalidJSON`, `errForbidden`, `errMatchInProgress`; `s.mux *http.ServeMux` (cada tarefa registra suas rotas em `registerAuthRoutes`, `registerReadRoutes`, `registerAdminRoutes`).

- [ ] **Step 1: Escrever os testes internos que falham**

`backend/internal/httpapi/internal_test.go`:

```go
package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/admin"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/auth"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/config"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/match"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/pagination"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/stats"
)

func TestDecodeJSON(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}
	cases := []struct {
		name, contentType, body string
		want                    error
	}{
		{"ok", "application/json", `{"name":"ana"}`, nil},
		{"charset", "application/json; charset=utf-8", `{"name":"ana"}`, nil},
		{"tipo errado", "text/plain", `{"name":"ana"}`, errUnsupportedMedia},
		{"campo desconhecido", "application/json", `{"name":"ana","seat":1}`, errInvalidJSON},
		{"malformado", "application/json", `{"name":`, errInvalidJSON},
		{"lixo depois", "application/json", `{"name":"ana"} {}`, errInvalidJSON},
		{"grande demais", "application/json", `{"name":"` + strings.Repeat("a", 17<<10) + `"}`, errBodyTooLarge},
	}
	for _, c := range cases {
		r := httptest.NewRequest(http.MethodPost, "/api/x", strings.NewReader(c.body))
		r.Header.Set("Content-Type", c.contentType)
		var p payload
		if err := decodeJSON(httptest.NewRecorder(), r, &p); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, quer %v", c.name, err, c.want)
		}
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{auth.FieldErrors{"email": "inválido"}, 400, "validation_failed"},
		{auth.ErrEmailDomainNotAllowed, 403, "email_domain_not_allowed"},
		{auth.ErrUsernameTaken, 409, "username_taken"},
		{auth.ErrInvalidToken, 400, "invalid_token"},
		{auth.ErrInvalidCredentials, 401, "invalid_credentials"},
		{auth.ErrEmailNotVerified, 403, "email_not_verified"},
		{auth.ErrAccountDisabled, 403, "account_disabled"},
		{auth.ErrPasswordResetRequired, 403, "password_reset_required"},
		{auth.ErrUnauthenticated, 401, "unauthenticated"},
		{auth.ErrUserNotFound, 404, "not_found"},
		{stats.ErrUserNotFound, 404, "not_found"},
		{match.ErrNotFound, 404, "not_found"},
		{admin.ErrNotFound, 404, "not_found"},
		{pagination.ErrInvalidCursor, 400, "validation_failed"},
		{admin.ErrInvalidDomain, 400, "validation_failed"},
		{admin.ErrSelfAction, 409, "conflict"},
		{admin.ErrLastAdmin, 409, "conflict"},
		{admin.ErrLastDomain, 409, "conflict"},
		{match.ErrMatchClosed, 409, "conflict"},
		{&replay.CorruptLogError{Ply: 3, Err: errors.New("x")}, 409, "conflict"},
		{fmt.Errorf("embrulhado: %w", replay.ErrIncompatibleVersion), 409, "conflict"},
		{errMatchInProgress, 409, "match_in_progress"},
		{errors.New("qualquer outra coisa"), 500, "internal"},
	}
	for _, c := range cases {
		he := classify(c.err)
		if he.status != c.status || he.code != c.code {
			t.Errorf("%v: %d %s, quer %d %s", c.err, he.status, he.code, c.status, c.code)
		}
	}
}

func TestRecovererReturnsJSON500(t *testing.T) {
	s := &Server{log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	h := s.recoverer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/x", nil))
	var body struct {
		Error struct{ Code string } `json:"error"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != 500 || body.Error.Code != "internal" {
		t.Fatalf("status %d corpo %s", rec.Code, rec.Body)
	}
}

func TestClientIP(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.5:5555"
	r.Header.Set("X-Forwarded-For", "1.1.1.1, 203.0.113.9")
	direct := &Server{deps: Deps{Config: config.Config{TrustProxy: false}}}
	proxied := &Server{deps: Deps{Config: config.Config{TrustProxy: true}}}
	if got := direct.clientIP(r); got != "10.0.0.5" {
		t.Fatalf("sem proxy: %q", got)
	}
	if got := proxied.clientIP(r); got != "203.0.113.9" {
		t.Fatalf("com proxy: %q (usa o último hop)", got)
	}
	r.Header.Set("X-Forwarded-For", "lixo")
	if got := proxied.clientIP(r); got != "10.0.0.5" {
		t.Fatalf("XFF inválido deve cair no RemoteAddr: %q", got)
	}
}
```

- [ ] **Step 2: Escrever o harness e os testes de servidor que falham**

`backend/internal/httpapi/harness_test.go`:

```go
package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/admin"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/auth"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/config"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/httpapi"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/mail"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/match"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/stats"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/testclock"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/testdb"
)

func TestMain(m *testing.M) { testdb.Run(m) }

const origin = "http://truco.test"

type harness struct {
	t      *testing.T
	srv    *httptest.Server
	mail   *mail.Fake
	clock  *testclock.Clock
	pool   *pgxpool.Pool
	store  *match.Store
	nextIP atomic.Int32
}

func newHarness(t *testing.T, mutate ...func(*httpapi.Deps)) *harness {
	t.Helper()
	pool := testdb.New(t)
	h := &harness{t: t, mail: &mail.Fake{}, clock: testclock.New(time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)), pool: pool}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	// TrustProxy permite que cada cliente de teste tenha seu próprio IP via X-Forwarded-For.
	cfg := config.Config{AppBaseURL: origin, AppOrigin: origin, CookieSecure: false, TrustProxy: true, RankingMinMatches: 1}
	authSvc := auth.NewService(auth.Options{Pool: pool, Mailer: h.mail, Hasher: auth.NewHasher(auth.TestingParams, 4), Now: h.clock.Now,
		BaseURL: origin, Cookies: auth.CookieConfig{Secure: false}, Logger: logger, Background: func(fn func()) { fn() }})
	if err := authSvc.BootstrapDomains(context.Background(), []string{"empresa.com"}); err != nil {
		t.Fatal(err)
	}
	h.store = match.NewStore(pool, h.clock.Now)
	deps := httpapi.Deps{Config: cfg, Auth: authSvc, Stats: stats.NewReader(pool, 1), Matches: h.store,
		Admin: admin.NewService(admin.Options{Pool: pool, Auth: authSvc, Now: h.clock.Now, Logger: logger}),
		Pool: pool, Logger: logger, Now: h.clock.Now}
	for _, m := range mutate {
		m(&deps)
	}
	h.srv = httptest.NewServer(httpapi.New(deps).Handler())
	t.Cleanup(h.srv.Close)
	return h
}

type client struct {
	h    *harness
	http *http.Client
	ip   string
}

// client cria um navegador de teste com cookies próprios e IP próprio.
func (h *harness) client() *client {
	jar, _ := cookiejar.New(nil)
	return &client{h: h, http: &http.Client{Jar: jar}, ip: fmt.Sprintf("198.51.100.%d", h.nextIP.Add(1))}
}

type rawBody string

type response struct {
	status int
	header http.Header
	body   []byte
}

func (r response) decode(t *testing.T, v any) {
	t.Helper()
	if err := json.Unmarshal(r.body, v); err != nil {
		t.Fatalf("JSON inválido (%d): %s", r.status, r.body)
	}
}

func (r response) code() string {
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(r.body, &body)
	return body.Error.Code
}

// do envia a requisição; headers são pares nome/valor (valor vazio remove o header padrão).
func (c *client) do(method, path string, body any, headers ...string) response {
	c.h.t.Helper()
	var reader io.Reader
	switch b := body.(type) {
	case nil:
	case rawBody:
		reader = bytes.NewReader([]byte(b))
	default:
		raw, _ := json.Marshal(b)
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, c.h.srv.URL+path, reader)
	if err != nil {
		c.h.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Origin", origin)
	req.Header.Set("X-Forwarded-For", c.ip)
	for i := 0; i+1 < len(headers); i += 2 {
		if headers[i+1] == "" {
			req.Header.Del(headers[i])
		} else {
			req.Header.Set(headers[i], headers[i+1])
		}
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.h.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return response{status: resp.StatusCode, header: resp.Header, body: raw}
}

func (c *client) get(path string, headers ...string) response { return c.do(http.MethodGet, path, nil, headers...) }
func (c *client) post(path string, body any, headers ...string) response {
	return c.do(http.MethodPost, path, body, headers...)
}

func expect(t *testing.T, r response, status int, code string) {
	t.Helper()
	if r.status != status || r.code() != code {
		t.Fatalf("status %d código %q, quer %d %q; corpo %s", r.status, r.code(), status, code, r.body)
	}
}
```

`backend/internal/httpapi/server_test.go`:

```go
package httpapi_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/httpapi"
)

func TestHealthz(t *testing.T) {
	h := newHarness(t)
	r := h.client().get("/healthz")
	if r.status != http.StatusOK || !strings.Contains(string(r.body), "ok") {
		t.Fatalf("healthz = %d %s", r.status, r.body)
	}
}

func TestUnknownAPIRouteIsJSONWithSecurityHeaders(t *testing.T) {
	h := newHarness(t)
	r := h.client().get("/api/nao-existe")
	expect(t, r, http.StatusNotFound, "not_found")
	for header, want := range map[string]string{
		"X-Content-Type-Options": "nosniff", "Referrer-Policy": "no-referrer", "X-Frame-Options": "DENY", "Cache-Control": "no-store",
	} {
		if got := r.header.Get(header); got != want {
			t.Errorf("%s = %q, quer %q", header, got, want)
		}
	}
	if r.header.Get("X-Request-Id") == "" || !strings.HasPrefix(r.header.Get("Content-Type"), "application/json") {
		t.Fatalf("headers = %v", r.header)
	}
}

func TestMutatingRequestsRequireOrigin(t *testing.T) {
	h := newHarness(t)
	c := h.client()
	expect(t, c.post("/api/qualquer", nil, "Origin", ""), http.StatusForbidden, "forbidden_origin")
	expect(t, c.post("/api/qualquer", nil, "Origin", "https://evil.test"), http.StatusForbidden, "forbidden_origin")
	expect(t, c.post("/api/qualquer", nil, "Origin", "", "Sec-Fetch-Site", "same-origin"), http.StatusNotFound, "not_found")
	expect(t, c.get("/api/qualquer", "Origin", ""), http.StatusNotFound, "not_found") // GET não exige Origin
}

func TestStaticSPAFallback(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>spa</html>"), 0o644)
	_ = os.MkdirAll(filepath.Join(dir, "assets"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "assets", "app.js"), []byte("console.log(1)"), 0o644)
	h := newHarness(t, func(d *httpapi.Deps) { d.Config.StaticDir = dir })
	c := h.client()
	if r := c.get("/assets/app.js"); r.status != 200 || string(r.body) != "console.log(1)" {
		t.Fatalf("arquivo: %d %s", r.status, r.body)
	}
	r := c.get("/ranking/semanal")
	if r.status != 200 || !strings.Contains(string(r.body), "spa") || !strings.Contains(r.header.Get("Content-Security-Policy"), "default-src 'self'") {
		t.Fatalf("fallback: %d %s %v", r.status, r.body, r.header)
	}
	expect(t, c.get("/api/nao-existe"), http.StatusNotFound, "not_found")
}
```

- [ ] **Step 3: Rodar e ver falhar**

Run: `cd backend && go test ./internal/httpapi/`
Expected: FAIL de compilação.

- [ ] **Step 4: Implementar `respond.go`**

```go
package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/admin"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/auth"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/match"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/pagination"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/stats"
)

const maxBodyBytes = 16 << 10

type httpError struct {
	status  int
	code    string
	message string
	fields  map[string]string
}

func (e *httpError) Error() string { return e.code + ": " + e.message }

var (
	errInvalidJSON      = &httpError{status: 400, code: "invalid_request", message: "Corpo da requisição inválido."}
	errUnsupportedMedia = &httpError{status: 415, code: "invalid_request", message: "Envie o corpo como application/json."}
	errBodyTooLarge     = &httpError{status: 413, code: "invalid_request", message: "Corpo da requisição grande demais."}
	errForbiddenOrigin  = &httpError{status: 403, code: "forbidden_origin", message: "Origem da requisição não permitida."}
	errForbidden        = &httpError{status: 403, code: "forbidden", message: "Acesso restrito a administradores."}
	errNotFoundRoute    = &httpError{status: 404, code: "not_found", message: "Recurso não encontrado."}
	errTooMany          = &httpError{status: 429, code: "too_many_attempts", message: "Muitas tentativas. Aguarde um pouco e tente de novo."}
	errMatchInProgress  = &httpError{status: 409, code: "match_in_progress", message: "A partida ainda está em andamento."}
	errInternal         = &httpError{status: 500, code: "internal", message: "Erro interno. Tente novamente."}
)

func conflict(message string) *httpError { return &httpError{status: 409, code: "conflict", message: message} }

func invalidField(field, reason string) *httpError {
	return &httpError{status: 400, code: "validation_failed", message: "Verifique os campos destacados.", fields: map[string]string{field: reason}}
}

// classify traduz erros de domínio para status e código da API.
func classify(err error) *httpError {
	var he *httpError
	if errors.As(err, &he) {
		return he
	}
	var fields auth.FieldErrors
	if errors.As(err, &fields) {
		return &httpError{status: 400, code: "validation_failed", message: "Verifique os campos destacados.", fields: fields}
	}
	var corrupt *replay.CorruptLogError
	switch {
	case errors.Is(err, auth.ErrEmailDomainNotAllowed):
		return &httpError{status: 403, code: "email_domain_not_allowed", message: "O cadastro só é permitido com o e-mail da empresa."}
	case errors.Is(err, auth.ErrUsernameTaken):
		return &httpError{status: 409, code: "username_taken", message: "Este nome de usuário já está em uso."}
	case errors.Is(err, auth.ErrInvalidToken):
		return &httpError{status: 400, code: "invalid_token", message: "Link inválido ou expirado."}
	case errors.Is(err, auth.ErrInvalidCredentials):
		return &httpError{status: 401, code: "invalid_credentials", message: "Usuário ou senha incorretos."}
	case errors.Is(err, auth.ErrEmailNotVerified):
		return &httpError{status: 403, code: "email_not_verified", message: "Confirme seu e-mail antes de entrar."}
	case errors.Is(err, auth.ErrAccountDisabled):
		return &httpError{status: 403, code: "account_disabled", message: "Esta conta está desativada."}
	case errors.Is(err, auth.ErrPasswordResetRequired):
		return &httpError{status: 403, code: "password_reset_required", message: "Você precisa redefinir sua senha. Use \"Esqueci minha senha\"."}
	case errors.Is(err, auth.ErrUnauthenticated):
		return &httpError{status: 401, code: "unauthenticated", message: "Faça login para continuar."}
	case errors.Is(err, auth.ErrUserNotFound), errors.Is(err, stats.ErrUserNotFound),
		errors.Is(err, match.ErrNotFound), errors.Is(err, admin.ErrNotFound):
		return errNotFoundRoute
	case errors.Is(err, pagination.ErrInvalidCursor):
		return invalidField("cursor", "cursor inválido")
	case errors.Is(err, admin.ErrInvalidDomain):
		return invalidField("domain", "domínio inválido")
	case errors.Is(err, admin.ErrSelfAction):
		return conflict("Você não pode aplicar esta ação à sua própria conta.")
	case errors.Is(err, admin.ErrLastAdmin):
		return conflict("É preciso manter ao menos um administrador ativo.")
	case errors.Is(err, admin.ErrLastDomain):
		return conflict("É preciso manter ao menos um domínio permitido.")
	case errors.Is(err, match.ErrMatchClosed):
		return conflict("A partida não está em andamento.")
	case errors.Is(err, replay.ErrIncompatibleVersion), errors.As(err, &corrupt):
		return conflict("O replay desta partida não está disponível.")
	}
	return errInternal
}

type errorBody struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) writeError(w http.ResponseWriter, r *http.Request, err error) {
	he := classify(err)
	if he.status >= 500 {
		s.log.ErrorContext(r.Context(), "erro interno", "request_id", requestIDFrom(r.Context()),
			"method", r.Method, "path", r.URL.Path, "err", err)
	}
	writeJSON(w, he.status, errorBody{Error: errorPayload{Code: he.code, Message: he.message, Fields: he.fields}})
}

// decodeJSON exige application/json, limita o corpo a 16 KB, rejeita campos
// desconhecidos e qualquer coisa depois do objeto.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mediaType != "application/json" {
		return errUnsupportedMedia
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return errBodyTooLarge
		}
		return errInvalidJSON
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return errInvalidJSON
	}
	return nil
}
```

- [ ] **Step 5: Implementar `middleware.go`**

```go
package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/netip"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/auth"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/ratelimit"
)

type ctxKey int

const (
	requestIDKey ctxKey = iota
	principalKey
)

func requestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

func principalFrom(ctx context.Context) (auth.Principal, bool) {
	p, ok := ctx.Value(principalKey).(auth.Principal)
	return p, ok
}

func (s *Server) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := make([]byte, 8)
		_, _ = rand.Read(raw)
		id := hex.EncodeToString(raw)
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			if rec == http.ErrAbortHandler {
				panic(rec)
			}
			s.log.ErrorContext(r.Context(), "panic na requisição", "request_id", requestIDFrom(r.Context()),
				"panic", fmt.Sprint(rec), "stack", string(debug.Stack()))
			s.writeError(w, r, errInternal)
		}()
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// accessLog registra método, caminho, status e duração. Nunca registra corpo nem cookies.
func (s *Server) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.log.InfoContext(r.Context(), "http", "method", r.Method, "path", r.URL.Path, "status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(), "request_id", requestIDFrom(r.Context()))
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		if isAPI(r) {
			h.Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func isAPI(r *http.Request) bool { return strings.HasPrefix(r.URL.Path, "/api/") }

func isMutating(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	return true
}

// session coloca o Principal no contexto. Cookie inválido é limpo e a requisição segue anônima.
func (s *Server) session(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookies := s.deps.Auth.Cookies()
		token := cookies.Read(r)
		if !isAPI(r) || token == "" {
			next.ServeHTTP(w, r)
			return
		}
		p, err := s.deps.Auth.Authenticate(r.Context(), token)
		switch {
		case err == nil:
			r = r.WithContext(context.WithValue(r.Context(), principalKey, p))
		case errors.Is(err, auth.ErrUnauthenticated):
			cookies.Clear(w)
		default:
			s.writeError(w, r, err)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) originCheck(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isAPI(r) && isMutating(r.Method) && !auth.CheckOrigin(r, s.deps.Config.AppOrigin) {
			s.writeError(w, r, errForbiddenOrigin)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type authedHandler func(http.ResponseWriter, *http.Request, auth.Principal)

func (s *Server) requireAuth(h authedHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := principalFrom(r.Context())
		if !ok {
			s.writeError(w, r, auth.ErrUnauthenticated)
			return
		}
		h(w, r, p)
	})
}

func (s *Server) requireAdmin(h authedHandler) http.Handler {
	return s.requireAuth(func(w http.ResponseWriter, r *http.Request, p auth.Principal) {
		if !p.IsAdmin {
			s.writeError(w, r, errForbidden)
			return
		}
		h(w, r, p)
	})
}

// clientIP usa RemoteAddr; com TRUST_PROXY, o último endereço válido de X-Forwarded-For.
func (s *Server) clientIP(r *http.Request) string {
	if s.deps.Config.TrustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			if addr, err := netip.ParseAddr(strings.TrimSpace(parts[len(parts)-1])); err == nil {
				return addr.String()
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) sessionMeta(r *http.Request) auth.SessionMeta {
	return auth.SessionMeta{UserAgent: r.UserAgent(), IP: s.clientIP(r)}
}

func (s *Server) writeTooMany(w http.ResponseWriter, r *http.Request, retry time.Duration) {
	w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retry.Seconds()))))
	s.writeError(w, r, errTooMany)
}

func (s *Server) allow(w http.ResponseWriter, r *http.Request, l *ratelimit.Limiter, key string) bool {
	ok, retry := l.Allow(key)
	if !ok {
		s.writeTooMany(w, r, retry)
	}
	return ok
}
```

- [ ] **Step 6: Implementar `static.go` e `server.go`**

`backend/internal/httpapi/static.go`:

```go
package httpapi

import (
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
)

const staticCSP = "default-src 'self'; connect-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'"

// spaHandler serve o build do frontend e devolve index.html para rotas do cliente.
func spaHandler(dir string) http.Handler {
	fsys := os.DirFS(dir)
	files := http.FileServerFS(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", staticCSP)
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		if info, err := fs.Stat(fsys, name); err != nil || info.IsDir() {
			http.ServeFileFS(w, r, fsys, "index.html")
			return
		}
		files.ServeHTTP(w, r)
	})
}
```

`backend/internal/httpapi/server.go`:

```go
// Package httpapi expõe a API JSON de contas, estatísticas e administração.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/admin"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/auth"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/config"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/match"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/ratelimit"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/stats"
)

type Deps struct {
	Config  config.Config
	Auth    *auth.Service
	Stats   *stats.Reader
	Matches *match.Store
	Admin   *admin.Service
	Pool    *pgxpool.Pool
	Logger  *slog.Logger
	Now     func() time.Time
}

type Server struct {
	deps          Deps
	log           *slog.Logger
	mux           *http.ServeMux
	handler       http.Handler
	static        http.Handler // build do frontend; nil sem STATIC_DIR
	loginIP       *ratelimit.Limiter
	loginLockout  *ratelimit.Lockout
	registerIP    *ratelimit.Limiter
	recoveryIP    *ratelimit.Limiter
	recoveryEmail *ratelimit.Limiter
	tokenIP       *ratelimit.Limiter
}

func New(d Deps) *Server {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	s := &Server{
		deps: d, log: d.Logger, mux: http.NewServeMux(),
		loginIP:       ratelimit.NewLimiter(10, time.Minute, d.Now),
		loginLockout:  ratelimit.NewLockout(5, 15*time.Minute, 15*time.Minute, d.Now),
		registerIP:    ratelimit.NewLimiter(5, time.Hour, d.Now),
		recoveryIP:    ratelimit.NewLimiter(5, time.Hour, d.Now),
		recoveryEmail: ratelimit.NewLimiter(3, time.Hour, d.Now),
		tokenIP:       ratelimit.NewLimiter(20, time.Hour, d.Now),
	}
	s.registerAuthRoutes()
	s.registerReadRoutes()
	s.registerAdminRoutes()
	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	if d.Config.StaticDir != "" {
		s.static = spaHandler(d.Config.StaticDir)
	}
	// Um único padrão "/" evita o conflito do ServeMux entre "GET /" (SPA) e "/api/" (404 JSON).
	s.mux.HandleFunc("/", s.handleFallback)
	s.handler = s.requestID(s.recoverer(s.accessLog(securityHeaders(s.session(s.originCheck(s.mux))))))
	return s
}

func (s *Server) Handler() http.Handler { return s.handler }

// handleFallback responde o que nenhuma rota tratou: SPA para GET fora de /api, 404 JSON no resto.
func (s *Server) handleFallback(w http.ResponseWriter, r *http.Request) {
	isRead := r.Method == http.MethodGet || r.Method == http.MethodHead
	if s.static != nil && isRead && !isAPI(r) {
		s.static.ServeHTTP(w, r)
		return
	}
	s.writeError(w, r, errNotFoundRoute)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.deps.Pool.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "indisponível"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// RunMaintenance roda até ctx terminar: varre os rate limits e limpa sessões e tokens vencidos.
func (s *Server) RunMaintenance(ctx context.Context) {
	sweep := time.NewTicker(5 * time.Minute)
	cleanup := time.NewTicker(time.Hour)
	defer sweep.Stop()
	defer cleanup.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-sweep.C:
			for _, l := range []*ratelimit.Limiter{s.loginIP, s.registerIP, s.recoveryIP, s.recoveryEmail, s.tokenIP} {
				l.Sweep()
			}
			s.loginLockout.Sweep()
		case <-cleanup.C:
			sessions, tokens, err := s.deps.Auth.CleanupExpired(ctx)
			if err != nil {
				s.log.ErrorContext(ctx, "falha na limpeza de sessões", "err", err)
				continue
			}
			s.log.InfoContext(ctx, "limpeza concluída", "sessions", sessions, "tokens", tokens)
		}
	}
}
```

Para compilar já nesta tarefa, crie stubs vazios que as Tarefas 18–20 substituem:

`backend/internal/httpapi/auth_handlers.go`:

```go
package httpapi

func (s *Server) registerAuthRoutes() {}
```

`backend/internal/httpapi/read_handlers.go`:

```go
package httpapi

func (s *Server) registerReadRoutes() {}
```

`backend/internal/httpapi/admin_handlers.go`:

```go
package httpapi

func (s *Server) registerAdminRoutes() {}
```

- [ ] **Step 7: Rodar e ver passar**

Run: `cd backend && go test ./internal/httpapi/ -race -v`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/httpapi
git commit -m "feat(httpapi): adiciona servidor HTTP com middlewares de segurança e erros JSON"
```

---

### Tarefa 18: `httpapi` — rotas de autenticação e conta

**Files:**
- Modify: `backend/internal/httpapi/auth_handlers.go` (substitui o stub)
- Modify: `backend/internal/httpapi/harness_test.go` (acrescenta `signup`, `makeAdmin`, `userID`)
- Test: `backend/internal/httpapi/auth_handlers_test.go`

**Interfaces:**
- Consumes: métodos de `auth.Service` das Tarefas 12–13; middlewares da Tarefa 17.
- Produces: rotas `POST /api/auth/register|verify-email|resend-verification|login|logout|logout-all|password/forgot|password/reset`, `GET/PATCH /api/me`, `POST /api/me/password`. Respostas: login e `/api/me` devolvem `{"user": {id, username, email, displayName, isAdmin, createdAt}}`; `register`, `resend-verification` e `forgot` devolvem `202 {"message": ...}` idêntico em todos os casos; demais ações `204`.
- Produces (testes): `func (h *harness) signup(username string) *client` (cadastra, verifica e loga); `func (h *harness) makeAdmin(username string)`; `func (c *client) userID() string`; `const password = "cavalo-bateria-grampo"`.

- [ ] **Step 1: Acrescentar helpers ao harness**

Ao fim de `backend/internal/httpapi/harness_test.go`:

```go
const password = "cavalo-bateria-grampo"

// signup cadastra, verifica o e-mail e faz login; devolve o cliente já autenticado.
func (h *harness) signup(username string) *client {
	h.t.Helper()
	c := h.client()
	email := username + "@empresa.com"
	r := c.post("/api/auth/register", map[string]string{"username": username, "email": email, "displayName": "Nome " + username, "password": password})
	if r.status != http.StatusAccepted {
		h.t.Fatalf("cadastro de %s: %d %s", username, r.status, r.body)
	}
	if r := c.post("/api/auth/verify-email", map[string]string{"token": h.mail.LastToken(email)}); r.status != http.StatusNoContent {
		h.t.Fatalf("verificação de %s: %d %s", username, r.status, r.body)
	}
	if r := c.post("/api/auth/login", map[string]string{"login": username, "password": password}); r.status != http.StatusOK {
		h.t.Fatalf("login de %s: %d %s", username, r.status, r.body)
	}
	return c
}

func (h *harness) makeAdmin(username string) {
	h.t.Helper()
	if _, err := h.pool.Exec(context.Background(), "UPDATE users SET is_admin = true WHERE username = $1", username); err != nil {
		h.t.Fatal(err)
	}
}

func (c *client) userID() string {
	c.h.t.Helper()
	var body struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	c.get("/api/me").decode(c.h.t, &body)
	return body.User.ID
}
```

- [ ] **Step 2: Escrever os testes que falham**

`backend/internal/httpapi/auth_handlers_test.go`:

```go
package httpapi_test

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
	"time"
)

func registerBody(username, email string) map[string]string {
	return map[string]string{"username": username, "email": email, "displayName": "Nome", "password": password}
}

func TestFullAccountFlow(t *testing.T) {
	h := newHarness(t)
	c := h.signup("ana")
	var me struct {
		User struct {
			Username, Email, DisplayName string
			IsAdmin                      bool
		} `json:"user"`
	}
	r := c.get("/api/me")
	r.decode(t, &me)
	if r.status != 200 || me.User.Username != "ana" || me.User.Email != "ana@empresa.com" || me.User.IsAdmin {
		t.Fatalf("me = %d %s", r.status, r.body)
	}
	r = c.do(http.MethodPatch, "/api/me", map[string]string{"displayName": "Ana Clara"})
	r.decode(t, &me)
	if r.status != 200 || me.User.DisplayName != "Ana Clara" {
		t.Fatalf("patch = %d %s", r.status, r.body)
	}
	if r := c.post("/api/auth/logout", nil); r.status != http.StatusNoContent {
		t.Fatalf("logout = %d", r.status)
	}
	expect(t, c.get("/api/me"), http.StatusUnauthorized, "unauthenticated")
}

func TestLoginSetsSecureCookieAttributes(t *testing.T) {
	h := newHarness(t)
	c := h.client()
	c.post("/api/auth/register", registerBody("ana", "ana@empresa.com"))
	c.post("/api/auth/verify-email", map[string]string{"token": h.mail.LastToken("ana@empresa.com")})
	r := c.post("/api/auth/login", map[string]string{"login": "ana@empresa.com", "password": password})
	cookie := r.header.Get("Set-Cookie")
	for _, want := range []string{"truco_session=", "HttpOnly", "SameSite=Lax", "Path=/", "Max-Age=7776000"} {
		if !strings.Contains(cookie, want) {
			t.Fatalf("Set-Cookie sem %q: %s", want, cookie)
		}
	}
}

func TestInvalidCookieIsCleared(t *testing.T) {
	h := newHarness(t)
	r := h.client().get("/api/me", "Cookie", "truco_session=forjado")
	expect(t, r, http.StatusUnauthorized, "unauthenticated")
	if !strings.Contains(r.header.Get("Set-Cookie"), "Max-Age=0") {
		t.Fatalf("cookie inválido não foi limpo: %v", r.header)
	}
}

func TestEnumerationResistantResponses(t *testing.T) {
	h := newHarness(t)
	h.signup("ana")
	fresh := h.client().post("/api/auth/register", registerBody("bia", "bia@empresa.com"))
	existing := h.client().post("/api/auth/register", registerBody("outra", "ana@empresa.com"))
	if fresh.status != existing.status || !bytes.Equal(fresh.body, existing.body) {
		t.Fatalf("cadastro revela e-mail existente: %d %s vs %d %s", fresh.status, fresh.body, existing.status, existing.body)
	}
	known := h.client().post("/api/auth/password/forgot", map[string]string{"email": "ana@empresa.com"})
	unknown := h.client().post("/api/auth/password/forgot", map[string]string{"email": "ninguem@empresa.com"})
	if known.status != http.StatusAccepted || known.status != unknown.status || !bytes.Equal(known.body, unknown.body) {
		t.Fatal("forgot revela e-mail existente")
	}
	resendKnown := h.client().post("/api/auth/resend-verification", map[string]string{"email": "bia@empresa.com"})
	resendUnknown := h.client().post("/api/auth/resend-verification", map[string]string{"email": "zzz@empresa.com"})
	if resendKnown.status != http.StatusAccepted || !bytes.Equal(resendKnown.body, resendUnknown.body) {
		t.Fatal("resend revela e-mail existente")
	}
	wrongUser := h.client().post("/api/auth/login", map[string]string{"login": "ninguem", "password": password})
	wrongPass := h.client().post("/api/auth/login", map[string]string{"login": "ana", "password": "senha-errada-123"})
	if !bytes.Equal(wrongUser.body, wrongPass.body) {
		t.Fatal("login diferencia usuário inexistente de senha errada")
	}
	expect(t, wrongPass, http.StatusUnauthorized, "invalid_credentials")
}

func TestRegisterErrors(t *testing.T) {
	h := newHarness(t)
	h.signup("ana")
	c := h.client()
	r := c.post("/api/auth/register", map[string]string{"username": "x", "email": "y", "displayName": "", "password": "z"})
	expect(t, r, http.StatusBadRequest, "validation_failed")
	var body struct {
		Error struct {
			Fields map[string]string `json:"fields"`
		} `json:"error"`
	}
	r.decode(t, &body)
	if len(body.Error.Fields) != 4 {
		t.Fatalf("fields = %v", body.Error.Fields)
	}
	expect(t, c.post("/api/auth/register", registerBody("bia", "bia@gmail.com")), http.StatusForbidden, "email_domain_not_allowed")
	expect(t, c.post("/api/auth/register", registerBody("ana", "outra@empresa.com")), http.StatusConflict, "username_taken")
	expect(t, c.post("/api/auth/register", rawBody(`{"username":"a","seat":2}`)), http.StatusBadRequest, "invalid_request")
	expect(t, c.post("/api/auth/verify-email", map[string]string{"token": "inexistente"}), http.StatusBadRequest, "invalid_token")
}

func TestLoginRequiresVerifiedEmail(t *testing.T) {
	h := newHarness(t)
	c := h.client()
	c.post("/api/auth/register", registerBody("ana", "ana@empresa.com"))
	expect(t, c.post("/api/auth/login", map[string]string{"login": "ana", "password": password}), http.StatusForbidden, "email_not_verified")
}

func TestLoginLockoutPerIdentifier(t *testing.T) {
	h := newHarness(t)
	h.signup("ana")
	for range 5 {
		expect(t, h.client().post("/api/auth/login", map[string]string{"login": "ana", "password": "senha-errada-123"}), http.StatusUnauthorized, "invalid_credentials")
	}
	r := h.client().post("/api/auth/login", map[string]string{"login": " ANA ", "password": password})
	expect(t, r, http.StatusTooManyRequests, "too_many_attempts")
	if r.header.Get("Retry-After") != "900" {
		t.Fatalf("Retry-After = %q", r.header.Get("Retry-After"))
	}
	h.clock.Advance(15 * time.Minute)
	if r := h.client().post("/api/auth/login", map[string]string{"login": "ana", "password": password}); r.status != http.StatusOK {
		t.Fatalf("após o bloqueio: %d %s", r.status, r.body)
	}
}

func TestPerIPRateLimits(t *testing.T) {
	h := newHarness(t)
	c := h.client()
	for i := range 10 {
		c.post("/api/auth/login", map[string]string{"login": "ninguem" + string(rune('a'+i)), "password": "x"})
	}
	expect(t, c.post("/api/auth/login", map[string]string{"login": "outro", "password": "x"}), http.StatusTooManyRequests, "too_many_attempts")

	reg := h.client()
	for i := range 5 {
		reg.post("/api/auth/register", registerBody("user"+string(rune('a'+i)), "u"+string(rune('a'+i))+"@empresa.com"))
	}
	expect(t, reg.post("/api/auth/register", registerBody("userz", "uz@empresa.com")), http.StatusTooManyRequests, "too_many_attempts")

	for range 3 {
		h.client().post("/api/auth/password/forgot", map[string]string{"email": "alvo@empresa.com"})
	}
	expect(t, h.client().post("/api/auth/password/forgot", map[string]string{"email": " ALVO@empresa.com"}), http.StatusTooManyRequests, "too_many_attempts")
}

func TestResetPasswordFlow(t *testing.T) {
	h := newHarness(t)
	ana := h.signup("ana")
	h.client().post("/api/auth/password/forgot", map[string]string{"email": "ana@empresa.com"})
	token := h.mail.LastToken("ana@empresa.com")
	other := h.client()
	expect(t, other.post("/api/auth/password/reset", map[string]string{"token": token, "password": "curta"}), http.StatusBadRequest, "validation_failed")
	if r := other.post("/api/auth/password/reset", map[string]string{"token": token, "password": "nova-senha-segura-1"}); r.status != http.StatusNoContent {
		t.Fatalf("reset = %d %s", r.status, r.body)
	}
	expect(t, ana.get("/api/me"), http.StatusUnauthorized, "unauthenticated")
	if r := other.post("/api/auth/login", map[string]string{"login": "ana", "password": "nova-senha-segura-1"}); r.status != http.StatusOK {
		t.Fatalf("login com nova senha = %d", r.status)
	}
}

func TestChangePasswordAndLogoutAll(t *testing.T) {
	h := newHarness(t)
	first := h.signup("ana")
	second := h.client()
	second.post("/api/auth/login", map[string]string{"login": "ana", "password": password})
	expect(t, first.post("/api/me/password", map[string]string{"currentPassword": "errada-errada-1", "newPassword": "nova-senha-segura-1"}), http.StatusUnauthorized, "invalid_credentials")
	r := first.post("/api/me/password", map[string]string{"currentPassword": password, "newPassword": "nova-senha-segura-1"})
	if r.status != http.StatusNoContent || !strings.Contains(r.header.Get("Set-Cookie"), "truco_session=") {
		t.Fatalf("troca = %d %v", r.status, r.header)
	}
	if r := first.get("/api/me"); r.status != 200 {
		t.Fatal("sessão rotacionada deve continuar valendo no mesmo navegador")
	}
	expect(t, second.get("/api/me"), http.StatusUnauthorized, "unauthenticated")

	second.post("/api/auth/login", map[string]string{"login": "ana", "password": "nova-senha-segura-1"})
	if r := first.post("/api/auth/logout-all", nil); r.status != http.StatusNoContent {
		t.Fatalf("logout-all = %d", r.status)
	}
	expect(t, first.get("/api/me"), http.StatusUnauthorized, "unauthenticated")
	expect(t, second.get("/api/me"), http.StatusUnauthorized, "unauthenticated")
	expect(t, h.client().post("/api/auth/logout-all", nil), http.StatusUnauthorized, "unauthenticated")
}

func TestPasswordResetRequiredBlocksLogin(t *testing.T) {
	h := newHarness(t)
	h.signup("ana")
	h.makeAdmin("ana") // nem administrador escapa da redefinição obrigatória
	_, _ = h.pool.Exec(t.Context(), "UPDATE users SET must_reset_password = true WHERE username = 'ana'")
	expect(t, h.client().post("/api/auth/login", map[string]string{"login": "ana", "password": password}), http.StatusForbidden, "password_reset_required")
}
```

- [ ] **Step 3: Rodar e ver falhar**

Run: `cd backend && go test ./internal/httpapi/ -run 'Account|Cookie|Enumeration|Register|Login|RateLimit|Reset|ChangePassword|PasswordReset'`
Expected: FAIL (rotas inexistentes respondem 404).

- [ ] **Step 4: Implementar**

`backend/internal/httpapi/auth_handlers.go` (substitui o stub):

```go
package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/auth"
)

// Mensagens idênticas em todos os casos para não revelar se o e-mail existe.
var (
	registerAccepted = map[string]string{"message": "Se os dados estiverem corretos, você receberá um e-mail para confirmar a conta."}
	emailAccepted    = map[string]string{"message": "Se houver uma conta com este e-mail, enviaremos as instruções."}
)

func (s *Server) registerAuthRoutes() {
	s.mux.HandleFunc("POST /api/auth/register", s.handleRegister)
	s.mux.HandleFunc("POST /api/auth/verify-email", s.handleVerifyEmail)
	s.mux.HandleFunc("POST /api/auth/resend-verification", s.handleResendVerification)
	s.mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	s.mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	s.mux.Handle("POST /api/auth/logout-all", s.requireAuth(s.handleLogoutAll))
	s.mux.HandleFunc("POST /api/auth/password/forgot", s.handleForgotPassword)
	s.mux.HandleFunc("POST /api/auth/password/reset", s.handleResetPassword)
	s.mux.Handle("GET /api/me", s.requireAuth(s.handleMe))
	s.mux.Handle("PATCH /api/me", s.requireAuth(s.handleUpdateMe))
	s.mux.Handle("POST /api/me/password", s.requireAuth(s.handleChangePassword))
}

type registerRequest struct {
	Username    string `json:"username"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Password    string `json:"password"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if !s.allow(w, r, s.registerIP, s.clientIP(r)) {
		return
	}
	var req registerRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, r, err)
		return
	}
	err := s.deps.Auth.Register(r.Context(), auth.RegisterInput{Username: req.Username, Email: req.Email, DisplayName: req.DisplayName, Password: req.Password})
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, registerAccepted)
}

type tokenRequest struct {
	Token string `json:"token"`
}

func (s *Server) handleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	if !s.allow(w, r, s.tokenIP, s.clientIP(r)) {
		return
	}
	var req tokenRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, r, err)
		return
	}
	if err := s.deps.Auth.VerifyEmail(r.Context(), req.Token); err != nil {
		s.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type emailRequest struct {
	Email string `json:"email"`
}

// emailAction aplica os limites por IP e por e-mail e responde sempre 202.
func (s *Server) emailAction(w http.ResponseWriter, r *http.Request, action func(email string) error) {
	if !s.allow(w, r, s.recoveryIP, s.clientIP(r)) {
		return
	}
	var req emailRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, r, err)
		return
	}
	if !s.allow(w, r, s.recoveryEmail, auth.NormalizeEmail(req.Email)) {
		return
	}
	if err := action(req.Email); err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, emailAccepted)
}

func (s *Server) handleResendVerification(w http.ResponseWriter, r *http.Request) {
	s.emailAction(w, r, func(email string) error { return s.deps.Auth.ResendVerification(r.Context(), email) })
}

func (s *Server) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	s.emailAction(w, r, func(email string) error { return s.deps.Auth.ForgotPassword(r.Context(), email) })
}

type loginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !s.allow(w, r, s.loginIP, s.clientIP(r)) {
		return
	}
	var req loginRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, r, err)
		return
	}
	key := strings.ToLower(strings.TrimSpace(req.Login))
	if locked, retry := s.loginLockout.Locked(key); locked {
		s.writeTooMany(w, r, retry)
		return
	}
	res, err := s.deps.Auth.Login(r.Context(), req.Login, req.Password, s.sessionMeta(r))
	if errors.Is(err, auth.ErrInvalidCredentials) {
		s.loginLockout.Fail(key)
	}
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	s.loginLockout.Reset(key)
	s.deps.Auth.Cookies().Set(w, res.Token)
	writeJSON(w, http.StatusOK, map[string]auth.User{"user": res.User})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookies := s.deps.Auth.Cookies()
	if err := s.deps.Auth.Logout(r.Context(), cookies.Read(r)); err != nil {
		s.writeError(w, r, err)
		return
	}
	cookies.Clear(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleLogoutAll(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	if err := s.deps.Auth.LogoutAll(r.Context(), p.UserID); err != nil {
		s.writeError(w, r, err)
		return
	}
	s.deps.Auth.Cookies().Clear(w)
	w.WriteHeader(http.StatusNoContent)
}

type resetRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	if !s.allow(w, r, s.tokenIP, s.clientIP(r)) {
		return
	}
	var req resetRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, r, err)
		return
	}
	if err := s.deps.Auth.ResetPassword(r.Context(), req.Token, req.Password); err != nil {
		s.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	user, err := s.deps.Auth.GetUser(r.Context(), p.UserID)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]auth.User{"user": user})
}

type updateMeRequest struct {
	DisplayName string `json:"displayName"`
}

func (s *Server) handleUpdateMe(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var req updateMeRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, r, err)
		return
	}
	user, err := s.deps.Auth.UpdateDisplayName(r.Context(), p.UserID, req.DisplayName)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]auth.User{"user": user})
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var req changePasswordRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, r, err)
		return
	}
	token, err := s.deps.Auth.ChangePassword(r.Context(), p, req.CurrentPassword, req.NewPassword, s.sessionMeta(r))
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	s.deps.Auth.Cookies().Set(w, token)
	w.WriteHeader(http.StatusNoContent)
}
```

> A troca de senha com a senha atual errada **não** alimenta o bloqueio de login: ela exige sessão válida, então não é vetor de força bruta anônima.

- [ ] **Step 5: Rodar e ver passar**

Run: `cd backend && go test ./internal/httpapi/ -race -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/httpapi
git commit -m "feat(httpapi): adiciona rotas de cadastro, login, sessão e recuperação de senha"
```

---

### Tarefa 19: `httpapi` — ranking, perfis, histórico e replay

**Files:**
- Modify: `backend/internal/httpapi/read_handlers.go` (substitui o stub)
- Test: `backend/internal/httpapi/read_handlers_test.go`

**Interfaces:**
- Consumes: `stats.Reader` (Tarefa 15), `match.Store.Summary/Load` (Tarefa 14), `replay.BuildFrames` (Tarefa 2), `enginetest.RandomRecord` nos testes.
- Produces: `GET /api/ranking?limit=` → `{"items": [RankingEntry]}`; `GET /api/users/{username}` → `Profile`; `GET /api/matches?user=&cursor=` → `HistoryPage` (sem `user`, usa o próprio usuário); `GET /api/matches/{id}` → `Summary`; `GET /api/matches/{id}/replay` → `{"id", "players": [{seat, displayName}], "steps": [Frame]}`, com gzip quando o cliente aceita. Todas exigem sessão.

- [ ] **Step 1: Escrever os testes que falham**

`backend/internal/httpapi/read_handlers_test.go`:

```go
package httpapi_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/enginetest"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
)

type table struct {
	clients [4]*client
	players [4]replay.PlayerSeat
}

func (h *harness) table() table {
	h.t.Helper()
	var tb table
	for i, name := range []string{"ana", "bia", "caio", "duda"} {
		tb.clients[i] = h.signup(name)
		tb.players[i] = replay.PlayerSeat{UserID: tb.clients[i].userID(), DisplayName: name}
	}
	return tb
}

// playMatch grava uma partida; actions < 0 grava todas e finaliza.
func (h *harness) playMatch(tb table, seed uint64, actions int) (string, replay.Record) {
	h.t.Helper()
	ctx := context.Background()
	rec, _ := enginetest.RandomRecord(seed)
	id, err := h.store.Create(ctx, rec.Seed, rec.FirstDealer, tb.players)
	if err != nil {
		h.t.Fatal(err)
	}
	if actions >= 0 {
		rec.Actions = rec.Actions[:actions]
	}
	for _, a := range rec.Actions {
		if err := h.store.AppendAction(ctx, id, a.Ply, a.Action, a.Source); err != nil {
			h.t.Fatal(err)
		}
	}
	if actions < 0 {
		if err := h.store.Finish(ctx, id); err != nil {
			h.t.Fatal(err)
		}
	}
	return id, rec
}

func TestReadRoutesRequireSession(t *testing.T) {
	h := newHarness(t)
	anon := h.client()
	for _, path := range []string{"/api/ranking", "/api/users/ana", "/api/matches", "/api/matches/x", "/api/matches/x/replay"} {
		expect(t, anon.get(path), http.StatusUnauthorized, "unauthenticated")
	}
}

func TestRankingProfileAndHistory(t *testing.T) {
	h := newHarness(t)
	tb := h.table()
	h.playMatch(tb, 1, -1)
	h.playMatch(tb, 2, -1)
	ana := tb.clients[0]

	var ranking struct {
		Items []struct {
			Position      int    `json:"position"`
			Username      string `json:"username"`
			MatchesPlayed int    `json:"matchesPlayed"`
		} `json:"items"`
	}
	r := ana.get("/api/ranking?limit=2")
	r.decode(t, &ranking)
	if r.status != 200 || len(ranking.Items) != 2 || ranking.Items[0].Position != 1 || ranking.Items[0].MatchesPlayed != 2 {
		t.Fatalf("ranking = %d %s", r.status, r.body)
	}

	var profile struct {
		Username string `json:"username"`
		Stats    struct {
			MatchesPlayed int `json:"matchesPlayed"`
		} `json:"stats"`
	}
	r = ana.get("/api/users/bia")
	r.decode(t, &profile)
	if r.status != 200 || profile.Username != "bia" || profile.Stats.MatchesPlayed != 2 {
		t.Fatalf("perfil = %d %s", r.status, r.body)
	}
	expect(t, ana.get("/api/users/ninguem"), http.StatusNotFound, "not_found")

	var page struct {
		Items      []struct{ ID string } `json:"items"`
		NextCursor string                `json:"nextCursor"`
	}
	r = ana.get("/api/matches")
	r.decode(t, &page)
	if r.status != 200 || len(page.Items) != 2 {
		t.Fatalf("histórico próprio = %d %s", r.status, r.body)
	}
	if r := ana.get("/api/matches?user=caio"); r.status != 200 {
		t.Fatalf("histórico de outro = %d", r.status)
	}
	expect(t, ana.get("/api/matches?cursor=lixo!"), http.StatusBadRequest, "validation_failed")
	expect(t, ana.get("/api/matches?user=ninguem"), http.StatusNotFound, "not_found")
}

func TestMatchSummary(t *testing.T) {
	h := newHarness(t)
	tb := h.table()
	id, _ := h.playMatch(tb, 3, -1)
	var summary struct {
		Status  string `json:"status"`
		Players []struct {
			Seat     int    `json:"seat"`
			Username string `json:"username"`
		} `json:"players"`
		Score *[2]int `json:"score"`
	}
	r := tb.clients[0].get("/api/matches/" + id)
	r.decode(t, &summary)
	if r.status != 200 || summary.Status != "finished" || len(summary.Players) != 4 || summary.Score == nil {
		t.Fatalf("resumo = %d %s", r.status, r.body)
	}
	expect(t, tb.clients[0].get("/api/matches/00000000-0000-7000-8000-000000000000"), http.StatusNotFound, "not_found")
	expect(t, tb.clients[0].get("/api/matches/nao-e-uuid"), http.StatusNotFound, "not_found")
}

func TestReplay(t *testing.T) {
	h := newHarness(t)
	tb := h.table()
	id, rec := h.playMatch(tb, 4, -1)
	var replayBody struct {
		ID      string `json:"id"`
		Players []struct {
			Seat        int    `json:"seat"`
			DisplayName string `json:"displayName"`
		} `json:"players"`
		Steps []struct {
			Ply   int                    `json:"ply"`
			Views [4]engine.PlayerView   `json:"views"`
			Events []map[string]any      `json:"events"`
		} `json:"steps"`
	}
	r := tb.clients[0].get("/api/matches/" + id + "/replay")
	r.decode(t, &replayBody)
	if r.status != 200 || len(replayBody.Steps) != len(rec.Actions) || len(replayBody.Players) != 4 {
		t.Fatalf("replay = %d, %d passos (quer %d)", r.status, len(replayBody.Steps), len(rec.Actions))
	}
	last := replayBody.Steps[len(replayBody.Steps)-1]
	if last.Views[0].Phase != engine.PhaseGameOver {
		t.Fatalf("último passo na fase %s", last.Views[0].Phase)
	}

	gz := tb.clients[0].get("/api/matches/"+id+"/replay", "Accept-Encoding", "gzip")
	if gz.header.Get("Content-Encoding") != "gzip" {
		t.Fatalf("sem gzip: %v", gz.header)
	}
	zr, err := gzip.NewReader(bytes.NewReader(gz.body))
	if err != nil {
		t.Fatal(err)
	}
	plain, _ := io.ReadAll(zr)
	if !json.Valid(plain) {
		t.Fatal("corpo gzip não é JSON válido")
	}
}

func TestReplayUnavailableCases(t *testing.T) {
	h := newHarness(t)
	tb := h.table()
	ctx := context.Background()
	inProgress, _ := h.playMatch(tb, 5, 10)
	expect(t, tb.clients[0].get("/api/matches/"+inProgress+"/replay"), http.StatusConflict, "match_in_progress")

	corrupt, _ := h.playMatch(tb, 6, 1)
	if err := h.store.AppendAction(ctx, corrupt, 1, engine.Action{Kind: engine.ActPlayCard, Seat: 0}, replay.SourcePlayer); err != nil {
		t.Fatal(err)
	}
	if err := h.store.Abandon(ctx, corrupt, "corrupt_log"); err != nil {
		t.Fatal(err)
	}
	expect(t, tb.clients[0].get("/api/matches/"+corrupt+"/replay"), http.StatusConflict, "conflict")
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/httpapi/ -run 'Read|Ranking|MatchSummary|Replay'`
Expected: FAIL (rotas inexistentes).

- [ ] **Step 3: Implementar**

`backend/internal/httpapi/read_handlers.go` (substitui o stub):

```go
package httpapi

import (
	"compress/gzip"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/auth"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
)

func (s *Server) registerReadRoutes() {
	s.mux.Handle("GET /api/ranking", s.requireAuth(s.handleRanking))
	s.mux.Handle("GET /api/users/{username}", s.requireAuth(s.handleProfile))
	s.mux.Handle("GET /api/matches", s.requireAuth(s.handleHistory))
	s.mux.Handle("GET /api/matches/{id}", s.requireAuth(s.handleMatch))
	s.mux.Handle("GET /api/matches/{id}/replay", s.requireAuth(s.handleReplay))
}

func (s *Server) handleRanking(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	entries, err := s.deps.Stats.Ranking(r.Context(), limit)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": entries})
}

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	profile, err := s.deps.Stats.Profile(r.Context(), r.PathValue("username"))
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	username := r.URL.Query().Get("user")
	if username == "" {
		username = p.Username
	}
	page, err := s.deps.Stats.History(r.Context(), username, r.URL.Query().Get("cursor"), 0)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) handleMatch(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	summary, err := s.deps.Matches.Summary(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

type replayPlayer struct {
	Seat        int    `json:"seat"`
	DisplayName string `json:"displayName"`
}

type replayResponse struct {
	ID      string         `json:"id"`
	Players []replayPlayer `json:"players"`
	Steps   []replay.Frame `json:"steps"`
}

// handleReplay só atende partidas encerradas: durante o jogo, o replay revelaria cartas ocultas.
func (s *Server) handleReplay(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	rec, err := s.deps.Matches.Load(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	if rec.Status == replay.StatusInProgress {
		s.writeError(w, r, errMatchInProgress)
		return
	}
	frames, err := replay.BuildFrames(rec)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	body := replayResponse{ID: rec.ID, Steps: frames}
	for seat, p := range rec.Players {
		body.Players = append(body.Players, replayPlayer{Seat: seat, DisplayName: p.DisplayName})
	}
	writeJSONMaybeGzip(w, r, http.StatusOK, body)
}

func writeJSONMaybeGzip(w http.ResponseWriter, r *http.Request, status int, v any) {
	if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		writeJSON(w, status, v)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Add("Vary", "Accept-Encoding")
	w.WriteHeader(status)
	gz := gzip.NewWriter(w)
	defer gz.Close()
	_ = json.NewEncoder(gz).Encode(v)
}
```

> O cliente HTTP do Go adiciona `Accept-Encoding: gzip` sozinho e descomprime de forma transparente; por isso `TestReplay` decodifica normalmente na primeira chamada e só vê o corpo comprimido quando define o header manualmente.

- [ ] **Step 4: Rodar e ver passar**

Run: `cd backend && go test ./internal/httpapi/ -race -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/httpapi
git commit -m "feat(httpapi): adiciona rotas de ranking, perfil, histórico e replay"
```

---

### Tarefa 20: `httpapi` — rotas de administração

**Files:**
- Modify: `backend/internal/httpapi/admin_handlers.go` (substitui o stub)
- Test: `backend/internal/httpapi/admin_handlers_test.go`

**Interfaces:**
- Consumes: `admin.Service` (Tarefa 16), `requireAdmin` (Tarefa 17).
- Produces: `GET /api/admin/users?q=&cursor=` → `UserPage`; `POST /api/admin/users/{id}/disable|enable|force-password-reset` → 204; `PUT /api/admin/users/{id}/admin` `{"isAdmin": bool}` → 204; `GET /api/admin/email-domains` → `{"items": [Domain]}`; `POST /api/admin/email-domains` `{"domain"}` → 204; `DELETE /api/admin/email-domains/{domain}` → 204; `POST /api/admin/matches/{id}/abort` `{"reason"}` → 204; `GET /api/admin/audit?cursor=` → `AuditPage`.

- [ ] **Step 1: Escrever os testes que falham**

`backend/internal/httpapi/admin_handlers_test.go`:

```go
package httpapi_test

import (
	"net/http"
	"testing"
)

func TestAdminRoutesRequireAdmin(t *testing.T) {
	h := newHarness(t)
	user := h.signup("ana")
	anon := h.client()
	routes := []struct{ method, path string }{
		{http.MethodGet, "/api/admin/users"},
		{http.MethodPost, "/api/admin/users/x/disable"},
		{http.MethodPost, "/api/admin/users/x/enable"},
		{http.MethodPost, "/api/admin/users/x/force-password-reset"},
		{http.MethodPut, "/api/admin/users/x/admin"},
		{http.MethodGet, "/api/admin/email-domains"},
		{http.MethodPost, "/api/admin/email-domains"},
		{http.MethodDelete, "/api/admin/email-domains/x.com"},
		{http.MethodPost, "/api/admin/matches/x/abort"},
		{http.MethodGet, "/api/admin/audit"},
	}
	for _, rt := range routes {
		expect(t, user.do(rt.method, rt.path, nil), http.StatusForbidden, "forbidden")
		expect(t, anon.do(rt.method, rt.path, nil), http.StatusUnauthorized, "unauthenticated")
	}
}

func TestAdminUserManagement(t *testing.T) {
	h := newHarness(t)
	boss := h.signup("chefe")
	h.makeAdmin("chefe")
	ana := h.signup("ana")
	anaID := ana.userID()

	var page struct {
		Items []struct {
			Username string `json:"username"`
			IsAdmin  bool   `json:"isAdmin"`
		} `json:"items"`
	}
	r := boss.get("/api/admin/users?q=an")
	r.decode(t, &page)
	if r.status != 200 || len(page.Items) != 1 || page.Items[0].Username != "ana" {
		t.Fatalf("busca = %d %s", r.status, r.body)
	}

	if r := boss.post("/api/admin/users/"+anaID+"/disable", nil); r.status != http.StatusNoContent {
		t.Fatalf("disable = %d %s", r.status, r.body)
	}
	expect(t, ana.get("/api/me"), http.StatusUnauthorized, "unauthenticated")
	expect(t, h.client().post("/api/auth/login", map[string]string{"login": "ana", "password": password}), http.StatusForbidden, "account_disabled")
	if r := boss.post("/api/admin/users/"+anaID+"/enable", nil); r.status != http.StatusNoContent {
		t.Fatalf("enable = %d", r.status)
	}

	if r := boss.post("/api/admin/users/"+anaID+"/force-password-reset", nil); r.status != http.StatusNoContent {
		t.Fatalf("force reset = %d", r.status)
	}
	expect(t, h.client().post("/api/auth/login", map[string]string{"login": "ana", "password": password}), http.StatusForbidden, "password_reset_required")

	if r := boss.do(http.MethodPut, "/api/admin/users/"+anaID+"/admin", map[string]bool{"isAdmin": true}); r.status != http.StatusNoContent {
		t.Fatalf("set admin = %d %s", r.status, r.body)
	}
	expect(t, boss.do(http.MethodPut, "/api/admin/users/"+anaID+"/admin", map[string]any{}), http.StatusBadRequest, "invalid_request")
	expect(t, boss.post("/api/admin/users/"+boss.userID()+"/disable", nil), http.StatusConflict, "conflict")
	expect(t, boss.post("/api/admin/users/nao-e-uuid/disable", nil), http.StatusNotFound, "not_found")
}

func TestAdminDomains(t *testing.T) {
	h := newHarness(t)
	boss := h.signup("chefe")
	h.makeAdmin("chefe")
	expect(t, boss.post("/api/admin/email-domains", map[string]string{"domain": "sem-ponto"}), http.StatusBadRequest, "validation_failed")
	if r := boss.post("/api/admin/email-domains", map[string]string{"domain": "filial.com.br"}); r.status != http.StatusNoContent {
		t.Fatalf("add = %d %s", r.status, r.body)
	}
	var domains struct {
		Items []struct {
			Domain string `json:"domain"`
		} `json:"items"`
	}
	boss.get("/api/admin/email-domains").decode(t, &domains)
	if len(domains.Items) != 2 {
		t.Fatalf("domínios = %+v", domains)
	}
	if r := h.client().post("/api/auth/register", registerBody("bia", "bia@filial.com.br")); r.status != http.StatusAccepted {
		t.Fatalf("domínio novo deve aceitar cadastro: %d %s", r.status, r.body)
	}
	if r := boss.do(http.MethodDelete, "/api/admin/email-domains/empresa.com", nil); r.status != http.StatusNoContent {
		t.Fatalf("remove = %d", r.status)
	}
	expect(t, boss.do(http.MethodDelete, "/api/admin/email-domains/filial.com.br", nil), http.StatusConflict, "conflict")
}

func TestAdminAbortMatchAndAudit(t *testing.T) {
	h := newHarness(t)
	tb := h.table()
	h.makeAdmin("ana")
	boss := tb.clients[0]
	id, _ := h.playMatch(tb, 7, 10)
	if r := boss.post("/api/admin/matches/"+id+"/abort", map[string]string{"reason": "teste"}); r.status != http.StatusNoContent {
		t.Fatalf("abort = %d %s", r.status, r.body)
	}
	expect(t, boss.post("/api/admin/matches/"+id+"/abort", map[string]string{"reason": "de novo"}), http.StatusConflict, "conflict")
	if r := boss.get("/api/matches/" + id + "/replay"); r.status != http.StatusOK {
		t.Fatalf("replay de partida abortada = %d", r.status)
	}
	var audit struct {
		Items []struct {
			Action string `json:"action"`
		} `json:"items"`
	}
	boss.get("/api/admin/audit").decode(t, &audit)
	if len(audit.Items) != 1 || audit.Items[0].Action != "match.abort" {
		t.Fatalf("auditoria = %+v", audit)
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/httpapi/ -run Admin`
Expected: FAIL.

- [ ] **Step 3: Implementar**

`backend/internal/httpapi/admin_handlers.go` (substitui o stub):

```go
package httpapi

import (
	"context"
	"net/http"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/auth"
)

func (s *Server) registerAdminRoutes() {
	s.mux.Handle("GET /api/admin/users", s.requireAdmin(s.handleAdminListUsers))
	s.mux.Handle("POST /api/admin/users/{id}/disable", s.requireAdmin(s.userAction(s.deps.Admin.DisableUser)))
	s.mux.Handle("POST /api/admin/users/{id}/enable", s.requireAdmin(s.userAction(s.deps.Admin.EnableUser)))
	s.mux.Handle("POST /api/admin/users/{id}/force-password-reset", s.requireAdmin(s.userAction(s.deps.Admin.ForcePasswordReset)))
	s.mux.Handle("PUT /api/admin/users/{id}/admin", s.requireAdmin(s.handleAdminSetAdmin))
	s.mux.Handle("GET /api/admin/email-domains", s.requireAdmin(s.handleAdminListDomains))
	s.mux.Handle("POST /api/admin/email-domains", s.requireAdmin(s.handleAdminAddDomain))
	s.mux.Handle("DELETE /api/admin/email-domains/{domain}", s.requireAdmin(s.handleAdminRemoveDomain))
	s.mux.Handle("POST /api/admin/matches/{id}/abort", s.requireAdmin(s.handleAdminAbortMatch))
	s.mux.Handle("GET /api/admin/audit", s.requireAdmin(s.handleAdminAudit))
}

func noContent(s *Server, w http.ResponseWriter, r *http.Request, err error) {
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) userAction(action func(context.Context, auth.Principal, string) error) authedHandler {
	return func(w http.ResponseWriter, r *http.Request, p auth.Principal) {
		noContent(s, w, r, action(r.Context(), p, r.PathValue("id")))
	}
}

func (s *Server) handleAdminListUsers(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	page, err := s.deps.Admin.ListUsers(r.Context(), r.URL.Query().Get("q"), r.URL.Query().Get("cursor"))
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

type setAdminRequest struct {
	IsAdmin *bool `json:"isAdmin"`
}

func (s *Server) handleAdminSetAdmin(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var req setAdminRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, r, err)
		return
	}
	if req.IsAdmin == nil {
		s.writeError(w, r, errInvalidJSON)
		return
	}
	noContent(s, w, r, s.deps.Admin.SetAdmin(r.Context(), p, r.PathValue("id"), *req.IsAdmin))
}

func (s *Server) handleAdminListDomains(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	domains, err := s.deps.Admin.ListDomains(r.Context())
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": domains})
}

type domainRequest struct {
	Domain string `json:"domain"`
}

func (s *Server) handleAdminAddDomain(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var req domainRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, r, err)
		return
	}
	noContent(s, w, r, s.deps.Admin.AddDomain(r.Context(), p, req.Domain))
}

func (s *Server) handleAdminRemoveDomain(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	noContent(s, w, r, s.deps.Admin.RemoveDomain(r.Context(), p, r.PathValue("domain")))
}

type abortRequest struct {
	Reason string `json:"reason"`
}

func (s *Server) handleAdminAbortMatch(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var req abortRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, r, err)
		return
	}
	noContent(s, w, r, s.deps.Admin.AbortMatch(r.Context(), p, r.PathValue("id"), req.Reason))
}

func (s *Server) handleAdminAudit(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	page, err := s.deps.Admin.ListAudit(r.Context(), r.URL.Query().Get("cursor"))
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}
```

> Em `TestAdminRoutesRequireAdmin`, as rotas com corpo recebem `nil`: o `requireAdmin` responde antes do `decodeJSON`, então 401/403 vêm primeiro — é o comportamento desejado (não revelar validação a quem não pode usar a rota).

- [ ] **Step 4: Rodar e ver passar**

Run: `cd backend && go test ./internal/httpapi/ -race -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/httpapi
git commit -m "feat(httpapi): adiciona rotas de administração com auditoria"
```

---
### Tarefa 21: `cmd/server` — subcomandos, servidor e desligamento gracioso

**Files:**
- Create: `backend/cmd/server/main.go`
- Test: `backend/cmd/server/main_test.go`
- Modify: `Makefile` (alvos `run` e `migrate`)

**Interfaces:**
- Consumes: `config.Load`, `db.Open/Migrate`, `auth.NewService`, `mail.NewResend/NewLog`, `match.NewStore`, `stats.NewReader`, `admin.NewService/PromoteAdmin`, `httpapi.New`.
- Produces: binário com subcomandos `serve` (padrão), `migrate`, `promote-admin <email|username>`, `recompute-stats`; `func run(ctx context.Context, args []string, getenv func(string) string, stdout io.Writer) error`; gancho de teste `var onListen = func(addr string) {}`.

- [ ] **Step 1: Escrever os testes que falham**

`backend/cmd/server/main_test.go`:

```go
package main

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/db"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/testdb"
)

func TestMain(m *testing.M) { testdb.Run(m) }

func envFor(values map[string]string) func(string) string {
	return func(k string) string { return values[k] }
}

func baseEnv(databaseURL string) map[string]string {
	return map[string]string{
		"DATABASE_URL": databaseURL, "APP_BASE_URL": "http://localhost:5173", "ALLOWED_EMAIL_DOMAINS": "empresa.com",
		"MAIL_DRIVER": "log", "HTTP_ADDR": "127.0.0.1:0", "LOG_FORMAT": "text",
	}
}

func TestRunRejectsUnknownSubcommandBeforeTouchingDatabase(t *testing.T) {
	err := run(context.Background(), []string{"voar"}, envFor(map[string]string{}), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "subcomando desconhecido") {
		t.Fatalf("err = %v", err)
	}
}

func TestRunReportsConfigErrors(t *testing.T) {
	err := run(context.Background(), []string{"migrate"}, envFor(map[string]string{}), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("err = %v", err)
	}
}

func TestMigratePromoteAndRecompute(t *testing.T) {
	ctx := context.Background()
	url := testdb.NewURL(t)
	env := envFor(baseEnv(url))
	var out bytes.Buffer
	if err := run(ctx, []string{"migrate"}, env, &out); err != nil {
		t.Fatal(err)
	}
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	testdb.CreateUser(t, pool, "ana")
	if err := run(ctx, []string{"promote-admin"}, env, &out); err == nil || !strings.Contains(err.Error(), "uso:") {
		t.Fatalf("sem argumento: %v", err)
	}
	if err := run(ctx, []string{"promote-admin", "ana"}, env, &out); err != nil {
		t.Fatal(err)
	}
	var isAdmin bool
	_ = pool.QueryRow(ctx, "SELECT is_admin FROM users WHERE username = 'ana'").Scan(&isAdmin)
	if !isAdmin {
		t.Fatal("promote-admin não promoveu")
	}
	out.Reset()
	if err := run(ctx, []string{"recompute-stats"}, env, &out); err != nil || !strings.Contains(out.String(), "partidas puladas: 0") {
		t.Fatalf("recompute: %v %q", err, out.String())
	}
}

func TestServeStartsAndShutsDown(t *testing.T) {
	url := testdb.NewURL(t)
	addrCh := make(chan string, 1)
	onListen = func(addr string) { addrCh <- addr }
	t.Cleanup(func() { onListen = func(string) {} })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, nil, envFor(baseEnv(url)), &bytes.Buffer{}) }()
	var addr string
	select {
	case addr = <-addrCh:
	case err := <-done:
		t.Fatalf("servidor saiu antes de ouvir: %v", err)
	case <-time.After(30 * time.Second):
		t.Fatal("servidor não começou a ouvir")
	}
	resp, err := http.Get("http://" + addr + "/healthz")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz: %v %v", resp, err)
	}
	resp.Body.Close()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("desligamento: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("servidor não desligou")
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./cmd/server/`
Expected: FAIL de compilação.

- [ ] **Step 3: Implementar**

`backend/cmd/server/main.go`:

```go
// Comando server sobe a API do Truco e oferece tarefas administrativas.
//
// Uso: server [serve|migrate|promote-admin <email|username>|recompute-stats]
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/admin"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/auth"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/config"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/db"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/httpapi"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/mail"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/match"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/stats"
)

// onListen é um gancho de teste chamado com o endereço real em que o servidor ouve.
var onListen = func(addr string) {}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Getenv, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, getenv func(string) string, stdout io.Writer) error {
	command := "serve"
	if len(args) > 0 {
		command, args = args[0], args[1:]
	}
	switch command {
	case "serve", "migrate", "promote-admin", "recompute-stats":
	default:
		return fmt.Errorf("subcomando desconhecido %q (use serve, migrate, promote-admin ou recompute-stats)", command)
	}
	if command == "promote-admin" && len(args) != 1 {
		return errors.New("uso: server promote-admin <email|username>")
	}
	cfg, err := config.Load(getenv)
	if err != nil {
		return fmt.Errorf("configuração inválida:\n%w", err)
	}
	logger := newLogger(cfg, stdout)
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	switch command {
	case "migrate":
		return db.Migrate(ctx, pool)
	case "promote-admin":
		if err := admin.PromoteAdmin(ctx, pool, args[0], time.Now()); err != nil {
			return fmt.Errorf("promover %q: %w", args[0], err)
		}
		fmt.Fprintf(stdout, "%s agora é administrador\n", args[0])
		return nil
	case "recompute-stats":
		skipped, err := match.NewStore(pool, nil).RecomputeStats(ctx)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "estatísticas recalculadas; partidas puladas: %d\n", len(skipped))
		for _, id := range skipped {
			fmt.Fprintf(stdout, "  pulada: %s\n", id)
		}
		return nil
	}
	if cfg.MigrateOnStart {
		if err := db.Migrate(ctx, pool); err != nil {
			return err
		}
	}
	return serve(ctx, cfg, pool, logger)
}

func newLogger(cfg config.Config, w io.Writer) *slog.Logger {
	opts := &slog.HandlerOptions{Level: cfg.LogLevel}
	if cfg.LogFormat == "text" {
		return slog.New(slog.NewTextHandler(w, opts))
	}
	return slog.New(slog.NewJSONHandler(w, opts))
}

func newMailer(cfg config.Config, logger *slog.Logger) mail.Mailer {
	if cfg.MailDriver == config.MailLog {
		return mail.NewLog(logger)
	}
	return mail.NewResend(cfg.ResendAPIKey, cfg.MailFrom)
}

func serve(ctx context.Context, cfg config.Config, pool *pgxpool.Pool, logger *slog.Logger) error {
	authSvc := auth.NewService(auth.Options{
		Pool: pool, Mailer: newMailer(cfg, logger), Hasher: auth.NewHasher(auth.DefaultParams, 4),
		BaseURL: cfg.AppBaseURL, Cookies: auth.CookieConfig{Secure: cfg.CookieSecure}, Logger: logger,
	})
	if err := authSvc.BootstrapDomains(ctx, cfg.AllowedEmailDomains); err != nil {
		return err
	}
	api := httpapi.New(httpapi.Deps{
		Config: cfg, Auth: authSvc, Stats: stats.NewReader(pool, cfg.RankingMinMatches), Matches: match.NewStore(pool, nil),
		Admin: admin.NewService(admin.Options{Pool: pool, Auth: authSvc, Aborter: admin.NoopAborter{}, Logger: logger}),
		Pool:  pool, Logger: logger,
	})
	listener, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("ouvir em %s: %w", cfg.HTTPAddr, err)
	}
	srv := &http.Server{
		Handler: api.Handler(), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second,
		WriteTimeout: 60 * time.Second, IdleTimeout: 120 * time.Second,
		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}
	go api.RunMaintenance(ctx)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(listener) }()
	logger.Info("servidor ouvindo", "addr", listener.Addr().String())
	onListen(listener.Addr().String())

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}
	logger.Info("desligando o servidor")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
```

No `Makefile`, acrescente `run migrate` ao `.PHONY` e os alvos (carregam o `.env` da raiz):

```make
run:
	cd backend && set -a && . ../.env && set +a && go run ./cmd/server serve

migrate:
	cd backend && set -a && . ../.env && set +a && go run ./cmd/server migrate
```

- [ ] **Step 4: Rodar e ver passar**

Run: `cd backend && go test ./cmd/server/ -race -v`
Expected: PASS.

- [ ] **Step 5: Teste manual de ponta a ponta**

Run (com `.env` copiado de `.env.example`, `MAIL_DRIVER=log` e `COOKIE_SECURE=false`):

```bash
make dev-db
make run
```

Em outro terminal:

```bash
curl -s localhost:8080/healthz
curl -s -X POST localhost:8080/api/auth/register -H 'Origin: http://localhost:5173' -H 'Content-Type: application/json' \
  -d '{"username":"ana","email":"ana@empresa.com.br","displayName":"Ana","password":"cavalo-bateria-grampo"}'
```

Expected: `{"status":"ok"}`; `202` com a mensagem de confirmação; o link `.../verificar-email?token=...` aparece no log do servidor.

- [ ] **Step 6: Commit**

```bash
git add backend/cmd Makefile
git commit -m "feat(server): adiciona binário com subcomandos e desligamento gracioso"
```

---

### Tarefa 22: CI, README e verificação final

**Files:**
- Create: `.github/workflows/ci.yml`
- Modify: `README.md`

**Interfaces:**
- Consumes: todos os alvos do `Makefile`.
- Produces: pipeline que roda `go vet`, `go test -race ./...` com `TRUCO_REQUIRE_DB=1` e falha se `make gen-sql` ou `make gen-types` mudarem arquivos versionados.

- [ ] **Step 1: Workflow de CI**

`.github/workflows/ci.yml`:

```yaml
name: CI

on:
  push:
    branches: [main, staging]
  pull_request:

jobs:
  backend:
    runs-on: ubuntu-latest
    env:
      TRUCO_REQUIRE_DB: "1"
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: backend/go.mod
          cache-dependency-path: backend/go.sum
      - name: go vet
        run: cd backend && go vet ./...
      - name: testes (com Postgres via testcontainers)
        run: cd backend && go test -race ./...
      - name: código gerado está em dia
        run: |
          make gen-sql gen-types
          git diff --exit-code
```

- [ ] **Step 2: Atualizar o README**

Edite `README.md`:

1. Badge de status: troque `status-engine%20conclu%C3%ADdo-blue` por `status-contas%20e%20persist%C3%AAncia-blue`.
2. Tabela "Camada | Tecnologia": em **Persistência**, troque `PostgreSQL *(planejado)*` por `PostgreSQL 18 + pgx + sqlc + goose`.
3. Tabela do roadmap: linha 2 com status `✅ **Concluído**` e o parágrafo seguinte passa a dizer que o repositório já tem a API de contas, a persistência de partidas e o ranking, faltando salas/WebSocket (Spec 3) e interface (Spec 4).
4. Na seção "Decisões já tomadas para as próximas etapas", remova os itens já implementados (banco, contas, sessão, recuperação, partidas) e deixe só o que é da Spec 3/4.
5. Em "Estrutura do repositório", acrescente `cmd/server/`, `internal/{config,db,auth,mail,ratelimit,replay,match,stats,admin,httpapi,pagination}` com uma linha de descrição cada (use as descrições da seção "Estrutura de Arquivos" deste plano), `docker-compose.yml`, `.env.example` e `.github/workflows/ci.yml`.
6. Em "Como começar": pré-requisitos ganham **Docker** (Postgres local e testes de integração) e **gcc** (para o `sqlc`); acrescente o bloco:

````markdown
### Subir o backend

```bash
cp .env.example .env        # ajuste ALLOWED_EMAIL_DOMAINS e o e-mail
make dev-db                 # Postgres 18 no Docker
make run                    # aplica migrations e sobe em :8080
cd backend && go run ./cmd/server promote-admin ana@empresa.com.br
```
````

7. Tabela de comandos: acrescente `make dev-db`, `make run`, `make migrate`, `make gen-sql`.
8. Nova seção **"🔐 API HTTP"** depois de "Protocolo WebSocket", com uma tabela das rotas de `/api/auth`, `/api/me`, leitura e `/api/admin` (método, rota, descrição em uma linha), o formato de erro `{"error":{"code","message","fields"}}` e a lista de códigos.
9. Nova subseção **"Variáveis de ambiente"** com a tabela da spec §10.
10. Seção "Testes": cite os testes de integração com Postgres real (testcontainers, banco-template clonado por teste), o `TRUCO_REQUIRE_DB=1` do CI e o round-trip de replay com partidas aleatórias. Atualize a contagem de funções de teste com `grep -rh "^func Test" backend --include=*_test.go | wc -l` e a cobertura com `cd backend && go test -cover ./...`.
11. "Documentação de projeto": acrescente os links da spec e do plano da Spec 2.

- [ ] **Step 3: Verificação final completa**

Run:

```bash
cd backend && go vet ./... && TRUCO_REQUIRE_DB=1 go test -race -cover ./... && cd .. && make gen-sql gen-types && git status --short
```

Expected: `vet` limpo; todos os pacotes `ok` (nenhum `SKIP` de integração, já que `TRUCO_REQUIRE_DB=1`); `git status` sem mudanças em arquivos gerados. Confirme também que `internal/engine` segue só com stdlib:

```bash
cd backend && go list -deps ./internal/engine | grep -v -E '^[a-z]+(/|$)' | grep -v truco-multiplayer
```

Expected: saída vazia.

- [ ] **Step 4: Commit**

```bash
git add .github README.md
git commit -m "docs: atualiza README e adiciona CI com testes de integração"
```

---

## Cobertura da spec

| Spec | Tarefa |
|---|---|
| §2 decisões (pgx+sqlc, goose, argon2id, sessão, Resend, CSRF, rate limit, testcontainers) | 6, 7, 8, 10, 11, 12, 17 |
| §3 estrutura de pacotes e regras de dependência | todas; verificação final na 22 |
| §4 schema completo | 6 |
| §5.1–5.3 normalização, senha, tokens | 8, 9 |
| §5.4 fluxos de autenticação | 13 (serviço), 18 (HTTP) |
| §5.5 sessão, cookie, `SessionFromRequest`, `CheckOrigin`, limpeza | 12, 17 (`RunMaintenance`) |
| §5.6 CSRF, enumeração, rate limits, limites de entrada, headers | 10, 13, 17, 18 |
| §5.7 e-mail (Resend, log, fake, templates) | 11 |
| §6 administração e auditoria | 16, 20, 21 (`promote-admin`) |
| §7.1 `RulesVersion` e `MarshalJSON` | 1 |
| §7.2–7.3 `Store`, `Rebuild`, `Recover` | 2, 3, 14 |
| §7.4 estatísticas | 4, 14 |
| §7.5–7.6 ranking, consultas, replay, `recompute-stats` | 14, 15, 19, 21 |
| §8 formato de erros | 17 |
| §9 HTTP (mux, middlewares, SPA, healthz, shutdown) | 17, 21 |
| §10 configuração | 5 |
| §11 ambiente de desenvolvimento | 5, 7, 21 |
| §12 estratégia de testes e CI | todas; 22 |
| §13 critérios de pronto | 22 |

**Desvios conscientes em relação à spec** (todos menores):
- `auth.SessionFromRequest` virou método `(*auth.Service).SessionFromRequest(r)`, porque precisa do serviço e da configuração de cookie.
- `match.ListInProgress` não existe como método público: `Store.Recover` cobre o único consumidor (Spec 3), tratando falhas partida a partida.
- Código de erro `invalid_request` adicionado para JSON malformado, `Content-Type` errado e corpo grande demais.
- Pacote `pagination` adicionado para não duplicar a codificação de cursores entre `stats` e `admin`.
