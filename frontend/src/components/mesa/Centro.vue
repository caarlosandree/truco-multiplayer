<script setup lang="ts">
import { computed } from 'vue'
import { rankLabel } from '@/game/cards'
import { useCena } from '@/game/cena'
import { useGameStore } from '@/stores/game'
import { usePrefsStore } from '@/stores/prefs'
import Carta from './Carta.vue'
import Vaza from './Vaza.vue'

const game = useGameStore()
const prefs = usePrefsStore()
const cena = useCena()
const v = computed(() => game.view)

// a vira da cena (revelada agora) ou a do view; só a revelada na cena faz o giro
const vira = computed(() => cena.vira ?? v.value?.vira)
const revelando = computed(() => cena.vira !== undefined)
// `latest` está à frente do `view` durante a cena, então a manilha da vira nova já está nele
const manilha = computed(() => (revelando.value ? game.latest?.view.manilha : v.value?.manilha) ?? v.value?.manilha)

type Marca = 'ganha' | 'perdida' | 'empate' | 'aberta'
const marcas = computed<{ marca: Marca; texto: string }[]>(() => {
  const view = v.value
  if (!view) return []
  const feitas: (number | undefined)[] = view.rounds.map((r) => r.winner)
  // a rodada que fechou na cena aparece na hora, antes do `view` chegar
  if (cena.round && feitas.length < 3) feitas.push(cena.round.team)
  return [0, 1, 2].map((i) => {
    if (i >= feitas.length) return { marca: 'aberta', texto: `Rodada ${i + 1}: em aberto` }
    const t = feitas[i]
    if (t === undefined) return { marca: 'empate', texto: `Rodada ${i + 1}: empate` }
    return t === game.myTeam
      ? { marca: 'ganha', texto: `Rodada ${i + 1}: nossa` }
      : { marca: 'perdida', texto: `Rodada ${i + 1}: deles` }
  })
})
</script>

<template>
  <div
    v-if="v"
    class="flex flex-col items-center gap-2 md:gap-3"
  >
    <div class="flex items-center justify-center gap-3 md:gap-8">
      <div class="flex w-16 flex-col items-center gap-1 md:w-24">
        <div
          v-if="vira !== undefined"
          :key="vira"
          class="relative"
          :class="revelando ? 'mesa-vira' : ''"
        >
          <Carta
            :card="vira"
            size="sm"
            :class="revelando ? 'mesa-vira__face' : ''"
          />
          <Carta
            v-if="revelando"
            hidden
            size="sm"
            decorative
            class="mesa-vira__verso absolute inset-0"
          />
        </div>
        <p
          v-if="manilha !== undefined"
          class="text-center text-sm leading-tight"
        >
          <span class="text-giz-muted">manilha</span>
          <strong
            class="relative ml-1 font-display text-valor"
          >{{ rankLabel(manilha, prefs.deck) }}<svg
            v-if="revelando"
            class="pointer-events-none absolute inset-x-0 -bottom-0.5 h-1.5 w-full"
            viewBox="0 0 20 4"
            preserveAspectRatio="none"
            aria-hidden="true"
          ><path
            d="M0 2 Q10 4 20 1"
            fill="none"
            pathLength="1"
            class="mesa-riscar stroke-giz"
            style="stroke-width: 1.5; stroke-dasharray: 1; animation-delay: 300ms"
          /></svg></strong>
        </p>
      </div>
      <Vaza />
      <div class="w-16 text-xs leading-snug text-giz-muted md:w-24">
        <p>monte {{ v.deckCount }}</p>
        <p>descarte {{ v.discardCount }}</p>
      </div>
    </div>
    <ol
      class="flex items-center gap-3"
      aria-label="Rodadas desta mão"
    >
      <li
        v-for="(m, i) in marcas"
        :key="i"
        class="grid size-7 place-items-center rounded-full"
        :class="m.marca === 'aberta' ? 'border-2 border-dashed border-giz-muted/60' : 'bg-lousa text-giz inset-ring-1 inset-ring-line'"
        :data-marca="m.marca"
      >
        <span class="sr-only">{{ m.texto }}</span>
        <svg
          v-if="m.marca !== 'aberta'"
          viewBox="0 0 16 16"
          class="size-4"
          fill="none"
          stroke="currentColor"
          stroke-width="2.5"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <path
            v-if="m.marca === 'ganha'"
            d="M3 8.5l3.2 3L13 4.5"
          />
          <path
            v-else-if="m.marca === 'perdida'"
            d="M4 4l8 8M12 4l-8 8"
          />
          <path
            v-else
            d="M3.5 6h9M3.5 10h9"
          />
        </svg>
      </li>
    </ol>
  </div>
</template>
