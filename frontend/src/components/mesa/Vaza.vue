<script setup lang="ts">
import { computed } from 'vue'
import type { Seat } from '@/protocol/engine'
import { cardLabel } from '@/game/cards'
import { useCena } from '@/game/cena'
import { relativePosition, type RelativePosition } from '@/game/seats'
import { useGameStore } from '@/stores/game'
import { usePrefsStore } from '@/stores/prefs'
import Carta from './Carta.vue'

const game = useGameStore()
const prefs = usePrefsStore()
const cena = useCena()

// cada carta fica deslocada para o lado de quem jogou (sem rotação)
const OFFSET: Record<RelativePosition, string> = {
  bottom: '--ox:0;--oy:1',
  top: '--ox:0;--oy:-1',
  left: '--ox:-1;--oy:0',
  right: '--ox:1;--oy:0',
}
const DIR: Record<RelativePosition, [number, number]> = { bottom: [0, 1], top: [0, -1], left: [-1, 0], right: [1, 0] }

const nameOf = (seat: Seat) => game.view?.players.find((p) => p.seat === seat)?.name ?? `Lugar ${seat + 1}`

// o que já está na mesa + as jogadas da cena que o `view` ainda não aplicou
const cards = computed(() => {
  const v = game.view
  if (!v) return []
  const table = [...v.table, ...cena.flights.filter((f) => !v.table.some((t) => t.seat === f.seat))]
  const winner = game.scene.event?.kind === 'round_ended' ? game.scene.event.seat : undefined
  return table.map((t) => ({
    ...t,
    pos: relativePosition(t.seat, v.seat),
    destaque: winner === t.seat,
    label: t.covered || t.card === undefined ? `${nameOf(t.seat)} jogou uma carta coberta` : `${nameOf(t.seat)} jogou ${cardLabel(t.card, prefs.deck)}`,
  }))
})

// a vaza recolhida vai para o lado de quem ganhou a rodada (a que acabou de entrar em `rounds`)
const recolhe = computed(() => {
  const v = game.view
  const seat = v?.rounds.at(-1)?.winnerSeat
  if (!v || seat === undefined) return '--wx:0;--wy:-1'
  const [x, y] = DIR[relativePosition(seat, v.seat)]
  return `--wx:${x};--wy:${y}`
})
</script>

<template>
  <TransitionGroup
    tag="div"
    name="mesa-vaza"
    data-vaza
    role="group"
    aria-label="Cartas na mesa"
    class="mesa-vaza"
    :style="recolhe"
  >
    <div
      v-for="c in cards"
      :key="c.seat"
      class="mesa-vaza__slot"
      :style="OFFSET[c.pos]"
      :data-vaza-seat="c.seat"
    >
      <Carta
        :card="c.card"
        :covered="c.covered"
        :label="c.label"
        size="md"
      />
      <!-- fim de rodada: círculo de giz em volta da carta vencedora -->
      <svg
        v-if="c.destaque"
        data-destaque
        class="pointer-events-none absolute -inset-2.5 size-[calc(100%+20px)] overflow-visible"
        aria-hidden="true"
      >
        <rect
          width="100%"
          height="100%"
          rx="16"
          fill="none"
          pathLength="1"
          class="mesa-riscar stroke-giz"
          style="stroke-width: 3px; stroke-dasharray: 1"
        />
      </svg>
    </div>
  </TransitionGroup>
</template>
