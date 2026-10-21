import { createPinia, setActivePinia } from 'pinia'
import { socialApi } from '@/api/social'
import { usePlayersStore } from './players'

beforeEach(() => setActivePinia(createPinia()))
afterEach(() => vi.restoreAllMocks())

it('searchDirectory devolve só os itens da primeira página', async () => {
  const spy = vi.spyOn(socialApi, 'directory').mockResolvedValue({ items: [{ username: 'caio', displayName: 'Caio' }], nextCursor: 'x' })
  const r = await usePlayersStore().searchDirectory('ca')
  expect(spy).toHaveBeenCalledWith('ca')
  expect(r).toEqual([{ username: 'caio', displayName: 'Caio' }])
})
