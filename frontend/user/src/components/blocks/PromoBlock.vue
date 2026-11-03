<template>
  <section class="blk-promo">
    <article
      v-for="(item, i) in p.items"
      :key="i"
      class="blk-promo-item"
      :class="{ 'blk-promo-item--dark': (item.theme || 'dark') === 'dark' }"
    >
      <div class="blk-promo-copy">
        <p v-if="item.eyebrow" class="blk-eyebrow">{{ txt(item.eyebrow) }}</p>
        <h2 class="blk-promo-title">{{ txt(item.title) }}</h2>
        <p v-if="item.subtitle" class="blk-sub">{{ txt(item.subtitle) }}</p>
        <CtaRow :items="item.ctas" />
      </div>
      <div v-if="item.media" class="blk-promo-media">
        <BlockMedia :media="item.media" />
      </div>
    </article>
  </section>
</template>

<script setup lang="ts">
import BlockMedia from './BlockMedia.vue'
import CtaRow from './CtaRow.vue'
import type { PromoBlock } from './types'
import { useBlockText } from './util'

defineProps<{ p: PromoBlock['props'] }>()
const { txt } = useBlockText()
</script>
