<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { RouterLink } from 'vue-router'
import { useRankingStore, type Metric, type Period, type RankingEntry } from '@/stores/ranking'
import { useSessionStore } from '@/stores/session'
import Botao from '@/components/ui/Botao.vue'

const ranking = useRankingStore()
const session = useSessionStore()

const ABAS: { id: Metric; label: string }[] = [
  { id: 'wins', label: 'Vitórias' },
  { id: 'winrate', label: 'Aproveitamento' },
]
const PERIODOS: { id: Period; label: string }[] = [
  { id: 'all', label: 'Geral' },
  { id: 'month', label: 'Mês' },
  { id: 'week', label: 'Semana' },
]
const VAZIO: Record<Period, string> = {
  all: 'Ninguém jogou ainda',
  month: 'Ninguém jogou neste mês ainda',
  week: 'Ninguém jogou nesta semana ainda',
}

const abaEl: Partial<Record<Metric, HTMLElement>> = {}
const percentual = new Intl.NumberFormat('pt-BR', { style: 'percent', minimumFractionDigits: 1, maximumFractionDigits: 1 })

const meuUsername = computed(() => session.user?.username)
const itens = computed(() => ranking.result?.items ?? [])
/** Linhas da tabela: o top e, no fim, minha linha quando estou fora dele. */
const linhas = computed(() => {
  const me = ranking.result?.me
  const top = itens.value.map((entry) => ({ entry, fora: false }))
  if (!me || itens.value.some((e) => e.username === me.username)) return top
  return [...top, { entry: me, fora: true }]
})
const destaque = (l: { entry: RankingEntry; fora: boolean }) => l.fora || l.entry.username === meuUsername.value

function guardarAba(metric: Metric, el: unknown) {
  if (el instanceof HTMLElement) abaEl[metric] = el
}
function carregar(period: Period, metric: Metric) {
  void ranking.load(period, metric)
}
/** Escolher o filtro já ativo não refaz a requisição. */
function escolher(period: Period, metric: Metric) {
  if (period === ranking.period && metric === ranking.metric) return
  carregar(period, metric)
}
function irParaAba(metric: Metric) {
  escolher(ranking.period, metric)
  abaEl[metric]?.focus()
}
function teclas(e: KeyboardEvent) {
  const i = ABAS.findIndex((a) => a.id === ranking.metric)
  const alvo =
    e.key === 'ArrowRight' ? ABAS[(i + 1) % ABAS.length]
    : e.key === 'ArrowLeft' ? ABAS[(i - 1 + ABAS.length) % ABAS.length]
    : e.key === 'Home' ? ABAS[0]
    : e.key === 'End' ? ABAS[ABAS.length - 1]
    : undefined
  if (!alvo) return
  e.preventDefault()
  irParaAba(alvo.id)
}

const perfil = (username: string) => ({ name: 'jogador', params: { username } })

onMounted(() => carregar(ranking.period, ranking.metric))
</script>

