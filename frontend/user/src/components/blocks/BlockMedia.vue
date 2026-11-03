<template>
  <img
    v-if="media.kind !== 'video'"
    :src="src"
    :alt="media.alt || ''"
    loading="lazy"
  />
  <video
    v-else
    :src="media.src"
    :poster="media.poster"
    autoplay
    muted
    loop
    playsinline
    preload="metadata"
  ></video>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { BlockMedia } from './types'

const props = defineProps<{ media: BlockMedia; mobile?: boolean }>()

const src = computed(() => (props.mobile && props.media.mobileSrc) || props.media.src)
</script>
