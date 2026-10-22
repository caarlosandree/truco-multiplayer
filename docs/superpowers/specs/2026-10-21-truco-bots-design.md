# Truco Multiplayer — Spec 5: Bots

- **Data:** 2026-10-21
- **Status:** rascunho para revisão do usuário
- **Depende de:** Specs 1–3 (engine, contas, salas/WebSocket), Spec 4a (frontend jogável) **e Spec 4b** (ranking, histórico, perfil, presença, admin). **Decisão do usuário:** a implementação começa depois que a 4b estiver mergeada em `staging`; o plano já edita as queries e telas da 4b (§9).
- **Escopo desta spec:** bots que completam as cadeiras vazias de uma mesa ("Começar com bots") e bots que assumem o lugar de quem ficou ausente no meio da partida. Inclui o cérebro do bot (forte), as mudanças de engine, sala, protocolo, persistência e telas.
- **Fora de escopo:** níveis de dificuldade escolhíveis, bot entrando sozinho em salas do lobby, matchmaking, bot que fala em chat livre, ranking próprio de bots.

## 1. Intenção

Colegas jogam truco entre si, mas nem sempre há quatro pessoas. Dois casos motivam isto:

1. **Faltou gente:** há 2 ou 3 pessoas na sala e elas querem jogar já, sem esperar o quarto.
2. **Treinar sozinho:** uma pessoa quer jogar contra/com bots para treinar ou passar o tempo.

E um terceiro, que nasce do mesmo mecanismo: **alguém caiu no meio da partida.** Hoje o ausente "joga" pela ação padrão (corre em toda votação, joga a primeira carta), o que estraga a mão para o parceiro. Com bots, um bot assume o lugar até a pessoa voltar.

O bot precisa ser **forte**: joga para ganhar, lê a mesa, usa e entende sinais, blefa de vez em quando. Um bot fraco torna o modo solo sem graça e faz o parceiro de quem caiu perder do mesmo jeito.

### Critérios de sucesso

- Sozinho na sala, aperto "Começar com bots" e em segundos estou jogando com um parceiro bot contra dois bots.
- Com 3 pessoas na sala, o dono aperta "Começar com bots", o quarto lugar vira bot e a partida começa.
- Na revanche, os bots continuam sentados; um colega que chega senta no lugar de um bot e a mesa segue.
- Um jogador cai no meio da mão; o bot assume na hora, o assento mostra "bot jogando por Fulano", e quando Fulano volta ele retoma o lugar na próxima decisão.
- Jogando com/contra o bot, a sensação é de um jogador competente (julgamento do usuário), **e** um teste automático mostra que o bot forte vence com folga um bot de referência simples (§5.5).

### Anti-metas (o que faria isto fracassar mesmo funcionando)

- **Bot que trapaceia:** o bot decide com informação que um humano naquele lugar não teria (mão dos outros, baralho, cartas cobertas alheias). O bot só enxerga o que `ViewFor(seat)` e os eventos visíveis para aquele assento mostram.
- **Bot que derruba o servidor:** a simulação do bot trava o ator da sala ou consome CPU sem limite quando há várias mesas com bots.
- **Brecha no ranking:** alguém escapar de uma derrota que contaria, ou partidas com bot inflarem o ranking/estatísticas.

## 2. Decisões do usuário

