# Truco Multiplayer — Spec 4a: Frontend jogável (contas, lobby e mesa)

- **Data:** 2026-10-15
- **Status:** aprovado em conversa, aguardando revisão escrita
- **Escopo desta spec:** aplicação web em Vue 3 que permite jogar ponta a ponta no navegador — base do app, contas (cadastro, login, verificação de e-mail, redefinição de senha, perfil), lobby ao vivo, sala de espera e mesa de jogo com animações e sons.
- **Depende de:** Spec 1 (engine + protocolo), Spec 2 (contas e persistência) e Spec 3 (salas e WebSocket), implementadas.
- **Consome:** API REST `/api/auth/*`, `/api/me*`; endpoint `/ws`; tipos gerados em `frontend/src/protocol/` (`engine.ts`, `messages.ts`, `union.ts`); `spaHandler` com `STATIC_DIR`.
- **Mudança no backend:** apenas `cmd/gen-protocol` passa a exportar `frontend/src/protocol/cards.json` (§6.4).
- **Fora de escopo:**
  - Spec 4b — histórico, ranking, perfil público, replay e painel admin.
  - Chat livre, espectadores, matchmaking automático.
  - Internacionalização (a interface é só pt-BR).
  - PWA/offline e notificações push.

## 1. Contexto e objetivo

Colegas de empresa (dezenas de usuários, uma instância) jogam truco em 2 duplas. O servidor já joga partidas completas por WebSocket, mas não há interface. Esta spec entrega a interface mínima **completa** para jogar: entrar, achar ou criar sala, sentar, marcar pronto, jogar até o fim e voltar para a revanche.

### Critérios de sucesso

- Quatro pessoas se cadastram, verificam o e-mail, entram, montam uma sala, jogam uma partida inteira e veem o resultado — tudo no navegador.
- Recarregar a página (ou cair a rede) no meio da partida retoma a mesa no mesmo estado.
- O e2e com 4 navegadores (§9.3) passa de forma estável.
- Nenhuma regra de jogo é reimplementada no cliente: o que é permitido vem do `PlayerView` e de `pendingSeats`.

## 2. Decisões

| Decisão | Escolha | Motivo |
|---|---|---|
| Fatiamento | 4a (jogável) e 4b (histórico/ranking/replay/admin) | Spec única ficaria grande demais; 4a já entrega valor |
| Plataforma | Desktop primeiro; celular funcional em retrato simplificado | Uso principal em notebook/monitor no trabalho |
| Stack | Vite + Vue 3 (`<script setup>`, TS strict) + Pinia + Vue Router + Tailwind v4 | Decidido na Spec 1 |
| Fluxo WS → UI | Snapshot do servidor como verdade + diretor de eventos que anima/sonoriza e então aplica o `state` | Segue o contrato do servidor (`events` narra, `state` corrige); UI sem regra |
| Cartas | Conjuntos SVG livres (domínio público/CC0; CC-BY com crédito) servidos localmente | Visual clássico; CSP `'self'` |
| Acabamento | Animações leves + efeitos sonoros com volume/mudo | Pedido do usuário |
| Animação | CSS + `<Transition>`/`<TransitionGroup>`; sem biblioteca | Leve; suficiente para o que a mesa precisa |
| Testes | Vitest + Vue Test Utils; Playwright para e2e | Padrão do ecossistema Vite/Vue |
| Dev | Vite em `:5173` com proxy de `/api` e `/ws` para `:8080` | Já bate com `APP_BASE_URL` do `.env.example` e com a checagem de origem |
| Produção | `vite build` → `frontend/dist`, servido pelo Go via `STATIC_DIR` | `spaHandler` já existe |

## 3. Uso obrigatório de skills de frontend

O plano e a implementação **devem** invocar as skills abaixo nas etapas indicadas. O plano marca em cada tarefa qual skill carregar antes de começar.

