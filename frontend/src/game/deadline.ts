/** Milissegundos que faltam para o prazo (epoch ms), ou null quando não há prazo. Nunca negativo. */
export function remainingMs(deadline: number | undefined, now: number): number | null {
  if (deadline === undefined) return null
  return Math.max(0, deadline - now)
}

/** Fração 0..1 do prazo que ainda resta, dado o total em ms. */
export function deadlineFraction(deadline: number | undefined, totalMs: number, now: number): number {
  const left = remainingMs(deadline, now)
  if (left === null || totalMs <= 0) return 0
  return Math.min(1, Math.max(0, left / totalMs))
}
