<script setup lang="ts">
import { computed } from 'vue'
import type { CardID } from '@/protocol/engine'
import { usePrefsStore } from '@/stores/prefs'
import { backUrl, cardUrl } from '@/game/cardAssets'
import { cardLabel } from '@/game/cards'

const props = withDefaults(
  defineProps<{
    card?: CardID
    covered?: boolean
    hidden?: boolean
    /** xs = mini-verso dos adversários, sm = vira, md = vaza, lg = minha mão */
    size?: 'xs' | 'sm' | 'md' | 'lg'
    interactive?: boolean
    disabled?: boolean
    /** só enfeite: o contêiner pai descreve o conjunto */
    decorative?: boolean
    label?: string
  }>(),
  { card: undefined, label: undefined, size: 'md', covered: false, hidden: false, interactive: false, disabled: false, decorative: false },
)
const emit = defineEmits<{ select: [] }>()
const prefs = usePrefsStore()

const faceDown = computed(() => props.covered || props.hidden || props.card === undefined)
const src = computed(() => (props.card === undefined || faceDown.value ? backUrl(prefs.deck) : cardUrl(props.card, prefs.deck)))
const alt = computed(() => {
  if (props.label) return props.label
  if (props.covered) return 'Carta coberta'
  if (props.card === undefined || faceDown.value) return 'Carta virada'
  return cardLabel(props.card, prefs.deck)
})

// larguras vêm dos tokens do brief; o celular reduz a vaza (52 px) e aumenta a mão (88 px, alvo >= 72x108)
const SIZES = {
  xs: 'w-8 max-md:w-6',
  sm: 'w-(--carta-largura-vira)',
  md: 'w-(--carta-largura-mesa) max-md:w-13',
  lg: 'w-(--carta-largura-mao) max-md:w-22',
} as const
</script>

<template>
  <button
    v-if="props.interactive"
    type="button"
    :disabled="props.disabled"
    :aria-label="alt"
    class="group block shrink-0 rounded-card transition-[translate,box-shadow] duration-(--dur-rapido) ease-desliza enabled:hover:translate-y-(--carta-ergue-y) enabled:hover:shadow-carta-erguida enabled:focus-visible:translate-y-(--carta-ergue-y) enabled:focus-visible:shadow-carta-erguida enabled:active:translate-y-(--carta-ergue-y) disabled:cursor-default"
    :class="SIZES[props.size]"
    @click="emit('select')"
  >
    <img
      :src="src"
      alt=""
      class="aspect-[167/243] w-full rounded-card shadow-card group-enabled:group-hover:shadow-none"
      draggable="false"
    >
  </button>
  <img
    v-else
    :src="src"
    :alt="props.decorative ? '' : alt"
    :aria-hidden="props.decorative ? 'true' : undefined"
    class="aspect-[167/243] shrink-0 rounded-card shadow-card"
    :class="SIZES[props.size]"
    draggable="false"
  >
</template>
