# Truco Multiplayer — Spec 2: Contas e Persistência

- **Data:** 2026-10-14
- **Status:** aprovado em conversa, aguardando revisão escrita
- **Escopo desta spec:** contas de usuário (cadastro, verificação de e-mail, login, sessão, recuperação e troca de senha), administração, persistência de partidas (seed + log de ações), reconstrução/recuperação de partidas, estatísticas e ranking, API HTTP JSON correspondente.
- **Depende de:** Spec 1 (engine + protocolo), já implementada.
- **Fora de escopo:**
  - Spec 3 — salas, lobby, WebSocket, timers, reconexão. A Spec 3 **consome** `match.Store`, `match.Recover`, `auth.SessionFromRequest`, `auth.CheckOrigin` e implementa `admin.MatchAborter`.
  - Spec 4 — frontend (páginas de cadastro, login, verificação, reset, ranking, replay).
  - Pendências da revisão da Spec 1 sobre tipos TS (tygo exportando `GameState`/`HandState`; `ServerMessage.Payload` como `any`): ficam para a Spec 3, que redefine as mensagens do protocolo.

## 1. Contexto e objetivo

O jogo é usado por colegas da empresa (dezenas de usuários, uma instância) e como portfólio. Esta spec dá identidade aos jogadores e durabilidade às partidas:

1. **Contas confiáveis:** só pessoas com e-mail de domínio permitido e **comprovadamente donas** do e-mail jogam.
2. **Sessão segura e revogável:** token opaco em cookie `HttpOnly`, só o hash no banco.
3. **Partidas duráveis:** o log de ações é a única fonte da verdade; dele saem replay, recuperação após restart, histórico, estatísticas e ranking.

### Critérios de sucesso

- `docker compose up -d` sobe o Postgres; `make run` sobe o servidor aplicando migrations.
- `go test ./...` roda testes de integração contra Postgres real (testcontainers).
- Fluxo HTTP completo funciona: cadastro → verificação → login → `/api/me` → logout; esqueci a senha → reset.
- Qualquer partida gravada é reconstruída bit a bit a partir de seed + log (`Rebuild` == estado ao vivo).

## 2. Decisões

| Decisão | Escolha | Motivo |
|---|---|---|
| Banco | PostgreSQL 18 (`uuidv7()` nativo); Docker Compose no dev | Decidido antes; UUID v7 ordenável sem extensão |
| Acesso a dados | `pgx/v5` + `sqlc` (código gerado e versionado, `make gen-sql`) | SQL explícito e tipado, sem ORM; mesma filosofia do tygo |
| Migrations | `goose/v3` com `embed.FS`; aplicadas no start (`MIGRATE_ON_START`) e via subcomando `migrate` | Binário único; o sqlc lê os mesmos arquivos como schema |
| Hash de senha | argon2id (`golang.org/x/crypto/argon2`), formato PHC | Decidido antes |
| Sessão | Token opaco 32 bytes, cookie `HttpOnly`/`Secure`/`SameSite=Lax`, `sha256` no banco; deslizante 30 dias, teto 90 dias | Revogável; app interno de uso esporádico |
| Verificação de e-mail | Obrigatória; login bloqueado até confirmar | O domínio restrito sozinho não prova posse do e-mail |
| E-mail | Interface `Mailer`; Resend (prod e dev) via `net/http`; driver `log` para quem não tem chave; fake nos testes | Um único `POST`, não justifica SDK |
| CSRF | `SameSite=Lax` + `Origin` obrigatório em métodos mutantes + `Content-Type: application/json` | Mesma origem; sem token extra |
| Rate limit | Em memória (`golang.org/x/time/rate`), por IP e por identificador | Instância única |
| API | JSON em `/api/*`, mesma origem do frontend (Go serve o build; Vite faz proxy no dev) | Sem CORS; cookie `SameSite` simples |
| Escrita do log | **Síncrona**, um insert por ação, antes do broadcast | Nada que os jogadores viram deixa de estar no banco |
| Estatísticas | Derivação **pura** por replay ao finalizar + agregado incremental `player_stats` | Testável sem banco; `recompute-stats` barato |
| Ranking | Vitórias, depois taxa de vitória, com mínimo de partidas | Simples de derivar e explicar |
| Testes de banco | `testcontainers-go` + banco-template clonado por teste | Sem setup manual; isolamento rápido; igual no CI |
| HTTP | stdlib `net/http` (roteamento 1.22+) e `log/slog` | Decidido antes |

