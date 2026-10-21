<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, provide, ref, watch } from 'vue'
import type { Seat } from '@/protocol/engine'
import { anuncioDoEvento, provideCena } from '@/game/cena'
import { relativePosition, seatsInDisplayOrder, type RelativePosition } from '@/game/seats'
import { sfx } from '@/audio/install'
import { useGameStore } from '@/stores/game'
import { useRoomStore } from '@/stores/room'
import AssentoMesa from './AssentoMesa.vue'
import CenaEvento from './CenaEvento.vue'
import Centro from './Centro.vue'
import Mao from './Mao.vue'
import Placar from './Placar.vue'

/**
 * A mesa. Encaixes para as próximas tarefas:
 *  - slot `painel` (T18): painel de ação; o padrão mostra de quem é a vez;
 *  - slot `sinais` (T19): barra de sinais; slot `aviso` (T19): banner de W.O./fim, no topo da coluna da direita;
 *  - `inject('coberta')` (Ref<boolean>): jogar a próxima carta coberta.
 */
const game = useGameStore()
const room = useRoomStore()
provideCena()
const covered = ref(false)
provide('coberta', covered)

const others = computed(() => {
  const me = game.view?.seat
  if (me === undefined) return []
  return seatsInDisplayOrder(me)
    .slice(1)
    .map((seat) => ({ seat, position: relativePosition(seat, me) }))
})

// celular: os três adversários numa faixa em cima; desktop: parceiro em cima, laterais ao lado do centro
const SLOT: Record<Exclude<RelativePosition, 'bottom'>, string> = {
  left: 'col-start-1 row-start-1 md:row-start-2 md:self-center md:justify-self-start',
  top: 'col-start-2 row-start-1',
  right: 'col-start-3 row-start-1 md:row-start-2 md:self-center md:justify-self-end',
}

const nameOf = (seat: Seat) => game.view?.players.find((p) => p.seat === seat)?.name ?? `Lugar ${seat + 1}`

// a carta clicada some da mão na hora (a jogada voa para a vaza); volta se o servidor não confirmar
const clicada = ref<number | null>(null)
let soltar: ReturnType<typeof setTimeout> | undefined
const mao = computed(() => (game.view?.myHand ?? []).filter((s) => s.slot !== clicada.value))
watch(
  () => game.view,
  () => {
    clicada.value = null
  },
)
async function play(slot: number) {
  const antes = game.latest
  clicada.value = slot
  const coberta = covered.value
  covered.value = false
  await game.playCard(slot, coberta)
  clearTimeout(soltar)
  soltar = setTimeout(() => {
    if (game.latest === antes) clicada.value = null
  }, 1500)
}

// região viva (visualmente oculta): vez e eventos da cena
const fala = ref('')
watch(
  () => game.scene.key,
  () => {
    const e = game.scene.event
    const texto = e ? anuncioDoEvento(e, nameOf, game.myTeam) : null
    if (texto) fala.value = texto
  },
)
watch(
  () => game.isMyTurn && game.canPlay,
  (vez) => {
    if (!vez) return
    fala.value = 'Sua vez.'
    // "sua vez": som só com a aba em foco (spec §8.2); o título da aba é tratado na T19
    if (typeof document !== 'undefined' && document.hasFocus()) sfx()?.play('yourTurn')
  },
)
const vezDe = computed(() => {
  const v = game.view
  if (!v) return ''
  const quem = v.turn === undefined ? null : nameOf(v.turn)
  if (game.canPlay) return 'Sua vez. Clique numa carta.'
  return quem ? `Vez de ${quem}` : 'Aguardando…'
})

// celular: toasts sobem acima da mão e do painel (a região lê --toast-offset)
const base = ref<HTMLElement | null>(null)
const painel = ref<HTMLElement | null>(null)
let observer: ResizeObserver | undefined
function medir() {
  const celular = typeof matchMedia === 'function' && matchMedia('(max-width: 767px)').matches
  const h = (base.value?.offsetHeight ?? 0) + (painel.value?.offsetHeight ?? 0)
  const raiz = document.documentElement.style
  if (celular && h > 0) raiz.setProperty('--toast-offset', `${h}px`)
  else raiz.removeProperty('--toast-offset')
}
onMounted(() => {
  medir()
  if (typeof ResizeObserver === 'function') {
    observer = new ResizeObserver(medir)
    if (base.value) observer.observe(base.value)
    if (painel.value) observer.observe(painel.value)
  }
  window.addEventListener('resize', medir)
})
onBeforeUnmount(() => {
  observer?.disconnect()
  window.removeEventListener('resize', medir)
  clearTimeout(soltar)
  document.documentElement.style.removeProperty('--toast-offset')
})
</script>

