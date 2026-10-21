import { defineStore } from 'pinia'
import type { Seat } from '@/protocol/engine'
import type { InviteExpiredPayload, InviteReceivedPayload, RoomInvitesPayload, RoomInvite } from '@/protocol/messages'
import { send } from '@/app/transport'
import { useRoomStore } from './room'

/**
 * Resultado do aceite por token (link do e-mail). `pending` até o servidor decidir. Todo aceite termina em room_state
 * (`accepted`) ou em erro com seq (`expired`, `not_yours`, `playing`); aceite duplicado ignorado pelo servidor não responde,
 * porque o primeiro ainda vai resolver. `accepted` só vale para room_state de uma sala diferente da que o usuário ocupava ao
 * enviar; o caso raro "já estou sentado na sala do convite" fica `pending` e a página trata com fallback (olhar `room`).
 */
export type TokenOutcome = 'pending' | 'accepted' | 'expired' | 'not_yours' | 'playing'

const MAX_VISIBLE = 3

const TOKEN_ERRORS: Record<string, 'expired' | 'not_yours' | 'playing'> = {
  invite_expired: 'expired',
  invite_not_found: 'expired',
  invite_not_yours: 'not_yours',
  invitee_playing: 'playing',
}

function errorCode(err: unknown): string | null {
  if (typeof err === 'object' && err !== null && typeof (err as { code?: unknown }).code === 'string') {
    return (err as { code: string }).code
  }
  return null
}

export const useInvitesStore = defineStore('invites', {
  state: () => ({
    /** Avisos de convite recebidos, do mais antigo ao mais novo. */
    received: [] as InviteReceivedPayload[],
    /** Lista "Convites" da sala em que estou. */
    roomInvites: [] as RoomInvite[],
    tokenOutcome: null as TokenOutcome | null,
    /** Último aceite em andamento: se o servidor recusar por partida em andamento, o aviso volta. */
    accepting: null as InviteReceivedPayload | null,
    /** Sala em que o usuário estava ao enviar o aceite por token (null = fora de sala). */
    tokenFromRoom: null as string | null,
    tokenVisit: 0,
  }),
  getters: {
    /** Até 3 avisos, o mais novo em cima. */
    visible: (s): InviteReceivedPayload[] => s.received.slice(-MAX_VISIBLE).reverse(),
  },
  actions: {
    onReceived(p: InviteReceivedPayload) {
      const i = this.received.findIndex((r) => r.inviteId === p.inviteId)
      if (i >= 0) this.received[i] = p // reenvio na reconexão: um aviso só
      else this.received.push(p)
    },
    onExpired(p: InviteExpiredPayload) {
      this.received = this.received.filter((r) => r.inviteId !== p.inviteId)
      if (this.accepting?.inviteId === p.inviteId) this.accepting = null
    },
    onRoomInvites(p: RoomInvitesPayload) {
      this.roomInvites = p.invites
    },
    clearRoom() {
      this.roomInvites = []
    },
    /** Chegou room_state: só uma sala diferente da que eu ocupava ao enviar o aceite conta como entrada pelo convite. */
    onRoomState(roomId: string) {
      if (this.tokenOutcome === 'pending' && roomId !== this.tokenFromRoom) this.tokenOutcome = 'accepted'
    },
    resetToken() {
      this.tokenVisit++ // aceite em voo de uma visita anterior passa a ser ignorado
      this.tokenOutcome = null
    },
    /** Convida por username. `already_in_room` aqui quer dizer "essa pessoa já está nesta sala", não "você". */
    async invite(username: string, seat?: Seat): Promise<void> {
      try {
        await send({ type: 'invite', payload: seat === undefined ? { username } : { username, seat } })
      } catch (err) {
        if (errorCode(err) === 'already_in_room') {
          throw Object.assign(new Error('Essa pessoa já está nesta sala.'), { code: 'invitee_already_here' })
        }
        throw err
      }
    },
    /** Retira o aviso na hora e envia a resposta; se o aceite for recusado por partida em andamento, o aviso volta. */
    async respond(inviteId: string, accept: boolean): Promise<void> {
      const notice = this.received.find((r) => r.inviteId === inviteId)
      this.received = this.received.filter((r) => r.inviteId !== inviteId)
      this.accepting = accept && notice ? notice : null
      try {
        await send({ type: 'invite_respond', payload: { inviteId, accept } })
      } catch (err) {
        if (errorCode(err) === 'invitee_playing') this.restoreAccepting()
        throw err
      }
    },
    async acceptToken(token: string): Promise<void> {
      this.tokenOutcome = 'pending'
      this.tokenFromRoom = useRoomStore().room?.id ?? null
      const visit = this.tokenVisit
      try {
        await send({ type: 'invite_accept_token', payload: { token } })
      } catch (err) {
        if (visit !== this.tokenVisit) return // a visita acabou (resetToken): o resultado não é mais desta página
        const outcome = TOKEN_ERRORS[errorCode(err) ?? '']
        if (outcome) {
          this.tokenOutcome = outcome
          return
        }
        this.tokenOutcome = null
        throw err
      }
    },
    /**
     * Erro do servidor que não casou com um envio pendente (broadcasts resolvem o envio antes de o erro chegar).
     * Devolve true quando o erro foi consumido aqui (aceite por token: a página mostra o resultado, sem toast).
     */
    onServerError(code: string): boolean {
      const outcome = TOKEN_ERRORS[code]
      if (outcome && this.tokenOutcome === 'pending') {
        this.tokenOutcome = outcome
        return true
      }
      if (code === 'invitee_playing') this.restoreAccepting()
      return false
    },
    restoreAccepting() {
      const notice = this.accepting
      this.accepting = null
      if (notice && !this.received.some((r) => r.inviteId === notice.inviteId)) this.received.push(notice)
    },
  },
})
