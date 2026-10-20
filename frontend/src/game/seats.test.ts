import { partnerOf, relativePosition, seatsInDisplayOrder, teamOf } from './seats'

it('roda os assentos a partir do meu, em sentido horário', () => {
  expect([0, 1, 2, 3].map((s) => relativePosition(s, 0))).toEqual(['bottom', 'left', 'top', 'right'])
  expect([0, 1, 2, 3].map((s) => relativePosition(s, 3))).toEqual(['left', 'top', 'right', 'bottom'])
  expect(seatsInDisplayOrder(2)).toEqual([2, 3, 0, 1])
})

it('calcula dupla e parceiro', () => {
  expect([0, 1, 2, 3].map(teamOf)).toEqual([0, 1, 0, 1])
  expect([0, 1, 2, 3].map(partnerOf)).toEqual([2, 3, 0, 1])
})
