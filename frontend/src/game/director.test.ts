import { Director, EVENT_DURATION_MS, type DirectorOptions } from './director'
import type { Event } from '@/protocol/engine'
import type { StatePayload } from '@/protocol/messages'

const st = (n: number) => ({ view: { handNumber: n }, deadlines: {}, pendingSeats: [] }) as unknown as StatePayload
const played = (seat: number): Event => ({ kind: 'card_played', seat, card: 5 })

function setup(over: Partial<DirectorOptions> = {}) {
  const log: string[] = []
  const d = new Director({
    onEvent: (ev, ms) => log.push(`event:${ev.kind}:${ev.seat ?? ''}:${ms}`),
    onApply: (s) => log.push(`apply:${s.view.handNumber}`),
    onIdle: () => log.push('idle'),
    reducedMotion: () => false,
    hidden: () => false,
    setTimer: (fn, ms) => setTimeout(fn, ms),
    clearTimer: (h) => clearTimeout(h as ReturnType<typeof setTimeout>),
    ...over,
  })
  return { d, log }
}

beforeEach(() => vi.useFakeTimers())
afterEach(() => vi.useRealTimers())

it('aplica state sem eventos na hora', () => {
  const { d, log } = setup()
  d.pushState(st(1))
  expect(log).toEqual(['apply:1'])
})

it('toca eventos em ordem e só então aplica o state', () => {
  const { d, log } = setup()
  d.pushEvents([played(1), played(2)])
  d.pushState(st(2))
  expect(log).toEqual([`event:card_played:1:${EVENT_DURATION_MS.card_played}`])
  vi.advanceTimersByTime(EVENT_DURATION_MS.card_played)
  expect(log.at(-1)).toBe(`event:card_played:2:${EVENT_DURATION_MS.card_played}`)
  vi.advanceTimersByTime(EVENT_DURATION_MS.card_played)
  expect(log.slice(-2)).toEqual(['apply:2', 'idle'])
})

it('mantém só o último state pendente', () => {
  const { d, log } = setup()
  d.pushEvents([played(1)])
  d.pushState(st(2))
  d.pushEvents([played(2)])
  d.pushState(st(3))
  vi.runAllTimers()
  expect(log.filter((l) => l.startsWith('apply'))).toEqual(['apply:3'])
})

it('descarta a fila quando o atraso passa de 3 s', () => {
  const { d, log } = setup()
  d.pushEvents([{ kind: 'raise_requested', seat: 1, stake: 3 }])
  d.pushState(st(1))
  d.pushEvents(Array.from({ length: 12 }, (_, i) => played(i % 4)))
  d.pushState(st(2))
  expect(log.at(-2)).toBe('apply:2')
  expect(d.busy).toBe(false)
})

it('com a aba oculta aplica direto sem eventos', () => {
  const { d, log } = setup({ hidden: () => true })
  d.pushEvents([played(1)])
  d.pushState(st(4))
  expect(log).toEqual(['apply:4', 'idle'])
})

it('reduced-motion encurta cada evento para 50 ms', () => {
  const { d, log } = setup({ reducedMotion: () => true })
  d.pushEvents([{ kind: 'raise_requested', seat: 1, stake: 3 }])
  expect(log).toEqual(['event:raise_requested:1:50'])
})

it('reset() cancela tudo', () => {
  const { d, log } = setup()
  d.pushEvents([played(1)])
  d.pushState(st(5))
  d.reset()
  vi.runAllTimers()
  expect(log).toEqual([`event:card_played:1:${EVENT_DURATION_MS.card_played}`])
})
