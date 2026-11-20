<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { AlertTriangle, CheckCircle2, Info, Megaphone, X } from 'lucide-vue-next'
import { useLocalized } from '../composables/useProduct'
import { useNavConfig } from '../composables/useNavConfig'
import { announcementToTickerItems } from '../utils/announcementTicker'
import type { HomeAnnouncement } from '../composables/useAnnouncement'

const props = defineProps<{
  announcement: HomeAnnouncement
}>()

const emit = defineEmits<{
  close: []
}>()

const { t } = useI18n()
const { getLocalizedText } = useLocalized()
const { noticeEnabled } = useNavConfig()

// 滚动速度（像素 / 秒）
const SPEED = 55

const label = computed(() => String(getLocalizedText(props.announcement.title) || '').trim() || t('announcement.label'))
const items = computed(() => announcementToTickerItems(String(getLocalizedText(props.announcement.content) || '')))

// 公告页开启时，点击条目跳转公告列表；关闭时保持纯文本
const itemAttrs = computed(() => (noticeEnabled.value ? { to: '/notice' } : {}))

const typeStyle = computed(() => {
  switch (props.announcement.type) {
    case 'warning':
      return { label: 'bg-warning-soft text-warning', icon: AlertTriangle }
    case 'info':
      return { label: 'bg-info-soft text-info', icon: Info }
    case 'success':
      return { label: 'bg-success-soft text-success', icon: CheckCircle2 }
    default:
      return { label: 'bg-primary/10 text-primary', icon: Megaphone }
  }
})

const viewportEl = ref<HTMLElement | null>(null)
const groupEl = ref<HTMLElement | null>(null)
const scrolling = ref(false)
const duration = ref(30)

// 内容比可视区窄就静止展示；否则复制一份首尾相接，匀速无缝循环
const measure = async () => {
  await nextTick()
  const viewport = viewportEl.value
  const group = groupEl.value
  if (!viewport || !group) return
  const groupWidth = group.getBoundingClientRect().width
  scrolling.value = groupWidth > viewport.clientWidth + 1
  duration.value = Math.max(8, Math.round(groupWidth / SPEED))
}

let observer: ResizeObserver | null = null

onMounted(() => {
  void measure()
  if (typeof ResizeObserver !== 'undefined' && viewportEl.value) {
    observer = new ResizeObserver(() => void measure())
    observer.observe(viewportEl.value)
  }
})

onBeforeUnmount(() => observer?.disconnect())

watch(items, () => void measure())
</script>

<template>
  <div
    v-if="items.length"
    class="ticker relative flex h-10 items-center gap-1 overflow-hidden rounded-xl border bg-card text-sm"
    role="region"
    :aria-label="label"
  >
    <div class="flex h-full shrink-0 items-center gap-1.5 px-3 font-semibold" :class="typeStyle.label">
      <component :is="typeStyle.icon" class="size-4 shrink-0" :stroke-width="2.25" />
      <span class="max-w-[6.5rem] truncate">{{ label }}</span>
    </div>

    <div ref="viewportEl" class="ticker-viewport min-w-0 flex-1 overflow-hidden" :class="{ 'ticker-fade': scrolling }">
      <div class="ticker-track flex w-max" :class="{ 'ticker-run': scrolling }" :style="{ '--ticker-duration': `${duration}s` }">
        <div ref="groupEl" class="flex shrink-0">
          <component
            :is="noticeEnabled ? RouterLink : 'span'"
            v-for="(item, i) in items"
            :key="i"
            v-bind="itemAttrs"
            class="ticker-item whitespace-nowrap px-5 text-muted-foreground"
            :class="{ 'ticker-link': noticeEnabled }"
          >{{ item }}</component>
        </div>
        <div v-if="scrolling" class="ticker-dup flex shrink-0" aria-hidden="true">
          <component
            :is="noticeEnabled ? RouterLink : 'span'"
            v-for="(item, i) in items"
            :key="i"
            v-bind="itemAttrs"
            class="ticker-item whitespace-nowrap px-5 text-muted-foreground"
            :class="{ 'ticker-link': noticeEnabled }"
          >{{ item }}</component>
        </div>
      </div>
    </div>

    <button
      type="button"
      class="mr-1.5 flex size-7 shrink-0 items-center justify-center rounded-full text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
      :aria-label="t('announcement.close')"
      @click="emit('close')"
    >
      <X class="size-4" :stroke-width="2" />
    </button>
  </div>
</template>

<style scoped>
.ticker-fade {
  -webkit-mask-image: linear-gradient(to right, transparent 0, #000 16px, #000 calc(100% - 16px), transparent 100%);
  mask-image: linear-gradient(to right, transparent 0, #000 16px, #000 calc(100% - 16px), transparent 100%);
}

.ticker-run {
  animation: ticker-scroll var(--ticker-duration, 30s) linear infinite;
  will-change: transform;
}

.ticker-link:hover {
  color: var(--ui-text-primary);
  text-decoration: underline;
  text-underline-offset: 3px;
}

/* 悬停 / 键盘聚焦时暂停，方便阅读与点击 */
.ticker:hover .ticker-run,
.ticker:focus-within .ticker-run {
  animation-play-state: paused;
}

@keyframes ticker-scroll {
  from {
    transform: translateX(0);
  }
  to {
    transform: translateX(-50%);
  }
}

/* 系统开启“减少动效”：不自动滚动，改为可手动横向滑动 */
@media (prefers-reduced-motion: reduce) {
  .ticker-run {
    animation: none;
  }
  .ticker-dup {
    display: none;
  }
  .ticker-viewport {
    overflow-x: auto;
    scrollbar-width: none;
  }
}
</style>
