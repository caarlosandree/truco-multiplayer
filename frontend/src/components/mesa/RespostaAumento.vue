<script setup lang="ts">
import { computed } from 'vue'
import { useGameStore } from '@/stores/game'
import { nextStake, stakeName } from '@/game/labels'
import MaoParceiro from './MaoParceiro.vue'
import TeclaDica from './TeclaDica.vue'
import Botao from '@/components/ui/Botao.vue'

const game = useGameStore()
const bloqueado = computed(() => game.inflight || !game.online)
const back = computed(() => {
  const to = game.latestView?.pending?.toStake
  return game.canRaiseBack && to !== undefined ? nextStake(to) : null
})
</script>

<template>
  <div class="flex flex-col gap-3">
    <MaoParceiro />
    <div class="flex flex-col gap-3 curto:flex-row curto:gap-2">
      <Botao
        data-voto="accept"
        data-primaria
        aria-keyshortcuts="A"
        class="w-full curto:w-auto curto:flex-[1.3]"
        :disabled="bloqueado"
        @click="game.vote('accept')"
      >
        Aceitar <TeclaDica tecla="A" />
      </Botao>
      <div class="flex gap-3 curto:contents">
        <Botao
          data-voto="run"
          variant="secondary"
          aria-keyshortcuts="R"
          class="flex-1"
          :disabled="bloqueado"
          @click="game.vote('run')"
        >
          Correr <TeclaDica tecla="R" />
        </Botao>
        <Botao
          v-if="back !== null"
          data-voto="raise"
          data-aposta
          variant="aposta"
          aria-keyshortcuts="U"
          class="flex-1"
          :disabled="bloqueado"
          @click="game.vote('raise')"
        >
          {{ stakeName(back) }}! <TeclaDica tecla="U" />
        </Botao>
      </div>
    </div>
  </div>
</template>
