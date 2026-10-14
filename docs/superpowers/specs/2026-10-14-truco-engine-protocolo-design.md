# Truco Multiplayer — Spec 1: Engine de Regras + Protocolo de Eventos

- **Data:** 2026-10-14
- **Status:** aprovado em conversa, aguardando revisão escrita
- **Escopo desta spec:** engine de regras em Go puro e contrato de mensagens WebSocket.
- **Fora de escopo (specs futuras, nesta ordem):**
  - Spec 2 — Contas e Persistência.
  - Spec 3 — Servidor de salas (`net/http` + `coder/websocket`, lobby, goroutine por sala, timers, reconexão).
  - Spec 4 — Frontend (Vue 3 + TS + Pinia + Tailwind).

### Decisões já tomadas para specs futuras

- **Banco:** PostgreSQL (dev via Docker Compose).
- **Contas:** cadastro com usuário, e-mail e senha (hash argon2id); login por e-mail **ou** usuário (ambos únicos, case-insensitive); logout; recuperação de senha por e-mail via **Resend**; cadastro restrito a domínio(s) de e-mail configurável(is) da empresa; rate limit no login.
- **Sessão:** token opaco aleatório em cookie `HttpOnly`/`Secure`/`SameSite`, apenas o hash armazenado no banco (revogável); o mesmo cookie autentica o handshake do WebSocket.
- **Persistência de partidas:** seed + log de ações de cada partida (gravado a cada `Apply` bem-sucedido). O mesmo log serve para **replay completo** e para **recuperar partidas em andamento** após restart (reaplicando as ações). Histórico, estatísticas e ranking são derivados ao fim de cada partida.
- **Impacto nesta spec:** nenhum além de `Player.ID` (id do usuário) e de as Actions serem serializáveis (já previsto).

## 1. Contexto e objetivo

Jogo de Truco (variante paulista com regras da casa) para 4 jogadores em 2 duplas, jogado em tempo real por colegas de empresa e usado como projeto de portfólio. Prioridades:

1. **Servidor como única fonte da verdade.** O cliente recebe apenas uma projeção do estado filtrada para seu assento.
2. **Regras corretas e testáveis.** Engine determinístico, sem I/O, coberto por testes unitários, de invariantes e de vazamento.
3. **Simplicidade operacional.** Uma instância; contas e persistência em PostgreSQL (Spec 2).

## 2. Decisões de arquitetura

| Decisão | Escolha | Motivo |
|---|---|---|
| Núcleo do engine | Reducer puro `Apply(state, action) → (state, events, error)` | Determinístico, replay, testes sem mocks; a sala apenas serializa ações |
| Tempo | Engine não lê relógio; a sala injeta `SysTimeout` / `SysStartHand` | Mantém pureza; timers são da Spec 3 |
| Aleatoriedade | `math/rand/v2` PCG derivado de `(Seed, HandNumber)` a cada mão | Replay determinístico sem estado de RNG mutável |
| Tipos TS | Gerados do Go via `tygo` | Go é a fonte; sem divergência manual |
| HTTP/WS (Spec 3) | stdlib `net/http` (roteamento 1.22+), `coder/websocket`, `slog` | Mínimo de dependências |
| Frameworks no engine | Nenhum | Engine é Go puro |

## 3. Estrutura do monorepo

```
truco/
├── backend/
│   ├── go.mod                      # module github.com/caarlosandree/truco-multiplayer/backend
│   ├── cmd/server/main.go          # (Spec 3)
│   ├── internal/
│   │   ├── engine/                 # PURO: sem I/O, goroutines ou time.Now
│   │   │   ├── card.go             # Rank, Suit, Card, CardID
│   │   │   ├── deck.go             # baralho de 40 cartas, Shuffle(rng)
│   │   │   ├── strength.go         # ManilhaRank, Strength, RoundWinner
│   │   │   ├── deal.go             # corte com escolha, distribuição, queima
│   │   │   ├── state.go            # GameState, HandState, Phase, Seat, Team
│   │   │   ├── actions.go          # tipos de Action
│   │   │   ├── events.go           # tipos de Event + Visibility
│   │   │   ├── errors.go           # RuleError
│   │   │   ├── apply.go            # Apply: despacho por fase
│   │   │   ├── betting.go          # escada de aposta e voto da dupla
│   │   │   ├── scoring.go          # resolução de rodadas/mão, Mão de 11, Ferro
│   │   │   └── view.go             # ViewFor: projeção anti-cheat
│   │   ├── protocol/               # envelopes WS e DTOs (fonte do tygo)
│   │   ├── room/                   # (Spec 3)
│   │   └── transport/ws/           # (Spec 3)
│   └── tygo.yaml
├── frontend/                       # (Spec 4)
│   └── src/protocol/engine.ts, messages.ts  # GERADOS
├── docs/superpowers/specs/
└── Makefile                        # test, gen-types, dev
```

