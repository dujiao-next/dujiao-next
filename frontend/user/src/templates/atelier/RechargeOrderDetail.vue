<template>
  <div class="min-h-screen pb-16 pt-[88px] text-foreground">
    <div class="mx-auto w-full max-w-[1240px] px-4 sm:px-6">
      <div class="mb-7 flex flex-wrap items-end justify-between gap-4 border-b pb-5">
        <div>
          <h1 class="text-[26px] font-bold tracking-[-0.025em] md:text-[30px]">{{ t('rechargeOrder.title') }}</h1>
          <p class="mt-1 text-sm text-muted-foreground">{{ t('rechargeOrder.subtitle') }}</p>
        </div>
        <RouterLink to="/me/orders" class="theme-pill-link pl-2.5 pr-3.5">
          <ChevronLeft class="h-3.5 w-3.5" aria-hidden="true" />
          {{ t('rechargeOrder.backList') }}
        </RouterLink>
      </div>

      <div v-if="loading" class="theme-skeleton h-40 rounded-[16px]"></div>

      <EmptyState
        v-else-if="!recharge"
        icon="alert"
        :title="t('rechargeOrder.notFound')"
        :action-label="t('errorBoundary.retry')"
        @action="loadDetail()"
      />

      <div v-else class="space-y-5">
        <!-- 头部信息 -->
        <section class="atelier-card rounded-[16px] bg-card p-5 md:p-6">
          <div class="flex flex-col justify-between gap-4 md:flex-row md:items-center">
            <div>
              <div class="text-xs text-muted-foreground">{{ t('personalCenter.wallet.rechargeNoLabel') }}</div>
              <div class="mt-1 font-mono text-sm font-semibold text-foreground">{{ recharge.recharge_no }}</div>
              <div class="mt-1.5 text-xs text-muted-foreground">
                {{ t('rechargeOrder.createdAtLabel') }}：<span class="font-mono">{{ formatDate(recharge.created_at) }}</span>
              </div>
            </div>
            <div class="flex flex-col items-start gap-1 md:items-end">
              <div class="text-xs text-muted-foreground">{{ t('rechargeOrder.rechargeAmount') }}</div>
              <div class="text-2xl font-bold leading-none tracking-[-0.025em] tabular-nums text-foreground">
                {{ formatMoney(recharge.amount, recharge.currency) }}
              </div>
            </div>
            <Badge :variant="rechargeStatusVariant(recharge.status)" size="sm" class="self-start md:self-auto">
              {{ rechargeStatusText(recharge.status) }}
            </Badge>
          </div>
        </section>

        <!-- 金额明细 -->
        <section class="atelier-card rounded-[16px] bg-card p-5 md:p-6">
          <h2 class="mb-4 border-b pb-3 text-base font-bold tracking-[-0.015em]">{{ t('rechargeOrder.amountTitle') }}</h2>
          <div class="grid grid-cols-1 gap-3 text-sm" :class="customerFeeApplied ? 'sm:grid-cols-3' : 'sm:grid-cols-2'">
            <div class="rounded-[14px] border bg-background p-3.5">
              <div class="text-xs text-muted-foreground">{{ t('rechargeOrder.rechargeAmount') }}</div>
              <div class="mt-1 font-semibold tabular-nums text-foreground">{{ formatMoney(recharge.amount, recharge.currency) }}</div>
            </div>
            <div v-if="customerFeeApplied" class="rounded-[14px] bg-warning-soft p-3.5 text-warning">
              <div class="text-xs">{{ t('payment.feeAmountLabel') }}</div>
              <div class="mt-1 font-semibold tabular-nums">{{ formatMoney(recharge.fee_amount, recharge.currency) }}</div>
            </div>
            <div class="rounded-[14px] border bg-background p-3.5">
              <div class="text-xs text-muted-foreground">{{ t('personalCenter.wallet.payAmountLabel') }}</div>
              <div class="mt-1 font-bold tabular-nums text-foreground">{{ formatMoney(recharge.payable_amount, recharge.currency) }}</div>
            </div>
            <div v-if="Number(recharge.refunded_amount) > 0" class="rounded-[14px] border bg-background p-3.5">
              <div class="text-xs text-muted-foreground">{{ t('orderDetail.amountRefunded') }}</div>
              <div class="mt-1 font-semibold tabular-nums text-foreground">{{ formatMoney(recharge.refunded_amount, recharge.currency) }}</div>
            </div>
          </div>
        </section>

        <!-- 时间信息 -->
        <section class="atelier-card rounded-[16px] bg-card p-5 md:p-6">
          <h2 class="mb-4 border-b pb-3 text-base font-bold tracking-[-0.015em]">{{ t('rechargeOrder.timeTitle') }}</h2>
          <div class="grid grid-cols-1 gap-3 text-sm sm:grid-cols-3">
            <div class="rounded-[14px] border bg-background p-3.5">
              <div class="text-xs text-muted-foreground">{{ t('rechargeOrder.createdAtLabel') }}</div>
              <div class="mt-1 font-mono text-xs text-foreground">{{ formatDate(recharge.created_at) }}</div>
            </div>
            <div v-if="recharge.paid_at" class="rounded-[14px] border bg-background p-3.5">
              <div class="text-xs text-muted-foreground">{{ t('rechargeOrder.paidAtLabel') }}</div>
              <div class="mt-1 font-mono text-xs text-foreground">{{ formatDate(recharge.paid_at) }}</div>
            </div>
            <div v-if="payment?.expires_at" class="rounded-[14px] border bg-background p-3.5">
              <div class="text-xs text-muted-foreground">{{ t('payment.expiresAt') }}</div>
              <div class="mt-1 font-mono text-xs text-foreground">{{ formatDate(payment.expires_at) }}</div>
            </div>
          </div>
        </section>

        <!-- 备注 -->
        <section v-if="recharge.remark" class="atelier-card rounded-[16px] bg-card p-5 md:p-6">
          <h2 class="mb-2 text-base font-bold">{{ t('rechargeOrder.remarkLabel') }}</h2>
          <p class="text-sm text-muted-foreground">{{ recharge.remark }}</p>
        </section>

        <!-- 支付区域 -->
        <section v-if="isPending" class="atelier-card rounded-[16px] bg-card p-5 md:p-6">
          <h2 class="mb-2 border-b pb-3 text-base font-bold tracking-[-0.015em]">{{ t('rechargeOrder.paymentTitle') }}</h2>
          <p class="mb-4 text-xs text-muted-foreground">{{ t('personalCenter.wallet.pendingHint') }}</p>

          <div class="grid grid-cols-1 gap-5 md:grid-cols-2">
            <div v-if="showQRCode" class="rounded-[16px] border bg-background p-5 text-center">
              <div class="mb-3 text-sm font-semibold text-foreground">{{ t('payment.qrTitle') }}</div>
              <div class="mx-auto inline-block rounded-[14px] border bg-white p-2.5">
                <img :src="qrImageUrl" alt="Recharge QR" class="h-48 w-48 object-contain" />
              </div>
              <div v-if="qrUsingPayLinkFallback" class="mt-3 text-xs text-muted-foreground">
                {{ t('payment.qrFallbackHint') }}
              </div>
            </div>

            <div class="rounded-[16px] border bg-background p-5">
              <div v-if="hasCryptoPaymentDetails" class="space-y-2 rounded-[12px] border bg-card p-3.5 text-sm">
                <div
                  v-for="item in cryptoPaymentDetails"
                  :key="item.key"
                  class="flex flex-col gap-1 border-b pb-2 last:border-b-0 last:pb-0"
                >
                  <span class="text-xs text-muted-foreground">{{ item.label }}</span>
                  <span class="min-w-0 break-all font-mono text-xs font-semibold text-foreground">
                    {{ item.value }}
                    <span v-if="item.detail" class="ml-1 font-sans font-normal text-muted-foreground">({{ item.detail }})</span>
                  </span>
                </div>
                <div v-if="cryptoWalletAddress" class="flex flex-wrap items-center gap-2 pt-1">
                  <Button type="button" variant="secondary" size="sm" class="rounded-[9px]" @click="handleCopyWalletAddress">
                    {{ t('payment.copyWalletAddress') }}
                  </Button>
                  <span v-if="walletAddressCopied" class="text-xs text-success">{{ t('payment.copied') }}</span>
                </div>
              </div>

              <div class="mt-4 flex flex-wrap items-center gap-2.5">
                <Button
                  v-if="payLink"
                  type="button"
                  size="sm"
                  class="h-9.5 rounded-[10px] bg-primary px-4 font-semibold text-primary-foreground hover:bg-primary/90"
                  @click="handleOpenPayLink"
                >
                  <ExternalLink class="h-3.5 w-3.5" aria-hidden="true" />
                  {{ t('payment.openPayLink') }}
                </Button>
                <Button type="button" variant="outline" size="sm" class="h-9.5 rounded-[10px] px-4" :disabled="checkingPayment" @click="checkPayment">
                  {{ checkingPayment ? t('personalCenter.wallet.checkingPayStatus') : t('personalCenter.wallet.checkPayStatus') }}
                </Button>
              </div>
              <div v-if="showTelegramPayHint" class="mt-3 text-xs text-muted-foreground">
                {{ t('payment.telegramExternalHint') }}
              </div>
            </div>
          </div>
        </section>

        <!-- 充值成功提示 -->
        <section v-if="recharge.status === 'success'" class="atelier-card rounded-[16px] border-l-4 border-l-success bg-card p-5 md:p-6">
          <p class="text-sm font-semibold text-success">{{ t('personalCenter.wallet.rechargeSuccess') }}</p>
        </section>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ChevronLeft, ExternalLink } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import EmptyState from '../../components/EmptyState.vue'
import { useRechargeOrderDetail } from '../../composables/useRechargeOrderDetail'

const { t } = useI18n()

const {
  loading, checkingPayment, recharge, payment, walletAddressCopied, qrImageUrl,
  isPending, payLink, showTelegramPayHint, qrUsingPayLinkFallback, showQRCode,
  cryptoWalletAddress, cryptoPaymentDetails, hasCryptoPaymentDetails, customerFeeApplied,
  rechargeStatusText, rechargeStatusVariant, formatMoney, formatDate,
  loadDetail, checkPayment, handleOpenPayLink, handleCopyWalletAddress,
} = useRechargeOrderDetail()
</script>
