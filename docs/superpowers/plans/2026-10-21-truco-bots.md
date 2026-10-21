# Spec 5 — Bots: Plano de Implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bots que completam as cadeiras vazias ("Começar com bots") e que assumem o lugar de quem fica ausente, com um cérebro forte que só joga com o que veria — e partidas com bot fora do ranking.

**Architecture:** O cérebro vive num pacote puro novo, `internal/bot` (importa só `engine` e a stdlib): uma política simples fixa (referência e fallback) e uma forte por determinização + simulação, ambas recebendo `bot.Input` (visão do assento + eventos visíveis + sinais) — nunca `engine.GameState`. Um `bot.Runner` executa decisões fora do ator da sala, com semáforo global e orçamento por contexto, e entrega o resultado por callback. A sala (`internal/room`) ganha assentos de bot (`seatState.bot`), "controlado por bot" (bot ou humano ausente), um log de eventos por assento e a decisão de `ranked` no `Finish`. Bots são linhas de `users` com `kind='bot'`, semeadas pela migração 0005, carregadas no boot e entregues à sala por `room.Options.Bots`.

**Tech Stack:** Go 1.27 (stdlib, pgx/sqlc, goose, testcontainers), Vue 3 + Pinia, Tailwind v4 com tokens do brief, Vitest, Playwright.

**Spec:** `docs/superpowers/specs/2026-10-21-truco-bots-design.md` (leia junto; os § abaixo são dela). Visual: `docs/superpowers/specs/2026-10-20-truco-frontend-design-brief.md`.

## Global Constraints

- Textos ao usuário, comentários, logs e commits em **pt-BR com acentos**; identificadores em inglês, exceto views/componentes em pt-BR (padrão da 4a/4b).
- Commits convencionais em pt-BR, **um por tarefa**, **sem** linhas de atribuição. Nada de push, merge ou PR. Branch: `feat/spec5-bots`.
- Ao terminar cada tarefa: `cd backend && TESTCONTAINERS_RYUK_DISABLED=true TRUCO_REQUIRE_DB=1 go test ./...` (se tocou backend) e `cd frontend && pnpm typecheck && pnpm lint && pnpm test` (se tocou frontend). Mudou SQL: `make gen-sql`. Mudou `internal/protocol`: `make gen-types` e golden (`cd backend && go test ./internal/protocol -run Golden -update`).
- **Nunca** edite `backend/internal/db/dbgen/*` nem `frontend/src/protocol/*` à mão.
- `internal/engine` e `internal/bot` importam **apenas a stdlib** (e `bot` também `engine`); `engine.RulesVersion` continua **1**.
- `internal/room` não importa `net/http`, `coder/websocket`, `match` nem `db`. Pode importar `bot`.
- **Anti-trapaça (§5.1):** nenhuma função exportada de `internal/bot` recebe `engine.GameState` de uma partida real. O único construtor de `bot.Input` a partir do estado vive em `internal/room` e usa só `engine.ViewFor` + `engine.FilterEvents`.
- **Anti-travamento (§5.4):** nada de cálculo de bot no goroutine do ator; resultado volta por `Options.Post`.
- **Mensagem WS nova** (`start_with_bots`): constante em `protocol/client.go`, entrada em `registry.go` (payload `nil`), roteada pela sala (cai no `command` da sala como os outros comandos de assento), `make gen-types`, golden, e o `frontend/src/app/wire.ts`/store de sala enviando. `ProtocolVersion` continua `1`.
- Frontend: TypeScript `strict` + `noUncheckedIndexedAccess`; sem `any`, sem `!`; componentes só falam com stores; regras visuais do brief (tokens, Kalam só em placar/anúncios, alvos ≥ 44 px, cor nunca é o único sinal). Tarefas de tela (T12) fecham com o ciclo de revisão visual da 4b (screenshots em 1440×900 e 375×812, claro/escuro, `impeccable` critique/polish, `fixing-accessibility`).
- **Banco de dev:** `truco` no Postgres do projeto `infra` em `:5432`; `make migrate` após a T1. Nunca `make dev-db`.
- **E2E:** só `scripts/e2e.sh`.
- Configuração nova (§10), validada em `internal/config`: `BOT_THINK_BUDGET_MS` 800 (50..5000), `BOT_MAX_CONCURRENT` = `GOMAXPROCS` (1..64), `BOT_DELAY_MIN_MS` 1000 (0..10000), `BOT_DELAY_MAX_MS` 3000 (≥ min, ..10000).
- Nomes dos bots (§6.1): `Seu Zé`, `Dona Cida`, `Tio Juca`, `Vó Nena`, `Compadre Tião`, `Dona Lurdes`, `Primo Dedé`, `Tia Fia`; usernames `bot_seu_ze`, `bot_dona_cida`, `bot_tio_juca`, `bot_vo_nena`, `bot_compadre_tiao`, `bot_dona_lurdes`, `bot_primo_dede`, `bot_tia_fia`.

## Review Focus

Cinco situações que a spec implica e que os testes de caminho feliz não pegariam; cada uma tem teste na tarefa dona:

1. **Humano age no mesmo instante em que o bot termina de pensar por ele** (reconectou durante a pausa): vale o que chegou primeiro ao ator; o resultado atrasado do bot é descartado sem aplicar nem gravar nada e sem erro ao humano — `TestStaleBotResultIsDiscarded` e `TestHumanActsBeforeBotDuringPause` (T10).
2. **Último humano sai da sala de espera com 3 bots sentados**: a sala é destruída e some da lista (não fica uma sala-fantasma só de bots) — `TestLastHumanLeavingDestroysRoomWithBots` (T9).
3. **Restart do servidor no meio de uma partida solo**: bots voltam jogando, o humano volta ausente com bot no lugar até reconectar, e a partida não é abandonada por "todos ausentes" contando bots — `TestRestoredBotsKeepPlaying` (T10) e `TestRestoreSkipsBotsInIndex` (T11).
4. **Duas mesas usam o mesmo bot ao mesmo tempo**: ambas jogam, o lobby não "move" o bot de uma para a outra, e `UNIQUE (match_id, user_id)` não estoura — `TestSameBotInTwoRooms` (T11).
5. **Cálculo do bot lento ou várias mesas ao mesmo tempo** (semáforo cheio): a decisão sai dentro do prazo pelo bot simples, o ator nunca bloqueia — `TestRunnerFallsBackWhenQueueExceedsDeadline` (T7) e `TestBotThinkingDoesNotBlockActor` (T10).

---

