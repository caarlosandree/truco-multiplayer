<script setup lang="ts">
import { computed, onBeforeUnmount, ref, useId, watch } from 'vue'
import type { SignalID } from '@/protocol/messages'
import { SIGNALS } from '@/game/labels'
import { useGameStore } from '@/stores/game'

/**
 * Sinais rápidos: secundários ao painel de ação. Desktop: grade 3×2 sempre à vista na coluna da lousa.
 * Celular: um botão "Sinais" abre a folha com a grade (some ao enviar ou com Esc). Cada envio põe todos os botões
 * em espera por 3 s (o servidor também limita).
 */
const game = useGameStore()
const gradeId = useId()
const aberto = ref(false)
const raiz = ref<HTMLElement | null>(null)
const toggle = ref<HTMLButtonElement | null>(null)

// fecha a folha e devolve o foco ao botão "Sinais" (os botões da grade somem; sem isso o foco cai no body)
function recolher(devolverFoco: boolean) {
  const estavaAberto = aberto.value
  aberto.value = false
  if (estavaAberto && devolverFoco) toggle.value?.focus()
}

const agora = ref(Date.now())
const restanteS = computed(() => Math.max(0, Math.ceil((game.signalCooldownUntil - agora.value) / 1000)))
const espera = computed(() => restanteS.value > 0)
const bloqueado = computed(() => espera.value || !game.online)

let timer: ReturnType<typeof setInterval> | undefined
watch(
  () => game.signalCooldownUntil,
  () => {
    agora.value = Date.now()
    if (timer || !espera.value) return
    timer = setInterval(() => {
      agora.value = Date.now()
      if (!espera.value) {
        clearInterval(timer)
        timer = undefined
      }
    }, 250)
  },
)

async function enviar(id: SignalID) {
  recolher(true)
  await game.sendSignal(id)
}

function fechar(e: Event) {
  if (e instanceof KeyboardEvent) {
    if (e.key === 'Escape') recolher(true)
  } else if (!raiz.value?.contains(e.target as Node)) recolher(false)
}
watch(aberto, (a) => {
  const m = a ? 'addEventListener' : 'removeEventListener'
  document[m]('keydown', fechar)
  document[m]('pointerdown', fechar)
})
onBeforeUnmount(() => {
  clearInterval(timer)
  document.removeEventListener('keydown', fechar)
  document.removeEventListener('pointerdown', fechar)
})
</script>

<template>
  <div
    ref="raiz"
    data-sinais
    class="relative flex flex-col gap-2"
  >
    <div class="flex items-center justify-between gap-3 text-sm text-(--painel-dica) max-lg:hidden">
      <span>Sinais</span>
      <span
        v-if="espera"
        class="tabular-nums"
        data-sinais-espera
      >Aguarde {{ restanteS }} s</span>
    </div>
    <button
      ref="toggle"
      type="button"
      data-sinais-toggle
      :aria-expanded="aberto"
      :aria-controls="gradeId"
      class="ml-auto inline-flex h-11 min-w-11 items-center gap-2 rounded-botao-2 px-3 text-sm font-bold text-(--painel-fg) inset-ring-1 inset-ring-line/60 hover:bg-giz/10 lg:hidden"
      @click="aberto = !aberto"
    >
      Sinais
      <span
        v-if="espera"
        class="font-normal text-(--painel-dica) tabular-nums"
      >{{ restanteS }} s</span>
      <svg
        viewBox="0 0 16 16"
        class="size-4 transition-transform duration-(--dur-rapido) ease-padrao"
        :class="aberto ? 'rotate-180' : ''"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      ><path d="M3.5 10.5 8 6l4.5 4.5" /></svg>
    </button>
    <div
      :id="gradeId"
      role="group"
      aria-label="Sinais rápidos"
      class="grid-cols-3 gap-2 lg:grid"
      :class="[
        aberto ? 'grid' : 'max-lg:hidden',
        'max-lg:absolute max-lg:right-0 max-lg:bottom-full max-lg:mb-2 max-lg:w-[min(20rem,calc(100vw-1.5rem))] max-lg:rounded-painel max-lg:bg-(--painel-bg) max-lg:p-2 max-lg:shadow-toast',
      ]"
    >
      <button
        v-for="s in SIGNALS"
        :key="s.id"
        type="button"
        :data-sinal="s.id"
        :disabled="bloqueado"
        class="h-11 min-w-11 rounded-botao-2 px-1 text-sm font-bold text-(--painel-fg) inset-ring-1 inset-ring-line/60 transition-colors duration-(--dur-rapido) ease-padrao hover:bg-giz/10 active:translate-y-(--botao-pressao-y) disabled:cursor-not-allowed disabled:opacity-(--botao-desabilitado-opacidade)"
        @click="enviar(s.id)"
      >
        {{ s.text }}
      </button>
    </div>
  </div>
</template>
