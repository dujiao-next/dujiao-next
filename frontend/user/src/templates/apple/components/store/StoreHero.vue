<template>
  <section class="st-container st-hero">
    <h1 class="st-h1">{{ title }}</h1>
    <div class="st-hero-side">
      <p v-if="subtitle" class="st-sub">{{ subtitle }}</p>
      <div v-if="links.length" class="st-links">
        <template v-for="link in links" :key="link.label">
          <a
            v-if="link.external"
            class="st-link"
            :href="link.to"
            target="_blank"
            rel="noopener noreferrer"
          >
            {{ link.label }}
            <ArrowUpRight />
          </a>
          <RouterLink v-else class="st-link" :to="link.to">
            {{ link.label }}
            <ChevronRight />
          </RouterLink>
        </template>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ArrowUpRight, ChevronRight } from 'lucide-vue-next'
import { RouterLink } from 'vue-router'

defineProps<{
  title: string
  subtitle?: string
  links: Array<{ label: string; to: string; external?: boolean }>
}>()
</script>
