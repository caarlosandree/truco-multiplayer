import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setSender, type Sender } from '@/app/transport'
import BarraSinais from './BarraSinais.vue'
import BalaoSinal from './BalaoSinal.vue'
import BannerWO from './BannerWO.vue'
import FimPartida from './FimPartida.vue'
import Anunciador from './Anunciador.vue'
import { useGameStore } from '@/stores/game'
import { useRoomStore } from '@/stores/room'
import { useSessionStore } from '@/stores/session'
import { useToastsStore } from '@/stores/toasts'
import { makeState } from '@/test/fixtures'

enableAutoUnmount(afterEach)
let pinia: ReturnType<typeof createPinia>
let send: ReturnType<typeof vi.fn<Sender>>
const seats = (n: number) =>
  [0, 1, 2, 3].map((s) => ({ userId: `u${s}`, displayName: ['Ana', 'Bia', 'Caio', 'Duda'][s]!, ready: true, connected: true, absent: false, n }))

function roomState(over: Partial<Parameters<ReturnType<typeof useRoomStore>['onRoomState']>[0]> = {}) {
  return {
    id: 'r', code: 'ABC234', name: 'Sala', visibility: 'public' as const, host: 0, status: 'playing' as const, matchId: 'm',
    seats: seats(0), yourSeat: 0, ...over,
  }
}

beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
  useSessionStore().connection = 'open'
  send = vi.fn<Sender>().mockResolvedValue(undefined)
  setSender(send)
  useRoomStore().onRoomState(roomState())
})
afterEach(() => {
  setSender(null)
  vi.useRealTimers()
  document.body.innerHTML = ''
})

describe('BarraSinais', () => {
  it('envia o sinal e todos os botões entram em espera por 3 s', async () => {
    vi.useFakeTimers()
    const w = mount(BarraSinais, { global: { plugins: [pinia] } })
    expect(w.findAll('[data-sinal]')).toHaveLength(6)
    await w.find('[data-sinal="boa"]').trigger('click')
    expect(send).toHaveBeenCalledWith({ type: 'signal', payload: { id: 'boa' } })
    await w.vm.$nextTick()
    for (const b of w.findAll('[data-sinal]')) expect(b.attributes('disabled')).toBeDefined()
    expect(w.text()).toMatch(/3 s|2 s/)
    await vi.advanceTimersByTimeAsync(3100)
    for (const b of w.findAll('[data-sinal]')) expect(b.attributes('disabled')).toBeUndefined()
  })

  it('sem conexão os sinais ficam desabilitados', () => {
    useSessionStore().connection = 'reconnecting'
    const w = mount(BarraSinais, { global: { plugins: [pinia] } })
    expect(w.find('[data-sinal="boa"]').attributes('disabled')).toBeDefined()
  })

  it('no celular o botão "Sinais" abre e fecha a folha; Esc fecha', async () => {
    const w = mount(BarraSinais, { global: { plugins: [pinia] }, attachTo: document.body })
    const toggle = w.find('[data-sinais-toggle]')
    expect(toggle.attributes('aria-expanded')).toBe('false')
    await toggle.trigger('click')
    expect(toggle.attributes('aria-expanded')).toBe('true')
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await w.vm.$nextTick()
    expect(toggle.attributes('aria-expanded')).toBe('false')
  })

  it('Esc com um modal aberto por cima não fecha a folha', async () => {
    const w = mount(BarraSinais, { global: { plugins: [pinia] }, attachTo: document.body })
    await w.find('[data-sinais-toggle]').trigger('click')
    const m = document.createElement('div')
    m.setAttribute('aria-modal', 'true')
    document.body.append(m)
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await w.vm.$nextTick()
    expect(w.find('[data-sinais-toggle]').attributes('aria-expanded')).toBe('true')
    m.remove()
  })

  it('erro de envio vira toast traduzido', async () => {
    send.mockRejectedValueOnce({ code: 'rate_limited', message: 'x' })
    const w = mount(BarraSinais, { global: { plugins: [pinia] } })
    await w.find('[data-sinal="boa"]').trigger('click')
    await flushPromises()
    expect(useToastsStore().items[0]?.text).toBe('Calma! Espere um pouco antes de repetir.')
  })
})

