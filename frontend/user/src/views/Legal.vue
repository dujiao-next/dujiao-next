<template>
  <div class="min-h-screen bg-background text-foreground flex flex-col">

    <!-- Navbar is global -->

    <div class="flex-1 container mx-auto px-4 py-20 max-w-4xl relative z-10 mt-12">
      <div v-if="loading" class="flex justify-center py-20">
        <Loader2 class="w-10 h-10 animate-spin text-primary" />
      </div>

      <Card v-else class="backdrop-blur-xl rounded-3xl p-8 md:p-12 shadow-2xl">
        <h1
          class="text-3xl md:text-4xl font-bold text-foreground mb-10 border-b pb-6 tracking-tight">
          {{ title }}
        </h1>

        <div
          class="prose prose-gray dark:prose-invert prose-lg max-w-none prose-headings:font-bold prose-headings:tracking-tight prose-a:no-underline transition-colors theme-prose"
          v-html="sanitizeRichHtml(content)"></div>

        <div v-if="!content" class="text-center text-muted-foreground py-20 flex flex-col items-center">
          <FileText class="w-12 h-12 mb-4 text-muted-foreground" :stroke-width="1.5" />
          <p class="text-lg font-medium">{{ t('common.noContent') }}</p>
        </div>
      </Card>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { FileText, Loader2 } from 'lucide-vue-next'
import { Card } from '@/components/ui/card'
import { useLegal } from '../composables/useLegal'
import { sanitizeRichHtml } from '../utils/richContent'

const { t } = useI18n()

const props = defineProps<{
  type: 'terms' | 'privacy'
}>()

const { loading, title, content } = useLegal(() => props.type)
</script>

<style scoped>
/* Tailwind Typography (prose) handles formatting */
</style>
