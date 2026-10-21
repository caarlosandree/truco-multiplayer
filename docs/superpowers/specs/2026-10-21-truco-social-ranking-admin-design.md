# Truco Multiplayer — Spec 4b: Social, ranking, histórico, perfil e admin

- **Data:** 2026-10-21
- **Status:** rascunho para revisão do usuário
- **Depende de:** Specs 1–3 (engine, contas, salas/WebSocket) e Spec 4a (frontend jogável, design brief "Lousa e tentos"), implementadas e em `staging`.
- **Escopo desta spec:** presença ("quem está online"), convites para a sala (no app, por e-mail e por link), ranking por período e métrica, histórico de partidas, perfil público e painel admin. Inclui as mudanças de backend necessárias e as telas.
- **Fora de escopo:** replay de partida (adiado), chat livre, espectadores, matchmaking automático, internacionalização, PWA/push com service worker, modo invisível.

## 1. Intenção

Colegas de trabalho jogam truco entre si. Depois de uma rodada, o que eles querem é: **ver quem está ganhando**, **saber quem está online agora** e **chamar os colegas para a mesa sem sair do app**. A 4a entregou o jogo; a 4b entrega a parte social que faz as pessoas voltarem: o ranking da semana para a resenha da sexta, a lista de quem está por aí e um convite que chega na hora.

### Critérios de sucesso

- De dentro da minha sala, convido um colega que está online; ele vê o aviso, clica "Entrar" e senta, sem eu precisar mandar link por fora.
- Um colega offline recebe e-mail com o convite e, ao clicar, entra na sala (passando pelo login se preciso), ou vê "convite expirado" se a sala encheu, começou ou fechou.
- No lobby vejo quem está online e o que cada um está fazendo.
- O ranking mostra vitórias e aproveitamento, no geral, no mês e na semana, com a minha posição destacada.
- Qualquer colega pode abrir o perfil de outro e ver posição, estatísticas, partidas recentes e parceiros frequentes.
- O admin gerencia usuários, domínios, partidas travadas e vê a auditoria — e **ninguém vira admin sem o dono permitir**.

### Anti-meta (o que faria isto fracassar mesmo funcionando)

- **Admin inseguro:** alguém ganhar poderes de admin sem autorização explícita do dono. Esta é a restrição mais importante da spec (§7).

## 2. Decisões do usuário

| Tema | Decisão |
|---|---|
| Escopo | Tudo numa spec: online, convites, ranking, histórico, perfil e admin. Replay fica de fora. |
| Convite — canais | Aviso no app (online), e-mail (offline) e o "Copiar link" que já existe. |
| Convite — e-mail | Só para quem está offline no momento do convite. |
| Convite — resposta | "Entrar" / "Agora não"; quem convidou vê na sala: aceitou, recusou, aguardando, e-mail enviado. |
| Convite — quem pode | Qualquer jogador sentado na sala (pública ou privada), enquanto houver lugar livre. |
| Convite — validade | Enquanto a sala tiver lugar; expira quando enche, a partida começa ou a sala fecha. Sem prazo por relógio. |
| Convite — ocupados | Pode convidar quem está em outra sala ou jogando; o convidado decide. Aceitar o tira de uma sala de espera, nunca de uma partida em andamento. |
| Convite — onde fica o botão | Os dois: "Convidar" no topo da sala de espera e "Convidar" em cada lugar vazio (o convidado entra naquele lugar). |
| Convite — aba em segundo plano | Som de convite, título da aba piscando e notificação do navegador. |
| Convite — limites | Sem limite próprio (além do limite geral do WebSocket). |
| Online | Lista com status (no lobby / esperando numa sala / jogando), numa coluna do lobby; todos os logados aparecem (sem modo invisível). |
| Ranking | Tabela sóbria (mockup A), abas Vitórias e Aproveitamento, períodos Geral, Mês e Semana. |
| Período | Fuso de Brasília (`America/Sao_Paulo`); semana começa segunda 00:00; mês no dia 1 00:00. |
| Visibilidade | Histórico e perfil abertos entre colegas logados (como o backend já faz). |
| Perfil | Posição no ranking (3 períodos), estatísticas, partidas recentes, parceiros frequentes e botão "Convidar". |
| Admin | Usuários (buscar, desativar/reativar, forçar troca de senha), domínios de e-mail, partidas em andamento (ver e abortar) e auditoria. |
| Admin — concessão | "Do jeito mais seguro possível": ninguém ganha admin sem o dono permitir (§7). |
| Admin × admin | Ações de admin (desativar, reativar, forçar troca de senha) **não** valem sobre usuários admin; só o dono, pelo CLI, mexe em admins. |
| Admins atuais | Ficam como estão; a partir da 4b, admin só pelo CLI. |
| Link de convite | Vale **só para o convidado**; outra conta que abrir vê "Este convite é para outra pessoa". |
| Reconvite | Depois de "Agora não" ou de expirado, pode convidar a mesma pessoa para a mesma sala de novo (novo aviso no app), mas o **e-mail** para a mesma pessoa e sala sai uma vez só. |

