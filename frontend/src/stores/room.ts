import { defineStore } from 'pinia'
import type { Seat, Team } from '@/protocol/engine'
import type {
  ForfeitAvailablePayload,
  LeftReason,
  LeftRoomPayload,
  MatchEndedPayload,
  RoomStatePayload,
} from '@/protocol/messages'
import { send } from '@/app/transport'
import { teamOf } from '@/game/seats'
import { useGameStore } from './game'
import { useToastsStore } from './toasts'

export type MatchSound = 'victory' | 'defeat'

let matchSoundPlayer: (id: MatchSound) => void = () => {}
/** Injetado no main.ts (via audio/install) para a store não importar Web Audio. */
export function setMatchSoundPlayer(fn: (id: MatchSound) => void): void {
  matchSoundPlayer = fn
}

export const useRoomStore = defineStore('room', {
  state: () => ({
    room: null as RoomStatePayload | null,
    forfeitTeam: null as Team | null,
    ended: null as MatchEndedPayload | null,
    lastLeft: null as LeftReason | null,
    /** a partida em curso começou com bot sentado (decide o texto do aviso quando um bot assume o lugar de alguém) */
    startedWithBots: false,
    /** lote mais recente de bots que assumiram o lugar de humanos (um room_state pode trazer vários); `key` muda a cada lote */
    takeover: null as { key: number; assentos: { seat: Seat; nome: string }[] } | null,
    /** assentos já avisados nesta partida: o aviso vale uma vez por assento por partida */
    avisados: [] as Seat[],
  }),
  getters: {
    inRoom: (s) => s.room !== null,
    mySeat: (s): Seat | null => s.room?.yourSeat ?? null,
    myTeam(): Team | null {
      return this.mySeat === null ? null : teamOf(this.mySeat)
    },
    isHost: (s) => s.room !== null && s.room.host === s.room.yourSeat,
    isPlaying: (s) => s.room?.status === 'playing',
    /** Lugares vazios da sala em que estou. */
    freeSeats: (s): Seat[] => (s.room ? s.room.seats.flatMap((x, i) => (x === null ? [i as Seat] : [])) : []),
    /** Posso convidar alguém: em sala de espera com lugar livre. */
    canInvite(): boolean {
      return this.room?.status === 'waiting' && this.freeSeats.length > 0
    },
    amReady: (s) => (s.room ? (s.room.seats[s.room.yourSeat]?.ready ?? false) : false),
  },
  actions: {
    onRoomState(p: RoomStatePayload) {
      const wasPlaying = this.room?.status === 'playing'
      // outra sala ou outra partida: o fim de partida anterior não volta a aparecer
      if (this.room?.id !== p.id || (!wasPlaying && p.status === 'playing')) this.ended = null
      this.trackBots(p, wasPlaying)
      this.room = p
      this.lastLeft = null
      if (wasPlaying && p.status === 'waiting') {
        this.forfeitTeam = null
        useGameStore().reset()
      }
    },
    /** Começou com bot? E algum humano acabou de ser substituído por um? (só transições observadas, não o estado inicial) */
    trackBots(p: RoomStatePayload, wasPlaying: boolean) {
      if (p.status !== 'playing') {
        this.startedWithBots = false
        this.avisados = []
        this.takeover = null
        return
      }
      const anterior = this.room
      const mesmaSala = anterior !== null && anterior.id === p.id
      if (!wasPlaying || !mesmaSala) {
        this.startedWithBots = p.seats.some((x) => x?.bot === true)
        this.avisados = []
      }
      if (!mesmaSala || !wasPlaying) return
      const novos: { seat: Seat; nome: string }[] = []
      p.seats.forEach((x, i) => {
        if (!x?.botPlaying || anterior.seats[i]?.botPlaying || this.avisados.includes(i as Seat)) return
        novos.push({ seat: i as Seat, nome: x.displayName })
      })
      if (novos.length === 0) return
      this.avisados = [...this.avisados, ...novos.map((n) => n.seat)]
      this.takeover = { key: (this.takeover?.key ?? 0) + 1, assentos: novos }
      for (const { nome } of novos) {
        useToastsStore().push(
          this.startedWithBots
            ? `${nome} saiu; um bot assumiu.`
            : `${nome} saiu; um bot assumiu. Se ${nome} não voltar até o fim, a partida não conta no ranking — vocês ainda podem pedir W.O.`,
        )
      }
    },
    onLeft(p: LeftRoomPayload) {
      this.room = null
      this.forfeitTeam = null
      this.ended = null
      this.startedWithBots = false
      this.avisados = []
      this.takeover = null
      this.lastLeft = p.reason
      useGameStore().reset()
    },
    /** Reconexão em que o servidor nos mandou ao lobby (lobby_snapshot): a sala e a partida antigas não existem mais. */
    onOutsideRoom() {
      if (this.room === null) return
      this.room = null
      this.forfeitTeam = null
      this.ended = null
      this.startedWithBots = false
      this.avisados = []
      this.takeover = null
      this.lastLeft = null
      useGameStore().reset()
    },
    onForfeit(p: ForfeitAvailablePayload) {
      this.forfeitTeam = p.team
    },
    onMatchEnded(p: MatchEndedPayload) {
      this.ended = p
      this.forfeitTeam = null
      const me = this.myTeam
      if (p.winner !== null && me !== null) matchSoundPlayer(p.winner === me ? 'victory' : 'defeat')
    },
    dismissEnded() {
      this.ended = null
    },
    takeSeat(seat: Seat) {
      return send({ type: 'take_seat', payload: { seat } })
    },
    setReady(ready: boolean) {
      return send({ type: 'set_ready', payload: { ready } })
    },
    startWithBots() {
      return send({ type: 'start_with_bots' })
    },
    kick(seat: Seat) {
      return send({ type: 'kick', payload: { seat } })
    },
    leave() {
      return send({ type: 'leave_room' })
    },
    claimForfeit() {
      return send({ type: 'claim_forfeit' })
    },
  },
})
