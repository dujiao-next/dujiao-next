<template>
  <div class="mx-auto w-full max-w-[1180px] px-4 pb-10 pt-5 sm:px-6">
    <!-- ============ 账户头部 ============ -->
    <header class="mb-5 flex flex-wrap items-center justify-between gap-4 rounded-[var(--r-lg)] border border-border bg-card px-6 py-6">
      <div class="flex min-w-0 items-center gap-4">
        <div class="grid h-14 w-14 flex-none place-items-center rounded-[var(--r)] bg-[color:var(--accent-soft)] text-[22px] font-semibold text-[color:var(--accent-strong)]">
          {{ displayInitial }}
        </div>
        <div class="min-w-0">
          <p class="text-[12px] font-medium uppercase tracking-[0.06em] text-[color:var(--accent)]">{{ t('personalCenter.title') }}</p>
          <h1 class="my-1 truncate text-[22px] font-semibold tracking-[-0.022em]">{{ userProfileStore.displayName }}</h1>
          <p class="truncate text-[13px] text-muted-foreground">{{ userProfileStore.profile?.email || t('personalCenter.subtitle') }}</p>
        </div>
      </div>

      <div class="flex flex-wrap items-center gap-2">
        <span class="ember-pill" :class="emailVerifiedVariant === 'success' ? 'ember-pill--success' : 'ember-pill--warning'">
          {{ emailVerifiedLabel }}
        </span>
        <span v-if="userProfileStore.currentLevel" class="ember-pill ember-pill--accent">
          <img
            v-if="isImagePath(userProfileStore.currentLevel?.icon)"
            :src="getImageUrl(userProfileStore.currentLevel!.icon)"
            class="h-3.5 w-3.5 object-contain"
            alt=""
          />
          <Crown v-else class="h-3.5 w-3.5" />
          {{ levelName(userProfileStore.currentLevel) }}
        </span>
      </div>
    </header>

    <div class="grid items-start gap-5 lg:grid-cols-[236px_minmax(0,1fr)]">
      <!-- ============ 侧栏 ============ -->
      <aside class="min-w-0 lg:sticky lg:top-[92px]">
        <nav class="flex flex-col gap-0.5 rounded-[var(--r-lg)] border border-border bg-card p-2.5 max-[900px]:flex-row max-[900px]:overflow-x-auto">
          <button
            v-for="item in visibleSectionItems"
            :key="item.key"
            type="button"
            class="flex w-full items-center gap-2.5 rounded-[var(--r-sm)] px-3.5 py-2.5 text-left text-[14px] transition-colors max-[900px]:w-auto max-[900px]:whitespace-nowrap"
            :class="currentSection === item.key
              ? 'bg-[color:var(--accent-soft)] font-medium text-[color:var(--accent-strong)]'
              : 'text-muted-foreground hover:bg-[color:var(--surface-2)] hover:text-foreground'"
            @click="switchSection(item.key)"
          >
            <component :is="item.icon" class="h-[17px] w-[17px] flex-none" />
            <span>{{ t(item.label) }}</span>
          </button>
        </nav>
      </aside>

      <!-- ============ 内容 ============ -->
      <section class="flex min-w-0 flex-col gap-4">
        <div
          v-if="globalAlert"
          class="rounded-[var(--r)] px-4 py-3 text-[13px]"
          :class="globalAlert.level === 'error'
            ? 'bg-[color:var(--danger-soft)] text-[color:var(--danger-strong)]'
            : globalAlert.level === 'success'
              ? 'bg-[color:var(--success-soft)] text-[color:var(--success-strong)]'
              : 'bg-[color:var(--warning-soft)] text-[color:var(--warning-strong)]'"
        >{{ globalAlert.message }}</div>

        <!-- ---------- 概览 ---------- -->
        <template v-if="currentSection === 'overview'">
          <div class="grid gap-3 grid-cols-2 lg:grid-cols-4">
            <div class="flex items-center gap-3 rounded-[var(--r-lg)] border border-border bg-card p-4">
              <div class="grid h-10 w-10 flex-none place-items-center rounded-[var(--r-sm)] bg-[color:var(--accent-soft)] text-[color:var(--accent-strong)]">
                <ShoppingBag class="h-[18px] w-[18px]" />
              </div>
              <div class="min-w-0">
                <div class="text-[12px] text-muted-foreground">{{ t('personalCenter.tabs.orders') }}</div>
                <div class="mt-0.5 text-[17px] font-semibold tabular-nums">
                  {{ userProfileStore.loadingOrders ? '—' : userProfileStore.ordersTotal }}
                </div>
              </div>
            </div>

            <div class="flex items-center gap-3 rounded-[var(--r-lg)] border border-border bg-card p-4">
              <div class="grid h-10 w-10 flex-none place-items-center rounded-[var(--r-sm)] bg-[color:var(--accent-soft)] text-[color:var(--accent-strong)]">
                <Crown class="h-[18px] w-[18px]" />
              </div>
              <div class="min-w-0">
                <div class="text-[12px] text-muted-foreground">{{ t('personalCenter.memberLevel.currentLevel') }}</div>
                <div class="mt-0.5 flex items-center gap-1.5 text-[14px] font-semibold">
                  <img
                    v-if="isImagePath(userProfileStore.currentLevel?.icon)"
                    :src="getImageUrl(userProfileStore.currentLevel!.icon)"
                    class="h-3.5 w-3.5 object-contain"
                    alt=""
                  />
                  <span class="truncate">{{ levelName(userProfileStore.currentLevel) }}</span>
                </div>
              </div>
            </div>

            <div class="flex items-center gap-3 rounded-[var(--r-lg)] border border-border bg-card p-4">
              <div class="grid h-10 w-10 flex-none place-items-center rounded-[var(--r-sm)] bg-[color:var(--warning-soft)] text-[color:var(--warning-strong)]">
                <Percent class="h-[18px] w-[18px]" />
              </div>
              <div class="min-w-0">
                <div class="text-[12px] text-muted-foreground">{{ t('personalCenter.memberLevel.discountRate') }}</div>
                <div class="mt-0.5 text-[17px] font-semibold">{{ discountText }}</div>
              </div>
            </div>

            <div class="flex items-center gap-3 rounded-[var(--r-lg)] border border-border bg-card p-4">
              <div class="grid h-10 w-10 flex-none place-items-center rounded-[var(--r-sm)] bg-[color:var(--success-soft)] text-[color:var(--success-strong)]">
                <ShieldCheck class="h-[18px] w-[18px]" />
              </div>
              <div class="min-w-0">
                <div class="text-[12px] text-muted-foreground">{{ t('personalCenter.overview.accountLabel') }}</div>
                <div class="mt-0.5">
                  <span class="ember-pill" :class="emailVerifiedVariant === 'success' ? 'ember-pill--success' : 'ember-pill--warning'">
                    {{ emailVerifiedLabel }}
                  </span>
                </div>
              </div>
            </div>
          </div>

          <!-- 会员等级 -->
          <div v-if="userProfileStore.memberLevels.length > 0" class="rounded-[var(--r-lg)] border border-border bg-card p-5">
            <div class="flex flex-wrap items-center justify-between gap-4">
              <div class="flex items-center gap-3.5">
                <div class="grid h-11 w-11 flex-none place-items-center rounded-[var(--r)] bg-[color:var(--accent-soft)]">
                  <img
                    v-if="isImagePath(userProfileStore.currentLevel?.icon)"
                    :src="getImageUrl(userProfileStore.currentLevel!.icon)"
                    class="h-6 w-6 object-contain"
                    alt=""
                  />
                  <Crown v-else class="h-5 w-5 text-[color:var(--accent-strong)]" />
                </div>
                <div>
                  <p class="text-[12px] uppercase tracking-[0.06em] text-muted-foreground">{{ t('personalCenter.memberLevel.currentLevel') }}</p>
                  <p class="mt-0.5 text-[16px] font-semibold">{{ levelName(userProfileStore.currentLevel) }}</p>
                </div>
              </div>

              <div class="flex flex-wrap items-center gap-2">
                <span class="ember-pill ember-pill--accent">
                  {{ t('personalCenter.memberLevel.discountRate') }}
                  {{ userProfileStore.currentLevel && userProfileStore.currentLevel.discount_rate < 100
                    ? t('personalCenter.memberLevel.discountOff', { n: userProfileStore.currentLevel.discount_rate / 10 })
                    : t('personalCenter.memberLevel.noDiscount') }}
                </span>
                <span v-if="!userProfileStore.nextLevel && userProfileStore.currentLevel" class="ember-pill ember-pill--success">
                  {{ t('personalCenter.memberLevel.highestLevel') }}
                </span>
              </div>
            </div>

            <div v-if="userProfileStore.nextLevel" class="mt-5 rounded-[var(--r)] border border-[color:var(--accent)] bg-[color:var(--accent-soft)] p-4">
              <div class="flex items-center gap-3">
                <div class="grid h-9 w-9 flex-none place-items-center rounded-[var(--r-xs)] bg-card">
                  <img
                    v-if="isImagePath(userProfileStore.nextLevel.icon)"
                    :src="getImageUrl(userProfileStore.nextLevel.icon)"
                    class="h-3.5 w-3.5 object-contain"
                    alt=""
                  />
                  <Crown v-else class="h-4 w-4 text-muted-foreground" />
                </div>
                <div>
                  <p class="text-[12px] uppercase tracking-[0.06em] text-muted-foreground">{{ t('personalCenter.memberLevel.nextLevel') }}</p>
                  <div class="flex items-center gap-2">
                    <span class="text-[13.5px] font-medium text-muted-foreground">{{ levelName(userProfileStore.nextLevel) }}</span>
                    <span v-if="userProfileStore.nextLevel.discount_rate < 100" class="text-[12px] text-[color:var(--accent-strong)]">
                      {{ t('personalCenter.memberLevel.discountOff', { n: userProfileStore.nextLevel.discount_rate / 10 }) }}
                    </span>
                  </div>
                </div>
              </div>

              <div v-if="userProfileStore.upgradeProgress" class="mt-4 flex flex-col gap-3">
                <div v-if="userProfileStore.upgradeProgress.rechargePercent !== null">
                  <div class="mb-1.5 flex justify-between text-[12px]">
                    <span class="text-muted-foreground">{{ t('personalCenter.memberLevel.rechargeProgress') }}</span>
                    <span class="tabular-nums text-muted-foreground">
                      {{ userProfileStore.upgradeProgress.recharged.toFixed(2) }} / {{ userProfileStore.upgradeProgress.rechargeThreshold.toFixed(2) }}
                    </span>
                  </div>
                  <div class="h-1.5 overflow-hidden rounded-full bg-card">
                    <div
                      class="h-full rounded-full bg-[color:var(--accent)] transition-[width] duration-700"
                      :style="{ width: userProfileStore.upgradeProgress.rechargePercent + '%' }"
                    ></div>
                  </div>
                </div>

                <div v-if="userProfileStore.upgradeProgress.spendPercent !== null">
                  <div class="mb-1.5 flex justify-between text-[12px]">
                    <span class="text-muted-foreground">{{ t('personalCenter.memberLevel.spendProgress') }}</span>
                    <span class="tabular-nums text-muted-foreground">
                      {{ userProfileStore.upgradeProgress.spent.toFixed(2) }} / {{ userProfileStore.upgradeProgress.spendThreshold.toFixed(2) }}
                    </span>
                  </div>
                  <div class="h-1.5 overflow-hidden rounded-full bg-card">
                    <div
                      class="h-full rounded-full bg-[color:var(--accent)] transition-[width] duration-700"
                      :style="{ width: userProfileStore.upgradeProgress.spendPercent + '%' }"
                    ></div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- 最近订单 -->
          <div class="rounded-[var(--r-lg)] border border-border bg-card p-5">
            <div class="mb-4 flex items-center justify-between gap-4">
              <h3 class="text-[15px] font-semibold tracking-[-0.015em]">{{ t('personalCenter.overview.recentOrdersTitle') }}</h3>
              <Button as-child variant="ghost" size="sm" class="rounded-full">
                <RouterLink to="/me/orders">{{ t('personalCenter.overview.viewAllOrders') }}</RouterLink>
              </Button>
            </div>

            <div v-if="userProfileStore.loadingOrders" class="flex flex-col gap-2.5">
              <div v-for="idx in 3" :key="idx" class="h-16 rounded-[var(--r)] bg-[color:var(--surface-2)]"></div>
            </div>

            <div v-else-if="userProfileStore.recentOrders.length === 0" class="rounded-[var(--r)] border border-dashed border-border p-4 text-[13px] text-muted-foreground">
              {{ t('personalCenter.overview.emptyOrders') }}
            </div>

            <div v-else class="flex flex-col gap-2.5">
              <div
                v-for="order in userProfileStore.recentOrders"
                :key="order.order_no"
                class="flex flex-wrap items-center justify-between gap-4 rounded-[var(--r)] border border-[color:var(--border-soft)] p-4 transition-all hover:-translate-y-0.5 hover:border-[color:var(--border-strong)] hover:shadow-[var(--shadow-sm)]"
              >
                <div>
                  <div class="text-[12px] text-muted-foreground">{{ t('orders.orderNo') }}：{{ order.order_no }}</div>
                  <div class="my-0.5 text-[15px] font-semibold tabular-nums">{{ formatMoney(order.total_amount, order.currency) }}</div>
                  <div class="text-[12px] text-muted-foreground">{{ formatDate(order.created_at) }}</div>
                </div>
                <div class="flex flex-wrap items-center gap-2.5">
                  <span class="ember-pill" :class="pillClass(order.status)">{{ statusLabel(order.status) }}</span>
                  <Button as-child variant="outline" size="sm" class="rounded-full border-[color:var(--border-strong)]">
                    <RouterLink :to="`/orders/${order.order_no}`">{{ t('orders.viewDetails') }}</RouterLink>
                  </Button>
                  <Button v-if="order.status === 'pending_payment'" as-child size="sm" class="rounded-full">
                    <RouterLink :to="`/pay?order_no=${order.order_no}`">{{ t('orders.payNow') }}</RouterLink>
                  </Button>
                </div>
              </div>
            </div>
          </div>
        </template>

        <!-- ---------- 其余面板：复用既有共享组件 ---------- -->
        <div v-else class="min-w-0">
          <ProfilePanel v-if="currentSection === 'profile'" />
          <SecurityPanel v-else-if="currentSection === 'security'" />
          <OrdersPanel v-else-if="currentSection === 'orders'" />
          <WalletPanel v-else-if="currentSection === 'wallet'" />
          <AffiliatePanel v-else-if="currentSection === 'affiliate'" />

          <div v-else-if="currentSection === 'reseller' && canAccessResellerConsole" class="rounded-[var(--r-lg)] border border-border bg-card p-5">
            <h2 class="text-[15px] font-semibold">{{ t('resellerConsole.title') }}</h2>
            <p class="mt-2 text-[14px] text-muted-foreground">{{ t('resellerConsole.dashboard.description') }}</p>
            <Button as-child size="sm" class="mt-4 rounded-full">
              <RouterLink to="/reseller">{{ t('resellerConsole.nav.dashboard') }}</RouterLink>
            </Button>
          </div>

          <GiftCardPanel v-else-if="currentSection === 'giftCard'" />
          <ApiPanel v-else-if="currentSection === 'api'" />
          <OrdersPanel v-else />
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Crown, Percent, ShieldCheck, ShoppingBag } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { getImageUrl } from '../../utils/image'
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
  emailVerifiedLabel, emailVerifiedVariant, discountText, isImagePath, levelName,
} = usePersonalCenter(() => props.section)

/** 状态徽标配色 */
const pillClass = (status?: string) => {
  const map: Record<string, string> = {
    success: 'ember-pill--success',
    warning: 'ember-pill--warning',
    danger: 'ember-pill--danger',
    info: 'ember-pill--accent',
    accent: 'ember-pill--accent',
    neutral: '',
  }
  return map[statusVariant(status)] || ''
}
</script>
