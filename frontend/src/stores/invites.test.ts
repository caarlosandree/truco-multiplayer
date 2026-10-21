import { createPinia, setActivePinia } from 'pinia'
import { setSender } from '@/app/transport'
import { FakeSocket } from '@/test/fakeSocket'
import { WsClient, WsSendError } from '@/ws/client'
import type { InviteReceivedPayload, RoomInvite, RoomStatePayload } from '@/protocol/messages'
import type { ServerMessage } from '@/protocol/union'
import { useInvitesStore } from './invites'
import { useRoomStore } from './room'

beforeEach(() => {
  setActivePinia(createPinia())
  setSender(null)
})

const recebido = (inviteId: string, over: Partial<InviteReceivedPayload> = {}): InviteReceivedPayload => ({
  inviteId,
  room: { id: 'r1', name: 'Mesa do RH', visibility: 'private' },
  from: { displayName: 'Ana' },
  freeSeats: 2,
  ...over,
})

it('empilha o mais novo em cima e mostra no máximo 3', () => {
  const s = useInvitesStore()
  for (const id of ['a', 'b', 'c', 'd']) s.onReceived(recebido(id))
  expect(s.visible.map((i) => i.inviteId)).toEqual(['d', 'c', 'b'])
})

it('não duplica aviso reenviado na reconexão', () => {
  const s = useInvitesStore()
  s.onReceived(recebido('a'))
  s.onReceived(recebido('a', { freeSeats: 1 }))
  expect(s.received).toHaveLength(1)
  expect(s.received[0]?.freeSeats).toBe(1)
})

it('responder tira o aviso e envia invite_respond', async () => {
  const send = vi.fn().mockResolvedValue(undefined)
  setSender(send)
  const s = useInvitesStore()
  s.onReceived(recebido('a'))
  const pronto = s.respond('a', true)
  expect(s.received).toHaveLength(0) // some antes de o servidor responder
  await pronto
  expect(send).toHaveBeenCalledWith({ type: 'invite_respond', payload: { inviteId: 'a', accept: true } })
})

it('invite_expired retira o aviso e é idempotente', () => {
  const s = useInvitesStore()
  s.onReceived(recebido('a'))
  s.onExpired({ inviteId: 'a' })
  s.onExpired({ inviteId: 'a' })
  s.onExpired({ inviteId: 'nunca-visto' })
  expect(s.received).toEqual([])
})

it('invite envia o username e o lugar, só quando há lugar', async () => {
  const send = vi.fn().mockResolvedValue(undefined)
  setSender(send)
  const s = useInvitesStore()
  await s.invite('bia')
  await s.invite('caio', 2)
  expect(send.mock.calls.map((c) => c[0])).toEqual([
    { type: 'invite', payload: { username: 'bia' } },
    { type: 'invite', payload: { username: 'caio', seat: 2 } },
  ])
})

it('lista da sala: onRoomInvites e clearRoom', () => {
  const s = useInvitesStore()
  const linha: RoomInvite = { inviteId: 'a', username: 'bia', displayName: 'Bia', state: 'pending', emailed: false }
  s.onRoomInvites({ invites: [linha] })
  expect(s.roomInvites).toEqual([linha])
  s.clearRoom()
  expect(s.roomInvites).toEqual([])
})

it('$reset zera tudo', () => {
  const s = useInvitesStore()
  s.onReceived(recebido('a'))
  s.onRoomInvites({ invites: [{ inviteId: 'a', username: 'b', displayName: 'B', state: 'pending', emailed: true }] })
  s.$reset()
  expect(s.received).toEqual([])
  expect(s.roomInvites).toEqual([])
  expect(s.tokenOutcome).toBeNull()
})

describe('aceite recusado por partida em andamento', () => {
  it('o aviso volta quando o envio é rejeitado com invitee_playing', async () => {
    setSender(vi.fn().mockRejectedValue(new WsSendError('invitee_playing', 'x')))
    const s = useInvitesStore()
    s.onReceived(recebido('a'))
    await expect(s.respond('a', true)).rejects.toMatchObject({ code: 'invitee_playing' })
    expect(s.received.map((i) => i.inviteId)).toEqual(['a'])
  })

  it('o aviso volta quando o erro chega depois de um broadcast (sem seq casando)', async () => {
    setSender(vi.fn().mockResolvedValue(undefined))
    const s = useInvitesStore()
    s.onReceived(recebido('a'))
    await s.respond('a', true)
    expect(s.received).toEqual([])
    expect(s.onServerError('invitee_playing')).toBe(false) // não consome: o toast aparece
    expect(s.received.map((i) => i.inviteId)).toEqual(['a'])
  })

  it('recusar não restaura nada', async () => {
    setSender(vi.fn().mockResolvedValue(undefined))
    const s = useInvitesStore()
    s.onReceived(recebido('a'))
    await s.respond('a', false)
    s.onServerError('invitee_playing')
    expect(s.received).toEqual([])
  })
})

