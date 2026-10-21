import { createPinia, setActivePinia } from 'pinia'
import { useGameStore } from './game'
import { setSender } from '@/app/transport'
import { makeState } from '@/test/fixtures'
import { WsSendError } from '@/ws/client'

beforeEach(() => {
  setActivePinia(createPinia())
  vi.useFakeTimers()
})
afterEach(() => {
  vi.useRealTimers()
  setSender(null)
})

it('state sem eventos aparece na hora; com eventos, depois da fila', () => {
  const g = useGameStore()
  g.onState(makeState({ handNumber: 1 }))
  expect(g.view?.handNumber).toBe(1)
  g.onEvents([{ kind: 'card_played', seat: 1, card: 4 }])
  g.onState(makeState({ handNumber: 2 }))
  expect(g.view?.handNumber).toBe(1)
  expect(g.latest?.view.handNumber).toBe(2)
  expect(g.scene.event?.kind).toBe('card_played')
  vi.runAllTimers()
  expect(g.view?.handNumber).toBe(2)
  expect(g.scene.event).toBeNull()
})

it('cada instância de Pinia tem o seu diretor', () => {
  const a = useGameStore()
  a.onEvents([{ kind: 'card_played', seat: 1, card: 4 }])
  a.onState(makeState({ handNumber: 2 }))
  expect(a.view).toBeNull()
  setActivePinia(createPinia())
  const b = useGameStore()
  b.onState(makeState({ handNumber: 7 }))
  expect(b.view?.handNumber).toBe(7)
  expect(a.view).toBeNull()
  vi.runAllTimers()
  expect(a.view?.handNumber).toBe(2)
  expect(b.view?.handNumber).toBe(7)
  a.reset()
  expect(a.view).toBeNull()
  expect(b.view?.handNumber).toBe(7)
})

it('getters de jogada e aumento', () => {
  const g = useGameStore()
  g.onState(makeState())
  expect([g.isMyTurn, g.amPending, g.canPlay, g.canCover, g.canRaise, g.nextStakeValue]).toEqual([true, true, true, true, true, 3])
  g.onState(makeState({ lastRaiser: 0, stake: 3 }))
  expect(g.canRaise).toBe(false)
  g.onState(makeState({ stake: 12 }))
  expect(g.canRaise).toBe(false)
  g.onState(makeState({ handKind: 'iron' }))
  expect([g.canCover, g.raiseIsSuicide]).toEqual([false, true])
  g.onState(makeState({ turn: 1 }, [1]))
  expect([g.isMyTurn, g.canPlay, g.canRaise]).toEqual([false, false, false])
})

it('resposta ao aumento só para a dupla que vota e ainda não votou', () => {
  const g = useGameStore()
  const pending = { requester: 1, fromTeam: 1, prevStake: 1, toStake: 3 }
  g.onState(makeState({ phase: 'awaiting_raise_response', votingTeam: 0, pending }, [0, 2]))
  expect([g.mustRespondRaise, g.canRaiseBack]).toEqual([true, true])
  g.onState(makeState({ phase: 'awaiting_raise_response', votingTeam: 0, pending: { ...pending, toStake: 12 } }, [0, 2]))
  expect(g.canRaiseBack).toBe(false)
  g.onState(makeState({ phase: 'awaiting_raise_response', votingTeam: 1, pending }, [1, 3]))
  expect(g.mustRespondRaise).toBe(false)
})

it('fases de corte, queima e mão de 11', () => {
  const g = useGameStore()
  g.onState(makeState({ phase: 'cut_choice', cutOffer: [{ keep: 0, card: 1 }, { keep: 1, card: 2 }] }))
  expect(g.mustChooseCut).toBe(true)
  // cartas 0..3 são os 4 (primeiro degrau do RANK_LADDER)
  g.onState(makeState({ phase: 'burn_fours', myHand: [{ slot: 0, card: 0, hidden: false }, { slot: 1, card: 3, hidden: false }, { slot: 2, card: 39, hidden: false }] }))
  expect([g.mustBurn, g.fourSlots]).toEqual([true, [0, 1]])
  g.onState(makeState({ phase: 'eleven_decision', score: [11, 5] }))
  expect(g.mustDecideEleven).toBe(true)
})