| Skill | Quando |
|---|---|
| `frontend-design:frontend-design` | Antes de definir tokens visuais, tema da mesa e layout das telas (§7.4) e antes de cada tela nova (lobby, sala de espera, mesa, contas) |
| `frontend-dev-guidelines` | No scaffold e ao estabelecer a arquitetura de pastas, stores e convenções de componentes |
| `typescript-expert` | No `ws/client.ts`, no roteamento exaustivo da união `ServerMessage` e nos tipos derivados das stores |
| `ui-ux-pro-max` | No desenho do painel de ação contextual da mesa e dos fluxos de contas (estados vazios, erros, carregamento) |
| `fixing-accessibility` / `ui-a11y` | Ao concluir cada tela: rótulos, foco, teclado, contraste, `aria-live` |
| `e2e-testing` / `playwright-skill` | Ao montar o e2e de 4 navegadores (§9.3) |
| `lint-and-validate` | Ao final de cada tarefa: `vue-tsc`, ESLint e Vitest sem erros |

As skills orientam o trabalho; quando uma recomendação delas conflitar com esta spec, vale a spec, e o conflito é registrado no PR.

## 4. Estrutura

```
frontend/
├── index.html
├── package.json            # pnpm; scripts dev, build, test, e2e, lint, typecheck
├── vite.config.ts          # proxy /api e /ws → http://localhost:8080 (ws: true)
├── tsconfig*.json          # strict
├── eslint.config.ts
├── playwright.config.ts
├── e2e/                    # testes Playwright (§9.3)
└── src/
    ├── main.ts
    ├── App.vue
    ├── router.ts
    ├── protocol/           # GERADO pelo make gen-types — não editar
    ├── api/
    │   ├── http.ts         # fetch JSON, erros tipados {status, code, message}, offset de relógio
    │   └── auth.ts         # register, verifyEmail, resendVerification, login, logout,
    │                       # logoutAll, forgotPassword, resetPassword, me, updateMe, changePassword
    ├── ws/
    │   └── client.ts       # conexão única, reconexão, seq, roteamento (§5)
    ├── stores/
    │   ├── session.ts
    │   ├── lobby.ts
    │   ├── room.ts
    │   ├── game.ts
    │   └── prefs.ts
    ├── game/
    │   ├── director.ts     # fila de eventos → cena/som → aplica state (§6)
    │   ├── cards.ts        # CardID → Card, rótulos e arquivos por baralho
    │   ├── seats.ts        # rotação de assentos relativa a yourSeat
    │   └── labels.ts       # textos pt-BR de eventos, fases, erros (por code)
    ├── audio/
    │   └── sfx.ts
    ├── views/
    │   ├── EntrarView.vue, CadastroView.vue, VerificarEmailView.vue,
    │   │   EsqueciSenhaView.vue, RedefinirSenhaView.vue, PerfilView.vue
    │   ├── LobbyView.vue
    │   └── SalaView.vue    # alterna SalaEspera / Mesa conforme room.status
    ├── components/
    │   ├── ui/             # Botao, Campo, Modal, Toast, Toggle, IndicadorConexao
    │   ├── lobby/          # ListaSalas, LinhaSala, CriarSala, EntrarComCodigo
    │   ├── sala/           # SalaEspera, Assento
    │   └── mesa/           # Mesa, AssentoMesa, Mao, Carta, Vaza, Centro, Placar, Prazo,
    │                       # PainelAcao, Corte, Queima, DecisaoOnze, RespostaAumento,
    │                       # BarraSinais, BalaoSinal, BannerWO, FimPartida
    └── assets/
        ├── cards/{frances,espanhol}/   # SVGs + LICENSES.md
        ├── sfx/                        # OGG + MP3 + LICENSES.md
        └── fonts/                      # auto-hospedadas
```

**Regras de dependência**

- `components/` e `views/` leem stores e chamam ações das stores; nunca importam `ws/client` nem `api/` diretamente (exceto as views de contas, que usam `api/auth` por meio da store `session`).
- `stores/` dependem de `ws/client`, `api/`, `game/` e `protocol/`.
- `game/` e `audio/` não dependem de Vue além de `ref`/`reactive` quando necessário; `cards.ts`, `seats.ts` e `labels.ts` são funções puras.
- `protocol/` é gerado; mudanças vêm do backend via `make gen-types`.

**Makefile (novos alvos):** `web-install`, `web-dev`, `web-build`, `web-test` (typecheck + lint + Vitest), `e2e`.

