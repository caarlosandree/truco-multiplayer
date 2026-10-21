<script setup lang="ts">
import { computed, nextTick, ref, useTemplateRef } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { useSessionStore } from '@/stores/session'
import { useToastsStore } from '@/stores/toasts'
import { useReenvio } from '@/app/useReenvio'
import { sanitizeReturnTo } from '@/router'
import { focusFirstInvalid, formError, type FormError } from '@/app/formError'
import CartaoConta from '@/components/conta/CartaoConta.vue'
import AvisoForm from '@/components/conta/AvisoForm.vue'
import Campo from '@/components/ui/Campo.vue'
import Botao from '@/components/ui/Botao.vue'

const session = useSessionStore()
const toasts = useToastsStore()
const route = useRoute()
const router = useRouter()
const form = useTemplateRef<HTMLFormElement>('form')
const login = ref('')
const password = ref('')
const reenvio = ref('')
const loading = ref(false)
const { enviando, limite, bloqueado, enviar, fimEspera } = useReenvio()
const falha = ref<FormError | null>(null)

const naoVerificado = computed(() => falha.value?.code === 'email_not_verified')
const precisaEmail = computed(() => naoVerificado.value && !login.value.includes('@'))

async function submit() {
  loading.value = true
  falha.value = null
  try {
    await session.login(login.value.trim(), password.value)
    await router.replace(sanitizeReturnTo(route.query.volta))
  } catch (err) {
    falha.value = formError(err)
    await nextTick()
    focusFirstInvalid(form.value)
  } finally {
    loading.value = false
  }
}

async function reenviar() {
  const email = (precisaEmail.value ? reenvio.value : login.value).trim()
  if (await enviar(email)) toasts.push('Se o e-mail estiver cadastrado, enviamos um novo link.')
}
</script>

<template>
  <CartaoConta title="Entrar">
    <form
      ref="form"
      class="flex flex-col gap-4"
      novalidate
      @submit.prevent="submit"
    >
      <Campo
        v-model="login"
        label="Usuário ou e-mail"
        autocomplete="username"
        :error="falha?.fields.login"
        required
      />
      <Campo
        v-model="password"
        label="Senha"
        type="password"
        autocomplete="current-password"
        :error="falha?.fields.password"
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
        Entrar
      </Botao>
    </form>
    <form
      v-if="naoVerificado"
      class="mt-6 flex flex-col gap-4 border-t-2 border-line/40 pt-6"
      novalidate
      @submit.prevent="reenviar"
    >
      <p class="text-sm text-ink-muted">
        Não recebeu o e-mail de confirmação?
      </p>
      <Campo
        v-if="precisaEmail"
        v-model="reenvio"
        label="E-mail para reenviar a confirmação"
        type="email"
        autocomplete="email"
      />
      <AvisoForm
        v-if="limite"
        :retry-after-sec="limite.retryAfterSec"
        @fim="fimEspera"
      >
        {{ limite.message }}
      </AvisoForm>
      <Botao
        type="submit"
        variant="secondary"
        :loading="enviando"
        :disabled="bloqueado"
      >
        Reenviar e-mail de confirmação
      </Botao>
    </form>
    <nav
      class="mt-4 flex flex-wrap justify-between gap-x-6 text-sm"
      aria-label="Outras opções"
    >
      <RouterLink
        to="/esqueci-senha"
        class="inline-flex min-h-11 items-center underline underline-offset-4"
      >
        Esqueci minha senha
      </RouterLink>
      <RouterLink
        to="/cadastro"
        class="inline-flex min-h-11 items-center underline underline-offset-4"
      >
        Criar conta
      </RouterLink>
    </nav>
  </CartaoConta>
</template>
