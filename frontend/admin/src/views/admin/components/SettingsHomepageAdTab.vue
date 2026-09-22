<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import RichEditor from '@/components/RichEditor.vue'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { notifyError, notifySuccess } from '@/utils/notify'

const { t } = useI18n()
const supportedLanguages = ['zh-CN', 'zh-TW', 'en-US'] as const
type SupportedLanguage = (typeof supportedLanguages)[number]
defineProps<{ currentLang: SupportedLanguage }>()
const emit = defineEmits<{ saved: [] }>()
const submitting = ref(false)
const loaded = ref(false)
const createLocalizedField = (): Record<SupportedLanguage, string> => ({ 'zh-CN': '', 'zh-TW': '', 'en-US': '' })
const form = reactive({ enabled: false, title: createLocalizedField(), content: createLocalizedField() })

const fetchAd = async () => {
  try {
    const res = await adminAPI.getHomepageAd()
    const data = res.data?.data as Record<string, unknown> | undefined
    if (data) {
      form.enabled = data.enabled === true
      const title = (data.title || {}) as Record<string, string>
      const content = (data.content || {}) as Record<string, string>
      for (const lang of supportedLanguages) {
        form.title[lang] = title[lang] || ''
        form.content[lang] = content[lang] || ''
      }
    }
  } catch {
    notifyError(t('admin.settings.alerts.loadFailed'))
  } finally {
    loaded.value = true
  }
}

const save = async () => {
  submitting.value = true
  try {
    await adminAPI.updateHomepageAd({ enabled: form.enabled, title: { ...form.title }, content: { ...form.content } })
    notifySuccess(t('admin.settings.alerts.saveSuccess'))
    emit('saved')
  } catch {
    notifyError(t('admin.settings.alerts.saveFailed'))
  } finally {
    submitting.value = false
  }
}

onMounted(fetchAd)
defineExpose({ save, submitting })
</script>

<template>
  <div v-if="loaded" class="rounded-xl border border-border bg-card">
    <div class="border-b border-border bg-muted/40 px-6 py-4">
      <h2 class="text-lg font-semibold">{{ t('admin.settings.homepageAd.title') }}</h2>
      <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.settings.homepageAd.subtitle') }}</p>
    </div>
    <div class="space-y-5 px-6 py-5">
      <div class="flex items-center justify-between gap-4">
        <div>
          <Label class="text-sm font-medium">{{ t('admin.settings.homepageAd.enabled') }}</Label>
          <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.settings.homepageAd.enabledDesc') }}</p>
        </div>
        <Switch v-model="form.enabled" />
      </div>
      <div class="space-y-2">
        <Label>{{ t('admin.settings.homepageAd.adTitle') }} ({{ currentLang }})</Label>
        <Input v-model="form.title[currentLang]" :placeholder="t('admin.settings.homepageAd.adTitlePlaceholder')" />
      </div>
      <div class="space-y-2">
        <Label>{{ t('admin.settings.homepageAd.content') }} ({{ currentLang }})</Label>
        <RichEditor :key="`homepage-ad-${currentLang}`" v-model="form.content[currentLang]" :placeholder="t('admin.settings.homepageAd.contentPlaceholder')" />
      </div>
    </div>
  </div>
</template>
