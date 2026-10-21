import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory } from 'vue-router'
import { setSender } from '@/app/transport'
import { createAppRouter } from '@/router'
import { authApi } from '@/api/auth'
import SalaEspera from './SalaEspera.vue'
import SalaView from '@/views/SalaView.vue'
import { useLobbyStore } from '@/stores/lobby'
import { useRoomStore } from '@/stores/room'
import { useSessionStore } from '@/stores/session'
import { useToastsStore } from '@/stores/toasts'
import { WsSendError } from '@/ws/client'
import type { RoomStatePayload } from '@/protocol/messages'

const base: RoomStatePayload = {
  id: 'r1', code: 'ABC234', name: 'Sala da Ana', visibility: 'private', host: 0, status: 'waiting',
  seats: [
    { userId: 'u1', displayName: 'Ana', ready: false, connected: true, absent: false },
    { userId: 'u2', displayName: 'Bia', ready: true, connected: false, absent: false },
    null,
    null,
  ],
  yourSeat: 0,
}
const dialog = () => document.querySelector('[role="dialog"]')
const wrappers: VueWrapper[] = []

let pinia: ReturnType<typeof createPinia>
beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
})
afterEach(() => {
  setSender(null)
  wrappers.splice(0).forEach((w) => w.unmount())
  vi.useRealTimers()
  vi.restoreAllMocks()
})

function mountEspera() {
  const w = mount(SalaEspera, { global: { plugins: [pinia] }, attachTo: document.body })
  wrappers.push(w)
  return w
}

it('mostra código, presença e pronto; sentar e pronto enviam; remover pede confirmação', async () => {
  const send = vi.fn().mockResolvedValue(undefined)
  setSender(send)
  useRoomStore().onRoomState(base)
  const w = mountEspera()
  expect(w.text()).toContain('ABC234')
  expect(w.find('[data-assento="1"]').text()).toContain('Pronto')
  expect(w.find('[data-assento="1"]').text()).toContain('Desconectado')
  await w.find('[data-assento="1"] [data-acao="remover"]').trigger('click')
  expect(send).not.toHaveBeenCalled()
  expect(dialog()?.textContent).toContain('Bia')
  await w.find('[data-assento="2"] [data-acao="sentar"]').trigger('click')
  await w.find('[role="switch"]').trigger('click')
  document.querySelector<HTMLButtonElement>('[data-acao="confirmar-remocao"]')?.click()
  await flushPromises()
  expect(send.mock.calls.map((c) => c[0])).toEqual([
    { type: 'take_seat', payload: { seat: 2 } },
    { type: 'set_ready', payload: { ready: true } },
    { type: 'kick', payload: { seat: 1 } },
  ])
})

it('cancelar a remoção não envia nada', async () => {
  const send = vi.fn().mockResolvedValue(undefined)
  setSender(send)
  useRoomStore().onRoomState(base)
  const w = mountEspera()
  await w.find('[data-acao="remover"]').trigger('click')
  document.querySelector<HTMLButtonElement>('[data-acao="cancelar-remocao"]')?.click()
  await flushPromises()
  expect(dialog()).toBeNull()
  expect(send).not.toHaveBeenCalled()
})

it('duplas relativas a mim: lugares 0/2 são "nós" e 1/3 "eles", com texto para leitor de tela', () => {
  useRoomStore().onRoomState({ ...base, yourSeat: 1 })
  const w = mountEspera()
  expect(w.find('[data-assento="0"]').text()).toContain('Eles')
  expect(w.find('[data-assento="1"]').text()).toContain('Nós')
  expect(w.find('[data-assento="3"]').text()).toContain('Nós')
  expect(w.find('[data-assento="1"]').text()).toContain('Lugar 2, nós')
})

it('pronto ganha carimbo de giz só em quem está pronto', () => {
  useRoomStore().onRoomState(base)
  const w = mountEspera()
  expect(w.find('[data-assento="1"] [data-pronto]').exists()).toBe(true)
  expect(w.find('[data-assento="0"] [data-pronto]').exists()).toBe(false)
})

