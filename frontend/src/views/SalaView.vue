<script setup lang="ts">
import { onBeforeUnmount, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useRoomStore } from '@/stores/room'
import { useSessionStore } from '@/stores/session'
import SalaEspera from '@/components/sala/SalaEspera.vue'

const GRACE_MS = 1000

const router = useRouter()
const room = useRoomStore()
const session = useSessionStore()
let timer: ReturnType<typeof setTimeout> | null = null

function clear() {
  if (timer !== null) clearTimeout(timer)
  timer = null
}

// /sala aberta direto, sem sala: só depois do hello (connection 'open') e de uma folga para o room_state chegar
watch(
  () => [session.connection, room.inRoom] as const,
  ([connection, inRoom]) => {
    clear()
    if (connection === 'open' && !inRoom) {
      timer = setTimeout(() => void router.replace({ name: 'lobby' }), GRACE_MS)
    }
  },
  { immediate: true },
)
onBeforeUnmount(clear)
</script>

<template>
  <template v-if="room.room">
    <SalaEspera v-if="!room.isPlaying" />
    <p
      v-else
      class="p-8"
      role="status"
    >
      Partida em andamento…
    </p>
  </template>
  <p
    v-else
    class="p-8 text-ink-muted"
    role="status"
  >
    Carregando sala…
  </p>
</template>
