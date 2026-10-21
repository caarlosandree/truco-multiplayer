import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory } from 'vue-router'
import { createAppRouter } from '@/router'
import EntrarView from './EntrarView.vue'
import CadastroView from './CadastroView.vue'
import VerificarEmailView from './VerificarEmailView.vue'
import EsqueciSenhaView from './EsqueciSenhaView.vue'
import RedefinirSenhaView from './RedefinirSenhaView.vue'
import PerfilView from './PerfilView.vue'
import { authApi, type User } from '@/api/auth'
import { ApiError } from '@/api/http'
import { useSessionStore } from '@/stores/session'
import { useToastsStore } from '@/stores/toasts'

const ana: User = { id: 'u1', username: 'ana', email: 'ana@e.com', displayName: 'Ana', isAdmin: false, createdAt: '' }

async function mountAt(component: object, path: string, user: User | null = null) {
  const pinia = createPinia()
  setActivePinia(pinia)
  if (user) vi.spyOn(authApi, 'me').mockResolvedValue(user)
  else vi.spyOn(authApi, 'me').mockRejectedValue(new ApiError(401, 'unauthenticated', 'x'))
  const router = createAppRouter(createMemoryHistory())
  await router.push(path)
  const w = mount(component, { global: { plugins: [pinia, router] }, attachTo: document.body })
  return { w, router }
}

async function preencheLogin(w: Awaited<ReturnType<typeof mountAt>>['w'], login: string, senha: string) {
  await w.find('input[autocomplete="username"]').setValue(login)
  await w.find('input[type="password"]').setValue(senha)
  await w.find('form').trigger('submit')
  await flushPromises()
}

afterEach(() => {
  vi.restoreAllMocks()
  document.body.innerHTML = ''
})

it('login com erro mostra o texto do código', async () => {
  vi.spyOn(authApi, 'login').mockRejectedValue(new ApiError(401, 'invalid_credentials', 'x'))
  const { w } = await mountAt(EntrarView, '/entrar')
  await preencheLogin(w, 'ana', 'senha-errada')
  expect(w.text()).toContain('Usuário ou senha incorretos.')
  expect(w.find('[role="alert"]').exists()).toBe(true)
})

it('login limitado mostra "Tente de novo em N s"', async () => {
  vi.spyOn(authApi, 'login').mockRejectedValue(new ApiError(429, 'too_many_attempts', 'x', {}, 30))
  const { w } = await mountAt(EntrarView, '/entrar')
  await preencheLogin(w, 'ana', 'x')
  expect(w.text()).toContain('Tente de novo em 30 s')
})

it('e-mail não verificado oferece reenviar', async () => {
  vi.spyOn(authApi, 'login').mockRejectedValue(new ApiError(403, 'email_not_verified', 'x'))
  const resend = vi.spyOn(authApi, 'resendVerification').mockResolvedValue(undefined)
  const { w } = await mountAt(EntrarView, '/entrar')
  await preencheLogin(w, 'ana@e.com', 'x')
  await w.findAll('button').find((b) => b.text().includes('Reenviar'))!.trigger('click')
  expect(resend).toHaveBeenCalledWith('ana@e.com')
})

it('e-mail não verificado com usuário pede o e-mail para reenviar', async () => {
  vi.spyOn(authApi, 'login').mockRejectedValue(new ApiError(403, 'email_not_verified', 'x'))
  const resend = vi.spyOn(authApi, 'resendVerification').mockResolvedValue(undefined)
  const { w } = await mountAt(EntrarView, '/entrar')
  await preencheLogin(w, 'ana', 'x')
  await w.find('input[type="email"]').setValue('ana@e.com')
  await w.findAll('button').find((b) => b.text().includes('Reenviar'))!.trigger('click')
  expect(resend).toHaveBeenCalledWith('ana@e.com')
})

