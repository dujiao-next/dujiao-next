<template>
  <section class="st-shelf">
    <div class="st-container">
      <div class="st-shelf-head">
        <h2 class="st-shelf-title">{{ title }}</h2>
        <p v-if="secondary" class="st-shelf-sub">{{ secondary }}</p>
      </div>
    </div>

    <div class="relative">
      <div ref="scroller" class="st-scroller st-container" @scroll="sync">
        <StoreCard
          v-for="card in cards"
          :key="card.key"
          :slug="card.slug"
          :title="card.title"
          :eyebrow="card.eyebrow"
          :price="card.price"
          :note="card.note"
          :image="card.image"
          :dark="card.dark"
        />
        <div class="st-scroller-tail" aria-hidden="true"></div>
      </div>

      <button class="st-arrow st-arrow--prev" type="button" :aria-label="t('common.previousBanner')" :disabled="atStart" @click="page(-1)">
        <ChevronLeft class="h-4 w-4" />
      </button>
      <button class="st-arrow st-arrow--next" type="button" :aria-label="t('common.nextBanner')" :disabled="atEnd" @click="page(1)">
        <ChevronRight class="h-4 w-4" />
      </button>
    </div>
  </section>
</template>

<script setup lang="ts">
import { nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ChevronLeft, ChevronRight } from 'lucide-vue-next'
import StoreCard, { type StoreCardData } from './StoreCard.vue'

const props = defineProps<{
  title: string
  secondary?: string
  cards: StoreCardData[]
}>()

const { t } = useI18n()
const scroller = ref<HTMLElement | null>(null)
const atStart = ref(true)
const atEnd = ref(false)

const sync = () => {
  const el = scroller.value
  if (!el) return
  atStart.value = el.scrollLeft <= 4
  atEnd.value = el.scrollLeft + el.clientWidth >= el.scrollWidth - 4
}

const page = (dir: number) => {
  const el = scroller.value
  if (!el) return
  const card = el.querySelector<HTMLElement>('.st-card')
  const step = card ? card.offsetWidth + 20 : el.clientWidth * 0.8
  el.scrollBy({ left: dir * step, behavior: 'smooth' })
}

onMounted(async () => {
  await nextTick()
  sync()
  window.addEventListener('resize', sync)
})

// 商品是异步加载的，卡片到位后重新计算两端箭头状态
watch(() => props.cards.length, async () => {
  await nextTick()
  sync()
})
</script>
