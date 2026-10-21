<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useSessionStore } from '@/stores/session'
import { usePrefsStore } from '@/stores/prefs'
import { useToastsStore } from '@/stores/toasts'
import { formError } from '@/app/formError'
import { cardUrl } from '@/game/cardAssets'
import type { DeckStyle } from '@/game/cards'
import Campo from '@/components/ui/Campo.vue'
import Botao from '@/components/ui/Botao.vue'
import Toggle from '@/components/ui/Toggle.vue'
import Modal from '@/components/ui/Modal.vue'

const session = useSessionStore()
const prefs = usePrefsStore()
const toasts = useToastsStore()
const router = useRouter()

const displayName = ref(session.user?.displayName ?? '')
const nameError = ref('')
const savingName = ref(false)
const current = ref('')
const next = ref('')
const currentError = ref('')
const nextError = ref('')
const savingPw = ref(false)
const confirmarSaidaTotal = ref(false)
const saindo = ref(false)

// 7 de ouros, ás de espadas e 3 de copas: ids 12, 29 e 38
const PREVIEW = [12, 29, 38]
const DECKS: { id: DeckStyle; label: string }[] = [
  { id: 'frances', label: 'Francês' },
  { id: 'espanhol', label: 'Espanhol' },
]

async function saveName() {
  nameError.value = ''
  savingName.value = true
  try {
    await session.updateDisplayName(displayName.value.trim())
    toasts.push('Nome atualizado.')
  } catch (err) {
    const f = formError(err)
    nameError.value = f.fields.displayName ?? f.message
  } finally {
    savingName.value = false
  }
}

async function savePassword() {
  currentError.value = ''
  nextError.value = ''
  savingPw.value = true
  try {
    await session.changePassword(current.value, next.value)
    current.value = ''
    next.value = ''
    toasts.push('Senha alterada.')
  } catch (err) {
    const f = formError(err)
    if (f.code === 'invalid_credentials') currentError.value = 'Senha atual incorreta.'
    else {
      currentError.value = f.fields.currentPassword ?? ''
      nextError.value = f.fields.newPassword ?? (currentError.value ? '' : f.message)
    }
  } finally {
    savingPw.value = false
  }
}

async function logout(all: boolean) {
  saindo.value = true
  try {
    await (all ? session.logoutAll() : session.logout())
  } catch (err) {
    saindo.value = false
    confirmarSaidaTotal.value = false
    toasts.fromError(err)
    return
  }
  confirmarSaidaTotal.value = false
  await router.replace({ name: 'entrar' })
}

function onVolume(e: Event) {
  prefs.setVolume(Number((e.target as HTMLInputElement).value))
}
</script>

