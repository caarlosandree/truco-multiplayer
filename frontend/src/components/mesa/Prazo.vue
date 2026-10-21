<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useSessionStore } from '@/stores/session'
import { deadlineFraction, remainingMs } from '@/game/deadline'
import { sfx } from '@/audio/install'

/** Anel em volta da placa de quem precisa agir + segundos num selo no canto. Pai precisa ser `relative`. */
const props = defineProps<{ deadline?: number; totalMs: number; mine: boolean }>()
const session = useSessionStore()
const fraction = ref(1)
const seconds = ref<number | null>(null)
const ALERT_S = 5

let raf: number | ReturnType<typeof setTimeout> = 0
let warned = false

function tick() {
  const now = session.serverNow()
  fraction.value = deadlineFraction(props.deadline, props.totalMs, now)
  const left = remainingMs(props.deadline, now)
  seconds.value = left === null ? null : Math.ceil(left / 1000)
  if (props.mine && left !== null && left > 0 && left <= ALERT_S * 1000 && !warned) {
    warned = true
    sfx()?.play('timeLow')
  }
  schedule()
}
function schedule() {
  raf = typeof requestAnimationFrame === 'function' ? requestAnimationFrame(tick) : setTimeout(tick, 100)
}
function stop() {
  if (typeof cancelAnimationFrame === 'function') cancelAnimationFrame(raf as number)
  clearTimeout(raf)
}

watch(
  () => props.deadline,
  () => {
    warned = false
  },
)
onMounted(tick)
onBeforeUnmount(stop)

const alerta = computed(() => seconds.value !== null && seconds.value <= ALERT_S)
</script>

<template>
  <div
    v-if="seconds !== null"
    data-prazo
    :data-alerta="alerta ? '' : undefined"
    role="timer"
    :aria-label="`${seconds} segundos restantes`"
    class="pointer-events-none absolute -inset-3 z-(--z-carta)"
  >
    <svg
      class="absolute inset-[2px] h-[calc(100%-4px)] w-[calc(100%-4px)] overflow-visible"
      aria-hidden="true"
    >
      <rect
        width="100%"
        height="100%"
        rx="15"
        fill="none"
        :class="alerta ? 'stroke-(--prazo-alerta)/40' : 'stroke-(--prazo-trilho)'"
        :style="{ strokeWidth: props.mine ? 'calc(var(--prazo-traco) + 2px)' : 'var(--prazo-traco)' }"
      />
      <rect
        width="100%"
        height="100%"
        rx="15"
        fill="none"
        pathLength="1"
        :class="alerta ? 'stroke-(--prazo-alerta)' : 'stroke-(--prazo-cor)'"
        :style="{
          strokeWidth: props.mine ? 'calc(var(--prazo-traco) + 2px)' : 'var(--prazo-traco)',
          strokeDasharray: `${fraction} 1`,
        }"
      />
    </svg>
    <span
      aria-hidden="true"
      class="absolute -top-2.5 -right-2.5 inline-flex min-w-7 items-center justify-center rounded-full px-1.5 font-bold tabular-nums inset-ring-1"
      :class="[
        alerta ? 'bg-danger text-on-danger inset-ring-danger' : 'bg-lousa-2 text-giz inset-ring-line',
        props.mine ? (alerta ? 'h-10 min-w-11 text-xl' : 'h-8 min-w-9 text-base') : 'h-6 text-sm',
      ]"
    >{{ seconds }}<span class="ml-0.5 text-xs font-normal">s</span></span>
  </div>
</template>
