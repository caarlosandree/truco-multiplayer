# Truco — design brief da direção visual "Lousa e tentos"

- **Data:** 2026-10-20
- **Status:** direção C escolhida pelo usuário na Fase D0 (prévia `.superpowers/sdd/2026-10-15-truco-frontend-jogavel/d0-direcoes.html`)
- **Escopo:** fonte da verdade visual das Tarefas 13–19 da Spec 4a. Complementa a spec `2026-10-15-truco-frontend-jogavel-design.md` (§6.2, §7, §8.3) sem mudar regras de jogo.
- **Implementação:** `frontend/src/styles.css` (tokens) e `frontend/src/assets/fonts/` (fontes e licenças).

## 1. Ideia e princípios

O placar é a lousa do bar: tentos riscados a giz em grupos de cinco, ao lado do feltro verde. Quando alguém pede truco, a palavra se escreve a giz por cima da mesa.

1. **A ousadia mora em dois lugares:** a lousa do placar (tentos, número, valor da mão) e o momento do pedido ("Truco!", "Seis!"…). Só ali entram a Kalam, o laranja e o movimento de giz.
2. **O resto é quieto:** cromo liso, Atkinson Hyperlegible Next, sem gradiente decorativo, sem rótulo em caixa-alta acima de título, sem grade de cartões iguais. Hierarquia por tamanho, peso e espaço.
3. **Feltro e lousa não mudam com o tema.** A mesa é sempre escura; o tema claro/escuro troca só o cromo (lobby, contas, sala de espera, cabeçalho, modais, toasts).
4. **Uma ação principal por momento.** Laranja significa aposta e nada mais; vermelho significa perigo e nada mais.
5. **Cor nunca é o único sinal:** dupla tem rótulo e posição; vez tem contorno tracejado; prazo tem número.
6. **Tom:** brincalhão só nos anúncios da mesa; botões, erros e avisos são diretos ("Correr", "Usuário ou senha incorretos.").

## 2. Tokens

Três camadas em `styles.css`. Componentes usam só a camada 2 (utilidades do Tailwind) e a 3 (`bg-(--botao-pri-bg)`); nunca primitivos nem hex. **Exceção deliberada:** tokens de componente com valor fixo nos dois temas apontam direto para primitivos ou valores crus — `--modal-scrim` (`rgb(14 19 18 / .6)`), `--tento-nos`/`--tento-eles` (giz amarelo/rosa da lousa), `--prazo-alerta` (giz vermelho) e `--botao-*-bg-hover/-active` do laranja (mistura com `white`/`black`). Isso só vale dentro da camada 3, nunca num componente `.vue`.

### 2.1 Primitivos (`--p-*`)

| Família | Token | Valor | Papel |
|---|---|---|---|
| Feltro | `--p-feltro-500` / `-700` | `#2B5E46` / `#1F4734` | mesa / borda e sombra do feltro |
| Lousa | `--p-lousa-950` / `-900` / `-800` / `-700` / `-500` | `#121817` / `#141A19` / `#1C2422` / `#26302D` / `#6A7A73` | cromo escuro, placar, painel, linhas |
| Giz | `--p-giz-100` / `-300` | `#EDEBE3` / `#CDD5D1` | texto sobre lousa e feltro / texto secundário |
| Giz de cor | `--p-giz-amarelo` / `--p-giz-rosa` / `--p-giz-rosa-200` | `#F5D76E` / `#F2A0B5` / `#F7B8C8` | nós / eles (traço) / eles (texto) |
| Aposta | `--p-laranja-400` / `-950` / `-700` | `#FF9A52` / `#2A1405` / `#A3420A` | botão e anúncio / texto sobre laranja / laranja como texto no claro |
| Perigo | `--p-giz-vermelho` / `--p-vermelho-700` / `-950` | `#FF8F80` / `#B3261E` / `#2A0C08` | perigo no escuro / no claro / texto sobre giz vermelho |
| Azulejo (cromo claro) | `--p-azulejo-50` / `-100` / `-500` / `-700` | `#F2F5F3` / `#E2E9E5` / `#6F7F78` / `#4A5752` | superfície / superfície 2 / linha / texto secundário |
| Duplas no claro | `--p-ocre-700` / `--p-framboesa-700` | `#7A5600` / `#A12F58` | giz amarelo e rosa escurecidos para texto |

**Cromo claro — por que "azulejo" e não o `#F1EFE7` da prévia:** o creme quente é o fundo-padrão de interface gerada e brigaria com o giz (`#EDEBE3`), que é quase o mesmo tom. O claro aqui é um cinza-esverdeado frio, do mesmo matiz do feltro (≈155°), como a parede de azulejo do bar: a mesa escura assenta nele sem moldura e o texto continua sendo "lousa" (`#141A19`, 16:1).

### 2.2 Semânticos (`--s-*`, expostos como `--color-*`)

