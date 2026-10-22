<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { Seat } from '@/protocol/engine'
import { teamOf, type RelativePosition } from '@/game/seats'
import { voteText } from '@/game/labels'
import { useGameStore } from '@/stores/game'
import { useRoomStore } from '@/stores/room'
import { useSessionStore } from '@/stores/session'
import SeloBot from '@/components/ui/SeloBot.vue'
import Carta from './Carta.vue'
import Prazo from './Prazo.vue'
import BalaoSinal from './BalaoSinal.vue'
import { BALAO_LADO } from './balaoLado'

const props = defineProps<{ seat: Seat; position: RelativePosition }>()
const game = useGameStore()
const room = useRoomStore()
const session = useSessionStore()

const player = computed(() => game.view?.players.find((p) => p.seat === props.seat))
const presence = computed(() => room.room?.seats[props.seat] ?? null)
const isMe = computed(() => props.seat === game.view?.seat)
const ally = computed(() => game.myTeam !== null && teamOf(props.seat) === game.myTeam)
const deadline = computed(() => game.deadlines[props.seat])
// quem age agora: os assentos pendentes (jogada, voto, corte, queima…); sem pendentes, a vez do turno
const active = computed(() =>
  game.pendingSeats.length > 0 ? game.pendingSeats.includes(props.seat) : game.view?.turn === props.seat,
)
const isBot = computed(() => presence.value?.bot === true)
// humano ausente com bot jogando por ele: nome esmaecido + "bot jogando por Fulano"
const botPlaying = computed(() => presence.value?.botPlaying === true)
const away = computed(() => presence.value !== null && (!presence.value.connected || presence.value.absent || botPlaying.value))
const estado = computed(() => {
  if (!presence.value) return null
  if (botPlaying.value) return 'Bot'
  if (!presence.value.connected) return 'Desconectado'
  return presence.value.absent ? 'Ausente' : null
})
const nome = computed(() => player.value?.name ?? presence.value?.displayName ?? `Lugar ${props.seat + 1}`)

// o servidor não manda a duração total do prazo: o anel usa o que restava quando o cliente o viu pela primeira vez
const firstSeen = ref<{ deadline: number; at: number } | null>(null)
watch(
  deadline,
  (d) => {
    firstSeen.value = d === undefined ? null : { deadline: d, at: session.serverNow() }
  },
  { immediate: true },
)
const totalMs = computed(() => (firstSeen.value ? Math.max(1, firstSeen.value.deadline - firstSeen.value.at) : 1))

// mão de 11: a dupla vê as cartas do parceiro
// (decidindo ou respondendo, a mão do parceiro fica no painel, junto da decisão; aqui seguem os versos)
const partnerCards = computed(() =>
  props.position === 'top' && !game.mustDecideEleven && !game.mustRespondRaise ? (game.view?.partnerHand ?? []) : [],
)
const cardCount = computed(() => player.value?.cardCount ?? 0)

// cena: halo no corte/escolha e balões
const cena = computed(() => (game.scene.event?.seat === props.seat ? game.scene.event : null))
const halo = computed(() => cena.value?.kind === 'cut_offered' || cena.value?.kind === 'card_chosen')
const balao = computed(() => {
  const ev = cena.value
  if (!ev) return null
  switch (ev.kind) {
    case 'raise_requested':
      return ev.stake === 3 ? 'Truco, ladrão!' : null
    case 'vote_cast':
      return ev.choice ? voteText(ev.choice) : null
    case 'eleven_decided':
      return ev.choice === 'run' ? 'Corre!' : 'Joga!'
    default:
      return null
  }
})
</script>

