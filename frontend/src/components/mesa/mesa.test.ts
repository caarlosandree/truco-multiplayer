import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setSender } from '@/app/transport'
import Mesa from './Mesa.vue'
import Carta from './Carta.vue'
import Prazo from './Prazo.vue'
import Placar from './Placar.vue'
import { aplicarEvento, anuncioDoEvento, type Cena } from '@/game/cena'
import { useGameStore } from '@/stores/game'
import { useRoomStore } from '@/stores/room'
import { useSessionStore } from '@/stores/session'
import { makeState } from '@/test/fixtures'
import { defineComponent, inject, onMounted, reactive, type Ref } from 'vue'

const play = vi.fn()
vi.mock('@/audio/install', () => ({ sfx: () => ({ play }) }))

enableAutoUnmount(afterEach)
let pinia: ReturnType<typeof createPinia>
beforeEach(() => {
  play.mockClear()
  pinia = createPinia()
  setActivePinia(pinia)
  useSessionStore().connection = 'open'
  useRoomStore().onRoomState({
    id: 'r', code: 'ABC234', name: 'Sala', visibility: 'public', host: 0, status: 'playing', matchId: 'm',
    seats: [0, 1, 2, 3].map((s) => ({ userId: `u${s}`, displayName: ['Ana', 'Bia', 'Caio', 'Duda'][s]!, ready: true, connected: true, absent: s === 3 })),
    yourSeat: 1,
  })
})
const offsetHeight = Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'offsetHeight')
afterEach(() => {
  setSender(null)
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
  if (offsetHeight) Object.defineProperty(HTMLElement.prototype, 'offsetHeight', offsetHeight)
})

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
  expect(w.find('[data-tipo]').text()).toContain('Mão de Ferro')
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

