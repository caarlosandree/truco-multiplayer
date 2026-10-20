import { FakeSocket } from '@/test/fakeSocket'
import { backoffDelay, WsClient, WsSendError, wsUrl, type WsClientOptions, type WsStatus } from './client'
import type { ServerMessage } from '@/protocol/union'

const hello: ServerMessage = { type: 'hello', payload: { user: { id: 'u1', username: 'ana', displayName: 'Ana' }, protocolVersion: 1 } }

function setup(over: Partial<WsClientOptions> = {}) {
  FakeSocket.instances = []
  const received: ServerMessage[] = []
  const statuses: WsStatus[] = []
  const unacked: string[] = []
  const versions: number[] = []
  const client = new WsClient({
    url: 'ws://x/ws',
    onMessage: (m) => received.push(m),
    onStatus: (s) => statuses.push(s),
    onVersionMismatch: (v) => versions.push(v),
    onUnackedError: (code) => unacked.push(code),
    checkSession: async () => true,
    createSocket: (url) => new FakeSocket(url),
    setTimer: (fn, ms) => setTimeout(fn, ms),
    clearTimer: (h) => clearTimeout(h as ReturnType<typeof setTimeout>),
    random: () => 0.5,
    ...over,
  })
  return { client, received, statuses, unacked, versions }
}

beforeEach(() => vi.useFakeTimers())
afterEach(() => vi.useRealTimers())

it('monta a URL ws/wss a partir da página', () => {
  expect(wsUrl({ protocol: 'https:', host: 'truco.empresa.com' })).toBe('wss://truco.empresa.com/ws')
  expect(wsUrl({ protocol: 'http:', host: 'localhost:5173' })).toBe('ws://localhost:5173/ws')
})

it('roteia mensagens e reporta status', () => {
  const { client, received, statuses } = setup()
  client.connect()
  FakeSocket.last().open()
  FakeSocket.last().receive(hello)
  FakeSocket.last().receive('não é json')
  expect(received).toEqual([hello])
  expect(statuses).toEqual(['connecting', 'open'])
})

it('avisa quando a versão do protocolo difere', () => {
  const { client, versions } = setup()
  client.connect()
  FakeSocket.last().open()
  FakeSocket.last().receive({ ...hello, payload: { ...hello.payload, protocolVersion: 2 } })
  expect(versions).toEqual([2])
})

it('numera envios e rejeita o que voltar com error{seq}', async () => {
  const { client } = setup()
  client.connect()
  FakeSocket.last().open()
  FakeSocket.last().receive(hello)
  const first = client.send({ type: 'set_ready', payload: { ready: true } })
  const second = client.send({ type: 'leave_room' })
  expect(FakeSocket.last().sentMessages()).toEqual([
    { type: 'set_ready', seq: 1, payload: { ready: true } },
    { type: 'leave_room', seq: 2 },
  ])
  FakeSocket.last().receive({ type: 'error', payload: { code: 'not_host', message: 'x', seq: 1 } })
  await expect(first).rejects.toEqual(new WsSendError('not_host', 'x'))
  FakeSocket.last().receive({ type: 'left_room', payload: { reason: 'left' } })
  await expect(second).resolves.toBeUndefined()
})

it('resolve envios sem resposta após 5 s e manda erro sem seq conhecido para o toast', async () => {
  const { client, unacked } = setup()
  client.connect()
  FakeSocket.last().open()
  FakeSocket.last().receive(hello)
  const p = client.send({ type: 'signal', payload: { id: 'boa' } })
  vi.advanceTimersByTime(5000)
  await expect(p).resolves.toBeUndefined()
  FakeSocket.last().receive({ type: 'error', payload: { code: 'rate_limited', message: 'x', seq: 99 } })
  expect(unacked).toEqual(['rate_limited'])
})

it('rejeita envio com a conexão fechada', async () => {
  const { client } = setup()
  await expect(client.send({ type: 'leave_room' })).rejects.toMatchObject({ code: 'network' })
})

it('reconecta com backoff e zera as tentativas no hello', async () => {
  const { client, statuses } = setup()
  client.connect()
  FakeSocket.last().open()
  FakeSocket.last().receive(hello)
  FakeSocket.last().serverClose(1006)
  await vi.advanceTimersByTimeAsync(0)
  expect(statuses.at(-1)).toBe('reconnecting')
  expect(FakeSocket.instances).toHaveLength(1)
  await vi.advanceTimersByTimeAsync(backoffDelay(0, () => 0.5))
  expect(FakeSocket.instances).toHaveLength(2)
  // segunda queda depois de um hello: o atraso tem de voltar ao da tentativa 0
  FakeSocket.last().open()
  FakeSocket.last().receive(hello)
  FakeSocket.last().serverClose(1006)
  await vi.advanceTimersByTimeAsync(0)
  await vi.advanceTimersByTimeAsync(backoffDelay(0, () => 0.5))
  expect(FakeSocket.instances).toHaveLength(3)
})

