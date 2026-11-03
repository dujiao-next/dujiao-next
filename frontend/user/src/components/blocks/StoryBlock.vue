<template>
  <section class="blk-story" :class="`blk-story--${p.theme || 'dark'}`">
    <!-- 手机 / 减少动效：普通堆叠，不做钉住 -->
    <div v-if="isStacked" class="blk-story-stack">
      <div v-if="p.title" class="blk-inner blk-story-head">
        <h2 class="blk-h2">{{ txt(p.title) }}</h2>
      </div>
      <article v-for="(step, i) in p.steps" :key="i" class="blk-story-step">
        <div class="blk-story-step-media"><BlockMedia :media="step.media" /></div>
        <div class="blk-inner">
          <p v-if="step.label" class="blk-eyebrow">{{ txt(step.label) }}</p>
          <h3 class="blk-story-title">{{ txt(step.title) }}</h3>
          <p v-if="step.body" class="blk-story-body">{{ txt(step.body) }}</p>
        </div>
      </article>
    </div>

    <!-- 桌面：媒体钉住，文案随滚动切换 -->
    <div v-else ref="root" class="blk-story-track" :style="{ height: `${p.steps.length * 100}vh` }">
      <div class="blk-story-sticky">
        <div class="blk-story-media">
          <BlockMedia v-for="(step, i) in p.steps" :key="i" v-show="i === active" :media="step.media" />
        </div>
        <div class="blk-story-copy">
          <div v-if="p.title" class="blk-story-kicker">{{ txt(p.title) }}</div>
          <div v-for="(step, i) in p.steps" :key="i" v-show="i === active" class="blk-story-text">
            <p v-if="step.label" class="blk-eyebrow">{{ txt(step.label) }}</p>
            <h3 class="blk-story-title">{{ txt(step.title) }}</h3>
            <p v-if="step.body" class="blk-story-body">{{ txt(step.body) }}</p>
          </div>
          <div class="blk-story-progress" aria-hidden="true">
            <span
              v-for="(_, i) in p.steps"
              :key="i"
              class="blk-story-pip"
              :class="{ 'is-active': i === active }"
            />
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import BlockMedia from './BlockMedia.vue'
import type { StoryBlock } from './types'
import { useBlockText } from './util'

const props = defineProps<{ p: StoryBlock['props'] }>()
const { txt } = useBlockText()

const isStacked = ref(false)
const root = ref<HTMLElement | null>(null)
const active = ref(0)
let frame = 0

const stepCount = computed(() => props.p.steps.length)

const measure = () => {
  isStacked.value =
    window.matchMedia?.('(max-width: 734px)').matches ||
    window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ||
    false
}

const update = () => {
  frame = 0
  const el = root.value
  if (!el || isStacked.value) return
  const rect = el.getBoundingClientRect()
  const total = rect.height - window.innerHeight
  if (total <= 0) return
  const passed = Math.min(Math.max(-rect.top, 0), total)
  const ratio = passed / total
  const index = Math.min(Math.floor(ratio * stepCount.value), stepCount.value - 1)
  active.value = Math.max(index, 0)
}

const onScroll = () => {
  if (frame) return
  frame = requestAnimationFrame(update)
}

const onResize = () => {
  measure()
  update()
}

onMounted(() => {
  measure()
  update()
  window.addEventListener('scroll', onScroll, { passive: true })
  window.addEventListener('resize', onResize)
})

onUnmounted(() => {
  window.removeEventListener('scroll', onScroll)
  window.removeEventListener('resize', onResize)
  if (frame) cancelAnimationFrame(frame)
})
</script>
