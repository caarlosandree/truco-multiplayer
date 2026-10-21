import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createMemoryHistory } from 'vue-router'
import { ApiError } from '@/api/http'
import { adminApi, type AdminUser, type AuditEntry, type LiveMatch } from '@/api/admin'
import { createAppRouter } from '@/router'
import { useSessionStore } from '@/stores/session'
import { useToastsStore } from '@/stores/toasts'
import AdminAuditoriaView from './AdminAuditoriaView.vue'
import AdminDominiosView from './AdminDominiosView.vue'
import AdminPartidasView from './AdminPartidasView.vue'
import AdminUsuariosView from './AdminUsuariosView.vue'

vi.mock('@/api/admin', () => ({
  adminApi: {
    users: vi.fn(),
    disable: vi.fn(),
    enable: vi.fn(),
    forceReset: vi.fn(),
    domains: vi.fn(),
    addDomain: vi.fn(),
    removeDomain: vi.fn(),
    liveMatches: vi.fn(),
    abort: vi.fn(),
    audit: vi.fn(),
  },
}))
const api = vi.mocked(adminApi)

const usuario = (username: string, over: Partial<AdminUser> = {}): AdminUser => ({
  id: `id-${username}`,
  username,
  email: `${username}@empresa.com`,
  displayName: username.toUpperCase(),
  isAdmin: false,
  emailVerified: true,
  disabled: false,
  mustResetPassword: false,
  createdAt: '2026-10-01T10:00:00Z',
  ...over,
})

const wrappers: VueWrapper[] = []
async function abrir(comp: Parameters<typeof mount>[0]) {
  const router = createAppRouter(createMemoryHistory())
  await router.push('/')
  const w = mount(comp, { global: { plugins: [router] }, attachTo: document.body })
  wrappers.push(w)
  await flushPromises()
  return w
}
const botoes = (w: VueWrapper) => w.findAll('button')
const botao = (w: VueWrapper, texto: string | RegExp) =>
  botoes(w).find((b) => (typeof texto === 'string' ? b.text() === texto : texto.test(b.text())))
const modal = () => document.querySelector<HTMLElement>('[role="dialog"]')
function noModal(texto: string) {
  return [...(modal()?.querySelectorAll('button') ?? [])].find((b) => b.textContent?.trim() === texto)
}

beforeEach(() => {
  setActivePinia(createPinia())
  Object.values(api).forEach((f) => f.mockReset())
  useSessionStore().user = { id: 'root', username: 'root', email: 'r@x.com', displayName: 'Root', isAdmin: true, createdAt: '' }
  useSessionStore().loaded = true
})
afterEach(() => {
  wrappers.splice(0).forEach((w) => w.unmount())
  document.body.innerHTML = ''
})

describe('usuários', () => {
  beforeEach(() => {
    api.users.mockResolvedValue({ items: [usuario('ana'), usuario('bia', { disabled: true }), usuario('root', { isAdmin: true })] })
  })

  it('busca pelo termo digitado', async () => {
    const w = await abrir(AdminUsuariosView)
    expect(api.users).toHaveBeenLastCalledWith('', undefined)
    await w.find('input[type="search"], input').setValue('an')
    await w.find('form').trigger('submit')
    await flushPromises()
    expect(api.users).toHaveBeenLastCalledWith('an', undefined)
  })

  it('linha de admin vem marcada "admin" e sem botões; o texto diz que admin só vem do servidor', async () => {
    const w = await abrir(AdminUsuariosView)
    expect(w.text()).toContain('Admin é concedido só pelo servidor.')
    const linha = w.findAll('[data-testid="usuario"]').find((l) => l.text().includes('@root'))
    expect(linha?.text()).toContain('admin')
    expect(linha?.findAll('button')).toHaveLength(0)
  })

  it('nenhum botão tem "admin" no nome', async () => {
    const w = await abrir(AdminUsuariosView)
    for (const b of botoes(w)) {
      expect(`${b.text()} ${b.attributes('aria-label') ?? ''}`.toLowerCase()).not.toContain('admin')
    }
  })

  it('desativar pede confirmação em Modal e só então chama a store', async () => {
    api.disable.mockResolvedValue(undefined)
    const w = await abrir(AdminUsuariosView)
    await botao(w, 'Desativar')?.trigger('click')
    await flushPromises()
    expect(modal()).not.toBeNull()
    expect(api.disable).not.toHaveBeenCalled()
    noModal('Desativar')?.click()
    await flushPromises()
    expect(api.disable).toHaveBeenCalledWith('id-ana')
    expect(modal()).toBeNull()
    expect(botao(w, 'Reativar')).toBeDefined()
  })

  it('cancelar não chama a store', async () => {
    const w = await abrir(AdminUsuariosView)
    await botao(w, 'Desativar')?.trigger('click')
    await flushPromises()
    noModal('Cancelar')?.click()
    await flushPromises()
    expect(api.disable).not.toHaveBeenCalled()
    expect(modal()).toBeNull()
  })

  it('reativar e forçar troca de senha também confirmam', async () => {
    api.enable.mockResolvedValue(undefined)
    api.forceReset.mockResolvedValue(undefined)
    const w = await abrir(AdminUsuariosView)
    await botao(w, 'Reativar')?.trigger('click')
    await flushPromises()
    expect(api.enable).not.toHaveBeenCalled()
    noModal('Reativar')?.click()
    await flushPromises()
    expect(api.enable).toHaveBeenCalledWith('id-bia')
    await botao(w, 'Forçar troca de senha')?.trigger('click')
    await flushPromises()
    expect(api.forceReset).not.toHaveBeenCalled()
    noModal('Forçar troca')?.click()
    await flushPromises()
    expect(api.forceReset).toHaveBeenCalledWith('id-ana')
  })

  it('403 target_is_admin vira toast e o modal fecha', async () => {
    api.disable.mockRejectedValue(new ApiError(403, 'target_is_admin', 'x'))
    const w = await abrir(AdminUsuariosView)
    await botao(w, 'Desativar')?.trigger('click')
    await flushPromises()
    noModal('Desativar')?.click()
    await flushPromises()
    expect(useToastsStore().items[0]?.text).toBe('Não é possível agir sobre uma conta de administrador.')
    expect(modal()).toBeNull()
  })

  it('erro de carga mostra "Tentar de novo"; lista vazia mostra mensagem', async () => {
    api.users.mockRejectedValueOnce(new ApiError(500, 'internal', 'x'))
    const w = await abrir(AdminUsuariosView)
    expect(botao(w, 'Tentar de novo')).toBeDefined()
    api.users.mockResolvedValueOnce({ items: [] })
    await botao(w, 'Tentar de novo')?.trigger('click')
    await flushPromises()
    expect(w.text()).toContain('Nenhum usuário encontrado')
  })
})

