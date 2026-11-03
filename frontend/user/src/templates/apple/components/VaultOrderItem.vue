<template>
  <div class="flex flex-wrap justify-between gap-3.5 border-b py-3.5 first:pt-0 last:border-b-0 last:pb-0 max-[640px]:flex-col">
    <div class="flex min-w-0 flex-1 gap-3 max-[640px]:flex-none">
      <div class="grid h-[60px] w-[60px] flex-none place-items-center overflow-hidden rounded-sm border bg-secondary">
        <img
          v-if="orderItemImage(item)"
          :src="orderItemImage(item)"
          :alt="getLocalizedText(item.title)"
          loading="lazy"
          decoding="async"
          class="h-full w-full object-cover"
        />
        <ImageIcon v-else :stroke-width="1.5" class="h-[22px] w-[22px] text-muted-foreground" />
      </div>
      <div class="min-w-0">
        <div class="font-bold">{{ getLocalizedText(item.title) }}</div>
        <div class="mt-0.5 text-[12.5px] text-muted-foreground">{{ t('orderDetail.quantityLabel') }}：{{ item.quantity }}</div>
        <div v-if="orderItemSkuText(item)" class="mt-0.5 text-[12.5px] text-muted-foreground">{{ t('orderDetail.itemSkuLabel') }}：{{ orderItemSkuText(item) }}</div>
        <div class="mt-0.5 text-[12.5px] text-muted-foreground">{{ t('orderDetail.itemFulfillmentLabel') }}：{{ fulfillmentTypeLabelText(item.fulfillment_type) }}</div>
        <div v-if="item.tags && item.tags.length" class="mt-2 flex flex-wrap gap-1.5">
          <Badge v-for="(tag, index) in item.tags" :key="index" variant="neutral" size="sm" class="rounded-full">{{ tag }}</Badge>
        </div>
        <div v-if="manualSubmissionRows(item.manual_form_submission, item.manual_form_schema_snapshot).length" class="mt-2.5 rounded-sm border bg-secondary px-3 py-2.5 text-[12.5px] text-muted-foreground">
          <div class="mb-1.5 font-bold text-muted-foreground">{{ t('orderDetail.manualSubmissionTitle') }}</div>
          <div v-for="row in manualSubmissionRows(item.manual_form_submission, item.manual_form_schema_snapshot)" :key="row.key" class="mb-0.5 last:mb-0">
            <span class="font-semibold text-foreground">{{ row.label }}</span>：{{ row.value }}
          </div>
        </div>
      </div>
    </div>
    <div class="grid min-w-[180px] content-start gap-0.5 text-right text-[12.5px] text-muted-foreground max-[640px]:min-w-0 max-[640px]:pl-[72px] max-[640px]:text-left">
      <div>{{ t('orderDetail.unitPriceLabel') }}：{{ formatMoney(item.original_unit_price, currency) }}</div>
      <div>{{ t('orderDetail.totalPriceLabel') }}：{{ formatMoney(item.original_total_price, currency) }}</div>
      <div v-if="hasDiscountAmount(item.coupon_discount_amount)" class="text-destructive">{{ t('orderDetail.couponDiscountLabel') }}：{{ formatDiscountMoney(item.coupon_discount_amount, currency) }}</div>
      <div v-if="hasDiscountAmount(item.promotion_discount_amount)" class="text-destructive">{{ t('orderDetail.promotionDiscountLabel') }}：{{ formatDiscountMoney(item.promotion_discount_amount, currency) }}</div>
      <div v-if="hasDiscountAmount(item.wholesale_discount_amount)" class="text-destructive">{{ t('orderDetail.wholesaleDiscountLabel') }}：{{ formatDiscountMoney(item.wholesale_discount_amount, currency) }}</div>
      <div v-if="hasDiscountAmount(item.member_discount_amount)" class="text-destructive">{{ t('orderDetail.memberDiscountLabel') }}：{{ formatDiscountMoney(item.member_discount_amount, currency) }}</div>
      <div v-if="hasItemDiscount(item)" class="font-bold text-destructive">{{ t('orderDetail.itemDiscountTotalLabel') }}：{{ formatItemDiscountTotal(item, currency) }}</div>
      <div class="mt-0.5 font-bold text-foreground">{{ t('orderDetail.itemPaidAmountLabel') }}：{{ formatItemPaidAmount(item, currency) }}</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Image as ImageIcon } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { useOrderDisplayHelpers } from '../../../composables/useOrderDisplayHelpers'

defineProps<{
  item: any
  currency?: string
}>()

const { t } = useI18n()

const {
  getLocalizedText, orderItemImage, orderItemSkuText, fulfillmentTypeLabelText, manualSubmissionRows,
  formatMoney, hasDiscountAmount, formatDiscountMoney, hasItemDiscount, formatItemDiscountTotal, formatItemPaidAmount,
} = useOrderDisplayHelpers(ref<any>(null))
</script>
