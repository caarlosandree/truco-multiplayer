import { defineStore } from 'pinia'
import type { Event } from '@/protocol/engine'
import type { SignalBroadcastPayload, StatePayload } from '@/protocol/messages'

export const useGameStore = defineStore('game', {
  state: () => ({}),
  actions: {
    onState(_p: StatePayload) {},
    onEvents(_p: Event[]) {},
    onSignal(_p: SignalBroadcastPayload) {},
    reset() {},
  },
})
