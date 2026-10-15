# Spec 4a — Frontend jogável (contas, lobby e mesa): Plano de Implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Aplicação web em Vue 3 que permite jogar truco ponta a ponta no navegador: contas, lobby ao vivo, sala de espera e mesa com animações, sons, prazos, sinais e W.O.

**Architecture:** Um `WsClient` por aba é dono da conexão `/ws` e roteia cada `ServerMessage` para stores Pinia por domínio (`session`, `lobby`, `room`, `game`, `prefs`). Na store `game`, um `Director` puro enfileira `events`, dispara cena e som de cada um e só então aplica o `state` (snapshot do servidor, sempre a verdade); `state` sem eventos é aplicado na hora. Componentes só leem stores e chamam ações delas; nenhuma regra de jogo vive no cliente.

**Tech Stack:** Node 24, pnpm 12, Vite, Vue 3 (`<script setup>`, TS strict), Pinia, Vue Router, Tailwind CSS v4 (`@tailwindcss/vite`), Vitest + Vue Test Utils + jsdom, Playwright, ESLint (flat config, `typescript-eslint`, `eslint-plugin-vue`), `vue-tsc`. Backend existente em Go (só `cmd/gen-protocol` muda).

**Spec:** `docs/superpowers/specs/2026-10-15-truco-frontend-jogavel-design.md` (leia junto com este plano).

## Global Constraints

- Textos ao usuário, comentários, mensagens de commit e logs em **pt-BR**; identificadores em inglês, exceto nomes de views/componentes já fixados na spec (`EntrarView`, `Mesa`, `PainelAcao`…).
- Commits convencionais em pt-BR, **sem** linhas de atribuição (`Co-Authored-By` etc.).
- **Nunca** edite `frontend/src/protocol/*` à mão: rode `make gen-types` da raiz.
- Rode comandos do frontend a partir de `frontend/` com `pnpm`; dependências instaladas com `pnpm add` (o lockfile fixa as versões). Nada de `npm`/`yarn`.
- TypeScript `strict: true`, `noUncheckedIndexedAccess: true`. Sem `any` fora de testes.
- CSP do servidor: `default-src 'self'; connect-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'` — **nenhum** recurso externo (fontes, imagens, sons e scripts são locais; sem CDN, sem Google Fonts).
- Componentes e views nunca importam `@/ws/client` nem `@/api/*` diretamente; passam por stores.
- Regra de jogo não é reimplementada: o que pode ser feito vem de `PlayerView`, `pendingSeats` e dos getters listados na spec §6.1.
- Assets só com licença domínio público/CC0 (CC-BY com crédito). **CC-BY-SA e LGPL proibidas.**
- `localStorage` sempre dentro de `try/catch`, com padrão quando indisponível.
- Intervalo de sinais: **3 s** (igual ao servidor, `signalInterval`).
- Fechamentos WS do servidor: `4001` substituída, `4003` sessão inválida, `4008` política.
- Banco de dev: o compose do projeto (`make dev-db`); nunca mexer em containers de outros projetos.
- Nada de push, merge ou ação fora do repositório sem confirmação do usuário.
- Ao terminar cada tarefa: `pnpm typecheck && pnpm lint && pnpm test` verdes em `frontend/` (e `go test ./...` quando a tarefa tocar o backend).
- **Skills obrigatórias (spec §3):** cada tarefa lista em "Skills" quais carregar com a ferramenta Skill **antes** do primeiro passo. Ao fim de toda tarefa que mexe em `.vue`/`.ts`, carregue `lint-and-validate`. Se uma skill conflitar com a spec, vale a spec e o conflito vai para a mensagem do commit.

## Review Focus

Cinco entradas que a spec implica mas nenhum fluxo principal exercita; cada uma tem teste na tarefa dona:

1. **Sessão expirada antes do upgrade do WebSocket:** o navegador não expõe o 401 do upgrade (chega um close 1006). O cliente consulta `GET /api/me`; se 401, para de reconectar e manda para `/entrar` — teste `para de reconectar e sinaliza unauthorized quando a sessão caiu antes do hello` (Tarefa 6).
2. **Clique duplo numa carta ou num botão de ação:** só uma mensagem sai até a resposta (ou erro) do servidor — teste `ignora segunda jogada enquanto a primeira está em voo` (Tarefa 10).
3. **`?volta=` apontando para fora do app** (`https://evil.com`, `//evil.com`): o login ignora e vai para `/` — teste `sanitizeReturnTo rejeita destinos externos` (Tarefa 13).
4. **Código de erro desconhecido do servidor** (um `code` novo no backend): a UI mostra a `message` que veio do servidor em vez de texto vazio ou o código cru — teste `cai na mensagem do servidor para código desconhecido` (Tarefa 5).
5. **Link de convite aberto deslogado ou em minúsculas** (`/sala/k7qm2p`): passa pelo login, volta e entra na sala com o código normalizado em maiúsculas — teste `rota de convite normaliza o código e preserva o destino após login` (Tarefa 13).

---

## Mapa de arquivos

| Arquivo | Responsabilidade | Tarefa |
|---|---|---|
| `backend/cmd/gen-protocol/main.go`, `cards.go`, `cards_test.go` | exporta `frontend/src/protocol/cards.json` | 1 |
| `frontend/package.json`, `vite.config.ts`, `tsconfig*.json`, `eslint.config.ts`, `index.html`, `src/main.ts`, `src/App.vue`, `src/env.d.ts`, `src/styles.css` | scaffold, build, lint, testes | 2 |
| `Makefile`, `.github/workflows/ci.yml`, `.gitignore` | alvos `web-*`/`e2e`, job `frontend` | 2 |
| `src/game/cards.ts`, `src/game/seats.ts` | `CardID → Card`, rótulos, arquivos; rotação de assentos e duplas | 3 |
| `src/api/http.ts`, `src/api/auth.ts` | fetch JSON, `ApiError`, offset de relógio; rotas de conta | 4 |
| `src/game/labels.ts` | textos pt-BR de erros, fases, eventos, sinais, fim de partida | 5 |
| `src/ws/client.ts` | conexão, reconexão, `seq`, roteamento, códigos de fechamento | 6 |
| `src/stores/prefs.ts`, `src/stores/session.ts`, `src/stores/toasts.ts`, `src/app/wire.ts` | preferências; usuário e conexão; toasts; ligação WS → stores | 7 |
| `src/stores/lobby.ts`, `src/stores/room.ts` | lista ao vivo; sala, W.O., fim de partida | 8 |
| `src/game/director.ts` | fila de eventos → cena/som → aplica `state` | 9 |
| `src/stores/game.ts`, `src/test/fixtures.ts` | view, prazos, sinais, getters, ações de jogo | 10 |
| `src/audio/sfx.ts`, `src/assets/sfx/*` | efeitos sonoros | 11 |
| `src/assets/cards/{frances,espanhol}/*`, `scripts/cards/*` | SVGs das cartas e licenças | 12 |
| `src/styles.css` (tokens), `src/components/ui/*`, `src/router.ts`, `src/components/AppShell.vue` | design system, shell, rotas e guarda | 13 |
| `src/views/{Entrar,Cadastro,VerificarEmail,EsqueciSenha,RedefinirSenha,Perfil}View.vue` | contas | 14 |
| `src/views/LobbyView.vue`, `src/components/lobby/*` | lobby | 15 |
| `src/views/SalaView.vue`, `src/components/sala/*` | sala de espera | 16 |
| `src/components/mesa/{Mesa,AssentoMesa,Mao,Carta,Centro,Vaza,Placar,Prazo,CenaEvento}.vue` | mesa (layout, cena, prazos) | 17 |
| `src/components/mesa/{PainelAcao,Corte,Queima,DecisaoOnze,RespostaAumento}.vue`, `src/game/shortcuts.ts` | painel de ação e teclado | 18 |
| `src/components/mesa/{BarraSinais,BalaoSinal,BannerWO,FimPartida,Anunciador}.vue`, `src/game/tabTitle.ts` | sinais, presença, W.O., fim, `aria-live`, título | 19 |
| `frontend/e2e/*`, `frontend/playwright.config.ts`, `scripts/e2e.sh` | e2e de 4 navegadores | 20 |
| `README.md` | status e como rodar | 21 |

---

### Task 1: Exportar a tabela de cartas para o frontend

**Skills:** nenhuma (tarefa Go).

**Files:**
- Create: `backend/cmd/gen-protocol/cards.go`
- Create: `backend/cmd/gen-protocol/cards_test.go`
- Modify: `backend/cmd/gen-protocol/main.go`
- Create (gerado): `frontend/src/protocol/cards.json`

**Interfaces:**
- Produces: `frontend/src/protocol/cards.json` — array de 40 objetos `{"id": number, "rank": number, "suit": number}` em ordem de `id`, gerado por `make gen-types`.

- [ ] **Step 1: Escrever o teste que falha**

`backend/cmd/gen-protocol/cards_test.go`:

```go
package main

import (
	"encoding/json"
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
)

func TestRenderCardsCoversWholeDeckInIDOrder(t *testing.T) {
	var got []cardRow
	if err := json.Unmarshal(renderCards(), &got); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	if len(got) != engine.DeckSize {
		t.Fatalf("esperava %d cartas, veio %d", engine.DeckSize, len(got))
	}
	for i, row := range got {
		c := engine.CardFromID(engine.CardID(i))
		if row.ID != i || row.Rank != int(c.Rank) || row.Suit != int(c.Suit) {
			t.Fatalf("linha %d = %+v, esperava rank %d naipe %d", i, row, c.Rank, c.Suit)
		}
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./cmd/gen-protocol/`
Expected: FAIL — `undefined: cardRow` / `undefined: renderCards`.

- [ ] **Step 3: Implementar**

`backend/cmd/gen-protocol/cards.go`:

```go
package main

import (
	"encoding/json"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
)

// cardRow é uma linha da tabela id → carta usada pelo teste de paridade do frontend.
type cardRow struct {
	ID   int `json:"id"`
	Rank int `json:"rank"`
	Suit int `json:"suit"`
}

// renderCards serializa engine.CardFromID para os 40 ids, com indentação estável.
func renderCards() []byte {
	rows := make([]cardRow, engine.DeckSize)
	for i := range rows {
		c := engine.CardFromID(engine.CardID(i))
		rows[i] = cardRow{ID: i, Rank: int(c.Rank), Suit: int(c.Suit)}
	}
	out, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		panic(err) // tipos fixos: não falha
	}
	return append(out, '\n')
}
```

`backend/cmd/gen-protocol/main.go` (substitui o corpo de `main`):

```go
func main() {
	out := flag.String("out", "../frontend/src/protocol/union.ts", "arquivo de saída das uniões")
	cards := flag.String("cards", "../frontend/src/protocol/cards.json", "arquivo de saída da tabela de cartas")
	flag.Parse()
	if err := os.WriteFile(*out, []byte(protocol.RenderUnion()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*cards, renderCards(), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}
```

Atualize o comentário do pacote: `// Comando gen-protocol escreve as uniões TypeScript das mensagens WebSocket e a tabela de cartas.`

- [ ] **Step 4: Rodar teste e gerar**

Run: `cd backend && go test ./cmd/gen-protocol/ && cd .. && make gen-types && head -12 frontend/src/protocol/cards.json`
Expected: PASS; o JSON começa com `{"id": 0, "rank": 4, "suit": 0}` (o primeiro degrau de `rankLadder`) — confira contra `backend/internal/engine/card.go`.

- [ ] **Step 5: Commit**

```bash
git add backend/cmd/gen-protocol frontend/src/protocol/cards.json
git commit -m "feat(protocol): exporta a tabela de cartas para o teste de paridade do frontend"
```

---

### Task 2: Scaffold do frontend, Makefile e CI

**Skills:** `frontend-dev-guidelines` (convenções de pastas e componentes), `typescript-expert` (tsconfig estrito).

**Files:**
- Create: `frontend/package.json`, `frontend/pnpm-lock.yaml` (gerado), `frontend/vite.config.ts`, `frontend/tsconfig.json`, `frontend/tsconfig.app.json`, `frontend/tsconfig.node.json`, `frontend/eslint.config.ts`, `frontend/index.html`, `frontend/src/main.ts`, `frontend/src/App.vue`, `frontend/src/env.d.ts`, `frontend/src/styles.css`, `frontend/src/test/setup.ts`, `frontend/src/smoke.test.ts`
- Modify: `Makefile`, `.github/workflows/ci.yml`, `.gitignore`

**Interfaces:**
- Produces: alias `@` → `frontend/src`; scripts `pnpm dev | build | test | typecheck | lint | e2e`; alvos `make web-install web-dev web-build web-test e2e`.

- [ ] **Step 1: Inicializar o pacote e instalar dependências**

```bash
cd frontend
pnpm init
pnpm add vue vue-router pinia
pnpm add -D vite @vitejs/plugin-vue typescript vue-tsc @types/node \
  tailwindcss @tailwindcss/vite \
  vitest jsdom @vue/test-utils \
  eslint @eslint/js typescript-eslint eslint-plugin-vue vue-eslint-parser jiti globals \
  @playwright/test
```

Edite `frontend/package.json` para conter (mantendo as versões que o pnpm gravou em `dependencies`/`devDependencies`):

```json
{
  "name": "truco-frontend",
  "private": true,
  "type": "module",
  "engines": { "node": ">=24" },
  "packageManager": "pnpm@12.11.2",
  "scripts": {
    "dev": "vite",
    "build": "vue-tsc -b && vite build",
    "preview": "vite preview",
    "typecheck": "vue-tsc -b --noEmit",
    "lint": "eslint .",
    "test": "vitest run",
    "test:watch": "vitest",
    "e2e": "playwright test"
  }
}
```

- [ ] **Step 2: Configuração do Vite, Vitest e TypeScript**

`frontend/vite.config.ts`:

```ts
/// <reference types="vitest/config" />
import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

const backend = process.env.TRUCO_BACKEND ?? 'http://localhost:8080'

export default defineConfig({
  plugins: [vue(), tailwindcss()],
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  server: {
    port: 5173,
    strictPort: true,
    // Sem changeOrigin: o backend confere Origin contra APP_BASE_URL (http://localhost:5173).
    proxy: {
      '/api': { target: backend },
      '/ws': { target: backend, ws: true },
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['src/test/setup.ts'],
    include: ['src/**/*.test.ts'],
    restoreMocks: true,
  },
})
```

`frontend/tsconfig.json`:

```json
{
  "files": [],
  "references": [{ "path": "./tsconfig.app.json" }, { "path": "./tsconfig.node.json" }]
}
```

`frontend/tsconfig.app.json`:

```json
{
  "compilerOptions": {
    "target": "ES2023",
    "lib": ["ES2023", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "jsx": "preserve",
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "noImplicitOverride": true,
    "noFallthroughCasesInSwitch": true,
    "verbatimModuleSyntax": true,
    "resolveJsonModule": true,
    "isolatedModules": true,
    "skipLibCheck": true,
    "composite": true,
    "tsBuildInfoFile": "./node_modules/.tmp/tsconfig.app.tsbuildinfo",
    "types": ["vitest/globals", "vite/client"],
    "paths": { "@/*": ["./src/*"] }
  },
  "include": ["src/**/*.ts", "src/**/*.vue", "src/**/*.json"]
}
```

`frontend/tsconfig.node.json`:

```json
{
  "compilerOptions": {
    "target": "ES2023",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "strict": true,
    "skipLibCheck": true,
    "composite": true,
    "tsBuildInfoFile": "./node_modules/.tmp/tsconfig.node.tsbuildinfo",
    "types": ["node"]
  },
  "include": ["vite.config.ts", "eslint.config.ts", "playwright.config.ts", "e2e/**/*.ts"]
}
```

Adicione `"globals": true` em `test` no `vite.config.ts` (para `vitest/globals`).

`frontend/src/env.d.ts`:

```ts
/// <reference types="vite/client" />
declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<object, object, unknown>
  export default component
}
```

`frontend/src/test/setup.ts`:

```ts
import { afterEach } from 'vitest'

// jsdom pode não ter requestAnimationFrame (usado pelo anel de prazo).
if (typeof globalThis.requestAnimationFrame !== 'function') {
  globalThis.requestAnimationFrame = (cb: FrameRequestCallback) => setTimeout(() => cb(performance.now()), 16) as unknown as number
  globalThis.cancelAnimationFrame = (id: number) => clearTimeout(id)
}

afterEach(() => {
  document.body.innerHTML = ''
  try {
    localStorage.clear()
  } catch {
    // ambiente sem storage
  }
})
```

- [ ] **Step 3: ESLint**

`frontend/eslint.config.ts`:

```ts
import js from '@eslint/js'
import tseslint from 'typescript-eslint'
import vue from 'eslint-plugin-vue'
import vueParser from 'vue-eslint-parser'
import globals from 'globals'

export default tseslint.config(
  { ignores: ['dist', 'node_modules', 'src/protocol/**', 'playwright-report', 'test-results'] },
  js.configs.recommended,
  ...tseslint.configs.strict,
  ...vue.configs['flat/recommended'],
  {
    files: ['**/*.vue'],
    languageOptions: { parser: vueParser, parserOptions: { parser: tseslint.parser, sourceType: 'module' } },
  },
  {
    languageOptions: { globals: { ...globals.browser, ...globals.node } },
    rules: {
      'vue/multi-word-component-names': 'off',
      'no-restricted-imports': [
        'error',
        {
          patterns: [
            { group: ['@/ws/*', '@/api/*'], message: 'Componentes e views falam com o servidor só por stores.' },
          ],
        },
      ],
    },
  },
  {
    // Quem pode importar ws/ e api/: as próprias camadas, stores e a ligação do app.
    files: ['src/stores/**', 'src/ws/**', 'src/api/**', 'src/app/**', 'src/**/*.test.ts'],
    rules: { 'no-restricted-imports': 'off' },
  },
)
```

- [ ] **Step 4: Entrada do app e teste de fumaça**

`frontend/index.html`:

```html
<!doctype html>
<html lang="pt-BR">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <meta name="color-scheme" content="light dark" />
    <title>Truco</title>
  </head>
  <body>
    <div id="app"></div>
    <script type="module" src="/src/main.ts"></script>
  </body>
</html>
```

`frontend/src/styles.css`:

```css
@import 'tailwindcss';
```

`frontend/src/App.vue`:

```vue
<template>
  <main class="min-h-dvh">Truco</main>
</template>
```

`frontend/src/main.ts`:

```ts
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import './styles.css'

createApp(App).use(createPinia()).mount('#app')
```

`frontend/src/smoke.test.ts`:

```ts
import { mount } from '@vue/test-utils'
import App from './App.vue'

it('monta o app', () => {
  expect(mount(App).text()).toContain('Truco')
})
```

- [ ] **Step 5: Rodar tudo**

Run: `cd frontend && pnpm typecheck && pnpm lint && pnpm test && pnpm build`
Expected: tudo verde; `frontend/dist/index.html` existe.

- [ ] **Step 6: Makefile, .gitignore e CI**

Acrescente ao `Makefile` (e os nomes à linha `.PHONY`):

```make
web-install:
	cd frontend && pnpm install --frozen-lockfile

web-dev:
	cd frontend && pnpm dev

web-build:
	cd frontend && pnpm build

web-test:
	cd frontend && pnpm typecheck && pnpm lint && pnpm test

e2e:
	./scripts/e2e.sh
```

Acrescente ao `.gitignore`:

```
frontend/node_modules/
frontend/dist/
frontend/playwright-report/
frontend/test-results/
```

Acrescente o job em `.github/workflows/ci.yml` (no mesmo nível de `backend:`):

```yaml
  frontend:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: frontend
    steps:
      - uses: actions/checkout@v4
      - uses: pnpm/action-setup@v4
      - uses: actions/setup-node@v4
        with:
          node-version: 24
          cache: pnpm
          cache-dependency-path: frontend/pnpm-lock.yaml
      - run: pnpm install --frozen-lockfile
      - run: pnpm typecheck
      - run: pnpm lint
      - run: pnpm test
      - run: pnpm build
```

(`pnpm/action-setup` lê `packageManager` do `frontend/package.json`; se reclamar por não achar o arquivo na raiz, adicione `with: { package_json_file: frontend/package.json }`.)

- [ ] **Step 7: Commit**

```bash
git add frontend Makefile .gitignore .github/workflows/ci.yml
git commit -m "feat(frontend): scaffold Vite + Vue 3 + Pinia + Tailwind com lint, testes e CI"
```

---

### Task 3: Cartas e assentos (funções puras)

**Skills:** `typescript-expert`.

**Files:**
- Create: `frontend/src/game/cards.ts`, `frontend/src/game/cards.test.ts`
- Create: `frontend/src/game/seats.ts`, `frontend/src/game/seats.test.ts`

**Interfaces:**
- Consumes: `@/protocol/engine` (`Card`, `CardID`, `Rank`, `Suit`, `Seat`, `Team`), `@/protocol/cards.json` (Task 1).
- Produces:
  - `type DeckStyle = 'frances' | 'espanhol'`
  - `cardFromId(id: CardID): Card`
  - `rankLabel(rank: Rank, deck: DeckStyle): string`
  - `suitName(suit: Suit): 'ouros' | 'espadas' | 'copas' | 'paus'`
  - `suitSymbol(suit: Suit): '♦' | '♠' | '♥' | '♣'`
  - `cardLabel(id: CardID, deck: DeckStyle): string` — ex.: `"A de espadas"`
  - `cardFileName(id: CardID): string` — ex.: `"1-espadas.svg"`
  - `RANK_LADDER: readonly Rank[]`
  - `type RelativePosition = 'bottom' | 'left' | 'top' | 'right'`
  - `relativePosition(seat: Seat, mySeat: Seat): RelativePosition`
  - `teamOf(seat: Seat): Team`, `partnerOf(seat: Seat): Seat`
  - `seatsInDisplayOrder(mySeat: Seat): Seat[]` — `[eu, esquerda, topo, direita]`

- [ ] **Step 1: Testes que falham**

`frontend/src/game/cards.test.ts`:

```ts
import table from '@/protocol/cards.json'
import { cardFromId, cardLabel, cardFileName, rankLabel, suitName, suitSymbol } from './cards'

describe('cardFromId', () => {
  it('espelha engine.CardFromID para os 40 ids', () => {
    expect(table).toHaveLength(40)
    for (const row of table) {
      expect(cardFromId(row.id)).toEqual({ rank: row.rank, suit: row.suit })
    }
  })
})

describe('rótulos', () => {
  it('converte figuras e ás no baralho francês', () => {
    expect([1, 10, 11, 12, 7].map((r) => rankLabel(r, 'frances'))).toEqual(['A', 'J', 'Q', 'K', '7'])
  })
  it('mantém o número no baralho espanhol', () => {
    expect([1, 10, 11, 12, 7].map((r) => rankLabel(r, 'espanhol'))).toEqual(['1', '10', '11', '12', '7'])
  })
  it('nomeia naipes na ordem do engine', () => {
    expect([0, 1, 2, 3].map(suitName)).toEqual(['ouros', 'espadas', 'copas', 'paus'])
    expect([0, 1, 2, 3].map(suitSymbol)).toEqual(['♦', '♠', '♥', '♣'])
  })
  it('monta rótulo acessível e nome de arquivo', () => {
    const asDeEspadas = table.find((r) => r.rank === 1 && r.suit === 1)!.id
    expect(cardLabel(asDeEspadas, 'frances')).toBe('A de espadas')
    expect(cardLabel(asDeEspadas, 'espanhol')).toBe('1 de espadas')
    expect(cardFileName(asDeEspadas)).toBe('1-espadas.svg')
  })
})
```

`frontend/src/game/seats.test.ts`:

```ts
import { partnerOf, relativePosition, seatsInDisplayOrder, teamOf } from './seats'

it('roda os assentos a partir do meu, em sentido horário', () => {
  expect([0, 1, 2, 3].map((s) => relativePosition(s, 0))).toEqual(['bottom', 'left', 'top', 'right'])
  expect([0, 1, 2, 3].map((s) => relativePosition(s, 3))).toEqual(['left', 'top', 'right', 'bottom'])
  expect(seatsInDisplayOrder(2)).toEqual([2, 3, 0, 1])
})

it('calcula dupla e parceiro', () => {
  expect([0, 1, 2, 3].map(teamOf)).toEqual([0, 1, 0, 1])
  expect([0, 1, 2, 3].map(partnerOf)).toEqual([2, 3, 0, 1])
})
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd frontend && pnpm vitest run src/game`
Expected: FAIL — módulos `./cards` e `./seats` não existem.

- [ ] **Step 3: Implementar**

`frontend/src/game/cards.ts`:

```ts
import type { Card, CardID, Rank, Suit } from '@/protocol/engine'

export type DeckStyle = 'frances' | 'espanhol'

/** Mesma ordem de engine.rankLadder (do menor para o maior na força comum). */
export const RANK_LADDER: readonly Rank[] = [4, 5, 6, 7, 10, 11, 12, 1, 2, 3]

const SUIT_NAMES = ['ouros', 'espadas', 'copas', 'paus'] as const
const SUIT_SYMBOLS = ['♦', '♠', '♥', '♣'] as const
const FRENCH: Partial<Record<Rank, string>> = { 1: 'A', 10: 'J', 11: 'Q', 12: 'K' }

export function cardFromId(id: CardID): Card {
  const rank = RANK_LADDER[Math.floor(id / 4)]
  if (rank === undefined || id < 0 || !Number.isInteger(id)) throw new RangeError(`CardID inválido: ${id}`)
  return { rank, suit: id % 4 }
}

export function rankLabel(rank: Rank, deck: DeckStyle): string {
  return deck === 'frances' ? (FRENCH[rank] ?? String(rank)) : String(rank)
}

export function suitName(suit: Suit): (typeof SUIT_NAMES)[number] {
  const name = SUIT_NAMES[suit]
  if (!name) throw new RangeError(`naipe inválido: ${suit}`)
  return name
}

export function suitSymbol(suit: Suit): (typeof SUIT_SYMBOLS)[number] {
  const symbol = SUIT_SYMBOLS[suit]
  if (!symbol) throw new RangeError(`naipe inválido: ${suit}`)
  return symbol
}

export function cardLabel(id: CardID, deck: DeckStyle): string {
  const { rank, suit } = cardFromId(id)
  return `${rankLabel(rank, deck)} de ${suitName(suit)}`
}

/** Nome do arquivo SVG; igual nos dois baralhos (usa o rank neutro). */
export function cardFileName(id: CardID): string {
  const { rank, suit } = cardFromId(id)
  return `${rank}-${suitName(suit)}.svg`
}
```

Se o teste de paridade falhar, corrija `RANK_LADDER` copiando a ordem de `rankLadder` em `backend/internal/engine/card.go` — o JSON é a fonte da verdade.

`frontend/src/game/seats.ts`:

```ts
import type { Seat, Team } from '@/protocol/engine'

export type RelativePosition = 'bottom' | 'left' | 'top' | 'right'
const POSITIONS: readonly RelativePosition[] = ['bottom', 'left', 'top', 'right']

export function relativePosition(seat: Seat, mySeat: Seat): RelativePosition {
  return POSITIONS[(seat - mySeat + 4) % 4] as RelativePosition
}

export function seatsInDisplayOrder(mySeat: Seat): Seat[] {
  return [0, 1, 2, 3].map((offset) => (mySeat + offset) % 4)
}

export function teamOf(seat: Seat): Team {
  return seat % 2
}

export function partnerOf(seat: Seat): Seat {
  return (seat + 2) % 4
}
```

- [ ] **Step 4: Rodar e ver passar**

Run: `cd frontend && pnpm vitest run src/game && pnpm typecheck`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/game
git commit -m "feat(frontend): cartas e rotação de assentos com paridade contra o engine"
```

---

### Task 4: Cliente HTTP e API de contas

**Skills:** `typescript-expert`, `error-handling-patterns`.

**Files:**
- Create: `frontend/src/api/http.ts`, `frontend/src/api/http.test.ts`
- Create: `frontend/src/api/auth.ts`, `frontend/src/api/auth.test.ts`

**Interfaces:**
- Produces:
  - `class ApiError extends Error { status: number; code: string; fields: Record<string, string> }` (`code` é `'network'` para falha de rede)
  - `request<T>(method: 'GET'|'POST'|'PATCH', path: string, body?: unknown): Promise<T>` — `T` é `undefined` para 202/204
  - `getClockOffsetMs(): number` — `serverNow − clientNow`, média móvel das últimas 5 amostras do cabeçalho `Date`
  - `serverNow(): number` — `Date.now() + getClockOffsetMs()`
  - `interface User { id: string; username: string; email: string; displayName: string; isAdmin: boolean; createdAt: string }`
  - `authApi = { register, verifyEmail, resendVerification, login, logout, logoutAll, forgotPassword, resetPassword, me, updateMe, changePassword }` com as assinaturas abaixo.

- [ ] **Step 1: Testes que falham**

`frontend/src/api/http.test.ts`:

```ts
import { ApiError, getClockOffsetMs, request, resetClockSamples } from './http'

function respond(status: number, body?: unknown, headers: Record<string, string> = {}) {
  return vi.fn().mockResolvedValue(
    new Response(body === undefined ? null : JSON.stringify(body), {
      status,
      headers: { 'Content-Type': 'application/json', ...headers },
    }),
  )
}

beforeEach(() => resetClockSamples())

it('envia JSON com credenciais e devolve o corpo', async () => {
  const fetchMock = respond(200, { user: { id: '1' } })
  vi.stubGlobal('fetch', fetchMock)
  await expect(request('POST', '/api/auth/login', { login: 'a', password: 'b' })).resolves.toEqual({ user: { id: '1' } })
  const [url, init] = fetchMock.mock.calls[0]!
  expect(url).toBe('/api/auth/login')
  expect(init.credentials).toBe('same-origin')
  expect(init.headers['Content-Type']).toBe('application/json')
  expect(init.body).toBe('{"login":"a","password":"b"}')
})

it('devolve undefined em 204', async () => {
  vi.stubGlobal('fetch', respond(204))
  await expect(request('POST', '/api/auth/logout')).resolves.toBeUndefined()
})

it('converte o corpo de erro em ApiError', async () => {
  vi.stubGlobal('fetch', respond(400, { error: { code: 'validation_failed', message: 'Verifique', fields: { email: 'inválido' } } }))
  const err = await request('POST', '/api/auth/register', {}).catch((e: unknown) => e)
  expect(err).toBeInstanceOf(ApiError)
  expect(err).toMatchObject({ status: 400, code: 'validation_failed', fields: { email: 'inválido' } })
})

it('marca falha de rede com code network', async () => {
  vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')))
  await expect(request('GET', '/api/me')).rejects.toMatchObject({ code: 'network', status: 0 })
})

it('estima o offset do relógio pelo cabeçalho Date', async () => {
  vi.useFakeTimers({ now: new Date('2026-10-15T12:00:00Z') })
  vi.stubGlobal('fetch', respond(200, {}, { Date: 'Thu, 15 Oct 2026 12:00:10 GMT' }))
  await request('GET', '/api/me')
  expect(getClockOffsetMs()).toBeCloseTo(10_000, -2)
  vi.useRealTimers()
})
```

`frontend/src/api/auth.test.ts`:

```ts
import { authApi } from './auth'

it('usa os caminhos e corpos do backend', async () => {
  const calls: [string, string | undefined][] = []
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string, init: RequestInit) => {
      calls.push([`${init.method} ${url}`, init.body as string | undefined])
      return new Response(JSON.stringify({ user: { id: '1' } }), { status: 200 })
    }),
  )
  await authApi.login('ana', 'segredo-longo')
  await authApi.verifyEmail('tok')
  await authApi.resetPassword('tok', 'nova-senha-123')
  await authApi.updateMe('Ana')
  await authApi.changePassword('a', 'b')
  expect(calls).toEqual([
    ['POST /api/auth/login', '{"login":"ana","password":"segredo-longo"}'],
    ['POST /api/auth/verify-email', '{"token":"tok"}'],
    ['POST /api/auth/password/reset', '{"token":"tok","password":"nova-senha-123"}'],
    ['PATCH /api/me', '{"displayName":"Ana"}'],
    ['POST /api/me/password', '{"currentPassword":"a","newPassword":"b"}'],
  ])
})
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd frontend && pnpm vitest run src/api`
Expected: FAIL — módulos inexistentes.

- [ ] **Step 3: Implementar**

`frontend/src/api/http.ts`:

```ts
export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
    readonly fields: Record<string, string> = {},
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

type Method = 'GET' | 'POST' | 'PATCH'

const MAX_SAMPLES = 5
let clockSamples: number[] = []

export function resetClockSamples(): void {
  clockSamples = []
}

export function getClockOffsetMs(): number {
  if (clockSamples.length === 0) return 0
  return clockSamples.reduce((a, b) => a + b, 0) / clockSamples.length
}

export function serverNow(): number {
  return Date.now() + getClockOffsetMs()
}

function sampleClock(res: Response): void {
  const header = res.headers.get('Date')
  if (!header) return
  const server = Date.parse(header)
  if (Number.isNaN(server)) return
  // O cabeçalho tem resolução de 1 s; a média suaviza.
  clockSamples = [...clockSamples, server - Date.now()].slice(-MAX_SAMPLES)
}

