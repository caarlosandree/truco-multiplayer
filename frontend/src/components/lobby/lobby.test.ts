import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setSender } from '@/app/transport'
import LobbyView from '@/views/LobbyView.vue'
import { useLobbyStore } from '@/stores/lobby'
import { useToastsStore } from '@/stores/toasts'
import Modal from '@/components/ui/Modal.vue'
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
  wrappers.splice(0).forEach((w) => w.unmount())
})

const wrappers: VueWrapper[] = []
function mountLobby() {
  const w = mount(LobbyView, { global: { plugins: [pinia] }, attachTo: document.body })
  wrappers.push(w)
  return w
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

it('sala cheia e em jogo ficam indisponíveis para leitor de tela, sem ações', async () => {
  useLobbyStore().onSnapshot({
    rooms: [
      room('c', { seats: [{ displayName: 'A' }, { displayName: 'B' }, { displayName: 'C' }, { displayName: 'D' }] }),
      room('p', { status: 'playing', seats: [{ displayName: 'A' }, { displayName: 'B' }, { displayName: 'C' }, { displayName: 'D' }], score: [9, 11] }),
    ],
  })
  const w = mountLobby()
  for (const id of ['c', 'p']) {
    const row = w.find(`[data-sala="${id}"]`)
    expect(row.text()).toContain('Indisponível')
    expect(row.find('button').exists()).toBe(false)
  }
  expect(w.find('[data-sala="c"]').text()).toContain('Cheia')
  expect(w.find('[data-sala="p"]').text()).toContain('Em jogo')
  expect(w.find('[data-sala="c"] [data-assento="0"]').text()).toContain('Lugar 1, dupla 1: A')
})

it('ordem de tab igual à visual: lugares e depois "Entrar"', () => {
  useLobbyStore().onSnapshot({ rooms: [room('a')] })
  const w = mountLobby()
  const tabbables = w.findAll('[data-sala="a"] button').map((b) => b.attributes('aria-label'))
  expect(tabbables).toEqual(['Sentar no lugar 2 de Sala a', 'Sentar no lugar 3 de Sala a', 'Sentar no lugar 4 de Sala a', 'Entrar em Sala a'])
})

it('não dispara dois join_room enquanto um está em andamento', async () => {
  let resolve: () => void = () => {}
  const send = vi.fn().mockReturnValue(new Promise<void>((r) => (resolve = r)))
  setSender(send)
  useLobbyStore().onSnapshot({ rooms: [room('a')] })
  const w = mountLobby()
  await w.find('[data-sala="a"] [data-assento="1"]').trigger('click')
  await w.find('[data-sala="a"] [data-assento="2"]').trigger('click')
  expect(send).toHaveBeenCalledTimes(1)
  resolve()
  await flushPromises()
})

it('modal fecha com Esc e com Cancelar; erro de validação leva o foco ao campo', async () => {
  useLobbyStore().onSnapshot({ rooms: [] })
  const w = mountLobby()
  await w.find('[data-abrir="codigo"]').trigger('click')
  await flushPromises()
  const form = document.querySelector('form[data-form="codigo"]') as HTMLFormElement
  form.dispatchEvent(new Event('submit', { cancelable: true }))
  await flushPromises()
  expect(document.activeElement).toBe(form.querySelector('input'))
  dialog()?.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
  await flushPromises()
  // a transição de saída não termina no jsdom: confere o estado aberto do Modal
  const abertos = () => w.findAllComponents(Modal).map((m) => m.props('open'))
  expect(abertos()).toEqual([false, false])
  await w.find('[data-abrir="criar"]').trigger('click')
  await flushPromises()
  const cancelar = [...document.querySelectorAll('[data-criar="public"]')[0]?.closest('[role="dialog"]')?.querySelectorAll('button') ?? []].find((b) => b.textContent.trim() === 'Cancelar')
  cancelar?.click()
  await flushPromises()
  expect(abertos()).toEqual([false, false])
})

it('só uma ação primária: com a lista vazia, o cabeçalho rebaixa "Criar sala"', async () => {
  const w = mountLobby()
  useLobbyStore().onSnapshot({ rooms: [] })
  await w.vm.$nextTick()
  const cls = (sel: string) => w.find(sel).classes().join(' ')
  expect(cls('[data-abrir="criar"]')).toContain('inset-ring-2')
})

it('erro inesperado no código vira toast e deixa o campo limpo', async () => {
  setSender(vi.fn().mockRejectedValue(new Error('boom')))
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
  expect(input.getAttribute('aria-invalid')).toBeNull()
  expect(useToastsStore().items).toHaveLength(1)
})

it('erro ao entrar vira toast', async () => {
  setSender(vi.fn().mockRejectedValue(new WsSendError('room_full', 'x')))
  useLobbyStore().onSnapshot({ rooms: [room('a')] })
  const w = mountLobby()
  await w.find('[data-sala="a"] [data-assento="1"]').trigger('click')
  await flushPromises()
  expect(useToastsStore().items.map((t) => t.text)).toEqual(['A sala está cheia.'])
})
