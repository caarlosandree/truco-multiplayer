<script setup lang="ts">
import { nextTick, reactive, ref, useTemplateRef } from 'vue'
import { RouterLink } from 'vue-router'
import { useSessionStore } from '@/stores/session'
import { useToastsStore } from '@/stores/toasts'
import { focusFirstInvalid, formError, type FormError } from '@/app/formError'
import CartaoConta from '@/components/conta/CartaoConta.vue'
import AvisoForm from '@/components/conta/AvisoForm.vue'
import Campo from '@/components/ui/Campo.vue'
import Botao from '@/components/ui/Botao.vue'
import BotaoLink from '@/components/ui/BotaoLink.vue'

const session = useSessionStore()
const toasts = useToastsStore()
const cartao = useTemplateRef<InstanceType<typeof CartaoConta>>('cartao')
const formEl = useTemplateRef<HTMLFormElement>('formEl')
const form = reactive({ username: '', email: '', displayName: '', password: '', confirm: '' })
const falha = ref<FormError | null>(null)
const confirmError = ref('')
const loading = ref(false)
const reenviando = ref(false)
const done = ref(false)

async function submit() {
  falha.value = null
  confirmError.value = ''
  if (form.password !== form.confirm) {
    confirmError.value = 'As senhas não conferem.'
    await nextTick()
    focusFirstInvalid(formEl.value)
    return
  }
  loading.value = true
  try {
    await session.register({
      username: form.username.trim(),
      email: form.email.trim(),
      displayName: form.displayName.trim(),
      password: form.password,
    })
    done.value = true
    await nextTick()
    cartao.value?.focusTitle()
  } catch (err) {
    falha.value = formError(err)
    await nextTick()
    focusFirstInvalid(formEl.value)
  } finally {
    loading.value = false
  }
}

async function reenviar() {
  reenviando.value = true
  try {
    await session.resendVerification(form.email.trim())
    toasts.push('Se o e-mail estiver cadastrado, enviamos um novo link.')
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
    :title="done ? 'Confira seu e-mail' : 'Criar conta'"
  >
    <div
      v-if="done"
      class="flex flex-col gap-4"
    >
      <p>Enviamos um link de confirmação para <strong class="break-all">{{ form.email }}</strong>. Abra o e-mail para ativar a conta.</p>
      <p class="text-sm text-ink-muted">
        Não chegou? Veja a caixa de spam ou peça outro link.
      </p>
      <Botao
        variant="secondary"
        :loading="reenviando"
        @click="reenviar"
      >
        Reenviar e-mail
      </Botao>
      <BotaoLink to="/entrar">
        Ir para o login
      </BotaoLink>
    </div>
    <template v-else>
      <form
        ref="formEl"
        class="flex flex-col gap-4"
        novalidate
        @submit.prevent="submit"
      >
        <Campo
          v-model="form.username"
          label="Usuário"
          hint="3 a 20 caracteres: letras minúsculas, números, _ e ."
          autocomplete="username"
          :error="falha?.fields.username"
          required
        />
        <Campo
          v-model="form.email"
          label="E-mail da empresa"
          type="email"
          autocomplete="email"
          :error="falha?.fields.email"
          required
        />
        <Campo
          v-model="form.displayName"
          label="Nome de exibição"
          hint="Como você aparece na mesa."
          autocomplete="nickname"
          :error="falha?.fields.displayName"
          required
        />
        <Campo
          v-model="form.password"
          label="Senha"
          hint="Mínimo de 10 caracteres."
          type="password"
          autocomplete="new-password"
          :error="falha?.fields.password"
          required
        />
        <Campo
          v-model="form.confirm"
          label="Confirmar senha"
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
          Criar conta
        </Botao>
      </form>
      <RouterLink
        to="/entrar"
        class="mt-4 inline-flex min-h-11 items-center self-start text-sm underline underline-offset-4"
      >
        Já tenho conta
      </RouterLink>
    </template>
  </CartaoConta>
</template>
