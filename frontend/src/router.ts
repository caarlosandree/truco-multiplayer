import { createRouter, createWebHistory, type Router, type RouterHistory } from 'vue-router'
import { useSessionStore } from './stores/session'
import { normalizeCode } from './stores/lobby'

export function sanitizeReturnTo(raw: unknown): string {
  if (typeof raw !== 'string') return '/'
  // Só caminhos internos: começa com "/" e não com "//" ou "/\" (URLs protocol-relative).
  if (!raw.startsWith('/') || raw.startsWith('//') || raw.startsWith('/\\')) return '/'
  return raw
}

export function createAppRouter(history: RouterHistory = createWebHistory()): Router {
  const router = createRouter({
    history,
    routes: [
      { path: '/entrar', name: 'entrar', component: () => import('./views/EntrarView.vue'), meta: { public: true, guestOnly: true } },
      { path: '/cadastro', name: 'cadastro', component: () => import('./views/CadastroView.vue'), meta: { public: true, guestOnly: true } },
      { path: '/verificar-email', name: 'verificar-email', component: () => import('./views/VerificarEmailView.vue'), meta: { public: true } },
      { path: '/esqueci-senha', name: 'esqueci-senha', component: () => import('./views/EsqueciSenhaView.vue'), meta: { public: true } },
      { path: '/redefinir-senha', name: 'redefinir-senha', component: () => import('./views/RedefinirSenhaView.vue'), meta: { public: true } },
      { path: '/', name: 'lobby', component: () => import('./views/LobbyView.vue') },
      { path: '/sala', name: 'sala', component: () => import('./views/SalaView.vue') },
      {
        path: '/sala/:codigo',
        name: 'convite',
        component: () => import('./views/ConviteView.vue'),
        beforeEnter: (to) => {
          const code = normalizeCode(String(to.params.codigo))
          if (code !== to.params.codigo) return { name: 'convite', params: { codigo: code }, replace: true }
        },
      },
      { path: '/entrar-convite/:token', name: 'entrar-convite', component: () => import('./views/EntrarConviteView.vue') },
      { path: '/ranking', name: 'ranking', component: () => import('./views/RankingView.vue') },
      { path: '/jogadores/:username', name: 'jogador', component: () => import('./views/JogadorView.vue') },
      { path: '/jogadores/:username/partidas', name: 'jogador-partidas', component: () => import('./views/PartidasView.vue') },
      {
        path: '/admin',
        component: () => import('./views/admin/AdminLayout.vue'),
        meta: { admin: true },
        children: [
          { path: '', redirect: { name: 'admin-usuarios' } },
          { path: 'usuarios', name: 'admin-usuarios', component: () => import('./views/admin/AdminUsuariosView.vue') },
          { path: 'dominios', name: 'admin-dominios', component: () => import('./views/admin/AdminDominiosView.vue') },
          { path: 'partidas', name: 'admin-partidas', component: () => import('./views/admin/AdminPartidasView.vue') },
          { path: 'auditoria', name: 'admin-auditoria', component: () => import('./views/admin/AdminAuditoriaView.vue') },
        ],
      },
      { path: '/perfil', name: 'perfil', component: () => import('./views/PerfilView.vue') },
      { path: '/:pathMatch(.*)*', redirect: '/' },
    ],
  })

  router.beforeEach(async (to) => {
    const session = useSessionStore()
    if (!session.loaded) {
      try {
        await session.load()
      } catch {
        // falha que não é 401 (ex.: rede fora): segue sem sessão; rotas privadas caem em /entrar
      }
    }
    if (!to.meta.public && !session.isLoggedIn) {
      // os guards globais rodam antes do beforeEnter: normaliza o código do convite aqui também
      const volta = to.name === 'convite' ? `/sala/${normalizeCode(String(to.params.codigo))}` : to.fullPath
      return { name: 'entrar', query: { volta } }
    }
    // a proteção real está no servidor; aqui só evitamos mostrar o painel a quem não é admin
    if (to.meta.admin && !session.user?.isAdmin) return { name: 'lobby' }
    if (to.meta.guestOnly && session.isLoggedIn) return sanitizeReturnTo(to.query.volta)
  })

  return router
}