## 3. Estrutura de pacotes

```
backend/
├── cmd/server/main.go        # subcomandos: serve (padrão), migrate, promote-admin, recompute-stats
├── sqlc.yaml
└── internal/
    ├── engine/               # + RulesVersion e GameState.MarshalJSON (§7.1); nada mais muda
    ├── protocol/             # sem mudanças
    ├── config/               # carrega/valida env (§10); falha no start com mensagem clara
    ├── db/
    │   ├── migrations/       # NNNN_nome.sql (goose Up/Down), embutidos
    │   ├── queries/          # *.sql de entrada do sqlc
    │   ├── dbgen/            # GERADO pelo sqlc — não editar
    │   └── db.go             # Open (pgxpool), Migrate, WithTx
    ├── auth/                 # senha, tokens, normalização, Service, sessão, Origin
    ├── mail/                 # Mailer, ResendMailer, LogMailer, Fake, templates
    ├── ratelimit/            # Limiter por chave com eviction e relógio injetável
    ├── replay/               # PURO: Record, LoggedAction, Rebuild, Walk (sem banco)
    ├── match/                # Store (Postgres), Recover, RecomputeStats
    ├── stats/                # Derive (puro, sobre replay.Walk) + consultas de ranking/perfil
    ├── admin/                # Service de administração + auditoria + MatchAborter
    └── httpapi/              # roteador, handlers, middlewares, erros JSON, estáticos
```

**Regras de dependência**

- `engine` continua importando apenas a stdlib.
- `replay` depende só de `engine` (puro, sem I/O).
- `stats` depende de `replay` (e de `db` apenas nas consultas de ranking); `stats.Derive` não toca o banco.
- `match` depende de `replay`, `stats` e `db`. Não há ciclo: `stats` nunca importa `match`.
- `auth`, `admin`, `match`, `stats` não importam `net/http` (exceto `auth` para `SessionFromRequest`/`CheckOrigin`, que operam sobre `*http.Request`).
- `httpapi` usa os serviços; nunca chama `dbgen` diretamente.
- Serviços recebem relógio injetável (`func() time.Time`) para testes de expiração.

**Dependências novas:** `github.com/jackc/pgx/v5`, `github.com/pressly/goose/v3`, `golang.org/x/crypto`, `golang.org/x/time`, `github.com/testcontainers/testcontainers-go` (+ módulo `postgres`, só em testes). `sqlc` roda via `go run` fixado em versão no Makefile, como o tygo.

## 4. Schema

Todas as tabelas têm `created_at timestamptz NOT NULL DEFAULT now()` salvo indicação. IDs são `uuid DEFAULT uuidv7()`.

### 4.1 `users`

| Coluna | Tipo | Regras |
|---|---|---|
| `id` | uuid PK | |
| `username` | text UNIQUE NOT NULL | normalizado para minúsculas; `CHECK (username ~ '^[a-z0-9_.]{3,20}$')` |
| `email` | text UNIQUE NOT NULL | normalizado (trim + minúsculas) |
| `display_name` | text NOT NULL | 1..30 caracteres após trim; livre |
| `password_hash` | text NOT NULL | PHC argon2id |
| `is_admin` | boolean NOT NULL DEFAULT false | |
| `email_verified_at` | timestamptz NULL | NULL = pendente |
| `disabled_at` | timestamptz NULL | NULL = ativa |
| `must_reset_password` | boolean NOT NULL DEFAULT false | definido pelo admin |
| `updated_at` | timestamptz NOT NULL | |

`Player.ID` do engine = `users.id` em texto.

### 4.2 `sessions`

| Coluna | Tipo | Regras |
|---|---|---|
| `id` | uuid PK | |
| `user_id` | uuid FK → users ON DELETE CASCADE | índice |
| `token_hash` | bytea UNIQUE NOT NULL | `sha256(token)` |
| `last_seen_at` | timestamptz NOT NULL | |
| `expires_at` | timestamptz NOT NULL | `last_seen_at + 30d` |
| `absolute_expires_at` | timestamptz NOT NULL | `created_at + 90d` |
| `user_agent` | text NOT NULL DEFAULT '' | truncado em 256 |
| `ip` | inet NULL | |

### 4.3 `user_tokens`

