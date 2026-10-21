<script setup lang="ts">
import { computed } from 'vue'
import { useToastsStore } from '@/stores/toasts'

const toasts = useToastsStore()
// no máximo 3 empilhados; erros em região role="alert", avisos em role="status" (R19)
const infos = computed(() => toasts.items.filter((t) => t.kind === 'info').slice(-3))
const erros = computed(() => toasts.items.filter((t) => t.kind === 'error').slice(-3))
</script>

<template>
  <div class="pointer-events-none fixed inset-x-0 top-14 z-(--z-toast) flex flex-col items-center gap-2 px-4">
    <TransitionGroup
      tag="div"
      name="toast"
      class="flex flex-col items-center gap-2"
      role="status"
      aria-live="polite"
    >
      <div
        v-for="t in infos"
        :key="t.id"
        class="pointer-events-auto flex max-w-md items-center gap-3 rounded-toast bg-(--toast-info-bg) py-1 pr-1 pl-4 text-(--toast-info-fg) shadow-toast"
      >
        <span>{{ t.text }}</span>
        <button
          type="button"
          class="grid size-11 shrink-0 place-items-center rounded-toast hover:bg-(--toast-info-fg)/15"
          aria-label="Fechar aviso"
          @click="toasts.dismiss(t.id)"
        >
          <svg
            viewBox="0 0 16 16"
            class="size-4"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            aria-hidden="true"
          >
            <path d="M3 3l10 10M13 3L3 13" />
          </svg>
        </button>
      </div>
    </TransitionGroup>
    <TransitionGroup
      tag="div"
      name="toast"
      class="flex flex-col items-center gap-2"
      role="alert"
    >
      <div
        v-for="t in erros"
        :key="t.id"
        class="pointer-events-auto flex max-w-md items-center gap-3 rounded-toast bg-(--toast-erro-bg) py-1 pr-1 pl-4 text-(--toast-erro-fg) shadow-toast"
      >
        <span>{{ t.text }}</span>
        <button
          type="button"
          class="grid size-11 shrink-0 place-items-center rounded-toast hover:bg-(--toast-erro-fg)/15"
          aria-label="Fechar aviso"
          @click="toasts.dismiss(t.id)"
        >
          <svg
            viewBox="0 0 16 16"
            class="size-4"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            aria-hidden="true"
          >
            <path d="M3 3l10 10M13 3L3 13" />
          </svg>
        </button>
      </div>
    </TransitionGroup>
  </div>
</template>

<style scoped>
.toast-enter-active {
  transition:
    opacity 200ms var(--ease-desliza),
    transform 200ms var(--ease-desliza);
}
.toast-leave-active {
  transition:
    opacity 150ms var(--ease-sai),
    transform 150ms var(--ease-sai);
}
.toast-enter-from,
.toast-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}
</style>
