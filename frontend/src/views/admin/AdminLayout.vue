<script setup lang="ts">
import { RouterLink, RouterView } from 'vue-router'

const SECOES = [
  { name: 'admin-usuarios', label: 'Usuários' },
  { name: 'admin-dominios', label: 'Domínios' },
  { name: 'admin-partidas', label: 'Partidas em andamento' },
  { name: 'admin-auditoria', label: 'Auditoria' },
] as const
</script>

<template>
  <div class="mx-auto flex w-full max-w-6xl flex-col gap-4 px-4 py-6 md:px-6 md:py-8">
    <h1 class="text-2xl font-bold">
      Painel
    </h1>
    <nav
      aria-label="Seções do painel"
      class="-mx-1 flex flex-wrap gap-x-1 border-b-2 border-line/50"
    >
      <RouterLink
        v-for="s in SECOES"
        :key="s.name"
        :to="{ name: s.name }"
        class="inline-flex h-11 items-center border-b-4 border-transparent px-3 font-bold text-ink-muted hover:text-ink"
        active-class="border-ink! text-ink!"
      >
        {{ s.label }}
      </RouterLink>
    </nav>
    <RouterView />
  </div>
</template>
