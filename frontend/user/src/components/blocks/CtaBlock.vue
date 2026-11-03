<template>
  <section class="blk-cta">
    <div class="blk-inner">
      <h2 class="blk-h2">{{ txt(p.title) }}</h2>
      <p v-if="p.subtitle" class="blk-sub">{{ txt(p.subtitle) }}</p>
      <p v-if="p.price" class="blk-cta-price">{{ p.price }}</p>
      <div v-if="p.media" class="blk-cta-media">
        <BlockMedia :media="p.media" />
      </div>
      <CtaRow :items="p.ctas" />
    </div>
  </section>
</template>

<script setup lang="ts">
import BlockMedia from './BlockMedia.vue'
import CtaRow from './CtaRow.vue'
import type { CtaBlock } from './types'
import { useBlockText } from './util'

defineProps<{ p: CtaBlock['props'] }>()
const { txt } = useBlockText()
</script>