it('enquanto a mesa ainda mostra a mão anterior, nenhuma ação da mão nova é liberada', () => {
  const g = useGameStore()
  // mão 14 terminou com cartas na mão (alguém correu); a 15 é Mão de Ferro, já na minha vez
  g.onState(makeState({ handNumber: 14, phase: 'hand_end', turn: undefined }, []))
  g.onEvents([{ kind: 'hand_started', seat: 3 }, { kind: 'cards_dealt' }, { kind: 'vira_revealed' }])
  const oculta = [0, 1, 2].map((slot) => ({ slot, hidden: true }))
  g.onState(makeState({ handNumber: 15, handKind: 'iron', score: [11, 11], myHand: oculta }))
  expect(g.view?.handNumber).toBe(14)
  expect([g.amPending, g.canPlay, g.canRaise]).toEqual([true, false, false])
  vi.runAllTimers()
  expect(g.view?.handNumber).toBe(15)
  expect(g.canPlay).toBe(true)

  // Mão de 11: a decisão espera a mesa mostrar a minha mão nova
  g.onState(makeState({ handNumber: 15, phase: 'hand_end', turn: undefined }, []))
  g.onEvents([{ kind: 'hand_started', seat: 0 }, { kind: 'cards_dealt' }])
  g.onState(makeState({ handNumber: 16, phase: 'eleven_decision', handKind: 'eleven', score: [11, 5], myHand: [{ slot: 0, card: 7, hidden: false }] }))
  expect(g.mustDecideEleven).toBe(false)
  vi.runAllTimers()
  expect(g.mustDecideEleven).toBe(true)
})

it('dentro da mesma mão, a fila de cenas não trava a vez (a minha mão não mudou)', () => {
  const g = useGameStore()
  g.onState(makeState({ turn: 3 }, [3]))
  g.onEvents([{ kind: 'card_played', seat: 3, card: 20 }])
  g.onState(makeState({ table: [{ seat: 3, card: 20, covered: false }] }))
  expect(g.view?.turn).toBe(3)
  expect(g.canPlay).toBe(true)
})

it('envia ações de jogo com os payloads do protocolo', async () => {
  const send = vi.fn().mockResolvedValue(undefined)
  setSender(send)
  const g = useGameStore()
  g.onState(makeState())
  await g.playCard(1, true)
  await g.requestRaise()
  await g.vote('accept')
  await g.chooseCard(1)
  await g.burn([0, 2])
  await g.passBurn()
  expect(send.mock.calls.map((c) => c[0])).toEqual([
    { type: 'play_card', payload: { slot: 1, covered: true } },
    { type: 'request_raise' },
    { type: 'vote', payload: { choice: 'accept' } },
    { type: 'choose_card', payload: { keep: 1 } },
    { type: 'burn', payload: { slots: [0, 2] } },
    { type: 'pass_burn' },
  ])
})

it('ignora segunda jogada enquanto a primeira está em voo', async () => {
  let release!: () => void
  const send = vi.fn(() => new Promise<void>((r) => (release = r)))
  setSender(send)
  const g = useGameStore()
  g.onState(makeState())
  const first = g.playCard(0, false)
  await g.playCard(0, false)
  expect(send).toHaveBeenCalledOnce()
  release()
  await first
  expect(g.inflight).toBe(false)
})

it('erro de regra vira toast e destrava', async () => {
  setSender(vi.fn().mockRejectedValue(new WsSendError('not_your_turn', 'x')))
  const g = useGameStore()
  g.onState(makeState())
  await g.playCard(0, false)
  expect(g.inflight).toBe(false)
  const { useToastsStore } = await import('./toasts')
  expect(useToastsStore().items.map((t) => t.text)).toEqual(['Não é a sua vez.'])
})

it('sinais: balão expira e envio respeita 3 s', async () => {
  const send = vi.fn().mockResolvedValue(undefined)
  setSender(send)
  const g = useGameStore()
  g.onSignal({ seat: 2, id: 'boa' })
  expect(g.signals[2]?.id).toBe('boa')
  vi.advanceTimersByTime(3000)
  expect(g.signals[2]).toBeUndefined()
  await g.sendSignal('valeu')
  await g.sendSignal('valeu')
  expect(send).toHaveBeenCalledOnce()
  vi.advanceTimersByTime(3000)
  await g.sendSignal('valeu')
  expect(send).toHaveBeenCalledTimes(2)
})