| Coluna | Tipo | Regras |
|---|---|---|
| `id` | uuid PK | |
| `user_id` | uuid FK → users ON DELETE CASCADE | |
| `purpose` | text NOT NULL | `CHECK (purpose IN ('verify_email','password_reset'))` |
| `token_hash` | bytea UNIQUE NOT NULL | `sha256(token)` |
| `expires_at` | timestamptz NOT NULL | verificação 24h; reset 30min |
| `used_at` | timestamptz NULL | uso único |

Emitir um token novo apaga os tokens não usados do mesmo `(user_id, purpose)`.

### 4.4 `allowed_email_domains`

`domain text PK` (minúsculas, sem `@`), `created_by uuid NULL FK → users`. Se a tabela estiver vazia no start, é populada a partir de `ALLOWED_EMAIL_DOMAINS`. Depois disso, a fonte é o banco (editável pelo admin).

### 4.5 `matches`

| Coluna | Tipo | Regras |
|---|---|---|
| `id` | uuid PK | |
| `seed` | bigint NOT NULL | `uint64` convertido bit a bit (`int64(seed)` / `uint64(v)`) |
| `first_dealer` | smallint NOT NULL | `CHECK 0..3` |
| `engine_version` | integer NOT NULL | `engine.RulesVersion` na criação |
| `status` | text NOT NULL DEFAULT 'in_progress' | `CHECK (status IN ('in_progress','finished','abandoned'))`; índice parcial para `in_progress` |
| `winner_team` | smallint NULL | 0 ou 1; preenchido em `finished` |
| `score_a`, `score_b` | smallint NULL | placar final (limitado a 12) |
| `finished_at` | timestamptz NULL | também usado em `abandoned` |
| `abandon_reason` | text NULL | |

### 4.6 `match_players`

`match_id` FK ON DELETE CASCADE, `seat smallint CHECK 0..3`, `user_id` FK, `display_name text` (snapshot no início). PK `(match_id, seat)`, `UNIQUE (match_id, user_id)`, índice em `user_id`.

### 4.7 `match_actions`

| Coluna | Tipo | Regras |
|---|---|---|
| `match_id` | uuid FK ON DELETE CASCADE | |
| `ply` | integer NOT NULL | 0, 1, 2… sem buracos |
| `action` | jsonb NOT NULL | `engine.Action` serializada |
| `source` | text NOT NULL | `CHECK (source IN ('player','timeout','system'))` |

PK `(match_id, ply)`.

**Timeouts:** a sala resolve `engine.DefaultAction` e grava a **ação concreta** com `source='timeout'`, nunca `sys_timeout`. Assim o replay não depende da regra de `DefaultAction` da versão em que é reproduzido. `sys_start_hand` é gravada com `source='system'`.

### 4.8 `match_player_stats`

PK `(match_id, user_id)`; `team smallint`, `won boolean` e os contadores da §7.4 (`integer NOT NULL DEFAULT 0`).

### 4.9 `player_stats`

`user_id uuid PK FK`, `matches_played`, `wins`, `losses` e as somas dos contadores da §7.4; `updated_at`.

### 4.10 `audit_log`

`id uuid PK`, `actor_id uuid FK → users`, `action text`, `target_type text`, `target_id text`, `details jsonb NOT NULL DEFAULT '{}'`. Índice em `created_at DESC`.

## 5. Autenticação

### 5.1 Normalização e validação

- `username`: trim, minúsculas, regex `^[a-z0-9_.]{3,20}$`.
- `email`: trim, minúsculas, formato validado com `net/mail.ParseAddress` e endereço "nu" (sem nome); domínio deve estar em `allowed_email_domains` (comparação exata, sem subdomínios).
- `display_name`: trim, 1..30 runes, sem caracteres de controle.
- `password`: 10..128 caracteres; não pode conter o username nem a parte local do e-mail (comparação sem diferenciar maiúsculas). Sem regras de composição.

### 5.2 Senha

- argon2id `m=64 MiB, t=3, p=2`, salt 16 bytes, chave 32 bytes, codificado em PHC (`$argon2id$v=19$m=65536,t=3,p=2$<salt>$<hash>`).
- Verificação em tempo constante (`subtle.ConstantTimeCompare`).
- Um semáforo global limita a 4 hashes simultâneos (evita DoS de memória).
- No login bem-sucedido, se os parâmetros do hash armazenado diferem dos atuais, re-hash e atualiza.
- Usuário inexistente no login: verificar contra um hash fictício pré-computado para igualar o tempo.

