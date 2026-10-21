import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createMemoryHistory } from 'vue-router'
import { createAppRouter } from '@/router'
import { socialApi } from '@/api/social'
import { useSessionStore } from '@/stores/session'
import type { RankingEntry, RankingResult } from '@/stores/ranking'
import RankingView from './RankingView.vue'

vi.mock('@/api/social', () => ({ socialApi: { ranking: vi.fn() } }))
const ranking = vi.mocked(socialApi.ranking)

const linha = (position: number, username: string, wins: number, losses: number): RankingEntry => ({
  position,
  username,
  displayName: username.toUpperCase(),
  matchesPlayed: wins + losses,
  wins,
  losses,
  winRate: wins / (wins + losses),
})

const cheio: RankingResult = {
  items: [linha(1, 'ana', 8, 2), linha(2, 'bia', 5, 5), linha(3, 'eu', 2, 1)],
  me: linha(3, 'eu', 2, 1),
  minMatches: 5,
}

const wrappers: VueWrapper[] = []
async function abrir(res: RankingResult | Error = cheio) {
  if (res instanceof Error) ranking.mockRejectedValue(res)
  else ranking.mockResolvedValue(res)
  const router = createAppRouter(createMemoryHistory())
  await router.push('/ranking')
  const w = mount(RankingView, { global: { plugins: [router] }, attachTo: document.body })
  wrappers.push(w)
  await flushPromises()
  return { w, router }
}

beforeEach(() => {
  setActivePinia(createPinia())
  ranking.mockReset()
  useSessionStore().user = { id: 'id-eu', username: 'eu', email: 'e@x.com', displayName: 'EU', isAdmin: false, createdAt: '' }
  useSessionStore().loaded = true
})
afterEach(() => wrappers.splice(0).forEach((w) => w.unmount()))

it('abas Vitórias/Aproveitamento e período Geral/Mês/Semana, com colunas #, jogador, V, D, %', async () => {
  const { w } = await abrir()
  const tabs = w.find('[role="tablist"]').findAll('[role="tab"]')
  expect(tabs.map((t) => t.text())).toEqual(['Vitórias', 'Aproveitamento'])
  expect(tabs[0]?.attributes('aria-selected')).toBe('true')
  const radios = w.find('[role="radiogroup"]').findAll('[role="radio"], input[type="radio"]')
  expect(radios).toHaveLength(3)
  expect(w.find('[role="radiogroup"]').text()).toMatch(/Geral\s*Mês\s*Semana/)
  expect(w.findAll('thead th').map((t) => t.text())).toEqual(['#', 'Jogador', 'V', 'D', '%'])
  expect(ranking).toHaveBeenLastCalledWith('all', 'wins')
  expect(w.find('tbody').text()).toContain('66,7%')
})

it('trocar aba ou período recarrega com o filtro novo', async () => {
  const { w } = await abrir()
  await w.findAll('[role="tab"]')[1]?.trigger('click')
  await flushPromises()
  expect(ranking).toHaveBeenLastCalledWith('all', 'winrate')
  await w.findAll('input[type="radio"]')[2]?.setValue(true)
  await flushPromises()
  expect(ranking).toHaveBeenLastCalledWith('week', 'winrate')
})

it('minha linha tem aria-current e não há duplicata quando estou no top', async () => {
  const { w } = await abrir()
  const atual = w.findAll('[aria-current="true"]')
  expect(atual).toHaveLength(1)
  expect(atual[0]?.text()).toContain('EU')
})

it('me fora do top aparece no fim, separado por divisor', async () => {
  const { w } = await abrir({ ...cheio, items: cheio.items.slice(0, 2), me: linha(42, 'eu', 1, 1) })
  const atual = w.find('[aria-current="true"]')
  expect(atual.text()).toContain('42')
  expect(w.find('[data-testid="ranking-divisor"]').exists()).toBe(true)
  expect(w.findAll('tbody tr').at(-1)?.attributes('aria-current')).toBe('true')
})

