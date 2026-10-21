# Spec 4b — Social, ranking, histórico, perfil e admin: Plano de Implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Presença ao vivo, convites para a sala (no app, por e-mail e por link), ranking por período e métrica, histórico e perfil enriquecidos e painel admin endurecido — backend e telas.

**Architecture:** Presença e convites vivem em memória no `Lobby` (mesmo mutex de `clients`/`index`), com três ganchos novos na sala (`Join.Invite`, `Vacate`, `Registry.Seated`) para que o aceite seja decidido pelo ator da sala, que é a autoridade sobre os lugares. Ranking, histórico, perfil e diretório são leituras SQL no `stats.Reader`; o motivo do fim vira a coluna `matches.end_reason` (migração 0004 com backfill). Admin só concede/retira admin pelo CLI; o serviço `admin` recusa agir sobre admins e derruba a conexão WS de quem é desativado por uma porta implementada pelo `Lobby`. No frontend, duas stores novas (`presence`, `invites`) recebem as mensagens WS pelo `wire.ts`; as telas novas passam por stores que encapsulam `@/api/*`.

**Tech Stack:** Go 1.27 (`net/http`, pgx/sqlc, goose, testcontainers), Vue 3 + Pinia + Vue Router, Tailwind v4 com os tokens do design brief, Vitest, Playwright (Chrome do sistema).

**Spec:** `docs/superpowers/specs/2026-10-21-truco-social-ranking-admin-design.md` (leia junto). Fonte da verdade visual: `docs/superpowers/specs/2026-10-20-truco-frontend-design-brief.md`.

## Global Constraints

- Textos ao usuário, comentários, logs e commits em **pt-BR com acentos**; identificadores em inglês, exceto nomes de views/componentes em pt-BR (padrão da 4a).
- Commits convencionais em pt-BR, **um por tarefa**, **sem** linhas de atribuição. Nada de push, merge ou PR.
- Ao terminar cada tarefa: `cd backend && go test ./...` (se tocou backend) e `cd frontend && pnpm typecheck && pnpm lint && pnpm test` verdes. Se mudou SQL: `make gen-sql`; se mudou `internal/protocol`: `make gen-types` e golden (`cd backend && go test ./internal/protocol -run Golden -update`).
- **Nunca** edite `frontend/src/protocol/*` à mão nem `backend/internal/db/dbgen/*`.
- **Mensagem WS nova:** constante/payload em `protocol/client.go` ou `server.go`, entrada em `registry.go`, `case` próprio em `Lobby.Dispatch` (senão cai em `not_in_room`), `make gen-types`, golden atualizado e `case` no `dispatch` exaustivo de `frontend/src/app/wire.ts` (sem ele o `pnpm typecheck` quebra). `ProtocolVersion` continua `1` (mudança aditiva).
- TypeScript fixado em `~6.0.3`, `strict` + `noUncheckedIndexedAccess`. Fora de testes: sem `any`, sem `!` não-nulo (use guarda/`??`/`?.`), sem `delete obj[k]` dinâmico (reatribua com `Object.fromEntries(filter)`).
- Componentes e views **nunca** importam `@/ws/*` nem `@/api/*`: passam por stores. Erros viram toast com `useToastsStore().fromError(err)`; nunca `.catch(() => {})` silencioso.
- **Um dono de anúncios:** o `Anunciador` é a única região `aria-live` de anúncios; o aviso de convite não cria região viva própria (Toasts mantém as suas, da 4a).
- `localStorage` sempre em `try/catch` com padrão quando indisponível.
- CSP sem recursos externos: sons, fontes e imagens locais. Som novo só **CC0 da Kenney**, OGG + MP3, total da pasta `src/assets/sfx` **< 300 KB** (hoje ~256 KB).
- **Visual (design brief):** só utilidades/tokens do brief; a paleta padrão do Tailwind não existe (`text-white`, `bg-red-500`…); nada de hex cru em componente; **Kalam só** em placar/tentos/valor/anúncios/botão de aposta; títulos em Atkinson 700; laranja só aposta, vermelho só perigo; cor nunca é o único sinal; alvos clicáveis **≥ 44 px** no celular; um primário por tela/painel; sentence case.
- **Tarefas de tela (T8–T14)** fecham com o ciclo de revisão visual antes do commit: (1) subir backend `cd backend && set -a && . ../.env && set +a && APP_BASE_URL=http://localhost:5173 MAIL_DRIVER=log go run ./cmd/server serve` com `setsid` e frontend `cd frontend && pnpm dev` (reutilize servidores já de pé; mate só o grupo de processos que você iniciou e confira `ss -ltnp | grep -E ':8080|:5173'`); (2) screenshots com `playwright-core` instalado **no scratchpad** (nunca no repo) e `executablePath: '/usr/bin/google-chrome'`, em **1440×900, 1366×768, 375×812 e 375×667**, claro e escuro, estados vazio/carregando/erro/cheio que a tela tiver (pode usar `page.route` ou `window.__pinia` só no script), salvos em `<scratchpad>/telas/<tarefa>/<tela>-<estado>-<largura>-<tema>.png`; (3) `impeccable` critique → corrigir → polish → novos screenshots (o brief faz o papel de PRODUCT.md: não rode `init`; `detect.mjs` fica em `~/.claude/skills`); (4) `fixing-accessibility` e `ui-a11y` (foco visível, teclado, rótulos, contraste); (5) relatório com os caminhos dos screenshots finais e o que a crítica mudou.
- **Banco de dev:** o banco `truco` no Postgres do projeto `infra` em `:5432` (use `make migrate` após a T1). **Nunca** `make dev-db`, nunca mexer em containers alheios.
- **E2E:** só `scripts/e2e.sh` (compose `truco-e2e`, Postgres `:55432`, servidor `:8090`, `MAIL_DRIVER=log`, `TRUST_PROXY=true` com `X-Forwarded-For` por jogador porque o cadastro limita 5/h/IP, timers no mínimo de 5 s, Playwright com `channel: 'chrome'`).
- **Skills:** cada tarefa lista em "Skills" o que carregar antes do primeiro passo; ao fim de toda tarefa que mexe em `.vue`/`.ts`, `lint-and-validate`. Conflito skill × spec/brief: vale a spec/brief, e o conflito vai no corpo do commit.

## Review Focus

Cinco entradas que a spec implica e que os testes "do caminho feliz" não pegariam; cada uma tem teste na tarefa dona:

1. **Aceite no mesmo instante em que a sala enche** (dois convidados aceitam a última vaga, ou um aceita enquanto outro entra pelo código): um senta, o outro recebe `invite_expired`, não fica preso em reserva (`index` liberado, volta a receber `lobby_snapshot`) e o aviso some — testes `TestJoinByInviteOnFullRoomExpires` (T6) e `TestTwoAcceptsForLastSeat` (T7).
2. **Bordas de semana/mês em `America/Sao_Paulo`**: partida encerrada domingo 23:30 de Brasília (segunda 02:30 UTC) conta na semana anterior; semana que atravessa a antiga virada de horário (nov/2018) começa na segunda 00:00 local certa — testes `TestPeriodStartBoundaries` e `TestRankingWeekBoundary` (T3).
3. **Link de convite aberto por outra conta logada** (e-mail encaminhado): `invite_not_yours`, ninguém senta, o convite continua pendente para o convidado; a tela mostra "Este convite é para outra pessoa" — `TestTokenOpenedByAnotherAccount` (T7) e `mostra convite de outra pessoa` (T11).
4. **Usuário desativado ainda online** (com WS aberto, numa sala de espera e com convites pendentes): a conexão cai com 4003 na hora, ele some da presença, os convites pendentes para ele expiram e a lista "Convites" de quem convidou atualiza; convidá-lo de novo dá `user_not_found` — `TestDropUserLeavesPresence` (T5) e `TestDropUserExpiresInvites` (T7).
5. **Reconexão com convite pendente** (recarregar a aba, rede piscar): o servidor reenvia o `invite_received` e o cliente mostra **um** aviso (sem duplicar), e o aceite enviado depois funciona — `TestReconnectResendsPendingInvites` (T7) e `não duplica aviso reenviado na reconexão` (T7, store).

---

## Mapa de arquivos

| Arquivo | Responsabilidade | Tarefa |
|---|---|---|
| `backend/internal/db/migrations/0004_end_reason.sql`, `internal/db/db.go`, `internal/db/queries/matches.sql`, `internal/replay/replay.go`, `internal/match/store.go`, `internal/admin/admin.go` (abort) | motivo do fim + backfill | 1 |
| `internal/db/queries/ranking.sql`, `internal/db/queries/users.sql`, `internal/stats/reader.go`, `internal/pagination/cursor.go`, `internal/httpapi/read_handlers.go` | histórico/perfil enriquecidos, diretório | 2 |
| `internal/stats/period.go`, `internal/stats/reader.go`, `internal/db/queries/ranking.sql`, `cmd/server/main.go` | ranking por período/métrica, `me`, posições | 3 |
| `internal/admin/admin.go`, `internal/db/queries/admin.sql`, `internal/httpapi/{admin_handlers,respond,server}.go`, `cmd/server/main.go` | admin endurecido, CLI `demote-admin`, partidas em andamento | 4 |
| `internal/protocol/{server,registry}.go`, `internal/lobby/{lobby,presence}.go`, `internal/transport/ws/client.go`, `frontend/src/stores/presence.ts`, `frontend/src/app/wire.ts` | presença ponta a ponta, `DropUser` | 5 |
| `internal/room/{msgs,ports,seats,room}.go`, `internal/protocol/errors.go` | aceite por convite na sala | 6 |
| `internal/protocol/{client,server,registry}.go`, `internal/lobby/{invites,people}.go`, `internal/auth/invite.go`, `internal/mail/templates.go`, `frontend/src/stores/invites.ts`, `frontend/src/game/labels.ts` | convites no servidor + store | 7 |
| `frontend/src/components/lobby/OnlineAgora.vue`, `src/views/LobbyView.vue`, `src/stores/room.ts` | "Online agora" | 8 |
| `src/api/social.ts`, `src/stores/players.ts`, `src/components/sala/{SeletorPessoas,ListaConvites}.vue`, `SalaEspera.vue`, `Assento.vue` | convidar na sala de espera | 9 |
| `src/components/convite/{AvisosConvite,AvisoConvite}.vue`, `src/components/AppShell.vue`, `src/components/mesa/{Anunciador,Mesa}.vue`, `src/game/tabTitle.ts`, `src/app/{inviteAlerts,notifications}.ts`, `src/audio/{sfx,install}.ts`, `src/assets/sfx/invite.*`, `src/stores/prefs.ts`, `src/views/PerfilView.vue` | aviso de convite e aba em segundo plano | 10 |
| `src/views/EntrarConviteView.vue`, `src/router.ts` | `/entrar-convite/:token` | 11 |
| `src/stores/ranking.ts`, `src/views/RankingView.vue`, `src/router.ts`, `AppShell.vue` | ranking | 12 |
| `src/views/{JogadorView,PartidasView}.vue`, `src/components/partidas/LinhaPartida.vue`, `src/stores/players.ts` | perfil público e histórico | 13 |
| `src/api/admin.ts`, `src/api/http.ts`, `src/stores/admin.ts`, `src/views/admin/*` | painel admin | 14 |
| `frontend/e2e/{helpers.ts,social.spec.ts,partida.spec.ts}`, `README.md` | e2e e documentação | 15 |

