<script setup lang="ts">
const props = defineProps<{ modelValue: boolean; label: string; disabled?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean] }>()
</script>

<template>
  <button
    type="button"
    role="switch"
    :aria-checked="props.modelValue"
    :disabled="props.disabled"
    class="inline-flex min-h-11 items-center gap-3 rounded-botao-2 px-1 disabled:cursor-not-allowed disabled:opacity-(--botao-desabilitado-opacidade)"
    @click="emit('update:modelValue', !props.modelValue)"
  >
    <span
      class="relative h-6 w-11 shrink-0 rounded-full border-2 transition-colors duration-(--dur-rapido) ease-padrao"
      :class="props.modelValue ? 'border-ink bg-ink' : 'border-line bg-transparent'"
    >
      <span
        class="absolute top-0.5 left-0.5 size-4 rounded-full transition-transform duration-(--dur-rapido) ease-padrao"
        :class="props.modelValue ? 'translate-x-5 bg-surface' : 'bg-line'"
      />
    </span>
    <span>{{ props.label }}</span>
  </button>
</template>
