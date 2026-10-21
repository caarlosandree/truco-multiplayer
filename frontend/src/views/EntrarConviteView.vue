<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, useTemplateRef, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useInvitesStore } from '@/stores/invites'
import { useRoomStore } from '@/stores/room'
import { useSessionStore } from '@/stores/session'
import { useToastsStore } from '@/stores/toasts'
import CartaoConta from '@/components/conta/CartaoConta.vue'
import BotaoLink from '@/components/ui/BotaoLink.vue'

/** Já sentado na sala do convite o servidor não responde: depois disso olhamos a sala em vez de esperar. */
const ESPERA_JA_SENTADO_MS = 3000

const route = useRoute()
const router = useRouter()
const invites = useInvitesStore()
const room = useRoomStore()
const session = useSessionStore()
const toasts = useToastsStore()
const cartao = useTemplateRef<InstanceType<typeof CartaoConta>>('cartao')

const outcome = computed(() => invites.tokenOutcome)
const conteudo = computed(() => {
  switch (outcome.value) {
    case 'expired':
      return {
        titulo: 'Convite expirado',
        frase: 'Essa mesa fechou.',
        texto: 'A sala encheu, a partida já começou ou a sala foi fechada.',
        botao: 'Ir para o lobby',
        destino: { name: 'lobby' },
      }
    case 'not_yours':
      return {
        titulo: 'Este convite é para outra pessoa',
        frase: 'Outra conta, outra cadeira.',
        texto: 'Entre com a conta que recebeu o e-mail para aceitar.',
        botao: 'Ir para o lobby',
        destino: { name: 'lobby' },
      }
    case 'playing':
      return {
        titulo: 'Você está numa partida',
        frase: 'Termine a mão primeiro.',
        texto: 'Termine ou saia da partida atual para aceitar o convite.',
        botao: 'Voltar para a partida',
        destino: { name: 'sala' },
      }
    default:
      return null
  }
})

let temporizador: ReturnType<typeof setTimeout> | undefined

function irParaSala() {
  // o installRoomNavigation pode já ter levado à sala; não repete a navegação
  if (router.currentRoute.value.name !== 'sala') void router.replace({ name: 'sala' })
}

async function aceitar() {
  try {
    await invites.acceptToken(String(route.params.token))
  } catch (err) {
    toasts.fromError(err)
    void router.replace({ name: 'lobby' })
  }
}

onMounted(() => {
  if (room.inRoom) {
    temporizador = setTimeout(() => {
      if (invites.tokenOutcome === 'pending' && room.inRoom) irParaSala()
    }, ESPERA_JA_SENTADO_MS)
  }
  if (session.connection === 'open') {
    void aceitar()
  } else {
    const parar = watch(
      () => session.connection,
      (s) => {
        if (s === 'open') {
          parar()
          void aceitar()
        }
      },
    )
  }
})

watch(outcome, async (o) => {
  if (o === 'accepted') {
    irParaSala()
  } else if (o === 'expired' || o === 'not_yours' || o === 'playing') {
    await nextTick()
    cartao.value?.focusTitle() // o resultado chegou: o foco vai para o título, que o leitor de tela lê
  }
})

onBeforeUnmount(() => {
  clearTimeout(temporizador)
  invites.resetToken()
})
</script>

<template>
  <CartaoConta
    ref="cartao"
    :title="conteudo?.titulo ?? 'Entrando na sala…'"
    :frase="conteudo?.frase"
  >
    <div
      v-if="conteudo"
      class="flex flex-col gap-4"
    >
      <p>{{ conteudo.texto }}</p>
      <BotaoLink :to="conteudo.destino">
        {{ conteudo.botao }}
      </BotaoLink>
    </div>
    <p
      v-else
      class="text-ink-muted"
    >
      Estamos te colocando na mesa.
    </p>
  </CartaoConta>
</template>
