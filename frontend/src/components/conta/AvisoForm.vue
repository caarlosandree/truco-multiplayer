<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'

const props = withDefaults(defineProps<{ kind?: 'erro' | 'ok'; retryAfterSec?: number }>(), { kind: 'erro', retryAfterSec: undefined })

const emit = defineEmits<{ fim: [] }>()
const restante = ref(0)
let timer: ReturnType<typeof setInterval> | undefined

function parar() {
  if (timer !== undefined) clearInterval(timer)
  timer = undefined
}

watch(
  () => props.retryAfterSec,
  (sec) => {
    parar()
    restante.value = sec ?? 0
    if (restante.value <= 0) return
    timer = setInterval(() => {
      restante.value -= 1
      if (restante.value <= 0) {
        parar()
        emit('fim')
      }
    }, 1000)
  },
  { immediate: true },
)
onBeforeUnmount(parar)

const erro = computed(() => props.kind === 'erro')
</script>

<template>
  <p
    :role="erro ? 'alert' : 'status'"
    :tabindex="erro ? -1 : undefined"
    class="flex items-start gap-2 text-sm"
    :class="erro ? 'text-danger' : 'text-ink'"
  >
    <svg
      viewBox="0 0 16 16"
      class="mt-0.5 size-4 shrink-0"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
      aria-hidden="true"
    >
      <path
        v-if="erro"
        d="M8 2v7M8 12.5v.5"
      />
      <path
        v-else
        d="M3 8.5l3.5 3.5L13 4.5"
      />
    </svg>
    <span class="min-w-0">
      <slot />
      <template v-if="props.retryAfterSec">
        <!-- leitor de tela ouve o tempo inicial uma vez; a contagem visual não é anunciada -->
        <span class="sr-only"> Tente de novo em {{ props.retryAfterSec }} s.</span>
        <span
          v-if="restante > 0"
          aria-hidden="true"
          class="tabular-nums"
        > Tente de novo em {{ restante }} s.</span>
      </template>
    </span>
  </p>
</template>