Dependências do `internal/engine`: apenas a stdlib. `internal/protocol` depende de `engine` (reexporta/mapeia tipos de view).

## 4. Modelo de cartas

### 4.1 Representação neutra

- `Suit` (uint8), valor = ordem de desempate entre manilhas: `Diamonds=0 (♦ Ouros) < Spades=1 (♠ Espadas) < Hearts=2 (♥ Copas) < Clubs=3 (♣ Paus)`.
- `Rank` (uint8) com valores neutros `1,2,3,4,5,6,7,10,11,12`. O frontend mapeia para exibição:
  - Francês: `1→A`, `10→J`, `11→Q`, `12→K`, demais iguais.
  - Espanhol: número sem alteração.
- `Card{Rank, Suit}`; serializada como `CardID uint8` em `0..39` (`id = rankIndex*4 + suit`, onde `rankIndex` é a posição na escada da §4.2).
- Baralho: 40 cartas (sem 8, 9 e Jokers).

### 4.2 Força

- Escada base (índice 0 = mais fraca): `[4, 5, 6, 7, 10, 11, 12, 1, 2, 3]`.
- `ManilhaRank(vira) = escada[(idx(vira.Rank) + 1) % 10]`. Ex.: Vira 3 → 4; Vira 7 → 10; Vira 12 → 1.
- `Strength(card, covered, manilhaRank)`:
  - Carta coberta: `0`.
  - Manilha: `11 + suit` (11..14).
  - Comum: `1 + idx(rank)` (1..10).
- Comuns de mesmo rank empatam (naipe não desempata). Coberta contra coberta empata.

### 4.3 Vencedor da rodada

Calcula a maior força de cada dupla entre as cartas jogadas na rodada. Maior força vence; igualdade = `Tie`. O **assento vencedor** é quem jogou a carta de maior força da dupla vencedora.

## 5. Assentos, duplas e turnos

- Assentos `0..3` em sentido horário. `Team(seat) = seat % 2` (dupla A = 0,2; dupla B = 1,3). `Partner(s) = (s+2) % 4`. `Next(s) = (s+1) % 4`.
- Quem corta: `(dealer + 3) % 4` (à direita do dealer).
- O dealer avança `Next(dealer)` a cada nova mão.

## 6. Máquina de estados da mão

```
CutChoice → DealerChoice → [Deal] → BurnFours → [RevealVira]
  → ElevenDecision? → Playing ⇄ AwaitingRaiseResponse → HandEnd → (GameOver | SysStartHand → nova mão)
```
Fases entre colchetes são automáticas (executadas dentro do mesmo `Apply`).

### 6.1 Início da mão (`SysStartHand`)

Embaralha o monte com `rand.New(rand.NewPCG(Seed, HandNumber))`. Determina o tipo de mão:
- **Normal:** nenhuma dupla com 11.
- **Mão de 11:** exatamente uma dupla com 11.
- **Mão de Ferro:** ambas com 11.

### 6.2 CutChoice e DealerChoice (não ocorrem na Mão de Ferro)

- **CutChoice:** quem corta recebe (só em sua projeção) as 2 cartas do topo. Ação `ChooseCard{Keep: 0|1}`: fica com a escolhida; a outra vai para `Partner(cortador)` = `dealer+1`.
- **DealerChoice:** idem para o dealer; a outra vai para `Partner(dealer)` = `dealer+2`.

