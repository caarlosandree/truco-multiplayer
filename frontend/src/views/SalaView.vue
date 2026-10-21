<script setup lang="ts">
import { watch } from 'vue'
import { useRouter } from 'vue-router'
import { useLobbyStore } from '@/stores/lobby'
import { useRoomStore } from '@/stores/room'
import { useSessionStore } from '@/stores/session'
import Mesa from '@/components/mesa/Mesa.vue'
import SalaEspera from '@/components/sala/SalaEspera.vue'

const router = useRouter()
const room = useRoomStore()
const session = useSessionStore()
const lobby = useLobbyStore()

// /sala aberta direto, sem sala: depois do hello o servidor manda o lobby_snapshot a quem NÃO está numa sala
// (quem está recebe room_state). Snapshot carregado + sem sala = sinal determinístico para voltar ao lobby.
watch(
  () => session.connection === 'open' && lobby.loaded && !room.inRoom,
  (semSala) => {
    if (semSala) void router.replace({ name: 'lobby' })
  },
  { immediate: true },
)
</script>

<template>
  <template v-if="room.room">
    <SalaEspera v-if="!room.isPlaying" />
    <Mesa v-else />
  </template>
  <p
    v-else
    class="p-8 text-ink-muted"
    role="status"
  >
    Carregando sala…
  </p>
</template>
