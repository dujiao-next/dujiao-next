<template>
  <div class="min-h-[50vh] flex items-center justify-center px-6 py-12 text-center">
    <div v-if="loading" class="text-sm text-muted-foreground">
      {{ t('auth.googleCallback.processing') }}
    </div>
    <div v-else-if="errMsg" class="space-y-3">
      <p class="text-sm text-red-500">{{ errMsg }}</p>
      <RouterLink
        :to="retryPath"
        class="text-sm underline text-muted-foreground transition-colors hover:text-foreground"
      >
        {{ t('auth.googleCallback.back') }}
      </RouterLink>
    </div>
    <InviteCodeDialog @success="continueAfterLogin" @cancel="cancelInvite" />
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import InviteCodeDialog from '../../components/InviteCodeDialog.vue'
import { useGoogleRedirectCallback } from '../../composables/useGoogleRedirectCallback'

const { t } = useI18n()
const { loading, errMsg, retryPath, continueAfterLogin, cancelInvite } = useGoogleRedirectCallback()
</script>
