<template>
  <div class="mx-auto flex w-full max-w-[520px] flex-col items-center gap-5 px-4 py-24 text-center sm:px-6">
    <span class="select-none text-[clamp(4rem,14vw,7rem)] font-semibold leading-none tracking-[-0.06em] text-[color:var(--accent)] opacity-20">
      404
    </span>
    <h1 class="text-[clamp(1.5rem,1.15rem+1.5vw,2.25rem)] font-semibold tracking-[-0.022em]">
      {{ t('ember.notFound.title') }}
    </h1>
    <p class="max-w-[42ch] text-[15px] leading-[1.65] text-muted-foreground">
      {{ t('ember.notFound.desc') }}
    </p>
    <div class="flex flex-wrap justify-center gap-3">
      <Button as-child size="lg" class="h-12 rounded-full px-6 text-[15px] font-medium">
        <RouterLink to="/">{{ t('ember.notFound.back') }}</RouterLink>
      </Button>
      <Button as-child variant="outline" size="lg" class="h-12 rounded-full border-[color:var(--border-strong)] px-6 text-[15px] font-medium">
        <RouterLink to="/products">{{ t('products.allCategories') }}</RouterLink>
      </Button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'

const { t } = useI18n()
</script>
