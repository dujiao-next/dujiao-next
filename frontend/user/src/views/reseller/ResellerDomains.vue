<template>
  <div class="space-y-5">
    <ResellerSectionHeader
      :title="t('resellerConsole.domains.title')"
      :description="t('resellerConsole.domains.description')"
    >
      <template #actions>
        <Button type="button" variant="outline" size="sm" @click="load">
          <RotateCw class="h-4 w-4" />
          {{ t('orders.filters.refresh') }}
        </Button>
      </template>
    </ResellerSectionHeader>

    <Alert
      v-if="alert"
      :variant="alert.level === 'error' ? 'destructive' : 'default'"
      :class="alert.level === 'success' ? 'border-success/40 text-success' : ''"
    >
      <AlertDescription>{{ alert.message }}</AlertDescription>
    </Alert>

    <ResellerPageState v-if="loading" loading :title="t('resellerConsole.common.loading')" />

    <template v-else>
      <section class="theme-float overflow-hidden rounded-[22px] bg-card">
        <div class="border-b border-border px-5 py-4 sm:px-6">
          <div class="flex flex-col gap-4 xl:flex-row xl:items-center xl:justify-between">
            <div class="flex min-w-0 items-start gap-3">
              <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
                <Globe2 class="h-5 w-5" />
              </span>
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <h2 class="text-base font-bold text-foreground">{{ t('resellerConsole.domains.workspaceTitle') }}</h2>
                  <ResellerStatusBadge v-if="primaryDomain" :label="t('resellerConsole.domains.primaryBadge')" tone="accent" />
                </div>
                <p class="mt-1 text-sm text-muted-foreground">{{ t('resellerConsole.domains.workspaceDescription') }}</p>
                <div class="mt-2 break-all tabular-nums text-sm font-semibold text-foreground">
                  {{ primaryDomain?.domain || t('resellerConsole.domains.noPrimaryDomain') }}
                </div>
              </div>
            </div>

            <div class="grid overflow-hidden rounded-2xl bg-secondary/60 sm:grid-cols-3">
              <div class="min-w-0 px-4 py-3">
                <div class="text-xs font-semibold text-muted-foreground">{{ t('resellerConsole.domains.primaryTitle') }}</div>
                <div class="mt-1 text-lg font-bold text-foreground">{{ primaryDomain ? 1 : 0 }}</div>
              </div>
              <div class="min-w-0 border-t border-border px-4 py-3 sm:border-l sm:border-t-0">
                <div class="text-xs font-semibold text-muted-foreground">{{ t('resellerConsole.domains.activeTitle') }}</div>
                <div class="mt-1 text-lg font-bold text-success">{{ activeDomains.length }}</div>
              </div>
              <div class="min-w-0 border-t border-border px-4 py-3 sm:border-l sm:border-t-0">
                <div class="text-xs font-semibold text-muted-foreground">{{ t('resellerConsole.domains.reviewTitle') }}</div>
                <div class="mt-1 text-lg font-bold text-warning">{{ pendingDomains.length }}</div>
              </div>
            </div>
          </div>
        </div>

        <div class="divide-y divide-border">
          <section class="grid gap-4 px-5 py-5 sm:px-6 lg:grid-cols-[180px_minmax(0,1fr)]">
            <div class="flex items-start gap-3">
              <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                <Link2 class="h-4 w-4" />
              </span>
              <div>
                <h3 class="text-sm font-bold text-foreground">{{ t('resellerConsole.domains.systemDomainTitle') }}</h3>
                <p class="mt-1 text-xs leading-relaxed text-muted-foreground">
                  {{ systemDomain ? t('resellerConsole.domains.systemAssignedDesc') : t('resellerConsole.domains.systemWaitingDesc') }}
                </p>
              </div>
            </div>

            <div v-if="systemDomain" class="min-w-0">
              <div class="flex flex-col gap-3 xl:flex-row xl:items-start xl:justify-between">
                <div class="min-w-0">
                  <div class="break-all tabular-nums text-base font-bold text-foreground">{{ systemDomain.domain }}</div>
                  <div class="mt-2 flex flex-wrap gap-2">
                    <ResellerStatusBadge :label="domainStatusLabel(systemDomain.status)" :tone="domainTone(systemDomain.status)" dot />
                    <ResellerStatusBadge :label="verificationLabel(systemDomain.verification_status)" :tone="verificationTone(systemDomain.verification_status)" />
                    <ResellerStatusBadge :label="domainTypeLabel(systemDomain.type)" tone="neutral" />
                    <ResellerStatusBadge v-if="systemDomain.is_primary && isActiveVerifiedDomain(systemDomain)" :label="t('personalCenter.reseller.primaryDomain')" tone="accent" />
                  </div>
                  <div class="mt-3 text-xs text-muted-foreground">
                    {{ t('personalCenter.reseller.updatedAt') }} {{ formatResellerConsoleDate(systemDomain.updated_at) }}
                  </div>
                </div>
                <div class="flex shrink-0 flex-wrap items-center gap-2">
                  <ResellerCopyButton :value="systemDomain.domain" :label="t('resellerConsole.common.copy')" />
                  <Button as-child variant="ghost" size="sm">
                    <a :href="`https://${systemDomain.domain}`" target="_blank" rel="noopener noreferrer">
                      <ExternalLink class="h-4 w-4" />
                      {{ t('resellerConsole.domains.visit') }}
                    </a>
                  </Button>
                </div>
              </div>
            </div>

            <div v-else class="rounded-lg border border-dashed border-border bg-muted/20 px-4 py-4">
              <div class="flex items-start gap-3">
                <CircleAlert class="mt-0.5 h-4 w-4 shrink-0 text-warning" />
                <div>
                  <h4 class="text-sm font-semibold text-foreground">{{ t('resellerConsole.domains.systemEmptyTitle') }}</h4>
                  <p class="mt-1 text-sm leading-relaxed text-muted-foreground">{{ t('resellerConsole.domains.systemEmptyDescription') }}</p>
                </div>
              </div>
            </div>
          </section>

          <section class="grid gap-4 px-5 py-5 sm:px-6 lg:grid-cols-[180px_minmax(0,1fr)]">
            <div class="flex items-start gap-3">
              <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                <Plus class="h-4 w-4" />
              </span>
              <div>
                <h3 class="text-sm font-bold text-foreground">{{ t('personalCenter.reseller.customDomainTitle') }}</h3>
                <p class="mt-1 text-xs leading-relaxed text-muted-foreground">{{ t('resellerConsole.domains.submitCustomDesc') }}</p>
              </div>
            </div>

            <div>
              <form v-if="canSubmitDomain" class="grid gap-3 md:grid-cols-[minmax(0,1fr)_auto]" @submit.prevent="submitDomain">
                <Input
                  v-model.trim="domainForm.domain"
                  type="text"
                  :disabled="submitting"
                  :placeholder="t('personalCenter.reseller.customDomainPlaceholder')"
                />
                <Button type="submit" class="md:min-w-32" :disabled="submitting || !domainForm.domain.trim()">
                  {{ submitting ? t('personalCenter.reseller.submittingDomain') : t('personalCenter.reseller.submitDomain') }}
                </Button>
              </form>
              <p v-else class="rounded-lg border border-dashed border-border bg-muted/20 px-4 py-3 text-sm text-muted-foreground">
                {{ t('resellerConsole.domains.submitDisabledDesc') }}
              </p>
              <p class="mt-2 text-xs leading-relaxed text-muted-foreground">{{ t('resellerConsole.domains.verifyDesc') }}</p>
            </div>
          </section>

          <section>
            <div class="flex flex-col gap-2 px-5 py-4 sm:flex-row sm:items-end sm:justify-between sm:px-6">
              <div>
                <h3 class="text-sm font-bold text-foreground">{{ t('resellerConsole.domains.customDomains') }}</h3>
                <p class="mt-1 text-sm text-muted-foreground">{{ t('resellerConsole.domains.customDescription') }}</p>
              </div>
              <ResellerStatusBadge :label="String(customDomains.length)" tone="neutral" />
            </div>

            <div v-if="customDomains.length > 0" class="border-t border-border">
              <div class="hidden grid-cols-[minmax(0,1.3fr)_140px_150px_150px_150px] gap-3 bg-muted/30 px-5 py-3 text-xs font-semibold text-muted-foreground sm:px-6 md:grid">
                <div>{{ t('resellerConsole.domains.tableDomain') }}</div>
                <div>{{ t('resellerConsole.domains.tableStatus') }}</div>
                <div>{{ t('resellerConsole.domains.tableVerification') }}</div>
                <div>{{ t('resellerConsole.domains.tableUpdated') }}</div>
                <div class="text-right">{{ t('resellerConsole.domains.tableActions') }}</div>
              </div>

              <div class="divide-y divide-border">
                <div
                  v-for="item in customDomains"
                  :key="item.id"
                  class="grid gap-3 px-5 py-4 sm:px-6 md:grid-cols-[minmax(0,1.3fr)_140px_150px_150px_150px] md:items-center"
                >
                  <div class="min-w-0">
                    <div class="flex flex-wrap items-center gap-2">
                      <div class="min-w-0 break-all tabular-nums text-sm font-bold text-foreground">{{ item.domain }}</div>
                      <ResellerStatusBadge v-if="item.is_primary && isActiveVerifiedDomain(item)" :label="t('personalCenter.reseller.primaryDomain')" tone="accent" />
                    </div>
                    <div class="mt-1 text-xs text-muted-foreground md:hidden">{{ domainTypeLabel(item.type) }}</div>
                  </div>

                  <div class="flex items-center justify-between gap-3 md:block">
                    <span class="text-xs font-medium text-muted-foreground md:hidden">{{ t('resellerConsole.domains.tableStatus') }}</span>
                    <ResellerStatusBadge :label="domainStatusLabel(item.status)" :tone="domainTone(item.status)" dot />
                  </div>

                  <div class="flex items-center justify-between gap-3 md:block">
                    <span class="text-xs font-medium text-muted-foreground md:hidden">{{ t('resellerConsole.domains.tableVerification') }}</span>
                    <ResellerStatusBadge :label="verificationLabel(item.verification_status)" :tone="verificationTone(item.verification_status)" />
                  </div>

                  <div class="flex items-center justify-between gap-3 text-xs text-muted-foreground md:block">
                    <span class="font-medium md:hidden">{{ t('resellerConsole.domains.tableUpdated') }}</span>
                    <span>{{ formatResellerConsoleDate(item.updated_at) }}</span>
                  </div>

                  <div class="flex flex-wrap items-center gap-2 md:justify-end">
                    <ResellerCopyButton :value="item.domain" :label="t('resellerConsole.common.copy')" />
                    <Button v-if="isActiveVerifiedDomain(item)" as-child variant="ghost" size="sm">
                      <a :href="`https://${item.domain}`" target="_blank" rel="noopener noreferrer">
                        <ExternalLink class="h-4 w-4" />
                        {{ t('resellerConsole.domains.visit') }}
                      </a>
                    </Button>
                  </div>
                </div>
              </div>
            </div>

            <div v-else class="border-t border-border px-5 py-8 text-center sm:px-6">
              <span class="mx-auto flex h-11 w-11 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                <Globe2 class="h-5 w-5" />
              </span>
              <h3 class="mt-3 text-sm font-bold text-foreground">{{ t('personalCenter.reseller.domainEmpty') }}</h3>
              <p class="mx-auto mt-2 max-w-xl text-sm text-muted-foreground">{{ t('resellerConsole.domains.customEmptyDescription') }}</p>
            </div>
          </section>
        </div>
      </section>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleAlert, ExternalLink, Globe2, Link2, Plus, RotateCw } from 'lucide-vue-next'
