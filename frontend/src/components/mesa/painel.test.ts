import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { ref } from 'vue'
import { setSender, type Sender } from '@/app/transport'
import PainelAcao from './PainelAcao.vue'
import { useGameStore } from '@/stores/game'
import { useSessionStore } from '@/stores/session'
import { makeState } from '@/test/fixtures'

let pinia: ReturnType<typeof createPinia>
let send: ReturnType<typeof vi.fn<Sender>>
beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
  useSessionStore().connection = 'open'
  send = vi.fn<Sender>().mockResolvedValue(undefined)
  setSender(send)
})
afterEach(() => setSender(null))

function mountPainel(coberta = ref(false)) {
  return mount(PainelAcao, { global: { plugins: [pinia], provide: { coberta } }, attachTo: document.body })
}

it('corte: escolhe uma das cartas oferecidas', async () => {
  useGameStore().onState(makeState({ phase: 'cut_choice', cutOffer: [{ keep: 0, card: 5 }, { keep: 1, card: 9 }] }))
  const w = mountPainel()
  await w.findAll('[data-corte] button')[1]!.trigger('click')
  expect(send).toHaveBeenCalledWith({ type: 'choose_card', payload: { keep: 1 } })
})

it('queima: seleciona os 4 e confirma, ou passa', async () => {
  useGameStore().onState(makeState({ phase: 'burn_fours', myHand: [{ slot: 0, card: 0, hidden: false }, { slot: 1, card: 3, hidden: false }, { slot: 2, card: 39, hidden: false }] }))
  const w = mountPainel()
  expect(w.findAll('[data-queima] [role="checkbox"]')).toHaveLength(2)
  expect(w.find('[data-acao="queimar"]').attributes('disabled')).toBeDefined()
  await w.findAll('[data-queima] [role="checkbox"]')[0]!.trigger('click')
  expect(w.findAll('[data-queima] [role="checkbox"]')[0]!.attributes('aria-checked')).toBe('true')
  await w.find('[data-acao="queimar"]').trigger('click')
  await flushPromises()
  await w.find('[data-acao="passar"]').trigger('click')
  expect(send.mock.calls.map((c) => c[0])).toEqual([
    { type: 'burn', payload: { slots: [0] } },
    { type: 'pass_burn' },
  ])
})

it('mão de 11: mostra a mão do parceiro e vota', async () => {
  useGameStore().onState(makeState({ phase: 'eleven_decision', score: [11, 4], partnerHand: [{ slot: 0, card: 1, hidden: false }], partnerVote: 'play' }))
  const w = mountPainel()
  expect(w.text()).toContain('Parceiro: Jogo')
  await w.find('[data-voto="run"]').trigger('click')
  expect(send).toHaveBeenCalledWith({ type: 'vote', payload: { choice: 'run' } })
})

it('mão de 11 da outra dupla: só aguarda', () => {
  useGameStore().onState(makeState({ phase: 'eleven_decision', score: [4, 11] }, [1, 3]))
  const w = mountPainel()
  expect(w.find('[data-painel]').exists()).toBe(false)
  expect(w.text()).toContain('Aguardando a dupla com 11 decidir')
})

it('resposta ao aumento: aceitar/correr/aumentar com o próximo valor', async () => {
  const pending = { requester: 1, fromTeam: 1, prevStake: 1, toStake: 3 }
  useGameStore().onState(makeState({ phase: 'awaiting_raise_response', votingTeam: 0, pending }, [0, 2]))
  const w = mountPainel()
  expect(w.find('[data-voto="raise"]').text()).toContain('Seis')
  expect(w.text()).toContain('Bia pediu Truco')
  await w.find('[data-voto="accept"]').trigger('click')
  expect(send).toHaveBeenCalledWith({ type: 'vote', payload: { choice: 'accept' } })
})

it('resposta a doze: não oferece aumentar', () => {
  const pending = { requester: 1, fromTeam: 1, prevStake: 9, toStake: 12 }
  useGameStore().onState(makeState({ phase: 'awaiting_raise_response', votingTeam: 0, pending }, [0, 2]))
  const w = mountPainel()
  expect(w.find('[data-voto="raise"]').exists()).toBe(false)
  expect(w.find('[data-voto="accept"]').exists()).toBe(true)
})

