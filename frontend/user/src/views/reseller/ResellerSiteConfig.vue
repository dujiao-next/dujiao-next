<template>
  <div class="space-y-6">
    <ResellerSectionHeader
      :title="t('resellerConsole.site.title')"
      :description="t('resellerConsole.site.description')"
    >
      <template #actions>
        <Button v-if="primaryDomainUrl" as-child variant="outline" size="sm">
          <a :href="primaryDomainUrl" target="_blank" rel="noopener noreferrer">
            <ExternalLink class="h-4 w-4" />
            {{ t('resellerConsole.site.previewStore') }}
          </a>
        </Button>
        <Button type="button" variant="outline" size="sm" :disabled="loading" @click="initialize">
          <RotateCw class="h-4 w-4" />
          {{ t('orders.filters.refresh') }}
        </Button>
      </template>
    </ResellerSectionHeader>

    <div class="grid gap-4 md:grid-cols-3">
      <Card class="p-5">
        <div class="flex items-center justify-between gap-3">
          <div>
            <p class="text-xs font-semibold text-muted-foreground">{{ t('resellerConsole.site.cards.brand') }}</p>
            <div class="mt-2 truncate text-lg font-bold text-foreground">{{ siteName || t('resellerConsole.site.cards.noSiteName') }}</div>
          </div>
          <span class="flex h-11 w-11 items-center justify-center rounded-xl bg-primary/10 text-primary">
            <Palette class="h-5 w-5" />
          </span>
        </div>
        <p class="mt-3 text-sm text-muted-foreground">{{ t('resellerConsole.site.cards.brandDescription') }}</p>
      </Card>
      <Card class="p-5">
        <div class="flex items-center justify-between gap-3">
          <div>
            <p class="text-xs font-semibold text-muted-foreground">{{ t('resellerConsole.site.cards.support') }}</p>
            <div class="mt-2 text-lg font-bold text-foreground">{{ supportChannelsReady }}/4</div>
          </div>
          <span class="flex h-11 w-11 items-center justify-center rounded-xl bg-secondary text-foreground">
            <LifeBuoy class="h-5 w-5" />
          </span>
        </div>
        <p class="mt-3 text-sm text-muted-foreground">{{ t('resellerConsole.site.cards.supportDescription') }}</p>
      </Card>
      <Card class="p-5">
        <div class="flex items-center justify-between gap-3">
          <div>
            <p class="text-xs font-semibold text-muted-foreground">{{ t('resellerConsole.site.cards.domain') }}</p>
            <div class="mt-2 truncate tabular-nums text-sm font-bold text-foreground">{{ primaryDomain || t('resellerConsole.site.cards.noPrimaryDomain') }}</div>
          </div>
          <span class="flex h-11 w-11 items-center justify-center rounded-xl bg-success/10 text-success">
            <Globe class="h-5 w-5" />
          </span>
        </div>
        <p class="mt-3 text-sm text-muted-foreground">{{ t('resellerConsole.site.cards.domainDescription') }}</p>
      </Card>
    </div>

    <Card class="p-5">
      <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <h2 class="text-sm font-bold text-foreground">{{ t('resellerConsole.site.readiness.title') }}</h2>
          <p class="mt-1 text-sm text-muted-foreground">{{ t('resellerConsole.site.readiness.description') }}</p>
        </div>
        <div class="grid gap-2 sm:grid-cols-4 lg:min-w-[560px]">
          <div v-for="item in readiness" :key="item.label" class="rounded-xl border bg-muted/30 p-3">
            <ResellerStatusBadge :label="item.done ? t('resellerConsole.site.readiness.configured') : t('resellerConsole.site.readiness.pending')" :tone="item.done ? 'success' : 'warning'" />
            <div class="mt-2 text-sm font-semibold text-foreground">{{ item.label }}</div>
          </div>
        </div>
      </div>
    </Card>

    <ResellerSiteConfigPanel :embedded="true" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ExternalLink, Globe, LifeBuoy, Palette, RotateCw } from 'lucide-vue-next'
import { resellerAPI } from '../../api'
import type { ResellerDomainData, ResellerSiteConfigSnapshotData } from '../../api/types'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import ResellerSectionHeader from '../../components/reseller-console/ResellerSectionHeader.vue'
import ResellerStatusBadge from '../../components/reseller-console/ResellerStatusBadge.vue'
import ResellerSiteConfigPanel from '../../components/reseller/ResellerSiteConfigPanel.vue'
import { useResellerProfile } from '../../composables/reseller/useResellerProfile'
import {
  RESELLER_DOMAIN_STATUS_ACTIVE,
  RESELLER_DOMAIN_VERIFICATION_VERIFIED,
} from '../../constants/reseller'
import { isResellerSiteSeoConfigured } from '../../utils/resellerSiteConfig'

const { t } = useI18n()
const { snapshot: profileSnapshot, load: loadProfile } = useResellerProfile()
const loading = ref(false)
const siteSnapshot = ref<ResellerSiteConfigSnapshotData | null>(null)

const isActiveVerifiedDomain = (domain: ResellerDomainData) =>
  domain.status === RESELLER_DOMAIN_STATUS_ACTIVE &&
  domain.verification_status === RESELLER_DOMAIN_VERIFICATION_VERIFIED

const primaryDomain = computed(() => {
  const domains = profileSnapshot.value?.domains || []
  const activeDomains = domains.filter(isActiveVerifiedDomain)
  return (activeDomains.find((item) => item.is_primary) || activeDomains[0])?.domain || ''
})
const primaryDomainUrl = computed(() => (primaryDomain.value ? `https://${primaryDomain.value}` : ''))
const config = computed(() => siteSnapshot.value?.config || null)
const siteName = computed(() => config.value?.site_name || '')
const supportChannelsReady = computed(() => {
  const support = config.value?.support || {}
  return ['telegram', 'whatsapp', 'email', 'support_url'].filter((key) => Boolean((support as Record<string, string | undefined>)[key])).length
})
const readiness = computed(() => [
  { label: t('resellerConsole.site.readiness.brand'), done: Boolean(config.value?.site_name || config.value?.logo || config.value?.favicon) },
  { label: t('resellerConsole.site.readiness.support'), done: supportChannelsReady.value > 0 },
  { label: t('resellerConsole.site.readiness.seo'), done: isResellerSiteSeoConfigured(config.value?.seo) },
  { label: t('resellerConsole.site.readiness.announcement'), done: Boolean(config.value?.announcement?.enabled) },
])

const loadSiteConfig = async () => {
  const response = await resellerAPI.siteConfig()
  siteSnapshot.value = response.data.data || null
}

const initialize = async () => {
  loading.value = true
  try {
    await Promise.all([loadProfile(), loadSiteConfig()])
  } finally {
    loading.value = false
  }
}

onMounted(initialize)
</script>