`@theme inline` liga cada `--color-x` a `--s-x`; os `--s-*` usam `light-dark()` e são declarados em `:root, [data-tema]`. Consequência: qualquer subárvore com `data-tema="escuro"` (mesa, placar, painel) ou `data-tema="claro"` resolve as mesmas utilidades no outro tema. **A raiz da mesa leva `data-tema="escuro"`.** A paleta padrão do Tailwind foi zerada (`--color-*: initial`): `text-white`, `bg-black`, `bg-red-500` não existem.

| Utilidade | Claro | Escuro | Uso |
|---|---|---|---|
| `felt`, `felt-deep` | `#2B5E46`, `#1F4734` | igual | feltro (use `textura-feltro`) |
| `lousa`, `lousa-2` | `#141A19`, `#1C2422` | igual | placar, placas de assento, painel (use `textura-lousa`) |
| `giz`, `giz-muted` | `#EDEBE3`, `#CDD5D1` | igual | texto sobre feltro/lousa |
| `surface` | `#F2F5F3` | `#121817` | fundo de página, campo, modal |
| `surface-2` | `#E2E9E5` | `#1C2422` | painéis, linhas do lobby, hover de fantasma |
| `ink`, `ink-muted` | `#141A19`, `#4A5752` | `#EDEBE3`, `#CDD5D1` | texto principal / secundário |
| `line` | `#6F7F78` | `#6A7A73` | bordas de campo e divisórias (≥3:1) |
| `focus` | `#141A19` | `#EDEBE3` | anel de foco |
| `accent`, `on-accent` | `#FF9A52`, `#2A1405` | igual | só aposta (botão Truco/Seis/Nove/Doze, anúncio) |
| `accent-ink` | `#A3420A` | `#FF9A52` | laranja como texto/traço |
| `danger`, `on-danger` | `#B3261E`, `#FFFFFF` | `#FF8F80`, `#2A0C08` | perigo (texto, borda, preenchimento) |
| `nos`, `eles` | `#7A5600`, `#A12F58` | `#F5D76E`, `#F7B8C8` | duplas relativas |
| `team-0`, `team-1` | `#7A5600`, `#A12F58` | `#F5D76E`, `#F7B8C8` | duplas absolutas |

**Duplas — qual token usar.** `team-0` = assentos 0 e 2, `team-1` = assentos 1 e 3 (absolutos). `nos`/`eles` são relativos a `game.myTeam`/assento do jogador. Regra: **dentro de uma sala em que o jogador está sentado (sala de espera e mesa), sempre `nos`/`eles`** — a dupla do jogador é sempre amarela. `team-0`/`team-1` só onde não há "nós": linhas do lobby e quem ainda não sentou na sala de espera. Nunca derive a dupla com `seat % 2` (R25): use `teamOf`/`game.myTeam`.

### 2.3 Componente (`--botao-*`, `--campo-*` …)

Declarados também em `:root, [data-tema]`. Estados comuns: **hover** `--dur-rapido` (150 ms); **active** desloca `--botao-pressao-y` (1 px) para baixo e usa a cor `-active`; **focus** é o anel global (§6.1); **disabled** usa `opacity: var(--botao-desabilitado-opacidade)` (0,45), `cursor: not-allowed` e atributo `disabled`; **carregando** mantém a largura, troca o rótulo por "…" + `aria-busy="true"`.

