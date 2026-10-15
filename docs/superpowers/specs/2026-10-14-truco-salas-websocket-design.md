# Truco Multiplayer — Spec 3: Salas, Lobby, WebSocket, Timers e Reconexão

- **Data:** 2026-10-14
- **Status:** aprovado em conversa, aguardando revisão escrita
- **Escopo desta spec:** servidor de jogo em tempo real — lobby com salas listadas e privadas, ator por sala, protocolo WebSocket tipado, timers por fase, presença/ausência, W.O., revanche, sinais rápidos, recuperação após restart, desligamento gracioso e as pendências herdadas da Spec 2.
- **Depende de:** Spec 1 (engine + protocolo) e Spec 2 (contas e persistência), implementadas.
- **Consome da Spec 2:** `match.Store` (`Create`, `AppendAction`, `Finish`, `Abandon`, `Load`), `match.Store.Recover`, `auth.Service.SessionFromRequest`, `auth.Service.Authenticate`, `auth.CheckOrigin`. **Implementa** `admin.MatchAborter`.
- **Fora de escopo:**
  - Spec 4 — frontend (lobby, mesa, contagem regressiva, botão de W.O., sinais).
  - Chat livre e espectadores.
  - Matchmaking automático (fila).
  - Múltiplas instâncias do servidor.

## 1. Contexto e objetivo

Colegas de empresa (dezenas de usuários, uma instância) jogam partidas de 4 jogadores em 2 duplas. O engine é a fonte da verdade e o log de ações já é durável. Esta spec liga tudo em tempo real:

1. **Encontrar partida sem atrito:** lista de salas ao vivo; sala privada por código/link; duplas escolhidas pelo assento.
2. **Partida que sempre anda:** timers por fase jogam por quem não decide; ausentes são acelerados.
3. **Sem derrota escondida:** quem some perde por W.O. (acionado pelos adversários) e a partida conta no ranking.
4. **Resiliência:** queda de conexão, troca de aba e restart do servidor não perdem a partida.

### Critérios de sucesso

- Quatro clientes WebSocket de teste criam uma sala, entram, marcam pronto, jogam até `GameOver` — com timeouts e reconexões no meio — e a partida termina `finished` no banco; a sala volta para revanche.
- Um restart no meio da partida retoma a sala com os mesmos jogadores e o mesmo estado.
- As 7 pendências herdadas (§11) resolvidas e cobertas por teste.

## 2. Decisões

| Decisão | Escolha | Motivo |
|---|---|---|
| Concorrência | Ator por sala (goroutine + caixa de entrada) e ator de lobby | Sem locks no domínio; timers, jogadas e abort serializados; testável com relógio manual |
| Conexão | Um único endpoint `/ws` por usuário, multiplexando lobby e sala | Um protocolo; troca de aba = substituição atômica |
| Biblioteca WS | `github.com/coder/websocket` | Decidido na Spec 1 |
| Encontro | Salas listadas em tempo real + salas privadas por código | Duplas combinadas entre colegas; link para privadas |
| Ocupação | Um usuário em no máximo uma sala | Simplifica reconexão e roteamento; reduz contenção em estatísticas |
| Persistência da sala | Nenhuma (só a partida) | Sala é efêmera; o `Recover` recria salas a partir das partidas |
| Timeout | Sala aplica `DefaultAction` e grava a ação concreta com `source=timeout` | Contrato da Spec 2 (§4.7) |
| Ausência | Desconectado **ou** 3 timeouts seguidos | Aba aberta parada não escapa do W.O. |
| Abandono | W.O. sob demanda dos adversários após prazo; `SysForfeit` no engine | Derrota conta; protege contra queda momentânea |
| Tempo | Interface `clock.Clock` injetada | Timers determinísticos em teste, sem `sleep` |
| Tipos TS | Payloads tipados + gerador próprio de uniões discriminadas | Fecha a pendência de `Payload any` |

## 3. Estrutura de pacotes

