<template>
  <section class="blk-video">
    <div v-if="p.title" class="blk-inner">
      <div class="blk-scroller-head">
        <h2 class="blk-scroller-title">{{ txt(p.title) }}</h2>
      </div>
    </div>

    <div v-if="!playing" class="blk-video-poster" @click="playing = true">
      <img v-if="poster" :src="poster" :alt="txt(p.title) || ''" />
      <button type="button" class="blk-play" aria-label="play">
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M8 5v14l11-7z" fill="currentColor" /></svg>
      </button>
    </div>

    <div v-else class="blk-video-player">
      <video :src="p.media.src" :poster="poster" controls autoplay playsinline></video>
    </div>

    <p v-if="p.caption" class="blk-inner blk-video-caption">{{ txt(p.caption) }}</p>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import type { VideoBlock } from './types'
import { useBlockText } from './util'

const props = defineProps<{ p: VideoBlock['props'] }>()
const { txt } = useBlockText()
const playing = ref(false)
const poster = computed(() => props.p.media.poster || (props.p.media.kind === 'image' ? props.p.media.src : ''))
</script>