export async function request<T>(method: Method, path: string, body?: unknown): Promise<T> {
  let res: Response
  try {
    res = await fetch(path, {
      method,
      credentials: 'same-origin',
      headers: body === undefined ? { Accept: 'application/json' } : { Accept: 'application/json', 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
    })
  } catch {
    throw new ApiError(0, 'network', 'Sem conexão com o servidor.')
  }
  sampleClock(res)
  if (res.status === 202 || res.status === 204) return undefined as T
  const text = await res.text()
  const data: unknown = text ? JSON.parse(text) : undefined
  if (!res.ok) {
    const err = (data as { error?: { code?: string; message?: string; fields?: Record<string, string> } } | undefined)?.error
    throw new ApiError(res.status, err?.code ?? 'internal', err?.message ?? 'Erro no servidor, tente de novo.', err?.fields ?? {})
  }
  return data as T
}
```

(O `202` do cadastro e do reenvio volta `undefined`: o corpo do backend é só confirmação.)

`frontend/src/api/auth.ts`:

```ts
import { request } from './http'

export interface User {
  id: string
  username: string
  email: string
  displayName: string
  isAdmin: boolean
  createdAt: string
}

export interface RegisterInput {
  username: string
  email: string
  displayName: string
  password: string
}

type UserBody = { user: User }

export const authApi = {
  register: (input: RegisterInput) => request<void>('POST', '/api/auth/register', input),
  verifyEmail: (token: string) => request<void>('POST', '/api/auth/verify-email', { token }),
  resendVerification: (email: string) => request<void>('POST', '/api/auth/resend-verification', { email }),
  login: async (login: string, password: string) => (await request<UserBody>('POST', '/api/auth/login', { login, password })).user,
  logout: () => request<void>('POST', '/api/auth/logout'),
  logoutAll: () => request<void>('POST', '/api/auth/logout-all'),
  forgotPassword: (email: string) => request<void>('POST', '/api/auth/password/forgot', { email }),
  resetPassword: (token: string, password: string) => request<void>('POST', '/api/auth/password/reset', { token, password }),
  me: async () => (await request<UserBody>('GET', '/api/me')).user,
  updateMe: async (displayName: string) => (await request<UserBody>('PATCH', '/api/me', { displayName })).user,
  changePassword: (currentPassword: string, newPassword: string) =>
    request<void>('POST', '/api/me/password', { currentPassword, newPassword }),
}
```

Antes do commit, confira contra `backend/internal/httpapi/auth_handlers.go` o corpo de resposta de `PATCH /api/me` e de `POST /api/auth/login` (`{"user": …}`); se `PATCH /api/me` devolver 204, troque `updateMe` para chamar `request<void>` e depois `me()`.

- [ ] **Step 4: Rodar e ver passar**

Run: `cd frontend && pnpm vitest run src/api && pnpm typecheck`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/api
git commit -m "feat(frontend): cliente HTTP com erros tipados, offset de relógio e API de contas"
```

---

### Task 5: Textos pt-BR (`labels.ts`)

**Skills:** `ui-ux-pro-max` (tom das mensagens de erro e estados).

**Files:**
- Create: `frontend/src/game/labels.ts`, `frontend/src/game/labels.test.ts`

**Interfaces:**
- Consumes: `@/protocol/engine` (`Event`, `Phase`, `HandKind`, `VoteChoice`), `@/protocol/messages` (`SignalID`, `EndReason`).
- Produces:
  - `errorText(code: string, serverMessage?: string): string`
  - `phaseText(phase: Phase): string`
  - `handKindText(kind: HandKind | undefined): string`
  - `stakeName(stake: number): string` — `3→'Truco'`, `6→'Seis'`, `9→'Nove'`, `12→'Doze'`, demais `String(stake)`
  - `nextStake(stake: number): number | null` — `1→3, 3→6, 6→9, 9→12, 12→null`
  - `voteText(choice: VoteChoice): string`
  - `SIGNALS: readonly { id: SignalID; text: string }[]`
  - `signalText(id: SignalID): string`
  - `endReasonText(reason: EndReason): string`
  - `eventAnnouncement(ev: Event, nameOf: (seat: number) => string): string | null` — frase para o `aria-live`; `null` quando o evento não merece anúncio

- [ ] **Step 1: Testes que falham**

`frontend/src/game/labels.test.ts`:

```ts
import { endReasonText, errorText, eventAnnouncement, nextStake, signalText, stakeName, SIGNALS } from './labels'

it('traduz códigos conhecidos de HTTP, sala e regra', () => {
  expect(errorText('invalid_credentials')).toBe('Usuário ou senha incorretos.')
  expect(errorText('room_full')).toBe('A sala está cheia.')
  expect(errorText('not_your_turn')).toBe('Não é a sua vez.')
})

it('cai na mensagem do servidor para código desconhecido', () => {
  expect(errorText('codigo_novo', 'Mensagem do servidor.')).toBe('Mensagem do servidor.')
  expect(errorText('codigo_novo')).toBe('Algo deu errado. Tente de novo.')
})

it('nomeia a escada de apostas', () => {
  expect([1, 3, 6, 9, 12].map(nextStake)).toEqual([3, 6, 9, 12, null])
  expect([3, 6, 9, 12].map(stakeName)).toEqual(['Truco', 'Seis', 'Nove', 'Doze'])
})

it('cobre os 6 sinais e os motivos de fim', () => {
  expect(SIGNALS.map((s) => s.id)).toEqual(['truco', 'boa', 'vamos', 'calma', 'desculpa', 'valeu'])
  expect(signalText('valeu')).toBe('Valeu!')
  expect(endReasonText('forfeit')).toBe('Vitória por W.O.')
})

it('anuncia pedidos de aumento e ignora eventos internos', () => {
  const nome = (s: number) => ['Ana', 'Bia', 'Caio', 'Duda'][s]!
  expect(eventAnnouncement({ kind: 'raise_requested', seat: 1, stake: 3 }, nome)).toBe('Bia pediu Truco!')
  // contra-aumento (voto "raise") vem sem seat, só com a dupla
  expect(eventAnnouncement({ kind: 'raise_requested', team: 0, stake: 6 }, nome)).toBe('A dupla 1 pediu Seis!')
  expect(eventAnnouncement({ kind: 'cut_offered', seat: 1 }, nome)).toBeNull()
})
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd frontend && pnpm vitest run src/game/labels.test.ts`
Expected: FAIL — módulo inexistente.

- [ ] **Step 3: Implementar**

Campos dos eventos (conferidos em `backend/internal/engine/{betting,deal,scoring,forfeit}.go`): `raise_requested{seat?, team, stake}` (sem `seat` no contra-aumento), `raise_resolved{team, choice, stake}`, `vote_cast{seat, choice}` (só para a dupla), `round_ended{team?, seat?}` (sem `team` = empate), `hand_ended{team?, stake, score}`, `game_over{team, score}`, `forfeited{team}`, `hand_started{seat=dealer}`, `card_chosen{seat}`, `cut_offered{seat}`, `fours_burned{seat, count}`, `vira_revealed{card}`, `card_played{seat, card?}`, `eleven_decided{team, choice}`, `raise_punished{seat, team}`.

`frontend/src/game/labels.ts`:

```ts
import type { Event, HandKind, Phase, VoteChoice } from '@/protocol/engine'
import type { EndReason, SignalID } from '@/protocol/messages'

const ERRORS: Record<string, string> = {
  // HTTP / contas
  network: 'Sem conexão com o servidor.',
  internal: 'Erro no servidor, tente de novo.',
  invalid_request: 'Requisição inválida.',
  validation_failed: 'Verifique os campos destacados.',
  invalid_credentials: 'Usuário ou senha incorretos.',
  email_not_verified: 'Confirme seu e-mail antes de entrar.',
  account_disabled: 'Esta conta está desativada.',
  password_reset_required: 'Você precisa redefinir sua senha. Use "Esqueci minha senha".',
  username_taken: 'Este nome de usuário já está em uso.',
  email_domain_not_allowed: 'O cadastro só é permitido com o e-mail da empresa.',
  invalid_token: 'Link inválido ou expirado.',
  unauthenticated: 'Faça login para continuar.',
  too_many_attempts: 'Muitas tentativas. Aguarde um pouco e tente de novo.',
  forbidden_origin: 'Origem da requisição não permitida.',
  // Sala / lobby
  bad_request: 'Ação inválida.',
  already_in_room: 'Você já está em uma sala.',
  room_full: 'A sala está cheia.',
  room_not_found: 'Sala não encontrada.',
  seat_taken: 'Esse lugar já está ocupado.',
  not_host: 'Só o anfitrião pode fazer isso.',
  not_in_room: 'Você não está em uma sala.',
  match_in_progress: 'A partida está em andamento.',
  forfeit_unavailable: 'O W.O. não está disponível agora.',
  too_many_rooms: 'O servidor atingiu o limite de salas. Tente mais tarde.',
  rate_limited: 'Calma! Espere um pouco antes de repetir.',
  // Regras do engine
  invalid_phase: 'Essa ação não vale nesta fase.',
  not_your_turn: 'Não é a sua vez.',
  invalid_slot: 'Carta inválida.',
  invalid_choice: 'Escolha inválida.',
  illegal_raise: 'Não dá para aumentar agora.',
  already_voted: 'Você já votou.',
  not_voting_team: 'Sua dupla não está votando.',
  covered_not_allowed: 'Não é permitido jogar coberta agora.',
  burn_not_four: 'Só dá para queimar cartas 4.',
  game_over: 'A partida já terminou.',
  already_decided: 'Você já decidiu.',
  invalid_seat: 'Lugar inválido.',
}

export function errorText(code: string, serverMessage?: string): string {
  return ERRORS[code] ?? serverMessage ?? 'Algo deu errado. Tente de novo.'
}

const PHASES: Record<Phase, string> = {
  waiting: 'Aguardando',
  cut_choice: 'Corte',
  dealer_choice: 'Escolha do pé',
  burn_fours: 'Queima dos 4',
  eleven_decision: 'Mão de 11',
  playing: 'Jogando',
  awaiting_raise_response: 'Resposta ao aumento',
  hand_end: 'Fim da mão',
  game_over: 'Fim de jogo',
}

export function phaseText(phase: Phase): string {
  return PHASES[phase] ?? phase
}

export function handKindText(kind: HandKind | undefined): string {
  if (kind === 'eleven') return 'Mão de 11'
  if (kind === 'iron') return 'Mão de Ferro'
  return 'Mão normal'
}

const STAKE_NAMES: Record<number, string> = { 3: 'Truco', 6: 'Seis', 9: 'Nove', 12: 'Doze' }
const NEXT_STAKE: Record<number, number> = { 1: 3, 3: 6, 6: 9, 9: 12 }

export function stakeName(stake: number): string {
  return STAKE_NAMES[stake] ?? String(stake)
}

export function nextStake(stake: number): number | null {
  return NEXT_STAKE[stake] ?? null
}

const VOTES: Record<VoteChoice, string> = { accept: 'Aceito', run: 'Corro', raise: 'Aumento', play: 'Jogo' }

export function voteText(choice: VoteChoice): string {
  return VOTES[choice] ?? choice
}

export const SIGNALS: readonly { id: SignalID; text: string }[] = [
  { id: 'truco', text: 'Truco!' },
  { id: 'boa', text: 'Boa!' },
  { id: 'vamos', text: 'Vamos!' },
  { id: 'calma', text: 'Calma…' },
  { id: 'desculpa', text: 'Desculpa!' },
  { id: 'valeu', text: 'Valeu!' },
]

export function signalText(id: SignalID): string {
  return SIGNALS.find((s) => s.id === id)?.text ?? id
}

const END_REASONS: Record<EndReason, string> = {
  game_over: 'Fim de jogo',
  forfeit: 'Vitória por W.O.',
  aborted: 'Partida abortada pelo administrador',
  abandoned: 'Partida abandonada',
}

export function endReasonText(reason: EndReason): string {
  return END_REASONS[reason] ?? reason
}

export function eventAnnouncement(ev: Event, nameOf: (seat: number) => string): string | null {
  const who = ev.seat !== undefined ? nameOf(ev.seat) : ev.team !== undefined ? `A dupla ${ev.team + 1}` : 'Alguém'
  switch (ev.kind) {
    case 'raise_requested':
      return `${who} pediu ${stakeName(ev.stake ?? 0)}!`
    case 'raise_resolved':
      return ev.choice === 'run' ? 'A dupla correu.' : `Aceito: a mão vale ${ev.stake ?? ''}.`
    case 'raise_punished':
      return 'Pedido proibido: a dupla perdeu a partida.'
    case 'round_ended':
      return ev.team === undefined ? 'Rodada empatada.' : `Rodada para a dupla ${ev.team + 1}.`
    case 'hand_ended':
      return ev.score ? `Fim da mão. Placar ${ev.score[0]} a ${ev.score[1]}.` : 'Fim da mão.'
    case 'vira_revealed':
      return 'Vira revelada.'
    default:
      return null
  }
}
```

- [ ] **Step 4: Rodar e ver passar**

Run: `cd frontend && pnpm vitest run src/game/labels.test.ts && pnpm typecheck`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/game/labels.ts frontend/src/game/labels.test.ts
git commit -m "feat(frontend): textos pt-BR de erros, fases, apostas, sinais e anúncios"
```

---

### Task 6: Cliente WebSocket

**Skills:** `typescript-expert`, `error-handling-patterns`.

**Files:**
- Create: `frontend/src/ws/client.ts`, `frontend/src/ws/client.test.ts`, `frontend/src/test/fakeSocket.ts`

**Interfaces:**
- Consumes: `@/protocol/union` (`ClientMessage`, `ServerMessage`).
- Produces:
  - `type WsStatus = 'idle' | 'connecting' | 'open' | 'reconnecting' | 'replaced' | 'unauthorized'`
  - `type Outgoing = ClientMessage extends infer M ? (M extends { seq?: number } ? Omit<M, 'seq'> : never) : never` (mensagem sem `seq`)
  - `const EXPECTED_PROTOCOL_VERSION = 1`
  - `interface WsClientOptions { url: string; onMessage(msg: ServerMessage): void; onStatus(status: WsStatus): void; onVersionMismatch(serverVersion: number): void; onUnackedError(code: string, message: string): void; checkSession(): Promise<boolean>; createSocket?(url: string): SocketLike; setTimer?(fn: () => void, ms: number): unknown; clearTimer?(handle: unknown): void; random?(): number }`
  - `interface SocketLike { readyState: number; send(data: string): void; close(code?: number, reason?: string): void; onopen: ((ev: unknown) => void) | null; onmessage: ((ev: { data: unknown }) => void) | null; onclose: ((ev: { code: number; reason: string }) => void) | null; onerror: ((ev: unknown) => void) | null }`
  - `class WsClient { constructor(opts: WsClientOptions); connect(): void; disconnect(): void; resume(): void; send(msg: Outgoing): Promise<void>; get status(): WsStatus }`
  - `class WsSendError extends Error { code: string }`
  - `backoffDelay(attempt: number, random: () => number): number` — `min(10000, 500 * 2^attempt) * (0.5 + random()/2)`
  - `wsUrl(loc: { protocol: string; host: string }): string`

- [ ] **Step 1: Socket falso para testes**

`frontend/src/test/fakeSocket.ts`:

```ts
import type { SocketLike } from '@/ws/client'
import type { ServerMessage } from '@/protocol/union'

export class FakeSocket implements SocketLike {
  static instances: FakeSocket[] = []
  readyState = 0
  sent: string[] = []
  onopen: ((ev: unknown) => void) | null = null
  onmessage: ((ev: { data: unknown }) => void) | null = null
  onclose: ((ev: { code: number; reason: string }) => void) | null = null
  onerror: ((ev: unknown) => void) | null = null

  constructor(readonly url: string) {
    FakeSocket.instances.push(this)
  }
  send(data: string) {
    this.sent.push(data)
  }
  close(code = 1000, reason = '') {
    this.serverClose(code, reason)
  }
  open() {
    this.readyState = 1
    this.onopen?.({})
  }
  receive(msg: ServerMessage | string) {
    this.onmessage?.({ data: typeof msg === 'string' ? msg : JSON.stringify(msg) })
  }
  serverClose(code: number, reason = '') {
    this.readyState = 3
    this.onclose?.({ code, reason })
  }
  sentMessages() {
    return this.sent.map((s) => JSON.parse(s) as Record<string, unknown>)
  }
  static last(): FakeSocket {
    const s = FakeSocket.instances.at(-1)
    if (!s) throw new Error('nenhum socket criado')
    return s
  }
}
```

- [ ] **Step 2: Testes que falham**

`frontend/src/ws/client.test.ts`:

```ts
import { FakeSocket } from '@/test/fakeSocket'
import { backoffDelay, WsClient, WsSendError, wsUrl, type WsClientOptions, type WsStatus } from './client'
import type { ServerMessage } from '@/protocol/union'

const hello: ServerMessage = { type: 'hello', payload: { user: { id: 'u1', username: 'ana', displayName: 'Ana' }, protocolVersion: 1 } }

function setup(over: Partial<WsClientOptions> = {}) {
  FakeSocket.instances = []
  const received: ServerMessage[] = []
  const statuses: WsStatus[] = []
  const unacked: string[] = []
  const versions: number[] = []
  const client = new WsClient({
    url: 'ws://x/ws',
    onMessage: (m) => received.push(m),
    onStatus: (s) => statuses.push(s),
    onVersionMismatch: (v) => versions.push(v),
    onUnackedError: (code) => unacked.push(code),
    checkSession: async () => true,
    createSocket: (url) => new FakeSocket(url),
    setTimer: (fn, ms) => setTimeout(fn, ms),
    clearTimer: (h) => clearTimeout(h as ReturnType<typeof setTimeout>),
    random: () => 0.5,
    ...over,
  })
  return { client, received, statuses, unacked, versions }
}

beforeEach(() => vi.useFakeTimers())
afterEach(() => vi.useRealTimers())

it('monta a URL ws/wss a partir da página', () => {
  expect(wsUrl({ protocol: 'https:', host: 'truco.empresa.com' })).toBe('wss://truco.empresa.com/ws')
  expect(wsUrl({ protocol: 'http:', host: 'localhost:5173' })).toBe('ws://localhost:5173/ws')
})

it('roteia mensagens e reporta status', () => {
  const { client, received, statuses } = setup()
  client.connect()
  FakeSocket.last().open()
  FakeSocket.last().receive(hello)
  FakeSocket.last().receive('não é json')
  expect(received).toEqual([hello])
  expect(statuses).toEqual(['connecting', 'open'])
})

it('avisa quando a versão do protocolo difere', () => {
  const { client, versions } = setup()
  client.connect()
  FakeSocket.last().open()
  FakeSocket.last().receive({ ...hello, payload: { ...hello.payload, protocolVersion: 2 } })
  expect(versions).toEqual([2])
})

it('numera envios e rejeita o que voltar com error{seq}', async () => {
  const { client } = setup()
  client.connect()
  FakeSocket.last().open()
  FakeSocket.last().receive(hello)
  const first = client.send({ type: 'set_ready', payload: { ready: true } })
  const second = client.send({ type: 'leave_room' })
  expect(FakeSocket.last().sentMessages()).toEqual([
    { type: 'set_ready', seq: 1, payload: { ready: true } },
    { type: 'leave_room', seq: 2 },
  ])
  FakeSocket.last().receive({ type: 'error', payload: { code: 'not_host', message: 'x', seq: 1 } })
  await expect(first).rejects.toEqual(new WsSendError('not_host', 'x'))
  FakeSocket.last().receive({ type: 'left_room', payload: { reason: 'left' } })
  await expect(second).resolves.toBeUndefined()
})

it('resolve envios sem resposta após 5 s e manda erro sem seq conhecido para o toast', async () => {
  const { client, unacked } = setup()
  client.connect()
  FakeSocket.last().open()
  FakeSocket.last().receive(hello)
  const p = client.send({ type: 'signal', payload: { id: 'boa' } })
  vi.advanceTimersByTime(5000)
  await expect(p).resolves.toBeUndefined()
  FakeSocket.last().receive({ type: 'error', payload: { code: 'rate_limited', message: 'x', seq: 99 } })
  expect(unacked).toEqual(['rate_limited'])
})

it('rejeita envio com a conexão fechada', async () => {
  const { client } = setup()
  await expect(client.send({ type: 'leave_room' })).rejects.toMatchObject({ code: 'network' })
})

it('reconecta com backoff e zera as tentativas no hello', async () => {
  const { client, statuses } = setup()
  client.connect()
  FakeSocket.last().open()
  FakeSocket.last().receive(hello)
  FakeSocket.last().serverClose(1006)
  await vi.advanceTimersByTimeAsync(0)
  expect(statuses.at(-1)).toBe('reconnecting')
  expect(FakeSocket.instances).toHaveLength(1)
  await vi.advanceTimersByTimeAsync(backoffDelay(0, () => 0.5))
  expect(FakeSocket.instances).toHaveLength(2)
})

it('não reconecta quando substituída por outra aba (4001) e retoma com resume()', async () => {
  const { client, statuses } = setup()
  client.connect()
  FakeSocket.last().open()
  FakeSocket.last().receive(hello)
  FakeSocket.last().serverClose(4001)
  await vi.advanceTimersByTimeAsync(60_000)
  expect(statuses.at(-1)).toBe('replaced')
  expect(FakeSocket.instances).toHaveLength(1)
  client.resume()
  expect(FakeSocket.instances).toHaveLength(2)
})

it('sessão inválida (4003) vira unauthorized sem reconectar', async () => {
  const { client, statuses } = setup()
  client.connect()
  FakeSocket.last().open()
  FakeSocket.last().receive(hello)
  FakeSocket.last().serverClose(4003)
  await vi.advanceTimersByTimeAsync(60_000)
  expect(statuses.at(-1)).toBe('unauthorized')
  expect(FakeSocket.instances).toHaveLength(1)
})

it('para de reconectar e sinaliza unauthorized quando a sessão caiu antes do hello', async () => {
  const checkSession = vi.fn().mockResolvedValue(false)
  const { client, statuses } = setup({ checkSession })
  client.connect()
  FakeSocket.last().serverClose(1006) // upgrade recusado com 401: o navegador só vê 1006
  await vi.advanceTimersByTimeAsync(0)
  expect(checkSession).toHaveBeenCalledOnce()
  expect(statuses.at(-1)).toBe('unauthorized')
  await vi.advanceTimersByTimeAsync(60_000)
  expect(FakeSocket.instances).toHaveLength(1)
})

it('disconnect() fecha sem reconectar', async () => {
  const { client, statuses } = setup()
  client.connect()
  FakeSocket.last().open()
  client.disconnect()
  await vi.advanceTimersByTimeAsync(60_000)
  expect(statuses.at(-1)).toBe('idle')
  expect(FakeSocket.instances).toHaveLength(1)
})

it('backoff cresce até 10 s com jitter', () => {
  expect(backoffDelay(0, () => 1)).toBe(500)
  expect(backoffDelay(3, () => 1)).toBe(4000)
  expect(backoffDelay(10, () => 1)).toBe(10_000)
  expect(backoffDelay(10, () => 0)).toBe(5000)
})
```

- [ ] **Step 3: Rodar e ver falhar**

Run: `cd frontend && pnpm vitest run src/ws`
Expected: FAIL — módulo inexistente.

- [ ] **Step 4: Implementar**

`frontend/src/ws/client.ts`:

```ts
import type { ClientMessage, ServerMessage } from '@/protocol/union'

/** Versão do protocolo que este build entende (backend: protocolVersion no hello). */
export const EXPECTED_PROTOCOL_VERSION = 1

const CLOSE_REPLACED = 4001
const CLOSE_SESSION_INVALID = 4003
const ACK_TIMEOUT_MS = 5000
const OPEN = 1

export type WsStatus = 'idle' | 'connecting' | 'open' | 'reconnecting' | 'replaced' | 'unauthorized'

type WithoutSeq<M> = M extends unknown ? Omit<M, 'seq'> : never
export type Outgoing = WithoutSeq<ClientMessage>

export interface SocketLike {
  readyState: number
  send(data: string): void
  close(code?: number, reason?: string): void
  onopen: ((ev: unknown) => void) | null
  onmessage: ((ev: { data: unknown }) => void) | null
  onclose: ((ev: { code: number; reason: string }) => void) | null
  onerror: ((ev: unknown) => void) | null
}

export interface WsClientOptions {
  url: string
  onMessage(msg: ServerMessage): void
  onStatus(status: WsStatus): void
  onVersionMismatch(serverVersion: number): void
  onUnackedError(code: string, message: string): void
  checkSession(): Promise<boolean>
  createSocket?(url: string): SocketLike
  setTimer?(fn: () => void, ms: number): unknown
  clearTimer?(handle: unknown): void
  random?(): number
}

export class WsSendError extends Error {
  constructor(
    readonly code: string,
    message: string,
  ) {
    super(message)
    this.name = 'WsSendError'
  }
}

export function backoffDelay(attempt: number, random: () => number): number {
  const base = Math.min(10_000, 500 * 2 ** attempt)
  return base * (0.5 + random() / 2)
}

export function wsUrl(loc: { protocol: string; host: string }): string {
  return `${loc.protocol === 'https:' ? 'wss' : 'ws'}://${loc.host}/ws`
}

interface Pending {
  resolve(): void
  reject(err: WsSendError): void
  timer: unknown
}

export class WsClient {
  private socket: SocketLike | null = null
  private current: WsStatus = 'idle'
  private seq = 0
  private attempt = 0
  private gotHello = false
  private wanted = false
  private reconnectTimer: unknown = null
  private pending = new Map<number, Pending>()

  constructor(private readonly opts: WsClientOptions) {}

  get status(): WsStatus {
    return this.current
  }

  connect(): void {
    this.wanted = true
    this.open()
  }

  /** Retoma depois de 'replaced' (o usuário escolheu usar esta aba). */
  resume(): void {
    this.attempt = 0
    this.connect()
  }

  disconnect(): void {
    this.wanted = false
    this.cancelReconnect()
    const s = this.socket
    this.socket = null
    if (s) {
      s.onclose = null
      s.close(1000, 'cliente saiu')
    }
    this.failPending('network', 'Conexão encerrada.')
    this.setStatus('idle')
  }

  send(msg: Outgoing): Promise<void> {
    const s = this.socket
    if (!s || s.readyState !== OPEN || !this.gotHello) {
      return Promise.reject(new WsSendError('network', 'Sem conexão com o servidor.'))
    }
    const seq = ++this.seq
    s.send(JSON.stringify({ type: msg.type, seq, ...('payload' in msg ? { payload: msg.payload } : {}) }))
    return new Promise<void>((resolve, reject) => {
      const timer = this.timer(() => {
        this.pending.delete(seq)
        resolve()
      }, ACK_TIMEOUT_MS)
      this.pending.set(seq, { resolve, reject, timer })
    })
  }

  private open(): void {
    this.cancelReconnect()
    this.gotHello = false
    this.setStatus(this.attempt === 0 ? 'connecting' : 'reconnecting')
    const s = (this.opts.createSocket ?? ((url) => new WebSocket(url) as unknown as SocketLike))(this.opts.url)
    this.socket = s
    s.onmessage = (ev) => this.handleData(ev.data)
    s.onclose = (ev) => void this.handleClose(s, ev.code)
    s.onerror = () => {
      // o onclose vem logo depois e decide o que fazer
    }
  }

  private handleData(data: unknown): void {
    if (typeof data !== 'string') return
    let msg: ServerMessage
    try {
      msg = JSON.parse(data) as ServerMessage
    } catch {
      console.warn('[ws] mensagem não-JSON descartada')
      return
    }
    if (typeof msg !== 'object' || msg === null || typeof msg.type !== 'string') return
    if (msg.type === 'hello') {
      this.gotHello = true
      this.attempt = 0
      this.setStatus('open')
      if (msg.payload.protocolVersion !== EXPECTED_PROTOCOL_VERSION) this.opts.onVersionMismatch(msg.payload.protocolVersion)
    }
    if (msg.type === 'error') {
      const seq = msg.payload.seq
      const p = seq === undefined ? undefined : this.pending.get(seq)
      if (p && seq !== undefined) {
        this.pending.delete(seq)
        this.clear(p.timer)
        p.reject(new WsSendError(msg.payload.code, msg.payload.message))
      } else {
        this.opts.onUnackedError(msg.payload.code, msg.payload.message)
      }
    } else {
      this.resolvePending()
    }
    this.opts.onMessage(msg)
  }

  private async handleClose(s: SocketLike, code: number): Promise<void> {
    if (s !== this.socket) return
    this.socket = null
    const hadHello = this.gotHello
    this.gotHello = false
    this.failPending('network', 'Conexão perdida.')
    if (!this.wanted) return
    if (code === CLOSE_REPLACED) {
      this.wanted = false
      this.setStatus('replaced')
      return
    }
    if (code === CLOSE_SESSION_INVALID || (!hadHello && !(await this.opts.checkSession()))) {
      this.wanted = false
      this.setStatus('unauthorized')
      return
    }
    if (!this.wanted) return
    this.setStatus('reconnecting')
    const delay = backoffDelay(this.attempt, this.opts.random ?? Math.random)
    this.attempt++
    this.reconnectTimer = this.timer(() => {
      this.reconnectTimer = null
      if (this.wanted) this.open()
    }, delay)
  }

  private resolvePending(): void {
    for (const [seq, p] of this.pending) {
      this.clear(p.timer)
      p.resolve()
      this.pending.delete(seq)
    }
  }

  private failPending(code: string, message: string): void {
    for (const [seq, p] of this.pending) {
      this.clear(p.timer)
      p.reject(new WsSendError(code, message))
      this.pending.delete(seq)
    }
  }

  private setStatus(s: WsStatus): void {
    if (this.current === s) return
    this.current = s
    this.opts.onStatus(s)
  }

  private cancelReconnect(): void {
    if (this.reconnectTimer !== null) this.clear(this.reconnectTimer)
    this.reconnectTimer = null
  }

  private timer(fn: () => void, ms: number): unknown {
    return (this.opts.setTimer ?? ((f, t) => setTimeout(f, t)))(fn, ms)
  }

  private clear(h: unknown): void {
    ;(this.opts.clearTimer ?? ((x) => clearTimeout(x as ReturnType<typeof setTimeout>)))(h)
  }
}
```

Observação: o teste "reconecta com backoff" espera `status = 'reconnecting'` e **nenhum** socket novo até o atraso passar; o teste "sessão caiu antes do hello" exige que `checkSession` só seja chamado quando não houve `hello` (com `hello`, uma queda 1006 é rede e reconecta direto).

- [ ] **Step 5: Rodar e ver passar**

Run: `cd frontend && pnpm vitest run src/ws && pnpm typecheck`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/ws frontend/src/test/fakeSocket.ts
git commit -m "feat(frontend): cliente WebSocket com seq, reconexão e códigos de fechamento"
```

---
### Task 7: Preferências, toasts, sessão e ligação WS → stores

**Skills:** `frontend-dev-guidelines` (padrão de stores), `typescript-expert` (switch exaustivo).

**Files:**
- Create: `frontend/src/stores/prefs.ts`, `frontend/src/stores/prefs.test.ts`
- Create: `frontend/src/stores/toasts.ts`, `frontend/src/stores/toasts.test.ts`
- Create: `frontend/src/stores/session.ts`, `frontend/src/stores/session.test.ts`
- Create: `frontend/src/app/transport.ts`
- Create: `frontend/src/app/wire.ts`, `frontend/src/app/wire.test.ts`

**Interfaces:**
- Consumes: `authApi`, `User`, `ApiError` (Task 4); `WsClient`, `WsStatus`, `Outgoing`, `WsSendError`, `wsUrl` (Task 6); `errorText` (Task 5); `DeckStyle` (Task 3).
- Produces:
  - `usePrefsStore()`: state `deck: DeckStyle`, `volume: number`, `muted: boolean`; actions `setDeck(d)`, `setVolume(v)`, `setMuted(m)`; persiste em `localStorage['truco.prefs']`.
  - `useToastsStore()`: state `items: { id: number; text: string; kind: 'error' | 'info' }[]`; actions `push(text, kind?)`, `error(code, serverMessage?)`, `dismiss(id)`; cada toast some sozinho em 5 s.
  - `useSessionStore()`: state `user: User | null`, `loaded: boolean`, `connection: WsStatus`, `versionMismatch: boolean`; actions `load()`, `login(login, password)`, `logout()`, `logoutAll()`, `updateDisplayName(name)`, `changePassword(current, next)`, `clear()`, `setConnection(s)`.
  - `app/transport.ts`: `type Sender = (m: Outgoing) => Promise<void>`; `setSender(s: Sender | null): void`; `send(m: Outgoing): Promise<void>`.
  - `app/wire.ts`: `dispatch(msg: ServerMessage): void`; `installConnection(router: Router): void`; `resumeConnection(): void`.
  - Tarefa 8 adiciona `useLobbyStore().onSnapshot/onUpsert/onRemove` e `useRoomStore().onRoomState/onLeft/onForfeit/onMatchEnded`; Tarefa 10 adiciona `useGameStore().onState/onEvents/onSignal`. Nesta tarefa o `dispatch` já chama esses nomes, e os stubs mínimos dessas stores são criados aqui (ver Step 5) para compilar.

- [ ] **Step 1: Testes de prefs e toasts**

`frontend/src/stores/prefs.test.ts`:

```ts
import { createPinia, setActivePinia } from 'pinia'
import { usePrefsStore } from './prefs'

beforeEach(() => setActivePinia(createPinia()))

it('usa padrões e persiste mudanças', () => {
  const prefs = usePrefsStore()
  expect(prefs.deck).toBe('frances')
  expect(prefs.volume).toBe(0.7)
  expect(prefs.muted).toBe(false)
  prefs.setDeck('espanhol')
  prefs.setVolume(2)
  expect(prefs.volume).toBe(1)
  expect(JSON.parse(localStorage.getItem('truco.prefs')!)).toEqual({ deck: 'espanhol', volume: 1, muted: false })
})

it('lê valores salvos e ignora lixo', () => {
  localStorage.setItem('truco.prefs', JSON.stringify({ deck: 'espanhol', volume: 'x', muted: true }))
  const prefs = usePrefsStore()
  expect(prefs.deck).toBe('espanhol')
  expect(prefs.volume).toBe(0.7)
  expect(prefs.muted).toBe(true)
})

it('funciona sem localStorage', () => {
  vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
    throw new Error('bloqueado')
  })
  vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
    throw new Error('bloqueado')
  })
  const prefs = usePrefsStore()
  expect(prefs.deck).toBe('frances')
  expect(() => prefs.setMuted(true)).not.toThrow()
  expect(prefs.muted).toBe(true)
})
```

`frontend/src/stores/toasts.test.ts`:

```ts
import { createPinia, setActivePinia } from 'pinia'
import { useToastsStore } from './toasts'

beforeEach(() => {
  setActivePinia(createPinia())
  vi.useFakeTimers()
})
afterEach(() => vi.useRealTimers())

it('traduz o código e some sozinho em 5 s', () => {
  const toasts = useToastsStore()
  toasts.error('room_full')
  expect(toasts.items.map((t) => t.text)).toEqual(['A sala está cheia.'])
  vi.advanceTimersByTime(5000)
  expect(toasts.items).toEqual([])
})
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd frontend && pnpm vitest run src/stores`
Expected: FAIL — módulos inexistentes.

- [ ] **Step 3: Implementar prefs e toasts**

`frontend/src/stores/prefs.ts`:

```ts
import { defineStore } from 'pinia'
import type { DeckStyle } from '@/game/cards'

const KEY = 'truco.prefs'

interface Prefs {
  deck: DeckStyle
  volume: number
  muted: boolean
}

const DEFAULTS: Prefs = { deck: 'frances', volume: 0.7, muted: false }

function read(): Prefs {
  try {
    const raw = localStorage.getItem(KEY)
    if (!raw) return { ...DEFAULTS }
    const data = JSON.parse(raw) as Partial<Record<keyof Prefs, unknown>>
    return {
      deck: data.deck === 'espanhol' || data.deck === 'frances' ? data.deck : DEFAULTS.deck,
      volume: typeof data.volume === 'number' && data.volume >= 0 && data.volume <= 1 ? data.volume : DEFAULTS.volume,
      muted: typeof data.muted === 'boolean' ? data.muted : DEFAULTS.muted,
    }
  } catch {
    return { ...DEFAULTS }
  }
}

export const usePrefsStore = defineStore('prefs', {
  state: (): Prefs => read(),
  actions: {
    setDeck(deck: DeckStyle) {
      this.deck = deck
      this.save()
    },
    setVolume(volume: number) {
      this.volume = Math.min(1, Math.max(0, volume))
      this.save()
    },
    setMuted(muted: boolean) {
      this.muted = muted
      this.save()
    },
    save() {
      try {
        localStorage.setItem(KEY, JSON.stringify({ deck: this.deck, volume: this.volume, muted: this.muted }))
      } catch {
        // storage indisponível: a preferência vale só nesta aba
      }
    },
  },
})
```

`frontend/src/stores/toasts.ts`:

```ts
import { defineStore } from 'pinia'
import { errorText } from '@/game/labels'

export interface Toast {
  id: number
  text: string
  kind: 'error' | 'info'
}

const TTL_MS = 5000
let nextId = 1

export const useToastsStore = defineStore('toasts', {
  state: () => ({ items: [] as Toast[] }),
  actions: {
    push(text: string, kind: Toast['kind'] = 'info') {
      const id = nextId++
      this.items.push({ id, text, kind })
      setTimeout(() => this.dismiss(id), TTL_MS)
    },
    error(code: string, serverMessage?: string) {
      this.push(errorText(code, serverMessage), 'error')
    },
    dismiss(id: number) {
      this.items = this.items.filter((t) => t.id !== id)
    },
  },
})
```

- [ ] **Step 4: Testes de sessão e do `dispatch`**

`frontend/src/stores/session.test.ts`:

```ts
import { createPinia, setActivePinia } from 'pinia'
import { useSessionStore } from './session'
import { authApi } from '@/api/auth'
import { ApiError } from '@/api/http'

const ana = { id: 'u1', username: 'ana', email: 'ana@empresa.com.br', displayName: 'Ana', isAdmin: false, createdAt: '' }

beforeEach(() => setActivePinia(createPinia()))

it('load() guarda o usuário ou null em 401', async () => {
  vi.spyOn(authApi, 'me').mockResolvedValueOnce(ana)
  const s = useSessionStore()
  await s.load()
  expect(s.user).toEqual(ana)
  expect(s.loaded).toBe(true)

  vi.spyOn(authApi, 'me').mockRejectedValueOnce(new ApiError(401, 'unauthenticated', 'x'))
  await s.load()
  expect(s.user).toBeNull()
})

it('load() propaga erro que não é 401', async () => {
  vi.spyOn(authApi, 'me').mockRejectedValueOnce(new ApiError(0, 'network', 'x'))
  await expect(useSessionStore().load()).rejects.toMatchObject({ code: 'network' })
})

it('logout limpa o usuário mesmo se a API falhar', async () => {
  vi.spyOn(authApi, 'logout').mockRejectedValueOnce(new ApiError(0, 'network', 'x'))
  const s = useSessionStore()
  s.user = ana
  await s.logout()
  expect(s.user).toBeNull()
})
```

`frontend/src/app/wire.test.ts`:

```ts
import { createPinia, setActivePinia } from 'pinia'
import { dispatch } from './wire'
import { useSessionStore } from '@/stores/session'
import { useLobbyStore } from '@/stores/lobby'
import { useRoomStore } from '@/stores/room'
import { useGameStore } from '@/stores/game'

beforeEach(() => setActivePinia(createPinia()))

it('entrega cada tipo à store dona', () => {
  const lobby = useLobbyStore()
  const room = useRoomStore()
  const game = useGameStore()
  const spies = {
    snapshot: vi.spyOn(lobby, 'onSnapshot'),
    upsert: vi.spyOn(lobby, 'onUpsert'),
    removed: vi.spyOn(lobby, 'onRemove'),
    roomState: vi.spyOn(room, 'onRoomState'),
    left: vi.spyOn(room, 'onLeft'),
    forfeit: vi.spyOn(room, 'onForfeit'),
    ended: vi.spyOn(room, 'onMatchEnded'),
    state: vi.spyOn(game, 'onState'),
    events: vi.spyOn(game, 'onEvents'),
    signal: vi.spyOn(game, 'onSignal'),
  }
  dispatch({ type: 'lobby_snapshot', payload: { rooms: [] } })
  dispatch({ type: 'room_upserted', payload: { room: { id: 'r', name: 'n', status: 'waiting', seats: [null, null, null, null] } } })
  dispatch({ type: 'room_removed', payload: { roomId: 'r' } })
  dispatch({ type: 'left_room', payload: { reason: 'left' } })
  dispatch({ type: 'forfeit_available', payload: { team: 1 } })
  dispatch({ type: 'match_ended', payload: { reason: 'game_over', winner: 0, score: [12, 3] } })
  dispatch({ type: 'events', payload: [] })
  dispatch({ type: 'signal', payload: { seat: 1, id: 'boa' } })
  for (const spy of Object.values(spies).filter((s) => s !== spies.roomState && s !== spies.state)) {
    expect(spy).toHaveBeenCalledOnce()
  }
})

it('hello guarda o usuário da conexão', () => {
  dispatch({ type: 'hello', payload: { user: { id: 'u1', username: 'ana', displayName: 'Ana' }, protocolVersion: 1 } })
  expect(useSessionStore().helloUser?.id).toBe('u1')
})
```

(Adicione a `useSessionStore` o campo `helloUser: UserInfo | null` — o `UserInfo` do `hello`, usado para conferir que a conexão é do mesmo usuário da sessão HTTP.)

- [ ] **Step 5: Implementar sessão, transporte e ligação**

`frontend/src/app/transport.ts`:

```ts
import { WsSendError, type Outgoing } from '@/ws/client'

export type Sender = (m: Outgoing) => Promise<void>

let sender: Sender | null = null

export function setSender(s: Sender | null): void {
  sender = s
}

export function send(m: Outgoing): Promise<void> {
  if (!sender) return Promise.reject(new WsSendError('network', 'Sem conexão com o servidor.'))
  return sender(m)
}
```

`frontend/src/stores/session.ts`:

```ts
import { defineStore } from 'pinia'
import { authApi, type User } from '@/api/auth'
import { ApiError } from '@/api/http'
import type { WsStatus } from '@/ws/client'
import type { UserInfo } from '@/protocol/messages'

export const useSessionStore = defineStore('session', {
  state: () => ({
    user: null as User | null,
    loaded: false,
    connection: 'idle' as WsStatus,
    versionMismatch: false,
    helloUser: null as UserInfo | null,
  }),
  getters: {
    isLoggedIn: (s) => s.user !== null,
  },
  actions: {
    async load() {
      try {
        this.user = await authApi.me()
      } catch (err) {
        if (err instanceof ApiError && err.status === 401) this.user = null
        else throw err
      } finally {
        this.loaded = true
      }
    },
    async login(login: string, password: string) {
      this.user = await authApi.login(login, password)
      this.loaded = true
    },
    async logout() {
      try {
        await authApi.logout()
      } finally {
        this.clear()
      }
    },
    async logoutAll() {
      await authApi.logoutAll()
      this.clear()
    },
    async updateDisplayName(name: string) {
      this.user = await authApi.updateMe(name)
    },
    async changePassword(current: string, next: string) {
      await authApi.changePassword(current, next)
    },
    clear() {
      this.user = null
      this.helloUser = null
    },
    setConnection(s: WsStatus) {
      this.connection = s
    },
  },
})
```

`frontend/src/app/wire.ts`:

```ts
import { watch } from 'vue'
import type { Router } from 'vue-router'
import type { ServerMessage } from '@/protocol/union'
import { WsClient, wsUrl } from '@/ws/client'
import { authApi } from '@/api/auth'
import { setSender } from './transport'
import { useSessionStore } from '@/stores/session'
import { useLobbyStore } from '@/stores/lobby'
import { useRoomStore } from '@/stores/room'
import { useGameStore } from '@/stores/game'
import { useToastsStore } from '@/stores/toasts'

let client: WsClient | null = null

export function dispatch(msg: ServerMessage): void {
  switch (msg.type) {
    case 'hello':
      useSessionStore().helloUser = msg.payload.user
      return
    case 'lobby_snapshot':
      return useLobbyStore().onSnapshot(msg.payload)
    case 'room_upserted':
      return useLobbyStore().onUpsert(msg.payload)
    case 'room_removed':
      return useLobbyStore().onRemove(msg.payload)
    case 'room_state':
      return useRoomStore().onRoomState(msg.payload)
    case 'left_room':
      return useRoomStore().onLeft(msg.payload)
    case 'forfeit_available':
      return useRoomStore().onForfeit(msg.payload)
    case 'match_ended':
      return useRoomStore().onMatchEnded(msg.payload)
    case 'state':
      return useGameStore().onState(msg.payload)
    case 'events':
      return useGameStore().onEvents(msg.payload)
    case 'signal':
      return useGameStore().onSignal(msg.payload)
    case 'error':
      return // tratado no WsClient (rejeição do send ou onUnackedError)
    default: {
      const unreachable: never = msg
      console.warn('[ws] tipo desconhecido', unreachable)
    }
  }
}

function start(): void {
  if (client) return
  const session = useSessionStore()
  const toasts = useToastsStore()
  client = new WsClient({
    url: wsUrl(location),
    onMessage: dispatch,
    onStatus: (s) => session.setConnection(s),
    onVersionMismatch: () => {
      session.versionMismatch = true
    },
    onUnackedError: (code, message) => toasts.error(code, message),
    checkSession: async () => {
      try {
        await authApi.me()
        return true
      } catch {
        return false
      }
    },
  })
  setSender((m) => client!.send(m))
  client.connect()
}

function stop(): void {
  client?.disconnect()
  client = null
  setSender(null)
  useLobbyStore().$reset()
  useRoomStore().$reset()
  useGameStore().reset()
}

export function resumeConnection(): void {
  client?.resume()
}

/** Liga a conexão ao login: abre com usuário, fecha sem; sessão inválida volta ao login. */
export function installConnection(router: Router): void {
  const session = useSessionStore()
  watch(
    () => session.user?.id ?? null,
    (id) => (id ? start() : stop()),
    { immediate: true },
  )
  watch(
    () => session.connection,
    (status) => {
      if (status !== 'unauthorized') return
      session.clear()
      void router.push({ name: 'entrar', query: { volta: router.currentRoute.value.fullPath } })
    },
  )
}
```

`$reset()` exige option stores (as stores de lobby e room usam `defineStore(id, { state, ... })`).

Crie os **stubs** que as Tarefas 8 e 10 vão completar, para esta tarefa compilar:

`frontend/src/stores/lobby.ts`:

```ts
import { defineStore } from 'pinia'
import type { LobbySnapshotPayload, RoomRemovedPayload, RoomUpsertedPayload } from '@/protocol/messages'

export const useLobbyStore = defineStore('lobby', {
  state: () => ({}),
  actions: {
    onSnapshot(_p: LobbySnapshotPayload) {},
    onUpsert(_p: RoomUpsertedPayload) {},
    onRemove(_p: RoomRemovedPayload) {},
  },
})
```

`frontend/src/stores/room.ts`:

```ts
import { defineStore } from 'pinia'
import type { ForfeitAvailablePayload, LeftRoomPayload, MatchEndedPayload, RoomStatePayload } from '@/protocol/messages'

export const useRoomStore = defineStore('room', {
  state: () => ({}),
  actions: {
    onRoomState(_p: RoomStatePayload) {},
    onLeft(_p: LeftRoomPayload) {},
    onForfeit(_p: ForfeitAvailablePayload) {},
    onMatchEnded(_p: MatchEndedPayload) {},
  },
})
```

`frontend/src/stores/game.ts`:

```ts
import { defineStore } from 'pinia'
import type { Event } from '@/protocol/engine'
import type { SignalBroadcastPayload, StatePayload } from '@/protocol/messages'

export const useGameStore = defineStore('game', {
  state: () => ({}),
  actions: {
    onState(_p: StatePayload) {},
    onEvents(_p: Event[]) {},
    onSignal(_p: SignalBroadcastPayload) {},
    reset() {},
  },
})
```

(Prefixos `_` evitam o aviso de parâmetro não usado; se o ESLint reclamar, configure `@typescript-eslint/no-unused-vars` com `argsIgnorePattern: '^_'` no `eslint.config.ts`.)

- [ ] **Step 6: Rodar e ver passar**

Run: `cd frontend && pnpm vitest run src/stores src/app && pnpm typecheck && pnpm lint`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/stores frontend/src/app frontend/eslint.config.ts
git commit -m "feat(frontend): stores de preferências, toasts e sessão e roteamento WS → stores"
```

---

### Task 8: Stores de lobby e sala

**Skills:** `frontend-dev-guidelines`.

**Files:**
- Modify: `frontend/src/stores/lobby.ts` (substitui o stub)
- Modify: `frontend/src/stores/room.ts` (substitui o stub)
- Create: `frontend/src/stores/lobby.test.ts`, `frontend/src/stores/room.test.ts`

**Interfaces:**
- Consumes: `send` (Task 7, `@/app/transport`), `useToastsStore`, `useGameStore().reset()` (stub da Task 7, real na Task 10), `teamOf` (Task 3).
- Produces:
  - `useLobbyStore()`: state `rooms: Record<string, RoomSummary>`, `loaded: boolean`; getter `list: RoomSummary[]` (ordem: `waiting` com assento livre primeiro, depois por nome); actions `onSnapshot`, `onUpsert`, `onRemove`, `createRoom(visibility: Visibility): Promise<void>`, `joinById(roomId: string, seat?: Seat)`, `joinByCode(code: string, seat?: Seat)`; `normalizeCode(raw: string): string` exportado (trim + maiúsculas + remove espaços).
  - `useRoomStore()`: state `room: RoomStatePayload | null`, `forfeitTeam: Team | null`, `ended: MatchEndedPayload | null`, `lastLeft: LeftReason | null`; getters `inRoom`, `isHost`, `mySeat`, `myTeam`, `isPlaying`, `amReady`; actions `onRoomState`, `onLeft`, `onForfeit`, `onMatchEnded`, `takeSeat(seat)`, `setReady(ready)`, `kick(seat)`, `leave()`, `claimForfeit()`, `dismissEnded()`.

- [ ] **Step 1: Testes que falham**

`frontend/src/stores/lobby.test.ts`:

```ts
import { createPinia, setActivePinia } from 'pinia'
import { normalizeCode, useLobbyStore } from './lobby'
import { setSender } from '@/app/transport'
import type { RoomSummary } from '@/protocol/messages'

const room = (id: string, over: Partial<RoomSummary> = {}): RoomSummary => ({
  id,
  name: `Sala ${id}`,
  status: 'waiting',
  seats: [{ displayName: 'Ana' }, null, null, null],
  ...over,
})

beforeEach(() => setActivePinia(createPinia()))
afterEach(() => setSender(null))

it('aplica snapshot, upsert e remove', () => {
  const lobby = useLobbyStore()
  lobby.onSnapshot({ rooms: [room('a'), room('b')] })
  lobby.onUpsert({ room: room('b', { status: 'playing' }) })
  lobby.onUpsert({ room: room('c') })
  lobby.onRemove({ roomId: 'a' })
  expect(lobby.loaded).toBe(true)
  expect(lobby.list.map((r) => r.id)).toEqual(['c', 'b'])
})

it('normaliza código digitado', () => {
  expect(normalizeCode(' k7q m2p ')).toBe('K7QM2P')
})

it('envia create e join', async () => {
  const send = vi.fn().mockResolvedValue(undefined)
  setSender(send)
  const lobby = useLobbyStore()
  await lobby.createRoom('private')
  await lobby.joinById('r1', 2)
  await lobby.joinByCode(' abc234 ')
  expect(send.mock.calls.map((c) => c[0])).toEqual([
    { type: 'create_room', payload: { visibility: 'private' } },
    { type: 'join_room', payload: { roomId: 'r1', seat: 2 } },
    { type: 'join_room', payload: { code: 'ABC234' } },
  ])
})
```

`frontend/src/stores/room.test.ts`:

```ts
import { createPinia, setActivePinia } from 'pinia'
import { useRoomStore } from './room'
import { useGameStore } from './game'
import { setSender } from '@/app/transport'
import type { RoomStatePayload } from '@/protocol/messages'

afterEach(() => setSender(null))

const state = (over: Partial<RoomStatePayload> = {}): RoomStatePayload => ({
  id: 'r1',
  code: 'ABC234',
  name: 'Sala da Ana',
  visibility: 'private',
  host: 0,
  status: 'waiting',
  seats: [
    { userId: 'u1', displayName: 'Ana', ready: true, connected: true, absent: false },
    null,
    { userId: 'u3', displayName: 'Caio', ready: false, connected: true, absent: false },
    null,
  ],
  yourSeat: 0,
  ...over,
})

beforeEach(() => setActivePinia(createPinia()))

it('deriva host, assento, dupla e pronto', () => {
  const r = useRoomStore()
  r.onRoomState(state())
  expect([r.inRoom, r.isHost, r.mySeat, r.myTeam, r.amReady, r.isPlaying]).toEqual([true, true, 0, 0, true, false])
})

it('volta a waiting limpa o jogo; left_room limpa tudo', () => {
  const r = useRoomStore()
  const game = useGameStore()
  const reset = vi.spyOn(game, 'reset')
  r.onRoomState(state({ status: 'playing', matchId: 'm1' }))
  r.onForfeit({ team: 1 })
  r.onRoomState(state({ status: 'waiting' }))
  expect(reset).toHaveBeenCalledOnce()
  expect(r.forfeitTeam).toBeNull()
  r.onLeft({ reason: 'kicked' })
  expect(r.room).toBeNull()
  expect(r.lastLeft).toBe('kicked')
  expect(reset).toHaveBeenCalledTimes(2)
})

it('guarda e descarta o fim de partida', () => {
  const r = useRoomStore()
  r.onMatchEnded({ reason: 'forfeit', winner: 0, score: [12, 4] })
  expect(r.ended?.reason).toBe('forfeit')
  r.dismissEnded()
  expect(r.ended).toBeNull()
})

it('envia as ações de sala', async () => {
  const send = vi.fn().mockResolvedValue(undefined)
  setSender(send)
  const r = useRoomStore()
  await r.takeSeat(1)
  await r.setReady(true)
  await r.kick(2)
  await r.claimForfeit()
  await r.leave()
  expect(send.mock.calls.map((c) => c[0])).toEqual([
    { type: 'take_seat', payload: { seat: 1 } },
    { type: 'set_ready', payload: { ready: true } },
    { type: 'kick', payload: { seat: 2 } },
    { type: 'claim_forfeit' },
    { type: 'leave_room' },
  ])
})
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd frontend && pnpm vitest run src/stores/lobby.test.ts src/stores/room.test.ts`
Expected: FAIL — ações/getters inexistentes no stub.

- [ ] **Step 3: Implementar**

`frontend/src/stores/lobby.ts`:

```ts
import { defineStore } from 'pinia'
import type { Seat } from '@/protocol/engine'
import type {
  LobbySnapshotPayload,
  RoomRemovedPayload,
  RoomSummary,
  RoomUpsertedPayload,
  Visibility,
} from '@/protocol/messages'
import { send } from '@/app/transport'

export function normalizeCode(raw: string): string {
  return raw.replace(/\s+/g, '').toUpperCase()
}

function hasFreeSeat(r: RoomSummary): boolean {
  return r.seats.some((s) => s === null)
}

export const useLobbyStore = defineStore('lobby', {
  state: () => ({ rooms: {} as Record<string, RoomSummary>, loaded: false }),
  getters: {
    list(state): RoomSummary[] {
      const rank = (r: RoomSummary) => (r.status === 'waiting' && hasFreeSeat(r) ? 0 : 1)
      return Object.values(state.rooms).sort((a, b) => rank(a) - rank(b) || a.name.localeCompare(b.name, 'pt-BR'))
    },
  },
  actions: {
    onSnapshot(p: LobbySnapshotPayload) {
      this.rooms = Object.fromEntries(p.rooms.map((r) => [r.id, r]))
      this.loaded = true
    },
    onUpsert(p: RoomUpsertedPayload) {
      this.rooms[p.room.id] = p.room
    },
    onRemove(p: RoomRemovedPayload) {
      delete this.rooms[p.roomId]
    },
    createRoom(visibility: Visibility) {
      return send({ type: 'create_room', payload: { visibility } })
    },
    joinById(roomId: string, seat?: Seat) {
      return send({ type: 'join_room', payload: seat === undefined ? { roomId } : { roomId, seat } })
    },
    joinByCode(code: string, seat?: Seat) {
      const normalized = normalizeCode(code)
      return send({ type: 'join_room', payload: seat === undefined ? { code: normalized } : { code: normalized, seat } })
    },
  },
})
```

`frontend/src/stores/room.ts`:

```ts
import { defineStore } from 'pinia'
import type { Seat, Team } from '@/protocol/engine'
import type {
  ForfeitAvailablePayload,
  LeftReason,
  LeftRoomPayload,
  MatchEndedPayload,
  RoomStatePayload,
} from '@/protocol/messages'
import { send } from '@/app/transport'
import { teamOf } from '@/game/seats'
import { useGameStore } from './game'

export const useRoomStore = defineStore('room', {
  state: () => ({
    room: null as RoomStatePayload | null,
    forfeitTeam: null as Team | null,
    ended: null as MatchEndedPayload | null,
    lastLeft: null as LeftReason | null,
  }),
  getters: {
    inRoom: (s) => s.room !== null,
    mySeat: (s): Seat | null => s.room?.yourSeat ?? null,
    myTeam(): Team | null {
      return this.mySeat === null ? null : teamOf(this.mySeat)
    },
    isHost: (s) => s.room !== null && s.room.host === s.room.yourSeat,
    isPlaying: (s) => s.room?.status === 'playing',
    amReady: (s) => (s.room ? (s.room.seats[s.room.yourSeat]?.ready ?? false) : false),
  },
  actions: {
    onRoomState(p: RoomStatePayload) {
      const wasPlaying = this.room?.status === 'playing'
      this.room = p
      this.lastLeft = null
      if (wasPlaying && p.status === 'waiting') {
        this.forfeitTeam = null
        useGameStore().reset()
      }
    },
    onLeft(p: LeftRoomPayload) {
      this.room = null
      this.forfeitTeam = null
      this.lastLeft = p.reason
      useGameStore().reset()
    },
    onForfeit(p: ForfeitAvailablePayload) {
      this.forfeitTeam = p.team
    },
    onMatchEnded(p: MatchEndedPayload) {
      this.ended = p
      this.forfeitTeam = null
    },
    dismissEnded() {
      this.ended = null
    },
    takeSeat(seat: Seat) {
      return send({ type: 'take_seat', payload: { seat } })
    },
    setReady(ready: boolean) {
      return send({ type: 'set_ready', payload: { ready } })
    },
    kick(seat: Seat) {
      return send({ type: 'kick', payload: { seat } })
    },
    leave() {
      return send({ type: 'leave_room' })
    },
    claimForfeit() {
      return send({ type: 'claim_forfeit' })
    },
  },
})
```

A navegação (`left_room` → `/`, `room_state` fora de `/sala` → `/sala`) e o toast de "Você foi removido da sala" ficam no roteador (Tarefa 13), que observa `room.inRoom` e `room.lastLeft`; a store não conhece o router.

- [ ] **Step 4: Rodar e ver passar**

Run: `cd frontend && pnpm vitest run src/stores src/app && pnpm typecheck`
Expected: PASS (o teste de `wire` continua verde).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/stores/lobby.ts frontend/src/stores/lobby.test.ts frontend/src/stores/room.ts frontend/src/stores/room.test.ts
git commit -m "feat(frontend): stores de lobby ao vivo e de sala com W.O. e fim de partida"
```

---

### Task 9: Diretor de eventos

**Skills:** `typescript-expert`.

**Files:**
- Create: `frontend/src/game/director.ts`, `frontend/src/game/director.test.ts`

**Interfaces:**
- Consumes: `Event`, `EventKind` (`@/protocol/engine`), `StatePayload` (`@/protocol/messages`).
- Produces:
  - `const EVENT_DURATION_MS: Record<EventKind, number>` (valores da spec §6.2)
  - `const MAX_LAG_MS = 3000`
  - `interface DirectorOptions { onEvent(ev: Event, durationMs: number): void; onApply(state: StatePayload): void; onIdle?(): void; reducedMotion(): boolean; hidden(): boolean; setTimer(fn: () => void, ms: number): unknown; clearTimer(h: unknown): void }`
  - `class Director { constructor(o: DirectorOptions); pushEvents(evs: Event[]): void; pushState(s: StatePayload): void; flush(): void; reset(): void; get busy(): boolean }`
  - Semântica: `pushState` com fila vazia e nada tocando aplica na hora; senão vira pendente (o último vence). Ao terminar a fila, aplica o pendente e chama `onIdle`. `pushEvents` descarta tudo e aplica direto quando `hidden()` ou quando a soma das durações na fila passar de `MAX_LAG_MS`. Com `reducedMotion()`, cada evento dura `min(duração, 50)`. `onEvent` não é chamado para eventos descartados.

- [ ] **Step 1: Testes que falham**

`frontend/src/game/director.test.ts`:

```ts
import { Director, EVENT_DURATION_MS, type DirectorOptions } from './director'
import type { Event } from '@/protocol/engine'
import type { StatePayload } from '@/protocol/messages'

const st = (n: number) => ({ view: { handNumber: n }, deadlines: {}, pendingSeats: [] }) as unknown as StatePayload
const played = (seat: number): Event => ({ kind: 'card_played', seat, card: 5 })

function setup(over: Partial<DirectorOptions> = {}) {
  const log: string[] = []
  const d = new Director({
    onEvent: (ev, ms) => log.push(`event:${ev.kind}:${ev.seat ?? ''}:${ms}`),
    onApply: (s) => log.push(`apply:${s.view.handNumber}`),
    onIdle: () => log.push('idle'),
    reducedMotion: () => false,
    hidden: () => false,
    setTimer: (fn, ms) => setTimeout(fn, ms),
    clearTimer: (h) => clearTimeout(h as ReturnType<typeof setTimeout>),
    ...over,
  })
  return { d, log }
}

beforeEach(() => vi.useFakeTimers())
afterEach(() => vi.useRealTimers())

it('aplica state sem eventos na hora', () => {
  const { d, log } = setup()
  d.pushState(st(1))
  expect(log).toEqual(['apply:1'])
})

it('toca eventos em ordem e só então aplica o state', () => {
  const { d, log } = setup()
  d.pushEvents([played(1), played(2)])
  d.pushState(st(2))
  expect(log).toEqual([`event:card_played:1:${EVENT_DURATION_MS.card_played}`])
  vi.advanceTimersByTime(EVENT_DURATION_MS.card_played)
  expect(log.at(-1)).toBe(`event:card_played:2:${EVENT_DURATION_MS.card_played}`)
  vi.advanceTimersByTime(EVENT_DURATION_MS.card_played)
  expect(log.slice(-2)).toEqual(['apply:2', 'idle'])
})

it('mantém só o último state pendente', () => {
  const { d, log } = setup()
  d.pushEvents([played(1)])
  d.pushState(st(2))
  d.pushEvents([played(2)])
  d.pushState(st(3))
  vi.runAllTimers()
  expect(log.filter((l) => l.startsWith('apply'))).toEqual(['apply:3'])
})

it('descarta a fila quando o atraso passa de 3 s', () => {
  const { d, log } = setup()
  d.pushEvents([{ kind: 'raise_requested', seat: 1, stake: 3 }])
  d.pushState(st(1))
  d.pushEvents(Array.from({ length: 12 }, (_, i) => played(i % 4)))
  d.pushState(st(2))
  expect(log.at(-2)).toBe('apply:2')
  expect(d.busy).toBe(false)
})

it('com a aba oculta aplica direto sem eventos', () => {
  const { d, log } = setup({ hidden: () => true })
  d.pushEvents([played(1)])
  d.pushState(st(4))
  expect(log).toEqual(['apply:4', 'idle'])
})

it('reduced-motion encurta cada evento para 50 ms', () => {
  const { d, log } = setup({ reducedMotion: () => true })
  d.pushEvents([{ kind: 'raise_requested', seat: 1, stake: 3 }])
  expect(log).toEqual(['event:raise_requested:1:50'])
})

it('reset() cancela tudo', () => {
  const { d, log } = setup()
  d.pushEvents([played(1)])
  d.pushState(st(5))
  d.reset()
  vi.runAllTimers()
  expect(log).toEqual([`event:card_played:1:${EVENT_DURATION_MS.card_played}`])
})
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd frontend && pnpm vitest run src/game/director.test.ts`
Expected: FAIL — módulo inexistente.

- [ ] **Step 3: Implementar**

`frontend/src/game/director.ts`:

```ts
import type { Event, EventKind } from '@/protocol/engine'
import type { StatePayload } from '@/protocol/messages'

export const EVENT_DURATION_MS: Record<EventKind, number> = {
  hand_started: 600,
  cut_offered: 300,
  card_chosen: 300,
  cards_dealt: 700,
  fours_burned: 500,
  vira_revealed: 500,
  eleven_decided: 600,
  card_played: 350,
  round_ended: 700,
  raise_requested: 900,
  vote_cast: 400,
  raise_resolved: 700,
  raise_punished: 1200,
  hand_ended: 800,
  game_over: 0,
  forfeited: 0,
}

export const MAX_LAG_MS = 3000
const REDUCED_MS = 50

export interface DirectorOptions {
  onEvent(ev: Event, durationMs: number): void
  onApply(state: StatePayload): void
  onIdle?(): void
  reducedMotion(): boolean
  hidden(): boolean
  setTimer(fn: () => void, ms: number): unknown
  clearTimer(h: unknown): void
}

export class Director {
  private queue: Event[] = []
  private pending: StatePayload | null = null
  private timer: unknown = null
  private playing = false
  /** Eventos chegaram com a aba oculta ou atrasados demais: o próximo state é aplicado direto. */
  private skip = false

  constructor(private readonly o: DirectorOptions) {}

  get busy(): boolean {
    return this.playing || this.queue.length > 0
  }

  pushEvents(evs: Event[]): void {
    this.queue.push(...evs)
    if (this.o.hidden() || this.queuedMs() > MAX_LAG_MS) {
      this.skip = true
    }
    if (!this.playing) this.next()
  }

  pushState(s: StatePayload): void {
    this.pending = s
    if (this.skip) {
      this.flush()
      return
    }
    if (!this.busy) this.applyPending()
  }

  /** Descarta a fila e aplica o estado pendente imediatamente. */
  flush(): void {
    this.skip = false
    this.cancelTimer()
    this.queue = []
    this.playing = false
    this.applyPending()
  }

  reset(): void {
    this.skip = false
    this.cancelTimer()
    this.queue = []
    this.pending = null
    this.playing = false
  }

  private next(): void {
    if (this.skip) {
      // aguardando o state que acompanha estes eventos para pular direto
      this.playing = false
      return
    }
    const ev = this.queue.shift()
    if (!ev) {
      this.playing = false
      this.applyPending()
      return
    }
    this.playing = true
    const ms = this.duration(ev)
    this.o.onEvent(ev, ms)
    this.timer = this.o.setTimer(() => {
      this.timer = null
      this.next()
    }, ms)
  }

  private applyPending(): void {
    const s = this.pending
    if (!s) return
    this.pending = null
    this.o.onApply(s)
    this.o.onIdle?.()
  }

  private duration(ev: Event): number {
    const base = EVENT_DURATION_MS[ev.kind] ?? 0
    return this.o.reducedMotion() ? Math.min(base, REDUCED_MS) : base
  }

  private queuedMs(): number {
    return this.queue.reduce((sum, ev) => sum + this.duration(ev), 0)
  }

  private cancelTimer(): void {
    if (this.timer !== null) this.o.clearTimer(this.timer)
    this.timer = null
  }
}
```

Leia de novo os testes "descarta a fila" e "aba oculta": em ambos os eventos chegam **antes** do `state` (ordem do servidor: `events` depois `state`), então `pushEvents` marca `skip` e o `pushState` seguinte faz `flush()`. No teste "descarta a fila", o primeiro `raise_requested` já começou a tocar (o `onEvent` dele foi chamado) — isso é esperado; o que importa é que o `state` 2 seja aplicado sem esperar.

- [ ] **Step 4: Rodar e ver passar**

Run: `cd frontend && pnpm vitest run src/game/director.test.ts && pnpm typecheck`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/game/director.ts frontend/src/game/director.test.ts
git commit -m "feat(frontend): diretor de eventos que anima a fila e aplica o snapshot ao fim"
```

---

### Task 10: Store `game` (estado da mesa, getters e ações)

**Skills:** `typescript-expert`, `frontend-dev-guidelines`.

**Files:**
- Modify: `frontend/src/stores/game.ts` (substitui o stub)
- Create: `frontend/src/stores/game.test.ts`, `frontend/src/test/fixtures.ts`

**Interfaces:**
- Consumes: `Director`, `EVENT_DURATION_MS` (Task 9); `send` (Task 7); `nextStake` (Task 5); `teamOf`, `partnerOf` (Task 3); `useSessionStore` (Task 7); `playSound`, `soundForEvent` (Task 11 — nesta tarefa, injete por `setGameSoundPlayer`, ver abaixo).
- Produces:
  - `interface Scene { event: Event | null; key: number; durationMs: number }`
  - `interface ActiveSignal { id: SignalID; until: number }`
  - `useGameStore()`:
    - state `view: PlayerView | null` (exibido), `latest: StatePayload | null` (último recebido), `deadlines: Record<number, number>`, `pendingSeats: Seat[]`, `scene: Scene`, `signals: Record<number, ActiveSignal>`, `inflight: boolean`, `signalCooldownUntil: number`
    - getters `mySeat`, `myTeam`, `phase`, `isMyTurn`, `amPending`, `canPlay`, `canCover`, `canRaise`, `nextStakeValue`, `mustRespondRaise`, `canRaiseBack`, `mustDecideEleven`, `mustBurn`, `mustChooseCut`, `raiseIsSuicide`, `fourSlots: number[]`
    - actions `onState(p)`, `onEvents(evs)`, `onSignal(p)`, `reset()`, `chooseCard(keep: number)`, `burn(slots: number[])`, `passBurn()`, `playCard(slot: number, covered: boolean)`, `requestRaise()`, `vote(choice: VoteChoice)`, `sendSignal(id: SignalID)`
  - `setGameSoundPlayer(fn: (ev: Event) => void): void` — chamado no `main.ts` (Task 11) para tocar sons sem a store importar Web Audio.
  - Regra de "em voo": ações de jogo marcam `inflight = true`; enquanto `true`, novas ações de jogo são ignoradas (resolvem sem enviar). `inflight` volta a `false` quando a promise do `send` termina (sucesso ou erro). Erro → toast (`useToastsStore().error(code)`).
  - Os getters usam `latest.view` (o estado mais novo recebido), não o `view` exibido, para habilitar botões (spec §6.2).

- [ ] **Step 1: Fixtures**

`frontend/src/test/fixtures.ts`:

```ts
import type { PlayerView } from '@/protocol/engine'
import type { StatePayload } from '@/protocol/messages'

export function makeView(over: Partial<PlayerView> = {}): PlayerView {
  return {
    seat: 0,
    phase: 'playing',
    handKind: 'normal',
    handNumber: 1,
    players: [0, 1, 2, 3].map((seat) => ({ seat, id: `u${seat}`, name: ['Ana', 'Bia', 'Caio', 'Duda'][seat]!, cardCount: 3, voted: false })),
    score: [0, 0],
    dealer: 3,
    turn: 0,
    stake: 1,
    table: [],
    rounds: [],
    deckCount: 28,
    discardCount: 0,
    myHand: [
      { slot: 0, card: 0, hidden: false },
      { slot: 1, card: 13, hidden: false },
      { slot: 2, card: 39, hidden: false },
    ],
    ...over,
  }
}

export function makeState(over: Partial<PlayerView> = {}, pendingSeats: number[] = [0]): StatePayload {
  return { view: makeView(over), deadlines: Object.fromEntries(pendingSeats.map((s) => [s, Date.now() + 30_000])), pendingSeats }
}
```

- [ ] **Step 2: Testes que falham**

`frontend/src/stores/game.test.ts`:

```ts
import { createPinia, setActivePinia } from 'pinia'
import { useGameStore } from './game'
import { setSender } from '@/app/transport'
import { makeState } from '@/test/fixtures'
import { WsSendError } from '@/ws/client'

beforeEach(() => {
  setActivePinia(createPinia())
  vi.useFakeTimers()
})
afterEach(() => {
  vi.useRealTimers()
  setSender(null)
})

it('state sem eventos aparece na hora; com eventos, depois da fila', () => {
  const g = useGameStore()
  g.onState(makeState({ handNumber: 1 }))
  expect(g.view?.handNumber).toBe(1)
  g.onEvents([{ kind: 'card_played', seat: 1, card: 4 }])
  g.onState(makeState({ handNumber: 2 }))
  expect(g.view?.handNumber).toBe(1)
  expect(g.latest?.view.handNumber).toBe(2)
  expect(g.scene.event?.kind).toBe('card_played')
  vi.runAllTimers()
  expect(g.view?.handNumber).toBe(2)
  expect(g.scene.event).toBeNull()
})

it('getters de jogada e aumento', () => {
  const g = useGameStore()
  g.onState(makeState())
  expect([g.isMyTurn, g.amPending, g.canPlay, g.canCover, g.canRaise, g.nextStakeValue]).toEqual([true, true, true, true, true, 3])
  g.onState(makeState({ lastRaiser: 0, stake: 3 }))
  expect(g.canRaise).toBe(false)
  g.onState(makeState({ stake: 12 }))
  expect(g.canRaise).toBe(false)
  g.onState(makeState({ handKind: 'iron' }))
  expect([g.canCover, g.raiseIsSuicide]).toEqual([false, true])
  g.onState(makeState({ turn: 1 }, [1]))
  expect([g.isMyTurn, g.canPlay, g.canRaise]).toEqual([false, false, false])
})

it('resposta ao aumento só para a dupla que vota e ainda não votou', () => {
  const g = useGameStore()
  const pending = { requester: 1, fromTeam: 1, prevStake: 1, toStake: 3 }
  g.onState(makeState({ phase: 'awaiting_raise_response', votingTeam: 0, pending }, [0, 2]))
  expect([g.mustRespondRaise, g.canRaiseBack]).toEqual([true, true])
  g.onState(makeState({ phase: 'awaiting_raise_response', votingTeam: 0, pending: { ...pending, toStake: 12 } }, [0, 2]))
  expect(g.canRaiseBack).toBe(false)
  g.onState(makeState({ phase: 'awaiting_raise_response', votingTeam: 1, pending }, [1, 3]))
  expect(g.mustRespondRaise).toBe(false)
})

it('fases de corte, queima e mão de 11', () => {
  const g = useGameStore()
  g.onState(makeState({ phase: 'cut_choice', cutOffer: [{ keep: 0, card: 1 }, { keep: 1, card: 2 }] }))
  expect(g.mustChooseCut).toBe(true)
  // cartas 0..3 são os 4 (primeiro degrau do RANK_LADDER)
  g.onState(makeState({ phase: 'burn_fours', myHand: [{ slot: 0, card: 0, hidden: false }, { slot: 1, card: 3, hidden: false }, { slot: 2, card: 39, hidden: false }] }))
  expect([g.mustBurn, g.fourSlots]).toEqual([true, [0, 1]])
  g.onState(makeState({ phase: 'eleven_decision', score: [11, 5] }))
  expect(g.mustDecideEleven).toBe(true)
})

it('envia ações de jogo com os payloads do protocolo', async () => {
  const send = vi.fn().mockResolvedValue(undefined)
  setSender(send)
  const g = useGameStore()
  g.onState(makeState())
  await g.playCard(1, true)
  await g.requestRaise()
  await g.vote('accept')
  await g.chooseCard(1)
  await g.burn([0, 2])
  await g.passBurn()
  expect(send.mock.calls.map((c) => c[0])).toEqual([
    { type: 'play_card', payload: { slot: 1, covered: true } },
    { type: 'request_raise' },
    { type: 'vote', payload: { choice: 'accept' } },
    { type: 'choose_card', payload: { keep: 1 } },
    { type: 'burn', payload: { slots: [0, 2] } },
    { type: 'pass_burn' },
  ])
})

it('ignora segunda jogada enquanto a primeira está em voo', async () => {
  let release!: () => void
  const send = vi.fn(() => new Promise<void>((r) => (release = r)))
  setSender(send)
  const g = useGameStore()
  g.onState(makeState())
  const first = g.playCard(0, false)
  await g.playCard(0, false)
  expect(send).toHaveBeenCalledOnce()
  release()
  await first
  expect(g.inflight).toBe(false)
})

it('erro de regra vira toast e destrava', async () => {
  setSender(vi.fn().mockRejectedValue(new WsSendError('not_your_turn', 'x')))
  const g = useGameStore()
  g.onState(makeState())
  await g.playCard(0, false)
  expect(g.inflight).toBe(false)
  const { useToastsStore } = await import('./toasts')
  expect(useToastsStore().items.map((t) => t.text)).toEqual(['Não é a sua vez.'])
})

it('sinais: balão expira e envio respeita 3 s', async () => {
  const send = vi.fn().mockResolvedValue(undefined)
  setSender(send)
  const g = useGameStore()
  g.onSignal({ seat: 2, id: 'boa' })
  expect(g.signals[2]?.id).toBe('boa')
  vi.advanceTimersByTime(3000)
  expect(g.signals[2]).toBeUndefined()
  await g.sendSignal('valeu')
  await g.sendSignal('valeu')
  expect(send).toHaveBeenCalledOnce()
  vi.advanceTimersByTime(3000)
  await g.sendSignal('valeu')
  expect(send).toHaveBeenCalledTimes(2)
})
```

- [ ] **Step 3: Rodar e ver falhar**

Run: `cd frontend && pnpm vitest run src/stores/game.test.ts`
Expected: FAIL — getters/ações inexistentes no stub.

- [ ] **Step 4: Implementar**

`frontend/src/stores/game.ts`:

```ts
import { defineStore } from 'pinia'
import type { Event, PlayerView, Seat, Team, VoteChoice } from '@/protocol/engine'
import type { SignalBroadcastPayload, SignalID, StatePayload } from '@/protocol/messages'
import type { Outgoing } from '@/ws/client'
import { WsSendError } from '@/ws/client'
import { send } from '@/app/transport'
import { Director } from '@/game/director'
import { nextStake } from '@/game/labels'
import { cardFromId } from '@/game/cards'
import { teamOf } from '@/game/seats'
import { useToastsStore } from './toasts'

export interface Scene {
  event: Event | null
  key: number
  durationMs: number
}

export interface ActiveSignal {
  id: SignalID
  until: number
}

const SIGNAL_SHOW_MS = 3000
const SIGNAL_COOLDOWN_MS = 3000

let soundPlayer: (ev: Event) => void = () => {}
export function setGameSoundPlayer(fn: (ev: Event) => void): void {
  soundPlayer = fn
}

// O diretor fica fora do state (não é reativo); recriado no reset.
let director: Director | null = null

function prefersReducedMotion(): boolean {
  return typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches
}

export const useGameStore = defineStore('game', {
  state: () => ({
    view: null as PlayerView | null,
    latest: null as StatePayload | null,
    deadlines: {} as Record<number, number>,
    pendingSeats: [] as Seat[],
    scene: { event: null, key: 0, durationMs: 0 } as Scene,
    signals: {} as Record<number, ActiveSignal>,
    inflight: false,
    signalCooldownUntil: 0,
  }),
  getters: {
    lv: (s): PlayerView | null => s.latest?.view ?? null,
    mySeat(): Seat | null {
      return this.lv?.seat ?? null
    },
    myTeam(): Team | null {
      return this.mySeat === null ? null : teamOf(this.mySeat)
    },
    phase(): PlayerView['phase'] | null {
      return this.lv?.phase ?? null
    },
    isMyTurn(): boolean {
      return this.lv !== null && this.lv.turn !== undefined && this.lv.turn === this.lv.seat
    },
    amPending(): boolean {
      return this.mySeat !== null && (this.latest?.pendingSeats ?? []).includes(this.mySeat)
    },
    canPlay(): boolean {
      return this.phase === 'playing' && this.isMyTurn
    },
    canCover(): boolean {
      return this.canPlay && this.lv?.handKind !== 'iron'
    },
    nextStakeValue(): number | null {
      return this.lv ? nextStake(this.lv.stake) : null
    },
    canRaise(): boolean {
      const v = this.lv
      if (!v || !this.canPlay || v.pending) return false
      if (v.lastRaiser !== undefined && v.lastRaiser === this.myTeam) return false
      return this.nextStakeValue !== null
    },
    raiseIsSuicide(): boolean {
      return this.lv?.handKind === 'eleven' || this.lv?.handKind === 'iron'
    },
    mustRespondRaise(): boolean {
      const v = this.lv
      return !!v && v.phase === 'awaiting_raise_response' && v.votingTeam === this.myTeam && this.amPending
    },
    canRaiseBack(): boolean {
      const to = this.lv?.pending?.toStake
      return this.mustRespondRaise && to !== undefined && nextStake(to) !== null
    },
    mustDecideEleven(): boolean {
      return this.phase === 'eleven_decision' && this.amPending
    },
    mustBurn(): boolean {
      return this.phase === 'burn_fours' && this.amPending
    },
    mustChooseCut(): boolean {
      return (this.phase === 'cut_choice' || this.phase === 'dealer_choice') && this.amPending && (this.lv?.cutOffer?.length ?? 0) > 0
    },
    fourSlots(): number[] {
      return (this.lv?.myHand ?? []).filter((h) => h.card !== undefined && cardFromId(h.card).rank === 4).map((h) => h.slot)
    },
  },
  actions: {
    ensureDirector(): Director {
      director ??= new Director({
        onEvent: (ev, ms) => {
          this.scene = { event: ev, key: this.scene.key + 1, durationMs: ms }
          soundPlayer(ev)
        },
        onApply: (s) => {
          this.view = s.view
          this.deadlines = s.deadlines as Record<number, number>
          this.pendingSeats = s.pendingSeats
        },
        onIdle: () => {
          this.scene = { event: null, key: this.scene.key + 1, durationMs: 0 }
        },
        reducedMotion: prefersReducedMotion,
        hidden: () => typeof document !== 'undefined' && document.hidden,
        setTimer: (fn, ms) => setTimeout(fn, ms),
        clearTimer: (h) => clearTimeout(h as ReturnType<typeof setTimeout>),
      })
      return director
    },
    onState(p: StatePayload) {
      this.latest = p
      this.ensureDirector().pushState(p)
    },
    onEvents(evs: Event[]) {
      this.ensureDirector().pushEvents(evs)
    },
    onSignal(p: SignalBroadcastPayload) {
      const until = Date.now() + SIGNAL_SHOW_MS
      this.signals[p.seat] = { id: p.id, until }
      setTimeout(() => {
        if (this.signals[p.seat]?.until === until) delete this.signals[p.seat]
      }, SIGNAL_SHOW_MS)
    },
    reset() {
      director?.reset()
      director = null
      this.$reset()
    },
    async act(msg: Outgoing) {
      if (this.inflight) return
      this.inflight = true
      try {
        await send(msg)
      } catch (err) {
        const code = err instanceof WsSendError ? err.code : 'internal'
        useToastsStore().error(code, err instanceof Error ? err.message : undefined)
      } finally {
        this.inflight = false
      }
    },
    chooseCard(keep: number) {
      return this.act({ type: 'choose_card', payload: { keep } })
    },
    burn(slots: number[]) {
      return this.act({ type: 'burn', payload: { slots } })
    },
    passBurn() {
      return this.act({ type: 'pass_burn' })
    },
    playCard(slot: number, covered: boolean) {
      return this.act({ type: 'play_card', payload: { slot, covered } })
    },
    requestRaise() {
      return this.act({ type: 'request_raise' })
    },
    vote(choice: VoteChoice) {
      return this.act({ type: 'vote', payload: { choice } })
    },
    async sendSignal(id: SignalID) {
      const now = Date.now()
      if (now < this.signalCooldownUntil) return
      this.signalCooldownUntil = now + SIGNAL_COOLDOWN_MS
      try {
        await send({ type: 'signal', payload: { id } })
      } catch (err) {
        if (err instanceof WsSendError) useToastsStore().error(err.code, err.message)
      }
    },
  },
})
```

Observações para o implementador:
- `mustDecideEleven` depende de `pendingSeats`: o servidor só põe em `pendingSeats` quem tem decisão a tomar, então a dupla sem 11 não vê o painel (spec §7.4).
- O teste de queima usa `cardFromId(0).rank === 4` e `cardFromId(3).rank === 4` (primeiro degrau de `RANK_LADDER`); se a Task 3 tiver corrigido a ordem do degrau, ajuste os ids do fixture para cartas de rank 4 segundo `cards.json`.

- [ ] **Step 5: Rodar e ver passar**

Run: `cd frontend && pnpm vitest run src/stores src/app && pnpm typecheck && pnpm lint`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/stores/game.ts frontend/src/stores/game.test.ts frontend/src/test/fixtures.ts
git commit -m "feat(frontend): store do jogo com cena, prazos, sinais e ações sem envio duplicado"
```

---

### Task 11: Efeitos sonoros

**Skills:** `frontend-dev-guidelines`.

**Files:**
- Create: `frontend/src/audio/sfx.ts`, `frontend/src/audio/sfx.test.ts`, `frontend/src/audio/install.ts`
- Create: `frontend/src/assets/sfx/*.ogg`, `frontend/src/assets/sfx/*.mp3`, `frontend/src/assets/sfx/LICENSES.md`
- Create: `scripts/sfx/fetch.sh`
- Modify: `frontend/src/main.ts`

**Interfaces:**
- Consumes: `Event` (`@/protocol/engine`), `usePrefsStore` (Task 7), `setGameSoundPlayer` (Task 10).
- Produces:
  - `type SoundId = 'shuffle' | 'deal' | 'card' | 'cardCovered' | 'burn' | 'flip' | 'trickWon' | 'truco' | 'accept' | 'run' | 'point' | 'yourTurn' | 'timeLow' | 'victory' | 'defeat'`
  - `soundForEvent(ev: Event, myTeam: number | null): SoundId | null`
  - `class Sfx { constructor(o: { createContext(): AudioContextLike; fetchBuffer(url: string): Promise<ArrayBuffer>; urls: Record<SoundId, string>; volume(): number; muted(): boolean }); unlock(): Promise<void>; play(id: SoundId): void }`
  - `audio/install.ts`: `installSfx(): void` — cria a instância real (assets via `import.meta.glob`, volume/mudo de `usePrefsStore`), desbloqueia no primeiro `pointerdown`/`keydown` e chama `setGameSoundPlayer`; `sfx(): Sfx | null` devolve a instância (usada para `yourTurn`/`timeLow` nas Tarefas 17 e 19).

- [ ] **Step 1: Baixar e converter os sons (CC0)**

`scripts/sfx/fetch.sh`:

```bash
#!/usr/bin/env bash
# Baixa os pacotes CC0 da Kenney e gera OGG + MP3 com os nomes usados pelo app.
set -euo pipefail
cd "$(dirname "$0")/../.."
out=frontend/src/assets/sfx
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

curl -fsSL -o "$tmp/casino.zip" "https://kenney.nl/media/pages/assets/casino-audio/ed1b2f3e0b-1677589334/kenney_casino-audio.zip"
curl -fsSL -o "$tmp/ui.zip" "https://kenney.nl/media/pages/assets/interface-sounds/3a0d4c8a6f-1677589452/kenney_interface-sounds.zip"
unzip -q "$tmp/casino.zip" -d "$tmp/casino"
unzip -q "$tmp/ui.zip" -d "$tmp/ui"

pick() { # nome-final arquivo-fonte
  ffmpeg -loglevel error -y -i "$2" -ac 1 -ar 44100 -c:a libvorbis -q:a 3 "$out/$1.ogg"
  ffmpeg -loglevel error -y -i "$2" -ac 1 -ar 44100 -c:a libmp3lame -b:a 64k "$out/$1.mp3"
}

mkdir -p "$out"
pick shuffle     "$tmp/casino/Audio/card-shuffle.ogg"
pick deal        "$tmp/casino/Audio/card-fan-1.ogg"
pick card        "$tmp/casino/Audio/card-place-1.ogg"
pick cardCovered "$tmp/casino/Audio/card-slide-1.ogg"
pick burn        "$tmp/casino/Audio/card-shove-1.ogg"
pick flip        "$tmp/casino/Audio/card-slide-3.ogg"
pick trickWon    "$tmp/casino/Audio/chips-collide-1.ogg"
pick truco       "$tmp/ui/Audio/confirmation_002.ogg"
pick accept      "$tmp/ui/Audio/confirmation_001.ogg"
pick run         "$tmp/ui/Audio/error_004.ogg"
pick point       "$tmp/casino/Audio/chips-stack-1.ogg"
pick yourTurn    "$tmp/ui/Audio/bong_001.ogg"
pick timeLow     "$tmp/ui/Audio/tick_002.ogg"
pick victory     "$tmp/ui/Audio/maximize_006.ogg"
pick defeat      "$tmp/ui/Audio/minimize_006.ogg"
du -ch "$out"/*.ogg | tail -1
```

As URLs dos zips mudam quando a Kenney republica: abra `https://kenney.nl/assets/casino-audio` e `https://kenney.nl/assets/interface-sounds`, copie o link "Download" atual e atualize o script. Se um arquivo-fonte não existir com o nome acima, liste o pacote (`ls "$tmp/casino/Audio"`) e escolha o equivalente mais próximo, mantendo o **nome final**. Requer `ffmpeg`.

Run: `chmod +x scripts/sfx/fetch.sh && ./scripts/sfx/fetch.sh`
Expected: 30 arquivos em `frontend/src/assets/sfx/`; total OGG < 300 KB (se passar, baixe `-q:a` para 2).

`frontend/src/assets/sfx/LICENSES.md`:

```markdown
# Créditos dos sons

Todos os arquivos desta pasta derivam de pacotes da Kenney (https://kenney.nl), licença **CC0 1.0** (domínio público).

| Arquivo | Pacote | Arquivo original |
|---|---|---|
| shuffle | Casino Audio | card-shuffle.ogg |
| deal | Casino Audio | card-fan-1.ogg |
| card | Casino Audio | card-place-1.ogg |
| cardCovered | Casino Audio | card-slide-1.ogg |
| burn | Casino Audio | card-shove-1.ogg |
| flip | Casino Audio | card-slide-3.ogg |
| trickWon | Casino Audio | chips-collide-1.ogg |
| truco | Interface Sounds | confirmation_002.ogg |
| accept | Interface Sounds | confirmation_001.ogg |
| run | Interface Sounds | error_004.ogg |
| point | Casino Audio | chips-stack-1.ogg |
| yourTurn | Interface Sounds | bong_001.ogg |
| timeLow | Interface Sounds | tick_002.ogg |
| victory | Interface Sounds | maximize_006.ogg |
| defeat | Interface Sounds | minimize_006.ogg |
```

(Atualize a coluna "Arquivo original" se trocar algum arquivo no script.)

- [ ] **Step 2: Testes que falham**

`frontend/src/audio/sfx.test.ts`:

```ts
import { Sfx, soundForEvent, type SoundId } from './sfx'

function fakeContext() {
  const started: number[] = []
  const gains: number[] = []
  const ctx = {
    state: 'suspended' as AudioContextState,
    destination: {},
    resume: vi.fn(async () => {
      ctx.state = 'running'
    }),
    decodeAudioData: vi.fn(async () => ({ duration: 1 }) as unknown as AudioBuffer),
    createBufferSource: vi.fn(() => ({ buffer: null, connect: vi.fn(), start: () => started.push(1) })),
    createGain: vi.fn(() => {
      const g = { gain: { value: 1 }, connect: vi.fn() }
      queueMicrotask(() => gains.push(g.gain.value))
      return g
    }),
  }
  return { ctx, started, gains }
}

const urls = Object.fromEntries(
  ['shuffle', 'deal', 'card', 'cardCovered', 'burn', 'flip', 'trickWon', 'truco', 'accept', 'run', 'point', 'yourTurn', 'timeLow', 'victory', 'defeat'].map((id) => [id, `/${id}.ogg`]),
) as Record<SoundId, string>

it('mapeia eventos para sons, inclusive vitória/derrota pela minha dupla', () => {
  expect(soundForEvent({ kind: 'card_played', seat: 1, card: 3 }, 0)).toBe('card')
  expect(soundForEvent({ kind: 'card_played', seat: 1 }, 0)).toBe('cardCovered')
  expect(soundForEvent({ kind: 'raise_requested', seat: 1, stake: 3 }, 0)).toBe('truco')
  expect(soundForEvent({ kind: 'raise_resolved', choice: 'run', stake: 1 }, 0)).toBe('run')
  expect(soundForEvent({ kind: 'game_over', team: 0 }, 0)).toBe('victory')
  expect(soundForEvent({ kind: 'game_over', team: 1 }, 0)).toBe('defeat')
  expect(soundForEvent({ kind: 'forfeited', team: 1 }, 0)).toBeNull()
  expect(soundForEvent({ kind: 'cut_offered', seat: 0 }, 0)).toBeNull()
})

it('não toca antes do desbloqueio nem com mudo; toca com o volume das preferências', async () => {
  const { ctx, started } = fakeContext()
  let muted = false
  const sfx = new Sfx({
    createContext: () => ctx as never,
    fetchBuffer: async () => new ArrayBuffer(8),
    urls,
    volume: () => 0.4,
    muted: () => muted,
  })
  sfx.play('card')
  expect(started).toHaveLength(0)
  await sfx.unlock()
  sfx.play('card')
  expect(started).toHaveLength(1)
  muted = true
  sfx.play('card')
  expect(started).toHaveLength(1)
})
```

- [ ] **Step 3: Rodar e ver falhar**

Run: `cd frontend && pnpm vitest run src/audio`
Expected: FAIL — módulo inexistente.

- [ ] **Step 4: Implementar**

`frontend/src/audio/sfx.ts`:

```ts
import type { Event } from '@/protocol/engine'

export type SoundId =
  | 'shuffle' | 'deal' | 'card' | 'cardCovered' | 'burn' | 'flip' | 'trickWon'
  | 'truco' | 'accept' | 'run' | 'point' | 'yourTurn' | 'timeLow' | 'victory' | 'defeat'

export const SOUND_IDS: readonly SoundId[] = [
  'shuffle', 'deal', 'card', 'cardCovered', 'burn', 'flip', 'trickWon',
  'truco', 'accept', 'run', 'point', 'yourTurn', 'timeLow', 'victory', 'defeat',
]

export function soundForEvent(ev: Event, myTeam: number | null): SoundId | null {
  switch (ev.kind) {
    case 'hand_started':
      return 'shuffle'
    case 'cards_dealt':
      return 'deal'
    case 'fours_burned':
      return 'burn'
    case 'vira_revealed':
      return 'flip'
    case 'card_played':
      return ev.card === undefined ? 'cardCovered' : 'card'
    case 'round_ended':
      return ev.team === undefined ? null : 'trickWon'
    case 'raise_requested':
      return 'truco'
    case 'raise_resolved':
      return ev.choice === 'run' ? 'run' : 'accept'
    case 'raise_punished':
      return 'defeat'
    case 'hand_ended':
      return 'point'
    case 'game_over':
      // no W.O. o engine emite forfeited e depois game_over: o som sai só aqui
      if (ev.team === undefined || myTeam === null) return null
      return ev.team === myTeam ? 'victory' : 'defeat'
    default:
      return null
  }
}

export interface AudioContextLike {
  state: AudioContextState
  destination: AudioNode | object
  resume(): Promise<void>
  decodeAudioData(data: ArrayBuffer): Promise<AudioBuffer>
  createBufferSource(): { buffer: AudioBuffer | null; connect(n: unknown): unknown; start(): void }
  createGain(): { gain: { value: number }; connect(n: unknown): unknown }
}

export interface SfxOptions {
  createContext(): AudioContextLike
  fetchBuffer(url: string): Promise<ArrayBuffer>
  urls: Record<SoundId, string>
  volume(): number
  muted(): boolean
}

export class Sfx {
  private ctx: AudioContextLike | null = null
  private buffers = new Map<SoundId, AudioBuffer>()

  constructor(private readonly o: SfxOptions) {}

  /** Chamado no primeiro gesto do usuário (política de autoplay dos navegadores). */
  async unlock(): Promise<void> {
    if (!this.ctx) this.ctx = this.o.createContext()
    if (this.ctx.state !== 'running') await this.ctx.resume()
    await Promise.all(
      (Object.keys(this.o.urls) as SoundId[]).map(async (id) => {
        if (this.buffers.has(id)) return
        try {
          const data = await this.o.fetchBuffer(this.o.urls[id])
          this.buffers.set(id, await this.ctx!.decodeAudioData(data))
        } catch {
          // som ausente não quebra o jogo
        }
      }),
    )
  }

  play(id: SoundId): void {
    const ctx = this.ctx
    const buffer = this.buffers.get(id)
    if (!ctx || ctx.state !== 'running' || !buffer || this.o.muted() || this.o.volume() <= 0) return
    const source = ctx.createBufferSource()
    const gain = ctx.createGain()
    gain.gain.value = this.o.volume()
    source.buffer = buffer
    source.connect(gain)
    gain.connect(ctx.destination)
    source.start()
  }
}
```

Crie `frontend/src/audio/install.ts` (fora do teste unitário; usa DOM e Pinia):

```ts
import { Sfx, SOUND_IDS, soundForEvent, type SoundId } from './sfx'
import { usePrefsStore } from '@/stores/prefs'
import { useGameStore, setGameSoundPlayer } from '@/stores/game'

const ogg = import.meta.glob('@/assets/sfx/*.ogg', { eager: true, query: '?url', import: 'default' }) as Record<string, string>
const mp3 = import.meta.glob('@/assets/sfx/*.mp3', { eager: true, query: '?url', import: 'default' }) as Record<string, string>

function urlsFor(): Record<SoundId, string> {
  const useOgg = typeof Audio !== 'undefined' && new Audio().canPlayType('audio/ogg; codecs="vorbis"') !== ''
  const table = useOgg ? ogg : mp3
  const ext = useOgg ? 'ogg' : 'mp3'
  return Object.fromEntries(
    SOUND_IDS.map((id) => {
      const entry = Object.entries(table).find(([path]) => path.endsWith(`/${id}.${ext}`))
      return [id, entry?.[1] ?? '']
    }),
  ) as Record<SoundId, string>
}

let instance: Sfx | null = null

export function sfx(): Sfx | null {
  return instance
}

export function installSfx(): void {
  const prefs = usePrefsStore()
  instance = new Sfx({
    createContext: () => new AudioContext() as never,
    fetchBuffer: async (url) => (await fetch(url)).arrayBuffer(),
    urls: urlsFor(),
    volume: () => prefs.volume,
    muted: () => prefs.muted,
  })
  const unlock = () => {
    void instance?.unlock()
    window.removeEventListener('pointerdown', unlock)
    window.removeEventListener('keydown', unlock)
  }
  window.addEventListener('pointerdown', unlock)
  window.addEventListener('keydown', unlock)
  setGameSoundPlayer((ev) => {
    const id = soundForEvent(ev, useGameStore().myTeam)
    if (id) instance?.play(id)
  })
}
```

Em `frontend/src/main.ts`, depois de `app.use(pinia)`, chame `installSfx()`.

- [ ] **Step 5: Rodar e ver passar**

Run: `cd frontend && pnpm vitest run src/audio && pnpm typecheck && pnpm build`
Expected: PASS; o build inclui os sons em `dist/assets`.

- [ ] **Step 6: Commit**

```bash
git add scripts/sfx frontend/src/audio frontend/src/assets/sfx frontend/src/main.ts
git commit -m "feat(frontend): efeitos sonoros CC0 com desbloqueio no primeiro gesto e volume"
```

---

### Task 12: Cartas em SVG (francês e espanhol)

**Skills:** `frontend-design:frontend-design` (verso e consistência visual dos dois baralhos).

**Files:**
- Create: `scripts/cards/french.mjs`, `scripts/cards/spanish-commons.mjs`, `scripts/cards/spanish-generate.mjs`
- Create: `frontend/src/assets/cards/frances/*.svg` (40 + `verso.svg`), `frontend/src/assets/cards/frances/LICENSES.md`
- Create: `frontend/src/assets/cards/espanhol/*.svg` (40 + `verso.svg`), `frontend/src/assets/cards/espanhol/LICENSES.md`
- Create: `frontend/src/game/cardAssets.ts`, `frontend/src/game/cardAssets.test.ts`

**Interfaces:**
- Consumes: `cardFileName`, `DeckStyle`, `RANK_LADDER` (Task 3).
- Produces: `cardUrl(id: CardID, deck: DeckStyle): string`, `backUrl(deck: DeckStyle): string` — URLs resolvidas pelo Vite (`import.meta.glob`).

- [ ] **Step 1: Teste que falha**

`frontend/src/game/cardAssets.test.ts`:

```ts
import { backUrl, cardUrl, CARD_FILES } from './cardAssets'

it.each(['frances', 'espanhol'] as const)('baralho %s tem as 40 cartas e o verso', (deck) => {
  for (let id = 0; id < 40; id++) expect(cardUrl(id, deck)).not.toBe('')
  expect(backUrl(deck)).not.toBe('')
  expect(Object.keys(CARD_FILES[deck])).toHaveLength(41)
})
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd frontend && pnpm vitest run src/game/cardAssets.test.ts`
Expected: FAIL — módulo inexistente.

- [ ] **Step 3: Implementar a resolução de URLs**

`frontend/src/game/cardAssets.ts`:

```ts
import type { CardID } from '@/protocol/engine'
import { cardFileName, type DeckStyle } from './cards'

const frances = import.meta.glob('@/assets/cards/frances/*.svg', { eager: true, query: '?url', import: 'default' }) as Record<string, string>
const espanhol = import.meta.glob('@/assets/cards/espanhol/*.svg', { eager: true, query: '?url', import: 'default' }) as Record<string, string>

function byFile(table: Record<string, string>): Record<string, string> {
  return Object.fromEntries(Object.entries(table).map(([path, url]) => [path.split('/').pop()!, url]))
}

export const CARD_FILES: Record<DeckStyle, Record<string, string>> = { frances: byFile(frances), espanhol: byFile(espanhol) }

export function cardUrl(id: CardID, deck: DeckStyle): string {
  return CARD_FILES[deck][cardFileName(id)] ?? ''
}

export function backUrl(deck: DeckStyle): string {
  return CARD_FILES[deck]['verso.svg'] ?? ''
}
```

- [ ] **Step 4: Baralho francês (Vector Playing Cards, domínio público)**

Baixe o pacote SVG de *Vector Playing Cards* (Byron Knoll), publicado em domínio público, a partir de `https://code.google.com/archive/p/vector-playing-cards/downloads` (arquivo `cards-svg.zip` / "SVG-cards") para uma pasta temporária **vazia** (ex.: `/tmp/vpc`). Confira a declaração de domínio público no `README`/página do projeto antes de seguir; se a licença não for domínio público, **pare** e avise o usuário.

`scripts/cards/french.mjs`:

```js
// Copia as 40 cartas do Vector Playing Cards para os nomes do app.
// Uso: node scripts/cards/french.mjs <pasta-dos-svgs>
import { copyFileSync, readdirSync, mkdirSync } from 'node:fs'
import { join } from 'node:path'

const src = process.argv[2]
if (!src) throw new Error('informe a pasta com os SVGs')
const out = 'frontend/src/assets/cards/frances'
mkdirSync(out, { recursive: true })

const RANKS = { 1: 'ace', 2: '2', 3: '3', 4: '4', 5: '5', 6: '6', 7: '7', 10: 'jack', 11: 'queen', 12: 'king' }
const SUITS = { ouros: 'diamonds', espadas: 'spades', copas: 'hearts', paus: 'clubs' }
const files = readdirSync(src)

for (const [rank, en] of Object.entries(RANKS)) {
  for (const [naipe, suit] of Object.entries(SUITS)) {
    // O pacote usa "<rank>_of_<suit>.svg"; figuras têm variante "...2.svg" com arte completa — prefira-a.
    const wanted = [`${en}_of_${suit}2.svg`, `${en}_of_${suit}.svg`]
    const found = wanted.find((f) => files.includes(f))
    if (!found) throw new Error(`não achei ${wanted.join(' ou ')} em ${src}`)
    copyFileSync(join(src, found), join(out, `${rank}-${naipe}.svg`))
  }
}
console.log('40 cartas copiadas para', out)
```

Se os nomes do pacote baixado forem outros (ex.: `AS.svg`, `10H.svg`), ajuste só a construção de `wanted` para o padrão encontrado (liste com `ls /tmp/vpc | head`).

O pacote não traz verso; crie `frontend/src/assets/cards/frances/verso.svg` (autoral, mesmo tamanho/proporção das cartas — confira o `viewBox` de uma carta copiada, tipicamente `0 0 169.075 244.64`):

```svg
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 169.075 244.64">
  <defs>
    <pattern id="p" width="12" height="12" patternUnits="userSpaceOnUse" patternTransform="rotate(45)">
      <rect width="12" height="12" fill="#7f1d1d"/>
      <rect width="6" height="12" fill="#991b1b"/>
    </pattern>
  </defs>
  <rect x="1" y="1" width="167.075" height="242.64" rx="9" fill="#fff" stroke="#111" stroke-width="1"/>
  <rect x="10" y="10" width="149.075" height="224.64" rx="6" fill="url(#p)"/>
</svg>
```

Run: `node scripts/cards/french.mjs /tmp/vpc && npx --yes svgo -f frontend/src/assets/cards/frances -q`

`frontend/src/assets/cards/frances/LICENSES.md`:

```markdown
# Créditos — baralho francês

- Cartas: *Vector Playing Cards* por Byron Knoll — **domínio público**.
  Origem: https://code.google.com/archive/p/vector-playing-cards/
  Renomeadas para `<rank>-<naipe>.svg` e otimizadas com SVGO.
- `verso.svg`: criado para este projeto (mesma licença do repositório).
```

- [ ] **Step 5: Baralho espanhol — tentar a Wikimedia Commons**

`scripts/cards/spanish-commons.mjs` lista arquivos de uma categoria com a licença de cada um:

```js
// Uso: node scripts/cards/spanish-commons.mjs "Category:SVG Spanish deck"
const category = process.argv[2] ?? 'Category:SVG Spanish deck'
const api = 'https://commons.wikimedia.org/w/api.php'
const ALLOWED = /^(public domain|pd|cc0)/i

const params = new URLSearchParams({
  action: 'query', format: 'json', generator: 'categorymembers', gcmtitle: category,
  gcmtype: 'file', gcmlimit: '200', prop: 'imageinfo', iiprop: 'url|extmetadata', origin: '*',
})
const res = await fetch(`${api}?${params}`, { headers: { 'User-Agent': 'truco-assets/1.0' } })
const data = await res.json()
const rows = Object.values(data.query?.pages ?? {}).map((p) => {
  const meta = p.imageinfo?.[0]?.extmetadata ?? {}
  return { title: p.title, license: meta.LicenseShortName?.value ?? '?', artist: (meta.Artist?.value ?? '').replace(/<[^>]+>/g, ''), url: p.imageinfo?.[0]?.url }
})
for (const r of rows) console.log(`${ALLOWED.test(r.license) ? 'OK ' : '-- '} ${r.license.padEnd(14)} ${r.title}`)
console.log(`\n${rows.filter((r) => ALLOWED.test(r.license)).length} de ${rows.length} com licença aceita`)
```

Run: `node scripts/cards/spanish-commons.mjs "Category:SVG Spanish deck"` (se a categoria não existir, tente `"Category:Baraja española (SVG)"` e `"Category:Spanish playing cards"`).

**Decisão (registre no commit):**
- Se houver as 40 cartas (1–7, 10, 11, 12 de oros, copas, espadas, bastos) **todas** com licença marcada `OK`, baixe-as para `frontend/src/assets/cards/espanhol/<rank>-<naipe>.svg` (bastos → `paus`, oros → `ouros`), otimize com SVGO e escreva o `LICENSES.md` listando título, autor, licença e URL de cada arquivo.
- Caso contrário (o mais provável), siga o Step 6.

- [ ] **Step 6: Baralho espanhol — gerar em SVG (fallback da spec §8.1)**

`scripts/cards/spanish-generate.mjs`:

```js
// Gera as 40 cartas espanholas e o verso em SVG autoral.
// Uso: node scripts/cards/spanish-generate.mjs
import { mkdirSync, writeFileSync } from 'node:fs'

const out = 'frontend/src/assets/cards/espanhol'
mkdirSync(out, { recursive: true })
const W = 169, H = 245

const SUITS = {
  ouros: { color: '#b45309', name: 'oros' },
  copas: { color: '#b91c1c', name: 'copas' },
  espadas: { color: '#1d4ed8', name: 'espadas' },
  paus: { color: '#15803d', name: 'bastos' },
}

// Símbolos desenhados em caixa 40x40 centrada em (0,0).
function pip(naipe) {
  const c = SUITS[naipe].color
  switch (naipe) {
    case 'ouros':
      return `<circle r="17" fill="${c}"/><circle r="11" fill="none" stroke="#fde68a" stroke-width="3"/><circle r="4" fill="#fde68a"/>`
    case 'copas':
      return `<path d="M-14 -16 H14 Q14 2 3 6 V12 H9 V17 H-9 V12 H-3 V6 Q-14 2 -14 -16 Z" fill="${c}"/><rect x="-14" y="-18" width="28" height="4" rx="2" fill="#fca5a5"/>`
    case 'espadas':
      return `<path d="M0 -19 L4 -6 V10 H-4 V-6 Z" fill="${c}"/><rect x="-10" y="9" width="20" height="4" rx="2" fill="#1e3a8a"/><rect x="-2.5" y="13" width="5" height="7" fill="#1e3a8a"/>`
    case 'paus':
      return `<path d="M-5 19 Q-8 0 -4 -16 Q0 -21 5 -16 Q9 0 5 19 Z" fill="${c}"/><circle cx="-6" cy="-6" r="3" fill="#166534"/><circle cx="6" cy="4" r="3" fill="#166534"/>`
  }
}

// Posições dos símbolos para 1..7 (coordenadas relativas ao centro da carta).
const LAYOUTS = {
  1: [[0, 0, 2.2]],
  2: [[0, -55, 1], [0, 55, 1]],
  3: [[0, -60, 1], [0, 0, 1], [0, 60, 1]],
  4: [[-32, -55, 1], [32, -55, 1], [-32, 55, 1], [32, 55, 1]],
  5: [[-32, -55, 1], [32, -55, 1], [0, 0, 1], [-32, 55, 1], [32, 55, 1]],
  6: [[-32, -60, 1], [32, -60, 1], [-32, 0, 1], [32, 0, 1], [-32, 60, 1], [32, 60, 1]],
  7: [[-32, -65, 1], [32, -65, 1], [0, -32, 1], [-32, 0, 1], [32, 0, 1], [-32, 65, 1], [32, 65, 1]],
}

const FIGURES = { 10: 'Sota', 11: 'Caballo', 12: 'Rey' }

function figure(rank, naipe) {
  const c = SUITS[naipe].color
  return `
    <rect x="-52" y="-82" width="104" height="164" rx="8" fill="${c}" opacity="0.12"/>
    <circle cx="0" cy="-38" r="16" fill="#fcd34d" stroke="${c}" stroke-width="2"/>
    <path d="M-28 30 Q-26 -18 0 -20 Q26 -18 28 30 Z" fill="${c}" opacity="0.85"/>
    ${rank === 12 ? `<path d="M-14 -56 L-8 -66 L0 -56 L8 -66 L14 -56 Z" fill="#f59e0b"/>` : ''}
    ${rank === 11 ? `<path d="M-30 40 Q0 20 30 40 L24 62 H-24 Z" fill="#78350f" opacity="0.8"/>` : ''}
    <g transform="translate(0 48) scale(0.8)">${pip(naipe)}</g>
    <text y="76" text-anchor="middle" font-family="Georgia, serif" font-size="13" fill="${c}">${FIGURES[rank]}</text>`
}

function card(rank, naipe) {
  const c = SUITS[naipe].color
  const body = rank <= 7
    ? LAYOUTS[rank].map(([x, y, s]) => `<g transform="translate(${x} ${y}) scale(${s})">${pip(naipe)}</g>`).join('')
    : figure(rank, naipe)
  const corner = (rot) => `<g transform="rotate(${rot} ${W / 2} ${H / 2})">
      <text x="12" y="26" font-family="Georgia, serif" font-size="22" font-weight="700" fill="${c}">${rank}</text>
      <g transform="translate(20 42) scale(0.35)">${pip(naipe)}</g></g>`
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${W} ${H}">
  <rect x="1" y="1" width="${W - 2}" height="${H - 2}" rx="9" fill="#fffdf7" stroke="#111" stroke-width="1"/>
  <rect x="6" y="6" width="${W - 12}" height="${H - 12}" rx="6" fill="none" stroke="${c}" stroke-width="1.5"/>
  ${corner(0)}${corner(180)}
  <g transform="translate(${W / 2} ${H / 2})">${body}</g>
</svg>
`
}

