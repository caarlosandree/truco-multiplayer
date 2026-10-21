<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { presenceText } from '@/game/labels'
import { isNotFound, usePlayersStore, type Profile } from '@/stores/players'
import { useInvitesStore } from '@/stores/invites'
import { usePresenceStore } from '@/stores/presence'
import { useRoomStore } from '@/stores/room'
import { useSessionStore } from '@/stores/session'
import { useToastsStore } from '@/stores/toasts'
import Botao from '@/components/ui/Botao.vue'
import BotaoLink from '@/components/ui/BotaoLink.vue'
import LinhaPartida from '@/components/partidas/LinhaPartida.vue'

type Estado = 'carregando' | 'ok' | 'nao-encontrado' | 'erro'

const route = useRoute()
const players = usePlayersStore()
const presence = usePresenceStore()
const room = useRoomStore()
const session = useSessionStore()
const invites = useInvitesStore()
const toasts = useToastsStore()

const estado = ref<Estado>('carregando')
const perfil = ref<Profile | null>(null)
const convidando = ref(false)
let seq = 0

const username = computed(() => String(route.params.username ?? ''))
const souEu = computed(() => session.user?.username === username.value)
const presenca = computed(() => presence.byUsername(username.value))
const statusTexto = computed(() => (presenca.value ? presenceText(presenca.value) : 'offline'))
const seatedIds = computed(() => new Set((room.room?.seats ?? []).flatMap((s) => (s ? [s.userId] : []))))
const jaNaSala = computed(() => {
  const e = presenca.value
  return e !== undefined && seatedIds.value.has(e.userId)
})
const podeConvidar = computed(() => !souEu.value && room.canInvite && !jaNaSala.value)

const percentual = new Intl.NumberFormat('pt-BR', { style: 'percent', minimumFractionDigits: 0, maximumFractionDigits: 1 })
const posicao = (n: number | null) => (n === null ? '—' : `#${n}`)
const partidas = (n: number) => (n === 1 ? '1 partida' : `${n} partidas`)

const PERIODOS = [
  { id: 'all', label: 'Geral' },
  { id: 'month', label: 'Mês' },
  { id: 'week', label: 'Semana' },
] as const

const estatisticas = computed(() => {
  const s = perfil.value?.stats
  if (!s) return []
  return [
    { label: 'Partidas', valor: String(s.matchesPlayed) },
    { label: 'Vitórias', valor: String(s.wins) },
    { label: 'Aproveitamento', valor: percentual.format(s.winRate) },
    { label: 'Aumentos pedidos', valor: String(s.raisesCalled) },
    { label: 'Aumentos corridos (dupla)', valor: String(s.raisesFolded) },
    { label: 'Mãos de 11 vencidas', valor: String(s.elevenWon) },
  ]
})

