import { createPinia, setActivePinia } from 'pinia'
import { socialApi } from '@/api/social'
import { ApiError } from '@/api/http'
import { isNotFound, usePlayersStore } from './players'

beforeEach(() => setActivePinia(createPinia()))
afterEach(() => vi.restoreAllMocks())

it('searchDirectory devolve só os itens da primeira página', async () => {
  const spy = vi.spyOn(socialApi, 'directory').mockResolvedValue({ items: [{ username: 'caio', displayName: 'Caio' }], nextCursor: 'x' })
  const r = await usePlayersStore().searchDirectory('ca')
  expect(spy).toHaveBeenCalledWith('ca')
  expect(r).toEqual([{ username: 'caio', displayName: 'Caio' }])
})

it('loadProfile e loadHistory repassam o username e o cursor', async () => {
  const perfil = vi.spyOn(socialApi, 'profile').mockResolvedValue({ username: 'ana' } as never)
  const hist = vi.spyOn(socialApi, 'history').mockResolvedValue({ items: [], nextCursor: 'c2' })
  const store = usePlayersStore()
  expect(await store.loadProfile('ana')).toEqual({ username: 'ana' })
  expect(perfil).toHaveBeenCalledWith('ana')
  expect(await store.loadHistory('ana', 'c1')).toEqual({ items: [], nextCursor: 'c2' })
  expect(hist).toHaveBeenCalledWith('ana', 'c1')
})

it('isNotFound reconhece o 404 da API e nada mais', () => {
  expect(isNotFound(new ApiError(404, 'not_found', 'x'))).toBe(true)
  expect(isNotFound(new ApiError(500, 'internal', 'x'))).toBe(false)
  expect(isNotFound(new Error('x'))).toBe(false)
})
