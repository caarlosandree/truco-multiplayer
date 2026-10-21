import { installShortcuts, shortcutAction, type ShortcutContext } from './shortcuts'

const ctx = (over: Partial<ShortcutContext> = {}): ShortcutContext => ({
  canPlay: true, canCover: true, canRaise: true, mustRespondRaise: false, canRaiseBack: false, handSize: 3, ...over,
})

it('mapeia teclas para ações permitidas', () => {
  expect(shortcutAction('2', ctx())).toEqual({ kind: 'play', index: 1 })
  expect(shortcutAction('3', ctx({ handSize: 2 }))).toBeNull()
  expect(shortcutAction('c', ctx())).toEqual({ kind: 'toggleCover' })
  expect(shortcutAction('t', ctx({ canRaise: false }))).toBeNull()
  expect(shortcutAction('a', ctx({ mustRespondRaise: true }))).toEqual({ kind: 'vote', choice: 'accept' })
  expect(shortcutAction('R', ctx({ mustRespondRaise: true }))).toEqual({ kind: 'vote', choice: 'run' })
  expect(shortcutAction('u', ctx({ mustRespondRaise: true, canRaiseBack: false }))).toBeNull()
  expect(shortcutAction('u', ctx({ mustRespondRaise: true, canRaiseBack: true }))).toEqual({ kind: 'vote', choice: 'raise' })
  expect(shortcutAction('1', ctx({ canPlay: false }))).toBeNull()
})

it('ignora teclas digitadas em campos de texto, com modificadores ou com modal aberto', () => {
  const handler = vi.fn()
  const off = installShortcuts(handler, () => ctx())
  const input = document.createElement('input')
  document.body.append(input)
  input.dispatchEvent(new KeyboardEvent('keydown', { key: '1', bubbles: true }))
  window.dispatchEvent(new KeyboardEvent('keydown', { key: '1', ctrlKey: true }))
  const modal = document.createElement('div')
  modal.setAttribute('aria-modal', 'true')
  document.body.append(modal)
  window.dispatchEvent(new KeyboardEvent('keydown', { key: '1' }))
  expect(handler).not.toHaveBeenCalled()
  modal.remove()
  window.dispatchEvent(new KeyboardEvent('keydown', { key: '1' }))
  expect(handler).toHaveBeenCalledOnce()
  off()
  window.dispatchEvent(new KeyboardEvent('keydown', { key: '1' }))
  expect(handler).toHaveBeenCalledOnce()
  input.remove()
})
