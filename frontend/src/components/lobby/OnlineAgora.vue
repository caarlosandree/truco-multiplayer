<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { presenceText } from '@/game/labels'
import { useInvitesStore } from '@/stores/invites'
import { usePresenceStore } from '@/stores/presence'
import { useRoomStore } from '@/stores/room'
import { useSessionStore } from '@/stores/session'
import { useToastsStore } from '@/stores/toasts'
import type { PresenceEntry } from '@/protocol/messages'

const presence = usePresenceStore()
const room = useRoomStore()
const session = useSessionStore()
const invites = useInvitesStore()
const toasts = useToastsStore()

const sendingTo = ref<string | null>(null)

const meId = computed(() => session.user?.id ?? null)
const seatedIds = computed(() => new Set((room.room?.seats ?? []).flatMap((s) => (s ? [s.userId] : []))))
const alone = computed(() => presence.loaded && presence.list.every((e) => e.userId === meId.value))

const isMe = (e: PresenceEntry) => e.userId === meId.value
const canInviteTo = (e: PresenceEntry) => room.canInvite && !isMe(e) && !seatedIds.value.has(e.userId)
const profileTo = (e: PresenceEntry) => ({ name: 'jogador', params: { username: e.username } })

/** Ponto vazado, em anel grosso ou cheio por status: o formato difere sem depender da cor (o texto ao lado já diz o status). */
const DOT: Record<string, string> = {
  lobby: 'size-3 border-2 border-ink-muted',
  waiting: 'size-3.5 border-4 border-ink-muted',
  playing: 'size-3.5 bg-ink',
}

async function convidar(e: PresenceEntry) {
  if (sendingTo.value !== null) return
  sendingTo.value = e.userId
  try {
    await invites.invite(e.username)
  } catch (err) {
    toasts.fromError(err)
  } finally {
    sendingTo.value = null
  }
}
</script>

<template>
  <section aria-labelledby="online-titulo">
    <h2
      id="online-titulo"
      class="mb-3 text-lg font-bold max-md:sr-only"
    >
      Online agora · {{ presence.count }}
    </h2>

    <div
      v-if="!presence.loaded"
      role="status"
      class="rounded-painel bg-surface-2"
    >
      <p class="sr-only">
        Carregando quem está online…
      </p>
      <div
        v-for="n in 3"
        :key="n"
        aria-hidden="true"
        class="esqueleto flex flex-col gap-2 border-t-2 border-line/40 px-4 py-4 first-of-type:border-t-0"
        :style="{ animationDelay: `${n * 120}ms` }"
      >
        <span class="h-4 w-28 rounded-placa bg-line/30" />
        <span class="h-3 w-20 rounded-placa bg-line/20" />
      </div>
    </div>

    <template v-else>
      <ul class="rounded-painel bg-surface-2">
        <li
          v-for="e in presence.list"
          :key="e.userId"
          :data-online="e.username"
          class="relative flex items-center gap-3 border-t-2 border-line/40 px-4 py-2.5 first:border-t-0"
        >
          <span
            data-ponto
            aria-hidden="true"
            class="flex size-4 shrink-0 items-center justify-center"
          >
            <span
              class="rounded-full"
              :class="DOT[e.status] ?? DOT.lobby"
            />
          </span>
          <div class="min-w-0 flex-1">
            <p class="flex items-baseline gap-2">
              <RouterLink
                :to="profileTo(e)"
                class="truncate font-bold text-ink after:absolute after:inset-0 hover:underline"
              >
                {{ e.displayName }}
              </RouterLink>
              <span
                v-if="isMe(e)"
                class="shrink-0 rounded-placa px-1.5 text-sm font-bold text-ink-muted inset-ring-2 inset-ring-line"
              >você</span>
            </p>
            <p class="text-sm text-pretty text-ink-muted [overflow-wrap:anywhere]">
              {{ presenceText(e) }}
            </p>
          </div>
          <button
            v-if="canInviteTo(e)"
            type="button"
            data-acao="convidar"
            :aria-label="`Convidar ${e.displayName}`"
            :disabled="sendingTo !== null"
            class="relative z-10 inline-flex h-11 shrink-0 items-center justify-center rounded-botao-2 px-3 text-sm font-bold text-ink inset-ring-2 inset-ring-(--botao-sec-borda) hover:bg-(--botao-sec-bg-hover) active:translate-y-(--botao-pressao-y) disabled:cursor-not-allowed disabled:opacity-(--botao-desabilitado-opacidade) md:h-9"
            @click="convidar(e)"
          >
            Convidar
          </button>
        </li>
      </ul>
      <p
        v-if="alone"
        class="mt-3 text-ink-muted"
      >
        Só você por aqui agora.
      </p>
    </template>
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