## 3. Estado atual (o que já existe)

Levantado no código em 2026-10-21:

- **Ranking:** `GET /api/ranking?limit=` lê a tabela acumulada `player_stats`; só período geral, ordem fixa por vitórias; `winRate` calculado em Go; filtro `RANKING_MIN_MATCHES` (padrão 5). `match_player_stats` (por partida, com `won`) + `matches.finished_at` permitem agregar por período.
- **Histórico:** `GET /api/matches?user=&cursor=` (keyset, 20 por página) sem jogadores nem motivo do fim; `GET /api/matches/{id}` traz jogadores, placar, `abandonReason`. Não há campo que distinga vitória normal de W.O. (`status='finished'` nos dois).
- **Perfil:** `GET /api/users/{username}` traz `Totals` detalhado; sem partidas recentes, parceiros ou posição.
- **Presença:** o `Lobby` mantém `clients` (usuário → conexão) e `index` (usuário → sala) sob mutex, mas nada é exposto.
- **Convites:** inexistentes. Toda sala (pública ou privada) recebe um código de 6 caracteres em `createRoomLocked`, registrado em `l.codes`; ele só é exibido nas privadas, e `join_room{code}` não checa visibilidade. `Lobby.Send(userID, msg)` já envia a um usuário específico. `kick` não impede voltar pelo código (nem será impedido de ser reconvidado — coerente com o comportamento atual).
- **E-mail:** `mail` com drivers `resend`/`log`, templates em `mail/templates.go`; envio assíncrono via `sendLater`.
- **Admin:** rotas `/api/admin/*` completas (usuários, domínios, abortar partida, auditoria) com `requireAdmin`; `PUT /api/admin/users/{id}/admin` permite a um admin promover outro; primeiro admin pelo CLI `server promote-admin`. Não há listagem de partidas em andamento.
- **Frontend 4a:** `AppShell`, kit de UI, tokens e design brief (`docs/superpowers/specs/2026-10-20-truco-frontend-design-brief.md`), stores Pinia, `ws/client.ts` com roteamento exaustivo de `ServerMessage`, toasts, `Anunciador`, `tabTitle.ts`, `sfx`.

## 4. Backend

### 4.1 Presença

- O `Lobby` passa a publicar a lista de pessoas online com status, derivada de `clients` + `index` + status da sala:
  - `lobby` — conectado, fora de sala;
  - `waiting` — numa sala em espera;
  - `playing` — numa sala em jogo.