describe('BalaoSinal', () => {
  it('mostra o sinal do assento e some depois de ~3 s', async () => {
    vi.useFakeTimers()
    const game = useGameStore()
    const w = mount(BalaoSinal, { props: { seat: 1, position: 'left' }, global: { plugins: [pinia] } })
    expect(w.text()).toBe('')
    game.onSignal({ seat: 1, id: 'truco' })
    await w.vm.$nextTick()
    expect(w.text()).toBe('Truco!')
    await vi.advanceTimersByTimeAsync(3100)
    expect(w.text()).toBe('')
  })
})

describe('BannerWO', () => {
  it('só aparece contra a dupla adversária e pede o W.O.', async () => {
    const room = useRoomStore()
    const w = mount(BannerWO, { global: { plugins: [pinia] } })
    expect(w.text()).toBe('')
    room.onForfeit({ team: 0 })
    await w.vm.$nextTick()
    expect(w.text()).toBe('')
    room.onForfeit({ team: 1 })
    await w.vm.$nextTick()
    expect(w.text()).toContain('Adversários ausentes')
    await w.find('button').trigger('click')
    expect(send).toHaveBeenCalledWith({ type: 'claim_forfeit' })
  })

  it('some quando a ausência acaba (team nulo)', async () => {
    const room = useRoomStore()
    room.onForfeit({ team: 1 })
    const w = mount(BannerWO, { global: { plugins: [pinia] } })
    expect(w.text()).toContain('Adversários ausentes')
    room.forfeitTeam = null
    await w.vm.$nextTick()
    expect(w.text()).toBe('')
  })

  it('falha no pedido vira toast', async () => {
    send.mockRejectedValueOnce({ code: 'forfeit_unavailable', message: 'x' })
    const room = useRoomStore()
    room.onForfeit({ team: 1 })
    const w = mount(BannerWO, { global: { plugins: [pinia] } })
    await w.find('button').trigger('click')
    await flushPromises()
    expect(useToastsStore().items[0]?.text).toBe('O W.O. não está disponível agora.')
  })
})

