/**
 * Pisca o título da aba enquanto ela está sem foco (spec §8.2: "Sua vez!"). Não faz nada com a aba em foco.
 * Para sozinho quando a janela ganha foco; devolve a função de parada (idempotente) que restaura o título.
 */
export function startTabTitleBlink(
  base: string,
  alert: string,
  opts: { intervalMs?: number; doc?: Document; win?: Window } = {},
): () => void {
  const doc = opts.doc ?? document
  const win = opts.win ?? window
  if (doc.hasFocus()) return () => {}
  let on = false
  let ativo = true
  const stop = () => {
    if (!ativo) return
    ativo = false
    win.clearInterval(timer)
    win.removeEventListener('focus', stop)
    doc.title = base
  }
  const timer = win.setInterval(() => {
    // a aba pode ter ganhado foco sem disparar `focus` (troca de aba pelo teclado, p. ex.)
    if (doc.hasFocus()) {
      stop()
      return
    }
    on = !on
    doc.title = on ? alert : base
  }, opts.intervalMs ?? 1000)
  win.addEventListener('focus', stop)
  return stop
}
