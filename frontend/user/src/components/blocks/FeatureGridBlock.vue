<template>
  <section class="blk-inner">
    <div class="blk-grid" :class="`blk-grid--${p.columns || 3}`">
      <article v-for="(item, i) in p.items" :key="i" class="blk-feature">
        <div v-if="item.media" class="blk-feature-media">
          <BlockMedia :media="item.media" />
        </div>
        <h3 class="blk-feature-title">{{ txt(item.title) }}</h3>
        <p v-if="item.body" class="blk-feature-body">{{ txt(item.body) }}</p>
      </article>
    </div>
  </section>
</template>

<script setup lang="ts">
import BlockMedia from './BlockMedia.vue'
import type { FeatureGridBlock } from './types'
import { useBlockText } from './util'

defineProps<{ p: FeatureGridBlock['props'] }>()
const { txt } = useBlockText()
</script>