**CI:** novo job `frontend` em `.github/workflows/ci.yml`: `pnpm install --frozen-lockfile`, `typecheck`, `lint`, `test`, `build`. O e2e roda localmente (`make e2e`), fora do CI nesta spec.

## 5. Cliente WebSocket (`ws/client.ts`)

- **Uma conexão por aba**, aberta após `session` confirmar o usuário (`GET /api/me`). URL relativa: `${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/ws`.
- **Roteamento:** `JSON.parse` → `ServerMessage`; `switch (msg.type)` exaustivo com `never` no `default`, de modo que um tipo novo no protocolo quebre a compilação. Mensagem malformada é logada e descartada.
- **Envio:** `send(msg: ClientMessage): Promise<void>` atribui `seq` incremental. A promise é rejeitada se chegar `error{seq}` correspondente e resolvida na próxima mensagem do servidor que não seja erro para esse `seq` dentro de 5 s (ou imediatamente, para ações sem resposta direta, como `signal`). Erro sem `seq` vai para o toast global.
- **Reconexão:** backoff exponencial com jitter, de 0,5 s até 10 s; zera ao receber `hello`. Enquanto desconectado, `session.connection = 'reconnecting'` e a UI mostra o indicador; ações de jogo ficam desabilitadas.
- **Códigos de fechamento do servidor:**

| Código | Significado | Reação |
|---|---|---|
| `4001` | conexão substituída por outra aba | não reconecta; mostra "Aberto em outra aba" com botão "Usar aqui" (reabre a conexão) |
| `4003` | sessão inválida | limpa `session`, vai para `/entrar?volta=…` |
| `4008` | violação de política (tamanho/taxa) | loga, reconecta com backoff |
| demais / queda | — | reconecta com backoff |

- **Upgrade recusado com 401** (sessão expirada antes do upgrade) é tratado como `4003`.
- **Versão:** `hello.protocolVersion` diferente da constante esperada pelo cliente → banner "Nova versão disponível — recarregue a página"; o cliente continua conectado.
- **Testável:** a fábrica de `WebSocket` e o relógio (`setTimeout`/`Math.random`) são injetáveis.

## 6. Stores e diretor de eventos

### 6.1 Stores Pinia

| Store | Estado | Entradas | Ações |
|---|---|---|---|
| `session` | `user`, `connection` (`idle`/`connecting`/`open`/`reconnecting`/`replaced`), `clockOffsetMs` | `hello`, respostas HTTP | `load`, `login`, `logout`, `logoutAll`, `register`, `updateDisplayName`, `changePassword` |
| `lobby` | `rooms: Map<id, RoomSummary>` | `lobby_snapshot`, `room_upserted`, `room_removed` | `createRoom(visibility)`, `joinById(id, seat?)`, `joinByCode(code, seat?)` |
| `room` | `room: RoomStatePayload \| null`, `forfeitTeam`, `ended: MatchEndedPayload \| null` | `room_state`, `left_room`, `forfeit_available`, `match_ended` | `takeSeat`, `setReady`, `kick`, `leave`, `claimForfeit`, `dismissEnded` |
| `game` | `view`, `deadlines`, `pendingSeats`, `signals` (por assento, com expiração) | `state`, `events` (via diretor), `signal` | `chooseCard`, `burn`, `passBurn`, `playCard`, `requestRaise`, `vote`, `sendSignal` |
| `prefs` | `deck` (`frances`/`espanhol`), `volume` (0–1), `muted` | — | setters; persistidos em `localStorage` dentro de `try/catch`, com padrão se indisponível |

- `left_room` limpa `room` e `game` e navega para `/`; motivo `kicked` mostra toast "Você foi removido da sala".
- `room_state` com `status = waiting` limpa `game` (fim de partida/revanche).
- **Getters do `game`** (derivados do `view`, sem regra própria): `mySeat`, `isMyTurn` (`view.turn === mySeat`), `amPending` (`pendingSeats` inclui `mySeat`), `canPlay`, `canRaise` (fase `playing`, minha vez, sem `pending`, `lastRaiser` ≠ minha dupla, `stake` < 12), `canCover` (fase `playing`, minha vez, `handKind` ≠ `iron`), `nextStakeLabel` (Truco/Seis/Nove/Doze), `mustRespondRaise` (fase `awaiting_raise_response` e `votingTeam` = minha dupla e eu ainda não votei), `raiseIsSuicide` (`handKind` ∈ {`eleven`, `iron`}), `seatsRelative` (rotação via `game/seats.ts`).
- **Quando um getter discorda do servidor**, vale o servidor: o erro de regra volta em `error{seq}`, vira toast e o botão é destravado.

