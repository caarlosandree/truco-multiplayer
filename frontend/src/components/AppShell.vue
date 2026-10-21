<script setup lang="ts">
import { RouterLink, RouterView } from 'vue-router'
import { useSessionStore } from '@/stores/session'
import Botao from './ui/Botao.vue'
import IndicadorConexao from './ui/IndicadorConexao.vue'
import Toasts from './ui/Toasts.vue'

const session = useSessionStore()
function reload() {
  location.reload()
}
</script>

<template>
  <div class="flex min-h-dvh flex-col bg-surface text-ink">
    <a
      href="#conteudo"
      class="sr-only focus:not-sr-only focus:absolute focus:top-2 focus:left-2 focus:z-(--z-modal) focus:rounded-botao focus:bg-surface focus:px-4 focus:py-2"
    >
      Pular para o conteúdo
    </a>
    <header
      v-if="session.isLoggedIn"
      class="sticky top-0 z-(--z-cabecalho) flex min-h-12 items-center gap-4 border-b border-line/40 bg-surface px-4 md:px-6"
    >
      <RouterLink
        to="/"
        class="inline-flex h-11 items-center rounded-botao-2 text-xl font-bold"
      >
        Truco
      </RouterLink>
      <IndicadorConexao class="ml-auto" />
      <RouterLink
        to="/perfil"
        class="inline-flex h-11 items-center rounded-botao-2 px-3 font-bold hover:bg-surface-2"
      >
        {{ session.user?.displayName }}
      </RouterLink>
    </header>
    <div
      v-if="session.versionMismatch"
      class="z-(--z-banner) flex flex-wrap items-center gap-3 bg-surface-2 px-4 py-2 text-sm md:px-6"
      role="status"
      data-testid="banner-versao"
    >
      <svg
        viewBox="0 0 16 16"
        class="size-4 shrink-0"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        aria-hidden="true"
      >
        <path d="M13 8a5 5 0 1 1-1.5-3.5M13 2v3h-3" />
      </svg>
      <span>Nova versão disponível — recarregue a página</span>
      <Botao
        variant="secondary"
        class="ml-auto"
        @click="reload"
      >
        Recarregar
      </Botao>
    </div>
    <main
      id="conteudo"
      class="flex flex-1 flex-col"
    >
      <RouterView />
    </main>
    <Toasts />
  </div>
</template>
