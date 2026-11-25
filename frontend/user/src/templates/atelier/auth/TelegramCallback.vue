<template>
  <div class="flex min-h-[70vh] items-center justify-center px-4 pb-16 pt-[96px]">
    <div class="atelier-card w-full max-w-md rounded-[16px] bg-card p-8 text-center">
      <div v-if="loading" class="space-y-3">
        <div class="mx-auto h-6 w-6 animate-spin rounded-full border-2 border-foreground/20 border-t-foreground"></div>
        <p class="text-sm text-muted-foreground">{{ t('auth.telegramCallback.processing') }}</p>
      </div>
      <div v-else-if="errMsg" class="space-y-4">
        <p class="text-sm font-medium text-danger">{{ errMsg }}</p>
        <RouterLink to="/auth/login" class="theme-pill-link">
          {{ t('auth.telegramCallback.backToLogin') }}
        </RouterLink>
      </div>
    </div>
    <InviteCodeDialog @success="continueAfterLogin" @cancel="cancelInvite" />
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import InviteCodeDialog from '../../../components/InviteCodeDialog.vue'
import { useTelegramCallback } from '../../../composables/useTelegramCallback'

const { t } = useI18n()
const { loading, errMsg, continueAfterLogin, cancelInvite } = useTelegramCallback()
</script>
