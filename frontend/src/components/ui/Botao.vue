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
    class="relative inline-flex h-(--botao-altura) min-w-11 items-center justify-center gap-2 px-4 transition-colors duration-(--dur-rapido) ease-padrao active:translate-y-(--botao-pressao-y) disabled:cursor-not-allowed disabled:opacity-(--botao-desabilitado-opacidade)"
    :class="styles[props.variant]"
  >
    <span
      class="inline-flex items-center justify-center gap-2"
      :class="props.loading ? 'invisible' : ''"
    ><slot /></span>
    <span
      v-if="props.loading"
      class="absolute inset-0 grid place-items-center"
      aria-hidden="true"
    >…</span>
  </button>
</template>
