import type { CardID } from '@/protocol/engine'
import { cardFileName, type DeckStyle } from './cards'

const frances = import.meta.glob('@/assets/cards/frances/*.svg', { eager: true, query: '?url', import: 'default' }) as Record<string, string>
const espanhol = import.meta.glob('@/assets/cards/espanhol/*.svg', { eager: true, query: '?url', import: 'default' }) as Record<string, string>

function byFile(table: Record<string, string>): Record<string, string> {
  return Object.fromEntries(Object.entries(table).map(([path, url]) => [path.slice(path.lastIndexOf('/') + 1), url]))
}

export const CARD_FILES: Record<DeckStyle, Record<string, string>> = { frances: byFile(frances), espanhol: byFile(espanhol) }

export function cardUrl(id: CardID, deck: DeckStyle): string {
  return CARD_FILES[deck][cardFileName(id)] ?? ''
}

export function backUrl(deck: DeckStyle): string {
  return CARD_FILES[deck]['verso.svg'] ?? ''
}