<template>
  <div class="mx-auto flex w-full max-w-100 flex-col px-4 py-8 md:px-0 md:py-12">
    <h1 class="mb-8 text-2xl font-bold">
      Perfil
    </h1>

    <section
      class="flex flex-col gap-4 pb-8"
      aria-labelledby="perfil-nome"
    >
      <h2
        id="perfil-nome"
        class="text-xl font-bold"
      >
        Nome de exibição
      </h2>
      <form
        class="flex flex-col gap-4"
        novalidate
        @submit.prevent="saveName"
      >
        <Campo
          v-model="displayName"
          label="Como você aparece na mesa"
          autocomplete="nickname"
          :error="nameError"
        />
        <Botao
          type="submit"
          :loading="savingName"
          class="self-start"
        >
          Salvar nome
        </Botao>
      </form>
    </section>

    <section
      class="flex flex-col gap-4 border-t-2 border-line/40 py-8"
      aria-labelledby="perfil-senha"
    >
      <h2
        id="perfil-senha"
        class="text-xl font-bold"
      >
        Trocar senha
      </h2>
      <form
        class="flex flex-col gap-4"
        novalidate
        @submit.prevent="savePassword"
      >
        <Campo
          v-model="current"
          label="Senha atual"
          type="password"
          autocomplete="current-password"
          :error="currentError"
        />
        <Campo
          v-model="next"
          label="Nova senha"
          hint="Mínimo de 10 caracteres."
          type="password"
          autocomplete="new-password"
          :error="nextError"
        />
        <Botao
          type="submit"
          variant="secondary"
          :loading="savingPw"
          class="self-start"
        >
          Trocar senha
        </Botao>
      </form>
    </section>

    <section
      class="flex flex-col gap-4 border-t-2 border-line/40 py-8"
      aria-labelledby="perfil-prefs"
    >
      <h2
        id="perfil-prefs"
        class="text-xl font-bold"
      >
        Preferências
      </h2>
      <fieldset class="flex flex-col gap-3">
        <legend class="mb-3 text-sm font-bold">
          Baralho
        </legend>
        <div class="grid gap-3 sm:grid-cols-2">
          <label
            v-for="deck in DECKS"
            :key="deck.id"
            class="flex min-h-11 cursor-pointer flex-col items-start gap-3 rounded-campo border-2 p-3 has-focus-visible:outline-3 has-focus-visible:outline-offset-2 has-focus-visible:outline-focus"
            :class="prefs.deck === deck.id ? 'border-ink' : 'border-line/40 hover:border-line'"
          >
            <span
              class="flex"
              aria-hidden="true"
            >
              <img
                v-for="(id, i) in PREVIEW"
                :key="id"
                :src="cardUrl(id, deck.id)"
                alt=""
                class="w-(--carta-largura-mesa) rounded-card shadow-card"
                :class="i > 0 ? '-ml-6' : ''"
              >
            </span>
            <span class="flex items-center gap-2 font-bold">
              <input
                type="radio"
                name="deck"
                class="size-4 accent-ink"
                :value="deck.id"
                :checked="prefs.deck === deck.id"
                @change="prefs.setDeck(deck.id)"
              >
              {{ deck.label }}
            </span>
          </label>
        </div>
      </fieldset>
      <label class="flex min-h-11 flex-col gap-1 text-sm font-bold">
        <span class="flex justify-between">Volume <span class="font-normal tabular-nums text-ink-muted">{{ Math.round(prefs.volume * 100) }}%</span></span>
        <input
          type="range"
          min="0"
          max="1"
          step="0.05"
          class="h-6 w-full accent-ink"
          :value="prefs.volume"
          :aria-valuetext="`${Math.round(prefs.volume * 100)}%`"
          @input="onVolume"
        >
      </label>
      <Toggle
        :model-value="prefs.muted"
        label="Sem som"
        @update:model-value="prefs.setMuted($event)"
      />
    </section>

    <section
      class="flex flex-col gap-3 border-t-2 border-line/40 py-8"
      aria-labelledby="perfil-sessoes"
    >
      <h2
        id="perfil-sessoes"
        class="text-xl font-bold"
      >
        Sessões
      </h2>
      <p class="text-ink-muted">
        Entrou como <strong class="text-ink">{{ session.user?.username }}</strong> ({{ session.user?.email }}).
      </p>
      <div class="flex flex-wrap gap-3">
        <Botao
          variant="secondary"
          :loading="saindo && !confirmarSaidaTotal"
          @click="logout(false)"
        >
          Sair
        </Botao>
        <Botao
          variant="secondary"
          class="text-danger!"
          @click="confirmarSaidaTotal = true"
        >
          Sair de todos os dispositivos
        </Botao>
      </div>
    </section>

    <footer class="border-t-2 border-line/40 pt-6 text-sm text-ink-muted">
      <h2 class="mb-2 text-base font-bold text-ink">
        Créditos
      </h2>
      <ul class="flex flex-col gap-1">
        <li>Cartas francesas: Vector Playing Cards, de Byron Knoll (domínio público). O baralho espanhol e os versos foram criados para este projeto.</li>
        <li>Sons: pacotes Casino Audio e Interface Sounds, da Kenney (CC0).</li>
        <li>Fontes: Atkinson Hyperlegible Next e Kalam (licença OFL 1.1).</li>
      </ul>
    </footer>

    <Modal
      :open="confirmarSaidaTotal"
      title="Sair de todos os dispositivos?"
      @close="confirmarSaidaTotal = false"
    >
      <p class="mb-6">
        Você será desconectado aqui e em qualquer outro aparelho. Para voltar, entre de novo com usuário e senha.
      </p>
      <div class="flex justify-end gap-3">
        <Botao
          variant="secondary"
          @click="confirmarSaidaTotal = false"
        >
          Cancelar
        </Botao>
        <Botao
          variant="danger"
          :loading="saindo"
          @click="logout(true)"
        >
          Sair de todos os dispositivos
        </Botao>
      </div>
    </Modal>
  </div>
</template>