it('connect() repetido não abre um segundo socket', () => {
  const { client, statuses } = setup()
  client.connect()
  FakeSocket.last().open()
  FakeSocket.last().receive(hello)
  client.connect()
  expect(FakeSocket.instances).toHaveLength(1)
  expect(statuses).toEqual(['connecting', 'open'])
})

it('connect() depois de disconnect() começa do atraso base', async () => {
  const { client, statuses } = setup()
  client.connect()
  FakeSocket.last().open()
  FakeSocket.last().receive(hello)
  FakeSocket.last().serverClose(1006)
  await vi.advanceTimersByTimeAsync(backoffDelay(0, () => 0.5))
  client.disconnect()
  client.connect()
  expect(statuses.at(-1)).toBe('connecting')
  FakeSocket.last().serverClose(1006) // cai antes do hello
  await vi.advanceTimersByTimeAsync(0)
  await vi.advanceTimersByTimeAsync(backoffDelay(0, () => 0.5))
  expect(FakeSocket.instances).toHaveLength(4)
})

it('checkSession que rejeita não quebra a reconexão (falha aberta)', async () => {
  const checkSession = vi.fn().mockRejectedValue(new Error('rede caiu'))
  const { client, statuses } = setup({ checkSession })
  client.connect()
  FakeSocket.last().serverClose(1006) // cai antes do hello
  await vi.advanceTimersByTimeAsync(0)
  expect(checkSession).toHaveBeenCalledOnce()
  expect(statuses.at(-1)).toBe('reconnecting')
  await vi.advanceTimersByTimeAsync(backoffDelay(0, () => 0.5))
  expect(FakeSocket.instances).toHaveLength(2)
})

it('código 4008 gera aviso em pt-BR e reconecta', async () => {
  const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
  const { client } = setup()
  client.connect()
  FakeSocket.last().open()
  FakeSocket.last().receive(hello)
  FakeSocket.last().serverClose(4008)
  await vi.advanceTimersByTimeAsync(0)
  expect(warn).toHaveBeenCalledWith(expect.stringContaining('4008'))
  await vi.advanceTimersByTimeAsync(backoffDelay(0, () => 0.5))
  expect(FakeSocket.instances).toHaveLength(2)
  warn.mockRestore()
})

it('não reconecta quando substituída por outra aba (4001) e retoma com resume()', async () => {
  const { client, statuses } = setup()
  client.connect()
  FakeSocket.last().open()
  FakeSocket.last().receive(hello)
  FakeSocket.last().serverClose(4001)
  await vi.advanceTimersByTimeAsync(60_000)
  expect(statuses.at(-1)).toBe('replaced')
  expect(FakeSocket.instances).toHaveLength(1)
  client.resume()
  expect(FakeSocket.instances).toHaveLength(2)
})

it('sessão inválida (4003) vira unauthorized sem reconectar', async () => {
  const { client, statuses } = setup()
  client.connect()
  FakeSocket.last().open()
  FakeSocket.last().receive(hello)
  FakeSocket.last().serverClose(4003)
  await vi.advanceTimersByTimeAsync(60_000)
  expect(statuses.at(-1)).toBe('unauthorized')
  expect(FakeSocket.instances).toHaveLength(1)
})

it('para de reconectar e sinaliza unauthorized quando a sessão caiu antes do hello', async () => {
  const checkSession = vi.fn().mockResolvedValue(false)
  const { client, statuses } = setup({ checkSession })
  client.connect()
  FakeSocket.last().serverClose(1006) // upgrade recusado com 401: o navegador só vê 1006
  await vi.advanceTimersByTimeAsync(0)
  expect(checkSession).toHaveBeenCalledOnce()
  expect(statuses.at(-1)).toBe('unauthorized')
  await vi.advanceTimersByTimeAsync(60_000)
  expect(FakeSocket.instances).toHaveLength(1)
})

it('disconnect() fecha sem reconectar', async () => {
  const { client, statuses } = setup()
  client.connect()
  FakeSocket.last().open()
  client.disconnect()
  await vi.advanceTimersByTimeAsync(60_000)
  expect(statuses.at(-1)).toBe('idle')
  expect(FakeSocket.instances).toHaveLength(1)
})

it('backoff cresce até 10 s com jitter', () => {
  expect(backoffDelay(0, () => 1)).toBe(500)
  expect(backoffDelay(3, () => 1)).toBe(4000)
  expect(backoffDelay(10, () => 1)).toBe(10_000)
  expect(backoffDelay(10, () => 0)).toBe(5000)
})
