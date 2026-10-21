import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory } from 'vue-router'
import { createAppRouter } from '@/router'
import { setSender } from '@/app/transport'
import LobbyView from '@/views/LobbyView.vue'
import OnlineAgora from './OnlineAgora.vue'
import { usePresenceStore } from '@/stores/presence'
import { useRoomStore } from '@/stores/room'
import { useSessionStore } from '@/stores/session'
import { useToastsStore } from '@/stores/toasts'
import { useLobbyStore } from '@/stores/lobby'
import { WsSendError } from '@/ws/client'
import type { PresenceEntry } from '@/protocol/messages'

const entry = (username: string, over: Partial<PresenceEntry> = {}): PresenceEntry => ({
  userId: `id-${username}`,
  username,
  displayName: username.toUpperCase(),
  status: 'lobby',
  ...over,
})

let pinia: ReturnType<typeof createPinia>
const wrappers: VueWrapper[] = []
beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
  useSessionStore().user = { id: 'id-eu', username: 'eu', email: 'e@x.com', displayName: 'EU', isAdmin: false, createdAt: '' }
})
afterEach(() => {
  setSender(null)
  wrappers.splice(0).forEach((w) => w.unmount())
})

function mountWith<T>(comp: T) {
  const router = createAppRouter(createMemoryHistory())
  const w = mount(comp as never, { global: { plugins: [pinia, router] }, attachTo: document.body }) as VueWrapper
  wrappers.push(w)
  return w
}

function populate(users: PresenceEntry[]) {
  usePresenceStore().onSnapshot({ users })
}

function sentarEmSala() {
  useRoomStore().onRoomState({
    id: 'r1', code: 'ABC234', name: 'Sala', visibility: 'public', host: 0, status: 'waiting',
    seats: [{ userId: 'id-eu', displayName: 'EU', ready: false, connected: true, absent: false }, null, null, null],
    yourSeat: 0,
  })
}

const base = [
  entry('eu', { displayName: 'EU' }),
  entry('bia', { displayName: 'Bia', status: 'waiting', roomName: 'Mesa do RH' }),
  entry('caio', { displayName: 'Caio', status: 'playing', privateRoom: true }),
]

it('lista "Online agora · 3" com ponto e texto de status, eu marcado "você"', () => {
  populate(base)
  const w = mountWith(OnlineAgora)
  expect(w.find('h2').text()).toBe('Online agora · 3')
  expect(w.find('h2').classes()).not.toContain('font-display')
  expect(w.findAll('[data-online]').map((l) => l.attributes('data-online'))).toEqual(['eu', 'bia', 'caio'])
  expect(w.find('[data-online="bia"]').text()).toContain('esperando · Mesa do RH')
  expect(w.find('[data-online="caio"]').text()).toContain('jogando · sala privada')
  expect(w.find('[data-online="eu"]').text()).toContain('você')
  expect(w.find('[data-online="bia"] [data-ponto]').attributes('aria-hidden')).toBe('true')
  expect(w.find('[aria-live]').exists()).toBe(false)
})

it('o nome leva ao perfil público', () => {
  populate([entry('ana lu')])
  const w = mountWith(OnlineAgora)
  expect(w.find('[data-online="ana lu"] a').attributes('href')).toBe('/jogadores/ana%20lu')
})

it('sozinho: "Só você por aqui agora."', () => {
  populate([entry('eu')])
  const w = mountWith(OnlineAgora)
  expect(w.text()).toContain('Só você por aqui agora.')
})

it('antes do snapshot mostra carregando', () => {
  const w = mountWith(OnlineAgora)
  expect(w.text()).toContain('Carregando')
})

it('sem sala não há "Convidar"', () => {
  populate(base)
  const w = mountWith(OnlineAgora)
  expect(w.find('[data-acao="convidar"]').exists()).toBe(false)
})

