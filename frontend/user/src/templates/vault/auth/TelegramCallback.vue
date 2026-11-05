<template>
  <div class="flex min-h-[60vh] items-center justify-center px-4 py-12">
    <Card class="w-full max-w-[420px] p-9 shadow-[var(--shadow-lg)]">
      <div v-if="loading" class="grid justify-items-center gap-4 text-center">
        <span class="h-9 w-9 animate-spin rounded-full border-[3px] border-border border-t-primary motion-reduce:animate-none" aria-hidden="true"></span>
        <p class="text-sm text-muted-foreground">{{ t('auth.telegramCallback.processing') }}</p>
      </div>
      <div v-else-if="errMsg" class="grid justify-items-center gap-4 text-center">
        <AlertCircle class="h-9 w-9 text-destructive" />
        <p class="font-bold text-foreground">{{ errMsg }}</p>
        <Button as-child variant="outline" size="sm" class="rounded-full">
          <RouterLink to="/auth/login">{{ t('auth.telegramCallback.backToLogin') }}</RouterLink>
        </Button>
      </div>
    </Card>
    <InviteCodeDialog @success="continueAfterLogin" @cancel="cancelInvite" />
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import InviteCodeDialog from '../../../components/InviteCodeDialog.vue'
import { AlertCircle } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { useTelegramCallback } from '../../../composables/useTelegramCallback'

const { t } = useI18n()

const { loading, errMsg, continueAfterLogin, cancelInvite } = useTelegramCallback()
</script>