import { resellerAPI, type ResellerDomainData, type ResellerManagementSnapshotData } from '../../api'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import ResellerCopyButton from '../../components/reseller-console/ResellerCopyButton.vue'
import ResellerPageState from '../../components/reseller-console/ResellerPageState.vue'
import ResellerSectionHeader from '../../components/reseller-console/ResellerSectionHeader.vue'
import ResellerStatusBadge, { type ResellerBadgeTone } from '../../components/reseller-console/ResellerStatusBadge.vue'
import {
  RESELLER_DOMAIN_STATUS_ACTIVE,
  RESELLER_DOMAIN_STATUS_DISABLED,
  RESELLER_DOMAIN_STATUS_PENDING_REVIEW,
  RESELLER_DOMAIN_TYPE_CUSTOM,
  RESELLER_DOMAIN_TYPE_SUBDOMAIN,
  RESELLER_DOMAIN_VERIFICATION_FAILED,
  RESELLER_DOMAIN_VERIFICATION_PENDING,
  RESELLER_DOMAIN_VERIFICATION_VERIFIED,
  RESELLER_PROFILE_STATUS_ACTIVE,
} from '../../constants/reseller'
import { type PageAlert } from '../../utils/alerts'
import { formatResellerConsoleDate } from '../../utils/resellerConsole'
import { getResellerDomainStatusKey } from '../../utils/resellerManagement'

