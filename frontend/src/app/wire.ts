import { watch } from 'vue'
import type { Router } from 'vue-router'
import type { ServerMessage } from '@/protocol/union'
import { WsClient, wsUrl } from '@/ws/client'
import { authApi } from '@/api/auth'
import { ApiError } from '@/api/http'
import { setSender } from './transport'
import { setResumeHandler, useSessionStore } from '@/stores/session'
import { useLobbyStore } from '@/stores/lobby'
import { useRoomStore } from '@/stores/room'
import { useGameStore } from '@/stores/game'
import { useToastsStore } from '@/stores/toasts'

let client: WsClient | null = null

export function dispatch(msg: ServerMessage): void {
  switch (msg.type) {
    case 'hello':
      return // tratado no WsClient (status e versão do protocolo)
    case 'lobby_snapshot':
      return useLobbyStore().onSnapshot(msg.payload)
    case 'room_upserted':
      return useLobbyStore().onUpsert(msg.payload)
    case 'room_removed':
      return useLobbyStore().onRemove(msg.payload)
    case 'room_state':
      return useRoomStore().onRoomState(msg.payload)
    case 'left_room':
      return useRoomStore().onLeft(msg.payload)
    case 'forfeit_available':
      return useRoomStore().onForfeit(msg.payload)
    case 'match_ended':
      return useRoomStore().onMatchEnded(msg.payload)
    case 'state':
      return useGameStore().onState(msg.payload)
    case 'events':
      return useGameStore().onEvents(msg.payload)
    case 'signal':
      return useGameStore().onSignal(msg.payload)
    case 'error':
      return // tratado no WsClient (rejeição do send ou onUnackedError)
    default: {
      const unreachable: never = msg
      console.warn('[ws] tipo desconhecido', unreachable)
    }
  }
}

function start(): void {
  if (client) return
  const session = useSessionStore()
  const toasts = useToastsStore()
  const c = new WsClient({
    url: wsUrl(location),
    onMessage: dispatch,
    onStatus: (s) => session.setConnection(s),
    onVersionMismatch: () => {
      session.versionMismatch = true
    },
    onUnackedError: (code, message) => toasts.error(code, message),
    checkSession: async () => {
      try {
        await authApi.me()
        return true
      } catch (err) {
        // só 401 invalida a sessão; rede/outros erros seguem reconectando
        return !(err instanceof ApiError && err.status === 401)
      }
    },
  })
  client = c
  setSender((m) => c.send(m))
  c.connect()
}

function stop(): void {
  client?.disconnect()
  client = null
  setSender(null)
  useLobbyStore().$reset()
  useRoomStore().$reset()
  useGameStore().reset()
}

export function resumeConnection(): void {
  client?.resume()
}

/** Liga a conexão ao login: abre com usuário, fecha sem; sessão inválida volta ao login. */
export function installConnection(router: Router): void {
  const session = useSessionStore()
  setResumeHandler(resumeConnection)
  watch(
    () => session.user?.id ?? null,
    (id) => (id ? start() : stop()),
    { immediate: true },
  )
  watch(
    () => session.connection,
    (status) => {
      if (status !== 'unauthorized') return
      session.clear()
      const atual = router.currentRoute.value
      // já em /entrar: não há para onde voltar (evita volta=/entrar em laço)
      if (atual.name === 'entrar') return
      void router.push({ name: 'entrar', query: { volta: atual.fullPath } })
    },
  )
}
