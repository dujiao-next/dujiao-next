<template>
  <div class="st-rail">
    <div ref="scroller" class="st-rail-scroller" @scroll="sync">
      <RouterLink
        v-for="item in items"
        :key="item.key"
        class="st-rail-item"
        :to="item.to"
      >
        <span class="st-rail-img">
          <img v-if="item.icon" :src="item.icon" :alt="item.label" loading="lazy" />
          <component :is="item.fallback" v-else class="h-12 w-12 opacity-70" />
        </span>
        <span class="st-rail-label">{{ item.label }}</span>
      </RouterLink>
    </div>

    <button class="st-arrow st-arrow--prev" type="button" aria-label="prev" :disabled="atStart" @click="scrollByPage(-1)">
      <ChevronLeft class="h-4 w-4" />
    </button>
    <button class="st-arrow st-arrow--next" type="button" aria-label="next" :disabled="atEnd" @click="scrollByPage(1)">
      <ChevronRight class="h-4 w-4" />
    </button>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { ChevronLeft, ChevronRight } from 'lucide-vue-next'

export interface RailItem {
  key: string | number
  label: string
  to: string
  icon?: string
  fallback: any
}

defineProps<{ items: RailItem[] }>()

const scroller = ref<HTMLElement | null>(null)
const atStart = ref(true)
const atEnd = ref(false)

const sync = () => {
  const el = scroller.value
  if (!el) return
  atStart.value = el.scrollLeft <= 4
  atEnd.value = el.scrollLeft + el.clientWidth >= el.scrollWidth - 4
}

const scrollByPage = (dir: number) => {
  const el = scroller.value
  if (!el) return
  el.scrollBy({ left: dir * el.clientWidth * 0.8, behavior: 'smooth' })
}

onMounted(() => {
  sync()
  window.addEventListener('resize', sync)
})
</script>