```
backend/
├── cmd/
│   ├── server/main.go          # serve: Recover → lobby.Restore → HTTP; shutdown ordenado (§9)
│   └── gen-protocol/main.go    # gera uniões TS a partir do registro de mensagens (§7.4)
└── internal/
    ├── engine/                 # + ActSysForfeit, EvForfeited; GameState/HandState movidos para gamestate.go
    ├── clock/                  # Clock, Timer; Real e Manual
    ├── room/                   # ator da sala (sem rede, sem banco concreto)
    │   ├── room.go             # loop, caixa de entrada, estados
    │   ├── seats.go            # assentos, pronto, host, kick
    │   ├── timers.go           # prazo de decisão, pausa entre mãos
    │   ├── presence.go         # conectado, timeouts seguidos, relógios de ausência, W.O.
    │   ├── signals.go          # sinais rápidos
    │   └── ports.go            # MatchStore, Outbox
    ├── lobby/                  # ator do lobby: registro, códigos, índice usuário→sala,
    │                           # lista ao vivo, Restore, Abort (MatchAborter), Shutdown
    ├── protocol/               # REDEFINIDO: envelopes e payloads tipados, registro de tipos
    └── transport/ws/           # handler /ws: autenticação, Accept, leitura/escrita, limites
```

**Regras de dependência**

- `engine` continua importando apenas a stdlib.
- `room` depende de `engine`, `protocol`, `clock`, `replay` (tipos `PlayerSeat`, `Source`) e das próprias interfaces. Não importa `match`, `net/http`, `websocket` nem `db`.
- `lobby` depende de `room`, `protocol`, `clock` e de uma interface sobre `match.Store`.
- `transport/ws` depende de `auth`, `lobby` e `protocol`; é o único a importar `coder/websocket`.
- `cmd/server` faz a montagem.
- `internal/testclock` é substituído por `internal/clock` (o `Manual` cobre o uso atual); usos existentes migram.

**Dependência nova:** `github.com/coder/websocket`.

### 3.1 Portas do ator da sala

```go
// package room
type MatchStore interface {
    Create(ctx context.Context, seed uint64, firstDealer engine.Seat, players [4]replay.PlayerSeat) (string, error)
    AppendAction(ctx context.Context, id string, ply int, a engine.Action, src replay.Source) error
    Finish(ctx context.Context, id string) error
    Abandon(ctx context.Context, id string, reason string) error
}

// Outbox entrega mensagens a um usuário; nunca bloqueia (a conexão tem fila própria).
type Outbox interface {
    Send(userID string, msg protocol.ServerMessage)
}

// package clock
type Clock interface {
    Now() time.Time
    AfterFunc(d time.Duration, f func()) Timer
}
type Timer interface{ Stop() bool }
```

Disparos de timer não executam lógica: apenas postam uma mensagem na caixa de entrada do ator.

## 4. Lobby e ciclo de vida da sala

### 4.1 Sala

| Campo | Regra |
|---|---|
| `id` | uuid em memória |
| `code` | 6 caracteres do alfabeto `23456789ABCDEFGHJKMNPQRSTUVWXYZ` (sem ambíguos); único entre salas vivas |
| `visibility` | `public` ou `private` |
| `host` | criador; ao sair, passa ao próximo ocupante por ordem de assento |
| `name` | `"Sala de {displayName do host original}"` |
| `status` | `waiting` ou `playing` |
| `seats[4]` | `{userID, displayName, ready, connected, absent}` ou vazio |
| `matchID` | preenchido em `playing` |

Estados: `waiting → playing → waiting`. Ao fim (`game_over`, W.O.) ou abort, volta a `waiting` com os mesmos ocupantes e `ready=false` para todos (revanche). Sala vazia é destruída.

### 4.2 Regras em `waiting`

