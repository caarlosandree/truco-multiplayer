import { createPinia, setActivePinia } from 'pinia'
import { useRoomStore, setMatchSoundPlayer } from './room'
import { useGameStore } from './game'
import { setSender } from '@/app/transport'
import type { RoomStatePayload } from '@/protocol/messages'

afterEach(() => {
  setSender(null)
  setMatchSoundPlayer(() => {})
})

const state = (over: Partial<RoomStatePayload> = {}): RoomStatePayload => ({
  id: 'r1',
  code: 'ABC234',
  name: 'Sala da Ana',
  visibility: 'private',
  host: 0,
  status: 'waiting',
  seats: [
    { userId: 'u1', displayName: 'Ana', ready: true, connected: true, absent: false, bot: false, botPlaying: false },
    null,
    { userId: 'u3', displayName: 'Caio', ready: false, connected: true, absent: false, bot: false, botPlaying: false },
    null,
  ],
  yourSeat: 0, ranked: null,
  ...over,
})

beforeEach(() => setActivePinia(createPinia()))

it('deriva host, assento, dupla e pronto', () => {
  const r = useRoomStore()
  r.onRoomState(state())
  expect([r.inRoom, r.isHost, r.mySeat, r.myTeam, r.amReady, r.isPlaying]).toEqual([true, true, 0, 0, true, false])
})

it('volta a waiting limpa o jogo; left_room limpa tudo', () => {
  const r = useRoomStore()
  const game = useGameStore()
  const reset = vi.spyOn(game, 'reset')
  r.onRoomState(state({ status: 'playing', matchId: 'm1' }))
  r.onForfeit({ team: 1 })
  r.onRoomState(state({ status: 'waiting' }))
  expect(reset).toHaveBeenCalledOnce()
  expect(r.forfeitTeam).toBeNull()
  r.onLeft({ reason: 'kicked' })
  expect(r.room).toBeNull()
  expect(r.lastLeft).toBe('kicked')
  expect(reset).toHaveBeenCalledTimes(2)
})

it('guarda e descarta o fim de partida', () => {
  const r = useRoomStore()
  r.onMatchEnded({ reason: 'forfeit', winner: 0, score: [12, 4], ranked: true })
  expect(r.ended?.reason).toBe('forfeit')
  r.dismissEnded()
  expect(r.ended).toBeNull()
})

it('envia as ações de sala', async () => {
  const send = vi.fn().mockResolvedValue(undefined)
  setSender(send)
  const r = useRoomStore()
  await r.takeSeat(1)
  await r.setReady(true)
  await r.kick(2)
  await r.claimForfeit()
  await r.leave()
  expect(send.mock.calls.map((c) => c[0])).toEqual([
    { type: 'take_seat', payload: { seat: 1 } },
    { type: 'set_ready', payload: { ready: true } },
    { type: 'kick', payload: { seat: 2 } },
    { type: 'claim_forfeit' },
    { type: 'leave_room' },
  ])
})

it('toca vitória ou derrota no match_ended conforme a dupla vencedora', () => {
  const play = vi.fn()
  setMatchSoundPlayer(play)
  const r = useRoomStore()
  r.onRoomState(state({ status: 'playing', matchId: 'm1' }))
  r.onMatchEnded({ reason: 'game_over', winner: 0, score: [12, 6], ranked: true })
  r.onMatchEnded({ reason: 'game_over', winner: 1, score: [6, 12], ranked: true })
  r.onMatchEnded({ reason: 'aborted', winner: null, score: [0, 0], ranked: true })
  expect(play.mock.calls.map((c) => c[0])).toEqual(['victory', 'defeat'])
})

it('W.O. com vencedor toca o som da dupla vencedora', () => {
  const play = vi.fn()
  setMatchSoundPlayer(play)
  const r = useRoomStore()
  r.onRoomState(state({ status: 'playing', matchId: 'm1' }))
  r.onMatchEnded({ reason: 'forfeit', winner: 1, score: [4, 12], ranked: true })
  expect(play.mock.calls.map((c) => c[0])).toEqual(['defeat'])
})

