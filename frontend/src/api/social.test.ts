import { socialApi } from './social'

function stubFetch() {
  const urls: string[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string) => {
      urls.push(url)
      return new Response(JSON.stringify({ items: [] }), { status: 200 })
    }),
  )
  return urls
}

afterEach(() => vi.unstubAllGlobals())

it('diretório codifica a busca e o cursor', async () => {
  const urls = stubFetch()
  await socialApi.directory('ana b')
  await socialApi.directory('', 'c/1')
  expect(urls).toEqual(['/api/users?q=ana%20b', '/api/users?cursor=c%2F1'])
})

it('ranking monta período e métrica', async () => {
  const urls = stubFetch()
  await socialApi.ranking('week', 'winrate')
  await socialApi.ranking('all', 'wins', 20)
  expect(urls).toEqual(['/api/ranking?period=week&metric=winrate', '/api/ranking?period=all&metric=wins&limit=20'])
})

it('perfil e histórico escapam o username', async () => {
  const urls = stubFetch()
  await socialApi.profile('ana.b')
  await socialApi.history('ana', 'x y')
  expect(urls).toEqual(['/api/users/ana.b', '/api/matches?user=ana&cursor=x%20y'])
})
