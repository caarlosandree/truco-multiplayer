import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory } from 'vue-router'
import { createAppRouter } from '@/router'
import { useRoomStore } from '@/stores/room'
import { useSessionStore } from '@/stores/session'
import { useToastsStore } from '@/stores/toasts'
import ConviteView from './ConviteView.vue'

beforeEach(() => setActivePinia(createPinia()))

it('convite estando em outra sala: avisa e leva à sala atual', async () => {
  const router = createAppRouter(createMemoryHistory())
  useSessionStore().connection = 'open'
  useRoomStore().onRoomState({
    id: 'r1',
    code: 'ABC234',
    name: 'Sala',
    visibility: 'private',
    host: 0,
    status: 'waiting',
    seats: [null, null, null, null],
    yourSeat: 0,
  })
  await router.push('/convite/XYZ789')
  const replace = vi.spyOn(router, 'replace')
  mount(ConviteView, { global: { plugins: [router] } })
  await flushPromises()
  expect(useToastsStore().items.at(-1)?.text).toBe('Você já está em outra sala.')
  expect(replace).toHaveBeenCalledWith({ name: 'sala' })
})
