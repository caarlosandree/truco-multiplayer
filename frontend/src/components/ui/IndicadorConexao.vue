<script setup lang="ts">
import { useSessionStore } from '@/stores/session'
import Botao from './Botao.vue'

const session = useSessionStore()
</script>

<template>
  <div
    class="flex items-center gap-3 text-sm"
    role="status"
  >
    <span
      v-if="session.connection === 'reconnecting' || session.connection === 'connecting'"
      class="flex items-center gap-2 text-ink-muted"
    >
      <span
        class="size-2 animate-pulse rounded-full bg-ink-muted"
        aria-hidden="true"
      />
      Reconectando…
    </span>
    <span
      v-else-if="session.connection === 'replaced'"
      class="flex items-center gap-2"
    >
      Aberto em outra aba
      <Botao
        variant="secondary"
        @click="session.resumeConnection()"
      >Usar aqui</Botao>
    </span>
  </div>
</template>
