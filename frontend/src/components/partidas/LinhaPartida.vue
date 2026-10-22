<script setup lang="ts">
import { computed } from 'vue'
import { teamOf } from '@/game/seats'
import type { HistoryItem } from '@/stores/players'
import SeloBot from '@/components/ui/SeloBot.vue'

const props = defineProps<{ item: HistoryItem; viewer: string }>()

const quando = new Intl.DateTimeFormat('pt-BR', { dateStyle: 'medium', timeStyle: 'short' })

/** Dupla de quem vê: pelo assento dele (teamOf); sem ele na lista, o time que o servidor mandou. */
const minhaDupla = computed(() => {
  const eu = props.item.players.find((p) => p.username === props.viewer)
  return eu ? teamOf(eu.seat) : props.item.team
})
const parceiro = computed(() => props.item.players.find((p) => p.username !== props.viewer && teamOf(p.seat) === minhaDupla.value))
const adversarios = computed(() =>
  props.item.players.filter((p) => teamOf(p.seat) !== minhaDupla.value).sort((a, b) => a.seat - b.seat),
)

const resultado = computed(() => {
  if (props.item.status === 'in_progress') return 'Em andamento'
  if (props.item.winnerTeam === null) return 'Sem vencedor'
  return props.item.won ? 'Vitória' : 'Derrota'
})
/** O que ocupa o lugar do placar: W.O., abortada, abandonada ou os tentos orientados à dupla de quem vê. */
const placar = computed(() => {
  const { endReason, score } = props.item
  if (endReason === 'forfeit') return 'W.O.'
  if (endReason === 'aborted') return 'Abortada'
  if (endReason === 'abandoned') return 'Abandonada'
  if (!score) return null
  const meu = score[minhaDupla.value] ?? 0
  const deles = score[minhaDupla.value === 0 ? 1 : 0] ?? 0
  return `${meu}×${deles}`
})
const comTentos = computed(() => placar.value?.includes('×') ?? false)
const data = computed(() => props.item.finishedAt ?? props.item.createdAt)
const dataTexto = computed(() => {
  const d = new Date(data.value)
  return Number.isNaN(d.getTime()) ? '' : quando.format(d)
})
/** Separador entre nomes da lista "A, B e C". */
const sep = (i: number, total: number) => (i === 0 ? '' : i === total - 1 ? ' e ' : ', ')
</script>

<template>
  <li
    data-testid="partida"
    class="flex items-start justify-between gap-4 border-t-2 border-line/40 py-3 first:border-t-0"
  >
    <div class="min-w-0">
      <p class="flex flex-wrap items-center gap-x-2 gap-y-1 font-bold">
        {{ resultado }}
        <span
          v-if="!item.ranked"
          data-com-bots
          class="rounded-placa px-1.5 py-px text-xs/4 font-bold text-ink-muted inset-ring-1 inset-ring-line"
        >com bots</span>
      </p>
      <!-- bots aparecem pelo nome, com selo e sem link de perfil -->
      <p
        v-if="parceiro"
        class="text-pretty text-ink-muted [overflow-wrap:anywhere]"
      >
        com {{ parceiro.displayName }}<SeloBot
          v-if="parceiro.bot"
          class="ml-1.5 align-middle"
        />
      </p>
      <p
        v-if="adversarios.length"
        class="text-pretty text-ink-muted [overflow-wrap:anywhere]"
      >
        contra <span
          v-for="(p, i) in adversarios"
          :key="p.seat"
        >{{ sep(i, adversarios.length) }}{{ p.displayName }}<SeloBot
          v-if="p.bot"
          class="ml-1.5 align-middle"
        /></span>
      </p>
      <p class="text-sm text-ink-muted">
        <time :datetime="data">{{ dataTexto }}</time>
      </p>
    </div>
    <p
      v-if="placar"
      class="shrink-0 pt-0.5 text-right tabular-nums"
      :class="comTentos ? 'font-display text-valor font-bold' : 'text-sm font-bold text-ink-muted'"
    >
      <span
        v-if="comTentos"
        class="sr-only"
      >Placar </span>{{ placar }}
    </p>
  </li>
</template>
