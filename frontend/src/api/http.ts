export class ApiError extends Error {
  retryAfterSec?: number

  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
    readonly fields: Record<string, string> = {},
    retryAfterSec?: number,
  ) {
    super(message)
    this.name = 'ApiError'
    if (retryAfterSec !== undefined) this.retryAfterSec = retryAfterSec
  }
}

type Method = 'GET' | 'POST' | 'PATCH'

const MAX_SAMPLES = 5
let clockSamples: number[] = []

export function resetClockSamples(): void {
  clockSamples = []
}

export function getClockOffsetMs(): number {
  if (clockSamples.length === 0) return 0
  return clockSamples.reduce((a, b) => a + b, 0) / clockSamples.length
}

export function serverNow(): number {
  return Date.now() + getClockOffsetMs()
}

function sampleClock(res: Response): void {
  const header = res.headers.get('Date')
  if (!header) return
  const server = Date.parse(header)
  if (Number.isNaN(server)) return
  // O cabeçalho tem resolução de 1 s; a média suaviza.
  clockSamples = [...clockSamples, server - Date.now()].slice(-MAX_SAMPLES)
}

function parseRetryAfter(res: Response): number | undefined {
  const raw = res.headers.get('Retry-After')?.trim()
  if (!raw || !/^\d+$/.test(raw)) return undefined
  return Number(raw)
}

export async function request<T>(method: Method, path: string, body?: unknown): Promise<T> {
  let res: Response
  try {
    res = await fetch(path, {
      method,
      credentials: 'same-origin',
      headers: body === undefined ? { Accept: 'application/json' } : { Accept: 'application/json', 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
    })
  } catch {
    throw new ApiError(0, 'network', 'Sem conexão com o servidor.')
  }
  sampleClock(res)
  if (res.status === 202 || res.status === 204) return undefined as T
  const text = await res.text()
  const data: unknown = text ? JSON.parse(text) : undefined
  if (!res.ok) {
    const err = (data as { error?: { code?: string; message?: string; fields?: Record<string, string> } } | undefined)?.error
    throw new ApiError(
      res.status,
      err?.code ?? 'internal',
      err?.message ?? 'Erro no servidor, tente de novo.',
      err?.fields ?? {},
      parseRetryAfter(res),
    )
  }
  return data as T
}