### 5.3 Tokens

- Sessão, verificação e reset: 32 bytes de `crypto/rand`, base64url sem padding.
- Banco guarda `sha256(token)`; busca por igualdade no hash.

### 5.4 Fluxos

| Rota | Entrada | Comportamento | Respostas |
|---|---|---|---|
| `POST /api/auth/register` | `{username, email, displayName, password}` | Valida (§5.1). Se o **e-mail** já existe: não cria nada, envia e-mail "você já tem conta" (com link para login/reset) e responde igual ao sucesso. Se o **username** já existe: `409 username_taken`. Senão cria usuário pendente e envia link de verificação. | `202` `{"message": …}`; `400 validation_failed` (com `fields`); `403 email_domain_not_allowed`; `409 username_taken` |
| `POST /api/auth/verify-email` | `{token}` | Token válido, não usado, não expirado, propósito `verify_email` → marca `used_at` e `email_verified_at`. | `204`; `400 invalid_token` |
| `POST /api/auth/resend-verification` | `{email}` | Se existe conta pendente com o e-mail, emite novo token e envia. | Sempre `202` |
| `POST /api/auth/login` | `{login, password}` | `login` com `@` → e-mail; senão username. Senha errada ou usuário inexistente → `401 invalid_credentials`. Só **após senha correta**: `403 email_not_verified`, `403 account_disabled`, `403 password_reset_required`. Sucesso: cria sessão, grava cookie, devolve o usuário. | `200 {user}`; `401`; `403`; `429 too_many_attempts` |
| `POST /api/auth/logout` | — | Apaga a sessão atual (se houver) e limpa o cookie. | `204` |
| `POST /api/auth/logout-all` | — (autenticado) | Apaga todas as sessões do usuário e limpa o cookie. | `204` |
| `POST /api/auth/password/forgot` | `{email}` | Conta ativa e verificada → token de reset (30min) e e-mail. Conta pendente → reenvia verificação. Inexistente ou desativada → nada. | Sempre `202` |
| `POST /api/auth/password/reset` | `{token, password}` | Valida token e senha; troca o hash; `must_reset_password=false`; `email_verified_at` preenchido se nulo; apaga **todas** as sessões. Sem login automático. | `204`; `400 invalid_token`; `400 validation_failed` |
| `GET /api/me` | — | Usuário da sessão. | `200 {user}`; `401 unauthenticated` |
| `PATCH /api/me` | `{displayName}` | Atualiza nome de exibição. | `200 {user}` |
| `POST /api/me/password` | `{currentPassword, newPassword}` | Confere a atual; troca o hash; apaga as **outras** sessões; **rotaciona** a atual (novo token e cookie). | `204`; `401 invalid_credentials`; `400 validation_failed` |

DTO de usuário: `{id, username, email, displayName, isAdmin, createdAt}` (só o próprio usuário e admins veem `email`).

### 5.5 Sessão

- Cookie `__Host-truco_session`, `HttpOnly; Secure; SameSite=Lax; Path=/; Max-Age=7776000` (90 dias). Com `COOKIE_SECURE=false` o nome passa a `truco_session` sem `Secure` (apenas desenvolvimento em HTTP fora de `localhost`).
- Middleware: lê o cookie, calcula o hash, busca sessão + usuário com `expires_at > now`, `absolute_expires_at > now` e `disabled_at IS NULL`. Ausente/inválida → requisição anônima (rotas protegidas respondem `401`).
- Renovação: se `now - last_seen_at > 24h`, atualiza `last_seen_at = now` e `expires_at = min(now + 30d, absolute_expires_at)`.
- Contexto da requisição carrega `auth.Principal{UserID, SessionID, Username, DisplayName, IsAdmin}`.
- Exportados para a Spec 3: `auth.SessionFromRequest(ctx, r) (Principal, error)` e `auth.CheckOrigin(r, allowedOrigin) bool`.
- Limpeza: goroutine a cada hora apaga sessões expiradas e `user_tokens` expirados ou usados há mais de 7 dias.

### 5.6 Proteções

- **CSRF:** em `POST/PUT/PATCH/DELETE` sob `/api`, exige `Origin == APP_ORIGIN`. Sem `Origin`, aceita apenas com `Sec-Fetch-Site: same-origin`. Exige `Content-Type: application/json` quando há corpo. Violação → `403 forbidden_origin`.
- **Enumeração:** `register` (e-mail existente), `resend-verification` e `forgot` respondem idêntico em todos os casos; login usa hash fictício. Envio de e-mail acontece em goroutine para não vazar tempo; falhas de envio são logadas.
- **Rate limit** (em memória, chaves ociosas descartadas após 30 min):

