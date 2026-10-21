<script setup lang="ts">
import { ref } from 'vue'
import { cardUrl } from '@/game/cardAssets'

withDefaults(defineProps<{ title: string; frase?: string }>(), { frase: 'Puxa a cadeira.' })
const titulo = ref<HTMLElement | null>(null)

// 3 de copas, ás de espadas e 7 de ouros: ids 38, 29 e 12
const LEQUE = [
  { id: 12, classe: '-rotate-12 translate-y-2' },
  { id: 29, classe: 'z-10 -ml-9' },
  { id: 38, classe: 'rotate-12 -ml-9 translate-y-2' },
]

/** Depois de trocar formulário por mensagem, o foco vai para o título (o formulário sumiu). */
defineExpose({ focusTitle: () => titulo.value?.focus() })
</script>

<template>
  <div class="flex flex-1 flex-col lg:grid lg:grid-cols-2">
    <!-- lousa: sempre escura, como a mesa; decorativa, fora da ordem de foco e da árvore de acessibilidade -->
    <aside
      data-tema="escuro"
      class="textura-lousa flex items-center justify-between gap-4 px-4 py-4 text-giz lg:flex-col lg:items-start lg:justify-between lg:gap-0 lg:border-r-2 lg:border-line/40 lg:px-12 lg:py-12"
      aria-hidden="true"
    >
      <div class="flex items-center gap-2 text-2xl font-bold tracking-tight">
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

      <p class="font-display text-xl font-bold lg:-rotate-2 lg:text-6xl lg:leading-tight">
        {{ frase }}
      </p>

      <div class="hidden w-full flex-col gap-8 lg:flex">
        <svg
          viewBox="0 0 200 56"
          class="w-56 max-w-full"
          fill="none"
          stroke-width="3"
          stroke-linecap="round"
        >
          <g class="tento-giz stroke-(--tento-nos)">
            <path
              d="M8 6v40M28 6v40M48 6v40M68 6v40M2 36L74 14"
              pathLength="1"
            />
          </g>
          <g class="tento-giz stroke-(--tento-eles)">
            <path
              d="M100 6v40M120 6v40M140 6v40"
              pathLength="1"
              style="animation-delay: 300ms"
            />
          </g>
        </svg>
        <div class="flex items-end pl-4">
          <img
            v-for="c in LEQUE"
            :key="c.id"
            :src="cardUrl(c.id, 'frances')"
            alt=""
            class="w-(--carta-largura-mao) rounded-card shadow-card"
            :class="c.classe"
          >
        </div>
      </div>
    </aside>

    <div class="mx-auto flex w-full max-w-100 flex-1 flex-col px-4 py-8 lg:justify-center lg:px-0 lg:py-16">
      <h1
        ref="titulo"
        tabindex="-1"
        class="mb-6 text-2xl font-bold text-balance focus:outline-none"
      >
        {{ title }}
      </h1>
      <slot />
    </div>
  </div>
</template>

<style scoped>
/* o tento se risca a giz uma vez ao abrir; com movimento reduzido já aparece pronto */
.tento-giz path {
  stroke-dasharray: 1;
  animation: tento-giz var(--dur-truco) var(--ease-giz) both;
}
@keyframes tento-giz {
  from {
    stroke-dashoffset: 1;
  }
  to {
    stroke-dashoffset: 0;
  }
}
@media (prefers-reduced-motion: reduce) {
  .tento-giz path {
    animation: none;
  }
}
</style>
