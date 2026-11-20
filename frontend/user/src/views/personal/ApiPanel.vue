<template>
  <div class="space-y-6 api-panel-enter">
    <div class="rounded-2xl border bg-card p-7 shadow-sm">
      <div>
        <div>
          <PanelHeading :title="t('personalCenter.apiPanel.title')" :description="t('personalCenter.apiPanel.subtitle')" :icon="Key">
            <template #actions>
              <Badge variant="accent" size="sm">{{ t('personalCenter.tabs.api') }}</Badge>
            </template>
          </PanelHeading>

          <Alert v-if="panelAlert" class="mb-5" :variant="pageAlertVariant(panelAlert.level)" :class="pageAlertToneClass(panelAlert.level)">
            <AlertDescription>{{ panelAlert.message }}</AlertDescription>
          </Alert>

          <!-- Loading -->
          <div v-if="loading" class="space-y-3">
            <div v-for="idx in 3" :key="idx" class="h-16 animate-pulse rounded-xl border bg-muted"></div>
          </div>

          <!-- No credential / null -->
          <div v-else-if="!credential" class="rounded-xl border border-dashed p-5">
            <p class="text-sm text-muted-foreground">
              {{ t('personalCenter.apiPanel.noCredential') }}
            </p>
            <Button type="button" :disabled="submitting" class="mt-4 font-bold" @click="handleApply">
              {{ submitting ? t('personalCenter.apiPanel.applying') : t('personalCenter.apiPanel.apply') }}
            </Button>
          </div>

          <!-- Pending review -->
          <div v-else-if="credential.status === 'pending_review'" class="rounded-xl border border-dashed p-5">
            <div class="flex items-start gap-3">
              <div class="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-warning/15 text-warning">
                <AlertTriangle class="h-4 w-4" />
              </div>
              <div>
                <h3 class="text-sm font-semibold text-foreground">{{ t('personalCenter.apiPanel.pendingTitle') }}</h3>
                <p class="mt-1 text-sm text-muted-foreground">{{ t('personalCenter.apiPanel.pendingDesc') }}</p>
              </div>
            </div>
          </div>

          <!-- Rejected -->
          <div v-else-if="credential.status === 'rejected'" class="rounded-xl border border-dashed p-5">
            <div class="flex items-start gap-3">
              <div class="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-destructive/15 text-destructive">
                <XCircle class="h-4 w-4" />
              </div>
              <div>
                <h3 class="text-sm font-semibold text-foreground">{{ t('personalCenter.apiPanel.rejectedTitle') }}</h3>
                <p v-if="credential.reject_reason" class="mt-1 text-sm text-muted-foreground">
                  {{ t('personalCenter.apiPanel.rejectReason', { reason: credential.reject_reason }) }}
                </p>
                <Button type="button" :disabled="submitting" class="mt-4 font-bold" @click="handleApply">
                  {{ submitting ? t('personalCenter.apiPanel.applying') : t('personalCenter.apiPanel.reapply') }}
                </Button>
              </div>
            </div>
          </div>

          <!-- Approved / Active -->
          <template v-else>
            <div class="space-y-4">
              <!-- First-time notice: secret never viewed -->
              <div
                v-if="!hasViewedSecret && !newSecret"
                class="rounded-2xl bg-secondary p-4"
              >
                <div class="flex items-start gap-3">
                  <div class="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-info text-white">
                    <Info class="h-4 w-4" />
                  </div>
                  <div class="flex-1">
                    <h3 class="text-sm font-semibold text-info">{{ t('personalCenter.apiPanel.approvedNoticeTitle') }}</h3>
                    <p class="mt-1 text-xs text-info/80">
                      {{ t('personalCenter.apiPanel.approvedNoticeDesc') }}
                    </p>
                    <Button size="sm" class="mt-3" :disabled="submitting" @click="handleFirstGenerate">
                      {{ submitting ? t('personalCenter.apiPanel.regenerating') : t('personalCenter.apiPanel.generateSecret') }}
                    </Button>
                  </div>
                </div>
              </div>

              <!-- API Key -->
              <div class="rounded-2xl bg-secondary/60 p-4">
                <div class="text-xs text-muted-foreground">API Key</div>
                <div class="mt-2 flex flex-wrap items-center gap-2">
                  <span class="rounded-lg bg-card px-2.5 py-1 font-mono text-sm text-foreground break-all">
                    {{ credential.api_key || '-' }}
                  </span>
                  <Button type="button" variant="outline" size="sm" @click="copyToClipboard(credential.api_key || '')">
                    {{ t('personalCenter.apiPanel.copy') }}
                  </Button>
                </div>
              </div>

              <!-- API Secret -->
              <div class="rounded-2xl bg-secondary/60 p-4">
                <div class="text-xs text-muted-foreground">API Secret</div>
                <div class="mt-2 flex flex-wrap items-center gap-2">
                  <span class="rounded-lg bg-card px-2.5 py-1 font-mono text-sm text-foreground">
                    {{ maskedSecret }}
                  </span>
                  <Button type="button" variant="outline" size="sm" :disabled="submitting" @click="handleRegenerate">
                    {{ t('personalCenter.apiPanel.regenerate') }}
                  </Button>
                </div>
                <p class="mt-2 text-xs text-muted-foreground">
                  {{ t('personalCenter.apiPanel.secretHint') }}
                </p>
              </div>

              <!-- Newly generated secret (shown once) -->
              <div
                v-if="newSecret"
                class="rounded-2xl border border-success/25 bg-success/10 p-4 shadow-sm new-secret-burst"
              >
                <div class="flex items-start gap-3">
                  <div class="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-success text-white">
                    <Check class="h-4 w-4" />
                  </div>
                  <div class="flex-1">
                    <h3 class="text-sm font-semibold text-success">{{ t('personalCenter.apiPanel.newSecretTitle') }}</h3>
                    <p class="mt-1 text-xs text-success/80">
                      {{ t('personalCenter.apiPanel.newSecretWarning') }}
                    </p>
                    <div class="mt-3 flex flex-wrap items-center gap-2">
                      <span class="rounded-lg border border-success/30 bg-card px-2.5 py-1 font-mono text-sm text-foreground break-all">
                        {{ newSecret }}
                      </span>
                      <Button size="xs" variant="outline" @click="copyToClipboard(newSecret)">
                        {{ t('personalCenter.apiPanel.copySecret') }}
                      </Button>
                    </div>
                  </div>
                </div>
              </div>

              <!-- Enable / Disable toggle -->
              <div class="rounded-2xl bg-secondary/60 p-4">
                <div class="flex items-center justify-between">
                  <div>
                    <div class="text-sm font-medium text-foreground">{{ t('personalCenter.apiPanel.statusLabel') }}</div>
                    <div class="mt-1 text-xs text-muted-foreground">
                      {{ credential.is_active ? t('personalCenter.apiPanel.statusEnabled') : t('personalCenter.apiPanel.statusDisabled') }}
                    </div>
                  </div>
                  <Switch :model-value="credential.is_active" :disabled="submitting" @update:model-value="handleToggleStatus" />
                </div>
              </div>
            </div>
          </template>

          <!-- Confirm dialog -->
          <Teleport to="body">
            <Transition name="fade">
              <div
                v-if="showConfirm"
                class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm cursor-pointer"
                @click.self="showConfirm = false"
              >
                <div role="dialog" aria-modal="true" class="mx-4 w-full max-w-sm rounded-2xl border bg-card p-6 shadow-2xl">
                  <h3 class="text-base font-bold text-foreground">{{ t('personalCenter.apiPanel.regenerateTitle') }}</h3>
                  <p class="mt-2 text-sm text-muted-foreground">
                    {{ t('personalCenter.apiPanel.regenerateDesc') }}
                  </p>
                  <div class="mt-5 flex justify-end gap-3">
                    <Button type="button" variant="outline" class="font-semibold" @click="showConfirm = false">
                      {{ t('personalCenter.apiPanel.cancel') }}
                    </Button>
                    <Button type="button" variant="destructive" :disabled="submitting" class="font-bold" @click="confirmRegenerate">
                      {{ submitting ? t('personalCenter.apiPanel.regenerating') : t('personalCenter.apiPanel.regenerateConfirm') }}
                    </Button>
                  </div>
                </div>
              </div>
            </Transition>
          </Teleport>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { apiCredentialAPI } from '../../api'