- Cada item: `userId`, `username`, `displayName`, `status`, e, quando em sala, `roomName` **só se a sala for pública** (sala privada aparece como "sala privada", sem nome nem código).
- **Entrega:** um snapshot na conexão e atualizações incrementais por WebSocket para **todos** os conectados (inclusive quem está em sala, porque o convite é feito de dentro da sala). Novos `ServerMessage` (nomes indicativos): `presence_snapshot`, `presence_updated`, `presence_removed`.
- Debounce/coalescência de atualizações a cargo do builder, contanto que a lista não fique visivelmente atrasada (alvo: < 1 s).
- Usuário desabilitado não aparece. Ao desativar um usuário, o serviço `admin` avisa o `Lobby` (porta nova, no mesmo padrão do `MatchAborter`) para derrubar a conexão WS dele na hora, tirando-o da presença e cancelando convites pendentes para ele — sem esperar a próxima revalidação de sessão do `ws/client.go`.

### 4.2 Convites

- **Modelo:** um convite liga `sala`, `quem convidou`, `convidado` e, opcionalmente, um `lugar` (quando feito pelo lugar vazio). No máximo **um convite pendente por convidado por sala**: convidar de novo enquanto há um pendente não gera novo aviso nem novo e-mail. Depois de `declined` ou `expired`, um novo convite para a mesma pessoa e sala é permitido e gera novo aviso no app; o **e-mail** para a mesma pessoa e sala é enviado no máximo uma vez enquanto a sala existir (decisão do usuário).
- **Estados:** `pending` → `accepted` | `declined` | `expired`. Expira quando a sala enche, a partida começa ou a sala fecha; **não** expira se quem convidou sair (o convite é da sala — decisão do controlador, §10). Expirar é avaliado pelo servidor; o cliente só exibe.
- **Fluxo no app (convidado online):**
  1. Quem está sentado envia `invite{userId, seat?}` (novo `ClientMessage`).
  2. Servidor valida: remetente sentado, sala com lugar, convidado existe e não está desabilitado, convidado não é o próprio remetente nem já está **nesta** sala (`already_in_room`), lugar (se dado) vazio.
  3. Convidado online recebe `invite_received{inviteId, room{id, name, visibility}, from{displayName}, seat?, freeSeats}`.
  4. Convidado responde `invite_respond{inviteId, accept: bool}`. Aceitar faz o equivalente a sair da sala de espera atual (se houver) e entrar na sala convidante (no lugar indicado, se ainda livre; senão, no primeiro livre). Se o convidado estiver **jogando**, aceitar é recusado pelo servidor com erro (o cliente nem oferece o botão nesse caso — mostra "Você está numa partida").
  5. Os membros da sala recebem a lista de convites e seus estados no `room_state` (ou mensagem dedicada), para a lista "Convites" da sala de espera.
  6. Quando o convite expira, o convidado recebe `invite_expired{inviteId}` e o aviso some.
- **Fluxo por e-mail (convidado offline no momento do convite):**
  - Servidor envia e-mail com template novo `RoomInvite` (nome de quem convidou, nome da sala, link).
  - O link usa um **token de convite** opaco, `/entrar-convite/{token}` — não o código da sala, porque o token vincula convite → convidado (é o que permite recusar outra conta com `invite_not_yours`), enquanto o código abre a sala para qualquer um. O token identifica o convite e só é válido enquanto o convite estiver `pending`.
  - O aceite por token usa uma mensagem própria (ex.: `invite_accept_token{token}`, ou `invite_respond` aceitando `token` em vez de `inviteId`), tratada no nível do `Lobby`, que resolve token → convite, confere o convidado e segue o mesmo caminho do aceite no app.
  - Abrir o link: guarda de rotas exige login (com `volta` preservado, como o convite por código da 4a); logado, o frontend envia a aceitação e segue para `/sala`; convite expirado mostra a tela "Convite expirado" com botão para o lobby.
  - O token vale **só para o convidado**: o servidor confere que o usuário logado é o convidado; outra conta recebe erro `invite_not_yours` e a tela "Este convite é para outra pessoa". E-mail encaminhado não dá acesso à sala.
  - Se o convidado ficar online enquanto o convite está pendente, ele também recebe o `invite_received` no app.