### 6.2 Diretor de eventos (`game/director.ts`)

- `events` recebidos são enfileirados. O `state` seguinte vira **estado pendente** e só é aplicado ao `game.view` quando a fila esvazia.
- Cada `EventKind` tem um handler com duração e efeito:

| Evento | Cena | Som | Duração base |
|---|---|---|---|
| `hand_started` | limpa a mesa, mostra "Mão N" (e "Mão de 11"/"de Ferro") | embaralhar | 600 ms |
| `cut_offered`, `card_chosen` | destaca o assento que escolhe | — | 300 ms |
| `cards_dealt` | cartas deslizam do monte para os assentos | distribuir | 700 ms |
| `fours_burned` | cartas saem da mão do assento para o descarte | queimar | 500 ms |
| `vira_revealed` | vira gira no centro; destaca manilha | virar | 500 ms |
| `eleven_decided` | balão "Joga"/"Corre" no assento | — | 600 ms |
| `card_played` | carta voa do assento para a vaza (coberta se `card` ausente) | carta / carta coberta | 350 ms |
| `round_ended` | destaca a carta vencedora; marca a rodada | vaza ganha | 700 ms |
| `raise_requested` | balão grande "TRUCO!/SEIS!/NOVE!/DOZE!" | truco | 900 ms |
| `vote_cast` | balão do voto no assento (só se visível) | — | 400 ms |
| `raise_resolved` | "Aceito"/"Correu" e novo valor | aceitar / correr | 700 ms |
| `raise_punished` | aviso "Pedido proibido — derrota" | derrota | 1200 ms |
| `hand_ended` | pontos sobem para o placar | ponto | 800 ms |
| `game_over`, `forfeited` | — (o modal vem com `match_ended`) | vitória / derrota | 0 ms |

- Os componentes leem a **cena** do diretor (`scene`: cartas em voo, balões, destaques) por cima do `view` aplicado.
- **Atraso:** se a soma das durações enfileiradas passar de 3 s, ou se `document.hidden`, o diretor descarta a fila e aplica o estado pendente imediatamente (sons não tocam).
- **`prefers-reduced-motion`:** durações caem para 0 ms (ou ~50 ms quando necessário para a ordem visual); sons continuam conforme `prefs`.
- **`state` sem `events` antes** (reconexão, primeira carga, revanche) é aplicado na hora.
- **Fila nunca bloqueia ações:** se o usuário agir com fila pendente, a ação é enviada normalmente; a UI usa `amPending`/`isMyTurn` do estado pendente mais recente para habilitar botões.
- O diretor é uma classe pura com relógio injetável, testável sem DOM.

### 6.3 Prazos

- `deadlines[seat]` é epoch ms UTC do servidor. `session.clockOffsetMs` é estimado pelo cabeçalho `Date` das respostas HTTP (média das últimas amostras, atualizado no `GET /api/me` e em cada resposta).
- `Prazo.vue` desenha um anel com o tempo restante, atualizado por `requestAnimationFrame`; nos últimos 5 s do **meu** prazo toca o som de alerta uma vez.
- O cliente nunca age quando o prazo acaba; o servidor aplica o timeout.

### 6.4 Cartas (`game/cards.ts`)

- `cardFromId(id) = { rank: RANK_LADDER[Math.floor(id / 4)], suit: id % 4 }`, espelhando `engine.CardFromID`.
- **Paridade:** `cmd/gen-protocol` (ou um teste Go) exporta `frontend/src/protocol/cards.json` com a tabela `id → {rank, suit}`; um teste Vitest compara com `cardFromId` para os 40 ids. O arquivo entra no `git diff --exit-code` do CI.
- Rótulos: francês `1→A`, `10→J`, `11→Q`, `12→K`, demais iguais; espanhol, número sem alteração. Naipes: `0` ouros ♦, `1` espadas ♠, `2` copas ♥, `3` paus ♣.
- Arquivo: `assets/cards/<baralho>/<rank>-<naipe>.svg` (`naipe` ∈ `ouros|espadas|copas|paus`) e `verso.svg`.
- Rótulo acessível: `"3 de copas"`, `"A de espadas"` (francês) / `"1 de espadas"` (espanhol).

