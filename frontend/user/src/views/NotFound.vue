<template>
  <div class="min-h-screen bg-background text-foreground pt-20 pb-16">
    <div class="container mx-auto px-4">
      <Card class="max-w-3xl mx-auto mt-12 backdrop-blur-xl rounded-3xl p-8 md:p-12 text-center">
        <Badge variant="neutral" class="rounded-full gap-1.5 px-3 py-1 text-xs">
          <Info class="w-3.5 h-3.5 opacity-60" />
          {{ brandSiteName }}
        </Badge>
        <div class="my-4 flex justify-center">
          <Frown class="w-20 h-20 md:w-24 md:h-24 text-muted-foreground opacity-30" :stroke-width="1" />
        </div>
        <p class="text-7xl md:text-8xl font-black tracking-tight text-foreground">404</p>
        <h1 class="mt-4 text-2xl md:text-4xl font-extrabold text-foreground">
          {{ t('notFoundPage.title') }}
        </h1>
        <p class="mt-4 text-base md:text-lg text-muted-foreground leading-relaxed">
          {{ t('notFoundPage.description', { site: brandSiteName }) }}
        </p>

        <div class="mt-10 flex items-center justify-center">
          <Button as-child class="min-w-[180px] h-12">
            <RouterLink to="/">
              <ShoppingBag />
              {{ t('notFoundPage.backProducts') }}
            </RouterLink>
          </Button>
        </div>
      </Card>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Frown, Info, ShoppingBag } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { useNotFound } from '../composables/useNotFound'

const { t } = useI18n()

const { brandSiteName } = useNotFound()
</script>