const { t } = useI18n()
const loading = ref(false)
const submitting = ref(false)
const snapshot = ref<ResellerManagementSnapshotData | null>(null)
const alert = ref<PageAlert | null>(null)
const domainForm = reactive({ domain: '' })

const domains = computed(() => snapshot.value?.domains || [])
const canSubmitDomain = computed(() => snapshot.value?.profile?.status === RESELLER_PROFILE_STATUS_ACTIVE)
const isActiveVerifiedDomain = (domain: ResellerDomainData) =>
  domain.status === RESELLER_DOMAIN_STATUS_ACTIVE &&
  domain.verification_status === RESELLER_DOMAIN_VERIFICATION_VERIFIED
const systemDomain = computed(() => domains.value.find((d) => d.type === RESELLER_DOMAIN_TYPE_SUBDOMAIN) || null)
const customDomains = computed(() => domains.value.filter((d) => d.type !== RESELLER_DOMAIN_TYPE_SUBDOMAIN))
const activeDomains = computed(() => domains.value.filter(isActiveVerifiedDomain))
const primaryDomain = computed(() => activeDomains.value.find((d) => d.is_primary) || null)
const pendingDomains = computed(() => domains.value.filter((d) => d.status === RESELLER_DOMAIN_STATUS_PENDING_REVIEW || d.verification_status === RESELLER_DOMAIN_VERIFICATION_PENDING))

