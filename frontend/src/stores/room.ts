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
      this.room = p
      this.lastLeft = null
      if (wasPlaying && p.status === 'waiting') {
        this.forfeitTeam = null
        useGameStore().reset()
      }
    },
    onLeft(p: LeftRoomPayload) {
      this.room = null
      this.forfeitTeam = null
      this.ended = null
      this.lastLeft = p.reason
      useGameStore().reset()
    },
    /** Reconexão em que o servidor nos mandou ao lobby (lobby_snapshot): a sala e a partida antigas não existem mais. */
    onOutsideRoom() {
      if (this.room === null) return
      this.room = null
      this.forfeitTeam = null
      this.ended = null
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
