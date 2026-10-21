<script setup lang="ts">
import { computed, ref, watch } from 'vue'

/**
 * Tentos de giz em grupos de cinco (quatro riscos e a diagonal). Os riscos novos se desenham um a um
 * (`stroke-dashoffset`); no primeiro render e com movimento reduzido já aparecem prontos.
 */
const props = defineProps<{ count: number; tone: 'nos' | 'eles' }>()

const GROUP_W = 30
const GAP = 6
const H = 24

const prev = ref(props.count)
const novos = ref<{ from: number; to: number; stamp: number } | null>(null)
let stamp = 0
watch(
  () => props.count,
  (n, o) => {
    prev.value = o
    novos.value = n > o ? { from: o, to: n, stamp: ++stamp } : null
  },
)

const grupos = computed(() => {
  const total = Math.max(0, Math.min(props.count, 12))
  return Array.from({ length: Math.ceil(total / 5) }, (_, g) => {
    const n = Math.min(5, total - g * 5)
    const x0 = g * (GROUP_W + GAP)
    const riscos = Array.from({ length: Math.min(n, 4) }, (_, i) => ({ index: g * 5 + i, d: `M${x0 + 4 + i * 7} 2v${H - 4}` }))
    // o quinto tento é a diagonal que corta os quatro riscos
    if (n === 5) riscos.push({ index: g * 5 + 4, d: `M${x0 + 1} ${H - 5}L${x0 + GROUP_W - 1} 5` })
    return riscos
  })
})
const width = computed(() => {
  const g = grupos.value
  const ultimo = g.at(-1)
  if (!ultimo) return 1
  // o último grupo só ocupa o que tem: cada risco 7 px; com a diagonal, o grupo todo
  const largura = ultimo.length === 5 ? GROUP_W : 4 + ultimo.length * 7
  return (g.length - 1) * (GROUP_W + GAP) + largura
})

// cada risco novo leva min(400, max(40, floor(600/N))) ms (brief §7.2)
const passo = computed(() => {
  const n = novos.value ? novos.value.to - novos.value.from : 1
  return Math.min(400, Math.max(40, Math.floor(600 / n)))
})
function estilo(index: number): Record<string, string> | undefined {
  const n = novos.value
  if (!n || index < n.from || index >= n.to) return undefined
  return { animationDuration: `${passo.value}ms`, animationDelay: `${(index - n.from) * passo.value}ms` }
}
function novo(index: number): boolean {
  const n = novos.value
  return !!n && index >= n.from && index < n.to
}
</script>

<template>
  <svg
    :viewBox="`0 0 ${width} ${H}`"
    :width="width"
    :height="H"
    class="h-6 shrink-0 overflow-visible max-md:h-5"
    :class="props.tone === 'nos' ? 'stroke-(--tento-nos)' : 'stroke-(--tento-eles)'"
    fill="none"
    stroke-linecap="round"
    aria-hidden="true"
    :style="{ strokeWidth: 'var(--tento-traco)', width: `${width}px` }"
  >
    <template
      v-for="g in grupos"
      :key="g[0]?.index"
    >
      <path
        v-for="r in g"
        :key="`${r.index}-${novo(r.index) ? novos?.stamp : 0}`"
        :d="r.d"
        pathLength="1"
        :class="novo(r.index) ? 'mesa-riscar mesa-riscar--tento' : ''"
        :style="estilo(r.index)"
      />
    </template>
  </svg>
</template>
