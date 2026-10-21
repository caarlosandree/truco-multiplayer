import type { Mock } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory } from 'vue-router'
import { createAppRouter } from '@/router'
import { setSender, type Sender } from '@/app/transport'
import { useInvitesStore } from '@/stores/invites'
import { useRoomStore } from '@/stores/room'
import { useSessionStore } from '@/stores/session'
import { useToastsStore } from '@/stores/toasts'
import { authApi } from '@/api/auth'
import EntrarConviteView from './EntrarConviteView.vue'

const sala = (id: string) => ({
  id,
  code: 'ABC234',
  name: 'Sala',
  visibility: 'private' as const,
  host: 0 as const,
  status: 'waiting' as const,
  seats: [null, null, null, null] as never,
  yourSeat: 1 as const,
})

async function montar(conexao: 'open' | 'connecting' = 'open') {
  const router = createAppRouter(createMemoryHistory())
  useSessionStore().connection = conexao
  await router.push('/entrar-convite/abc')
  const wrapper = mount(EntrarConviteView, { global: { plugins: [router] } })
  await flushPromises()
  return { router, wrapper }
}

let sender: Mock<Sender>

beforeEach(() => {
  vi.spyOn(authApi, 'me').mockResolvedValue({ id: 'u1', username: 'ana', email: 'a@e.com', displayName: 'Ana', isAdmin: false, createdAt: '' })
  setActivePinia(createPinia())
  sender = vi.fn<Sender>().mockResolvedValue(undefined)
  setSender(sender)
})
afterEach(() => {
  setSender(null)
  vi.useRealTimers()
})

it('envia o aceite uma única vez e mostra "Entrando na sala…"', async () => {
  const { wrapper } = await montar()
  expect(sender).toHaveBeenCalledTimes(1)
  expect(sender).toHaveBeenCalledWith({ type: 'invite_accept_token', payload: { token: 'abc' } })
  expect(wrapper.find('h1').text()).toBe('Entrando na sala…')
  useSessionStore().connection = 'open'
  await flushPromises()
  expect(sender).toHaveBeenCalledTimes(1)
})

it('entrando em outra sala pelo convite leva à sala', async () => {
  const { router, wrapper } = await montar()
  const replace = vi.spyOn(router, 'replace')
  useRoomStore().onRoomState(sala('r1'))
  useInvitesStore().onRoomState('r1')
  await flushPromises()
  expect(replace).toHaveBeenCalledWith({ name: 'sala' })
  wrapper.unmount()
})

it('não repete a navegação quando a rota já é a sala', async () => {
  const { router, wrapper } = await montar()
  await router.push({ name: 'sala' })
  const replace = vi.spyOn(router, 'replace')
  useRoomStore().onRoomState(sala('r1'))
  useInvitesStore().onRoomState('r1')
  await flushPromises()
  expect(replace).not.toHaveBeenCalled()
  wrapper.unmount()
})

it.each([
  ['invite_expired', 'Convite expirado', 'Ir para o lobby'],
  ['invite_not_found', 'Convite expirado', 'Ir para o lobby'],
  ['invitee_playing', 'Você está numa partida', 'Voltar para a partida'],
])('erro %s mostra "%s" na página, sem toast', async (code, titulo, botao) => {
  sender.mockRejectedValue(Object.assign(new Error('x'), { code }))
  const { wrapper } = await montar()
  expect(wrapper.find('h1').text()).toBe(titulo)
  expect(wrapper.find('a').text()).toBe(botao)
  expect(useToastsStore().items).toHaveLength(0)
})

it('mostra convite de outra pessoa', async () => {
  const { wrapper } = await montar()
  expect(useInvitesStore().onServerError('invite_not_yours')).toBe(true)
  await flushPromises()
  expect(wrapper.find('h1').text()).toBe('Este convite é para outra pessoa')
  expect(wrapper.find('a').text()).toBe('Ir para o lobby')
  expect(useToastsStore().items).toHaveLength(0)
})

it('erro inesperado vira toast', async () => {
  sender.mockRejectedValue(Object.assign(new Error('Sem rede.'), { code: 'internal' }))
  await montar()
  expect(useToastsStore().items).toHaveLength(1)
})

it('conexão abre depois da montagem: envia ao abrir', async () => {
  await montar('connecting')
  expect(sender).not.toHaveBeenCalled()
  useSessionStore().connection = 'open'
  await flushPromises()
  expect(sender).toHaveBeenCalledTimes(1)
})

it('já sentado na sala do convite: após 3 s ainda pendente, vai para a sala', async () => {
  vi.useFakeTimers()
  useRoomStore().onRoomState(sala('r1'))
  const { router, wrapper } = await montar()
  const replace = vi.spyOn(router, 'replace')
  await vi.advanceTimersByTimeAsync(2900)
  expect(replace).not.toHaveBeenCalled()
  await vi.advanceTimersByTimeAsync(200)
  expect(replace).toHaveBeenCalledWith({ name: 'sala' })
  wrapper.unmount()
})

it('ao sair da página zera o resultado', async () => {
  const { wrapper } = await montar()
  useInvitesStore().onServerError('invite_expired')
  wrapper.unmount()
  expect(useInvitesStore().tokenOutcome).toBeNull()
})
