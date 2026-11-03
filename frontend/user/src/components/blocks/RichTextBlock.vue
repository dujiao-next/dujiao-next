<template>
  <section class="blk-inner blk-rich">
    <div class="blk-rich-body prose" v-html="html"></div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { processHtmlForDisplay } from '../../utils/content'
import type { RichTextBlock } from './types'
import { useBlockText } from './util'

const props = defineProps<{ p: RichTextBlock['props'] }>()
const { txt } = useBlockText()
const html = computed(() => processHtmlForDisplay(txt(props.p.html as any)))
</script>
