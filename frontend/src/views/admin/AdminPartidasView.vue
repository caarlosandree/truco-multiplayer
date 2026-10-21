<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useAdminStore, type LiveMatch } from '@/stores/admin'
import Botao from '@/components/ui/Botao.vue'
import Campo from '@/components/ui/Campo.vue'
import Modal from '@/components/ui/Modal.vue'
import EstadoLista from './EstadoLista.vue'
import { quando } from './formato'

const admin = useAdminStore()
const aberto = ref(false)
const alvo = ref<LiveMatch | null>(null)
const motivo = ref('')
const enviando = ref(false)
const podeConfirmar = computed(() => motivo.value.trim() !== '')

const jogadores = (m: LiveMatch) =>
  [...m.players].sort((a, b) => a.seat - b.seat).map((p) => p.displayName).join(', ')

function pedir(m: LiveMatch) {
  alvo.value = m
  motivo.value = ''
  aberto.value = true
}
async function abortar() {
  const m = alvo.value
  if (!m || !podeConfirmar.value || enviando.value) return
  enviando.value = true
  await admin.abortMatch(m.id, motivo.value.trim())
  enviando.value = false
  aberto.value = false
}

onMounted(() => void admin.loadMatches())
</script>

<template>
  <section
    aria-labelledby="partidas-titulo"
    class="flex flex-col gap-4"
  >
    <h2
      id="partidas-titulo"
      class="sr-only"
    >
      Partidas em andamento
    </h2>
    <div class="flex flex-wrap items-center justify-between gap-3">
      <p class="max-w-prose text-sm text-ink-muted">
        Partidas que ainda não terminaram. Abortar encerra a partida para todos.
      </p>
      <Botao
        variant="secondary"
        :disabled="admin.matches.status === 'loading'"
        @click="admin.loadMatches()"
      >
        Atualizar
      </Botao>
    </div>
    <EstadoLista
      :status="admin.matches.status"
      :total="admin.matches.items.length"
      carregando="Carregando partidas"
      erro="Não foi possível carregar as partidas."
      vazio="Nenhuma partida em andamento"
      vazio-dica="Quando alguém começar a jogar, a partida aparece aqui."
      @again="admin.loadMatches()"
    >
      <ul class="flex flex-col text-sm">
        <li
          v-for="m in admin.matches.items"
          :key="m.id"
          data-testid="partida"
          class="flex flex-wrap items-center gap-x-4 gap-y-1 border-b border-line/30 py-2"
        >
          <span class="min-w-0 flex-1 basis-60">
            <span class="block font-bold">{{ jogadores(m) }}</span>
            <span class="block text-ink-muted">Início em {{ quando(m.createdAt) }}</span>
          </span>
          <Botao
            variant="secondary"
            class="h-11! shrink-0 text-sm md:h-9!"
            :aria-label="`Abortar a partida de ${jogadores(m)}`"
            @click="pedir(m)"
          >
            Abortar
          </Botao>
        </li>
      </ul>
      <Botao
        v-if="admin.matches.nextCursor"
        variant="secondary"
        class="self-start"
        :loading="admin.matches.more"
        @click="admin.loadMoreMatches()"
      >
        Carregar mais
      </Botao>
    </EstadoLista>

    <Modal
      :open="aberto"
      title="Abortar esta partida?"
      descricao-id="partidas-modal-texto"
      @close="aberto = false"
    >
      <p
        id="partidas-modal-texto"
        class="mb-4 text-ink-muted"
      >
        {{ alvo ? jogadores(alvo) : '' }}. A partida é encerrada para todos e fica registrada na auditoria com o motivo.
      </p>
      <form
        class="flex flex-col gap-5"
        @submit.prevent="abortar"
      >
        <Campo
          v-model="motivo"
          label="Motivo"
          hint="Obrigatório. Aparece na auditoria."
          autocomplete="off"
          required
        />
        <div class="flex flex-wrap justify-end gap-2">
          <Botao
            variant="secondary"
            data-autofocus
            @click="aberto = false"
          >
            Cancelar
          </Botao>
          <Botao
            type="submit"
            variant="danger"
            :disabled="!podeConfirmar"
            :loading="enviando"
          >
            Abortar partida
          </Botao>
        </div>
      </form>
    </Modal>
  </section>
</template>
