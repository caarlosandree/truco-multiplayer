import { backUrl, cardUrl, CARD_FILES } from './cardAssets'

it.each(['frances', 'espanhol'] as const)('baralho %s tem as 40 cartas e o verso', (deck) => {
  for (let id = 0; id < 40; id++) expect(cardUrl(id, deck)).not.toBe('')
  expect(backUrl(deck)).not.toBe('')
  expect(Object.keys(CARD_FILES[deck])).toHaveLength(41)
})