- `create_room{visibility}`: cria a sala com o criador no assento 0 → `room_state`.
- `join_room{roomId | code, seat?}`: assento pedido (se livre) ou o primeiro livre. Sala privada só aceita `code`. Erros: `room_not_found`, `room_full`, `seat_taken`, `already_in_room`.
- `take_seat{seat}`: move para assento livre; desmarca o próprio `ready`.
- `set_ready{ready}`.
- `leave_room`: libera o assento → `left_room{reason: left}`.
- `kick{seat}`: só o host (`not_host`); alvo recebe `left_room{reason: kicked}`.
- Ocupante desconectado por mais de `WAITING_DISCONNECT_SECONDS` é removido do assento.
- **Início:** com 4 ocupantes e todos `ready`:
  1. `seed` (uint64) e `firstDealer` (0..3) de `crypto/rand`.
  2. `Store.Create` com o snapshot `{userID, displayName}` dos assentos.
  3. `engine.NewGame`, `Apply(sys_start_hand)`, `AppendAction(ply 0, source=system)`.
  4. `status = playing`; envia `room_state` e `state`/`events` por assento.
  Falha em 2 ou 3 → `Abandon(id, "start_failed")` quando a partida já existir, `status = waiting`, `ready=false` para todos e `error{internal}` aos ocupantes.

### 4.3 Regras em `playing`

- `leave_room`, `kick`, `take_seat`, `set_ready` → `match_in_progress`. Sair é fechar a conexão; a ausência (§5.3) cuida do resto.
- Mensagens de jogo e `claim_forfeit` são processadas conforme §5.

### 4.4 Lobby

- Mantém: registro de salas por `id` e por `code`; índice `userID → roomID`; conexões ativas por usuário; conjunto de usuários **fora de sala** (assinantes da lista).
- **Uma sala por usuário:** `create_room`/`join_room` com entrada no índice → `already_in_room`.
- **Lista ao vivo:** salas `public` em `waiting` e `playing`, com `{id, name, status, seats[4]{displayName}|null, score?}`. Ao ficar fora de sala, o usuário recebe `lobby_snapshot`; depois, deltas `room_upserted`/`room_removed`. As salas notificam o lobby de mudanças relevantes para a lista.
- `MAX_ROOMS` salas simultâneas → `too_many_rooms`.
- **Roteamento:** a conexão envia mensagens de sala diretamente para a caixa de entrada da sala corrente (referência atualizada pelo lobby ao entrar/sair); mensagens de lobby vão para o ator do lobby.

### 4.5 Restore (após restart)

`lobby.Restore([]match.Recovered)`: cada partida vira uma sala `playing`, `public`, `code` novo, host no assento 0, ocupantes do `Record.Players`, todos `connected=false`. O ator recebe `State` e `NextPly`; o prazo de decisão é armado a partir do estado (§5.1) e os relógios de ausência começam do zero no boot. Os 4 usuários entram no índice.

### 4.6 Abort do administrador (`admin.MatchAborter`)

`lobby.Abort(ctx, matchID)`: localiza a sala pela partida (sem sala → `nil`, já que o `admin.Service` grava `Abandon` antes). A sala cancela timers, envia `match_ended{reason: aborted}` e volta a `waiting`. Se a sala receber `ErrMatchClosed` de `AppendAction` (abort concorrente), trata como abort da mesma forma.

## 5. Timers, presença e W.O.

### 5.1 Prazo de decisão

- Após cada `Apply` bem-sucedido (ou no `Restore`), `pendingSeats = {s | engine.DefaultAction(state, s) ok}`.
- Uma **janela de decisão** é identificada por `(HandNumber, Phase, Turn, Pending)`; quando muda, o prazo é rearmado com a duração da fase:

| Fase | Duração |
|---|---|
| `playing` (jogar ou pedir aumento) | `TIMER_PLAY_SECONDS` |
| `awaiting_raise_response`, `eleven_decision` | `TIMER_VOTE_SECONDS` |
| `cut_choice`, `dealer_choice` | `TIMER_CHOICE_SECONDS` |
| `burn_fours` | `TIMER_BURN_SECONDS` |

- Em fases simultâneas (queima, votos), a ação de um assento **não** rearma o prazo dos demais.
- O prazo é **por assento pendente**: ao abrir a janela, cada assento pendente recebe `agora + duração`, ou `agora + min(duração, TIMER_ABSENT_SECONDS)` se estiver ausente. Se um assento pendente fica ausente durante a janela, seu prazo é encurtado para `min(prazo atual, agora + TIMER_ABSENT_SECONDS)`; voltar não o alonga.
- O `state` leva `deadlines` (`{seat: epochMs UTC}` para cada assento pendente) e `pendingSeats`.
- Cada timer carrega uma geração; disparos com geração antiga são ignorados.

