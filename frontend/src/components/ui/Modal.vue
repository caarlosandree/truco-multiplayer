<script setup lang="ts">
import { nextTick, ref, useId, watch } from 'vue'

const props = defineProps<{ open: boolean; title: string }>()
const emit = defineEmits<{ close: [] }>()
const dialog = ref<HTMLElement | null>(null)
const titleId = useId()
let previous: HTMLElement | null = null

const FOCUSABLE = 'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'

function focusables(): HTMLElement[] {
  return dialog.value
    ? [...dialog.value.querySelectorAll<HTMLElement>(FOCUSABLE)].filter((el) => !el.hasAttribute('disabled'))
    : []
}

watch(
  () => props.open,
  async (open) => {
    if (open) {
      previous = document.activeElement as HTMLElement | null
      await nextTick()
      ;(focusables()[0] ?? dialog.value)?.focus()
    } else {
      previous?.focus()
      previous = null
    }
  },
  { immediate: true },
)

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    e.stopPropagation()
    emit('close')
    return
  }
  if (e.key !== 'Tab') return
  const items = focusables()
  const first = items[0]
  const last = items.at(-1)
  if (!first || !last) {
    e.preventDefault()
    return
  }
  if (e.shiftKey && document.activeElement === first) {
    e.preventDefault()
    last.focus()
  } else if (!e.shiftKey && document.activeElement === last) {
    e.preventDefault()
    first.focus()
  }
}
</script>

<template>
  <Teleport to="body">
    <Transition name="modal">
      <div
        v-if="props.open"
        class="fixed inset-0 z-(--z-modal) grid place-items-center bg-(--modal-scrim) p-4"
        @click.self="emit('close')"
      >
        <div
          ref="dialog"
          role="dialog"
          aria-modal="true"
          :aria-labelledby="titleId"
          tabindex="-1"
          class="modal-caixa w-full max-w-(--modal-largura) rounded-modal bg-(--modal-bg) p-6 text-(--modal-fg) shadow-modal"
          @keydown="onKeydown"
        >
          <h2
            :id="titleId"
            class="mb-4 text-xl font-bold"
          >
            {{ props.title }}
          </h2>
          <slot />
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.modal-enter-active {
  transition: background-color var(--dur-medio) var(--ease-padrao);
}
.modal-leave-active {
  transition: opacity 150ms var(--ease-sai);
}
.modal-enter-active .modal-caixa {
  transition:
    opacity var(--dur-medio) var(--ease-padrao),
    transform var(--dur-medio) var(--ease-padrao);
}
.modal-enter-from {
  background-color: transparent;
}
.modal-enter-from .modal-caixa {
  opacity: 0;
  transform: scale(0.98);
}
.modal-leave-to {
  opacity: 0;
}
</style>