const load = async () => {
  loading.value = true
  alert.value = null
  try {
    const response = await resellerAPI.managementProfile()
    snapshot.value = response.data.data || null
  } finally {
    loading.value = false
  }
}

const submitDomain = async () => {
  if (!domainForm.domain.trim()) return
  submitting.value = true
  alert.value = null
  try {
    await resellerAPI.submitDomain({ domain: domainForm.domain.trim() })
    domainForm.domain = ''
    await load()
    alert.value = { level: 'success', message: t('personalCenter.common.saveSuccess') }
  } catch (err) {
    // 透传后端细分错误（域名格式无效 / 不能用主站域名 / 已被占用等）。
    const message = err instanceof Error && err.message ? err.message : t('personalCenter.common.saveFailed')
    alert.value = { level: 'error', message }
  } finally {
    submitting.value = false
  }
}

const domainStatusLabel = (status?: string) => t(`personalCenter.reseller.domainStatus.${getResellerDomainStatusKey(status)}`)

const domainTone = (status?: string): ResellerBadgeTone => {
  if (status === RESELLER_DOMAIN_STATUS_ACTIVE) return 'success'
  if (status === RESELLER_DOMAIN_STATUS_PENDING_REVIEW) return 'warning'
  if (status === RESELLER_DOMAIN_STATUS_DISABLED) return 'neutral'
  return 'neutral'
}

const verificationLabel = (status?: string) => {
  if (status === RESELLER_DOMAIN_VERIFICATION_VERIFIED) return t('personalCenter.reseller.domainVerification.verified')
  if (status === RESELLER_DOMAIN_VERIFICATION_PENDING) return t('personalCenter.reseller.domainVerification.pending')
  if (status === RESELLER_DOMAIN_VERIFICATION_FAILED) return t('personalCenter.reseller.domainVerification.failed')
  return t('personalCenter.reseller.domainVerification.unknown')
}

const verificationTone = (status?: string): ResellerBadgeTone => {
  if (status === RESELLER_DOMAIN_VERIFICATION_VERIFIED) return 'success'
  if (status === RESELLER_DOMAIN_VERIFICATION_PENDING) return 'warning'
  if (status === RESELLER_DOMAIN_VERIFICATION_FAILED) return 'warning'
  return 'neutral'
}

const domainTypeLabel = (type?: string) => {
  if (type === RESELLER_DOMAIN_TYPE_SUBDOMAIN) return t('personalCenter.reseller.domainType.subdomain')
  if (type === RESELLER_DOMAIN_TYPE_CUSTOM) return t('personalCenter.reseller.domainType.custom')
  return type || '-'
}

onMounted(load)
</script>
