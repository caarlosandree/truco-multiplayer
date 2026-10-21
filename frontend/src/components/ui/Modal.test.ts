import { mount } from '@vue/test-utils'
import Modal from './Modal.vue'

function dialog(): HTMLElement {
  const el = document.querySelector<HTMLElement>('[role="dialog"]')
  if (!el) throw new Error('diálogo não encontrado')
  return el
}

it('foca o diálogo, fecha com Esc e devolve o foco', async () => {
  const opener = document.createElement('button')
  document.body.append(opener)
  opener.focus()
  const w = mount(Modal, {
    props: { open: true, title: 'Confirmar' },
    slots: { default: '<button>ok</button>' },
    attachTo: document.body,
  })
  await w.vm.$nextTick()
  await w.vm.$nextTick()
  expect(document.activeElement?.textContent).toBe('ok')
  expect(dialog().getAttribute('aria-labelledby')).toBeTruthy()
  dialog().dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
  expect(w.emitted('close')).toHaveLength(1)
  await w.setProps({ open: false })
  expect(document.activeElement).toBe(opener)
  w.unmount()
})

it('prende o foco com Tab', async () => {
  const w = mount(Modal, {
    props: { open: true, title: 'Confirmar' },
    slots: { default: '<button>a</button><button>b</button>' },
    attachTo: document.body,
  })
  await w.vm.$nextTick()
  await w.vm.$nextTick()
  const buttons = dialog().querySelectorAll('button')
  buttons[1]?.focus()
  dialog().dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true }))
  expect(document.activeElement).toBe(buttons[0])
  w.unmount()
})

it('Shift+Tab com o foco no próprio diálogo continua dentro dele', async () => {
  const w = mount(Modal, {
    props: { open: true, title: 'Confirmar' },
    slots: { default: '<button>a</button><button>b</button>' },
    attachTo: document.body,
  })
  await w.vm.$nextTick()
  await w.vm.$nextTick()
  dialog().focus()
  const ev = new KeyboardEvent('keydown', { key: 'Tab', shiftKey: true, bubbles: true, cancelable: true })
  dialog().dispatchEvent(ev)
  expect(ev.defaultPrevented).toBe(true)
  expect(document.activeElement).toBe(dialog().querySelectorAll('button')[1])
  w.unmount()
})
