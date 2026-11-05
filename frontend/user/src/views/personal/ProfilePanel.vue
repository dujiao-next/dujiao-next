<template>
  <div class="rounded-2xl border bg-card p-7 shadow-sm">
    <PanelHeading :title="t('personalCenter.profile.title')" :description="t('personalCenter.profile.subtitle')" :icon="UserCircle">
      <template #actions>
        <Badge variant="accent" size="sm">{{ t('personalCenter.tabs.profile') }}</Badge>
      </template>
    </PanelHeading>

    <Alert v-if="profileAlert" class="mb-5" :variant="pageAlertVariant(profileAlert.level)" :class="pageAlertToneClass(profileAlert.level)">
      <AlertDescription>{{ profileAlert.message }}</AlertDescription>
    </Alert>

    <form class="space-y-6" @submit.prevent="handleSaveProfile">
      <div class="grid grid-cols-1 gap-5 md:grid-cols-2">
        <div class="md:col-span-2">
          <Label class="mb-2 block">{{ t('personalCenter.profile.emailLabel') }}</Label>
          <Input :model-value="userProfileStore.profile?.email || ''" disabled class="h-11" />
        </div>

        <div>
          <Label class="mb-2 block">{{ t('personalCenter.profile.nicknameLabel') }}</Label>
          <Input
            v-model="profileForm.nickname"
            :placeholder="t('personalCenter.profile.nicknamePlaceholder')"
            maxlength="32"
            class="h-11"
          />
        </div>

        <div>
          <Label class="mb-2 block">{{ t('personalCenter.profile.localeLabel') }}</Label>
          <Select v-model="profileForm.locale">
            <SelectTrigger class="h-11 w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="zh-CN">简体中文</SelectItem>
              <SelectItem value="zh-TW">繁體中文</SelectItem>
              <SelectItem value="en-US">English</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>

      <div class="flex flex-col gap-3 border-t pt-5 sm:flex-row sm:items-center sm:justify-between">
        <p class="text-xs text-muted-foreground">{{ t('personalCenter.profile.subtitle') }}</p>
        <Button type="submit" :disabled="userProfileStore.savingProfile" class="h-11 font-bold">
          {{ userProfileStore.savingProfile ? t('personalCenter.profile.saving') : t('personalCenter.profile.save') }}
        </Button>
      </div>
    </form>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { UserCircle } from 'lucide-vue-next'
import { pageAlertVariant, pageAlertToneClass, type PageAlert } from '../../utils/alerts'
import { useUserProfileStore } from '../../stores/userProfile'
import PanelHeading from '../../components/shared/PanelHeading.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

const { t } = useI18n()
const userProfileStore = useUserProfileStore()

const profileForm = reactive({
  nickname: '',
  locale: 'zh-CN',
})

const profileAlert = ref<PageAlert | null>(null)

const handleSaveProfile = async () => {
  profileAlert.value = null
  const payload = {
    nickname: profileForm.nickname.trim(),
    locale: profileForm.locale,
  }
  const ok = await userProfileStore.saveProfile(payload)
  if (!ok) {
    profileAlert.value = {
      level: 'error',
      message: userProfileStore.profileError || t('personalCenter.common.saveFailed'),
    }
    return
  }
  profileAlert.value = {
    level: 'success',
    message: t('personalCenter.profile.saveSuccess'),
  }
}

watch(
  () => userProfileStore.profile,
  (profile) => {
    if (!profile) return
    profileForm.nickname = profile.nickname || ''
    profileForm.locale = profile.locale || 'zh-CN'
  },
  { immediate: true }
)
</script>
