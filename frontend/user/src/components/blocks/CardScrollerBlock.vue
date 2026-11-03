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
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref, watch, nextTick } from 'vue'
import BlockMedia from './BlockMedia.vue'
import type { CardScrollerBlock } from './types'
import { useBlockText } from './util'

const props = defineProps<{ p: CardScrollerBlock['props'] }>()
const { txt } = useBlockText()

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
  const card = el.querySelector<HTMLElement>('.blk-card')
  const step = card ? card.offsetWidth + 12 : el.clientWidth * 0.8
  el.scrollBy({ left: dir * step, behavior: 'smooth' })
}

onMounted(async () => { await nextTick(); sync(); window.addEventListener('resize', sync) })
watch(() => props.p.cards.length, async () => { await nextTick(); sync() })
</script>