async function carregar() {
  const id = ++seq
  estado.value = 'carregando'
  perfil.value = null
  try {
    const p = await players.loadProfile(username.value)
    if (id !== seq) return
    perfil.value = p
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

async function convidar() {
  if (convidando.value) return
  convidando.value = true
  try {
    await invites.invite(username.value)
  } catch (err) {
    toasts.fromError(err)
  } finally {
    convidando.value = false
  }
}

watch(username, () => void carregar(), { immediate: true })
</script>

<template>
  <div class="mx-auto flex w-full max-w-[65ch] flex-col px-4 py-6 md:px-6 md:py-8">
    <div
      v-if="estado === 'carregando'"
      class="flex flex-col gap-4"
      data-testid="perfil-carregando"
      aria-busy="true"
    >
      <p class="sr-only">
        Carregando o perfil
      </p>
      <div
        v-for="n in 4"
        :key="n"
        aria-hidden="true"
        class="esqueleto h-16 rounded-painel bg-surface-2"
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
        Não foi possível carregar o perfil.
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

    <template v-else-if="perfil">
      <header class="flex flex-col gap-3 pb-6">
        <div class="min-w-0">
          <h1 class="text-2xl font-bold [overflow-wrap:anywhere]">
            {{ perfil.displayName }}
          </h1>
          <p class="text-ink-muted [overflow-wrap:anywhere]">
            @{{ perfil.username }}
          </p>
        </div>
        <p class="flex items-center gap-2">
          <span
            aria-hidden="true"
            class="size-3 shrink-0 rounded-full"
            :class="presenca ? 'bg-ink' : 'border-2 border-ink-muted'"
          />
          <span>{{ statusTexto }}</span>
        </p>
        <div
          v-if="souEu || podeConvidar"
          class="flex flex-wrap gap-3"
        >
          <BotaoLink
            v-if="souEu"
            to="/perfil"
            variant="secondary"
          >
            Editar conta
          </BotaoLink>
          <Botao
            v-else
            :loading="convidando"
            @click="convidar"
          >
            Convidar pra minha sala
          </Botao>
        </div>
      </header>

      <section
        v-if="perfil.stats.matchesPlayed > 0"
        class="flex flex-col gap-3 border-t-2 border-line/40 py-6"
        aria-labelledby="jogador-posicoes"
      >
        <h2
          id="jogador-posicoes"
          class="text-xl font-bold"
        >
          Posição no ranking
        </h2>
        <table
          data-testid="posicoes"
          aria-labelledby="jogador-posicoes"
          class="w-full table-fixed border-collapse tabular-nums"
        >
          <thead>
            <tr class="border-b-2 border-line/50 text-left text-sm text-ink-muted">
              <th
                scope="col"
                class="h-10 font-normal"
              >
                Período
              </th>
              <th
                scope="col"
                class="text-right font-normal"
              >
                Vitórias
              </th>
              <th
                scope="col"
                class="text-right font-normal"
              >
                Aproveitamento
              </th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="p in PERIODOS"
              :key="p.id"
              class="border-b border-line/30"
            >
              <th
                scope="row"
                class="h-11 text-left font-bold"
              >
                {{ p.label }}
              </th>
              <td class="text-right">
                {{ posicao(perfil.positions[p.id].wins) }}
              </td>
              <td class="text-right">
                {{ posicao(perfil.positions[p.id].winrate) }}
              </td>
            </tr>
          </tbody>
        </table>
      </section>

      <section
        v-if="perfil.stats.matchesPlayed > 0"
        class="flex flex-col gap-3 border-t-2 border-line/40 py-6"
        aria-labelledby="jogador-numeros"
      >
        <h2
          id="jogador-numeros"
          class="text-xl font-bold"
        >
          Em números
        </h2>
        <dl
          data-testid="estatisticas"
          class="grid grid-cols-2 gap-x-4 gap-y-3 sm:grid-cols-3"
        >
          <div
            v-for="e in estatisticas"
            :key="e.label"
            class="flex flex-col"
          >
            <dt class="text-sm text-ink-muted">
              {{ e.label }}
            </dt>
            <dd class="text-lg font-bold tabular-nums">
              {{ e.valor }}
            </dd>
          </div>
        </dl>
      </section>

      <section
        v-if="perfil.recentMatches.length === 0"
        class="flex flex-col gap-1 border-t-2 border-line/40 py-6"
        aria-labelledby="jogador-vazio"
      >
        <h2
          id="jogador-vazio"
          class="text-xl font-bold"
        >
          Ainda sem partidas
        </h2>
        <p class="text-ink-muted">
          As partidas terminadas aparecem aqui.
        </p>
      </section>

      <template v-else>
        <section
          class="flex flex-col gap-2 border-t-2 border-line/40 py-6"
          aria-labelledby="jogador-recentes"
        >
          <h2
            id="jogador-recentes"
            class="text-xl font-bold"
          >
            Partidas recentes
          </h2>
          <ul>
            <LinhaPartida
              v-for="m in perfil.recentMatches"
              :key="m.id"
              :item="m"
              :viewer="perfil.username"
            />
          </ul>
          <RouterLink
            :to="{ name: 'jogador-partidas', params: { username: perfil.username } }"
            class="inline-flex h-11 items-center self-start font-bold underline decoration-2 underline-offset-4"
          >
            Ver todas as partidas
          </RouterLink>
        </section>

        <section
          v-if="perfil.partners.length > 0"
          data-testid="parceiros"
          class="flex flex-col gap-2 border-t-2 border-line/40 py-6"
          aria-labelledby="jogador-parceiros"
        >
          <h2
            id="jogador-parceiros"
            class="text-xl font-bold"
          >
            Parceiros frequentes
          </h2>
          <ul>
            <li
              v-for="p in perfil.partners"
              :key="p.username"
              class="flex items-center justify-between gap-4 border-t-2 border-line/40 first:border-t-0"
            >
              <RouterLink
                :to="{ name: 'jogador', params: { username: p.username } }"
                class="inline-flex min-h-11 min-w-0 items-center font-bold hover:underline"
              >
                <span class="truncate">{{ p.displayName }}</span>
              </RouterLink>
              <span class="shrink-0 text-ink-muted tabular-nums">{{ partidas(p.matches) }}, {{ p.wins }} {{ p.wins === 1 ? 'vitória' : 'vitórias' }}</span>
            </li>
          </ul>
        </section>
      </template>
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
