<template>
  <div class="mx-auto w-full max-w-[1100px] px-6 pb-8">
    <div class="my-5 flex items-start justify-between gap-4">
      <div>
        <h1 class="mb-1.5 text-3xl font-extrabold">{{ t('rechargeOrder.title') }}</h1>
        <p class="text-muted-foreground">{{ t('rechargeOrder.subtitle') }}</p>
      </div>
      <RouterLink class="text-[13px] font-semibold text-muted-foreground transition-colors hover:text-primary" to="/me/orders">{{ t('rechargeOrder.backList') }}</RouterLink>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="luna-panel p-[22px]">
      <div class="mb-4 h-5 w-[35%] rounded bg-secondary"></div>
      <div class="h-[180px] rounded-md bg-secondary"></div>
    </div>

    <!-- 不存在 -->
    <div v-else-if="!recharge" class="my-6 flex flex-col items-center gap-3 rounded-xl border border-dashed py-16 text-center text-muted-foreground">
      <AlertCircle class="h-10 w-10 opacity-60" />
      <p>{{ t('rechargeOrder.notFound') }}</p>
      <Button class="mt-2 rounded-full" size="sm" @click="loadDetail()">{{ t('errorBoundary.retry') }}</Button>
    </div>

    <template v-else>
      <!-- 头部 -->
      <div class="mb-[18px] flex flex-wrap items-start justify-between gap-[18px] luna-panel p-[22px]">
        <div>
          <div class="text-[11px] uppercase tracking-[0.06em] text-muted-foreground">{{ t('personalCenter.wallet.rechargeNoLabel') }}</div>
          <div class="mt-1 font-bold">{{ recharge.recharge_no }}</div>
          <div class="mt-1.5 text-[13px] text-muted-foreground">{{ t('rechargeOrder.createdAtLabel') }}：{{ formatDate(recharge.created_at) }}</div>
        </div>
        <div>
          <div class="text-[11px] uppercase tracking-[0.06em] text-muted-foreground">{{ t('rechargeOrder.rechargeAmount') }}</div>
          <div class="mt-1 text-2xl font-extrabold tabular-nums">{{ formatMoney(recharge.amount, recharge.currency) }}</div>
        </div>
        <Badge :variant="rechargeStatusVariant(recharge.status)" class="rounded-full">{{ rechargeStatusText(recharge.status) }}</Badge>
      </div>

      <!-- 金额明细 -->
      <section class="mb-[18px] luna-panel p-[22px]">
        <h2 class="mb-4 text-lg font-bold">{{ t('rechargeOrder.amountTitle') }}</h2>
        <div class="grid gap-3 grid-cols-[repeat(auto-fill,minmax(180px,1fr))]">
          <div class="rounded-md border bg-secondary px-3.5 py-3">
            <div class="text-xs text-muted-foreground">{{ t('rechargeOrder.rechargeAmount') }}</div>
            <div class="mt-1.5 font-bold tabular-nums">{{ formatMoney(recharge.amount, recharge.currency) }}</div>
          </div>
          <div v-if="customerFeeApplied" class="rounded-md border border-warning/40 bg-secondary px-3.5 py-3 text-warning">
            <div class="text-xs">{{ t('payment.feeAmountLabel') }}</div>
            <div class="mt-1.5 font-bold tabular-nums">{{ formatMoney(recharge.fee_amount, recharge.currency) }}</div>
          </div>
          <div class="rounded-md border border-primary bg-secondary px-3.5 py-3 text-primary">
            <div class="text-xs">{{ t('personalCenter.wallet.payAmountLabel') }}</div>
            <div class="mt-1.5 font-bold tabular-nums">{{ formatMoney(recharge.payable_amount, recharge.currency) }}</div>
          </div>
        </div>
      </section>

      <!-- 时间 -->
      <section class="mb-[18px] luna-panel p-[22px]">
        <h2 class="mb-4 text-lg font-bold">{{ t('rechargeOrder.timeTitle') }}</h2>
        <div class="grid gap-3 grid-cols-[repeat(auto-fill,minmax(180px,1fr))]">
          <div class="rounded-md border bg-secondary px-3.5 py-3">
            <div class="text-xs text-muted-foreground">{{ t('rechargeOrder.createdAtLabel') }}</div>
            <div class="mt-1.5 font-bold">{{ formatDate(recharge.created_at) }}</div>
          </div>
          <div v-if="recharge.paid_at" class="rounded-md border bg-secondary px-3.5 py-3">
            <div class="text-xs text-muted-foreground">{{ t('rechargeOrder.paidAtLabel') }}</div>
            <div class="mt-1.5 font-bold">{{ formatDate(recharge.paid_at) }}</div>
          </div>
          <div v-if="payment?.expires_at" class="rounded-md border bg-secondary px-3.5 py-3">
            <div class="text-xs text-muted-foreground">{{ t('payment.expiresAt') }}</div>
            <div class="mt-1.5 font-bold">{{ formatDate(payment.expires_at) }}</div>
          </div>
        </div>
      </section>

      <!-- 备注 -->
      <section v-if="recharge.remark" class="mb-[18px] luna-panel p-[22px]">
        <h2 class="mb-4 text-lg font-bold">{{ t('rechargeOrder.remarkLabel') }}</h2>
        <p class="text-muted-foreground">{{ recharge.remark }}</p>
      </section>

      <!-- 支付区域 -->
      <section v-if="isPending" class="mb-[18px] luna-panel p-[22px]">
        <h2 class="mb-4 text-lg font-bold">{{ t('rechargeOrder.paymentTitle') }}</h2>
        <div class="mb-3.5 text-[13px] text-muted-foreground">{{ t('personalCenter.wallet.pendingHint') }}</div>
        <div class="grid grid-cols-1 gap-[18px] md:grid-cols-2">
          <div v-if="showQRCode" class="flex flex-col items-center rounded-xl border bg-secondary p-5 text-center">
            <div class="mb-3 text-[13px] text-muted-foreground">{{ t('payment.qrTitle') }}</div>
            <div class="aspect-square w-full max-w-[220px] overflow-hidden rounded-md bg-white p-2"><img :src="qrImageUrl" alt="Recharge QR" class="h-full w-full object-contain" /></div>
            <div v-if="qrUsingPayLinkFallback" class="mt-3 text-[13px] text-muted-foreground">{{ t('payment.qrFallbackHint') }}</div>
          </div>
          <div class="rounded-xl border p-[18px]">
            <div v-if="hasCryptoPaymentDetails" class="grid grid-cols-1 gap-2 rounded-md border p-3">
              <div v-for="item in cryptoPaymentDetails" :key="item.key" class="flex justify-between gap-3 border-b pb-1.5 last:border-b-0 last:pb-0">
                <span class="flex-none text-xs text-muted-foreground">{{ item.label }}</span>
                <span class="break-all text-right text-[13px] font-semibold text-foreground">{{ item.value }}<span v-if="item.detail" class="text-muted-foreground"> ({{ item.detail }})</span></span>
              </div>
              <div v-if="cryptoWalletAddress" class="flex items-center justify-end gap-2 pt-1.5">
                <Button type="button" variant="outline" size="sm" class="rounded-full" @click="handleCopyWalletAddress">{{ t('payment.copyWalletAddress') }}</Button>
                <span v-if="walletAddressCopied" class="text-xs text-success">{{ t('payment.copied') }}</span>
              </div>
            </div>
            <div class="mt-3.5 flex flex-wrap gap-2.5">
              <Button v-if="payLink" type="button" variant="outline" size="sm" class="rounded-full" @click="handleOpenPayLink">{{ t('payment.openPayLink') }}</Button>
              <Button type="button" size="sm" class="rounded-full" :disabled="checkingPayment" @click="checkPayment">
                {{ checkingPayment ? t('personalCenter.wallet.checkingPayStatus') : t('personalCenter.wallet.checkPayStatus') }}
              </Button>
            </div>
            <div v-if="showTelegramPayHint" class="mt-3 text-[13px] text-muted-foreground">{{ t('payment.telegramExternalHint') }}</div>
          </div>
        </div>
      </section>

      <!-- 成功 -->
      <section v-if="recharge.status === 'success'" class="mb-[18px] flex items-center gap-3 rounded-xl border border-l-4 border-l-success bg-card p-[22px]">
        <CheckCircle2 class="h-[26px] w-[26px] flex-none text-success" />
        <p class="font-bold text-foreground">{{ t('personalCenter.wallet.rechargeSuccess') }}</p>
      </section>
    </template>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { AlertCircle, CheckCircle2 } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
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
</script>
