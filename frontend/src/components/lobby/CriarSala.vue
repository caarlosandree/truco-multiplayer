<script setup lang="ts">
import { ref } from 'vue'
import type { Visibility } from '@/protocol/messages'
import { useLobbyStore } from '@/stores/lobby'
import { useToastsStore } from '@/stores/toasts'
import Botao from '@/components/ui/Botao.vue'
import Modal from '@/components/ui/Modal.vue'

defineProps<{ open: boolean }>()
const emit = defineEmits<{ close: [] }>()
const lobby = useLobbyStore()
const toasts = useToastsStore()
const busy = ref<Visibility | null>(null)

async function create(visibility: Visibility) {
  busy.value = visibility
  try {
    await lobby.createRoom(visibility)
    emit('close')
  } catch (err) {
    toasts.fromError(err)
  } finally {
    busy.value = null
  }
}
</script>

<template>
  <Modal
    :open="open"
    title="Criar sala"
    @close="emit('close')"
  >
    <div class="flex flex-col gap-5">
      <div class="flex flex-col gap-2">
        <Botao
          data-criar="public"
          :loading="busy === 'public'"
          :disabled="busy !== null"
          @click="create('public')"
        >
          Sala pública
        </Botao>
        <p class="text-sm text-ink-muted">
          Aparece na lista e qualquer pessoa pode sentar.
        </p>
      </div>
      <div class="flex flex-col gap-2">
        <Botao
          data-criar="private"
          variant="secondary"
          :loading="busy === 'private'"
          :disabled="busy !== null"
          @click="create('private')"
        >
          Sala privada
        </Botao>
        <p class="text-sm text-ink-muted">
          Fica fora da lista; entra quem tiver o código ou o link.
        </p>
      </div>
      <div class="flex justify-end">
        <Botao
          variant="ghost"
          @click="emit('close')"
        >
          Cancelar
        </Botao>
      </div>
    </div>
  </Modal>
</template>
