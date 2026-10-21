import type { Event } from '@/protocol/engine'
import type { StatePayload } from '@/protocol/messages'

// EventKind é `string` no protocolo gerado, então a tabela é tipada por literais
// (`as const satisfies`) e o acesso por tipo dinâmico passa por `DURATIONS` com `?? 0`.
export const EVENT_DURATION_MS = {
  hand_started: 600,
  cut_offered: 300,
  card_chosen: 300,
  cards_dealt: 700,
  fours_burned: 500,
  vira_revealed: 500,
  eleven_decided: 600,
  card_played: 350,
  round_ended: 700,
  raise_requested: 900,
  vote_cast: 400,
  raise_resolved: 700,
  raise_punished: 1200,
  hand_ended: 800,
  game_over: 0,
  forfeited: 0,
} as const satisfies Record<string, number>

const DURATIONS: Readonly<Record<string, number | undefined>> = EVENT_DURATION_MS

export const MAX_LAG_MS = 3000
const REDUCED_MS = 50

export interface DirectorOptions {
  onEvent(ev: Event, durationMs: number): void
  onApply(state: StatePayload): void
  onIdle?(): void
  reducedMotion(): boolean
  hidden(): boolean
  setTimer(fn: () => void, ms: number): unknown
  clearTimer(h: unknown): void
}

export class Director {
  private queue: Event[] = []
  private pending: StatePayload | null = null
  private timer: unknown = null
  private playing = false
  /** Eventos chegaram com a aba oculta ou atrasados demais: o próximo state é aplicado direto. */
  private skip = false

  constructor(private readonly o: DirectorOptions) {}

  get busy(): boolean {
    return this.playing || this.queue.length > 0
  }

  pushEvents(evs: Event[]): void {
    this.queue.push(...evs)
    if (this.o.hidden() || this.queuedMs() > MAX_LAG_MS) {
      this.skip = true
    }
    if (!this.playing) this.next()
  }

  pushState(s: StatePayload): void {
    this.pending = s
    if (this.skip) {
      this.flush()
      return
    }
    if (!this.busy) this.applyPending()
  }

  /** Descarta a fila e aplica o estado pendente imediatamente. */
  flush(): void {
    this.skip = false
    this.cancelTimer()
    this.queue = []
    this.playing = false
    this.applyPending()
    this.guard('onIdle', () => this.o.onIdle?.())
  }

  reset(): void {
    this.skip = false
    this.cancelTimer()
    this.queue = []
    this.pending = null
    this.playing = false
  }

  private next(): void {
    if (this.skip) {
      // aguardando o state que acompanha estes eventos para pular direto
      this.playing = false
      return
    }
    const ev = this.queue.shift()
    if (!ev) {
      this.playing = false
      this.applyPending()
      this.guard('onIdle', () => this.o.onIdle?.())
      return
    }
    this.playing = true
    const ms = this.duration(ev)
    this.guard('onEvent', () => this.o.onEvent(ev, ms))
    this.timer = this.o.setTimer(() => {
      this.timer = null
      this.next()
    }, ms)
  }

  private applyPending(): void {
    const s = this.pending
    if (!s) return
    this.pending = null
    this.guard('onApply', () => this.o.onApply(s))
  }

  /** Erro num callback (som, render) não pode travar a fila nem deixar de aplicar o state. */
  private guard(nome: string, fn: () => void): void {
    try {
      fn()
    } catch (e) {
      console.error(`[diretor] falha em ${nome}`, e)
    }
  }

  private duration(ev: Event): number {
    const base = DURATIONS[ev.kind] ?? 0
    return this.o.reducedMotion() ? Math.min(base, REDUCED_MS) : base
  }

  private queuedMs(): number {
    return this.queue.reduce((sum, ev) => sum + this.duration(ev), 0)
  }

  private cancelTimer(): void {
    if (this.timer !== null) this.o.clearTimer(this.timer)
    this.timer = null
  }
}
