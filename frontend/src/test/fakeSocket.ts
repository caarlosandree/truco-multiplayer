import type { SocketLike } from '@/ws/client'
import type { ServerMessage } from '@/protocol/union'

export class FakeSocket implements SocketLike {
  static instances: FakeSocket[] = []
  readyState = 0
  sent: string[] = []
  onopen: ((ev: unknown) => void) | null = null
  onmessage: ((ev: { data: unknown }) => void) | null = null
  onclose: ((ev: { code: number; reason: string }) => void) | null = null
  onerror: ((ev: unknown) => void) | null = null

  constructor(readonly url: string) {
    FakeSocket.instances.push(this)
  }
  send(data: string) {
    this.sent.push(data)
  }
  close(code = 1000, reason = '') {
    this.serverClose(code, reason)
  }
  open() {
    this.readyState = 1
    this.onopen?.({})
  }
  receive(msg: ServerMessage | string) {
    this.onmessage?.({ data: typeof msg === 'string' ? msg : JSON.stringify(msg) })
  }
  serverClose(code: number, reason = '') {
    this.readyState = 3
    this.onclose?.({ code, reason })
  }
  sentMessages() {
    return this.sent.map((s) => JSON.parse(s) as Record<string, unknown>)
  }
  static last(): FakeSocket {
    const s = FakeSocket.instances.at(-1)
    if (!s) throw new Error('nenhum socket criado')
    return s
  }
}
