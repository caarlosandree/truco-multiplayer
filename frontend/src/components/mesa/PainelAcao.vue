<script setup lang="ts">
import { computed, inject, nextTick, ref, watch, type Ref } from 'vue'
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
const painel = ref<HTMLElement | null>(null)
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

// região viva única e persistente (sem trocar role), só do estado do painel: como agir nas decisões que o painel
// apresenta (corte, queima, Mão de 11) e "Reconectando…". "Sua vez", pedidos de aumento e fim de rodada são do
// Anunciador; repetir aqui faria os dois competirem pela leitura.
const anuncio = computed(() => {
  if (!game.online) return 'Reconectando…'
  return mode.value === 'corte' || mode.value === 'queima' || mode.value === 'onze' ? contexto.value : ''
})

function digitando(el: Element | null): boolean {
  return el instanceof HTMLElement && (el.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(el.tagName))
}

// o painel trocou: se o foco estava nele (ou caiu para o body), vai para a ação principal da nova fase.
// Nunca rouba o foco de um modal aberto, de um campo de texto ou de outra região da página.
let focoNoPainel = false
watch(mode, () => {
  const ativo = document.activeElement
  focoNoPainel = ativo === null || ativo === document.body || (painel.value?.contains(ativo) ?? false)
}, { flush: 'pre' })
watch(mode, async (novo) => {
  if (!novo || !focoNoPainel || document.querySelector('[aria-modal="true"]') || digitando(document.activeElement)) return
  await nextTick()
  const alvo =
    painel.value?.querySelector<HTMLElement>('[data-primaria]:not(:disabled)') ??
    painel.value?.querySelector<HTMLElement>('[data-corte] button:not(:disabled), [data-queima] [role="checkbox"]:not(:disabled)')
  alvo?.focus()
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
    ref="painel"
    data-painel
    class="flex w-full flex-col gap-3 rounded-painel bg-(--painel-bg) p-4 text-(--painel-fg) curto:gap-2 curto:p-3"
  >
    <p
      class="text-sm"
      :class="game.online ? 'text-(--painel-dica)' : 'font-bold text-(--painel-fg)'"
      data-contexto
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
    class="rounded-painel bg-(--painel-bg) px-4 py-3 text-base font-bold text-(--painel-fg)"
  >
    {{ aguardo }}
  </p>

  <p
    class="sr-only"
    role="status"
    aria-live="polite"
    data-anuncio
  >
    {{ anuncio }}
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
