<script setup lang="ts">
import type { HandSlotView } from '@/protocol/engine'
import Carta from './Carta.vue'
import TeclaDica from './TeclaDica.vue'

const props = defineProps<{ slots: HandSlotView[]; interactive: boolean; compacta?: boolean }>()
const emit = defineEmits<{ play: [slot: number] }>()
</script>

<template>
  <TransitionGroup
    tag="div"
    name="mesa-mao"
    data-mao
    role="group"
    aria-label="Sua mão"
    :class="props.compacta ? 'max-md:[--mao-altura:4.9rem] curto:max-md:[--mao-altura:3.7rem]' : 'max-md:[--mao-altura:8rem] curto:max-md:[--mao-altura:6.6rem]'"
    class="relative flex min-h-(--mao-altura) justify-center gap-3 [--mao-altura:7.7rem] md:mb-6"
  >
    <div
      v-for="(s, i) in props.slots"
      :key="s.slot"
      class="relative"
    >
      <Carta
        :card="s.card"
        :hidden="s.hidden"
        :label="s.hidden ? `Carta ${i + 1}` : undefined"
        :aria-keyshortcuts="String(i + 1)"
        :size="props.compacta ? 'lgc' : 'lg'"
        interactive
        :disabled="!props.interactive"
        @select="emit('play', s.slot)"
      />
      <TeclaDica
        v-show="props.interactive"
        :tecla="String(i + 1)"
        class="absolute -bottom-6 left-1/2 -translate-x-1/2 text-giz-muted"
      />
    </div>
  </TransitionGroup>
</template>
