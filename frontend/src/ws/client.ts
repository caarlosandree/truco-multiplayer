import type { ClientMessage, ServerMessage } from '@/protocol/union'

/** Versão do protocolo que este build entende (backend: protocolVersion no hello). */
export const EXPECTED_PROTOCOL_VERSION = 1

const CLOSE_REPLACED = 4001
const CLOSE_SESSION_INVALID = 4003
const CLOSE_POLICY = 4008
const ACK_TIMEOUT_MS = 5000
const CONNECTING = 0
const OPEN = 1

export type WsStatus = 'idle' | 'connecting' | 'open' | 'reconnecting' | 'replaced' | 'unauthorized'

type WithoutSeq<M> = M extends unknown ? Omit<M, 'seq'> : never
export type Outgoing = WithoutSeq<ClientMessage>

export interface SocketLike {
  readyState: number
  send(data: string): void
  close(code?: number, reason?: string): void
  onopen: ((ev: unknown) => void) | null
  onmessage: ((ev: { data: unknown }) => void) | null
  onclose: ((ev: { code: number; reason: string }) => void) | null
  onerror: ((ev: unknown) => void) | null
}

export interface WsClientOptions {
  url: string
  onMessage(msg: ServerMessage): void
  onStatus(status: WsStatus): void
  onVersionMismatch(serverVersion: number): void
  onUnackedError(code: string, message: string): void
  checkSession(): Promise<boolean>
  createSocket?(url: string): SocketLike
  setTimer?(fn: () => void, ms: number): unknown
  clearTimer?(handle: unknown): void
  random?(): number
}

export class WsSendError extends Error {
  constructor(
    readonly code: string,
    message: string,
  ) {
    super(message)
    this.name = 'WsSendError'
  }
}

export function backoffDelay(attempt: number, random: () => number): number {
  const base = Math.min(10_000, 500 * 2 ** attempt)
  return base * (0.5 + random() / 2)
}

export function wsUrl(loc: { protocol: string; host: string }): string {
  return `${loc.protocol === 'https:' ? 'wss' : 'ws'}://${loc.host}/ws`
}

interface Pending {
  resolve(): void
  reject(err: WsSendError): void
  timer: unknown
}

export class WsClient {
  private socket: SocketLike | null = null
  private current: WsStatus = 'idle'
  private seq = 0
  private attempt = 0
  private gotHello = false
  private wanted = false
  private reconnectTimer: unknown = null
  private pending = new Map<number, Pending>()

  constructor(private readonly opts: WsClientOptions) {}

  get status(): WsStatus {
    return this.current
  }

  connect(): void {
    this.wanted = true
    this.attempt = 0
    this.open()
  }

  /** Retoma depois de 'replaced' (o usuário escolheu usar esta aba). */
  resume(): void {
    this.connect()
  }

  disconnect(): void {
    this.wanted = false
    this.cancelReconnect()
    const s = this.socket
    this.socket = null
    if (s) {
      s.onclose = null
      s.close(1000, 'cliente saiu')
    }
    this.failPending('network', 'Conexão encerrada.')
    this.setStatus('idle')
  }

  send(msg: Outgoing): Promise<void> {
    const s = this.socket
    if (!s || s.readyState !== OPEN || !this.gotHello) {
      return Promise.reject(new WsSendError('network', 'Sem conexão com o servidor.'))
    }
    const seq = ++this.seq
    s.send(JSON.stringify({ type: msg.type, seq, ...('payload' in msg ? { payload: msg.payload } : {}) }))
    return new Promise<void>((resolve, reject) => {
      const timer = this.timer(() => {
        this.pending.delete(seq)
        resolve()
      }, ACK_TIMEOUT_MS)
      this.pending.set(seq, { resolve, reject, timer })
    })
  }

