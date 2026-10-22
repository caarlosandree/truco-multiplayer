import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createMemoryHistory } from 'vue-router'
import { ApiError } from '@/api/http'
import { createAppRouter } from '@/router'
import { socialApi, type HistoryItem, type Profile } from '@/api/social'
import { useInvitesStore } from '@/stores/invites'
import { useRoomStore } from '@/stores/room'
import { useSessionStore } from '@/stores/session'
import { usePresenceStore } from '@/stores/presence'
import { useToastsStore } from '@/stores/toasts'
import LinhaPartida from '@/components/partidas/LinhaPartida.vue'
import AppShell from '@/components/AppShell.vue'
import OnlineAgora from '@/components/lobby/OnlineAgora.vue'
import JogadorView from './JogadorView.vue'
import PartidasView from './PartidasView.vue'

vi.mock('@/api/social', () => ({ socialApi: { profile: vi.fn(), history: vi.fn(), directory: vi.fn() } }))
const profile = vi.mocked(socialApi.profile)
const history = vi.mocked(socialApi.history)

const jogadores = (a: string, b: string, c: string, d: string) => [
  { seat: 0, username: a, displayName: a.toUpperCase(), bot: false },
  { seat: 1, username: b, displayName: b.toUpperCase(), bot: false },
  { seat: 2, username: c, displayName: c.toUpperCase(), bot: false },
  { seat: 3, username: d, displayName: d.toUpperCase(), bot: false },
]

const partida = (over: Partial<HistoryItem> = {}): HistoryItem => ({
  id: 'm1',
  status: 'finished',
  team: 0,
  won: true,
  winnerTeam: 0,
  score: [12, 7],
  createdAt: '2026-10-20T15:00:00Z',
  finishedAt: '2026-10-20T15:30:00Z',
  players: jogadores('ana', 'bia', 'caio', 'duda'),
  endReason: 'game_over',
  ranked: true,
  ...over,
})

const perfilAna: Profile = {
  username: 'ana',
  displayName: 'ANA',
  createdAt: '2026-01-01T00:00:00Z',
  stats: {
    matchesPlayed: 10, wins: 6, losses: 4, winRate: 0.6, pointsFor: 100, pointsAgainst: 80, handsPlayed: 50, handsWon: 30,
    raisesCalled: 14, raisesFolded: 3, raisesWonByFold: 5, elevenHands: 4, elevenRuns: 1, elevenWon: 2, ironHands: 0, ironWon: 0,
    punished: 0, timeouts: 0,
  },
  recentMatches: [partida()],
  partners: [{ username: 'caio', displayName: 'CAIO', matches: 4, wins: 3, winRate: 0.75 }],
  positions: {
    all: { wins: 3, winrate: 5 },
    month: { wins: null, winrate: null },
    week: { wins: 1, winrate: null },
  },
}

const wrappers: VueWrapper[] = []
async function abrirPerfil(username = 'ana') {
  const router = createAppRouter(createMemoryHistory())
  await router.push(`/jogadores/${username}`)
  const w = mount(JogadorView, { global: { plugins: [router] }, attachTo: document.body })
  wrappers.push(w)
  await flushPromises()
  return { w, router }
}
async function abrirPartidas(username = 'ana') {
  const router = createAppRouter(createMemoryHistory())
  await router.push(`/jogadores/${username}/partidas`)
  const w = mount(PartidasView, { global: { plugins: [router] }, attachTo: document.body })
  wrappers.push(w)
  await flushPromises()
  return { w, router }
}

beforeEach(() => {
  setActivePinia(createPinia())
  profile.mockReset()
  history.mockReset()
  profile.mockResolvedValue(perfilAna)
  useSessionStore().user = { id: 'id-eu', username: 'eu', email: 'e@x.com', displayName: 'EU', isAdmin: false, createdAt: '' }
  useSessionStore().loaded = true
})
afterEach(() => wrappers.splice(0).forEach((w) => w.unmount()))