- **"Copiar link"** continua como está (código da sala privada).
- **Persistência:** convites vivem em memória no `Lobby`/sala, como as salas de espera (que não sobrevivem a restart). O token de e-mail precisa resolver o convite enquanto o servidor estiver de pé; após restart, o link cai em "Convite expirado". (Se o builder preferir persistir, é aceitável, mas não exigido.)
- **Erros novos** em `protocol/errors.go` (indicativos): `invite_not_found`, `invite_expired`, `invite_not_yours`, `invitee_playing`, `cannot_invite_self`, `user_not_found`.
- Todas as mensagens novas seguem o fluxo da 4a: constantes/payloads em `protocol/client.go`/`server.go`, registro em `registry.go`, `case` próprio em `Lobby.Dispatch` (tipos que não são de sala não podem cair em `not_in_room`), `make gen-types`, testes de contrato/goldens atualizados.

### 4.3 Diretório de pessoas (para convidar quem está offline)

- Novo `GET /api/users?q=` (autenticado): busca por username/nome de exibição, paginado (envelope `{items, nextCursor?}` do pacote `pagination`), devolvendo só `username`, `displayName` — **sem e-mail**. Exclui desabilitados.

### 4.4 Ranking por período e métrica

- `GET /api/ranking?period=all|month|week&metric=wins|winrate&limit=`:
  - `month`/`week` agregam `match_player_stats` ⨝ `matches.finished_at` dentro da janela no fuso `America/Sao_Paulo` (semana de segunda 00:00; mês do dia 1 00:00). `all` pode ler `player_stats`, mas com query própria por métrica (a query atual aplica o mínimo a tudo).
  - `metric=wins`: lista **todos** com ≥ 1 partida no período (sem o mínimo, senão a aba ficaria vazia toda segunda de manhã); ordem por vitórias, depois aproveitamento, partidas, username.
  - `metric=winrate`: só quem tem `≥ RANKING_MIN_MATCHES` partidas **no período**; ordem por aproveitamento, depois vitórias, partidas, username.
  - A resposta inclui `me` — a linha do usuário logado com sua posição (ou `null` se não ranqueado no período), para destacá-lo mesmo fora do top.
  - Só partidas `finished` contam (abandonadas não).
- Índices necessários para as janelas ficam a cargo do builder (há `matches_closed_idx` em `finished_at`).

### 4.5 Histórico enriquecido

- `GET /api/matches` passa a trazer, por item: jogadores (`seat`, `username`, `displayName`), minha dupla, resultado do ponto de vista do usuário consultado, placar e **motivo do fim**: `game_over`, `forfeit` (W.O.), `aborted` (admin) ou `abandoned`.
- Hoje W.O. e fim normal são indistinguíveis no banco, e aborto de admin e abandono são ambos `status='abandoned'`, separados só pelo texto livre `abandon_reason` ("abortada pelo administrador", "all_absent", "diverged"…). **Decisão:** nova coluna `end_reason` em `matches` (migração 0004, `CHECK` em `game_over|forfeit|aborted|abandoned`), preenchida no `FinishMatch`/`Abandon`/abortar. Backfill das partidas antigas: W.O. pelo `match_actions` (`source='system'`, ação `sys_forfeit`); aborto de admin pelo `audit_log` (`match.abort`), nunca pelo texto livre; demais abandonadas → `abandoned`; demais finalizadas → `game_over`.

### 4.6 Perfil público enriquecido

- `GET /api/users/{username}` acrescenta:
  - `positions`: posição no ranking nos três períodos e nas duas métricas (ou `null` quando não ranqueado);
  - `recentMatches`: as 5 mais recentes no mesmo formato do histórico (§4.5);
  - `partners`: até 5 parceiros mais frequentes, com partidas juntos e aproveitamento juntos.
