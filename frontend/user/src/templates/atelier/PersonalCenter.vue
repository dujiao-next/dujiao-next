<template>
  <div class="min-h-screen pb-16 pt-[88px] text-foreground">
    <div class="mx-auto w-full max-w-[1240px] px-4 sm:px-6">
      <!-- 顶部账户名片头卡 -->
      <header class="atelier-card mb-6 rounded-[16px] bg-card p-5 md:p-7">
        <div class="flex flex-col gap-5 lg:flex-row lg:items-center lg:justify-between">
          <div class="flex min-w-0 items-center gap-4">
            <div class="grid h-14 w-14 shrink-0 place-items-center rounded-[16px] bg-gradient-to-br from-indigo-500 to-cyan-400 text-xl font-bold text-white">
              {{ displayInitial }}
            </div>
            <div class="min-w-0">
              <p class="text-xs font-semibold tracking-[0.04em] text-muted-foreground uppercase">
                {{ t('personalCenter.title') }}
              </p>
              <h1 class="mt-0.5 truncate text-2xl font-bold tracking-[-0.025em] lg:text-[28px]">
                {{ userProfileStore.displayName }}
              </h1>
              <p class="mt-0.5 truncate text-sm text-muted-foreground">
                {{ userProfileStore.profile?.email || t('personalCenter.subtitle') }}
              </p>
            </div>
          </div>

          <div class="flex flex-wrap items-center gap-2">
            <Badge :variant="emailVerifiedVariant" size="sm">{{ emailVerifiedLabel }}</Badge>
            <span
              v-if="userProfileStore.currentLevel"
              class="inline-flex h-6 items-center gap-1.5 rounded-[7px] border bg-background px-2.5 text-xs font-semibold text-foreground"
            >
              <img
                v-if="isImagePath(userProfileStore.currentLevel?.icon)"
                :src="getImageUrl(userProfileStore.currentLevel!.icon)"
                class="h-3.5 w-3.5 object-contain"
                alt=""
              />
              <span v-else-if="userProfileStore.currentLevel?.icon">{{ userProfileStore.currentLevel.icon }}</span>
              <Crown v-else class="h-3.5 w-3.5" />
              {{ levelName(userProfileStore.currentLevel) }}
            </span>
          </div>
        </div>
      </header>

      <!-- 左侧开放式导航 + 右侧面板 -->
      <div class="lg:flex lg:items-start lg:gap-7">
        <aside class="mb-5 lg:sticky lg:top-[84px] lg:mb-0 lg:w-[224px] lg:shrink-0">
          <div class="hidden flex-col gap-1 lg:flex">
            <button
              v-for="item in visibleSectionItems"
              :key="item.key"
              type="button"
              class="flex h-10 w-full items-center gap-2.5 rounded-[10px] px-3 text-left text-[13.5px] transition-all focus-visible:outline-2 focus-visible:outline-primary"
              :class="currentSection === item.key
                ? 'bg-card font-semibold text-foreground shadow-xs ring-1 ring-border'
                : 'font-medium text-muted-foreground hover:bg-secondary hover:text-foreground'"
              :aria-current="currentSection === item.key ? 'page' : undefined"
              @click="switchSection(item.key)"
            >
              <component :is="item.icon" class="h-4 w-4 shrink-0 opacity-75" />
              <span class="truncate">{{ t(item.label) }}</span>
            </button>
          </div>

          <!-- 移动端横向胶囊选项卡 -->
          <div class="flex gap-1.5 overflow-x-auto pb-1 scrollbar-hide lg:hidden">
            <button
              v-for="item in visibleSectionItems"
              :key="item.key"
              type="button"
              class="inline-flex h-9 shrink-0 items-center gap-1.5 rounded-full border px-3.5 text-xs font-semibold transition-colors"
              :class="currentSection === item.key
                ? 'border-primary bg-primary text-primary-foreground'
                : 'bg-card text-muted-foreground hover:text-foreground'"
              @click="switchSection(item.key)"
            >
              <component :is="item.icon" class="h-3.5 w-3.5" />
              <span>{{ t(item.label) }}</span>
            </button>
          </div>
        </aside>

        <section class="at-pc-panels min-w-0 flex-1 space-y-5">
          <Alert
            v-if="globalAlert"
            :variant="pageAlertVariant(globalAlert.level)"
            :class="pageAlertToneClass(globalAlert.level)"
          >
            <AlertDescription>{{ globalAlert.message }}</AlertDescription>
          </Alert>

          <template v-if="currentSection === 'overview'">
            <!-- 核心指标卡 -->
            <div class="grid grid-cols-2 gap-3.5 lg:grid-cols-4">
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
                    <img
                      v-if="isImagePath(userProfileStore.currentLevel?.icon)"
                      :src="getImageUrl(userProfileStore.currentLevel!.icon)"
                      class="h-5 w-5 shrink-0 object-contain"
                      alt=""
                    />
                    <span class="truncate">{{ levelName(userProfileStore.currentLevel) }}</span>
                  </span>
                </template>
              </StatCard>
              <StatCard :label="t('personalCenter.memberLevel.discountRate')" :value="discountText" :icon="Percent" tone="warning" />
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

            <!-- 会员等级与升级进度 -->
            <div v-if="userProfileStore.memberLevels.length > 0" class="atelier-card rounded-[16px] bg-card p-5 md:p-6">
              <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
                <div class="flex items-center gap-3.5">
                  <div class="grid h-11 w-11 shrink-0 place-items-center rounded-[12px] border bg-background text-xl">
                    <img
                      v-if="isImagePath(userProfileStore.currentLevel?.icon)"
                      :src="getImageUrl(userProfileStore.currentLevel!.icon)"
                      class="h-7 w-7 object-contain"
                      alt=""
                    />
                    <span v-else>{{ userProfileStore.currentLevel?.icon || '👤' }}</span>
                  </div>
                  <div class="min-w-0">
                    <p class="text-xs font-medium text-muted-foreground">{{ t('personalCenter.memberLevel.currentLevel') }}</p>
                    <p class="mt-0.5 truncate text-base font-bold">{{ levelName(userProfileStore.currentLevel) }}</p>
                  </div>
                </div>
                <div class="flex flex-wrap items-center gap-2">
                  <Badge variant="accent" class="rounded-full px-3 py-1">
                    {{ t('personalCenter.memberLevel.discountRate') }}
                    {{ userProfileStore.currentLevel && userProfileStore.currentLevel.discount_rate < 100
                      ? t('personalCenter.memberLevel.discountOff', { n: userProfileStore.currentLevel.discount_rate / 10 })
                      : t('personalCenter.memberLevel.noDiscount') }}
                  </Badge>
                  <Badge v-if="!userProfileStore.nextLevel && userProfileStore.currentLevel" variant="success" class="rounded-full px-3 py-1">
                    {{ t('personalCenter.memberLevel.highestLevel') }}
                  </Badge>
                </div>
              </div>

              <div v-if="userProfileStore.nextLevel" class="mt-5 rounded-[16px] border bg-background p-4">
                <div class="flex items-center gap-3">
                  <div class="grid h-9 w-9 shrink-0 place-items-center rounded-[10px] border bg-card text-base opacity-70">
                    <img
                      v-if="isImagePath(userProfileStore.nextLevel.icon)"
                      :src="getImageUrl(userProfileStore.nextLevel.icon)"
                      class="h-5 w-5 object-contain"
                      alt=""
                    />
                    <span v-else>{{ userProfileStore.nextLevel.icon || '⭐' }}</span>
                  </div>
                  <div class="min-w-0">
                    <p class="text-xs text-muted-foreground">{{ t('personalCenter.memberLevel.nextLevel') }}</p>
                    <div class="mt-0.5 flex items-center gap-2">
                      <span class="truncate text-sm font-bold text-foreground">{{ levelName(userProfileStore.nextLevel) }}</span>
                      <span v-if="userProfileStore.nextLevel.discount_rate < 100" class="shrink-0 text-xs font-semibold text-success">
                        {{ t('personalCenter.memberLevel.discountOff', { n: userProfileStore.nextLevel.discount_rate / 10 }) }}
                      </span>
                    </div>
                  </div>
                </div>

                <div v-if="userProfileStore.upgradeProgress" class="mt-4 space-y-3">
                  <div v-if="userProfileStore.upgradeProgress.rechargePercent !== null">
                    <div class="mb-1.5 flex items-center justify-between text-xs text-muted-foreground">
                      <span>{{ t('personalCenter.memberLevel.rechargeProgress') }}</span>
                      <span class="font-mono tabular-nums">
                        {{ userProfileStore.upgradeProgress.recharged.toFixed(2) }} / {{ userProfileStore.upgradeProgress.rechargeThreshold.toFixed(2) }}
                      </span>
                    </div>
                    <div class="relative h-1.5 w-full overflow-hidden rounded-full bg-secondary">
                      <div
                        class="absolute inset-y-0 left-0 rounded-full bg-primary transition-all duration-700"
                        :style="{ width: userProfileStore.upgradeProgress.rechargePercent + '%' }"
                      ></div>
                    </div>
                  </div>
                  <div v-if="userProfileStore.upgradeProgress.spendPercent !== null">
                    <div class="mb-1.5 flex items-center justify-between text-xs text-muted-foreground">
                      <span>{{ t('personalCenter.memberLevel.spendProgress') }}</span>
                      <span class="font-mono tabular-nums">
                        {{ userProfileStore.upgradeProgress.spent.toFixed(2) }} / {{ userProfileStore.upgradeProgress.spendThreshold.toFixed(2) }}
                      </span>
                    </div>
                    <div class="relative h-1.5 w-full overflow-hidden rounded-full bg-secondary">
                      <div
                        class="absolute inset-y-0 left-0 rounded-full bg-primary transition-all duration-700"
                        :style="{ width: userProfileStore.upgradeProgress.spendPercent + '%' }"
                      ></div>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <!-- 我的专属价 -->
            <div v-if="exclusivePriceItems.length > 0" class="atelier-card rounded-[16px] bg-card p-5 md:p-6">
              <div class="mb-1 flex items-center gap-2">
                <BadgePercent class="h-4.5 w-4.5 text-muted-foreground" />
                <h3 class="text-base font-bold">{{ t('personalCenter.exclusivePrice.title') }}</h3>
                <span v-if="hasMoreExclusivePrices" class="font-mono text-xs text-muted-foreground">
                  {{ t('personalCenter.exclusivePrice.total', { n: exclusivePriceItems.length }) }}
                </span>
              </div>
              <p class="mb-4 text-xs text-muted-foreground">{{ t('personalCenter.exclusivePrice.hint') }}</p>
              <div class="divide-y rounded-[14px] border bg-background">
                <div
                  v-for="item in visibleExclusivePriceItems"
                  :key="item.key"
                  class="flex flex-col gap-2 px-4 py-3 sm:flex-row sm:items-center sm:justify-between"
                >
                  <div class="min-w-0">
                    <RouterLink v-if="item.to" :to="item.to" class="block truncate text-sm font-semibold text-foreground hover:underline">
                      {{ item.title }}
                    </RouterLink>
                    <span v-else class="block truncate text-sm font-semibold text-muted-foreground">{{ item.title }}</span>
                    <div v-if="item.skuText" class="mt-0.5 truncate text-xs text-muted-foreground">{{ item.skuText }}</div>
                    <div v-if="item.unavailable" class="mt-0.5 text-xs text-muted-foreground">{{ t('personalCenter.exclusivePrice.unavailable') }}</div>
                  </div>
                  <div class="shrink-0 text-base font-bold tabular-nums text-foreground">{{ item.priceText }}</div>
                </div>
              </div>
              <div v-if="hasMoreExclusivePrices" class="mt-3 text-center">
                <Button variant="ghost" size="sm" class="rounded-full" @click="toggleExclusivePrices">
                  {{ exclusivePriceExpanded
                    ? t('personalCenter.exclusivePrice.collapse')
                    : t('personalCenter.exclusivePrice.showAll', { n: exclusivePriceItems.length }) }}
                </Button>
              </div>
            </div>

            <!-- 近期订单 -->
            <div class="atelier-card rounded-[16px] bg-card p-5 md:p-6">
              <div class="mb-4 flex flex-wrap items-center justify-between gap-2 border-b pb-3">
                <h3 class="text-base font-bold">{{ t('personalCenter.overview.recentOrdersTitle') }}</h3>
                <RouterLink to="/me/orders" class="theme-pill-link">
                  {{ t('personalCenter.overview.viewAllOrders') }}<ChevronRight class="h-3.5 w-3.5" aria-hidden="true" />
                </RouterLink>
              </div>

              <div v-if="userProfileStore.loadingOrders" class="space-y-3">
                <div v-for="idx in 3" :key="idx" class="theme-skeleton h-16 rounded-[14px]"></div>
              </div>

              <div v-else-if="userProfileStore.recentOrders.length === 0" class="rounded-[14px] border border-dashed px-4 py-6 text-center text-sm text-muted-foreground">
                {{ t('personalCenter.overview.emptyOrders') }}
              </div>

              <div v-else class="divide-y rounded-[16px] border bg-background">
                <div
                  v-for="order in userProfileStore.recentOrders"
                  :key="order.order_no"
                  class="flex flex-col justify-between gap-3 px-4 py-3.5 transition-colors hover:bg-card sm:flex-row sm:items-center"
                >
                  <div>
                    <div class="font-mono text-xs text-muted-foreground">{{ t('orders.orderNo') }}：{{ order.order_no }}</div>
                    <div class="mt-1 text-base font-bold tabular-nums text-foreground">
                      {{ formatMoney(order.total_amount, order.currency) }}
                    </div>
                    <div class="mt-0.5 font-mono text-xs text-muted-foreground">{{ formatDate(order.created_at) }}</div>
                  </div>
                  <div class="flex flex-wrap items-center gap-2">
                    <Badge :variant="statusVariant(order.status)" size="sm">{{ statusLabel(order.status) }}</Badge>
                    <RouterLink :to="`/orders/${order.order_no}`" class="theme-pill-link">
                      {{ t('orders.viewDetails') }}<ChevronRight class="h-3.5 w-3.5" aria-hidden="true" />
                    </RouterLink>
                    <Button v-if="order.status === 'pending_payment'" as-child size="sm" class="rounded-[9px] bg-primary text-primary-foreground hover:bg-primary/90">
                      <RouterLink :to="`/pay?order_no=${order.order_no}`">{{ t('orders.payNow') }}</RouterLink>
                    </Button>
                  </div>
                </div>
              </div>
            </div>
          </template>

          <ProfilePanel v-else-if="currentSection === 'profile'" />
          <SecurityPanel v-else-if="currentSection === 'security'" />
          <OrdersPanel v-else-if="currentSection === 'orders'" />
          <WalletPanel v-else-if="currentSection === 'wallet'" />
          <AffiliatePanel v-else-if="currentSection === 'affiliate'" />
          <div v-else-if="currentSection === 'reseller' && canAccessResellerConsole" class="atelier-card rounded-[16px] bg-card p-6">
            <h2 class="text-lg font-bold text-foreground">{{ t('resellerConsole.title') }}</h2>
            <p class="mt-2 text-sm text-muted-foreground">{{ t('resellerConsole.dashboard.description') }}</p>
            <Button as-child class="mt-5 rounded-[11px] bg-primary font-semibold text-primary-foreground shadow-xs hover:bg-primary/90">
              <RouterLink to="/reseller">{{ t('resellerConsole.nav.dashboard') }}</RouterLink>
            </Button>
          </div>
          <GiftCardPanel v-else-if="currentSection === 'giftCard'" />
          <ApiPanel v-else-if="currentSection === 'api'" />
          <OrdersPanel v-else />
        </section>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { BadgePercent, ChevronRight, Crown, Percent, ShieldCheck, ShoppingBag } from 'lucide-vue-next'
