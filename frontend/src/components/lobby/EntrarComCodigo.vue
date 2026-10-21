<script setup lang="ts">
import { ref, watch } from 'vue'
import { normalizeCode, useLobbyStore } from '@/stores/lobby'
import { errorText } from '@/game/labels'
import Botao from '@/components/ui/Botao.vue'
import Campo from '@/components/ui/Campo.vue'
import Modal from '@/components/ui/Modal.vue'

const CODE_LENGTH = 6
const CODE_ERRORS = ['room_not_found', 'room_full', 'seat_taken', 'already_in_room']

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ close: [] }>()
const lobby = useLobbyStore()
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

async function submit() {
  const normalized = normalizeCode(code.value)
  if (!normalized) {
    error.value = 'Digite o código da sala.'
    return
  }
  if (normalized.length !== CODE_LENGTH) {
    error.value = `O código tem ${CODE_LENGTH} caracteres.`
    return
  }
  error.value = ''
  loading.value = true
  try {
    await lobby.joinByCode(code.value)
    emit('close')
  } catch (err) {
    const c = (err as { code?: unknown } | null)?.code
    error.value = typeof c === 'string' && CODE_ERRORS.includes(c) ? errorText(c) : errorText('internal')
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
