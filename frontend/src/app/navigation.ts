import { watch } from 'vue'
import type { Router } from 'vue-router'
import { useRoomStore } from '@/stores/room'
import { useToastsStore } from '@/stores/toasts'

/** Sala manda na rota: room_state fora de /sala leva a /sala; sair (ou ser removido) volta ao lobby. */
export function installRoomNavigation(router: Router): void {
  const room = useRoomStore()
  // `room.room` é um objeto novo a cada room_state, então o watch dispara também em reconexão/outra aba.
  watch(
    () => room.room,
    (state) => {
      const name = router.currentRoute.value.name
      if (state) {
        if (name !== 'sala') void router.push({ name: 'sala' })
      } else if (name === 'sala' || name === 'convite') {
        void router.push({ name: 'lobby' })
      }
    },
  )
  // lastLeft é zerado por room_state e preenchido por left_room: cada saída dispara uma vez.
  watch(
    () => room.lastLeft,
    (reason) => {
      if (reason === 'kicked') useToastsStore().push('Você foi removido da sala.', 'error')
      if (reason === 'destroyed') useToastsStore().push('A sala foi encerrada.', 'info')
    },
  )
}