### 5.2 Estouro do prazo

Para cada assento pendente cujo prazo estourou, em ordem de assento: `a := DefaultAction(state, seat)`, `Apply(a)`, `AppendAction(a, source=timeout)`, broadcast; `timeoutsSeguidos[seat]++`. Qualquer ação do próprio jogador, ou sua reconexão, zera o contador.

### 5.3 Ausência

- **Ausente** = `!connected || timeoutsSeguidos >= 3`.
- Por assento, a sala mede `ausenteDesde` (início da ausência contínua) e `ausenteAcumulado` na partida. Zeram a cada nova partida e no `Restore`.
- A ausência termina com reconexão ou ação do jogador.
- `room_state.seats[i].absent` reflete o estado para todos.

### 5.4 W.O.

- **Disponível contra a dupla T** quando, simultaneamente:
  - algum assento de T está ausente agora **e** (`now - ausenteDesde >= ABSENCE_GRACE_SECONDS` **ou** `ausenteAcumulado >= ABSENCE_BUDGET_SECONDS`);
  - nenhum assento da outra dupla está ausente.
- A sala arma um timer para o instante em que a condição passa a valer; ao valer, envia `forfeit_available{team: T}` aos dois assentos da outra dupla. Quando deixa de valer, envia `forfeit_available{team: null}`.
- `claim_forfeit` de um assento da outra dupla com o W.O. disponível: `Apply(SysForfeit{Team: T})`, `AppendAction(source=system)`, broadcast, `Finish`, `match_ended{reason: forfeit, winner, score}`, sala volta a `waiting`. Caso contrário → `forfeit_unavailable`.
- **Todos ausentes:** se os 4 assentos ficarem ausentes simultaneamente por `ABSENCE_BUDGET_SECONDS`, a sala chama `Abandon(id, "all_absent")`, envia `match_ended{reason: abandoned}` a quem estiver conectado e é destruída.

### 5.5 Pausa entre mãos e fim

- Fase `hand_end`: timer de `HAND_PAUSE_SECONDS`; ao disparar, `Apply(sys_start_hand)` + `AppendAction(source=system)`.
- Fase `game_over`: `Finish`; `match_ended{reason: game_over, winner, score}`; sala volta a `waiting`. Falha de `Finish` é logada; a partida permanece `in_progress` e será finalizada pelo `Recover` no próximo boot (que detecta `GameOver`). A sala volta a `waiting` mesmo assim.

### 5.6 Falha de persistência

Se `AppendAction` falhar com erro diferente de `ErrMatchClosed`, a sala descarta o novo estado; se a ação veio de jogador, responde `error{internal, seq}`; se veio de timer, rearma o prazo com a duração mínima de 1 s para tentar de novo. Erros são logados com `room_id`/`match_id`/`ply`.

### 5.7 Sinais rápidos

`signal{id}` com `id ∈ {truco, boa, vamos, calma, desculpa, valeu}`, só em `playing` e para ocupantes; limite de 1 a cada 3 s por usuário (`rate_limited`). Reenviado a todos os ocupantes como `signal{seat, id}`. Não é persistido.

### 5.8 Reconexão

Nova conexão de um usuário presente no índice: o lobby troca a conexão e notifica a sala (`connected=true`, fim da ausência, `timeoutsSeguidos=0`). O usuário recebe `room_state`; em `playing`, também `state` (com `deadline`) e o `forfeit_available` corrente, se for o caso. Eventos perdidos não são reenviados — o `state` é snapshot idempotente.

## 6. Mudanças no engine