### 6.3 Deal (automático)

A partir de `dealer+1`, sentido horário, entrega 1 carta por vez pulando quem já tem 3, até todos terem 3. Na Mão normal/de 11 são 8 cartas; na Mão de Ferro, 12 (todas às cegas, sem fase de escolha).

### 6.4 BurnFours (não ocorre na Mão de Ferro)

- Cada jogador com ao menos um 4 envia `Burn{Slots}` (subconjunto de slots contendo 4) ou `PassBurn`. Jogadores sem 4 são marcados como decididos automaticamente; se ninguém tem 4, a fase é pulada.
- Decisões são **simultâneas**; ao completar as 4, as trocas são resolvidas em ordem horária a partir de `dealer+1`: cada 4 queimado vai para a pilha de descarte (fora do jogo) e é reposto pela carta do topo, no mesmo slot.
- Carta comprada que seja 4 **não** pode ser queimada novamente.

### 6.5 RevealVira (automático)

A carta do topo do monte vira a Vira (pública) e define `ManilhaRank`.

### 6.6 ElevenDecision (somente Mão de 11)

- A dupla com 11 vota `Play` ou `Run` (mecanismo de voto da §7.3). Durante a votação os parceiros veem a mão um do outro.
- `Run`: adversário +1; mão encerra.
- `Play`: `Stake = 3`; segue para `Playing`.

### 6.7 Playing

- Rodada 1 começa em `dealer+1`. Rodadas seguintes começam no assento vencedor da rodada anterior; após rodada empatada, começa **quem abriu a rodada empatada**.
- Na sua vez, o jogador pode:
  - `PlayCard{Slot, Covered}` — joga carta aberta ou coberta (coberta permitida em qualquer rodada; **proibida na Mão de Ferro**, onde `Covered=true` gera erro).
  - `RequestRaise` — antes de jogar (ver §7).
- Na Mão de Ferro, o jogador escolhe apenas o `Slot`; a carta só é revelada ao ser jogada.
- Ao completar 4 cartas, resolve a rodada (§4.3) e verifica se a mão está decidida (§8.1).

### 6.8 HandEnd / GameOver

- Aplica pontos (§8.2). Se alguma dupla atingir ≥ 12, fase `GameOver` (placar exibido limitado a 12).
- Caso contrário, aguarda `SysStartHand` (a sala envia após pausa para animação).

## 7. Apostas

### 7.1 Escada

`[1, 3, 6, 9, 12]`. `HandState.Stake` começa em 1 (3 na Mão de 11, 1 na Mão de Ferro). `HandState.LastRaiser` (`*Team`) registra a dupla que aumentou por último.

### 7.2 Pedido (`RequestRaise`)

Válido somente se: fase `Playing`, é a vez do solicitante, `Stake < 12`, `LastRaiser != Team(solicitante)`.

**Punição:** na Mão de 11 ou na Mão de Ferro, o pedido é aceito pelo engine e a dupla infratora perde a partida — o adversário vai a 12 e a fase vira `GameOver` (evento `RaisePunished`). O frontend deve exibir confirmação antes de enviar.

Fora disso, cria `Pending{Requester, FromTeam, PrevStake: Stake, ToStake: próximo degrau}` e muda para `AwaitingRaiseResponse`.

### 7.3 Voto da dupla

Usado em `AwaitingRaiseResponse` (dupla adversária a `FromTeam`) e em `ElevenDecision` (dupla com 11).
- Cada parceiro envia `Vote{Choice}` uma vez (`already_voted` em repetição).
- Quando ambos votarem, prevalece o mais ousado: `Raise > Accept > Run` (truco) e `Play > Run` (Mão de 11).
- Durante a votação, os parceiros votantes veem a mão um do outro e o voto do parceiro; adversários veem apenas "votou/não votou".

### 7.4 Resultado

- **Accept:** `Stake = ToStake`, `LastRaiser = FromTeam`, volta para `Playing` com a vez em `Requester`.
- **Run:** `FromTeam` ganha `PrevStake`; mão encerra.
- **Raise:** permitido só se `ToStake < 12`. Novo `Pending{Requester (inalterado), FromTeam: dupla votante, PrevStake: ToStake anterior, ToStake: próximo degrau}`; `LastRaiser` = dupla votante; a outra dupla passa a votar. Ao final de aceitações, a vez continua com o `Requester` original.

