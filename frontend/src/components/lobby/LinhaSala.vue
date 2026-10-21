<script setup lang="ts">
import { computed } from 'vue'
import type { RoomSummary } from '@/protocol/messages'
import type { Seat } from '@/protocol/engine'
import { teamOf } from '@/game/seats'

const props = defineProps<{ room: RoomSummary }>()
const emit = defineEmits<{ join: [seat?: Seat] }>()

const open = computed(() => props.room.status === 'waiting' && props.room.seats.some((s) => s === null))
const playing = computed(() => props.room.status === 'playing')
const free = computed(() => props.room.seats.filter((s) => s === null).length)
const status = computed(() => {
  if (open.value) return free.value === 1 ? 'Falta 1 jogador' : `Faltam ${free.value} jogadores`
  return playing.value ? 'Em jogo' : 'Cheia'
})
</script>

<template>
  <li
    :data-sala="room.id"
    class="grid grid-cols-1 items-center gap-x-4 gap-y-2 border-t-2 border-line/40 px-4 py-4 first:border-t-0 md:grid-cols-[minmax(0,14rem)_minmax(0,1fr)_7.5rem] md:gap-x-6 md:px-6"
    :class="open ? '' : 'text-ink-muted'"
  >
    <div class="min-w-0">
      <p
        class="truncate text-base font-bold"
        :class="open ? 'text-ink' : ''"
      >
        {{ room.name }}
      </p>
      <p
        v-if="open"
        class="text-sm font-bold"
      >
        {{ status }}
      </p>
      <p
        v-else
        class="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-sm font-bold tabular-nums"
      >
        <span class="sr-only">Indisponível: </span>
        <span class="rounded-placa px-2 py-0.5 inset-ring-2 inset-ring-line">{{ status }}</span>
        <span
          v-if="playing && room.score"
          class="whitespace-nowrap text-base"
        >
          <span class="text-team-0">{{ room.score[0] }}</span> × <span class="text-team-1">{{ room.score[1] }}</span>
        </span>
      </p>
    </div>

    <ul
      class="grid grid-cols-4 gap-x-1 md:gap-x-4"
      aria-label="Lugares"
    >
      <li
        v-for="(seat, i) in room.seats"
        :key="i"
        class="min-w-0"
      >
        <button
          v-if="seat === null && open"
          type="button"
          :data-assento="i"
          class="flex h-11 w-full items-center justify-center rounded-botao-2 border-2 border-dashed border-line/70 text-sm font-bold text-ink hover:bg-(--botao-fantasma-bg-hover) active:translate-y-(--botao-pressao-y)"
          :aria-label="`Sentar no lugar ${i + 1} de ${room.name}`"
          @click="emit('join', i as Seat)"
        >
          Sentar
        </button>
        <span
          v-else
          :data-assento="i"
          class="flex h-11 items-center justify-center truncate px-1 text-sm"
          :class="seat ? 'font-bold' : 'italic'"
        >
          <span
            v-if="seat"
            class="sr-only"
          >Lugar {{ i + 1 }}, dupla {{ teamOf(i as Seat) + 1 }}: </span>
          <span class="truncate">{{ seat ? seat.displayName : 'Livre' }}</span>
        </span>
        <!-- traço de giz da dupla: contínuo (dupla 0) ou pontilhado (dupla 1); o rótulo é o lugar na lista -->
        <span
          aria-hidden="true"
          class="mx-auto mt-1.5 block h-0 w-10 border-t-3"
          :class="[teamOf(i as Seat) === 0 ? 'border-solid border-team-0' : 'border-dotted border-team-1', seat ? '' : 'opacity-40']"
        />
      </li>
    </ul>

    <button
      v-if="open"
      type="button"
      class="inline-flex h-11 items-center justify-center gap-1 justify-self-stretch rounded-botao-2 px-4 font-bold text-ink inset-ring-2 inset-ring-(--botao-sec-borda) hover:bg-(--botao-sec-bg-hover) active:translate-y-(--botao-pressao-y) md:justify-self-end"
      :aria-label="`Entrar em ${room.name}`"
      @click="emit('join')"
    >
      Entrar
      <svg
        viewBox="0 0 16 16"
        class="size-4"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      ><path d="M6 3l5 5-5 5" /></svg>
    </button>
  </li>
</template>
