<template>
  <div class="space-y-2">
    <div class="flex h-2.5 w-full overflow-hidden rounded-full bg-muted">
      <div
        v-for="(seg, i) in normalizedSegments"
        :key="i"
        class="h-full transition-all"
        :style="{ width: `${seg.pct}%`, background: seg.color }"
      ></div>
    </div>
    <ul v-if="showLegend" class="flex flex-wrap gap-x-4 gap-y-1 text-xs">
      <li v-for="(seg, i) in normalizedSegments" :key="i" class="flex items-center gap-1.5">
        <span class="h-2 w-2 rounded-sm" :style="{ background: seg.color }"></span>
        <span class="text-muted-foreground">{{ seg.label }}</span>
        <span class="tabular-nums text-foreground">{{ seg.display ?? seg.value }}</span>
      </li>
    </ul>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    segments: Array<{ value: number; color: string; label?: string; display?: string }>
    showLegend?: boolean
  }>(),
  { showLegend: true },
)

const total = computed(() => props.segments.reduce((sum, s) => sum + (s.value > 0 ? s.value : 0), 0))

const normalizedSegments = computed(() =>
  props.segments.map((s) => ({
    ...s,
    pct: total.value > 0 ? (Math.max(s.value, 0) / total.value) * 100 : 0,
  })),
)
</script>
