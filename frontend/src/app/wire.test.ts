import { createPinia, setActivePinia } from 'pinia'
import { dispatch } from './wire'
import { useLobbyStore } from '@/stores/lobby'
import { useRoomStore } from '@/stores/room'
import { useGameStore } from '@/stores/game'
import type { PlayerView } from '@/protocol/engine'

beforeEach(() => setActivePinia(createPinia()))

const view: PlayerView = {
  seat: 0,
  phase: 'waiting',
  handNumber: 0,
  players: [],
  score: [0, 0],
  dealer: 0,
  stake: 1,
  table: [],
  rounds: [],
  deckCount: 0,
  discardCount: 0,
  myHand: [],
}

it('entrega cada tipo à store dona', () => {
  const lobby = useLobbyStore()
  const room = useRoomStore()
  const game = useGameStore()
  const spies = {
    snapshot: vi.spyOn(lobby, 'onSnapshot'),
    upsert: vi.spyOn(lobby, 'onUpsert'),
    removed: vi.spyOn(lobby, 'onRemove'),
    roomState: vi.spyOn(room, 'onRoomState'),
    left: vi.spyOn(room, 'onLeft'),
    forfeit: vi.spyOn(room, 'onForfeit'),
    ended: vi.spyOn(room, 'onMatchEnded'),
    state: vi.spyOn(game, 'onState'),
    events: vi.spyOn(game, 'onEvents'),
    signal: vi.spyOn(game, 'onSignal'),
  }
  dispatch({ type: 'lobby_snapshot', payload: { rooms: [] } })
  dispatch({ type: 'room_upserted', payload: { room: { id: 'r', name: 'n', status: 'waiting', seats: [null, null, null, null] } } })
  dispatch({ type: 'room_removed', payload: { roomId: 'r' } })
  dispatch({
    type: 'room_state',
    payload: { id: 'r', code: 'ABC', name: 'n', visibility: 'public', host: 0, status: 'waiting', seats: [null, null, null, null], yourSeat: 0 },
  })
  dispatch({ type: 'left_room', payload: { reason: 'left' } })
  dispatch({ type: 'forfeit_available', payload: { team: 1 } })
  dispatch({ type: 'match_ended', payload: { reason: 'game_over', winner: 0, score: [12, 3] } })
  dispatch({ type: 'state', payload: { view, deadlines: {}, pendingSeats: [] } })
  dispatch({ type: 'events', payload: [] })
  dispatch({ type: 'signal', payload: { seat: 1, id: 'boa' } })
  for (const spy of Object.values(spies)) {
    expect(spy).toHaveBeenCalledOnce()
  }
})