it('login ok volta para o destino interno', async () => {
  vi.spyOn(authApi, 'login').mockResolvedValue(ana)
  const { w, router } = await mountAt(EntrarView, '/entrar?volta=/perfil')
  await preencheLogin(w, 'ana', 'senha-certa-123')
  await vi.waitFor(() => expect(router.currentRoute.value.path).toBe('/perfil'))
})

it('cadastro mostra erros de campo do servidor', async () => {
  vi.spyOn(authApi, 'register').mockRejectedValue(
    new ApiError(400, 'validation_failed', 'x', { password: 'a senha deve ter de 10 a 128 caracteres' }),
  )
  const { w } = await mountAt(CadastroView, '/cadastro')
  await w.find('form').trigger('submit')
  await flushPromises()
  expect(w.text()).toContain('a senha deve ter de 10 a 128 caracteres')
})

it('cadastro bloqueia senhas diferentes sem chamar a API', async () => {
  const register = vi.spyOn(authApi, 'register').mockResolvedValue(undefined)
  const { w } = await mountAt(CadastroView, '/cadastro')
  const [nova, confirma] = w.findAll('input[type="password"]')
  await nova!.setValue('senha-segura-123')
  await confirma!.setValue('outra-coisa-123')
  await w.find('form').trigger('submit')
  await flushPromises()
  expect(register).not.toHaveBeenCalled()
  expect(w.text()).toContain('As senhas não conferem.')
})

it('cadastro: só um campo começa com "Senha"', async () => {
  const { w } = await mountAt(CadastroView, '/cadastro')
  const rotulos = w.findAll('label').map((l) => l.text())
  expect(rotulos.filter((t) => /^Senha/.test(t))).toHaveLength(1)
  expect(rotulos).toContain('Confirmar senha')
})

it('cadastro ok mostra "confira seu e-mail"', async () => {
  vi.spyOn(authApi, 'register').mockResolvedValue(undefined)
  const { w } = await mountAt(CadastroView, '/cadastro')
  await w.find('form').trigger('submit')
  await flushPromises()
  expect(w.text()).toContain('Confira seu e-mail')
})

it('verificação faz POST do token ao abrir e trata link expirado com reenvio', async () => {
  const verify = vi.spyOn(authApi, 'verifyEmail').mockRejectedValue(new ApiError(400, 'invalid_token', 'x'))
  const resend = vi.spyOn(authApi, 'resendVerification').mockResolvedValue(undefined)
  const { w } = await mountAt(VerificarEmailView, '/verificar-email?token=abc')
  await flushPromises()
  expect(verify).toHaveBeenCalledWith('abc')
  expect(w.text()).toContain('Link inválido ou expirado.')
  await w.find('input[type="email"]').setValue('ana@e.com')
  await w.find('form').trigger('submit')
  await flushPromises()
  expect(resend).toHaveBeenCalledWith('ana@e.com')
})

it('verificação ok mostra confirmação', async () => {
  vi.spyOn(authApi, 'verifyEmail').mockResolvedValue(undefined)
  const { w } = await mountAt(VerificarEmailView, '/verificar-email?token=abc')
  await flushPromises()
  expect(w.text()).toContain('E-mail confirmado')
})

it('esqueci a senha responde sempre de forma neutra', async () => {
  const forgot = vi.spyOn(authApi, 'forgotPassword').mockResolvedValue(undefined)
  const { w } = await mountAt(EsqueciSenhaView, '/esqueci-senha')
  await w.find('input[type="email"]').setValue('x@e.com')
  await w.find('form').trigger('submit')
  await flushPromises()
  expect(forgot).toHaveBeenCalledWith('x@e.com')
  expect(w.text()).toContain('Se o e-mail existir')
})