<template>
  <div class="mx-auto flex w-full max-w-2xl flex-col gap-5 px-4 py-6 md:px-6 md:py-8">
    <h1 class="text-2xl font-bold">
      Ranking
    </h1>
    <div class="flex flex-wrap items-center justify-between gap-x-4 gap-y-3">
      <div
        role="tablist"
        aria-label="Critério do ranking"
        class="flex gap-2"
      >
        <button
          v-for="a in ABAS"
          :id="`aba-ranking-${a.id}`"
          :key="a.id"
          :ref="(el) => guardarAba(a.id, el)"
          type="button"
          role="tab"
          :aria-selected="ranking.metric === a.id"
          aria-controls="painel-ranking"
          :tabindex="ranking.metric === a.id ? 0 : -1"
          class="h-11 border-b-4 px-3 font-bold"
          :class="ranking.metric === a.id ? 'border-ink text-ink' : 'border-transparent text-ink-muted hover:text-ink'"
          @click="escolher(ranking.period, a.id)"
          @keydown="teclas"
        >
          {{ a.label }}
        </button>
      </div>
      <div
        role="radiogroup"
        aria-label="Período"
        class="flex rounded-botao-2 inset-ring-2 inset-ring-line"
      >
        <label
          v-for="p in PERIODOS"
          :key="p.id"
          class="has-focus-visible:outline-3 has-focus-visible:outline-focus relative inline-flex h-11 min-w-20 cursor-pointer items-center justify-center rounded-botao-2 px-3 font-bold has-checked:bg-ink has-checked:text-surface"
        >
          <input
            type="radio"
            name="periodo-ranking"
            class="sr-only"
            :value="p.id"
            :checked="ranking.period === p.id"
            @change="escolher(p.id, ranking.metric)"
          >
          {{ p.label }}
        </label>
      </div>
    </div>
    <div
      id="painel-ranking"
      role="tabpanel"
      :aria-labelledby="`aba-ranking-${ranking.metric}`"
      class="flex flex-col gap-3"
    >
      <p
        v-if="ranking.metric === 'winrate' && ranking.result"
        class="text-sm text-ink-muted"
      >
        Só quem jogou {{ ranking.result.minMatches }} ou mais partidas no período.
      </p>
      <div
        v-if="ranking.status === 'loading' || ranking.status === 'idle'"
        class="flex flex-col gap-1"
        data-testid="ranking-carregando"
        aria-busy="true"
      >
        <p class="sr-only">
          Carregando o ranking
        </p>
        <div
          v-for="n in 6"
          :key="n"
          class="carregando h-11 rounded-placa bg-surface-2"
          aria-hidden="true"
        />
      </div>
      <div
        v-else-if="ranking.status === 'error'"
        class="flex flex-col items-start gap-3 rounded-painel bg-surface-2 p-5"
      >
        <p class="font-bold">
          Não foi possível carregar o ranking.
        </p>
        <p class="text-ink-muted">
          Confira sua conexão e tente de novo.
        </p>
        <Botao
          variant="secondary"
          @click="carregar(ranking.period, ranking.metric)"
        >
          Tentar de novo
        </Botao>
      </div>
      <div
        v-else-if="itens.length === 0"
        class="rounded-painel bg-surface-2 p-5"
      >
        <p class="font-bold">
          {{ VAZIO[ranking.period] }}
        </p>
        <p class="text-ink-muted">
          As partidas terminadas entram aqui.
        </p>
      </div>
      <table
        v-else
        class="w-full table-fixed border-collapse tabular-nums"
      >
        <caption class="sr-only">
          Ranking por {{ ranking.metric === 'wins' ? 'vitórias' : 'aproveitamento' }}
        </caption>
        <thead>
          <tr class="border-b-2 border-line/50 text-left text-sm text-ink-muted">
            <th
              scope="col"
              class="h-10 w-11 pr-1 text-right font-normal sm:w-14"
            >
              #
            </th>
            <th
              scope="col"
              class="pl-3 font-normal"
            >
              Jogador
            </th>
            <th
              scope="col"
              class="w-11 text-right font-normal sm:w-16"
            >
              V
            </th>
            <th
              scope="col"
              class="w-11 text-right font-normal sm:w-16"
            >
              D
            </th>
            <th
              scope="col"
              class="w-18 pr-2 text-right font-normal sm:w-24"
            >
              %
            </th>
          </tr>
        </thead>
        <tbody>
          <template
            v-for="l in linhas"
            :key="l.entry.username"
          >
            <tr
              v-if="l.fora"
              data-testid="ranking-divisor"
              aria-hidden="true"
            >
              <td
                colspan="5"
                class="h-9 pl-3 text-sm text-ink-muted"
              >
                Sua posição
              </td>
            </tr>
            <tr
              :aria-current="destaque(l) ? 'true' : undefined"
              class="border-b border-line/30"
              :class="[destaque(l) ? 'bg-surface-2 font-bold' : '', l.fora ? 'border-y-2 border-line/50' : '']"
            >
              <td class="h-11 pr-1 text-right">
                {{ l.entry.position }}
              </td>
              <td class="min-w-0 pl-3">
                <div class="flex min-w-0 items-center gap-2">
                  <RouterLink
                    :to="perfil(l.entry.username)"
                    class="inline-flex h-11 min-w-0 items-center truncate hover:underline"
                  >
                    <span class="truncate">{{ l.entry.displayName }}</span>
                  </RouterLink>
                  <span
                    v-if="destaque(l)"
                    class="shrink-0 text-sm font-normal text-ink-muted"
                  >você</span>
                </div>
              </td>
              <td
                class="text-right"
                :class="ranking.metric === 'wins' ? 'font-bold' : ''"
              >
                {{ l.entry.wins }}
              </td>
              <td class="text-right">
                {{ l.entry.losses }}
              </td>
              <td
                class="pr-2 text-right"
                :class="ranking.metric === 'winrate' ? 'font-bold' : ''"
              >
                {{ percentual.format(l.entry.winRate) }}
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
.carregando {
  animation: ranking-pulso 1.4s var(--ease-giz) infinite;
}
@keyframes ranking-pulso {
  50% {
    opacity: 0.55;
  }
}
@media (prefers-reduced-motion: reduce) {
  .carregando {
    animation: none;
  }
}
</style>
