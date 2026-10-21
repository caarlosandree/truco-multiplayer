import { mount } from '@vue/test-utils'
import Campo from './Campo.vue'

it('liga o rótulo e emite o valor', async () => {
  const w = mount(Campo, { props: { label: 'Usuário', modelValue: '' } })
  const input = w.get('input')
  expect(w.get('label').attributes('for')).toBe(input.attributes('id'))
  expect(input.attributes('aria-invalid')).toBeUndefined()
  await input.setValue('ana')
  expect(w.emitted('update:modelValue')?.[0]).toEqual(['ana'])
})

it('com erro marca aria-invalid e aponta aria-describedby para a mensagem', () => {
  const w = mount(Campo, { props: { label: 'Senha', modelValue: '', error: 'Usuário ou senha incorretos.' } })
  const input = w.get('input')
  expect(input.attributes('aria-invalid')).toBe('true')
  const msg = w.get(`#${CSS.escape(input.attributes('aria-describedby') ?? '')}`)
  expect(msg.text()).toBe('Usuário ou senha incorretos.')
})
