<template>
  <div class="space-y-6">
    <div class="rounded-2xl border bg-card p-7 shadow-sm">
      <PanelHeading
        :title="t('personalCenter.security.title')"
        :description="t('personalCenter.security.subtitle')"
        :icon="ShieldCheck"
      >
        <template #actions>
          <Badge variant="accent" size="sm">{{ t('personalCenter.tabs.security') }}</Badge>
        </template>
      </PanelHeading>

      <Alert v-if="securityAlert" class="mb-5" :variant="pageAlertVariant(securityAlert.level)" :class="pageAlertToneClass(securityAlert.level)">
        <AlertDescription>{{ securityAlert.message }}</AlertDescription>
      </Alert>

      <Button variant="outline" class="w-full sm:w-auto" @click="userAuthStore.logout()">
        <LogOut class="h-4 w-4" />
        {{ t('navbar.logout') }}
      </Button>
    </div>

    <LoginHistorySection
      :loading="userProfileStore.loadingLoginLogs"
      :logs="userProfileStore.recentLoginLogs"
    />

    <PasswordChangeForm
      :requires-old-password="requiresOldPassword"
      v-model:old-password="passwordForm.oldPassword"
      v-model:new-password="passwordForm.newPassword"
      v-model:confirm-password="passwordForm.confirmPassword"
      :changing-password="userProfileStore.changingPassword"
      @submit="handleChangePassword"
    />

    <TwoFactorSection />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { LogOut, ShieldCheck } from 'lucide-vue-next'
import { pageAlertVariant, pageAlertToneClass, type PageAlert } from '../../utils/alerts'
import PanelHeading from '../../components/shared/PanelHeading.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { useUserProfileStore } from '../../stores/userProfile'
import { useUserAuthStore } from '../../stores/userAuth'
import LoginHistorySection from '../../components/security/LoginHistorySection.vue'
import PasswordChangeForm from '../../components/security/PasswordChangeForm.vue'
import TwoFactorSection from '../../components/security/TwoFactorSection.vue'

const { t } = useI18n()
const userProfileStore = useUserProfileStore()
const userAuthStore = useUserAuthStore()

const passwordForm = reactive({
  oldPassword: '',
  newPassword: '',
  confirmPassword: '',
})

const securityAlert = ref<PageAlert | null>(null)
const passwordChangeMode = computed(() => userProfileStore.profile?.password_change_mode || 'change_with_old')
const requiresOldPassword = computed(() => passwordChangeMode.value !== 'set_without_old')

const handleChangePassword = async () => {
  securityAlert.value = null
  const oldPassword = passwordForm.oldPassword.trim()
  const newPassword = passwordForm.newPassword.trim()
  const confirmPassword = passwordForm.confirmPassword.trim()
  const needOldPassword = requiresOldPassword.value

  if (!newPassword || !confirmPassword || (needOldPassword && !oldPassword)) {
    securityAlert.value = {
      level: 'warning',
      message: needOldPassword
        ? t('personalCenter.security.changePasswordRequired')
        : t('personalCenter.security.setPasswordRequired'),
    }
    return
  }

  if (newPassword !== confirmPassword) {
    securityAlert.value = {
      level: 'warning',
      message: t('personalCenter.security.passwordMismatch'),
    }
    return
  }

  const ok = await userProfileStore.changePassword({
    ...(needOldPassword ? { old_password: oldPassword } : {}),
    new_password: newPassword,
  })

  if (!ok) {
    securityAlert.value = {
      level: 'error',
      message: userProfileStore.securityError || t('personalCenter.security.changePasswordFailed'),
    }
    return
  }

  passwordForm.oldPassword = ''
  passwordForm.newPassword = ''
  passwordForm.confirmPassword = ''
  userAuthStore.logout('/auth/login?reason=password_changed')
}

onMounted(() => {
  void userProfileStore.loadRecentLoginLogs(10)
})
</script>