---

### Task 1: Motivo do fim (`end_reason`) com backfill

**Skills:** `postgresql`, `database-migration`.

**Files:**
- Create: `backend/internal/db/migrations/0004_end_reason.sql`
- Modify: `backend/internal/db/db.go`, `backend/internal/db/queries/matches.sql`, `backend/internal/replay/replay.go`, `backend/internal/match/store.go`, `backend/internal/admin/admin.go` (`AbortMatch`)
- Test: `backend/internal/db/db_test.go`, `backend/internal/match/store_test.go`, `backend/internal/admin/admin_test.go`

**Interfaces:**
- Produces: `replay.EndReason` (`string`) com `replay.EndGameOver = "game_over"`, `EndForfeit = "forfeit"`, `EndAborted = "aborted"`, `EndAbandoned = "abandoned"`; coluna `matches.end_reason` (`NULL` só em `in_progress`); `match.AbandonWith(ctx, q *dbgen.Queries, id, reason string, end replay.EndReason, now time.Time) error`; `match.Summary.EndReason *replay.EndReason` (`json:"endReason"`); `db.MigrateDownTo(ctx, pool, version int64) error`.

- [ ] **Step 1: Testes que falham**

`db_test.go` — `TestEndReasonBackfill`: `MigrateDownTo(ctx, pool, 3)`; insere com SQL cru (users via `testdb.CreateUser`) quatro partidas: (a) `finished` com uma ação `{"kind":"sys_forfeit","seat":0,"team":1}` `source='system'`; (b) `finished` sem `sys_forfeit`; (c) `abandoned` com `abandon_reason='qualquer texto'` e linha em `audit_log` `action='match.abort'`, `target_id=<id>`; (d) `abandoned` com `abandon_reason='abortada pelo administrador'` **sem** auditoria; e (e) `in_progress`. `db.Migrate` e então:

```go
want := map[string]*string{a: ptr("forfeit"), b: ptr("game_over"), c: ptr("aborted"), d: ptr("abandoned"), e: nil}
// e um INSERT de partida finished com end_reason NULL deve violar o CHECK novo
```

`store_test.go` — `TestFinishRecordsEndReason` (partida completa → `game_over`; partida encerrada por `ActSysForfeit` → `forfeit`, via `Summary(...).EndReason`) e `TestAbandonRecordsEndReason` (`Abandon` → `abandoned`). `admin_test.go` — em `TestAbortMatch`, `Summary(...).EndReason` = `aborted`.

Run: `cd backend && go test ./internal/db/ ./internal/match/ ./internal/admin/`
Expected: FAIL (`MigrateDownTo` indefinido, coluna inexistente).

- [ ] **Step 2: Migração 0004**

`ALTER TABLE matches ADD COLUMN end_reason text CHECK (end_reason IN ('game_over','forfeit','aborted','abandoned'))`; backfill nesta ordem: `forfeit` (finished com `EXISTS` em `match_actions` `source='system' AND action->>'kind'='sys_forfeit'`), `game_over` (demais finished), `aborted` (abandoned com `EXISTS` em `audit_log` `action='match.abort' AND target_id = matches.id::text`), `abandoned` (demais abandoned); por fim `ADD CONSTRAINT matches_end_reason_check CHECK ((status = 'in_progress') = (end_reason IS NULL))`. **Nunca** usar `abandon_reason` no backfill. Down: `DROP CONSTRAINT` + `DROP COLUMN`.

- [ ] **Step 3: Código**

`db.MigrateDownTo` usa `p.DownTo(ctx, version)` via `withProvider`. Queries `FinishMatch` e `AbandonMatch` ganham `end_reason = @end_reason::text`. `Store.Finish` grava `forfeit` quando a última ação de `rec.Actions` tem `Kind == engine.ActSysForfeit`, senão `game_over`. `Store.Abandon` passa `replay.EndAbandoned`; `admin.AbortMatch` passa `replay.EndAborted`. `Summary` lê a coluna. `make gen-sql`.

- [ ] **Step 4: Verificar**

Run: `make gen-sql && cd backend && go test ./...`
Expected: PASS. Depois `make migrate` no banco de dev.

- [ ] **Step 5: Commit**

```bash
git add backend
git commit -m "feat(partidas): grava o motivo do fim (end_reason) com backfill das partidas antigas"
```

---

### Task 2: Histórico e perfil enriquecidos, diretório de pessoas

**Skills:** `postgresql`, `api-design-principles`.

**Files:**
- Modify: `backend/internal/db/queries/ranking.sql`, `backend/internal/db/queries/users.sql`, `backend/internal/pagination/cursor.go`, `backend/internal/stats/reader.go`, `backend/internal/httpapi/read_handlers.go`
- Test: `backend/internal/stats/reader_test.go`, `backend/internal/pagination/cursor_test.go`, `backend/internal/httpapi/read_handlers_test.go`

**Interfaces:**
- Consumes: `replay.EndReason`, coluna `end_reason` (T1).
- Produces:
  - `stats.MatchPlayer{Seat int "seat"; Username string "username"; DisplayName string "displayName"}`
  - `stats.HistoryItem` ganha `Players []MatchPlayer "players"` e `EndReason replay.EndReason "endReason"` (mantém `id, status, team, won, winnerTeam, score, createdAt, finishedAt`).
  - `stats.Partner{Username, DisplayName string; Matches, Wins int; WinRate float64}` (JSON `username, displayName, matches, wins, winRate`).
  - `stats.Profile` ganha `RecentMatches []HistoryItem "recentMatches"` (5) e `Partners []Partner "partners"` (até 5); slices nunca `nil` no JSON.
  - `(*Reader).Profile(ctx, username string, viewerIsAdmin bool) (Profile, error)` e `(*Reader).History(ctx, username, cursor string, limit int, viewerIsAdmin bool) (HistoryPage, error)` — desativado → `ErrUserNotFound` (404) para não-admin.
  - `stats.DirectoryEntry{Username, DisplayName string}`, `stats.DirectoryPage{Items []DirectoryEntry "items"; NextCursor string "nextCursor,omitempty"}`, `(*Reader).Directory(ctx, query, cursor string) (DirectoryPage, error)` (20 por página, ordem por `username`).
  - `pagination.EncodeKey(key string) string`, `pagination.DecodeKey(cursor string) (string, error)` (base64url; `""` → `""`; inválido → `ErrInvalidCursor`).
  - `GET /api/users?q=&cursor=` → `DirectoryPage` (autenticado).

- [ ] **Step 1: Testes que falham**

`reader_test.go`:
- `TestHistoryHasPlayersAndEndReason`: duas partidas (uma por W.O. via log com `sys_forfeit`, uma normal); itens trazem 4 `players` ordenados por `seat` com `username` e `endReason` `forfeit`/`game_over`; partida abortada pelo admin aparece com `endReason == "aborted"` e `won == false`, `winnerTeam == nil`.
- `TestProfileRecentMatchesAndPartners`: após 6 partidas com `ana` e `caio` na mesma dupla (assentos 0 e 2) em 4 delas, `RecentMatches` tem 5 itens (mais recente primeiro) e `Partners[0] == {Username: "caio", Matches: 4, Wins: <contado do match_player_stats>, ...}`; `len(Partners) <= 5`; parceiro desativado não aparece.
- `TestDisabledProfileIsNotFoundForNonAdmin`: `Profile(ctx, "duda", false)` → `ErrUserNotFound`; `Profile(ctx, "duda", true)` ok; idem `History`.
- `TestDirectorySearchHidesEmailAndDisabled`: busca `"an"` acha `ana` por username e `"Fulana"` acha pelo nome de exibição (case-insensitive); desativado e e-mail não verificado ficam de fora; o JSON de um item tem exatamente as chaves `username` e `displayName`; paginação de 25 usuários → 20 + `nextCursor`, segunda página 5 sem cursor.

`cursor_test.go`: `TestKeyCursorRoundTrip` (`DecodeKey(EncodeKey("ana.b")) == "ana.b"`; lixo → `ErrInvalidCursor`).

`read_handlers_test.go`: atualize `TestDisabledUserHiddenFromRankingButKeepsProfile` para o novo comportamento (perfil 404 para não-admin, 200 para admin) e acrescente `TestDirectoryRoute` (401 sem sessão; 200 com `items`).

Run: `cd backend && go test ./internal/stats/ ./internal/pagination/ ./internal/httpapi/`
Expected: FAIL.

