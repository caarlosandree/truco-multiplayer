import { createPinia, setActivePinia } from 'pinia'
import { normalizeCode, useLobbyStore } from './lobby'
import { setSender } from '@/app/transport'
import type { RoomSummary } from '@/protocol/messages'

const room = (id: string, over: Partial<RoomSummary> = {}): RoomSummary => ({
  id,
  name: `Sala ${id}`,
  status: 'waiting',
  seats: [{ displayName: 'Ana', bot: false }, null, null, null],
  ...over,
})

beforeEach(() => setActivePinia(createPinia()))
afterEach(() => setSender(null))

it('aplica snapshot, upsert e remove', () => {
  const lobby = useLobbyStore()
  lobby.onSnapshot({ rooms: [room('a'), room('b')] })
  lobby.onUpsert({ room: room('b', { status: 'playing' }) })
  lobby.onUpsert({ room: room('c') })
  lobby.onRemove({ roomId: 'a' })
  expect(lobby.loaded).toBe(true)
  expect(lobby.list.map((r) => r.id)).toEqual(['c', 'b'])
})

it('normaliza código digitado', () => {
  expect(normalizeCode(' k7q m2p ')).toBe('K7QM2P')
})

it('envia create e join', async () => {
  const send = vi.fn().mockResolvedValue(undefined)
  setSender(send)
  const lobby = useLobbyStore()
  await lobby.createRoom('private')
  await lobby.joinById('r1', 2)
  await lobby.joinByCode(' abc234 ')
  expect(send.mock.calls.map((c) => c[0])).toEqual([
    { type: 'create_room', payload: { visibility: 'private' } },
    { type: 'join_room', payload: { roomId: 'r1', seat: 2 } },
    { type: 'join_room', payload: { code: 'ABC234' } },
  ])
})