| Ação | Por IP | Por identificador |
|---|---|---|
| Login | 10/min (burst 10) | 5 falhas em 15 min bloqueiam o identificador normalizado por 15 min |
| Cadastro | 5/h | — |
| Forgot / resend | 5/h | 3/h por e-mail normalizado |
| Verify / reset | 20/h | — |

  Excesso → `429 too_many_attempts` com `Retry-After`. IP = `RemoteAddr`; com `TRUST_PROXY=true`, o último endereço de `X-Forwarded-For`.
- **Entrada:** corpo limitado a 16 KB (`http.MaxBytesReader`), `DisallowUnknownFields`.
- **Headers:** `/api` responde `Cache-Control: no-store`; todas as respostas com `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, `X-Frame-Options: DENY`; estáticos com CSP `default-src 'self'; connect-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'`.

### 5.7 E-mail

- `mail.Mailer` interface: `Send(ctx, Message{To, Subject, Text, HTML}) error`.
- `ResendMailer`: `POST https://api.resend.com/emails` com `Authorization: Bearer RESEND_API_KEY`, timeout 10s, até 3 tentativas com backoff em `5xx`/rede.
- `LogMailer`: registra destinatário, assunto e links no `slog` (para quem clona o repositório sem chave).
- `mail.Fake`: captura mensagens em memória para os testes.
- Templates `text/template` + `html/template` embutidos, em pt-BR: verificação, conta já existente, reset de senha, senha alterada (aviso), reset forçado pelo admin.
- Links: `APP_BASE_URL/verificar-email?token=…` e `APP_BASE_URL/redefinir-senha?token=…`; as páginas (Spec 4) fazem o `POST`.

## 6. Administração

Rotas em `/api/admin/*` com middleware `requireAdmin` (`403 forbidden` para não admins). Toda escrita grava `audit_log` **na mesma transação**.

| Rota | Efeito | Restrições |
|---|---|---|
| `GET /api/admin/users?q=&cursor=` | Lista/busca por username ou e-mail (keyset por `created_at, id`, 50 por página) | |
| `POST /api/admin/users/{id}/disable` | `disabled_at = now`, apaga sessões | Não pode a si mesmo |
| `POST /api/admin/users/{id}/enable` | `disabled_at = NULL` | |
| `POST /api/admin/users/{id}/force-password-reset` | `must_reset_password = true`, apaga sessões, envia e-mail de reset | |
| `PUT /api/admin/users/{id}/admin` `{isAdmin}` | Concede/remove admin | Nunca remove o último admin ativo |
| `GET /api/admin/email-domains` | Lista domínios | |
| `POST /api/admin/email-domains` `{domain}` | Adiciona | Valida formato de domínio |
| `DELETE /api/admin/email-domains/{domain}` | Remove | Nunca remove o último |
| `POST /api/admin/matches/{id}/abort` `{reason}` | `match.Store.Abandon` + `MatchAborter.Abort(id)` | Só `in_progress` |
| `GET /api/admin/audit?cursor=` | Lista auditoria (keyset) | |

`admin.MatchAborter` é uma interface com implementação no-op nesta spec; a Spec 3 a implementa para encerrar a sala viva.

Primeiro admin: `server promote-admin <email|username>` (grava auditoria com `actor_id` = o próprio usuário promovido e `details.via = "cli"`).

## 7. Partidas

### 7.1 Mudanças no engine

- `const RulesVersion = 1` em `engine` — incrementada sempre que uma mudança de regra alterar o resultado de `Apply` para um log existente.
- `func (GameState) MarshalJSON() ([]byte, error)` devolve erro (`engine: GameState é privado; use ViewFor`). Fecha a pendência da Spec 1: o estado completo nunca é serializado; o banco guarda ações.

### 7.2 `replay` e `match.Store`

Tipos e reconstrução ficam no pacote puro `replay`; o `match.Store` só persiste.