- [ ] **Step 2: SQL**

Em `ranking.sql`: `ListUserMatches` passa a selecionar `m.end_reason`; nova `ListPlayersForMatches(@ids::uuid[])` (seat, username, display_name, match_id) para evitar N+1; nova `ListPartners(@user_id, @lim)`:

```sql
SELECT u.username, u.display_name, count(*)::integer AS matches,
       count(*) FILTER (WHERE s.won)::integer AS wins
FROM match_players me
JOIN match_players p ON p.match_id = me.match_id AND p.seat = (me.seat + 2) % 4
JOIN matches m ON m.id = me.match_id AND m.status = 'finished'
JOIN match_player_stats s ON s.match_id = me.match_id AND s.user_id = me.user_id
JOIN users u ON u.id = p.user_id AND u.disabled_at IS NULL
WHERE me.user_id = @user_id
GROUP BY u.username, u.display_name
ORDER BY matches DESC, wins DESC, u.username
LIMIT @lim::integer;
```

Em `users.sql`: `SearchDirectory(@query, @after, @lim)` — `disabled_at IS NULL AND email_verified_at IS NOT NULL`, `username LIKE` ou `lower(display_name) LIKE` (escape com o `likeEscaper` do padrão do admin), `username > @after`, `ORDER BY username`. `make gen-sql`.

- [ ] **Step 3: Reader e rota**

Implemente as assinaturas do bloco Interfaces; `Profile` chama `History(..., 5, ...)` e `ListPartners(..., 5)`. Handlers: `handleProfile`/`handleHistory` passam `p.IsAdmin`; `registerReadRoutes` ganha `GET /api/users` → `s.handleDirectory` (lê `q` e `cursor`).

- [ ] **Step 4: Verificar**

Run: `make gen-sql && cd backend && go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend
git commit -m "feat(stats): histórico com jogadores e motivo do fim, perfil com recentes e parceiros, diretório de pessoas"
```

---

### Task 3: Ranking por período e métrica, com `me` e posições no perfil

**Skills:** `postgresql`.

**Files:**
- Create: `backend/internal/stats/period.go`, `backend/internal/stats/period_test.go`
- Modify: `backend/internal/db/queries/ranking.sql`, `backend/internal/stats/reader.go`, `backend/internal/httpapi/read_handlers.go`, `backend/internal/httpapi/harness_test.go`, `backend/cmd/server/main.go`
- Test: `backend/internal/stats/reader_test.go`, `backend/internal/httpapi/read_handlers_test.go`

**Interfaces:**
- Consumes: `stats.Profile` (T2).
- Produces:
  - `stats.Period` (`"all" | "month" | "week"`), `stats.Metric` (`"wins" | "winrate"`), `stats.ErrInvalidRankingQuery` (→ 400 `invalid_request`).
  - `stats.PeriodStart(p Period, now time.Time) *time.Time` — `nil` para `all`; segunda 00:00 (semana) ou dia 1 00:00 (mês) em `America/Sao_Paulo`. O pacote importa `_ "time/tzdata"` e carrega o fuso uma vez.
  - `stats.NewReader(pool *pgxpool.Pool, minMatches int, now func() time.Time) *Reader` (`nil` → `time.Now`).
  - `stats.RankingQuery{Period Period; Metric Metric; Limit int; ViewerID string}`; `stats.RankingResult{Items []RankingEntry "items"; Me *RankingEntry "me"; MinMatches int "minMatches"}`; `(*Reader).Ranking(ctx, RankingQuery) (RankingResult, error)`.
  - `stats.PositionPair{Wins *int "wins"; WinRate *int "winrate"}`, `stats.Positions{All, Month, Week PositionPair}` (JSON `all, month, week`); `stats.Profile.Positions Positions "positions"`.
  - `GET /api/ranking?period=&metric=&limit=` (padrões `all`/`wins`) → `RankingResult`.

- [ ] **Step 1: Testes que falham**

`period_test.go` — `TestPeriodStartBoundaries` (tabela; `sp := time.LoadLocation("America/Sao_Paulo")`):

| now | período | início esperado |
|---|---|---|
| `2026-10-19 00:00 -03` (segunda) | week | `2026-10-19 00:00 -03` |
| `2026-10-19 02:30 UTC` (domingo 23:30 -03) | week | `2026-10-12 00:00 -03` |
| `2026-11-01 02:59 UTC` (31/10 23:59 -03) | month | `2026-10-01 00:00 -03` |
| `2018-11-07 12:00 -02` (horário de verão) | week | `2018-11-05 00:00 -02` |
| `2019-02-18 10:00 -03` (dia seguinte ao fim do verão) | week | `2019-02-18 00:00 -03` |
| qualquer | all | `nil` |

`reader_test.go` (com `world.clock`):
- `TestRankingWeekBoundary`: partida finalizada às `2026-10-19 02:30 UTC` e outra às `2026-10-19 03:00 UTC`; com `now = 2026-10-21`, `week` conta só a segunda; `month` conta as duas.
- `TestRankingWinsHasNoMinimumWinrateHas`: com `minMatches = 3`, um jogador com 1 partida aparece em `wins` e não em `winrate`; `MinMatches == 3` na resposta; ordem de `wins` por vitórias → aproveitamento → partidas → username; de `winrate` por aproveitamento → vitórias → partidas → username.
- `TestRankingMeOutsideTop`: `Limit: 2`, `ViewerID` do 4º colocado → `len(Items) == 2` e `Me.Position == 4`; viewer sem partidas no período → `Me == nil`.
- `TestRankingIgnoresAbandonedAndDisabled`: partida abortada não conta em nenhum período; desativado some dos três períodos.
- `TestProfilePositions`: `Positions.Week.WinRate == nil` para quem não atinge o mínimo na semana; `Positions.All.Wins` igual ao `Position` do ranking geral.

`read_handlers_test.go`: `TestRankingQueryValidation` (`period=ano` → 400 `invalid_request`; sem parâmetros → 200 com `items`, `me`, `minMatches`).

Run: `cd backend && go test ./internal/stats/ ./internal/httpapi/`
Expected: FAIL.

- [ ] **Step 2: SQL**

Substitua `Ranking` por `RankingByWins` e `RankingByWinRate`, ambas sobre `match_player_stats ⨝ matches` (`status = 'finished'`, `finished_at >= sqlc.narg(since)` quando não nulo) ⨝ `users` (`disabled_at IS NULL`), agregando por usuário num CTE, numerando com `row_number() OVER (ORDER BY …)` na ordem do Step 1, filtrando `HAVING count(*) >= @min_matches` (1 em `wins`, `RANKING_MIN_MATCHES` em `winrate`) e devolvendo `WHERE pos <= @lim OR user_id = sqlc.narg(viewer)`. Adicione `CREATE INDEX` só se `EXPLAIN` no banco de teste mostrar varredura cara (o `matches_closed_idx` já cobre `finished_at`); se criar, é migração `0005`. `make gen-sql`.

- [ ] **Step 3: Implementar**

`Ranking` separa a linha do viewer de `Items` quando ela está fora do top (`Me` sempre preenchido se o viewer está ranqueado, inclusive dentro do top). `Profile` preenche `Positions` com seis chamadas (`Limit: 0` não traz itens, só `Me`). Atualize as chamadas de `NewReader` em `cmd/server/main.go` e `httpapi/harness_test.go` (passando `h.clock.Now`).

- [ ] **Step 4: Verificar**

Run: `make gen-sql && cd backend && go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend
git commit -m "feat(ranking): períodos geral/mês/semana em horário de Brasília, métricas vitórias e aproveitamento e posição do usuário"
```

---

### Task 4: Admin endurecido

**Skills:** `backend-security-coder`, `postgresql`.

**Files:**
- Modify: `backend/internal/admin/admin.go`, `backend/internal/db/queries/admin.sql`, `backend/internal/db/queries/matches.sql`, `backend/internal/httpapi/{admin_handlers,respond,server,read_handlers}.go`, `backend/cmd/server/main.go`
- Create: `backend/internal/httpapi/export_test.go`
- Test: `backend/internal/admin/admin_test.go`, `backend/internal/httpapi/admin_handlers_test.go`, `backend/cmd/server/main_test.go`

**Interfaces:**
- Consumes: `match.PlayerSummary`, `ListPlayersForMatches` (T2).
- Produces:
  - `admin.Service.SetAdmin` **removido**; `admin.ErrTargetIsAdmin` (→ 403 `target_is_admin`).
  - `admin.SessionDropper interface{ DropUser(userID string) }`, `admin.NoopDropper`, `admin.Options.Dropper SessionDropper` (o `Lobby` implementa na T5).
  - `admin.DemoteAdmin(ctx, pool *pgxpool.Pool, login string, now time.Time) error` (`ErrLastAdmin` se o alvo é o último admin ativo); `PromoteAdmin` inalterado.
  - `admin.LiveMatch{ID string "id"; CreatedAt time.Time "createdAt"; Players []match.PlayerSummary "players"}`, `admin.MatchPage{Items []LiveMatch "items"; NextCursor string "nextCursor,omitempty"}`, `(*Service).ListInProgressMatches(ctx, cursor string) (MatchPage, error)` (mais antiga primeiro, 50 por página).
  - Rotas: `GET /api/admin/matches?status=in_progress&cursor=` (outro `status` → 400); `PUT /api/admin/users/{id}/admin` deixa de existir (404).
  - `(*Server).Patterns() []string` só para testes (via `export_test.go`); `Server` registra rotas por `s.handle(pattern string, h http.Handler)`, que guarda o padrão.
  - CLI: `server demote-admin <email|username>`.

- [ ] **Step 1: Testes que falham**

