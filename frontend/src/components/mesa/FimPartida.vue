<script setup lang="ts">
import { computed, ref, useId } from 'vue'
import { endReasonText } from '@/game/labels'
import { useRoomStore } from '@/stores/room'
import { useToastsStore } from '@/stores/toasts'
import Botao from '@/components/ui/Botao.vue'
import Modal from '@/components/ui/Modal.vue'
import SeloBot from '@/components/ui/SeloBot.vue'
import Tentos from './Tentos.vue'

/**
 * O fim da partida: o momento grande. O resultado é escrito na lousa (Kalam, como os anúncios da mesa) com o
 * placar final em tentos; o resto do modal é cromo quieto. Duplas sempre relativas: "Nós" é a minha.
 */
const room = useRoomStore()
const toasts = useToastsStore()
const descId = useId()
const saindo = ref(false)

const e = computed(() => room.ended)
const resultado = computed<'vitoria' | 'derrota' | 'sem'>(() => {
  const fim = e.value
  if (!fim || fim.winner === null || room.myTeam === null) return 'sem'
  return fim.winner === room.myTeam ? 'vitoria' : 'derrota'
})
const TEXTO = { vitoria: 'Vitória!', derrota: 'Derrota', sem: 'Sem vencedor' } as const
const me = computed(() => room.myTeam ?? 0)
const nos = computed(() => e.value?.score[me.value] ?? 0)
const eles = computed(() => e.value?.score[1 - me.value] ?? 0)

async function sair() {
  saindo.value = true
  try {
    await room.leave()
    room.dismissEnded()
  } catch (err) {
    toasts.fromError(err)
  } finally {
    saindo.value = false
  }
}
</script>

<template>
  <Modal
    :open="e !== null"
    title="Fim da partida"
    :descricao-id="descId"
    @close="room.dismissEnded()"
  >
    <template v-if="e">
      <div
        :id="descId"
        data-tema="escuro"
        data-fim
        :data-resultado="resultado"
        class="textura-lousa rounded-valor px-4 py-6 text-center text-giz"
      >
        <p
          class="relative mx-auto w-fit -rotate-2 px-2 font-display font-bold"
          :class="resultado === 'sem' ? 'text-4xl' : 'text-anuncio-sm'"
        >
          <span :class="resultado === 'vitoria' ? 'text-giz' : 'text-giz-muted'">{{ TEXTO[resultado] }}</span>
          <svg
            v-if="resultado === 'vitoria'"
            class="absolute inset-x-2 -bottom-2 h-3 w-[calc(100%-1rem)]"
            viewBox="0 0 100 8"
            preserveAspectRatio="none"
            aria-hidden="true"
          >
            <path
              d="M1 5 Q30 1 60 5 T99 3"
              fill="none"
              pathLength="1"
              class="mesa-riscar mesa-riscar--sublinha stroke-nos"
              style="stroke-width: 3px; stroke-dasharray: 1; stroke-linecap: round; vector-effect: non-scaling-stroke"
            />
          </svg>
        </p>
        <p
          v-if="e.reason !== 'game_over'"
          class="mt-3 text-base text-giz-muted"
          data-motivo
        >
          {{ endReasonText(e.reason) }}
        </p>
        <p
          v-if="e.ranked === false"
          data-sem-ranking
          class="mt-3 text-balance text-base text-giz-muted"
        >
          <SeloBot class="mr-2 align-middle" />
          Partida com bots — não conta no ranking.
        </p>
        <p
          class="sr-only"
          data-placar-final
        >
          Placar final: Nós {{ nos }}, Eles {{ eles }}.
        </p>
        <div
          class="mt-6 flex items-start justify-center gap-10"
          aria-hidden="true"
        >
          <div class="flex flex-col items-center gap-1">
            <span class="text-sm font-bold text-nos">Nós</span>
            <span class="min-w-[2ch] text-center font-display text-placar font-bold text-nos">{{ nos }}</span>
            <Tentos
              :count="nos"
              tone="nos"
            />
          </div>
          <div class="flex flex-col items-center gap-1">
            <span class="text-sm font-bold text-eles">Eles</span>
            <span class="min-w-[2ch] text-center font-display text-placar font-bold text-eles">{{ eles }}</span>
            <Tentos
              :count="eles"
              tone="eles"
            />
          </div>
        </div>
      </div>
      <div class="mt-6 flex justify-end gap-3">
        <Botao
          variant="secondary"
          data-acao="sair"
          :loading="saindo"
          @click="sair"
        >
          Sair da sala
        </Botao>
        <Botao
          data-acao="revanche"
          data-autofocus
          @click="room.dismissEnded()"
        >
          Revanche
        </Botao>
      </div>
    </template>
  </Modal>
</template>
