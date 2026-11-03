<template>
  <section>
    <div v-if="p.title" class="blk-inner">
      <div class="blk-scroller-head">
        <h2 class="blk-scroller-title">{{ txt(p.title) }}</h2>
      </div>
    </div>
    <div class="blk-gallery blk-inner">
      <figure v-for="(item, i) in p.items" :key="i" class="blk-gallery-item">
        <BlockMedia :media="item.media" />
        <figcaption v-if="item.caption" class="blk-gallery-caption">{{ txt(item.caption) }}</figcaption>
      </figure>
    </div>
  </section>
</template>

<script setup lang="ts">
import BlockMedia from './BlockMedia.vue'
import type { GalleryBlock } from './types'
import { useBlockText } from './util'

defineProps<{ p: GalleryBlock['props'] }>()
const { txt } = useBlockText()
</script>