`admin_handlers_test.go`:
- `TestNoRouteChangesAdminFlag`: em `New(deps).Patterns()`, nenhum padrão casa com ``regexp.MustCompile(`(?i)/admin$|is_?admin|promote|demote`)`` (o prefixo `/api/admin/` não casa); e, logado como admin, `PUT`, `PATCH` e `POST` em `/api/admin/users/<id>/admin` respondem 404 e o alvo continua sem `is_admin`.
- Em `TestAdminRoutesRequireAdmin`, troque a linha do `PUT …/admin` por `GET /api/admin/matches` (403 para comum, 401 anônimo).
- `TestAdminCannotActOnAdmin`: com dois admins, `disable`, `enable` e `force-password-reset` sobre o outro admin → 403 `target_is_admin`, sem linha nova em `audit_log`.
- `TestAdminInProgressMatches`: partida em andamento aparece com 4 `players`; `status=finished` → 400.

`admin_test.go`: remova `TestSetAdminProtectsLastAdmin`; `TestDemoteAdminKeepsLastAdmin` (último admin ativo → `ErrLastAdmin`; com dois, rebaixa e audita `user.demote_admin` com `details.via == "cli"`); `TestDisableDropsConnection` (`recordingDropper` recebe o id só depois do commit; com o alvo admin, não recebe).

`main_test.go`: em `TestMigratePromoteAndRecompute`, `run(ctx, []string{"demote-admin", "ana"}, …)` com um único admin → erro contendo "ao menos um administrador"; após promover outro, sucesso e saída `"ana não é mais administrador"`.

Run: `cd backend && go test ./internal/admin/ ./internal/httpapi/ ./cmd/server/`
Expected: FAIL.

- [ ] **Step 2: Implementar**

- `DisableUser`, `EnableUser`, `ForcePasswordReset`: dentro da transação, `GetUserByID` → `IsAdmin` → `ErrTargetIsAdmin` antes de qualquer escrita (a regra mora no serviço, não no handler). `DisableUser` chama `s.dropper.DropUser(userID)` após o commit.
- `DemoteAdmin`: mesma transação com `LockAdmins` + `CountActiveAdmins`; audita com ator = o próprio alvo (a FK de `audit_log.actor_id` exige usuário) e `details {"via":"cli"}`, como o `PromoteAdmin` existente.
- `ListInProgressMatches`: query nova `ListInProgressMatches(cursor_created_at, cursor_id, lim)` em `matches.sql` + `ListPlayersForMatches`.
- `httpapi`: remova `handleAdminSetAdmin`/`setAdminRequest`; `classify` mapeia `ErrTargetIsAdmin`; `cmd/server/main.go` aceita `demote-admin` (mensagens e uso no comentário do pacote).

- [ ] **Step 3: Verificar**

Run: `make gen-sql && cd backend && go test ./...`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add backend
git commit -m "feat(admin)!: admin só pelo CLI (demote-admin), admin não age sobre admin e lista partidas em andamento"
```

---

### Task 5: Presença ponta a ponta e `DropUser`

**Skills:** `backend-dev-guidelines`, `typescript-expert`.

**Files:**
- Create: `backend/internal/lobby/presence.go`, `backend/internal/lobby/presence_test.go`, `frontend/src/stores/presence.ts`, `frontend/src/stores/presence.test.ts`
- Modify: `backend/internal/protocol/{server,registry}.go`, `backend/internal/lobby/lobby.go`, `backend/internal/transport/ws/client.go`, `backend/cmd/server/main.go`, `frontend/src/app/wire.ts`, `frontend/src/app/wire.test.ts`, `frontend/src/game/labels.ts`
- Generated: `frontend/src/protocol/{messages,union}.ts`, `backend/internal/protocol/testdata/union.ts.golden`

**Interfaces:**
- Consumes: `admin.SessionDropper` (T4).
- Produces:
  - Protocolo: `PresenceStatus` (`"lobby" | "waiting" | "playing"`), `PresenceEntry{UserID "userId"; Username "username"; DisplayName "displayName"; Status "status"; RoomName string "roomName,omitempty"; PrivateRoom bool "privateRoom,omitempty"}`; mensagens `presence_snapshot{users: PresenceEntry[]}`, `presence_updated{user: PresenceEntry}`, `presence_removed{userId}`; construtores `protocol.PresenceSnapshot([]PresenceEntry)`, `PresenceUpdated(PresenceEntry)`, `PresenceRemoved(string)`.
  - `lobby.CloseRevoked` (nova `CloseReason`; o WS fecha com **4003**); `(*Lobby).DropUser(userID string)` (implementa `admin.SessionDropper`).
  - Frontend: `usePresenceStore` com state `{ users: Record<string, PresenceEntry>, loaded: boolean }`, getters `list: PresenceEntry[]` (ordem: `lobby`, `waiting`, `playing`, depois nome em `pt-BR`), `count: number`, `byUsername(username): PresenceEntry | undefined`; ações `onSnapshot`, `onUpdated`, `onRemoved`, `$reset`. `labels.presenceText(e: PresenceEntry): string` → `"no lobby"`, `"esperando · Mesa do RH"`, `"esperando · sala privada"`, `"jogando · Truco da TI"`.

- [ ] **Step 1: Testes Go que falham**

`presence_test.go` (usa `newLobby`/`connect`/`dispatch` de `lobby_test.go`):
- `TestPresenceSnapshotOnConnect`: todo `Connect` recebe `presence_snapshot` com todos os conectados (inclusive quem está em sala), e os demais recebem `presence_updated` do novo.
- `TestPresenceTransitions`: criar sala → `waiting` com `roomName` (pública) ou `privateRoom: true` e sem `roomName` (privada); os quatro prontos → `playing`; sair → `lobby`; `Disconnect` → `presence_removed`. Uma transição gera **um** `presence_updated` por usuário afetado (sem repetir entrada idêntica).
- `TestDropUserLeavesPresence` (Review Focus 4): usuário numa sala de espera; `DropUser` fecha o cliente com `CloseRevoked`, os outros recebem `presence_removed` na hora, a sala recebe `Disconnected` (assento marcado desconectado) e um `Disconnect` posterior do mesmo cliente é ignorado.
- `ws/handler_test.go`: `TestRevokedClosesWith4003`.

Run: `cd backend && go test ./internal/lobby/ ./internal/transport/ws/`
Expected: FAIL.

- [ ] **Step 2: Implementar backend**

`presence.go`: `entryLocked(userID) (PresenceEntry, bool)` deriva o status de `clients` + `index` + `rooms[id].summary` (`Status`, `Public`, `Room.Name`); `touchPresenceLocked(userIDs ...string)` compara com o último enviado (`l.lastPresence map[string]protocol.PresenceEntry`) e transmite `presence_updated` a **todos** os conectados só quando mudou. Chame-o em `Connect`, nas reservas de `createRoomLocked`/`joinRoomLocked`, em `Released`, `Destroyed` (usuários liberados) e em `Update` quando o `Status` da sala mudou (membros = usuários com `index == roomID`). `Disconnect` e `DropUser` apagam `lastPresence` e transmitem `presence_removed`. Sem debounce (mudanças são raras; alvo < 1 s). `ws/client.go`: `Close(CloseRevoked)` → `closeWith(codeSessionInvalid, "session_invalid")`. `main.go`: `admin.Options{…, Dropper: lob}`.

- [ ] **Step 3: Gerar tipos**

Run: `make gen-types && cd backend && go test ./internal/protocol -run Golden -update && go test ./...`
Expected: PASS; `pnpm typecheck` agora falha no `dispatch` (casos ausentes) — esperado até o Step 5.

- [ ] **Step 4: Testes do frontend que falham**

`presence.test.ts`:

```ts
it('snapshot, atualização e remoção', () => {
  const s = usePresenceStore()
  s.onSnapshot({ users: [ana('lobby'), bia('playing')] })
  s.onUpdated({ user: { ...bia('waiting'), roomName: 'Mesa do RH' } })
  s.onRemoved({ userId: 'u-ana' })
  expect(s.list.map((e) => e.username)).toEqual(['bia'])
  expect(s.count).toBe(1)
  const [primeira] = s.list
  expect(primeira && presenceText(primeira)).toBe('esperando · Mesa do RH')
})
it('sala privada não mostra nome', () => {
  expect(presenceText({ ...bia('waiting'), privateRoom: true })).toBe('esperando · sala privada')
})
```

`wire.test.ts`: despacha `presence_snapshot`/`presence_updated`/`presence_removed` e confere a store; `stop()` (logout) zera a presença.

Run: `cd frontend && pnpm vitest run src/stores/presence.test.ts src/app/wire.test.ts`
Expected: FAIL.

- [ ] **Step 5: Implementar e verificar**

Store, `presenceText` e os três `case` no `dispatch` de `wire.ts`; `stop()` chama `usePresenceStore().$reset()`.

Run: `cd frontend && pnpm typecheck && pnpm lint && pnpm test`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend frontend/src
git commit -m "feat(presença): lista de quem está online com status e derrubada imediata de usuário desativado"
```

---

### Task 6: Aceite por convite na sala (ganchos do ator)

**Skills:** `backend-dev-guidelines`.

**Files:**
- Modify: `backend/internal/room/{msgs,ports,seats,room}.go`, `backend/internal/protocol/errors.go`, `backend/internal/lobby/lobby.go` (stub `Seated`), `backend/internal/room/harness_test.go` (fake `registry`)
- Test: `backend/internal/room/seats_test.go`

**Interfaces:**
- Produces:
  - `room.Join.Invite bool` — com `Invite`, lugar pedido ocupado cai no primeiro livre; sala cheia ou em jogo falha com `protocol.CodeInviteExpired` (em vez de `room_full`/`match_in_progress`) e chama `Registry.Released`.
  - `room.Vacate{UserID string; Refused func()}` — em espera: `removeSeat(seat, protocol.LeftLeft)` (que chama `Released`); em jogo: chama `Refused()` (se não-nil) sem mexer em nada; usuário não sentado: `Registry.Released(id, user)`.
  - `room.Registry.Seated(roomID, userID string)` — chamado em `join` logo após ocupar o assento (antes do `publish`).
  - Códigos em `protocol/errors.go` com textos pt-BR: `CodeInviteNotFound = "invite_not_found"`, `CodeInviteExpired = "invite_expired"`, `CodeInviteNotYours = "invite_not_yours"`, `CodeInviteePlaying = "invitee_playing"`, `CodeCannotInviteSelf = "cannot_invite_self"`, `CodeUserNotFound = "user_not_found"`.

