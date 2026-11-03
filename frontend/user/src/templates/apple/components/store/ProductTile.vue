<template>
  <RouterLink class="pt" :to="`/products/${slug}`">
    <span class="pt-media">
      <img v-if="image" :src="image" :alt="title" loading="lazy" />
      <Package v-else class="pt-empty" />
    </span>
    <span class="pt-body">
      <span v-if="eyebrow" class="pt-eyebrow">{{ eyebrow }}</span>
      <span class="pt-name">{{ title }}</span>
      <span v-if="price" class="pt-price">{{ price }}</span>
    </span>
  </RouterLink>
</template>

<script setup lang="ts">
import { RouterLink } from 'vue-router'
import { Package } from 'lucide-vue-next'

defineProps<{
  slug: string
  title: string
  price?: string
  eyebrow?: string
  image?: string
}>()
</script>

<style scoped>
.pt {
  height: 100%;
  display: flex;
  flex-direction: column;
  border-radius: 18px;
  overflow: hidden;
  background: var(--st-bg-alt);
  text-decoration: none;
  color: var(--st-text);
  transition: transform 320ms var(--st-ease);
}
@media (min-width: 1024px) {
  .pt:hover {
    transform: scale(1.01);
  }
}
.pt-media {
  display: grid;
  place-items: center;
  aspect-ratio: 4 / 3;
  overflow: hidden;
  background: var(--st-bg-alt);
}
.pt-media img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
.pt-empty {
  width: 42px;
  height: 42px;
  opacity: 0.3;
}
.pt-body {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 18px 16px 22px;
  text-align: center;
}
.pt-eyebrow {
  font-size: 12px;
  font-weight: 600;
  color: var(--st-orange);
}
.pt-name {
  font-family: var(--st-font-display);
  font-size: 17px;
  line-height: 1.29;
  font-weight: 600;
  min-height: 2.6em;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
@media (min-width: 735px) {
  .pt-name {
    font-size: 21px;
  }
}
.pt-price {
  font-size: 14px;
  line-height: 1.29;
  color: var(--st-text-2);
}
</style>
