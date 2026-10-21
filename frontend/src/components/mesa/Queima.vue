<script setup lang="ts">
import { computed, ref } from 'vue'
import { useGameStore } from '@/stores/game'
import Carta from './Carta.vue'
import Botao from '@/components/ui/Botao.vue'

const game = useGameStore()
const selected = ref<number[]>([])
const bloqueado = computed(() => game.inflight || !game.online)
const fours = computed(() => (game.latestView?.myHand ?? []).filter((h) => game.fourSlots.includes(h.slot)))

function toggle(slot: number) {
  selected.value = selected.value.includes(slot) ? selected.value.filter((s) => s !== slot) : [...selected.value, slot].sort((a, b) => a - b)
}
</script>

<template>
  <div
    data-queima
    class="flex flex-col gap-3"
  >
    <div
      role="group"
      aria-label="Cartas 4 da sua mão"
      class="flex flex-wrap justify-center gap-3"
    >
      <button
        v-for="h in fours"
        :key="h.slot"
        type="button"
        role="checkbox"
        :aria-checked="selected.includes(h.slot)"
        :disabled="bloqueado"
        class="relative rounded-card transition-[translate,box-shadow] duration-(--dur-rapido) ease-padrao disabled:cursor-not-allowed disabled:opacity-(--botao-desabilitado-opacidade)"
        :class="selected.includes(h.slot) ? '-translate-y-2 ring-4 ring-ink' : ''"
        @click="toggle(h.slot)"
      >
        <Carta
          :card="h.card"
          size="md"
        />
        <span
          v-if="selected.includes(h.slot)"
          aria-hidden="true"
          class="absolute -top-2 -right-2 grid size-6 place-items-center rounded-full bg-ink text-sm font-bold text-surface"
        >✓</span>
      </button>
    </div>
    <Botao
      data-acao="queimar"
      data-primaria
      class="w-full"
      :disabled="selected.length === 0 || bloqueado"
      @click="game.burn(selected)"
    >
      Queimar {{ selected.length > 0 ? `(${selected.length})` : '' }}
    </Botao>
    <Botao
      data-acao="passar"
      variant="secondary"
      class="w-full"
      :disabled="bloqueado"
      @click="game.passBurn()"
    >
      Passar
    </Botao>
  </div>
</template>
