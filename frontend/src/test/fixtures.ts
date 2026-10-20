import type { PlayerView } from '@/protocol/engine'
import type { StatePayload } from '@/protocol/messages'

export function makeView(over: Partial<PlayerView> = {}): PlayerView {
  return {
    seat: 0,
    phase: 'playing',
    handKind: 'normal',
    handNumber: 1,
    players: [0, 1, 2, 3].map((seat) => ({ seat, id: `u${seat}`, name: ['Ana', 'Bia', 'Caio', 'Duda'][seat]!, cardCount: 3, voted: false })),
    score: [0, 0],
    dealer: 3,
    turn: 0,
    stake: 1,
    table: [],
    rounds: [],
    deckCount: 28,
    discardCount: 0,
    myHand: [
      { slot: 0, card: 0, hidden: false },
      { slot: 1, card: 13, hidden: false },
      { slot: 2, card: 39, hidden: false },
    ],
    ...over,
  }
}

export function makeState(over: Partial<PlayerView> = {}, pendingSeats: number[] = [0]): StatePayload {
  return { view: makeView(over), deadlines: Object.fromEntries(pendingSeats.map((s) => [s, Date.now() + 30_000])), pendingSeats }
}