<template>
  <div
    v-if="game.view"
    data-tema="escuro"
    data-mesa
    class="textura-feltro flex min-h-[calc(100dvh-3.5rem)] flex-col text-giz lg:grid lg:grid-cols-[minmax(0,1fr)_20rem]"
  >
    <h1 class="sr-only">
      Partida · {{ room.room?.name }}
    </h1>
    <section
      aria-label="Mesa"
      class="relative flex min-w-0 flex-1 flex-col overflow-hidden"
    >
      <Placar />
      <div class="relative grid flex-1 grid-cols-3 grid-rows-[auto_1fr] content-start gap-x-2 gap-y-3 px-2 pt-4 md:grid-cols-[1fr_auto_1fr] md:gap-x-4 md:px-6 md:pt-6">
        <!-- o oval da mesa, igual ao da sala de espera: só decoração, atrás dos lugares -->
        <div
          aria-hidden="true"
          class="pointer-events-none absolute inset-x-12 inset-y-3 rounded-[50%] border-4 border-felt-deep bg-felt/60 max-md:hidden"
        />
        <AssentoMesa
          v-for="o in others"
          :key="o.seat"
          :seat="o.seat"
          :position="o.position"
          :class="SLOT[o.position as Exclude<RelativePosition, 'bottom'>]"
          class="max-md:justify-self-center"
        />
        <div class="relative col-span-3 row-start-2 place-self-center py-2 md:col-span-1 md:col-start-2">
          <Centro />
        </div>
      </div>
      <div
        ref="base"
        class="flex flex-col items-center gap-3 px-2 pt-2 pb-3 md:pb-6"
      >
        <AssentoMesa
          :seat="game.view.seat"
          position="bottom"
        />
        <Mao
          :slots="mao"
          :interactive="game.canPlay && !game.inflight"
          @play="play"
        />
      </div>
      <CenaEvento />
    </section>

    <aside
      ref="painel"
      aria-label="Ações"
      class="textura-lousa flex flex-col gap-3 border-t-2 border-felt-deep p-3 pb-[max(0.75rem,env(safe-area-inset-bottom))] lg:border-t-0 lg:border-l-2 lg:p-4"
    >
      <slot name="aviso" />
      <slot name="painel">
        <p
          class="rounded-painel bg-(--painel-bg) px-4 py-3 text-sm text-(--painel-dica)"
          data-vez-de
        >
          {{ vezDe }}
        </p>
      </slot>
      <slot name="sinais" />
    </aside>

    <p
      class="sr-only"
      role="status"
      aria-live="polite"
      data-fala
    >
      {{ fala }}
    </p>
  </div>
  <p
    v-else
    class="p-8 text-ink-muted"
    role="status"
  >
    Carregando mesa…
  </p>
</template>

<style>
/* ---- cena da mesa: só transform, opacity, stroke-dashoffset e clip-path animam (brief §7) ---- */

/* giz que se desenha: o traço aparece da esquerda; o risco do tento corre pelo caminho */
@keyframes mesa-riscar {
  from {
    stroke-dashoffset: 1;
  }
  to {
    stroke-dashoffset: 0;
  }
}
@keyframes mesa-escrever {
  from {
    clip-path: inset(-20% 100% -20% 0);
  }
  to {
    clip-path: inset(-20% -10% -20% 0);
  }
}
@keyframes mesa-distribui {
  from {
    transform: translate(0, 0);
    opacity: 0;
  }
  12% {
    opacity: 1;
  }
  to {
    transform: translate(var(--tx), var(--ty));
    opacity: 1;
  }
}
@keyframes mesa-queima {
  from {
    transform: translate(var(--tx), var(--ty));
    opacity: 1;
  }
  to {
    transform: translate(0, 0);
    opacity: 0;
  }
}
@keyframes mesa-gira-sai {
  from {
    transform: scaleX(1);
  }
  to {
    transform: scaleX(0);
  }
}
@keyframes mesa-gira-entra {
  from {
    transform: scaleX(0);
  }
  to {
    transform: scaleX(1);
  }
}
@keyframes mesa-halo {
  from {
    opacity: 0;
  }
  to {
    opacity: 1;
  }
}
@keyframes mesa-balao {
  from {
    opacity: 0;
    transform: translateX(-50%) scale(0.92);
  }
  to {
    opacity: 1;
    transform: translateX(-50%) scale(1);
  }
}

.mesa-riscar {
  animation: mesa-riscar var(--dur-truco) var(--ease-giz) both;
}
.mesa-riscar--tento {
  animation-timing-function: var(--ease-giz);
}
.mesa-riscar--sublinha {
  animation-duration: 200ms;
  animation-delay: 150ms;
}
.mesa-escrever {
  animation: mesa-escrever 300ms var(--ease-giz) 150ms both;
}
.mesa-halo {
  animation: mesa-halo 300ms var(--ease-padrao) both;
}

/* últimos 5 s de quem age: o contorno da vez também vira alerta */
[data-vez]:has([data-prazo][data-alerta]) [data-vez-contorno] {
  border-color: var(--prazo-alerta);
}

