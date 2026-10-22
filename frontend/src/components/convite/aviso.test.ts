import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setSender } from '@/app/transport'
import AvisosConvite from './AvisosConvite.vue'
import AvisoConvite from './AvisoConvite.vue'
import { useInvitesStore } from '@/stores/invites'
import { useRoomStore } from '@/stores/room'
import type { InviteReceivedPayload, RoomStatePayload } from '@/protocol/messages'

const pedirPermissao = vi.hoisted(() => vi.fn(() => Promise.resolve()))
vi.mock('@/app/notifications', () => ({
  askNotificationPermissionOnce: pedirPermissao,
  showInviteNotification: vi.fn(),
}))

const convite = (inviteId: string, over: Partial<InviteReceivedPayload> = {}): InviteReceivedPayload => ({
  inviteId,
  room: { id: 'r1', name: 'Mesa do RH', visibility: 'private' },
  from: { displayName: 'Ana' },
  freeSeats: 2,
  ...over,
})
const sala = (status: 'waiting' | 'playing'): RoomStatePayload => ({
  id: 'r0', code: 'ABC234', name: 'Minha sala', visibility: 'private', host: 0, status,
  seats: [null, null, null, null], yourSeat: 0, ranked: null,
})

let pinia: ReturnType<typeof createPinia>
const wrappers: VueWrapper[] = []
const montar = (c: unknown, props?: Record<string, unknown>): VueWrapper => {
  const w = mount(c as never, { props: props as never, global: { plugins: [pinia] }, attachTo: document.body }) as VueWrapper
  wrappers.push(w)
  return w
}
beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
  pedirPermissao.mockClear()
})
afterEach(() => {
  setSender(null)
  wrappers.splice(0).forEach((w) => w.unmount())
  vi.restoreAllMocks()
})

it('Entrar aceita e Agora não recusa', async () => {
  const send = vi.fn().mockResolvedValue(undefined)
  setSender(send)
  const w = montar(AvisoConvite, { invite: convite('a') })
  expect(w.text()).toContain('Ana te chamou pra mesa!')
  expect(w.text()).toContain('Mesa do RH')
  expect(w.text()).toContain('2 lugares livres')
  await w.get('[data-acao="entrar"]').trigger('click')
  expect(send).toHaveBeenLastCalledWith({ type: 'invite_respond', payload: { inviteId: 'a', accept: true } })
  await w.get('[data-acao="agora-nao"]').trigger('click')
  expect(send).toHaveBeenLastCalledWith({ type: 'invite_respond', payload: { inviteId: 'a', accept: false } })
})

it('um lugar livre fica no singular', () => {
  const w = montar(AvisoConvite, { invite: convite('a', { freeSeats: 1 }) })
  expect(w.text()).toContain('1 lugar livre')
})

it('Esc com o aviso focado é Agora não', async () => {
  const send = vi.fn().mockResolvedValue(undefined)
  setSender(send)
  const w = montar(AvisoConvite, { invite: convite('a') })
  await w.get('[data-acao="entrar"]').trigger('keydown', { key: 'Escape' })
  expect(send).toHaveBeenCalledWith({ type: 'invite_respond', payload: { inviteId: 'a', accept: false } })
})

it('Entrar recebe foco ao chegar o convite', () => {
  const w = montar(AvisoConvite, { invite: convite('a') })
  expect(document.activeElement).toBe(w.get('[data-acao="entrar"]').element)
})

it('Entrar não rouba o foco de um campo de texto', () => {
  const campo = document.createElement('input')
  document.body.append(campo)
  campo.focus()
  const w = montar(AvisoConvite, { invite: convite('a') })
  expect(document.activeElement).toBe(campo)
  expect(document.activeElement).not.toBe(w.get('[data-acao="entrar"]').element)
})

it('Entrar não rouba o foco de um diálogo aberto', () => {
  const dlg = document.createElement('div')
  dlg.setAttribute('role', 'dialog')
  const botao = document.createElement('button')
  dlg.append(botao)
  document.body.append(dlg)
  botao.focus()
  montar(AvisoConvite, { invite: convite('a') })
  expect(document.activeElement).toBe(botao)
})

it('jogando: sem Entrar, com "Você está numa partida"', () => {
  useRoomStore().onRoomState(sala('playing'))
  const w = montar(AvisoConvite, { invite: convite('a') })
  expect(w.find('[data-acao="entrar"]').exists()).toBe(false)
  expect(w.text()).toContain('Você está numa partida')
  expect(w.find('[data-acao="agora-nao"]').exists()).toBe(true)
})

