import { defineStore } from 'pinia'
import { authApi, type User } from '@/api/auth'
import { ApiError } from '@/api/http'
import type { WsStatus } from '@/ws/client'

let resumeHandler: (() => void) | null = null

/** Injetado pelo wire (installConnection) para que stores/componentes retomem a conexão sem importar o wire. */
export function setResumeHandler(fn: (() => void) | null): void {
  resumeHandler = fn
}

export const useSessionStore = defineStore('session', {
  state: () => ({
    user: null as User | null,
    loaded: false,
    connection: 'idle' as WsStatus,
    versionMismatch: false,
  }),
  getters: {
    isLoggedIn: (s) => s.user !== null,
  },
  actions: {
    async load() {
      try {
        this.user = await authApi.me()
      } catch (err) {
        if (err instanceof ApiError && err.status === 401) this.user = null
        else throw err
      } finally {
        this.loaded = true
      }
    },
    async login(login: string, password: string) {
      this.user = await authApi.login(login, password)
      this.loaded = true
    },
    async logout() {
      try {
        await authApi.logout()
      } catch {
        // ignora: sair localmente vale mesmo se a API falhar
      } finally {
        this.clear()
      }
    },
    async logoutAll() {
      await authApi.logoutAll()
      this.clear()
    },
    async updateDisplayName(name: string) {
      this.user = await authApi.updateMe(name)
    },
    async changePassword(current: string, next: string) {
      await authApi.changePassword(current, next)
    },
    clear() {
      this.user = null
    },
    setConnection(s: WsStatus) {
      this.connection = s
    },
    resumeConnection() {
      resumeHandler?.()
    },
  },
})
