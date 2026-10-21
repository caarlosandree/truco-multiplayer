<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useAdminStore, type AdminUser } from '@/stores/admin'
import Botao from '@/components/ui/Botao.vue'
import Campo from '@/components/ui/Campo.vue'
import Modal from '@/components/ui/Modal.vue'
import EstadoLista from './EstadoLista.vue'
import { quando } from './formato'

type Acao = 'disable' | 'enable' | 'reset'

const admin = useAdminStore()
const busca = ref('')
const aberto = ref(false)
const alvo = ref<AdminUser | null>(null)
const acao = ref<Acao>('disable')
const enviando = ref(false)

const TEXTOS: Record<Acao, { titulo: string; corpo: string; botao: string }> = {
  disable: {
    titulo: 'Desativar',
    corpo: 'A conta perde o acesso agora: a conexão aberta cai e os convites pendentes dessa pessoa expiram.',
    botao: 'Desativar',
  },
  enable: { titulo: 'Reativar', corpo: 'A pessoa volta a poder entrar com a mesma senha.', botao: 'Reativar' },
  reset: {
    titulo: 'Forçar troca de senha de',
    corpo: 'As sessões abertas são encerradas e a pessoa recebe por e-mail um link para definir uma senha nova.',
    botao: 'Forçar troca',
  },
}

function pedir(a: Acao, u: AdminUser) {
  acao.value = a
  alvo.value = u
  aberto.value = true
}
async function confirmar() {
  const u = alvo.value
  if (!u || enviando.value) return
  enviando.value = true
  const a = acao.value
  const ok = await (a === 'disable' ? admin.disableUser(u.id) : a === 'enable' ? admin.enableUser(u.id) : admin.forceReset(u.id))
  enviando.value = false
  // sucesso ou erro (já virou toast): o modal fecha e a linha reflete o estado real
  void ok
  aberto.value = false
}
function buscar() {
  void admin.loadUsers(busca.value.trim())
}

onMounted(buscar)
</script>