## 7. Telas e fluxos

### 7.1 Rotas e guarda

| Rota | Acesso | Tela |
|---|---|---|
| `/entrar` | pública (logado → `/`) | login |
| `/cadastro` | pública | cadastro |
| `/verificar-email?token=` | pública | verificação |
| `/esqueci-senha` | pública | pedido de redefinição |
| `/redefinir-senha?token=` | pública | nova senha |
| `/` | autenticada | lobby |
| `/sala` | autenticada | sala de espera / mesa |
| `/sala/:codigo` | autenticada | entra por código e redireciona para `/sala` |
| `/perfil` | autenticada | perfil e preferências |

- O guarda chama `session.load()` (`GET /api/me`) uma vez por carregamento; 401 → `/entrar?volta=<rota>`. Após login, volta para `volta` (só caminhos internos).
- Se chegar `room_state` enquanto o usuário está fora de `/sala`, o app navega para `/sala` (usuário já em sala: reconexão, outra aba).
- `/sala` sem `room` (após `hello` e sem `room_state`) redireciona para `/`.

### 7.2 Contas

- **Cadastro:** `username`, `email`, `displayName`, `password` (com confirmação). Sucesso → tela "Confira seu e-mail" com "Reenviar" (`resend-verification`, respeitando rate limit).
- **Verificar e-mail / Redefinir senha:** fazem `POST` do token ao abrir (verificação) ou ao enviar a nova senha (redefinição). Sucesso → mensagem e link para entrar; token inválido/expirado → mensagem e ação de recuperação (reenviar verificação / pedir nova redefinição).
- **Login:** campo `login` aceita usuário ou e-mail. Erros por `code` com texto em pt-BR de `labels.ts` (credenciais inválidas, e-mail não verificado com botão de reenviar, rate limit/lockout com tempo de espera quando informado).
- **Esqueci a senha:** resposta sempre neutra ("se o e-mail existir, enviamos um link").
- **Perfil:** alterar `displayName`; alterar senha (atual + nova); "Sair"; "Sair de todos os dispositivos"; preferências (baralho com pré-visualização, volume, mudo).
- Formulários: validação local só de formato/obrigatoriedade; a validação real é do servidor, com erros de campo exibidos junto ao campo quando o `code` permitir.

### 7.3 Lobby e sala de espera

- **Lobby:** lista ao vivo das salas públicas (nome, status, 4 assentos vazios/nomes, placar se em jogo). Salas cheias ou em jogo ficam desabilitadas. Botões "Criar sala" (pública/privada) e "Entrar com código". Clicar num assento vazio de uma linha entra direto naquele assento.
- **Sala de espera (`status = waiting`):** mesa com 4 assentos clicáveis (`take_seat`), duplas indicadas por cor (0 e 2 × 1 e 3); cada assento mostra nome, pronto, conectado e coroa do host. Toggle "Pronto". Host vê "Remover" em assentos ocupados. Sala privada mostra o código e "Copiar link" (`/sala/CODIGO`). "Sair da sala" (`leave_room`).

### 7.4 Mesa (`status = playing`)

- **Disposição:** eu embaixo, parceiro em cima, adversários à esquerda e à direita (rotação a partir de `yourSeat`). Centro: vira, manilha, vaza atual, rodadas da mão, monte (`deckCount`) e descarte (`discardCount`). Topo: placar por dupla, valor da mão (`stake`), tipo da mão, dealer.
- **Vez e prazos:** o assento da vez fica destacado; cada assento em `pendingSeats` exibe o anel de prazo.
- **Painel de ação contextual** (só aparece quando eu tenho algo a decidir):