import { pageAlertVariant, pageAlertToneClass, type PageAlert } from '../../utils/alerts'
import { AlertTriangle, XCircle, Info, Check, Key } from 'lucide-vue-next'
import PanelHeading from '../../components/shared/PanelHeading.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'

const { t } = useI18n()

interface CredentialData {
  id: number
  api_key: string
  api_secret_masked?: string
  status: string
  is_active: boolean
  reject_reason?: string
  created_at?: string
  updated_at?: string
}

const loading = ref(true)
const submitting = ref(false)
const credential = ref<CredentialData | null>(null)
const panelAlert = ref<PageAlert | null>(null)
const newSecret = ref('')
const showConfirm = ref(false)
const hasViewedSecret = ref(false)

const maskedSecret = computed(() => {
  if (credential.value?.api_secret_masked) {
    return credential.value.api_secret_masked
  }
  return '••••••••••••'
})

const secretViewedKey = 'api_secret_viewed'

const loadCredential = async () => {
  loading.value = true
  panelAlert.value = null
  try {
    const response = await apiCredentialAPI.getMy()
    const data = response.data.data
    // 后端返回 {status: "none"} 表示无凭证
    if (!data || data.status === 'none') {
      credential.value = null
    } else {
      // 将 api_secret_tail 映射为前端需要的 api_secret_masked 格式
      if (data.api_secret_tail) {
        data.api_secret_masked = '••••••••' + data.api_secret_tail
      }
      credential.value = data
      // 检查用户是否已生成过密钥
      hasViewedSecret.value = localStorage.getItem(secretViewedKey) === String(data.id)
    }
  } catch (err: any) {
    // 404 or no credential is normal
    credential.value = null
  } finally {
    loading.value = false
  }
}

