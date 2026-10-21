<script setup lang="ts">
import { nextTick, onMounted, ref, useTemplateRef } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useSessionStore } from '@/stores/session'
import { useToastsStore } from '@/stores/toasts'
import { formError } from '@/app/formError'
import CartaoConta from '@/components/conta/CartaoConta.vue'
import AvisoForm from '@/components/conta/AvisoForm.vue'
import Campo from '@/components/ui/Campo.vue'
import Botao from '@/components/ui/Botao.vue'
import BotaoLink from '@/components/ui/BotaoLink.vue'

const session = useSessionStore()
const toasts = useToastsStore()
const route = useRoute()
const cartao = useTemplateRef<InstanceType<typeof CartaoConta>>('cartao')
const state = ref<'loading' | 'ok' | 'error' | 'sent'>('loading')
const message = ref('')
const email = ref('')
const reenviando = ref(false)

async function resolver(next: 'ok' | 'error') {
  state.value = next
  await nextTick()
  cartao.value?.focusTitle()
}

onMounted(async () => {
  const token = typeof route.query.token === 'string' ? route.query.token : ''
  if (!token) {
    message.value = 'Link inválido ou expirado.'
    await resolver('error')
    return
  }
  try {
    await session.verifyEmail(token)
    await resolver('ok')
  } catch (err) {
    message.value = formError(err).message
    await resolver('error')
  }
})

async function reenviar() {
  reenviando.value = true
  try {
    await session.resendVerification(email.value.trim())
    state.value = 'sent'
  } catch (err) {
    toasts.fromError(err)
  } finally {
    reenviando.value = false
  }
}
</script>

<template>
  <CartaoConta
    ref="cartao"
    :title="state === 'ok' ? 'E-mail confirmado' : state === 'error' ? 'Não deu para confirmar' : state === 'sent' ? 'Confira seu e-mail' : 'Confirmando e-mail'"
  >
    <p
      v-if="state === 'loading'"
      role="status"
      class="text-ink-muted"
    >
      Confirmando…
    </p>
    <div
      v-else-if="state === 'ok'"
      class="flex flex-col gap-4"
    >
      <p>E-mail confirmado! Agora é só entrar.</p>
      <BotaoLink to="/entrar">
        Entrar
      </BotaoLink>
    </div>
    <div
      v-else-if="state === 'sent'"
      class="flex flex-col gap-4"
    >
      <p role="status">
        Se o e-mail estiver cadastrado, enviamos um novo link.
      </p>
      <RouterLink
        to="/entrar"
        class="inline-flex min-h-11 items-center self-start underline underline-offset-4"
      >
        Ir para o login
      </RouterLink>
    </div>
    <form
      v-else
      class="flex flex-col gap-4"
      novalidate
      @submit.prevent="reenviar"
    >
      <AvisoForm>{{ message }}</AvisoForm>
      <Campo
        v-model="email"
        label="E-mail para receber um novo link"
        type="email"
        autocomplete="email"
        required
      />
      <Botao
        type="submit"
        :loading="reenviando"
      >
        Reenviar
      </Botao>
      <RouterLink
        to="/entrar"
        class="inline-flex min-h-11 items-center self-start text-sm underline underline-offset-4"
      >
        Ir para o login
      </RouterLink>
    </form>
  </CartaoConta>
</template>
