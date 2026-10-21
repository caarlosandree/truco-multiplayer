import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setSender } from '@/app/transport'
import SalaEspera from './SalaEspera.vue'
import ListaConvites from './ListaConvites.vue'
import { useInvitesStore } from '@/stores/invites'
import { usePlayersStore } from '@/stores/players'
import { usePresenceStore } from '@/stores/presence'
import { useRoomStore } from '@/stores/room'
import { useSessionStore } from '@/stores/session'
import type { PresenceEntry, RoomInvite, RoomStatePayload } from '@/protocol/messages'

const base: RoomStatePayload = {
  id: 'r1', code: 'ABC234', name: 'Sala da Ana', visibility: 'private', host: 0, status: 'waiting',
  seats: [
    { userId: 'id-eu', displayName: 'EU', ready: false, connected: true, absent: false },
    { userId: 'id-bia', displayName: 'BIA', ready: true, connected: true, absent: false },
    null,
    null,
  ],
  yourSeat: 0,
}
const entry = (username: string, over: Partial<PresenceEntry> = {}): PresenceEntry => ({
  userId: `id-${username}`, username, displayName: username.toUpperCase(), status: 'lobby', ...over,
})
const dialog = () => document.querySelector('[role="dialog"]')
const botoes = () => [...document.querySelectorAll<HTMLButtonElement>('[role="dialog"] [data-convidar]')]

let pinia: ReturnType<typeof createPinia>
const wrappers: VueWrapper[] = []
beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
  useSessionStore().user = { id: 'id-eu', username: 'eu', email: 'e@x.com', displayName: 'EU', isAdmin: false, createdAt: '' }
  usePresenceStore().onSnapshot({
    users: [entry('eu'), entry('bia', { status: 'waiting', roomName: 'Sala da Ana' }), entry('caio'), entry('duda', { status: 'playing', roomName: 'Mesa 2' })],
  })
})
afterEach(() => {
  setSender(null)
  wrappers.splice(0).forEach((w) => w.unmount())
  vi.useRealTimers()
  vi.restoreAllMocks()
})

function mountEspera(room: RoomStatePayload = base) {
  useRoomStore().onRoomState(room)
  const w = mount(SalaEspera, { global: { plugins: [pinia] }, attachTo: document.body })
  wrappers.push(w)
  return w
}

it('"Convidar" do topo abre o seletor sem lugar; escolher caio convida sem seat e fecha', async () => {
  const send = vi.fn().mockResolvedValue(undefined)
  setSender(send)
  const w = mountEspera()
  await w.find('[data-acao="convidar"]').trigger('click')
  expect(dialog()?.textContent).toContain('Convidar')
  document.querySelector<HTMLButtonElement>('[data-convidar="caio"]')?.click()
  await flushPromises()
  expect(send).toHaveBeenCalledWith({ type: 'invite', payload: { username: 'caio' } })
  expect(dialog()).toBeNull()
})

it('"Convidar" do lugar vazio 3 abre com seat 3 e envia o lugar', async () => {
  const send = vi.fn().mockResolvedValue(undefined)
  setSender(send)
  const w = mountEspera()
  await w.find('[data-assento="3"] [data-acao="convidar-lugar"]').trigger('click')
  expect(dialog()?.textContent).toContain('lugar 4')
  document.querySelector<HTMLButtonElement>('[data-convidar="caio"]')?.click()
  await flushPromises()
  expect(send).toHaveBeenCalledWith({ type: 'invite', payload: { username: 'caio', seat: 3 } })
})

it('Online omite eu e quem está sentado; quem joga aparece marcado e continua convidável', async () => {
  const w = mountEspera()
  await w.find('[data-acao="convidar"]').trigger('click')
  expect(botoes().map((b) => b.dataset.convidar)).toEqual(['caio', 'duda'])
  const duda = document.querySelector('[data-pessoa="duda"]')
  expect(duda?.textContent).toContain('jogando — pode recusar')
  expect(botoes().find((b) => b.dataset.convidar === 'duda')?.disabled).toBe(false)
})

