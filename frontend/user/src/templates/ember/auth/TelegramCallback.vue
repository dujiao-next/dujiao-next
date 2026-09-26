<template>
  <div class="flex min-h-[60vh] items-center justify-center px-4 py-12">
    <div class="w-full max-w-[400px] rounded-[var(--r-lg)] border border-border bg-card p-9 shadow-[var(--shadow-lg)]">
      <div v-if="loading" class="grid justify-items-center gap-4 text-center">
        <span
          class="h-8 w-8 rounded-full border-[3px] border-border border-t-[color:var(--accent)] motion-reduce:animate-none"
          style="animation: ember-spin 720ms linear infinite"
          aria-hidden="true"
        ></span>
        <p class="text-[13.5px] text-muted-foreground">{{ t('auth.telegramCallback.processing') }}</p>
      </div>

      <div v-else-if="errMsg" class="grid justify-items-center gap-4 text-center">
        <AlertCircle class="h-8 w-8 text-[color:var(--danger)]" />
        <p class="text-[14px] font-medium">{{ errMsg }}</p>
        <Button as-child variant="outline" size="sm" class="rounded-full border-[color:var(--border-strong)]">
          <RouterLink to="/auth/login">{{ t('auth.telegramCallback.backToLogin') }}</RouterLink>
        </Button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { AlertCircle } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { useTelegramCallback } from '../../../composables/useTelegramCallback'

const { t } = useI18n()

const { loading, errMsg } = useTelegramCallback()
</script>

<style scoped>
@keyframes ember-spin {
  to { transform: rotate(360deg); }
}
</style>
