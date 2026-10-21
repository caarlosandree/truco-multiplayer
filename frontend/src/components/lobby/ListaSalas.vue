<script setup lang="ts">
import type { Seat } from '@/protocol/engine'
import { useLobbyStore } from '@/stores/lobby'
import { useToastsStore } from '@/stores/toasts'
import Botao from '@/components/ui/Botao.vue'
import LinhaSala from './LinhaSala.vue'

const emit = defineEmits<{ criar: [] }>()
const lobby = useLobbyStore()
const toasts = useToastsStore()

async function join(roomId: string, seat?: Seat) {
  try {
    await lobby.joinById(roomId, seat)
  } catch (err) {
    toasts.fromError(err)
  }
}
</script>

<template>
  <section aria-labelledby="salas-titulo">
    <h2
      id="salas-titulo"
      class="sr-only"
    >
      Salas abertas
    </h2>

    <div
      v-if="!lobby.loaded"
      role="status"
      class="rounded-painel bg-surface-2"
    >
      <p class="sr-only">
        Carregando salas…
      </p>
      <div
        v-for="n in 3"
        :key="n"
        aria-hidden="true"
        class="esqueleto flex flex-col gap-2 border-t-2 border-line/40 px-4 py-5 first-of-type:border-t-0 md:px-6"
        :style="{ animationDelay: `${n * 120}ms` }"
      >
        <span class="h-4 w-40 rounded-placa bg-line/30" />
        <span class="h-3 w-24 rounded-placa bg-line/20" />
      </div>
    </div>

    <div
      v-else-if="lobby.list.length === 0"
      class="flex flex-col items-start gap-4 rounded-painel bg-surface-2 p-6 md:p-8"
    >
      <svg
        viewBox="0 0 120 8"
        class="h-2 w-30"
        fill="none"
        stroke-width="3"
        stroke-linecap="round"
        aria-hidden="true"
      >
        <!-- quatro lugares vazios: as marcas das duas duplas, ainda sem ninguém -->
        <path
          class="stroke-team-0"
          d="M2 4h20M62 4h20"
          opacity=".5"
        />
        <path
          class="stroke-team-1"
          d="M32 4h.1M42 4h.1M52 4h.1M92 4h.1M102 4h.1M112 4h.1"
          opacity=".5"
        />
      </svg>
      <div>
        <h3 class="text-xl font-bold">
          Nenhuma sala aberta
        </h3>
        <p class="mt-1 max-w-prose text-ink-muted">
          Crie uma e chame a turma.
        </p>
      </div>
      <Botao @click="emit('criar')">
        Criar sala
      </Botao>
    </div>

    <ul
      v-else
      class="rounded-painel bg-surface-2"
    >
      <LinhaSala
        v-for="r in lobby.list"
        :key="r.id"
        :room="r"
        @join="(seat) => join(r.id, seat)"
      />
    </ul>
  </section>
</template>

<style scoped>
.esqueleto {
  animation: esqueleto 1.4s var(--ease-padrao) infinite alternate;
}
@keyframes esqueleto {
  from { opacity: 0.5; }
  to { opacity: 1; }
}
@media (prefers-reduced-motion: reduce) {
  .esqueleto { animation: none; }
}
</style>
