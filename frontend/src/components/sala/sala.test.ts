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
    { userId: 'u1', displayName: 'Ana', ready: false, connected: true, absent: false, bot: false, botPlaying: false },
    { userId: 'u2', displayName: 'Bia', ready: true, connected: false, absent: false, bot: false, botPlaying: false },
    null,
    null,
  ],
  yourSeat: 0, ranked: null,
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

  it('fim de partida: o modal aparece sobre a sala de espera, "Revanche" o fecha e ele não volta', async () => {
    await mountView()
    useSessionStore().setConnection('open')
    const room = useRoomStore()
    room.onRoomState({ ...base, status: 'playing', matchId: 'm' })
    room.onMatchEnded({ reason: 'game_over', winner: 0, score: [12, 7], ranked: true })
    // o servidor volta a sala a "waiting" (sem prontos)
    room.onRoomState({ ...base, seats: base.seats.map((s) => (s ? { ...s, ready: false } : s)) })
    await flushPromises()
    expect(dialog()?.textContent).toContain('Vitória')
    expect(wrappers[0]?.text()).toContain('Sala da Ana')
    ;(document.querySelector('[data-acao="revanche"]') as HTMLButtonElement).click()
    await flushPromises()
    expect(dialog()).toBeNull()
    expect(room.ended).toBeNull()
    expect(room.isPlaying).toBe(false)
    // sair e voltar a uma sala: nenhum fim de partida antigo reaparece
    room.onLeft({ reason: 'left' })
    room.onRoomState({ ...base, id: 'r2' })
    await flushPromises()
    expect(dialog()).toBeNull()
  })
})

describe('bots na sala de espera', () => {
  const seat = (displayName: string, over: Partial<NonNullable<RoomStatePayload['seats'][number]>> = {}) => ({
    userId: `u-${displayName}`, displayName, ready: false, connected: true, absent: false, bot: false, botPlaying: false, ...over,
  })
  const sala = (seats: RoomStatePayload['seats'], over: Partial<RoomStatePayload> = {}): RoomStatePayload => ({ ...base, seats, ...over })
  const botSeat = (nome: string) => seat(nome, { ready: true, bot: true })

  it('dono com cadeira vazia vê "Começar com bots" e o clique envia start_with_bots', async () => {
    const send = vi.fn().mockResolvedValue(undefined)
    setSender(send)
    useRoomStore().onRoomState(sala([seat('Ana', { ready: true }), null, null, null]))
    const w = mountEspera()
    const botao = w.find('[data-acao="comecar-com-bots"]')
    expect(botao.text()).toContain('Começar com bots')
    expect(botao.attributes('disabled')).toBeUndefined()
    await botao.trigger('click')
    expect(send).toHaveBeenCalledWith({ type: 'start_with_bots' })
  })

  it('com outro humano não pronto o botão fica desabilitado e diz quantos faltam', () => {
    useRoomStore().onRoomState(sala([seat('Ana'), seat('Bia'), null, null]))
    const w = mountEspera()
    expect(w.find('[data-acao="comecar-com-bots"]').attributes('disabled')).toBeDefined()
    expect(w.text()).toContain('Faltam 1 prontos')
  })

  it('não dono não vê o botão; sem cadeira vazia (bots já sentados) o botão some', () => {
    useRoomStore().onRoomState(sala([seat('Ana', { ready: true }), seat('Bia'), null, null], { host: 0, yourSeat: 1 }))
    const w = mountEspera()
    expect(w.find('[data-acao="comecar-com-bots"]').exists()).toBe(false)
    useRoomStore().onRoomState(sala([seat('Ana'), botSeat('Seu Zé'), botSeat('Dona Cida'), botSeat('Tio Juca')]))
    return w.vm.$nextTick().then(() => {
      expect(w.find('[data-acao="comecar-com-bots"]').exists()).toBe(false)
    })
  })

  it('cadeira de bot: nome, selo e pronto; aviso de ranking', () => {
    useRoomStore().onRoomState(sala([seat('Ana'), botSeat('Seu Zé'), null, null]))
    const w = mountEspera()
    const bot = w.find('[data-assento="1"]')
    expect(bot.text()).toContain('Seu Zé')
    expect(bot.find('[data-selo-bot]').text()).toContain('bot')
    expect(bot.text()).toContain('Pronto')
    expect(w.text()).toContain('Partidas com bots não contam no ranking.')
  })

  it('sem bot não mostra o aviso de ranking', () => {
    useRoomStore().onRoomState(base)
    expect(mountEspera().text()).not.toContain('não contam no ranking')
  })

  it('não dono vê "Sentar aqui" na cadeira do bot e o clique envia take_seat', async () => {
    const send = vi.fn().mockResolvedValue(undefined)
    setSender(send)
    useRoomStore().onRoomState(sala([seat('Ana'), seat('Bia'), botSeat('Seu Zé'), null], { yourSeat: 1 }))
    const w = mountEspera()
    const sentar = w.find('[data-assento="2"] [data-acao="sentar"]')
    expect(sentar.text()).toContain('Sentar aqui')
    expect(w.find('[data-assento="2"] [data-acao="remover"]').exists()).toBe(false)
    await sentar.trigger('click')
    expect(send).toHaveBeenCalledWith({ type: 'take_seat', payload: { seat: 2 } })
  })

  it('dono vê "Remover" na cadeira do bot, sem "Sentar aqui", e remove sem abrir o modal', async () => {
    const send = vi.fn().mockResolvedValue(undefined)
    setSender(send)
    useRoomStore().onRoomState(sala([seat('Ana'), botSeat('Seu Zé'), null, null]))
    const w = mountEspera()
    expect(w.find('[data-assento="1"] [data-acao="sentar"]').exists()).toBe(false)
    const remover = w.find('[data-assento="1"] [data-acao="remover"]')
    expect(remover.text()).toContain('Remover')
    await remover.trigger('click')
    await flushPromises()
    expect(dialog()).toBeNull()
    expect(send).toHaveBeenCalledWith({ type: 'kick', payload: { seat: 1 } })
  })
})
