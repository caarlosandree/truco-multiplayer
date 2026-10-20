import { defineStore } from 'pinia'
import type { ForfeitAvailablePayload, LeftRoomPayload, MatchEndedPayload, RoomStatePayload } from '@/protocol/messages'

export const useRoomStore = defineStore('room', {
  state: () => ({}),
  actions: {
    onRoomState(_p: RoomStatePayload) {},
    onLeft(_p: LeftRoomPayload) {},
    onForfeit(_p: ForfeitAvailablePayload) {},
    onMatchEnded(_p: MatchEndedPayload) {},
  },
})
