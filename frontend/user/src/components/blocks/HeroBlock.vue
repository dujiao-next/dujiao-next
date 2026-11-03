<template>
  <section
    class="blk-hero"
    :class="[
      `blk-hero--${p.theme || 'dark'}`,
      `blk-hero--${p.variant || 'stacked'}`,
      p.align === 'left' ? 'blk-hero--left' : '',
    ]"
  >
    <template v-if="(p.variant || 'stacked') === 'overlay'">
      <div v-if="p.media" class="blk-hero-media">
        <BlockMedia :media="p.media" />
      </div>
      <div class="blk-hero-overlay-copy">
        <p v-if="p.eyebrow" class="blk-eyebrow">{{ txt(p.eyebrow) }}</p>
        <h1 class="blk-display">{{ txt(p.title) }}</h1>
        <p v-if="p.subtitle" class="blk-sub">{{ txt(p.subtitle) }}</p>
        <CtaRow :items="p.ctas" />
      </div>
    </template>

    <template v-else>
      <div class="blk-inner blk-hero-copy">
        <p v-if="p.eyebrow" class="blk-eyebrow">{{ txt(p.eyebrow) }}</p>
        <h1 class="blk-display">{{ txt(p.title) }}</h1>
        <p v-if="p.subtitle" class="blk-sub">{{ txt(p.subtitle) }}</p>
        <CtaRow :items="p.ctas" />
      </div>
      <div v-if="p.media" class="blk-hero-stack-media">
        <BlockMedia :media="p.media" />
      </div>
    </template>
  </section>
</template>

<script setup lang="ts">
import BlockMedia from './BlockMedia.vue'
import CtaRow from './CtaRow.vue'
import type { HeroBlock } from './types'
import { useBlockText } from './util'

defineProps<{ p: HeroBlock['props'] }>()
const { txt } = useBlockText()
</script>