  private open(): void {
    // Uma conexão por aba: se já há socket vivo (ou conectando), não abre outro.
    if (this.socket && (this.socket.readyState === CONNECTING || this.socket.readyState === OPEN)) return
    this.detachSocket()
    this.cancelReconnect()
    this.gotHello = false
    this.setStatus(this.attempt === 0 ? 'connecting' : 'reconnecting')
    const s = (this.opts.createSocket ?? ((url) => new WebSocket(url) as unknown as SocketLike))(this.opts.url)
    this.socket = s
    s.onmessage = (ev) => this.handleData(ev.data)
    s.onclose = (ev) => void this.handleClose(s, ev.code)
    s.onerror = () => {
      // o onclose vem logo depois e decide o que fazer
    }
  }

  private handleData(data: unknown): void {
    if (typeof data !== 'string') return
    let msg: ServerMessage
    try {
      msg = JSON.parse(data) as ServerMessage
    } catch {
      console.warn('[ws] mensagem não-JSON descartada')
      return
    }
    if (typeof msg !== 'object' || msg === null || typeof msg.type !== 'string') return
    if (msg.type === 'hello') {
      this.gotHello = true
      this.attempt = 0
      this.setStatus('open')
      if (msg.payload.protocolVersion !== EXPECTED_PROTOCOL_VERSION) this.opts.onVersionMismatch(msg.payload.protocolVersion)
    }
    if (msg.type === 'error') {
      const seq = msg.payload.seq
      const p = seq === undefined ? undefined : this.pending.get(seq)
      if (p && seq !== undefined) {
        this.pending.delete(seq)
        this.clear(p.timer)
        p.reject(new WsSendError(msg.payload.code, msg.payload.message))
      } else {
        this.opts.onUnackedError(msg.payload.code, msg.payload.message)
      }
    } else {
      this.resolvePending()
    }
    this.opts.onMessage(msg)
  }

  /** Solta os handlers de um socket anterior e o fecha, para não sobrar conexão viva. */
  private detachSocket(): void {
    const old = this.socket
    this.socket = null
    if (!old) return
    old.onopen = null
    old.onmessage = null
    old.onclose = null
    old.onerror = null
    old.close(1000, 'substituído')
  }

  private async handleClose(s: SocketLike, code: number): Promise<void> {
    if (s !== this.socket) return
    this.socket = null
    const hadHello = this.gotHello
    this.gotHello = false
    this.failPending('network', 'Conexão perdida.')
    if (!this.wanted) return
    if (code === CLOSE_REPLACED) {
      this.wanted = false
      this.setStatus('replaced')
      return
    }
    if (code === CLOSE_POLICY) {
      console.warn(`[ws] fechado pelo servidor por política (código ${CLOSE_POLICY}); reconectando`)
    }
    // Sem hello, a queda pode ser sessão expirada: checa. Falha aberta: erro na checagem = seguimos reconectando.
    const sessionOk = hadHello || code === CLOSE_SESSION_INVALID ? true : await this.opts.checkSession().catch(() => true)
    if (code === CLOSE_SESSION_INVALID || !sessionOk) {
      this.wanted = false
      this.setStatus('unauthorized')
      return
    }
    if (!this.wanted) return
    this.setStatus('reconnecting')
    const delay = backoffDelay(this.attempt, this.opts.random ?? Math.random)
    this.attempt++
    this.reconnectTimer = this.timer(() => {
      this.reconnectTimer = null
      if (this.wanted) this.open()
    }, delay)
  }

  private resolvePending(): void {
    for (const [seq, p] of this.pending) {
      this.clear(p.timer)
      p.resolve()
      this.pending.delete(seq)
    }
  }

  private failPending(code: string, message: string): void {
    for (const [seq, p] of this.pending) {
      this.clear(p.timer)
      p.reject(new WsSendError(code, message))
      this.pending.delete(seq)
    }
  }

  private setStatus(s: WsStatus): void {
    if (this.current === s) return
    this.current = s
    this.opts.onStatus(s)
  }

  private cancelReconnect(): void {
    if (this.reconnectTimer !== null) this.clear(this.reconnectTimer)
    this.reconnectTimer = null
  }

  private timer(fn: () => void, ms: number): unknown {
    return (this.opts.setTimer ?? ((f, t) => setTimeout(f, t)))(fn, ms)
  }

  private clear(h: unknown): void {
    ;(this.opts.clearTimer ?? ((x) => clearTimeout(x as ReturnType<typeof setTimeout>)))(h)
  }
}
