<template>
  <div class="flex flex-col gap-4">
    <!-- ==================== 卡密 / 交付内容 ==================== -->
    <section v-if="secretLines.length" class="rounded-[var(--r-lg)] border border-border bg-card p-5">
      <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
        <h2 class="text-[15px] font-semibold tracking-[-0.015em]">{{ t('orderDetail.fulfillmentTitle') }}</h2>
        <button
          type="button"
          class="rounded-full border border-border px-3.5 py-1.5 text-[12.5px] text-muted-foreground transition-colors hover:border-[color:var(--border-strong)] hover:text-foreground"
          @click="handleCopyFulfillment(order.fulfillment)"
        >{{ fulfillmentCopied ? t('orderDetail.fulfillmentCopied') : t('orderDetail.fulfillmentCopy') }}</button>
      </div>

      <div class="flex flex-col gap-2">
        <div
          v-for="(line, idx) in secretLines"
          :key="idx"
          class="grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-3 rounded-[var(--r-sm)] border border-[color:var(--border-soft)] bg-[color:var(--surface-2)] px-3 py-2.5"
        >
          <span class="text-[11.5px] tabular-nums text-muted-foreground">#{{ idx + 1 }}</span>
          <span class="break-all font-mono text-[13px] text-foreground">{{ line }}</span>
          <button
            type="button"
            class="grid h-7 w-7 place-items-center rounded-[var(--r-xs)] text-muted-foreground transition-colors hover:bg-card hover:text-foreground"
            :aria-label="t('ember.common.copy')"
            @click="copyLine(line)"
          >
            <Check v-if="copiedLine === line" class="h-3.5 w-3.5 text-[color:var(--success)]" />
            <Copy v-else class="h-3.5 w-3.5" />
          </button>
        </div>
      </div>

      <div v-if="deliveryRows.length" class="mt-4 flex flex-col gap-1.5 rounded-[var(--r)] border border-[color:var(--border-soft)] bg-[color:var(--surface-2)] p-3.5">
        <div v-for="(row, idx) in deliveryRows" :key="idx" class="flex flex-wrap justify-between gap-3 text-[13px]">
          <span class="text-muted-foreground">{{ row.key }}</span>
          <span class="break-all text-right font-medium">{{ row.value }}</span>
        </div>
      </div>

      <button
        v-if="variant === 'member'"
        type="button"
        class="mt-4 rounded-full border border-border px-4 py-2 text-[13px] text-muted-foreground transition-colors hover:border-[color:var(--border-strong)] hover:text-foreground disabled:opacity-50"
        :disabled="fulfillmentDownloading"
        @click="$emit('download')"
      >{{ t('orderDetail.fulfillmentDownload') }}</button>
    </section>

    <!-- ==================== 使用说明 ==================== -->
    <section v-if="instructions.length" class="rounded-[var(--r-lg)] border border-border bg-card p-5">
      <h2 class="mb-4 text-[15px] font-semibold tracking-[-0.015em]">{{ t('orderDetail.instructionsTitle') }}</h2>
      <div class="flex flex-col gap-4">
        <div v-for="(block, idx) in instructions" :key="idx">
          <div v-if="block.title" class="mb-1.5 text-[13px] font-medium text-muted-foreground">{{ block.title }}</div>
          <div class="ember-prose text-[14px] leading-[1.7] text-[color:var(--ink-2)]" v-html="block.html"></div>
        </div>
      </div>
    </section>

    <!-- ==================== 商品明细 ==================== -->
    <section class="rounded-[var(--r-lg)] border border-border bg-card p-5">
      <h2 class="mb-4 text-[15px] font-semibold tracking-[-0.015em]">{{ t('orderDetail.itemsTitle') }}</h2>
      <div class="flex flex-col gap-3">
        <div
          v-for="(item, idx) in orderItems"
          :key="idx"
          class="grid grid-cols-[minmax(0,1fr)_auto] items-start gap-4 rounded-[var(--r)] border border-[color:var(--border-soft)] bg-[color:var(--surface-2)] p-4"
        >
          <div class="min-w-0">
            <div class="text-[14px] font-medium">{{ getLocalizedText(item.title) }}</div>
            <div class="mt-1 text-[12px] text-muted-foreground">
              {{ t('orderDetail.quantityLabel') }}：{{ item.quantity }} · {{ t('orderDetail.itemFulfillmentLabel') }}：{{ fulfillmentTypeLabelText(item.fulfillment_type) }}
            </div>
            <div v-if="orderItemSkuText(item)" class="mt-0.5 text-[12px] text-muted-foreground">
              {{ t('orderDetail.itemSkuLabel') }}：{{ orderItemSkuText(item) }}
            </div>
            <div v-if="hasItemDiscount(item)" class="mt-0.5 text-[12px] text-[color:var(--accent-strong)]">
              {{ t('orderDetail.itemDiscountTotalLabel') }}：-{{ formatItemDiscountTotal(item, order.currency) }}
            </div>

            <!-- 手动发货表单快照 -->
            <dl v-if="manualRows(item).length" class="mt-3 flex flex-col gap-1">
              <div v-for="row in manualRows(item)" :key="row.key" class="flex flex-wrap gap-2 text-[12px]">
                <dt class="text-muted-foreground">{{ row.label }}</dt>
                <dd class="text-foreground">{{ row.value }}</dd>
              </div>
            </dl>
          </div>

          <div class="text-right">
            <div class="text-[14px] font-semibold tabular-nums">{{ formatItemPaidAmount(item, order.currency) }}</div>
            <div v-if="hasItemDiscount(item)" class="mt-0.5 text-[11.5px] text-muted-foreground line-through">
              {{ formatMoney(item.total_price, order.currency) }}
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- ==================== 金额 ==================== -->
    <section class="rounded-[var(--r-lg)] border border-border bg-card p-5">
      <h2 class="mb-4 text-[15px] font-semibold tracking-[-0.015em]">{{ t('orderDetail.amountTitle') }}</h2>
      <div class="flex flex-col gap-2.5">
        <div class="flex items-center justify-between text-[13.5px]">
          <span class="text-muted-foreground">{{ t('orderDetail.amountOriginal') }}</span>
          <span class="tabular-nums">{{ formatMoney(order.original_amount, order.currency) }}</span>
        </div>
        <div v-if="hasDiscountAmount(order.discount_amount)" class="flex items-center justify-between text-[13.5px]">
          <span class="text-muted-foreground">{{ t('orderDetail.amountDiscount') }}</span>
          <span class="tabular-nums text-[color:var(--accent-strong)]">{{ formatDiscountMoney(order.discount_amount, order.currency) }}</span>
        </div>
        <div v-if="hasDiscountAmount(order.promotion_discount_amount)" class="flex items-center justify-between text-[13.5px]">
          <span class="text-muted-foreground">{{ t('orderDetail.promotionDiscountLabel') }}</span>
          <span class="tabular-nums text-[color:var(--accent-strong)]">{{ formatDiscountMoney(order.promotion_discount_amount, order.currency) }}</span>
        </div>
        <div v-if="hasDiscountAmount(order.member_discount_amount)" class="flex items-center justify-between text-[13.5px]">
          <span class="text-muted-foreground">{{ t('orderDetail.memberDiscountLabel') }}</span>
          <span class="tabular-nums text-[color:var(--success-strong)]">{{ formatDiscountMoney(order.member_discount_amount, order.currency) }}</span>
        </div>
        <div v-if="hasDiscountAmount(order.wholesale_discount_amount)" class="flex items-center justify-between text-[13.5px]">
          <span class="text-muted-foreground">{{ t('orderDetail.amountWholesaleDiscount') }}</span>
          <span class="tabular-nums text-[color:var(--success-strong)]">{{ formatDiscountMoney(order.wholesale_discount_amount, order.currency) }}</span>
        </div>
        <div v-if="hasAmount(order.wallet_paid_amount)" class="flex items-center justify-between text-[13.5px]">
          <span class="text-muted-foreground">{{ t('orderDetail.amountWalletPaid') }}</span>
          <span class="tabular-nums">{{ formatMoney(order.wallet_paid_amount, order.currency) }}</span>
        </div>
        <div v-if="hasAmount(order.online_paid_amount)" class="flex items-center justify-between text-[13.5px]">
          <span class="text-muted-foreground">{{ t('orderDetail.amountOnlinePaid') }}</span>
          <span class="tabular-nums">{{ formatMoney(order.online_paid_amount, order.currency) }}</span>
        </div>
        <div class="mt-1 flex items-baseline justify-between border-t border-border pt-3">
          <span class="text-[13.5px] text-muted-foreground">{{ t('orderDetail.amountTotal') }}</span>
          <span class="text-[20px] font-semibold tabular-nums tracking-[-0.03em]">{{ formatMoney(order.total_amount, order.currency) }}</span>
        </div>
      </div>
    </section>

    <!-- ==================== 子订单（拆单） ==================== -->
    <section v-if="childOrders.length" class="rounded-[var(--r-lg)] border border-border bg-card p-5">
      <h2 class="mb-4 text-[15px] font-semibold tracking-[-0.015em]">{{ t('orderDetail.childOrdersTitle') }}</h2>
      <div class="flex flex-col gap-3">
        <div
          v-for="child in childOrders"
          :key="child.order_no"
          class="rounded-[var(--r)] border border-[color:var(--border-soft)] bg-[color:var(--surface-2)] p-4"
        >
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div class="flex flex-wrap items-center gap-2.5">
              <span class="ember-pill" :class="pillClass(resolvedChildStatus(child))">
                {{ statusLabel(resolvedChildStatus(child)) }}
              </span>
              <span class="font-mono text-[12px] text-muted-foreground">{{ child.order_no }}</span>
            </div>
            <span class="text-[14px] font-semibold tabular-nums">
              {{ formatMoney(child.total_amount, child.currency || order.currency) }}
            </span>
          </div>

          <!-- 子订单商品 -->
          <div v-if="Array.isArray(child.items) && child.items.length" class="mt-3 flex flex-col gap-1.5">
            <div
              v-for="(item, idx) in child.items"
              :key="idx"
              class="flex flex-wrap justify-between gap-3 text-[12.5px] text-muted-foreground"
            >
              <span>{{ getLocalizedText(item.title) }} × {{ item.quantity }}</span>
              <span class="tabular-nums">{{ formatMoney(item.total_price, child.currency || order.currency) }}</span>
            </div>
          </div>

          <!-- 子订单交付内容 -->
          <div v-if="child.fulfillment?.payload" class="mt-3">
            <div class="mb-1.5 text-[12px] text-muted-foreground">{{ t('orderDetail.childFulfillmentTitle') }}</div>
            <div class="flex flex-col gap-1.5">
              <div
                v-for="(line, idx) in splitPayload(child.fulfillment.payload)"
                :key="idx"
                class="rounded-[var(--r-xs)] border border-[color:var(--border-soft)] bg-card px-3 py-2 font-mono text-[12.5px] break-all"
              >{{ line }}</div>
            </div>
          </div>
          <p v-else class="mt-3 text-[12.5px] text-muted-foreground">{{ t('orderDetail.childFulfillmentEmpty') }}</p>
        </div>
      </div>
    </section>

    <!-- ==================== 退款记录 ==================== -->
    <section v-if="showRefundRecordsCard && refundRecords.length" class="rounded-[var(--r-lg)] border border-border bg-card p-5">
      <h2 class="mb-4 text-[15px] font-semibold tracking-[-0.015em]">{{ t('orderDetail.refundRecordsTitle') }}</h2>
      <div class="flex flex-col gap-3">
        <div
          v-for="(record, idx) in refundRecords"
          :key="idx"
          class="rounded-[var(--r)] border border-[color:var(--border-soft)] bg-[color:var(--surface-2)] p-4"
        >
          <div class="flex flex-wrap items-center justify-between gap-3">
            <span class="text-[13.5px] font-medium">{{ record.type || t('orderDetail.refundRecordsTitle') }}</span>
            <span class="text-[14px] font-semibold tabular-nums text-[color:var(--accent-strong)]">
              {{ formatMoney(record.amount, record.currency || order.currency) }}
            </span>
          </div>
          <div class="mt-2 flex flex-col gap-1 text-[12.5px] text-muted-foreground">
            <span>{{ t('orderDetail.refundRecordTime') }}：{{ formatDate(record.created_at) }}</span>
            <span>{{ t('orderDetail.refundRecordReason') }}：{{ refundReasonText(record.remark) }}</span>
          </div>
        </div>
      </div>
    </section>

    <!-- ==================== 时间 ==================== -->
    <section v-if="showTimeCard" class="rounded-[var(--r-lg)] border border-border bg-card p-5">
      <h2 class="mb-4 text-[15px] font-semibold tracking-[-0.015em]">{{ t('orderDetail.timeTitle') }}</h2>
      <div class="flex flex-col gap-2 text-[13.5px]">
        <div v-if="order.created_at" class="flex justify-between gap-3">
          <span class="text-muted-foreground">{{ t('orderDetail.createdAtLabel') }}</span>
          <span>{{ formatDate(order.created_at) }}</span>
        </div>
        <div v-if="order.paid_at" class="flex justify-between gap-3">
          <span class="text-muted-foreground">{{ t('orderDetail.paidAtLabel') }}</span>
          <span>{{ formatDate(order.paid_at) }}</span>
        </div>
        <div v-if="order.expires_at" class="flex justify-between gap-3">
          <span class="text-muted-foreground">{{ t('orderDetail.expiresAtLabel') }}</span>
          <span>{{ formatDate(order.expires_at) }}</span>
        </div>
        <div v-if="order.canceled_at" class="flex justify-between gap-3">
          <span class="text-muted-foreground">{{ t('orderDetail.canceledAtLabel') }}</span>
          <span>{{ formatDate(order.canceled_at) }}</span>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, toRef } from 'vue'