- [ ] **Step 1: Testes que falham** (`seats_test.go`, com o harness da sala)

- `TestJoinByInviteFallsBackToFreeSeat`: lugar 2 ocupado, `Join{Seat: &2, Invite: true}` senta no primeiro livre e o fake registra `Seated`.
- `TestJoinByInviteOnFullRoomExpires` (Review Focus 1): sala com 4, `Join{Invite: true, Seq: 7}` → erro `invite_expired` com `seq 7` e `wasReleased(user)`; sem `Invite`, continua `room_full`.
- `TestVacate`: em espera, o usuário sai com `left_room{left}` e é liberado; em jogo, `Refused` é chamado uma vez e o assento continua ocupado.

Run: `cd backend && go test ./internal/room/`
Expected: FAIL.

- [ ] **Step 2: Implementar**

Campos/mensagens/método do bloco Interfaces; `Lobby.Seated` fica vazio nesta tarefa (a T7 preenche). Atualize o fake `registry` do `harness_test.go` para gravar `Seated`.

- [ ] **Step 3: Verificar**

Run: `cd backend && go test ./...`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add backend
git commit -m "feat(sala): entrada por convite, saída para aceitar convite e aviso de assento ocupado ao lobby"
```

---

### Task 7: Convites no servidor, e-mail e store do cliente

**Skills:** `backend-dev-guidelines`, `backend-security-coder`, `typescript-expert`.

**Files:**
- Create: `backend/internal/lobby/invites.go`, `backend/internal/lobby/invites_test.go`, `backend/internal/lobby/people.go`, `backend/internal/auth/invite.go`, `backend/internal/auth/invite_test.go`, `frontend/src/stores/invites.ts`, `frontend/src/stores/invites.test.ts`
- Modify: `backend/internal/protocol/{client,server,registry}.go`, `backend/internal/protocol/messages_test.go`, `backend/internal/lobby/{lobby,lobby_test}.go`, `backend/internal/mail/{templates,mail_test}.go`, `backend/cmd/server/main.go`, `backend/internal/httpapi/ws_e2e_test.go` (opções do lobby), `frontend/src/app/wire.ts`, `frontend/src/app/wire.test.ts`, `frontend/src/game/labels.ts`
- Generated: `frontend/src/protocol/*`, golden

**Interfaces:**
- Consumes: `room.Join.Invite`, `room.Vacate`, `Registry.Seated`, códigos de erro (T6); `touchPresenceLocked` (T5).
- Produces:
  - Cliente→servidor: `invite{username: string, seat?: Seat}` (`TypeInvite`), `invite_respond{inviteId, accept: boolean}` (`TypeInviteRespond`), `invite_accept_token{token}` (`TypeInviteAcceptToken`). Convida-se por **username** (o diretório da T2 não expõe ids).
  - Servidor→cliente: `invite_received{inviteId, room: {id, name, visibility}, from: {displayName}, seat?, freeSeats}`, `invite_expired{inviteId}` (também usado para retirar o aviso quando o convite deixa de valer para o convidado), `room_invites{invites: RoomInvite[]}` com `RoomInvite{inviteId, username, displayName, state: "pending" | "accepted" | "declined", emailed: boolean, seat?}` (convites expirados saem da lista).
  - `lobby.Invitee{ID, Username, DisplayName string}`, `lobby.ErrNoInvitee`, `lobby.People interface { Invitee(ctx, username string) (Invitee, error); MailInvite(inviteeID, inviterName, roomName, token string) }` (`MailInvite` nunca bloqueia); `lobby.Options.People People` (obrigatório: `lobby_test.go` e `ws_e2e_test.go` passam um fake); `lobby.NewPeople(a *auth.Service) People`.
  - `(*auth.Service).ActiveUserByUsername(ctx, username string) (User, error)` (inexistente, desativado ou não verificado → `ErrUserNotFound`); `(*auth.Service).SendRoomInvite(userID, inviterName, roomName, token string)` (busca o e-mail e usa `sendLater` com link `s.link("/entrar-convite/" + token)`).
  - `mail.RoomInvite(to, inviterName, roomName, link string) Message`.
  - Frontend: `useInvitesStore` state `{ received: InviteReceivedPayload[], roomInvites: RoomInvite[] }`; getter `visible` (até 3, mais novo primeiro); ações `invite(username: string, seat?: Seat): Promise<void>`, `respond(inviteId: string, accept: boolean): Promise<void>` (retira o aviso localmente antes de enviar), `acceptToken(token: string): Promise<void>`, `onReceived`, `onExpired`, `onRoomInvites`, `clearRoom()`, `$reset`. `labels.inviteStateText(i: RoomInvite)` → `"aceitou"`, `"aguardando…"`, `"e-mail enviado"` (pendente com `emailed`), `"agora não"`; textos de erro novos em `ERRORS` (ex.: `invite_expired: 'O convite expirou.'`, `invite_not_yours: 'Este convite é para outra pessoa.'`, `invitee_playing: 'Você está numa partida.'`).

**Algoritmo (no `Lobby`, sob o mutex):** estado `invites map[id]*invite` (`id, roomID, token, inviterName, invitee Invitee, seat *Seat, state, emailed`), `tokens map[token]id`, `mailed map[roomID]map[userID]bool`, `moves map[userID]move{inviteID, seq}`, `accepting map[userID]inviteID`. Ids e tokens: 16/32 bytes de `l.rand` em base64url.
1. `invite`: `Dispatch` decodifica o payload e chama `People.Invitee` **antes** de travar o mutex (goroutine de leitura do remetente, timeout 5 s). Depois valida, nesta ordem: remetente em sala (`not_in_room`), sala em espera (`match_in_progress`), com lugar (`room_full`), lugar pedido livre na `summary` (`seat_taken`), convidado existe (`user_not_found`), não é o remetente (`cannot_invite_self`), não está nesta sala (`already_in_room`). Já há pendente para (sala, convidado) → não faz nada (sem novo aviso, sem e-mail). Senão cria `pending`; online → `invite_received`; offline e ainda não mandou e-mail para (sala, convidado) → `People.MailInvite` e `emailed = true`. Transmite `room_invites` aos membros da sala.
2. `invite_respond{accept:false}` → `declined`, `room_invites`. `accept:true` ou `invite_accept_token`: convite inexistente/não pendente → `invite_expired` (por id de outro usuário → `invite_not_found`; por token de outro usuário pendente → `invite_not_yours`, sem tocar no convite). Convidado em sala `playing` → `invitee_playing` (convite segue pendente). Em sala de espera (outra) → grava `moves[user]` e envia `room.Vacate{Refused: …}` (recusa → apaga o move e responde `invitee_playing`). Já nesta sala → `accepted`. Fora de sala → reserva `index`, `accepting[user] = id` e `room.Join{Seat: inv.seat, Invite: true, Seq}`.
3. `Released(room, user)`: com `moves[user]` saindo desta sala → segue o passo de entrada; com `accepting[user]` desta sala → convite `expired`, `invite_expired{id}` ao convidado, `room_invites`.
4. `Seated(room, user)`: qualquer pendente de (sala, usuário) — inclusive quem entrou pelo código — vira `accepted`; limpa `accepting`; `invite_expired{id}` ao convidado para retirar o aviso (ex.: entrou pelo código); `room_invites`.
5. `Update`: sala `playing` ou sem lugar livre → todos os pendentes dela expiram (`invite_expired` a cada convidado). `Destroyed`: expira os da sala, apaga `mailed[room]`, cancela `moves` que apontavam para ela.
6. `Connect`: reenvia `invite_received` de cada pendente do usuário; se ele está em sala, manda `room_invites` dela. `DropUser`: expira os pendentes para ele (avisando as salas) e apaga `moves`/`accepting`.

- [ ] **Step 1: Testes Go que falham**

`mail_test.go`: `TestRoomInviteTemplate` — assunto `"Ana te chamou pra mesa no Truco"`, texto e HTML contêm `Mesa do RH` e o link; HTML escapa `<b>` no nome da sala.

`auth/invite_test.go`: `TestActiveUserByUsername` (desativado → `ErrUserNotFound`) e `TestSendRoomInviteLink` (o `mail.Fake` recebe link `https://truco.test/entrar-convite/<token>`).

`invites_test.go` (lobby com `fakePeople` que grava `MailInvite`):
- `TestInviteValidation`: cada erro do passo 1 na ordem, incl. `cannot_invite_self` e `already_in_room`.
- `TestInviteOnlineAndOffline`: online recebe `invite_received` com `freeSeats` certo e nenhum e-mail; offline gera exatamente um `MailInvite`; inviter vê `room_invites` com `emailed: true`.
- `TestOnePendingPerInviteeAndRoom`: convidar duas vezes não gera segundo aviso nem e-mail; após `declined`, novo convite gera novo `invite_received`; convidado offline reconvidado após recusa não recebe segundo e-mail.
- `TestAcceptMovesFromWaitingRoom`: convidado sentado em outra sala de espera aceita → recebe `left_room` da antiga, senta na nova (no lugar do convite) e a lista mostra `accepted`.
- `TestAcceptWhilePlayingIsRefused`: → `invitee_playing`; convite continua `pending`.
- `TestInviteExpiresOnFullStartAndClose`: encher, começar e fechar a sala mandam `invite_expired` aos convidados pendentes; convite **não** expira quando quem convidou sai e a sala continua com gente.
- `TestTwoAcceptsForLastSeat` (Review Focus 1): dois convidados aceitam com uma vaga; um senta, o outro recebe `invite_expired` (erro com seq e mensagem `invite_expired{id}`), fica fora de sala e recebe `lobby_snapshot`.
- `TestTokenOpenedByAnotherAccount` (Review Focus 3, spec §7e): `invite_accept_token` de outro usuário → `invite_not_yours`, ninguém senta e o convidado ainda aceita com o mesmo token depois.
- `TestUnknownTokenIsExpired`: token desconhecido (ex.: após restart) → `invite_expired`.
- `TestDropUserExpiresInvites` (Review Focus 4): `DropUser` do convidado → inviter recebe `room_invites` sem ele; novo `invite` com `fakePeople` devolvendo `ErrNoInvitee` → `user_not_found`.
- `TestReconnectResendsPendingInvites` (Review Focus 5): convidado reconecta (novo `Connect`) → recebe de novo o `invite_received` com o **mesmo** `inviteId`; `invite_respond` aceitando funciona.

`messages_test.go`: `TestDecodeInvitePayloads` (`invite` sem `username` falha; `invite_respond` exige `inviteId` e `accept`; `invite_accept_token` exige `token`).

Run: `cd backend && go test ./internal/mail/ ./internal/auth/ ./internal/lobby/ ./internal/protocol/`
Expected: FAIL.

- [ ] **Step 2: Implementar backend**

Template (assunto `"{{inviter}} te chamou pra mesa no Truco"`; texto: quem chamou, nome da sala, link, "o convite vale enquanto a sala tiver lugar"), `auth/invite.go`, `lobby/people.go`, `lobby/invites.go` pelo algoritmo acima, três `case` em `Dispatch`, mensagens no `registry.go`, `main.go` passa `People: lobby.NewPeople(authSvc)`. `make gen-types`, golden.

Run: `cd backend && go test ./...`
Expected: PASS.

- [ ] **Step 3: Testes do frontend que falham**

`invites.test.ts`:

```ts
it('empilha o mais novo em cima e mostra no máximo 3', () => {
  const s = useInvitesStore()
  for (const id of ['a', 'b', 'c', 'd']) s.onReceived(recebido(id))
  expect(s.visible.map((i) => i.inviteId)).toEqual(['d', 'c', 'b'])
})
it('não duplica aviso reenviado na reconexão', () => {
  const s = useInvitesStore()
  s.onReceived(recebido('a'))
  s.onReceived(recebido('a'))
  expect(s.received).toHaveLength(1)
})
it('responder tira o aviso e envia invite_respond', async () => {
  const send = vi.fn().mockResolvedValue(undefined)
  setSender(send)
  const s = useInvitesStore()
  s.onReceived(recebido('a'))
  await s.respond('a', true)
  expect(s.received).toHaveLength(0)
  expect(send).toHaveBeenCalledWith({ type: 'invite_respond', payload: { inviteId: 'a', accept: true } })
})
it('invite_expired retira o aviso', () => { /* onExpired({ inviteId: 'a' }) → received vazio */ })
```

`wire.test.ts`: os três tipos novos chegam à store; `left_room` e `lobby_snapshot` chamam `clearRoom()`; `stop()` zera.

Run: `cd frontend && pnpm vitest run src/stores/invites.test.ts src/app/wire.test.ts`
Expected: FAIL.

- [ ] **Step 4: Implementar e verificar**

Run: `cd frontend && pnpm typecheck && pnpm lint && pnpm test`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend frontend/src
git commit -m "feat(convites): convite para a sala no app e por e-mail com token só do convidado"
```