| Componente | Tokens | Decisão |
|---|---|---|
| Botão primário | `--botao-pri-bg/-fg/-bg-hover/-bg-active`, `rounded-botao`, `h-(--botao-altura)` 44 px | preenchido com `ink` (lousa no claro, giz no escuro), texto `surface`, peso 700. Um por tela/painel |
| Botão secundário | `--botao-sec-*`, `rounded-botao-2` | transparente, contorno interno 2 px `--botao-sec-borda`, texto `ink` |
| Botão aposta | `--botao-aposta-*`, `rounded-aposta` | laranja, texto `on-accent`, **Kalam 700 20 px**: "Truco!", "Seis!", "Nove!", "Doze!" |
| Botão perigo | `--botao-perigo-*`, `rounded-botao` | `danger`/`on-danger`; só em confirmações (pedido proibido, sair de todos os dispositivos) |
| Botão fantasma | `--botao-fantasma-bg-hover` | texto `ink`, sem borda; ações terciárias ("Copiar link") |
| Campo | `--campo-*`, `rounded-campo`, 44 px | fundo `surface`, borda 2 px `line`; erro: borda `danger` + texto `danger` 14 px abaixo, ligado por `aria-describedby`. Rótulo visível acima, 14 px 700 |
| Modal | `--modal-*`, `rounded-modal`, `shadow-modal`, `max-w-(--modal-largura)` 448 px | título Atkinson 20 px 700 (não Kalam); scrim `--modal-scrim`; ações à direita, primária por último |
| Toast | `--toast-info-*`, `--toast-erro-*`, `rounded-toast`, `shadow-toast` | info = invertido (`ink` com texto `surface`); erro = `danger`/`on-danger`; canto inferior direito (384 px) no desktop; no celular (< 768 px) largura total acima da área segura; até 3 empilhados no total, 5 s. Nunca cobre cabeçalho nem banner. A região lê `--toast-offset` (padrão 0): **a view da mesa deve defini-lo no celular** com a altura da mão + painel de ação, para que o toast não cubra a mão nem os botões |
| Assento (mesa) | `--assento-*`, `rounded-placa` | placa de lousa com nome em giz 14 px 700; marca de dupla = traço de 3×40 px abaixo (nós contínuo, eles pontilhado); vez = contorno 2 px **tracejado** giz, afastado 3 px; ausente = opacidade `--assento-ausente-opacidade` (0,5) **só na placa do nome e na marca de dupla**; o ícone e o texto de estado "desconectado"/"ausente" (giz-muted 14 px, 5,03:1 no feltro) ficam fora do elemento esmaecido, em opacidade total |
| Carta | `rounded-card` 6 px, `shadow-card`; `--carta-largura-mao/-mesa/-vira` 84/68/48 px | mão: erguida `--carta-ergue-y` (−12 px) + `shadow-carta-erguida` no hover/seleção; nunca gira em movimento |
| Placar/tento | `--placar-bg`, `--tento-nos/-eles`, `--tento-traco` 3 px, `--valor-borda`, `rounded-valor` | ver §3 e §5 |
| Prazo (anel) | `--prazo-traco` 3 px, `--prazo-trilho`, `--prazo-cor`, `--prazo-alerta` | anel SVG em volta da placa; últimos 5 s em `--prazo-alerta` + segundos em texto |
| Painel de ação | `--painel-bg/-fg/-dica`, `rounded-painel` | lousa-2 sobre lousa; uma linha de contexto em `giz-muted` 14 px, depois os botões |
| Balão de sinal | `--balao-bg/-fg`, `rounded-toast` | placa de lousa, giz 14 px 700, rabicho apontando o assento, contorno 1 px `line` |

## 3. Tipografia

| Papel | Fonte | Tamanho / altura | Peso | Notas |
|---|---|---|---|---|
| Número do placar | Kalam | `text-placar` 40/40 | 700 | Kalam não tem algarismos tabulares: caixa `min-w-[2ch] text-center` |
| Tentos | SVG (traço de giz) | 28 px de altura por grupo | — | não é fonte; grupos de 5 (4 riscos + diagonal) |
| Valor da mão | Kalam | `text-valor` 20/24 | 400 | "vale 3", "vale 12"; ao lado, "Mão de 11"/"Mão de Ferro" em Atkinson 14 |
| Anúncio da mesa | Kalam | `text-anuncio` 72/72 (celular `text-anuncio-sm` 48) | 700 | inclinado −4°; "Truco!", "Seis!", "Mão 3" |
| Título de tela | Atkinson | `text-2xl` 24/32 | 700 | um por tela |
| Título de seção/modal | Atkinson | `text-xl` 20/28 | 700 | |
| Corpo | Atkinson | `text-base` 16/24 | 400 | mínimo para texto corrido; medida ≤ 65 ch |
| Rótulos, botões secundários, dicas | Atkinson | `text-sm` 14/20 | 400/700 | |
| Meta (contadores, "monte 27") | Atkinson | `text-xs` 12/16 | 400 | só informação repetida em outro lugar |
| Prazo, placar do lobby | Atkinson | `text-sm` + `tabular-nums` | 700 | |

- **Kalam só** em placar, tentos, valor da mão, anúncios da mesa e no rótulo do botão de aposta. Logo "Truco" do cabeçalho, títulos de tela e de modal usam Atkinson 700.
- Sentence case em tudo; sem caixa-alta, sem tracking aumentado.

## 4. Espaço, raios, sombras e camadas

- **Espaço:** base 4 px = escala padrão do Tailwind (`1` = 4 px). Usar 1, 2, 3, 4, 6, 8, 12. Gutter de página 16 px no celular, 24 px no desktop; entre botões ≥ 8 px.
- **Raios:** cartas e mini-cartas retas (6/4 px). Peças "de giz" têm cantos irregulares, nunca iguais em dois componentes vizinhos: `rounded-botao` 14/8/12/9, `rounded-botao-2` 12/9/14/8, `rounded-aposta` 9/14/8/12, `rounded-campo` 10/7/9/8, `rounded-painel` 16/10/14/12, `rounded-modal` 18/12/16/14, `rounded-toast` 12/9/13/8, `rounded-placa` 10/6/9/7, `rounded-valor` 18/10/16/12.
- **Sombras:** só onde há objeto físico ou camada: `shadow-card`, `shadow-carta-erguida`, `shadow-modal`, `shadow-toast`. Painéis e botões não têm sombra.
- **Camadas (`z-(--z-*)`):** mesa 0 · carta 10 · carta em voo 20 · balão 30 · anúncio 40 · painel 50 · banner 60 · cabeçalho 70 · modal 100 · toast 110.
- **Texturas:** `textura-feltro` (ruído fino + vinheta) e `textura-lousa` (pó de giz a 5 %), ambas `data:` SVG (CSP `img-src data:`).

