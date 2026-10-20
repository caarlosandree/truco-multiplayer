import type { Card, CardID, Rank, Suit } from '@/protocol/engine'

export type DeckStyle = 'frances' | 'espanhol'

/** Mesma ordem de engine.rankLadder (do menor para o maior na força comum). */
export const RANK_LADDER: readonly Rank[] = [4, 5, 6, 7, 10, 11, 12, 1, 2, 3]

const SUIT_NAMES = ['ouros', 'espadas', 'copas', 'paus'] as const
const SUIT_SYMBOLS = ['♦', '♠', '♥', '♣'] as const
const FRENCH: Partial<Record<Rank, string>> = { 1: 'A', 10: 'J', 11: 'Q', 12: 'K' }

export function cardFromId(id: CardID): Card {
  if (!Number.isInteger(id) || id < 0) throw new RangeError(`CardID inválido: ${id}`)
  const rank = RANK_LADDER[Math.floor(id / 4)]
  if (rank === undefined) throw new RangeError(`CardID inválido: ${id}`)
  return { rank, suit: id % 4 }
}

export function rankLabel(rank: Rank, deck: DeckStyle): string {
  return deck === 'frances' ? (FRENCH[rank] ?? String(rank)) : String(rank)
}

export function suitName(suit: Suit): (typeof SUIT_NAMES)[number] {
  const name = SUIT_NAMES[suit]
  if (!name) throw new RangeError(`naipe inválido: ${suit}`)
  return name
}

export function suitSymbol(suit: Suit): (typeof SUIT_SYMBOLS)[number] {
  const symbol = SUIT_SYMBOLS[suit]
  if (!symbol) throw new RangeError(`naipe inválido: ${suit}`)
  return symbol
}

export function cardLabel(id: CardID, deck: DeckStyle): string {
  const { rank, suit } = cardFromId(id)
  return `${rankLabel(rank, deck)} de ${suitName(suit)}`
}

/** Nome do arquivo SVG; igual nos dois baralhos (usa o rank neutro). */
export function cardFileName(id: CardID): string {
  const { rank, suit } = cardFromId(id)
  return `${rank}-${suitName(suit)}.svg`
}