- `ActSysForfeit ActionKind = "sys_forfeit"`, campo `Action.Team` (dupla que perde).
- Válida em qualquer fase exceto `game_over` (`game_over` → `RuleError{game_over}`); `Team` fora de 0..1 → `invalid_choice`.
- Efeito: `Score[Other(Team)] = 12`, `Winner = Other(Team)`, `Phase = game_over`, `Hand` mantido como está; eventos `forfeited{team}` (visível a todos) e `game_over`.
- Nunca aceita do cliente: `protocol.DecodeAction` rejeita tipos `sys_*`.
- `DefaultAction` inalterado. `RulesVersion` permanece **1**: logs existentes reproduzem o mesmo resultado.
- `stats.Derive` não muda: `won`, `points_for` e `points_against` vêm do estado final.
- `GameState`, `HandState` e tipos estritamente internos saem de `state.go` para `gamestate.go`, excluído do tygo (§7.4). `state.go` fica com `Seat`, `Team`, `Phase`, `HandKind` e afins.

## 7. Protocolo

### 7.1 Envelope

```json
{ "type": "play_card", "seq": 17, "payload": { "slot": 1, "covered": false } }   // cliente → servidor
{ "type": "state", "payload": { ... } }                                         // servidor → cliente
```

`seq` é opcional e ecoado em `error`. `protocolVersion = 1` enviado no `hello`.

### 7.2 Cliente → servidor

| Tipo | Payload | Contexto |
|---|---|---|
| `create_room` | `{visibility: "public"\|"private"}` | fora de sala |
| `join_room` | `{roomId?: string, code?: string, seat?: 0..3}` (exatamente um de `roomId`/`code`) | fora de sala |
| `leave_room` | — | `waiting` |
| `take_seat` | `{seat}` | `waiting` |
| `set_ready` | `{ready: bool}` | `waiting` |
| `kick` | `{seat}` | `waiting`, host |
| `choose_card`, `burn`, `pass_burn`, `play_card`, `request_raise`, `vote` | Spec 1 §11 | `playing` |
| `claim_forfeit` | — | `playing` |
| `signal` | `{id}` | `playing` |

Payloads com campos desconhecidos ou obrigatórios ausentes → `error{bad_request}`.

### 7.3 Servidor → cliente

| Tipo | Payload |
|---|---|
| `hello` | `{user: {id, username, displayName}, protocolVersion}` |
| `lobby_snapshot` | `{rooms: RoomSummary[]}` |
| `room_upserted` | `{room: RoomSummary}` |
| `room_removed` | `{roomId}` |
| `room_state` | `{id, code, name, visibility, host: seat, status, seats: (SeatInfo\|null)[4], matchId?, yourSeat}` |
| `left_room` | `{reason: "left"\|"kicked"\|"aborted"\|"destroyed"}` |
| `state` | `{view: PlayerView, deadlines: {[seat]: epochMs}, pendingSeats: Seat[]}` |
| `events` | `Event[]` filtrados por assento |
| `forfeit_available` | `{team: Team\|null}` |
| `match_ended` | `{reason: "game_over"\|"forfeit"\|"aborted"\|"abandoned", winner: Team\|null, score: [2]int}` |
| `signal` | `{seat, id}` |
| `error` | `{code, message, seq?}` |

`RoomSummary = {id, name, status, seats: ({displayName}\|null)[4], score?: [2]int}`; `SeatInfo = {userId, displayName, ready, connected, absent}`.

Códigos de erro novos: `already_in_room`, `room_full`, `room_not_found`, `seat_taken`, `not_host`, `not_in_room`, `match_in_progress`, `forfeit_unavailable`, `too_many_rooms`, `rate_limited`, `bad_request`, `internal` — além dos códigos de regra do engine (Spec 1 §12).

### 7.4 Tipos TypeScript

- Cada tipo de mensagem tem uma struct Go de payload. `protocol` expõe um registro `ClientTypes`/`ServerTypes` (`map[string]reflect.Type` ou lista ordenada de `{Type, Payload}`).
- `make gen-types` roda o tygo (structs) e depois `go run ./cmd/gen-protocol`, que escreve `frontend/src/protocol/union.ts` com:
  ```ts
  export type ServerMessage = { type: "state"; payload: StatePayload } | { type: "events"; payload: engine.Event[] } | ...;
  export type ClientMessage = { type: "play_card"; seq?: number; payload: PlayCardPayload } | ...;
  ```
  Saída ordenada e estável (testada por golden file).