## 8. Pontuação

### 8.1 Resolução da mão (melar paulista)

Com resultados `r1, r2, r3 ∈ {A, B, Tie}`:
- Uma dupla que vence 2 rodadas vence a mão.
- `r1 = Tie`: vence quem vencer `r2`; se `r2 = Tie`, vence quem vencer `r3`; se `r3 = Tie`, **ninguém pontua**.
- `r1 = X` e `r2 = Tie`: vence X (sem jogar a 3ª).
- `r1 = X`, `r2 = Y ≠ X`: decide `r3`; se `r3 = Tie`, vence X.
- A mão termina assim que o vencedor estiver determinado.

### 8.2 Pontos

- Vencedor da mão soma `Stake`.
- Correr de aumento: `FromTeam` soma `PrevStake`.
- Correr na Mão de 11: adversário +1.
- Punição por pedir aumento na Mão de 11/Ferro: adversário vai a 12.

## 9. Projeção anti-cheat (`ViewFor(state, seat) PlayerView`)

Único caminho pelo qual o estado sai do servidor.

**Público:** placar, dealer, fase, vez, tipo de mão, `Stake`, `Pending`, resultados das rodadas, Vira (após revelada), cartas na mesa (abertas com `CardID`; cobertas como `{covered: true}` sem ID — exceto para o próprio dono), contagem de cartas na mão de cada jogador, contagem do monte, contagem de queimadas, status de voto (votou/não votou).

**Privado ao assento:**
- Própria mão como `[]HandSlot{Slot, Card *CardID, Hidden bool}`; na Mão de Ferro todos `Hidden: true` e `Card: nil`.
- Opções do corte (2 cartas, como `[{keep, card}]`) somente para quem está escolhendo, durante sua fase.
- `PartnerHand` quando a dupla do assento está votando (truco ou ElevenDecision).
- Voto do parceiro, durante votação da própria dupla.

**Nunca enviado:** ordem/conteúdo do monte, identidade das cartas queimadas, seed do RNG, mãos de adversários, cartas cobertas alheias.

## 10. Tipos principais (contrato)

```go
type Seat uint8 // 0..3
type Team uint8 // 0 = assentos 0,2; 1 = assentos 1,3

type Phase string // "cut_choice", "dealer_choice", "burn_fours", "eleven_decision",
                  // "playing", "awaiting_raise_response", "hand_end", "game_over"

type HandKind string // "normal", "eleven", "iron"

type GameState struct {
    Seed    uint64
    HandNumber int          // RNG da mão = PCG(Seed, HandNumber)
    Players [4]Player
    Score   [2]int
    Dealer  Seat
    Hand    *HandState
    Phase   Phase
    Winner  *Team
}

type Player struct {
    Seat Seat
    ID   string // atribuído pela sala
    Name string
}

type HandState struct {
    Kind        HandKind
    Deck        []Card     // monte restante (topo = último)
    Hands       [4][]Card  // índice = slot
    Discarded   []Card     // 4s queimados
    CutOffer    []Card     // 2 cartas em oferta na fase de escolha
    Vira        *Card
    Manilha     Rank
    Stake       int
    LastRaiser  *Team
    Pending     *PendingRaise
    Votes       [4]*VoteChoice
    BurnDecided [4]bool
    BurnSlots   [4][]int
    Turn        Seat
    RoundStarter Seat
    Table       []PlayedCard  // rodada corrente
    Rounds      []RoundResult // até 3
}

type PlayedCard struct {
    Seat    Seat
    Card    Card
    Covered bool
}

type RoundResult struct {
    Winner     *Team // nil = empate
    WinnerSeat *Seat
    Starter    Seat
}

type PendingRaise struct {
    Requester Seat
    FromTeam  Team
    PrevStake int
    ToStake   int
}
```