/* destino das cartas distribuídas/queimadas, em unidades do contêiner da cena */
.mesa-dest-bottom {
  --tx: 0cqw;
  --ty: 38cqh;
}
.mesa-dest-top {
  --tx: 0cqw;
  --ty: -30cqh;
}
.mesa-dest-left {
  --tx: -40cqw;
  --ty: 0cqh;
}
.mesa-dest-right {
  --tx: 40cqw;
  --ty: 0cqh;
}
@media (max-width: 767px) {
  .mesa-dest-top {
    --ty: -28cqh;
  }
  .mesa-dest-left {
    --tx: -32cqw;
    --ty: -28cqh;
  }
  .mesa-dest-right {
    --tx: 32cqw;
    --ty: -28cqh;
  }
}
.mesa-voo {
  z-index: var(--z-carta-voo);
}
.mesa-voo--distribui {
  animation: mesa-distribui var(--dur-carta, 300ms) var(--ease-desliza) both;
  animation-duration: 280ms;
}
.mesa-voo--queima {
  animation: mesa-queima 300ms var(--ease-desliza) both;
}

/* vira: gira em scaleX (sem rotação) — o verso encolhe e some, a face entra */
.mesa-vira__verso {
  animation:
    mesa-gira-sai 150ms var(--ease-padrao) both,
    mesa-some 1ms linear 150ms forwards;
}
.mesa-vira__face {
  animation: mesa-gira-entra 150ms var(--ease-padrao) 150ms both;
}
@keyframes mesa-some {
  to {
    visibility: hidden;
  }
}

/* vaza: o contêiner empilha as cartas numa célula e desloca cada uma para o lado de quem jogou */
.mesa-vaza {
  --dx: 3.6rem;
  --dy: 3.1rem;
  --voo: 8rem;
  position: relative;
  display: grid;
  place-items: center;
  width: 12.5rem;
  height: 12.75rem;
}
@media (max-width: 767px) {
  .mesa-vaza {
    --dx: 2.7rem;
    --dy: 2.3rem;
    --voo: 6rem;
    width: 9.5rem;
    height: 9.5rem;
  }
}
/* a vaza assenta num círculo de giz apagado: o lugar das cartas fica evidente mesmo vazio */
.mesa-vaza::before {
  content: '';
  position: absolute;
  inset: 0.25rem 0;
  border-radius: 50%;
  border: 2px solid color-mix(in oklab, var(--s-giz) 16%, transparent);
}
.mesa-vaza__slot {
  position: relative;
  grid-area: 1 / 1;
  translate: calc(var(--ox) * var(--dx)) calc(var(--oy) * var(--dy));
}
.mesa-vaza-enter-active {
  transition:
    transform var(--dur-carta) var(--ease-desliza),
    opacity var(--dur-rapido) var(--ease-desliza);
}
.mesa-vaza-enter-from {
  opacity: 0;
  transform: translate(calc(var(--ox) * var(--voo)), calc(var(--oy) * var(--voo)));
}
.mesa-vaza-leave-active {
  transition:
    transform var(--dur-carta) var(--ease-sai),
    opacity var(--dur-carta) var(--ease-sai);
}
.mesa-vaza-leave-to {
  opacity: 0;
  transform: translate(calc(var(--wx) * var(--voo)), calc(var(--wy) * var(--voo)));
}

/* mão */
.mesa-mao-enter-active {
  transition: opacity var(--dur-rapido) var(--ease-desliza);
}
.mesa-mao-enter-from {
  opacity: 0;
}
.mesa-mao-leave-active {
  position: absolute;
  transition: opacity var(--dur-rapido) var(--ease-sai);
}
.mesa-mao-leave-to {
  opacity: 0;
}
.mesa-mao-move {
  transition: transform var(--dur-carta) var(--ease-desliza);
}

/* anúncios, balões, valor, escurecimento */
.mesa-anuncio-enter-active {
  transition: opacity 60ms linear;
}
.mesa-anuncio-leave-active {
  transition: opacity var(--dur-rapido) var(--ease-sai);
}
.mesa-anuncio-enter-from,
.mesa-anuncio-leave-to {
  opacity: 0;
}
.mesa-escurece-enter-active {
  transition: opacity var(--dur-rapido) var(--ease-padrao);
}
.mesa-escurece-leave-active {
  transition: opacity var(--dur-rapido) var(--ease-sai);
}
.mesa-escurece-enter-from,
.mesa-escurece-leave-to {
  opacity: 0;
}
.mesa-balao-enter-active {
  animation: mesa-balao var(--dur-rapido) var(--ease-quique) both;
}
.mesa-balao-leave-active {
  transition: opacity 200ms var(--ease-sai);
}
.mesa-balao-leave-to {
  opacity: 0;
}
.mesa-valor-leave-active {
  transition: opacity var(--dur-rapido) var(--ease-sai);
}
.mesa-valor-leave-to {
  opacity: 0;
}
.mesa-valor-enter-active {
  animation: mesa-escrever var(--dur-medio) var(--ease-giz) both;
}

@media (prefers-reduced-motion: reduce) {
  [data-mesa] *,
  [data-mesa] *::before {
    animation-delay: 0ms !important;
  }
}
</style>