- `tygo.yaml` exclui `internal/engine/gamestate.go` (`exclude_files`); `engine.ts` deixa de exportar `GameState`/`HandState`.
- `ServerMessage.Payload any` deixa de existir no contrato gerado. Arquivos gerados nunca são editados à mão.

## 8. Transporte (`GET /ws`)

- **Autenticação antes do upgrade:** `SessionFromRequest` falha → `401` JSON; `CheckOrigin(r, APP_ORIGIN)` falha → `403 forbidden_origin`. Depois, `websocket.Accept` com `InsecureSkipVerify: true` (a origem já foi verificada contra `APP_ORIGIN`, o que funciona atrás de proxy).
- **Deadlines (pendência 2):** o `http.Server` mantém `ReadTimeout`/`WriteTimeout` para a API; o handler `/ws` zera os deadlines de leitura e escrita com `http.ResponseController` antes do `Accept`. A conexão passa a ser regida por:
  - ping a cada 20 s, pong em até 10 s;
  - timeout de 5 s por escrita;
  - `SetReadLimit(4096)`;
  - limite de taxa de 20 msg/s (burst 40) por conexão.
- **Violações:** mensagem acima do limite ou taxa excedida → fechamento `4008 policy_violation`. Mensagem malformada ou tipo desconhecido → `error{bad_request}` (conexão segue).
- **Fila de saída:** 64 mensagens por conexão; cheia → conexão fechada (`1013 try_again_later`); o cliente reconecta e recebe snapshot.
- **Uma conexão por usuário:** nova conexão substitui a antiga atomicamente no lobby; a antiga fecha com `4001 replaced`. A sala não vê desconexão nesse caso.
- **Revalidação de sessão:** a cada 5 min a conexão chama `auth.Service.Authenticate` com o token do handshake; sessão inválida → `4003 session_invalid`.
- **Desconexão:** encerramento da leitura notifica o lobby, que notifica a sala (`connected=false`).
- **Shutdown:** `1001 going_away`.

## 9. Boot e desligamento

### 9.1 Boot (`serve`)

1. Config, logger, pool, migrations (como hoje).
2. `recovered, err := store.Recover(ctx, logger)` — erro só se a listagem falhar (§10.1); nesse caso o boot falha.
3. `lobby := lobby.New(...)`; `lobby.Restore(recovered)`; `go lobby.Run(ctx)`.
4. `admin.NewService(... Aborter: lobby ...)`; rota `/ws` registrada no `httpapi`.
5. `ListenAndServe`; `RunMaintenance` em goroutine rastreada.

### 9.2 Desligamento (pendência 7)

`SIGINT`/`SIGTERM`, orçamento total `SHUTDOWN_TIMEOUT_SECONDS`, em ordem:

1. `lobby.Shutdown(ctx)`: recusa conexões, salas e mensagens novas; cada sala cancela timers, conclui o `AppendAction` em curso e para; conexões fecham com `1001`. Partidas ficam `in_progress` para o próximo `Recover`; salas em `waiting` se perdem.
2. `http.Server.Shutdown(ctx)`.
3. Cancela o contexto de `RunMaintenance` e aguarda o retorno.
4. `auth.Service.Wait(ctx)`: os envios de `sendLater` passam a ser rastreados por `sync.WaitGroup`; ao estourar o contexto, registra quantos ficaram pendentes.
5. `pool.Close()`.

## 10. Persistência — ajustes na Spec 2

### 10.1 `Recover` resiliente (pendência 1)

- Falha ao listar partidas `in_progress` → erro.
- Por partida: `incompatible_version`/`corrupt_log` → `Abandon` (como hoje); qualquer outra falha (`Load`, `Finish` no caminho de `GameOver`, `Abandon`) → `slog.Error` e **pula** a partida, que segue `in_progress` para o próximo boot.
- `logger == nil` → `slog.Default()`.
- Usuários de partidas puladas não entram no índice do lobby e ficam livres.

