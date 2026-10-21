import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory } from 'vue-router'
import { nextTick } from 'vue'
import { createAppRouter } from '@/router'
import { installRoomNavigation } from './navigation'
import { useRoomStore } from '@/stores/room'
import { useToastsStore } from '@/stores/toasts'
import { authApi } from '@/api/auth'
import type { RoomStatePayload } from '@/protocol/messages'

beforeEach(() => {
  setActivePinia(createPinia())
  vi.spyOn(authApi, 'me').mockResolvedValue({ id: 'u1', username: 'ana', email: '', displayName: 'Ana', isAdmin: false, createdAt: '' })
})

const roomState: RoomStatePayload = {
  id: 'r1', code: 'ABC234', name: 'Sala', visibility: 'public', host: 0, status: 'waiting',
  seats: [{ userId: 'u1', displayName: 'Ana', ready: false, connected: true, absent: false }, null, null, null], yourSeat: 0,
}

it('entrar numa sala leva a /sala; ser removido volta ao lobby com aviso', async () => {
  const router = createAppRouter(createMemoryHistory())
  installRoomNavigation(router)
  await router.push('/perfil')
  const room = useRoomStore()
  room.onRoomState(roomState)
  await nextTick()
  await vi.waitFor(() => expect(router.currentRoute.value.name).toBe('sala'))
  room.onLeft({ reason: 'kicked' })
  await nextTick()
  await vi.waitFor(() => expect(router.currentRoute.value.name).toBe('lobby'))
  expect(useToastsStore().items.map((t) => t.text)).toContain('Você foi removido da sala.')
})

it('room_state novo fora de /sala (reconexão) volta para /sala', async () => {
  const router = createAppRouter(createMemoryHistory())
  installRoomNavigation(router)
  await router.push('/sala')
  const room = useRoomStore()
  room.onRoomState(roomState)
  await router.push('/perfil')
  room.onRoomState({ ...roomState })
  await vi.waitFor(() => expect(router.currentRoute.value.name).toBe('sala'))
})

it('sala encerrada avisa sem erro', async () => {
  const router = createAppRouter(createMemoryHistory())
  installRoomNavigation(router)
  await router.push('/sala')
  const room = useRoomStore()
  room.onRoomState(roomState)
  room.onLeft({ reason: 'destroyed' })
  await vi.waitFor(() => expect(useToastsStore().items).toHaveLength(1))
  expect(useToastsStore().items[0]).toMatchObject({ text: 'A sala foi encerrada.', kind: 'info' })
})