| Tema | Decisão |
|---|---|
| Para que serve | Os dois casos: completar mesa e jogar solo (1 humano + 3 bots). |
| Força | Forte. Sem níveis escolhíveis. |
| Ranking e estatísticas | Partida com bot **não conta** para ranking nem estatísticas. Fica no histórico, marcada "com bots". |
| Como os bots entram | Só um botão, "Começar com bots": preenche todas as cadeiras vazias e inicia. Sem "adicionar bot" por lugar. |
| Quem pode | Só o dono da sala (`host`). |
| Pessoa chega e há bot sentado | Pode tomar o lugar do bot **só na sala de espera**. Durante a partida, não. |
| Revanche | Os bots continuam sentados e são trocáveis por quem chegar. |
| Queda no meio da partida | O bot assume o lugar do ausente. |
| Quando assume | Quando fica ausente pela regra de hoje: desconectado **ou** 3 prazos estourados seguidos. |
| Efeito no ranking ao assumir | **Revisto em 2026-10-22 (decisão do usuário):** a partida só sai do ranking (para todos) se **terminar** com um bot controlando um humano da dupla **vencedora** (ou se começou com bot). Ausência com retorno antes do fim, ou bot no lugar de humano da dupla perdedora = partida segue valendo. W.O. continua contando. |
| Brecha (sair de propósito para apagar a derrota) | Fechada em duas camadas: (1) sair e voltar não anula nada, então quem abandona continua prejudicando a própria dupla, como hoje; (2) quem não volta pode levar W.O. dos adversários, que vale derrota no ranking como hoje. **Brecha da regra anterior ("qualquer humano com bot no fim anula"), fechada em 2026-10-22:** quem estava perdendo fechava a aba perto do fim e anulava a própria derrota antes de o W.O. ficar disponível. Agora a partida só deixa de contar se o bot joga por alguém da dupla vencedora; humano da dupla perdedora que abandona leva a derrota. |
| Volta de quem caiu | Retoma o lugar do bot na hora, na próxima decisão. |
| Sinais | O bot manda sinais ao parceiro e leva em conta os do parceiro. |
| Aparência | Pausa de "pensar" antes de jogar; nome brincalhão; selo de bot no assento; assento de quem caiu mostra "bot jogando por Fulano". |
| Como medir "forte" | Os dois: meta medida em teste automático (para não regredir) + julgamento do usuário jogando. |
| Entrega | Tudo numa spec, já com o bot forte. |
| Sala solo no lobby | Segue a visibilidade da sala: se pública, aparece (alguém pode entrar na revanche). |

## 3. Estado atual (o que já existe)

- **Engine** (`backend/internal/engine`): redutor puro `Apply(s, a)` sobre `GameState`; `Clone` faz cópia profunda; `HandState` tem campos exportados (`Hands`, `Deck`, `Played`, `Table`, `Rounds`, `Stake`, `Votes`…). Fases: `cut_choice`, `dealer_choice`, `burn_fours`, `eleven_decision`, `playing`, `awaiting_raise_response`, `hand_end`, `game_over`. Tipos de mão: `normal`, `eleven`, `iron` (ferro: sem carta coberta, mãos escondidas). Projeção de informação: `ViewFor(g, seat)`. Não há enumerador de ações legais exportado; `enginetest/random.go` gera candidatos e filtra por `Apply`. `DefaultAction` (`timeout.go`) é a ação de prazo estourado: fica com a carta 0, não queima, **corre** em votação, joga a primeira carta (coberta se não for ferro).
- **Sala** (`backend/internal/room`): ator por sala; `seatState` com `userID`, `name`, `ready`, `connected`, `timeouts`, `absentSince`…; `host`; partida começa em `maybeStart` quando há 4 ocupantes todos prontos; revanche volta para `waiting` com os mesmos ocupantes e `ready=false`. Ausência = `!connected || timeouts >= 3`; ausente tem prazo curto (`TIMER_ABSENT`, 5 s); W.O. oferecido aos adversários após carência/orçamento; todos ausentes → `abandoned` (`all_absent`). Sinais: `truco`, `boa`, `vamos`, `calma`, `desculpa`, `valeu`, só em `playing`, limite de 1 a cada 3 s, não persistidos.
- **Persistência** (`0002_partidas.sql`): `match_players.user_id NOT NULL REFERENCES users(id)`, `UNIQUE (match_id, user_id)`; `match_actions.source CHECK IN ('player','timeout','system')`; `match_player_stats`/`player_stats` gravados para os 4 assentos no `Finish`; ranking (`queries/ranking.sql`) junta `users`. A 4b adiciona `end_reason` em `matches` (migração 0004).
- **Protocolo:** `SeatInfo{userId, displayName, ready, connected, absent}`; `RoomStatePayload{…, host, status, seats, matchId, yourSeat}`.
- **Frontend:** `Assento.vue` (sala de espera: "Sentar aqui", coroa do dono, pronto, remover), `AssentoMesa.vue` (mesa: "Ausente"), `SalaEspera.vue`, `LinhaSala.vue` (lobby).

