<template>
  <div class="min-h-screen pb-16 pt-[96px] text-foreground">
    <div class="mx-auto w-full max-w-[1240px] px-4 sm:px-6">
      <div class="atelier-card mx-auto max-w-xl rounded-[16px] bg-card p-7 text-center md:p-11">
        <p class="font-mono text-6xl font-bold tracking-[-0.05em] text-foreground md:text-7xl">404</p>
        <h1 class="mt-4 text-2xl font-bold tracking-[-0.025em]">
          {{ t('notFoundPage.title') }}
        </h1>
        <p class="mt-2.5 text-sm leading-relaxed text-muted-foreground">
          {{ t('notFoundPage.description', { site: brandSiteName }) }}
        </p>

        <div class="mt-7 rounded-[16px] border bg-background p-4 text-left">
          <p class="mb-3 text-xs font-semibold tracking-[0.04em] text-muted-foreground uppercase">
            {{ t('notFoundPage.quickLinksTitle') }}
          </p>
          <div class="flex flex-wrap gap-2">
            <RouterLink to="/products" class="theme-pill-link">
              <ShoppingBag class="mr-1 h-3.5 w-3.5 opacity-70" />
              {{ t('nav.products') }}
            </RouterLink>
            <RouterLink to="/notice" class="theme-pill-link">
              <Bell class="mr-1 h-3.5 w-3.5 opacity-70" />
              {{ t('nav.notice') }}
            </RouterLink>
            <RouterLink to="/blog" class="theme-pill-link">
              <BookOpen class="mr-1 h-3.5 w-3.5 opacity-70" />
              {{ t('nav.blog') }}
            </RouterLink>
          </div>
        </div>

        <div class="mt-7 flex flex-col items-center justify-center gap-2.5 sm:flex-row">
          <Button as-child class="h-11 min-w-[168px] rounded-[11px] bg-primary font-semibold text-primary-foreground shadow-xs hover:bg-primary/90">
            <RouterLink to="/">
              <Home class="h-4 w-4" />
              {{ t('notFoundPage.backHome') }}
            </RouterLink>
          </Button>
          <Button type="button" variant="secondary" class="h-11 min-w-[168px] rounded-[11px] font-semibold" @click="goBack">
            <ArrowLeft class="h-4 w-4" />
            {{ t('notFoundPage.backPrevious') }}
          </Button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ArrowLeft, Bell, BookOpen, Home, ShoppingBag } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { useNotFound } from '../../composables/useNotFound'

const { t } = useI18n()
const { brandSiteName, goBack } = useNotFound()
</script>