describe('FimPartida', () => {
  const abrir = () => mount(FimPartida, { global: { plugins: [pinia] }, attachTo: document.body })
  const corpo = () => document.body.textContent ?? ''

  it('vitória por W.O.: resultado, motivo neutro e placar da minha dupla', async () => {
    useRoomStore().onMatchEnded({ reason: 'forfeit', winner: 0, score: [12, 7] })
    abrir()
    await flushPromises()
    expect(corpo()).toContain('Vitória')
    expect(corpo()).toContain('Encerrada por W.O.')
    expect(document.querySelector('[data-placar-final]')?.textContent).toContain('Nós 12')
    expect(document.querySelector('[data-placar-final]')?.textContent).toContain('Eles 7')
  })

  it('derrota: o placar é relativo à minha dupla', async () => {
    useRoomStore().onRoomState(roomState({ yourSeat: 1 }))
    useRoomStore().onMatchEnded({ reason: 'game_over', winner: 0, score: [12, 7] })
    abrir()
    await flushPromises()
    expect(corpo()).toContain('Derrota')
    expect(document.querySelector('[data-placar-final]')?.textContent).toContain('Nós 7')
    expect(document.querySelector('[data-placar-final]')?.textContent).toContain('Eles 12')
  })

  it.each([
    ['aborted', 'Partida abortada pelo administrador'],
    ['abandoned', 'Partida abandonada'],
  ] as const)('sem vencedor (%s)', async (reason, texto) => {
    useRoomStore().onMatchEnded({ reason, winner: null, score: [3, 2] })
    abrir()
    await flushPromises()
    expect(corpo()).toContain(texto)
    expect(corpo()).toContain('Sem vencedor')
  })

  it('o foco inicial vai para "Revanche" (ação principal), não para "Sair da sala"', async () => {
    useRoomStore().onMatchEnded({ reason: 'game_over', winner: 0, score: [12, 7] })
    abrir()
    await flushPromises()
    expect(document.activeElement).toBe(document.querySelector('[data-acao="revanche"]'))
  })

  it('o diálogo é descrito pelo resultado e o foco entra nele', async () => {
    useRoomStore().onMatchEnded({ reason: 'game_over', winner: 0, score: [12, 7] })
    abrir()
    await flushPromises()
    const dlg = document.querySelector('[role="dialog"]')
    const desc = dlg?.getAttribute('aria-describedby')
    expect(desc).toBeTruthy()
    expect(document.getElementById(desc ?? '')?.textContent).toContain('Vitória')
    expect(dlg?.contains(document.activeElement)).toBe(true)
  })

  it('revanche fecha o modal e a sala de espera continua; o modal antigo não volta', async () => {
    const room = useRoomStore()
    room.onMatchEnded({ reason: 'game_over', winner: 0, score: [12, 7] })
    // o servidor volta a sala para "waiting" com todos sem pronto
    room.onRoomState(roomState({ status: 'waiting', seats: seats(1).map((s) => ({ ...s, ready: false })) }))
    abrir()
    await flushPromises()
    ;(document.querySelector('[data-acao="revanche"]') as HTMLButtonElement).click()
    await flushPromises()
    expect(room.ended).toBeNull()
    expect(document.querySelector('[role="dialog"]')).toBeNull()
    expect(room.isPlaying).toBe(false)
    expect(room.amReady).toBe(false)
    // nova partida e novo fim: o modal aparece de novo, uma vez só
    room.onRoomState(roomState())
    room.onMatchEnded({ reason: 'game_over', winner: 1, score: [3, 12] })
    await flushPromises()
    expect(document.querySelectorAll('[role="dialog"]')).toHaveLength(1)
    expect(corpo()).toContain('Derrota')
  })

  it('sair da sala: envia leave_room e fecha; erro vira toast e mantém o modal', async () => {
    const room = useRoomStore()
    room.onMatchEnded({ reason: 'game_over', winner: 0, score: [12, 7] })
    abrir()
    await flushPromises()
    send.mockRejectedValueOnce({ code: 'internal', message: 'x' })
    ;(document.querySelector('[data-acao="sair"]') as HTMLButtonElement).click()
    await flushPromises()
    expect(useToastsStore().items[0]?.text).toBe('Erro no servidor, tente de novo.')
    expect(room.ended).not.toBeNull()
    ;(document.querySelector('[data-acao="sair"]') as HTMLButtonElement).click()
    await flushPromises()
    expect(send).toHaveBeenLastCalledWith({ type: 'leave_room' })
    expect(room.ended).toBeNull()
  })

  it('left_room (expulso, abandono) limpa o fim de partida antigo', () => {
    const room = useRoomStore()
    room.onMatchEnded({ reason: 'game_over', winner: 0, score: [12, 7] })
    room.onLeft({ reason: 'kicked' })
    expect(room.ended).toBeNull()
  })

  it('entrar em outra sala não traz o fim de partida antigo de volta', () => {
    const room = useRoomStore()
    room.onMatchEnded({ reason: 'game_over', winner: 0, score: [12, 7] })
    room.onRoomState(roomState({ id: 'outra', status: 'waiting' }))
    expect(room.ended).toBeNull()
  })
})