## 5. Contraste medido (WCAG 2.x)

Texto normal ≥ 4,5:1; texto grande (≥ 24 px, ou ≥ 18,66 px em 700) e não-texto ≥ 3:1.

| Cor | Feltro `#2B5E46` | Lousa `#141A19` | Lousa-2 `#1C2422` | Escuro surface `#121817` | Claro surface `#F2F5F3` | Claro surface-2 `#E2E9E5` |
|---|---|---|---|---|---|---|
| giz `#EDEBE3` | 6,30 | 14,77 | 13,28 | 15,05 | — | — |
| giz-muted `#CDD5D1` | 5,03 | 11,78 | 10,59 | 12,01 | — | — |
| nós escuro `#F5D76E` | 5,31 | 12,44 | 11,19 | 12,69 | 1,29 ✗ | 1,15 ✗ |
| eles escuro `#F7B8C8` | 4,53 | 10,61 | 9,54 | 10,82 | ✗ | ✗ |
| giz rosa `#F2A0B5` (só traço) | 3,75 | 8,79 | 7,90 | 8,96 | ✗ | ✗ |
| laranja `#FF9A52` | 3,58 (só grande) | 8,39 | 7,55 | 8,55 | 1,91 ✗ | 1,70 ✗ |
| perigo escuro `#FF8F80` | 3,40 (só não-texto) | 7,97 | 7,17 | 8,12 | ✗ | ✗ |
| nós claro `#7A5600` | — | — | — | — | 6,06 | 5,39 |
| eles claro `#A12F58` | — | — | — | — | 6,25 | 5,56 |
| accent-ink claro `#A3420A` | — | — | — | — | 5,73 | 5,09 |
| perigo claro `#B3261E` | — | — | — | — | 5,95 | 5,30 |
| ink claro `#141A19` | — | — | — | — | 16,06 | 14,28 |
| ink-muted claro `#4A5752` | — | — | — | — | 6,89 | 6,13 |
| line claro `#6F7F78` | — | — | — | — | 3,84 | 3,42 |
| line escuro `#6A7A73` | — | 3,90 | 3,51 | 3,97 | — | — |

Pares de preenchimento: `on-accent` sobre laranja 8,33; giz sobre lousa (botão primário no escuro, texto lousa sobre giz) 14,77; surface claro sobre ink claro (primário no claro, toast info) 16,06; `#FFFFFF` sobre `#B3261E` 6,54; `#2A0C08` sobre `#FF8F80` 8,22.

**Consequências:**
- O giz rosa original `#F2A0B5` reprova texto no feltro (3,75). Texto de "eles" usa `#F7B8C8` (4,53 no feltro); o rosa original fica para tentos e traços.
- Laranja e perigo escuro sobre o feltro só como texto grande ou não-texto. O anúncio "Truco!" (72/48 px) passa; avisos de perigo na mesa vão numa placa de lousa.
- Giz amarelo/rosa e laranja reprovam no cromo claro: lá entram `nos`/`eles`/`accent-ink`/`danger` escurecidos (tabela acima). Como são tokens `light-dark()`, o componente usa sempre a mesma utilidade.
- **Cor nunca sozinha:** dupla = rótulo "Nós"/"Eles" no placar + posição (parceiro em frente) + traço contínuo (nós) ou pontilhado (eles) sob a placa; vez = contorno tracejado + anúncio "Sua vez" no `aria-live`; prazo = anel + segundos em texto; perigo = ícone + texto; laranja sempre com rótulo de aposta.

## 6. Acessibilidade

### 6.1 Foco

`:focus-visible` global: contorno 3 px sólido `focus`, afastado 2 px. Claro: `#141A19` sobre surface 16,06 / surface-2 14,28. Escuro e mesa: `#EDEBE3` sobre feltro 6,30, lousa 14,77, surface 15,05 — todos ≥ 3:1. O contorno de foco é **sólido**; o de vez é **tracejado** e mais fino: nunca se confundem. Elemento fixo (cabeçalho, painel, barra de sinais) não pode cobrir o foco (`scroll-margin` quando preciso).

### 6.2 Alvos de toque

≥ 44×44 px no celular para tudo que é clicável: botões (`--botao-altura`), campos, botões de sinal, cartas da mão (≥ 72×108 px no celular), assentos clicáveis da sala de espera. Espaço entre alvos ≥ 8 px. `touch-action: manipulation` no base.

