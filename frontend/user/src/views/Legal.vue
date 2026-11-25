<template>
  <div class="min-h-screen bg-background text-foreground flex flex-col">

    <!-- Navbar is global -->

    <div class="container mx-auto max-w-3xl flex-1 px-4 pb-16 pt-24">
      <div v-if="loading" class="flex justify-center py-20">
        <Loader2 class="h-8 w-8 animate-spin text-muted-foreground" />
      </div>

      <div v-else class="theme-float rounded-[22px] bg-card p-5 md:rounded-[28px] md:p-10">
        <h1 class="mb-8 border-b pb-6 text-[28px] font-bold leading-tight tracking-tight md:text-[40px]">
          {{ title }}
        </h1>

        <div
          class="prose prose-gray max-w-none dark:prose-invert md:prose-lg prose-headings:font-bold prose-headings:tracking-tight prose-a:no-underline transition-colors theme-prose"
          v-html="content"></div>

        <div v-if="!content" class="text-center text-muted-foreground py-20 flex flex-col items-center">
          <FileText class="w-12 h-12 mb-4 text-muted-foreground" :stroke-width="1.5" />
          <p class="text-lg font-medium">{{ t('common.noContent') }}</p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { FileText, Loader2 } from 'lucide-vue-next'
import { useLegal } from '../composables/useLegal'

const { t } = useI18n()

const props = defineProps<{
  type: 'terms' | 'privacy'
}>()

const { loading, title, content } = useLegal(() => props.type)
</script>

<style scoped>
/* Tailwind Typography (prose) handles formatting */
</style>