const handleApply = async () => {
  submitting.value = true
  panelAlert.value = null
  try {
    await apiCredentialAPI.apply()
    panelAlert.value = {
      level: 'success',
      message: t('personalCenter.apiPanel.applySuccess'),
    }
    await loadCredential()
  } catch (err: any) {
    panelAlert.value = {
      level: 'error',
      message: err?.message || t('personalCenter.apiPanel.applyFailed'),
    }
  } finally {
    submitting.value = false
  }
}

const handleFirstGenerate = async () => {
  submitting.value = true
  panelAlert.value = null
  try {
    const response = await apiCredentialAPI.regenerate()
    const data = response.data.data
    if (data?.api_secret) {
      newSecret.value = data.api_secret
    }
    if (credential.value) {
      localStorage.setItem(secretViewedKey, String(credential.value.id))
      hasViewedSecret.value = true
    }
    await loadCredential()
    panelAlert.value = {
      level: 'success',
      message: t('personalCenter.apiPanel.generateSuccess'),
    }
  } catch (err: any) {
    panelAlert.value = {
      level: 'error',
      message: err?.message || t('personalCenter.apiPanel.regenerateFailed'),
    }
  } finally {
    submitting.value = false
  }
}

const handleRegenerate = () => {
  newSecret.value = ''
  showConfirm.value = true
}

const confirmRegenerate = async () => {
  submitting.value = true
  panelAlert.value = null
  try {
    const response = await apiCredentialAPI.regenerate()
    const data = response.data.data
    if (data?.api_secret) {
      newSecret.value = data.api_secret
    }
    if (credential.value) {
      localStorage.setItem(secretViewedKey, String(credential.value.id))
      hasViewedSecret.value = true
    }
    // Reload credential to get updated masked secret
    await loadCredential()
    panelAlert.value = {
      level: 'success',
      message: t('personalCenter.apiPanel.regenerateSuccess'),
    }
  } catch (err: any) {
    panelAlert.value = {
      level: 'error',
      message: err?.message || t('personalCenter.apiPanel.regenerateFailed'),
    }
  } finally {
    submitting.value = false
    showConfirm.value = false
  }
}

const handleToggleStatus = async () => {
  if (!credential.value) return
  submitting.value = true
  panelAlert.value = null
  const newStatus = !credential.value.is_active
  try {
    await apiCredentialAPI.updateStatus({ is_active: newStatus })
    credential.value.is_active = newStatus
    panelAlert.value = {
      level: 'success',
      message: newStatus ? t('personalCenter.apiPanel.enabled') : t('personalCenter.apiPanel.disabled'),
    }
  } catch (err: any) {
    panelAlert.value = {
      level: 'error',
      message: err?.message || t('personalCenter.apiPanel.toggleFailed'),
    }
  } finally {
    submitting.value = false
  }
}

const copyToClipboard = async (text: string) => {
  if (!text) return
  try {
    await navigator.clipboard.writeText(text)
    panelAlert.value = {
      level: 'success',
      message: t('personalCenter.apiPanel.copied'),
    }
  } catch {
    panelAlert.value = {
      level: 'error',
      message: t('personalCenter.apiPanel.copyFailed'),
    }
  }
}

onMounted(() => {
  loadCredential()
})
</script>

<style scoped>
.api-panel-enter {
  animation: api-panel-enter 0.45s ease both;
}

.new-secret-burst {
  animation: new-secret-burst 0.45s ease both;
}

@keyframes api-panel-enter {
  from {
    opacity: 0;
    transform: translateY(10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@keyframes new-secret-burst {
  0% {
    opacity: 0;
    transform: translateY(8px) scale(0.98);
  }
  100% {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