for (const rank of [1, 2, 3, 4, 5, 6, 7, 10, 11, 12]) {
  for (const naipe of Object.keys(SUITS)) writeFileSync(`${out}/${rank}-${naipe}.svg`, card(rank, naipe))
}
writeFileSync(`${out}/verso.svg`, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${W} ${H}">
  <defs><pattern id="p" width="14" height="14" patternUnits="userSpaceOnUse">
    <rect width="14" height="14" fill="#1e3a8a"/><circle cx="7" cy="7" r="3" fill="#fbbf24"/></pattern></defs>
  <rect x="1" y="1" width="${W - 2}" height="${H - 2}" rx="9" fill="#fff" stroke="#111" stroke-width="1"/>
  <rect x="10" y="10" width="${W - 20}" height="${H - 20}" rx="6" fill="url(#p)"/>
</svg>
`)
console.log('40 cartas + verso geradas em', out)
```

Run: `node scripts/cards/spanish-generate.mjs`

Abra três cartas no navegador (`frontend/src/assets/cards/espanhol/1-ouros.svg`, `7-paus.svg`, `12-copas.svg`) e ajuste o desenho com a skill `frontend-design:frontend-design` até ficar legível no tamanho da mão (~90 px de largura). Mantenha os nomes de arquivo.

`frontend/src/assets/cards/espanhol/LICENSES.md` (no caso gerado):

```markdown
# Créditos — baralho espanhol

Cartas e verso gerados por `scripts/cards/spanish-generate.mjs`, criados para este projeto (mesma licença do repositório).
Nenhum conjunto completo da Wikimedia Commons em domínio público/CC0 foi encontrado em <data da verificação>.
```

(Substitua `<data da verificação>` pela data real em que rodou o Step 5.)

- [ ] **Step 7: Rodar e ver passar**

Run: `cd frontend && pnpm vitest run src/game/cardAssets.test.ts && pnpm build && du -sh dist/assets`
Expected: PASS; build ok.

- [ ] **Step 8: Commit**

```bash
git add scripts/cards frontend/src/assets/cards frontend/src/game/cardAssets.ts frontend/src/game/cardAssets.test.ts
git commit -m "feat(frontend): baralhos francês (domínio público) e espanhol com licenças registradas"
```

---
### Task 13: Design system, shell, rotas e guarda

**Skills:** `frontend-design:frontend-design` (tokens, tipografia, tema da mesa — **antes** do Step 1), `ui-ux-pro-max` (estados globais), `fixing-accessibility` (Modal e toasts, ao final).

**Files:**
- Modify: `frontend/src/styles.css`, `frontend/src/main.ts`, `frontend/src/App.vue`
- Create: `frontend/src/assets/fonts/*` (auto-hospedadas, licença OFL) + `LICENSES.md`
- Create: `frontend/src/components/ui/{Botao,Campo,Modal,Toasts,Toggle,IndicadorConexao}.vue`
- Create: `frontend/src/components/ui/Modal.test.ts`, `frontend/src/components/ui/IndicadorConexao.test.ts`
- Create: `frontend/src/components/AppShell.vue`
- Create: `frontend/src/router.ts`, `frontend/src/router.test.ts`
- Create: `frontend/src/app/navigation.ts`, `frontend/src/app/navigation.test.ts`
- Create: `frontend/src/views/ConviteView.vue`, e views provisórias (`<template><h1>…</h1></template>`) para as rotas das Tarefas 14–16, substituídas nelas
- Modify: `frontend/src/stores/session.ts` (ação `resume` não entra aqui — o indicador chama `resumeConnection` de `@/app/wire`)

**Interfaces:**
- Consumes: `useSessionStore`, `useToastsStore`, `useRoomStore`, `useLobbyStore` (`normalizeCode`), `installConnection`, `resumeConnection` (Tarefas 7–8).
- Produces:
  - Tokens Tailwind (`@theme`): `--color-felt`, `--color-felt-deep`, `--color-surface`, `--color-surface-2`, `--color-ink`, `--color-ink-muted`, `--color-accent`, `--color-danger`, `--color-team-0`, `--color-team-1`, `--radius-card`, `--shadow-card`, `--font-display`, `--font-body` — usáveis como `bg-felt`, `text-ink`, `rounded-card` etc.
  - `Botao.vue` props `{ variant?: 'primary' | 'secondary' | 'danger' | 'ghost'; loading?: boolean; disabled?: boolean; type?: 'button' | 'submit' }`, slot padrão.
  - `Campo.vue` props `{ label: string; modelValue: string; type?: string; error?: string; autocomplete?: string; required?: boolean }`, emite `update:modelValue`; liga `aria-invalid` e `aria-describedby` ao erro.
  - `Modal.vue` props `{ open: boolean; title: string }`, emite `close`; foco preso, `Esc` fecha, foco volta ao elemento anterior.
  - `Toggle.vue` props `{ modelValue: boolean; label: string }`, `role="switch"`.
  - `Toasts.vue` (lê `useToastsStore`), `IndicadorConexao.vue` (lê `useSessionStore`).
  - `router.ts`: `createAppRouter(history?: RouterHistory): Router`; `sanitizeReturnTo(raw: unknown): string`; nomes de rota `entrar`, `cadastro`, `verificar-email`, `esqueci-senha`, `redefinir-senha`, `lobby`, `sala`, `convite`, `perfil`.
  - `app/navigation.ts`: `installRoomNavigation(router: Router): void`.

- [ ] **Step 1: Tokens e fontes**

Carregue `frontend-design:frontend-design` e defina a direção visual: mesa de feltro verde, cartas com sombra suave, tipografia display para placar/valores e sans legível para o resto; tema claro/escuro seguindo o sistema. Escolha duas fontes **OFL** (ex.: uma display e uma sans), baixe os `.woff2` para `frontend/src/assets/fonts/` e registre origem e licença em `frontend/src/assets/fonts/LICENSES.md`.

`frontend/src/styles.css` (valores de cor são o ponto de partida; a skill pode refiná-los mantendo os nomes):

```css
@import 'tailwindcss';

@font-face {
  font-family: 'Display';
  src: url('./assets/fonts/display.woff2') format('woff2');
  font-display: swap;
}
@font-face {
  font-family: 'Body';
  src: url('./assets/fonts/body.woff2') format('woff2');
  font-display: swap;
}

@theme {
  --font-display: 'Display', ui-serif, Georgia, serif;
  --font-body: 'Body', ui-sans-serif, system-ui, sans-serif;
  --color-felt: oklch(0.45 0.09 155);
  --color-felt-deep: oklch(0.32 0.07 155);
  --color-surface: oklch(0.99 0 0);
  --color-surface-2: oklch(0.95 0.01 155);
  --color-ink: oklch(0.2 0.01 155);
  --color-ink-muted: oklch(0.45 0.02 155);
  --color-accent: oklch(0.78 0.15 85);
  --color-danger: oklch(0.55 0.2 27);
  --color-team-0: oklch(0.62 0.15 250);
  --color-team-1: oklch(0.65 0.17 30);
  --radius-card: 0.6rem;
  --shadow-card: 0 2px 6px rgb(0 0 0 / 0.25), 0 8px 24px rgb(0 0 0 / 0.15);
}

@media (prefers-color-scheme: dark) {
  :root {
    --color-surface: oklch(0.2 0.01 155);
    --color-surface-2: oklch(0.26 0.02 155);
    --color-ink: oklch(0.95 0.01 155);
    --color-ink-muted: oklch(0.72 0.02 155);
    --color-felt: oklch(0.36 0.07 155);
    --color-felt-deep: oklch(0.24 0.05 155);
  }
}

html {
  font-family: var(--font-body);
  color: var(--color-ink);
  background: var(--color-surface);
}

@media (prefers-reduced-motion: reduce) {
  *,
  *::before,
  *::after {
    animation-duration: 0.01ms !important;
    transition-duration: 0.01ms !important;
  }
}

:focus-visible {
  outline: 3px solid var(--color-accent);
  outline-offset: 2px;
}
```

- [ ] **Step 2: Testes que falham (rotas, navegação, modal, indicador)**

`frontend/src/router.test.ts`:

```ts
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory } from 'vue-router'
import { createAppRouter, sanitizeReturnTo } from './router'
import { useSessionStore } from './stores/session'
import { authApi } from './api/auth'
import { ApiError } from './api/http'

const ana = { id: 'u1', username: 'ana', email: 'a@e.com', displayName: 'Ana', isAdmin: false, createdAt: '' }

beforeEach(() => setActivePinia(createPinia()))

it('sanitizeReturnTo rejeita destinos externos', () => {
  expect(sanitizeReturnTo('/sala/ABC234')).toBe('/sala/ABC234')
  for (const bad of ['https://evil.com', '//evil.com', '/\\evil.com', 'javascript:alert(1)', 42, undefined]) {
    expect(sanitizeReturnTo(bad)).toBe('/')
  }
})

it('manda deslogado para /entrar com volta', async () => {
  vi.spyOn(authApi, 'me').mockRejectedValue(new ApiError(401, 'unauthenticated', 'x'))
  const router = createAppRouter(createMemoryHistory())
  await router.push('/perfil')
  expect(router.currentRoute.value.name).toBe('entrar')
  expect(router.currentRoute.value.query.volta).toBe('/perfil')
})

it('rota de convite normaliza o código e preserva o destino após login', async () => {
  vi.spyOn(authApi, 'me').mockRejectedValue(new ApiError(401, 'unauthenticated', 'x'))
  const router = createAppRouter(createMemoryHistory())
  await router.push('/sala/k7qm2p')
  expect(router.currentRoute.value.query.volta).toBe('/sala/K7QM2P')
})

it('logado em /entrar vai para o lobby', async () => {
  vi.spyOn(authApi, 'me').mockResolvedValue(ana)
  const router = createAppRouter(createMemoryHistory())
  await router.push('/entrar')
  expect(router.currentRoute.value.name).toBe('lobby')
  expect(useSessionStore().user?.id).toBe('u1')
})
```

`frontend/src/app/navigation.test.ts`:

```ts
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory } from 'vue-router'
import { nextTick } from 'vue'
import { createAppRouter } from '@/router'
import { installRoomNavigation } from './navigation'
import { useRoomStore } from '@/stores/room'
import { useToastsStore } from '@/stores/toasts'
import { authApi } from '@/api/auth'
import type { RoomStatePayload } from '@/protocol/messages'

beforeEach(() => {
  setActivePinia(createPinia())
  vi.spyOn(authApi, 'me').mockResolvedValue({ id: 'u1', username: 'ana', email: '', displayName: 'Ana', isAdmin: false, createdAt: '' })
})

const roomState: RoomStatePayload = {
  id: 'r1', code: 'ABC234', name: 'Sala', visibility: 'public', host: 0, status: 'waiting',
  seats: [{ userId: 'u1', displayName: 'Ana', ready: false, connected: true, absent: false }, null, null, null], yourSeat: 0,
}

it('entrar numa sala leva a /sala; ser removido volta ao lobby com aviso', async () => {
  const router = createAppRouter(createMemoryHistory())
  installRoomNavigation(router)
  await router.push('/perfil')
  const room = useRoomStore()
  room.onRoomState(roomState)
  await nextTick()
  await vi.waitFor(() => expect(router.currentRoute.value.name).toBe('sala'))
  room.onLeft({ reason: 'kicked' })
  await nextTick()
  await vi.waitFor(() => expect(router.currentRoute.value.name).toBe('lobby'))
  expect(useToastsStore().items.map((t) => t.text)).toContain('Você foi removido da sala.')
})
```

`frontend/src/components/ui/Modal.test.ts`:

```ts
import { mount } from '@vue/test-utils'
import Modal from './Modal.vue'

it('foca o diálogo, fecha com Esc e devolve o foco', async () => {
  const opener = document.createElement('button')
  document.body.append(opener)
  opener.focus()
  const w = mount(Modal, { props: { open: true, title: 'Confirmar' }, slots: { default: '<button>ok</button>' }, attachTo: document.body })
  await w.vm.$nextTick()
  expect(document.activeElement?.textContent).toBe('ok')
  await w.find('[role="dialog"]').trigger('keydown', { key: 'Escape' })
  expect(w.emitted('close')).toHaveLength(1)
  await w.setProps({ open: false })
  expect(document.activeElement).toBe(opener)
  w.unmount()
})
```

`frontend/src/components/ui/IndicadorConexao.test.ts`:

```ts
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import IndicadorConexao from './IndicadorConexao.vue'
import { useSessionStore } from '@/stores/session'
import { resumeConnection } from '@/app/wire'

vi.mock('@/app/wire', () => ({ resumeConnection: vi.fn() }))

beforeEach(() => setActivePinia(createPinia()))

it('mostra reconectando e oferece retomar quando aberto em outra aba', async () => {
  const s = useSessionStore()
  const w = mount(IndicadorConexao)
  s.setConnection('reconnecting')
  await w.vm.$nextTick()
  expect(w.text()).toContain('Reconectando')
  s.setConnection('replaced')
  await w.vm.$nextTick()
  expect(w.text()).toContain('Aberto em outra aba')
  await w.find('button').trigger('click')
  expect(resumeConnection).toHaveBeenCalled()
})
```

Run: `cd frontend && pnpm vitest run src/router.test.ts src/app/navigation.test.ts src/components/ui`
Expected: FAIL — módulos inexistentes.

- [ ] **Step 3: Componentes de UI**

`frontend/src/components/ui/Botao.vue`:

```vue
<script setup lang="ts">
const props = withDefaults(
  defineProps<{ variant?: 'primary' | 'secondary' | 'danger' | 'ghost'; loading?: boolean; disabled?: boolean; type?: 'button' | 'submit' }>(),
  { variant: 'primary', loading: false, disabled: false, type: 'button' },
)
const styles = {
  primary: 'bg-accent text-ink hover:brightness-105',
  secondary: 'bg-surface-2 text-ink hover:brightness-95',
  danger: 'bg-danger text-white hover:brightness-110',
  ghost: 'bg-transparent text-ink hover:bg-surface-2',
}
</script>

<template>
  <button
    :type="props.type"
    :disabled="props.disabled || props.loading"
    :aria-busy="props.loading || undefined"
    class="inline-flex min-h-10 items-center justify-center gap-2 rounded-lg px-4 font-semibold transition disabled:cursor-not-allowed disabled:opacity-50"
    :class="styles[props.variant]"
  >
    <span v-if="props.loading" class="size-4 animate-spin rounded-full border-2 border-current border-t-transparent" aria-hidden="true" />
    <slot />
  </button>
</template>
```

`frontend/src/components/ui/Campo.vue`:

```vue
<script setup lang="ts">
import { useId } from 'vue'

const props = defineProps<{ label: string; modelValue: string; type?: string; error?: string; autocomplete?: string; required?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
const id = useId()
const errorId = `${id}-erro`
</script>

<template>
  <div class="flex flex-col gap-1">
    <label :for="id" class="text-sm font-medium">{{ props.label }}</label>
    <input
      :id="id"
      :type="props.type ?? 'text'"
      :value="props.modelValue"
      :autocomplete="props.autocomplete"
      :required="props.required"
      :aria-invalid="props.error ? 'true' : undefined"
      :aria-describedby="props.error ? errorId : undefined"
      class="min-h-10 rounded-lg border border-ink-muted/40 bg-surface px-3 text-ink"
      :class="props.error ? 'border-danger' : ''"
      @input="emit('update:modelValue', ($event.target as HTMLInputElement).value)"
    />
    <p v-if="props.error" :id="errorId" class="text-sm text-danger">{{ props.error }}</p>
  </div>
</template>
```

`frontend/src/components/ui/Modal.vue`:

```vue
<script setup lang="ts">
import { nextTick, ref, watch, useId } from 'vue'

const props = defineProps<{ open: boolean; title: string }>()
const emit = defineEmits<{ close: [] }>()
const dialog = ref<HTMLElement | null>(null)
const titleId = useId()
let previous: HTMLElement | null = null

const FOCUSABLE = 'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'

function focusables(): HTMLElement[] {
  return dialog.value ? [...dialog.value.querySelectorAll<HTMLElement>(FOCUSABLE)].filter((el) => !el.hasAttribute('disabled')) : []
}

watch(
  () => props.open,
  async (open) => {
    if (open) {
      previous = document.activeElement as HTMLElement | null
      await nextTick()
      ;(focusables()[0] ?? dialog.value)?.focus()
    } else {
      previous?.focus()
      previous = null
    }
  },
  { immediate: true },
)

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    e.stopPropagation()
    emit('close')
    return
  }
  if (e.key !== 'Tab') return
  const items = focusables()
  if (items.length === 0) return
  const first = items[0]!
  const last = items.at(-1)!
  if (e.shiftKey && document.activeElement === first) {
    e.preventDefault()
    last.focus()
  } else if (!e.shiftKey && document.activeElement === last) {
    e.preventDefault()
    first.focus()
  }
}
</script>

<template>
  <Teleport to="body">
    <div v-if="props.open" class="fixed inset-0 z-50 grid place-items-center bg-black/50 p-4" @click.self="emit('close')">
      <div
        ref="dialog"
        role="dialog"
        aria-modal="true"
        :aria-labelledby="titleId"
        tabindex="-1"
        class="w-full max-w-md rounded-2xl bg-surface p-6 text-ink shadow-card"
        @keydown="onKeydown"
      >
        <h2 :id="titleId" class="mb-4 font-display text-xl">{{ props.title }}</h2>
        <slot />
      </div>
    </div>
  </Teleport>
</template>
```

`frontend/src/components/ui/Toggle.vue`:

```vue
<script setup lang="ts">
const props = defineProps<{ modelValue: boolean; label: string; disabled?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean] }>()
</script>

<template>
  <button
    type="button"
    role="switch"
    :aria-checked="props.modelValue"
    :disabled="props.disabled"
    class="inline-flex items-center gap-2 disabled:opacity-50"
    @click="emit('update:modelValue', !props.modelValue)"
  >
    <span class="relative h-6 w-11 rounded-full transition" :class="props.modelValue ? 'bg-accent' : 'bg-ink-muted/40'">
      <span class="absolute top-0.5 size-5 rounded-full bg-surface shadow transition" :class="props.modelValue ? 'left-5.5' : 'left-0.5'" />
    </span>
    <span>{{ props.label }}</span>
  </button>
</template>
```

`frontend/src/components/ui/Toasts.vue`:

```vue
<script setup lang="ts">
import { useToastsStore } from '@/stores/toasts'
const toasts = useToastsStore()
</script>

<template>
  <div class="pointer-events-none fixed inset-x-0 bottom-4 z-50 flex flex-col items-center gap-2 px-4" aria-live="assertive" role="status">
    <TransitionGroup name="toast">
      <div
        v-for="t in toasts.items"
        :key="t.id"
        class="pointer-events-auto flex max-w-md items-center gap-3 rounded-xl px-4 py-3 shadow-card"
        :class="t.kind === 'error' ? 'bg-danger text-white' : 'bg-surface-2 text-ink'"
      >
        <span>{{ t.text }}</span>
        <button type="button" class="ml-auto opacity-80 hover:opacity-100" aria-label="Fechar aviso" @click="toasts.dismiss(t.id)">✕</button>
      </div>
    </TransitionGroup>
  </div>
</template>

<style scoped>
.toast-enter-active,
.toast-leave-active {
  transition: all 0.2s ease;
}
.toast-enter-from,
.toast-leave-to {
  opacity: 0;
  transform: translateY(8px);
}
</style>
```

`frontend/src/components/ui/IndicadorConexao.vue`:

```vue
<script setup lang="ts">
import { useSessionStore } from '@/stores/session'
import { resumeConnection } from '@/app/wire'
import Botao from './Botao.vue'

const session = useSessionStore()
function reload() {
  location.reload()
}
</script>

<template>
  <div class="flex items-center gap-3 text-sm" role="status">
    <span v-if="session.connection === 'reconnecting' || session.connection === 'connecting'" class="flex items-center gap-2 text-ink-muted">
      <span class="size-2 animate-pulse rounded-full bg-accent" aria-hidden="true" /> Reconectando…
    </span>
    <span v-else-if="session.connection === 'replaced'" class="flex items-center gap-2">
      Aberto em outra aba
      <Botao variant="secondary" @click="resumeConnection()">Usar aqui</Botao>
    </span>
    <span v-if="session.versionMismatch" class="flex items-center gap-2">
      Nova versão disponível
      <Botao variant="secondary" @click="reload">Recarregar</Botao>
    </span>
  </div>
</template>
```

- [ ] **Step 4: Rotas, guarda e navegação por sala**

`frontend/src/router.ts`:

```ts
import { createRouter, createWebHistory, type Router, type RouterHistory } from 'vue-router'
import { useSessionStore } from './stores/session'
import { normalizeCode } from './stores/lobby'

export function sanitizeReturnTo(raw: unknown): string {
  if (typeof raw !== 'string') return '/'
  // Só caminhos internos: começa com "/" e não com "//" ou "/\" (URLs protocol-relative).
  if (!raw.startsWith('/') || raw.startsWith('//') || raw.startsWith('/\\')) return '/'
  return raw
}

export function createAppRouter(history: RouterHistory = createWebHistory()): Router {
  const router = createRouter({
    history,
    routes: [
      { path: '/entrar', name: 'entrar', component: () => import('./views/EntrarView.vue'), meta: { public: true, guestOnly: true } },
      { path: '/cadastro', name: 'cadastro', component: () => import('./views/CadastroView.vue'), meta: { public: true, guestOnly: true } },
      { path: '/verificar-email', name: 'verificar-email', component: () => import('./views/VerificarEmailView.vue'), meta: { public: true } },
      { path: '/esqueci-senha', name: 'esqueci-senha', component: () => import('./views/EsqueciSenhaView.vue'), meta: { public: true } },
      { path: '/redefinir-senha', name: 'redefinir-senha', component: () => import('./views/RedefinirSenhaView.vue'), meta: { public: true } },
      { path: '/', name: 'lobby', component: () => import('./views/LobbyView.vue') },
      { path: '/sala', name: 'sala', component: () => import('./views/SalaView.vue') },
      {
        path: '/sala/:codigo',
        name: 'convite',
        component: () => import('./views/ConviteView.vue'),
        beforeEnter: (to) => {
          const code = normalizeCode(String(to.params.codigo))
          if (code !== to.params.codigo) return { name: 'convite', params: { codigo: code }, replace: true }
        },
      },
      { path: '/perfil', name: 'perfil', component: () => import('./views/PerfilView.vue') },
      { path: '/:pathMatch(.*)*', redirect: '/' },
    ],
  })

  router.beforeEach(async (to) => {
    const session = useSessionStore()
    if (!session.loaded) {
      try {
        await session.load()
      } catch {
        // rede fora: deixa seguir; a tela mostra o erro e o WS tenta reconectar
      }
    }
    if (!to.meta.public && !session.isLoggedIn) {
      return { name: 'entrar', query: { volta: to.fullPath } }
    }
    if (to.meta.guestOnly && session.isLoggedIn) return sanitizeReturnTo(to.query.volta)
  })

  return router
}
```

O `beforeEnter` de convite roda antes do `beforeEach` global? Não — os globais rodam primeiro. Por isso o teste de convite espera `volta=/sala/K7QM2P`: ajuste a ordem normalizando **também** no `beforeEach` antes de redirecionar ao login:

```ts
    if (!to.meta.public && !session.isLoggedIn) {
      const volta = to.name === 'convite' ? `/sala/${normalizeCode(String(to.params.codigo))}` : to.fullPath
      return { name: 'entrar', query: { volta } }
    }
```

`frontend/src/app/navigation.ts`:

```ts
import { watch } from 'vue'
import type { Router } from 'vue-router'
import { useRoomStore } from '@/stores/room'
import { useToastsStore } from '@/stores/toasts'

/** Sala manda na rota: entrar leva a /sala; sair (ou ser removido) volta ao lobby. */
export function installRoomNavigation(router: Router): void {
  const room = useRoomStore()
  watch(
    () => room.inRoom,
    (inRoom) => {
      const name = router.currentRoute.value.name
      if (inRoom && name !== 'sala') void router.push({ name: 'sala' })
      if (!inRoom && (name === 'sala' || name === 'convite')) void router.push({ name: 'lobby' })
    },
  )
  watch(
    () => room.lastLeft,
    (reason) => {
      if (reason === 'kicked') useToastsStore().push('Você foi removido da sala.', 'error')
      if (reason === 'destroyed') useToastsStore().push('A sala foi encerrada.', 'info')
    },
  )
}
```

`frontend/src/views/ConviteView.vue`:

```vue
<script setup lang="ts">
import { onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useLobbyStore } from '@/stores/lobby'
import { useRoomStore } from '@/stores/room'
import { useSessionStore } from '@/stores/session'
import { useToastsStore } from '@/stores/toasts'

const route = useRoute()
const router = useRouter()
const lobby = useLobbyStore()
const room = useRoomStore()
const session = useSessionStore()
const toasts = useToastsStore()

async function join() {
  if (room.inRoom) return // já em sala: a navegação leva a /sala
  try {
    await lobby.joinByCode(String(route.params.codigo))
  } catch (err) {
    const code = (err as { code?: string }).code ?? 'internal'
    toasts.error(code, (err as Error).message)
    void router.replace({ name: 'lobby' })
  }
}

onMounted(() => {
  if (session.connection === 'open') void join()
  else {
    const stop = watch(
      () => session.connection,
      (s) => {
        if (s === 'open') {
          stop()
          void join()
        }
      },
    )
  }
})
</script>

<template>
  <p class="p-8 text-center text-ink-muted" role="status">Entrando na sala {{ route.params.codigo }}…</p>
</template>
```

Lint: `ConviteView` não importa `ws/` nem `api/` — ok.

`frontend/src/components/AppShell.vue`:

```vue
<script setup lang="ts">
import { RouterLink, RouterView } from 'vue-router'
import { useSessionStore } from '@/stores/session'
import IndicadorConexao from './ui/IndicadorConexao.vue'
import Toasts from './ui/Toasts.vue'

const session = useSessionStore()
</script>

<template>
  <div class="flex min-h-dvh flex-col">
    <header v-if="session.isLoggedIn" class="flex items-center gap-4 border-b border-ink-muted/20 px-4 py-2">
      <RouterLink to="/" class="font-display text-xl">Truco</RouterLink>
      <IndicadorConexao class="ml-auto" />
      <RouterLink to="/perfil" class="rounded-lg px-3 py-1 hover:bg-surface-2">{{ session.user?.displayName }}</RouterLink>
    </header>
    <main class="flex flex-1 flex-col">
      <RouterView />
    </main>
    <Toasts />
  </div>
</template>
```

`frontend/src/App.vue`:

```vue
<script setup lang="ts">
import AppShell from './components/AppShell.vue'
</script>

<template>
  <AppShell />
</template>
```

`frontend/src/main.ts`:

```ts
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import { createAppRouter } from './router'
import { installConnection } from './app/wire'
import { installRoomNavigation } from './app/navigation'
import { installSfx } from './audio/install'
import './styles.css'

const app = createApp(App)
const pinia = createPinia()
app.use(pinia)
const router = createAppRouter()
app.use(router)
installSfx()
installConnection(router)
installRoomNavigation(router)
app.mount('#app')
```

Crie views provisórias para que as rotas compilem (`EntrarView`, `CadastroView`, `VerificarEmailView`, `EsqueciSenhaView`, `RedefinirSenhaView`, `LobbyView`, `SalaView`, `PerfilView`), cada uma com `<template><h1 class="p-8 font-display text-2xl">Nome da tela</h1></template>`. Apague `src/smoke.test.ts` (o `App` agora depende do router) ou atualize-o para montar `AppShell` com `global.plugins: [createPinia(), createAppRouter(createMemoryHistory())]`.

- [ ] **Step 5: Rodar e ver passar**

Run: `cd frontend && pnpm vitest run && pnpm typecheck && pnpm lint && pnpm build`
Expected: PASS.

- [ ] **Step 6: Revisão de acessibilidade**

Carregue `fixing-accessibility` e revise `Modal`, `Toasts`, `Campo`, `Toggle`, `IndicadorConexao` (foco visível, rótulos, `role`, contraste dos tokens nos dois temas). Corrija o que apontar e rode de novo o Step 5.

- [ ] **Step 7: Commit**

```bash
git add frontend
git commit -m "feat(frontend): design system, shell, rotas com guarda, convite e navegação por sala"
```

---

### Task 14: Telas de conta

**Skills:** `frontend-design:frontend-design` (antes do Step 3), `ui-ux-pro-max` (estados de erro, carregamento e sucesso), `fixing-accessibility` (ao final).

**Files:**
- Modify: `frontend/src/stores/session.ts` (ações de conta)
- Create/Modify: `frontend/src/views/{Entrar,Cadastro,VerificarEmail,EsqueciSenha,RedefinirSenha,Perfil}View.vue`
- Create: `frontend/src/components/conta/CartaoConta.vue` (moldura comum das telas públicas)
- Create: `frontend/src/views/contas.test.ts`
- Create: `frontend/src/app/formError.ts`

**Interfaces:**
- Consumes: `authApi` (via session), `ApiError`, `errorText`, `sanitizeReturnTo`, `usePrefsStore`, `cardUrl`, `backUrl`, UI (Task 13).
- Produces:
  - `useSessionStore()` ganha: `register(input: RegisterInput)`, `verifyEmail(token)`, `resendVerification(email)`, `forgotPassword(email)`, `resetPassword(token, password)` (repasses de `authApi`).
  - `formError(err: unknown): { message: string; fields: Record<string, string> }` em `app/formError.ts` — `message` via `errorText(code, serverMessage)`; `fields` vindos de `ApiError.fields`.

- [ ] **Step 1: Testes que falham**

`frontend/src/views/contas.test.ts`:

```ts
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory } from 'vue-router'
import { createAppRouter } from '@/router'
import EntrarView from './EntrarView.vue'
import CadastroView from './CadastroView.vue'
import VerificarEmailView from './VerificarEmailView.vue'
import RedefinirSenhaView from './RedefinirSenhaView.vue'
import { authApi } from '@/api/auth'
import { ApiError } from '@/api/http'

const ana = { id: 'u1', username: 'ana', email: 'ana@e.com', displayName: 'Ana', isAdmin: false, createdAt: '' }

async function mountAt(component: object, path: string) {
  const pinia = createPinia()
  setActivePinia(pinia)
  vi.spyOn(authApi, 'me').mockRejectedValue(new ApiError(401, 'unauthenticated', 'x'))
  const router = createAppRouter(createMemoryHistory())
  await router.push(path)
  const w = mount(component, { global: { plugins: [pinia, router] }, attachTo: document.body })
  return { w, router }
}

it('login com erro mostra o texto do código', async () => {
  vi.spyOn(authApi, 'login').mockRejectedValue(new ApiError(401, 'invalid_credentials', 'x'))
  const { w } = await mountAt(EntrarView, '/entrar')
  await w.find('input[autocomplete="username"]').setValue('ana')
  await w.find('input[type="password"]').setValue('senha-errada')
  await w.find('form').trigger('submit')
  await flushPromises()
  expect(w.text()).toContain('Usuário ou senha incorretos.')
})

it('e-mail não verificado oferece reenviar', async () => {
  vi.spyOn(authApi, 'login').mockRejectedValue(new ApiError(403, 'email_not_verified', 'x'))
  const resend = vi.spyOn(authApi, 'resendVerification').mockResolvedValue()
  const { w } = await mountAt(EntrarView, '/entrar')
  await w.find('input[autocomplete="username"]').setValue('ana@e.com')
  await w.find('input[type="password"]').setValue('x')
  await w.find('form').trigger('submit')
  await flushPromises()
  await w.findAll('button').find((b) => b.text().includes('Reenviar'))!.trigger('click')
  expect(resend).toHaveBeenCalledWith('ana@e.com')
})

it('login ok volta para o destino interno', async () => {
  vi.spyOn(authApi, 'login').mockResolvedValue(ana)
  const { w, router } = await mountAt(EntrarView, '/entrar?volta=/perfil')
  await w.find('input[autocomplete="username"]').setValue('ana')
  await w.find('input[type="password"]').setValue('senha-certa-123')
  await w.find('form').trigger('submit')
  await flushPromises()
  expect(router.currentRoute.value.path).toBe('/perfil')
})

it('cadastro mostra erros de campo do servidor', async () => {
  vi.spyOn(authApi, 'register').mockRejectedValue(new ApiError(400, 'validation_failed', 'x', { password: 'a senha deve ter de 10 a 128 caracteres' }))
  const { w } = await mountAt(CadastroView, '/cadastro')
  await w.find('form').trigger('submit')
  await flushPromises()
  expect(w.text()).toContain('a senha deve ter de 10 a 128 caracteres')
})

it('cadastro ok mostra "confira seu e-mail"', async () => {
  vi.spyOn(authApi, 'register').mockResolvedValue()
  const { w } = await mountAt(CadastroView, '/cadastro')
  await w.find('form').trigger('submit')
  await flushPromises()
  expect(w.text()).toContain('Confira seu e-mail')
})

it('verificação faz POST do token ao abrir e trata link expirado', async () => {
  const verify = vi.spyOn(authApi, 'verifyEmail').mockRejectedValue(new ApiError(400, 'invalid_token', 'x'))
  const { w } = await mountAt(VerificarEmailView, '/verificar-email?token=abc')
  await flushPromises()
  expect(verify).toHaveBeenCalledWith('abc')
  expect(w.text()).toContain('Link inválido ou expirado.')
})

it('redefinição envia token e nova senha', async () => {
  const reset = vi.spyOn(authApi, 'resetPassword').mockResolvedValue()
  const { w } = await mountAt(RedefinirSenhaView, '/redefinir-senha?token=tok')
  const [nova, confirma] = w.findAll('input[type="password"]')
  await nova!.setValue('nova-senha-segura')
  await confirma!.setValue('nova-senha-segura')
  await w.find('form').trigger('submit')
  await flushPromises()
  expect(reset).toHaveBeenCalledWith('tok', 'nova-senha-segura')
  expect(w.text()).toContain('Senha redefinida')
})

it('redefinição bloqueia senhas diferentes sem chamar a API', async () => {
  const reset = vi.spyOn(authApi, 'resetPassword').mockResolvedValue()
  const { w } = await mountAt(RedefinirSenhaView, '/redefinir-senha?token=tok')
  const [nova, confirma] = w.findAll('input[type="password"]')
  await nova!.setValue('nova-senha-segura')
  await confirma!.setValue('outra-senha-qualquer')
  await w.find('form').trigger('submit')
  expect(reset).not.toHaveBeenCalled()
  expect(w.text()).toContain('As senhas não conferem.')
})
```

Run: `cd frontend && pnpm vitest run src/views/contas.test.ts`
Expected: FAIL.

- [ ] **Step 2: Ações de conta na sessão e `formError`**

Acrescente em `useSessionStore().actions`:

```ts
    register(input: RegisterInput) {
      return authApi.register(input)
    },
    verifyEmail(token: string) {
      return authApi.verifyEmail(token)
    },
    resendVerification(email: string) {
      return authApi.resendVerification(email)
    },
    forgotPassword(email: string) {
      return authApi.forgotPassword(email)
    },
    resetPassword(token: string, password: string) {
      return authApi.resetPassword(token, password)
    },
```

(importe `type RegisterInput` de `@/api/auth`).

`frontend/src/app/formError.ts`:

```ts
import { ApiError } from '@/api/http'
import { errorText } from '@/game/labels'

export function formError(err: unknown): { message: string; fields: Record<string, string> } {
  if (err instanceof ApiError) return { message: errorText(err.code, err.message), fields: err.fields }
  return { message: errorText('internal'), fields: {} }
}
```

(`app/` pode importar `api/`; as views importam só `formError`.)

- [ ] **Step 3: Views**

Carregue `frontend-design:frontend-design` e `ui-ux-pro-max` e desenhe a moldura das telas públicas (logo/título, cartão central sobre o feltro, link de troca entre entrar/cadastrar). Implementação base:

`frontend/src/components/conta/CartaoConta.vue`:

```vue
<script setup lang="ts">
defineProps<{ title: string }>()
</script>

<template>
  <div class="grid min-h-dvh place-items-center bg-felt p-4">
    <section class="w-full max-w-sm rounded-2xl bg-surface p-6 shadow-card">
      <h1 class="mb-6 font-display text-3xl">{{ title }}</h1>
      <slot />
    </section>
  </div>
</template>
```

`frontend/src/views/EntrarView.vue`:

```vue
<script setup lang="ts">
import { ref } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { useSessionStore } from '@/stores/session'
import { useToastsStore } from '@/stores/toasts'
import { sanitizeReturnTo } from '@/router'
import { formError } from '@/app/formError'
import CartaoConta from '@/components/conta/CartaoConta.vue'
import Campo from '@/components/ui/Campo.vue'
import Botao from '@/components/ui/Botao.vue'

const session = useSessionStore()
const toasts = useToastsStore()
const route = useRoute()
const router = useRouter()
const login = ref('')
const password = ref('')
const loading = ref(false)
const error = ref('')
const notVerified = ref(false)

async function submit() {
  loading.value = true
  error.value = ''
  notVerified.value = false
  try {
    await session.login(login.value.trim(), password.value)
    await router.replace(sanitizeReturnTo(route.query.volta))
  } catch (err) {
    error.value = formError(err).message
    notVerified.value = (err as { code?: string }).code === 'email_not_verified'
  } finally {
    loading.value = false
  }
}

async function resend() {
  try {
    await session.resendVerification(login.value.trim())
    toasts.push('Se o e-mail estiver cadastrado, enviamos um novo link.')
  } catch (err) {
    toasts.push(formError(err).message, 'error')
  }
}
</script>

<template>
  <CartaoConta title="Entrar">
    <form class="flex flex-col gap-4" novalidate @submit.prevent="submit">
      <Campo v-model="login" label="Usuário ou e-mail" autocomplete="username" required />
      <Campo v-model="password" label="Senha" type="password" autocomplete="current-password" required />
      <p v-if="error" class="text-sm text-danger" role="alert">{{ error }}</p>
      <Botao v-if="notVerified" variant="secondary" @click="resend">Reenviar e-mail de confirmação</Botao>
      <Botao type="submit" :loading="loading">Entrar</Botao>
    </form>
    <nav class="mt-6 flex justify-between text-sm">
      <RouterLink to="/cadastro" class="underline">Criar conta</RouterLink>
      <RouterLink to="/esqueci-senha" class="underline">Esqueci minha senha</RouterLink>
    </nav>
  </CartaoConta>
</template>
```

O botão de reenvio só aparece no caso `email_not_verified` (o teste espera que o campo `login` seja o e-mail nesse caso; se o usuário digitou o nome de usuário, o backend responde 202 neutro do mesmo jeito).

`frontend/src/views/CadastroView.vue`:

```vue
<script setup lang="ts">
import { reactive, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useSessionStore } from '@/stores/session'
import { useToastsStore } from '@/stores/toasts'
import { formError } from '@/app/formError'
import CartaoConta from '@/components/conta/CartaoConta.vue'
import Campo from '@/components/ui/Campo.vue'
import Botao from '@/components/ui/Botao.vue'

const session = useSessionStore()
const toasts = useToastsStore()
const form = reactive({ username: '', email: '', displayName: '', password: '' })
const fields = ref<Record<string, string>>({})
const error = ref('')
const loading = ref(false)
const done = ref(false)

async function submit() {
  loading.value = true
  error.value = ''
  fields.value = {}
  try {
    await session.register({ ...form, username: form.username.trim(), email: form.email.trim(), displayName: form.displayName.trim() })
    done.value = true
  } catch (err) {
    const f = formError(err)
    error.value = f.message
    fields.value = f.fields
  } finally {
    loading.value = false
  }
}

async function resend() {
  try {
    await session.resendVerification(form.email.trim())
    toasts.push('Enviamos um novo link, se o e-mail estiver cadastrado.')
  } catch (err) {
    toasts.push(formError(err).message, 'error')
  }
}
</script>

<template>
  <CartaoConta :title="done ? 'Confira seu e-mail' : 'Criar conta'">
    <div v-if="done" class="flex flex-col gap-4">
      <p>Enviamos um link de confirmação para <strong>{{ form.email }}</strong>. Abra o e-mail para ativar a conta.</p>
      <Botao variant="secondary" @click="resend">Reenviar e-mail</Botao>
      <RouterLink to="/entrar" class="text-center underline">Ir para o login</RouterLink>
    </div>
    <form v-else class="flex flex-col gap-4" novalidate @submit.prevent="submit">
      <Campo v-model="form.username" label="Usuário (3 a 20: a-z, 0-9, _ e .)" autocomplete="username" :error="fields.username" required />
      <Campo v-model="form.email" label="E-mail da empresa" type="email" autocomplete="email" :error="fields.email" required />
      <Campo v-model="form.displayName" label="Nome de exibição" autocomplete="nickname" :error="fields.displayName" required />
      <Campo v-model="form.password" label="Senha (mín. 10 caracteres)" type="password" autocomplete="new-password" :error="fields.password" required />
      <p v-if="error" class="text-sm text-danger" role="alert">{{ error }}</p>
      <Botao type="submit" :loading="loading">Criar conta</Botao>
      <RouterLink to="/entrar" class="text-center text-sm underline">Já tenho conta</RouterLink>
    </form>
  </CartaoConta>
</template>
```

`frontend/src/views/VerificarEmailView.vue`:

```vue
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useSessionStore } from '@/stores/session'
import { formError } from '@/app/formError'
import CartaoConta from '@/components/conta/CartaoConta.vue'

const session = useSessionStore()
const route = useRoute()
const state = ref<'loading' | 'ok' | 'error'>('loading')
const message = ref('')

onMounted(async () => {
  const token = typeof route.query.token === 'string' ? route.query.token : ''
  if (!token) {
    state.value = 'error'
    message.value = 'Link inválido ou expirado.'
    return
  }
  try {
    await session.verifyEmail(token)
    state.value = 'ok'
  } catch (err) {
    state.value = 'error'
    message.value = formError(err).message
  }
})
</script>

<template>
  <CartaoConta title="Confirmação de e-mail">
    <p v-if="state === 'loading'" role="status">Confirmando…</p>
    <div v-else-if="state === 'ok'" class="flex flex-col gap-4">
      <p>E-mail confirmado! Agora é só entrar.</p>
      <RouterLink to="/entrar" class="underline">Entrar</RouterLink>
    </div>
    <div v-else class="flex flex-col gap-4" role="alert">
      <p>{{ message }}</p>
      <p class="text-sm">Peça um novo link na tela de login, em "Reenviar e-mail de confirmação".</p>
      <RouterLink to="/entrar" class="underline">Ir para o login</RouterLink>
    </div>
  </CartaoConta>
</template>
```

`frontend/src/views/EsqueciSenhaView.vue`:

```vue
<script setup lang="ts">
import { ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useSessionStore } from '@/stores/session'
import { formError } from '@/app/formError'
import CartaoConta from '@/components/conta/CartaoConta.vue'
import Campo from '@/components/ui/Campo.vue'
import Botao from '@/components/ui/Botao.vue'

const session = useSessionStore()
const email = ref('')
const sent = ref(false)
const error = ref('')
const loading = ref(false)

async function submit() {
  loading.value = true
  error.value = ''
  try {
    await session.forgotPassword(email.value.trim())
    sent.value = true
  } catch (err) {
    error.value = formError(err).message
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <CartaoConta title="Esqueci minha senha">
    <p v-if="sent" role="status">Se o e-mail existir, enviamos um link para redefinir a senha.</p>
    <form v-else class="flex flex-col gap-4" novalidate @submit.prevent="submit">
      <Campo v-model="email" label="E-mail" type="email" autocomplete="email" required />
      <p v-if="error" class="text-sm text-danger" role="alert">{{ error }}</p>
      <Botao type="submit" :loading="loading">Enviar link</Botao>
    </form>
    <RouterLink to="/entrar" class="mt-6 block text-sm underline">Voltar ao login</RouterLink>
  </CartaoConta>
</template>
```

`frontend/src/views/RedefinirSenhaView.vue`:

```vue
<script setup lang="ts">
import { ref } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useSessionStore } from '@/stores/session'
import { formError } from '@/app/formError'
import CartaoConta from '@/components/conta/CartaoConta.vue'
import Campo from '@/components/ui/Campo.vue'
import Botao from '@/components/ui/Botao.vue'

const session = useSessionStore()
const route = useRoute()
const password = ref('')
const confirm = ref('')
const error = ref('')
const fields = ref<Record<string, string>>({})
const loading = ref(false)
const done = ref(false)
const invalidLink = ref(false)

async function submit() {
  error.value = ''
  fields.value = {}
  if (password.value !== confirm.value) {
    error.value = 'As senhas não conferem.'
    return
  }
  const token = typeof route.query.token === 'string' ? route.query.token : ''
  loading.value = true
  try {
    await session.resetPassword(token, password.value)
    done.value = true
  } catch (err) {
    const f = formError(err)
    error.value = f.message
    fields.value = f.fields
    invalidLink.value = (err as { code?: string }).code === 'invalid_token'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <CartaoConta title="Redefinir senha">
    <div v-if="done" class="flex flex-col gap-4" role="status">
      <p>Senha redefinida. Entre com a nova senha.</p>
      <RouterLink to="/entrar" class="underline">Entrar</RouterLink>
    </div>
    <form v-else class="flex flex-col gap-4" novalidate @submit.prevent="submit">
      <Campo v-model="password" label="Nova senha (mín. 10 caracteres)" type="password" autocomplete="new-password" :error="fields.password" required />
      <Campo v-model="confirm" label="Confirme a nova senha" type="password" autocomplete="new-password" required />
      <p v-if="error" class="text-sm text-danger" role="alert">{{ error }}</p>
      <RouterLink v-if="invalidLink" to="/esqueci-senha" class="text-sm underline">Pedir um novo link</RouterLink>
      <Botao type="submit" :loading="loading">Redefinir</Botao>
    </form>
  </CartaoConta>
</template>
```

`frontend/src/views/PerfilView.vue`:

```vue
<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useSessionStore } from '@/stores/session'
import { usePrefsStore } from '@/stores/prefs'
import { useToastsStore } from '@/stores/toasts'
import { formError } from '@/app/formError'
import { cardUrl } from '@/game/cardAssets'
import Campo from '@/components/ui/Campo.vue'
import Botao from '@/components/ui/Botao.vue'
import Toggle from '@/components/ui/Toggle.vue'

const session = useSessionStore()
const prefs = usePrefsStore()
const toasts = useToastsStore()
const router = useRouter()
const displayName = ref(session.user?.displayName ?? '')
const nameError = ref('')
const current = ref('')
const next = ref('')
const pwError = ref('')
const PREVIEW = [28, 8, 39] // A, 6, 3 — conferir os ids no cards.json e trocar por cartas bonitas

async function saveName() {
  nameError.value = ''
  try {
    await session.updateDisplayName(displayName.value.trim())
    toasts.push('Nome atualizado.')
  } catch (err) {
    const f = formError(err)
    nameError.value = f.fields.displayName ?? f.message
  }
}

async function savePassword() {
  pwError.value = ''
  try {
    await session.changePassword(current.value, next.value)
    current.value = ''
    next.value = ''
    toasts.push('Senha alterada.')
  } catch (err) {
    const f = formError(err)
    pwError.value = f.fields.newPassword ?? f.message
  }
}

async function logout(all: boolean) {
  await (all ? session.logoutAll() : session.logout())
  await router.replace({ name: 'entrar' })
}
</script>

<template>
  <div class="mx-auto flex w-full max-w-2xl flex-col gap-8 p-4">
    <h1 class="font-display text-3xl">Perfil</h1>

    <section class="flex flex-col gap-3">
      <h2 class="text-lg font-semibold">Nome de exibição</h2>
      <form class="flex items-end gap-3" @submit.prevent="saveName">
        <Campo v-model="displayName" class="flex-1" label="Como você aparece na mesa" :error="nameError" />
        <Botao type="submit">Salvar</Botao>
      </form>
    </section>

    <section class="flex flex-col gap-3">
      <h2 class="text-lg font-semibold">Preferências</h2>
      <fieldset class="flex flex-col gap-2">
        <legend class="text-sm font-medium">Baralho</legend>
        <div class="flex gap-4">
          <label v-for="deck in (['frances', 'espanhol'] as const)" :key="deck" class="flex cursor-pointer flex-col items-center gap-2">
            <span class="flex">
              <img v-for="id in PREVIEW" :key="id" :src="cardUrl(id, deck)" alt="" class="-ml-6 h-24 first:ml-0" />
            </span>
            <span><input type="radio" name="deck" :value="deck" :checked="prefs.deck === deck" @change="prefs.setDeck(deck)" /> {{ deck === 'frances' ? 'Francês' : 'Espanhol' }}</span>
          </label>
        </div>
      </fieldset>
      <label class="flex items-center gap-3">
        Volume
        <input type="range" min="0" max="1" step="0.05" :value="prefs.volume" @input="prefs.setVolume(Number(($event.target as HTMLInputElement).value))" />
      </label>
      <Toggle :model-value="prefs.muted" label="Sem som" @update:model-value="prefs.setMuted($event)" />
    </section>

    <section class="flex flex-col gap-3">
      <h2 class="text-lg font-semibold">Trocar senha</h2>
      <form class="flex flex-col gap-3" @submit.prevent="savePassword">
        <Campo v-model="current" label="Senha atual" type="password" autocomplete="current-password" />
        <Campo v-model="next" label="Nova senha" type="password" autocomplete="new-password" :error="pwError" />
        <Botao type="submit" class="self-start">Trocar senha</Botao>
      </form>
    </section>

    <section class="flex gap-3">
      <Botao variant="secondary" @click="logout(false)">Sair</Botao>
      <Botao variant="danger" @click="logout(true)">Sair de todos os dispositivos</Botao>
    </section>

    <footer class="text-sm text-ink-muted">
      Créditos: cartas francesas <em>Vector Playing Cards</em> (Byron Knoll, domínio público); sons da Kenney (CC0); fontes OFL.
    </footer>
  </div>
</template>
```

- [ ] **Step 4: Rodar e ver passar**

Run: `cd frontend && pnpm vitest run && pnpm typecheck && pnpm lint`
Expected: PASS.

- [ ] **Step 5: Acessibilidade e conferência manual**

Carregue `fixing-accessibility`. Suba `make dev-db`, `make run` (com `MAIL_DRIVER=log`) e `make web-dev`; percorra cadastro → link do log → verificação → login → perfil → sair → esqueci senha → redefinir. Corrija o que a skill apontar.

- [ ] **Step 6: Commit**

```bash
git add frontend/src
git commit -m "feat(frontend): telas de cadastro, login, verificação, redefinição de senha e perfil"
```

---

### Task 15: Lobby

**Skills:** `frontend-design:frontend-design` (antes do Step 3), `ui-ux-pro-max` (lista vazia, carregando, salas cheias), `fixing-accessibility` (ao final).

**Files:**
- Modify: `frontend/src/views/LobbyView.vue`
- Create: `frontend/src/components/lobby/{ListaSalas,LinhaSala,CriarSala,EntrarComCodigo}.vue`
- Create: `frontend/src/components/lobby/lobby.test.ts`

**Interfaces:**
- Consumes: `useLobbyStore` (`list`, `loaded`, `createRoom`, `joinById`, `joinByCode`), `useToastsStore`, `formError`, `teamOf`, UI.
- Produces: `LinhaSala.vue` props `{ room: RoomSummary }`, emite `join: [seat?: Seat]`; `CriarSala.vue` (botões pública/privada); `EntrarComCodigo.vue` (campo + botão).

- [ ] **Step 1: Testes que falham**

`frontend/src/components/lobby/lobby.test.ts`:

```ts
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setSender } from '@/app/transport'
import LobbyView from '@/views/LobbyView.vue'
import { useLobbyStore } from '@/stores/lobby'
import { WsSendError } from '@/ws/client'

const room = (id: string, over = {}) => ({ id, name: `Sala ${id}`, status: 'waiting', seats: [{ displayName: 'Ana' }, null, null, null], ...over })

let pinia: ReturnType<typeof createPinia>
beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
})
afterEach(() => setSender(null))

it('mostra carregando, depois lista vazia', async () => {
  const w = mount(LobbyView, { global: { plugins: [pinia] } })
  expect(w.text()).toContain('Carregando salas')
  useLobbyStore().onSnapshot({ rooms: [] })
  await w.vm.$nextTick()
  expect(w.text()).toContain('Nenhuma sala aberta')
})

it('clicar num lugar vazio entra naquele assento; sala em jogo fica desabilitada', async () => {
  const send = vi.fn().mockResolvedValue(undefined)
  setSender(send)
  useLobbyStore().onSnapshot({ rooms: [room('a'), room('b', { status: 'playing', score: [3, 5] })] })
  const w = mount(LobbyView, { global: { plugins: [pinia] } })
  await w.find('[data-sala="a"] [data-assento="2"]').trigger('click')
  expect(send).toHaveBeenCalledWith({ type: 'join_room', payload: { roomId: 'a', seat: 2 } })
  expect(w.find('[data-sala="b"] button').attributes('disabled')).toBeDefined()
  expect(w.find('[data-sala="b"]').text()).toContain('3 × 5')
})

it('entrar com código e criar sala', async () => {
  const send = vi.fn().mockResolvedValue(undefined)
  setSender(send)
  useLobbyStore().onSnapshot({ rooms: [] })
  const w = mount(LobbyView, { global: { plugins: [pinia] } })
  await w.find('input[name="codigo"]').setValue('abc 234')
  await w.find('form[data-form="codigo"]').trigger('submit')
  await w.find('[data-criar="private"]').trigger('click')
  expect(send.mock.calls.map((c) => c[0])).toEqual([
    { type: 'join_room', payload: { code: 'ABC234' } },
    { type: 'create_room', payload: { visibility: 'private' } },
  ])
})

it('erro ao entrar vira toast', async () => {
  setSender(vi.fn().mockRejectedValue(new WsSendError('room_full', 'x')))
  useLobbyStore().onSnapshot({ rooms: [room('a')] })
  const w = mount(LobbyView, { global: { plugins: [pinia] } })
  await w.find('[data-sala="a"] [data-assento="1"]').trigger('click')
  await flushPromises()
  const { useToastsStore } = await import('@/stores/toasts')
  expect(useToastsStore().items.map((t) => t.text)).toEqual(['A sala está cheia.'])
})
```

Run: `cd frontend && pnpm vitest run src/components/lobby`
Expected: FAIL.

- [ ] **Step 2: Implementar**

`frontend/src/components/lobby/LinhaSala.vue`:

```vue
<script setup lang="ts">
import { computed } from 'vue'
import type { RoomSummary } from '@/protocol/messages'
import type { Seat } from '@/protocol/engine'
import { teamOf } from '@/game/seats'

const props = defineProps<{ room: RoomSummary }>()
const emit = defineEmits<{ join: [seat?: Seat] }>()
const open = computed(() => props.room.status === 'waiting' && props.room.seats.some((s) => s === null))
</script>

<template>
  <li :data-sala="room.id" class="flex flex-wrap items-center gap-4 rounded-xl bg-surface-2 p-4">
    <div class="min-w-40 flex-1">
      <p class="font-semibold">{{ room.name }}</p>
      <p class="text-sm text-ink-muted">
        <template v-if="room.status === 'playing'">Em jogo<span v-if="room.score"> · {{ room.score[0] }} × {{ room.score[1] }}</span></template>
        <template v-else-if="open">Aguardando jogadores</template>
        <template v-else>Cheia</template>
      </p>
    </div>
    <ul class="grid grid-cols-4 gap-2" aria-label="Lugares">
      <li v-for="(seat, i) in room.seats" :key="i">
        <button
          type="button"
          :data-assento="i"
          :disabled="!open || seat !== null"
          class="flex h-12 w-24 flex-col items-center justify-center rounded-lg border-2 text-xs disabled:cursor-default"
          :class="teamOf(i) === 0 ? 'border-team-0' : 'border-team-1'"
          :aria-label="seat ? `Lugar ${i + 1}: ${seat.displayName}` : `Sentar no lugar ${i + 1}`"
          @click="emit('join', i)"
        >
          <span v-if="seat" class="max-w-full truncate px-1">{{ seat.displayName }}</span>
          <span v-else class="text-ink-muted">Livre</span>
        </button>
      </li>
    </ul>
    <button type="button" class="sr-only" :disabled="!open" @click="emit('join')">Entrar em {{ room.name }}</button>
  </li>
</template>
```

(O teste de sala em jogo usa `[data-sala="b"] button` — o primeiro botão é um assento desabilitado; mantenha os assentos antes do botão `sr-only`.)

`frontend/src/components/lobby/ListaSalas.vue`:

```vue
<script setup lang="ts">
import type { Seat } from '@/protocol/engine'
import { useLobbyStore } from '@/stores/lobby'
import { useToastsStore } from '@/stores/toasts'
import LinhaSala from './LinhaSala.vue'

const lobby = useLobbyStore()
const toasts = useToastsStore()

async function join(roomId: string, seat?: Seat) {
  try {
    await lobby.joinById(roomId, seat)
  } catch (err) {
    toasts.error((err as { code?: string }).code ?? 'internal', (err as Error).message)
  }
}
</script>

<template>
  <section aria-labelledby="salas-titulo" class="flex flex-col gap-3">
    <h2 id="salas-titulo" class="text-lg font-semibold">Salas abertas</h2>
    <p v-if="!lobby.loaded" role="status" class="text-ink-muted">Carregando salas…</p>
    <p v-else-if="lobby.list.length === 0" class="text-ink-muted">Nenhuma sala aberta. Crie uma e chame a galera!</p>
    <ul v-else class="flex flex-col gap-3">
      <LinhaSala v-for="r in lobby.list" :key="r.id" :room="r" @join="(seat) => join(r.id, seat)" />
    </ul>
  </section>
</template>
```

`frontend/src/components/lobby/CriarSala.vue`:

```vue
<script setup lang="ts">
import { ref } from 'vue'
import type { Visibility } from '@/protocol/messages'
import { useLobbyStore } from '@/stores/lobby'
import { useToastsStore } from '@/stores/toasts'
import Botao from '@/components/ui/Botao.vue'

const lobby = useLobbyStore()
const toasts = useToastsStore()
const busy = ref(false)

async function create(visibility: Visibility) {
  busy.value = true
  try {
    await lobby.createRoom(visibility)
  } catch (err) {
    toasts.error((err as { code?: string }).code ?? 'internal', (err as Error).message)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="flex flex-wrap gap-3">
    <Botao data-criar="public" :loading="busy" @click="create('public')">Criar sala pública</Botao>
    <Botao data-criar="private" variant="secondary" :disabled="busy" @click="create('private')">Criar sala privada</Botao>
  </div>
</template>
```

`frontend/src/components/lobby/EntrarComCodigo.vue`:

```vue
<script setup lang="ts">
import { ref } from 'vue'
import { useLobbyStore } from '@/stores/lobby'
import { useToastsStore } from '@/stores/toasts'
import Botao from '@/components/ui/Botao.vue'

const lobby = useLobbyStore()
const toasts = useToastsStore()
const code = ref('')

async function submit() {
  if (!code.value.trim()) return
  try {
    await lobby.joinByCode(code.value)
  } catch (err) {
    toasts.error((err as { code?: string }).code ?? 'internal', (err as Error).message)
  }
}
</script>

<template>
  <form data-form="codigo" class="flex items-end gap-2" @submit.prevent="submit">
    <label class="flex flex-col gap-1 text-sm font-medium">
      Código da sala
      <input
        v-model="code"
        name="codigo"
        maxlength="8"
        autocomplete="off"
        autocapitalize="characters"
        class="min-h-10 w-32 rounded-lg border border-ink-muted/40 bg-surface px-3 uppercase tracking-widest"
      />
    </label>
    <Botao type="submit" variant="secondary">Entrar</Botao>
  </form>
</template>
```

`frontend/src/views/LobbyView.vue`:

```vue
<script setup lang="ts">
import ListaSalas from '@/components/lobby/ListaSalas.vue'
import CriarSala from '@/components/lobby/CriarSala.vue'
import EntrarComCodigo from '@/components/lobby/EntrarComCodigo.vue'
</script>

<template>
  <div class="mx-auto flex w-full max-w-4xl flex-col gap-6 p-4">
    <div class="flex flex-wrap items-end justify-between gap-4">
      <h1 class="font-display text-3xl">Lobby</h1>
      <EntrarComCodigo />
    </div>
    <CriarSala />
    <ListaSalas />
  </div>
</template>
```

- [ ] **Step 3: Visual e acessibilidade**

Aplique `frontend-design:frontend-design` à lista (densidade, cores de dupla, estado vazio com ilustração simples de cartas) e `fixing-accessibility` (rótulos dos assentos, ordem de tab). Rode `pnpm vitest run src/components/lobby` de novo após os ajustes.

- [ ] **Step 4: Rodar e ver passar**

Run: `cd frontend && pnpm vitest run && pnpm typecheck && pnpm lint`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src
git commit -m "feat(frontend): lobby ao vivo com criar sala, entrar por lugar e por código"
```

---

### Task 16: Sala de espera

**Skills:** `frontend-design:frontend-design` (antes do Step 2), `fixing-accessibility` (ao final).

**Files:**
- Modify: `frontend/src/views/SalaView.vue`
- Create: `frontend/src/components/sala/{SalaEspera,Assento}.vue`, `frontend/src/components/sala/sala.test.ts`

**Interfaces:**
- Consumes: `useRoomStore` (`room`, `isHost`, `amReady`, `takeSeat`, `setReady`, `kick`, `leave`, `isPlaying`), `useToastsStore`, `relativePosition`, `teamOf`, UI. A `Mesa` (Tarefa 17) entra no `SalaView` quando `room.isPlaying`; até lá, mostre um placeholder "Partida em andamento".
- Produces: `SalaView.vue` alterna `SalaEspera` / `Mesa` (por `room.status`) e mostra o modal `FimPartida` (Tarefa 19) quando `room.ended`; `Assento.vue` props `{ seat: Seat; info: SeatInfo | null; isMe: boolean; isHost: boolean; canKick: boolean }`, emite `take`, `kick`.

- [ ] **Step 1: Testes que falham**

`frontend/src/components/sala/sala.test.ts`:

```ts
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setSender } from '@/app/transport'
import SalaEspera from './SalaEspera.vue'
import { useRoomStore } from '@/stores/room'
import type { RoomStatePayload } from '@/protocol/messages'

