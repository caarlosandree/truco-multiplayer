<script setup lang="ts">
const props = withDefaults(
  defineProps<{
    variant?: 'primary' | 'secondary' | 'danger' | 'ghost' | 'aposta'
    loading?: boolean
    disabled?: boolean
    type?: 'button' | 'submit'
  }>(),
  { variant: 'primary', loading: false, disabled: false, type: 'button' },
)

const styles = {
  primary:
    'rounded-botao bg-(--botao-pri-bg) text-(--botao-pri-fg) hover:bg-(--botao-pri-bg-hover) active:bg-(--botao-pri-bg-active) font-bold',
  secondary:
    'rounded-botao-2 bg-(--botao-sec-bg) text-(--botao-sec-fg) inset-ring-2 inset-ring-(--botao-sec-borda) hover:bg-(--botao-sec-bg-hover) active:bg-(--botao-sec-bg-active) font-bold',
  danger:
    'rounded-botao bg-(--botao-perigo-bg) text-(--botao-perigo-fg) hover:bg-(--botao-perigo-bg-hover) active:bg-(--botao-perigo-bg-active) font-bold',
  ghost: 'rounded-botao-2 bg-transparent text-ink hover:bg-(--botao-fantasma-bg-hover) active:bg-(--botao-sec-bg-active) font-bold',
  aposta:
    'rounded-aposta bg-(--botao-aposta-bg) text-(--botao-aposta-fg) hover:bg-(--botao-aposta-bg-hover) active:bg-(--botao-aposta-bg-active) font-display text-xl font-bold',
} as const
</script>

<template>
  <button
    :type="props.type"
    :disabled="props.disabled || props.loading"
    :aria-busy="props.loading || undefined"
    class="relative inline-flex h-(--botao-altura) min-w-11 items-center justify-center gap-2 px-4 transition-colors duration-(--dur-rapido) ease-padrao active:translate-y-(--botao-pressao-y) disabled:cursor-not-allowed"
    :class="[styles[props.variant], props.loading ? '' : 'disabled:opacity-(--botao-desabilitado-opacidade)']"
  >
    <span
      class="inline-flex items-center justify-center gap-2"
      :class="props.loading ? 'opacity-0' : ''"
    ><slot /></span>
    <span
      v-if="props.loading"
      class="absolute inset-0 flex items-center justify-center gap-1"
      aria-hidden="true"
      data-testid="botao-carregando"
    >
      <span
        v-for="n in 3"
        :key="n"
        class="botao-ponto size-1.5 rounded-full bg-current"
        :style="{ animationDelay: `${(n - 1) * 150}ms` }"
      />
    </span>
  </button>
</template>

<style scoped>
/* três pontos de giz que sobem e descem em sequência; só transform/opacity */
.botao-ponto {
  animation: botao-ponto 900ms var(--ease-giz) infinite;
}
@keyframes botao-ponto {
  0%,
  70%,
  100% {
    opacity: 0.35;
    transform: translateY(0);
  }
  35% {
    opacity: 1;
    transform: translateY(-3px);
  }
}
</style>