```go
// package replay
type PlayerSeat struct {
    UserID      string
    DisplayName string
}

type Record struct {
    ID            string
    Seed          uint64
    FirstDealer   engine.Seat
    EngineVersion int
    Status        Status
    Players       [4]PlayerSeat
    Actions       []LoggedAction // ordenadas por Ply
}

type LoggedAction struct {
    Ply    int
    Action engine.Action
    Source Source // player | timeout | system
}

// Step é um passo da reconstrução: a ação aplicada, os eventos e os estados antes/depois.
type Step struct {
    Logged LoggedAction
    Before engine.GameState
    After  engine.GameState
    Events []engine.Event
}

func Rebuild(rec Record) (engine.GameState, error)
func Walk(rec Record, fn func(Step) error) (engine.GameState, error) // Rebuild = Walk com fn nil

// package match (Record/LoggedAction/Source vêm de replay)
func (s *Store) Create(ctx, seed uint64, firstDealer engine.Seat, players [4]PlayerSeat) (id string, err error)
func (s *Store) AppendAction(ctx, id string, ply int, a engine.Action, src Source) error // ErrPlyConflict, ErrMatchClosed
func (s *Store) Finish(ctx, id string) error   // ErrMatchClosed, ErrNotGameOver, ErrCorruptLog...
func (s *Store) Abandon(ctx, id string, reason string) error
func (s *Store) Load(ctx, id string) (Record, error)
func (s *Store) ListInProgress(ctx) ([]Record, error)
func (s *Store) RecomputeStats(ctx) (skipped []string, err error)
```

- `Create`: transação com `matches` + 4 `match_players`; `engine_version = engine.RulesVersion`. Usuários repetidos → erro.
- `AppendAction`: insert simples. Conflito de PK → `ErrPlyConflict`. Partida não `in_progress` → `ErrMatchClosed` (checado no mesmo statement: `INSERT … SELECT … WHERE status = 'in_progress'`).
- **Contrato com a sala (Spec 3):** a sala aplica `Apply`; se der certo, chama `AppendAction` **antes** de enviar estado/eventos. Se `AppendAction` falhar, a sala descarta o novo estado e responde erro ao jogador, como se a jogada tivesse sido rejeitada. Quando o estado chega a `GameOver`, a sala chama `Finish`.
- `Finish`: transação com `SELECT … FOR UPDATE` em `matches`; exige `in_progress`; carrega o log; `Rebuild`; exige `PhaseGameOver`; `stats.Derive`; grava `status='finished'`, `winner_team`, `score_a/b`, `finished_at`, linhas de `match_player_stats` e upsert somando em `player_stats`. Não recebe estado da sala.
- `Abandon`: `status='abandoned'`, `finished_at=now`, `abandon_reason`. Não gera estatísticas.

### 7.3 Rebuild e Recover

- `replay.Rebuild(rec Record) (engine.GameState, error)`:
  - `rec.EngineVersion != engine.RulesVersion` → `ErrIncompatibleVersion`.
  - `engine.NewGame(players, seed, firstDealer)` com `Player{ID: UserID, Name: DisplayName}`.
  - Aplica as ações em ordem; exige `Ply` contíguo a partir de 0; qualquer erro de `Apply` → `*CorruptLogError{Ply, Err}`.
- `match.Recover(ctx, store) ([]Recovered, error)` com `Recovered{Record, State engine.GameState, NextPly int}`:
  - Para cada partida `in_progress`, executa `Rebuild`.
  - Falha de reconstrução → `Abandon(id, "corrupt_log" | "incompatible_version")` e `slog.Error`; segue para a próxima. O servidor sobe mesmo assim.
  - Partida reconstruída já em `GameOver` (crash entre o último `AppendAction` e `Finish`) → chama `Finish` e não a devolve.

### 7.4 Estatísticas (`stats.Derive`)

`stats.Derive(rec replay.Record) ([4]PlayerMatchStats, error)` é pura: percorre o log com `replay.Walk`, observando **ações** (decisões individuais, com `Source`) e **eventos/estado** (resultados). Para cada assento:

| Contador | Definição |
|---|---|
| `won` | Dupla do assento é `GameState.Winner` |
| `points_for` / `points_against` | Placar final da própria dupla / da adversária |
| `hands_played` | Eventos `hand_ended` |
| `hands_won` | `hand_ended` com `Team` = dupla do assento (empate total não conta) |
| `raises_called` | Ações do assento `request_raise` + `vote` com `choice=raise` |
| `raises_folded` | `raise_resolved` com `choice=run` em que a dupla votante (`Team`) é a do assento |
| `raises_won_by_fold` | `raise_resolved` com `choice=run` em que a dupla votante é a adversária |
| `eleven_hands` | Mãos `eleven` em que a dupla do assento tinha 11 |
| `eleven_runs` | `eleven_decided` com `choice=run` e `Team` = dupla do assento |
| `eleven_won` | Mãos `eleven` em que a dupla do assento tinha 11 e venceu a mão |
| `iron_hands` / `iron_won` | Mãos `iron` jogadas / vencidas pela dupla do assento |
| `punished` | `raise_punished` com `Seat` = assento |
| `timeouts` | Ações do assento com `Source = timeout` |

O tipo da mão vem de `state.Hand.Kind` observado após `sys_start_hand`.

### 7.5 Ranking e consultas

- Ranking: `player_stats` com `matches_played >= RANKING_MIN_MATCHES`, ordenado por `wins DESC`, `wins::float / matches_played DESC`, `matches_played DESC`, `username ASC`. Limite padrão 50.
- `recompute-stats` (`match.Store.RecomputeStats`): transação única que esvazia `match_player_stats` e `player_stats` e re-deriva todas as partidas `finished` em ordem de `finished_at`. Partidas cujo `Rebuild` falhe são reportadas e puladas.

### 7.6 Endpoints de leitura (autenticados)

| Rota | Retorno |
|---|---|
| `GET /api/ranking?limit=` | `[{position, username, displayName, matchesPlayed, wins, losses, winRate}]` |
| `GET /api/users/{username}` | `{username, displayName, createdAt, stats}` |
| `GET /api/matches?user={username}&cursor=` | Partidas `finished`/`abandoned` do usuário, keyset por `(finished_at DESC, id DESC)`, 20 por página, `nextCursor` opaco (base64url) |
| `GET /api/matches/{id}` | `{id, status, players[4], winnerTeam, score, createdAt, finishedAt}` |
| `GET /api/matches/{id}/replay` | Só `finished`/`abandoned` (senão `409 match_in_progress`). `{id, players, steps: [{ply, source, action, events, views[4]}]}`: `events` são todos os eventos da ação; `views[i] = engine.ViewFor(Step.After, i)`, montados com `replay.Walk`. Resposta comprimida com gzip quando o cliente aceita. |

## 8. Erros

Formato único: `{"error": {"code": "...", "message": "...", "fields": {"campo": "motivo"}?}}` com mensagem em pt-BR. Códigos: `validation_failed`, `email_domain_not_allowed`, `username_taken`, `invalid_token`, `invalid_credentials`, `email_not_verified`, `account_disabled`, `password_reset_required`, `unauthenticated`, `forbidden`, `forbidden_origin`, `too_many_attempts`, `not_found`, `match_in_progress`, `conflict`, `internal`. Erros internos são logados com `request_id` e respondidos como `500 internal` sem detalhes.

## 9. HTTP

- Roteamento `http.ServeMux` com padrões de método (`POST /api/auth/login`).
- Cadeia de middlewares: request id → recover (500) → log de acesso (`slog`, sem corpo nem cookie) → headers de segurança → sessão → (Origin em mutantes) → (requireAuth/requireAdmin por rota).
- `STATIC_DIR` definido: serve arquivos e devolve `index.html` para rotas que não são `/api/*` nem arquivos existentes (SPA).
- `GET /healthz`: 200 se o pool responde a `SELECT 1`.
- Desligamento gracioso em `SIGINT`/`SIGTERM` (`http.Server.Shutdown`, 10s).

## 10. Configuração

| Variável | Padrão | Notas |
|---|---|---|
| `DATABASE_URL` | obrigatória | |
| `HTTP_ADDR` | `:8080` | |
| `APP_BASE_URL` | obrigatória | Ex.: `https://truco.empresa.com.br`; `APP_ORIGIN` = esquema + host |
| `COOKIE_SECURE` | `true` | |
| `TRUST_PROXY` | `false` | |
| `ALLOWED_EMAIL_DOMAINS` | — | Lista separada por vírgula; só popula tabela vazia. Tabela vazia e variável vazia → start falha |
| `MAIL_DRIVER` | `resend` | `resend` ou `log` |
| `RESEND_API_KEY` | — | Obrigatória com `resend` |
| `MAIL_FROM` | — | Obrigatória com `resend`; ex.: `Truco <truco@empresa.com.br>` |
| `RANKING_MIN_MATCHES` | `5` | |
| `MIGRATE_ON_START` | `true` | |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | `json` | `json` ou `text` |
| `STATIC_DIR` | — | Opcional |

