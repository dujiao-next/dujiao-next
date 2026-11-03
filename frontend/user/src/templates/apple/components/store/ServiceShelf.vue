<template>
  <section class="st-container st-shelf">
    <div class="st-shelf-head">
      <h2 class="st-shelf-title">{{ title }}</h2>
      <p v-if="secondary" class="st-shelf-sub">{{ secondary }}</p>
    </div>

    <div class="st-tiles">
      <template v-for="item in items" :key="item.key">
        <a
          v-if="item.external"
          class="st-tile"
          :href="item.to"
          target="_blank"
          rel="noopener noreferrer"
        >
          <component :is="item.icon" class="st-tile-icon" />
          <span class="st-tile-title">{{ item.title }}</span>
          <span class="st-tile-desc">{{ item.desc }}</span>
        </a>
        <RouterLink v-else class="st-tile" :to="item.to">
          <component :is="item.icon" class="st-tile-icon" />
          <span class="st-tile-title">{{ item.title }}</span>
          <span class="st-tile-desc">{{ item.desc }}</span>
        </RouterLink>
      </template>
    </div>
  </section>
</template>

<script setup lang="ts">
import { RouterLink } from 'vue-router'

export interface ServiceTile {
  key: string
  title: string
  desc: string
  to: string
  external?: boolean
  icon: any
}

defineProps<{
  title: string
  secondary?: string
  items: ServiceTile[]
}>()
</script>
