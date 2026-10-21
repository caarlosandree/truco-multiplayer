<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoomStore } from '@/stores/room'
import { useToastsStore } from '@/stores/toasts'
import Botao from '@/components/ui/Botao.vue'

/**
 * Faixa de lousa no topo da coluna da direita: a dupla adversária está ausente e dá para pedir o W.O.
 * Não é alarme (sem vermelho): é uma informação com uma ação. O anúncio para leitores de tela é do Anunciador.
 */
const room = useRoomStore()
const toasts = useToastsStore()
const show = computed(() => room.forfeitTeam !== null && room.forfeitTeam !== room.myTeam)
const enviando = ref(false)

async function claim() {
  enviando.value = true
  try {
    await room.claimForfeit()
  } catch (err) {
    toasts.fromError(err)
  } finally {
    enviando.value = false
  }
}
</script>

<template>
  <div
    v-if="show"
    data-banner-wo
    class="flex flex-wrap items-center gap-3 rounded-painel bg-(--painel-bg) p-3 text-(--painel-fg)"
  >
    <svg
      viewBox="0 0 16 16"
      class="size-5 shrink-0 text-giz-muted"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
      aria-hidden="true"
    >
      <path d="M8 14a6 6 0 1 0 0-12 6 6 0 0 0 0 12zM8 4.5V8l2.5 1.5" />
    </svg>
    <p class="min-w-0 flex-1 basis-40 text-sm">
      <strong class="block">Adversários ausentes</strong>
      <span class="text-(--painel-dica)">Você pode encerrar a partida por W.O.</span>
    </p>
    <Botao
      variant="secondary"
      class="shrink-0 lg:w-full"
      :loading="enviando"
      @click="claim"
    >
      Pedir W.O.
    </Botao>
  </div>
</template>