it('em sala com lugar livre, "Convidar" chama invite; eu e quem já está na sala ficam sem botão', async () => {
  const send = vi.fn().mockResolvedValue(undefined)
  setSender(send)
  populate([...base, entry('dani', { displayName: 'Dani', status: 'waiting', roomName: 'Sala' })])
  sentarEmSala()
  useRoomStore().onRoomState({
    ...useRoomStore().room!,
    seats: [
      { userId: 'id-eu', displayName: 'EU', ready: false, connected: true, absent: false },
      { userId: 'id-dani', displayName: 'Dani', ready: false, connected: true, absent: false },
      null,
      null,
    ],
  })
  const w = mountWith(OnlineAgora)
  expect(w.find('[data-online="eu"] [data-acao="convidar"]').exists()).toBe(false)
  expect(w.find('[data-online="dani"] [data-acao="convidar"]').exists()).toBe(false)
  const btn = w.find('[data-online="bia"] [data-acao="convidar"]')
  expect(btn.attributes('aria-label')).toBe('Convidar Bia')
  await btn.trigger('click')
  expect(send).toHaveBeenCalledWith({ type: 'invite', payload: { username: 'bia' } })
})

it('erro do convite vira toast contextual; já na sala não diz que "você" está em sala', async () => {
  setSender(vi.fn().mockRejectedValue(new WsSendError('already_in_room', 'x')))
  populate(base)
  sentarEmSala()
  const w = mountWith(OnlineAgora)
  await w.find('[data-online="bia"] [data-acao="convidar"]').trigger('click')
  await flushPromises()
  expect(useToastsStore().items.at(-1)?.text).toBe('Essa pessoa já está nesta sala.')
})

it('outros erros do convite usam o texto geral', async () => {
  setSender(vi.fn().mockRejectedValue(new WsSendError('room_full', 'x')))
  populate(base)
  sentarEmSala()
  const w = mountWith(OnlineAgora)
  await w.find('[data-online="bia"] [data-acao="convidar"]').trigger('click')
  await flushPromises()
  expect(useToastsStore().items.at(-1)?.text).toBe('A sala está cheia.')
})

it('sala sem lugar livre ou em jogo: sem "Convidar"', () => {
  populate(base)
  sentarEmSala()
  useRoomStore().onRoomState({ ...useRoomStore().room!, status: 'playing' })
  const w = mountWith(OnlineAgora)
  expect(w.find('[data-acao="convidar"]').exists()).toBe(false)
})

describe('LobbyView', () => {
  beforeEach(() => {
    useLobbyStore().onSnapshot({ rooms: [] })
    populate(base)
  })

  it('tem abas "Salas" e "Online (3)" no celular e troca o painel', async () => {
    const w = mountWith(LobbyView)
    const tablist = w.find('[role="tablist"]')
    expect(tablist.exists()).toBe(true)
    const tabs = tablist.findAll('[role="tab"]')
    expect(tabs.map((t) => t.text())).toEqual(['Salas', 'Online (3)'])
    expect(tabs[0]?.attributes('aria-selected')).toBe('true')
    expect(tabs[1]?.attributes('aria-selected')).toBe('false')
    expect(tabs[0]?.attributes('tabindex')).toBe('0')
    expect(tabs[1]?.attributes('tabindex')).toBe('-1')
    const painelOnline = w.find(`#${tabs[1]?.attributes('aria-controls')}`)
    expect(painelOnline.attributes('role')).toBe('tabpanel')
    expect(painelOnline.attributes('aria-labelledby')).toBe(tabs[1]?.attributes('id'))
    await tabs[1]?.trigger('click')
    expect(tabs[1]?.attributes('aria-selected')).toBe('true')
    expect(w.find(`#${tabs[0]?.attributes('aria-controls')}`).classes()).toContain('max-md:hidden')
    expect(painelOnline.classes()).not.toContain('max-md:hidden')
  })

  it('setas, Home e End movem a aba (padrão WAI)', async () => {
    const w = mountWith(LobbyView)
    const tabs = () => w.findAll('[role="tab"]')
    await tabs()[0]?.trigger('keydown', { key: 'ArrowRight' })
    expect(tabs()[1]?.attributes('aria-selected')).toBe('true')
    await tabs()[1]?.trigger('keydown', { key: 'ArrowRight' })
    expect(tabs()[0]?.attributes('aria-selected')).toBe('true')
    await tabs()[0]?.trigger('keydown', { key: 'End' })
    expect(tabs()[1]?.attributes('aria-selected')).toBe('true')
    await tabs()[1]?.trigger('keydown', { key: 'Home' })
    expect(tabs()[0]?.attributes('aria-selected')).toBe('true')
  })

  it('a mesma OnlineAgora serve desktop e celular (uma só lista no DOM)', () => {
    const w = mountWith(LobbyView)
    expect(w.findAllComponents(OnlineAgora)).toHaveLength(1)
    expect(w.findAll('[data-online]')).toHaveLength(3)
  })
})
