import { deadlineFraction, remainingMs } from './deadline'

it('calcula o restante e a fração sem passar de 0..1', () => {
  expect(remainingMs(undefined, 0)).toBeNull()
  expect(remainingMs(10_000, 4_000)).toBe(6_000)
  expect(remainingMs(10_000, 12_000)).toBe(0)
  expect(deadlineFraction(30_000, 30_000, 15_000)).toBe(0.5)
  expect(deadlineFraction(30_000, 30_000, 60_000)).toBe(0)
  expect(deadlineFraction(90_000, 30_000, 0)).toBe(1)
})

it('total inválido ou sem prazo dá fração 0', () => {
  expect(deadlineFraction(undefined, 30_000, 0)).toBe(0)
  expect(deadlineFraction(10_000, 0, 0)).toBe(0)
})
