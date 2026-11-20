<template>
  <div class="flex items-center gap-4">
    <div class="relative shrink-0" :style="{ width: `${size}px`, height: `${size}px` }">
      <div class="absolute inset-0 rounded-full" :style="ringStyle"></div>
      <div class="absolute inset-0 flex flex-col items-center justify-center text-center">
        <span class="tabular-nums text-base font-bold text-foreground">{{ centerValue }}</span>
        <span v-if="centerLabel" class="text-[10px] text-muted-foreground">{{ centerLabel }}</span>
      </div>
    </div>
    <ul v-if="showLegend" class="min-w-0 flex-1 space-y-1.5">
      <li v-for="(seg, i) in normalizedSegments" :key="i" class="flex items-center justify-between gap-2 text-xs">
        <span class="flex min-w-0 items-center gap-2">
          <span class="h-2.5 w-2.5 shrink-0 rounded-sm" :style="{ background: seg.color }"></span>
          <span class="truncate text-muted-foreground">{{ seg.label }}</span>
        </span>
        <span class="shrink-0 tabular-nums text-foreground">{{ seg.percentText }}</span>
      </li>
    </ul>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    segments: Array<{ value: number; color: string; label?: string }>
    size?: number
    thickness?: number
    centerValue?: string | number
    centerLabel?: string
    showLegend?: boolean
  }>(),
  { size: 120, thickness: 16, showLegend: true },
)

const total = computed(() => props.segments.reduce((sum, s) => sum + (s.value > 0 ? s.value : 0), 0))

const normalizedSegments = computed(() =>
  props.segments.map((s) => {
    const pct = total.value > 0 ? (Math.max(s.value, 0) / total.value) * 100 : 0
    return { ...s, pct, percentText: `${pct.toFixed(0)}%` }
  }),
)

const gradient = computed(() => {
  if (total.value <= 0) return 'conic-gradient(rgba(148,163,184,0.25) 0% 100%)'
  let acc = 0
  const stops: string[] = []
  for (const seg of normalizedSegments.value) {
    const start = acc
    acc += seg.pct
    stops.push(`${seg.color} ${start}% ${acc}%`)
  }
  return `conic-gradient(${stops.join(', ')})`
})

// 用 mask 打透明镂空，使中心透出所在容器背景（明暗主题、任意卡片色均适配）
const ringStyle = computed(() => {
  const mask = `radial-gradient(farthest-side, transparent calc(100% - ${props.thickness}px), #000 calc(100% - ${props.thickness}px))`
  return {
    background: gradient.value,
    maskImage: mask,
    WebkitMaskImage: mask,
  }
})
</script>