### 6.3 Anúncios

- Uma região `aria-live="polite"` na mesa (visualmente oculta) anuncia: "Sua vez", "<nome> pediu truco/seis/nove/doze", "Rodada ganha por <nome>", "Mão para nós/eles, +N", "Aceito"/"Correu".
- Toasts: info com `role="status"` (`aria-live="polite"`); erro com `role="alert"` (R19). Nunca roubam foco.
- Modal: foco preso, foco inicial na ação segura (ex.: "Cancelar" no pedido proibido), `Esc` fecha, foco volta ao gatilho.

### 6.4 Teclado (spec §8.3)

`1`–`3` jogam a carta do slot · `C` alterna "Coberta" · `T` pede aumento · `A`/`R`/`U` = Aceitar/Correr/Aumentar · `Esc` fecha modais. Ignorados com foco em campo de texto. Botões com atalho mostram a tecla como dica (`aria-keyshortcuts` + texto "T" discreto em `ink-muted`).

## 7. Movimento

### 7.1 Tokens

| Token | Valor | Uso |
|---|---|---|
| `ease-giz` | `cubic-bezier(.37,0,.63,1)` | traço de giz (tentos, anúncios, círculo da vaza) |
| `ease-desliza` | `cubic-bezier(.22,1,.36,1)` | cartas chegando, carta erguida |
| `ease-sai` | `cubic-bezier(.55,0,1,.45)` | saídas (toast, balão, limpar mesa) |
| `ease-padrao` | `cubic-bezier(.2,0,0,1)` | hover, modal, destaques |
| `ease-quique` | `cubic-bezier(.34,1.56,.64,1)` | entrada de balão (só escala, ≤ 1,04) |
| `--dur-instante/-rapido/-medio/-carta/-tento/-truco` | 80 / 150 / 250 / 300 / 400 / 600 ms | — |

Só `transform`, `opacity`, `stroke-dashoffset` e `clip-path` animam. Nenhuma carta gira em movimento (a vira vira com `scaleX`). Nada sacode.

### 7.2 Eventos do diretor

Cada cena cabe no orçamento de `EVENT_DURATION_MS` (`frontend/src/game/director.ts`); o diretor aplica o `state` ao fim, então a cena termina dentro do orçamento.

| Evento | Orçamento | Cena | Duração e easing |
|---|---|---|---|
| `hand_started` | 600 | limpa a mesa (cartas somem); "Mão N" / "Mão de 11" / "Mão de Ferro" se escreve a giz no centro e some | 150 `ease-sai` + escrita 300 `ease-giz` (clip-path da esquerda) + 150 parado = 600 |
| `cut_offered` | 300 | placa de quem corta ganha halo de giz | 300 `ease-padrao` (opacidade 0→1) |
| `card_chosen` | 300 | idem para quem escolheu; carta escolhida ergue 8 px | 300 `ease-padrao` |
| `cards_dealt` | 700 | cartas deslizam do monte aos assentos, uma a uma | 12 cartas × 280 `ease-desliza`, intervalo 35 → 665 |
| `fours_burned` | 500 | cartas saem do assento para o descarte | até 4 × 300 `ease-desliza`, intervalo 50 → 450 |
| `vira_revealed` | 500 | vira vira no centro (`scaleX` 1→0→1, sem rotação); manilha ganha sublinhado de giz | 300 `ease-padrao` + traço 200 `ease-giz` |
| `eleven_decided` | 600 | balão "Joga!"/"Corre!" no assento | entrada 150 `ease-quique` + 450 parado |
| `card_played` | 350 | carta voa do assento à vaza (verso se coberta) e assenta no lado de quem jogou | 300 `ease-desliza` + 50 assentar |
| `round_ended` | 700 | círculo de giz se desenha em volta da carta vencedora; marca da rodada (✓/✗/=) no placar | círculo 400 `ease-giz` + marca 200 `ease-giz` + 100 parado |
| `raise_requested` | 900 | mesa escurece a 70 %; "Truco!"/"Seis!"/"Nove!"/"Doze!" se escreve a giz laranja sobre a mesa, com sublinhado; balão "Truco, ladrão!" no assento de quem pediu (só no truco) | escurecer 150 + escrita 600 `ease-giz` (traço do contorno, depois preenchimento) + 150 parado = 900 |
| `vote_cast` | 400 | balão do voto no assento (só se o voto é visível) | 150 `ease-quique` + 250 parado |
| `raise_resolved` | 700 | "Aceito!" ou "Correu!" a giz; valor da mão é apagado e reescrito | escrita 300 `ease-giz` + apaga 150 `ease-sai` + reescreve 250 `ease-giz` |
| `raise_punished` | 1200 | placa de lousa com borda `danger`: "Pedido proibido — derrota" | entrada 200 `ease-padrao` + 1000 parado (sem tremer) |
| `hand_ended` | 800 | tentos novos riscam um a um na lousa; número reescreve | N tentos em sequência, cada risco `min(400, max(40, floor(600/N)))` ms (o 400 é `--dur-tento`) (`ease-giz`), + número 200 ms. Como N ≤ 12, `floor(600/N)` ≥ 50: N × risco ≤ 600 e o total ≤ 800 (N = 12: 12 × 50 + 200 = 800; N = 1: 400 + 200 = 600) |
| `game_over` | 0 | nada; o modal vem com `match_ended` | — |
| `forfeited` | 0 | nada; idem | — |