const base: RoomStatePayload = {
  id: 'r1', code: 'ABC234', name: 'Sala da Ana', visibility: 'private', host: 0, status: 'waiting',
  seats: [
    { userId: 'u1', displayName: 'Ana', ready: false, connected: true, absent: false },
    { userId: 'u2', displayName: 'Bia', ready: true, connected: false, absent: false },
    null,
    null,
  ],
  yourSeat: 0,
}

let pinia: ReturnType<typeof createPinia>
beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
})
afterEach(() => setSender(null))

it('mostra código, presença e pronto; host pode remover', async () => {
  const send = vi.fn().mockResolvedValue(undefined)
  setSender(send)
  useRoomStore().onRoomState(base)
  const w = mount(SalaEspera, { global: { plugins: [pinia] } })
  expect(w.text()).toContain('ABC234')
  expect(w.find('[data-assento="1"]').text()).toContain('Pronto')
  expect(w.find('[data-assento="1"]').text()).toContain('Desconectado')
  await w.find('[data-assento="1"] [data-acao="remover"]').trigger('click')
  await w.find('[data-assento="2"] [data-acao="sentar"]').trigger('click')
  await w.find('[role="switch"]').trigger('click')
  expect(send.mock.calls.map((c) => c[0])).toEqual([
    { type: 'kick', payload: { seat: 1 } },
    { type: 'take_seat', payload: { seat: 2 } },
    { type: 'set_ready', payload: { ready: true } },
  ])
})

