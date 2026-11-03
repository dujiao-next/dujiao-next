<template>
  <RouterLink :to="`/products/${slug}`" class="st-card" :class="{ 'st-card--dark': dark }">
    <div class="st-card-info">
      <p v-if="eyebrow" class="st-eyebrow">{{ eyebrow }}</p>
      <h3 class="st-card-title">{{ title }}</h3>
      <p v-if="note || price" class="st-card-desc">
        <span class="st-card-price">{{ price }}</span>
        <span v-if="note"> {{ note }}</span>
      </p>
    </div>
    <div class="st-card-img">
      <img v-if="image" :src="image" :alt="title" loading="lazy" />
      <Package v-else class="mb-10 h-16 w-16 opacity-30" />
    </div>
  </RouterLink>
</template>

<script setup lang="ts">
import { RouterLink } from 'vue-router'
import { Package } from 'lucide-vue-next'

export interface StoreCardData {
  key: string | number
  slug: string
  eyebrow?: string
  title: string
  price?: string
  note?: string
  image?: string
  dark?: boolean
}

defineProps<{
  slug: string
  title: string
  eyebrow?: string
  price?: string
  note?: string
  image?: string
  dark?: boolean
}>()
</script>
