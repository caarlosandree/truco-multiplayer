import { afterEach } from 'vitest'

// jsdom pode não ter requestAnimationFrame (usado pelo anel de prazo).
if (typeof globalThis.requestAnimationFrame !== 'function') {
  globalThis.requestAnimationFrame = (cb: FrameRequestCallback) =>
    setTimeout(() => cb(performance.now()), 16) as unknown as number
  globalThis.cancelAnimationFrame = (id: number) => clearTimeout(id)
}

afterEach(() => {
  document.body.innerHTML = ''
  try {
    localStorage.clear()
  } catch {
    // ambiente sem storage
  }
})
