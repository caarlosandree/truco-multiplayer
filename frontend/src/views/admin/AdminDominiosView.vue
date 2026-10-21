<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useAdminStore } from '@/stores/admin'
import Botao from '@/components/ui/Botao.vue'
import Campo from '@/components/ui/Campo.vue'
import Modal from '@/components/ui/Modal.vue'
import EstadoLista from './EstadoLista.vue'
import { quando } from './formato'

const admin = useAdminStore()
const novo = ref('')
const adicionando = ref(false)
const aberto = ref(false)
const alvo = ref('')
const removendo = ref(false)

async function adicionar() {
  const d = novo.value.trim()
  if (!d || adicionando.value) return
  adicionando.value = true
  const ok = await admin.addDomain(d)
  adicionando.value = false
  if (ok) novo.value = ''
}
function pedirRemocao(domain: string) {
  alvo.value = domain
  aberto.value = true
}
async function remover() {
  if (removendo.value) return
  removendo.value = true
  await admin.removeDomain(alvo.value)
  removendo.value = false
  aberto.value = false
}

onMounted(() => void admin.loadDomains())
</script>

<template>
  <section
    aria-labelledby="dominios-titulo"
    class="flex flex-col gap-4"
  >
    <h2
      id="dominios-titulo"
      class="sr-only"
    >
      Domínios de e-mail
    </h2>
    <p class="max-w-prose text-sm text-ink-muted">
      Só quem tem e-mail de um destes domínios consegue criar conta (por exemplo, empresa.com.br). Quem já tem conta não é afetado.
    </p>
    <form
      class="flex items-end gap-2"
      @submit.prevent="adicionar"
    >
      <div class="min-w-0 flex-1 md:max-w-sm">
        <Campo
          v-model="novo"
          label="Novo domínio"
          autocomplete="off"
        />
      </div>
      <Botao
        type="submit"
        :loading="adicionando"
        :disabled="novo.trim() === ''"
      >
        Adicionar
      </Botao>
    </form>
    <EstadoLista
      :status="admin.domains.status"
      :total="admin.domains.items.length"
      carregando="Carregando domínios"
      erro="Não foi possível carregar os domínios."
      vazio="Nenhum domínio permitido"
      @again="admin.loadDomains()"
    >
      <ul class="flex flex-col text-sm">
        <li
          v-for="d in admin.domains.items"
          :key="d.domain"
          data-testid="dominio"
          class="flex min-h-12 flex-wrap items-center gap-x-3 gap-y-1 border-b border-line/30 py-1"
        >
          <span class="min-w-0 flex-1 basis-40 break-all font-bold">{{ d.domain }}</span>
          <span class="shrink-0 tabular-nums text-ink-muted">{{ quando(d.createdAt) }}</span>
          <Botao
            variant="secondary"
            class="h-11! shrink-0 text-sm md:h-9!"
            :aria-label="`Remover ${d.domain}`"
            @click="pedirRemocao(d.domain)"
          >
            Remover
          </Botao>
        </li>
      </ul>
    </EstadoLista>

    <Modal
      :open="aberto"
      :title="`Remover ${alvo}?`"
      descricao-id="dominios-modal-texto"
      @close="aberto = false"
    >
      <p
        id="dominios-modal-texto"
        class="mb-5 text-ink-muted"
      >
        Novos cadastros com este domínio deixam de ser aceitos. Quem já tem conta continua entrando.
      </p>
      <div class="flex flex-wrap justify-end gap-2">
        <Botao
          variant="secondary"
          data-autofocus
          @click="aberto = false"
        >
          Cancelar
        </Botao>
        <Botao
          variant="danger"
          :loading="removendo"
          @click="remover"
        >
          Remover
        </Botao>
      </div>
    </Modal>
  </section>
</template>
