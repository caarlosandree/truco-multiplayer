import { createPinia, setActivePinia } from 'pinia'
import { setResumeHandler, useSessionStore } from './session'
import { authApi } from '@/api/auth'
import { ApiError } from '@/api/http'

const ana = { id: 'u1', username: 'ana', email: 'ana@empresa.com.br', displayName: 'Ana', isAdmin: false, createdAt: '' }

beforeEach(() => setActivePinia(createPinia()))
afterEach(() => {
  vi.restoreAllMocks()
  setResumeHandler(null)
})

it('load() guarda o usuário ou null em 401', async () => {
  vi.spyOn(authApi, 'me').mockResolvedValueOnce(ana)
  const s = useSessionStore()
  await s.load()
  expect(s.user).toEqual(ana)
  expect(s.loaded).toBe(true)

  vi.spyOn(authApi, 'me').mockRejectedValueOnce(new ApiError(401, 'unauthenticated', 'x'))
  await s.load()
  expect(s.user).toBeNull()
})

it('load() propaga erro que não é 401', async () => {
  vi.spyOn(authApi, 'me').mockRejectedValueOnce(new ApiError(0, 'network', 'x'))
  await expect(useSessionStore().load()).rejects.toMatchObject({ code: 'network' })
})

it('logout limpa o usuário e nunca rejeita, mesmo se a API falhar', async () => {
  vi.spyOn(authApi, 'logout').mockRejectedValueOnce(new ApiError(0, 'network', 'x'))
  const s = useSessionStore()
  s.user = ana
  await expect(s.logout()).resolves.toBeUndefined()
  expect(s.user).toBeNull()
})

it('resumeConnection delega ao handler injetado e é no-op sem ele', () => {
  const s = useSessionStore()
  expect(() => s.resumeConnection()).not.toThrow()
  const handler = vi.fn()
  setResumeHandler(handler)
  s.resumeConnection()
  expect(handler).toHaveBeenCalledOnce()
})
