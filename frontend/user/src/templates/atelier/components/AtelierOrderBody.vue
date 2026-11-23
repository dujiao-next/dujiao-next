<template>
  <div class="space-y-5">
    <!-- 子订单（拆分交付） -->
    <section v-if="order.children && order.children.length > 0" class="atelier-card rounded-[16px] bg-card p-5 md:p-6">
      <h2 class="mb-4 border-b pb-3 text-base font-bold tracking-[-0.015em]">{{ t('orderDetail.childOrdersTitle') }}</h2>
      <div class="space-y-4">
        <div v-for="child in order.children" :key="child.id" class="rounded-[16px] border bg-background p-4 md:p-5">
          <div class="flex flex-col justify-between gap-3 border-b pb-3.5 md:flex-row md:items-center">
            <div>
              <div class="font-mono text-xs text-muted-foreground">
                {{ t('orderDetail.childOrderNo') }}：<span class="font-semibold text-foreground">{{ child.order_no }}</span>
              </div>
              <div class="mt-1 text-xs text-muted-foreground tabular-nums">
                {{ t('orderDetail.childOrderAmount') }}：{{ formatMoney(child.total_amount, child.currency || order.currency) }}
              </div>
            </div>
            <Badge :variant="statusVariant(resolvedChildStatus(child))" size="sm" class="self-start md:self-auto">
              {{ statusLabel(resolvedChildStatus(child)) }}
            </Badge>
          </div>

          <!-- 子订单商品 -->
          <div class="mt-4">
            <h3 class="mb-3 text-xs font-bold tracking-[0.04em] text-muted-foreground uppercase">
              {{ t('orderDetail.childItemsTitle') }}
            </h3>
            <div v-if="child.items && child.items.length" class="divide-y">
              <div
                v-for="(item, cidx) in child.items"
                :key="cidx"
                class="flex flex-col justify-between gap-3 py-3 first:pt-0 last:pb-0 text-sm text-muted-foreground sm:flex-row sm:items-start"
              >
                <div class="flex min-w-0 items-start gap-3">
                  <div class="h-14 w-14 shrink-0 overflow-hidden rounded-[12px] border bg-card sm:h-16 sm:w-16">
                    <SmartImage :src="orderItemImage(item)" :alt="getLocalizedText(item.title)" img-class="h-full w-full object-cover" />
                  </div>
                  <div class="min-w-0">
                    <div class="font-semibold text-foreground">{{ getLocalizedText(item.title) }}</div>
                    <div class="mt-0.5 text-xs text-muted-foreground">{{ t('orderDetail.quantityLabel') }}：{{ item.quantity }}</div>
                    <div v-if="orderItemSkuText(item)" class="mt-0.5 text-xs text-muted-foreground">
                      {{ t('orderDetail.itemSkuLabel') }}：{{ orderItemSkuText(item) }}
                    </div>
                    <div class="mt-0.5 text-xs text-muted-foreground">
                      {{ t('orderDetail.itemFulfillmentLabel') }}：{{ fulfillmentTypeLabelText(item.fulfillment_type) }}
                    </div>
                    <div v-if="item.tags && item.tags.length" class="mt-2 flex flex-wrap gap-1.5">
                      <Badge v-for="(tag, index) in item.tags" :key="index" variant="neutral" size="sm" class="rounded-[6px]">
                        {{ tag }}
                      </Badge>
                    </div>
                    <div
                      v-if="manualSubmissionRows(item.manual_form_submission, item.manual_form_schema_snapshot).length"
                      class="mt-3 rounded-[12px] border bg-card p-3 text-xs text-muted-foreground"
                    >
                      <div class="mb-1.5 font-semibold text-foreground">{{ t('orderDetail.manualSubmissionTitle') }}</div>
                      <div
                        v-for="row in manualSubmissionRows(item.manual_form_submission, item.manual_form_schema_snapshot)"
                        :key="row.key"
                        class="mb-1 last:mb-0"
                      >
                        <span class="text-foreground">{{ row.label }}</span>：{{ row.value }}
                      </div>
                    </div>
                  </div>
                </div>
                <div class="shrink-0 space-y-1 pl-[4.25rem] text-left text-xs tabular-nums text-muted-foreground sm:pl-0 sm:text-right">
                  <div>{{ t('orderDetail.unitPriceLabel') }}：{{ formatMoney(item.original_unit_price, order.currency) }}</div>
                  <div>{{ t('orderDetail.totalPriceLabel') }}：{{ formatMoney(item.original_total_price, order.currency) }}</div>
                  <div v-if="hasDiscountAmount(item.coupon_discount_amount)">
                    {{ t('orderDetail.couponDiscountLabel') }}：{{ formatDiscountMoney(item.coupon_discount_amount, order.currency) }}
                  </div>
                  <div v-if="hasDiscountAmount(item.promotion_discount_amount)">
                    {{ t('orderDetail.promotionDiscountLabel') }}：{{ formatDiscountMoney(item.promotion_discount_amount, order.currency) }}
                  </div>
                  <div v-if="hasDiscountAmount(item.member_discount_amount)">
                    {{ t('orderDetail.memberDiscountLabel') }}：{{ formatDiscountMoney(item.member_discount_amount, order.currency) }}
                  </div>
                  <div v-if="hasItemDiscount(item)" class="font-medium text-danger">
                    {{ t('orderDetail.itemDiscountTotalLabel') }}：{{ formatItemDiscountTotal(item, order.currency) }}
                  </div>
                  <div class="text-sm font-semibold text-foreground">
                    {{ t('orderDetail.itemPaidAmountLabel') }}：{{ formatItemPaidAmount(item, order.currency) }}
                  </div>
                </div>
              </div>
            </div>
            <div v-else class="text-sm text-muted-foreground">{{ t('orderDetail.noItems') }}</div>
          </div>

          <!-- 子订单交付凭证 -->
          <div class="mt-4 border-t pt-4">
            <div class="mb-3 flex items-center justify-between gap-2">
              <h3 class="text-xs font-bold tracking-[0.04em] text-muted-foreground uppercase">
                {{ t('orderDetail.childFulfillmentTitle') }}
              </h3>
              <Button
                v-if="child.fulfillment?.status === 'delivered'"
                size="sm"
                class="h-8 rounded-[9px] bg-primary px-3.5 text-xs font-semibold text-primary-foreground hover:bg-primary/90"
                @click="$emit('copyFulfillment', child.fulfillment)"
              >
                <Copy v-if="!fulfillmentCopied" class="h-3.5 w-3.5" :stroke-width="2" />
                <Check v-else class="h-3.5 w-3.5" :stroke-width="2" />
                {{ fulfillmentCopied ? t('orderDetail.fulfillmentCopied') : t('orderDetail.fulfillmentCopy') }}
              </Button>
            </div>
            <div v-if="child.fulfillment" class="space-y-2">
              <div class="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
                <span>{{ t('orderDetail.fulfillmentType') }}：{{ fulfillmentTypeLabelText(child.fulfillment.type) }}</span>
                <span>{{ t('orderDetail.fulfillmentStatus') }}：{{ fulfillmentStatusLabelText(child.fulfillment.status) }}</span>
              </div>
              <div v-if="isFulfillmentTruncated(child.fulfillment)" class="mt-2 space-y-2">
                <div class="flex flex-wrap items-center justify-between gap-2">
                  <span class="text-xs text-muted-foreground">
                    {{ t('orderDetail.fulfillmentTotalLines', { count: child.fulfillment.payload_line_count }) }}
                  </span>
                  <Button
                    size="sm"
                    class="h-8 rounded-[9px] bg-primary px-3.5 text-xs font-semibold text-primary-foreground hover:bg-primary/90"
                    :disabled="fulfillmentDownloading"
                    @click="$emit('downloadFulfillment', child.order_no || order.order_no)"
                  >
                    <Download class="h-3.5 w-3.5" :stroke-width="2" />
                    {{ fulfillmentDownloading ? t('orderDetail.fulfillmentDownloading') : t('orderDetail.fulfillmentDownload') }}
                  </Button>
                </div>
                <div class="rounded-[10px] bg-warning-soft px-3 py-2 text-xs text-warning">
                  {{ t('orderDetail.fulfillmentTruncatedHint') }}
                </div>
                <div class="max-h-48 overflow-y-auto break-all whitespace-pre-wrap rounded-[12px] border bg-card p-3.5 font-mono text-xs text-foreground">
                  {{ child.fulfillment.payload }}
                </div>
              </div>
              <div
                v-else-if="fulfillmentDeliveryLines(child.fulfillment).length"
                class="space-y-1 break-all rounded-[12px] border bg-card p-3.5 font-mono text-xs text-foreground"
              >
                <div v-for="(line, index) in fulfillmentDeliveryLines(child.fulfillment)" :key="`at-child-f-${child.id}-${index}`">
                  {{ line }}
                </div>
              </div>
              <div
                v-else-if="child.fulfillment.payload"
                class="break-all whitespace-pre-wrap rounded-[12px] border bg-card p-3.5 font-mono text-xs text-foreground"
              >
                {{ child.fulfillment.payload }}
              </div>
              <div v-if="child.fulfillment.status === 'delivered' && instructionBlocks(child.items).length" class="mt-3 space-y-2.5">
                <div
                  v-for="(block, bi) in instructionBlocks(child.items)"
                  :key="`at-child-inst-${child.id}-${bi}`"
                  class="rounded-[12px] border bg-card p-4"
                >
                  <div class="mb-2 flex items-center gap-1.5 text-xs font-bold text-foreground">
                    <Info class="h-3.5 w-3.5" />
                    {{ t('orderDetail.instructionsTitle') }}
                  </div>
                  <div class="prose prose-sm max-w-none break-words text-muted-foreground dark:prose-invert [&_img]:max-h-80 [&_img]:w-auto" v-html="block.html"></div>
                </div>
              </div>
            </div>
            <div v-else class="text-xs text-muted-foreground">{{ t('orderDetail.childFulfillmentEmpty') }}</div>
          </div>
        </div>
      </div>
    </section>

    <!-- 顶层交付凭证 -->
    <section v-if="order.fulfillment" class="atelier-card rounded-[16px] bg-card p-5 md:p-6">
      <div class="mb-4 flex flex-wrap items-center justify-between gap-2 border-b pb-3">
        <h2 class="text-base font-bold tracking-[-0.015em]">{{ t('orderDetail.fulfillmentTitle') }}</h2>
        <div class="flex items-center gap-2">
          <Button
            v-if="isFulfillmentTruncated(order.fulfillment)"
            size="sm"
            class="h-8.5 rounded-[9px] bg-primary px-4 text-xs font-semibold text-primary-foreground hover:bg-primary/90"
            :disabled="fulfillmentDownloading"
            @click="$emit('downloadFulfillment', order.order_no)"
          >
            <Download class="h-3.5 w-3.5" :stroke-width="2" />
            {{ fulfillmentDownloading ? t('orderDetail.fulfillmentDownloading') : t('orderDetail.fulfillmentDownload') }}
          </Button>
          <Button
            v-if="order.fulfillment.status === 'delivered' && !isFulfillmentTruncated(order.fulfillment)"
            size="sm"
            class="h-8.5 rounded-[9px] bg-primary px-4 text-xs font-semibold text-primary-foreground hover:bg-primary/90"
            @click="$emit('copyFulfillment', order.fulfillment)"
          >
            <Copy v-if="!fulfillmentCopied" class="h-3.5 w-3.5" :stroke-width="2" />
            <Check v-else class="h-3.5 w-3.5" :stroke-width="2" />
            {{ fulfillmentCopied ? t('orderDetail.fulfillmentCopied') : t('orderDetail.fulfillmentCopy') }}
          </Button>
        </div>
      </div>

      <div class="flex flex-wrap gap-x-5 gap-y-1 text-xs text-muted-foreground">
        <span>{{ t('orderDetail.fulfillmentType') }}：{{ fulfillmentTypeLabelText(order.fulfillment.type) }}</span>
        <span>{{ t('orderDetail.fulfillmentStatus') }}：{{ fulfillmentStatusLabelText(order.fulfillment.status) }}</span>
      </div>

      <div v-if="isFulfillmentTruncated(order.fulfillment)" class="mt-3.5 space-y-2">
        <div class="text-xs text-muted-foreground">
          {{ t('orderDetail.fulfillmentTotalLines', { count: order.fulfillment.payload_line_count }) }}
        </div>
        <div class="rounded-[10px] bg-warning-soft px-3 py-2 text-xs text-warning">
          {{ t('orderDetail.fulfillmentTruncatedHint') }}
        </div>
        <div class="max-h-64 overflow-y-auto break-all whitespace-pre-wrap rounded-[14px] border bg-background p-4 font-mono text-xs text-foreground">
          {{ order.fulfillment.payload }}
        </div>
      </div>
      <div
        v-else-if="fulfillmentDeliveryLines(order.fulfillment).length"
        class="mt-3.5 space-y-1 break-all rounded-[14px] border bg-background p-4 font-mono text-xs text-foreground"
      >
        <div v-for="(line, index) in fulfillmentDeliveryLines(order.fulfillment)" :key="`at-order-f-${index}`">
          {{ line }}
        </div>
      </div>
      <div
        v-else
        class="mt-3.5 break-all whitespace-pre-wrap rounded-[14px] border bg-background p-4 font-mono text-xs text-foreground"
      >
        {{ order.fulfillment.payload }}
      </div>

      <div v-if="order.fulfillment.status === 'delivered' && instructionBlocks(order.items).length" class="mt-4 space-y-3">
        <div v-for="(block, bi) in instructionBlocks(order.items)" :key="`at-order-inst-${bi}`" class="rounded-[14px] border bg-background p-4">
          <div class="mb-2 flex items-center gap-1.5 text-xs font-bold text-foreground">
            <Info class="h-3.5 w-3.5" />
            {{ t('orderDetail.instructionsTitle') }}
          </div>
          <div class="prose prose-sm max-w-none break-words text-muted-foreground dark:prose-invert [&_img]:max-h-80 [&_img]:w-auto" v-html="block.html"></div>
        </div>
      </div>
    </section>

    <!-- 商品明细 -->
    <section class="atelier-card rounded-[16px] bg-card p-5 md:p-6">
      <h2 class="mb-4 border-b pb-3 text-base font-bold tracking-[-0.015em]">{{ t('orderDetail.itemsTitle') }}</h2>
      <div v-if="order.items && order.items.length > 0" class="divide-y">
        <div
          v-for="(item, idx) in order.items"
          :key="idx"
          class="flex flex-col justify-between gap-3 py-3.5 first:pt-0 last:pb-0 sm:flex-row sm:items-start"
        >
          <div class="flex min-w-0 items-start gap-3.5">
            <div class="h-14 w-14 shrink-0 overflow-hidden rounded-[12px] border bg-background sm:h-16 sm:w-16">
              <SmartImage :src="orderItemImage(item)" :alt="getLocalizedText(item.title)" img-class="h-full w-full object-cover" />
            </div>
            <div class="min-w-0">
              <div class="text-[15px] font-semibold text-foreground">{{ getLocalizedText(item.title) }}</div>
              <div class="mt-0.5 text-xs text-muted-foreground">{{ t('orderDetail.quantityLabel') }}：{{ item.quantity }}</div>
              <div v-if="orderItemSkuText(item)" class="mt-0.5 text-xs text-muted-foreground">
                {{ t('orderDetail.itemSkuLabel') }}：{{ orderItemSkuText(item) }}
              </div>
              <div class="mt-0.5 text-xs text-muted-foreground">
                {{ t('orderDetail.itemFulfillmentLabel') }}：{{ fulfillmentTypeLabelText(item.fulfillment_type) }}
              </div>
              <div v-if="item.tags && item.tags.length" class="mt-2 flex flex-wrap gap-1.5">
                <Badge v-for="(tag, index) in item.tags" :key="index" variant="neutral" size="sm" class="rounded-[6px]">
                  {{ tag }}
                </Badge>
              </div>
              <div
                v-if="manualSubmissionRows(item.manual_form_submission, item.manual_form_schema_snapshot).length"
                class="mt-3 rounded-[12px] border bg-background p-3 text-xs text-muted-foreground"
              >
                <div class="mb-1.5 font-semibold text-foreground">{{ t('orderDetail.manualSubmissionTitle') }}</div>
                <div
                  v-for="row in manualSubmissionRows(item.manual_form_submission, item.manual_form_schema_snapshot)"
                  :key="row.key"
                  class="mb-1 last:mb-0"
                >
                  <span class="text-foreground">{{ row.label }}</span>：{{ row.value }}
                </div>
              </div>
            </div>
          </div>

          <div class="shrink-0 space-y-1 pl-[4.5rem] text-left text-xs tabular-nums text-muted-foreground sm:pl-0 sm:text-right">
            <div>{{ t('orderDetail.unitPriceLabel') }}：{{ formatMoney(item.original_unit_price, order.currency) }}</div>
            <div>{{ t('orderDetail.totalPriceLabel') }}：{{ formatMoney(item.original_total_price, order.currency) }}</div>
            <div v-if="hasDiscountAmount(item.coupon_discount_amount)">
              {{ t('orderDetail.couponDiscountLabel') }}：{{ formatDiscountMoney(item.coupon_discount_amount, order.currency) }}
            </div>
            <div v-if="hasDiscountAmount(item.promotion_discount_amount)">
              {{ t('orderDetail.promotionDiscountLabel') }}：{{ formatDiscountMoney(item.promotion_discount_amount, order.currency) }}
            </div>
            <div v-if="hasDiscountAmount(item.wholesale_discount_amount)">
              {{ t('orderDetail.wholesaleDiscountLabel') }}：{{ formatDiscountMoney(item.wholesale_discount_amount, order.currency) }}
            </div>
            <div v-if="hasDiscountAmount(item.member_discount_amount)">
              {{ t('orderDetail.memberDiscountLabel') }}：{{ formatDiscountMoney(item.member_discount_amount, order.currency) }}
            </div>
            <div v-if="hasItemDiscount(item)" class="font-medium text-danger">
              {{ t('orderDetail.itemDiscountTotalLabel') }}：{{ formatItemDiscountTotal(item, order.currency) }}
            </div>
            <div class="text-sm font-semibold text-foreground">
              {{ t('orderDetail.itemPaidAmountLabel') }}：{{ formatItemPaidAmount(item, order.currency) }}
            </div>
          </div>
        </div>
      </div>
      <div v-else class="text-sm text-muted-foreground">{{ t('orderDetail.noItems') }}</div>
    </section>

    <!-- 金额明细 -->
    <section class="atelier-card rounded-[16px] bg-card p-5 md:p-6">
      <h2 class="mb-4 border-b pb-3 text-base font-bold tracking-[-0.015em]">{{ t('orderDetail.amountTitle') }}</h2>
      <div class="grid grid-cols-1 gap-3 text-sm sm:grid-cols-3">
        <div class="rounded-[14px] border bg-background p-3.5">
          <div class="text-xs text-muted-foreground">{{ t('orderDetail.amountOriginal') }}</div>
          <div class="mt-1 font-semibold tabular-nums text-foreground">{{ formatMoney(order.original_amount, order.currency) }}</div>
        </div>
        <div class="rounded-[14px] border bg-background p-3.5">
          <div class="text-xs text-muted-foreground">{{ t('orderDetail.amountDiscount') }}</div>
          <div class="mt-1 font-semibold tabular-nums" :class="hasDiscountAmount(order.discount_amount) ? 'text-danger' : 'text-foreground'">
            {{ formatDiscountMoney(order.discount_amount, order.currency) }}
          </div>
        </div>
        <div class="rounded-[14px] border bg-background p-3.5">
          <div class="text-xs text-muted-foreground">{{ t('orderDetail.amountTotal') }}</div>
          <div class="mt-1 font-bold tabular-nums text-foreground">{{ formatMoney(order.total_amount, order.currency) }}</div>
        </div>
        <div v-if="showWalletFields && hasAmount(order.wallet_paid_amount)" class="rounded-[14px] border bg-background p-3.5">
          <div class="text-xs text-muted-foreground">{{ t('orderDetail.amountWalletPaid') }}</div>
          <div class="mt-1 font-semibold tabular-nums text-foreground">{{ formatMoney(order.wallet_paid_amount, order.currency) }}</div>
        </div>
        <div v-if="showWalletFields && hasAmount(order.online_paid_amount)" class="rounded-[14px] border bg-background p-3.5">
          <div class="text-xs text-muted-foreground">{{ t('orderDetail.amountOnlinePaid') }}</div>
          <div class="mt-1 font-semibold tabular-nums text-foreground">{{ formatMoney(order.online_paid_amount, order.currency) }}</div>
        </div>
        <div v-if="showWalletFields && hasAmount(order.refunded_amount)" class="rounded-[14px] border bg-background p-3.5">
          <div class="text-xs text-muted-foreground">{{ t('orderDetail.amountRefunded') }}</div>
          <div class="mt-1 font-semibold tabular-nums text-foreground">{{ formatMoney(order.refunded_amount, order.currency) }}</div>
        </div>
        <div v-if="hasDiscountAmount(order.member_discount_amount)" class="rounded-[14px] bg-warning-soft p-3.5 text-warning">
          <div class="text-xs">{{ t('orderDetail.amountMemberDiscount') }}</div>
          <div class="mt-1 font-semibold tabular-nums">{{ formatDiscountMoney(order.member_discount_amount, order.currency) }}</div>
        </div>
        <div v-if="hasDiscountAmount(order.wholesale_discount_amount)" class="rounded-[14px] bg-success-soft p-3.5 text-success">
          <div class="text-xs">{{ t('orderDetail.amountWholesaleDiscount') }}</div>
          <div class="mt-1 font-semibold tabular-nums">{{ formatDiscountMoney(order.wholesale_discount_amount, order.currency) }}</div>
        </div>
      </div>
    </section>

    <!-- 退款记录 -->
    <section v-if="showRefundRecordsCard" class="atelier-card rounded-[16px] bg-card p-5 md:p-6">
      <h2 class="mb-4 border-b pb-3 text-base font-bold tracking-[-0.015em]">{{ t('orderDetail.refundRecordsTitle') }}</h2>
      <div v-if="refundRecords.length > 0" class="overflow-x-auto rounded-[14px] border">
        <Table>
          <TableHeader>
            <TableRow class="bg-background">
              <TableHead class="px-4">{{ t('orderDetail.refundRecordTime') }}</TableHead>
              <TableHead class="px-4">{{ t('orderDetail.refundRecordAmount') }}</TableHead>
              <TableHead class="px-4">{{ t('orderDetail.refundRecordReason') }}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-for="(record, idx) in refundRecords" :key="`at-refund-row-${idx}`">
              <TableCell class="whitespace-nowrap px-4 font-mono text-xs text-muted-foreground">{{ formatDate(record.created_at) }}</TableCell>
              <TableCell class="whitespace-nowrap px-4 text-sm font-semibold tabular-nums text-foreground">
                {{ formatMoney(record.amount, record.currency || order.currency) }}
              </TableCell>
              <TableCell class="break-words whitespace-pre-wrap px-4 text-xs text-muted-foreground">
                {{ refundReasonText(record.remark) }}
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>
      <div v-else class="text-sm text-muted-foreground">{{ t('orderDetail.refundRecordsEmpty') }}</div>
    </section>

    <!-- 时间节点 -->
    <section v-if="showTimeCard" class="atelier-card rounded-[16px] bg-card p-5 md:p-6">
      <h2 class="mb-4 border-b pb-3 text-base font-bold tracking-[-0.015em]">{{ t('orderDetail.timeTitle') }}</h2>
      <div class="grid grid-cols-1 gap-3 text-sm sm:grid-cols-3">
        <div class="rounded-[14px] border bg-background p-3.5">
          <div class="text-xs text-muted-foreground">{{ t('orderDetail.createdAtLabel') }}</div>
          <div class="mt-1 font-mono text-xs text-foreground">{{ formatDate(order.created_at) }}</div>
        </div>
        <div v-if="order.paid_at" class="rounded-[14px] border bg-background p-3.5">
          <div class="text-xs text-muted-foreground">{{ t('orderDetail.paidAtLabel') }}</div>
          <div class="mt-1 font-mono text-xs text-foreground">{{ formatDate(order.paid_at) }}</div>
        </div>
        <div v-if="order.expires_at" class="rounded-[14px] border bg-background p-3.5">
          <div class="text-xs text-muted-foreground">{{ t('orderDetail.expiresAtLabel') }}</div>
          <div class="mt-1 font-mono text-xs text-foreground">{{ formatDate(order.expires_at) }}</div>
        </div>
        <div v-if="order.canceled_at" class="rounded-[14px] border bg-background p-3.5">
          <div class="text-xs text-muted-foreground">{{ t('orderDetail.canceledAtLabel') }}</div>
          <div class="mt-1 font-mono text-xs text-foreground">{{ formatDate(order.canceled_at) }}</div>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Check, Copy, Download, Info } from 'lucide-vue-next'