## 4. Regras de mesa

### 4.1 "Começar com bots"

- Nova mensagem cliente → servidor `start_with_bots` (sem payload). Só o `host`, só em `waiting`, só se houver ao menos 1 cadeira vazia **e** ao menos 1 humano sentado. Caso contrário → erro (`not_host`, `no_empty_seat`, ou `not_waiting`).
- Efeito: cada cadeira vazia recebe um bot (identidade em §6.1), já `ready=true`; o `host` é marcado `ready=true`. A partida começa pelo caminho normal de `maybeStart` — ou seja, **só começa quando todos os humanos sentados estão prontos**. *Decisão minha:* o botão fica desabilitado no frontend enquanto algum outro humano não estiver pronto ("Faltam N prontos"), para "preenche e já inicia" ser verdade quando ele é apertado; o servidor aceita mesmo assim e os bots ficam esperando os prontos.
- Os bots escolhidos para uma mesa têm nomes distintos entre si.

### 4.2 Bots na sala de espera (revanche)

- Ao fim da partida (`resetToWaiting`), bots continuam sentados com `ready=true`; humanos voltam a `ready=false` como hoje. Quando todos os humanos ficam prontos, a revanche começa sozinha (4 ocupantes prontos).
- **Tomar o lugar de um bot:** uma pessoa que entra na sala (ou que já está nela, trocando de cadeira) pode `take_seat` numa cadeira com bot; o bot sai. No lobby, cadeira com bot conta como **disponível**: sala com 4 ocupantes dos quais algum é bot não é "cheia". Entrar pelo lobby/código numa sala sem cadeira vazia, mas com bot, ocupa o lugar de um bot (*decisão minha:* o de menor número de assento).
- **Sem trava extra:** numa sala pública, qualquer pessoa pode tomar qualquer cadeira de bot, inclusive a do parceiro do dono — é o mesmo que uma cadeira vazia hoje. O dono pode remover quem não quiser. Quem quer jogar só com bots usa sala privada.
- **Tirar bots:** o `host` remove bot com o mesmo "remover" que já usa para humanos (sem modal de confirmação para bot). Apertar "Começar com bots" de novo recompleta.
- Bots **não mantêm a sala viva**: se o último humano sai, a sala é destruída como hoje. Atenção: hoje `removeSeat` destrói em `members()==0` e `members()` conta assentos ocupados — passa a contar só humanos (bots sentados são descartados junto).
- Bots nunca são `host`. Se o host sai, o novo host é o primeiro humano — hoje `host = firstOccupied()` (`seats.go`) escolheria qualquer ocupante; passa a ser o primeiro ocupante humano.
- O mesmo bot (mesma linha de `users`) pode estar em duas mesas ao mesmo tempo; é intencional. Numa mesma mesa, nunca repete.
- *Decisão minha:* sala de espera com bots e só humanos que ainda não marcaram pronto não tem prazo extra; vale a regra de hoje de desconexão em `waiting`.

### 4.3 Bot assume o ausente (durante `playing`)