it('quem não é host não vê remover; sala pública não mostra código', () => {
  useRoomStore().onRoomState({ ...base, visibility: 'public', yourSeat: 1 })
  const w = mount(SalaEspera, { global: { plugins: [pinia] } })
  expect(w.find('[data-acao="remover"]').exists()).toBe(false)
  expect(w.text()).not.toContain('ABC234')
})

it('copia o link de convite', async () => {
  const writeText = vi.fn().mockResolvedValue(undefined)
  Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
  useRoomStore().onRoomState(base)
  const w = mount(SalaEspera, { global: { plugins: [pinia] } })
  await w.find('[data-acao="copiar"]').trigger('click')
  expect(writeText).toHaveBeenCalledWith(`${location.origin}/sala/ABC234`)
})
```

Run: `cd frontend && pnpm vitest run src/components/sala`
Expected: FAIL.

- [ ] **Step 2: Implementar**

`frontend/src/components/sala/Assento.vue`:

```vue
<script setup lang="ts">
import type { Seat } from '@/protocol/engine'
import type { SeatInfo } from '@/protocol/messages'
import { teamOf } from '@/game/seats'

const props = defineProps<{ seat: Seat; info: SeatInfo | null; isMe: boolean; isHost: boolean; canKick: boolean }>()
const emit = defineEmits<{ take: []; kick: [] }>()
</script>

