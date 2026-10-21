<script setup lang="ts">
import type { Seat } from '@/protocol/engine'
import type { SeatInfo } from '@/protocol/messages'

const props = defineProps<{
  seat: Seat
  info: SeatInfo | null
  isMe: boolean
  isHost: boolean
  canKick: boolean
  /** mesma dupla de quem está sentado vendo a tela: "nós" (contínuo) ou "eles" (pontilhado) */
  ally: boolean
}>()
const emit = defineEmits<{ take: []; kick: [] }>()
</script>

<template>
  <div
    :data-assento="props.seat"
    class="flex w-full max-w-52 flex-col items-stretch gap-1.5 text-giz"
  >
    <template v-if="props.info">
      <!-- só a placa esmaece quando a pessoa cai; o texto de estado fica em opacidade total -->
      <div class="rounded-placa bg-felt">
        <div
          class="textura-lousa flex min-h-14 flex-col justify-center rounded-placa px-3 py-2 text-center"
          :class="[
            props.isMe ? 'inset-ring-2 inset-ring-giz/70' : 'inset-ring-1 inset-ring-line/70',
            props.info.connected ? '' : 'opacity-(--assento-ausente-opacidade)',
          ]"
        >
          <p class="flex items-center justify-center gap-1.5 text-sm font-bold">
            <svg
              v-if="props.isHost"
              viewBox="0 0 16 16"
              class="size-4 shrink-0"
              fill="currentColor"
              aria-hidden="true"
            ><path d="M2 12.5V5l3.2 3L8 3.5 10.8 8 14 5v7.5z" /></svg>
            <span class="sr-only">Lugar {{ props.seat + 1 }}, {{ props.ally ? 'nós' : 'eles' }}. </span>
            <span
              v-if="props.isHost"
              class="sr-only"
            >Anfitrião. </span>
            <span class="truncate">{{ props.info.displayName }}</span>
            <span
              v-if="props.isMe"
              class="shrink-0 font-normal text-giz-muted"
            >(você)</span>
          </p>
        </div>
      </div>
      <p class="flex min-h-5 flex-wrap items-center justify-center gap-x-3 gap-y-0.5 text-sm text-giz-muted">
        <span class="inline-flex items-center gap-1.5">
          <svg
            v-if="props.info.ready"
            viewBox="0 0 16 16"
            class="size-4 shrink-0"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
            aria-hidden="true"
          ><path d="M3 8.5l3.2 3L13 4.5" /></svg>
          <span :class="props.info.ready ? 'font-bold text-giz' : ''">{{ props.info.ready ? 'Pronto' : 'Aguardando' }}</span>
        </span>
        <span
          v-if="!props.info.connected"
          class="inline-flex items-center gap-1.5"
        >
          <svg
            viewBox="0 0 16 16"
            class="size-4 shrink-0"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            aria-hidden="true"
          ><path d="M3 3l10 10M6 6.5a5 5 0 0 1 6 .5M2.5 8.5a8 8 0 0 1 3-1.8M6 11a3 3 0 0 1 3.5-.4" /></svg>
          Desconectado
        </span>
      </p>
    </template>
    <button
      v-else
      type="button"
      data-acao="sentar"
      class="flex min-h-14 w-full items-center justify-center rounded-placa border-2 border-dashed bg-felt border-giz/60 px-3 py-2 text-sm font-bold text-giz hover:bg-giz/10 active:translate-y-(--botao-pressao-y)"
      :aria-label="`Sentar no lugar ${props.seat + 1}, ${props.ally ? 'nós' : 'eles'}`"
      @click="emit('take')"
    >
      Sentar aqui
    </button>

    <!-- marca da dupla: contínuo = nós, pontilhado = eles; o rótulo diz a mesma coisa em texto -->
    <p class="flex items-center justify-center gap-2 text-sm text-giz-muted">
      <span
        aria-hidden="true"
        class="block h-0 w-10 border-t-3"
        :class="[props.ally ? 'border-solid border-nos' : 'border-dotted border-eles', props.info ? '' : 'opacity-(--assento-ausente-opacidade)']"
      />
      <span aria-hidden="true">{{ props.ally ? 'Nós' : 'Eles' }}</span>
    </p>

    <button
      v-if="props.canKick && props.info"
      type="button"
      data-acao="remover"
      class="mx-auto inline-flex h-11 min-w-11 items-center justify-center rounded-botao-2 px-3 text-sm font-bold text-giz hover:bg-giz/10 active:translate-y-(--botao-pressao-y)"
      :aria-label="`Remover ${props.info.displayName}`"
      @click="emit('kick')"
    >
      Remover
    </button>
  </div>
</template>
