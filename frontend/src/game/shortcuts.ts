export interface ShortcutContext {
  canPlay: boolean
  canCover: boolean
  canRaise: boolean
  mustRespondRaise: boolean
  canRaiseBack: boolean
  handSize: number
}

export type ShortcutAction =
  | { kind: 'play'; index: number }
  | { kind: 'toggleCover' }
  | { kind: 'raise' }
  | { kind: 'vote'; choice: 'accept' | 'run' | 'raise' }

/** Teclas do brief §6.4. Só devolve ações que o contexto (vindo dos getters) permite. */
export function shortcutAction(key: string, ctx: ShortcutContext): ShortcutAction | null {
  const k = key.toLowerCase()
  if (k === '1' || k === '2' || k === '3') {
    const index = Number(k) - 1
    return ctx.canPlay && index < ctx.handSize ? { kind: 'play', index } : null
  }
  if (k === 'c') return ctx.canCover ? { kind: 'toggleCover' } : null
  if (k === 't') return ctx.canRaise ? { kind: 'raise' } : null
  if (k === 'a') return ctx.mustRespondRaise ? { kind: 'vote', choice: 'accept' } : null
  if (k === 'r') return ctx.mustRespondRaise ? { kind: 'vote', choice: 'run' } : null
  if (k === 'u') return ctx.mustRespondRaise && ctx.canRaiseBack ? { kind: 'vote', choice: 'raise' } : null
  return null
}

function isTyping(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  return target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName)
}

export function installShortcuts(handler: (a: ShortcutAction) => void, ctx: () => ShortcutContext): () => void {
  const onKey = (e: KeyboardEvent) => {
    if (e.ctrlKey || e.metaKey || e.altKey || e.repeat || isTyping(e.target) || isTyping(document.activeElement)) return
    if (document.querySelector('[aria-modal="true"]')) return // modal aberto: Esc e Tab são dele
    const action = shortcutAction(e.key, ctx())
    if (!action) return
    e.preventDefault()
    handler(action)
  }
  window.addEventListener('keydown', onKey)
  return () => window.removeEventListener('keydown', onKey)
}
