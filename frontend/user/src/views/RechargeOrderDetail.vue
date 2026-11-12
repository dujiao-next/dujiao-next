<template>
  <div class="min-h-screen bg-background text-foreground pt-24 pb-16">
    <div class="container mx-auto px-4">
      <div class="mb-8 flex items-center justify-between">
        <div>
          <h1 class="text-3xl font-black text-foreground mb-2">{{ t('rechargeOrder.title') }}</h1>
          <p class="text-muted-foreground text-sm">{{ t('rechargeOrder.subtitle') }}</p>
        </div>
        <router-link to="/me/orders" class="text-muted-foreground transition-colors hover:text-foreground text-sm">{{ t('rechargeOrder.backList') }}</router-link>
      </div>

      <div v-if="loading" class="h-40 border bg-muted rounded-2xl animate-pulse"></div>

      <EmptyState
        v-else-if="!recharge"
        icon="alert"
        :title="t('rechargeOrder.notFound')"
        :action-label="t('errorBoundary.retry')"
        @action="loadDetail()"
      />

      <div v-else class="space-y-6">
        <!-- 头部信息 -->
        <div class="rounded-2xl border bg-card shadow-sm p-6">
          <div class="flex flex-col md:flex-row md:items-center md:justify-between gap-4">
            <div>
              <div class="text-xs uppercase tracking-wider text-muted-foreground">{{ t('personalCenter.wallet.rechargeNoLabel') }}</div>
              <div class="text-sm font-semibold text-foreground mt-1">{{ recharge.recharge_no }}</div>
              <div class="text-xs text-muted-foreground mt-2">{{ t('rechargeOrder.createdAtLabel') }}：{{ formatDate(recharge.created_at) }}</div>
            </div>
            <div class="flex flex-col items-start md:items-end gap-2">
              <div class="text-xs uppercase tracking-wider text-muted-foreground">{{ t('rechargeOrder.rechargeAmount') }}</div>
              <div class="text-lg font-bold text-foreground">{{ formatMoney(recharge.amount, recharge.currency) }}</div>
            </div>
            <div class="flex items-center gap-3">
              <Badge :variant="rechargeStatusVariant(recharge.status)" size="sm">
                {{ rechargeStatusText(recharge.status) }}
              </Badge>
            </div>
          </div>
        </div>

        <!-- 金额明细 -->
        <div class="rounded-2xl border bg-card shadow-sm p-6">
          <h2 class="text-lg font-bold mb-4">{{ t('rechargeOrder.amountTitle') }}</h2>
          <div class="grid grid-cols-1 gap-4 text-sm" :class="customerFeeApplied ? 'md:grid-cols-3' : 'md:grid-cols-2'">
            <div class="border rounded-xl p-4">
              <div class="text-xs text-muted-foreground">{{ t('rechargeOrder.rechargeAmount') }}</div>
              <div class="text-foreground font-mono mt-1">{{ formatMoney(recharge.amount, recharge.currency) }}</div>
            </div>
            <div v-if="customerFeeApplied" class="border rounded-xl p-4 text-warning">
              <div class="text-xs">{{ t('payment.feeAmountLabel') }}</div>
              <div class="font-mono mt-1 font-semibold">{{ formatMoney(recharge.fee_amount, recharge.currency) }}</div>
            </div>
            <div class="border rounded-xl p-4">
              <div class="text-xs text-muted-foreground">{{ t('personalCenter.wallet.payAmountLabel') }}</div>
              <div class="text-foreground font-mono mt-1 font-bold">{{ formatMoney(recharge.payable_amount, recharge.currency) }}</div>
            </div>
            <div v-if="Number(recharge.refunded_amount) > 0" class="border rounded-xl p-4">
              <div class="text-xs text-muted-foreground">{{ t('orderDetail.amountRefunded') }}</div>
              <div class="text-foreground font-mono mt-1">{{ formatMoney(recharge.refunded_amount, recharge.currency) }}</div>
            </div>
          </div>
        </div>

        <!-- 时间信息 -->
        <div class="rounded-2xl border bg-card shadow-sm p-6">
          <h2 class="text-lg font-bold mb-4">{{ t('rechargeOrder.timeTitle') }}</h2>
          <div class="grid grid-cols-1 md:grid-cols-3 gap-4 text-sm">
            <div class="border rounded-xl p-4">
              <div class="text-xs text-muted-foreground">{{ t('rechargeOrder.createdAtLabel') }}</div>
              <div class="text-foreground font-mono mt-1">{{ formatDate(recharge.created_at) }}</div>
            </div>
            <div v-if="recharge.paid_at" class="border rounded-xl p-4">
              <div class="text-xs text-muted-foreground">{{ t('rechargeOrder.paidAtLabel') }}</div>
              <div class="text-foreground font-mono mt-1">{{ formatDate(recharge.paid_at) }}</div>
            </div>
            <div v-if="payment?.expires_at" class="border rounded-xl p-4">
              <div class="text-xs text-muted-foreground">{{ t('payment.expiresAt') }}</div>
              <div class="text-foreground font-mono mt-1">{{ formatDate(payment.expires_at) }}</div>
            </div>
          </div>
        </div>

        <!-- 备注 -->
        <div v-if="recharge.remark" class="rounded-2xl border bg-card shadow-sm p-6">
          <h2 class="text-lg font-bold mb-2">{{ t('rechargeOrder.remarkLabel') }}</h2>
          <p class="text-sm text-muted-foreground">{{ recharge.remark }}</p>
        </div>

        <!-- 支付区域（仅待支付状态） -->
        <div v-if="isPending" class="rounded-2xl border bg-card shadow-sm p-6">
          <h2 class="text-lg font-bold mb-4">{{ t('rechargeOrder.paymentTitle') }}</h2>
          <div v-if="isPending" class="mb-3 text-xs text-muted-foreground">
            {{ t('personalCenter.wallet.pendingHint') }}
          </div>
          <div class="grid grid-cols-1 gap-6 md:grid-cols-2">
            <div v-if="showQRCode" class="rounded-xl border p-4">
              <div class="mb-3 text-sm font-semibold text-foreground">{{ t('payment.qrTitle') }}</div>
              <div class="flex items-center justify-center">
                <img :src="qrImageUrl" alt="Recharge QR" class="h-52 w-52 object-contain" />
              </div>
              <div v-if="qrUsingPayLinkFallback" class="mt-3 text-xs text-muted-foreground">
                {{ t('payment.qrFallbackHint') }}
              </div>
            </div>
            <div class="rounded-xl border p-4">
              <div v-if="hasCryptoPaymentDetails" class="space-y-2 rounded-xl border bg-muted/40 p-3 text-sm">
                <div
                  v-for="item in cryptoPaymentDetails"
                  :key="item.key"
                  class="flex flex-col gap-1 border-b pb-2 last:border-b-0 last:pb-0"
                >
                  <span class="text-xs text-muted-foreground">{{ item.label }}</span>
                  <span class="min-w-0 font-semibold text-foreground break-all">
                    {{ item.value }}
                    <span v-if="item.detail" class="ml-1 font-normal text-muted-foreground">({{ item.detail }})</span>
                  </span>
                </div>
                <div v-if="cryptoWalletAddress" class="flex flex-wrap items-center gap-2 pt-1">
                  <Button type="button" variant="outline" size="sm" @click="handleCopyWalletAddress">
                    {{ t('payment.copyWalletAddress') }}
                  </Button>
                  <span v-if="walletAddressCopied" class="text-xs text-emerald-500">{{ t('payment.copied') }}</span>
                </div>
              </div>
              <div class="mt-4 flex flex-wrap items-center gap-3">
                <Button v-if="payLink" type="button" variant="default" size="sm" class="font-bold" @click="handleOpenPayLink">
                  <ExternalLink class="h-4 w-4" aria-hidden="true" />
                  {{ t('payment.openPayLink') }}
                </Button>
                <Button type="button" variant="outline" size="sm" :disabled="checkingPayment" @click="checkPayment">
                  {{ checkingPayment ? t('personalCenter.wallet.checkingPayStatus') : t('personalCenter.wallet.checkPayStatus') }}
                </Button>
              </div>
              <div v-if="showTelegramPayHint" class="mt-3 text-xs text-muted-foreground">
                {{ t('payment.telegramExternalHint') }}
              </div>
            </div>
          </div>
        </div>

        <!-- 支付成功提示 -->
        <div v-if="recharge.status === 'success'" class="rounded-2xl border bg-card shadow-sm p-6 border-l-4 border-green-500">
          <p class="text-sm font-semibold text-foreground">{{ t('personalCenter.wallet.rechargeSuccess') }}</p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ExternalLink } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import EmptyState from '../components/EmptyState.vue'
import { useRechargeOrderDetail } from '../composables/useRechargeOrderDetail'

const { t } = useI18n()

const {
  loading, checkingPayment, recharge, payment, walletAddressCopied, qrImageUrl,
  isPending, payLink, showTelegramPayHint, qrUsingPayLinkFallback, showQRCode,
  cryptoWalletAddress, cryptoPaymentDetails, hasCryptoPaymentDetails, customerFeeApplied,
  rechargeStatusText, rechargeStatusVariant, formatMoney, formatDate,
  loadDetail, checkPayment, handleOpenPayLink, handleCopyWalletAddress,
} = useRechargeOrderDetail()
</script>
