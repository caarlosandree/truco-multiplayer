import { mount } from '@vue/test-utils'
import App from './App.vue'

it('monta o app', () => {
  expect(mount(App).text()).toContain('Truco')
})
