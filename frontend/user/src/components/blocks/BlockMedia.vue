<template>
  <!-- 允许 media 缺失：配方/新加的图库行常常还没选图，不能因此整段页面崩掉 -->
  <div v-if="!resolved.src && resolved.kind !== 'video'" class="blk-media-empty" aria-hidden="true"></div>
  <img
    v-else-if="resolved.kind !== 'video'"
    :src="src"
    :alt="resolved.alt || ''"
    loading="lazy"
  />
  <video
    v-else
    ref="videoEl"
    :src="resolved.src"
    :poster="resolved.poster"
    muted
    loop
    playsinline
    preload="metadata"
  ></video>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import type { BlockMedia } from './types'

const props = defineProps<{ media?: BlockMedia; mobile?: boolean }>()

const resolved = computed<BlockMedia>(
  () => props.media || ({ kind: 'image', src: '' } as BlockMedia),
)

const src = computed(() => (props.mobile && resolved.value.mobileSrc) || resolved.value.src)

/**
 * 视频只在进入视口时播放（苹果的做法）：
 * 滚出视口就暂停，不浪费电量和流量；减少动效时保持暂停，显示封面帧。
 */
const videoEl = ref<HTMLVideoElement | null>(null)
let observer: IntersectionObserver | null = null

onMounted(() => {
  if (resolved.value.kind !== 'video') return
  const el = videoEl.value
  if (!el) return
  if (window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) return
  if (typeof IntersectionObserver === 'undefined') {
    void el.play().catch(() => {})
    return
  }
  observer = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (entry.isIntersecting) {
          void el.play().catch(() => {})
        } else {
          el.pause()
        }
      }
    },
    { threshold: 0.25 },
  )
  observer.observe(el)
})

onUnmounted(() => observer?.disconnect())
</script>