it('hierarquia: uma primária por momento; aposta usa o estilo de aposta', () => {
  const pending = { requester: 1, fromTeam: 1, prevStake: 1, toStake: 3 }
  useGameStore().onState(makeState({ phase: 'awaiting_raise_response', votingTeam: 0, pending }, [0, 2]))
  const w = mountPainel()
  expect(w.findAll('[data-primaria]')).toHaveLength(1)
  expect(w.find('[data-voto="accept"]').attributes('data-primaria')).toBeDefined()
  expect(w.find('[data-voto="raise"]').attributes('data-aposta')).toBeDefined()
  expect(w.find('[data-voto="run"]').attributes('data-primaria')).toBeUndefined()
})

it('jogando: toggle coberta lê e escreve a ref da mesa e não aparece na Mão de Ferro', async () => {
  const coberta = ref(true)
  useGameStore().onState(makeState({ stake: 1 }))
  const w = mountPainel(coberta)
  const toggle = w.find('[role="switch"]')
  expect(toggle.attributes('aria-checked')).toBe('true')
  await toggle.trigger('click')
  expect(coberta.value).toBe(false)
  expect(w.find('[data-acao="aumentar"]').text()).toContain('Truco!')
  useGameStore().onState(makeState({ handKind: 'iron' }))
  await flushPromises()
  expect(w.find('[role="switch"]').exists()).toBe(false)
})

it('pedido na mão de 11 exige confirmação', async () => {
  useGameStore().onState(makeState({ handKind: 'eleven', stake: 3 }))
  const w = mountPainel()
  await w.find('[data-acao="aumentar"]').trigger('click')
  expect(send).not.toHaveBeenCalled()
  expect(document.body.textContent).toContain('faz sua dupla perder a partida')
  ;(document.querySelector('[data-acao="confirmar-aumento"]') as HTMLButtonElement).click()
  await flushPromises()
  expect(send).toHaveBeenCalledWith({ type: 'request_raise' })
  w.unmount()
})

it('fora de decisão não mostra o painel', () => {
  useGameStore().onState(makeState({ turn: 2 }, [2]))
  const w = mountPainel()
  expect(w.find('[data-painel]').exists()).toBe(false)
})

it('pendente sem ação possível diz o que fazer', () => {
  useGameStore().onState(makeState({ phase: 'cut_choice', cutOffer: [] }))
  const w = mountPainel()
  expect(w.find('[data-painel]').exists()).toBe(false)
  expect(w.find('[data-vez-de]').text()).toContain('Sua vez')
})

it('reconectando: painel desabilitado com a razão visível e sem enviar', async () => {
  useSessionStore().connection = 'reconnecting'
  const pending = { requester: 1, fromTeam: 1, prevStake: 1, toStake: 3 }
  useGameStore().onState(makeState({ phase: 'awaiting_raise_response', votingTeam: 0, pending }, [0, 2]))
  const w = mountPainel()
  expect(w.text()).toContain('Reconectando…')
  for (const b of w.findAll('[data-painel] button')) expect(b.attributes('disabled')).toBeDefined()
  await w.find('[data-voto="accept"]').trigger('click')
  expect(send).not.toHaveBeenCalled()
})

it('reconectando na vez: toggle e aumentar desabilitados', () => {
  useSessionStore().connection = 'connecting'
  useGameStore().onState(makeState({ stake: 1 }))
  const w = mountPainel()
  expect(w.text()).toContain('Reconectando…')
  expect(w.find('[role="switch"]').attributes('disabled')).toBeDefined()
  expect(w.find('[data-acao="aumentar"]').attributes('disabled')).toBeDefined()
})

it('outra aba: o texto diz isso, não "Reconectando…"', () => {
  useSessionStore().connection = 'replaced'
  useGameStore().onState(makeState({ stake: 1 }))
  const w = mountPainel()
  expect(w.text()).toContain('Aberto em outra aba')
  expect(w.text()).not.toContain('Reconectando')
  expect(w.find('[data-anuncio]').text()).toBe('')
})

