<template>
  <div class="min-h-screen pb-16 pt-[88px] text-foreground">
    <div class="mx-auto w-full max-w-[1240px] px-4 sm:px-6">
      <BreadcrumbNav
        class="mb-5"
        :items="[
          { label: t('nav.home'), to: '/' },
          { label: t('orders.title'), to: '/me/orders' },
          { label: t('orderDetail.title') },
        ]"
      />

      <div class="mb-6 border-b pb-5">
        <h1 class="text-[26px] font-bold tracking-[-0.025em] md:text-[30px]">{{ t('orderDetail.title') }}</h1>
        <p class="mt-1 text-sm text-muted-foreground">{{ t('orderDetail.subtitle') }}</p>
      </div>

      <!-- 骨架屏 -->
      <div v-if="loading" class="space-y-5">
        <div class="atelier-card space-y-4 rounded-[16px] bg-card p-6">
          <div class="flex flex-col justify-between gap-4 md:flex-row md:items-center">
            <div class="space-y-2">
              <div class="h-3 w-20 rounded theme-skeleton"></div>
              <div class="h-5 w-56 rounded theme-skeleton"></div>
            </div>
            <div class="h-7 w-24 rounded-full theme-skeleton"></div>
          </div>
          <div class="grid grid-cols-2 gap-4 border-t pt-4 md:grid-cols-4">
            <div v-for="i in 4" :key="i" class="space-y-2">
              <div class="h-3 w-16 rounded theme-skeleton"></div>
              <div class="h-4 w-24 rounded theme-skeleton"></div>
            </div>
          </div>
        </div>
      </div>

      <EmptyState
        v-else-if="!order"
        icon="alert"
        :title="t('orderDetail.notFound')"
        :action-label="t('errorBoundary.retry')"
        @action="debouncedLoadOrder()"
      />

      <div v-else class="space-y-5">
        <!-- 订单总览头卡 -->
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
                <RouterLink :to="`/pay?order_no=${order.order_no}`">{{ t('orderDetail.payNow') }}</RouterLink>
              </Button>
              <Button v-if="order.status === 'pending_payment'" variant="destructive" size="sm" class="rounded-[10px] px-4" @click="cancelOrder">
                {{ t('orderDetail.cancel') }}
              </Button>
            </div>
          </div>
        </section>

        <AtelierOrderBody
          :order="order"
          show-wallet-fields
          :status-label="statusLabel"
          :status-variant="statusVariant"
          :fulfillment-type-label-text="fulfillmentTypeLabelText"
          :fulfillment-status-label-text="fulfillmentStatusLabelText"
          :format-date="formatDate"
          :get-localized-text="getLocalizedText"
          :format-money="formatMoney"
          :format-discount-money="formatDiscountMoney"
          :has-discount-amount="hasDiscountAmount"
          :has-amount="hasAmount"
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
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import EmptyState from '../../components/EmptyState.vue'
import BreadcrumbNav from '../../components/BreadcrumbNav.vue'
import AtelierOrderBody from './components/AtelierOrderBody.vue'
import { useOrderDetail } from '../../composables/useOrderDetail'

const { t } = useI18n()

const {
  loading, order, debouncedLoadOrder, cancelOrder,
  statusLabel, statusVariant, fulfillmentTypeLabelText, fulfillmentStatusLabelText,
  formatDate, getLocalizedText, formatMoney, formatDiscountMoney, hasDiscountAmount, hasAmount,
  refundReasonText, showRefundRecordsCard, refundRecords, showTimeCard,
  orderItemImage, orderItemSkuText, manualSubmissionRows,
  hasItemDiscount, formatItemDiscountTotal, formatItemPaidAmount, resolvedChildStatus,
  fulfillmentDeliveryLines, instructionBlocks, isFulfillmentTruncated,
  fulfillmentCopied, handleCopyFulfillment, fulfillmentDownloading, handleDownloadFulfillment,
} = useOrderDetail()
</script>
