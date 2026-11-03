<template>
  <section class="blk-compare">
    <div class="blk-inner">
      <div v-if="p.title || p.subtitle" class="blk-compare-head">
        <h2 v-if="p.title" class="blk-h2">{{ txt(p.title) }}</h2>
        <p v-if="p.subtitle" class="blk-lead">{{ txt(p.subtitle) }}</p>
      </div>

      <div class="blk-compare-scroll">
        <div class="blk-compare-grid" :style="{ '--cols': p.columns.length }">
          <!-- 表头空位（行标签列） -->
          <div class="blk-compare-corner" aria-hidden="true" />

          <!-- 每个产品列 -->
          <div v-for="(col, i) in p.columns" :key="`c-${i}`" class="blk-compare-col">
            <div v-if="col.media" class="blk-compare-media">
              <BlockMedia :media="col.media" />
            </div>
            <h3 class="blk-compare-title">{{ txt(col.title) }}</h3>
            <p v-if="col.subtitle" class="blk-compare-sub">{{ txt(col.subtitle) }}</p>
            <p v-if="col.price" class="blk-compare-price">{{ col.price }}</p>
            <a v-if="col.ctaUrl" class="blk-btn blk-btn--primary blk-compare-cta" :href="col.ctaUrl">
              {{ txt(col.ctaLabel) || '购买' }}
            </a>
          </div>

          <!-- 行 -->
          <template v-for="(row, r) in p.rows" :key="`r-${r}`">
            <div class="blk-compare-label">{{ txt(row.label) }}</div>
            <div v-for="(_, i) in p.columns" :key="`v-${r}-${i}`" class="blk-compare-value">
              {{ row.values[i] || '—' }}
            </div>
          </template>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import BlockMedia from './BlockMedia.vue'
import type { CompareBlock } from './types'
import { useBlockText } from './util'

defineProps<{ p: CompareBlock['props'] }>()
const { txt } = useBlockText()
</script>
