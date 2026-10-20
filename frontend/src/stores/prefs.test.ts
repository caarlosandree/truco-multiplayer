import { createPinia, setActivePinia } from 'pinia'
import { usePrefsStore } from './prefs'

beforeEach(() => {
  localStorage.clear()
  setActivePinia(createPinia())
})
afterEach(() => vi.restoreAllMocks())

it('usa padrões e persiste mudanças', () => {
  const prefs = usePrefsStore()
  expect(prefs.deck).toBe('frances')
  expect(prefs.volume).toBe(0.7)
  expect(prefs.muted).toBe(false)
  prefs.setDeck('espanhol')
  prefs.setVolume(2)
  expect(prefs.volume).toBe(1)
  expect(JSON.parse(localStorage.getItem('truco.prefs')!)).toEqual({ deck: 'espanhol', volume: 1, muted: false })
})

it('lê valores salvos e ignora lixo', () => {
  localStorage.setItem('truco.prefs', JSON.stringify({ deck: 'espanhol', volume: 'x', muted: true }))
  const prefs = usePrefsStore()
  expect(prefs.deck).toBe('espanhol')
  expect(prefs.volume).toBe(0.7)
  expect(prefs.muted).toBe(true)
})

it('funciona sem localStorage', () => {
  vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
    throw new Error('bloqueado')
  })
  vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
    throw new Error('bloqueado')
  })
  const prefs = usePrefsStore()
  expect(prefs.deck).toBe('frances')
  expect(() => prefs.setMuted(true)).not.toThrow()
  expect(prefs.muted).toBe(true)
})