- Quando um assento humano fica **ausente** (regra de hoje), ele passa a ser **controlado por bot**: nas decisões daquele assento, em vez de esperar o prazo curto e aplicar `DefaultAction`, o bot decide (com a pausa de "pensar", §5.4). O assento continua pertencendo ao humano (`match_players` não muda).
- **O ranking só se decide no fim** (§6.2): se a partida terminar por `game_over` com algum assento humano **da dupla vencedora** ainda controlado por bot, ela não conta, para todos. Humano da dupla perdedora com bot no lugar no fim não anula nada: a partida conta e quem abandonou leva a derrota (decisão do usuário de 2026-10-22, que fecha a brecha de apagar a própria derrota fechando a aba). Se o humano voltou antes do fim, conta normalmente. Ausência curta (um blip, uma queda de 30 s com volta) não anula nada.
- **W.O. continua valendo:** a contagem de carência/orçamento de ausência, `forfeit_available` e `claim_forfeit` seguem exatamente como hoje, mesmo com o bot jogando no lugar. W.O. pedido pelos adversários termina a partida como W.O. normal e **conta no ranking**, desde que nenhuma cadeira tenha sido preenchida por bot no início.
- **Conectado mas inativo** (`timeouts >= 3` com a aba aberta): *decisão minha:* nesse caso o bot não usa a pausa de 1–3 s; ele só age ao fim da janela curta de ausente (`TIMER_ABSENT`), dando ao humano a chance de agir primeiro. Desconectado de fato, vale a pausa normal.
- **Volta:** quando o humano deixa de estar ausente (reconecta, ou age na janela com a aba aberta), ele retoma o controle **a partir da próxima decisão** do assento. Uma decisão que o bot já está calculando é descartada se o humano agir antes (vale o que chegar primeiro ao ator; o resultado do bot é validado contra a janela corrente, §5.4).
- **Todos ausentes:** a regra de abandono considera só humanos. Se todos os humanos da mesa estiverem ausentes pelo orçamento de hoje, a partida é abandonada (`all_absent`) mesmo com bots jogando. Em partida com cadeiras de bot, "todos os humanos" pode ser um só.
- Bots nunca pedem W.O. e nunca são alvo de W.O.: assento de bot **não acumula timeout nem ausência** — nem quando a rede de segurança de §5.4 aplica `DefaultAction` por ele (aí não incrementa `timeouts`, só registra `warn`). Em partida com dupla adversária inteira de bots, não há quem peça W.O. contra o humano ausente; a partida segue com bot até o fim (já não conta mesmo).

### 4.4 Restore (após restart do servidor)

- Assentos de bot são restaurados a partir de `match_players` (§6.1) e voltam a jogar. Assento humano restaurado começa desconectado (regra de hoje), portanto ausente, portanto com bot até reconectar. A parte "começou com bot" vem do banco (`ranked=false` gravado na criação, §6.2); a parte "terminou com bot no controle" só é avaliada no `Finish`, então não há estado intermediário a restaurar.

## 5. O bot

### 5.1 Princípio: só joga com o que veria

O bot recebe, para seu assento: `ViewFor(g, seat)` atual e o histórico de eventos visíveis para aquele assento (cartas abertas, quem jogou coberto, pedidos e votos, sinais recebidos).

**De onde vem o histórico** (*decisão minha*): a sala mantém, por partida, o log de eventos já filtrados por assento (os mesmos que ela envia aos clientes) mais os sinais recebidos. O bot é sem estado entre decisões: cada cálculo recebe visão + log do assento. No restore, a sala reconstrói o log reprocessando as ações gravadas (como já faz para o estado); sinais não são persistidos, então se perdem no restart — aceitável. Nunca recebe `GameState` inteiro. Isso é verificado em teste (§8): o pacote do bot não importa nem recebe o estado completo, ou, se receber por conveniência de arquitetura, um teste prova que trocar as cartas escondidas não muda a decisão.

### 5.2 Decisões que cobre

Todas as que um jogador toma: carta do corte/escolha do dealer (`choose_card`), queimar quatros (`burn`/`pass_burn`), mão de 11 (`play`/`run`), pedir aumento (`request_raise`, incluindo nunca pedir quando isso perde a partida na mão de 11/ferro), responder aumento (`accept`/`run`/`raise`), jogar carta (`play_card` com slot e se vai coberta) e sinais.

### 5.3 Como decide (minha proposta técnica, decisão minha)

