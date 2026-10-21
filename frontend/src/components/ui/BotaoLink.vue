<script setup lang="ts">
import { RouterLink, type RouteLocationRaw } from 'vue-router'

const props = withDefaults(defineProps<{ to: RouteLocationRaw; variant?: 'primary' | 'secondary' }>(), { variant: 'primary' })
const styles = {
  primary: 'rounded-botao bg-(--botao-pri-bg) text-(--botao-pri-fg) hover:bg-(--botao-pri-bg-hover) active:bg-(--botao-pri-bg-active)',
  secondary:
    'rounded-botao-2 bg-(--botao-sec-bg) text-(--botao-sec-fg) inset-ring-2 inset-ring-(--botao-sec-borda) hover:bg-(--botao-sec-bg-hover) active:bg-(--botao-sec-bg-active)',
} as const
</script>

<template>
  <RouterLink
    :to="props.to"
    class="inline-flex h-(--botao-altura) items-center justify-center px-4 font-bold transition-colors duration-(--dur-rapido) ease-padrao active:translate-y-(--botao-pressao-y)"
    :class="styles[props.variant]"
  >
    <slot />
  </RouterLink>
</template>
