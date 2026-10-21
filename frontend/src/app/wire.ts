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
import { usePresenceStore } from '@/stores/presence'
import { useInvitesStore } from '@/stores/invites'
import { useToastsStore } from '@/stores/toasts'

let client: WsClient | null = null

export function dispatch(msg: ServerMessage): void {
  switch (msg.type) {
    case 'hello':
      return // tratado no WsClient (status e versão do protocolo)
    case 'lobby_snapshot':
      // o servidor só manda o snapshot a quem está fora de sala (quem está recebe room_state): sala antiga = fantasma
      useRoomStore().onOutsideRoom()
      useInvitesStore().clearRoom()
      return useLobbyStore().onSnapshot(msg.payload)
    case 'room_upserted':
      return useLobbyStore().onUpsert(msg.payload)
    case 'room_removed':
      return useLobbyStore().onRemove(msg.payload)
    case 'room_state':
      useInvitesStore().onRoomState(msg.payload.id)
      return useRoomStore().onRoomState(msg.payload)
    case 'left_room':
      useInvitesStore().clearRoom()
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
    case 'presence_snapshot':
      return usePresenceStore().onSnapshot(msg.payload)
    case 'presence_updated':
      return usePresenceStore().onUpdated(msg.payload)
    case 'presence_removed':
      return usePresenceStore().onRemoved(msg.payload)
    case 'invite_received':
      return useInvitesStore().onReceived(msg.payload)
    case 'invite_expired':
      return useInvitesStore().onExpired(msg.payload)
    case 'room_invites':
      return useInvitesStore().onRoomInvites(msg.payload)
    case 'error':
      return // tratado no WsClient (rejeição do send ou onUnackedError)
    default: {
      const unreachable: never = msg
      console.warn('[ws] tipo desconhecido', unreachable)
    }
  }
}

/** Erro sem envio pendente para casar (um broadcast resolveu o envio antes): convites alimentam a store, o resto vira toast. */
export function handleUnackedError(code: string, message: string): void {
  if (useInvitesStore().onServerError(code)) return
  useToastsStore().error(code, message)
}

function start(): void {
  if (client) return
  const session = useSessionStore()
  const c = new WsClient({
    url: wsUrl(location),
    onMessage: dispatch,
    onStatus: (s) => {
      if (s === 'open') useLobbyStore().markStale() // hello recebido; o snapshot desta conexão vem depois
      session.setConnection(s)
    },
    onVersionMismatch: () => {
      session.versionMismatch = true
    },
    onUnackedError: handleUnackedError,
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
  usePresenceStore().$reset()
  useInvitesStore().$reset()
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
