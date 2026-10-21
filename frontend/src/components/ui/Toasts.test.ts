import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import Toasts from './Toasts.vue'
import { useToastsStore } from '@/stores/toasts'

beforeEach(() => setActivePinia(createPinia()))

it('separa avisos (status/polite) de erros (alert)', async () => {
  const t = useToastsStore()
  const w = mount(Toasts, { global: { stubs: { TransitionGroup: false } } })
  t.push('Link copiado.', 'info')
  t.push('Deu ruim.', 'error')
  await w.vm.$nextTick()
  const status = w.get('[role="status"]')
  const alert = w.get('[role="alert"]')
  expect(status.attributes('aria-live')).toBe('polite')
  expect(status.text()).toContain('Link copiado.')
  expect(status.text()).not.toContain('Deu ruim.')
  expect(alert.text()).toContain('Deu ruim.')
})

it('mostra no máximo 3 avisos no total e fecha pelo botão', async () => {
  const t = useToastsStore()
  const w = mount(Toasts)
  for (let i = 1; i <= 4; i++) t.push(`aviso ${i}`, i % 2 ? 'info' : 'error')
  await w.vm.$nextTick()
  expect(w.findAll('button[aria-label="Fechar aviso"]')).toHaveLength(3)
  expect(w.text()).not.toContain('aviso 1')
  await w.get('button[aria-label="Fechar aviso"]').trigger('click')
  expect(t.items).toHaveLength(3)
})