### 10.2 `AppendAction` (pendência 3)

- **Contrato:** o ator da sala é o único escritor de ações de sua partida.
- Mesmo assim, `AppendAction` executa em transação curta:
  1. `SELECT status FROM matches WHERE id = $1 FOR SHARE` (ausente → `ErrNotFound`);
  2. `status != 'in_progress'` → `ErrMatchClosed`;
  3. `next := coalesce(max(ply)+1, 0)` em `match_actions`; `ply < next` → `ErrPlyConflict`; `ply > next` → **`ErrPlyGap`** (novo);
  4. `INSERT`.
- O `FOR SHARE` serializa com `Abandon` (`UPDATE`) e `Finish` (`FOR UPDATE`): nenhuma ação entra depois de um abort confirmado.

### 10.3 Estatísticas (pendências 4 e 5)

- `writeStats` ordena as linhas por `user_id` antes dos inserts/upserts (ordem de lock determinística).
- `RecomputeStats` inicia a transação com `LOCK TABLE match_player_stats, player_stats IN EXCLUSIVE MODE`. `Finish` concorrente espera. Sem deadlock: `Finish` trava a linha de `matches` e depois linhas de estatística; o recompute trava as tabelas de estatística e apenas lê `matches`.

## 11. Pendências herdadas

| # | Pendência | Resolução |
|---|---|---|
| 1 | `serve` não chama `Recover`; falhas derrubam/entram em pânico | §9.1 e §10.1 |
| 2 | `WriteTimeout` derruba WebSocket | §8 (deadlines zerados no `/ws`) |
| 3 | `AppendAction` sem trava e sem checagem de contiguidade | §10.2 + contrato de escritor único |
| 4 | Deadlock potencial em `writeStats` | §10.3 (ordem por `user_id`) |
| 5 | `RecomputeStats` vs `Finish` concorrente | §10.3 (`LOCK TABLE`) |
| 6 | Tipos TS (`GameState` exportado; `Payload any`) | §6 e §7.4 |
| 7 | Goroutines de e-mail e manutenção não aguardadas | §9.2 |

## 12. Configuração

Novas variáveis (inteiros positivos, validados no start com mensagem clara):

| Variável | Padrão | Limites |
|---|---|---|
| `TIMER_PLAY_SECONDS` | 30 | 5..300 |
| `TIMER_VOTE_SECONDS` | 20 | 5..300 |
| `TIMER_CHOICE_SECONDS` | 15 | 5..300 |
| `TIMER_BURN_SECONDS` | 15 | 5..300 |
| `TIMER_ABSENT_SECONDS` | 5 | 1..60 |
| `HAND_PAUSE_SECONDS` | 4 | 0..30 |
| `ABSENCE_GRACE_SECONDS` | 120 | 10..3600 |
| `ABSENCE_BUDGET_SECONDS` | 300 | ≥ `ABSENCE_GRACE_SECONDS`, ≤ 7200 |
| `WAITING_DISCONNECT_SECONDS` | 60 | 5..3600 |
| `MAX_ROOMS` | 50 | 1..1000 |
| `SHUTDOWN_TIMEOUT_SECONDS` | 15 | 1..120 |

`.env.example` atualizado.

## 13. Observabilidade

`slog` com `room_id`, `match_id`, `user_id`, `seat` quando aplicável. Nível `info`: sala criada/destruída, partida iniciada/terminada (motivo), W.O., abort, abandono, restore. `warn`: timeout aplicado, conexão fechada por política. `error`: falhas de persistência. `debug`: conexão/desconexão.

## 14. Estratégia de testes

TDD, como nas specs anteriores. Testes de sala e lobby usam `clock.Manual`, `MatchStore` fake (que guarda o log) e `Outbox` gravador; nenhum `time.Sleep`.

**Unitários**