import { Badge, type BadgeVariants } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import SmartImage from '../../../components/SmartImage.vue'

withDefaults(defineProps<{
  order: any
  showWalletFields?: boolean
  statusLabel: (status: string) => string
  statusVariant: (status: string) => BadgeVariants['variant']
  fulfillmentTypeLabelText: (type: string) => string
  fulfillmentStatusLabelText: (status: string) => string
  formatDate: (raw?: string) => string
  getLocalizedText: (jsonData: any) => string
  formatMoney: (amount?: string, currency?: string) => string
  formatDiscountMoney: (amount?: string, currency?: string) => string
  hasDiscountAmount: (amount?: string) => boolean
  hasAmount?: (amount?: string) => boolean
  refundReasonText: (remark?: string) => string
  showRefundRecordsCard: boolean
  refundRecords: any[]
  showTimeCard: boolean
  orderItemImage: (item: any) => string
  orderItemSkuText: (item: any) => string
  manualSubmissionRows: (submission?: any, schema?: any) => Array<{ key: string; label: string; value: string }>
  hasItemDiscount: (item: any) => boolean
  formatItemDiscountTotal: (item: any, currency?: string) => string
  formatItemPaidAmount: (item: any, currency?: string) => string
  resolvedChildStatus: (child: any) => string
  fulfillmentDeliveryLines: (fulfillment?: any) => string[]
  instructionBlocks: (items?: any[]) => Array<{ html: string }>
  isFulfillmentTruncated: (fulfillment?: any) => boolean
  fulfillmentCopied: boolean
  fulfillmentDownloading: boolean
}>(), {
  showWalletFields: false,
  hasAmount: () => false,
})

defineEmits<{
  copyFulfillment: [fulfillment: any]
  downloadFulfillment: [orderNo: string]
}>()

const { t } = useI18n()
</script>