it('redefinição envia token e nova senha', async () => {
  const reset = vi.spyOn(authApi, 'resetPassword').mockResolvedValue(undefined)
  const { w } = await mountAt(RedefinirSenhaView, '/redefinir-senha?token=tok')
  const [nova, confirma] = w.findAll('input[type="password"]')
  await nova!.setValue('nova-senha-segura')
  await confirma!.setValue('nova-senha-segura')
  await w.find('form').trigger('submit')
  await flushPromises()
  expect(reset).toHaveBeenCalledWith('tok', 'nova-senha-segura')
  expect(w.text()).toContain('Senha redefinida')
})

it('redefinição bloqueia senhas diferentes sem chamar a API', async () => {
  const reset = vi.spyOn(authApi, 'resetPassword').mockResolvedValue(undefined)
  const { w } = await mountAt(RedefinirSenhaView, '/redefinir-senha?token=tok')
  const [nova, confirma] = w.findAll('input[type="password"]')
  await nova!.setValue('nova-senha-segura')
  await confirma!.setValue('outra-senha-qualquer')
  await w.find('form').trigger('submit')
  expect(reset).not.toHaveBeenCalled()
  expect(w.text()).toContain('As senhas não conferem.')
})

it('perfil: salva o nome, troca a senha e mostra erro de senha atual', async () => {
  const upd = vi.spyOn(authApi, 'updateMe').mockResolvedValue({ ...ana, displayName: 'Ana B' })
  const chg = vi.spyOn(authApi, 'changePassword').mockRejectedValue(new ApiError(401, 'invalid_credentials', 'x'))
  const { w } = await mountAt(PerfilView, '/perfil', ana)
  await useSessionStore().load()
  const forms = w.findAll('form')
  await forms[0]!.find('input').setValue('Ana B')
  await forms[0]!.trigger('submit')
  await flushPromises()
  expect(upd).toHaveBeenCalledWith('Ana B')
  const [atual, nova] = forms[1]!.findAll('input')
  await atual!.setValue('velha-senha-1')
  await nova!.setValue('nova-senha-1234')
  await forms[1]!.trigger('submit')
  await flushPromises()
  expect(chg).toHaveBeenCalledWith('velha-senha-1', 'nova-senha-1234')
  expect(w.text()).toContain('Senha atual incorreta.')
})

it('perfil: sair de todos os dispositivos pede confirmação', async () => {
  const all = vi.spyOn(authApi, 'logoutAll').mockResolvedValue(undefined)
  const { w, router } = await mountAt(PerfilView, '/perfil', ana)
  await w.findAll('button').find((b) => b.text().includes('Sair de todos'))!.trigger('click')
  await flushPromises()
  expect(all).not.toHaveBeenCalled()
  const dialog = document.querySelector('[role="dialog"]')!
  expect(dialog).toBeTruthy()
  const confirmar = [...dialog.querySelectorAll('button')].find((b) => b.textContent?.includes('Sair de todos'))!
  confirmar.click()
  await flushPromises()
  expect(all).toHaveBeenCalled()
  await vi.waitFor(() => expect(router.currentRoute.value.name).toBe('entrar'))
})

it('perfil: créditos e prefs', async () => {
  const { w } = await mountAt(PerfilView, '/perfil', ana)
  expect(w.text()).toContain('Créditos')
  expect(w.text()).toContain('Kenney')
  expect(w.findAll('input[type="radio"]')).toHaveLength(2)
  expect(w.find('input[type="range"]').exists()).toBe(true)
})

it('erro de rede no reenvio vira toast', async () => {
  vi.spyOn(authApi, 'login').mockRejectedValue(new ApiError(403, 'email_not_verified', 'x'))
  vi.spyOn(authApi, 'resendVerification').mockRejectedValue(new ApiError(0, 'network', 'x'))
  const { w } = await mountAt(EntrarView, '/entrar')
  await preencheLogin(w, 'ana@e.com', 'x')
  await w.findAll('button').find((b) => b.text().includes('Reenviar'))!.trigger('click')
  await flushPromises()
  expect(useToastsStore().items.at(-1)?.text).toBe('Sem conexão com o servidor.')
})