- `engine`: `SysForfeit` em cada fase (placar, `Winner`, fase, eventos); erro após `game_over`; `Team` inválido; `replay.Rebuild` de log com `sys_forfeit`; teste de vazamento continua verde.
- `clock`: `Manual.Advance` dispara em ordem de prazo; `Stop` impede o disparo.
- `room`:
  - ciclo `waiting → playing → waiting` e revanche;
  - assentos, pronto, kick, transferência de host, remoção por desconexão em `waiting`;
  - prazo por fase; fase simultânea não rearma; prazo encurtado para ausente; geração antiga ignorada;
  - timeout grava ação concreta com `source=timeout`; 3 timeouts seguidos → ausente; ação ou reconexão encerra ausência;
  - `forfeit_available` aos `ABSENCE_GRACE` contínuos e aos `ABSENCE_BUDGET` acumulados; retirada ao voltar; `claim_forfeit` válido e inválido; ambas as duplas com ausentes → indisponível;
  - 4 ausentes → `Abandon("all_absent")`;
  - falha de `AppendAction` descarta estado; `ErrMatchClosed` vira abort; falha de `Create` volta a `waiting`; falha de `Finish` não trava a sala;
  - sinais: ids válidos, limite de taxa.
- **Propriedade:** centenas de partidas com ações aleatórias, desconexões/reconexões e avanço de relógio até o fim; a cada passo, `replay.Rebuild` do log do fake == estado da sala; nenhuma mensagem enviada ao assento X contém `CardID` proibido para X.
- `lobby`: uma sala por usuário; privada fora da lista e só por código; snapshot e deltas; `MAX_ROOMS`; troca atômica de conexão; `Restore` popula o índice; `Abort` com e sem sala; `Shutdown` recusa novidades.
- `protocol`: decodificação de cada tipo de cliente, rejeição de campos desconhecidos e de `sys_*`; gerador de uniões contra golden file.
- `config`: padrões, limites e `ABSENCE_BUDGET >= ABSENCE_GRACE`.

**Integração (Postgres via testcontainers; neste host `TESTCONTAINERS_RYUK_DISABLED=true TRUCO_REQUIRE_DB=1`)**

- `AppendAction`: `ErrPlyGap`; `ErrPlyConflict`; `Abandon` concorrente → ação seguinte `ErrMatchClosed` (teste de corrida com goroutines).
- `Finish` simultâneo de partidas com os mesmos usuários sem deadlock (várias iterações, `-race`); `RecomputeStats` concorrente com `Finish` produz totais corretos.
- `Recover`: falha de `Load` em uma partida não impede as outras; logger `nil` não entra em pânico.
- **Ponta a ponta `transport/ws`** (`httptest.Server`, 4 clientes `coder/websocket` com cookies reais, relógio manual):
  - cadastro/login de 4 usuários → criar sala → entrar → prontos → jogar até `GameOver` → partida `finished` no banco → revanche;
  - `401` sem sessão; `403` com Origin errado;
  - segunda conexão do mesmo usuário fecha a primeira com `4001`;
  - reconexão recebe `room_state` + `state`;
  - W.O. ponta a ponta (desconexão, avanço de relógio, `claim_forfeit`, partida `finished`);
  - abort via `POST /api/admin/matches/{id}/abort` encerra a sala viva;
  - restart: `lobby.Shutdown` no meio da partida, `Recover` + `Restore` em lobby novo, 4 reconexões, partida concluída;
  - mensagem acima de 4 KB e excesso de taxa → `4008`;
  - shutdown gracioso fecha conexões com `1001` dentro do orçamento.

**CI:** `go vet ./...`, `go test -race ./...`, `make gen-sql gen-types` + `git diff --exit-code`.

## 15. Critérios de pronto (Spec 3)

- `go test -race ./...` verde com integração executada (não pulada) localmente e no CI.
- Ponta a ponta de 4 clientes e de restart passando.
- Pendências 1–7 resolvidas e cobertas por teste.
- `make gen-types` gera `union.ts` com uniões discriminadas; `engine.ts` sem `GameState`/`HandState`.
- `cmd/server serve` usa o lobby como `admin.MatchAborter` (sem `NoopAborter`).
- `internal/engine` importa apenas a stdlib; `RulesVersion` continua 1.
- README atualizado: status da Spec 3, protocolo WebSocket, variáveis novas.
