<template>
  <div class="relative min-h-screen overflow-hidden bg-background text-foreground pt-24 pb-16">
    <div class="container relative z-10 mx-auto px-4">
      <header class="relative mb-8 overflow-hidden rounded-3xl border bg-card shadow-sm">
        <div class="relative flex flex-col gap-6 p-6 lg:flex-row lg:items-center lg:justify-between lg:p-8">
          <div class="flex min-w-0 items-center gap-4">
            <div class="flex h-16 w-16 shrink-0 items-center justify-center rounded-2xl bg-primary/10 text-2xl font-black text-primary">
              {{ displayInitial }}
            </div>
            <div class="min-w-0">
              <p class="text-xs font-semibold uppercase tracking-[0.24em] text-primary">
                {{ t('personalCenter.title') }}
              </p>
              <h1 class="mt-1.5 truncate text-2xl font-black text-foreground lg:text-[2rem]">{{ userProfileStore.displayName }}</h1>
              <p class="mt-1 truncate text-sm text-muted-foreground">{{ userProfileStore.profile?.email || t('personalCenter.subtitle') }}</p>
            </div>
          </div>

          <div class="relative flex flex-wrap items-center gap-2">
            <Badge :variant="emailVerifiedVariant" size="sm">{{ emailVerifiedLabel }}</Badge>
            <span
              v-if="userProfileStore.currentLevel"
              class="inline-flex items-center gap-1.5 rounded-full border border-primary/30 bg-primary/10 px-3 py-1 text-xs font-semibold text-primary"
            >
              <img v-if="isImagePath(userProfileStore.currentLevel?.icon)" :src="getImageUrl(userProfileStore.currentLevel!.icon)" class="h-3.5 w-3.5 object-contain" alt="" />
              <span v-else-if="userProfileStore.currentLevel?.icon">{{ userProfileStore.currentLevel.icon }}</span>
              <Crown v-else class="h-3.5 w-3.5" />
              {{ levelName(userProfileStore.currentLevel) }}
            </span>
          </div>
        </div>
      </header>

      <div class="grid grid-cols-1 gap-6 lg:grid-cols-12">
        <aside class="lg:col-span-3">
          <div class="rounded-2xl border bg-card p-4 shadow-sm lg:sticky lg:top-24">
            <div class="hidden flex-col gap-0.5 lg:flex">
              <button
                v-for="item in visibleSectionItems"
                :key="item.key"
                type="button"
                @click="switchSection(item.key)"
                class="group relative flex w-full items-center gap-2.5 rounded-lg py-2.5 pl-4 pr-3 text-left text-sm font-semibold transition-colors"
                :class="currentSection === item.key
                  ? 'bg-primary/10 text-primary'
                  : 'text-muted-foreground hover:bg-accent hover:text-foreground'"
              >
                <span
                  class="absolute left-0 top-1/2 h-5 w-1 -translate-y-1/2 rounded-r-full transition-all"
                  :class="currentSection === item.key ? 'bg-primary' : 'bg-transparent'"
                ></span>
                <component :is="item.icon" class="h-4 w-4 shrink-0" />
                <span class="truncate">{{ t(item.label) }}</span>
              </button>
            </div>

            <div class="lg:hidden">
              <div class="flex gap-1.5 overflow-x-auto pb-1">
                <button
                  v-for="item in visibleSectionItems"
                  :key="item.key"
                  type="button"
                  @click="switchSection(item.key)"
                  class="shrink-0 rounded-lg px-3.5 py-2 text-xs font-semibold transition-colors"
                  :class="currentSection === item.key
                    ? 'bg-primary/10 text-primary'
                    : 'text-muted-foreground hover:bg-accent hover:text-foreground'"
                >
                  <span class="flex items-center gap-1.5">
                    <component :is="item.icon" class="h-3.5 w-3.5" />
                    <span>{{ t(item.label) }}</span>
                  </span>
                </button>
              </div>
            </div>
          </div>
        </aside>

        <section class="space-y-6 lg:col-span-9">
          <Alert
            v-if="globalAlert"
            :variant="pageAlertVariant(globalAlert.level)"
            :class="pageAlertToneClass(globalAlert.level)"
          >
            <AlertDescription>{{ globalAlert.message }}</AlertDescription>
          </Alert>

          <template v-if="currentSection === 'overview'">
            <!-- 数据一览 -->
            <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
              <StatCard
                :label="t('personalCenter.tabs.orders')"
                :value="userProfileStore.loadingOrders ? '—' : userProfileStore.ordersTotal"
                :icon="ShoppingBag"
                tone="info"
                mono
              />
              <StatCard :label="t('personalCenter.memberLevel.currentLevel')" :icon="Crown" tone="accent">
                <template #value>
                  <span class="flex items-center gap-1.5">
                    <img v-if="isImagePath(userProfileStore.currentLevel?.icon)" :src="getImageUrl(userProfileStore.currentLevel!.icon)" class="h-5 w-5 shrink-0 object-contain" alt="" />
                    <span class="truncate">{{ levelName(userProfileStore.currentLevel) }}</span>
                  </span>
                </template>
              </StatCard>
              <StatCard
                :label="t('personalCenter.memberLevel.discountRate')"
                :value="discountText"
                :icon="Percent"
                tone="warning"
              />
              <StatCard
                :label="t('personalCenter.overview.accountLabel')"
                :icon="ShieldCheck"
                :tone="emailVerifiedVariant === 'success' ? 'success' : 'warning'"
              >
                <template #value>
                  <Badge :variant="emailVerifiedVariant" size="sm">{{ emailVerifiedLabel }}</Badge>
                </template>
              </StatCard>
            </div>

            <!-- Member level card -->
            <div v-if="userProfileStore.memberLevels.length > 0" class="rounded-2xl border bg-card p-6 shadow-sm">
              <!-- Current level header -->
              <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
                <div class="flex items-center gap-3.5">
                  <div class="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl border-primary/40 bg-primary/10 text-xl">
                    <img v-if="isImagePath(userProfileStore.currentLevel?.icon)" :src="getImageUrl(userProfileStore.currentLevel!.icon)" class="h-7 w-7 object-contain" alt="" />
                    <span v-else>{{ userProfileStore.currentLevel?.icon || '👤' }}</span>
                  </div>
                  <div class="min-w-0">
                    <p class="text-[11px] font-semibold uppercase tracking-[0.14em] text-muted-foreground">{{ t('personalCenter.memberLevel.currentLevel') }}</p>
                    <p class="mt-0.5 truncate text-lg font-bold text-foreground">{{ levelName(userProfileStore.currentLevel) }}</p>
                  </div>
                </div>
                <div class="flex flex-wrap items-center gap-2">
                  <Badge variant="accent" class="rounded-full px-3 py-1">
                    {{ t('personalCenter.memberLevel.discountRate') }}
                    {{ userProfileStore.currentLevel && userProfileStore.currentLevel.discount_rate < 100
                      ? t('personalCenter.memberLevel.discountOff', { n: userProfileStore.currentLevel.discount_rate / 10 })
                      : t('personalCenter.memberLevel.noDiscount')
                    }}
                  </Badge>
                  <Badge
                    v-if="!userProfileStore.nextLevel && userProfileStore.currentLevel"
                    variant="success"
                    class="rounded-full px-3 py-1"
                  >
                    {{ t('personalCenter.memberLevel.highestLevel') }}
                  </Badge>
                </div>
              </div>

              <!-- Next level upgrade progress -->
              <div v-if="userProfileStore.nextLevel" class="mt-5 rounded-xl border border-primary/15 bg-primary/5 p-4">
                <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                  <!-- Next level info -->
                  <div class="flex items-center gap-3">
                    <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-base opacity-60">
                      <img v-if="isImagePath(userProfileStore.nextLevel.icon)" :src="getImageUrl(userProfileStore.nextLevel.icon)" class="h-6 w-6 object-contain" alt="" />
                      <span v-else>{{ userProfileStore.nextLevel.icon || '⭐' }}</span>
                    </div>
                    <div class="min-w-0">
                      <p class="text-[11px] font-semibold uppercase tracking-[0.14em] text-muted-foreground">{{ t('personalCenter.memberLevel.nextLevel') }}</p>
                      <div class="mt-0.5 flex items-center gap-2">
                        <span class="truncate text-sm font-bold text-muted-foreground">{{ levelName(userProfileStore.nextLevel) }}</span>
                        <span v-if="userProfileStore.nextLevel.discount_rate < 100" class="shrink-0 text-xs font-medium text-primary">
                          {{ t('personalCenter.memberLevel.discountOff', { n: userProfileStore.nextLevel.discount_rate / 10 }) }}
                        </span>
                      </div>
                    </div>
                  </div>
                </div>

                <!-- Progress bars -->
                <div v-if="userProfileStore.upgradeProgress" class="mt-3.5 space-y-3">
                  <div v-if="userProfileStore.upgradeProgress.rechargePercent !== null">
                    <div class="mb-1.5 flex items-center justify-between">
                      <span class="text-xs font-medium text-muted-foreground">{{ t('personalCenter.memberLevel.rechargeProgress') }}</span>
                      <span class="text-xs tabular-nums text-muted-foreground">
                        {{ userProfileStore.upgradeProgress.recharged.toFixed(2) }}
                        <span class="mx-0.5 opacity-40">/</span>
                        {{ userProfileStore.upgradeProgress.rechargeThreshold.toFixed(2) }}
                      </span>
                    </div>
                    <div class="relative h-1.5 w-full overflow-hidden rounded-full bg-muted">
                      <div
                        class="absolute inset-y-0 left-0 rounded-full bg-[var(--ui-accent)] transition-all duration-700 ease-out"
                        :style="{ width: userProfileStore.upgradeProgress.rechargePercent + '%' }"
                      ></div>
                    </div>
                  </div>
                  <div v-if="userProfileStore.upgradeProgress.spendPercent !== null">
                    <div class="mb-1.5 flex items-center justify-between">
                      <span class="text-xs font-medium text-muted-foreground">{{ t('personalCenter.memberLevel.spendProgress') }}</span>
                      <span class="text-xs tabular-nums text-muted-foreground">
                        {{ userProfileStore.upgradeProgress.spent.toFixed(2) }}
                        <span class="mx-0.5 opacity-40">/</span>
                        {{ userProfileStore.upgradeProgress.spendThreshold.toFixed(2) }}
                      </span>
                    </div>
                    <div class="relative h-1.5 w-full overflow-hidden rounded-full bg-muted">
                      <div
                        class="absolute inset-y-0 left-0 rounded-full bg-[var(--ui-accent)] transition-all duration-700 ease-out"
                        :style="{ width: userProfileStore.upgradeProgress.spendPercent + '%' }"
                      ></div>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <div class="rounded-2xl border bg-card p-6 shadow-sm">
              <div class="mb-4 flex flex-wrap items-center justify-between gap-2">
                <h3 class="text-lg font-bold text-foreground">{{ t('personalCenter.overview.recentOrdersTitle') }}</h3>
                <Button as-child variant="ghost" size="sm" class="rounded-full">
                  <router-link to="/me/orders">{{ t('personalCenter.overview.viewAllOrders') }}</router-link>
                </Button>
              </div>

              <div v-if="userProfileStore.loadingOrders" class="space-y-3">
                <div
                  v-for="idx in 3"
                  :key="idx"
                  class="h-16 animate-pulse rounded-xl border bg-muted"
                ></div>
              </div>

              <div v-else-if="userProfileStore.recentOrders.length === 0" class="rounded-xl border border-dashed px-4 py-5 text-sm text-muted-foreground">
                {{ t('personalCenter.overview.emptyOrders') }}
              </div>

              <div v-else class="space-y-3">
                <div
                  v-for="order in userProfileStore.recentOrders"
                  :key="order.order_no"
                  class="rounded-xl border bg-card px-4 py-3 transition-all hover:-translate-y-0.5 hover:border-primary/30 hover:shadow-md"
                >
                  <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                    <div>
                      <div class="text-xs text-muted-foreground">{{ t('orders.orderNo') }}：{{ order.order_no }}</div>
                      <div class="mt-1 text-sm font-semibold text-foreground">
                        {{ formatMoney(order.total_amount, order.currency) }}
                      </div>
                      <div class="mt-1 text-xs text-muted-foreground">{{ formatDate(order.created_at) }}</div>
                    </div>
                    <div class="flex flex-wrap items-center gap-2">
                      <Badge :variant="statusVariant(order.status)" size="sm">
                        {{ statusLabel(order.status) }}
                      </Badge>
                      <Button as-child variant="outline" size="sm">
                        <router-link :to="`/orders/${order.order_no}`">{{ t('orders.viewDetails') }}</router-link>
                      </Button>
                      <Button v-if="order.status === 'pending_payment'" as-child size="sm">
                        <router-link :to="`/pay?order_no=${order.order_no}`">{{ t('orders.payNow') }}</router-link>
                      </Button>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </template>

          <ProfilePanel v-else-if="currentSection === 'profile'" />
          <SecurityPanel v-else-if="currentSection === 'security'" />
          <OrdersPanel v-else-if="currentSection === 'orders'" />
          <WalletPanel v-else-if="currentSection === 'wallet'" />
          <GiftCardPanel v-else-if="currentSection === 'giftCard'" />
          <OrdersPanel v-else />
        </section>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Crown, ShoppingBag, ShieldCheck, Percent } from 'lucide-vue-next'
import { getImageUrl } from '../utils/image'
import { pageAlertVariant, pageAlertToneClass } from '../utils/alerts'
import StatCard from '../components/shared/StatCard.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import ProfilePanel from './personal/ProfilePanel.vue'
import SecurityPanel from './personal/SecurityPanel.vue'
import OrdersPanel from './personal/OrdersPanel.vue'
import WalletPanel from './personal/WalletPanel.vue'
import GiftCardPanel from './personal/GiftCardPanel.vue'

import { usePersonalCenter, type PersonalSection } from '../composables/usePersonalCenter'

const { t } = useI18n()

const props = withDefaults(defineProps<{ section?: PersonalSection }>(), {
  section: 'overview',
})

const {
  userProfileStore, visibleSectionItems, currentSection, globalAlert,
  displayInitial, switchSection, statusLabel, statusVariant, formatMoney, formatDate,
  emailVerifiedLabel, emailVerifiedVariant, discountText, isImagePath, levelName,
} = usePersonalCenter(() => props.section)
</script>
