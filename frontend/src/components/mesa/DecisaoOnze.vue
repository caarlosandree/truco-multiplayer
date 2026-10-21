<script setup lang="ts">
import { computed } from 'vue'
import { useGameStore } from '@/stores/game'
import MaoParceiro from './MaoParceiro.vue'
import Botao from '@/components/ui/Botao.vue'

const game = useGameStore()
const bloqueado = computed(() => game.inflight || !game.online)
</script>

<template>
  <div class="flex flex-col gap-3">
    <MaoParceiro />
    <div class="flex gap-3 curto:gap-2">
      <Botao
        data-voto="play"
        data-primaria
        class="flex-1"
        :disabled="bloqueado"
        @click="game.vote('play')"
      >
        Jogar
      </Botao>
      <Botao
        data-voto="run"
        variant="secondary"
        class="flex-1"
        :disabled="bloqueado"
        @click="game.vote('run')"
      >
        Correr
      </Botao>
    </div>
  </div>
</template>