| Fase | Painel |
|---|---|
| `cut_choice` | as cartas de `cutOffer` para escolher 1 (`choose_card{keep}`) |
| `dealer_choice` | idem, para o dealer |
| `burn_fours` | selecionar os 4 da mão para queimar (`burn{slots}`) ou "Passar" (`pass_burn`) |
| `eleven_decision` | mostra `partnerHand`, voto do parceiro (`partnerVote`) e "Jogar"/"Correr" (`vote{play|run}`) |
| `playing` | clicar numa carta joga (`play_card{slot, covered}`); toggle "Coberta"; botão "Truco/Seis/Nove/Doze" (`request_raise`) |
| `awaiting_raise_response` | para a dupla que responde: `partnerHand`, voto do parceiro (`partnerVote`) e "Aceitar", "Correr", "Aumentar" (`vote{accept|run|raise}`); "Aumentar" some quando `pending.toStake = 12` |

- **Cobertura:** o toggle "Coberta" aparece na minha vez em qualquer rodada, exceto na Mão de Ferro; se o servidor recusar (`covered_not_allowed`), o toast explica e o toggle é desmarcado.
- **Mão de Ferro:** as cartas da mão vêm com `hidden: true` inclusive para mim; a mão mostra versos clicáveis ("carta 1/2/3") e a carta só é revelada ao bater na mesa.
- **Mão de 11:** só a dupla com 11 vê o painel `eleven_decision`; a outra dupla vê "Aguardando a dupla com 11 decidir".
- **Confirmação de pedido proibido:** se `raiseIsSuicide`, o botão de aumento abre um modal "Pedir truco na Mão de 11/de Ferro faz sua dupla perder a partida. Confirmar?".
- **Sinais:** barra com `truco`, `boa`, `vamos`, `calma`, `desculpa`, `valeu`; o sinal recebido aparece como balão sobre o assento por ~3 s. Envio com limite local de 1 por segundo (o servidor também limita).
- **Presença:** assento com `connected=false` ou `absent=true` fica esmaecido com ícone e dica. `forfeit_available{team}` mostra o banner "Adversários ausentes — pedir W.O." com botão (`claim_forfeit`); `team: null` retira o banner.
- **Fim de partida:** `match_ended` abre modal com resultado (vitória/derrota da minha dupla), motivo (`game_over`, `forfeit`, `aborted`, `abandoned`) e placar. "Revanche" fecha o modal e mostra a sala de espera (o servidor já voltou a `waiting` com `ready=false`); "Sair da sala" envia `leave_room`.
- **Celular (< 768 px):** adversários e parceiro numa faixa compacta no topo, centro reduzido, mão e painel de ação embaixo em largura total.
- **Visual:** mesa de feltro verde; tema claro/escuro seguindo o sistema; cores, raios e sombras como tokens no Tailwind. O desenho final sai da aplicação de `frontend-design:frontend-design` (§3).

### 7.5 Erros e estados globais

- `error{seq}` → toast com texto de `labels.ts` por `code` (sala e regra); destrava o botão que aguardava.
- Indicador de conexão no cabeçalho (`reconnecting`, `replaced`).
- Falha de rede em HTTP → toast "Sem conexão com o servidor"; 5xx → "Erro no servidor, tente de novo".

## 8. Assets, áudio e acessibilidade

### 8.1 Cartas

- **Licença:** apenas domínio público ou CC0; CC-BY aceito com crédito em `LICENSES.md` e na página de perfil (rodapé "Créditos"). **CC-BY-SA e LGPL não entram.**
- **Francês:** *Vector Playing Cards* (Byron Knoll), domínio público. Só as 40 cartas usadas (sem 8, 9, 10 numérico e coringas; os rótulos J/Q/K mapeiam ranks 10/11/12) e um verso.
- **Espanhol:** conjunto da Wikimedia Commons em domínio público/CC0, com a licença de cada arquivo verificada antes do commit. **Fallback** se não houver conjunto completo nessas condições: cartas espanholas geradas em SVG por código (índice, naipe e figura estilizada simples), mantendo os mesmos nomes de arquivo.
- SVGs otimizados (SVGO) antes do commit; `LICENSES.md` por pasta com origem, autor, licença e URL por arquivo ou por conjunto.
- Teste: as 40 cartas e o verso existem nos dois baralhos.

### 8.2 Sons

