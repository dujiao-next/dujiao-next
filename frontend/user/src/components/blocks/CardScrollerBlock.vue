<template>
  <section>
    <div class="blk-inner">
      <div v-if="p.title || p.secondary" class="blk-scroller-head">
        <h2 v-if="p.title" class="blk-scroller-title">{{ txt(p.title) }}</h2>
        <p v-if="p.secondary" class="blk-scroller-sub">{{ txt(p.secondary) }}</p>
      </div>
    </div>

    <div ref="wrap" class="blk-scroller-wrap">
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

      <button class="blk-arrow blk-arrow--prev" type="button" :disabled="atStart" aria-label="上一张" @click="page(-1)">
        <svg class="blk-arrow-icon" viewBox="0 0 36 36" aria-hidden="true"><path d="m20 25c-.3838 0-.7676-.1465-1.0605-.4395l-5.5-5.5c-.5859-.5854-.5859-1.5356 0-2.1211l5.5-5.5c.5859-.5859 1.5352-.5859 2.1211 0 .5859.5854.5859 1.5356 0 2.1211l-4.4395 4.4395 4.4395 4.4395c.5859.5854.5859 1.5356 0 2.1211-.293.293-.6768.4395-1.0605.4395z"/></svg>
      </button>
      <button class="blk-arrow blk-arrow--next" type="button" :disabled="atEnd" aria-label="下一张" @click="page(1)">
        <svg class="blk-arrow-icon" viewBox="0 0 36 36" aria-hidden="true"><path d="m20 25c-.3838 0-.7676-.1465-1.0605-.4395l-5.5-5.5c-.5859-.5854-.5859-1.5356 0-2.1211l5.5-5.5c.5859-.5859 1.5352-.5859 2.1211 0 .5859.5854.5859 1.5356 0 2.1211l-4.4395 4.4395 4.4395 4.4395c.5859.5854.5859 1.5356 0 2.1211-.293.293-.6768.4395-1.0605.4395z"/></svg>
      </button>

      <!-- 底部控制条：苹果 dotnav（216×56 胶囊 + 48 宽进度点 + 56 圆播放键） -->
      <div v-if="p.cards.length > 1" class="blk-scroller-dock">
        <div
          class="blk-dock"
          :class="{ 'is-paused': paused, 'is-entered': entered }"
          :style="{ '--blk-slide': `${SLIDE_MS}ms` }"
        >
          <button
            v-for="(_, i) in p.cards"
            :key="i"
            type="button"
            class="blk-dot"
            :class="{ 'is-active': i === activeIndex }"
            :aria-label="`第 ${i + 1} 张`"
            :aria-current="i === activeIndex ? 'true' : undefined"
            @click="goTo(i)"
          >
            <!-- 当前项的白色进度：随停留时间从 8px 长到 48px，播完自动翻下一页 -->
            <span
              v-if="i === activeIndex"
              :key="activeIndex"
              class="blk-dot-fill"
              @animationend="advance"
            />
          </button>
        </div>

        <button
          type="button"
          class="blk-dock-btn"
          :aria-label="playing ? '暂停' : '播放'"
          @click="togglePlay"
        >
          <svg v-if="playing" class="blk-dock-icon" viewBox="0 0 16 18" aria-hidden="true">
            <rect x="0" y="0" width="6" height="18" rx="2" />
            <rect x="10" y="0" width="6" height="18" rx="2" />
          </svg>
          <svg v-else class="blk-dock-icon blk-dock-icon--play" viewBox="0 0 16 18" aria-hidden="true">
            <path d="M2.2 1.6c0-.95 1.03-1.54 1.85-1.06l10.5 6.4c.8.49.8 1.63 0 2.12l-10.5 6.4c-.82.48-1.85-.11-1.85-1.06V1.6Z" />
          </svg>
        </button>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch, nextTick } from 'vue'
import BlockMedia from './BlockMedia.vue'
import type { CardScrollerBlock } from './types'
import { useBlockText } from './util'

const props = defineProps<{ p: CardScrollerBlock['props'] }>()
const { txt } = useBlockText()

/** 每张停留时长，和苹果 dotnav 的进度条一致 */
const SLIDE_MS = 4500

const scroller = ref<HTMLElement | null>(null)
const wrap = ref<HTMLElement | null>(null)
const atStart = ref(true)
const atEnd = ref(false)
const activeIndex = ref(0)

