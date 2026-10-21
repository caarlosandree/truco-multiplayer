<script setup lang="ts">
import { nextTick, ref, useTemplateRef } from 'vue'
import { RouterLink } from 'vue-router'
import { useSessionStore } from '@/stores/session'
import { formError, type FormError } from '@/app/formError'
import CartaoConta from '@/components/conta/CartaoConta.vue'
import AvisoForm from '@/components/conta/AvisoForm.vue'
import Campo from '@/components/ui/Campo.vue'
import Botao from '@/components/ui/Botao.vue'

const session = useSessionStore()
const cartao = useTemplateRef<InstanceType<typeof CartaoConta>>('cartao')
const email = ref('')
const sent = ref(false)
const falha = ref<FormError | null>(null)
const loading = ref(false)

async function submit() {
  loading.value = true
  falha.value = null
  try {
    await session.forgotPassword(email.value.trim())
    sent.value = true
    await nextTick()
    cartao.value?.focusTitle()
  } catch (err) {
    falha.value = formError(err)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <CartaoConta
    ref="cartao"
    :title="sent ? 'Confira seu e-mail' : 'Esqueci minha senha'"
  >
    <p
      v-if="sent"
      role="status"
    >
      Se o e-mail existir, enviamos um link para redefinir a senha.
    </p>
    <form
      v-else
      class="flex flex-col gap-4"
      novalidate
      @submit.prevent="submit"
    >
      <p class="text-ink-muted">
        Digite o e-mail da conta e enviaremos um link para criar uma nova senha.
      </p>
      <Campo
        v-model="email"
        label="E-mail"
        type="email"
        autocomplete="email"
        :error="falha?.fields.email"
        required
      />
      <AvisoForm
        v-if="falha"
        :retry-after-sec="falha.retryAfterSec"
      >
        {{ falha.message }}
      </AvisoForm>
      <Botao
        type="submit"
        :loading="loading"
      >
        Enviar link
      </Botao>
    </form>
    <RouterLink
      to="/entrar"
      class="mt-4 inline-flex min-h-11 items-center self-start text-sm underline underline-offset-4"
    >
      Voltar ao login
    </RouterLink>
  </CartaoConta>
</template>
