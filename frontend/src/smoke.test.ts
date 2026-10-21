import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory } from 'vue-router'
import AppShell from './components/AppShell.vue'
import { createAppRouter } from './router'
import { useSessionStore } from './stores/session'
import { authApi } from './api/auth'

const ana = { id: 'u1', username: 'ana', email: '', displayName: 'Ana', isAdmin: false, createdAt: '' }

async function montar() {
  const pinia = createPinia()
  setActivePinia(pinia)
  vi.spyOn(authApi, 'me').mockResolvedValue(ana)
  const router = createAppRouter(createMemoryHistory())
  await router.push('/')
  await router.isReady()
  return mount(AppShell, { global: { plugins: [pinia, router] } })
}

it('monta o shell logado com o nome do usuário', async () => {
  const w = await montar()
  expect(w.text()).toContain('Truco')
  expect(w.text()).toContain('Ana')
})

it('mostra o banner de nova versão', async () => {
  const w = await montar()
  expect(w.find('[data-testid="banner-versao"]').exists()).toBe(false)
  useSessionStore().versionMismatch = true
  await w.vm.$nextTick()
  expect(w.get('[data-testid="banner-versao"]').text()).toContain('Nova versão disponível — recarregue a página')
})

it('o conteúdo principal recebe foco programático (link de pular)', async () => {
  const w = await montar()
  const main = w.get('main#conteudo')
  expect(main.attributes('tabindex')).toBe('-1')
})
