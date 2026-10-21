import { watch } from 'vue'
import type { Router } from 'vue-router'
import { useRoomStore } from '@/stores/room'
import { useSessionStore } from '@/stores/session'
import { useToastsStore } from '@/stores/toasts'

/**
 * Sala manda na rota, mas só nos momentos certos: entrar numa sala (null→sala), trocar de sala, ou o
 * primeiro room_state depois de cada (re)conexão (hello) levam a /sala. Atualizações de assento/pronto
 * da mesma sala não tiram o usuário de /perfil ou /. Sair (ou ser removido) volta ao lobby.
 */
export function installRoomNavigation(router: Router): void {
  const room = useRoomStore()
  const session = useSessionStore()
  let afterHello = false

  // status 'open' = hello recebido (o WsClient só abre depois dele); o próximo room_state é de (re)conexão
  watch(
    () => session.connection,
    (status) => {
      if (status === 'open') afterHello = true
    },
    { immediate: true },
  )

  watch(
    () => room.room,
    (state, prev) => {
      const name = router.currentRoute.value.name
      if (state) {
        const entering = prev === null || prev === undefined || prev.id !== state.id || afterHello
        afterHello = false
        if (entering && name !== 'sala') void router.push({ name: 'sala' })
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
