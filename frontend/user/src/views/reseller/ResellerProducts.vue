<template>
  <div class="space-y-6">
    <ResellerSectionHeader
      :title="t('resellerConsole.products.title')"
      :description="t('resellerConsole.products.description')"
    >
      <template #actions>
        <Button v-if="primaryDomainUrl" as-child variant="outline" size="sm">
          <a :href="primaryDomainUrl" target="_blank" rel="noopener noreferrer">
            <ExternalLink class="h-4 w-4" />
            {{ t('resellerConsole.products.previewStore') }}
          </a>
        </Button>
        <Button as-child variant="outline" size="sm">
          <RouterLink to="/reseller/orders">
            <ShoppingBag class="h-4 w-4" />
            {{ t('resellerConsole.products.viewOrders') }}
          </RouterLink>
        </Button>
      </template>
    </ResellerSectionHeader>

    <div class="grid gap-4 md:grid-cols-3">
      <Card class="p-5">
        <div class="flex items-center justify-between gap-3">
          <div>
            <p class="text-xs font-semibold text-muted-foreground">{{ t('resellerConsole.products.cards.defaultMarkup') }}</p>
            <div class="mt-2 tabular-nums text-2xl font-bold text-foreground">{{ profile?.default_markup_percent || '0.00' }}%</div>
          </div>
          <span class="flex h-11 w-11 items-center justify-center rounded-xl bg-primary/10 text-primary">
            <Percent class="h-5 w-5" />
          </span>
        </div>
        <p class="mt-3 text-sm text-muted-foreground">{{ t('resellerConsole.products.cards.defaultMarkupDescription') }}</p>
      </Card>
      <Card class="p-5">
        <div class="flex items-center justify-between gap-3">
          <div>
            <p class="text-xs font-semibold text-muted-foreground">{{ t('resellerConsole.products.cards.maxMarkup') }}</p>
            <div class="mt-2 tabular-nums text-2xl font-bold text-foreground">{{ profile?.max_markup_percent || '0.00' }}%</div>
          </div>
          <span class="flex h-11 w-11 items-center justify-center rounded-xl bg-secondary text-foreground">
            <Gauge class="h-5 w-5" />
          </span>
        </div>
        <p class="mt-3 text-sm text-muted-foreground">{{ t('resellerConsole.products.cards.maxMarkupDescription') }}</p>
      </Card>
      <Card class="p-5">
        <div class="flex items-center justify-between gap-3">
          <div>
            <p class="text-xs font-semibold text-muted-foreground">{{ t('resellerConsole.products.cards.storefront') }}</p>
            <div class="mt-2 truncate tabular-nums text-sm font-bold text-foreground">{{ primaryDomain || t('resellerConsole.products.cards.noPrimaryDomain') }}</div>
          </div>
          <span class="flex h-11 w-11 items-center justify-center rounded-xl bg-success/10 text-success">
            <Globe class="h-5 w-5" />
          </span>
        </div>
        <p class="mt-3 text-sm text-muted-foreground">{{ t('resellerConsole.products.cards.storefrontDescription') }}</p>
      </Card>
    </div>

    <Card class="p-5">
      <div class="grid gap-4 lg:grid-cols-3">
        <div class="lg:col-span-1">
          <h2 class="text-sm font-bold text-foreground">{{ t('resellerConsole.products.focusTitle') }}</h2>
          <p class="mt-2 text-sm text-muted-foreground">{{ t('resellerConsole.products.focusDescription') }}</p>
        </div>
        <div class="grid gap-3 sm:grid-cols-3 lg:col-span-2">
          <div v-for="item in ruleHints" :key="item.title" class="rounded-xl border bg-muted/30 p-4">
            <component :is="item.icon" class="h-5 w-5 text-primary" />
            <div class="mt-3 text-sm font-semibold text-foreground">{{ item.title }}</div>
            <p class="mt-1 text-xs text-muted-foreground">{{ item.description }}</p>
          </div>
        </div>
      </div>
    </Card>

    <ResellerProductSettingsPanel :embedded="true" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ExternalLink, Gauge, Globe, Percent, ShoppingBag, SlidersHorizontal, Tag, ToggleLeft } from 'lucide-vue-next'
import type { ResellerDomainData } from '../../api/types'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import ResellerSectionHeader from '../../components/reseller-console/ResellerSectionHeader.vue'
import ResellerProductSettingsPanel from '../personal/ResellerProductSettingsPanel.vue'
import { useResellerProfile } from '../../composables/reseller/useResellerProfile'
import {
  RESELLER_DOMAIN_STATUS_ACTIVE,
  RESELLER_DOMAIN_VERIFICATION_VERIFIED,
} from '../../constants/reseller'

const { t } = useI18n()
const { snapshot, load } = useResellerProfile()

const isActiveVerifiedDomain = (domain: ResellerDomainData) =>
  domain.status === RESELLER_DOMAIN_STATUS_ACTIVE &&
  domain.verification_status === RESELLER_DOMAIN_VERIFICATION_VERIFIED

const profile = computed(() => snapshot.value?.profile || null)
const primaryDomain = computed(() => {
  const domains = snapshot.value?.domains || []
  const activeDomains = domains.filter(isActiveVerifiedDomain)
  return (activeDomains.find((item) => item.is_primary) || activeDomains[0])?.domain || ''
})
const primaryDomainUrl = computed(() => (primaryDomain.value ? `https://${primaryDomain.value}` : ''))

const ruleHints = computed(() => [
  {
    title: t('resellerConsole.products.hints.listing'),
    description: t('resellerConsole.products.hints.listingDescription'),
    icon: ToggleLeft,
  },
  {
    title: t('resellerConsole.products.hints.pricing'),
    description: t('resellerConsole.products.hints.pricingDescription'),
    icon: SlidersHorizontal,
  },
  {
    title: t('resellerConsole.products.hints.sku'),
    description: t('resellerConsole.products.hints.skuDescription'),
    icon: Tag,
  },
])

onMounted(load)
</script>
