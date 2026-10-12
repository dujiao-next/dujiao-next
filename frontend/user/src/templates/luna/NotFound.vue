<template>
  <div class="flex min-h-[60vh] items-center justify-center px-4 py-12">
    <Card class="w-full max-w-[680px] p-10 text-center shadow-[var(--luna-shadow-lg)] sm:p-12">
      <Badge variant="neutral" size="sm" class="mx-auto rounded-full"><Info class="h-3.5 w-3.5" /> {{ brandSiteName }}</Badge>
      <Frown class="mx-auto mt-4 h-20 w-20 text-muted-foreground opacity-30" />
      <p class="text-[84px] font-extrabold leading-none">404</p>
      <h1 class="mt-3 text-3xl font-extrabold">{{ t('notFoundPage.title') }}</h1>
      <p class="mx-auto mt-3 max-w-prose leading-relaxed text-muted-foreground">{{ t('notFoundPage.description', { site: brandSiteName }) }}</p>

      <div class="mt-7 rounded-md border bg-secondary p-5 text-left">
        <p class="mb-3 text-[11px] uppercase tracking-[0.12em] text-muted-foreground">{{ t('notFoundPage.quickLinksTitle') }}</p>
        <div class="flex flex-wrap gap-2">
          <Button as-child variant="outline" size="sm" class="rounded-full">
            <RouterLink to="/products"><ShoppingBag /> {{ t('nav.products') }}</RouterLink>
          </Button>
          <Button as-child variant="outline" size="sm" class="rounded-full">
            <RouterLink to="/notice"><Bell /> {{ t('nav.notice') }}</RouterLink>
          </Button>
          <Button as-child variant="outline" size="sm" class="rounded-full">
            <RouterLink to="/blog"><BookOpen /> {{ t('nav.blog') }}</RouterLink>
          </Button>
        </div>
      </div>

      <div class="mt-7 flex flex-wrap justify-center gap-3.5">
        <Button as-child class="luna-gradient h-11 rounded-full px-6 font-bold max-[640px]:w-full">
          <RouterLink to="/"><Home /> {{ t('notFoundPage.backHome') }}</RouterLink>
        </Button>
        <Button variant="outline" class="h-11 rounded-full px-6 font-bold max-[640px]:w-full" @click="goBack">
          <ArrowLeft /> {{ t('notFoundPage.backPrevious') }}
        </Button>
      </div>
    </Card>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ArrowLeft, Bell, BookOpen, Frown, Home, Info, ShoppingBag } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { useNotFound } from '../../composables/useNotFound'

const { t } = useI18n()

const { brandSiteName, goBack } = useNotFound()
</script>