it('onOutsideRoom limpa sala, desistência, fim de partida e jogo; sem sala não faz nada', () => {
  const r = useRoomStore()
  const reset = vi.spyOn(useGameStore(), 'reset')
  r.onOutsideRoom()
  expect(reset).not.toHaveBeenCalled()
  r.onRoomState(state({ status: 'playing', matchId: 'm1' }))
  r.onForfeit({ team: 1 })
  r.onMatchEnded({ reason: 'game_over', winner: 0, score: [12, 3], ranked: true })
  r.onOutsideRoom()
  expect([r.room, r.forfeitTeam, r.ended, r.lastLeft]).toEqual([null, null, null, null])
  expect(reset).toHaveBeenCalledOnce()
})

describe('convidar para a sala', () => {
  it('freeSeats lista os lugares vazios; canInvite só em espera com lugar livre', () => {
    const r = useRoomStore()
    expect([r.freeSeats, r.canInvite]).toEqual([[], false])
    r.onRoomState(state())
    expect([r.freeSeats, r.canInvite]).toEqual([[1, 3], true])
  })

  it('canInvite é falso com 4 sentados e durante o jogo', () => {
    const r = useRoomStore()
    const seat = (userId: string) => ({ userId, displayName: userId, ready: true, connected: true, absent: false, bot: false, botPlaying: false })
    r.onRoomState(state({ seats: [seat('a'), seat('b'), seat('c'), seat('d')] }))
    expect([r.freeSeats, r.canInvite]).toEqual([[], false])
    r.onRoomState(state({ status: 'playing' }))
    expect([r.freeSeats, r.canInvite]).toEqual([[1, 3], false])
  })
})

