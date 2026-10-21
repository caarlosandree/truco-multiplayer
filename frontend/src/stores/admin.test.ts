import { createPinia, setActivePinia } from 'pinia'
import { ApiError } from '@/api/http'
import { adminApi, type AdminUser, type AuditEntry } from '@/api/admin'
import { useAdminStore } from './admin'
import { useToastsStore } from './toasts'

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

const usuario = (id: string, over: Partial<AdminUser> = {}): AdminUser => ({
  id,
  username: id,
  email: `${id}@x.com`,
  displayName: id.toUpperCase(),
  isAdmin: false,
  emailVerified: true,
  disabled: false,
  mustResetPassword: false,
  createdAt: '2026-10-01T10:00:00Z',
  ...over,
})
const entrada = (id: string): AuditEntry => ({
  id,
  actorId: 'a',
  actorUsername: 'root',
  action: 'user.disable',
  targetType: 'user',
  targetId: 'u1',
  details: null,
  createdAt: '2026-10-02T10:00:00Z',
})

beforeEach(() => {
  setActivePinia(createPinia())
  Object.values(api).forEach((f) => f.mockReset())
})

describe('usuários', () => {
  it('busca com q e guarda a página; loadMoreUsers usa o cursor e acrescenta', async () => {
    api.users.mockResolvedValueOnce({ items: [usuario('a')], nextCursor: 'c1' })
    const s = useAdminStore()
    await s.loadUsers('an')
    expect(api.users).toHaveBeenLastCalledWith('an', undefined)
    expect(s.users.items.map((u) => u.id)).toEqual(['a'])
    expect(s.users.status).toBe('ok')
    api.users.mockResolvedValueOnce({ items: [usuario('b')] })
    await s.loadMoreUsers()
    expect(api.users).toHaveBeenLastCalledWith('an', 'c1')
    expect(s.users.items.map((u) => u.id)).toEqual(['a', 'b'])
    expect(s.users.nextCursor).toBeUndefined()
  })

  it('resposta de busca superada é descartada', async () => {
    let soltar: (v: { items: AdminUser[] }) => void = () => {}
    api.users.mockReturnValueOnce(new Promise((r) => (soltar = r)))
    api.users.mockResolvedValueOnce({ items: [usuario('novo')] })
    const s = useAdminStore()
    const velha = s.loadUsers('a')
    await s.loadUsers('b')
    soltar({ items: [usuario('velho')] })
    await velha
    expect(s.users.items.map((u) => u.id)).toEqual(['novo'])
  })

  it('erro na carga vira status error e toast', async () => {
    api.users.mockRejectedValue(new ApiError(500, 'internal', 'x'))
    const s = useAdminStore()
    await s.loadUsers('')
    expect(s.users.status).toBe('error')
    expect(useToastsStore().items).toHaveLength(1)
  })

  it('desativar, reativar e forçar troca atualizam a linha e devolvem true', async () => {
    api.users.mockResolvedValue({ items: [usuario('a')] })
    const s = useAdminStore()
    await s.loadUsers('')
    expect(await s.disableUser('a')).toBe(true)
    expect(api.disable).toHaveBeenCalledWith('a')
    expect(s.users.items[0]?.disabled).toBe(true)
    expect(await s.enableUser('a')).toBe(true)
    expect(s.users.items[0]?.disabled).toBe(false)
    expect(await s.forceReset('a')).toBe(true)
    expect(s.users.items[0]?.mustResetPassword).toBe(true)
  })

  it('403 target_is_admin vira toast e devolve false sem mudar a linha', async () => {
    api.users.mockResolvedValue({ items: [usuario('a')] })
    api.disable.mockRejectedValue(new ApiError(403, 'target_is_admin', 'x'))
    const s = useAdminStore()
    await s.loadUsers('')
    expect(await s.disableUser('a')).toBe(false)
    expect(s.users.items[0]?.disabled).toBe(false)
    expect(useToastsStore().items[0]?.text).toBe('Não é possível agir sobre uma conta de administrador.')
  })
})

describe('domínios', () => {
  it('lista, adiciona e remove recarregando', async () => {
    api.domains.mockResolvedValue([{ domain: 'a.com', createdAt: '', createdBy: null }])
    const s = useAdminStore()
    await s.loadDomains()
    expect(s.domains.items.map((d) => d.domain)).toEqual(['a.com'])
    expect(await s.addDomain('b.com')).toBe(true)
    expect(api.addDomain).toHaveBeenCalledWith('b.com')
    expect(api.domains).toHaveBeenCalledTimes(2)
    expect(await s.removeDomain('a.com')).toBe(true)
    expect(api.removeDomain).toHaveBeenCalledWith('a.com')
    expect(api.domains).toHaveBeenCalledTimes(3)
  })

  it('erro ao adicionar vira toast e false', async () => {
    api.addDomain.mockRejectedValue(new ApiError(400, 'invalid_request', 'x'))
    const s = useAdminStore()
    expect(await s.addDomain('???')).toBe(false)
    expect(useToastsStore().items).toHaveLength(1)
  })
})

describe('partidas e auditoria', () => {
  it('aborta com motivo e recarrega a lista', async () => {
    api.liveMatches.mockResolvedValue({ items: [] })
    const s = useAdminStore()
    expect(await s.abortMatch('m1', 'travada')).toBe(true)
    expect(api.abort).toHaveBeenCalledWith('m1', 'travada')
    expect(api.liveMatches).toHaveBeenCalledTimes(1)
  })

  it('auditoria pagina com cursor', async () => {
    api.audit.mockResolvedValueOnce({ items: [entrada('1')], nextCursor: 'k' })
    const s = useAdminStore()
    await s.loadAudit()
    api.audit.mockResolvedValueOnce({ items: [entrada('2')] })
    await s.loadMoreAudit()
    expect(api.audit).toHaveBeenLastCalledWith('k')
    expect(s.audit.items.map((e) => e.id)).toEqual(['1', '2'])
  })
})
