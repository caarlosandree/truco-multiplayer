import { createPinia, setActivePinia } from 'pinia'
import { installSfx, sfx } from './install'

function fakeAudioContext(resume: () => Promise<void>) {
  const ctx = {
    state: 'suspended' as AudioContextState,
    destination: {},
    resume: vi.fn(async () => {
      await resume()
      ctx.state = 'running'
    }),
    decodeAudioData: vi.fn(async () => ({ duration: 1 }) as unknown as AudioBuffer),
    createBufferSource: vi.fn(),
    createGain: vi.fn(),
  }
  return ctx
}

const gesture = (type: string) => window.dispatchEvent(new Event(type))

beforeEach(() => {
  setActivePinia(createPinia())
  vi.stubGlobal('fetch', vi.fn(async () => ({ ok: true, arrayBuffer: async () => new ArrayBuffer(8) })))
})

afterEach(() => vi.unstubAllGlobals())

it('tenta de novo após falha e para de ouvir gestos só quando o contexto roda', async () => {
  let attempt = 0
  const ctx = fakeAudioContext(async () => {
    attempt += 1
    if (attempt === 1) throw new Error('bloqueado')
  })
  vi.stubGlobal('AudioContext', function FakeAudioContext() { return ctx })
  installSfx()

  gesture('pointerdown')
  await vi.waitFor(() => expect(ctx.resume).toHaveBeenCalledTimes(1))
  // deixa a tentativa falha assentar antes do próximo gesto
  await new Promise((r) => setTimeout(r, 0))
  expect(sfx()?.running).toBe(false)

  gesture('touchend')
  await vi.waitFor(() => expect(sfx()?.running).toBe(true))
  expect(ctx.resume).toHaveBeenCalledTimes(2)

  gesture('click')
  gesture('keydown')
  await Promise.resolve()
  expect(ctx.resume).toHaveBeenCalledTimes(2)
})
