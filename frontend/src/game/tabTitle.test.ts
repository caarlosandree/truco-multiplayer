import { startTabTitleBlink } from './tabTitle'

beforeEach(() => vi.useFakeTimers())
afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
})

it('alterna o título sem foco e restaura ao focar', () => {
  vi.spyOn(document, 'hasFocus').mockReturnValue(false)
  document.title = 'Truco'
  const stop = startTabTitleBlink('Truco', 'Sua vez!', { intervalMs: 1000 })
  vi.advanceTimersByTime(1000)
  expect(document.title).toBe('Sua vez!')
  vi.advanceTimersByTime(1000)
  expect(document.title).toBe('Truco')
  vi.advanceTimersByTime(1000)
  expect(document.title).toBe('Sua vez!')
  window.dispatchEvent(new Event('focus'))
  expect(document.title).toBe('Truco')
  vi.advanceTimersByTime(5000)
  expect(document.title).toBe('Truco')
  stop()
})

it('com a aba em foco não faz nada', () => {
  vi.spyOn(document, 'hasFocus').mockReturnValue(true)
  document.title = 'Truco'
  const stop = startTabTitleBlink('Truco', 'Sua vez!', { intervalMs: 1000 })
  vi.advanceTimersByTime(3000)
  expect(document.title).toBe('Truco')
  stop()
})

it('parar restaura o título e é idempotente', () => {
  vi.spyOn(document, 'hasFocus').mockReturnValue(false)
  document.title = 'Truco'
  const stop = startTabTitleBlink('Truco', 'Sua vez!', { intervalMs: 1000 })
  vi.advanceTimersByTime(1000)
  stop()
  stop()
  expect(document.title).toBe('Truco')
  vi.advanceTimersByTime(5000)
  expect(document.title).toBe('Truco')
})

it('para sozinho se a aba ganhou foco entre dois ciclos (sem evento focus)', () => {
  const focus = vi.spyOn(document, 'hasFocus').mockReturnValue(false)
  document.title = 'Truco'
  startTabTitleBlink('Truco', 'Sua vez!', { intervalMs: 1000 })
  vi.advanceTimersByTime(1000)
  expect(document.title).toBe('Sua vez!')
  focus.mockReturnValue(true)
  vi.advanceTimersByTime(1000)
  expect(document.title).toBe('Truco')
  vi.advanceTimersByTime(3000)
  expect(document.title).toBe('Truco')
})
