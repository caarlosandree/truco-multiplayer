<script setup lang="ts">
import { useId } from 'vue'

const props = defineProps<{
  label: string
  modelValue: string
  type?: string
  error?: string
  autocomplete?: string
  required?: boolean
}>()
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
const id = useId()
const errorId = `${id}-erro`
</script>

<template>
  <div class="flex flex-col gap-1">
    <label
      :for="id"
      class="text-sm font-bold"
    >{{ props.label }}</label>
    <input
      :id="id"
      :type="props.type ?? 'text'"
      :value="props.modelValue"
      :autocomplete="props.autocomplete"
      :required="props.required"
      :aria-invalid="props.error ? 'true' : undefined"
      :aria-describedby="props.error ? errorId : undefined"
      class="h-(--campo-altura) rounded-campo border-2 bg-(--campo-bg) px-3 text-base text-(--campo-fg) placeholder:text-(--campo-placeholder)"
      :class="props.error ? 'border-(--campo-borda-erro)' : 'border-(--campo-borda)'"
      @input="emit('update:modelValue', ($event.target as HTMLInputElement).value)"
    >
    <p
      v-if="props.error"
      :id="errorId"
      class="text-sm text-danger"
    >
      {{ props.error }}
    </p>
  </div>
</template>
