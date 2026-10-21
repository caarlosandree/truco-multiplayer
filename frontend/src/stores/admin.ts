import { defineStore } from 'pinia'
import { adminApi, type AdminDomain, type AdminUser, type AuditEntry, type LiveMatch, type Page } from '@/api/admin'
import { useToastsStore } from './toasts'

export type { AdminDomain, AdminUser, AuditEntry, LiveMatch }

export type AdminStatus = 'idle' | 'loading' | 'ok' | 'error'

interface Lista<T> {
  items: T[]
  nextCursor: string | undefined
  status: AdminStatus
  /** Uma página a mais em voo ("Carregar mais"). */
  more: boolean
}

const vazia = <T>(): Lista<T> => ({ items: [], nextCursor: undefined, status: 'idle', more: false })

// Cada seção descarta a resposta de um pedido já superado por outro.
const seqs = { users: 0, domains: 0, matches: 0, audit: 0 }

export const useAdminStore = defineStore('admin', {
  state: () => ({
    users: { ...vazia<AdminUser>(), q: '' },
    domains: vazia<AdminDomain>(),
    matches: vazia<LiveMatch>(),
    audit: vazia<AuditEntry>(),
  }),
  actions: {
    // --- usuários ---
    async loadUsers(q: string) {
      this.users = { ...vazia<AdminUser>(), status: 'loading', q }
      const id = ++seqs.users
      try {
        const page = await adminApi.users(q, undefined)
        if (id !== seqs.users) return
        this.users = { items: page.items, nextCursor: page.nextCursor, status: 'ok', more: false, q }
      } catch (err) {
        if (id !== seqs.users) return
        this.users.status = 'error'
        useToastsStore().fromError(err)
      }
    },
    async loadMoreUsers() {
      const cursor = this.users.nextCursor
      if (!cursor || this.users.more) return
      const id = seqs.users
      this.users.more = true
      try {
        const page = await adminApi.users(this.users.q, cursor)
        if (id !== seqs.users) return
        this.users.items = [...this.users.items, ...page.items]
        this.users.nextCursor = page.nextCursor
      } catch (err) {
        if (id !== seqs.users) return
        useToastsStore().fromError(err)
      } finally {
        if (id === seqs.users) this.users.more = false
      }
    },
    patchUser(id: string, patch: Partial<AdminUser>) {
      this.users.items = this.users.items.map((u) => (u.id === id ? { ...u, ...patch } : u))
    },
    disableUser(id: string) {
      return this.act(() => adminApi.disable(id), () => this.patchUser(id, { disabled: true }))
    },
    enableUser(id: string) {
      return this.act(() => adminApi.enable(id), () => this.patchUser(id, { disabled: false }))
    },
    forceReset(id: string) {
      return this.act(() => adminApi.forceReset(id), () => this.patchUser(id, { mustResetPassword: true }))
    },

    // --- domínios ---
    async loadDomains() {
      this.domains = { ...vazia<AdminDomain>(), status: 'loading' }
      const id = ++seqs.domains
      try {
        const items = await adminApi.domains()
        if (id !== seqs.domains) return
        this.domains = { items, nextCursor: undefined, status: 'ok', more: false }
      } catch (err) {
        if (id !== seqs.domains) return
        this.domains.status = 'error'
        useToastsStore().fromError(err)
      }
    },
    addDomain(domain: string) {
      return this.act(() => adminApi.addDomain(domain), () => this.loadDomains())
    },
    removeDomain(domain: string) {
      return this.act(() => adminApi.removeDomain(domain), () => this.loadDomains())
    },

    // --- partidas em andamento ---
    async loadMatches() {
      this.matches = { ...vazia<LiveMatch>(), status: 'loading' }
      const id = ++seqs.matches
      try {
        const page = await adminApi.liveMatches(undefined)
        if (id !== seqs.matches) return
        this.matches = { items: page.items, nextCursor: page.nextCursor, status: 'ok', more: false }
      } catch (err) {
        if (id !== seqs.matches) return
        this.matches.status = 'error'
        useToastsStore().fromError(err)
      }
    },
    async loadMoreMatches() {
      await this.more('matches', (c) => adminApi.liveMatches(c))
    },
    abortMatch(id: string, reason: string) {
      return this.act(() => adminApi.abort(id, reason), () => this.loadMatches())
    },

    // --- auditoria ---
    async loadAudit() {
      this.audit = { ...vazia<AuditEntry>(), status: 'loading' }
      const id = ++seqs.audit
      try {
        const page = await adminApi.audit(undefined)
        if (id !== seqs.audit) return
        this.audit = { items: page.items, nextCursor: page.nextCursor, status: 'ok', more: false }
      } catch (err) {
        if (id !== seqs.audit) return
        this.audit.status = 'error'
        useToastsStore().fromError(err)
      }
    },
    async loadMoreAudit() {
      await this.more('audit', (c) => adminApi.audit(c))
    },

    // --- internos ---
    /** Próxima página de uma lista paginada (partidas ou auditoria). */
    async more<K extends 'matches' | 'audit'>(
      key: K,
      fetchPage: (cursor: string) => Promise<Page<K extends 'matches' ? LiveMatch : AuditEntry>>,
    ) {
      const lista = this[key]
      const cursor = lista.nextCursor
      if (!cursor || lista.more) return
      const id = seqs[key]
      lista.more = true
      try {
        const page = await fetchPage(cursor)
        if (id !== seqs[key]) return
        // as duas listas têm o mesmo formato; o cast só junta os dois tipos de item
        const atual = this[key] as Lista<LiveMatch | AuditEntry>
        atual.items = [...atual.items, ...page.items]
        atual.nextCursor = page.nextCursor
      } catch (err) {
        if (id !== seqs[key]) return
        useToastsStore().fromError(err)
      } finally {
        if (id === seqs[key]) this[key].more = false
      }
    },
    /** Executa a ação; sucesso roda `depois` e devolve true, falha vira toast e devolve false. */
    async act(fn: () => Promise<unknown>, depois: () => unknown): Promise<boolean> {
      try {
        await fn()
      } catch (err) {
        useToastsStore().fromError(err)
        return false
      }
      await depois()
      return true
    },
  },
})
