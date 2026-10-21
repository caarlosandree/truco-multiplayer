<script setup lang="ts">
import { computed } from 'vue'
import type { Seat } from '@/protocol/engine'
import type { RelativePosition } from '@/game/seats'
import { signalText } from '@/game/labels'
import { useGameStore } from '@/stores/game'
import { BALAO_LADO } from './balaoLado'

/**
 * Balão de sinal sobre a placa do assento (~3 s, quem some é a store). Decorativo: quem escuta o sinal
 * é o Anunciador. `deslocado`: já há um balão de cena no assento; este sobe um degrau para não se sobrepor.
 */
const props = defineProps<{ seat: Seat; position: RelativePosition; deslocado?: boolean }>()
const game = useGameStore()
const signal = computed(() => game.signals[props.seat])
const lado = computed(() => BALAO_LADO[props.position])
</script>

<template>
  <Transition name="mesa-balao">
    <p
      v-if="signal"
      :key="signal.until"
      data-balao-sinal
      aria-hidden="true"
      class="absolute left-1/2 z-(--z-balao) w-max max-w-44 -translate-x-1/2 rounded-toast bg-(--balao-bg) px-3 py-1.5 text-center text-sm font-bold text-(--balao-fg) inset-ring-1 inset-ring-line"
      :class="props.deslocado ? lado.deslocado : lado.balao"
    >
      {{ signalText(signal.id) }}
      <span
        class="absolute left-1/2 size-2 -translate-x-1/2 rotate-45 bg-(--balao-bg) inset-ring-1 inset-ring-line"
        :class="lado.rabicho"
      />
    </p>
  </Transition>
</template>
