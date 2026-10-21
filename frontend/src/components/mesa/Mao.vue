<script setup lang="ts">
import type { HandSlotView } from '@/protocol/engine'
import Carta from './Carta.vue'

const props = defineProps<{ slots: HandSlotView[]; interactive: boolean }>()
const emit = defineEmits<{ play: [slot: number] }>()
</script>

<template>
  <TransitionGroup
    tag="div"
    name="mesa-mao"
    data-mao
    role="group"
    aria-label="Sua mão"
    class="relative flex min-h-(--mao-altura) justify-center gap-3 [--mao-altura:7.7rem] max-md:[--mao-altura:8rem]"
  >
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
</template>
