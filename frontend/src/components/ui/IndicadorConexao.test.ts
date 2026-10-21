import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import IndicadorConexao from './IndicadorConexao.vue'
import { setResumeHandler, useSessionStore } from '@/stores/session'

beforeEach(() => setActivePinia(createPinia()))
afterEach(() => setResumeHandler(null))

it('mostra reconectando e oferece retomar quando aberto em outra aba', async () => {
  const resume = vi.fn()
  setResumeHandler(resume)
  const s = useSessionStore()
  const w = mount(IndicadorConexao)
  s.setConnection('reconnecting')
  await w.vm.$nextTick()
  expect(w.text()).toContain('Reconectando')
  s.setConnection('replaced')
  await w.vm.$nextTick()
  expect(w.text()).toContain('Aberto em outra aba')
  await w.find('button').trigger('click')
  expect(resume).toHaveBeenCalled()
})

it('não mostra nada com a conexão aberta', async () => {
  const s = useSessionStore()
  s.setConnection('open')
  const w = mount(IndicadorConexao)
  expect(w.text()).toBe('')
})
