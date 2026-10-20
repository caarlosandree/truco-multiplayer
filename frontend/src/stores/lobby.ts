import { defineStore } from 'pinia'
import type { LobbySnapshotPayload, RoomRemovedPayload, RoomUpsertedPayload } from '@/protocol/messages'

export const useLobbyStore = defineStore('lobby', {
  state: () => ({}),
  actions: {
    onSnapshot(_p: LobbySnapshotPayload) {},
    onUpsert(_p: RoomUpsertedPayload) {},
    onRemove(_p: RoomRemovedPayload) {},
  },
})
