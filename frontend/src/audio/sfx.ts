import type { Event } from '@/protocol/engine'

export type SoundId =
  | 'shuffle' | 'deal' | 'card' | 'cardCovered' | 'burn' | 'flip' | 'trickWon'
  | 'truco' | 'accept' | 'run' | 'point' | 'yourTurn' | 'timeLow' | 'victory' | 'defeat'

export const SOUND_IDS: readonly SoundId[] = [
  'shuffle', 'deal', 'card', 'cardCovered', 'burn', 'flip', 'trickWon',
  'truco', 'accept', 'run', 'point', 'yourTurn', 'timeLow', 'victory', 'defeat',
]

export function soundForEvent(ev: Event, _myTeam: number | null): SoundId | null {
  switch (ev.kind) {
    case 'hand_started':
      return 'shuffle'
    case 'cards_dealt':
      return 'deal'
    case 'fours_burned':
      return 'burn'
    case 'vira_revealed':
      return 'flip'
    case 'card_played':
      return ev.card === undefined ? 'cardCovered' : 'card'
    case 'round_ended':
      return ev.team === undefined ? null : 'trickWon'
    case 'raise_requested':
      return 'truco'
    case 'raise_resolved':
      return ev.choice === 'run' ? 'run' : 'accept'
    case 'raise_punished':
      return 'defeat'
    case 'hand_ended':
      return 'point'
    // game_over e forfeited não tocam aqui: a vitória/derrota sai do match_ended (room store)
    default:
      return null
  }
}

export interface AudioContextLike {
  state: AudioContextState
  destination: AudioNode | object
  resume(): Promise<void>
  decodeAudioData(data: ArrayBuffer): Promise<AudioBuffer>
  createBufferSource(): { buffer: AudioBuffer | null; connect(n: unknown): unknown; start(): void }
  createGain(): { gain: { value: number }; connect(n: unknown): unknown }
}

export interface SfxOptions {
  createContext(): AudioContextLike
  fetchBuffer(url: string): Promise<ArrayBuffer>
  urls: Record<SoundId, string>
  volume(): number
  muted(): boolean
}

export class Sfx {
  private ctx: AudioContextLike | null = null
  private buffers = new Map<SoundId, AudioBuffer>()

  constructor(private readonly o: SfxOptions) {}

  /** Chamado no primeiro gesto do usuário (política de autoplay dos navegadores). */
  async unlock(): Promise<void> {
    const ctx = this.ctx ?? this.o.createContext()
    this.ctx = ctx
    if (ctx.state !== 'running') await ctx.resume()
    await Promise.all(
      SOUND_IDS.map(async (id) => {
        if (this.buffers.has(id)) return
        try {
          const data = await this.o.fetchBuffer(this.o.urls[id])
          this.buffers.set(id, await ctx.decodeAudioData(data))
        } catch {
          // som ausente não quebra o jogo
        }
      }),
    )
  }

  play(id: SoundId): void {
    const ctx = this.ctx
    const buffer = this.buffers.get(id)
    if (!ctx || ctx.state !== 'running' || !buffer || this.o.muted() || this.o.volume() <= 0) return
    const source = ctx.createBufferSource()
    const gain = ctx.createGain()
    gain.gain.value = this.o.volume()
    source.buffer = buffer
    source.connect(gain)
    gain.connect(ctx.destination)
    source.start()
  }
}
