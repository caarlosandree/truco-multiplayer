<script setup lang="ts">
import { computed } from 'vue'
import { useGameStore } from '@/stores/game'
import Carta from './Carta.vue'

const game = useGameStore()
const bloqueado = computed(() => game.inflight || !game.online)
</script>

<template>
  <div
    data-corte
    class="flex justify-center gap-4"
  >
    <Carta
      v-for="o in game.latestView?.cutOffer ?? []"
      :key="o.keep"
      :card="o.card"
      size="lg"
      interactive
      :disabled="bloqueado"
      @select="game.chooseCard(o.keep)"
    />
  </div>
</template>