- Usuário desabilitado: perfil devolve 404 para não-admin.

### 4.7 Admin

- **Partidas em andamento:** novo `GET /api/admin/matches?status=in_progress&cursor=` com jogadores, placar atual se disponível, início; o abortar continua em `POST /api/admin/matches/{id}/abort`.
- **Concessão de admin — restrição dura (§7):**
  - Remover (ou desativar com 404) a rota `PUT /api/admin/users/{id}/admin`. **Nenhuma** rota HTTP/WS pode conceder ou retirar admin.
  - Admin só é concedido/retirado pelo CLI no servidor (`server promote-admin` existente; acrescentar `server demote-admin` com a mesma proteção de "não remover o último admin").
  - O CLI registra a ação na auditoria (ator: `cli`/sistema).
- **Admin não age sobre admin:** desativar, reativar e forçar troca de senha recusam alvo com `is_admin` (403, código próprio, ex.: `target_is_admin`), no serviço `admin` e não só no handler. Admins existentes continuam como estão; mudanças neles só pelo CLI.
- Usuários, domínios e auditoria continuam com as rotas existentes.

### 4.8 E-mail

- Novo template `RoomInvite(to, inviterName, roomName, link)` em `mail/templates.go`, texto e HTML em pt-BR, com teste em `mail_test.go`. Envio assíncrono como os demais.

## 5. Frontend

Fonte da verdade visual: o design brief da 4a. Tudo reaproveita `AppShell`, kit de UI, tokens e o tom (botões e erros diretos; anúncios com o tom da mesa).

### 5.1 Rotas novas

| Rota | Tela |
|---|---|
| `/ranking` | Ranking |
| `/jogadores/:username` | Perfil público (o meu também) |
| `/jogadores/:username/partidas` | Histórico completo, paginado |
| `/entrar-convite/:token` | Aceitar convite por e-mail (exige login; preserva `volta`). Nome escolhido para não confundir com a rota nomeada `convite` da 4a (`/sala/:codigo`). |
| `/admin`, `/admin/usuarios`, `/admin/dominios`, `/admin/partidas`, `/admin/auditoria` | Painel admin |

Cabeçalho ganha "Ranking"; o link "Admin" só aparece para `isAdmin` (a proteção real é no servidor). `/perfil` (preferências e conta, da 4a) continua; o nome no cabeçalho leva ao meu perfil público, e o perfil público tem "Editar conta" quando sou eu.

### 5.2 Lobby com "Online agora"

- Desktop: coluna à direita das salas (mockup aprovado): "Online agora · N", cada pessoa com ponto de status, nome e status em texto ("no lobby", "esperando · Mesa do RH", "esperando · sala privada", "jogando · Truco da TI"). Cor nunca é o único sinal.
- Clicar no nome abre o perfil. Quando eu estou numa sala com lugar livre, cada pessoa ganha "Convidar".
- Celular (< 768 px): a coluna vira uma aba ou gaveta "Online (N)" no lobby, sem competir com a lista de salas.

### 5.3 Convites

- **Sala de espera:** botão "Convidar" no topo e "Convidar" em cada lugar vazio (ao lado de "Sentar"). Ambos abrem o seletor de pessoas: seção "Online" (da presença) e seção "Outros colegas" (busca no diretório, §4.3); quem está jogando aparece marcado ("jogando — pode recusar") mas é convidável. Abaixo da mesa, a lista "Convites" com o estado de cada um: aceitou, aguardando…, agora não, e-mail enviado.
- **Convidado:** aviso persistente (não o toast de 5 s) com "Fulano te chamou pra mesa!", nome da sala, lugares livres, e "Entrar" / "Agora não". Se estou numa sala de espera, o aviso diz que aceitar me tira dela; se estou jogando, não há "Entrar" — só "Agora não" e o texto "Você está numa partida". Vários convites empilham (no máximo 3 visíveis, o mais novo em cima).
- **Aba em segundo plano:** som `invite` (novo, CC0, dentro do orçamento de áudio da 4a), título da aba alternando "Convite: <sala>" (reaproveitar `tabTitle.ts`, sem brigar com o "Sua vez!") e notificação do navegador (`Notification`) com clique levando ao aviso. A permissão é pedida uma única vez, num momento contextual (ex.: primeira vez que convido ou sou convidado), com toggle em Perfil › Preferências; recusa é respeitada sem insistir.
- **Acessibilidade:** aviso anunciado pelo `Anunciador` (uma única região viva, como na 4a); "Entrar" recebe foco só se o usuário não estiver digitando nem com modal aberto; Esc = "Agora não".
- **`/entrar-convite/:token`:** estados "Entrando na sala…", "Convite expirado" (sala cheia/começou/fechou) e "Você está numa partida".

