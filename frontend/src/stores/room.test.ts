import { createPinia, setActivePinia } from 'pinia'
import { useRoomStore } from './room'
import { useGameStore } from './game'
import { setSender } from '@/app/transport'
import type { RoomStatePayload } from '@/protocol/messages'

afterEach(() => setSender(null))

const state = (over: Partial<RoomStatePayload> = {}): RoomStatePayload => ({
  id: 'r1',
  code: 'ABC234',
  name: 'Sala da Ana',
  visibility: 'private',
  host: 0,
  status: 'waiting',
  seats: [
    { userId: 'u1', displayName: 'Ana', ready: true, connected: true, absent: false },
    null,
    { userId: 'u3', displayName: 'Caio', ready: false, connected: true, absent: false },
    null,
  ],
  yourSeat: 0,
  ...over,
})

beforeEach(() => setActivePinia(createPinia()))

it('deriva host, assento, dupla e pronto', () => {
  const r = useRoomStore()
  r.onRoomState(state())
  expect([r.inRoom, r.isHost, r.mySeat, r.myTeam, r.amReady, r.isPlaying]).toEqual([true, true, 0, 0, true, false])
})

it('volta a waiting limpa o jogo; left_room limpa tudo', () => {
  const r = useRoomStore()
  const game = useGameStore()
  const reset = vi.spyOn(game, 'reset')
  r.onRoomState(state({ status: 'playing', matchId: 'm1' }))
  r.onForfeit({ team: 1 })
  r.onRoomState(state({ status: 'waiting' }))
  expect(reset).toHaveBeenCalledOnce()
  expect(r.forfeitTeam).toBeNull()
  r.onLeft({ reason: 'kicked' })
  expect(r.room).toBeNull()
  expect(r.lastLeft).toBe('kicked')
  expect(reset).toHaveBeenCalledTimes(2)
})

it('guarda e descarta o fim de partida', () => {
  const r = useRoomStore()
  r.onMatchEnded({ reason: 'forfeit', winner: 0, score: [12, 4] })
  expect(r.ended?.reason).toBe('forfeit')
  r.dismissEnded()
  expect(r.ended).toBeNull()
})

it('envia as ações de sala', async () => {
  const send = vi.fn().mockResolvedValue(undefined)
  setSender(send)
  const r = useRoomStore()
  await r.takeSeat(1)
  await r.setReady(true)
  await r.kick(2)
  await r.claimForfeit()
  await r.leave()
  expect(send.mock.calls.map((c) => c[0])).toEqual([
    { type: 'take_seat', payload: { seat: 1 } },
    { type: 'set_ready', payload: { ready: true } },
    { type: 'kick', payload: { seat: 2 } },
    { type: 'claim_forfeit' },
    { type: 'leave_room' },
  ])
})