it('busca em "Outros colegas" usa o diretório com debounce e não duplica quem está online', async () => {
  vi.useFakeTimers()
  const search = vi
    .spyOn(usePlayersStore(), 'searchDirectory')
    .mockResolvedValue([
      { username: 'caio', displayName: 'CAIO' },
      { username: 'eu', displayName: 'EU' },
      { username: 'carla', displayName: 'Carla' },
    ])
  const w = mountEspera()
  await w.find('[data-acao="convidar"]').trigger('click')
  const campo = document.querySelector<HTMLInputElement>('[role="dialog"] input[type="search"]')
  expect(campo).not.toBeNull()
  if (!campo) return
  campo.value = 'c'
  campo.dispatchEvent(new Event('input'))
  campo.value = 'ca'
  campo.dispatchEvent(new Event('input'))
  await vi.advanceTimersByTimeAsync(249)
  expect(search).not.toHaveBeenCalled()
  await vi.advanceTimersByTimeAsync(2)
  await flushPromises()
  expect(search).toHaveBeenCalledTimes(1)
  expect(search).toHaveBeenCalledWith('ca')
  expect(botoes().map((b) => b.dataset.convidar)).toEqual(['caio', 'duda', 'carla'])
})

it('erro na busca vira toast e a sala continua utilizável', async () => {
  vi.useFakeTimers()
  vi.spyOn(usePlayersStore(), 'searchDirectory').mockRejectedValue(Object.assign(new Error('x'), { code: 'network', message: 'Sem conexão com o servidor.' }))
  const w = mountEspera()
  await w.find('[data-acao="convidar"]').trigger('click')
  const campo = document.querySelector<HTMLInputElement>('[role="dialog"] input[type="search"]')
  if (!campo) throw new Error('sem campo')
  campo.value = 'zz'
  campo.dispatchEvent(new Event('input'))
  await vi.advanceTimersByTimeAsync(300)
  await flushPromises()
  expect(document.querySelector('[role="dialog"]')?.textContent).toContain('Não deu pra buscar')
})

it('sem ninguém online mostra o vazio e a busca continua disponível', async () => {
  usePresenceStore().onSnapshot({ users: [entry('eu')] })
  const w = mountEspera()
  await w.find('[data-acao="convidar"]').trigger('click')
  expect(dialog()?.textContent).toContain('Ninguém online agora')
  expect(document.querySelector('[role="dialog"] input[type="search"]')).not.toBeNull()
})

it('erro ao convidar mostra toast, mantém o seletor aberto', async () => {
  setSender(vi.fn().mockRejectedValue(Object.assign(new Error('Essa pessoa já está nesta sala.'), { code: 'already_in_room' })))
  const w = mountEspera()
  await w.find('[data-acao="convidar"]').trigger('click')
  document.querySelector<HTMLButtonElement>('[data-convidar="caio"]')?.click()
  await flushPromises()
  expect(dialog()).not.toBeNull()
})

it('sem canInvite (sala cheia ou em jogo) nenhum botão "Convidar" aparece', () => {
  const cheia: RoomStatePayload = {
    ...base,
    seats: [0, 1, 2, 3].map((i) => ({ userId: `u${i}`, displayName: `P${i}`, ready: false, connected: true, absent: false })),
  }
  const w = mountEspera(cheia)
  expect(w.find('[data-acao="convidar"]').exists()).toBe(false)
  expect(w.find('[data-acao="convidar-lugar"]').exists()).toBe(false)
})

it('ListaConvites mostra aceitou, aguardando…, agora não e e-mail enviado', () => {
  const mk = (inviteId: string, state: RoomInvite['state'], emailed = false, seat?: 2): RoomInvite => ({
    inviteId, username: inviteId, displayName: inviteId.toUpperCase(), state, emailed, ...(seat === undefined ? {} : { seat }),
  })
  useInvitesStore().onRoomInvites({
    invites: [mk('ana', 'accepted'), mk('bia', 'pending', false, 2), mk('caio', 'declined'), mk('duda', 'pending', true)],
  })
  const w = mount(ListaConvites, { global: { plugins: [pinia] } })
  wrappers.push(w)
  const txt = w.text()
  for (const t of ['aceitou', 'aguardando…', 'agora não', 'e-mail enviado', 'lugar 3']) expect(txt).toContain(t)
  expect(w.find('[aria-live]').exists()).toBe(false)
})

it('ListaConvites some quando não há convites', () => {
  const w = mount(ListaConvites, { global: { plugins: [pinia] } })
  wrappers.push(w)
  expect(w.text()).toBe('')
})

it('o foco inicial do seletor vai para o campo de busca, não para o primeiro "Convidar"', async () => {
  const w = mountEspera()
  await w.find('[data-acao="convidar"]').trigger('click')
  await flushPromises()
  expect(document.activeElement?.id).toBe('sel-busca')
})