<template>
  <div
    :data-assento="props.seat"
    class="flex min-h-28 w-44 flex-col items-center justify-center gap-1 rounded-2xl border-4 bg-surface p-3 text-center shadow-card"
    :class="[teamOf(props.seat) === 0 ? 'border-team-0' : 'border-team-1', props.info && !props.info.connected ? 'opacity-60' : '']"
  >
    <template v-if="props.info">
      <p class="max-w-full truncate font-semibold">
        <span v-if="props.isHost" aria-label="Anfitrião" title="Anfitrião">👑 </span>{{ props.info.displayName }}<span v-if="props.isMe"> (você)</span>
      </p>
      <p class="text-sm" :class="props.info.ready ? 'text-team-0' : 'text-ink-muted'">{{ props.info.ready ? 'Pronto' : 'Aguardando' }}</p>
      <p v-if="!props.info.connected" class="text-xs text-ink-muted">Desconectado</p>
      <button v-if="props.canKick" type="button" data-acao="remover" class="text-xs text-danger underline" @click="emit('kick')">Remover</button>
    </template>
    <button v-else type="button" data-acao="sentar" class="h-full w-full rounded-xl text-ink-muted hover:bg-surface-2" @click="emit('take')">
      Sentar aqui
    </button>
  </div>
</template>
```

`frontend/src/components/sala/SalaEspera.vue`:

```vue
<script setup lang="ts">
import { computed } from 'vue'
import type { Seat } from '@/protocol/engine'
import { useRoomStore } from '@/stores/room'
import { useToastsStore } from '@/stores/toasts'
import { relativePosition } from '@/game/seats'
import Assento from './Assento.vue'
import Toggle from '@/components/ui/Toggle.vue'
import Botao from '@/components/ui/Botao.vue'

const room = useRoomStore()
const toasts = useToastsStore()
const r = computed(() => room.room!)
const inviteLink = computed(() => `${location.origin}/sala/${r.value.code}`)
const POS_CLASS = { bottom: 'col-start-2 row-start-3', left: 'col-start-1 row-start-2', top: 'col-start-2 row-start-1', right: 'col-start-3 row-start-2' }

