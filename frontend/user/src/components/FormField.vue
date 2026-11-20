<template>
  <div class="form-field">
    <label
      v-if="label || $slots.icon"
      :for="fieldId"
      class="mb-2 flex items-center gap-1.5 text-xs font-semibold [.vault-tokens_&]:uppercase [.vault-tokens_&]:tracking-[0.18em] text-muted-foreground"
    >
      <span v-if="$slots.icon" class="opacity-60 shrink-0"><slot name="icon" /></span>
      <span>{{ label }}</span>
      <span v-if="required" class="text-destructive/80" aria-hidden="true">*</span>
    </label>

    <div class="form-field-control">
      <slot
        :id="fieldId"
        :hasError="!!error"
        :describedBy="describedBy"
      />
    </div>

    <p
      v-if="error"
      :id="errorId"
      class="mt-1.5 text-xs text-destructive"
      role="alert"
    >{{ error }}</p>
    <p
      v-else-if="hint"
      :id="hintId"
      class="mt-1.5 text-xs text-muted-foreground"
    >{{ hint }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed, useId } from 'vue'

const props = defineProps<{
  label?: string
  error?: string
  hint?: string
  required?: boolean
  fieldId?: string
}>()

const autoId = useId()
const fieldId = computed(() => props.fieldId || `field-${autoId}`)
const errorId = computed(() => `${fieldId.value}-error`)
const hintId = computed(() => `${fieldId.value}-hint`)
const describedBy = computed(() => {
  if (props.error) return errorId.value
  if (props.hint) return hintId.value
  return undefined
})
</script>
