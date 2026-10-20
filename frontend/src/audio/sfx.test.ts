import { Sfx, soundForEvent, SOUND_IDS, type SoundId } from './sfx'

function fakeContext() {
  const started: number[] = []
  const gainNodes: { gain: { value: number }; connect: ReturnType<typeof vi.fn> }[] = []
  const ctx = {
    state: 'suspended' as AudioContextState,
    destination: {},
    resume: vi.fn(async () => {
      ctx.state = 'running'
    }),
    decodeAudioData: vi.fn(async () => ({ duration: 1 }) as unknown as AudioBuffer),
    createBufferSource: vi.fn(() => ({ buffer: null, connect: vi.fn(), start: () => started.push(1) })),
    createGain: vi.fn(() => {
      const g = { gain: { value: 1 }, connect: vi.fn() }
      gainNodes.push(g)
      return g
    }),
  }
  return { ctx, started, gainNodes }
}

const urls = Object.fromEntries(SOUND_IDS.map((id) => [id, `/${id}.ogg`])) as Record<SoundId, string>

it('mapeia eventos para sons; vitória/derrota saem do match_ended, não do game_over', () => {
  expect(soundForEvent({ kind: 'card_played', seat: 1, card: 3 }, 0)).toBe('card')
  expect(soundForEvent({ kind: 'card_played', seat: 1 }, 0)).toBe('cardCovered')
  expect(soundForEvent({ kind: 'raise_requested', seat: 1, stake: 3 }, 0)).toBe('truco')
  expect(soundForEvent({ kind: 'raise_resolved', choice: 'run', stake: 1 }, 0)).toBe('run')
  expect(soundForEvent({ kind: 'game_over', team: 0 }, 0)).toBeNull()
  expect(soundForEvent({ kind: 'game_over', team: 1 }, 0)).toBeNull()
  expect(soundForEvent({ kind: 'forfeited', team: 1 }, 0)).toBeNull()
  expect(soundForEvent({ kind: 'cut_offered', seat: 0 }, 0)).toBeNull()
})

it('não toca antes do desbloqueio nem com mudo; toca com o volume das preferências', async () => {
  const { ctx, started, gainNodes } = fakeContext()
  let muted = false
  const sfx = new Sfx({
    createContext: () => ctx as never,
    fetchBuffer: async () => new ArrayBuffer(8),
    urls,
    volume: () => 0.4,
    muted: () => muted,
  })
  sfx.play('card')
  expect(started).toHaveLength(0)
  await sfx.unlock()
  sfx.play('card')
  expect(started).toHaveLength(1)
  expect(gainNodes.map((g) => g.gain.value)).toEqual([0.4])
  muted = true
  sfx.play('card')
  expect(started).toHaveLength(1)
})