it('contra-aumento (só com a dupla, sem seat) é anunciado em nós/eles', () => {
  const nome = (s: number) => ['Ana', 'Bia', 'Caio', 'Duda'][s]!
  expect(anuncioDoEvento({ kind: 'raise_requested', team: 1, stake: 6 }, nome, 1)).toBe('Nós pedimos Seis.')
  expect(anuncioDoEvento({ kind: 'raise_requested', team: 0, stake: 6 }, nome, 1)).toBe('Eles pediram Seis.')
  expect(anuncioDoEvento({ kind: 'raise_requested', team: 0, stake: 9 }, nome, null)).toBe('A dupla 1 pediu Nove.')
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

it('com a aba sem foco a vez não toca som: o título pisca "Sua vez!" até focar', async () => {
  vi.useFakeTimers()
  document.title = 'Truco'
  const game = useGameStore()
  game.onState(makeState({ seat: 1, turn: 2 }, [2]))
  vi.spyOn(document, 'hasFocus').mockReturnValue(false)
  const w = mount(Mesa, { global: { plugins: [pinia] } })
  game.onState(makeState({ seat: 1, turn: 1 }, [1]))
  await flushPromises()
  expect(play).not.toHaveBeenCalled()
  vi.advanceTimersByTime(1000)
  expect(document.title).toBe('Sua vez!')
  window.dispatchEvent(new Event('focus'))
  expect(document.title).toBe('Truco')
  // sair da mesa também restaura o título
  game.onState(makeState({ seat: 1, turn: 2 }, [2]))
  await flushPromises()
  game.onState(makeState({ seat: 1, turn: 1 }, [1]))
  await flushPromises()
  vi.advanceTimersByTime(1000)
  expect(document.title).toBe('Sua vez!')
  w.unmount()
  expect(document.title).toBe('Truco')
  vi.useRealTimers()
})

it('a mesa não tem região viva própria: o Anunciador é o único dono dos anúncios de mesa', async () => {
  const game = useGameStore()
  game.onState(makeState({ seat: 1, turn: 2 }, [2]))
  const w = mount(Mesa, { global: { plugins: [pinia] } })
  expect(w.find('[data-fala]').exists()).toBe(false)
  expect(w.findAll('[data-anunciador]')).toHaveLength(1)
  expect(w.find('[data-banner-wo]').exists()).toBe(false)
  expect(w.find('[data-sinais]').exists()).toBe(true)
})

it('no celular a Mesa define --toast-offset (mão + painel); no desktop largo reserva a coluna do painel; limpa ao sair', () => {
  useGameStore().onState(makeState({ seat: 1, turn: 1 }, [1]))
  const q = { celular: true, largo: true }
  vi.stubGlobal('matchMedia', (m: string) => ({
    matches: m.includes('max-width: 767px') ? q.celular : m.includes('min-width: 1024px') ? q.largo : false,
    addEventListener() {},
    removeEventListener() {},
  }))
  Object.defineProperty(HTMLElement.prototype, 'offsetHeight', { configurable: true, get: () => 100 })
  const raiz = document.documentElement.style
  q.celular = true
  q.largo = false
  const celular = mount(Mesa, { global: { plugins: [pinia] } })
  expect(raiz.getPropertyValue('--toast-offset')).toBe('200px')
  expect(raiz.getPropertyValue('--toast-right')).toBe('')
  celular.unmount()
  expect(raiz.getPropertyValue('--toast-offset')).toBe('')
  q.celular = false
  q.largo = true
  const desktop = mount(Mesa, { global: { plugins: [pinia] } })
  expect(raiz.getPropertyValue('--toast-offset')).toBe('')
  expect(raiz.getPropertyValue('--toast-right')).toContain('20rem')
  desktop.unmount()
  expect(raiz.getPropertyValue('--toast-right')).toBe('')
})

it('em awaiting_raise_response quem age vem de pendingSeats, não do turno', () => {
  const resposta = { phase: 'awaiting_raise_response', turn: 1, votingTeam: 0 }
  // eu (assento 1) só espero a dupla 0 responder
  useGameStore().onState(makeState({ seat: 1, ...resposta }, [0, 2]))
  const w = mount(Mesa, { global: { plugins: [pinia] } })
  expect(w.find('[data-vez-de]').text()).toBe('Aguardando Ana e Caio')
  expect(w.find('[data-posicao="bottom"]').attributes('data-vez')).toBeUndefined()
  expect(w.find('[data-posicao="left"]').attributes('data-vez')).toBeDefined()
  expect(w.find('[data-posicao="top"]').attributes('data-vez')).toBeUndefined()
  expect(w.find('[data-vez-de]').text()).not.toContain('Bia')
  w.unmount()
})

it('em awaiting_raise_response, se sou eu quem responde o painel diz "Sua vez" e meu assento fica ativo', () => {
  useGameStore().onState(makeState({ seat: 1, phase: 'awaiting_raise_response', turn: 0, votingTeam: 1 }, [1, 3]))
  const w = mount(Mesa, { global: { plugins: [pinia] } })
  expect(w.find('[data-painel]').text()).toContain('Sua dupla responde')
  expect(w.find('[data-posicao="bottom"]').attributes('data-vez')).toBeDefined()
  expect(w.find('[data-posicao="bottom"]').text()).toContain('Sua vez')
  expect(w.find('[data-posicao="right"]').attributes('data-vez')).toBeUndefined()
  w.unmount()
})

it('o meu nome fica só para leitores de tela e a vira tem rótulo e legenda', () => {
  useGameStore().onState(makeState({ seat: 1, turn: 1, vira: 23, manilha: 11 }, [1]))
  const w = mount(Mesa, { global: { plugins: [pinia] } })
  const eu = w.find('[data-posicao="bottom"]')
  expect(eu.findAll('.sr-only').map((e) => e.text()).join(' ')).toContain('Bia (você)')
  expect(eu.text()).toContain('Você')
  expect(w.text()).toContain('vira · manilha')
  expect(w.findAll('img').some((i) => (i.attributes('alt') ?? '').startsWith('Vira: '))).toBe(true)
  w.unmount()
})

const Coberta = defineComponent({
  setup() {
    const c = inject<Ref<boolean>>('coberta')
    onMounted(() => {
      if (c) c.value = true
    })
    return { c }
  },
  template: '<p data-coberta>{{ c }}</p>',
})

it('"coberta" só desarma quando a minha jogada é aceita', async () => {
  const send = vi.fn().mockResolvedValue(undefined)
  setSender(send)
  const game = useGameStore()
  game.onState(makeState({ seat: 1, turn: 1 }, [1]))
  const w = mount(Mesa, { global: { plugins: [pinia] }, slots: { painel: Coberta } })
  await flushPromises()
  await w.findAll('[data-mao] button')[0]!.trigger('click')
  expect(send).toHaveBeenCalledWith({ type: 'play_card', payload: { slot: 0, covered: true } })
  await flushPromises()
  expect(w.find('[data-coberta]').text()).toBe('true') // servidor ainda não confirmou
  game.onEvents([{ kind: 'card_played', seat: 1 }])
  await flushPromises()
  expect(w.find('[data-coberta]').text()).toBe('false')
  w.unmount()
})

it('reconectando: a mão fica desabilitada e os atalhos não jogam', async () => {
  const send = vi.fn().mockResolvedValue(undefined)
  setSender(send)
  useSessionStore().connection = 'reconnecting'
  useGameStore().onState(makeState({ seat: 1, turn: 1 }, [1]))
  const w = mount(Mesa, { global: { plugins: [pinia] }, attachTo: document.body })
  for (const b of w.findAll('[data-mao] button')) expect(b.attributes('disabled')).toBeDefined()
  expect(w.text()).toContain('Reconectando…')
  window.dispatchEvent(new KeyboardEvent('keydown', { key: '1' }))
  await flushPromises()
  expect(send).not.toHaveBeenCalled()
  useSessionStore().connection = 'open'
  await flushPromises()
  window.dispatchEvent(new KeyboardEvent('keydown', { key: '1' }))
  await flushPromises()
  expect(send).toHaveBeenCalledWith({ type: 'play_card', payload: { slot: 0, covered: false } })
  w.unmount()
})

it('coberta recusada pelo servidor: avisa e desarma a chave', async () => {
  setSender(vi.fn().mockRejectedValue({ code: 'covered_not_allowed', message: 'x' }))
  useGameStore().onState(makeState({ seat: 1, turn: 1, stake: 1 }, [1]))
  const w = mount(Mesa, { global: { plugins: [pinia] }, attachTo: document.body })
  window.dispatchEvent(new KeyboardEvent('keydown', { key: 'c' }))
  await flushPromises()
  expect(w.find('[role="switch"]').attributes('aria-checked')).toBe('true')
  await w.findAll('[data-mao] button')[0]!.trigger('click')
  await flushPromises()
  expect(w.find('[role="switch"]').attributes('aria-checked')).toBe('false')
  w.unmount()
})

it('mão de 11: a mão do parceiro aparece só no painel; no assento seguem os versos', () => {
  useGameStore().onState(
    makeState({ seat: 1, phase: 'eleven_decision', turn: undefined, score: [4, 11], partnerHand: [{ slot: 0, card: 1, hidden: false }] }, [1, 3]),
  )
  const w = mount(Mesa, { global: { plugins: [pinia] } })
  expect(w.findAll('[data-parceiro]')).toHaveLength(1)
  expect(w.find('[data-posicao="top"] [aria-label="Cartas do parceiro"]').exists()).toBe(false)
})

it('placar: tipo da mão completo para leitores de tela e chip curto visível no celular', () => {
  useGameStore().onState(makeState({ seat: 1, turn: 1, handKind: 'eleven', score: [11, 4] }, [1]))
  const w = mount(Placar, { global: { plugins: [pinia] } })
  const tipo = w.find('[data-tipo]')
  expect(tipo.find('span:not([aria-hidden])').text()).toBe('Mão de 11')
  expect(tipo.find('span:not([aria-hidden])').classes()).toContain('max-md:sr-only')
  const curto = tipo.find('[data-tipo-curto]')
  expect(curto.text()).toBe('Mão 11')
  expect(curto.attributes('aria-hidden')).toBe('true')
})

it('Prazo não agenda quadros sem prazo, religa quando ele chega e cancela ao sumir', async () => {
  const raf = vi.spyOn(globalThis, 'requestAnimationFrame')
  const caf = vi.spyOn(globalThis, 'cancelAnimationFrame')
  const w = mount(Prazo, { props: { totalMs: 30_000, mine: false }, global: { plugins: [pinia] } })
  expect(raf).not.toHaveBeenCalled()
  await w.setProps({ deadline: Date.now() + 20_000 })
  expect(raf).toHaveBeenCalled()
  const antes = caf.mock.calls.length
  await w.setProps({ deadline: undefined })
  expect(caf.mock.calls.length).toBeGreaterThan(antes)
  const chamadas = raf.mock.calls.length
  await new Promise((r) => setTimeout(r, 80))
  expect(raf.mock.calls.length).toBe(chamadas)
  w.unmount()
  raf.mockRestore()
  caf.mockRestore()
})
