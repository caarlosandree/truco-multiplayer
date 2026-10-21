import { createPinia, setActivePinia } from 'pinia'
import type { RankingResult } from './ranking'
import { useRankingStore } from './ranking'
import { useToastsStore } from './toasts'
import { socialApi } from '@/api/social'

vi.mock('@/api/social', () => ({ socialApi: { ranking: vi.fn() } }))

const ranking = vi.mocked(socialApi.ranking)

const resultado = (username: string): RankingResult => ({
  items: [{ position: 1, username, displayName: username, matchesPlayed: 3, wins: 2, losses: 1, winRate: 2 / 3 }],
  me: null,
  minMatches: 5,
})

beforeEach(() => {
  setActivePinia(createPinia())
  ranking.mockReset()
})

it('trocar de week para month antes da primeira resposta mantém só o resultado de month', async () => {
  let resolveWeek: (r: RankingResult) => void = () => {}
  ranking.mockImplementationOnce(() => new Promise((res) => (resolveWeek = res)))
  ranking.mockResolvedValueOnce(resultado('do_mes'))
  const s = useRankingStore()
  const semana = s.load('week', 'wins')
  const mes = s.load('month', 'wins')
  await mes
  resolveWeek(resultado('da_semana'))
  await semana
  expect(s.period).toBe('month')
  expect(s.status).toBe('ok')
  expect(s.result?.items[0]?.username).toBe('do_mes')
})

it('guarda o filtro, limpa o resultado antigo e passa os parâmetros à API', async () => {
  ranking.mockResolvedValue(resultado('a'))
  const s = useRankingStore()
  const p = s.load('all', 'winrate')
  expect(s.status).toBe('loading')
  expect(s.result).toBeNull()
  await p
  expect(ranking).toHaveBeenCalledWith('all', 'winrate')
  expect(s.metric).toBe('winrate')
})

it('erro vira status error com um toast, e o erro de resposta velha é ignorado', async () => {
  let rejeita: (e: unknown) => void = () => {}
  ranking.mockImplementationOnce(() => new Promise((_, rej) => (rejeita = rej)))
  ranking.mockResolvedValueOnce(resultado('ok'))
  const s = useRankingStore()
  const velha = s.load('week', 'wins')
  await s.load('month', 'wins')
  rejeita(new Error('boom'))
  await velha
  expect(s.status).toBe('ok')
  expect(useToastsStore().items).toHaveLength(0)

  ranking.mockRejectedValueOnce(new Error('fora'))
  await s.load('all', 'wins')
  expect(s.status).toBe('error')
  expect(useToastsStore().items).toHaveLength(1)
})

it('semana → mês → semana: só a última resposta vale, mesmo com as velhas chegando depois', async () => {
  const resolvers: ((r: RankingResult) => void)[] = []
  ranking.mockImplementation(() => new Promise((res) => resolvers.push(res)))
  const s = useRankingStore()
  const a = s.load('week', 'wins')
  const b = s.load('month', 'wins')
  const c = s.load('week', 'wins')
  resolvers[2]?.(resultado('terceira'))
  await c
  resolvers[0]?.(resultado('primeira'))
  resolvers[1]?.(resultado('segunda'))
  await Promise.all([a, b])
  expect(s.result?.items[0]?.username).toBe('terceira')
  expect(s.period).toBe('week')
})
