import type { Seat, Team } from '@/protocol/engine'

export type RelativePosition = 'bottom' | 'left' | 'top' | 'right'
const POSITIONS = ['bottom', 'left', 'top', 'right'] as const satisfies readonly RelativePosition[]

export function relativePosition(seat: Seat, mySeat: Seat): RelativePosition {
  const position = POSITIONS[(seat - mySeat + 4) % 4]
  if (!position) throw new RangeError(`assento inválido: ${seat}`)
  return position
}

export function seatsInDisplayOrder(mySeat: Seat): Seat[] {
  return [0, 1, 2, 3].map((offset) => (mySeat + offset) % 4)
}

export function teamOf(seat: Seat): Team {
  return seat % 2
}
