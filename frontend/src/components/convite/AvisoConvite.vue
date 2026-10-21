<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import type { InviteReceivedPayload } from '@/protocol/messages'
import { askNotificationPermissionOnce } from '@/app/notifications'
import { useInvitesStore } from '@/stores/invites'
import { useRoomStore } from '@/stores/room'
import { useToastsStore } from '@/stores/toasts'
import Botao from '@/components/ui/Botao.vue'

const props = defineProps<{ invite: InviteReceivedPayload }>()

const invites = useInvitesStore()
const room = useRoomStore()
const toasts = useToastsStore()
const entrar = ref<InstanceType<typeof Botao> | null>(null)

const jogando = computed(() => room.isPlaying)
const trocaDeSala = computed(() => room.inRoom && !room.isPlaying)
const lugares = computed(() => `${props.invite.freeSeats} ${props.invite.freeSeats === 1 ? 'lugar livre' : 'lugares livres'}`)
const titulo = computed(() => `${props.invite.from.displayName} te chamou pra mesa!`)

async function responder(aceitar: boolean, porClique = true) {
  // a permissão de notificação só pode ser pedida numa ativação do usuário: clique sim, Esc não (o pedido sumiria e queimaria o único pedido)
  if (porClique) void askNotificationPermissionOnce()
  try {
    await invites.respond(props.invite.inviteId, aceitar)
  } catch (err) {
    toasts.fromError(err)
  }
}

// Entrar recebe o foco ao chegar, exceto se a pessoa está digitando ou há um diálogo aberto
onMounted(() => {
  const el = entrar.value?.$el as HTMLElement | undefined
  if (!el) return
  const ativo = document.activeElement
  const digitando = ativo instanceof HTMLInputElement || ativo instanceof HTMLTextAreaElement || (ativo instanceof HTMLElement && ativo.isContentEditable)
  if (digitando || document.querySelector('[role="dialog"]')) return
  el.focus()
})
</script>

<template>
  <article
    data-aviso
    :data-invite-id="invite.inviteId"
    :aria-labelledby="`aviso-${invite.inviteId}`"
    class="textura-lousa pointer-events-auto flex flex-col gap-3 rounded-painel border-2 border-line bg-surface-2 p-3 text-ink shadow-toast"
    @keydown.esc.stop="responder(false, false)"
  >
    <div class="flex flex-col gap-0.5">
      <h2
        :id="`aviso-${invite.inviteId}`"
        class="text-base font-bold wrap-anywhere"
      >
        {{ titulo }}
      </h2>
      <p class="flex flex-wrap items-baseline gap-x-2 text-sm">
        <span class="min-w-0 font-bold wrap-anywhere">{{ invite.room.name }}</span>
        <span class="shrink-0 text-ink-muted">{{ lugares }}</span>
      </p>
      <p
        v-if="jogando"
        class="text-sm text-ink-muted"
      >
        Você está numa partida
      </p>
      <p
        v-else-if="trocaDeSala"
        class="text-sm text-ink-muted"
      >
        Aceitar tira você da sala atual.
      </p>
    </div>
    <div class="flex gap-2">
      <Botao
        v-if="!jogando"
        ref="entrar"
        data-acao="entrar"
        class="flex-1"
        @click="responder(true)"
      >
        Entrar
      </Botao>
      <Botao
        variant="secondary"
        data-acao="agora-nao"
        :class="jogando ? 'w-full' : 'flex-1'"
        @click="responder(false)"
      >
        Agora não
      </Botao>
    </div>
  </article>
</template>