---

### Task 8: Lobby com "Online agora"

**Skills:** `frontend-design:frontend-design`, `impeccable`, `fixing-accessibility`, `ui-a11y`, `lint-and-validate`.

**Files:**
- Create: `frontend/src/components/lobby/OnlineAgora.vue`, `frontend/src/components/lobby/online.test.ts`
- Modify: `frontend/src/views/LobbyView.vue`, `frontend/src/stores/room.ts`, `frontend/src/stores/room.test.ts`

**Interfaces:**
- Consumes: `usePresenceStore`, `presenceText` (T5); `useInvitesStore.invite` (T7); `useSessionStore().user`.
- Produces: getters em `useRoomStore`: `freeSeats: Seat[]`, `canInvite: boolean` (em sala, `waiting`, com lugar livre). `OnlineAgora.vue` sem props; cada linha `data-online="<username>"`, nome é `RouterLink` para `{ name: 'jogador', params: { username } }` (rota criada na T13 — até lá, `RouterLink` com `to` em string `/jogadores/<username>`), botão `data-acao="convidar"` só com `room.canInvite` e quando a pessoa não é eu nem está na minha sala.

- [ ] **Step 1: Testes que falham** (`online.test.ts`, `room.test.ts`)

- lista "Online agora · 3" com ponto **e** texto de status (`presenceText`), excluindo nada (eu apareço marcado "você"); ordem da store.
- com `room.canInvite`, clicar "Convidar" chama `invites.invite('bia')`; sem sala, nenhum botão "Convidar"; erro do envio vira toast (`fromError`).
- `LobbyView` em 375 px (`window.innerWidth` não importa no jsdom: teste a estrutura): existe `role="tablist"` com abas "Salas" e "Online (3)"; trocar de aba mostra o painel certo (`aria-selected`, `aria-controls`); no desktop a coluna é a mesma `OnlineAgora`.
- `room.test.ts`: `canInvite` falso em jogo, falso com 4 sentados, verdadeiro em espera com lugar.

Run: `cd frontend && pnpm vitest run src/components/lobby src/stores/room.test.ts`
Expected: FAIL.

- [ ] **Step 2: Implementar**

Desktop (≥ 768 px): grade `[salas | coluna de 18 rem]`, coluna com título "Online agora · N" (Atkinson 700, não Kalam), linhas em `surface-2` como as salas; status no ponto + texto `ink-muted`. Celular (< 768 px): abas "Salas" / "Online (N)" com teclado de setas (padrão WAI tabs), sem competir com a lista de salas. Estado vazio: "Só você por aqui agora."

- [ ] **Step 3: Revisão visual, acessibilidade e testes**

Ciclo de revisão visual (Global Constraints) no lobby com 0, 3 e 15 pessoas online, eu numa sala (botões "Convidar") e fora. Run: `cd frontend && pnpm typecheck && pnpm lint && pnpm test`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add frontend/src
git commit -m "feat(lobby): coluna Online agora com status e convite rápido (abas no celular)"
```

---

### Task 9: Convidar na sala de espera

**Skills:** `frontend-design:frontend-design`, `impeccable`, `fixing-accessibility`, `ui-a11y`, `lint-and-validate`.

**Files:**
- Create: `frontend/src/api/social.ts`, `frontend/src/api/social.test.ts`, `frontend/src/stores/players.ts`, `frontend/src/stores/players.test.ts`, `frontend/src/components/sala/SeletorPessoas.vue`, `frontend/src/components/sala/ListaConvites.vue`, `frontend/src/components/sala/convites.test.ts`
- Modify: `frontend/src/components/sala/SalaEspera.vue`, `frontend/src/components/sala/Assento.vue`, `frontend/src/components/sala/sala.test.ts`

**Interfaces:**
- Consumes: `usePresenceStore`, `useInvitesStore` (`invite`, `roomInvites`), `useRoomStore` (`room`, `canInvite`, `freeSeats`), `inviteStateText`.
- Produces:
  - `api/social.ts`: tipos `DirectoryEntry`, `DirectoryPage`, `MatchPlayer`, `HistoryItem` (`endReason: EndReason` de `@/protocol/messages`, mesmos valores do Go), `HistoryPage`, `Partner`, `PositionPair`, `Positions`, `Profile`, `Period`, `Metric`, `RankingEntry`, `RankingResult` (espelham o JSON das T2/T3); `socialApi.directory(q: string, cursor?: string)`, `profile(username)`, `history(username, cursor?)`, `ranking(period, metric, limit?)` (query string com `encodeURIComponent`).
  - `usePlayersStore`: `searchDirectory(q: string): Promise<DirectoryEntry[]>` (só a primeira página; o seletor busca com debounce de 250 ms); T13 acrescenta perfil e histórico.
  - `SeletorPessoas.vue` props `{ open: boolean; seat?: Seat }`, emite `close`; usa `Modal` (foco preso, Esc fecha). Seções "Online" (presença menos eu e quem já está na sala; quem está `playing` aparece com "jogando — pode recusar" e continua convidável) e "Outros colegas" (campo de busca → diretório, sem quem já aparece em Online). Cada pessoa tem botão "Convidar" (`data-convidar="<username>"`) que chama `invites.invite(username, seat)` e fecha.
  - `ListaConvites.vue` (sem props) lista `roomInvites` com nome e `inviteStateText`.
  - `Assento.vue` ganha prop `canInvite: boolean` e emit `invite` (botão "Convidar" ao lado de "Sentar" no lugar vazio).

- [ ] **Step 1: Testes que falham**

`social.test.ts`: `directory('ana b')` chama `GET /api/users?q=ana%20b`; `ranking('week','winrate')` monta `?period=week&metric=winrate`.
`convites.test.ts`:
- botão "Convidar" no topo abre o seletor sem lugar; "Convidar" do lugar vazio 3 abre com `seat: 3`, e escolher `bia` envia `{ type: 'invite', payload: { username: 'bia', seat: 3 } }`.
- seção Online omite eu e quem está sentado; pessoa `playing` mostra "jogando — pode recusar" e o botão continua habilitado.
- busca em "Outros colegas" (fake timers 250 ms) chama `players.searchDirectory('ca')` uma vez e lista o resultado sem duplicar quem está online.
- `ListaConvites` mostra "aceitou", "aguardando…", "agora não" e "e-mail enviado".
- sem `room.canInvite` (sala cheia ou em jogo), nenhum botão "Convidar" aparece.

Run: `cd frontend && pnpm vitest run src/api/social.test.ts src/components/sala`
Expected: FAIL.

- [ ] **Step 2: Implementar**

Composição pelo brief §8.4: "Convidar" (secundário) no cabeçalho ao lado de "Copiar link"; no lugar vazio, "Convidar" fantasma de 44 px junto de "Sentar"; "Convites" abaixo da mesa em lista simples (sem cartões). Seletor no celular ocupa a largura útil do modal; listas com rolagem própria.

- [ ] **Step 3: Revisão visual, acessibilidade e testes**

Ciclo de revisão visual com sala vazia, sala com 2 convites (estados variados), seletor aberto (com e sem busca, sem ninguém online). Run: `cd frontend && pnpm typecheck && pnpm lint && pnpm test`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add frontend/src
git commit -m "feat(sala): convidar pelo topo ou pelo lugar vazio, seletor de pessoas e lista de convites"
```

