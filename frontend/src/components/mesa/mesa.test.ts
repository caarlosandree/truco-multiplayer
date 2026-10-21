import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setSender } from '@/app/transport'
import Mesa from './Mesa.vue'
import Carta from './Carta.vue'
import Prazo from './Prazo.vue'
import Placar from './Placar.vue'
import { aplicarEvento, anuncioDoEvento, type Cena } from '@/game/cena'
import { useGameStore } from '@/stores/game'
import { useRoomStore } from '@/stores/room'
import { makeState } from '@/test/fixtures'
import { reactive } from 'vue'

const play = vi.fn()
vi.mock('@/audio/install', () => ({ sfx: () => ({ play }) }))

let pinia: ReturnType<typeof createPinia>
beforeEach(() => {
  play.mockClear()
  pinia = createPinia()
  setActivePinia(pinia)
  useRoomStore().onRoomState({
    id: 'r', code: 'ABC234', name: 'Sala', visibility: 'public', host: 0, status: 'playing', matchId: 'm',
    seats: [0, 1, 2, 3].map((s) => ({ userId: `u${s}`, displayName: ['Ana', 'Bia', 'Caio', 'Duda'][s]!, ready: true, connected: true, absent: s === 3 })),
    yourSeat: 1,
  })
})
afterEach(() => setSender(null))

it('posiciona assentos a partir do meu e marca ausente', () => {
  useGameStore().onState(makeState({ seat: 1, turn: 1 }, [1]))
  const w = mount(Mesa, { global: { plugins: [pinia] } })
  expect(w.find('[data-posicao="bottom"]').text()).toContain('Bia')
  expect(w.find('[data-posicao="top"]').text()).toContain('Duda')
  expect(w.find('[data-posicao="top"]').text()).toContain('Ausente')
  expect(w.find('[data-posicao="left"]').text()).toContain('Caio')
})

it('clicar numa carta da mão joga aquele slot quando é a minha vez', async () => {
  const send = vi.fn().mockResolvedValue(undefined)
  setSender(send)
  useGameStore().onState(makeState({ seat: 1, turn: 1 }, [1]))
  const w = mount(Mesa, { global: { plugins: [pinia] } })
  await w.findAll('[data-mao] button')[2]!.trigger('click')
  expect(send).toHaveBeenCalledWith({ type: 'play_card', payload: { slot: 2, covered: false } })
})

it('fora da vez as cartas da mão ficam desabilitadas', () => {
  useGameStore().onState(makeState({ seat: 1, turn: 2 }, [2]))
  const w = mount(Mesa, { global: { plugins: [pinia] } })
  const buttons = w.findAll('[data-mao] button')
  expect(buttons).toHaveLength(3)
  for (const b of buttons) expect(b.attributes('disabled')).toBeDefined()
})

it('carta coberta e oculta mostram o verso com rótulo', () => {
  const coberta = mount(Carta, { props: { covered: true }, global: { plugins: [pinia] } })
  expect(coberta.find('img').attributes('alt')).toBe('Carta coberta')
  const oculta = mount(Carta, { props: { hidden: true, label: 'Carta 2' }, global: { plugins: [pinia] } })
  expect(oculta.find('img').attributes('alt')).toBe('Carta 2')
})

it('carta da mão é botão com rótulo acessível e a imagem decorativa', () => {
  const w = mount(Carta, { props: { card: 39, interactive: true }, global: { plugins: [pinia] } })
  expect(w.find('button').attributes('aria-label')).toMatch(/de paus/)
  expect(w.find('img').attributes('alt')).toBe('')
})

it('placar mostra nós/eles pelo ponto de vista de quem joga, não pelo assento absoluto', () => {
  useGameStore().onState(makeState({ seat: 1, score: [11, 10] }, [1]))
  const w = mount(Placar, { global: { plugins: [pinia] } })
  // sou o assento 1 (dupla 1): "nós" = 10, "eles" = 11
  expect(w.find('[data-placar-nos]').text()).toBe('10')
  expect(w.find('[data-placar-eles]').text()).toBe('11')
  expect(w.find('[data-valor]').text()).toContain('vale 1')
})

it('mão de ferro: placar indica o tipo e as minhas cartas mostram o verso', () => {
  useGameStore().onState(
    makeState({ seat: 1, turn: 1, handKind: 'iron', myHand: [0, 1, 2].map((slot) => ({ slot, hidden: true })) }, [1]),
  )
  const w = mount(Mesa, { global: { plugins: [pinia] } })
  expect(w.find('[data-tipo]').text()).toBe('Mão de Ferro')
  expect(w.findAll('[data-mao] button').map((b) => b.attributes('aria-label'))).toEqual(['Carta 1', 'Carta 2', 'Carta 3'])
})

it('vaza mostra a carta coberta com o verso e a visível com o rótulo de quem jogou', () => {
  useGameStore().onState(
    makeState({ seat: 1, turn: 1, table: [{ seat: 0, card: 13, covered: false }, { seat: 2, covered: true }] }, [1]),
  )
  const w = mount(Mesa, { global: { plugins: [pinia] } })
  const imgs = w.findAll('[data-vaza] img').map((i) => i.attributes('alt'))
  expect(imgs).toHaveLength(2)
  expect(imgs[0]).toMatch(/^Ana jogou /)
  expect(imgs[1]).toBe('Caio jogou uma carta coberta')
})

