<script setup lang="ts">
import { computed, ref } from 'vue'
import type { Seat } from '@/protocol/engine'
import { useRoomStore } from '@/stores/room'
import { useToastsStore } from '@/stores/toasts'
import { relativePosition, teamOf, type RelativePosition } from '@/game/seats'
import Assento from './Assento.vue'
import Toggle from '@/components/ui/Toggle.vue'
import Botao from '@/components/ui/Botao.vue'
import Modal from '@/components/ui/Modal.vue'

const room = useRoomStore()
const toasts = useToastsStore()
const r = computed(() => {
  if (!room.room) throw new Error('SalaEspera sem sala')
  return room.room
})
const inviteLink = computed(() => `${location.origin}/sala/${r.value.code}`)
const removendo = ref<Seat | null>(null)
const removendoNome = computed(() => (removendo.value === null ? '' : (r.value.seats[removendo.value]?.displayName ?? '')))

// celular: parceiro em cima, mesa, adversários lado a lado, eu embaixo; desktop: losango em volta do oval
const POS_CLASS: Record<RelativePosition, string> = {
  top: 'col-span-2 row-start-1 md:col-span-1 md:col-start-2',
  left: 'col-start-1 row-start-3 md:row-start-2',
  right: 'col-start-2 row-start-3 md:col-start-3 md:row-start-2',
  bottom: 'col-span-2 row-start-4 md:col-span-1 md:col-start-2 md:row-start-3',
}

async function run(p: Promise<void>) {
  try {
    await p
  } catch (err) {
    toasts.fromError(err)
  }
}

async function copy() {
  try {
    await navigator.clipboard.writeText(inviteLink.value)
    toasts.push('Link copiado.')
  } catch {
    toasts.push(`Copie o link: ${inviteLink.value}`)
  }
}

async function confirmarRemocao() {
  const seat = removendo.value
  removendo.value = null
  if (seat !== null) await run(room.kick(seat))
}

const seats = computed(() =>
  ([0, 1, 2, 3] as Seat[]).map((s) => ({
    seat: s,
    info: r.value.seats[s] ?? null,
    pos: relativePosition(s, r.value.yourSeat),
    ally: teamOf(s) === teamOf(r.value.yourSeat),
  })),
)
const faltam = computed(() => r.value.seats.filter((s) => s === null).length)
const faltamProntos = computed(() => r.value.seats.filter((s) => s !== null && !s.ready).length)
const resumo = computed(() => {
  if (faltam.value > 0) return faltam.value === 1 ? 'Falta 1 jogador.' : `Faltam ${faltam.value} jogadores.`
  if (faltamProntos.value > 0) return faltamProntos.value === 1 ? 'Falta 1 pronto.' : `Faltam ${faltamProntos.value} prontos.`
  return 'Todos prontos. Começando…'
})
</script>

<template>
  <div class="flex flex-1 flex-col">
    <header class="flex flex-wrap items-center justify-between gap-x-6 gap-y-3 px-4 py-4 md:px-6">
      <div class="min-w-0">
        <h1 class="truncate text-2xl font-bold">
          {{ r.name }}
        </h1>
        <p class="text-sm text-ink-muted">
          <template v-if="r.visibility === 'private'">
            Sala privada · código <strong
              class="rounded-placa px-1.5 py-0.5 text-base text-ink inset-ring-2 inset-ring-line"
              data-testid="codigo"
            >{{ r.code }}</strong>
          </template>
          <template v-else>
            Sala pública
          </template>
        </p>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <Botao
          v-if="r.visibility === 'private'"
          data-acao="copiar"
          variant="ghost"
          @click="copy"
        >
          Copiar link
        </Botao>
        <Botao
          data-acao="sair"
          variant="secondary"
          @click="run(room.leave())"
        >
          Sair da sala
        </Botao>
      </div>
    </header>

    <section
      data-tema="escuro"
      aria-label="Mesa de espera"
      class="textura-feltro flex flex-1 flex-col items-center justify-center gap-8 px-4 py-8 text-giz md:px-6 md:py-10"
    >
      <div class="relative w-full max-w-3xl">
        <!-- o oval da mesa: só decoração, atrás dos lugares -->
        <div
          aria-hidden="true"
          class="absolute inset-x-0 inset-y-6 rounded-[50%] border-4 border-felt-deep bg-felt/60 md:inset-x-16 md:inset-y-4"
        />
        <ul
          class="relative grid grid-cols-2 gap-x-4 gap-y-5 md:grid-cols-3 md:gap-y-6"
          aria-label="Lugares da mesa"
        >
          <li
            v-for="s in seats"
            :key="s.seat"
            class="flex justify-center"
            :class="POS_CLASS[s.pos]"
          >
            <Assento
              :seat="s.seat"
              :info="s.info"
              :is-me="s.seat === r.yourSeat"
              :is-host="s.seat === r.host"
              :can-kick="room.isHost && s.info !== null && s.seat !== r.yourSeat"
              :ally="s.ally"
              @take="run(room.takeSeat(s.seat))"
              @kick="removendo = s.seat"
            />
          </li>
          <li class="col-span-2 row-start-2 flex items-center justify-center text-center md:col-span-1 md:col-start-2">
            <p
              class="max-w-56 text-base text-giz"
              role="status"
            >
              {{ resumo }}
              <span class="mt-1 block text-sm text-giz-muted">A partida começa quando os 4 estiverem prontos.</span>
            </p>
          </li>
        </ul>
      </div>

      <div class="textura-lousa rounded-painel px-4 py-2">
        <Toggle
          :model-value="room.amReady"
          label="Estou pronto"
          @update:model-value="run(room.setReady($event))"
        />
      </div>
    </section>

    <Modal
      :open="removendo !== null"
      title="Remover jogador?"
      @close="removendo = null"
    >
      <p class="text-base">
        {{ removendoNome }} sai da sala e perde o lugar.
      </p>
      <div class="mt-6 flex justify-end gap-2">
        <Botao
          data-acao="cancelar-remocao"
          variant="secondary"
          @click="removendo = null"
        >
          Cancelar
        </Botao>
        <Botao
          data-acao="confirmar-remocao"
          variant="danger"
          @click="confirmarRemocao"
        >
          Remover
        </Botao>
      </div>
    </Modal>
  </div>
</template>
