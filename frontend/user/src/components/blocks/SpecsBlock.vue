<template>
  <section class="blk-inner blk-specs">
    <h2 v-if="p.title" class="blk-h2 blk-specs-title">{{ txt(p.title) }}</h2>
    <dl class="blk-specs-grid" :class="`blk-specs-grid--${p.columns || 2}`">
      <div v-for="(item, i) in p.items" :key="i" class="blk-spec">
        <dt>{{ txt(item.label) }}</dt>
        <dd>{{ txt(item.value) }}</dd>
      </div>
    </dl>
  </section>
</template>

<script setup lang="ts">
import type { SpecsBlock } from './types'
import { useBlockText } from './util'

defineProps<{ p: SpecsBlock['props'] }>()
const { txt } = useBlockText()
</script>
