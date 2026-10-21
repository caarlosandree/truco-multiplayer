<script setup lang="ts">
import { computed } from 'vue'
import { useInvitesStore } from '@/stores/invites'
import { useRoomStore } from '@/stores/room'
import { useSessionStore } from '@/stores/session'
import AvisoConvite from './AvisoConvite.vue'

/**
 * Pilha de avisos de convite (até 3, o mais novo em cima). Não é toast: persiste até a resposta ou a expiração.
 * Canto inferior esquerdo a partir de 768 px (os toasts ficam à direita); no celular, no topo da área útil, abaixo do
 * cabeçalho (e do banner de versão, quando há). A pilha tem altura máxima e rola por dentro. Durante a partida só o aviso
 * mais novo aparece, com "+N convites" para os demais, e no celular ele fica acima da mão e do painel (como os toasts),
 * sem cobrir o placar. Sem região viva: o Anunciador é o dono dos anúncios.
 */
const invites = useInvitesStore()
const room = useRoomStore()
const session = useSessionStore()

const mostrados = computed(() => (room.isPlaying ? invites.visible.slice(0, 1) : invites.visible))
const extras = computed(() => invites.received.length - mostrados.value.length)
const posicaoCelular = computed(() =>
  room.isPlaying
    ? 'bottom-[var(--toast-offset,0px)] max-md:pb-2'
    : session.versionMismatch
      ? 'top-28 pt-2'
      : 'top-14 pt-2',
)
</script>

<template>
  <div
    v-if="mostrados.length > 0"
    role="group"
    aria-label="Convites para a mesa"
    class="pointer-events-none fixed inset-x-0 z-(--z-banner) px-3 md:inset-x-auto md:top-auto md:bottom-6 md:left-5 md:w-[21rem] md:p-0"
    :class="posicaoCelular"
  >
    <div class="pointer-events-auto flex max-h-[50dvh] flex-col gap-2 overflow-y-auto overscroll-contain p-1 md:max-h-[60dvh]">
      <AvisoConvite
        v-for="inv in mostrados"
        :key="inv.inviteId"
        :invite="inv"
      />
      <p
        v-if="extras > 0"
        class="pointer-events-none rounded-placa bg-surface-2 px-3 py-1 text-center text-sm font-bold text-ink shadow-toast"
        data-mais-convites
      >
        +{{ extras }} {{ extras === 1 ? 'convite' : 'convites' }}
      </p>
    </div>
  </div>
</template>
