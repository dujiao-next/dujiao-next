<template>
  <div v-if="items?.length" class="blk-ctas">
    <a
      v-for="(cta, i) in items"
      :key="i"
      class="blk-btn"
      :class="`blk-btn--${cta.style || 'primary'}`"
      :href="cta.url"
    >{{ txt(cta.label) }}</a>
  </div>
</template>

<script setup lang="ts">
import type { BlockCta } from './types'
import { useBlockText } from './util'

defineProps<{ items?: BlockCta[] }>()
const { txt } = useBlockText()
</script>