it('em sala de espera avisa que aceitar tira da sala atual', () => {
  useRoomStore().onRoomState(sala('waiting'))
  const w = montar(AvisoConvite, { invite: convite('a') })
  expect(w.text()).toContain('Aceitar tira você da sala atual.')
  expect(w.find('[data-acao="entrar"]').exists()).toBe(true)
})

it('fora de sala não mostra aviso de troca', () => {
  const w = montar(AvisoConvite, { invite: convite('a') })
  expect(w.text()).not.toContain('Aceitar tira')
})

it('4 convites viram 3 avisos, o mais novo primeiro, sem aria-live próprio', () => {
  const s = useInvitesStore()
  for (const id of ['a', 'b', 'c', 'd']) s.onReceived(convite(id, { from: { displayName: `De ${id}` } }))
  const w = montar(AvisosConvite)
  const avisos = w.findAll('[data-aviso]')
  expect(avisos.map((a) => a.attributes('data-invite-id'))).toEqual(['d', 'c', 'b'])
  expect(w.find('[aria-live]').exists()).toBe(false)
  expect(w.find('[role="alert"],[role="status"],[role="log"]').exists()).toBe(false)
})

it('a primeira resposta pede a permissão de notificação (uma chamada por resposta; o módulo decide)', async () => {
  setSender(vi.fn().mockResolvedValue(undefined))
  const w = montar(AvisoConvite, { invite: convite('a') })
  await w.get('[data-acao="agora-nao"]').trigger('click')
  await flushPromises()
  expect(pedirPermissao).toHaveBeenCalledTimes(1)
})

it('Esc não pede a permissão de notificação (só os cliques pedem)', async () => {
  setSender(vi.fn().mockResolvedValue(undefined))
  const w = montar(AvisoConvite, { invite: convite('a') })
  await w.get('[data-acao="entrar"]').trigger('keydown', { key: 'Escape' })
  await flushPromises()
  expect(pedirPermissao).not.toHaveBeenCalled()
  const w2 = montar(AvisoConvite, { invite: convite('b') })
  await w2.get('[data-acao="entrar"]').trigger('click')
  expect(pedirPermissao).toHaveBeenCalledTimes(1)
})

it('jogando: só o aviso mais novo, com "+N convites"; os demais aparecem ao responder', async () => {
  setSender(vi.fn().mockResolvedValue(undefined))
  useRoomStore().onRoomState(sala('playing'))
  const s = useInvitesStore()
  for (const id of ['a', 'b', 'c', 'd']) s.onReceived(convite(id))
  const w = montar(AvisosConvite)
  expect(w.findAll('[data-aviso]').map((a) => a.attributes('data-invite-id'))).toEqual(['d'])
  expect(w.get('[data-mais-convites]').text()).toBe('+3 convites')
  await w.get('[data-acao="agora-nao"]').trigger('click')
  await flushPromises()
  expect(w.findAll('[data-aviso]').map((a) => a.attributes('data-invite-id'))).toEqual(['c'])
  expect(w.get('[data-mais-convites]').text()).toBe('+2 convites')
})

it('fora da partida mantém até 3 avisos e não mostra "+N"', () => {
  useRoomStore().onRoomState(sala('waiting'))
  const s = useInvitesStore()
  for (const id of ['a', 'b', 'c']) s.onReceived(convite(id))
  const w = montar(AvisosConvite)
  expect(w.findAll('[data-aviso]')).toHaveLength(3)
  expect(w.find('[data-mais-convites]').exists()).toBe(false)
})

it('erro ao responder vira toast', async () => {
  setSender(vi.fn().mockRejectedValue(new Error('falhou')))
  const w = montar(AvisoConvite, { invite: convite('a') })
  await w.get('[data-acao="entrar"]').trigger('click')
  await flushPromises()
  const { useToastsStore } = await import('@/stores/toasts')
  expect(useToastsStore().items.length).toBeGreaterThan(0)
})

describe('AppShell', () => {
  it('monta exatamente um Anunciador e os avisos de convite', async () => {
    const { default: AppShell } = await import('@/components/AppShell.vue')
    const { useSessionStore } = await import('@/stores/session')
    useSessionStore().user = { id: 'u', username: 'eu', email: 'e@x.com', displayName: 'EU', isAdmin: false, createdAt: '' }
    useInvitesStore().onReceived(convite('a'))
    const w = mount(AppShell, {
      global: { plugins: [pinia], stubs: { RouterLink: true, RouterView: true } },
    })
    wrappers.push(w as VueWrapper)
    expect(w.findAll('[data-anunciador]')).toHaveLength(1)
    expect(w.findAll('[data-aviso]')).toHaveLength(1)
  })
})
