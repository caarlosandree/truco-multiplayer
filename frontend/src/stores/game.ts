import { defineStore } from 'pinia'
import type { Event, PlayerView, Seat, Team, VoteChoice } from '@/protocol/engine'
import type { SignalBroadcastPayload, SignalID, StatePayload } from '@/protocol/messages'
import type { Outgoing } from '@/ws/client'
import { send } from '@/app/transport'
import { Director } from '@/game/director'
import { nextStake } from '@/game/labels'
import { cardFromId } from '@/game/cards'
import { teamOf } from '@/game/seats'
import { useToastsStore } from './toasts'
import { useSessionStore } from './session'

export interface Scene {
  event: Event | null
  key: number
  durationMs: number
}

/** Resultado de uma ação enviada: `code` é o código do erro do servidor (ex.: covered_not_allowed). */
export interface ActResult {
  ok: boolean
  code?: string
}

export interface ActiveSignal {
  id: SignalID
  until: number
}

const SIGNAL_SHOW_MS = 3000
const SIGNAL_COOLDOWN_MS = 3000

let soundPlayer: (ev: Event) => void = () => {}
/** Injetado no main.ts para a store não importar Web Audio. */
export function setGameSoundPlayer(fn: (ev: Event) => void): void {
  soundPlayer = fn
}

// Um diretor por instância da store (chave: o objeto $state, estável e único por Pinia); fora do state, não reativo.
const directors = new WeakMap<object, Director>()

function prefersReducedMotion(): boolean {
  return typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches
}

export const useGameStore = defineStore('game', {
  state: () => ({
    view: null as PlayerView | null,
    latest: null as StatePayload | null,
    deadlines: {} as Record<number, number>,
    pendingSeats: [] as Seat[],
    scene: { event: null, key: 0, durationMs: 0 } as Scene,
    signals: {} as Record<number, ActiveSignal>,
    inflight: false,
    signalCooldownUntil: 0,
  }),
  getters: {
    latestView: (s): PlayerView | null => s.latest?.view ?? null,
    mySeat(): Seat | null {
      return this.latestView?.seat ?? null
    },
    myTeam(): Team | null {
      return this.mySeat === null ? null : teamOf(this.mySeat)
    },
    phase(): PlayerView['phase'] | null {
      return this.latestView?.phase ?? null
    },
    /** Ações de jogo só valem com a conexão aberta (spec §5). */
    online(): boolean {
      return useSessionStore().connection === 'open'
    },
    isMyTurn(): boolean {
      const v = this.latestView
      return v !== null && v.turn !== undefined && v.turn === v.seat
    },
    amPending(): boolean {
      return this.mySeat !== null && (this.latest?.pendingSeats ?? []).includes(this.mySeat)
    },
    canPlay(): boolean {
      return this.phase === 'playing' && this.isMyTurn
    },
    canCover(): boolean {
      return this.canPlay && this.latestView?.handKind !== 'iron'
    },
    nextStakeValue(): number | null {
      return this.latestView ? nextStake(this.latestView.stake) : null
    },
    canRaise(): boolean {
      const v = this.latestView
      if (!v || !this.canPlay || v.pending) return false
      if (v.lastRaiser !== undefined && v.lastRaiser === this.myTeam) return false
      return this.nextStakeValue !== null
    },
    raiseIsSuicide(): boolean {
      return this.latestView?.handKind === 'eleven' || this.latestView?.handKind === 'iron'
    },
    mustRespondRaise(): boolean {
      const v = this.latestView
      return !!v && v.phase === 'awaiting_raise_response' && v.votingTeam === this.myTeam && this.amPending
    },
    canRaiseBack(): boolean {
      const to = this.latestView?.pending?.toStake
      return this.mustRespondRaise && to !== undefined && nextStake(to) !== null
    },
    mustDecideEleven(): boolean {
      return this.phase === 'eleven_decision' && this.amPending
    },
    mustBurn(): boolean {
      return this.phase === 'burn_fours' && this.amPending
    },
    mustChooseCut(): boolean {
      return (this.phase === 'cut_choice' || this.phase === 'dealer_choice') && this.amPending && (this.latestView?.cutOffer?.length ?? 0) > 0
    },
    fourSlots(): number[] {
      return (this.latestView?.myHand ?? []).filter((h) => h.card !== undefined && cardFromId(h.card).rank === 4).map((h) => h.slot)
    },
  },
  actions: {
    ensureDirector(): Director {
      const existing = directors.get(this.$state)
      if (existing) return existing
      const d = new Director({
        onEvent: (ev, ms) => {
          this.scene = { event: ev, key: this.scene.key + 1, durationMs: ms }
          soundPlayer(ev)
        },
        onApply: (s) => {
          this.view = s.view
          this.deadlines = s.deadlines as Record<number, number>
          this.pendingSeats = s.pendingSeats
        },
        onIdle: () => {
          this.scene = { event: null, key: this.scene.key + 1, durationMs: 0 }
        },
        reducedMotion: prefersReducedMotion,
        hidden: () => typeof document !== 'undefined' && document.hidden,
        setTimer: (fn, ms) => setTimeout(fn, ms),
        clearTimer: (h) => clearTimeout(h as ReturnType<typeof setTimeout>),
      })
      directors.set(this.$state, d)
      return d
    },
    onState(p: StatePayload) {
      this.latest = p
      this.ensureDirector().pushState(p)
    },
    onEvents(evs: Event[]) {
      this.ensureDirector().pushEvents(evs)
    },
    onSignal(p: SignalBroadcastPayload) {
      const until = Date.now() + SIGNAL_SHOW_MS
      this.signals = { ...this.signals, [p.seat]: { id: p.id, until } }
      setTimeout(() => {
        if (this.signals[p.seat]?.until === until) {
          this.signals = Object.fromEntries(Object.entries(this.signals).filter(([k]) => k !== String(p.seat)))
        }
      }, SIGNAL_SHOW_MS)
    },
    reset() {
      directors.get(this.$state)?.reset()
      directors.delete(this.$state)
      this.$reset()
    },
    async act(msg: Outgoing): Promise<ActResult> {
      if (this.inflight) return { ok: false }
      this.inflight = true
      try {
        await send(msg)
        return { ok: true }
      } catch (err) {
        useToastsStore().fromError(err)
        const code = (err as { code?: unknown } | null)?.code
        return typeof code === 'string' ? { ok: false, code } : { ok: false }
      } finally {
        this.inflight = false
      }
    },
    chooseCard(keep: number) {
      return this.act({ type: 'choose_card', payload: { keep } })
    },
    burn(slots: number[]) {
      return this.act({ type: 'burn', payload: { slots } })
    },
    passBurn() {
      return this.act({ type: 'pass_burn' })
    },
    playCard(slot: number, covered: boolean) {
      return this.act({ type: 'play_card', payload: { slot, covered } })
    },
    requestRaise() {
      return this.act({ type: 'request_raise' })
    },
    vote(choice: VoteChoice) {
      return this.act({ type: 'vote', payload: { choice } })
    },
    async sendSignal(id: SignalID) {
      const now = Date.now()
      if (now < this.signalCooldownUntil) return
      this.signalCooldownUntil = now + SIGNAL_COOLDOWN_MS
      try {
        await send({ type: 'signal', payload: { id } })
      } catch (err) {
        useToastsStore().fromError(err)
      }
    },
  },
})
