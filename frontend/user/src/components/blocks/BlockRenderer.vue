<template>
  <div ref="root" class="blk-root">
    <div v-for="(block, index) in blocks" :key="block.id" class="blk-reveal" :class="{ 'is-visible': index === 0 }">
      <component :is="resolve(block.type)" :p="block.props" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import type { PageBlock } from './types'
import HeroBlock from './HeroBlock.vue'
import PromoBlock from './PromoBlock.vue'
import SectionHeaderBlock from './SectionHeaderBlock.vue'
import FeatureGridBlock from './FeatureGridBlock.vue'
import CardScrollerBlock from './CardScrollerBlock.vue'
import GalleryBlock from './GalleryBlock.vue'
import CtaBlock from './CtaBlock.vue'
import VideoBlock from './VideoBlock.vue'
import SpecsBlock from './SpecsBlock.vue'
import RichTextBlock from './RichTextBlock.vue'
import './blocks.css'

defineProps<{ blocks: PageBlock[] }>()

const registry: Record<string, any> = {
  hero: HeroBlock,
  promo: PromoBlock,
  sectionHeader: SectionHeaderBlock,
  featureGrid: FeatureGridBlock,
  cardScroller: CardScrollerBlock,
  gallery: GalleryBlock,
  cta: CtaBlock,
  video: VideoBlock,
  specs: SpecsBlock,
  richText: RichTextBlock,
}

const resolve = (type: string) => registry[type] || null

/* 进入视口渐显（首块直接显示，避免首屏闪） */
const root = ref<HTMLElement | null>(null)
let observer: IntersectionObserver | null = null

onMounted(() => {
  const targets = root.value?.querySelectorAll<HTMLElement>('.blk-reveal:not(.is-visible)') || []
  if (!targets.length || typeof IntersectionObserver === 'undefined') {
    targets.forEach((el) => el.classList.add('is-visible'))
    return
  }
  observer = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (entry.isIntersecting) {
          entry.target.classList.add('is-visible')
          observer?.unobserve(entry.target)
        }
      }
    },
    { rootMargin: '0px 0px -8% 0px', threshold: 0.05 },
  )
  targets.forEach((el) => observer?.observe(el))

  // 兜底：1 秒后无论是否进入视口都显示，避免截图/无滚动时内容一直隐藏
  window.setTimeout(() => {
    targets.forEach((el) => el.classList.add('is-visible'))
  }, 1000)
})

onUnmounted(() => observer?.disconnect())
</script>