## Mapa de arquivos

| Arquivo | Responsabilidade | Tarefa |
|---|---|---|
| `backend/internal/db/migrations/0005_bots.sql`, `internal/db/queries/{users,matches,stats}.sql`, `internal/replay/replay.go`, `internal/match/{store,recover}.go`, `internal/stats/derive.go` | `users.kind`, bots semeados, `matches.ranked`, `source='bot'`, stats só em `ranked` | 1 |
| `internal/db/queries/{ranking,users,admin}.sql`, `internal/stats/reader.go`, `internal/auth/{flows,invite}.go` | filtros de bot nas leituras da 4b, perfil de bot 404, histórico com `ranked`/`bot` | 2 |
| `internal/engine/legal.go` | `LegalActions` | 3 |
| `internal/bot/{input,simple,legal_test}.go` | entrada do bot, política simples fixa | 4 |
| `internal/bot/{determinize,belief}.go` | estado hipotético coerente + inferência | 5 |
| `internal/bot/{strong,signals}.go` | política forte e sinais | 6 |
| `internal/bot/runner.go`, `internal/config/config.go`, `cmd/botbench/main.go`, `Makefile` | execução com orçamento/semáforo, benchmark | 7 |
| `internal/protocol/{client,server,errors,registry}.go`, `internal/lobby/{lobby,invites}.go` | protocolo novo, cadeira de bot conta como vaga | 8 |
| `internal/room/{room,seats,ports,playing}.go` | bots na sala de espera, `start_with_bots`, revanche | 9 |
| `internal/room/{botdrive,playing,timers,presence,signals}.go` | bot joga/assume, pausa, descarte, `ranked`, restore | 10 |
| `internal/lobby/{lobby,store}.go`, `cmd/server/main.go`, `.env.example`, `README.md` | roster de bots no boot, restore, `Runner` montado | 11 |
| `frontend/src/{stores/room.ts,game/labels.ts,components/sala/*,components/lobby/LinhaSala.vue,components/mesa/*,components/partidas/LinhaPartida.vue}` | telas | 12 |
| `backend/internal/httpapi/ws_e2e_test.go`, `frontend/e2e/bots.spec.ts` | ponta a ponta | 13 |

---

### Task 1: Esquema de bots, `ranked` e estatísticas só de partidas que contam

**Skills:** `postgresql`, `database-migration`.

**Files:**
- Create: `backend/internal/db/migrations/0005_bots.sql`
- Modify: `backend/internal/db/queries/{users,matches,stats}.sql`, `backend/internal/replay/replay.go`, `backend/internal/match/store.go`, `backend/internal/match/recover.go`, `backend/internal/stats/derive.go`, `backend/internal/room/ports.go` (só a assinatura de `MatchStore`), adaptador em `backend/internal/lobby/store.go`
- Test: `backend/internal/db/db_test.go`, `backend/internal/match/store_test.go`, `backend/internal/stats/derive_test.go`, `backend/internal/replay/replay_test.go`

**Interfaces:**
- Produces:
  - `replay.SourceBot Source = "bot"`; `Source.Valid()` aceita.
  - `replay.PlayerSeat{UserID, DisplayName string; Bot bool}`.
  - `replay.Record.Ranked bool` (lido de `matches.ranked`).
  - `match.Store.Create(ctx, seed uint64, firstDealer engine.Seat, players [4]replay.PlayerSeat) (string, error)` — grava `ranked = !anyBot(players)`.
  - `match.Store.Finish(ctx, id string, ranked bool) error` — `ranked` final = `matches.ranked AND ranked`; grava `match_player_stats`/`player_stats` **só** se o final for `true`.
  - `room.MatchStore.Finish(ctx, id string, ranked bool) error` (interface atualizada; a sala passa `true` até a T10).
  - `match.Bot{ID, Username, DisplayName string}`; `(*match.Store).ListBots(ctx) ([]match.Bot, error)` (ordem por `username`).
  - `match.Recover` finaliza partidas já terminadas com `Finish(ctx, id, true)` (o `AND` com a coluna mantém `false` de quem começou com bot).
  - `match.Recovered.Record.Players[s].Bot` preenchido de `users.kind`.

- [ ] **Step 1: Testes que falham**

`db_test.go` — `TestBotsMigration`: após `db.Migrate`, `SELECT count(*) FROM users WHERE kind='bot'` = 8 com os usernames e nomes de Global Constraints, `email IS NULL AND password_hash IS NULL`; `INSERT` de humano com `email NULL` viola o `CHECK`; `INSERT` em `match_actions` com `source='bot'` é aceito; partidas antigas têm `ranked = true`; `MigrateDownTo(ctx, pool, 4)` remove tudo sem erro.

`store_test.go`:
- `TestCreateWithBotIsUnranked`: `Create` com `players[1].Bot = true` (id de `ListBots()[0]`) → `Load(...).Ranked == false`.
- `TestFinishUnrankedSkipsStats`: partida só humana completa; `Finish(ctx, id, false)` → `matches.ranked = false`, `status='finished'`, nenhuma linha em `match_player_stats` e `player_stats` intactas.
- `TestFinishRankedKeepsStats`: igual com `true` → stats gravados como hoje.
- `TestFinishCannotRerankBotMatch`: partida criada com bot, `Finish(ctx, id, true)` → continua `ranked=false`, sem stats.
- `TestListBots`: 8 bots, `Username` com prefixo `bot_`.

`derive_test.go` — `TestDeriveSkipsSeatCountersForBotActions`: log onde o assento 1 pede truco com `SourceBot` e depois com `SourcePlayer`; `RaisesCalled[1] == 1`; um `SourceTimeout` vira timeout, um `SourceBot` não; contadores de equipe/mão iguais ao log com todas as ações `SourcePlayer`.

Run: `cd backend && go test ./internal/db/ ./internal/match/ ./internal/stats/ ./internal/replay/`
Expected: FAIL (coluna `kind` inexistente, `SourceBot` indefinido).

- [ ] **Step 2: Migração 0005**

`ALTER TABLE users ADD COLUMN kind text NOT NULL DEFAULT 'human' CHECK (kind IN ('human','bot'))`; `ALTER COLUMN email DROP NOT NULL`, `ALTER COLUMN password_hash DROP NOT NULL`; `ADD CONSTRAINT users_human_credentials CHECK (kind = 'bot' OR (email IS NOT NULL AND password_hash IS NOT NULL))`; `INSERT` dos 8 bots (`kind='bot'`, `email_verified_at NULL`). `ALTER TABLE matches ADD COLUMN ranked boolean NOT NULL DEFAULT true`. Troca o `CHECK` de `match_actions.source` para incluir `'bot'`. Down: apaga bots (só se não houver `match_players` deles — senão falha com mensagem clara), reverte colunas e `CHECK`s.

