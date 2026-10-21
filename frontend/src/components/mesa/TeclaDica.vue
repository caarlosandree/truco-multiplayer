<script setup lang="ts">
/** Dica da tecla de atalho: discreta, some no celular (sem teclado) e fica fora da leitura do botão. */
defineProps<{ tecla: string }>()
</script>

<template>
  <kbd
    aria-hidden="true"
    class="rounded-sm border border-current/60 px-1 font-sans text-xs leading-4 font-normal opacity-85 max-md:hidden"
  >{{ tecla }}</kbd>
</template>
