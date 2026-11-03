<template>
  <img
    v-if="media.kind !== 'video'"
    :src="src"
    :alt="media.alt || ''"
    loading="lazy"
  />
  <video
    v-else
    ref="videoEl"
    :src="media.src"
    :poster="media.poster"
    muted
    loop
    playsinline
    preload="metadata"
  ></video>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import type { BlockMedia } from './types'

const props = defineProps<{ media: BlockMedia; mobile?: boolean }>()

const src = computed(() => (props.mobile && props.media.mobileSrc) || props.media.src)

/**
 * 视频只在进入视口时播放（苹果的做法）：
 * 滚出视口就暂停，不浪费电量和流量；减少动效时保持暂停，显示封面帧。
 */
const videoEl = ref<HTMLVideoElement | null>(null)
let observer: IntersectionObserver | null = null

onMounted(() => {
  if (props.media.kind !== 'video') return
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