<template>
  <div
    :data-posicao="props.position"
    :data-vez="active ? '' : undefined"
    class="relative flex min-w-0 flex-col items-center gap-1 text-giz md:gap-1.5"
    :class="props.position === 'bottom' ? 'max-md:flex-row max-md:flex-wrap max-md:justify-center max-md:gap-x-3' : 'max-md:flex-row max-md:flex-wrap max-md:items-center max-md:justify-center max-md:gap-x-1.5'"
  >
    <div class="relative max-w-full max-md:mb-1 md:mb-1.5">
      <!-- só a placa esmaece quando a pessoa cai; o texto de estado fica em opacidade total -->
      <div
        :data-placa-esmaecida="away ? '' : undefined"
        class="rounded-placa px-3 py-1.5 text-center inset-ring-1 inset-ring-line/70"
        :class="[
          active ? 'bg-giz text-lousa' : 'textura-lousa',
          away ? 'opacity-(--assento-ausente-opacidade)' : '',
          props.position === 'bottom' ? 'min-w-24' : 'min-w-24 max-md:min-w-0',
        ]"
      >
        <p class="flex items-center justify-center gap-1.5 text-sm font-bold max-md:flex-col max-md:gap-0.5">
          <span class="sr-only">{{ ally ? 'Nós: ' : 'Eles: ' }}</span>
          <!-- eu: a mão faz as vezes de placa, então o nome só vai para leitores de tela -->
          <template v-if="isMe">
            <span class="sr-only">{{ nome }} (você)</span><span aria-hidden="true">Você</span>
          </template>
          <span
            v-else
            class="truncate"
          >{{ nome }}</span>
          <SeloBot v-if="isBot" />
        </p>
      </div>
      <!-- vez: contorno tracejado, mais fino que o foco (que é sólido) -->
      <span
        v-if="active"
        aria-hidden="true"
        data-vez-contorno
        class="pointer-events-none absolute -inset-[5px] rounded-placa border-2 border-dashed border-giz"
      />
      <span
        v-if="halo"
        aria-hidden="true"
        class="mesa-halo pointer-events-none absolute -inset-[5px] rounded-placa bg-giz/20"
      />
      <Prazo
        v-if="deadline !== undefined"
        :deadline="deadline"
        :total-ms="totalMs"
        :mine="isMe"
      />
      <Transition name="mesa-balao">
        <p
          v-if="balao"
          :key="game.scene.key"
          aria-hidden="true"
          class="absolute left-1/2 z-(--z-balao) w-max max-w-44 -translate-x-1/2 rounded-toast bg-(--balao-bg) px-3 py-1.5 text-center text-sm font-bold text-(--balao-fg) inset-ring-1 inset-ring-line"
          :class="BALAO_LADO[props.position].balao"
        >
          {{ balao }}
          <span
            class="absolute left-1/2 size-2 -translate-x-1/2 rotate-45 bg-(--balao-bg) inset-ring-1 inset-ring-line"
            :class="BALAO_LADO[props.position].rabicho"
          />
        </p>
      </Transition>
      <BalaoSinal
        :seat="props.seat"
        :position="props.position"
        :deslocado="balao !== null"
      />
    </div>

    <!-- mini-verso dos adversários; a dupla vê as cartas do parceiro na mão de 11 -->
    <div
      v-if="!isMe && partnerCards.length > 0"
      class="flex gap-1"
      role="group"
      aria-label="Cartas do parceiro"
    >
      <Carta
        v-for="c in partnerCards"
        :key="c.slot"
        :card="c.card"
        :hidden="c.hidden"
        size="sm"
      />
    </div>
    <div
      v-else-if="!isMe && cardCount > 0"
      class="flex -space-x-4 max-md:-space-x-4"
      role="img"
      :aria-label="`${cardCount} ${cardCount === 1 ? 'carta' : 'cartas'}`"
    >
      <Carta
        v-for="i in cardCount"
        :key="i"
        size="xs"
        hidden
        decorative
      />
    </div>

    <!-- marca da dupla: contínuo = nós, pontilhado = eles; o rótulo diz a mesma coisa em texto -->
    <p
      class="flex items-center justify-center gap-2 text-sm text-giz-muted"
      :class="props.position === 'bottom' ? '' : 'max-md:basis-full'"
    >
      <span
        aria-hidden="true"
        class="block h-0 w-10 border-t-3 max-md:w-6"
        :class="[ally ? 'border-solid border-nos' : 'border-dotted border-eles', away ? 'opacity-(--assento-ausente-opacidade)' : '']"
      />
      <span aria-hidden="true">{{ ally ? 'Nós' : 'Eles' }}</span>
      <strong
        v-if="isMe && active"
        class="text-giz"
      >· Sua vez</strong>
    </p>

    <p
      v-if="estado"
      class="inline-flex max-w-44 items-start gap-1.5 text-sm text-giz-muted"
    >
      <svg
        viewBox="0 0 16 16"
        class="mt-0.5 size-4 shrink-0"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <template v-if="estado === 'Bot'">
          <path d="M8 2.5V4.5M3.5 6h9a1 1 0 0 1 1 1v5a1 1 0 0 1-1 1h-9a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1zM1 8.5v2M15 8.5v2" />
          <circle
            cx="6"
            cy="9"
            r=".6"
            fill="currentColor"
          />
          <circle
            cx="10"
            cy="9"
            r=".6"
            fill="currentColor"
          />
        </template>
        <path
          v-else-if="estado === 'Desconectado'"
          d="M3 3l10 10M6 6.5a5 5 0 0 1 6 .5M2.5 8.5a8 8 0 0 1 3-1.8M6 11a3 3 0 0 1 3.5-.4"
        />
        <path
          v-else
          d="M8 3.5V8l3 2M8 14a6 6 0 1 0 0-12 6 6 0 0 0 0 12z"
        />
      </svg>
      <template v-if="estado === 'Bot'">
        bot jogando por {{ nome }}
      </template>
      <template v-else>
        {{ estado }}
      </template>
    </p>
  </div>
</template>