### 5.4 Ranking

- Tabela sóbria (mockup A): abas "Vitórias" e "Aproveitamento"; seletor de período "Geral / Mês / Semana"; colunas #, jogador, V, D, %; minha linha destacada; se eu estiver fora do top, minha linha aparece no fim separada. Em "Aproveitamento", nota "só quem jogou ≥ N partidas no período". Estados vazio ("Ninguém jogou nesta semana ainda"), carregando e erro. Clicar num jogador abre o perfil.

### 5.5 Perfil público e histórico

- Perfil (mockup aprovado): nome, @username, status online; posição nos três períodos; estatísticas principais (partidas, % vitórias, truco aceito/corrido, mãos de 11 vencidas e outras de `Totals` à escolha do builder, sem poluir); partidas recentes (resultado, placar ou "W.O.", parceiro, adversários, quando); parceiros frequentes; "Convidar pra minha sala" quando faz sentido.
- Histórico: lista paginada (keyset, "Carregar mais"), mesmo formato das recentes, com filtro opcional por resultado a critério do builder.

### 5.6 Painel admin

- Ferramenta interna: sóbria, densa, consistente com o cromo; sem a identidade "de mesa".
- Usuários: busca, tabela, ações desativar/reativar e forçar troca de senha com modal de confirmação. **Nenhum controle de admin** (sem promover/rebaixar), e um texto explicando que admin é concedido só pelo servidor. Linhas de usuários admin aparecem marcadas "admin" e **sem** ações.
- Domínios: listar, adicionar, remover (com confirmação).
- Partidas em andamento: lista com jogadores e início; "Abortar" com confirmação e motivo.
- Auditoria: lista paginada (ator, ação, alvo, quando, detalhes).

## 6. Restrições (herdadas da 4a, vinculantes)

- CSP do servidor sem recursos externos; fontes, sons e imagens locais.
- `frontend/src/protocol/*` só via `make gen-types`.
- pnpm; TS strict com `noUncheckedIndexedAccess`; sem `any` fora de testes; TypeScript fixado em ~6.0.3.
- Componentes e views não importam `@/ws` nem `@/api`; passam por stores.
- Nenhuma regra de jogo no cliente.
- `localStorage` dentro de `try/catch`.
- Textos, comentários e commits em pt-BR com acentos; commits convencionais sem linhas de atribuição.
- Banco de dev: o banco `truco` no Postgres do projeto `infra` (:5432); não mexer em containers alheios; e2e no compose isolado `truco-e2e`.
- Nada de push, merge ou PR sem confirmação do usuário.

## 7. Segurança do admin (anti-meta)

- Conceder e retirar admin **só** pelo CLI no servidor. Nenhuma rota HTTP/WS, nenhuma tela.
- Todas as rotas `/api/admin/*` continuam exigindo `requireAdmin` no servidor; a UI esconder links é cosmético.
- Toda ação admin (e do CLI) vai para a auditoria.
- Nenhuma ação admin (desativar, reativar, forçar troca de senha) vale sobre outro admin.
- Testes: (a) não existe rota que altere `is_admin` (teste que varre o roteador e/ou chamada ao antigo `PUT` retorna 404/405); (b) usuário comum recebe 403 em todas as rotas `/api/admin/*`; (c) CLI `demote-admin` não remove o último admin ativo; (d) admin recebe 403 ao desativar/reativar/forçar troca de senha de outro admin; (e) token de convite aberto por conta diferente do convidado é recusado (`invite_not_yours`).