- [ ] **Step 3: Código**

`make gen-sql` (o modelo `dbgen.User` passa a ter `Email`/`PasswordHash` anuláveis: ajuste os usos de humano — login, cadastro, admin, e-mail — tratando nulo como impossível para `kind='human'`). `Derive`: em `observe`, contadores de `a.seat[...]` só quando `Source != SourceBot`; eventos de equipe/mão seguem. `Store.Finish` faz `UPDATE ... SET ranked = ranked AND @ranked RETURNING ranked` e só chama `writeStats` se `true`. A sala chama `Finish(ctx, id, true)` por enquanto.

- [ ] **Step 4: Verificar**

Run: `make gen-sql && cd backend && TESTCONTAINERS_RYUK_DISABLED=true TRUCO_REQUIRE_DB=1 go test ./...`
Expected: PASS. Depois `make migrate`.

- [ ] **Step 5: Commit**

```bash
git add backend
git commit -m "feat(bots): contas de bot, partidas ranqueadas e ações de bot no log"
```

---

### Task 2: Leituras da 4b sem bots

**Skills:** `postgresql`.

**Files:**
- Modify: `backend/internal/db/queries/ranking.sql` (ranking, posições, `ListPartners`, `ListUserMatches`, `ListPlayersForMatches`), `backend/internal/db/queries/users.sql` (`SearchDirectory`, lookups de login), `backend/internal/db/queries/admin.sql` (`SearchUsers`), `backend/internal/stats/reader.go`, `backend/internal/auth/flows.go`, `backend/internal/auth/invite.go`
- Test: `backend/internal/stats/reader_test.go`, `backend/internal/auth/flows_test.go`, `backend/internal/auth/invite_test.go`, `backend/internal/admin/admin_test.go`

**Interfaces:**
- Consumes: `users.kind`, `matches.ranked` (T1).
- Produces: `stats.HistoryItem.Ranked bool` (`json:"ranked"`); `stats.MatchPlayer.Bot bool` (`json:"bot"`); `Reader.Profile`/`Reader.History` devolvem `ErrUserNotFound` para `kind='bot'` **mesmo com `viewerIsAdmin`**.

- [ ] **Step 1: Testes que falham**

`reader_test.go`:
- `TestRankingIgnoresUnrankedAndBots`: `ana` vence 3 partidas `ranked=true` e 2 `ranked=false` (com bot) → ranking mostra 3 vitórias para `ana` em todos os períodos; nenhum bot aparece; posições idem.
- `TestPartnersIgnoreBotsAndUnranked`: `ana` com `caio` em 2 partidas ranqueadas e com `bot_seu_ze` em 4 sem ranking → `Partners == [{caio, 2, ...}]`.
- `TestHistoryShowsUnrankedWithBotFlag`: partida `ranked=false` aparece no histórico de `ana` com `Ranked == false` e o jogador bot com `Bot == true`.
- `TestBotProfileIsNotFoundEvenForAdmin`: `Profile(ctx, "bot_seu_ze", true)` e `History(ctx, "bot_seu_ze", "", 20, true)` → `ErrUserNotFound`.
- `TestDirectoryHidesBots`: `Directory(ctx, "bot", "")` vazio.

`flows_test.go` — `TestLoginRefusesBot`: login `bot_seu_ze` com qualquer senha → mesmo erro de credenciais inválidas, sem tentar comparar hash nulo. `invite_test.go` — `ActiveUserByUsername("bot_tio_juca")` → não encontrado. `admin_test.go` — `ListUsers` não lista bots.

Run: `cd backend && go test ./internal/stats/ ./internal/auth/ ./internal/admin/`
Expected: FAIL.

- [ ] **Step 2: Filtros**

Em cada query, `u.kind = 'human'` explícito (não depender de `email_verified_at`); ranking, posições e parceiros também `m.ranked`. `ListUserMatches` devolve `m.ranked`; `ListPlayersForMatches` devolve `u.kind = 'bot' AS bot`. `Reader.user()` checa `kind` antes do desvio de admin. Login: lookup filtra `kind='human'`. `make gen-sql`.

- [ ] **Step 3: Verificar e commitar**

Run: `make gen-sql && cd backend && TESTCONTAINERS_RYUK_DISABLED=true TRUCO_REQUIRE_DB=1 go test ./...` → PASS.

```bash
git add backend
git commit -m "feat(bots): ranking, perfil, diretório, convites, login e admin ignoram bots"
```

---

### Task 3: `engine.LegalActions`

**Skills:** nenhuma.

**Files:**
- Create: `backend/internal/engine/legal.go`, `backend/internal/engine/legal_test.go`
- Modify: `backend/internal/engine/random_test.go` (passa a usar `LegalActions` onde hoje filtra candidatos por tentativa)

**Interfaces:**
- Produces: `func LegalActions(g GameState, seat Seat) []Action` — todas as ações que `Apply` aceita para `seat` no estado (inclui `request_raise` em qualquer tipo de mão onde o engine aceita, inclusive o pedido suicida em mão de 11/ferro, e todas as combinações de `burn` com `Slots`); nunca devolve ações `sys_*`; ordem determinística; `nil` sem decisão pendente.

- [ ] **Step 1: Testes que falham**

`TestLegalActionsMatchApply` (propriedade sobre 300 partidas aleatórias, seeds 1..300, todas as fases e `HandNormal`/`HandEleven`/`HandIron`): para cada estado e assento, (a) toda ação listada passa em `Apply` sem erro; (b) um gerador exaustivo local (o `candidateActions` de hoje ampliado com `Slots` de 1 a 4 quatros e `request_raise` para os 4 assentos) não acha ação aceita por `Apply` fora da lista. `TestLegalActionsEmptyWithoutDecision`: assento sem vez em `playing` → `nil`.

Run: `cd backend && go test ./internal/engine/ -run LegalActions` → FAIL (indefinido).

- [ ] **Step 2: Implementar** em `legal.go` gerando por fase (mesma lógica de `DefaultAction` em `timeout.go`) e filtrando por `Apply` só quando a regra é cara de reproduzir (ex.: aumento). Atualizar `random_test.go`.

- [ ] **Step 3: Verificar e commitar**

Run: `cd backend && go test ./internal/engine/` → PASS.

