<script setup lang="ts">
import Botao from '@/components/ui/Botao.vue'

/** Carregando / erro / vazio / conteúdo de uma lista do painel. O conteúdo vem no slot padrão. */
defineProps<{
  status: 'idle' | 'loading' | 'ok' | 'error'
  total: number
  carregando: string
  erro: string
  vazio: string
  vazioDica?: string
}>()
const emit = defineEmits<{ again: [] }>()
</script>

<template>
  <div
    v-if="status === 'loading' || status === 'idle'"
    class="flex flex-col gap-1"
    data-testid="admin-carregando"
    aria-busy="true"
  >
    <p class="sr-only">
      {{ carregando }}
    </p>
    <div
      v-for="n in 5"
      :key="n"
      class="carregando h-11 rounded-placa bg-surface-2"
      aria-hidden="true"
    />
  </div>
  <div
    v-else-if="status === 'error'"
    class="flex flex-col items-start gap-3 rounded-painel bg-surface-2 p-5"
  >
    <p class="font-bold">
      {{ erro }}
    </p>
    <p class="text-ink-muted">
      Confira sua conexão e tente de novo.
    </p>
    <Botao
      variant="secondary"
      @click="emit('again')"
    >
      Tentar de novo
    </Botao>
  </div>
  <div
    v-else-if="total === 0"
    class="rounded-painel bg-surface-2 p-5"
  >
    <p class="font-bold">
      {{ vazio }}
    </p>
    <p
      v-if="vazioDica"
      class="text-ink-muted"
    >
      {{ vazioDica }}
    </p>
  </div>
  <slot v-else />
</template>

<style scoped>
.carregando {
  animation: admin-pulso 1.4s var(--ease-giz) infinite;
}
@keyframes admin-pulso {
  50% {
    opacity: 0.55;
  }
}
@media (prefers-reduced-motion: reduce) {
  .carregando {
    animation: none;
  }
}
</style>
