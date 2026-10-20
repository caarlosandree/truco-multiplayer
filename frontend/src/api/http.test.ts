import { ApiError, getClockOffsetMs, request, resetClockSamples } from './http'

function respond(status: number, body?: unknown, headers: Record<string, string> = {}) {
  return vi.fn().mockResolvedValue(
    new Response(body === undefined ? null : JSON.stringify(body), {
      status,
      headers: { 'Content-Type': 'application/json', ...headers },
    }),
  )
}

beforeEach(() => resetClockSamples())

it('envia JSON com credenciais e devolve o corpo', async () => {
  const fetchMock = respond(200, { user: { id: '1' } })
  vi.stubGlobal('fetch', fetchMock)
  await expect(request('POST', '/api/auth/login', { login: 'a', password: 'b' })).resolves.toEqual({ user: { id: '1' } })
  const [url, init] = fetchMock.mock.calls[0]!
  expect(url).toBe('/api/auth/login')
  expect(init.credentials).toBe('same-origin')
  expect(init.headers['Content-Type']).toBe('application/json')
  expect(init.body).toBe('{"login":"a","password":"b"}')
})

it('devolve undefined em 204', async () => {
  vi.stubGlobal('fetch', respond(204))
  await expect(request('POST', '/api/auth/logout')).resolves.toBeUndefined()
})

it('converte o corpo de erro em ApiError', async () => {
  vi.stubGlobal('fetch', respond(400, { error: { code: 'validation_failed', message: 'Verifique', fields: { email: 'inválido' } } }))
  const err = await request('POST', '/api/auth/register', {}).catch((e: unknown) => e)
  expect(err).toBeInstanceOf(ApiError)
  expect(err).toMatchObject({ status: 400, code: 'validation_failed', fields: { email: 'inválido' } })
})

it('marca falha de rede com code network', async () => {
  vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')))
  await expect(request('GET', '/api/me')).rejects.toMatchObject({ code: 'network', status: 0 })
})

it('lê Retry-After em segundos inteiros para 429', async () => {
  vi.stubGlobal(
    'fetch',
    respond(429, { error: { code: 'too_many_attempts', message: 'Muitas tentativas.' } }, { 'Retry-After': '30' }),
  )
  const err = await request('POST', '/api/auth/login', {}).catch((e: unknown) => e)
  expect(err).toBeInstanceOf(ApiError)
  expect(err).toMatchObject({ status: 429, code: 'too_many_attempts', retryAfterSec: 30 })
})

it('ignora Retry-After ausente ou inválido', async () => {
  vi.stubGlobal('fetch', respond(429, { error: { code: 'too_many_attempts', message: 'x' } }))
  const semHeader = await request('POST', '/api/auth/login', {}).catch((e: unknown) => e)
  expect(semHeader).toBeInstanceOf(ApiError)
  expect((semHeader as ApiError).retryAfterSec).toBeUndefined()

  vi.stubGlobal('fetch', respond(429, { error: { code: 'too_many_attempts', message: 'x' } }, { 'Retry-After': 'depois' }))
  const invalido = await request('POST', '/api/auth/login', {}).catch((e: unknown) => e)
  expect((invalido as ApiError).retryAfterSec).toBeUndefined()
})

it('estima o offset do relógio pelo cabeçalho Date', async () => {
  vi.useFakeTimers({ now: new Date('2026-10-15T12:00:00Z') })
  vi.stubGlobal('fetch', respond(200, {}, { Date: 'Thu, 15 Oct 2026 12:00:10 GMT' }))
  await request('GET', '/api/me')
  expect(getClockOffsetMs()).toBeCloseTo(10_000, -2)
  vi.useRealTimers()
})