- **Enumerador de ações legais no engine:** `engine.LegalActions(g, seat) []Action`, exportado, testado contra `Apply` (toda ação listada é aceita; toda ação aceita num gerador exaustivo está listada). Serve ao bot e substitui o `candidates()` de `enginetest`.
- **Determinização + simulação:** a partir da visão do assento, sortear muitas vezes uma distribuição das cartas não vistas coerente com o que já aconteceu (cartas jogadas, queimadas, vira/manilha, e inferências de sinais e de comportamento — p.ex. quem pediu truco tende a ter carta forte), montar um `GameState` completo hipotético, e simular cada ação legal até o fim da mão com uma política de jogo rápida. Escolher a ação com melhor valor esperado **em pontos da partida** (não da mão), considerando o placar (mão de 11, perto de 12). A abordagem exata (Monte Carlo plano, ISMCTS, heurística de rollout) fica com o builder, dentro do orçamento de §5.4 e da meta de §5.5.
- **Blefe:** o bot pede/aumenta com mão fraca às vezes, com frequência ajustada para ser lucrativa na simulação, não um número fixo.
- **Helper de determinização:** como nada no engine valida conservação de cartas, o pacote do bot tem um helper que monta o estado hipotético e um teste de propriedade garante 40 cartas, sem duplicatas, mãos com o tamanho certo.
- **Bot de referência ("simples"):** existe também uma política simples por regras (joga carta legal razoável, pede truco só com manilha, aceita com mão média). Usa-se como rollout barato, como fallback quando o forte estoura o orçamento, e como adversário na meta de §5.5.

### 5.4 Execução na sala (anti-meta do servidor)

- O cálculo **não roda no goroutine do ator da sala.** Quando um assento controlado por bot tem decisão pendente, a sala dispara o cálculo fora do ator com uma cópia da visão; o resultado volta como mensagem ao ator, carimbada com a chave da janela de decisão (`windowKey`) e o `seq`/geração vigente. O ator descarta resultados de janela velha.
- **Orçamento por decisão:** tempo de parede limitado (*decisão minha:* até ~800 ms de cálculo; config `BOT_THINK_BUDGET_MS`). Estourou → usa a melhor ação até ali; sem nenhuma → bot simples.
- **Limite global:** semáforo de cálculos simultâneos no processo (config `BOT_MAX_CONCURRENT`, *decisão minha:* `GOMAXPROCS`). Na fila, o bot espera; se a espera passar do prazo da decisão, cai no bot simples.
- **Pausa de "pensar":** a ação do bot só é aplicada após um atraso aleatório (*decisão minha:* 1–3 s, menor em jogadas óbvias como última carta; config `BOT_DELAY_MIN_MS`/`BOT_DELAY_MAX_MS`). O cálculo corre durante a pausa. Testes injetam relógio e zeram a pausa.
- O prazo de decisão humano não se aplica ao bot; se por algum motivo o bot não agir até o prazo, a regra de timeout de hoje aplica `DefaultAction` (rede de segurança), sem contar timeout para o assento (§4.3).
- Ação do bot é gravada com `source='bot'` (§6.3).

### 5.5 Meta medida de força

- **Bot simples fixado** (para a meta não depender de um adversário móvel): é exatamente a política de §5.3 — joga a carta mais fraca que ganha a rodada, senão a mais fraca; nunca cobre; pede truco só com manilha; aceita com manilha ou duas cartas de rank A, 2 ou 3; na mão de 11 joga com manilha; nunca queima; nunca blefa; não usa sinais. Ele não é ajustado depois de escrito.
- Teste/benchmark determinístico (seeds fixas): **dupla de bots fortes contra dupla de bots simples, ≥ 75% das partidas em 400 partidas** (*decisão minha* nos números). É **meta de aceite**: se não fechar após iterar no bot forte, o builder para e reporta ao usuário com o número obtido, em vez de baixar a meta ou mexer no bot simples.
- Roda num alvo próprio (`make bot-bench`), fora do `go test` padrão. O CI roda uma versão reduzida que **bloqueia o merge**: forte vence o simples com margem estatística (p < 0,01) em um número pequeno de partidas. Como as seeds são fixas, o resultado é determinístico (não flaka): a margem é uma régua de projeto, não um teste estatístico por execução, e qualquer mudança no bot forte pode, legitimamente, reprovar o alvo — aí se corrige o bot, não a régua.
- Além disso, o usuário joga e julga; o design considera isso critério de aceite.