Atraso > 3 s ou aba oculta: o diretor descarta a fila; os componentes mostram o estado final sem animar (spec §6.2).

### 7.3 Micro-interações

| Interação | Movimento |
|---|---|
| Botão pressionado | `translateY(1px)` + cor `-active`, `--dur-instante` |
| Hover de botão/linha | cor, `--dur-rapido` (150 ms) `ease-padrao` |
| Carta da mão (hover/seleção) | ergue −12 px + sombra, `--dur-rapido` `ease-desliza` |
| Toast | entra 200 ms (sobe 8 px + opacidade) `ease-desliza`; sai 150 ms `ease-sai` |
| Modal | scrim 200 ms; caixa 200 ms opacidade + escala 0,98→1 `ease-padrao`; sai 150 ms |
| Balão de sinal | entra 150 ms `ease-quique`; fica ~3 s; sai 200 ms `ease-sai` |
| Anel de prazo | contínuo (rAF), linear — é progresso real |

### 7.4 `prefers-reduced-motion`

`styles.css` zera `--dur-*` (cartas ficam em 50 ms) e força `animation/transition-duration: 0.01ms`. O diretor já usa ~50 ms por evento. Na prática: giz aparece pronto (tento, anúncio, círculo), cartas mudam de lugar sem viagem, carta erguida sobe só 4 px, nada escala nem sacode, escurecimento da mesa vira troca instantânea. Sons seguem `prefs`.

## 8. Composição

### 8.1 Mesa — desktop 1440×900

Mesa à esquerda (fluida), coluna de lousa de 320 px à direita. O placar é a faixa de lousa no topo da mesa.

```
┌──────────────────────────────────────────────────────────────────────────────┐
│ Truco   Mesa do RH · privada                     (Reconectando…)   Carlos ▾ │ 48 cabeçalho (cromo)
├──────────────────────────────────────────────────────┬───────────────────────┤
│ Nós  ┼┼┼┼ ||  7     Eles ||||  4    Mão 6 · dá: Tetê  ( vale 3 ) Mão de 11  │  PAINEL DE AÇÃO       │
│──────────────────────────── lousa ─────────────────── │  Jô pediu truco.      │
│                     [ Bia ]  ◯prazo                    │  Sua dupla responde.  │
│                      ─────  (nós, contínuo)            │  [     Aceitar     ]  │ primária
│                                                        │  [Correr] [ Seis! ]   │ sec. · aposta
│  [ Jô ]┆                ┌──┐  ┌──┐ ┌──┐      [ Tetê ]  │  ☐ Coberta  (C)       │
│  ◯ 9 s ┆   vira ┌──┐    │7♦│  │A♠│ │  │       ······   │───────────────────────│
│  ·····      manilha 6   └──┘  └──┘ └──┘   monte 27     │  SINAIS               │
│                      vaza (sem rotação)    descarte 1  │  [truco][boa][vamos]  │
│               ✓ ✗ ·  rodadas                           │  [calma][desc.][valeu]│
│                                                        │───────────────────────│
│                ┌────┐ ┌────┐ ┌────┐                    │  banner W.O. / aviso  │
│                │ 3♥ │ │ K♣ │ │ ▒▒ │  minha mão  1 2 3  │  "Adversários ausen-  │
│                └────┘ └────┘ └────┘                    │   tes — pedir W.O."   │
└──────────────────────────── feltro ───────────────────┴───────────────────────┘
```

- **Placar:** esquerda→direita: Nós (rótulo 14 px + tentos + número), Eles, mão/dealer (`giz-muted` 14), à direita o valor em pílula de giz e o tipo da mão. Marcas das rodadas da mão atual (✓ ganha, ✗ perdida, = empate) aparecem no centro da mesa, abaixo da vaza.
- **Assentos:** eu embaixo (minha mão faz as vezes de placa; nome não se repete), parceiro em cima, adversários nos lados. Anel de prazo em volta da placa de quem está em `pendingSeats`; vez = contorno tracejado.
- **Centro:** vira (48 px, rótulo "vira · manilha N"), vaza com cartas de 68 px deslocadas para o lado de quem jogou, monte e descarte como contadores `text-xs`.
- **Painel de ação:** só aparece quando há algo a decidir; quando não, o espaço mostra "Vez de Jô" em `giz-muted`. **Hierarquia fixa:** no máximo uma primária (giz preenchido), aposta sempre laranja com Kalam e nunca primária, secundárias contornadas, perigo só no modal de confirmação.

