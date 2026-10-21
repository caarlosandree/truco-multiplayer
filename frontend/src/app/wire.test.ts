import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory } from 'vue-router'
import { createAppRouter } from '@/router'
import { useSessionStore } from '@/stores/session'
import { authApi } from '@/api/auth'
import { dispatch, installConnection } from './wire'
import { useLobbyStore } from '@/stores/lobby'
import { useRoomStore } from '@/stores/room'
import { useGameStore } from '@/stores/game'
import { usePresenceStore } from '@/stores/presence'
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
  const presence = usePresenceStore()
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
    presSnapshot: vi.spyOn(presence, 'onSnapshot'),
    presUpdated: vi.spyOn(presence, 'onUpdated'),
    presRemoved: vi.spyOn(presence, 'onRemoved'),
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
  dispatch({ type: 'presence_snapshot', payload: { users: [] } })
  dispatch({ type: 'presence_updated', payload: { user: { userId: 'u', username: 'u', displayName: 'U', status: 'lobby' } } })
  dispatch({ type: 'presence_removed', payload: { userId: 'u' } })
  for (const spy of Object.values(spies)) {
    expect(spy).toHaveBeenCalledOnce()
  }
})

describe('sessão inválida (4003)', () => {
  it('manda para /entrar com volta e não entra em laço estando em /entrar', async () => {
    vi.spyOn(authApi, 'me').mockResolvedValue({ id: 'u1', username: 'a', email: '', displayName: 'A', isAdmin: false, createdAt: '' })
    const router = createAppRouter(createMemoryHistory())
    await router.push('/perfil')
    useSessionStore().user = null // deslogado no wire: sem WebSocket real
    installConnection(router)
    useSessionStore().setConnection('unauthorized')
    await vi.waitFor(() => expect(router.currentRoute.value.name).toBe('entrar'))
    expect(router.currentRoute.value.query.volta).toBe('/perfil')
    useSessionStore().setConnection('open')
    useSessionStore().setConnection('unauthorized')
    await new Promise((r) => setTimeout(r, 10))
    expect(router.currentRoute.value.query.volta).toBe('/perfil')
  })
})

describe('reconexão e sala fantasma', () => {
  const roomState = {
    id: 'r',
    code: 'ABC',
    name: 'n',
    visibility: 'public' as const,
    host: 0,
    status: 'waiting' as const,
    seats: [null, null, null, null],
    yourSeat: 0,
  }

  it('lobby_snapshot com sala antiga no store: o servidor diz que estamos fora, limpa sala e jogo', () => {
    const room = useRoomStore()
    const game = useGameStore()
    dispatch({ type: 'room_state', payload: roomState })
    expect(room.inRoom).toBe(true)
    const reset = vi.spyOn(game, 'reset')
    dispatch({ type: 'lobby_snapshot', payload: { rooms: [] } })
    expect(room.inRoom).toBe(false)
    expect(room.forfeitTeam).toBeNull()
    expect(reset).toHaveBeenCalled()
    expect(useLobbyStore().loaded).toBe(true)
  })

  it('reconexão dentro da sala (room_state, sem snapshot): mantém a sala', () => {
    const room = useRoomStore()
    dispatch({ type: 'room_state', payload: roomState })
    dispatch({ type: 'room_state', payload: { ...roomState, name: 'novo' } })
    expect(room.inRoom).toBe(true)
    expect(room.room?.name).toBe('novo')
  })

  it('lobby_snapshot sem sala não mexe no jogo', () => {
    const reset = vi.spyOn(useGameStore(), 'reset')
    dispatch({ type: 'lobby_snapshot', payload: { rooms: [] } })
    expect(reset).not.toHaveBeenCalled()
  })
})

describe('presença', () => {
  const ana = { userId: 'u-ana', username: 'ana', displayName: 'Ana', status: 'lobby' as const }

  it('snapshot, atualização e remoção chegam à store', () => {
    const presence = usePresenceStore()
    dispatch({ type: 'presence_snapshot', payload: { users: [ana] } })
    dispatch({ type: 'presence_updated', payload: { user: { ...ana, status: 'waiting', roomName: 'Mesa' } } })
    expect(presence.byUsername('ana')?.status).toBe('waiting')
    dispatch({ type: 'presence_removed', payload: { userId: 'u-ana' } })
    expect(presence.count).toBe(0)
  })

  it('logout (stop) zera a presença', () => {
    const presence = usePresenceStore()
    dispatch({ type: 'presence_snapshot', payload: { users: [ana] } })
    expect(presence.count).toBe(1)
    useSessionStore().user = null
    installConnection(createAppRouter(createMemoryHistory())) // sem usuário, o watch imediato roda stop()
    expect(presence.count).toBe(0)
    expect(presence.loaded).toBe(false)
  })
})