- Origem: Kenney (*Casino Audio*, *Interface Sounds*), CC0. OGG com fallback MP3; total < 300 KB.
- Lista: embaralhar, distribuir, carta, carta coberta, queimar, virar, vaza ganha, truco (impacto, sem voz), aceitar, correr, ponto, sua vez, prazo acabando, vitória, derrota.
- `audio/sfx.ts`: Web Audio com buffers pré-carregados; contexto desbloqueado no primeiro gesto do usuário; volume e mudo de `prefs`. "Sua vez" só toca com a aba em foco; fora de foco, o título da aba alterna para "Sua vez!" até a aba ganhar foco.

### 8.3 Acessibilidade

- Cartas da mão são `<button>` com rótulo acessível; cartas de outros assentos têm `aria-label` descritivo.
- Teclado: `1`–`3` jogam a carta do slot; `C` alterna "Coberta"; `T` pede aumento; `A`/`R`/`U` respondem Aceitar/Correr/Aumentar; `Esc` fecha modais. Atalhos ignorados quando o foco está num campo de texto.
- Região `aria-live="polite"` anuncia "Sua vez", pedidos de truco, fim de rodada e de mão.
- Contraste AA; foco visível; modais com foco preso e retorno de foco.
- Animações respeitam `prefers-reduced-motion`.

## 9. Testes

### 9.1 Unitários (Vitest)

- `cards.ts`: paridade com `cards.json`; rótulos por baralho; nomes de arquivo; rótulo acessível.
- `seats.ts`: rotação para cada `yourSeat`.
- `ws/client.ts` (WebSocket falso, relógio falso): roteamento por `type`; `seq` e rejeição por `error{seq}`; backoff e reset no `hello`; `4001` não reconecta; `4003` limpa sessão; banner de versão.
- `director.ts` (relógio falso): ordem dos handlers; aplicação do estado pendente ao fim; descarte quando a fila passa de 3 s ou a aba está oculta; reduced-motion; `state` sem eventos aplicado na hora.
- Stores: getters do `game` a partir de `PlayerView` fixos de cada fase (fixtures JSON); `lobby` com snapshot/upsert/remove; `room` com `left_room`, `forfeit_available`, `match_ended`; `prefs` com `localStorage` indisponível.

### 9.2 Componentes (Vue Test Utils)

- `PainelAcao` em cada fase (botões certos habilitados).
- Modal de confirmação na Mão de 11/de Ferro.
- `BannerWO` aparece e some.
- `FimPartida` por motivo.
- Formulários de contas: exibição de erros por `code`.

### 9.3 E2E (Playwright)

- Sobe Postgres (`make dev-db`) e o servidor com `STATIC_DIR=frontend/dist`, `MAIL_DRIVER=log`, timers curtos (`TIMER_*_SECONDS=3`, `HAND_PAUSE_SECONDS=1`) e domínio de e-mail de teste.
- **Partida completa:** 4 contextos de navegador → cadastro → token de verificação lido do log do servidor → login → um cria sala privada, os outros entram pelo link → todos prontos → joga até `match_ended` (cada jogador, quando tem ação pendente, executa a primeira ação habilitada do painel: carta de menor slot, "Passar", "Jogar", "Aceitar") → confere o modal de fim nos 4.
- **Retomada:** no meio da partida, um jogador recarrega a página → a mesa reaparece com a mesma mão e o mesmo placar.
- `make e2e` faz o build, sobe os processos, roda e derruba.

## 10. Critérios de aceite

- [ ] `pnpm typecheck`, `lint`, `test` e `build` sem erros; job `frontend` no CI verde.
- [ ] Fluxos de contas funcionando contra o backend real (cadastro → verificação → login → perfil → redefinição).
- [ ] Lobby ao vivo, criação de sala pública e privada, entrada por lista, por código e por link.
- [ ] Partida completa jogável com animações, sons, prazos, sinais e W.O.
- [ ] Reconexão e "aberto em outra aba" funcionando.
- [ ] Baralho francês e espanhol alternáveis, com licenças registradas.
- [ ] E2E de partida completa e de retomada passando.
- [ ] Skills de frontend (§3) aplicadas nas etapas indicadas.
- [ ] README atualizado: status da Spec 4a, como rodar o frontend em dev e em produção.