describe('Anunciador (dono dos anúncios da mesa)', () => {
  const regiao = (w: ReturnType<typeof mount>) => w.find('[aria-live="polite"]')

  it('lê pedidos de aumento e cenas do diretor', async () => {
    const game = useGameStore()
    game.onState(makeState())
    const w = mount(Anunciador, { global: { plugins: [pinia] } })
    game.onEvents([{ kind: 'raise_requested', seat: 1, stake: 3 }])
    await w.vm.$nextTick()
    expect(regiao(w).text()).toContain('Bia pediu Truco')
  })

  it('anuncia "Sua vez" quando passo a agir, uma vez por transição', async () => {
    const game = useGameStore()
    game.onState(makeState({ seat: 0, turn: 1 }, [1]))
    const w = mount(Anunciador, { global: { plugins: [pinia] } })
    expect(regiao(w).text()).toBe('')
    game.onState(makeState({ seat: 0, turn: 0 }, [0]))
    await w.vm.$nextTick()
    expect(regiao(w).text()).toBe('Sua vez')
    game.onState(makeState({ seat: 0, turn: 0 }, [0]))
    await w.vm.$nextTick()
    expect(regiao(w).findAll('p')).toHaveLength(1)
  })

  it('o mesmo texto de novo é um novo anúncio (itens acrescentados, não reescritos)', async () => {
    const game = useGameStore()
    game.onState(makeState({ seat: 0, turn: 1 }, [1]))
    const w = mount(Anunciador, { global: { plugins: [pinia] } })
    for (let i = 0; i < 2; i++) {
      game.onState(makeState({ seat: 0, turn: 0 }, [0]))
      await w.vm.$nextTick()
      game.onState(makeState({ seat: 0, turn: 1 }, [1]))
      await w.vm.$nextTick()
    }
    expect(regiao(w).findAll('p').map((p) => p.text())).toEqual(['Sua vez', 'Sua vez'])
    expect(regiao(w).attributes('aria-relevant')).toBe('additions')
  })

  it('montar a mesa já na minha vez anuncia "Sua vez" (reconexão)', () => {
    const game = useGameStore()
    game.onState(makeState({ seat: 0, turn: 0 }, [0]))
    const w = mount(Anunciador, { global: { plugins: [pinia] } })
    expect(regiao(w).text()).toBe('Sua vez')
  })

  it('anuncia sinais dos outros (não os meus) e o W.O. disponível', async () => {
    const game = useGameStore()
    game.onState(makeState({ seat: 0 }))
    const w = mount(Anunciador, { global: { plugins: [pinia] } })
    game.onSignal({ seat: 2, id: 'vamos' })
    game.onSignal({ seat: 0, id: 'boa' })
    useRoomStore().onForfeit({ team: 1 })
    await w.vm.$nextTick()
    const t = regiao(w).text()
    expect(t).toContain('Caio: Vamos!')
    expect(t).not.toContain('Boa!')
    expect(t).toContain('Adversários ausentes')
  })

  it('as mensagens antigas saem da região depois de um tempo', async () => {
    vi.useFakeTimers()
    const game = useGameStore()
    game.onState(makeState({ seat: 0 }))
    const w = mount(Anunciador, { global: { plugins: [pinia] } })
    game.onEvents([{ kind: 'vira_revealed' }])
    await w.vm.$nextTick()
    expect(regiao(w).text()).toContain('Vira')
    await vi.advanceTimersByTimeAsync(9000)
    expect(regiao(w).text()).toBe('')
  })
})

it('BarraSinais: fechar a folha devolve o foco ao botão "Sinais"', async () => {
  const w = mount(BarraSinais, { global: { plugins: [pinia] }, attachTo: document.body })
  await w.find('[data-sinais-toggle]').trigger('click')
  ;(w.find('[data-sinal="boa"]').element as HTMLButtonElement).focus()
  await w.find('[data-sinal="boa"]').trigger('click')
  await flushPromises()
  expect(document.activeElement).toBe(w.find('[data-sinais-toggle]').element)
})