import { useI18n } from 'vue-i18n'
import { Check, Copy } from 'lucide-vue-next'
import { copyText } from '../../../utils/clipboard'
import { useOrderDisplayHelpers } from '../../../composables/useOrderDisplayHelpers'

const props = withDefaults(
  defineProps<{ order: any; variant?: 'member' | 'guest'; fulfillmentDownloading?: boolean }>(),
  { variant: 'guest', fulfillmentDownloading: false },
)

defineEmits<{ download: [] }>()

const { t } = useI18n()

const {
  showTimeCard,
  showRefundRecordsCard,
  refundRecords,
  statusLabel,
  statusVariant,
  resolvedChildStatus,
  fulfillmentTypeLabelText,
  isFulfillmentTruncated,
  formatDate, getLocalizedText,
  orderItemSkuText,
  formatMoney, formatDiscountMoney, hasDiscountAmount, hasAmount,
  hasItemDiscount, formatItemDiscountTotal, formatItemPaidAmount,
  refundReasonText,
  manualSubmissionRows,
  fulfillmentDeliveryLines,
  instructionBlocks,
  fulfillmentCopied,
  handleCopyFulfillment,
} = useOrderDisplayHelpers(toRef(props, 'order'))

const orderItems = computed<any[]>(() => (Array.isArray(props.order?.items) ? props.order.items : []))