---

### Task 10: Aviso de convite e aba em segundo plano

**Skills:** `frontend-design:frontend-design`, `impeccable`, `fixing-accessibility`, `ui-a11y`, `lint-and-validate`.

**Files:**
- Create: `frontend/src/components/convite/AvisosConvite.vue`, `frontend/src/components/convite/AvisoConvite.vue`, `frontend/src/components/convite/aviso.test.ts`, `frontend/src/app/inviteAlerts.ts`, `frontend/src/app/inviteAlerts.test.ts`, `frontend/src/app/notifications.ts`, `frontend/src/assets/sfx/invite.ogg`, `frontend/src/assets/sfx/invite.mp3`
- Modify: `frontend/src/components/AppShell.vue`, `frontend/src/components/mesa/{Mesa,Anunciador}.vue`, `frontend/src/components/mesa/{mesa,extras}.test.ts`, `frontend/src/game/tabTitle.ts`, `frontend/src/game/tabTitle.test.ts`, `frontend/src/audio/{sfx,install}.ts`, `frontend/src/audio/sfx.test.ts`, `frontend/src/assets/sfx/LICENSES.md`, `frontend/src/stores/prefs.ts`, `frontend/src/stores/prefs.test.ts`, `frontend/src/views/PerfilView.vue`, `frontend/src/main.ts`

**Interfaces:**
- Consumes: `useInvitesStore` (`visible`, `respond`), `useRoomStore` (`inRoom`, `isPlaying`), `sfx()`.
- Produces:
  - `SoundId` ganha `'invite'` (em `SOUND_IDS`).
  - `tabTitle.ts`: `startTabTitleBlink(base, alert, opts?)` mantém a assinatura, mas todas as chamadas compartilham **um** intervalo que alterna `base → alerta1 → base → alerta2 …` entre os alertas ativos; parar um alerta não apaga os outros; foco na janela para todos e restaura `base`.
  - `prefs`: campos `notifications: boolean` (padrão `true`) e `notifyAsked: boolean` (padrão `false`), ações `setNotifications(on)`, `markNotifyAsked()`, persistidos em `truco.prefs`.
  - `notifications.ts`: `askNotificationPermissionOnce(): Promise<void>` (só se `'Notification' in window`, permissão `default`, `prefs.notifications` e `!prefs.notifyAsked`; marca `notifyAsked` antes de pedir; recusa nunca é pedida de novo) e `showInviteNotification(inv: InviteReceivedPayload, onClick: () => void): void` (só com permissão `granted` e `prefs.notifications`).
  - `inviteAlerts.ts`: `installInviteAlerts(deps?: { hidden?: () => boolean; notify?: typeof showInviteNotification; play?: (id: SoundId) => void }): void` — a cada convite **novo** na store: com a aba oculta ou sem foco, toca `invite` (`tocarQuandoLiberado`), pisca o título `"Convite: <sala>"` e mostra a notificação (clique: `window.focus()` e foco no "Entrar" do aviso); com a aba em foco, só o som.
  - `AvisosConvite.vue` (montado no `AppShell`, logado) empilha `invites.visible`; `AvisoConvite.vue` props `{ invite: InviteReceivedPayload }` com "Fulano te chamou pra mesa!", nome da sala, "N lugares livres", "Entrar" (`data-acao="entrar"`) e "Agora não" (`data-acao="agora-nao"`); em sala de espera, a linha "Aceitar tira você da sala atual."; jogando, sem "Entrar" e com "Você está numa partida".
  - `Anunciador` sai da `Mesa` e passa a ser montado **uma vez** no `AppShell` (continua a única região viva de anúncios); ganha o anúncio `"<Fulano> te chamou pra mesa: <sala>"` a cada convite novo.

- [ ] **Step 1: Testes que falham**

`aviso.test.ts`:
- "Entrar" chama `respond(id, true)` e "Agora não" `respond(id, false)`; Esc com o aviso focado = "Agora não".
- "Entrar" recebe foco ao chegar o convite, **exceto** com foco num `input`/`textarea` ou com `[role="dialog"]` aberto.
- jogando: não há `[data-acao="entrar"]` e o texto "Você está numa partida" aparece; em sala de espera aparece "Aceitar tira você da sala atual.".
- 4 convites → 3 avisos visíveis, o mais novo primeiro; o aviso não tem `aria-live` próprio.
- primeira resposta a um aviso chama `askNotificationPermissionOnce` (mock).

`tabTitle.test.ts`: `alterna entre dois alertas sem um apagar o outro` ("Sua vez!" e "Convite: Mesa do RH" ativos: a sequência de títulos contém os dois; parar o de convite mantém "Sua vez!" piscando; foco restaura a base).
`inviteAlerts.test.ts`: aba oculta → `play('invite')`, título e `notify` uma vez por convite; convite reenviado (mesmo id) não repete; aba em foco → só som.
`prefs.test.ts`: `notifications`/`notifyAsked` persistem e têm padrão com `localStorage` quebrado.
`extras.test.ts`/`mesa.test.ts`: a `Mesa` não monta mais `Anunciador`; o `AppShell` monta exatamente um `[data-anunciador]`; os testes do `Anunciador` continuam e ganham `anuncia convite novo`.

Run: `cd frontend && pnpm vitest run src/components/convite src/game/tabTitle.test.ts src/app src/stores/prefs.test.ts src/components/mesa`
Expected: FAIL.

- [ ] **Step 2: Som**

Baixe o pacote **Interface Sounds** da Kenney (CC0) para um diretório novo e vazio no scratchpad; use `question_001.ogg` (se não existir, `pluck_001.ogg`), converta para MP3 como os demais (mono, curto), confira `du -ch frontend/src/assets/sfx/*.{ogg,mp3}` **< 300 KB** e acrescente a linha em `LICENSES.md`.

- [ ] **Step 3: Implementar**

Itens do bloco Interfaces; `main.ts` chama `installInviteAlerts()` depois de `installSfx()`; `PerfilView` › Preferências ganha o `Toggle` "Notificações do navegador para convites" (desligar não revoga a permissão; só para de notificar). Aviso: placa de lousa no canto inferior esquerdo (≥ 768 px) ou topo da área útil no celular, sem cobrir toasts nem cabeçalho; não é toast (persiste até resposta ou expiração).

- [ ] **Step 4: Revisão visual, acessibilidade e testes**

Ciclo de revisão visual com 1 e 3 avisos, no lobby, na sala de espera e sobre a mesa (jogando). Run: `cd frontend && pnpm typecheck && pnpm lint && pnpm test && pnpm build`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src
git commit -m "feat(convites): aviso persistente com Entrar/Agora não, som, título e notificação com a aba em segundo plano"
```

---

### Task 11: Aceitar convite por e-mail (`/entrar-convite/:token`)

**Skills:** `frontend-design:frontend-design`, `impeccable`, `fixing-accessibility`, `lint-and-validate`.

**Files:**
- Create: `frontend/src/views/EntrarConviteView.vue`, `frontend/src/views/entrarConvite.test.ts`
- Modify: `frontend/src/router.ts`, `frontend/src/router.test.ts`

**Interfaces:**
- Consumes: `useInvitesStore.acceptToken` (T7), `useRoomStore.inRoom`, `useSessionStore.connection`.
- Produces: rota `{ path: '/entrar-convite/:token', name: 'entrar-convite' }` (privada; o guard existente preserva `volta`).

- [ ] **Step 1: Testes que falham**

- `router.test.ts`: deslogado em `/entrar-convite/abc` → `/entrar?volta=/entrar-convite/abc`.
- `entrarConvite.test.ts`: com conexão `open`, envia `{ type: 'invite_accept_token', payload: { token: 'abc' } }` uma única vez e mostra "Entrando na sala…"; quando `room.inRoom` vira `true`, `router.replace({ name: 'sala' })`; erro `invite_expired` → "Convite expirado" + botão "Ir para o lobby"; `invite_not_yours` → `mostra convite de outra pessoa` ("Este convite é para outra pessoa"); `invitee_playing` → "Você está numa partida"; conexão abre depois da montagem → envia ao abrir.

Run: `cd frontend && pnpm vitest run src/views/entrarConvite.test.ts src/router.test.ts`
Expected: FAIL.

- [ ] **Step 2: Implementar, revisar e verificar**

Mesmo padrão de espera da conexão do `ConviteView`. Estados com título 24/700 e uma única ação primária. Ciclo de revisão visual dos quatro estados. Run: `cd frontend && pnpm typecheck && pnpm lint && pnpm test`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add frontend/src
git commit -m "feat(convites): página de aceite do convite por e-mail com estados de expirado e de outra conta"
```

---

### Task 12: Ranking

**Skills:** `frontend-design:frontend-design`, `impeccable`, `fixing-accessibility`, `ui-a11y`, `lint-and-validate`.

**Files:**
- Create: `frontend/src/stores/ranking.ts`, `frontend/src/stores/ranking.test.ts`, `frontend/src/views/RankingView.vue`, `frontend/src/views/ranking.test.ts`
- Modify: `frontend/src/router.ts`, `frontend/src/components/AppShell.vue`

