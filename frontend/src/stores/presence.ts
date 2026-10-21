import { defineStore } from 'pinia'
import type {
  PresenceEntry,
  PresenceRemovedPayload,
  PresenceSnapshotPayload,
  PresenceUpdatedPayload,
} from '@/protocol/messages'

const RANK: Record<string, number> = { lobby: 0, waiting: 1, playing: 2 }
const rank = (e: PresenceEntry): number => RANK[e.status] ?? 3

export const usePresenceStore = defineStore('presence', {
  state: () => ({ users: {} as Record<string, PresenceEntry>, loaded: false }),
  getters: {
    list(state): PresenceEntry[] {
      return Object.values(state.users).sort(
        (a, b) => rank(a) - rank(b) || a.displayName.localeCompare(b.displayName, 'pt-BR'),
      )
    },
    count(state): number {
      return Object.keys(state.users).length
    },
    byUsername(state) {
      return (username: string): PresenceEntry | undefined => Object.values(state.users).find((e) => e.username === username)
    },
  },
  actions: {
    onSnapshot(p: PresenceSnapshotPayload) {
      this.users = Object.fromEntries(p.users.map((e) => [e.userId, e]))
      this.loaded = true
    },
    onUpdated(p: PresenceUpdatedPayload) {
      this.users[p.user.userId] = p.user
    },
    onRemoved(p: PresenceRemovedPayload) {
      this.users = Object.fromEntries(Object.entries(this.users).filter(([id]) => id !== p.userId))
    },
  },
})
