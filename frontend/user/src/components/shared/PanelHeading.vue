<template>
  <div class="mb-6 flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
    <div class="flex min-w-0 items-center gap-3.5">
      <span
        v-if="icon"
        class="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl bg-primary/10 text-primary"
      >
        <component :is="icon" class="h-5 w-5" />
      </span>
      <div class="min-w-0">
        <p v-if="eyebrow" class="text-[11px] font-semibold text-primary">{{ eyebrow }}</p>
        <h2 class="truncate text-xl font-bold text-foreground sm:text-[1.35rem]">{{ title }}</h2>
        <p v-if="description" class="mt-1 text-sm text-muted-foreground">{{ description }}</p>
      </div>
    </div>
    <div v-if="$slots.actions" class="flex shrink-0 flex-wrap items-center gap-2">
      <slot name="actions" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { type Component } from 'vue'

defineProps<{ eyebrow?: string; title: string; description?: string; icon?: Component }>()
</script>
