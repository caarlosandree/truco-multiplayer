<script setup lang="ts">
import { computed } from 'vue'
import { relativePosition } from '@/game/seats'
import { stakeName } from '@/game/labels'
import { useGameStore } from '@/stores/game'
import Carta from './Carta.vue'

/**
 * Camada de cena sobre a mesa (decorativa, `aria-hidden`; o texto vai para a região viva da Mesa):
 * distribuição e queima das cartas, anúncios escritos a giz e a placa do pedido proibido.
 */
const game = useGameStore()

const ev = computed(() => game.scene.event)
const view = computed(() => game.latest?.view ?? game.view)

interface Anuncio {
  texto: string
  tom: 'giz' | 'aposta'
  escurece: boolean
}
const anuncio = computed<Anuncio | null>(() => {
  const e = ev.value
  if (!e) return null
  switch (e.kind) {
    case 'hand_started': {
      const v = view.value
      const texto = v?.handKind === 'eleven' ? 'Mão de 11' : v?.handKind === 'iron' ? 'Mão de Ferro' : `Mão ${v?.handNumber ?? ''}`
      return { texto, tom: 'giz', escurece: false }
    }
    case 'raise_requested':
      return { texto: `${stakeName(e.stake ?? 0)}!`, tom: 'aposta', escurece: true }
    case 'raise_resolved':
      return { texto: e.choice === 'run' ? 'Correu!' : 'Aceito!', tom: 'giz', escurece: false }
    default:
      return null
  }
})

const distribuicao = computed(() => {
  if (ev.value?.kind !== 'cards_dealt' || !view.value) return []
  const { dealer, seat } = view.value
  // 12 cartas, uma a uma, a partir de quem fica depois do pé. O evento cards_dealt não traz a ordem; esta é a
  // do servidor (backend/internal/engine/deal.go, dealRemaining: Next(dealer), Next(...) em volta da mesa).
  // Na mão normal o corte já entregou duas cartas, então é uma aproximação visual, não o registro exato.
  return Array.from({ length: 12 }, (_, i) => ({
    i,
    pos: relativePosition((dealer + 1 + i) % 4, seat),
  }))
})
const queima = computed(() => {
  const e = ev.value
  if (e?.kind !== 'fours_burned' || e.seat === undefined || !view.value) return []
  const pos = relativePosition(e.seat, view.value.seat)
  return Array.from({ length: Math.min(e.count ?? 1, 4) }, (_, i) => ({ i, pos }))
})
const punido = computed(() => ev.value?.kind === 'raise_punished')
</script>

<template>
  <div
    class="pointer-events-none absolute inset-0 overflow-hidden [container-type:size]"
    aria-hidden="true"
    data-cena
  >
    <Transition name="mesa-escurece">
      <div
        v-if="anuncio?.escurece"
        class="absolute inset-0 bg-lousa/55"
      />
    </Transition>

    <div
      v-for="c in distribuicao"
      :key="`${game.scene.key}-d${c.i}`"
      class="mesa-voo mesa-voo--distribui absolute top-1/2 left-1/2 -mt-5 -ml-3.5"
      :class="`mesa-dest-${c.pos}`"
      :style="{ animationDelay: `${c.i * 35}ms` }"
    >
      <Carta
        size="xs"
        hidden
        decorative
      />
    </div>
    <div
      v-for="c in queima"
      :key="`${game.scene.key}-q${c.i}`"
      class="mesa-voo mesa-voo--queima absolute top-1/2 left-1/2 -mt-5 -ml-3.5"
      :class="`mesa-dest-${c.pos}`"
      :style="{ animationDelay: `${c.i * 50}ms` }"
    >
      <Carta
        size="xs"
        hidden
        decorative
      />
    </div>

    <div class="absolute inset-0 z-(--z-anuncio) grid place-items-center">
      <Transition name="mesa-anuncio">
        <p
          v-if="anuncio"
          :key="game.scene.key"
          data-anuncio
          class="relative -rotate-4 px-4 font-display text-anuncio-sm font-bold md:text-anuncio"
          :class="anuncio.tom === 'aposta' ? 'text-accent' : 'text-giz'"
        >
          <span class="mesa-escrever block">{{ anuncio.texto }}</span>
          <svg
            v-if="anuncio.tom === 'aposta'"
            class="absolute inset-x-3 -bottom-1 h-3 w-[calc(100%-1.5rem)]"
            viewBox="0 0 100 8"
            preserveAspectRatio="none"
            aria-hidden="true"
          >
            <path
              d="M1 5 Q30 1 60 5 T99 3"
              fill="none"
              pathLength="1"
              class="mesa-riscar mesa-riscar--sublinha stroke-accent"
              style="stroke-width: 3px; stroke-dasharray: 1; stroke-linecap: round; vector-effect: non-scaling-stroke"
            />
          </svg>
        </p>
      </Transition>
      <Transition name="mesa-anuncio">
        <p
          v-if="punido"
          :key="game.scene.key"
          class="flex max-w-xs items-center gap-3 rounded-painel bg-lousa px-5 py-4 text-lg font-bold text-giz border-2 border-danger"
        >
          <svg
            viewBox="0 0 16 16"
            class="size-6 shrink-0 text-danger"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
            aria-hidden="true"
          >
            <path d="M8 2l6.5 11.5h-13zM8 6.5v3.2M8 11.7v.1" />
          </svg>
          Pedido proibido — derrota
        </p>
      </Transition>
    </div>
  </div>
</template>
