<template>
  <header class="mb-5 flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
    <div class="min-w-0">
      <p v-if="eyebrow" class="text-xs font-semibold text-primary">{{ eyebrow }}</p>
      <h1 class="text-2xl font-bold text-foreground lg:text-[1.75rem]">{{ title }}</h1>
      <p v-if="description" class="mt-1.5 max-w-3xl text-sm text-muted-foreground">{{ description }}</p>
    </div>
    <div v-if="$slots.actions" class="flex shrink-0 flex-wrap items-center gap-2">
      <slot name="actions" />
    </div>
  </header>
</template>

<script setup lang="ts">
defineProps<{ eyebrow?: string; title: string; description?: string }>()
</script>
