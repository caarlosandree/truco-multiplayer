<script setup lang="ts">
import { computed, ref, useId } from 'vue'
import { useRoomStore } from '@/stores/room'
import { useToastsStore } from '@/stores/toasts'
import Botao from '@/components/ui/Botao.vue'
import Modal from '@/components/ui/Modal.vue'

/**
 * Sair no meio da partida (leave_room em jogo). Sozinho com bots, a partida termina na hora e não conta;
 * com outros humanos na mesa, um bot assume o lugar até o fim e a cadeira fica vazia na revanche.
 * Sempre pede confirmação: não há volta para a mesma partida.
 */
const room = useRoomStore()
const toasts = useToastsStore()
const aberto = ref(false)
const saindo = ref(false)
const descricaoId = useId()

const comOutrosHumanos = computed(() => {
  const r = room.room
  return r !== null && r.seats.some((s, i) => s !== null && !s.bot && i !== r.yourSeat)
})
const conta = computed(() => room.room?.ranked === true)

async function sair() {
  saindo.value = true
  try {
    await room.leave()
  } catch (err) {
    toasts.fromError(err)
  } finally {
    saindo.value = false
    aberto.value = false
  }
}
</script>

<template>
  <div class="max-lg:absolute max-lg:bottom-[max(0.75rem,env(safe-area-inset-bottom))] max-lg:left-3 lg:mt-auto">
    <button
      type="button"
      data-acao="sair-partida"
      class="inline-flex h-11 min-w-11 items-center gap-2 rounded-botao-2 px-3 text-sm font-bold text-(--painel-dica) hover:bg-giz/10 hover:text-(--painel-fg)"
      @click="aberto = true"
    >
      <svg
        viewBox="0 0 16 16"
        class="size-4 shrink-0"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <path d="M6 14H3V2h3M10.5 11.5 14 8l-3.5-3.5M14 8H6" />
      </svg>
      Sair da partida
    </button>
    <Modal
      :open="aberto"
      title="Sair da partida?"
      :descricao-id="descricaoId"
      @close="aberto = false"
    >
      <div
        :id="descricaoId"
        class="space-y-2 text-base"
      >
        <template v-if="comOutrosHumanos">
          <p>Um bot assume o seu lugar até o fim da partida, e você não volta para esta mesa.</p>
          <p v-if="conta">
            Se a sua dupla perder, a derrota conta para você no ranking.
          </p>
        </template>
        <p v-else>
          A partida termina agora e não conta no ranking.
        </p>
      </div>
      <div class="mt-6 flex justify-end gap-2">
        <Botao
          data-acao="cancelar-saida"
          data-autofocus
          variant="secondary"
          @click="aberto = false"
        >
          Continuar jogando
        </Botao>
        <Botao
          data-acao="confirmar-saida"
          variant="danger"
          :loading="saindo"
          @click="sair"
        >
          Sair
        </Botao>
      </div>
    </Modal>
  </div>
</template>