it('quem não é host não vê remover; sala pública não mostra código nem copiar', () => {
  useRoomStore().onRoomState({ ...base, visibility: 'public', yourSeat: 1 })
  const w = mountEspera()
  expect(w.find('[data-acao="remover"]').exists()).toBe(false)
  expect(w.find('[data-acao="copiar"]').exists()).toBe(false)
  expect(w.text()).not.toContain('ABC234')
})

it('copia o link de convite e avisa; sem clipboard mostra o link', async () => {
  const writeText = vi.fn().mockResolvedValue(undefined)
  Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
  useRoomStore().onRoomState(base)
  const w = mountEspera()
  await w.find('[data-acao="copiar"]').trigger('click')
  await flushPromises()
  expect(writeText).toHaveBeenCalledWith(`${location.origin}/sala/ABC234`)
  expect(useToastsStore().items.map((t) => t.text)).toContain('Link copiado.')
  writeText.mockRejectedValue(new Error('negado'))
  await w.find('[data-acao="copiar"]').trigger('click')
  await flushPromises()
  expect(useToastsStore().items.map((t) => t.text)).toContain(`Copie o link: ${location.origin}/sala/ABC234`)
})

it('erro do servidor ao sentar vira toast traduzido', async () => {
  setSender(vi.fn().mockRejectedValue(new WsSendError('seat_taken', 'x')))
  useRoomStore().onRoomState(base)
  const w = mountEspera()
  await w.find('[data-acao="sentar"]').trigger('click')
  await flushPromises()
  expect(useToastsStore().items).toHaveLength(1)
  expect(useToastsStore().items[0]).toMatchObject({ kind: 'error', text: 'Esse lugar já está ocupado.' })
})

it('sair da sala envia leave_room', async () => {
  const send = vi.fn().mockResolvedValue(undefined)
  setSender(send)
  useRoomStore().onRoomState(base)
  const w = mountEspera()
  await w.find('[data-acao="sair"]').trigger('click')
  expect(send).toHaveBeenCalledWith({ type: 'leave_room' })
})

describe('SalaView', () => {
  async function mountView() {
    vi.spyOn(authApi, 'me').mockResolvedValue({ id: 'u1', username: 'ana', email: '', displayName: 'Ana', isAdmin: false, createdAt: '' })
    const router = createAppRouter(createMemoryHistory())
    await router.push('/sala')
    const w = mount(SalaView, { global: { plugins: [pinia, router] } })
    wrappers.push(w)
    return router
  }

  it('/sala sem sala: só volta ao lobby depois do hello E do lobby_snapshot', async () => {
    const router = await mountView()
    const session = useSessionStore()
    const lobby = useLobbyStore()
    await new Promise((r) => setTimeout(r, 30))
    expect(router.currentRoute.value.name).toBe('sala') // antes do hello
    session.setConnection('open')
    await new Promise((r) => setTimeout(r, 30))
    expect(router.currentRoute.value.name).toBe('sala') // hello sem snapshot: ainda pode vir room_state
    lobby.onSnapshot({ rooms: [] })
    await vi.waitFor(() => expect(router.currentRoute.value.name).toBe('lobby'))
  })

  it('room_state em vez de snapshot não redireciona; novo hello invalida o snapshot antigo', async () => {
    const router = await mountView()
    const lobby = useLobbyStore()
    lobby.markStale()
    useSessionStore().setConnection('open')
    useRoomStore().onRoomState(base)
    lobby.onSnapshot({ rooms: [] }) // com sala, o snapshot não tira ninguém daqui
    await new Promise((r) => setTimeout(r, 30))
    expect(router.currentRoute.value.name).toBe('sala')
    expect(lobby.loaded).toBe(true)
    lobby.markStale()
    expect(lobby.loaded).toBe(false)
  })
})
