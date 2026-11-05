<template>
  <div ref="root" class="blk-root">
    <div
      v-for="(block, index) in blocks"
      :key="block.id"
      class="blk-reveal"
      :class="{ 'is-visible': index === 0 || revealed[block.id] === true }"
      :data-block-id="block.id"
    >
      <component :is="resolve(block.type)" :p="block.props" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import type { PageBlock } from './types'
import HeroBlock from './HeroBlock.vue'
import PromoBlock from './PromoBlock.vue'
import SectionHeaderBlock from './SectionHeaderBlock.vue'
import FeatureGridBlock from './FeatureGridBlock.vue'
import CardScrollerBlock from './CardScrollerBlock.vue'
import GalleryBlock from './GalleryBlock.vue'
import CtaBlock from './CtaBlock.vue'
import VideoBlock from './VideoBlock.vue'
import StoryBlock from './StoryBlock.vue'
import CompareBlock from './CompareBlock.vue'
import SpecsBlock from './SpecsBlock.vue'
import RichTextBlock from './RichTextBlock.vue'
import './blocks.css'

const props = defineProps<{ blocks: PageBlock[] }>()

const registry: Record<string, any> = {
  hero: HeroBlock,
  promo: PromoBlock,
  sectionHeader: SectionHeaderBlock,
  featureGrid: FeatureGridBlock,
  cardScroller: CardScrollerBlock,
  gallery: GalleryBlock,
  cta: CtaBlock,
  video: VideoBlock,
  story: StoryBlock,
  compare: CompareBlock,
  specs: SpecsBlock,
  richText: RichTextBlock,
}

const resolve = (type: string) => registry[type] || null

/* 进入视口渐显（首块直接显示，避免首屏闪）
   注意：可见状态必须走响应式，不能直接改 DOM class ——
   1) 后台预览里新增的区块是后插入的，必须能被接管，否则永远 opacity:0（一块空白）
   2) Vue 重渲染时会重写 class 属性，手工加的 is-visible 会被冲掉 */
const root = ref<HTMLElement | null>(null)
const revealed = reactive<Record<string, boolean>>({})
let observer: IntersectionObserver | null = null
const watching = new Map<Element, string>()

const armNewBlocks = () => {
  const nodes = root.value?.querySelectorAll<HTMLElement>('.blk-reveal') || []
  const pending: Array<{ el: HTMLElement; id: string }> = []
  nodes.forEach((el, index) => {
    const id = el.dataset.blockId || ''
    if (!id || index === 0 || revealed[id]) return
    pending.push({ el, id })
  })
  if (!pending.length) return

  if (!observer || typeof IntersectionObserver === 'undefined') {
    pending.forEach(({ id }) => { revealed[id] = true })
    return
  }
  pending.forEach(({ el, id }) => {
    watching.set(el, id)
    observer?.observe(el)
  })
  // 兜底：1 秒后无论是否进入视口都显示，避免截图/无滚动时内容一直隐藏
  window.setTimeout(() => {
    pending.forEach(({ id }) => { revealed[id] = true })
  }, 1000)
}

onMounted(() => {
  if (typeof IntersectionObserver !== 'undefined') {
    observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (!entry.isIntersecting) continue
          const id = watching.get(entry.target)
          if (id) revealed[id] = true
          watching.delete(entry.target)
          observer?.unobserve(entry.target)
        }
      },
      { rootMargin: '0px 0px -8% 0px', threshold: 0.05 },
    )
  }
  armNewBlocks()
})

// 后台实时预览会整段替换 blocks：新出现的区块也要接管
watch(
  () => props.blocks,
  async () => {
    await nextTick()
    armNewBlocks()
  },
)

onUnmounted(() => {
  observer?.disconnect()
  watching.clear()
})
</script>