it('região viva única e persistente, sem aria-busy nem role dinâmico', async () => {
  useSessionStore().connection = 'reconnecting'
  const pending = { requester: 1, fromTeam: 1, prevStake: 1, toStake: 3 }
  useGameStore().onState(makeState({ phase: 'awaiting_raise_response', votingTeam: 0, pending }, [0, 2]))
  const w = mountPainel()
  const vivo = w.findAll('[role="status"]')
  expect(vivo).toHaveLength(1)
  expect(vivo[0]!.text()).toBe('') // a conexão é anunciada só pelo IndicadorConexao
  expect(w.find('[aria-busy]').exists()).toBe(false)
  useSessionStore().connection = 'open'
  await flushPromises()
  expect(w.findAll('[role="status"]')).toHaveLength(1)
  // resposta ao aumento e "sua vez" são do Anunciador; o painel só fala das decisões que ele apresenta
  expect(w.find('[data-anuncio]').text()).toBe('')
  useGameStore().onState(makeState({ phase: 'burn_fours', myHand: [{ slot: 0, card: 0, hidden: false }, { slot: 1, card: 3, hidden: false }, { slot: 2, card: 39, hidden: false }] }))
  await flushPromises()
  expect(w.find('[data-anuncio]').text()).toContain('Queima dos 4')
  useGameStore().onState(makeState({ turn: 2 }, [2]))
  await flushPromises()
  expect(w.findAll('[role="status"]')).toHaveLength(1)
  expect(w.find('[data-anuncio]').text()).toBe('')
  w.unmount()
})

it('troca de painel: o foco vai para a ação principal; não rouba de modal nem de campo', async () => {
  const game = useGameStore()
  game.onState(makeState({ stake: 1 }))
  const w = mountPainel()
  const pending = { requester: 1, fromTeam: 1, prevStake: 1, toStake: 3 }
  ;(document.activeElement as HTMLElement | null)?.blur()
  game.onState(makeState({ phase: 'awaiting_raise_response', votingTeam: 0, pending }, [0, 2]))
  await flushPromises()
  expect(document.activeElement).toBe(w.find('[data-voto="accept"]').element)
  // foco num campo de texto: não rouba
  game.onState(makeState({ turn: 2 }, [2]))
  await flushPromises()
  const input = document.createElement('input')
  document.body.append(input)
  input.focus()
  game.onState(makeState({ phase: 'awaiting_raise_response', votingTeam: 0, pending }, [0, 2]))
  await flushPromises()
  expect(document.activeElement).toBe(input)
  input.remove()
  w.unmount()
})

it('corte: o foco vai para a primeira carta oferecida', async () => {
  const game = useGameStore()
  game.onState(makeState({ turn: 2 }, [2]))
  const w = mountPainel()
  ;(document.activeElement as HTMLElement | null)?.blur()
  game.onState(makeState({ phase: 'cut_choice', cutOffer: [{ keep: 0, card: 5 }, { keep: 1, card: 9 }] }))
  await flushPromises()
  expect(document.activeElement).toBe(w.findAll('[data-corte] button')[0]!.element)
  w.unmount()
})

it('queima: marcas somem quando a carta deixa de ser 4 e burn recebe uma cópia', async () => {
  const game = useGameStore()
  game.onState(makeState({ phase: 'burn_fours', myHand: [{ slot: 0, card: 0, hidden: false }, { slot: 1, card: 3, hidden: false }, { slot: 2, card: 39, hidden: false }] }))
  const w = mountPainel()
  await w.findAll('[data-queima] [role="checkbox"]')[1]!.trigger('click')
  game.onState(makeState({ phase: 'burn_fours', myHand: [{ slot: 0, card: 0, hidden: false }, { slot: 1, card: 20, hidden: false }, { slot: 2, card: 39, hidden: false }] }))
  await flushPromises()
  expect(w.find('[data-acao="queimar"]').attributes('disabled')).toBeDefined()
  w.unmount()
})