### 5.6 Sinais

Significado que o bot usa (*decisão minha*, convenção da mesa, documentada em `labels`/ajuda para os humanos saberem):

| Sinal | O bot manda quando | O bot entende do parceiro |
|---|---|---|
| `truco` | tem mão para pedir/aceitar truco | parceiro forte: mais disposto a pedir/aceitar |
| `vamos` | quer que o parceiro aceite/siga | aceita mais fácil |
| `calma` | mão fraca, não pede | evita pedir; corre mais fácil |
| `boa`, `valeu`, `desculpa` | reações sociais (ganhou rodada, parceiro jogou bem, errou) | ignora para decisão |

**Sinais dos adversários** (*decisão minha*): como os sinais são vistos por todos, o bot também os lê, mas como evidência fraca (podem ser blefe ou dirigidos ao parceiro): ajustam pouco a determinização das mãos adversárias. Os do parceiro pesam mais.

O bot respeita o mesmo limite de 1 sinal a cada 3 s e manda pouco (*decisão minha:* no máximo 1 sinal de intenção por mão). Sinais entram na determinização como evidência (ajustam a probabilidade das mãos do parceiro).

## 6. Persistência e ranking

### 6.1 Identidade do bot

- *Decisão minha:* bots são linhas em `users` com `kind='bot'` (nova coluna, `CHECK` em `human|bot`, default `human`), sem senha, sem e-mail, que não fazem login, não aparecem em presença, diretório, busca, ranking nem admin de usuários. Um conjunto fixo semeado pela migração (*decisão minha:* 8 bots, para nunca faltar nome distinto numa mesa), com nomes brincalhões no tom da mesa ("Seu Zé", "Dona Cida", "Tio Juca", "Vó Nena", "Compadre Tião", "Dona Lurdes", "Primo Dedé", "Tia Fia" — o builder pode trocar a lista). Assim `match_players` (FK e `UNIQUE (match_id, user_id)`) e o restore continuam funcionando sem mudança de esquema nessas tabelas.
- O nome exibido é o nome do bot; o selo "bot" vem do protocolo (`bot: true`), não de sufixo no nome.

### 6.2 Partida conta ou não

- Nova coluna `matches.ranked boolean NOT NULL DEFAULT true`.
- **Na criação:** `ranked=false` gravado já no `INSERT` se alguma cadeira é bot. Fica `false` para sempre.
- **No `Finish`** (partida que começou só com humanos): `ranked = (fim por W.O.) || (nenhum humano da dupla VENCEDORA controlado por bot no momento do fim)`. Humano da dupla perdedora com bot no fim não afeta o resultado. Gravado junto com o resultado.
- Partida abandonada/abortada não entra no ranking de qualquer forma; o valor de `ranked` nela é irrelevante.
- **Estatísticas:** `match_player_stats` e `player_stats` só são gravados/atualizados para partidas `ranked=true`. Partidas `ranked=false` ficam em `matches`/`match_players`/`match_actions` (histórico).
- **Partida que conta mas teve bot no meio** (humano voltou, ou W.O. com bot no controle): *decisão minha:* vitória/derrota e pontos contam normalmente. As ações `source='bot'` continuam no replay (o estado precisa delas); o que `stats.Derive` pula é a **atribuição ao assento** dos contadores individuais (`raises_called`, `punished`, `timeouts` e afins) quando a ação veio do bot. Contadores de equipe e de mão seguem contando. O humano não é premiado nem punido por jogadas que não fez.
- Migração: número 0005 (a 4b usa 0004); backfill trivial (todas as antigas `ranked=true`, todos os usuários `kind='human'`).
- **Esquema de `users` para bots:** *decisão minha:* `email` e `password_hash` passam a aceitar `NULL` só quando `kind='bot'` (`CHECK` condicional: humano continua exigindo os dois); `username` do bot segue a regex atual (ex.: `bot_seu_ze`). Nenhum código de login/admin pode assumir que `email` nunca é nulo sem filtrar `kind='human'`.

