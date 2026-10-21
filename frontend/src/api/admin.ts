import { request } from './http'

export interface AdminUser {
  id: string
  username: string
  email: string
  displayName: string
  isAdmin: boolean
  emailVerified: boolean
  disabled: boolean
  mustResetPassword: boolean
  createdAt: string
}

export interface AdminDomain {
  domain: string
  createdAt: string
  createdBy: string | null
}

export interface LiveMatchPlayer {
  seat: number
  username: string
  displayName: string
}

export interface LiveMatch {
  id: string
  createdAt: string
  players: LiveMatchPlayer[]
}

export interface AuditEntry {
  id: string
  actorId: string
  actorUsername: string
  action: string
  targetType: string
  targetId: string
  /** Objeto livre por ação (ex.: `{ reason }`, `{ via: 'cli' }`); `null` quando não há detalhes. */
  details: Record<string, unknown> | null
  createdAt: string
}

export interface Page<T> {
  items: T[]
  nextCursor?: string
}

function query(params: Record<string, string | undefined>): string {
  const q = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) if (v) q.set(k, v)
  const s = q.toString()
  return s ? `?${s}` : ''
}

const user = (id: string) => `/api/admin/users/${encodeURIComponent(id)}`

export const adminApi = {
  users: (q: string, cursor?: string) => request<Page<AdminUser>>('GET', `/api/admin/users${query({ q, cursor })}`),
  disable: (id: string) => request<undefined>('POST', `${user(id)}/disable`),
  enable: (id: string) => request<undefined>('POST', `${user(id)}/enable`),
  forceReset: (id: string) => request<undefined>('POST', `${user(id)}/force-password-reset`),
  domains: async () => (await request<{ items: AdminDomain[] }>('GET', '/api/admin/email-domains')).items,
  addDomain: (domain: string) => request<undefined>('POST', '/api/admin/email-domains', { domain }),
  removeDomain: (domain: string) => request<undefined>('DELETE', `/api/admin/email-domains/${encodeURIComponent(domain)}`),
  liveMatches: (cursor?: string) =>
    request<Page<LiveMatch>>('GET', `/api/admin/matches${query({ status: 'in_progress', cursor })}`),
  abort: (id: string, reason: string) =>
    request<undefined>('POST', `/api/admin/matches/${encodeURIComponent(id)}/abort`, { reason }),
  audit: (cursor?: string) => request<Page<AuditEntry>>('GET', `/api/admin/audit${query({ cursor })}`),
}
