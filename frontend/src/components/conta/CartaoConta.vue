<script setup lang="ts">
import { ref } from 'vue'

defineProps<{ title: string }>()
const titulo = ref<HTMLElement | null>(null)

/** Depois de trocar formulário por mensagem, o foco vai para o título (o formulário sumiu). */
defineExpose({ focusTitle: () => titulo.value?.focus() })
</script>

<template>
  <div class="mx-auto flex w-full max-w-100 flex-1 flex-col px-4 py-8 md:px-0 md:py-16">
    <div
      class="flex items-center gap-2 text-2xl font-bold tracking-tight"
      aria-hidden="true"
    >
      <!-- o mesmo tento do cabeçalho: quatro riscos e a diagonal -->
      <svg
        viewBox="0 0 32 24"
        class="h-6 w-8 shrink-0"
        fill="none"
        stroke="currentColor"
        stroke-width="2.5"
        stroke-linecap="round"
      >
        <path d="M6 3v18M13 3v18M20 3v18M27 3v18M2 17L31 7" />
      </svg>
      Truco
    </div>
    <h1
      ref="titulo"
      tabindex="-1"
      class="mt-10 mb-6 text-2xl font-bold text-balance focus:outline-none"
    >
      {{ title }}
    </h1>
    <slot />
  </div>
</template>