### 6.3 Ações

- `match_actions.source` passa a aceitar `'bot'` (migração ajusta o `CHECK`; `replay` e `stats.Derive` aceitam o novo valor). `stats.Derive` não conta ação de bot como timeout.

## 7. Protocolo e telas

### 7.1 Protocolo

- Cliente → servidor: `start_with_bots` (§4.1). Códigos de erro novos: `no_empty_seat`, `not_waiting` (`not_host` já existe), com texto em `labels.ts`.
- `SeatInfo` ganha `bot: boolean` (cadeira ocupada por bot) e `botPlaying: boolean` (humano ausente com bot jogando por ele).
- `RoomStatePayload` ganha `ranked: boolean | null` (`null` fora de `playing`): `false` se a partida começou com bot (nunca conta), senão `true`. Como o vencedor só é conhecido no fim, o campo **não** antecipa a anulação por `botPlaying`; o resultado definitivo vem em `match_ended.ranked`.
- `match_ended` ganha `ranked: boolean`.
- Resumo do lobby: cada assento indica bot; sala com bot não é "cheia".
- Tipos TypeScript em `frontend/src/protocol/messages.ts` acompanham.

### 7.2 Telas (seguem o design brief da 4a)

- **Sala de espera (`SalaEspera.vue`):** para o dono, botão "Começar com bots" quando há cadeira vazia; desabilitado com "Faltam N prontos" se algum outro humano não está pronto. Cadeira com bot (`Assento.vue`): nome do bot + selo de bot, sempre pronto; para quem não é o dono e está sem lugar ou quer trocar, "Sentar aqui" no lugar do bot; para o dono, "remover" (sem modal). Aviso discreto na sala: "Partidas com bots não contam no ranking."
- **Lobby (`LinhaSala.vue`):** cadeira com bot aparece com selo e é clicável como vaga.
- **Mesa (`AssentoMesa.vue`):** selo de bot nos assentos de bot. Assento de humano com `botPlaying`: nome do humano esmaecido + "bot jogando por Fulano". Quando um bot assume pela primeira vez, anúncio da mesa (tom brincalhão do brief) e aviso direto: "Fulano saiu; um bot assumiu. Se a dupla de Fulano vencer com o bot no lugar, a partida não conta no ranking — vocês ainda podem pedir W.O." para os adversários; o parceiro vê "Se vocês vencerem com o bot no lugar de Fulano, a partida não conta no ranking." (em partida que começou com bot, só "Fulano saiu; um bot assumiu.") O banner de W.O. de hoje continua.
- **Fim de partida (`FimPartida.vue`):** se `ranked=false`, "Partida com bots — não conta no ranking." (vale também para quem terminou com bot no lugar de alguém).
- **Sinais:** balão do sinal do bot aparece como de qualquer jogador. *Decisão minha:* um "?" discreto perto da barra de sinais abre a legenda da tabela de §5.6.

## 8. Testes

- **Engine:** `LegalActions` vs `Apply` (propriedade, todas as fases e tipos de mão).
- **Bot:** determinização preserva cartas (propriedade); bot não depende de informação escondida (trocar mãos alheias consistentes com a visão não muda a distribuição de decisões — teste com seed fixa do bot); nunca pede aumento suicida; sempre devolve ação legal; respeita orçamento; meta de força (§5.5).
- **Sala** (harness de hoje, relógio injetado): `start_with_bots` (permissões, erros, preenche e começa quando humanos prontos); revanche mantém bots; `take_seat` em bot; remover bot; sala destruída quando sai o último humano; ausente → bot assume → humano volta e retoma na próxima decisão → partida termina `ranked=true`; ausente sem volta até o `game_over` → `ranked=false` só se ele é da dupla vencedora (dupla perdedora → `true`; humanos das duas duplas com bot → `false`); `ranked` ao vivo do `room_state` só `false` se começou com bot; resultado de bot de janela velha descartado; W.O. com bot no controle → `ranked=true` (partida só humana) / `false` (com cadeira de bot); assento de bot não acumula timeout/ausência; sala destruída e host eleito contando só humanos; todos humanos ausentes → abandono; restore com bots; limite global de cálculos.
- **Persistência:** estatísticas não gravadas para `ranked=false`; `source='bot'`; bots fora de ranking/diretório/presença.
- **Frontend:** botão "Começar com bots" (visível só ao dono, desabilitado com faltam prontos), selo de bot, "Sentar aqui" em bot, "bot jogando por Fulano", aviso de fim de partida sem ranking.
- **Ponta a ponta:** um cliente de teste sozinho cria sala, `start_with_bots`, partida vai até `game_over` com 3 bots e termina `finished`, `ranked=false`.

