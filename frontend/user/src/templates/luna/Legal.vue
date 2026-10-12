<template>
  <div class="mx-auto w-full max-w-[860px] px-6 pb-10 pt-2">
    <div v-if="loading" class="flex justify-center py-20">
      <Loader2 class="h-10 w-10 animate-spin text-primary motion-reduce:animate-none" />
    </div>

    <Card v-else class="mt-7 p-7 sm:p-9">
      <h1 class="mb-6 border-b pb-5 text-3xl font-extrabold">{{ title }}</h1>
      <div
        v-if="content"
        class="prose max-w-none dark:prose-invert prose-a:text-primary prose-img:rounded-md"
        v-html="content"
      ></div>
      <div v-else class="flex flex-col items-center gap-3 py-16 text-center text-muted-foreground">
        <FileText class="h-10 w-10 opacity-60" />
        <p>{{ t('common.noContent') }}</p>
      </div>
    </Card>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { FileText, Loader2 } from 'lucide-vue-next'
import { Card } from '@/components/ui/card'
import { useLegal } from '../../composables/useLegal'

const { t } = useI18n()

const props = defineProps<{
  type: 'terms' | 'privacy'
}>()

const { loading, title, content } = useLegal(() => props.type)
</script>
