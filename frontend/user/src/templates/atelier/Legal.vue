<template>
  <div class="flex min-h-screen flex-col text-foreground">
    <div class="mx-auto w-full max-w-3xl flex-1 px-4 pb-16 pt-[88px] sm:px-6">
      <div v-if="loading" class="flex justify-center py-20">
        <Loader2 class="h-7 w-7 animate-spin text-muted-foreground" />
      </div>

      <div v-else class="atelier-card rounded-[16px] bg-card p-6 md:p-10">
        <h1 class="mb-7 border-b pb-5 text-[26px] font-bold leading-tight tracking-[-0.025em] md:text-[34px]">
          {{ title }}
        </h1>

        <div
          v-if="content"
          class="prose prose-gray max-w-none theme-prose dark:prose-invert md:prose-lg prose-headings:font-bold prose-headings:tracking-tight prose-a:no-underline"
          v-html="content"
        ></div>

        <div v-else class="flex flex-col items-center py-16 text-center text-muted-foreground">
          <FileText class="mb-3 h-11 w-11 opacity-60" :stroke-width="1.5" />
          <p class="text-base font-medium">{{ t('common.noContent') }}</p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { FileText, Loader2 } from 'lucide-vue-next'
import { useLegal } from '../../composables/useLegal'

const { t } = useI18n()

const props = defineProps<{
  type: 'terms' | 'privacy'
}>()

const { loading, title, content } = useLegal(() => props.type)
</script>