## 9. Integração com a Spec 4b (já mergeada quando esta começar)

A 4b mexe em ranking, histórico, perfil, presença, diretório e admin de usuários. Esta spec edita essas partes para:

- Ranking (todos os períodos e métricas), perfil (posição e estatísticas) e "parceiros frequentes": só partidas `ranked=true` e só usuários `kind='human'`.
- Histórico: inclui partidas `ranked=false`, com marca "com bots"; bots aparecem pelo nome com selo e **sem link de perfil**.
- Perfil direto de bot (`/jogadores/bot_seu_ze`, `GET /api/users/{username}`): **404**, mesma resposta de usuário desabilitado — mas **também para admin** (hoje `stats/reader.go` `user()` deixa admin ver desabilitado; para bot não há exceção). O mesmo vale para `?user=` do histórico.
- Histórico (`ListUserMatches`/`ListPlayersForMatches`, `HistoryItem`/`MatchPlayer` em `stats/reader.go`): ganham `ranked` na partida e `bot` no jogador, para `LinhaPartida.vue` marcar "com bots" e o selo.
- Filtros explícitos (não depender de `email_verified_at IS NOT NULL` excluir bots por acaso): ranking e posições (`ranking.sql`), `ListPartners`, `SearchDirectory`, `SearchUsers` (admin), `ActiveUserByUsername` (convites) e login (`auth/flows.go`) filtram `kind='human'`; ranking e parceiros também `ranked`.
- **Índice do lobby é um-usuário-uma-sala** (`lobby.go`, inclusive no restore): bots **não entram nesse índice** — o mesmo bot pode estar em várias mesas. A sala sabe que o assento é bot pelo `seatState`, não pelo índice.
- **Restore:** `NewRestored` hoje põe todos os assentos `connected=false`; assento de bot volta como bot ativo (nunca ausente), e o cálculo de "todos ausentes" (`allAbsentAt`) considera só assentos humanos.
- `dbgen`: `Email`/`PasswordHash` viram anuláveis no modelo gerado (regen do sqlc); código que lê usuário humano trata o tipo novo.
- Presença, diretório, convites e admin de usuários: ignoram `kind='bot'`.
- Migrações: 4b = 0004 (`end_reason`), esta = 0005 (se a 4b tiver criado mais migrações, esta usa o próximo número livre).
- Login (e qualquer fluxo de autenticação da 4b) recusa `kind='bot'`.

## 10. Configuração nova

| Variável | Default | Faixa |
|---|---|---|
| `BOT_THINK_BUDGET_MS` | 800 | 50..5000 |
| `BOT_MAX_CONCURRENT` | `GOMAXPROCS` | 1..64 |
| `BOT_DELAY_MIN_MS` | 1000 | 0..10000 |
| `BOT_DELAY_MAX_MS` | 3000 | ≥ min, ..10000 |

## 11. Deixado ao builder (com o critério que importa)

- Algoritmo exato de busca e de inferência, desde que cumpra §5.1 (sem trapaça), §5.4 (orçamento, fora do ator) e §5.5 (meta).
- Estrutura de pacotes (sugestão: `backend/internal/bot`), formato da mensagem interna do resultado do bot ao ator.
- Lista final de nomes dos bots e o visual exato do selo (dentro dos tokens do brief).
- Texto final dos anúncios da mesa.
