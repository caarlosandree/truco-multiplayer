import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setSender } from '@/app/transport'
import LobbyView from '@/views/LobbyView.vue'
import { useLobbyStore } from '@/stores/lobby'
import { useToastsStore } from '@/stores/toasts'
import { WsSendError } from '@/ws/client'

const room = (id: string, over = {}) => ({ id, name: `Sala ${id}`, status: 'waiting', seats: [{ displayName: 'Ana' }, null, null, null], ...over })
const dialog = () => document.querySelector('[role="dialog"]')

let pinia: ReturnType<typeof createPinia>
beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
})
afterEach(() => {
  setSender(null)
  document.body.innerHTML = ''
})

function mountLobby() {
  return mount(LobbyView, { global: { plugins: [pinia] }, attachTo: document.body })
}

it('mostra carregando, depois lista vazia com convite para criar', async () => {
  const w = mountLobby()
  expect(w.text()).toContain('Carregando salas')
  useLobbyStore().onSnapshot({ rooms: [] })
  await w.vm.$nextTick()
  expect(w.text()).toContain('Nenhuma sala aberta')
  expect(w.text()).toContain('Crie uma e chame a turma.')
  const cta = w.findAll('button').filter((b) => b.text() === 'Criar sala')
  expect(cta).toHaveLength(2)
  await cta[1]?.trigger('click')
  expect(dialog()).not.toBeNull()
})

it('clicar num lugar vazio entra naquele assento; sala em jogo não tem ação', async () => {
  const send = vi.fn().mockResolvedValue(undefined)
  setSender(send)
  useLobbyStore().onSnapshot({ rooms: [room('a'), room('b', { status: 'playing', score: [3, 5] })] })
  const w = mountLobby()
  await w.find('[data-sala="a"] [data-assento="2"]').trigger('click')
  expect(send).toHaveBeenCalledWith({ type: 'join_room', payload: { roomId: 'a', seat: 2 } })
  expect(w.find('[data-sala="b"] button').exists()).toBe(false)
  expect(w.find('[data-sala="b"]').text()).toContain('3 × 5')
  expect(w.find('[data-sala="a"]').text()).toContain('Faltam 3 jogadores')
})

it('"Entrar" na linha entra sem escolher lugar', async () => {
  const send = vi.fn().mockResolvedValue(undefined)
  setSender(send)
  useLobbyStore().onSnapshot({ rooms: [room('a')] })
  const w = mountLobby()
  await w.find('[aria-label="Entrar em Sala a"]').trigger('click')
  expect(send).toHaveBeenCalledWith({ type: 'join_room', payload: { roomId: 'a' } })
})

it('entrar com código normaliza e valida; criar sala privada', async () => {
  const send = vi.fn().mockResolvedValue(undefined)
  setSender(send)
  useLobbyStore().onSnapshot({ rooms: [] })
  const w = mountLobby()
  await w.find('[data-abrir="codigo"]').trigger('click')
  await flushPromises()
  const form = () => document.querySelector('form[data-form="codigo"]') as HTMLFormElement
  const input = () => form().querySelector('input') as HTMLInputElement
  form().dispatchEvent(new Event('submit', { cancelable: true }))
  await flushPromises()
  expect(dialog()?.textContent).toContain('Digite o código da sala.')
  input().value = 'abc'
  input().dispatchEvent(new Event('input'))
  form().dispatchEvent(new Event('submit', { cancelable: true }))
  await flushPromises()
  expect(dialog()?.textContent).toContain('O código tem 6 caracteres.')
  input().value = 'abc 234'
  input().dispatchEvent(new Event('input'))
  form().dispatchEvent(new Event('submit', { cancelable: true }))
  await flushPromises()
  expect(send).toHaveBeenLastCalledWith({ type: 'join_room', payload: { code: 'ABC234' } })

  await w.find('[data-abrir="criar"]').trigger('click')
  await flushPromises()
  ;(document.querySelector('[data-criar="private"]') as HTMLElement).click()
  await flushPromises()
  expect(send).toHaveBeenLastCalledWith({ type: 'create_room', payload: { visibility: 'private' } })
})

it('código inexistente aparece junto ao campo, sem toast', async () => {
  setSender(vi.fn().mockRejectedValue(new WsSendError('room_not_found', 'x')))
  useLobbyStore().onSnapshot({ rooms: [] })
  const w = mountLobby()
  await w.find('[data-abrir="codigo"]').trigger('click')
  await flushPromises()
  const form = document.querySelector('form[data-form="codigo"]') as HTMLFormElement
  const input = form.querySelector('input') as HTMLInputElement
  input.value = 'ABC234'
  input.dispatchEvent(new Event('input'))
  form.dispatchEvent(new Event('submit', { cancelable: true }))
  await flushPromises()
  expect(input.getAttribute('aria-invalid')).toBe('true')
  expect(dialog()?.textContent).toContain('Sala não encontrada.')
  expect(useToastsStore().items).toEqual([])
})

it('erro ao entrar vira toast', async () => {
  setSender(vi.fn().mockRejectedValue(new WsSendError('room_full', 'x')))
  useLobbyStore().onSnapshot({ rooms: [room('a')] })
  const w = mountLobby()
  await w.find('[data-sala="a"] [data-assento="1"]').trigger('click')
  await flushPromises()
  expect(useToastsStore().items.map((t) => t.text)).toEqual(['A sala está cheia.'])
})