import { getImageUrl } from '../../utils/image'
import { pageAlertVariant, pageAlertToneClass } from '../../utils/alerts'
import StatCard from '../../components/shared/StatCard.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import ProfilePanel from '../../views/personal/ProfilePanel.vue'
import SecurityPanel from '../../views/personal/SecurityPanel.vue'
import OrdersPanel from '../../views/personal/OrdersPanel.vue'
import WalletPanel from '../../views/personal/WalletPanel.vue'
import GiftCardPanel from '../../views/personal/GiftCardPanel.vue'
import AffiliatePanel from '../../views/personal/AffiliatePanel.vue'
import ApiPanel from '../../views/personal/ApiPanel.vue'
import { usePersonalCenter, type PersonalSection } from '../../composables/usePersonalCenter'

const { t } = useI18n()

const props = withDefaults(defineProps<{ section?: PersonalSection }>(), {
  section: 'overview',
})

const {
  userProfileStore, canAccessResellerConsole, visibleSectionItems, currentSection, globalAlert,
  displayInitial, switchSection, statusLabel, statusVariant, formatMoney, formatDate,
  emailVerifiedLabel, emailVerifiedVariant, discountText, isImagePath, levelName, exclusivePriceItems,
  visibleExclusivePriceItems, hasMoreExclusivePrices, exclusivePriceExpanded, toggleExclusivePrices,
} = usePersonalCenter(() => props.section)
</script>

<style scoped>
.scrollbar-hide {
  -ms-overflow-style: none;
  scrollbar-width: none;
}
.scrollbar-hide::-webkit-scrollbar {
  display: none;
}

/* 共享个人中心子面板的卡片统一对齐 Prism 的 1px 细描边 + 16px 精准圆角 */
.at-pc-panels :deep(.rounded-2xl.border.bg-card),
.at-pc-panels :deep(.rounded-xl.border.bg-card.shadow) {
  border: 1px solid var(--at-line);
  border-radius: 16px;
  box-shadow: var(--at-shadow-card);
}
</style>