describe('perfil público', () => {
  it('mostra nome, @username, offline quando ausente da presença, posições, estatísticas, recentes e parceiros', async () => {
    const { w } = await abrirPerfil()
    expect(w.find('h1').text()).toBe('ANA')
    expect(w.text()).toContain('@ana')
    expect(w.text()).toContain('offline')
    const pos = w.find('[data-testid="posicoes"]')
    expect(pos.text()).toMatch(/Geral\s*#3\s*#5/)
    expect(pos.text()).toMatch(/Mês\s*—\s*—/)
    expect(pos.text()).toMatch(/Semana\s*#1\s*—/)
    const est = w.find('[data-testid="estatisticas"]').text()
    expect(est).toContain('Partidas')
    expect(est).toContain('10')
    expect(est).toContain('60%')
    expect(est).toMatch(/Aumentos pedidos\s*14/)
    expect(est).toMatch(/Aumentos corridos \(dupla\)\s*3/)
    expect(est).toMatch(/Mãos de 11 vencidas\s*2/)
    expect(w.findAll('[data-testid="partida"]')).toHaveLength(1)
    expect(w.find('[data-testid="parceiros"]').text()).toContain('CAIO')
    expect(w.find('[data-testid="parceiros"]').text()).toContain('4 partidas')
    expect(w.find('a[href="/jogadores/ana/partidas"]').exists()).toBe(true)
  })

  it('mostra o status vindo da presença', async () => {
    usePresenceStore().onSnapshot({
      users: [{ userId: 'u-ana', username: 'ana', displayName: 'ANA', status: 'waiting', roomName: 'Mesa do RH', privateRoom: false }],
    } as never)
    const { w } = await abrirPerfil()
    expect(w.text()).toContain('esperando · Mesa do RH')
    expect(w.text()).not.toContain('offline')
  })

  it('jogador novo sem partidas: estado vazio e sem link de histórico', async () => {
    profile.mockResolvedValue({ ...perfilAna, stats: { ...perfilAna.stats, matchesPlayed: 0 }, recentMatches: [], partners: [] })
    const { w } = await abrirPerfil()
    expect(w.text()).toContain('Ainda sem partidas')
    expect(w.find('[data-testid="estatisticas"]').exists()).toBe(false)
    expect(w.find('[data-testid="posicoes"]').exists()).toBe(false)
    expect(w.find('a[href="/jogadores/ana/partidas"]').exists()).toBe(false)
    expect(w.find('[data-testid="parceiros"]').exists()).toBe(false)
  })

  it('sou eu: "Editar conta" leva a /perfil e não há Convidar', async () => {
    profile.mockResolvedValue({ ...perfilAna, username: 'eu', displayName: 'EU' })
    useRoomStore().onRoomState({
      id: 'r1', code: 'ABC234', name: 'Sala', visibility: 'private', host: 0, status: 'waiting',
      seats: [null, null, null, null], yourSeat: 0,
    } as never)
    const { w } = await abrirPerfil('eu')
    expect(w.find('a[href="/perfil"]').text()).toBe('Editar conta')
    expect(w.text()).not.toContain('Convidar pra minha sala')
  })

  it('outra pessoa com sala aberta: Convidar pra minha sala envia o convite', async () => {
    useRoomStore().onRoomState({
      id: 'r1', code: 'ABC234', name: 'Sala', visibility: 'private', host: 0, status: 'waiting',
      seats: [null, null, null, null], yourSeat: 0,
    } as never)
    const invite = vi.spyOn(useInvitesStore(), 'invite').mockResolvedValue()
    const { w } = await abrirPerfil()
    expect(w.find('a[href="/perfil"]').exists()).toBe(false)
    const botao = w.findAll('button').find((b) => b.text() === 'Convidar pra minha sala')
    await botao?.trigger('click')
    expect(invite).toHaveBeenCalledWith('ana')
  })

  it('falha no convite vira toast', async () => {
    useRoomStore().onRoomState({
      id: 'r1', code: 'ABC234', name: 'Sala', visibility: 'private', host: 0, status: 'waiting',
      seats: [null, null, null, null], yourSeat: 0,
    } as never)
    vi.spyOn(useInvitesStore(), 'invite').mockRejectedValue(Object.assign(new Error('x'), { code: 'user_not_found' }))
    const { w } = await abrirPerfil()
    await w.findAll('button').find((b) => b.text() === 'Convidar pra minha sala')?.trigger('click')
    await flushPromises()
    expect(useToastsStore().items.at(-1)?.text).toBe('Usuário não encontrado.')
  })

  it('pessoa já sentada na minha sala: sem Convidar', async () => {
    usePresenceStore().onSnapshot({
      users: [{ userId: 'u-ana', username: 'ana', displayName: 'ANA', status: 'waiting', privateRoom: true }],
    } as never)
    useRoomStore().onRoomState({
      id: 'r1', code: 'ABC234', name: 'Sala', visibility: 'private', host: 0, status: 'waiting',
      seats: [{ userId: 'id-eu', displayName: 'EU', ready: false }, { userId: 'u-ana', displayName: 'ANA', ready: false }, null, null], yourSeat: 0,
    } as never)
    const { w } = await abrirPerfil()
    expect(w.text()).not.toContain('Convidar pra minha sala')
    // controle positivo: quem não está sentado recebe o botão
    usePresenceStore().onUpdated({ user: { userId: 'u-bia', username: 'bia', displayName: 'BIA', status: 'lobby', privateRoom: false } } as never)
    profile.mockResolvedValueOnce({ ...perfilAna, username: 'bia', displayName: 'BIA' })
    await w.vm.$router.push('/jogadores/bia')
    await flushPromises()
    expect(w.findAll('button').some((b) => b.text() === 'Convidar pra minha sala')).toBe(true)
  })

  it('trocar de jogador na mesma instância: mostra b e ignora a resposta atrasada de a', async () => {
    let resolveA: (p: Profile) => void = () => {}
    profile.mockImplementationOnce(() => new Promise((r) => { resolveA = r }))
    const { w, router } = await abrirPerfil('ana')
    profile.mockResolvedValueOnce({ ...perfilAna, username: 'bia', displayName: 'BIA' })
    await router.push('/jogadores/bia')
    await flushPromises()
    expect(w.find('h1').text()).toBe('BIA')
    resolveA(perfilAna)
    await flushPromises()
    expect(w.find('h1').text()).toBe('BIA')
    expect(profile).toHaveBeenLastCalledWith('bia')
  })

  it('sem sala com lugar livre não há Convidar', async () => {
    const { w } = await abrirPerfil()
    expect(w.text()).not.toContain('Convidar pra minha sala')
  })

  it('404: "Jogador não encontrado", sem toast', async () => {
    profile.mockRejectedValue(new ApiError(404, 'not_found', 'x'))
    const { w } = await abrirPerfil('fulano')
    expect(w.text()).toContain('Jogador não encontrado')
    expect(useToastsStore().items).toHaveLength(0)
  })

  it('erro: toast, mensagem e Tentar de novo', async () => {
    profile.mockRejectedValueOnce(new ApiError(500, 'internal', 'x'))
    const { w } = await abrirPerfil()
    expect(w.text()).toContain('Não foi possível carregar o perfil.')
    expect(useToastsStore().items).toHaveLength(1)
    await w.findAll('button').find((b) => b.text() === 'Tentar de novo')?.trigger('click')
    await flushPromises()
    expect(w.find('h1').text()).toBe('ANA')
  })

  it('carregando: aria-busy', async () => {
    profile.mockReturnValue(new Promise(() => {}))
    const { w } = await abrirPerfil()
    expect(w.find('[data-testid="perfil-carregando"]').attributes('aria-busy')).toBe('true')
  })
})

describe('LinhaPartida', () => {
  const montar = (item: HistoryItem, viewer = 'ana') => mount(LinhaPartida, { props: { item, viewer } })

  it('vitória: placar orientado e parceiro/adversários', () => {
    const w = montar(partida())
    expect(w.text()).toContain('Vitória')
    expect(w.text()).toContain('12×7')
    expect(w.text()).toContain('com CAIO')
    expect(w.text()).toContain('contra BIA e DUDA')
  })

  it('time 1 vê score[1]×score[0] e o parceiro é o assento oposto via teamOf', () => {
    const item = partida({ team: 1, won: false, winnerTeam: 0, score: [12, 7], players: jogadores('x', 'ana', 'y', 'z') })
    const w = montar(item)
    expect(w.text()).toContain('Derrota')
    expect(w.text()).toContain('7×12')
    expect(w.text()).toContain('com Z')
    expect(w.text()).toContain('contra X e Y')
  })

  it('assentos fora de ordem: parceiro não vem de seat % 2 do índice', () => {
    const players = [
      { seat: 3, username: 'duda', displayName: 'DUDA', bot: false },
      { seat: 2, username: 'ana', displayName: 'ANA', bot: false },
      { seat: 1, username: 'bia', displayName: 'BIA', bot: false },
      { seat: 0, username: 'caio', displayName: 'CAIO', bot: false },
    ]
    const w = montar(partida({ players }))
    expect(w.text()).toContain('com CAIO')
    expect(w.text()).toContain('contra BIA e DUDA')
  })

  it('W.O. no lugar do placar', () => {
    const w = montar(partida({ endReason: 'forfeit' }))
    expect(w.text()).toContain('W.O.')
    expect(w.text()).not.toContain('12×7')
  })

  it('abortada mostra "Abortada" e não declara vitória', () => {
    const w = montar(partida({ endReason: 'aborted', won: false, winnerTeam: null, score: null, status: 'abandoned' }))
    expect(w.text()).toContain('Abortada')
    expect(w.text()).not.toContain('Derrota')
  })
})

describe('histórico', () => {
  it('Carregar mais pede a próxima página com nextCursor e some no fim', async () => {
    history.mockResolvedValueOnce({ items: [partida({ id: 'a' })], nextCursor: 'c2' })
    history.mockResolvedValueOnce({ items: [partida({ id: 'b' })] })
    const { w } = await abrirPartidas()
    expect(w.findAll('[data-testid="partida"]')).toHaveLength(1)
    const mais = () => w.findAll('button').find((b) => b.text() === 'Carregar mais')
    await mais()?.trigger('click')
    await flushPromises()
    expect(history).toHaveBeenLastCalledWith('ana', 'c2')
    expect(w.findAll('[data-testid="partida"]')).toHaveLength(2)
    expect(mais()).toBeUndefined()
  })

  it('vazio, 404 e erro', async () => {
    history.mockResolvedValueOnce({ items: [] })
    const { w } = await abrirPartidas()
    expect(w.text()).toContain('Ainda sem partidas')
    history.mockRejectedValueOnce(new ApiError(404, 'not_found', 'x'))
    const b = await abrirPartidas('fulano')
    expect(b.w.text()).toContain('Jogador não encontrado')
  })

  it('404 do histórico sem toast; 500 com toast e Tentar de novo', async () => {
    history.mockRejectedValueOnce(new ApiError(404, 'not_found', 'x'))
    const a = await abrirPartidas('fulano')
    expect(a.w.text()).toContain('Jogador não encontrado')
    expect(useToastsStore().items).toHaveLength(0)
    history.mockRejectedValueOnce(new ApiError(500, 'internal', 'x'))
    const { w } = await abrirPartidas()
    expect(w.text()).toContain('Não foi possível carregar as partidas.')
    expect(useToastsStore().items).toHaveLength(1)
    history.mockResolvedValueOnce({ items: [partida()] })
    await w.findAll('button').find((b) => b.text() === 'Tentar de novo')?.trigger('click')
    await flushPromises()
    expect(w.findAll('[data-testid="partida"]')).toHaveLength(1)
  })

  it('trocar de jogador: lista de b, history(b) sem cursor, resposta atrasada de a ignorada', async () => {
    let resolveA: (p: { items: HistoryItem[]; nextCursor?: string }) => void = () => {}
    history.mockImplementationOnce(() => new Promise((r) => { resolveA = r }))
    const { w, router } = await abrirPartidas('ana')
    history.mockResolvedValueOnce({ items: [partida({ id: 'b1' }), partida({ id: 'b2' })] })
    await router.push('/jogadores/bia/partidas')
    await flushPromises()
    expect(history).toHaveBeenLastCalledWith('bia', undefined)
    expect(w.findAll('[data-testid="partida"]')).toHaveLength(2)
    resolveA({ items: [partida({ id: 'a1' })], nextCursor: 'x' })
    await flushPromises()
    expect(w.findAll('[data-testid="partida"]')).toHaveLength(2)
    expect(w.findAll('button').some((b) => b.text() === 'Carregar mais')).toBe(false)
  })

  it('Carregar mais em voo em a, troca para b: botão de b habilitado e pede o cursor de b', async () => {
    history.mockResolvedValueOnce({ items: [partida({ id: 'a1' })], nextCursor: 'ca' })
    const { w, router } = await abrirPartidas('ana')
    let resolveA: (p: { items: HistoryItem[]; nextCursor?: string }) => void = () => {}
    history.mockImplementationOnce(() => new Promise((r) => { resolveA = r }))
    await w.findAll('button').find((b) => b.text() === 'Carregar mais')?.trigger('click')
    history.mockResolvedValueOnce({ items: [partida({ id: 'b1' })], nextCursor: 'cb' })
    await router.push('/jogadores/bia/partidas')
    await flushPromises()
    resolveA({ items: [partida({ id: 'a2' })] })
    await flushPromises()
    const mais = w.findAll('button').find((b) => b.text() === 'Carregar mais')
    expect(mais?.attributes('disabled')).toBeUndefined()
    history.mockResolvedValueOnce({ items: [partida({ id: 'b2' })] })
    await mais?.trigger('click')
    await flushPromises()
    expect(history).toHaveBeenLastCalledWith('bia', 'cb')
    expect(w.findAll('[data-testid="partida"]')).toHaveLength(2)
  })

  it('erro ao carregar mais mantém a lista e dá toast', async () => {
    history.mockResolvedValueOnce({ items: [partida({ id: 'a' })], nextCursor: 'c2' })
    const { w } = await abrirPartidas()
    history.mockRejectedValueOnce(new ApiError(500, 'internal', 'x'))
    await w.findAll('button').find((b) => b.text() === 'Carregar mais')?.trigger('click')
    await flushPromises()
    expect(w.findAll('[data-testid="partida"]')).toHaveLength(1)
    expect(useToastsStore().items).toHaveLength(1)
  })
})

describe('navegação', () => {
  it('cabeçalho: o nome leva a /jogadores/<meu username>', async () => {
    const router = createAppRouter(createMemoryHistory())
    await router.push('/ranking')
    const w = mount(AppShell, { global: { plugins: [router], stubs: { RouterView: true } } })
    wrappers.push(w)
    expect(w.find('a[href="/jogadores/eu"]').text()).toContain('EU')
    expect(w.find('a[href="/perfil"]').exists()).toBe(false)
  })

  it('Online agora usa a rota nomeada jogador', async () => {
    const router = createAppRouter(createMemoryHistory())
    usePresenceStore().onSnapshot({
      users: [{ userId: 'u-ana', username: 'ana', displayName: 'ANA', status: 'lobby', privateRoom: false }],
    } as never)
    const w = mount(OnlineAgora, { global: { plugins: [router] } })
    wrappers.push(w)
    expect(w.find('a[href="/jogadores/ana"]').exists()).toBe(true)
    expect(router.resolve({ name: 'jogador', params: { username: 'ana' } }).href).toBe('/jogadores/ana')
    expect(router.resolve({ name: 'jogador-partidas', params: { username: 'ana' } }).href).toBe('/jogadores/ana/partidas')
  })
})

describe('LinhaPartida com bots', () => {
  const montar = (item: HistoryItem) => mount(LinhaPartida, { props: { item, viewer: 'ana' } })

  it('partida ranked=false mostra "com bots"; ranked=true não', () => {
    expect(montar(partida({ ranked: false })).text()).toContain('com bots')
    expect(montar(partida({ ranked: true })).text()).not.toContain('com bots')
  })

  it('jogador bot aparece com selo e sem link de perfil', () => {
    const players = jogadores('ana', 'bot_seu_ze', 'caio', 'bot_dona_cida').map((p) => ({ ...p, bot: p.username.startsWith('bot_') }))
    const w = montar(partida({ ranked: false, players }))
    expect(w.findAll('[data-selo-bot]')).toHaveLength(2)
    expect(w.find('a').exists()).toBe(false)
    expect(w.text()).toContain('BOT_SEU_ZE')
  })
})
