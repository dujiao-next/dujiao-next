<template>
  <div class="min-h-screen pb-16 pt-[88px] text-foreground">
    <div class="mx-auto w-full max-w-[1240px] px-4 sm:px-6">
      <div class="mb-6 flex flex-wrap items-end justify-between gap-4 border-b pb-5">
        <div>
          <h1 class="text-[26px] font-bold tracking-[-0.025em] md:text-[30px]">{{ t('guestOrderDetail.title') }}</h1>
          <p class="mt-1 text-sm text-muted-foreground">{{ t('guestOrderDetail.subtitle') }}</p>
        </div>
        <RouterLink to="/guest/orders" class="theme-pill-link pl-2.5 pr-3.5">
          <ChevronLeft class="h-3.5 w-3.5" aria-hidden="true" />
          {{ t('guestOrderDetail.backSearch') }}
        </RouterLink>
      </div>

      <!-- 游客鉴权表单 -->
      <section v-if="viewState === 'auth'" class="atelier-card mx-auto max-w-xl rounded-[16px] bg-card p-6">
        <h2 class="text-lg font-bold">{{ t('guestOrderDetail.authTitle') }}</h2>
        <p class="mt-1 text-xs text-muted-foreground">{{ t('guestOrderDetail.authHint') }}</p>
        <div class="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2">
          <Input v-model="auth.email" type="email" class="h-11 rounded-[12px]" :placeholder="t('guestOrders.emailPlaceholder')" @keydown.enter="handleAuthSubmit" />
          <Input v-model="auth.order_password" type="password" class="h-11 rounded-[12px]" :placeholder="t('guestOrders.passwordPlaceholder')" @keydown.enter="handleAuthSubmit" />
        </div>
        <Alert v-if="authError" variant="destructive" class="mt-4">
          <AlertDescription>{{ authError }}</AlertDescription>
        </Alert>
        <div class="mt-5 flex items-center gap-3">
          <Button class="h-10.5 rounded-[11px] bg-primary px-6 font-semibold text-primary-foreground shadow-xs hover:bg-primary/90" @click="handleAuthSubmit">
            {{ t('guestOrderDetail.authSubmit') }}
          </Button>
          <Button type="button" variant="link" class="h-auto p-0 text-xs text-muted-foreground hover:text-foreground" @click="clearAuth">
            {{ t('guestOrderDetail.authClear') }}
          </Button>
        </div>
      </section>

      <div v-else-if="viewState === 'loading'" class="theme-skeleton h-40 rounded-[16px]"></div>

      <div v-else-if="viewState === 'empty'" class="atelier-card rounded-[16px] bg-card p-12 text-center">
        <p class="text-sm text-muted-foreground">{{ t('guestOrderDetail.notFound') }}</p>
      </div>

      <div v-else-if="viewState === 'detail' && order" class="space-y-5">
        <section class="atelier-card rounded-[16px] bg-card p-5 md:p-6">
          <div class="flex flex-col justify-between gap-4 md:flex-row md:items-center">
            <div>
              <div class="text-xs text-muted-foreground">{{ t('orders.orderNo') }}</div>
              <div class="mt-1 font-mono text-sm font-semibold text-foreground">{{ order.order_no }}</div>
              <div class="mt-1.5 text-xs text-muted-foreground">
                {{ t('orderDetail.createdAtLabel') }}：<span class="font-mono">{{ formatDate(order.created_at) }}</span>
              </div>
            </div>
            <div class="flex flex-col items-start gap-1 md:items-end">
              <div class="text-xs text-muted-foreground">{{ t('orderDetail.amountTotal') }}</div>
              <div class="text-2xl font-bold leading-none tracking-[-0.025em] tabular-nums text-foreground">
                {{ formatMoney(order.total_amount, order.currency) }}
              </div>
            </div>
            <div class="flex flex-wrap items-center gap-2.5">
              <Badge :variant="statusVariant(order.status)" size="sm">
                {{ statusLabel(order.status) }}
              </Badge>
              <Button
                v-if="order.status === 'pending_payment'"
                as-child
                size="sm"
                class="rounded-[10px] bg-primary px-4 font-semibold text-primary-foreground hover:bg-primary/90"
              >
                <RouterLink :to="`/pay?guest=1&order_no=${order.order_no}`">{{ t('orders.payNow') }}</RouterLink>
              </Button>
            </div>
          </div>
        </section>

        <AtelierOrderBody
          :order="order"
          :status-label="statusLabel"
          :status-variant="statusVariant"
          :fulfillment-type-label-text="fulfillmentTypeLabelText"
          :fulfillment-status-label-text="fulfillmentStatusLabelText"
          :format-date="formatDate"
          :get-localized-text="getLocalizedText"
          :format-money="formatMoney"
          :format-discount-money="formatDiscountMoney"
          :has-discount-amount="hasDiscountAmount"
          :refund-reason-text="refundReasonText"
          :show-refund-records-card="showRefundRecordsCard"
          :refund-records="refundRecords"
          :show-time-card="showTimeCard"
          :order-item-image="orderItemImage"
          :order-item-sku-text="orderItemSkuText"
          :manual-submission-rows="manualSubmissionRows"
          :has-item-discount="hasItemDiscount"
          :format-item-discount-total="formatItemDiscountTotal"
          :format-item-paid-amount="formatItemPaidAmount"
          :resolved-child-status="resolvedChildStatus"
          :fulfillment-delivery-lines="fulfillmentDeliveryLines"
          :instruction-blocks="instructionBlocks"
          :is-fulfillment-truncated="isFulfillmentTruncated"
          :fulfillment-copied="fulfillmentCopied"
          :fulfillment-downloading="fulfillmentDownloading"
          @copy-fulfillment="handleCopyFulfillment"
          @download-fulfillment="handleDownloadFulfillment"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ChevronLeft } from 'lucide-vue-next'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import AtelierOrderBody from './components/AtelierOrderBody.vue'
import { useGuestOrderDetail } from '../../composables/useGuestOrderDetail'

const { t } = useI18n()

const {
  order, authError, auth, viewState, handleAuthSubmit, clearAuth,
  fulfillmentDownloading, handleDownloadFulfillment,
  statusLabel, statusVariant, fulfillmentTypeLabelText, fulfillmentStatusLabelText,
  formatDate, getLocalizedText, formatMoney, formatDiscountMoney, hasDiscountAmount,
  refundReasonText, showRefundRecordsCard, refundRecords, showTimeCard,
  orderItemImage, orderItemSkuText, manualSubmissionRows,
  hasItemDiscount, formatItemDiscountTotal, formatItemPaidAmount, resolvedChildStatus,
  fulfillmentDeliveryLines, instructionBlocks, isFulfillmentTruncated,
  fulfillmentCopied, handleCopyFulfillment,
} = useGuestOrderDetail()
</script>
