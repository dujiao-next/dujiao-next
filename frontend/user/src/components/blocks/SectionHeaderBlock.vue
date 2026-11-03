<template>
  <header class="blk-inner">
    <div class="blk-section-header" :class="{ 'blk-section-header--accent': !!p.accent }" :style="p.accent ? { '--blk-accent': p.accent } : {}">
      <p v-if="p.eyebrow" class="blk-eyebrow">{{ txt(p.eyebrow) }}</p>
      <h2 class="blk-h2">{{ txt(p.title) }}</h2>
      <p v-if="p.lead" class="blk-sub">{{ txt(p.lead) }}</p>
      <CtaRow v-if="p.link" :items="[p.link]" />
    </div>
  </header>
</template>

<script setup lang="ts">
import CtaRow from './CtaRow.vue'
import type { SectionHeaderBlock } from './types'
import { useBlockText } from './util'

defineProps<{ p: SectionHeaderBlock['props'] }>()
const { txt } = useBlockText()
</script>
