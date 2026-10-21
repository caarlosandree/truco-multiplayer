<script setup lang="ts">
import { nextTick, ref, useTemplateRef, watch } from 'vue'
import { normalizeCode, useLobbyStore } from '@/stores/lobby'
import { errorText } from '@/game/labels'
import { focusFirstInvalid } from '@/app/formError'
import { useToastsStore } from '@/stores/toasts'
import Botao from '@/components/ui/Botao.vue'
import Campo from '@/components/ui/Campo.vue'
import Modal from '@/components/ui/Modal.vue'

const CODE_LENGTH = 6
const CODE_ERRORS = ['room_not_found', 'room_full', 'seat_taken', 'already_in_room']

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ close: [] }>()
const lobby = useLobbyStore()
const toasts = useToastsStore()
const form = useTemplateRef<HTMLFormElement>('form')
const code = ref('')
const error = ref('')
const loading = ref(false)

watch(
  () => props.open,
  (open) => {
    if (open) {
      code.value = ''
      error.value = ''
    }
  },
)

async function falhar(message: string) {
  error.value = message
  await nextTick()
  focusFirstInvalid(form.value)
}

async function submit() {
  const normalized = normalizeCode(code.value)
  if (!normalized) {
    await falhar('Digite o código da sala.')
    return
  }
  if (normalized.length !== CODE_LENGTH) {
    await falhar(`O código tem ${CODE_LENGTH} caracteres.`)
    return
  }
  error.value = ''
  loading.value = true
  try {
    await lobby.joinByCode(code.value)
    emit('close')
  } catch (err) {
    const c = (err as { code?: unknown } | null)?.code
    if (typeof c === 'string' && CODE_ERRORS.includes(c)) await falhar(errorText(c))
    else toasts.fromError(err)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <Modal
    :open="open"
    title="Entrar com código"
    @close="emit('close')"
  >
    <form
      ref="form"
      data-form="codigo"
      class="flex flex-col gap-5"
      novalidate
      @submit.prevent="submit"
    >
      <Campo
        v-model="code"
        label="Código da sala"
        hint="Seis letras e números, como K7Q2XM."
        autocomplete="off"
        :error="error"
      />
      <div class="flex justify-end gap-2">
        <Botao
          variant="ghost"
          @click="emit('close')"
        >
          Cancelar
        </Botao>
        <Botao
          type="submit"
          :loading="loading"
        >
          Entrar
        </Botao>
      </div>
    </form>
  </Modal>
</template>
