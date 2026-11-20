<template>
  <Card class="group relative overflow-hidden transition-shadow hover:shadow-md">
    <div class="flex items-start justify-between gap-3 p-4 sm:p-5">
      <div class="min-w-0">
        <p class="truncate text-xs font-semibold text-muted-foreground">{{ label }}</p>
        <p class="mt-2 tabular-nums text-xl font-bold text-foreground sm:text-2xl">{{ value }}</p>
        <p v-if="hint" class="mt-1 text-xs text-muted-foreground">{{ hint }}</p>
      </div>
      <span
        v-if="icon"
        class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl transition-transform group-hover:scale-105"
        :class="toneClass"
      >
        <component :is="icon" class="h-5 w-5" />
      </span>
    </div>
  </Card>
</template>

<script setup lang="ts">
import { computed, type Component } from 'vue'
import { Card } from '@/components/ui/card'

const props = withDefaults(
  defineProps<{
    label: string
    value: string | number
    hint?: string
    icon?: Component
    tone?: 'accent' | 'success' | 'warning' | 'info' | 'neutral'
  }>(),
  { tone: 'accent' },
)

const toneClass = computed(
  () =>
    ({
      success: 'bg-success/10 text-success',
      warning: 'bg-warning/10 text-warning',
      info: 'bg-secondary text-foreground',
      neutral: 'bg-muted text-muted-foreground',
      accent: 'bg-secondary text-foreground',
    })[props.tone],
)
</script>