`.env.example` versionado; `.env` já está no `.gitignore`.

## 11. Ambiente de desenvolvimento

- `docker-compose.yml` na raiz: serviço `postgres` (`postgres:18`), usuário/banco `truco`, porta `5432`, volume nomeado, healthcheck `pg_isready`.
- Makefile ganha: `dev-db` (`docker compose up -d postgres`), `run` (`go run ./cmd/server`), `migrate`, `gen-sql` (`sqlc generate` em versão fixada). `test`, `test-short` e `gen-types` continuam.
- `sqlc.yaml`: engine `postgresql`, schema `internal/db/migrations`, queries `internal/db/queries`, saída `internal/db/dbgen`, `sql_package: pgx/v5`, `emit_json_tags: false`, overrides de `uuid` para `pgtype.UUID`/string conforme necessário.

## 12. Estratégia de testes

TDD, como na Spec 1.

**Unitários (sem banco)**

- `auth`: hash/verify argon2id, rejeição de hash malformado, rehash quando os parâmetros mudam, geração e hash de tokens, normalização e validação (tabela com casos válidos/inválidos).
- `ratelimit`: buckets por chave, bloqueio por falhas e expiração, eviction de chaves ociosas (relógio injetado).
- `replay.Rebuild`/`Walk`: round-trip com centenas de partidas aleatórias (gerador da Spec 1 exposto via helper de teste) — estado reconstruído `reflect.DeepEqual` ao estado ao vivo; log com ação inválida → `CorruptLogError` com `Ply` correto; buraco de ply; versão incompatível.
- `stats.Derive`: partidas roteirizadas com contagens exatas para cada contador; invariantes em partidas aleatórias (`points_for` = placar final; `hands_won` das duas duplas ≤ `hands_played`; exatamente uma dupla com `won`).
- `engine`: `json.Marshal(GameState{})` falha; `ViewFor` continua serializando.
- `mail`: `ResendMailer` contra `httptest.Server` (headers, corpo, retry em 5xx, sem retry em 4xx); renderização dos templates.
- `config`: obrigatórias ausentes, valores inválidos, derivação de `APP_ORIGIN`.

**Integração (Postgres real via testcontainers)**

- Helper `internal/testdb`: um contêiner `postgres:18` por pacote (`TestMain`), migra um banco-template uma vez; cada teste recebe `CREATE DATABASE t_<n> TEMPLATE truco_template`. Sem Docker → `t.Skip` com mensagem clara; `-short` pula.
- Migrations: up → down → up sem erro.
- `match.Store`: create/append/finish/abandon, `ErrPlyConflict`, `ErrMatchClosed`, `Finish` de partida não terminada, estatísticas gravadas e agregadas corretamente após várias partidas, `Recover` (incluindo log corrompido → abandonada e partida em `GameOver` → finalizada).
- `auth.Service`: todos os fluxos da §5.4 com `mail.Fake` e relógio fake; expiração e renovação deslizante; teto absoluto; rotação na troca de senha; revogações.
- `admin.Service`: todas as operações e restrições da §6, auditoria na mesma transação.
- `httpapi` ponta a ponta com `httptest.Server` + cookie jar: cadastro → token do `mail.Fake` → verificação → login → `/api/me` → logout; forgot → reset → sessões antigas inválidas; respostas idênticas nos fluxos anti-enumeração; `403 forbidden_origin`; `429` com `Retry-After`; rotas de admin com e sem permissão; replay de partida finalizada e `409` para partida em andamento.

**CI (GitHub Actions)**

`go vet ./...`, `go test -race ./...` (Docker disponível no runner), `make gen-sql` e `make gen-types` seguidos de `git diff --exit-code`.

## 13. Critérios de pronto (Spec 2)

- `make dev-db && make run` sobe o servidor com migrations aplicadas e `/healthz` verde.
- `go test -race ./...` passa com testes de integração executados (não pulados) localmente e no CI.
- Fluxos da §5.4 e da §6 cobertos por testes HTTP ponta a ponta.
- `Rebuild` reproduz centenas de partidas aleatórias sem divergência.
- `internal/engine` continua importando só a stdlib; `GameState` não é serializável.
- README atualizado (status do sub-projeto 2, como rodar, variáveis de ambiente).
