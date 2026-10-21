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
      class="sticky top-0 z-(--z-cabecalho) flex min-h-14 items-center gap-4 border-b-2 border-line/50 bg-surface px-4 md:px-6"
    >
      <RouterLink
        to="/"
        class="inline-flex h-11 items-center gap-2 rounded-botao-2 text-2xl font-bold tracking-tight"
        aria-label="Truco, ir para as salas"
      >
        <!-- quatro riscos e a diagonal: o tento de giz da lousa em tamanho de logo -->
        <svg
          viewBox="0 0 32 24"
          class="h-6 w-8 shrink-0"
          fill="none"
          stroke="currentColor"
          stroke-width="2.5"
          stroke-linecap="round"
          aria-hidden="true"
        >
          <path d="M6 3v18M13 3v18M20 3v18M27 3v18M2 17L31 7" />
        </svg>
        <span aria-hidden="true">Truco</span>
      </RouterLink>
      <IndicadorConexao class="ml-auto" />
      <RouterLink
        to="/perfil"
        class="inline-flex h-11 items-center gap-2 rounded-botao-2 pr-3 pl-1 font-bold hover:bg-surface-2"
      >
        <span
          class="textura-lousa grid size-9 place-items-center rounded-placa border border-line text-sm text-giz"
          aria-hidden="true"
        >{{ session.user?.displayName.charAt(0).toUpperCase() }}</span>
        <span class="max-w-40 truncate">{{ session.user?.displayName }}</span>
      </RouterLink>
    </header>
    <div
      v-if="session.versionMismatch"
      class="z-(--z-banner) flex items-center gap-3 bg-surface-2 px-4 py-2 text-sm md:px-6"
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
      <span class="min-w-0 flex-1">Nova versão disponível — recarregue a página</span>
      <Botao
        variant="secondary"
        class="shrink-0"
        @click="reload"
      >
        Recarregar
      </Botao>
    </div>
    <main
      id="conteudo"
      tabindex="-1"
      class="flex flex-1 flex-col focus:outline-none"
    >
      <RouterView />
    </main>
    <Toasts />
  </div>
</template>
