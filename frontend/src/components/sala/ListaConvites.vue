<script setup lang="ts">
import { inviteStateText } from '@/game/labels'
import { useInvitesStore } from '@/stores/invites'

const invites = useInvitesStore()
</script>

<template>
  <section
    v-if="invites.roomInvites.length > 0"
    aria-labelledby="convites-titulo"
  >
    <h2
      id="convites-titulo"
      class="mb-2 text-xl font-bold"
    >
      Convites
    </h2>
    <ul class="rounded-painel bg-surface-2">
      <li
        v-for="i in invites.roomInvites"
        :key="i.inviteId"
        :data-convite="i.username"
        class="flex items-baseline justify-between gap-3 border-t-2 border-line/40 px-4 py-3 first:border-t-0"
      >
        <p class="min-w-0 font-bold [overflow-wrap:anywhere]">
          {{ i.displayName }}
          <span
            v-if="i.seat !== undefined"
            class="font-normal text-ink-muted"
          >· lugar {{ i.seat + 1 }}</span>
        </p>
        <p
          class="inline-flex shrink-0 items-center gap-1.5 text-sm"
          :class="i.state === 'accepted' ? 'font-bold text-ink' : 'text-ink-muted'"
        >
          <svg
            v-if="i.state === 'accepted'"
            viewBox="0 0 16 16"
            class="size-4"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
            aria-hidden="true"
          ><path d="M3 8.5l3.2 3L13 4.5" /></svg>
          {{ inviteStateText(i) }}
        </p>
      </li>
    </ul>
  </section>
</template>
