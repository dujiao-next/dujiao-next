<template>
  <div class="min-h-screen bg-background pb-16 pt-24 text-foreground">
    <div class="container mx-auto px-4">
      <div class="theme-float mx-auto max-w-2xl rounded-[22px] bg-card p-6 text-center md:rounded-[28px] md:p-12">
        <p class="text-7xl font-bold tracking-tighter tabular-nums md:text-8xl">404</p>
        <h1 class="mt-4 text-2xl font-bold tracking-tight md:text-[28px]">
          {{ t('notFoundPage.title') }}
        </h1>
        <p class="mt-3 text-[15px] leading-relaxed text-muted-foreground">
          {{ t('notFoundPage.description', { site: brandSiteName }) }}
        </p>

        <div class="mt-8 rounded-2xl bg-secondary/60 p-4 text-left">
          <p class="mb-3 text-xs font-medium text-muted-foreground">
            {{ t('notFoundPage.quickLinksTitle') }}
          </p>
          <div class="flex flex-wrap gap-2 [&_a]:rounded-full [&_a]:bg-card">
            <Button variant="secondary" size="sm" as-child>
              <RouterLink to="/products">
                <ShoppingBag class="opacity-70" />
                {{ t('nav.products') }}
              </RouterLink>
            </Button>
            <Button variant="secondary" size="sm" as-child>
              <RouterLink to="/notice">
                <Bell class="opacity-70" />
                {{ t('nav.notice') }}
              </RouterLink>
            </Button>
            <Button variant="secondary" size="sm" as-child>
              <RouterLink to="/blog">
                <BookOpen class="opacity-70" />
                {{ t('nav.blog') }}
              </RouterLink>
            </Button>
          </div>
        </div>

        <div class="mt-8 flex flex-col items-center justify-center gap-3 sm:flex-row">
          <Button as-child class="h-12 min-w-[180px] rounded-full bg-foreground font-semibold text-background hover:bg-foreground/85">
            <RouterLink to="/">
              <Home />
              {{ t('notFoundPage.backHome') }}
            </RouterLink>
          </Button>
          <Button type="button" variant="secondary" class="h-12 min-w-[180px] rounded-full font-semibold" @click="goBack">
            <ArrowLeft />
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
import { useNotFound } from '../composables/useNotFound'

const { t } = useI18n()

const { brandSiteName, goBack } = useNotFound()
</script>