it('nota do mínimo só em Aproveitamento, com minMatches da resposta', async () => {
  const { w } = await abrir({ ...cheio, minMatches: 7 })
  expect(w.text()).not.toContain('Só quem jogou')
  await w.findAll('[role="tab"]')[1]?.trigger('click')
  await flushPromises()
  expect(w.text()).toContain('Só quem jogou 7 ou mais partidas no período.')
})

it('nota usa 5 ou mais quando minMatches é 5', async () => {
  const { w } = await abrir()
  await w.findAll('[role="tab"]')[1]?.trigger('click')
  await flushPromises()
  expect(w.text()).toContain('Só quem jogou 5 ou mais partidas no período.')
})

it('vazio na semana', async () => {
  const { w } = await abrir({ items: [], me: null, minMatches: 5 })
  await w.findAll('input[type="radio"]')[2]?.setValue(true)
  await flushPromises()
  expect(w.text()).toContain('Ninguém jogou nesta semana ainda')
  expect(w.find('table').exists()).toBe(false)
})

it('erro: mensagem e Tentar de novo recarrega', async () => {
  const { w } = await abrir(new Error('fora'))
  expect(w.text()).toContain('Não foi possível carregar o ranking.')
  ranking.mockResolvedValue(cheio)
  const botao = w.findAll('button').find((b) => b.text() === 'Tentar de novo')
  await botao?.trigger('click')
  await flushPromises()
  expect(w.find('table').exists()).toBe(true)
})

it('clicar no jogador abre o perfil', async () => {
  const { w, router } = await abrir()
  const push = vi.spyOn(router, 'push')
  await w.find('a[href="/jogadores/ana"]').trigger('click')
  expect(push).toHaveBeenCalledWith({ name: 'jogador', params: { username: 'ana' } })
})

it('teclado nas abas: setas, Home e End carregam a métrica certa e movem o foco', async () => {
  const { w } = await abrir()
  const tabs = () => w.findAll('[role="tab"]')
  await tabs()[0]?.trigger('keydown', { key: 'ArrowRight' })
  await flushPromises()
  expect(ranking).toHaveBeenLastCalledWith('all', 'winrate')
  expect(document.activeElement?.id).toBe('aba-ranking-winrate')
  await tabs()[1]?.trigger('keydown', { key: 'ArrowRight' })
  await flushPromises()
  expect(ranking).toHaveBeenLastCalledWith('all', 'wins')
  expect(document.activeElement?.id).toBe('aba-ranking-wins')
  await tabs()[0]?.trigger('keydown', { key: 'ArrowLeft' })
  await flushPromises()
  expect(ranking).toHaveBeenLastCalledWith('all', 'winrate')
  await tabs()[1]?.trigger('keydown', { key: 'Home' })
  await flushPromises()
  expect(ranking).toHaveBeenLastCalledWith('all', 'wins')
  await tabs()[0]?.trigger('keydown', { key: 'End' })
  await flushPromises()
  expect(ranking).toHaveBeenLastCalledWith('all', 'winrate')
  expect(document.activeElement?.id).toBe('aba-ranking-winrate')
})

it('estado carregando: aria-busy e marcador de teste', async () => {
  ranking.mockReturnValue(new Promise(() => {}))
  const router = createAppRouter(createMemoryHistory())
  await router.push('/ranking')
  const w = mount(RankingView, { global: { plugins: [router] }, attachTo: document.body })
  wrappers.push(w)
  await flushPromises()
  const c = w.find('[data-testid="ranking-carregando"]')
  expect(c.exists()).toBe(true)
  expect(c.attributes('aria-busy')).toBe('true')
  expect(w.find('table').exists()).toBe(false)
})

it('clicar na aba ou no período já selecionado não refaz a requisição', async () => {
  const { w } = await abrir()
  ranking.mockClear()
  await w.findAll('[role="tab"]')[0]?.trigger('click')
  await w.findAll('input[type="radio"]')[0]?.trigger('change')
  await flushPromises()
  expect(ranking).not.toHaveBeenCalled()
})
