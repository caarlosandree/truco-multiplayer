<script setup lang="ts">
/**
 * Selo de bot: ícone de robozinho + a palavra "bot" (nunca só cor). Herda a cor do texto em volta, então serve na
 * lousa (giz) e no papel (tinta). Quem lê com leitor de tela ouve "bot" logo após o nome.
 */
</script>

<template>
  <span
    data-selo-bot
    class="inline-flex shrink-0 items-center gap-1 rounded-placa px-1.5 py-px text-xs/4 font-bold inset-ring-1 inset-ring-current"
  >
    <svg
      viewBox="0 0 16 16"
      class="size-3.5 shrink-0"
      fill="none"
      stroke="currentColor"
      stroke-width="1.75"
      stroke-linecap="round"
      stroke-linejoin="round"
      aria-hidden="true"
    >
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
    </svg>
    <span>bot</span>
  </span>
</template>