async function run(p: Promise<void>) {
  try {
    await p
  } catch (err) {
    toasts.error((err as { code?: string }).code ?? 'internal', (err as Error).message)
  }
}

async function copy() {
  try {
    await navigator.clipboard.writeText(inviteLink.value)
    toasts.push('Link copiado.')
  } catch {
    toasts.push(`Copie o link: ${inviteLink.value}`)
  }
}

const seats = computed(() => ([0, 1, 2, 3] as Seat[]).map((s) => ({ seat: s, info: r.value.seats[s] ?? null, pos: relativePosition(s, r.value.yourSeat) })))
</script>

<template>
  <div class="flex flex-1 flex-col items-center gap-6 bg-felt p-4 text-white">
    <header class="flex w-full max-w-3xl flex-wrap items-center justify-between gap-3">
      <h1 class="font-display text-2xl">{{ r.name }}</h1>
      <div v-if="r.visibility === 'private'" class="flex items-center gap-2">
        <span>Código <strong class="tracking-widest">{{ r.code }}</strong></span>
        <Botao data-acao="copiar" variant="secondary" @click="copy">Copiar link</Botao>
      </div>
    </header>

    <div class="grid grid-cols-3 grid-rows-3 place-items-center gap-4" aria-label="Lugares da mesa">
      <div v-for="s in seats" :key="s.seat" :class="POS_CLASS[s.pos]">
        <Assento
          :seat="s.seat"
          :info="s.info"
          :is-me="s.seat === r.yourSeat"
          :is-host="s.seat === r.host"
          :can-kick="room.isHost && s.info !== null && s.seat !== r.yourSeat"
          @take="run(room.takeSeat(s.seat))"
          @kick="run(room.kick(s.seat))"
        />
      </div>
      <p class="col-start-2 row-start-2 text-center text-sm opacity-80">Duplas: lugares 1 e 3 contra 2 e 4.<br />A partida começa quando os 4 estiverem prontos.</p>
    </div>

    <div class="flex items-center gap-4">
      <Toggle :model-value="room.amReady" label="Estou pronto" @update:model-value="run(room.setReady($event))" />
      <Botao variant="ghost" class="text-white" @click="run(room.leave())">Sair da sala</Botao>
    </div>
  </div>
</template>
```

`frontend/src/views/SalaView.vue`:

```vue
<script setup lang="ts">
import { useRoomStore } from '@/stores/room'
import SalaEspera from '@/components/sala/SalaEspera.vue'

const room = useRoomStore()
</script>

<template>
  <template v-if="room.room">
    <SalaEspera v-if="!room.isPlaying" />
    <p v-else class="p-8" role="status">Partida em andamento…</p>
  </template>
  <p v-else class="p-8 text-ink-muted" role="status">Carregando sala…</p>
</template>
```

`/sala` sem sala após a conexão abrir volta ao lobby: acrescente em `SalaView` um `watch` em `[session.connection, room.inRoom]` que, com `connection === 'open'` e `!inRoom` por mais de 1 s (dá tempo do `room_state` chegar após o `hello`), faz `router.replace({ name: 'lobby' })`.

- [ ] **Step 3: Visual, acessibilidade e testes**

Aplique `frontend-design:frontend-design` (assentos em volta de uma mesa oval, cores das duplas) e `fixing-accessibility`. Run: `cd frontend && pnpm vitest run && pnpm typecheck && pnpm lint`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add frontend/src
git commit -m "feat(frontend): sala de espera com lugares, pronto, convite e remoção pelo anfitrião"
```

---

### Task 17: Mesa — layout, cartas, cena e prazos

**Skills:** `frontend-design:frontend-design` (**antes** do Step 2: composição da mesa desktop e retrato no celular), `ui-ux-pro-max` (hierarquia de informação: vez, prazo, placar), `fixing-accessibility` (ao final).

**Files:**
- Create: `frontend/src/components/mesa/{Mesa,AssentoMesa,Mao,Carta,Centro,Vaza,Placar,Prazo,CenaEvento}.vue`
- Create: `frontend/src/components/mesa/mesa.test.ts`
- Create: `frontend/src/game/deadline.ts`, `frontend/src/game/deadline.test.ts`
- Modify: `frontend/src/views/SalaView.vue` (usa `Mesa` quando `room.isPlaying`)

**Interfaces:**
- Consumes: `useGameStore` (`view`, `deadlines`, `pendingSeats`, `scene`, `canPlay`, `playCard`, `mySeat`), `useRoomStore` (`room.seats` para presença), `usePrefsStore` (`deck`), `cardUrl`, `backUrl`, `cardLabel`, `relativePosition`, `seatsInDisplayOrder`, `teamOf`, `serverNow`… — **atenção:** `serverNow` está em `@/api/http`, que componentes não importam; exponha-o via `useSessionStore().serverNow()` (ação que chama `serverNow()` de `@/api/http`) — acrescente-a nesta tarefa.
- Produces:
  - `remainingMs(deadline: number | undefined, now: number): number | null` e `deadlineFraction(deadline: number | undefined, totalMs: number, now: number): number` em `game/deadline.ts`.
  - `Carta.vue` props `{ card?: CardID; covered?: boolean; hidden?: boolean; size?: 'sm' | 'md' | 'lg'; interactive?: boolean; label?: string }`, emite `select`.
  - `Mao.vue` props `{ slots: HandSlotView[]; interactive: boolean }`, emite `play: [slot: number]` (a "coberta" fica no `Mesa`).
  - `Prazo.vue` props `{ deadline?: number; totalMs: number; mine: boolean }`; toca `timeLow` uma vez nos últimos 5 s quando `mine`.
  - `Mesa.vue` monta tudo e expõe a "coberta" (`ref<boolean>`) para o painel (Tarefa 18) via `provide('coberta', ref)`.
  - `AssentoMesa.vue` props `{ seat: Seat; position: RelativePosition }`.

- [ ] **Step 1: Testes que falham**

`frontend/src/game/deadline.test.ts`:

```ts
import { deadlineFraction, remainingMs } from './deadline'

it('calcula o restante e a fração sem passar de 0..1', () => {
  expect(remainingMs(undefined, 0)).toBeNull()
  expect(remainingMs(10_000, 4_000)).toBe(6_000)
  expect(remainingMs(10_000, 12_000)).toBe(0)
  expect(deadlineFraction(30_000, 30_000, 15_000)).toBe(0.5)
  expect(deadlineFraction(30_000, 30_000, 60_000)).toBe(0)
  expect(deadlineFraction(90_000, 30_000, 0)).toBe(1)
})
```

`frontend/src/components/mesa/mesa.test.ts`:

```ts
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setSender } from '@/app/transport'
import Mesa from './Mesa.vue'
import Carta from './Carta.vue'
import { useGameStore } from '@/stores/game'
import { useRoomStore } from '@/stores/room'
import { makeState } from '@/test/fixtures'

let pinia: ReturnType<typeof createPinia>
beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
  useRoomStore().onRoomState({
    id: 'r', code: 'ABC234', name: 'Sala', visibility: 'public', host: 0, status: 'playing', matchId: 'm',
    seats: [0, 1, 2, 3].map((s) => ({ userId: `u${s}`, displayName: ['Ana', 'Bia', 'Caio', 'Duda'][s]!, ready: true, connected: true, absent: s === 3 })),
    yourSeat: 1,
  })
})
afterEach(() => setSender(null))

it('posiciona assentos a partir do meu e marca ausente', () => {
  useGameStore().onState(makeState({ seat: 1, turn: 1 }, [1]))
  const w = mount(Mesa, { global: { plugins: [pinia] } })
  expect(w.find('[data-posicao="bottom"]').text()).toContain('Bia')
  expect(w.find('[data-posicao="top"]').text()).toContain('Duda')
  expect(w.find('[data-posicao="top"]').text()).toContain('Ausente')
  expect(w.find('[data-posicao="left"]').text()).toContain('Caio')
})

it('clicar numa carta da mão joga aquele slot quando é a minha vez', async () => {
  const send = vi.fn().mockResolvedValue(undefined)
  setSender(send)
  useGameStore().onState(makeState({ seat: 1, turn: 1 }, [1]))
  const w = mount(Mesa, { global: { plugins: [pinia] } })
  await w.findAll('[data-mao] button')[2]!.trigger('click')
  expect(send).toHaveBeenCalledWith({ type: 'play_card', payload: { slot: 2, covered: false } })
})

it('fora da vez as cartas da mão ficam desabilitadas', () => {
  useGameStore().onState(makeState({ seat: 1, turn: 2 }, [2]))
  const w = mount(Mesa, { global: { plugins: [pinia] } })
  for (const b of w.findAll('[data-mao] button')) expect(b.attributes('disabled')).toBeDefined()
})

it('carta coberta e oculta mostram o verso com rótulo', () => {
  const coberta = mount(Carta, { props: { covered: true }, global: { plugins: [pinia] } })
  expect(coberta.find('img').attributes('alt')).toBe('Carta coberta')
  const oculta = mount(Carta, { props: { hidden: true, label: 'Carta 2' }, global: { plugins: [pinia] } })
  expect(oculta.find('img').attributes('alt')).toBe('Carta 2')
})
```

Run: `cd frontend && pnpm vitest run src/game/deadline.test.ts src/components/mesa`
Expected: FAIL.

- [ ] **Step 2: Implementar funções de prazo e a ação `serverNow`**

`frontend/src/game/deadline.ts`:

```ts
export function remainingMs(deadline: number | undefined, now: number): number | null {
  if (deadline === undefined) return null
  return Math.max(0, deadline - now)
}

export function deadlineFraction(deadline: number | undefined, totalMs: number, now: number): number {
  const left = remainingMs(deadline, now)
  if (left === null || totalMs <= 0) return 0
  return Math.min(1, Math.max(0, left / totalMs))
}
```

Em `useSessionStore().actions` acrescente:

```ts
    serverNow(): number {
      return serverNow()
    },
```

(com `import { ApiError, serverNow } from '@/api/http'`).

- [ ] **Step 3: Componentes da mesa**

`frontend/src/components/mesa/Carta.vue`:

```vue
<script setup lang="ts">
import { computed } from 'vue'
import type { CardID } from '@/protocol/engine'
import { usePrefsStore } from '@/stores/prefs'
import { backUrl, cardUrl } from '@/game/cardAssets'
import { cardLabel } from '@/game/cards'

const props = withDefaults(
  defineProps<{ card?: CardID; covered?: boolean; hidden?: boolean; size?: 'sm' | 'md' | 'lg'; interactive?: boolean; disabled?: boolean; label?: string }>(),
  { size: 'md', covered: false, hidden: false, interactive: false, disabled: false },
)
const emit = defineEmits<{ select: [] }>()
const prefs = usePrefsStore()

const faceDown = computed(() => props.covered || props.hidden || props.card === undefined)
const src = computed(() => (faceDown.value ? backUrl(prefs.deck) : cardUrl(props.card!, prefs.deck)))
const alt = computed(() => {
  if (props.label) return props.label
  if (props.covered) return 'Carta coberta'
  if (faceDown.value) return 'Carta virada'
  return cardLabel(props.card!, prefs.deck)
})
const SIZES = { sm: 'h-16', md: 'h-28', lg: 'h-36' }
</script>

<template>
  <button
    v-if="interactive"
    type="button"
    :disabled="disabled"
    :aria-label="alt"
    class="rounded-card transition hover:-translate-y-2 focus-visible:-translate-y-2 disabled:hover:translate-y-0"
    @click="emit('select')"
  >
    <img :src="src" :alt="alt" class="aspect-[169/245] rounded-card shadow-card" :class="SIZES[size]" draggable="false" />
  </button>
  <img v-else :src="src" :alt="alt" class="aspect-[169/245] rounded-card shadow-card" :class="SIZES[size]" draggable="false" />
</template>
```

`frontend/src/components/mesa/Mao.vue`:

```vue
<script setup lang="ts">
import type { HandSlotView } from '@/protocol/engine'
import Carta from './Carta.vue'

const props = defineProps<{ slots: HandSlotView[]; interactive: boolean }>()
const emit = defineEmits<{ play: [slot: number] }>()
</script>

<template>
  <div data-mao class="flex justify-center gap-2" aria-label="Sua mão">
    <TransitionGroup name="mao">
      <Carta
        v-for="(s, i) in props.slots"
        :key="s.slot"
        :card="s.card"
        :hidden="s.hidden"
        :label="s.hidden ? `Carta ${i + 1}` : undefined"
        size="lg"
        interactive
        :disabled="!props.interactive"
        @select="emit('play', s.slot)"
      />
    </TransitionGroup>
  </div>
</template>

<style scoped>
.mao-leave-active {
  transition: all 0.3s ease;
}
.mao-leave-to {
  opacity: 0;
  transform: translateY(-40px);
}
</style>
```

`frontend/src/components/mesa/Prazo.vue`:

```vue
<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useSessionStore } from '@/stores/session'
import { deadlineFraction, remainingMs } from '@/game/deadline'
import { sfx } from '@/audio/install'

const props = defineProps<{ deadline?: number; totalMs: number; mine: boolean }>()
const session = useSessionStore()
const fraction = ref(0)
const seconds = ref<number | null>(null)
let raf = 0
let warned = false

function tick() {
  const now = session.serverNow()
  fraction.value = deadlineFraction(props.deadline, props.totalMs, now)
  const left = remainingMs(props.deadline, now)
  seconds.value = left === null ? null : Math.ceil(left / 1000)
  if (props.mine && left !== null && left > 0 && left <= 5000 && !warned) {
    warned = true
    sfx()?.play('timeLow')
  }
  raf = requestAnimationFrame(tick)
}

watch(
  () => props.deadline,
  () => {
    warned = false
  },
)
onMounted(() => (raf = requestAnimationFrame(tick)))
onBeforeUnmount(() => cancelAnimationFrame(raf))
</script>

<template>
  <div v-if="seconds !== null" class="relative size-10" role="timer" :aria-label="`${seconds} segundos`">
    <svg viewBox="0 0 36 36" class="size-10 -rotate-90">
      <circle cx="18" cy="18" r="16" fill="none" stroke="currentColor" stroke-opacity="0.2" stroke-width="3" />
      <circle
        cx="18" cy="18" r="16" fill="none" stroke-width="3" stroke-linecap="round"
        :stroke="seconds <= 5 ? 'var(--color-danger)' : 'var(--color-accent)'"
        :stroke-dasharray="`${fraction * 100.5} 100.5`"
      />
    </svg>
    <span class="absolute inset-0 grid place-items-center text-xs font-bold">{{ seconds }}</span>
  </div>
</template>
```

`totalMs` vem da fase: `playing` 30 s, votos 20 s, escolha/queima 15 s, ausente 5 s — o cliente não sabe o valor configurado no servidor. Use como `totalMs` o **maior** entre `deadline − momento em que o prazo apareceu` (guarde o primeiro `serverNow()` em que viu aquele `deadline`) e 1 ms; isso dá um anel correto sem replicar configuração. Implemente isso no `AssentoMesa` (abaixo), que passa `totalMs` calculado.

`frontend/src/components/mesa/AssentoMesa.vue`:

```vue
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { Seat } from '@/protocol/engine'
import type { RelativePosition } from '@/game/seats'
import { teamOf } from '@/game/seats'
import { useGameStore } from '@/stores/game'
import { useRoomStore } from '@/stores/room'
import { useSessionStore } from '@/stores/session'
import Carta from './Carta.vue'
import Prazo from './Prazo.vue'

const props = defineProps<{ seat: Seat; position: RelativePosition }>()
const game = useGameStore()
const room = useRoomStore()
const session = useSessionStore()

const player = computed(() => game.view?.players.find((p) => p.seat === props.seat))
const presence = computed(() => room.room?.seats[props.seat] ?? null)
const isTurn = computed(() => game.view?.turn === props.seat)
const deadline = computed(() => game.deadlines[props.seat])
const isMe = computed(() => props.seat === game.view?.seat)
const isDealer = computed(() => game.view?.dealer === props.seat)

const firstSeen = ref<{ deadline: number; at: number } | null>(null)
watch(
  deadline,
  (d) => {
    firstSeen.value = d === undefined ? null : { deadline: d, at: session.serverNow() }
  },
  { immediate: true },
)
const totalMs = computed(() => (firstSeen.value ? Math.max(1, firstSeen.value.deadline - firstSeen.value.at) : 1))
</script>

<template>
  <div
    :data-posicao="props.position"
    class="flex flex-col items-center gap-2"
    :class="presence && (!presence.connected || presence.absent) ? 'opacity-60' : ''"
  >
    <div
      class="flex items-center gap-2 rounded-full px-3 py-1 text-sm text-white"
      :class="[teamOf(props.seat) === 0 ? 'bg-team-0/80' : 'bg-team-1/80', isTurn ? 'ring-4 ring-accent' : '']"
    >
      <span class="max-w-32 truncate font-semibold">{{ player?.name }}</span>
      <span v-if="isDealer" title="Pé (quem dá as cartas)" aria-label="pé">🂠</span>
      <span v-if="presence && !presence.connected" class="text-xs">Desconectado</span>
      <span v-else-if="presence?.absent" class="text-xs">Ausente</span>
      <Prazo :deadline="deadline" :total-ms="totalMs" :mine="isMe" />
    </div>
    <div v-if="!isMe" class="flex -space-x-8" :aria-label="`${player?.cardCount ?? 0} cartas`">
      <Carta v-for="i in player?.cardCount ?? 0" :key="i" size="sm" hidden />
    </div>
    <slot />
  </div>
</template>
```

`frontend/src/components/mesa/Vaza.vue` (cartas da rodada atual, cada uma na direção de quem jogou):

```vue
<script setup lang="ts">
import { computed } from 'vue'
import { useGameStore } from '@/stores/game'
import { relativePosition } from '@/game/seats'
import Carta from './Carta.vue'

const game = useGameStore()
const OFFSET = { bottom: 'translate-y-10', top: '-translate-y-10', left: '-translate-x-14', right: 'translate-x-14' }
const cards = computed(() =>
  (game.view?.table ?? []).map((t) => ({ ...t, pos: relativePosition(t.seat, game.view!.seat) })),
)
</script>

<template>
  <div class="relative grid h-56 w-56 place-items-center" aria-label="Cartas na mesa">
    <TransitionGroup name="vaza">
      <div v-for="c in cards" :key="c.seat" class="absolute" :class="OFFSET[c.pos]">
        <Carta :card="c.card" :covered="c.covered" size="md" />
      </div>
    </TransitionGroup>
  </div>
</template>

<style scoped>
.vaza-enter-active {
  transition: all 0.3s ease-out;
}
.vaza-enter-from {
  opacity: 0;
  transform: scale(0.8);
}
</style>
```

`frontend/src/components/mesa/Centro.vue`:

```vue
<script setup lang="ts">
import { computed } from 'vue'
import { useGameStore } from '@/stores/game'
import { usePrefsStore } from '@/stores/prefs'
import { rankLabel } from '@/game/cards'
import Carta from './Carta.vue'
import Vaza from './Vaza.vue'

const game = useGameStore()
const prefs = usePrefsStore()
const v = computed(() => game.view)
const rounds = computed(() => v.value?.rounds ?? [])
</script>

<template>
  <div v-if="v" class="flex flex-col items-center gap-3">
    <div class="flex items-center gap-6 text-white">
      <div v-if="v.vira !== undefined" class="flex flex-col items-center gap-1">
        <Carta :card="v.vira" size="sm" />
        <span class="text-xs">Vira</span>
      </div>
      <div v-if="v.manilha !== undefined" class="text-center">
        <p class="text-xs opacity-80">Manilha</p>
        <p class="font-display text-2xl">{{ rankLabel(v.manilha, prefs.deck) }}</p>
      </div>
      <div class="text-center text-xs opacity-80">
        <p>Monte {{ v.deckCount }}</p>
        <p>Descarte {{ v.discardCount }}</p>
      </div>
    </div>
    <Vaza />
    <ol class="flex gap-2" aria-label="Rodadas desta mão">
      <li
        v-for="(r, i) in rounds"
        :key="i"
        class="size-3 rounded-full"
        :class="r.winner === undefined ? 'bg-white/50' : r.winner === 0 ? 'bg-team-0' : 'bg-team-1'"
        :aria-label="`Rodada ${i + 1}: ${r.winner === undefined ? 'empate ou em andamento' : `dupla ${r.winner + 1}`}`"
      />
    </ol>
  </div>
</template>
```

`frontend/src/components/mesa/Placar.vue`:

```vue
<script setup lang="ts">
import { computed } from 'vue'
import { useGameStore } from '@/stores/game'
import { handKindText, stakeName } from '@/game/labels'

const game = useGameStore()
const v = computed(() => game.view)
const myTeam = computed(() => (v.value ? v.value.seat % 2 : 0))
</script>

<template>
  <div v-if="v" class="flex flex-wrap items-center gap-4 rounded-xl bg-black/30 px-4 py-2 text-white" aria-label="Placar">
    <div class="flex items-baseline gap-2 font-display">
      <span class="text-team-0 text-3xl">{{ v.score[0] }}</span>
      <span class="opacity-60">×</span>
      <span class="text-team-1 text-3xl">{{ v.score[1] }}</span>
    </div>
    <span class="text-sm opacity-80">Vocês: dupla {{ myTeam + 1 }}</span>
    <span class="rounded bg-accent px-2 py-0.5 text-sm font-bold text-ink">
      Vale {{ v.stake }}<template v-if="v.stake > 1"> ({{ stakeName(v.stake) }})</template>
    </span>
    <span class="text-sm">{{ handKindText(v.handKind) }} · Mão {{ v.handNumber }}</span>
  </div>
</template>
```

`frontend/src/components/mesa/CenaEvento.vue` (balões transitórios da cena do diretor):

```vue
<script setup lang="ts">
import { computed } from 'vue'
import { useGameStore } from '@/stores/game'
import { stakeName, voteText } from '@/game/labels'

const game = useGameStore()
const text = computed(() => {
  const ev = game.scene.event
  if (!ev) return null
  switch (ev.kind) {
    case 'raise_requested':
      return `${stakeName(ev.stake ?? 0).toUpperCase()}!`
    case 'raise_resolved':
      return ev.choice === 'run' ? 'Correu!' : 'Aceito!'
    case 'raise_punished':
      return 'Pedido proibido — derrota!'
    case 'eleven_decided':
      return ev.choice === 'run' ? 'Correu da mão de 11' : 'Vai jogar a mão de 11'
    case 'vote_cast':
      return ev.choice ? voteText(ev.choice) : null
    case 'hand_started':
      return `Mão ${(game.latest?.view.handNumber ?? 0)}`
    default:
      return null
  }
})
</script>

<template>
  <Transition name="balao">
    <div v-if="text" :key="game.scene.key" class="pointer-events-none absolute inset-0 grid place-items-center" aria-hidden="true">
      <p class="rounded-2xl bg-accent px-6 py-3 font-display text-4xl text-ink shadow-card">{{ text }}</p>
    </div>
  </Transition>
</template>

<style scoped>
.balao-enter-active,
.balao-leave-active {
  transition: all 0.25s ease;
}
.balao-enter-from,
.balao-leave-to {
  opacity: 0;
  transform: scale(0.6);
}
</style>
```

`frontend/src/components/mesa/Mesa.vue`:

```vue
<script setup lang="ts">
import { computed, provide, ref, watch } from 'vue'
import { useGameStore } from '@/stores/game'
import { relativePosition, seatsInDisplayOrder } from '@/game/seats'
import { sfx } from '@/audio/install'
import AssentoMesa from './AssentoMesa.vue'
import Centro from './Centro.vue'
import Mao from './Mao.vue'
import Placar from './Placar.vue'
import CenaEvento from './CenaEvento.vue'

const game = useGameStore()
const covered = ref(false)
provide('coberta', covered)

const others = computed(() => {
  const me = game.view?.seat
  if (me === undefined) return []
  return seatsInDisplayOrder(me)
    .slice(1)
    .map((seat) => ({ seat, position: relativePosition(seat, me) }))
})

const GRID = { left: 'col-start-1 row-start-2', top: 'col-start-2 row-start-1', right: 'col-start-3 row-start-2' }

function play(slot: number) {
  void game.playCard(slot, covered.value)
  covered.value = false
}

// "Sua vez": som só com a aba em foco (spec §8.2); o título da aba é tratado na Tarefa 19.
watch(
  () => game.isMyTurn,
  (mine) => {
    if (mine && document.hasFocus()) sfx()?.play('yourTurn')
  },
)
</script>

<template>
  <div v-if="game.view" class="relative flex flex-1 flex-col bg-felt p-2 md:p-4">
    <Placar class="self-center" />
    <div class="grid flex-1 grid-cols-3 grid-rows-[auto_1fr] place-items-center gap-2 max-md:grid-cols-3 max-md:text-xs">
      <AssentoMesa v-for="o in others" :key="o.seat" :seat="o.seat" :position="o.position" :class="GRID[o.position as 'left' | 'top' | 'right']" />
      <div class="col-start-2 row-start-2">
        <Centro />
      </div>
    </div>
    <div class="flex flex-col items-center gap-2">
      <AssentoMesa :seat="game.view.seat" position="bottom" />
      <Mao :slots="game.view.myHand" :interactive="game.canPlay && !game.inflight" @play="play" />
      <slot name="painel" />
    </div>
    <CenaEvento />
  </div>
  <p v-else class="p-8" role="status">Carregando mesa…</p>
</template>
```

No celular (`< md`), os assentos de cima/lados ficam numa faixa compacta: a grade já encolhe; com a skill `frontend-design`, ajuste `max-md:` para colocar esquerda/topo/direita numa única linha (`max-md:grid-rows-1`, `max-md:row-start-1` nos três) e reduzir o `Centro`.

`frontend/src/views/SalaView.vue`: troque o `<p v-else>Partida em andamento…</p>` por `<Mesa v-else />` (import de `@/components/mesa/Mesa.vue`).

- [ ] **Step 4: Rodar e ver passar**

Run: `cd frontend && pnpm vitest run && pnpm typecheck && pnpm lint`
Expected: PASS. (Se `requestAnimationFrame` não existir no jsdom em algum teste, o `Prazo` só monta quando há `deadline` — nos testes da mesa há; o jsdom tem `requestAnimationFrame`.)

- [ ] **Step 5: Visual e acessibilidade**

Com `frontend-design:frontend-design`, refine a mesa (feltro com vinheta, sombras das cartas, destaque de vez, posição dos nomes) e o retrato no celular (teste em 375×812 no navegador). Com `fixing-accessibility`, revise rótulos das cartas, contraste do placar e o `role="timer"`.

- [ ] **Step 6: Commit**

```bash
git add frontend/src
git commit -m "feat(frontend): mesa com assentos relativos, mão jogável, vaza, placar, prazos e balões de evento"
```

---

### Task 18: Painel de ação contextual e atalhos de teclado

**Skills:** `ui-ux-pro-max` (antes do Step 2: hierarquia e estados dos botões), `frontend-design:frontend-design`, `fixing-accessibility` (ao final).

**Files:**
- Create: `frontend/src/components/mesa/{PainelAcao,Corte,Queima,DecisaoOnze,RespostaAumento}.vue`
- Create: `frontend/src/components/mesa/painel.test.ts`
- Create: `frontend/src/game/shortcuts.ts`, `frontend/src/game/shortcuts.test.ts`
- Modify: `frontend/src/components/mesa/Mesa.vue` (insere `PainelAcao` e instala atalhos)

**Interfaces:**
- Consumes: getters/ações de `useGameStore` (Task 10), `inject('coberta')` (Task 17), `Modal`, `Botao`, `Toggle`, `Carta`, `stakeName`, `nextStake`, `voteText`.
- Produces:
  - `PainelAcao.vue`: escolhe o subpainel pela fase/getters; no `playing` mostra o toggle "Coberta" (`canCover`) e o botão de aumento (`canRaise`, rótulo `stakeName(nextStakeValue)`); com `raiseIsSuicide` abre `Modal` de confirmação antes de `requestRaise()`.
  - `shortcutAction(key: string, ctx: ShortcutContext): ShortcutAction | null` em `game/shortcuts.ts` com `interface ShortcutContext { canPlay: boolean; canCover: boolean; canRaise: boolean; mustRespondRaise: boolean; canRaiseBack: boolean; handSize: number }` e `type ShortcutAction = { kind: 'play'; index: number } | { kind: 'toggleCover' } | { kind: 'raise' } | { kind: 'vote'; choice: 'accept' | 'run' | 'raise' }`.
  - `installShortcuts(handler: (a: ShortcutAction) => void, ctx: () => ShortcutContext): () => void` — escuta `keydown` no `window`, ignora quando o foco está em `input`/`textarea`/`select`/`[contenteditable]` ou com modificadores, devolve a função de remoção.

- [ ] **Step 1: Testes que falham**

`frontend/src/game/shortcuts.test.ts`:

```ts
import { installShortcuts, shortcutAction, type ShortcutContext } from './shortcuts'

const ctx = (over: Partial<ShortcutContext> = {}): ShortcutContext => ({
  canPlay: true, canCover: true, canRaise: true, mustRespondRaise: false, canRaiseBack: false, handSize: 3, ...over,
})

it('mapeia teclas para ações permitidas', () => {
  expect(shortcutAction('2', ctx())).toEqual({ kind: 'play', index: 1 })
  expect(shortcutAction('3', ctx({ handSize: 2 }))).toBeNull()
  expect(shortcutAction('c', ctx())).toEqual({ kind: 'toggleCover' })
  expect(shortcutAction('t', ctx({ canRaise: false }))).toBeNull()
  expect(shortcutAction('a', ctx({ mustRespondRaise: true }))).toEqual({ kind: 'vote', choice: 'accept' })
  expect(shortcutAction('u', ctx({ mustRespondRaise: true, canRaiseBack: false }))).toBeNull()
  expect(shortcutAction('1', ctx({ canPlay: false }))).toBeNull()
})

it('ignora teclas digitadas em campos de texto', () => {
  const handler = vi.fn()
  const off = installShortcuts(handler, () => ctx())
  const input = document.createElement('input')
  document.body.append(input)
  input.dispatchEvent(new KeyboardEvent('keydown', { key: '1', bubbles: true }))
  window.dispatchEvent(new KeyboardEvent('keydown', { key: '1' }))
  window.dispatchEvent(new KeyboardEvent('keydown', { key: '1', ctrlKey: true }))
  expect(handler).toHaveBeenCalledOnce()
  off()
})
```

`frontend/src/components/mesa/painel.test.ts`:

```ts
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { ref } from 'vue'
import { setSender } from '@/app/transport'
import PainelAcao from './PainelAcao.vue'
import { useGameStore } from '@/stores/game'
import { makeState } from '@/test/fixtures'

let pinia: ReturnType<typeof createPinia>
let send: ReturnType<typeof vi.fn>
beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
  send = vi.fn().mockResolvedValue(undefined)
  setSender(send)
})
afterEach(() => setSender(null))

function mountPainel() {
  return mount(PainelAcao, { global: { plugins: [pinia], provide: { coberta: ref(false) } }, attachTo: document.body })
}

it('corte: escolhe uma das cartas oferecidas', async () => {
  useGameStore().onState(makeState({ phase: 'cut_choice', cutOffer: [{ keep: 0, card: 5 }, { keep: 1, card: 9 }] }))
  const w = mountPainel()
  await w.findAll('[data-corte] button')[1]!.trigger('click')
  expect(send).toHaveBeenCalledWith({ type: 'choose_card', payload: { keep: 1 } })
})

it('queima: seleciona os 4 e confirma, ou passa', async () => {
  useGameStore().onState(makeState({ phase: 'burn_fours', myHand: [{ slot: 0, card: 0, hidden: false }, { slot: 1, card: 3, hidden: false }, { slot: 2, card: 39, hidden: false }] }))
  const w = mountPainel()
  expect(w.findAll('[data-queima] [role="checkbox"]')).toHaveLength(2)
  await w.findAll('[data-queima] [role="checkbox"]')[0]!.trigger('click')
  await w.find('[data-acao="queimar"]').trigger('click')
  await w.find('[data-acao="passar"]').trigger('click')
  expect(send.mock.calls.map((c) => c[0])).toEqual([
    { type: 'burn', payload: { slots: [0] } },
    { type: 'pass_burn' },
  ])
})

it('mão de 11: mostra a mão do parceiro e vota', async () => {
  useGameStore().onState(makeState({ phase: 'eleven_decision', score: [11, 4], partnerHand: [{ slot: 0, card: 1, hidden: false }], partnerVote: 'play' }))
  const w = mountPainel()
  expect(w.text()).toContain('Parceiro: Jogo')
  await w.find('[data-voto="run"]').trigger('click')
  expect(send).toHaveBeenCalledWith({ type: 'vote', payload: { choice: 'run' } })
})

it('resposta ao aumento: aceitar/correr/aumentar com o próximo valor', async () => {
  const pending = { requester: 1, fromTeam: 1, prevStake: 1, toStake: 3 }
  useGameStore().onState(makeState({ phase: 'awaiting_raise_response', votingTeam: 0, pending }, [0, 2]))
  const w = mountPainel()
  expect(w.find('[data-voto="raise"]').text()).toContain('Seis')
  await w.find('[data-voto="accept"]').trigger('click')
  expect(send).toHaveBeenCalledWith({ type: 'vote', payload: { choice: 'accept' } })
})

it('pedido na mão de 11 exige confirmação', async () => {
  useGameStore().onState(makeState({ handKind: 'eleven', stake: 3 }))
  const w = mountPainel()
  await w.find('[data-acao="aumentar"]').trigger('click')
  expect(send).not.toHaveBeenCalled()
  expect(document.body.textContent).toContain('faz sua dupla perder a partida')
  ;(document.querySelector('[data-acao="confirmar-aumento"]') as HTMLButtonElement).click()
  await flushPromises()
  expect(send).toHaveBeenCalledWith({ type: 'request_raise' })
  w.unmount()
})

it('fora de decisão não mostra nada', () => {
  useGameStore().onState(makeState({ turn: 2 }, [2]))
  const w = mountPainel()
  expect(w.find('[data-painel]').exists()).toBe(false)
})
```

Run: `cd frontend && pnpm vitest run src/game/shortcuts.test.ts src/components/mesa/painel.test.ts`
Expected: FAIL.

- [ ] **Step 2: Atalhos**

`frontend/src/game/shortcuts.ts`:

```ts
export interface ShortcutContext {
  canPlay: boolean
  canCover: boolean
  canRaise: boolean
  mustRespondRaise: boolean
  canRaiseBack: boolean
  handSize: number
}

export type ShortcutAction =
  | { kind: 'play'; index: number }
  | { kind: 'toggleCover' }
  | { kind: 'raise' }
  | { kind: 'vote'; choice: 'accept' | 'run' | 'raise' }

export function shortcutAction(key: string, ctx: ShortcutContext): ShortcutAction | null {
  const k = key.toLowerCase()
  if (['1', '2', '3'].includes(k)) {
    const index = Number(k) - 1
    return ctx.canPlay && index < ctx.handSize ? { kind: 'play', index } : null
  }
  if (k === 'c') return ctx.canCover ? { kind: 'toggleCover' } : null
  if (k === 't') return ctx.canRaise ? { kind: 'raise' } : null
  if (k === 'a') return ctx.mustRespondRaise ? { kind: 'vote', choice: 'accept' } : null
  if (k === 'r') return ctx.mustRespondRaise ? { kind: 'vote', choice: 'run' } : null
  if (k === 'u') return ctx.mustRespondRaise && ctx.canRaiseBack ? { kind: 'vote', choice: 'raise' } : null
  return null
}

function isTyping(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  return target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName)
}

export function installShortcuts(handler: (a: ShortcutAction) => void, ctx: () => ShortcutContext): () => void {
  const onKey = (e: KeyboardEvent) => {
    if (e.ctrlKey || e.metaKey || e.altKey || isTyping(e.target) || isTyping(document.activeElement)) return
    if (document.querySelector('[aria-modal="true"]')) return // modal aberto: Esc e Tab são dele
    const action = shortcutAction(e.key, ctx())
    if (!action) return
    e.preventDefault()
    handler(action)
  }
  window.addEventListener('keydown', onKey)
  return () => window.removeEventListener('keydown', onKey)
}
```