describe('aceite por token', () => {
  it('acceptToken envia invite_accept_token e fica pendente até o servidor decidir', async () => {
    const send = vi.fn().mockResolvedValue(undefined)
    setSender(send)
    const s = useInvitesStore()
    await s.acceptToken('tok')
    expect(send).toHaveBeenCalledWith({ type: 'invite_accept_token', payload: { token: 'tok' } })
    expect(s.tokenOutcome).toBe('pending')
  })

  it.each([
    ['invite_expired', 'expired'],
    ['invite_not_found', 'expired'],
    ['invite_not_yours', 'not_yours'],
    ['invitee_playing', 'playing'],
  ])('erro %s na rejeição do envio vira %s sem lançar', async (code, outcome) => {
    setSender(vi.fn().mockRejectedValue(new WsSendError(code, 'x')))
    const s = useInvitesStore()
    await expect(s.acceptToken('tok')).resolves.toBeUndefined()
    expect(s.tokenOutcome).toBe(outcome)
  })

  it('falha de rede é lançada e não deixa pendente', async () => {
    setSender(vi.fn().mockRejectedValue(new WsSendError('network', 'Sem conexão')))
    const s = useInvitesStore()
    await expect(s.acceptToken('tok')).rejects.toMatchObject({ code: 'network' })
    expect(s.tokenOutcome).toBeNull()
  })

  it('erro que chega pelo caminho sem seq também alimenta o resultado, sem toast', async () => {
    setSender(vi.fn().mockResolvedValue(undefined))
    const s = useInvitesStore()
    await s.acceptToken('tok')
    expect(s.onServerError('invite_not_yours')).toBe(true)
    expect(s.tokenOutcome).toBe('not_yours')
  })

  it('erro de convite fora de um aceite por token não é consumido', () => {
    const s = useInvitesStore()
    expect(s.onServerError('invite_expired')).toBe(false)
    expect(s.tokenOutcome).toBeNull()
  })

  it('room_state durante o aceite por token: deixa de ser pending e vira accepted', async () => {
    setSender(vi.fn().mockResolvedValue(undefined))
    const s = useInvitesStore()
    s.onRoomState('nova') // sem aceite por token em andamento, nada muda
    expect(s.tokenOutcome).toBeNull()
    await s.acceptToken('tok')
    s.onRoomState('nova')
    expect(s.tokenOutcome).toBe('accepted')
    expect(s.onServerError('invite_expired')).toBe(false) // erro tardio não desfaz a entrada
    expect(s.tokenOutcome).toBe('accepted')
  })

  it('room_state da sala em que eu já estava não conta; da sala nova conta; erro tardio ainda é consumido', async () => {
    setSender(vi.fn().mockResolvedValue(undefined))
    useRoomStore().room = { id: 'antiga' } as RoomStatePayload
    const s = useInvitesStore()
    await s.acceptToken('tok')
    s.onRoomState('antiga') // broadcast de outro jogador na sala atual
    expect(s.tokenOutcome).toBe('pending')
    expect(s.onServerError('invitee_playing')).toBe(true)
    expect(s.tokenOutcome).toBe('playing')
    await s.acceptToken('tok2')
    s.onRoomState('nova')
    expect(s.tokenOutcome).toBe('accepted')
  })

  it('resetToken volta a null', async () => {
    setSender(vi.fn().mockResolvedValue(undefined))
    const s = useInvitesStore()
    await s.acceptToken('tok')
    s.resetToken()
    expect(s.tokenOutcome).toBeNull()
  })
})

describe('com o WsClient de verdade: broadcasts entre o envio e o erro', () => {
  const hello: ServerMessage = { type: 'hello', payload: { user: { id: 'u1', username: 'bia', displayName: 'Bia' }, protocolVersion: 1 } }

  function ligar() {
    FakeSocket.instances = []
    const s = useInvitesStore()
    const client = new WsClient({
      url: 'ws://x/ws',
      onMessage: () => {},
      onStatus: () => {},
      onVersionMismatch: () => {},
      onUnackedError: (code) => void s.onServerError(code),
      checkSession: async () => true,
      createSocket: (url) => new FakeSocket(url),
    })
    setSender((m) => client.send(m))
    client.connect()
    FakeSocket.last().open()
    FakeSocket.last().receive(hello)
    return s
  }

  it('presença chega antes do erro com seq: o resultado ainda é not_yours', async () => {
    const s = ligar()
    const envio = s.acceptToken('tok')
    FakeSocket.last().receive({ type: 'presence_updated', payload: { user: { userId: 'u2', username: 'x', displayName: 'X', status: 'lobby' } } })
    await envio // resolvido pelo broadcast
    expect(s.tokenOutcome).toBe('pending')
    FakeSocket.last().receive({ type: 'error', payload: { code: 'invite_not_yours', message: 'x', seq: 1 } })
    expect(s.tokenOutcome).toBe('not_yours')
  })

  it('erro antes de qualquer broadcast: a rejeição do envio alimenta o resultado', async () => {
    const s = ligar()
    const envio = s.acceptToken('tok')
    FakeSocket.last().receive({ type: 'error', payload: { code: 'invite_expired', message: 'x', seq: 1 } })
    await envio
    expect(s.tokenOutcome).toBe('expired')
  })
})
