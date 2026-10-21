<script setup lang="ts">
import { computed, nextTick, ref, useTemplateRef } from 'vue'
import { useRoute } from 'vue-router'
import { useSessionStore } from '@/stores/session'
import { focusFirstInvalid, formError, type FormError } from '@/app/formError'
import CartaoConta from '@/components/conta/CartaoConta.vue'
import AvisoForm from '@/components/conta/AvisoForm.vue'
import Campo from '@/components/ui/Campo.vue'
import Botao from '@/components/ui/Botao.vue'
import BotaoLink from '@/components/ui/BotaoLink.vue'

const session = useSessionStore()
const route = useRoute()
const cartao = useTemplateRef<InstanceType<typeof CartaoConta>>('cartao')
const formEl = useTemplateRef<HTMLFormElement>('formEl')
const token = computed(() => (typeof route.query.token === 'string' ? route.query.token : ''))
const password = ref('')
const confirm = ref('')
const falha = ref<FormError | null>(null)
const confirmError = ref('')
const loading = ref(false)
const done = ref(false)
const invalidLink = ref(token.value === '')

async function submit() {
  falha.value = null
  confirmError.value = ''
  if (password.value !== confirm.value) {
    confirmError.value = 'As senhas não conferem.'
    await nextTick()
    focusFirstInvalid(formEl.value)
    return
  }
  loading.value = true
  try {
    await session.resetPassword(token.value, password.value)
    done.value = true
    await nextTick()
    cartao.value?.focusTitle()
  } catch (err) {
    falha.value = formError(err)
    invalidLink.value = falha.value.code === 'invalid_token'
    await nextTick()
    focusFirstInvalid(formEl.value)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <CartaoConta
    ref="cartao"
    :title="done ? 'Senha redefinida' : 'Redefinir senha'"
  >
    <div
      v-if="done"
      class="flex flex-col gap-4"
    >
      <p role="status">
        Senha redefinida. Entre com a nova senha.
      </p>
      <BotaoLink to="/entrar">
        Entrar
      </BotaoLink>
    </div>
    <div
      v-else-if="invalidLink"
      class="flex flex-col gap-4"
    >
      <AvisoForm>Link inválido ou expirado.</AvisoForm>
      <BotaoLink to="/esqueci-senha">
        Pedir um novo link
      </BotaoLink>
    </div>
    <form
      v-else
      ref="formEl"
      class="flex flex-col gap-4"
      novalidate
      @submit.prevent="submit"
    >
      <Campo
        v-model="password"
        label="Nova senha"
        hint="Mínimo de 10 caracteres."
        type="password"
        autocomplete="new-password"
        :error="falha?.fields.password"
        required
      />
      <Campo
        v-model="confirm"
        label="Confirmar nova senha"
        type="password"
        autocomplete="new-password"
        :error="confirmError"
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
        Redefinir senha
      </Botao>
    </form>
  </CartaoConta>
</template>
