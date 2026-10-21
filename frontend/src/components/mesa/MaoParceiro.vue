<script setup lang="ts">
import { voteText } from '@/game/labels'
import { useGameStore } from '@/stores/game'
import Carta from './Carta.vue'

const game = useGameStore()
</script>

<template>
  <div
    v-if="game.latestView?.partnerHand?.length"
    data-parceiro
    class="flex flex-col items-center gap-2 max-lg:flex-row max-lg:gap-3"
  >
    <div
      role="group"
      aria-label="Mão do parceiro"
      class="flex justify-center gap-2"
    >
      <Carta
        v-for="h in game.latestView.partnerHand"
        :key="h.slot"
        :card="h.card"
        :hidden="h.hidden"
        size="md"
      />
    </div>
    <p class="text-sm text-(--painel-dica)">
      <span class="text-giz">Parceiro:</span>
      {{ game.latestView.partnerVote ? voteText(game.latestView.partnerVote) : 'ainda não decidiu' }}
    </p>
  </div>
</template>