```bash
git add backend/internal/engine
git commit -m "feat(engine): enumera as ações legais de um assento"
```

---

### Task 4: Pacote `bot` — entrada e política simples fixa

**Skills:** nenhuma.

**Files:**
- Create: `backend/internal/bot/input.go`, `backend/internal/bot/simple.go`, `backend/internal/bot/simple_test.go`, `backend/internal/bot/legal_test.go`

**Interfaces:**
- Consumes: `engine.PlayerView`, `engine.Event`, `engine.LegalActions` (T3).
- Produces:
  - `type Signal string` com `SigTruco, SigBoa, SigVamos, SigCalma, SigDesculpa, SigValeu` (valores iguais aos `protocol.SignalID`; teste na T8 compara).
  - `type SignalSeen struct{ Seat engine.Seat; Signal Signal; HandNumber int }`
  - `type Input struct{ View engine.PlayerView; Events []engine.Event; Signals []SignalSeen; Legal []engine.Action }` — `Events` são os do assento na partida inteira, já filtrados; `Legal` vem de `LegalActions` calculado pela sala.
  - `type Decision struct{ Action engine.Action; Signal *Signal }`
  - `type Policy interface{ Decide(ctx context.Context, in Input, rng *rand.Rand) Decision }` (`math/rand/v2`).
  - `type Simple struct{}` implementando `Policy`.

- [ ] **Step 1: Testes que falham**

`simple_test.go`, com `View`s montadas à mão:
- `TestSimplePlaysWeakestWinningCard`: mesa com 7 de ouros do adversário, mão `{4♣, 2♠, A♥}` (sem manilha) → joga `2♠`; sem carta que ganhe → joga a mais fraca.
- `TestSimpleNeverCovers` (`Covered == false` sempre), `TestSimpleNeverBurns` (`pass_burn`).
- `TestSimpleRaisesOnlyWithManilha`; `TestSimpleAcceptsWithManilhaOrTwoHighCards`: aceita com manilha ou com duas cartas não jogadas de rank `A`, `2` ou `3` (as três mais fortes fora a manilha); senão `run`.
- `TestSimpleElevenPlaysOnlyWithManilha`.
- `TestSimpleNeverSignals` (`Signal == nil`).

`legal_test.go` — `TestPoliciesAlwaysReturnLegalAction` (tabela de políticas; nesta tarefa só `Simple`): em 200 partidas aleatórias com os 4 assentos jogando pela política, toda decisão está em `in.Legal` e `Apply` aceita; nunca pede aumento que `Apply` pune (`EvRaisePunished` nunca aparece). Monte o `Input` neste teste com um helper **de teste** `inputFor(g, seat, evs)` usando `ViewFor`/`FilterEvents` (a sala terá o seu na T10).

Run: `cd backend && go test ./internal/bot/` → FAIL.

- [ ] **Step 2: Implementar** `Simple.Decide` exatamente com as regras de §5.5 (bot simples fixado). Ele não é ajustado depois desta tarefa: qualquer mudança futura exige aval do usuário.

- [ ] **Step 3: Verificar e commitar**

Run: `cd backend && go test ./internal/bot/` → PASS.

```bash
git add backend/internal/bot
git commit -m "feat(bot): entrada do bot e política simples de referência"
```

---

### Task 5: Determinização e crença

**Skills:** nenhuma.

**Files:**
- Create: `backend/internal/bot/determinize.go`, `backend/internal/bot/belief.go`, `backend/internal/bot/determinize_test.go`

**Interfaces:**
- Consumes: `Input` (T4).
- Produces:
  - `type Belief struct{ ... }` com `func NewBelief(in Input) Belief` — pesos por (assento, carta) das cartas não vistas, ajustados por: cartas já vistas (eliminam), pedidos/aceites de aumento do assento (puxam para cartas fortes), corrida (puxa para fracas), sinais (parceiro com peso maior, adversário com peso menor; §5.6).
  - `func Sample(in Input, b Belief, rng *rand.Rand) engine.GameState` — estado completo hipotético coerente com `in.View`: mão do próprio assento exata, mesa/rodadas/vira/manilha/placar/fase/vez/pendências iguais, cartas cobertas alheias e mãos alheias sorteadas pela crença, resto no `Deck`.

- [ ] **Step 1: Testes que falham**

- `TestSampleConservesCards` (propriedade, 500 estados de partidas aleatórias × 20 amostras): 40 cartas distintas no total (`Hands` não jogadas + `Table` + `Discarded` + `Deck` + `CutOffer` + rodadas), cada mão com o tamanho real, nenhuma duplicata.
- `TestSampleMatchesView`: `engine.ViewFor(Sample(in, ...), in.View.Seat)` é igual a `in.View` (comparação estrutural).
- `TestSampleKeepsHiddenHidden`: duas `GameState` reais que diferem só nas mãos alheias produzem o mesmo `Input` e, com a mesma seed de `rng`, a mesma amostra.
- `TestBeliefRaiserHoldsStrongerCards`: depois de um `raise_requested` do assento 1, a força média esperada da mão do assento 1 na crença é maior que a uniforme.

Run: `cd backend && go test ./internal/bot/ -run 'Sample|Belief'` → FAIL.

- [ ] **Step 2: Implementar.** Amostragem por rejeição ponderada; se `Belief` tornar impossível (pesos zerados), cair para uniforme sobre as não vistas.

- [ ] **Step 3: Verificar e commitar**

```bash
git add backend/internal/bot
git commit -m "feat(bot): determinização coerente com a visão e crença sobre mãos"
```

---

### Task 6: Política forte e sinais

**Skills:** nenhuma.

**Files:**
- Create: `backend/internal/bot/strong.go`, `backend/internal/bot/signals.go`, `backend/internal/bot/strong_test.go`, `backend/internal/bot/signals_test.go`, `backend/internal/bot/strength_test.go`
- Modify: `backend/internal/bot/legal_test.go` (tabela inclui `Strong`)

