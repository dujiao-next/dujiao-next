<template>
  <div class="mx-auto w-full max-w-[1180px] px-6 pb-10 pt-[22px]">
    <!-- 账户头部 -->
    <header class="mb-[22px] flex flex-wrap items-center justify-between gap-[18px] rounded-xl border bg-card px-7 py-[26px]">
      <div class="flex min-w-0 items-center gap-4">
        <div class="grid h-[62px] w-[62px] flex-none place-items-center rounded-md bg-primary/10 text-[26px] font-extrabold text-primary">{{ displayInitial }}</div>
        <div class="min-w-0">
          <p class="text-[13px] font-bold uppercase tracking-[0.04em] text-primary">{{ t('personalCenter.title') }}</p>
          <h1 class="my-1 text-[26px] font-extrabold">{{ userProfileStore.displayName }}</h1>
          <p class="text-sm text-muted-foreground">{{ userProfileStore.profile?.email || t('personalCenter.subtitle') }}</p>
        </div>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <span class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-[12.5px] font-semibold" :class="emailVerifiedVariant === 'success' ? 'bg-[color:var(--teal-soft)] text-[color:var(--teal-strong)]' : 'bg-[color:var(--gold-soft)] text-[color:var(--gold-strong)]'">{{ emailVerifiedLabel }}</span>
        <span v-if="userProfileStore.currentLevel" class="inline-flex items-center gap-1.5 rounded-full bg-[color:var(--gold-soft)] px-2.5 py-1 text-[12.5px] font-semibold text-[color:var(--gold-strong)]">
          <img v-if="isImagePath(userProfileStore.currentLevel?.icon)" :src="getImageUrl(userProfileStore.currentLevel!.icon)" class="h-3.5 w-3.5 object-contain" alt="" />
          <span v-else-if="userProfileStore.currentLevel?.icon">{{ userProfileStore.currentLevel.icon }}</span>
          <Crown v-else class="h-3.5 w-3.5" />
          {{ levelName(userProfileStore.currentLevel) }}
        </span>
      </div>
    </header>

    <div class="grid items-start gap-[22px] lg:grid-cols-[248px_1fr]">
      <!-- 侧栏 -->
      <aside class="min-w-0 sticky top-[90px] max-[900px]:static">
        <nav class="flex flex-col gap-0.5 rounded-xl border bg-card p-2.5 max-[900px]:flex-row max-[900px]:overflow-x-auto">
          <button
            v-for="item in visibleSectionItems"
            :key="item.key"
            type="button"
            class="flex w-full items-center gap-[11px] rounded-sm px-3.5 py-[11px] text-left text-[14.5px] font-semibold transition-colors max-[900px]:whitespace-nowrap"
            :class="currentSection === item.key ? 'bg-primary/10 text-primary' : 'text-muted-foreground hover:bg-secondary hover:text-foreground'"
            @click="switchSection(item.key)"
          >
            <component :is="item.icon" class="h-[18px] w-[18px] flex-none" />
            <span>{{ t(item.label) }}</span>
          </button>
        </nav>
      </aside>

      <!-- 内容 -->
      <section class="grid min-w-0 gap-[18px]">
        <div v-if="globalAlert" class="rounded-sm px-3.5 py-3 text-[13px] font-semibold" :class="globalAlert.level === 'error' ? 'bg-destructive/10 text-destructive' : (globalAlert.level === 'success' ? 'bg-[color:var(--teal-soft)] text-[color:var(--teal-strong)]' : 'bg-warning/10 text-warning')">
          {{ globalAlert.message }}
        </div>

        <!-- 概览 -->
        <template v-if="currentSection === 'overview'">
          <div class="grid gap-3.5 grid-cols-2 lg:grid-cols-4">
            <div class="flex items-center gap-3 rounded-lg border bg-card p-4">
              <div class="grid h-[42px] w-[42px] flex-none place-items-center rounded-xl bg-primary/10 text-primary"><ShoppingBag class="h-5 w-5" /></div>
              <div>
                <div class="text-xs text-muted-foreground">{{ t('personalCenter.tabs.orders') }}</div>
                <div class="mt-[3px] text-lg font-extrabold tabular-nums">{{ userProfileStore.loadingOrders ? '—' : userProfileStore.ordersTotal }}</div>
              </div>
            </div>
            <div class="flex items-center gap-3 rounded-lg border bg-card p-4">
              <div class="grid h-[42px] w-[42px] flex-none place-items-center rounded-xl bg-[color:var(--plum-soft)] text-[color:var(--plum)]"><Crown class="h-5 w-5" /></div>
              <div>
                <div class="text-xs text-muted-foreground">{{ t('personalCenter.memberLevel.currentLevel') }}</div>
                <div class="mt-[3px] flex items-center gap-1.5 text-[15px] font-extrabold">
                  <img v-if="isImagePath(userProfileStore.currentLevel?.icon)" :src="getImageUrl(userProfileStore.currentLevel!.icon)" class="h-3.5 w-3.5 object-contain" alt="" />
                  <span>{{ levelName(userProfileStore.currentLevel) }}</span>
                </div>
              </div>
            </div>
            <div class="flex items-center gap-3 rounded-lg border bg-card p-4">
              <div class="grid h-[42px] w-[42px] flex-none place-items-center rounded-xl bg-[color:var(--gold-soft)] text-[color:var(--gold-strong)]"><Percent class="h-5 w-5" /></div>
              <div>
                <div class="text-xs text-muted-foreground">{{ t('personalCenter.memberLevel.discountRate') }}</div>
                <div class="mt-[3px] text-lg font-extrabold">{{ discountText }}</div>
              </div>
            </div>
            <div class="flex items-center gap-3 rounded-lg border bg-card p-4">
              <div class="grid h-[42px] w-[42px] flex-none place-items-center rounded-xl bg-[color:var(--teal-soft)] text-[color:var(--teal-strong)]"><ShieldCheck class="h-5 w-5" /></div>
              <div>
                <div class="text-xs text-muted-foreground">{{ t('personalCenter.overview.accountLabel') }}</div>
                <div class="mt-[3px]"><span class="inline-flex items-center rounded-full px-2.5 py-1 text-[12.5px] font-semibold" :class="emailVerifiedVariant === 'success' ? 'bg-[color:var(--teal-soft)] text-[color:var(--teal-strong)]' : 'bg-[color:var(--gold-soft)] text-[color:var(--gold-strong)]'">{{ emailVerifiedLabel }}</span></div>
              </div>
            </div>
          </div>

          <!-- 会员等级卡 -->
          <div v-if="userProfileStore.memberLevels.length > 0" class="rounded-xl border bg-card p-[22px]">
            <div class="flex flex-wrap items-center justify-between gap-3.5">
              <div class="flex items-center gap-3.5">
                <div class="grid h-[46px] w-[46px] flex-none place-items-center rounded-md bg-primary/10 text-[22px]">
                  <img v-if="isImagePath(userProfileStore.currentLevel?.icon)" :src="getImageUrl(userProfileStore.currentLevel!.icon)" class="h-7 w-7 object-contain" alt="" />
                  <span v-else>{{ userProfileStore.currentLevel?.icon || '👤' }}</span>
                </div>
                <div>
                  <p class="text-[11px] uppercase tracking-[0.06em] text-muted-foreground">{{ t('personalCenter.memberLevel.currentLevel') }}</p>
                  <p class="mt-0.5 text-lg font-bold">{{ levelName(userProfileStore.currentLevel) }}</p>
                </div>
              </div>
              <div class="flex flex-wrap items-center gap-2">
                <span class="inline-flex items-center rounded-full bg-primary/10 px-2.5 py-1 text-[12.5px] font-semibold text-primary">
                  {{ t('personalCenter.memberLevel.discountRate') }}
                  {{ userProfileStore.currentLevel && userProfileStore.currentLevel.discount_rate < 100
                    ? t('personalCenter.memberLevel.discountOff', { n: userProfileStore.currentLevel.discount_rate / 10 })
                    : t('personalCenter.memberLevel.noDiscount') }}
                </span>
                <span v-if="!userProfileStore.nextLevel && userProfileStore.currentLevel" class="inline-flex items-center rounded-full bg-[color:var(--teal-soft)] px-2.5 py-1 text-[12.5px] font-semibold text-[color:var(--teal-strong)]">{{ t('personalCenter.memberLevel.highestLevel') }}</span>
              </div>
            </div>

            <div v-if="userProfileStore.nextLevel" class="mt-[18px] rounded-lg border border-primary/20 bg-primary/10 p-4">
              <div class="flex items-center gap-3">
                <div class="grid h-[38px] w-[38px] flex-none place-items-center rounded-md bg-secondary text-lg opacity-80">
                  <img v-if="isImagePath(userProfileStore.nextLevel.icon)" :src="getImageUrl(userProfileStore.nextLevel.icon)" class="h-3.5 w-3.5 object-contain" alt="" />
                  <span v-else>{{ userProfileStore.nextLevel.icon || '⭐' }}</span>
                </div>
                <div>
                  <p class="text-[11px] uppercase tracking-[0.06em] text-muted-foreground">{{ t('personalCenter.memberLevel.nextLevel') }}</p>
                  <div class="flex items-center gap-2">
                    <span class="text-sm font-bold text-muted-foreground">{{ levelName(userProfileStore.nextLevel) }}</span>
                    <span v-if="userProfileStore.nextLevel.discount_rate < 100" class="text-xs font-semibold text-primary">{{ t('personalCenter.memberLevel.discountOff', { n: userProfileStore.nextLevel.discount_rate / 10 }) }}</span>
                  </div>
                </div>
              </div>

              <div v-if="userProfileStore.upgradeProgress" class="mt-3.5 grid gap-3">
                <div v-if="userProfileStore.upgradeProgress.rechargePercent !== null">
                  <div class="mb-1.5 flex justify-between text-xs">
                    <span class="text-muted-foreground">{{ t('personalCenter.memberLevel.rechargeProgress') }}</span>
                    <span class="text-muted-foreground tabular-nums">{{ userProfileStore.upgradeProgress.recharged.toFixed(2) }} / {{ userProfileStore.upgradeProgress.rechargeThreshold.toFixed(2) }}</span>
                  </div>
                  <div class="h-1.5 overflow-hidden rounded-full bg-secondary"><div class="h-full rounded-full bg-primary transition-[width] duration-700" :style="{ width: userProfileStore.upgradeProgress.rechargePercent + '%' }"></div></div>
                </div>
                <div v-if="userProfileStore.upgradeProgress.spendPercent !== null">
                  <div class="mb-1.5 flex justify-between text-xs">
                    <span class="text-muted-foreground">{{ t('personalCenter.memberLevel.spendProgress') }}</span>
                    <span class="text-muted-foreground tabular-nums">{{ userProfileStore.upgradeProgress.spent.toFixed(2) }} / {{ userProfileStore.upgradeProgress.spendThreshold.toFixed(2) }}</span>
                  </div>
                  <div class="h-1.5 overflow-hidden rounded-full bg-secondary"><div class="h-full rounded-full bg-primary transition-[width] duration-700" :style="{ width: userProfileStore.upgradeProgress.spendPercent + '%' }"></div></div>
                </div>
              </div>
            </div>
          </div>

          <!-- 最近订单 -->
          <div class="rounded-xl border bg-card p-[22px]">
            <div class="mb-4 flex items-center justify-between gap-3.5">
              <h3 class="text-lg font-bold">{{ t('personalCenter.overview.recentOrdersTitle') }}</h3>
              <Button as-child variant="ghost" size="sm" class="rounded-full"><RouterLink to="/me/orders">{{ t('personalCenter.overview.viewAllOrders') }}</RouterLink></Button>
            </div>
            <div v-if="userProfileStore.loadingOrders" class="grid gap-2.5">
              <div v-for="idx in 3" :key="idx" class="h-16 rounded-md bg-secondary"></div>
            </div>
            <div v-else-if="userProfileStore.recentOrders.length === 0" class="rounded-md border border-dashed p-[18px] text-[13px] text-muted-foreground">{{ t('personalCenter.overview.emptyOrders') }}</div>
            <div v-else class="grid gap-2.5">
              <div v-for="order in userProfileStore.recentOrders" :key="order.order_no" class="flex flex-wrap items-center justify-between gap-3.5 rounded-lg border p-4 transition hover:-translate-y-0.5 hover:border-hairline-strong hover:shadow-[var(--shadow)]">
                <div>
                  <div class="text-[12.5px] text-muted-foreground">{{ t('orders.orderNo') }}：{{ order.order_no }}</div>
                  <div class="my-[3px] text-base font-bold tabular-nums">{{ formatMoney(order.total_amount, order.currency) }}</div>
                  <div class="text-[12.5px] text-muted-foreground">{{ formatDate(order.created_at) }}</div>
                </div>
                <div class="flex flex-wrap items-center gap-2.5">
                  <Badge :variant="statusVariant(order.status)" class="rounded-full">{{ statusLabel(order.status) }}</Badge>
                  <Button as-child variant="outline" size="sm" class="rounded-full"><RouterLink :to="`/orders/${order.order_no}`">{{ t('orders.viewDetails') }}</RouterLink></Button>
                  <Button v-if="order.status === 'pending_payment'" as-child size="sm" class="rounded-full"><RouterLink :to="`/pay?order_no=${order.order_no}`">{{ t('orders.payNow') }}</RouterLink></Button>
                </div>
              </div>
            </div>
          </div>
        </template>

        <!-- 其余面板：复用现有组件（shadcn 风，嵌于 vault 作用域内） -->
        <div v-else class="min-w-0">
          <ProfilePanel v-if="currentSection === 'profile'" />
          <SecurityPanel v-else-if="currentSection === 'security'" />
          <OrdersPanel v-else-if="currentSection === 'orders'" />
          <WalletPanel v-else-if="currentSection === 'wallet'" />
          <GiftCardPanel v-else-if="currentSection === 'giftCard'" />
          <OrdersPanel v-else />
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Crown, ShoppingBag, ShieldCheck, Percent } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { getImageUrl } from '../../utils/image'
import ProfilePanel from '../../views/personal/ProfilePanel.vue'
import SecurityPanel from '../../views/personal/SecurityPanel.vue'
import OrdersPanel from '../../views/personal/OrdersPanel.vue'
import WalletPanel from '../../views/personal/WalletPanel.vue'
import GiftCardPanel from '../../views/personal/GiftCardPanel.vue'

import { usePersonalCenter, type PersonalSection } from '../../composables/usePersonalCenter'

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
