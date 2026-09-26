<template>
  <div class="mx-auto w-full max-w-[880px] px-4 pb-10 pt-5 sm:px-6">
    <div class="mb-5 flex flex-wrap items-start justify-between gap-4">
      <div class="flex flex-col gap-2">
        <h1 class="text-[clamp(1.75rem,1.3rem+1.9vw,2.6rem)] font-semibold tracking-[-0.022em]">{{ t('rechargeOrder.title') }}</h1>
        <p class="text-[15px] text-muted-foreground">{{ t('rechargeOrder.subtitle') }}</p>
      </div>
      <RouterLink class="text-[13px] text-muted-foreground transition-colors hover:text-foreground" to="/me/orders">
        {{ t('rechargeOrder.backList') }}
      </RouterLink>
    </div>

    <!-- 加载 -->
    <div v-if="loading" class="flex flex-col gap-4">
      <div class="h-[130px] rounded-[var(--r-lg)] border border-border bg-card"></div>
      <div class="h-[170px] rounded-[var(--r-lg)] border border-border bg-card"></div>
    </div>

    <!-- 不存在 -->
    <div v-else-if="!recharge" class="flex flex-col items-center gap-3 rounded-[var(--r-lg)] border border-dashed border-border py-16 text-center">
      <AlertCircle class="h-11 w-11 text-muted-foreground" />
      <p class="text-[14px] text-muted-foreground">{{ t('rechargeOrder.notFound') }}</p>
      <Button size="sm" class="mt-2 rounded-full" @click="loadDetail()">{{ t('errorBoundary.retry') }}</Button>
    </div>

    <template v-else>
      <!-- 头部 -->
      <div class="mb-4 flex flex-wrap items-start justify-between gap-5 rounded-[var(--r-lg)] border border-border bg-card p-5">
        <div>
          <div class="text-[12px] text-muted-foreground">{{ t('personalCenter.wallet.rechargeNoLabel') }}</div>
          <div class="mt-1 font-mono text-[14px] font-medium">{{ recharge.recharge_no }}</div>
          <div class="mt-1.5 text-[12.5px] text-muted-foreground">
            {{ t('rechargeOrder.createdAtLabel') }}：{{ formatDate(recharge.created_at) }}
          </div>
        </div>
        <div>
          <div class="text-[12px] text-muted-foreground">{{ t('rechargeOrder.rechargeAmount') }}</div>
          <div class="mt-1 text-[24px] font-semibold tabular-nums tracking-[-0.03em]">
            {{ formatMoney(recharge.amount, recharge.currency) }}
          </div>
        </div>
        <span class="ember-pill" :class="pillClass(rechargeStatusVariant(recharge.status))">
          {{ rechargeStatusText(recharge.status) }}
        </span>
      </div>

      <!-- 金额明细 -->
      <section class="mb-4 rounded-[var(--r-lg)] border border-border bg-card p-5">
        <h2 class="mb-4 text-[15px] font-semibold tracking-[-0.015em]">{{ t('rechargeOrder.amountTitle') }}</h2>
        <div class="grid gap-3 grid-cols-[repeat(auto-fill,minmax(180px,1fr))]">
          <div class="rounded-[var(--r)] border border-[color:var(--border-soft)] bg-[color:var(--surface-2)] px-4 py-3">
            <div class="text-[12px] text-muted-foreground">{{ t('rechargeOrder.rechargeAmount') }}</div>
            <div class="mt-1.5 font-medium tabular-nums">{{ formatMoney(recharge.amount, recharge.currency) }}</div>
          </div>
          <div
            v-if="customerFeeApplied"
            class="rounded-[var(--r)] border border-[color:var(--warning)] bg-[color:var(--warning-soft)] px-4 py-3 text-[color:var(--warning-strong)]"
          >
            <div class="text-[12px]">{{ t('payment.feeAmountLabel') }}</div>
            <div class="mt-1.5 font-medium tabular-nums">{{ formatMoney(recharge.fee_amount, recharge.currency) }}</div>
          </div>
          <div class="rounded-[var(--r)] border border-[color:var(--accent)] bg-[color:var(--accent-soft)] px-4 py-3 text-[color:var(--accent-strong)]">
            <div class="text-[12px]">{{ t('personalCenter.wallet.payAmountLabel') }}</div>
            <div class="mt-1.5 font-medium tabular-nums">{{ formatMoney(recharge.payable_amount, recharge.currency) }}</div>
          </div>
        </div>
      </section>

      <!-- 时间 -->
      <section class="mb-4 rounded-[var(--r-lg)] border border-border bg-card p-5">
        <h2 class="mb-4 text-[15px] font-semibold tracking-[-0.015em]">{{ t('rechargeOrder.timeTitle') }}</h2>
        <div class="grid gap-3 grid-cols-[repeat(auto-fill,minmax(180px,1fr))]">
          <div class="rounded-[var(--r)] border border-[color:var(--border-soft)] bg-[color:var(--surface-2)] px-4 py-3">
            <div class="text-[12px] text-muted-foreground">{{ t('rechargeOrder.createdAtLabel') }}</div>
            <div class="mt-1.5 font-medium">{{ formatDate(recharge.created_at) }}</div>
          </div>
          <div v-if="recharge.paid_at" class="rounded-[var(--r)] border border-[color:var(--border-soft)] bg-[color:var(--surface-2)] px-4 py-3">
            <div class="text-[12px] text-muted-foreground">{{ t('rechargeOrder.paidAtLabel') }}</div>
            <div class="mt-1.5 font-medium">{{ formatDate(recharge.paid_at) }}</div>
          </div>
          <div v-if="payment?.expires_at" class="rounded-[var(--r)] border border-[color:var(--border-soft)] bg-[color:var(--surface-2)] px-4 py-3">
            <div class="text-[12px] text-muted-foreground">{{ t('payment.expiresAt') }}</div>
            <div class="mt-1.5 font-medium">{{ formatDate(payment.expires_at) }}</div>
          </div>
        </div>
      </section>

      <!-- 备注 -->
      <section v-if="recharge.remark" class="mb-4 rounded-[var(--r-lg)] border border-border bg-card p-5">
        <h2 class="mb-3 text-[15px] font-semibold tracking-[-0.015em]">{{ t('rechargeOrder.remarkLabel') }}</h2>
        <p class="text-[14px] text-muted-foreground">{{ recharge.remark }}</p>
      </section>

      <!-- 支付 -->
      <section v-if="isPending" class="mb-4 rounded-[var(--r-lg)] border border-border bg-card p-5">
        <h2 class="mb-3 text-[15px] font-semibold tracking-[-0.015em]">{{ t('rechargeOrder.paymentTitle') }}</h2>
        <p class="mb-4 text-[13px] text-muted-foreground">{{ t('personalCenter.wallet.pendingHint') }}</p>

        <div class="grid gap-5 md:grid-cols-2">
          <div v-if="showQRCode" class="flex flex-col items-center rounded-[var(--r-lg)] border border-border bg-[color:var(--surface-2)] p-5 text-center">
            <div class="mb-3 text-[13px] text-muted-foreground">{{ t('payment.qrTitle') }}</div>
            <div class="aspect-square w-full max-w-[220px] overflow-hidden rounded-[var(--r)] bg-white p-3">
              <img :src="qrImageUrl" alt="Recharge QR" class="h-full w-full object-contain" />
            </div>
            <div v-if="qrUsingPayLinkFallback" class="mt-3 text-[12.5px] text-muted-foreground">{{ t('payment.qrFallbackHint') }}</div>
          </div>

          <div class="rounded-[var(--r-lg)] border border-border p-4">
            <div v-if="hasCryptoPaymentDetails" class="flex flex-col gap-2 rounded-[var(--r)] border border-[color:var(--border-soft)] p-3">
              <div
                v-for="item in cryptoPaymentDetails"
                :key="item.key"
                class="flex justify-between gap-3 border-b border-[color:var(--border-soft)] pb-1.5 last:border-b-0 last:pb-0"
              >
                <span class="flex-none text-[12px] text-muted-foreground">{{ item.label }}</span>
                <span class="break-all text-right text-[13px] font-medium">
                  {{ item.value }}<span v-if="item.detail" class="text-muted-foreground"> ({{ item.detail }})</span>
                </span>
              </div>
              <div v-if="cryptoWalletAddress" class="flex items-center justify-end gap-2 pt-1.5">
                <Button type="button" variant="outline" size="sm" class="rounded-full border-[color:var(--border-strong)]" @click="handleCopyWalletAddress">
                  {{ t('payment.copyWalletAddress') }}
                </Button>
                <span v-if="walletAddressCopied" class="text-[12px] text-[color:var(--success-strong)]">{{ t('payment.copied') }}</span>
              </div>
            </div>

            <div class="mt-3.5 flex flex-wrap gap-2.5">
              <Button v-if="payLink" type="button" variant="outline" size="sm" class="rounded-full border-[color:var(--border-strong)]" @click="handleOpenPayLink">
                {{ t('payment.openPayLink') }}
              </Button>
              <Button type="button" size="sm" class="rounded-full" :disabled="checkingPayment" @click="checkPayment">
                {{ checkingPayment ? t('personalCenter.wallet.checkingPayStatus') : t('personalCenter.wallet.checkPayStatus') }}
              </Button>
            </div>

            <p v-if="showTelegramPayHint" class="mt-3 text-[12.5px] text-muted-foreground">{{ t('payment.telegramExternalHint') }}</p>
          </div>
        </div>
      </section>

      <!-- 充值成功 -->
      <section
        v-if="recharge.status === 'success'"
        class="flex items-center gap-3 rounded-[var(--r-lg)] border border-border border-l-[3px] border-l-[color:var(--success)] bg-card p-5"
      >
        <CheckCircle2 class="h-6 w-6 flex-none text-[color:var(--success)]" />
        <p class="text-[14px] font-medium">{{ t('personalCenter.wallet.rechargeSuccess') }}</p>
      </section>
    </template>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { AlertCircle, CheckCircle2 } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { useRechargeOrderDetail } from '../../composables/useRechargeOrderDetail'

const { t } = useI18n()

const {
  loading, checkingPayment, recharge, payment, walletAddressCopied, qrImageUrl,
  isPending, payLink, showTelegramPayHint, qrUsingPayLinkFallback, showQRCode,
  cryptoWalletAddress, cryptoPaymentDetails, hasCryptoPaymentDetails, customerFeeApplied,
  rechargeStatusText, rechargeStatusVariant, formatMoney, formatDate,
  loadDetail, checkPayment, handleOpenPayLink, handleCopyWalletAddress,
} = useRechargeOrderDetail()

/** 状态徽标配色 */
const pillClass = (variant?: string) => {
  const map: Record<string, string> = {
    success: 'ember-pill--success',
    warning: 'ember-pill--warning',
    danger: 'ember-pill--danger',
    info: 'ember-pill--accent',
    accent: 'ember-pill--accent',
    neutral: '',
  }
  return map[variant || ''] || ''
}
</script>
