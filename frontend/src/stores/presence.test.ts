import { createPinia, setActivePinia } from 'pinia'
import type { PresenceEntry } from '@/protocol/messages'
import { presenceText } from '@/game/labels'
import { usePresenceStore } from './presence'

beforeEach(() => setActivePinia(createPinia()))

const pessoa = (userId: string, username: string, displayName: string) => (status: string): PresenceEntry => ({
  userId,
  username,
  displayName,
  status,
})
const ana = pessoa('u-ana', 'ana', 'Ana')
const bia = pessoa('u-bia', 'bia', 'Bia')

it('snapshot, atualização e remoção', () => {
  const s = usePresenceStore()
  s.onSnapshot({ users: [ana('lobby'), bia('playing')] })
  expect(s.loaded).toBe(true)
  s.onUpdated({ user: { ...bia('waiting'), roomName: 'Mesa do RH' } })
  s.onRemoved({ userId: 'u-ana' })
  expect(s.list.map((e) => e.username)).toEqual(['bia'])
  expect(s.count).toBe(1)
  const [primeira] = s.list
  expect(primeira && presenceText(primeira)).toBe('esperando · Mesa do RH')
})

it('ordena lobby, esperando, jogando e depois por nome', () => {
  const s = usePresenceStore()
  const caio = pessoa('u-caio', 'caio', 'Caio')
  const edu = pessoa('u-edu', 'edu', 'Édu')
  s.onSnapshot({ users: [caio('playing'), bia('waiting'), edu('lobby'), ana('lobby')] })
  expect(s.list.map((e) => e.username)).toEqual(['ana', 'edu', 'bia', 'caio'])
})

it('byUsername e $reset', () => {
  const s = usePresenceStore()
  s.onSnapshot({ users: [ana('lobby')] })
  expect(s.byUsername('ana')?.userId).toBe('u-ana')
  expect(s.byUsername('zé')).toBeUndefined()
  s.$reset()
  expect(s.count).toBe(0)
  expect(s.loaded).toBe(false)
})

describe('presenceText', () => {
  it('lobby, jogando e sala privada', () => {
    expect(presenceText(ana('lobby'))).toBe('no lobby')
    expect(presenceText({ ...bia('playing'), roomName: 'Truco da TI' })).toBe('jogando · Truco da TI')
    expect(presenceText({ ...bia('waiting'), privateRoom: true })).toBe('esperando · sala privada')
  })
})