**Interfaces:**
- Consumes: `Sample`, `NewBelief` (T5), `Simple` (T4).
- Produces:
  - `type Strong struct{ Samples int; Rollout Policy }` — `Samples` determinizações por decisão (determinístico dada a seed); `Rollout` padrão `Simple{}`. Para cedo se `ctx` vencer, devolvendo a melhor ação até ali (ou `Simple` sem nenhuma avaliação).
  - Valor de cada ação = média, sobre as amostras, da variação esperada de **pontos da partida** até o fim da mão, ponderada pelo placar (mão de 11, perto de 12). Busca exata (Monte Carlo plano, ISMCTS…) a critério do implementador.
  - `func DefaultStrong() Strong` (`Samples: 200`).
  - `func intentSignal(in Input, d engine.Action, rng *rand.Rand) *Signal` em `signals.go`: no máximo 1 sinal de intenção por mão (`truco`/`vamos`/`calma` pela tabela de §5.6), e reações sociais `boa` (dupla ganhou a rodada), `valeu` (parceiro jogou a carta que ganhou), `desculpa` (o próprio bot perdeu rodada que a dupla precisava) com baixa frequência.
  - `const MinGamesCI = 60` e `const MinWinsCI = 40` (p < 0,01 contra 50%).
  - `func PlayMatch(policies [4]Policy, seed uint64) engine.Team` em `bot/match.go` — partida completa do `NewGame` ao `game_over`, montando o `Input` de cada decisão pelo helper de teste da T4 (promovido a não exportado `inputFromState` neste arquivo, usado **só** por `PlayMatch` e testes; a regra anti-trapaça de Global Constraints continua: nenhuma função exportada recebe `GameState` de partida real).

- [ ] **Step 1: Testes que falham**

- `TestStrongNeverSuicideRaise`: mão de 11 e ferro → nenhuma decisão `request_raise`/`vote raise` punida (500 estados).
- `TestStrongStopsOnContextDeadline`: `ctx` já vencido → devolve ação legal em < 5 ms.
- `TestStrongDeterministicWithSeed`: mesmo `Input` + mesma seed → mesma decisão.
- `TestStrongBluffsSometimes`: em 300 mãos com só cartas fracas e placar neutro, pede/aumenta em mais de 0 e menos de 50% das vezes.
- `signals_test.go`: `TestAtMostOneIntentSignalPerHand`; `TestPartnerTrucoSignalRaisesWillingness` (com `SignalSeen{Partner, SigTruco}` a frequência de aceitar com mão média sobe); `TestOpponentSignalWeighsLess`.
- `strength_test.go` — `TestStrongBeatsSimple`: `MinGamesCI` partidas completas via `PlayMatch`, seeds `1..60`, dupla 0 `DefaultStrong()` com `Samples: 40`, dupla 1 `Simple`, metade das partidas com as duplas trocadas de lado; vitórias da forte ≥ `MinWinsCI`. Determinístico; se uma mudança no bot o reprovar, corrige-se o bot, não a régua.

Run: `cd backend && go test ./internal/bot/` → FAIL.

- [ ] **Step 2: Implementar** `Strong.Decide` e `signals.go`. `Strong` chama `intentSignal` ao decidir e a crença usa os sinais já vistos (T5).

- [ ] **Step 3: Verificar** — `cd backend && go test ./internal/bot/` → PASS, com `TestStrongBeatsSimple` em menos de ~60 s nesta máquina (se passar disso, reduza `Samples` só do teste e registre no commit).

- [ ] **Step 4: Commit**

```bash
git add backend/internal/bot
git commit -m "feat(bot): política forte por simulação, blefe e sinais"
```

---

### Task 7: `Runner`, configuração e benchmark

**Skills:** nenhuma.

**Files:**
- Create: `backend/internal/bot/runner.go`, `backend/internal/bot/runner_test.go`, `backend/cmd/botbench/main.go`
- Modify: `backend/internal/config/config.go` (+ `_test`), `Makefile`, `.env.example`

**Interfaces:**
- Produces:
  - `type Runner struct{...}`; `func NewRunner(o RunnerOptions) *Runner` com `RunnerOptions{Strong Policy; Fallback Policy; Budget time.Duration; MaxConcurrent int; Seed func() uint64}`.
  - `func (r *Runner) Think(in Input, deadline time.Time, done func(Decision))` — nunca bloqueia o chamador: roda numa goroutine, espera vaga no semáforo; se a espera passar de `deadline`, usa `Fallback`; senão chama `Strong` com `ctx` de `min(Budget, até deadline)`; chama `done` exatamente uma vez.
  - `config.BotConfig{ThinkBudget, DelayMin, DelayMax time.Duration; MaxConcurrent int}` em `Config.Bots`, com as faixas de Global Constraints.
  - `make bot-bench` → `cd backend && go run ./cmd/botbench -games 400 -seed 1`; imprime vitórias da forte (`DefaultStrong()`) contra a simples e sai com código ≠ 0 abaixo de 300/400 (75%).

- [ ] **Step 1: Testes que falham**

`runner_test.go` (políticas falsas controladas por canais):
- `TestRunnerDoesNotBlockCaller`: `Think` retorna em < 1 ms com a política travada.
- `TestRunnerRespectsMaxConcurrent`: 10 chamadas, `MaxConcurrent: 2` → nunca mais de 2 `Decide` simultâneos.
- `TestRunnerFallsBackWhenQueueExceedsDeadline`: semáforo cheio e `deadline` em 20 ms → `done` com a decisão do `Fallback`.
- `TestRunnerCallsDoneOnce` (inclusive quando a forte entra em pânico: recupera, loga e usa o fallback).

`config_test.go` — defaults e faixas de `BOT_*`; `BOT_DELAY_MAX_MS < BOT_DELAY_MIN_MS` → erro.

Run: `cd backend && go test ./internal/bot/ ./internal/config/` → FAIL.

- [ ] **Step 2: Implementar** `Runner`, `BotConfig`, `cmd/botbench` (usa `bot.PlayMatch` da T6, alternando lados como o teste), alvo `bot-bench`, variáveis no `.env.example`.

- [ ] **Step 3: Verificar**

Run: `cd backend && go test ./internal/bot/ ./internal/config/` → PASS; `make bot-bench` → imprime ≥ 300/400. Se não fechar após iterar no bot forte, **pare e reporte ao usuário** o número obtido (§5.5); não baixe a meta nem mexa no `Simple`.

- [ ] **Step 4: Commit**

```bash
git add backend Makefile .env.example
git commit -m "feat(bot): execução com orçamento e limite global, benchmark de força"
```

---

### Task 8: Protocolo e lobby (cadeira de bot é vaga)

**Skills:** nenhuma.

**Files:**
- Modify: `backend/internal/protocol/{client,server,errors,registry}.go` (+ golden), `backend/internal/lobby/lobby.go` (checagem de vaga no `Update`/`joinRoomLocked`), `backend/internal/lobby/invites.go` (`freeSeats`)
- Test: `backend/internal/protocol/messages_test.go`, `backend/internal/lobby/lobby_test.go`, `backend/internal/lobby/invites_test.go`

