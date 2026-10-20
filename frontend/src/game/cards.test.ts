import table from '@/protocol/cards.json'
import { cardFromId, cardLabel, cardFileName, rankLabel, suitName, suitSymbol } from './cards'

describe('cardFromId', () => {
  it('espelha engine.CardFromID para os 40 ids', () => {
    expect(table).toHaveLength(40)
    for (const row of table) {
      expect(cardFromId(row.id)).toEqual({ rank: row.rank, suit: row.suit })
    }
  })
})

describe('rótulos', () => {
  it('converte figuras e ás no baralho francês', () => {
    expect([1, 10, 11, 12, 7].map((r) => rankLabel(r, 'frances'))).toEqual(['A', 'J', 'Q', 'K', '7'])
  })
  it('mantém o número no baralho espanhol', () => {
    expect([1, 10, 11, 12, 7].map((r) => rankLabel(r, 'espanhol'))).toEqual(['1', '10', '11', '12', '7'])
  })
  it('nomeia naipes na ordem do engine', () => {
    expect([0, 1, 2, 3].map(suitName)).toEqual(['ouros', 'espadas', 'copas', 'paus'])
    expect([0, 1, 2, 3].map(suitSymbol)).toEqual(['♦', '♠', '♥', '♣'])
  })
  it('monta rótulo acessível e nome de arquivo', () => {
    const asDeEspadas = table.find((r) => r.rank === 1 && r.suit === 1)!.id
    expect(cardLabel(asDeEspadas, 'frances')).toBe('A de espadas')
    expect(cardLabel(asDeEspadas, 'espanhol')).toBe('1 de espadas')
    expect(cardFileName(asDeEspadas)).toBe('1-espadas.svg')
  })
})