(No teste, o evento disparado no `input` borbulha até o `window` com `target = input` e é ignorado; o disparado no `window` sem modificador conta; o com `ctrlKey` é ignorado.)

- [ ] **Step 3: Subpainéis**

`frontend/src/components/mesa/Corte.vue`:

```vue
<script setup lang="ts">
import { useGameStore } from '@/stores/game'
import Carta from './Carta.vue'

const game = useGameStore()
</script>

<template>
  <div data-corte class="flex flex-col items-center gap-2">
    <p class="font-semibold">{{ game.phase === 'dealer_choice' ? 'Você é o pé: escolha a sua carta' : 'Corte: escolha a sua carta' }} — a outra vai para o parceiro.</p>
    <div class="flex gap-3">
      <Carta
        v-for="o in game.lv?.cutOffer ?? []"
        :key="o.keep"
        :card="o.card"
        size="lg"
        interactive
        :disabled="game.inflight"
        @select="game.chooseCard(o.keep)"
      />
    </div>
  </div>
</template>
```

`frontend/src/components/mesa/Queima.vue`:

```vue
<script setup lang="ts">
import { computed, ref } from 'vue'
import { useGameStore } from '@/stores/game'
import Carta from './Carta.vue'
import Botao from '@/components/ui/Botao.vue'

const game = useGameStore()
const selected = ref<number[]>([])
const fours = computed(() => (game.lv?.myHand ?? []).filter((h) => game.fourSlots.includes(h.slot)))

function toggle(slot: number) {
  selected.value = selected.value.includes(slot) ? selected.value.filter((s) => s !== slot) : [...selected.value, slot].sort()
}
</script>

<template>
  <div data-queima class="flex flex-col items-center gap-2">
    <p class="font-semibold">Queima dos 4: escolha quais 4 trocar pela carta do monte.</p>
    <div class="flex gap-3">
      <button
        v-for="h in fours"
        :key="h.slot"
        type="button"
        role="checkbox"
        :aria-checked="selected.includes(h.slot)"
        class="rounded-card ring-offset-2"
        :class="selected.includes(h.slot) ? 'ring-4 ring-accent' : ''"
        @click="toggle(h.slot)"
      >
        <Carta :card="h.card" size="md" />
      </button>
    </div>
    <div class="flex gap-3">
      <Botao data-acao="queimar" :disabled="selected.length === 0 || game.inflight" @click="game.burn(selected)">Queimar selecionadas</Botao>
      <Botao data-acao="passar" variant="secondary" :disabled="game.inflight" @click="game.passBurn()">Não queimar</Botao>
    </div>
  </div>
</template>
```

O teste chama "Queimar" e depois "Não queimar" em sequência; como a store bloqueia a segunda ação enquanto a primeira está em voo, e o sender do teste resolve na hora, `inflight` já voltou a `false` no segundo clique. Se o teste falhar por timing, adicione `await flushPromises()` entre os cliques no teste.

`frontend/src/components/mesa/DecisaoOnze.vue`:

```vue
<script setup lang="ts">
import { useGameStore } from '@/stores/game'
import { voteText } from '@/game/labels'
import Carta from './Carta.vue'
import Botao from '@/components/ui/Botao.vue'

const game = useGameStore()
</script>

<template>
  <div class="flex flex-col items-center gap-2">
    <p class="font-semibold">Mão de 11: jogar (vale 3) ou correr (adversário ganha 1)?</p>
    <div class="flex items-center gap-2">
      <span class="text-sm">Mão do parceiro:</span>
      <Carta v-for="h in game.lv?.partnerHand ?? []" :key="h.slot" :card="h.card" :hidden="h.hidden" size="sm" />
    </div>
    <p v-if="game.lv?.partnerVote" class="text-sm">Parceiro: {{ voteText(game.lv.partnerVote) }}</p>
    <div class="flex gap-3">
      <Botao data-voto="play" :disabled="game.inflight" @click="game.vote('play')">Jogar</Botao>
      <Botao data-voto="run" variant="secondary" :disabled="game.inflight" @click="game.vote('run')">Correr</Botao>
    </div>
  </div>
</template>
```

`frontend/src/components/mesa/RespostaAumento.vue`:

```vue
<script setup lang="ts">
import { computed } from 'vue'
import { useGameStore } from '@/stores/game'
import { nextStake, stakeName, voteText } from '@/game/labels'
import Carta from './Carta.vue'
import Botao from '@/components/ui/Botao.vue'

const game = useGameStore()
const to = computed(() => game.lv?.pending?.toStake ?? 0)
const back = computed(() => nextStake(to.value))
</script>

<template>
  <div class="flex flex-col items-center gap-2">
    <p class="font-semibold">Pediram {{ stakeName(to) }}! A mão passa a valer {{ to }}.</p>
    <div v-if="game.lv?.partnerHand" class="flex items-center gap-2">
      <span class="text-sm">Mão do parceiro:</span>
      <Carta v-for="h in game.lv.partnerHand" :key="h.slot" :card="h.card" :hidden="h.hidden" size="sm" />
    </div>
    <p v-if="game.lv?.partnerVote" class="text-sm">Parceiro: {{ voteText(game.lv.partnerVote) }}</p>
    <div class="flex gap-3">
      <Botao data-voto="accept" :disabled="game.inflight" @click="game.vote('accept')">Aceitar <kbd class="opacity-60">A</kbd></Botao>
      <Botao data-voto="run" variant="secondary" :disabled="game.inflight" @click="game.vote('run')">Correr <kbd class="opacity-60">R</kbd></Botao>
      <Botao v-if="back !== null" data-voto="raise" variant="danger" :disabled="game.inflight" @click="game.vote('raise')">
        {{ stakeName(back) }}! <kbd class="opacity-60">U</kbd>
      </Botao>
    </div>
  </div>
</template>
```

`frontend/src/components/mesa/PainelAcao.vue`:

```vue
<script setup lang="ts">
import { computed, inject, ref, type Ref } from 'vue'
import { useGameStore } from '@/stores/game'
import { stakeName } from '@/game/labels'
import Corte from './Corte.vue'
import Queima from './Queima.vue'
import DecisaoOnze from './DecisaoOnze.vue'
import RespostaAumento from './RespostaAumento.vue'
import Botao from '@/components/ui/Botao.vue'
import Toggle from '@/components/ui/Toggle.vue'
import Modal from '@/components/ui/Modal.vue'

const game = useGameStore()
const covered = inject<Ref<boolean>>('coberta', ref(false))
const confirming = ref(false)

const mode = computed(() => {
  if (game.mustChooseCut) return 'corte'
  if (game.mustBurn) return 'queima'
  if (game.mustDecideEleven) return 'onze'
  if (game.mustRespondRaise) return 'resposta'
  if (game.canPlay) return 'jogar'
  return null
})

function raise() {
  if (game.raiseIsSuicide) {
    confirming.value = true
    return
  }
  void game.requestRaise()
}

function confirmRaise() {
  confirming.value = false
  void game.requestRaise()
}
</script>

<template>
  <div v-if="mode" data-painel class="w-full max-w-2xl rounded-2xl bg-surface/95 p-4 text-ink shadow-card">
    <Corte v-if="mode === 'corte'" />
    <Queima v-else-if="mode === 'queima'" />
    <DecisaoOnze v-else-if="mode === 'onze'" />
    <RespostaAumento v-else-if="mode === 'resposta'" />
    <div v-else class="flex flex-wrap items-center justify-center gap-4">
      <p class="font-semibold">Sua vez: clique numa carta (teclas 1–3).</p>
      <Toggle v-if="game.canCover" v-model="covered" label="Jogar coberta (C)" />
      <Botao v-if="game.canRaise && game.nextStakeValue" data-acao="aumentar" variant="danger" :disabled="game.inflight" @click="raise">
        {{ stakeName(game.nextStakeValue) }}! <kbd class="opacity-60">T</kbd>
      </Botao>
    </div>
  </div>
  <Modal :open="confirming" title="Pedido proibido" @close="confirming = false">
    <p class="mb-4">
      Pedir {{ game.nextStakeValue ? stakeName(game.nextStakeValue) : 'truco' }} na {{ game.lv?.handKind === 'iron' ? 'Mão de Ferro' : 'Mão de 11' }}
      faz sua dupla perder a partida. Confirmar?
    </p>
    <div class="flex justify-end gap-3">
      <Botao variant="secondary" @click="confirming = false">Cancelar</Botao>
      <Botao data-acao="confirmar-aumento" variant="danger" @click="confirmRaise">Pedir mesmo assim</Botao>
    </div>
  </Modal>
</template>
```

Na Mão de 11, `mustDecideEleven` ocorre **antes** do `playing`; a outra dupla vê a mesa sem painel e o texto "Aguardando a dupla com 11 decidir" — acrescente em `PainelAcao` um `v-else-if` fora do `mode` quando `game.phase === 'eleven_decision' && !game.mustDecideEleven` mostrando esse texto (sem `data-painel`, para o teste "fora de decisão" seguir válido).

- [ ] **Step 4: Ligar painel e atalhos na mesa**

Em `Mesa.vue`, substitua `<slot name="painel" />` por `<PainelAcao />` e instale os atalhos:

```ts
import { onBeforeUnmount, onMounted } from 'vue'
import PainelAcao from './PainelAcao.vue'
import { installShortcuts } from '@/game/shortcuts'

let off: (() => void) | null = null
onMounted(() => {
  off = installShortcuts(
    (a) => {
      const hand = game.view?.myHand ?? []
      if (a.kind === 'play') {
        const slot = hand[a.index]?.slot
        if (slot !== undefined) play(slot)
      } else if (a.kind === 'toggleCover') covered.value = !covered.value
      else if (a.kind === 'raise') document.querySelector<HTMLButtonElement>('[data-acao="aumentar"]')?.click()
      else void game.vote(a.choice)
    },
    () => ({
      canPlay: game.canPlay && !game.inflight,
      canCover: game.canCover,
      canRaise: game.canRaise,
      mustRespondRaise: game.mustRespondRaise,
      canRaiseBack: game.canRaiseBack,
      handSize: game.view?.myHand.length ?? 0,
    }),
  )
})
onBeforeUnmount(() => off?.())
```

(`raise` aciona o próprio botão para passar pela confirmação da Mão de 11/Ferro.)

- [ ] **Step 5: Rodar e ver passar**

Run: `cd frontend && pnpm vitest run && pnpm typecheck && pnpm lint`
Expected: PASS.

- [ ] **Step 6: Acessibilidade e commit**

Carregue `fixing-accessibility` (botões com teclas visíveis, `role="checkbox"` da queima, foco no modal). Rode os testes de novo.

```bash
git add frontend/src
git commit -m "feat(frontend): painel de ação por fase, confirmação de pedido proibido e atalhos de teclado"
```

---

### Task 19: Sinais, presença, W.O., fim de partida, anúncios e título da aba

**Skills:** `frontend-design:frontend-design`, `ui-ux-pro-max` (modal de fim e banner), `fixing-accessibility` (`aria-live`, ao final).

**Files:**
- Create: `frontend/src/components/mesa/{BarraSinais,BalaoSinal,BannerWO,FimPartida,Anunciador}.vue`
- Create: `frontend/src/components/mesa/extras.test.ts`
- Create: `frontend/src/game/tabTitle.ts`, `frontend/src/game/tabTitle.test.ts`
- Modify: `frontend/src/components/mesa/Mesa.vue`, `frontend/src/components/mesa/AssentoMesa.vue`, `frontend/src/views/SalaView.vue`, `frontend/src/stores/game.ts` (fila de anúncios)

**Interfaces:**
- Consumes: `useGameStore` (`signals`, `sendSignal`, `signalCooldownUntil`, `scene`), `useRoomStore` (`forfeitTeam`, `myTeam`, `claimForfeit`, `ended`, `dismissEnded`, `leave`), `SIGNALS`, `signalText`, `endReasonText`, `eventAnnouncement`, `Modal`, `Botao`.
- Produces:
  - `useGameStore()` ganha `announcement: string` (último texto para o `aria-live`), atualizado no `onEvent` do diretor via `eventAnnouncement(ev, nome)`; e "Sua vez" quando `isMyTurn` vira `true`.
  - `startTabTitleBlink(base: string, alert: string, opts?: { intervalMs?: number; doc?: Document; win?: Window }): () => void` — alterna o título enquanto a aba estiver sem foco; para no `focus`; devolve a função de parada.
  - `FimPartida.vue` lê `room.ended`; "Revanche" → `dismissEnded()`; "Sair da sala" → `leave()` e `dismissEnded()`.
  - `BannerWO.vue` aparece quando `forfeitTeam !== null && forfeitTeam !== myTeam`.

- [ ] **Step 1: Testes que falham**

`frontend/src/game/tabTitle.test.ts`:

```ts
import { startTabTitleBlink } from './tabTitle'

beforeEach(() => vi.useFakeTimers())
afterEach(() => vi.useRealTimers())

it('alterna o título sem foco e restaura ao focar', () => {
  vi.spyOn(document, 'hasFocus').mockReturnValue(false)
  document.title = 'Truco'
  const stop = startTabTitleBlink('Truco', 'Sua vez!', { intervalMs: 1000 })
  vi.advanceTimersByTime(1000)
  expect(document.title).toBe('Sua vez!')
  vi.advanceTimersByTime(1000)
  expect(document.title).toBe('Truco')
  vi.advanceTimersByTime(1000)
  window.dispatchEvent(new Event('focus'))
  expect(document.title).toBe('Truco')
  vi.advanceTimersByTime(5000)
  expect(document.title).toBe('Truco')
  stop()
})

it('com a aba em foco não faz nada', () => {
  vi.spyOn(document, 'hasFocus').mockReturnValue(true)
  document.title = 'Truco'
  startTabTitleBlink('Truco', 'Sua vez!', { intervalMs: 1000 })()
  vi.advanceTimersByTime(3000)
  expect(document.title).toBe('Truco')
})
```

`frontend/src/components/mesa/extras.test.ts`:

```ts
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setSender } from '@/app/transport'
import BarraSinais from './BarraSinais.vue'
import BannerWO from './BannerWO.vue'
import FimPartida from './FimPartida.vue'
import Anunciador from './Anunciador.vue'
import { useGameStore } from '@/stores/game'
import { useRoomStore } from '@/stores/room'
import { makeState } from '@/test/fixtures'

let pinia: ReturnType<typeof createPinia>
let send: ReturnType<typeof vi.fn>
beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
  send = vi.fn().mockResolvedValue(undefined)
  setSender(send)
  useRoomStore().onRoomState({
    id: 'r', code: 'ABC234', name: 'Sala', visibility: 'public', host: 0, status: 'playing', matchId: 'm',
    seats: [0, 1, 2, 3].map((s) => ({ userId: `u${s}`, displayName: `J${s}`, ready: true, connected: true, absent: false })),
    yourSeat: 0,
  })
})
afterEach(() => setSender(null))

it('barra de sinais envia e entra em espera', async () => {
  vi.useFakeTimers()
  const w = mount(BarraSinais, { global: { plugins: [pinia] } })
  await w.find('[data-sinal="boa"]').trigger('click')
  expect(send).toHaveBeenCalledWith({ type: 'signal', payload: { id: 'boa' } })
  await w.vm.$nextTick()
  expect(w.find('[data-sinal="vamos"]').attributes('disabled')).toBeDefined()
  vi.useRealTimers()
})

it('banner de W.O. só contra a dupla adversária', async () => {
  const room = useRoomStore()
  const w = mount(BannerWO, { global: { plugins: [pinia] } })
  expect(w.text()).toBe('')
  room.onForfeit({ team: 0 })
  await w.vm.$nextTick()
  expect(w.text()).toBe('')
  room.onForfeit({ team: 1 })
  await w.vm.$nextTick()
  await w.find('button').trigger('click')
  expect(send).toHaveBeenCalledWith({ type: 'claim_forfeit' })
})

it('fim de partida mostra resultado e motivo; revanche fecha', async () => {
  const room = useRoomStore()
  room.onMatchEnded({ reason: 'forfeit', winner: 0, score: [12, 7] })
  const w = mount(FimPartida, { global: { plugins: [pinia] }, attachTo: document.body })
  await w.vm.$nextTick()
  expect(document.body.textContent).toContain('Vitória')
  expect(document.body.textContent).toContain('Vitória por W.O.')
  expect(document.body.textContent).toContain('12 × 7')
  ;(document.querySelector('[data-acao="revanche"]') as HTMLButtonElement).click()
  expect(room.ended).toBeNull()
  w.unmount()
})

it('anunciador lê o último evento relevante', async () => {
  const game = useGameStore()
  game.onState(makeState())
  const w = mount(Anunciador, { global: { plugins: [pinia] } })
  game.onEvents([{ kind: 'raise_requested', seat: 1, stake: 3 }])
  await w.vm.$nextTick()
  expect(w.find('[aria-live="polite"]').text()).toBe('Bia pediu Truco!')
})
```

Run: `cd frontend && pnpm vitest run src/game/tabTitle.test.ts src/components/mesa/extras.test.ts`
Expected: FAIL.

- [ ] **Step 2: Título da aba e anúncios na store**

`frontend/src/game/tabTitle.ts`:

```ts
export function startTabTitleBlink(
  base: string,
  alert: string,
  opts: { intervalMs?: number; doc?: Document; win?: Window } = {},
): () => void {
  const doc = opts.doc ?? document
  const win = opts.win ?? window
  if (doc.hasFocus()) return () => {}
  let on = false
  const timer = win.setInterval(() => {
    on = !on
    doc.title = on ? alert : base
  }, opts.intervalMs ?? 1000)
  const stop = () => {
    win.clearInterval(timer)
    win.removeEventListener('focus', stop)
    doc.title = base
  }
  win.addEventListener('focus', stop)
  return stop
}
```

Em `stores/game.ts`:
- acrescente `announcement: ''` ao state;
- no `onEvent` do diretor, depois de `soundPlayer(ev)`:

```ts
          const text = eventAnnouncement(ev, (seat) => this.latest?.view.players.find((p) => p.seat === seat)?.name ?? `Jogador ${seat + 1}`)
          if (text) this.announcement = text
```

(import de `eventAnnouncement` de `@/game/labels`).

- [ ] **Step 3: Componentes**

`frontend/src/components/mesa/BarraSinais.vue`:

```vue
<script setup lang="ts">
import { onBeforeUnmount, ref } from 'vue'
import { useGameStore } from '@/stores/game'
import { SIGNALS } from '@/game/labels'

const game = useGameStore()
const now = ref(Date.now())
const timer = setInterval(() => (now.value = Date.now()), 250)
onBeforeUnmount(() => clearInterval(timer))

async function send(id: (typeof SIGNALS)[number]['id']) {
  await game.sendSignal(id)
  now.value = Date.now()
}
</script>

<template>
  <div class="flex flex-wrap justify-center gap-2" aria-label="Sinais rápidos">
    <button
      v-for="s in SIGNALS"
      :key="s.id"
      type="button"
      :data-sinal="s.id"
      :disabled="now < game.signalCooldownUntil"
      class="rounded-full bg-black/30 px-3 py-1 text-sm text-white hover:bg-black/50 disabled:opacity-40"
      @click="send(s.id)"
    >
      {{ s.text }}
    </button>
  </div>
</template>
```

`frontend/src/components/mesa/BalaoSinal.vue`:

```vue
<script setup lang="ts">
import { computed } from 'vue'
import type { Seat } from '@/protocol/engine'
import { useGameStore } from '@/stores/game'
import { signalText } from '@/game/labels'

const props = defineProps<{ seat: Seat }>()
const game = useGameStore()
const signal = computed(() => game.signals[props.seat])
</script>

<template>
  <Transition name="balao">
    <p v-if="signal" class="rounded-2xl bg-surface px-3 py-1 text-sm font-semibold text-ink shadow-card">{{ signalText(signal.id) }}</p>
  </Transition>
</template>

<style scoped>
.balao-enter-active,
.balao-leave-active {
  transition: all 0.2s ease;
}
.balao-enter-from,
.balao-leave-to {
  opacity: 0;
  transform: translateY(6px);
}
</style>
```

`frontend/src/components/mesa/BannerWO.vue`:

```vue
<script setup lang="ts">
import { computed } from 'vue'
import { useRoomStore } from '@/stores/room'
import { useToastsStore } from '@/stores/toasts'
import Botao from '@/components/ui/Botao.vue'

const room = useRoomStore()
const toasts = useToastsStore()
const show = computed(() => room.forfeitTeam !== null && room.forfeitTeam !== room.myTeam)

async function claim() {
  try {
    await room.claimForfeit()
  } catch (err) {
    toasts.error((err as { code?: string }).code ?? 'internal', (err as Error).message)
  }
}
</script>

<template>
  <div v-if="show" role="alert" class="flex items-center gap-3 rounded-xl bg-accent px-4 py-2 text-ink shadow-card">
    <span>Adversários ausentes — você pode pedir W.O.</span>
    <Botao variant="danger" @click="claim">Pedir W.O.</Botao>
  </div>
</template>
```

`frontend/src/components/mesa/FimPartida.vue`:

```vue
<script setup lang="ts">
import { computed } from 'vue'
import { useRoomStore } from '@/stores/room'
import { endReasonText } from '@/game/labels'
import Modal from '@/components/ui/Modal.vue'
import Botao from '@/components/ui/Botao.vue'

const room = useRoomStore()
const e = computed(() => room.ended)
const title = computed(() => {
  if (!e.value || e.value.winner === null) return 'Partida encerrada'
  return e.value.winner === room.myTeam ? 'Vitória!' : 'Derrota'
})

async function leave() {
  room.dismissEnded()
  await room.leave().catch(() => {})
}
</script>

<template>
  <Modal :open="e !== null" :title="title" @close="room.dismissEnded()">
    <template v-if="e">
      <p class="mb-2">{{ endReasonText(e.reason) }}</p>
      <p class="mb-6 font-display text-3xl">{{ e.score[0] }} × {{ e.score[1] }}</p>
      <div class="flex justify-end gap-3">
        <Botao variant="secondary" data-acao="sair" @click="leave">Sair da sala</Botao>
        <Botao data-acao="revanche" @click="room.dismissEnded()">Revanche</Botao>
      </div>
    </template>
  </Modal>
</template>
```

`frontend/src/components/mesa/Anunciador.vue`:

```vue
<script setup lang="ts">
import { useGameStore } from '@/stores/game'
const game = useGameStore()
</script>

<template>
  <div class="sr-only" aria-live="polite" aria-atomic="true">{{ game.announcement }}</div>
</template>
```

- [ ] **Step 4: Ligar na mesa e na sala**

- `AssentoMesa.vue`: no `<slot />`, renderize `<BalaoSinal :seat="props.seat" />`.
- `Mesa.vue`: acima do `Placar`, `<BannerWO class="self-center" />`; abaixo do `PainelAcao`, `<BarraSinais />`; no fim, `<Anunciador />`. No `watch` de `game.isMyTurn`: quando virar `true`, `game.announcement = 'Sua vez'` e, se `!document.hasFocus()`, `stopBlink = startTabTitleBlink(document.title, 'Sua vez!')`; quando virar `false`, `stopBlink?.()`.
- `SalaView.vue`: acrescente `<FimPartida />` fora dos `v-if` (o modal aparece sobre a sala de espera após a revanche).

- [ ] **Step 5: Rodar e ver passar**

Run: `cd frontend && pnpm vitest run && pnpm typecheck && pnpm lint && pnpm build`
Expected: PASS.

- [ ] **Step 6: Acessibilidade e commit**

Carregue `fixing-accessibility`: confirme que o `aria-live` não repete anúncios a cada render, que o modal de fim recebe foco e que o banner de W.O. é anunciado.

```bash
git add frontend/src
git commit -m "feat(frontend): sinais rápidos, W.O., modal de fim, anúncios aria-live e alerta no título da aba"
```

---

### Task 20: E2E com quatro navegadores

**Skills:** `e2e-testing`, `playwright-skill`.

**Files:**
- Create: `frontend/playwright.config.ts`
- Create: `frontend/e2e/helpers.ts`, `frontend/e2e/partida.spec.ts`
- Create: `scripts/e2e.sh`

**Interfaces:**
- Consumes: todas as telas; seletores `data-*` definidos nas Tarefas 14–19; log JSON do servidor (`LOG_FORMAT=json`) com a linha `"msg":"e-mail (MAIL_DRIVER=log)"` e campo `text` contendo `/verificar-email?token=…`.
- Produces: `make e2e` (build + backend + Playwright + limpeza).

- [ ] **Step 1: Script de orquestração**

`scripts/e2e.sh`:

```bash
#!/usr/bin/env bash
# Sobe Postgres + backend servindo o build do frontend, roda o Playwright e derruba tudo.
set -euo pipefail
cd "$(dirname "$0")/.."

export POSTGRES_PORT="${POSTGRES_PORT:-5433}"
docker compose up -d --wait postgres

(cd frontend && pnpm build)

log=$(mktemp)
export DATABASE_URL="postgres://truco:truco@localhost:${POSTGRES_PORT}/truco?sslmode=disable"
export HTTP_ADDR=":8090"
export APP_BASE_URL="http://localhost:8090"
export COOKIE_SECURE=false
export ALLOWED_EMAIL_DOMAINS="e2e.test"
export MAIL_DRIVER=log
export LOG_FORMAT=json
export LOG_LEVEL=info
export MIGRATE_ON_START=true
export STATIC_DIR="$PWD/frontend/dist"
export TIMER_PLAY_SECONDS=3 TIMER_VOTE_SECONDS=3 TIMER_CHOICE_SECONDS=3 TIMER_BURN_SECONDS=3
export TIMER_ABSENT_SECONDS=2 HAND_PAUSE_SECONDS=1
export RANKING_MIN_MATCHES=1

(cd backend && go run ./cmd/server serve >"$log" 2>&1) &
server=$!
trap 'kill $server 2>/dev/null || true; wait $server 2>/dev/null || true; rm -f "$log"' EXIT

for _ in $(seq 1 60); do
  curl -fsS http://localhost:8090/healthz >/dev/null 2>&1 && break
  sleep 1
done

(cd frontend && E2E_BASE_URL=http://localhost:8090 E2E_SERVER_LOG="$log" pnpm exec playwright test "$@")
```

`chmod +x scripts/e2e.sh`. Se `COOKIE_SECURE=false` não for aceito pela config (`backend/internal/config/config.go`), confira o nome/valor aceito e ajuste. O banco de e2e é o mesmo do compose; os usuários usam sufixo aleatório para não colidir entre execuções.

- [ ] **Step 2: Configuração do Playwright**

`frontend/playwright.config.ts`:

```ts
import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: 'e2e',
  timeout: 10 * 60_000,
  expect: { timeout: 15_000 },
  fullyParallel: false,
  workers: 1,
  reporter: [['list'], ['html', { open: 'never' }]],
  use: {
    baseURL: process.env.E2E_BASE_URL ?? 'http://localhost:8090',
    trace: 'retain-on-failure',
    ...devices['Desktop Chrome'],
  },
})
```

Run: `cd frontend && pnpm exec playwright install chromium`

- [ ] **Step 3: Helpers**

`frontend/e2e/helpers.ts`:

```ts
import { readFileSync } from 'node:fs'
import { expect, type Browser, type Page } from '@playwright/test'

export const PASSWORD = 'senha-de-teste-e2e-123'

/** Lê o log JSON do servidor e devolve o token de verificação mais recente para o e-mail. */
export async function verificationToken(email: string): Promise<string> {
  const file = process.env.E2E_SERVER_LOG
  if (!file) throw new Error('E2E_SERVER_LOG não definido')
  for (let i = 0; i < 50; i++) {
    const lines = readFileSync(file, 'utf8').split('\n').filter(Boolean)
    for (const line of lines.reverse()) {
      try {
        const entry = JSON.parse(line) as { msg?: string; to?: string; text?: string }
        if (entry.msg?.startsWith('e-mail') && entry.to === email) {
          const m = /verificar-email\?token=([A-Za-z0-9_-]+)/.exec(entry.text ?? '')
          if (m?.[1]) return m[1]
        }
      } catch {
        // linha não-JSON (go run)
      }
    }
    await new Promise((r) => setTimeout(r, 200))
  }
  throw new Error(`token de verificação não encontrado para ${email}`)
}

export async function newPlayer(browser: Browser, name: string): Promise<Page> {
  const suffix = Math.random().toString(36).slice(2, 7)
  const username = `${name.toLowerCase()}_${suffix}`
  const email = `${username}@e2e.test`
  const page = await (await browser.newContext()).newPage()
  await page.goto('/cadastro')
  await page.getByLabel(/Usuário/).fill(username)
  await page.getByLabel(/E-mail/).fill(email)
  await page.getByLabel(/Nome de exibição/).fill(name)
  await page.getByLabel(/Senha/).fill(PASSWORD)
  await page.getByRole('button', { name: 'Criar conta' }).click()
  await expect(page.getByText('Confira seu e-mail')).toBeVisible()
  await page.goto(`/verificar-email?token=${await verificationToken(email)}`)
  await expect(page.getByText('E-mail confirmado')).toBeVisible()
  await page.goto('/entrar')
  await page.getByLabel(/Usuário ou e-mail/).fill(username)
  await page.getByLabel(/^Senha$/).fill(PASSWORD)
  await page.getByRole('button', { name: 'Entrar' }).click()
  await expect(page.getByRole('heading', { name: 'Lobby' })).toBeVisible()
  return page
}

/** Executa a primeira ação disponível do painel; devolve true se agiu. */
export async function actOnce(page: Page): Promise<boolean> {
  const painel = page.locator('[data-painel]')
  if (!(await painel.isVisible().catch(() => false))) return false
  const candidates = [
    page.locator('[data-corte] button').first(),
    page.locator('[data-acao="passar"]'),
    page.locator('[data-voto="play"]'),
    page.locator('[data-voto="accept"]'),
    page.locator('[data-mao] button:not([disabled])').first(),
  ]
  for (const c of candidates) {
    if (await c.isVisible().catch(() => false)) {
      if (await c.isEnabled().catch(() => false)) {
        await c.click({ timeout: 2000 }).catch(() => {})
        return true
      }
    }
  }
  return false
}
```

- [ ] **Step 4: Cenários**

`frontend/e2e/partida.spec.ts`:

```ts
import { expect, test, type Page } from '@playwright/test'
import { actOnce, newPlayer } from './helpers'

async function playUntilEnd(players: Page[], onRound?: (round: number) => Promise<void>) {
  for (let round = 0; round < 2000; round++) {
    if (await players[0]!.locator('[data-acao="revanche"]').isVisible().catch(() => false)) return
    await onRound?.(round)
    let acted = false
    for (const p of players) acted = (await actOnce(p)) || acted
    if (!acted) await players[0]!.waitForTimeout(150)
  }
  throw new Error('partida não terminou')
}

async function setupMatch(browser: import('@playwright/test').Browser) {
  const names = ['Ana', 'Bia', 'Caio', 'Duda']
  const players: Page[] = []
  for (const n of names) players.push(await newPlayer(browser, n))
  const [host, ...rest] = players as [Page, Page, Page, Page]
  await host.locator('[data-criar="private"]').click()
  await expect(host.getByText(/Código/)).toBeVisible()
  const code = (await host.locator('header strong').innerText()).trim()
  for (const p of rest) {
    await p.goto(`/sala/${code.toLowerCase()}`)
    await expect(p.locator('[role="switch"]')).toBeVisible()
  }
  for (const p of players) await p.locator('[role="switch"]').click()
  for (const p of players) await expect(p.locator('[data-mao]')).toBeVisible()
  return players
}

test('quatro jogadores jogam uma partida completa', async ({ browser }) => {
  const players = await setupMatch(browser)
  await playUntilEnd(players)
  for (const p of players) {
    await expect(p.getByRole('dialog')).toBeVisible()
    await expect(p.getByRole('dialog')).toContainText(/Vitória|Derrota/)
  }
})

test('recarregar no meio da partida retoma a mesa', async ({ browser }) => {
  const players = await setupMatch(browser)
  // joga algumas rodadas, recarrega um jogador e confere placar e mão
  for (let i = 0; i < 20; i++) for (const p of players) await actOnce(p)
  const p = players[1]!
  const placarAntes = await p.locator('[aria-label="Placar"]').innerText()
  await p.reload()
  await expect(p.locator('[data-mao]')).toBeVisible()
  const placarDepois = await p.locator('[aria-label="Placar"]').innerText()
  expect(placarDepois.split('\n')[0]).toBe(placarAntes.split('\n')[0])
  await playUntilEnd(players)
})
```

Observações:
- O **timer curto** (3 s) garante que a partida anda mesmo se uma ação do helper falhar; o helper só acelera.
- A seleção "carta de menor slot" vem da ordem do DOM (`[data-mao] button:not([disabled])` primeiro). "Passar" na queima, "Jogar" na mão de 11 e "Aceitar" no aumento seguem a spec §9.3.
- O placar pode mudar entre ler e recarregar se uma mão terminar nesse intervalo; se o teste ficar instável por isso, compare o placar logo depois do reload com o placar atual de **outro** jogador (`players[0]`).

- [ ] **Step 5: Rodar**

Run: `make e2e`
Expected: 2 testes passam. Rode 3 vezes seguidas para confirmar estabilidade; se falhar, abra `frontend/playwright-report` e o trace.

- [ ] **Step 6: Commit**

```bash
git add frontend/playwright.config.ts frontend/e2e scripts/e2e.sh Makefile
git commit -m "test(frontend): e2e com quatro navegadores jogando uma partida e retomando após reload"
```

---

### Task 21: README, validação final e revisão de acessibilidade

**Skills:** `lint-and-validate`, `fixing-accessibility`, `documentation`.

**Files:**
- Modify: `README.md`

- [ ] **Step 1: README**

- Tabela de status: linha 4 vira **Frontend jogável (Spec 4a)** ✅ e acrescente a linha **4b — Histórico, ranking, replay e admin** 🗓️ Planejado.
- Atualize o badge de status e o parágrafo "Falta a interface (Spec 4)" para "Falta a Spec 4b (histórico, ranking, replay e painel admin)".
- Nova seção "Rodando o frontend":
  - dev: `make dev-db`, `make run` (com `APP_BASE_URL=http://localhost:5173`), `make web-install`, `make web-dev` → `http://localhost:5173`;
  - produção: `make web-build` e `STATIC_DIR=frontend/dist` no backend;
  - testes: `make web-test`; e2e: `make e2e` (requer Docker, Go, Node 24, pnpm, `pnpm exec playwright install chromium`).
- Em "Documentação", links para a spec e o plano da 4a.
- Seção de créditos de assets apontando para os `LICENSES.md` de cartas, sons e fontes.

- [ ] **Step 2: Validação final**

Run: `make web-test && make web-build && make test-short && make e2e`
Expected: tudo verde.

- [ ] **Step 3: Revisão de acessibilidade com o app rodando**

Carregue `fixing-accessibility`. Com o app em dev, navegue só com teclado: login → lobby → criar sala → pronto → jogar uma mão com `1`–`3`, `C`, `T`, `A`/`R`/`U` → modal de fim com `Esc`. Teste `prefers-reduced-motion` (DevTools → Rendering) e o tema escuro. Corrija e rode o Step 2 de novo.

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs: status da Spec 4a e como rodar o frontend"
```