it('a jogada da cena aparece na vaza antes do state ser aplicado e some do estado ao chegar o view', async () => {
  const game = useGameStore()
  game.onState(makeState({ seat: 1, turn: 2 }, [2]))
  const w = mount(Mesa, { global: { plugins: [pinia] } })
  expect(w.findAll('[data-vaza] img')).toHaveLength(0)
  game.onEvents([{ kind: 'card_played', seat: 2, card: 13 }])
  await flushPromises()
  expect(w.findAll('[data-vaza] img')).toHaveLength(1)
})

it('cena: eventos adiantam placar, valor e rodada; o view novo zera os adiantamentos', () => {
  const cena = reactive<Cena>({ flights: [], vira: undefined, score: null, stake: null, round: null })
  aplicarEvento(cena, { kind: 'hand_ended', team: 0, stake: 3, score: [3, 0] })
  aplicarEvento(cena, { kind: 'raise_resolved', choice: 'accept', stake: 6 })
  aplicarEvento(cena, { kind: 'raise_resolved', choice: 'run', stake: 9 })
  aplicarEvento(cena, { kind: 'round_ended', team: 1, seat: 3 })
  aplicarEvento(cena, { kind: 'card_played', seat: 3 })
  aplicarEvento(cena, { kind: 'card_played', seat: 3, card: 5 })
  expect(cena).toMatchObject({ score: [3, 0], stake: 6, round: { team: 1 } })
  expect(cena.flights).toEqual([{ seat: 3, card: undefined, covered: true }])
})

it('anúncios da região viva falam em nós/eles', () => {
  const nome = (s: number) => ['Ana', 'Bia', 'Caio', 'Duda'][s]!
  expect(anuncioDoEvento({ kind: 'hand_ended', team: 1, stake: 3 }, nome, 1)).toBe('Mão para nós, mais 3.')
  expect(anuncioDoEvento({ kind: 'hand_ended', team: 0, stake: 1 }, nome, 1)).toBe('Mão para eles, mais 1.')
  expect(anuncioDoEvento({ kind: 'round_ended', team: 0, seat: 2 }, nome, 1)).toBe('Rodada ganha por Caio.')
  expect(anuncioDoEvento({ kind: 'raise_requested', seat: 0, stake: 3 }, nome, 1)).toBe('Ana pediu Truco.')
  expect(anuncioDoEvento({ kind: 'card_played', seat: 0 }, nome, 1)).toBeNull()
})

it('Prazo mostra os segundos e toca timeLow uma vez nos últimos 5 s quando é o meu', async () => {
  const w = mount(Prazo, { props: { deadline: Date.now() + 4_000, totalMs: 30_000, mine: true }, global: { plugins: [pinia] } })
  await vi.waitFor(() => expect(play).toHaveBeenCalledWith('timeLow'))
  await new Promise((r) => setTimeout(r, 80))
  expect(play).toHaveBeenCalledTimes(1)
  expect(w.find('[data-prazo]').attributes('role')).toBe('timer')
  expect(w.find('[data-prazo]').attributes('data-alerta')).toBeDefined()
  w.unmount()
})

it('Prazo de outro jogador não toca o aviso', async () => {
  const w = mount(Prazo, { props: { deadline: Date.now() + 4_000, totalMs: 30_000, mine: false }, global: { plugins: [pinia] } })
  await new Promise((r) => setTimeout(r, 80))
  expect(play).not.toHaveBeenCalled()
  w.unmount()
})

it('Mesa toca "sua vez" quando a vez chega e define --toast-offset só no celular', async () => {
  const game = useGameStore()
  game.onState(makeState({ seat: 1, turn: 2 }, [2]))
  vi.spyOn(document, 'hasFocus').mockReturnValue(true)
  const w = mount(Mesa, { global: { plugins: [pinia] } })
  game.onState(makeState({ seat: 1, turn: 1 }, [1]))
  await flushPromises()
  expect(play).toHaveBeenCalledWith('yourTurn')
  w.unmount()
  expect(document.documentElement.style.getPropertyValue('--toast-offset')).toBe('')
})

it('no celular a Mesa define --toast-offset com a altura de mão + painel e remove ao sair', () => {
  useGameStore().onState(makeState({ seat: 1, turn: 1 }, [1]))
  vi.stubGlobal('matchMedia', (q: string) => ({ matches: q.includes('max-width: 767px'), addEventListener() {}, removeEventListener() {} }))
  const desc = Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'offsetHeight')
  Object.defineProperty(HTMLElement.prototype, 'offsetHeight', { configurable: true, get: () => 100 })
  const w = mount(Mesa, { global: { plugins: [pinia] } })
  expect(document.documentElement.style.getPropertyValue('--toast-offset')).toBe('200px')
  w.unmount()
  expect(document.documentElement.style.getPropertyValue('--toast-offset')).toBe('')
  if (desc) Object.defineProperty(HTMLElement.prototype, 'offsetHeight', desc)
  vi.unstubAllGlobals()
})