const reducedMotion = typeof window !== 'undefined'
  && window.matchMedia('(prefers-reduced-motion: reduce)').matches
const playing = ref(!reducedMotion)
const inView = ref(false)
/** 首次进入视口时播一次出场动画 */
const entered = ref(false)
/** 只有「在播 + 在视口里」才走；苹果也是这样，鼠标悬停不停 */
const paused = computed(() => !playing.value || !inView.value)

let settleTimer: ReturnType<typeof setTimeout> | null = null
/** 用户自己拖/滚时才用滚动位置反推当前项；程序滚动（点圆点、自动翻页）以点击的为准 */
let userScrolling = false
let userScrollTimer: ReturnType<typeof setTimeout> | null = null

/** 一张卡片的滚动步长：两张卡的 offsetLeft 之差（自带宽 gap 和留白） */
const cardStep = () => {
  const el = scroller.value
  const cards = el?.querySelectorAll<HTMLElement>('.blk-card')
  const first = cards?.[0]
  const second = cards?.[1]
  if (first && second) return second.offsetLeft - first.offsetLeft
  if (first) return first.offsetWidth + 12
  return (el?.clientWidth ?? 0) * 0.8
}

const sync = () => {
  const el = scroller.value
  if (!el) return
  atStart.value = el.scrollLeft <= 4
  atEnd.value = el.scrollLeft + el.clientWidth >= el.scrollWidth - 4
  // 滚动停稳后再更新当前项，避免平滑滚动途中进度点来回跳
  if (settleTimer) clearTimeout(settleTimer)
  settleTimer = setTimeout(() => {
    settleTimer = null
    if (!userScrolling) return
    const step = cardStep()
    if (!step) return
    const index = Math.min(
      Math.max(Math.round(el.scrollLeft / step), 0),
      props.p.cards.length - 1,
    )
    if (index !== activeIndex.value) activeIndex.value = index
  }, 140)
}

/** 用户拖拽/滚轮/触摸 → 之后 800ms 内以滚动位置为准 */
const onUserScrollIntent = () => {
  userScrolling = true
  if (userScrollTimer) clearTimeout(userScrollTimer)
  userScrollTimer = setTimeout(() => { userScrolling = false }, 800)
}

const goTo = (index: number) => {
  const el = scroller.value
  if (!el) return
  activeIndex.value = index
  el.scrollTo({ left: index * cardStep(), behavior: 'smooth' })
}

/** 进度条走完 → 下一张 */
const advance = () => {
  if (paused.value) return
  goTo((activeIndex.value + 1) % props.p.cards.length)
}

const togglePlay = () => {
  playing.value = !playing.value
}

const page = (dir: number) => {
  const el = scroller.value
  if (!el) return
  const step = cardStep()
  if (!step) return
  const current = Math.round(el.scrollLeft / step)
  const next = Math.min(Math.max(current + dir, 0), props.p.cards.length - 1)
  goTo(next)
}

let observer: IntersectionObserver | null = null

onMounted(async () => {
  await nextTick()
  sync()
  window.addEventListener('resize', sync)
  // 进视口才开始走，离开就停（苹果的 activated/inactive 也是这个逻辑）
  if (wrap.value && 'IntersectionObserver' in window) {
    observer = new IntersectionObserver((entries) => {
      for (const entry of entries) {
        inView.value = entry.isIntersecting
        if (entry.isIntersecting && !entered.value) entered.value = true
      }
    }, { threshold: 0.15 })
    observer.observe(wrap.value)
  } else {
    inView.value = true
    entered.value = true
  }
  const el = scroller.value
  el?.addEventListener('pointerdown', onUserScrollIntent)
  el?.addEventListener('wheel', onUserScrollIntent, { passive: true })
  el?.addEventListener('touchstart', onUserScrollIntent, { passive: true })
})

onUnmounted(() => {
  if (settleTimer) clearTimeout(settleTimer)
  if (userScrollTimer) clearTimeout(userScrollTimer)
  window.removeEventListener('resize', sync)
  observer?.disconnect()
  observer = null
  const el = scroller.value
  el?.removeEventListener('pointerdown', onUserScrollIntent)
  el?.removeEventListener('wheel', onUserScrollIntent)
  el?.removeEventListener('touchstart', onUserScrollIntent)
})

watch(() => props.p.cards.length, async () => {
  await nextTick()
  if (activeIndex.value > props.p.cards.length - 1) activeIndex.value = 0
  sync()
})
</script>