Actions (cliente): `ChooseCard{Keep}`, `Burn{Slots}`, `PassBurn`, `PlayCard{Slot, Covered}`, `RequestRaise`, `Vote{Choice}`. Actions de sistema (sala): `SysStartHand`, `SysTimeout{Seat}`. Toda action carrega o `Seat` de origem, preenchido pela sala (nunca pelo cliente).

Events (para animação), cada um com `Visibility{Kind: all|seat|team, Seat, Team}`: `HandStarted`, `CutOffered` (seat), `CardChosen`, `CardsDealt`, `FoursBurned` (contagem pública), `ViraRevealed`, `ElevenDecided`, `CardPlayed`, `RoundEnded`, `RaiseRequested`, `VoteCast` (team), `RaiseResolved`, `RaisePunished`, `HandEnded`, `GameOver`.

## 11. Protocolo WebSocket

Envelope JSON:

```json
// cliente → servidor
{ "type": "play_card", "seq": 17, "payload": { "slot": 1, "covered": false } }

// servidor → cliente
{ "type": "state",  "payload": { /* PlayerView */ } }
{ "type": "events", "payload": [ /* Event filtrados */ ] }
{ "type": "error",  "payload": { "code": "not_your_turn", "seq": 17 } }
```

- Tipos de cliente (engine): `choose_card`, `burn`, `pass_burn`, `play_card`, `request_raise`, `vote` (`choice ∈ accept|run|raise|play`). Mensagens de lobby (`join`, `ready` etc.) são da Spec 3.
- Após cada `Apply` bem-sucedido a sala envia, para cada assento, `events` filtrados e um `state` com a `PlayerView` completa (snapshot idempotente; também usado na reconexão).
- `seq` é eco do cliente para correlacionar erros.
- Tipos TS gerados em `frontend/src/protocol/engine.ts` e `messages.ts` via `make gen-types` (tygo).

## 12. Erros

`Apply` retorna `*RuleError{Code, Msg}` e o estado inalterado. Códigos: `invalid_phase`, `not_your_turn`, `invalid_slot`, `invalid_choice`, `illegal_raise`, `already_voted`, `not_voting_team`, `covered_not_allowed`, `burn_not_four`, `game_over`, `already_decided` (queima já decidida), `invalid_seat` (assento fora de 0..3).

## 13. Ação padrão por fase (para `SysTimeout`)

O engine expõe `DefaultAction(state, seat) (Action, bool)`; a sala decide quando usar (Spec 3).
- CutChoice/DealerChoice: `ChooseCard{Keep: 0}`.
- BurnFours: `PassBurn`.
- ElevenDecision: `Vote{Run}`.
- AwaitingRaiseResponse: `Vote{Run}`.
- Playing: `PlayCard{menor slot, Covered: !iron}`.

## 14. Estratégia de testes

TDD com `go test`, sem dependências externas.
- **Força:** testes de tabela para as 10 Viras (ciclo 3→4), ordem de naipes das manilhas, empates comuns e cobertas.
- **Distribuição:** conservação das 40 cartas, ausência de duplicatas, 3 cartas por jogador, destino correto das cartas do corte (cortador→`dealer+1`, dealer→`dealer+2`), ordem de entrega, queima (descarte, reposição no mesmo slot, sem re-queima), Ferro sem corte/queima.
- **Apostas:** escada, bloqueio de aumento consecutivo pela mesma dupla, voto ousado, correr com `PrevStake`, retorno da vez ao `Requester`, punição na Mão de 11/Ferro.
- **Pontuação:** todas as combinações de `r1,r2,r3` da §8.1.
- **Partidas aleatórias:** milhares de seeds com ações legais aleatórias até `GameOver`, verificando invariantes a cada passo (conservação de cartas, placar monotônico, fase consistente).
- **Vazamento:** para cada estado visitado e cada assento, serializar `ViewFor` em JSON e afirmar que só aparecem `CardID`s permitidos para aquele assento naquele momento.

## 15. Critérios de pronto (Spec 1)

- `go test ./...` passa com cobertura alta do pacote `engine`.
- `make gen-types` gera `engine.ts` e `messages.ts`.
- Teste de vazamento e de partidas aleatórias verdes.
- Nenhuma importação fora da stdlib em `internal/engine`.
