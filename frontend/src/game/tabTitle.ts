interface Alerta {
  texto: string
  parar: () => void
}

interface Grupo {
  base: string
  doc: Document
  win: Window
  timer: number
  passo: number
  alertas: Alerta[]
  aoFocar: () => void
}

/** Um único intervalo para todos os alertas ativos: eles se revezam no título (base → alerta 1 → base → alerta 2 …). */
let grupo: Grupo | null = null

function encerrar(g: Grupo) {
  g.win.clearInterval(g.timer)
  g.win.removeEventListener('focus', g.aoFocar)
  g.doc.title = g.base
  grupo = null
}

/**
 * Pisca o título da aba enquanto ela está sem foco (spec §8.2: "Sua vez!", "Convite: <sala>"). Não faz nada com a aba em foco.
 * Todas as chamadas compartilham um intervalo e se revezam entre si; parar um alerta não apaga os outros. A janela ganhando
 * foco para todos e restaura o título. Devolve a função de parada (idempotente) deste alerta.
 */
export function startTabTitleBlink(
  base: string,
  alert: string,
  opts: { intervalMs?: number; doc?: Document; win?: Window } = {},
): () => void {
  const doc = grupo?.doc ?? opts.doc ?? document
  const win = grupo?.win ?? opts.win ?? window
  if (doc.hasFocus()) return () => {}
  if (!grupo) {
    const g: Grupo = {
      base,
      doc,
      win,
      passo: 0,
      alertas: [],
      timer: 0,
      aoFocar: () => {
        for (const a of [...g.alertas]) a.parar()
      },
    }
    g.timer = win.setInterval(() => {
      // a aba pode ter ganhado foco sem disparar `focus` (troca de aba pelo teclado, p. ex.)
      if (g.doc.hasFocus()) {
        g.aoFocar()
        return
      }
      const n = g.alertas.length
      const k = g.passo++
      g.doc.title = k % 2 === 0 ? (g.alertas[(k / 2) % n]?.texto ?? g.base) : g.base
    }, opts.intervalMs ?? 1000)
    win.addEventListener('focus', g.aoFocar)
    grupo = g
  }
  const g = grupo
  const alerta: Alerta = {
    texto: alert,
    parar: () => {
      const i = g.alertas.indexOf(alerta)
      if (i < 0) return
      g.alertas.splice(i, 1)
      if (g.alertas.length === 0) encerrar(g)
      else g.doc.title = g.base
    },
  }
  g.alertas.push(alerta)
  return alerta.parar
}
