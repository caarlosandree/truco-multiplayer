<script setup lang="ts">
import { computed } from 'vue'
import { useCena } from '@/game/cena'
import { useGameStore } from '@/stores/game'
import { handKindText } from '@/game/labels'
import Tentos from './Tentos.vue'

/** A lousa: tentos de giz, número, valor da mão e tipo de mão. Dupla sempre relativa (nós/eles). */
const game = useGameStore()
const cena = useCena()
const v = computed(() => game.view)
const me = computed(() => game.myTeam ?? 0)
const placar = computed(() => cena.score ?? v.value?.score ?? [0, 0])
const nos = computed(() => placar.value[me.value] ?? 0)
const eles = computed(() => placar.value[1 - me.value] ?? 0)
const valor = computed(() => cena.stake ?? v.value?.stake ?? 1)
const dealer = computed(() => v.value?.players.find((p) => p.seat === v.value?.dealer)?.name)
const tipoCurto = computed(() => (v.value?.handKind === 'eleven' ? 'Mão 11' : 'Ferro'))
const tipo = computed(() => (v.value?.handKind && v.value.handKind !== 'normal' ? handKindText(v.value.handKind) : null))
</script>

<template>
  <section
    v-if="v"
    data-placar
    aria-label="Placar"
    class="textura-lousa relative z-(--z-painel) flex flex-wrap items-center justify-center gap-x-4 gap-y-1 curto:gap-x-1.5 curto:flex-nowrap curto:px-2 border-b-2 border-felt-deep px-3 py-2 text-giz curto:py-1 md:gap-x-8 md:px-6"
  >
    <div
      class="flex items-center gap-2 curto:gap-1 md:gap-3"
      role="group"
      :aria-label="`Nós: ${nos} ${nos === 1 ? 'ponto' : 'pontos'}`"
    >
      <span
        class="text-sm font-bold text-nos"
        aria-hidden="true"
      >Nós</span>
      <Tentos
        :count="nos"
        tone="nos"
      />
      <span
        data-placar-nos
        class="min-w-[2ch] text-center font-display text-placar font-bold text-nos max-md:text-3xl curto:max-md:text-2xl"
        aria-hidden="true"
      >{{ nos }}</span>
    </div>
    <div
      class="flex items-center gap-2 curto:gap-1 md:gap-3"
      role="group"
      :aria-label="`Eles: ${eles} ${eles === 1 ? 'ponto' : 'pontos'}`"
    >
      <span
        class="text-sm font-bold text-eles"
        aria-hidden="true"
      >Eles</span>
      <Tentos
        :count="eles"
        tone="eles"
      />
      <span
        data-placar-eles
        class="min-w-[2ch] text-center font-display text-placar font-bold text-eles max-md:text-3xl curto:max-md:text-2xl"
        aria-hidden="true"
      >{{ eles }}</span>
    </div>
    <p class="text-sm text-giz-muted max-md:hidden">
      Mão {{ v.handNumber }}{{ dealer ? ` · dá: ${dealer}` : '' }}
    </p>
    <div class="flex items-center gap-2 curto:flex-col curto:gap-0.5">
      <Transition
        name="mesa-valor"
        mode="out-in"
      >
        <p
          :key="valor"
          data-valor
          class="rounded-valor border-2 whitespace-nowrap border-(--valor-borda) px-3 font-display text-valor curto:px-1.5 curto:text-base"
        >
          vale {{ valor }}<span class="sr-only"> {{ valor === 1 ? 'ponto' : 'pontos' }}</span>
        </p>
      </Transition>
      <!-- celular: chip curto ("Mão 11", "Ferro"); o texto completo continua para leitores de tela -->
      <span
        v-if="tipo"
        data-tipo
        class="text-sm font-bold curto:text-xs curto:leading-3"
      >
        <span class="max-md:sr-only">{{ tipo }}</span>
        <span
          aria-hidden="true"
          data-tipo-curto
          class="md:hidden"
        >{{ tipoCurto }}</span>
      </span>
    </div>
  </section>
</template>
