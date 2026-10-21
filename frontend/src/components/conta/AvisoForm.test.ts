import { mount } from '@vue/test-utils'
import AvisoForm from './AvisoForm.vue'

beforeEach(() => vi.useFakeTimers())
afterEach(() => vi.useRealTimers())

it('conta regressivamente, emite fim e some ao chegar em zero', async () => {
  const w = mount(AvisoForm, { props: { retryAfterSec: 3 }, slots: { default: 'Muitas tentativas.' } })
  expect(w.find('[aria-hidden="true"].tabular-nums').text()).toContain('em 3 s')
  await vi.advanceTimersByTimeAsync(1000)
  expect(w.find('[aria-hidden="true"].tabular-nums').text()).toContain('em 2 s')
  expect(w.emitted('fim')).toBeUndefined()
  await vi.advanceTimersByTimeAsync(2000)
  expect(w.emitted('fim')).toHaveLength(1)
  expect(w.find('.tabular-nums').exists()).toBe(false)
  // o leitor de tela só ouviu o valor inicial, uma vez
  expect(w.get('.sr-only').text()).toContain('em 3 s')
})

it('limpa o intervalo ao desmontar', () => {
  const w = mount(AvisoForm, { props: { retryAfterSec: 5 }, slots: { default: 'x' } })
  expect(vi.getTimerCount()).toBe(1)
  w.unmount()
  expect(vi.getTimerCount()).toBe(0)
})
