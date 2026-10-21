<script setup lang="ts">
import { computed, inject, ref, type Ref } from 'vue'
import { useGameStore } from '@/stores/game'
import { handKindText, stakeName } from '@/game/labels'
import { teamOf } from '@/game/seats'
import Corte from './Corte.vue'
import Queima from './Queima.vue'
import DecisaoOnze from './DecisaoOnze.vue'
import RespostaAumento from './RespostaAumento.vue'
import TeclaDica from './TeclaDica.vue'
import Botao from '@/components/ui/Botao.vue'
import Toggle from '@/components/ui/Toggle.vue'
import Modal from '@/components/ui/Modal.vue'

/** `ocioso`: o que dizer quando não há nada a decidir (ex.: "Vez de Jô"); vem da mesa. */
const props = withDefaults(defineProps<{ ocioso?: string }>(), { ocioso: '' })

const game = useGameStore()
const covered = inject<Ref<boolean>>('coberta', ref(false))
const confirming = ref(false)
const bloqueado = computed(() => game.inflight || !game.online)

const nameOf = (seat: number) => game.latestView?.players.find((p) => p.seat === seat)?.name ?? `Lugar ${seat + 1}`

const mode = computed(() => {
  if (game.mustChooseCut) return 'corte'
  if (game.mustBurn) return 'queima'
  if (game.mustDecideEleven) return 'onze'
  if (game.mustRespondRaise) return 'resposta'
  if (game.canPlay) return 'jogar'
  return null
})

// uma linha de contexto acima dos botões (brief §2.3); offline ela vira a razão do bloqueio
const contexto = computed(() => {
  if (!game.online) return 'Reconectando…'
  switch (mode.value) {
    case 'corte':
      return game.phase === 'dealer_choice'
        ? 'Você é o pé: escolha a sua carta. A outra vai para o parceiro.'
        : 'Corte: escolha a sua carta. A outra vai para o parceiro.'
    case 'queima':
      return 'Queima dos 4: marque os que quer trocar, ou passe.'
    case 'onze':
      return 'Mão de 11: sua dupla joga ou corre?'
    case 'resposta': {
      const p = game.latestView?.pending
      return p ? `${nameOf(p.requester)} pediu ${stakeName(p.toStake)}. Sua dupla responde.` : 'Sua dupla responde ao pedido.'
    }
    case 'jogar':
      return 'Sua vez: jogue uma carta.'
    default:
      return ''
  }
})

// sem painel: o que dizer no lugar (pendente sem ação, espera da Mão de 11, ou a vez de outro)
const aguardo = computed(() => {
  if (!game.online) return 'Reconectando…'
  if (game.phase === 'eleven_decision' && !game.amPending) {
    const meu = game.myTeam
    const parceiroPendente = game.pendingSeats.some((s) => meu !== null && teamOf(s) === meu)
    return parceiroPendente ? 'Aguardando o seu parceiro decidir' : 'Aguardando a dupla com 11 decidir'
  }
  if (game.amPending) {
    if (game.phase === 'cut_choice' || game.phase === 'dealer_choice') return 'Sua vez de escolher a carta. Aguarde, as opções já vão aparecer.'
    return 'Sua vez. Aguarde a mesa liberar a ação.'
  }
  return props.ocioso
})

const raiseLabel = computed(() => (game.nextStakeValue ? `${stakeName(game.nextStakeValue)}!` : ''))

function raise() {
  if (bloqueado.value) return
  if (game.raiseIsSuicide) {
    confirming.value = true
    return
  }
  void game.requestRaise()
}

function confirmRaise() {
  confirming.value = false
  void game.requestRaise()
}

defineExpose({ raise })
</script>

<template>
  <div
    v-if="mode"
    data-painel
    :aria-busy="!game.online || undefined"
    class="flex w-full flex-col gap-3 rounded-painel bg-(--painel-bg) p-4 text-(--painel-fg)"
  >
    <p
      class="text-sm"
      :class="game.online ? 'text-(--painel-dica)' : 'font-bold text-(--painel-fg)'"
      data-contexto
      :role="game.online ? undefined : 'status'"
    >
      {{ contexto }}
    </p>
    <Corte v-if="mode === 'corte'" />
    <Queima v-else-if="mode === 'queima'" />
    <DecisaoOnze v-else-if="mode === 'onze'" />
    <RespostaAumento v-else-if="mode === 'resposta'" />
    <div
      v-else
      class="flex flex-col gap-3 max-lg:flex-row-reverse max-lg:items-center max-lg:justify-between"
    >
      <Botao
        v-if="game.canRaise && raiseLabel"
        data-acao="aumentar"
        data-aposta
        variant="aposta"
        aria-keyshortcuts="T"
        class="w-full max-lg:flex-1"
        :disabled="bloqueado"
        @click="raise"
      >
        {{ raiseLabel }} <TeclaDica tecla="T" />
      </Botao>
      <Toggle
        v-if="game.canCover"
        v-model="covered"
        label="Jogar coberta"
        aria-keyshortcuts="C"
        class="shrink-0"
        :disabled="bloqueado"
      >
        <TeclaDica tecla="C" />
      </Toggle>
    </div>
  </div>
  <p
    v-else-if="aguardo"
    data-vez-de
    :role="game.online ? undefined : 'status'"
    class="rounded-painel bg-(--painel-bg) px-4 py-3 text-base font-bold text-(--painel-fg)"
  >
    {{ aguardo }}
  </p>

  <Modal
    :open="confirming"
    title="Pedido proibido"
    @close="confirming = false"
  >
    <p class="mb-6">
      Pedir {{ game.nextStakeValue ? stakeName(game.nextStakeValue).toLowerCase() : 'aumento' }} na
      {{ handKindText(game.latestView?.handKind) }} faz sua dupla perder a partida. Confirmar?
    </p>
    <div class="flex justify-end gap-3">
      <Botao
        variant="secondary"
        @click="confirming = false"
      >
        Cancelar
      </Botao>
      <Botao
        data-acao="confirmar-aumento"
        variant="danger"
        :disabled="!game.online"
        @click="confirmRaise"
      >
        Pedir mesmo assim
      </Botao>
    </div>
  </Modal>
</template>