**Interfaces:**
- Consumes: `socialApi.ranking` e tipos (T9).
- Produces: `useRankingStore` state `{ period: Period, metric: Metric, result: RankingResult | null, status: 'idle' | 'loading' | 'ok' | 'error' }`, ação `load(period, metric)` (descarta resposta velha se o filtro mudou no meio); rota `{ path: '/ranking', name: 'ranking' }`; link "Ranking" no cabeçalho.

- [ ] **Step 1: Testes que falham**

- store: trocar de `week` para `month` antes da primeira resposta voltar mantém só o resultado de `month`.
- view: abas "Vitórias"/"Aproveitamento" (`role="tablist"`) e período "Geral / Mês / Semana" (`radiogroup`); colunas `#`, jogador, V, D, %; minha linha com `aria-current="true"`; `me` fora do top aparece no fim separado por divisor; em "Aproveitamento", a nota "Só quem jogou 5 ou mais partidas no período." usa `result.minMatches`; vazio em Semana → "Ninguém jogou nesta semana ainda"; erro → mensagem + "Tentar de novo"; clicar no jogador navega para `/jogadores/<username>`.

Run: `cd frontend && pnpm vitest run src/stores/ranking.test.ts src/views/ranking.test.ts`
Expected: FAIL.

- [ ] **Step 2: Implementar, revisar e verificar**

Tabela sóbria (mockup A), números `tabular-nums`, % com uma casa decimal em pt-BR; no celular a tabela cabe em 375 px sem rolagem horizontal (nome trunca). Ciclo de revisão visual (vazio, carregando, erro, cheio com `me` fora do top). Run: `cd frontend && pnpm typecheck && pnpm lint && pnpm test`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add frontend/src
git commit -m "feat(ranking): tela com vitórias e aproveitamento por período e minha posição destacada"
```

---

### Task 13: Perfil público e histórico

**Skills:** `frontend-design:frontend-design`, `impeccable`, `fixing-accessibility`, `ui-a11y`, `lint-and-validate`.

**Files:**
- Create: `frontend/src/views/JogadorView.vue`, `frontend/src/views/PartidasView.vue`, `frontend/src/components/partidas/LinhaPartida.vue`, `frontend/src/views/jogador.test.ts`
- Modify: `frontend/src/stores/players.ts`, `frontend/src/stores/players.test.ts`, `frontend/src/router.ts`, `frontend/src/components/AppShell.vue`, `frontend/src/components/lobby/OnlineAgora.vue` (troca para a rota nomeada)

**Interfaces:**
- Consumes: `socialApi.profile`/`history` (T9), `usePresenceStore.byUsername`, `useInvitesStore.invite`, `useRoomStore.canInvite`, `endReasonText`.
- Produces: `usePlayersStore` ganha `loadProfile(username): Promise<Profile>` e `loadHistory(username, cursor?): Promise<HistoryPage>`; rotas `{ path: '/jogadores/:username', name: 'jogador' }` e `{ path: '/jogadores/:username/partidas', name: 'jogador-partidas' }`; `LinhaPartida.vue` props `{ item: HistoryItem; viewer: string }` (resultado do ponto de vista de `viewer`, placar orientado para a dupla dele ou "W.O.", parceiro, adversários, quando).

- [ ] **Step 1: Testes que falham** (`jogador.test.ts`, `players.test.ts`)

- perfil mostra nome, `@username`, status online (da presença; ausente = "offline"), posições dos três períodos ("—" quando `null`), estatísticas (partidas, % vitórias, truco aceito/corrido, mãos de 11 vencidas), 5 recentes e parceiros.
- `LinhaPartida`: W.O. mostra "W.O." no lugar do placar; abortada mostra "Abortada"; placar orientado à dupla do `viewer` (time 1 vê `score[1]×score[0]`); parceiro é quem está no assento oposto (via `teamOf`, nunca `seat % 2`).
- sou eu → "Editar conta" leva a `/perfil` e não há "Convidar"; outra pessoa com `room.canInvite` → "Convidar pra minha sala" envia `invite`; 404 → "Jogador não encontrado".
- histórico: "Carregar mais" pede a próxima página com `nextCursor` e some quando não há mais.
- cabeçalho: o nome leva a `/jogadores/<meu username>`.

Run: `cd frontend && pnpm vitest run src/views/jogador.test.ts src/stores/players.test.ts`
Expected: FAIL.

- [ ] **Step 2: Implementar, revisar e verificar**

Perfil em coluna única (≤ 65 ch de medida), seções separadas por traço como a `PerfilView` da 4a. Ciclo de revisão visual (jogador novo sem partidas, jogador com histórico, eu, 404). Run: `cd frontend && pnpm typecheck && pnpm lint && pnpm test`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add frontend/src
git commit -m "feat(perfil): perfil público com posições, recentes e parceiros e histórico paginado"
```

---

### Task 14: Painel admin

**Skills:** `frontend-design:frontend-design`, `impeccable`, `fixing-accessibility`, `ui-a11y`, `lint-and-validate`.

**Files:**
- Create: `frontend/src/api/admin.ts`, `frontend/src/stores/admin.ts`, `frontend/src/stores/admin.test.ts`, `frontend/src/views/admin/{AdminLayout,AdminUsuariosView,AdminDominiosView,AdminPartidasView,AdminAuditoriaView}.vue`, `frontend/src/views/admin/admin.test.ts`
- Modify: `frontend/src/api/http.ts` (`Method` ganha `'DELETE'`), `frontend/src/router.ts`, `frontend/src/router.test.ts`, `frontend/src/components/AppShell.vue`, `frontend/src/game/labels.ts` (`target_is_admin`)

**Interfaces:**
- Consumes: rotas `/api/admin/*` (T4).
- Produces: `adminApi.users(q, cursor?)`, `disable(id)`, `enable(id)`, `forceReset(id)`, `domains()`, `addDomain(d)`, `removeDomain(d)`, `liveMatches(cursor?)`, `abort(id, reason)`, `audit(cursor?)`; `useAdminStore` com as mesmas ações; rotas filhas de `{ path: '/admin', component: AdminLayout, meta: { admin: true } }`: `''` → redirect `admin-usuarios`, `usuarios` (`admin-usuarios`), `dominios`, `partidas`, `auditoria`; guard: `meta.admin` sem `session.user?.isAdmin` → `{ name: 'lobby' }`; link "Admin" no cabeçalho só com `isAdmin`.

- [ ] **Step 1: Testes que falham**

- `router.test.ts`: não-admin em `/admin/usuarios` vai para `/`.
- `admin.test.ts`: busca de usuários; linha de admin marcada "admin" e **sem** botões; texto "Admin é concedido só pelo servidor." presente; nenhum controle cujo nome contenha "admin" além do rótulo (varredura de `button`); desativar/reativar/forçar troca pedem confirmação em `Modal` e só então chamam a store; `403 target_is_admin` vira toast; domínios: adicionar e remover (com confirmação); partidas: "Abortar" exige motivo e confirma; auditoria: "Carregar mais" com cursor; ator `via: cli` aparece como "CLI do servidor".

Run: `cd frontend && pnpm vitest run src/views/admin src/stores/admin.test.ts src/router.test.ts`
Expected: FAIL.

- [ ] **Step 2: Implementar, revisar e verificar**

Ferramenta interna sóbria e densa: tabelas, sem identidade de mesa (sem lousa nem Kalam), sub-navegação por abas no topo; no celular, tabelas viram listas empilhadas. Ciclo de revisão visual das quatro seções. Run: `cd frontend && pnpm typecheck && pnpm lint && pnpm test`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add frontend/src
git commit -m "feat(admin): painel com usuários, domínios, partidas em andamento e auditoria, sem controle de admin"
```

---

### Task 15: E2E social e README

**Skills:** `e2e-testing`, `playwright-skill`, `readme`.

**Files:**
- Create: `frontend/e2e/social.spec.ts`
- Modify: `frontend/e2e/helpers.ts`, `frontend/e2e/partida.spec.ts`, `README.md`

**Interfaces:**
- Produces (helpers): `createAccount(browser, name): Promise<Account>` (`Account = { username, email, ip }`), `loginAs(browser, account): Promise<Page>`, `newPlayer` passa a compor os dois; `inviteLink(email): Promise<string>` lê do `E2E_SERVER_LOG` o último `/entrar-convite/<token>` enviado a `email` (mesmo laço de `verificationToken`).

- [ ] **Step 1: Escrever os testes**

`social.spec.ts`:
- `convite no app`: A e B logados; A cria sala privada e abre "Convidar"; B aparece na seção "Online" do seletor; A convida B; B vê "te chamou pra mesa!", clica "Entrar" e senta (`[data-assento]` com o nome de B em A); a lista "Convites" de A mostra "aceitou".
- `convite por e-mail`: C só cria a conta (`createAccount`, offline); A convida C pela busca "Outros colegas"; a lista mostra "e-mail enviado"; `inviteLink(c.email)`; C abre o link deslogado, faz login, volta ao link e entra na sala.

`partida.spec.ts` (fim do teste da partida completa): `/ranking` lista o nome de um dos jogadores; o perfil dele mostra 1 partida recente com placar ou "W.O.".

- [ ] **Step 2: Rodar**

Run: `make e2e`
Expected: todos os testes passam.

- [ ] **Step 3: README**

Status/roadmap (4b entregue), rotas HTTP novas (`/api/users`, `/api/ranking?period&metric`, `/api/admin/matches`; `PUT …/admin` removida), mensagens WS novas, CLI `demote-admin`, "admin só pelo CLI" em segurança, telas novas.

- [ ] **Step 4: Validação final**

Run: `cd backend && go vet ./... && go test ./... && cd ../frontend && pnpm typecheck && pnpm lint && pnpm test && pnpm build`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/e2e README.md
git commit -m "test(e2e): convite no app e por e-mail, ranking e perfil após partida; docs da 4b"
```
