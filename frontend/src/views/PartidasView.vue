<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { isNotFound, usePlayersStore, type HistoryItem } from '@/stores/players'
import { useToastsStore } from '@/stores/toasts'
import Botao from '@/components/ui/Botao.vue'
import BotaoLink from '@/components/ui/BotaoLink.vue'
import LinhaPartida from '@/components/partidas/LinhaPartida.vue'

type Estado = 'carregando' | 'ok' | 'nao-encontrado' | 'erro'

const route = useRoute()
const players = usePlayersStore()
const toasts = useToastsStore()

const estado = ref<Estado>('carregando')
const itens = ref<HistoryItem[]>([])
const cursor = ref<string | undefined>(undefined)
const carregandoMais = ref(false)
let seq = 0

const username = computed(() => String(route.params.username ?? ''))
/** O nome de exibição vem das próprias partidas (a pessoa está em todas); sem elas, o @. */
const nome = computed(() => itens.value[0]?.players.find((p) => p.username === username.value)?.displayName ?? `@${username.value}`)

async function carregar() {
  const id = ++seq
  estado.value = 'carregando'
  itens.value = []
  cursor.value = undefined
  carregandoMais.value = false
  try {
    const page = await players.loadHistory(username.value)
    if (id !== seq) return
    itens.value = page.items
    cursor.value = page.nextCursor
    estado.value = 'ok'
  } catch (err) {
    if (id !== seq) return
    if (isNotFound(err)) {
      estado.value = 'nao-encontrado'
      return
    }
    estado.value = 'erro'
    toasts.fromError(err)
  }
}

async function carregarMais() {
  const atual = cursor.value
  if (atual === undefined || carregandoMais.value) return
  const id = seq
  carregandoMais.value = true
  try {
    const page = await players.loadHistory(username.value, atual)
    if (id !== seq) return
    itens.value = [...itens.value, ...page.items]
    cursor.value = page.nextCursor
  } catch (err) {
    if (id === seq) toasts.fromError(err)
  } finally {
    if (id === seq) carregandoMais.value = false
  }
}

watch(username, () => void carregar(), { immediate: true })
</script>

<template>
  <div class="mx-auto flex w-full max-w-[65ch] flex-col gap-4 px-4 py-6 md:px-6 md:py-8">
    <RouterLink
      :to="{ name: 'jogador', params: { username } }"
      class="inline-flex h-11 items-center self-start font-bold underline decoration-2 underline-offset-4"
    >
      Voltar ao perfil
    </RouterLink>

    <div
      v-if="estado === 'carregando'"
      class="flex flex-col gap-3"
      data-testid="partidas-carregando"
      aria-busy="true"
    >
      <p class="sr-only">
        Carregando as partidas
      </p>
      <div
        v-for="n in 5"
        :key="n"
        aria-hidden="true"
        class="esqueleto h-16 rounded-placa bg-surface-2"
        :style="{ animationDelay: `${n * 120}ms` }"
      />
    </div>

    <div
      v-else-if="estado === 'nao-encontrado'"
      class="flex flex-col items-start gap-3 rounded-painel bg-surface-2 p-5"
    >
      <h1 class="text-2xl font-bold">
        Jogador não encontrado
      </h1>
      <p class="text-ink-muted">
        Essa pessoa não existe ou não está mais jogando.
      </p>
      <BotaoLink
        to="/ranking"
        variant="secondary"
      >
        Ver o ranking
      </BotaoLink>
    </div>

    <div
      v-else-if="estado === 'erro'"
      class="flex flex-col items-start gap-3 rounded-painel bg-surface-2 p-5"
    >
      <h1 class="text-2xl font-bold">
        Não foi possível carregar as partidas.
      </h1>
      <p class="text-ink-muted">
        Confira sua conexão e tente de novo.
      </p>
      <Botao
        variant="secondary"
        @click="carregar"
      >
        Tentar de novo
      </Botao>
    </div>

    <template v-else>
      <h1 class="text-2xl font-bold [overflow-wrap:anywhere]">
        Partidas de {{ nome }}
      </h1>
      <div
        v-if="itens.length === 0"
        class="rounded-painel bg-surface-2 p-5"
      >
        <p class="font-bold">
          Ainda sem partidas
        </p>
        <p class="text-ink-muted">
          As partidas terminadas aparecem aqui.
        </p>
      </div>
      <ul v-else>
        <LinhaPartida
          v-for="m in itens"
          :key="m.id"
          :item="m"
          :viewer="username"
        />
      </ul>
      <Botao
        v-if="cursor !== undefined"
        variant="secondary"
        class="self-start"
        :loading="carregandoMais"
        @click="carregarMais"
      >
        Carregar mais
      </Botao>
    </template>
  </div>
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
