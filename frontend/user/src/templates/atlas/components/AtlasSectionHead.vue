<template>
  <!-- 参考站的分区头：左「标题 + 一行副标题」，右「发现更多 →」。
       两侧基线对齐，不居中对齐 —— 这是它和常见商城模板最明显的差别。 -->
  <div class="atlas-section-head">
    <div>
      <h2 class="atlas-section-title">{{ title }}</h2>
      <p v-if="subtitle" class="atlas-section-sub">{{ subtitle }}</p>
    </div>
    <RouterLink v-if="moreTo" :to="moreTo" class="atlas-more">
      {{ moreLabel || t('atlas.section.more') }}
      <ChevronRight class="ml-0.5 inline h-3.5 w-3.5 align-[-2px]" />
    </RouterLink>
    <a
      v-else-if="moreHref"
      :href="moreHref"
      target="_blank"
      rel="noopener noreferrer"
      class="atlas-more"
    >
      {{ moreLabel || t('atlas.section.more') }}
      <ChevronRight class="ml-0.5 inline h-3.5 w-3.5 align-[-2px]" />
    </a>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ChevronRight } from 'lucide-vue-next'

defineProps<{
  title: string
  subtitle?: string
  moreTo?: string
  moreHref?: string
  moreLabel?: string
}>()

const { t } = useI18n()
</script>