| Fase | Primária | Secundária | Aposta |
|---|---|---|---|
| `cut_choice`, `dealer_choice` | — (as cartas são a ação) | — | — |
| `burn_fours` | "Queimar" (habilita com a seleção) | "Passar" | — |
| `eleven_decision` | "Jogar" | "Correr" | — |
| `playing` (minha vez) | — (clicar na carta joga) | toggle "Coberta" | "Truco!/Seis!/Nove!/Doze!" (`nextStakeLabel`) |
| `awaiting_raise_response` | "Aceitar" | "Correr" | "Seis!/Nove!/Doze!" (some quando `toStake = 12`) |
| pedido proibido (modal) | — | "Cancelar" (foco inicial) | perigo: "Pedir mesmo assim" |

- **Banners** (W.O., "Aberto em outra aba", "Nova versão disponível", reconexão): faixa de lousa-2 com ícone, texto 14 px e uma ação; W.O. no topo da coluna da direita; conexão/versão sob o cabeçalho, largura total, `z-(--z-banner)`. Durante reconexão, painel e mão ficam desabilitados com dica "Reconectando…".

### 8.2 Mesa — celular retrato 375×812 (< 768 px)

```
┌───────────────────────────────────┐
│ Truco                   Carlos ▾  │ 44
├───────────────────────────────────┤
│ Nós ┼┼┼┼|| 7   Eles |||| 4 (vale3)│ 48 placar (lousa)
├───────────────────────────────────┤
│ [Jô]┆◯9s   [ Bia ]◯   [Tetê]····  │ 64 faixa de assentos
│  ···        ─────                 │
├───────────────────────────────────┤
│        vira  ┌─┐ ┌─┐ ┌─┐          │
│        ┌─┐   │7│ │A│ │ │  monte27 │ centro reduzido
│        └─┘   └─┘ └─┘ └─┘          │ (cartas 52 px)
│             ✓ ✗ ·                 │
│            Truco!  (48 px)        │
├───────────────────────────────────┤
│   ┌─────┐  ┌─────┐  ┌─────┐       │ minha mão
│   │ 3♥  │  │ K♣  │  │ ▒▒  │       │ (cartas 88 px)
│   └─────┘  └─────┘  └─────┘       │
├───────────────────────────────────┤
│ Jô pediu truco. Sua dupla responde│ painel de ação
│ [          Aceitar             ]  │ largura total
│ [  Correr  ]   [    Seis!     ]   │
│ ☐ Coberta              [Sinais ▴] │ 44
└───────────────────────────────────┘
```

- Sem rolagem horizontal. A barra de sinais vira o botão "Sinais", que abre uma folha inferior com os 6 sinais em grade 3×2 (44 px cada).
- Painel e mão ficam fixos embaixo, com `padding-bottom: env(safe-area-inset-bottom)`; o centro encolhe, nunca a mão.

### 8.3 Lobby

```
┌──────────────────────────────────────────────────────────────┐
│ Truco                                         Carlos ▾        │
├──────────────────────────────────────────────────────────────┤
│ Salas                         [Entrar com código] [Criar sala]│  (Criar = primária)
│                                                               │
│ Mesa do RH       aguardando   Ana · [livre] · Léo · [livre]  ›│  surface-2, linhas
│ Sala da TI       em jogo 9×11 Bia · Jô · Teo · Tetê   (cheia) │  separadas por
│ …                                                             │  traço de giz
│ Nenhuma sala aberta. Crie uma e chame a turma.  [Criar sala]  │  (vazio)
└──────────────────────────────────────────────────────────────┘
```

Lista única (não cartões), largura máx. 960 px, alinhada à esquerda. Assentos da linha: `team-0`/`team-1` no traço sob o nome; "livre" é botão fantasma de 44 px. Sala cheia/em jogo: linha com `ink-muted` e `aria-disabled`.

### 8.4 Sala de espera

```
┌──────────────────────────────────────────────────────────────┐
│ Mesa do RH · privada   código K7Q2  [Copiar link]  [Sair da sala]│
├──────────────────────────────────────────────────────────────┤
│                       [ Bia  ✓ pronta ]                        │
│                         ───── nós                              │
│    [ Jô ✓ ]  ·····       feltro         [ (livre) Sentar ]     │
│                                                                │
│                       [ Carlos ♛ você ]                        │
│                                                                │
│               [ Estou pronto ]  (toggle, primária)             │
└──────────────────────────────────────────────────────────────┘
```

Mesmo feltro e mesmas placas da mesa (continuidade visual), com `nos`/`eles` relativos a quem está sentado. Host vê "Remover" (fantasma) nas placas ocupadas.

