import { defineStore } from 'pinia'
import type { Seat } from '@/protocol/engine'
import type {
  LobbySnapshotPayload,
  RoomRemovedPayload,
  RoomSummary,
  RoomUpsertedPayload,
  Visibility,
} from '@/protocol/messages'
import { send } from '@/app/transport'

export function normalizeCode(raw: string): string {
  return raw.replace(/\s+/g, '').toUpperCase()
}

function hasFreeSeat(r: RoomSummary): boolean {
  // cadeira de bot também é vaga: o humano pode sentar no lugar dela
  return r.seats.some((s) => s === null || s.bot)
}

export const useLobbyStore = defineStore('lobby', {
  state: () => ({ rooms: {} as Record<string, RoomSummary>, loaded: false }),
  getters: {
    list(state): RoomSummary[] {
      const rank = (r: RoomSummary) => (r.status === 'waiting' && hasFreeSeat(r) ? 0 : 1)
      return Object.values(state.rooms).sort((a, b) => rank(a) - rank(b) || a.name.localeCompare(b.name, 'pt-BR'))
    },
  },
  actions: {
    /** Novo hello: o snapshot anterior é velho; `loaded` volta a valer só depois do próximo lobby_snapshot. */
    markStale() {
      this.loaded = false
    },
    onSnapshot(p: LobbySnapshotPayload) {
      this.rooms = Object.fromEntries(p.rooms.map((r) => [r.id, r]))
      this.loaded = true
    },
    onUpsert(p: RoomUpsertedPayload) {
      this.rooms[p.room.id] = p.room
    },
    onRemove(p: RoomRemovedPayload) {
      this.rooms = Object.fromEntries(Object.entries(this.rooms).filter(([id]) => id !== p.roomId))
    },
    createRoom(visibility: Visibility) {
      return send({ type: 'create_room', payload: { visibility } })
    },
    joinById(roomId: string, seat?: Seat) {
      return send({ type: 'join_room', payload: seat === undefined ? { roomId } : { roomId, seat } })
    },
    joinByCode(code: string, seat?: Seat) {
      const normalized = normalizeCode(code)
      return send({ type: 'join_room', payload: seat === undefined ? { code: normalized } : { code: normalized, seat } })
    },
  },
})