**Interfaces:**
- Produces:
  - `protocol.TypeStartWithBots = "start_with_bots"` (payload `nil`).
  - `protocol.CodeNoEmptySeat = "no_empty_seat"`, `protocol.CodeNotWaiting = "not_waiting"` com mensagens "não há cadeira vazia" e "a partida já começou".
  - `SeatInfo` ganha `Bot bool "bot"` e `BotPlaying bool "botPlaying"`.
  - `RoomStatePayload.Ranked *bool "ranked"` (`tstype:"boolean | null,required"`).
  - `MatchEndedPayload.Ranked bool "ranked"`; `protocol.MatchEnded(reason, winner, score, ranked bool)`.
  - `SeatSummary.Bot bool "bot"`.
  - `lobby.freeSeats(s room.Summary) int` conta `nil` **ou** `Bot`.

- [ ] **Step 1: Testes que falham**

- `TestSignalIDsMatchBotSignals` (em `protocol`, importando `bot` só no teste): os 6 `SignalID` == os 6 `bot.Signal`.
- Golden de `room_state`, `match_ended`, `lobby_snapshot` com os campos novos.
- `lobby_test.go` — `TestRoomWithBotSeatIsListedAsJoinable`: `Update` com 3 humanos + 1 `Bot` em `waiting` mantém a sala na lista e `join_room` sem assento é encaminhado à sala (não responde `room_full`).
- `invites_test.go` — `freeSeats` conta cadeira de bot.

Run: `cd backend && go test ./internal/protocol/ ./internal/lobby/` → FAIL.

- [ ] **Step 2: Implementar**; `make gen-types`; golden `-update`; ajustar chamadas de `MatchEnded` na sala passando `false` por enquanto (a T10 calcula). `frontend`: só os tipos regenerados nesta tarefa (o `pnpm typecheck` deve continuar verde).

- [ ] **Step 3: Verificar e commitar**

Run: `make gen-types && cd backend && go test ./... && cd ../frontend && pnpm typecheck` → PASS.

```bash
git add backend frontend/src/protocol
git commit -m "feat(protocolo): start_with_bots, selo de bot nos assentos e ranked no fim da partida"
```

---

### Task 9: Sala — bots na espera, "Começar com bots" e revanche

**Skills:** nenhuma.

**Files:**
- Modify: `backend/internal/room/{room,seats,ports,playing}.go`
- Test: `backend/internal/room/seats_test.go`, `backend/internal/room/harness_test.go` (fake de roster)

**Interfaces:**
- Consumes: `protocol.TypeStartWithBots`, códigos novos, `SeatInfo.Bot` (T8); `replay.PlayerSeat.Bot` (T1).
- Produces:
  - `room.BotIdentity{UserID, DisplayName string}`; `room.Options.Bots []BotIdentity` (roster; a sala escolhe os de menor índice ainda não sentados nela).
  - `seatState.bot bool` (bot: `connected=true`, `ready=true` sempre).
  - `(r *Room) humans() int` (substitui `members()` onde a semântica é "a sala vive": `removeSeat`, eleição de host) e `(r *Room) occupied() int` (para `maybeStart`, que exige 4 ocupados e todos prontos).
  - `(r *Room) firstHuman() engine.Seat` substitui `firstOccupied()`.

- [ ] **Step 1: Testes que falham** (harness atual, relógio manual)

- `TestStartWithBotsFillsAndStarts`: host sozinho → `start_with_bots` → 3 bots com nomes distintos e `Bot=true` no `room_state`, host `ready=true`, partida começa (`Store.Create` recebe 3 `PlayerSeat{Bot:true}`).
- `TestStartWithBotsWaitsForReadyHumans`: host + 1 humano não pronto → bots entram, partida **não** começa; humano marca pronto → começa.
- `TestStartWithBotsErrors`: não host → `not_host`; sala cheia de humanos → `no_empty_seat`; em `playing` → `not_waiting`.
- `TestRematchKeepsBotsReady`: fim da partida → bots sentados `ready=true`, humanos `false`; humanos prontos → revanche começa sozinha.
- `TestTakeSeatReplacesBot`: humano entra pedindo a cadeira 2 (bot) → senta, bot sai; `Join` sem assento com sala sem vaga → ocupa o bot de menor assento; `Join` com sala de 4 humanos → `room_full`.
- `TestHostKicksBotWithoutConfirmation`: `kick` em bot → sai; não host → `not_host`.
- `TestLastHumanLeavingDestroysRoomWithBots`: host + 3 bots, host sai → `Registry.Destroyed`.
- `TestHostPassesToHumanNotBot`: host sai com outro humano e bots → novo host é o humano.
- `TestBotSeatNeverRemovedForDisconnect`: em `waiting` após `WaitingDisconnect`, bots continuam.

Run: `cd backend && go test ./internal/room/ -run 'Bot|Rematch|TakeSeat|Host'` → FAIL.

- [ ] **Step 2: Implementar.** Bots não passam por `Registry.Seated`/`Released` (nunca entram no índice do lobby, §9). `publish`/`broadcast` pulam assentos de bot ao enviar (o `Outbox` já ignora, mas não envie à toa). `summary()` marca `SeatSummary.Bot`.

- [ ] **Step 3: Verificar e commitar**

Run: `cd backend && go test ./internal/room/` → PASS.

```bash
git add backend/internal/room
git commit -m "feat(sala): começar com bots, cadeira de bot trocável e revanche com bots"
```

---

### Task 10: Sala — bot joga, assume o ausente, pausa, `ranked` e restore

**Skills:** nenhuma.

**Files:**
- Create: `backend/internal/room/botdrive.go`, `backend/internal/room/botdrive_test.go`
- Modify: `backend/internal/room/{room,ports,playing,timers,presence,signals,msgs}.go`
- Test: `backend/internal/room/{botdrive,presence,playing}_test.go`