<template>
  <section
    aria-labelledby="usuarios-titulo"
    class="flex flex-col gap-4"
  >
    <h2
      id="usuarios-titulo"
      class="sr-only"
    >
      Usuários
    </h2>
    <p class="max-w-prose text-sm text-ink-muted">
      Admin é concedido só pelo servidor. Esta tela não concede nem retira esse acesso, e contas de administrador
      não têm ações aqui.
    </p>
    <form
      role="search"
      class="flex items-end gap-2"
      @submit.prevent="buscar"
    >
      <div class="min-w-0 flex-1 md:max-w-sm">
        <Campo
          v-model="busca"
          label="Buscar usuário"
          type="search"
          autocomplete="off"
        />
      </div>
      <Botao
        type="submit"
        variant="secondary"
      >
        Buscar
      </Botao>
    </form>
    <EstadoLista
      :status="admin.users.status"
      :total="admin.users.items.length"
      carregando="Carregando usuários"
      erro="Não foi possível carregar os usuários."
      vazio="Nenhum usuário encontrado"
      vazio-dica="Tente outro nome, usuário ou e-mail."
      @again="buscar"
    >
      <table
        role="table"
        class="w-full text-left text-sm max-md:block"
      >
        <caption class="sr-only">
          Usuários
        </caption>
        <thead
          role="rowgroup"
          class="max-md:sr-only"
        >
          <tr
            role="row"
            class="border-b-2 border-line/50 text-ink-muted"
          >
            <th
              role="columnheader"
              scope="col"
              class="h-10 pr-3 font-normal"
            >
              Usuário
            </th>
            <th
              role="columnheader"
              scope="col"
              class="pr-3 font-normal"
            >
              E-mail
            </th>
            <th
              role="columnheader"
              scope="col"
              class="pr-3 font-normal"
            >
              Situação
            </th>
            <th
              role="columnheader"
              scope="col"
              class="pr-3 font-normal"
            >
              Desde
            </th>
            <th
              role="columnheader"
              scope="col"
              class="text-right font-normal"
            >
              Ações
            </th>
          </tr>
        </thead>
        <tbody
          role="rowgroup"
          class="max-md:block"
        >
          <tr
            v-for="u in admin.users.items"
            :key="u.id"
            role="row"
            data-testid="usuario"
            class="border-b border-line/30 max-md:grid max-md:gap-1 max-md:py-3"
          >
            <td
              role="cell"
              class="py-2 pr-3 max-md:py-0"
            >
              <span class="font-bold">{{ u.displayName }}</span>
              <span class="ml-2 text-ink-muted">@{{ u.username }}</span>
            </td>
            <td
              role="cell"
              class="min-w-0 py-2 pr-3 break-all max-md:py-0"
            >
              {{ u.email }}
            </td>
            <td
              role="cell"
              class="py-2 pr-3 max-md:py-0"
            >
              <span class="flex flex-wrap gap-1">
                <span
                  v-if="u.isAdmin"
                  class="rounded-placa border border-line px-2 py-0.5 text-xs whitespace-nowrap font-bold"
                >admin</span>
                <span
                  v-if="u.disabled"
                  class="rounded-placa border border-danger px-2 py-0.5 text-xs whitespace-nowrap font-bold text-danger"
                >Desativada</span>
                <span
                  v-if="u.mustResetPassword"
                  class="rounded-placa border border-line px-2 py-0.5 text-xs whitespace-nowrap"
                >Troca de senha pendente</span>
                <span
                  v-if="!u.emailVerified"
                  class="rounded-placa border border-line px-2 py-0.5 text-xs whitespace-nowrap"
                >E-mail não confirmado</span>
                <span
                  v-if="!u.isAdmin && !u.disabled && !u.mustResetPassword && u.emailVerified"
                  class="text-ink-muted"
                >Ativa</span>
              </span>
            </td>
            <td
              role="cell"
              class="py-2 pr-3 whitespace-nowrap text-ink-muted max-md:py-0"
            >
              {{ quando(u.createdAt) }}
            </td>
            <td
              role="cell"
              class="py-1 text-right max-md:py-0 max-md:text-left"
            >
              <span
                v-if="u.isAdmin"
                class="text-ink-muted"
              >Sem ações</span>
              <span
                v-else
                class="inline-flex justify-end gap-2 whitespace-nowrap max-md:mt-1 max-md:flex-wrap"
              >
                <Botao
                  v-if="!u.disabled"
                  variant="secondary"
                  class="h-11! text-sm md:h-9!"
                  :aria-label="`Desativar ${u.displayName}`"
                  @click="pedir('disable', u)"
                >
                  Desativar
                </Botao>
                <Botao
                  v-else
                  variant="secondary"
                  class="h-11! text-sm md:h-9!"
                  :aria-label="`Reativar ${u.displayName}`"
                  @click="pedir('enable', u)"
                >
                  Reativar
                </Botao>
                <Botao
                  variant="ghost"
                  class="h-11! text-sm md:h-9!"
                  :aria-label="`Forçar troca de senha de ${u.displayName}`"
                  @click="pedir('reset', u)"
                >
                  Forçar troca de senha
                </Botao>
              </span>
            </td>
          </tr>
        </tbody>
      </table>
      <Botao
        v-if="admin.users.nextCursor"
        variant="secondary"
        class="self-start"
        :loading="admin.users.more"
        @click="admin.loadMoreUsers()"
      >
        Carregar mais
      </Botao>
    </EstadoLista>

    <Modal
      :open="aberto"
      :title="`${TEXTOS[acao].titulo} ${alvo?.displayName ?? ''}?`"
      descricao-id="usuarios-modal-texto"
      @close="aberto = false"
    >
      <p
        id="usuarios-modal-texto"
        class="mb-5 text-ink-muted"
      >
        {{ TEXTOS[acao].corpo }}
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
          :variant="acao === 'enable' ? 'primary' : 'danger'"
          :loading="enviando"
          @click="confirmar"
        >
          {{ TEXTOS[acao].botao }}
        </Botao>
      </div>
    </Modal>
  </section>
</template>
