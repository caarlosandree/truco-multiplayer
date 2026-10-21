<script setup lang="ts">
import { onMounted } from 'vue'
import { useAdminStore, type AuditEntry } from '@/stores/admin'
import Botao from '@/components/ui/Botao.vue'
import EstadoLista from './EstadoLista.vue'
import { quando } from './formato'

const admin = useAdminStore()

const ACOES: Record<string, string> = {
  'user.disable': 'Desativou a conta',
  'user.enable': 'Reativou a conta',
  'user.force_password_reset': 'Forçou a troca de senha',
  'user.promote_admin': 'Concedeu acesso de administrador',
  'user.demote_admin': 'Retirou acesso de administrador',
  'user.set_admin': 'Admin alterado (rota antiga)',
  'domain.add': 'Adicionou o domínio',
  'domain.remove': 'Removeu o domínio',
  'match.abort': 'Abortou a partida',
}
const ALVOS: Record<string, string> = { user: 'Usuário', domain: 'Domínio', match: 'Partida' }
const ROTULOS: Record<string, string> = { reason: 'Motivo' }

const acao = (e: AuditEntry) => ACOES[e.action] ?? e.action
const alvo = (e: AuditEntry) => ALVOS[e.targetType] ?? e.targetType
/** O CLI grava o próprio alvo como ator, com details.via == "cli". */
const doCli = (e: AuditEntry) => e.details?.via === 'cli'
const ator = (e: AuditEntry) => (doCli(e) ? 'CLI do servidor' : e.actorUsername)
/** Detalhes legíveis, sem o marcador técnico `via`. */
function detalhes(e: AuditEntry): string {
  return Object.entries(e.details ?? {})
    .filter(([k]) => k !== 'via')
    .map(([k, v]) => `${ROTULOS[k] ?? k}: ${typeof v === 'string' ? v : JSON.stringify(v)}`)
    .join(' · ')
}

onMounted(() => void admin.loadAudit())
</script>

<template>
  <section
    aria-labelledby="auditoria-titulo"
    class="flex flex-col gap-4"
  >
    <h2
      id="auditoria-titulo"
      class="sr-only"
    >
      Auditoria
    </h2>
    <p class="max-w-prose text-sm text-ink-muted">
      Tudo o que mudou pelo painel e pelo servidor, da ação mais recente para a mais antiga.
    </p>
    <EstadoLista
      :status="admin.audit.status"
      :total="admin.audit.items.length"
      carregando="Carregando a auditoria"
      erro="Não foi possível carregar a auditoria."
      vazio="Nenhuma ação registrada"
      @again="admin.loadAudit()"
    >
      <table
        role="table"
        class="w-full text-left text-sm max-md:block"
      >
        <caption class="sr-only">
          Auditoria
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
              Quando
            </th>
            <th
              role="columnheader"
              scope="col"
              class="pr-3 font-normal"
            >
              Ator
            </th>
            <th
              role="columnheader"
              scope="col"
              class="pr-3 font-normal"
            >
              Ação
            </th>
            <th
              role="columnheader"
              scope="col"
              class="pr-3 font-normal"
            >
              Alvo
            </th>
            <th
              role="columnheader"
              scope="col"
              class="font-normal"
            >
              Detalhes
            </th>
          </tr>
        </thead>
        <tbody
          role="rowgroup"
          class="max-md:block"
        >
          <tr
            v-for="e in admin.audit.items"
            :key="e.id"
            role="row"
            data-testid="auditoria"
            class="border-b border-line/30 align-top max-md:grid max-md:gap-0.5 max-md:py-3"
          >
            <td
              role="cell"
              class="py-2 pr-3 whitespace-nowrap text-ink-muted max-md:py-0"
            >
              {{ quando(e.createdAt) }}
            </td>
            <td
              role="cell"
              class="py-2 pr-3 font-bold max-md:py-0"
            >
              {{ ator(e) }}
            </td>
            <td
              role="cell"
              class="py-2 pr-3 max-md:py-0"
            >
              {{ acao(e) }}
            </td>
            <td
              role="cell"
              class="py-2 pr-3 max-md:py-0"
            >
              <span>{{ alvo(e) }}</span>
              <span class="ml-1 break-all text-ink-muted">{{ e.targetId }}</span>
            </td>
            <td
              role="cell"
              class="py-2 break-words max-md:py-0"
              :class="detalhes(e) ? '' : 'max-md:hidden'"
            >
              <template v-if="detalhes(e)">
                {{ detalhes(e) }}
              </template>
              <span
                v-else
                class="text-ink-muted"
              >—</span>
            </td>
          </tr>
        </tbody>
      </table>
      <Botao
        v-if="admin.audit.nextCursor"
        variant="secondary"
        class="self-start"
        :loading="admin.audit.more"
        @click="admin.loadMoreAudit()"
      >
        Carregar mais
      </Botao>
    </EstadoLista>
  </section>
</template>
