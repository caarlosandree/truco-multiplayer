import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory } from 'vue-router'
import { createAppRouter, sanitizeReturnTo } from './router'
import { useSessionStore } from './stores/session'
import { authApi } from './api/auth'
import { ApiError } from './api/http'

const ana = { id: 'u1', username: 'ana', email: 'a@e.com', displayName: 'Ana', isAdmin: false, createdAt: '' }

beforeEach(() => setActivePinia(createPinia()))

it('sanitizeReturnTo rejeita destinos externos', () => {
  expect(sanitizeReturnTo('/sala/ABC234')).toBe('/sala/ABC234')
  for (const bad of ['https://evil.com', '//evil.com', '/\\evil.com', 'javascript:alert(1)', 42, undefined]) {
    expect(sanitizeReturnTo(bad)).toBe('/')
  }
})

it('manda deslogado para /entrar com volta', async () => {
  vi.spyOn(authApi, 'me').mockRejectedValue(new ApiError(401, 'unauthenticated', 'x'))
  const router = createAppRouter(createMemoryHistory())
  await router.push('/perfil')
  expect(router.currentRoute.value.name).toBe('entrar')
  expect(router.currentRoute.value.query.volta).toBe('/perfil')
})

it('rota de convite normaliza o código e preserva o destino após login', async () => {
  vi.spyOn(authApi, 'me').mockRejectedValue(new ApiError(401, 'unauthenticated', 'x'))
  const router = createAppRouter(createMemoryHistory())
  await router.push('/sala/k7qm2p')
  expect(router.currentRoute.value.query.volta).toBe('/sala/K7QM2P')
})

it('logado, o convite com código minúsculo é normalizado', async () => {
  vi.spyOn(authApi, 'me').mockResolvedValue(ana)
  const router = createAppRouter(createMemoryHistory())
  await router.push('/sala/k7qm2p')
  expect(router.currentRoute.value.params.codigo).toBe('K7QM2P')
})

it('logado em /entrar vai para o lobby', async () => {
  vi.spyOn(authApi, 'me').mockResolvedValue(ana)
  const router = createAppRouter(createMemoryHistory())
  await router.push('/entrar')
  expect(router.currentRoute.value.name).toBe('lobby')
  expect(useSessionStore().user?.id).toBe('u1')
})

it('logado em /entrar?volta=externo ignora o destino externo', async () => {
  vi.spyOn(authApi, 'me').mockResolvedValue(ana)
  const router = createAppRouter(createMemoryHistory())
  await router.push('/entrar?volta=//evil.com')
  expect(router.currentRoute.value.name).toBe('lobby')
})
