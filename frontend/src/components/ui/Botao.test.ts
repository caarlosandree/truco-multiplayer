import { mount } from '@vue/test-utils'
import Botao from './Botao.vue'

it('carregando: aria-busy, desabilitado e o rótulo continua na árvore de acessibilidade', () => {
  const w = mount(Botao, { props: { loading: true }, slots: { default: 'Entrar' } })
  const b = w.get('button')
  expect(b.attributes('aria-busy')).toBe('true')
  expect(b.attributes('disabled')).toBeDefined()
  expect(b.text()).toContain('Entrar')
  expect(w.get('[data-testid="botao-carregando"]').attributes('aria-hidden')).toBe('true')
  expect(w.find('.invisible').exists()).toBe(false)
})

it('normal: sem aria-busy e clicável', async () => {
  const w = mount(Botao, { slots: { default: 'Ok' } })
  expect(w.get('button').attributes('aria-busy')).toBeUndefined()
  await w.get('button').trigger('click')
  expect(w.emitted('click')).toHaveLength(1)
})