describe('domínios', () => {
  beforeEach(() => {
    api.domains.mockResolvedValue([{ domain: 'empresa.com', createdAt: '2026-10-01T10:00:00Z', createdBy: null }])
  })

  it('adiciona um domínio e recarrega', async () => {
    api.addDomain.mockResolvedValue(undefined)
    const w = await abrir(AdminDominiosView)
    await w.find('input').setValue('novo.com')
    await w.find('form').trigger('submit')
    await flushPromises()
    expect(api.addDomain).toHaveBeenCalledWith('novo.com')
    expect(api.domains).toHaveBeenCalledTimes(2)
  })

  it('remover pede confirmação', async () => {
    api.removeDomain.mockResolvedValue(undefined)
    const w = await abrir(AdminDominiosView)
    await botao(w, 'Remover')?.trigger('click')
    await flushPromises()
    expect(api.removeDomain).not.toHaveBeenCalled()
    noModal('Remover')?.click()
    await flushPromises()
    expect(api.removeDomain).toHaveBeenCalledWith('empresa.com')
  })
})

describe('partidas em andamento', () => {
  const partida: LiveMatch = {
    id: 'm1',
    createdAt: '2026-10-21T12:00:00Z',
    players: [
      { seat: 0, username: 'ana', displayName: 'ANA' },
      { seat: 1, username: 'bia', displayName: 'BIA' },
    ],
  }

  it('lista jogadores e Abortar exige motivo antes de confirmar', async () => {
    api.liveMatches.mockResolvedValue({ items: [partida] })
    api.abort.mockResolvedValue(undefined)
    const w = await abrir(AdminPartidasView)
    expect(api.liveMatches).toHaveBeenCalledWith(undefined)
    expect(w.text()).toContain('ANA')
    await botao(w, 'Abortar')?.trigger('click')
    await flushPromises()
    const confirmar = noModal('Abortar partida') as HTMLButtonElement
    expect(confirmar.disabled).toBe(true)
    const campo = modal()?.querySelector('input, textarea') as HTMLInputElement
    campo.value = 'sala travada'
    campo.dispatchEvent(new Event('input'))
    await flushPromises()
    expect((noModal('Abortar partida') as HTMLButtonElement).disabled).toBe(false)
    noModal('Abortar partida')?.click()
    await flushPromises()
    expect(api.abort).toHaveBeenCalledWith('m1', 'sala travada')
  })

  it('vazio', async () => {
    api.liveMatches.mockResolvedValue({ items: [] })
    const w = await abrir(AdminPartidasView)
    expect(w.text()).toContain('Nenhuma partida em andamento')
  })
})

describe('auditoria', () => {
  const entrada = (id: string, over: Partial<AuditEntry> = {}): AuditEntry => ({
    id,
    actorId: 'a',
    actorUsername: 'root',
    action: 'match.abort',
    targetType: 'match',
    targetId: 'm1',
    details: { reason: 'travada' },
    createdAt: '2026-10-21T12:00:00Z',
    ...over,
  })

  it('Carregar mais usa o cursor e some no fim', async () => {
    api.audit.mockResolvedValueOnce({ items: [entrada('1')], nextCursor: 'k' })
    const w = await abrir(AdminAuditoriaView)
    expect(w.text()).toContain('travada')
    api.audit.mockResolvedValueOnce({ items: [entrada('2')] })
    await botao(w, 'Carregar mais')?.trigger('click')
    await flushPromises()
    expect(api.audit).toHaveBeenLastCalledWith('k')
    expect(w.findAll('[data-testid="auditoria"]')).toHaveLength(2)
    expect(botao(w, 'Carregar mais')).toBeUndefined()
  })

  it('ator com via cli aparece como "CLI do servidor"', async () => {
    api.audit.mockResolvedValue({
      items: [entrada('1', { actorUsername: 'ana', action: 'user.promote_admin', targetType: 'user', targetId: 'u9', details: { via: 'cli' } })],
    })
    const w = await abrir(AdminAuditoriaView)
    expect(w.text()).toContain('CLI do servidor')
    expect(w.text()).not.toContain('ana')
  })
})

describe('cabeçalho', () => {
  it('o link "Admin" aparece só para quem é admin', async () => {
    const AppShell = (await import('@/components/AppShell.vue')).default
    const links = async (isAdmin: boolean) => {
      const s = useSessionStore()
      if (s.user) s.user = { ...s.user, isAdmin }
      const w = await abrir(AppShell)
      return w.findAll('header a').map((a) => a.text())
    }
    expect(await links(true)).toContain('Admin')
    wrappers.splice(0).forEach((w) => w.unmount())
    expect(await links(false)).not.toContain('Admin')
  })
})
