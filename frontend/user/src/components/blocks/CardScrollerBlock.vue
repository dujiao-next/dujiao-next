<template>
  <section>
    <div class="blk-inner">
      <div v-if="p.title || p.secondary" class="blk-scroller-head">
        <h2 v-if="p.title" class="blk-scroller-title">{{ txt(p.title) }}</h2>
        <p v-if="p.secondary" class="blk-scroller-sub">{{ txt(p.secondary) }}</p>
      </div>
    </div>

    <div class="blk-scroller-wrap">
      <div ref="scroller" class="blk-scroller blk-inner" @scroll="sync">
        <component
          :is="card.url ? 'a' : 'div'"
          v-for="(card, i) in p.cards"
          :key="i"
          class="blk-card"
          :href="card.url || undefined"
        >
          <div v-if="card.media" class="blk-card-media">
            <BlockMedia :media="card.media" />
          </div>
          <div class="blk-card-body">
            <p v-if="card.label" class="blk-card-label">{{ txt(card.label) }}</p>
            <h3 class="blk-card-title">{{ txt(card.title) }}</h3>
            <p v-if="card.body" class="blk-card-text">{{ txt(card.body) }}</p>
          </div>
        </component>
      </div>

      <button class="blk-arrow blk-arrow--prev" type="button" :disabled="atStart" @click="page(-1)">‹</button>
      <button class="blk-arrow blk-arrow--next" type="button" :disabled="atEnd" @click="page(1)">›</button>
    </div>

    <!-- 底部控制条：进度点 + 播放/暂停（苹果「先刷重点」那种） -->
    <div v-if="p.cards.length > 1" class="blk-scroller-dock">
      <div class="blk-dock">
        <button
          v-for="(_, i) in p.cards"
          :key="i"
          type="button"
          class="blk-dot"
          :class="{ 'is-active': i === activeIndex }"
          :aria-label="`第 ${i + 1} 张`"
          @click="goTo(i)"
        />
      </div>
      <button type="button" class="blk-dock-btn" :aria-label="playing ? '暂停' : '播放'" @click="togglePlay">
        <span v-if="playing" class="blk-pause-icon" />
        <span v-else class="blk-play-icon" />
      </button>
    </div>
  </section>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch, nextTick } from 'vue'
import BlockMedia from './BlockMedia.vue'
import type { CardScrollerBlock } from './types'
import { useBlockText } from './util'

const props = defineProps<{ p: CardScrollerBlock['props'] }>()
const { txt } = useBlockText()

const scroller = ref<HTMLElement | null>(null)
const atStart = ref(true)
const atEnd = ref(false)
const activeIndex = ref(0)
const playing = ref(true)
let autoTimer: ReturnType<typeof setInterval> | null = null

const sync = () => {
  const el = scroller.value
  if (!el) return
  atStart.value = el.scrollLeft <= 4
  atEnd.value = el.scrollLeft + el.clientWidth >= el.scrollWidth - 4
  const card = el.querySelector<HTMLElement>('.blk-card')
  if (card) {
    const step = card.offsetWidth + 12
    activeIndex.value = Math.min(
      Math.max(Math.round(el.scrollLeft / step), 0),
      props.p.cards.length - 1,
    )
  }
}

const goTo = (index: number) => {
  const el = scroller.value
  const card = el?.querySelector<HTMLElement>('.blk-card')
  if (!el || !card) return
  el.scrollTo({ left: index * (card.offsetWidth + 12), behavior: 'smooth' })
  activeIndex.value = index
}

const stopAuto = () => {
  if (autoTimer) {
    clearInterval(autoTimer)
    autoTimer = null
  }
}

const startAuto = () => {
  stopAuto()
  if (!playing.value || props.p.cards.length < 2) return
  autoTimer = setInterval(() => {
    const next = (activeIndex.value + 1) % props.p.cards.length
    goTo(next)
  }, 4500)
}

const togglePlay = () => {
  playing.value = !playing.value
  if (playing.value) startAuto()
  else stopAuto()
}
const page = (dir: number) => {
  const el = scroller.value
  if (!el) return
  const card = el.querySelector<HTMLElement>('.blk-card')
  const step = card ? card.offsetWidth + 12 : el.clientWidth * 0.8
  el.scrollBy({ left: dir * step, behavior: 'smooth' })
}

onMounted(async () => {
  await nextTick()
  sync()
  window.addEventListener('resize', sync)
  // 指针进入时暂停自动播放，离开后恢复
  const el = scroller.value
  el?.addEventListener('pointerenter', () => stopAuto())
  el?.addEventListener('pointerleave', () => { if (playing.value) startAuto() })
  startAuto()
})
onUnmounted(() => {
  stopAuto()
  window.removeEventListener('resize', sync)
})
watch(() => props.p.cards.length, async () => {
  await nextTick()
  sync()
  startAuto()
})
</script>