describe('bots', () => {
  const seatsBots = (over: { bot?: boolean; botPlaying?: boolean }[] = []) =>
    ['Ana', 'Bia', 'Caio', 'Duda'].map((displayName, i) => ({
      userId: `u${i}`, displayName, ready: true, connected: true, absent: false, bot: false, botPlaying: false, ...over[i],
    }))

  it('startWithBots envia start_with_bots sem payload', async () => {
    const send = vi.fn().mockResolvedValue(undefined)
    setSender(send)
    await useRoomStore().startWithBots()
    expect(send).toHaveBeenCalledWith({ type: 'start_with_bots' })
  })

  it('botPlaying vira true em partida só de humanos: aviso longo e anúncio', async () => {
    const { useToastsStore } = await import('./toasts')
    const r = useRoomStore()
    r.onRoomState(state({ status: 'playing', seats: seatsBots(), ranked: true }))
    expect(r.takeover).toBeNull()
    r.onRoomState(state({ status: 'playing', seats: seatsBots([{}, {}, {}, { botPlaying: true }]), ranked: false }))
    expect(r.takeover?.assentos.map((a) => a.nome)).toEqual(['Duda'])
    expect(useToastsStore().items.at(-1)?.text).toBe(
      'Duda saiu; um bot assumiu. Se a dupla de Duda vencer com o bot no lugar, a partida não conta no ranking — vocês ainda podem pedir W.O.',
    )
  })

  it('parceiro do ausente: só o aviso de ranking, sem a parte do W.O.', async () => {
    const { useToastsStore } = await import('./toasts')
    const r = useRoomStore()
    r.onRoomState(state({ status: 'playing', seats: seatsBots(), ranked: true }))
    r.onRoomState(state({ status: 'playing', seats: seatsBots([{}, {}, { botPlaying: true }]), ranked: false }))
    expect(useToastsStore().items.at(-1)?.text).toBe(
      'Caio saiu; um bot assumiu. Se vocês vencerem com o bot no lugar de Caio, a partida não conta no ranking.',
    )
  })

  it('o próprio humano substituído (conectado e inativo) não recebe aviso sobre si', async () => {
    const { useToastsStore } = await import('./toasts')
    const r = useRoomStore()
    r.onRoomState(state({ status: 'playing', seats: seatsBots(), ranked: true }))
    r.onRoomState(state({ status: 'playing', seats: seatsBots([{ botPlaying: true }]), ranked: false }))
    expect(useToastsStore().items).toHaveLength(0)
    expect(r.takeover?.assentos.map((x) => x.nome)).toEqual(['Ana'])
  })

  it('partida que começou com bot: só "saiu; um bot assumiu."', async () => {
    const { useToastsStore } = await import('./toasts')
    const r = useRoomStore()
    r.onRoomState(state({ status: 'waiting', seats: seatsBots([{}, {}, { bot: true }, { bot: true }]) }))
    r.onRoomState(state({ status: 'playing', seats: seatsBots([{}, {}, { bot: true }, { bot: true }]), ranked: false }))
    r.onRoomState(state({ status: 'playing', seats: seatsBots([{}, { botPlaying: true }, { bot: true }, { bot: true }]), ranked: false }))
    expect(useToastsStore().items.at(-1)?.text).toBe('Bia saiu; um bot assumiu.')
  })

  it('não repete o aviso enquanto o bot segue no lugar nem ao reconectar já com bot jogando', async () => {
    const { useToastsStore } = await import('./toasts')
    const r = useRoomStore()
    const jogando = seatsBots([{}, {}, {}, { botPlaying: true }])
    r.onRoomState(state({ status: 'playing', seats: jogando, ranked: false }))
    r.onRoomState(state({ status: 'playing', seats: jogando, ranked: false }))
    expect(useToastsStore().items).toHaveLength(0)
  })

  it('cai, volta e cai de novo: um aviso só por assento por partida', async () => {
    const { useToastsStore } = await import('./toasts')
    const r = useRoomStore()
    const normal = seatsBots()
    const caido = seatsBots([{}, {}, {}, { botPlaying: true }])
    r.onRoomState(state({ status: 'playing', seats: normal, ranked: true }))
    r.onRoomState(state({ status: 'playing', seats: caido, ranked: false }))
    r.onRoomState(state({ status: 'playing', seats: normal, ranked: true }))
    r.onRoomState(state({ status: 'playing', seats: caido, ranked: false }))
    expect(useToastsStore().items).toHaveLength(1)
    // nova partida: avisa de novo
    r.onRoomState(state({ status: 'waiting', seats: normal }))
    r.onRoomState(state({ status: 'playing', seats: normal, ranked: true }))
    r.onRoomState(state({ status: 'playing', seats: caido, ranked: false }))
    expect(useToastsStore().items).toHaveLength(2)
  })

  it('dois assentos assumidos no mesmo room_state: dois avisos e os dois no lote', async () => {
    const { useToastsStore } = await import('./toasts')
    const r = useRoomStore()
    r.onRoomState(state({ status: 'playing', seats: seatsBots(), ranked: true }))
    r.onRoomState(state({ status: 'playing', seats: seatsBots([{}, { botPlaying: true }, {}, { botPlaying: true }]), ranked: false }))
    expect(useToastsStore().items.map((t) => t.text.split(' saiu')[0])).toEqual(['Bia', 'Duda'])
    expect(r.takeover?.assentos.map((a) => a.nome)).toEqual(['Bia', 'Duda'])
  })

  it('voltar para waiting zera o aviso', () => {
    const r = useRoomStore()
    r.onRoomState(state({ status: 'playing', seats: seatsBots(), ranked: true }))
    r.onRoomState(state({ status: 'playing', seats: seatsBots([{}, {}, {}, { botPlaying: true }]), ranked: false }))
    expect(r.takeover).not.toBeNull()
    r.onRoomState(state({ status: 'waiting', seats: seatsBots() }))
    expect(r.takeover).toBeNull()
  })
})
