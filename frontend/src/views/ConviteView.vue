<script setup lang="ts">
import { onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useLobbyStore } from '@/stores/lobby'
import { useRoomStore } from '@/stores/room'
import { useSessionStore } from '@/stores/session'
import { useToastsStore } from '@/stores/toasts'

const route = useRoute()
const router = useRouter()
const lobby = useLobbyStore()
const room = useRoomStore()
const session = useSessionStore()
const toasts = useToastsStore()

async function join() {
  if (room.inRoom) {
    toasts.push('Você já está em outra sala.')
    void router.replace({ name: 'sala' }) // já em sala: o convite é ignorado, com aviso
    return
  }
  try {
    await lobby.joinByCode(String(route.params.codigo))
  } catch (err) {
    toasts.fromError(err)
    void router.replace({ name: 'lobby' })
  }
}

onMounted(() => {
  if (session.connection === 'open') {
    void join()
    return
  }
  const stop = watch(
    () => session.connection,
    (s) => {
      if (s === 'open') {
        stop()
        void join()
      }
    },
  )
})
</script>

<template>
  <p
    class="p-8 text-center text-ink-muted"
    role="status"
  >
    Entrando na sala {{ route.params.codigo }}…
  </p>
</template>