### 8.5 Contas (entrar, cadastro, senha, perfil)

Telas públicas (entrar, cadastro, verificar e-mail, esqueci/redefinir senha) usam `CartaoConta`: **lousa à esquerda, formulário à direita**.

```
desktop ≥ 1024 px                                 celular / tablet
┌────────────────────┬───────────────────────┐    ┌───────────────────────────┐
│ ┼┼┼┼ Truco          │                       │    │ ┼┼┼┼ Truco   Puxa a cadeira.│  tira de lousa (~64 px)
│                    │ Entrar                │    ├───────────────────────────┤
│  Puxa a            │ Usuário ou e-mail     │    │ Entrar                    │
│  cadeira.          │ [___________________] │    │ Usuário ou e-mail         │
│  (Kalam, −2°)      │ Senha                 │    │ [_____________________]   │
│                    │ [___________________] │    │ Senha                     │
│ ┼┼┼┼ |||           │ [       Entrar      ] │    │ [        Entrar        ]  │
│  ▯ ▯ ▯ (leque)     │ Esqueci · Criar conta │    │ Esqueci · Criar           │
└────────────────────┴───────────────────────┘    └───────────────────────────┘
```

- **Por quê:** o formulário sozinho sobre `surface` lia como página genérica. A lousa é a assinatura do jogo (§1) e leva a identidade para a primeira tela, sem mexer no formulário.
- **A lousa não muda com o tema** (`data-tema="escuro"`, `textura-lousa`), como a mesa; só a coluna do formulário troca claro/escuro. No tema escuro, `border-r-2 border-line/40` separa as duas colunas.
- **Conteúdo da lousa (decorativo, `aria-hidden`, sem foco):** logo-tento + "Truco" (Atkinson 700); uma frase curta em Kalam 700, tom de anúncio da mesa (padrão "Puxa a cadeira."; cadastro "Mais um pra mesa."); só no desktop, tentos em giz amarelo (grupo de 5) e rosa (grupo de 3) que se riscam uma vez (`--dur-truco`, `ease-giz`, `stroke-dashoffset`) e um leque estático de três cartas de 84 px. Com `prefers-reduced-motion` os tentos já aparecem prontos. Nenhuma carta se move.
- **Formulário:** coluna única de 400 px, sóbrio, título 24/700, campo 44 px, erro junto ao campo, uma primária; links de troca abaixo. Sem cartão nem sombra. Kalam fica fora do formulário.
- **Celular/tablet:** a lousa vira uma tira compacta no topo (logo à esquerda, frase à direita), depois o formulário.

**Perfil** (dentro do cabeçalho do app, sem lousa): coluna de 400 px, seções empilhadas separadas por traço (nome, senha, preferências com prévia do baralho em cartas de 68 px, sessões, créditos); "Sair de todos os dispositivos" é perigo e pede confirmação.

## 9. Fontes

| Fonte | Pesos | Subconjuntos | Licença | Inclusão |
|---|---|---|---|---|
| Kalam (Indian Type Foundry) | 400, 700 | latin, latin-ext | OFL 1.1 | `@fontsource/kalam@5.3.0` |
| Atkinson Hyperlegible Next (Braille Institute / The Atkinson Hyperlegible Next Project Authors) | 400, 700 | latin, latin-ext | OFL 1.1 | `@fontsource/atkinson-hyperlegible-next@5.3.0` |

`frontend/src/assets/fonts/fonts.css` declara 8 `@font-face` (`.woff2`, `unicode-range`, `font-display: swap`) apontando para os arquivos dos pacotes; o Vite os emite como assets separados (CSP `default-src 'self'`, sem `data:` para fontes). Licenças em `frontend/src/assets/fonts/LICENSES.md`. Fallbacks: `--font-display` → Comic Neue/Segoe Print/cursive; `--font-body` → Atkinson Hyperlegible/system-ui.

## 10. Notas para as tarefas consumidoras

- **T13** (corrige o código de exemplo do brief da T13): primário = `bg-(--botao-pri-bg) text-(--botao-pri-fg)` (não `bg-accent text-ink`); perigo = `bg-(--botao-perigo-bg) text-(--botao-perigo-fg)` (não `text-white`, que não existe); toast de erro idem com `--toast-erro-*`; Toggle ligado = `bg-ink` (laranja é só aposta); indicador "Reconectando…" pulsa em `ink-muted`; títulos de modal e o logo em Atkinson 700 (não `font-display`); campo com `h-(--campo-altura)` 44 px e `border-2 border-line`; `border-ink-muted/20` vira `border-line/40`.
- **T15–T16:** lobby e sala de espera conforme §8.3–8.4; duplas por §2.2.
- **T17–T19:** mesa conforme §8.1–8.2 com `data-tema="escuro"` na raiz; tentos em SVG com `stroke-dasharray`; animações pela tabela §7.2.
