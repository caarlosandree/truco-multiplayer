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

it('pedido feito antes do desbloqueio toca uma única vez ao liberar; cancelado não toca', async () => {
  const { ctx, started } = fakeContext()
  const sfx = new Sfx({
    createContext: () => ctx as never,
    fetchBuffer: async () => new ArrayBuffer(8),
    urls,
    volume: () => 1,
    muted: () => false,
  })
  sfx.tocarQuandoLiberado('yourTurn')
  sfx.tocarQuandoLiberado('yourTurn')
  expect(started).toHaveLength(0)
  await sfx.unlock()
  expect(started).toHaveLength(1)
  await sfx.unlock()
  expect(started).toHaveLength(1)

  const outro = new Sfx({
    createContext: () => ctx as never,
    fetchBuffer: async () => new ArrayBuffer(8),
    urls,
    volume: () => 1,
    muted: () => false,
  })
  outro.tocarQuandoLiberado('yourTurn')
  outro.cancelarPendente()
  await outro.unlock()
  expect(started).toHaveLength(1)
})

it('chamadas concorrentes de unlock compartilham uma promessa (sem baixar em dobro)', async () => {
  const { ctx } = fakeContext()
  const createContext = vi.fn(() => ctx as never)
  const fetchBuffer = vi.fn(async () => new ArrayBuffer(8))
  const sfx = new Sfx({ createContext, fetchBuffer, urls, volume: () => 1, muted: () => false })
  await Promise.all([sfx.unlock(), sfx.unlock()])
  expect(createContext).toHaveBeenCalledOnce()
  expect(fetchBuffer).toHaveBeenCalledTimes(SOUND_IDS.length)
  expect(sfx.running).toBe(true)
})

it('falha no resume libera nova tentativa no próximo gesto', async () => {
  const { ctx } = fakeContext()
  ctx.resume.mockRejectedValueOnce(new Error('bloqueado'))
  const sfx = new Sfx({
    createContext: () => ctx as never,
    fetchBuffer: async () => new ArrayBuffer(8),
    urls,
    volume: () => 1,
    muted: () => false,
  })
  await expect(sfx.unlock()).rejects.toThrow('bloqueado')
  expect(sfx.running).toBe(false)
  await sfx.unlock()
  expect(ctx.resume).toHaveBeenCalledTimes(2)
  expect(sfx.running).toBe(true)
})

it('não baixa sons com URL vazia', async () => {
  const { ctx } = fakeContext()
  const fetchBuffer = vi.fn(async () => new ArrayBuffer(8))
  const sfx = new Sfx({
    createContext: () => ctx as never,
    fetchBuffer,
    urls: { ...urls, burn: '' },
    volume: () => 1,
    muted: () => false,
  })
  await sfx.unlock()
  expect(fetchBuffer).toHaveBeenCalledTimes(SOUND_IDS.length - 1)
  expect(fetchBuffer).not.toHaveBeenCalledWith('')
})

it('o som de convite é um id conhecido', () => {
  expect(SOUND_IDS).toContain('invite')
})