**Interfaces:**
- Consumes: `bot.Input`, `bot.Decision`, `bot.Signal` (T4); `engine.LegalActions` (T3); `Store.Finish(ctx, id, ranked)` (T1); `MatchEnded(..., ranked)` (T8).
- Produces:
  - Porta `room.Thinker interface{ Think(in bot.Input, deadline time.Time, done func(bot.Decision)) }` em `ports.go`; `Options.Thinker Thinker`; `Options.BotDelay func() time.Duration` (sorteio entre `DelayMin` e `DelayMax`; testes devolvem 0 ou valor fixo).
  - `Restored.Events [4][]engine.Event` e `Restored.Bots [4]bool`.
  - Msg interna `botDecided{gen uint64; window string; seat engine.Seat; d bot.Decision}`.
  - `(r *Room) botControlled(s engine.Seat) bool` = assento de bot **ou** humano `isAbsent()` em `playing`.
  - `(r *Room) inputFor(s engine.Seat) bot.Input` — **único** construtor de `bot.Input`: `engine.ViewFor`, `r.seatEvents[s]` (acumulados com `engine.FilterEvents` em `broadcastGame`), `r.signalsSeen` e `engine.LegalActions`.
  - `r.startedWithBots bool`, `r.everBotControlled [4]bool` (só para o aviso).

- [ ] **Step 1: Testes que falham** (harness com `Thinker` falso que guarda o `done` para o teste disparar)

- `TestBotSeatDecidesAfterDelay`: partida com bot no assento 1; quando é a vez dele, `Think` é chamado uma vez; `done` com ação legal antes do fim da pausa (`BotDelay` 2 s) não aplica; após 2 s aplica e grava `source='bot'`.
- `TestObviousMoveHasShorterPause`: com `len(in.Legal) == 1` (ex.: última carta), `readyAt = now + BotDelay()/4`.
- `TestBotThinkingDoesNotBlockActor`: com `Think` que nunca chama `done`, a sala continua processando `Command`s de outros assentos e sinais.
- `TestStaleBotResultIsDiscarded`: `done` chega depois que a janela mudou (humano agiu, mão andou) → nada aplicado, nada gravado, nenhum erro enviado.
- `TestAbsentHumanIsPlayedByBot`: humano desconecta em `playing` → próxima decisão dele é do bot (sem `DefaultAction`), `SeatInfo.BotPlaying=true`, `ranked` do `room_state` passa a `false`; ao reconectar, `BotPlaying=false` e a decisão seguinte espera o humano.
- `TestHumanActsBeforeBotDuringPause`: humano reconecta e joga durante a pausa → vale a jogada do humano; o `done` posterior é descartado.
- `TestConnectedInactiveBotActsAtAbsentWindowEnd`: humano conectado com 3 timeouts → bot só age em `deadline - 250ms` (constante `botInactiveLead`), não após a pausa.
- `TestBotSeatAccumulatesNoTimeout`: `Thinker` que nunca responde → no prazo, `DefaultAction` aplicado com `source='timeout'`, `timeouts` do assento de bot continua 0, assento não fica ausente nem alvo de W.O.
- `TestRankedDecidedAtFinish`: (a) só humanos, um cai e volta antes do fim → `Finish(..., true)`, `match_ended.ranked=true`; (b) cai e não volta até o `game_over` → `Finish(..., false)`; (c) W.O. pedido com bot no controle → `Finish(..., true)`; (d) começou com bot → `false` em todos os casos.
- `TestAllAbsentCountsOnlyHumans`: 1 humano + 3 bots, humano ausente pelo orçamento → `abandoned` (`all_absent`); 2 humanos, um ausente → nada.
- `TestBotSendsSignals`: `Decision.Signal` é transmitido como `signal` do assento do bot, respeitando o intervalo de 3 s; sinais recebidos entram em `r.signalsSeen` com `HandNumber`.
- `TestInputForHidesOtherHands` (anti-trapaça): duas salas com estados que diferem só nas mãos alheias produzem `inputFor` iguais.
- `TestRestoredBotsKeepPlaying`: `NewRestored` com `Restored.Bots[1..3]=true` → bots `connected=true`, jogam; humano desconectado é jogado por bot; `Events` restaurados aparecem em `inputFor`.

Run: `cd backend && go test ./internal/room/` → FAIL.

- [ ] **Step 2: Implementar.** Em `settle`, depois de `runDue`: para cada assento pendente `botControlled` sem pedido em voo na janela corrente, chamar `Thinker.Think(r.inputFor(s), deadline, func(d){ r.opt.Post(botDecided{...}) })` e registrar `readyAt` (`now + BotDelay()`, um quarto disso quando só há uma ação legal, ou `deadline - botInactiveLead` para conectado-inativo). `Handle(botDecided)`: descarta se `window`/`gen` não batem ou o assento já não é `botControlled`; senão guarda a decisão e aplica quando `readyAt` passar (o timer considera `readyAt` em `wakeups`). Em `runDue`, não incrementar `timeouts` de assento `bot`. `allAbsentAt` só humanos. `finish`: `ranked := !r.startedWithBots && (last.Kind == engine.ActSysForfeit || !anyHumanBotControlled())`. O aviso de §7.2 sai pelo `room_state` (`BotPlaying`); não há mensagem nova.

- [ ] **Step 3: Verificar e commitar**

Run: `cd backend && go test ./internal/room/` → PASS (incluindo o teste de propriedade existente, agora com um assento de bot simples).

```bash
git add backend/internal/room
git commit -m "feat(sala): bot joga e assume ausentes fora do ator, ranking decidido no fim"
```

---

### Task 11: Montagem — roster no boot, `Runner`, restore e lobby

**Skills:** nenhuma.

**Files:**
- Modify: `backend/internal/lobby/{lobby,store}.go`, `backend/cmd/server/main.go`, `README.md`
- Test: `backend/internal/lobby/lobby_test.go`, `backend/internal/httpapi/ws_e2e_test.go` (só boot)

**Interfaces:**
- Consumes: `match.Store.ListBots` (T1), `bot.NewRunner`, `config.BotConfig` (T7), `room.Options.{Bots,Thinker,BotDelay}`, `Restored.{Events,Bots}` (T10).
- Produces: `lobby.Options.Bots []room.BotIdentity`, `lobby.Options.Thinker room.Thinker`, `lobby.Options.BotDelay func() time.Duration`; `lobby.Restore` monta `Restored.Bots` de `Record.Players[s].Bot` e `Restored.Events` reprocessando `Record` com `replay` (eventos filtrados por assento).

- [ ] **Step 1: Testes que falham**

- `TestRestoreSkipsBotsInIndex`: `Restore` com partida de 1 humano + 3 bots → `index` só tem o humano.
- `TestSameBotInTwoRooms`: duas salas fazem `start_with_bots` com o mesmo roster → ambas começam e jogam; `index` sem bots.
- `TestServerBootsWithBotRoster` (e2e de boot): servidor sobe, `ListBots` carregado (8).