/** 子订单（拆单场景） */
const childOrders = computed<any[]>(() =>
  Array.isArray(props.order?.children) ? props.order.children : [],
)

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

/** 交付内容按行拆分（去重、去空行） */
const splitPayload = (payload?: string) => {
  if (typeof payload !== 'string' || !payload.trim()) return []
  return [...new Set(payload.split('\n').map((line) => line.trim()).filter(Boolean))]
}

/** 卡密逐行拆分：fulfillment.payload 是换行分隔的明文凭证 */
const secretLines = computed<string[]>(() => {
  const payload = props.order?.fulfillment?.payload
  if (typeof payload !== 'string' || !payload.trim()) return []
  return [...new Set(payload.split('\n').map((line) => line.trim()).filter(Boolean))]
})

/** 结构化的交付信息（delivery_data） */
const deliveryRows = computed(() =>
  fulfillmentDeliveryLines(props.order?.fulfillment).map((line) => {
    const idx = line.indexOf(': ')
    return idx > 0
      ? { key: line.slice(0, idx), value: line.slice(idx + 2) }
      : { key: '', value: line }
  }),
)

/** 使用说明（订单项的 instructions，仅已支付时有值） */
const instructions = computed(() => instructionBlocks(orderItems.value))

const manualRows = (item: any) => manualSubmissionRows(item?.manual_form_submission, item?.manual_form_schema_snapshot)

// 逐条复制的成功态
const copiedLine = ref('')
let copiedTimer: ReturnType<typeof setTimeout> | null = null

const copyLine = async (line: string) => {
  try {
    await copyText(line)
    copiedLine.value = line
    if (copiedTimer) clearTimeout(copiedTimer)
    copiedTimer = setTimeout(() => { copiedLine.value = '' }, 1500)
  } catch {
    /* 剪贴板不可用时静默忽略 */
  }
}
</script>

<style scoped>
.ember-prose :deep(img) {
  border-radius: var(--r);
  margin-block: 12px;
}
.ember-prose :deep(a) {
  color: var(--accent);
}
.ember-prose :deep(p) {
  margin-block: 6px;
}
.ember-prose :deep(ul),
.ember-prose :deep(ol) {
  margin-block: 6px;
  padding-left: 20px;
  list-style: revert;
}
</style>