## 8. Testes

- **Go:** presença (status por transição: conectar, entrar em sala, começar partida, sair, desconectar; sala privada sem nome; usuário desativado sai da presença na hora e perde convites pendentes), convites (validações incl. `already_in_room` e convidar a si mesmo, um pendente por convidado/sala, reconvite após recusa sem novo e-mail, expirar ao encher/começar/fechar, aceitar tirando da sala de espera, recusa de quem está jogando, e-mail só para offline, token), ranking por período com fronteiras de semana/mês em `America/Sao_Paulo` (inclusive virada de horário e partidas na borda), métrica winrate com mínimo no período e wins sem mínimo (≥ 1 partida), `me` fora do top, histórico com motivo do fim (W.O. e aborto de admin, inclusive backfill de partidas antigas), perfil com posições/recentes/parceiros, diretório sem e-mail, admin (§7), template de e-mail.
- **Vitest:** stores de presença e convites (roteamento exaustivo das mensagens novas), seletor de pessoas, aviso de convite (foco, Esc, empilhamento, jogando), ranking (abas, período, `me`), perfil, histórico paginado, painel admin (sem controle de admin), notificação/som/título com aba oculta.
- **E2E (Playwright, compose isolado):** dois navegadores — A cria sala e convida B pelo "Online"; B aceita e senta; um terceiro convite a alguém offline gera e-mail (link lido do log, `MAIL_DRIVER=log`) que leva à sala; ranking e perfil carregam após uma partida.
- Ciclo de revisão visual da 4a (screenshots 1440/1366/375, claro/escuro; `impeccable` critique/polish; `fixing-accessibility`/`ui-a11y`) para cada tela nova.

## 9. Critérios de aceite

- [ ] Convite no app: enviar, receber, aceitar/recusar, estados visíveis na sala, expirar; pelo lugar vazio senta no lugar.
- [ ] Convite por e-mail para offline, com link que entra na sala ou mostra "Convite expirado".
- [ ] Lista "Online agora" com status no lobby (desktop e celular).
- [ ] Aviso de convite com som, título piscando e notificação do navegador com a aba em segundo plano.
- [ ] Ranking com Vitórias/Aproveitamento × Geral/Mês/Semana e minha posição.
- [ ] Histórico com jogadores e motivo do fim (W.O. correto); perfil com posições, recentes e parceiros.
- [ ] Painel admin com usuários, domínios, partidas em andamento e auditoria; **nenhuma forma de conceder admin fora do CLI**, com testes.
- [ ] `go test ./...`, `pnpm typecheck && lint && test && build` e `make e2e` verdes; CI verde; README atualizado.

## 10. Deixado ao builder (com o critério que importa)

- Nomes exatos das mensagens WS, campos e códigos de erro (mantendo o padrão do protocolo existente).
- Onde guardar o estado dos convites (dentro do ator da sala ou no `Lobby`), desde que não haja corrida entre "sala encheu" e "convite aceito" (o servidor é a autoridade; aceitar sem lugar vira `invite_expired`).
- Se o convite expira quando **quem convidou** sai da sala: decisão do controlador — **não expira** (o convite é da sala, não da pessoa), enquanto a sala existir com lugar.
- Detalhes da migração e do backfill de `end_reason` (§4.5).
- Debounce da presença, índices SQL para as janelas de ranking, quais estatísticas de `Totals` aparecem no perfil.
- Layout exato do seletor de pessoas e do aviso de convite no celular, seguindo o design brief.
