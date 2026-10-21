<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import type { Seat } from '@/protocol/engine'
import { anuncioDoEvento } from '@/game/cena'
import { signalText } from '@/game/labels'
import { useGameStore } from '@/stores/game'
import { useRoomStore } from '@/stores/room'

/**
 * Dono dos anúncios da mesa para leitores de tela (brief §6.3): "Sua vez", pedidos e respostas de aumento,
 * fim de rodada e de mão, sinais dos outros jogadores e W.O. disponível. Uma única região viva, que só ACRESCENTA
 * itens (aria-relevant="additions"): o mesmo texto duas vezes seguidas é lido duas vezes, e nada é reescrito.
 * O painel de ação (PainelAcao) fala só do estado do próprio painel (como agir, "Reconectando…").
 */
const game = useGameStore()
const room = useRoomStore()

const KEEP = 4
const TTL_MS = 8000
const itens = ref<{ id: number; texto: string }[]>([])
let proximo = 0
const timers = new Set<ReturnType<typeof setTimeout>>()

function dizer(texto: string) {
  const id = ++proximo
  itens.value = [...itens.value.slice(-(KEEP - 1)), { id, texto }]
  const t = setTimeout(() => {
    timers.delete(t)
    itens.value = itens.value.filter((i) => i.id !== id)
  }, TTL_MS)
  timers.add(t)
}

const nome = (seat: Seat) => game.view?.players.find((p) => p.seat === seat)?.name ?? `Lugar ${seat + 1}`

watch(
  () => game.scene.key,
  () => {
    const e = game.scene.event
    const texto = e ? anuncioDoEvento(e, nome, game.myTeam) : null
    if (texto) dizer(texto)
  },
)
watch(
  () => game.euAjo,
  (vez) => {
    if (vez) dizer('Sua vez')
  },
  { immediate: true },
)
watch(
  () => game.signals,
  (agora, antes) => {
    for (const [k, s] of Object.entries(agora)) {
      const seat = Number(k)
      if (seat === game.mySeat || antes[seat]?.until === s.until) continue
      dizer(`${nome(seat)}: ${signalText(s.id)}`)
    }
  },
)
watch(
  () => room.forfeitTeam !== null && room.forfeitTeam !== room.myTeam,
  (disponivel) => {
    if (disponivel) dizer('Adversários ausentes. Você pode pedir W.O.')
  },
)

onBeforeUnmount(() => timers.forEach(clearTimeout))
</script>

<template>
  <div
    class="sr-only"
    role="log"
    aria-live="polite"
    aria-relevant="additions"
    aria-atomic="false"
    data-anunciador
  >
    <p
      v-for="i in itens"
      :key="i.id"
    >
      {{ i.texto }}
    </p>
  </div>
</template>