Run: `cd backend && TESTCONTAINERS_RYUK_DISABLED=true TRUCO_REQUIRE_DB=1 go test ./internal/lobby/ ./internal/httpapi/ -run 'Bot|Restore'` → FAIL.

- [ ] **Step 2: Implementar.** `main.go`: `ListBots` no boot (falha → erro de boot claro), `bot.NewRunner(RunnerOptions{Strong: bot.DefaultStrong(), Fallback: bot.Simple{}, Budget: cfg.Bots.ThinkBudget, MaxConcurrent: cfg.Bots.MaxConcurrent})`, `BotDelay` com `math/rand/v2`. README: seção "Bots" (como jogar, que não contam no ranking, `BOT_*`, `make bot-bench`).

- [ ] **Step 3: Verificar e commitar**

Run: `cd backend && TESTCONTAINERS_RYUK_DISABLED=true TRUCO_REQUIRE_DB=1 go test ./...` → PASS.

```bash
git add backend README.md
git commit -m "feat(bots): roster no boot, runner montado e restore de mesas com bots"
```

---

### Task 12: Telas

**Skills:** `frontend-design:frontend-design`, `impeccable`, `fixing-accessibility`, `ui-a11y`, `lint-and-validate`.

**Files:**
- Modify: `frontend/src/stores/room.ts` (+ `room.test.ts`), `frontend/src/game/labels.ts` (+ `labels.test.ts`), `frontend/src/components/sala/{SalaEspera,Assento}.vue` (+ `sala.test.ts`), `frontend/src/components/lobby/LinhaSala.vue` (+ `lobby.test.ts`), `frontend/src/components/mesa/{AssentoMesa,FimPartida,Anunciador,BarraSinais}.vue` (+ `mesa.test.ts`, `extras.test.ts`), `frontend/src/components/partidas/LinhaPartida.vue`
- Create: `frontend/src/components/ui/SeloBot.vue`, `frontend/src/components/mesa/LegendaSinais.vue`

**Interfaces:**
- Consumes: `SeatInfo.bot/botPlaying`, `RoomStatePayload.ranked`, `MatchEndedPayload.ranked`, `SeatSummary.bot`, `start_with_bots` (T8); `HistoryItem.ranked`, `MatchPlayer.bot` (T2).
- Produces: `useRoomStore().startWithBots(): void`; `labels.ts`: `no_empty_seat` → "Não há cadeira vazia.", `not_waiting` → "A partida já começou."; `SeloBot` (texto "bot" + ícone, não só cor).

- [ ] **Step 1: Testes que falham** (Vitest)

- `sala.test.ts`: dono com cadeira vazia vê "Começar com bots"; com outro humano não pronto, botão desabilitado e "Faltam 1 prontos"; não dono não vê; clique chama `startWithBots`. Cadeira de bot mostra nome + `SeloBot` + pronto; não dono vê "Sentar aqui"; dono vê "Remover" e o clique **não** abre o modal. Aviso "Partidas com bots não contam no ranking." quando há bot.
- `lobby.test.ts`: cadeira de bot em `LinhaSala` tem selo e é clicável como vaga.
- `mesa.test.ts`: selo no assento de bot; `botPlaying` → nome esmaecido + "bot jogando por Ana"; primeira vez que `botPlaying` vira `true` em partida que começou sem bot → anúncio no `Anunciador` e toast "Ana saiu; um bot assumiu. Se Ana não voltar até o fim, a partida não conta no ranking — vocês ainda podem pedir W.O."; em partida que começou com bot, só "Ana saiu; um bot assumiu.". `FimPartida` com `ranked=false` → "Partida com bots — não conta no ranking.".
- `extras.test.ts`: "?" na `BarraSinais` abre `LegendaSinais` com a tabela de §5.6 (foco preso, Esc fecha).
- `LinhaPartida`: partida `ranked=false` mostra "com bots"; jogador bot com selo e sem link.

Run: `cd frontend && pnpm test` → FAIL.

- [ ] **Step 2: Implementar** seguindo o brief (anúncio no tom da mesa, ex.: "Seu Zé pegou as cartas da Ana"; texto final a critério, §11).

- [ ] **Step 3: Revisão visual** (ciclo de Global Constraints): sala de espera com bots (dono e não dono), lobby com sala com bot, mesa com bot e com `botPlaying`, fim de partida sem ranking, legenda de sinais; 1440×900 e 375×812, claro e escuro.

- [ ] **Step 4: Verificar e commitar**

Run: `cd frontend && pnpm typecheck && pnpm lint && pnpm test` → PASS.

```bash
git add frontend
git commit -m "feat(frontend): começar com bots, selo de bot, bot jogando por ausente e partidas sem ranking"
```

---

### Task 13: Ponta a ponta

**Skills:** `e2e-testing`.

**Files:**
- Modify: `backend/internal/httpapi/ws_e2e_test.go`, `scripts/e2e.sh` (env `BOT_DELAY_MIN_MS=0 BOT_DELAY_MAX_MS=0 BOT_THINK_BUDGET_MS=50`)
- Create: `frontend/e2e/bots.spec.ts`

- [ ] **Step 1: Testes**

- Go, `TestSoloMatchWithBotsFinishesUnranked`: um cliente WS cria sala, `start_with_bots`, joga uma carta e depois para de responder; com `BOT_DELAY_*=0` e timers mínimos, o bot assume após 3 timeouts e a partida chega a `match_ended{reason:"game_over", ranked:false}`; no banco `status='finished'`, `ranked=false`, ações `source='bot'` presentes, nenhuma linha em `match_player_stats`, ranking sem a partida.
- Playwright `bots.spec.ts`: login, criar sala privada, "Começar com bots", ver 3 selos de bot na mesa, jogar a primeira carta, aguardar "Partida com bots — não conta no ranking." (timeout generoso), voltar à sala de espera com os bots sentados.

- [ ] **Step 2: Rodar**

Run: `cd backend && TESTCONTAINERS_RYUK_DISABLED=true TRUCO_REQUIRE_DB=1 go test ./internal/httpapi/ -run Solo` e `./scripts/e2e.sh` → PASS.

- [ ] **Step 3: Commit**

```bash
git add backend frontend/e2e scripts/e2e.sh
git commit -m "test(e2e): partida solo com bots até o fim, fora do ranking"
```

---

## Fora deste plano (precisa do usuário)

- Julgamento de força jogando (§1, critério de aceite): ao fim, o usuário joga algumas partidas solo e diz se o bot "parece competente".
- Se `make bot-bench` não chegar a 300/400, a decisão de aceitar outro número é do usuário (T7, Step 3).
