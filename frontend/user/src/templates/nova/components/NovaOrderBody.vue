<template>
  <div class="flex flex-col gap-5">
    <!-- ==================== 卡密 / 交付内容 ==================== -->
    <section v-if="secretLines.length" class="nova-card p-6">
      <div class="mb-5 flex flex-wrap items-center justify-between gap-3">
        <h2 class="nova-eyebrow">{{ t('orderDetail.fulfillmentTitle') }}</h2>
        <button
          type="button"
          class="rounded-full border border-border px-3.5 py-1.5 text-[12.5px] text-[color:var(--ink-3)] transition-colors duration-200 hover:border-[color:var(--accent)] hover:text-[color:var(--accent-strong)]"
          @click="handleCopyFulfillment(order.fulfillment)"
        >{{ fulfillmentCopied ? t('orderDetail.fulfillmentCopied') : t('orderDetail.fulfillmentCopy') }}</button>
      </div>

      <!-- 交付内容超过 100 条时只给预览，引导下载完整内容 -->
      <template v-if="isFulfillmentTruncated(order.fulfillment)">
        <div class="text-[12.5px] text-[color:var(--ink-3)]">
          {{ t('orderDetail.fulfillmentTotalLines', { count: order.fulfillment.payload_line_count }) }}
        </div>
        <div class="mt-3 rounded-[var(--r-sm)] bg-[color:var(--warning-soft)] px-3.5 py-2.5 text-[12.5px] leading-[1.65] text-[color:var(--warning-strong)]">
          {{ t('orderDetail.fulfillmentTruncatedHint') }}
        </div>
        <pre class="mt-3 max-h-[280px] overflow-y-auto whitespace-pre-wrap break-all rounded-[var(--r-sm)] border border-[color:var(--border)] bg-[color:var(--surface-2)] p-3.5 font-mono text-[12.5px] text-[color:var(--ink-2)]">{{ order.fulfillment.payload }}</pre>
      </template>

      <div v-else class="flex flex-col">
        <div
          v-for="(line, idx) in secretLines"
          :key="idx"
          class="grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-3 py-2.5"
          :class="idx > 0 ? 'border-t border-[color:var(--border-soft)]' : ''"
        >
          <span class="text-[11.5px] tabular-nums text-[color:var(--ink-4)]">{{ String(idx + 1).padStart(2, '0') }}</span>
          <span class="break-all font-mono text-[13px] text-foreground">{{ line }}</span>
          <button
            type="button"
            class="grid h-7 w-7 place-items-center rounded-[var(--r-xs)] text-[color:var(--ink-3)] transition-colors duration-200 hover:bg-[color:var(--surface-2)] hover:text-foreground"
            :aria-label="t('common.copy')"
            @click="copyLine(line)"
          >
            <Check v-if="copiedLine === line" class="h-3.5 w-3.5 text-[color:var(--success)]" />
            <Copy v-else class="h-3.5 w-3.5" />
          </button>
        </div>
      </div>

      <div v-if="deliveryRows.length" class="mt-4 flex flex-col gap-1.5 rounded-[var(--r-sm)] bg-[color:var(--surface-2)] p-4">
        <div v-for="(row, idx) in deliveryRows" :key="idx" class="flex flex-wrap justify-between gap-3 text-[13px]">
          <span class="text-[color:var(--ink-3)]">{{ row.key }}</span>
          <span class="break-all text-right font-medium tabular-nums">{{ row.value }}</span>
        </div>
      </div>

      <button
        v-if="variant === 'member'"
        type="button"
        class="mt-5 rounded-full border border-border px-4 py-2 text-[13px] text-[color:var(--ink-2)] transition-colors duration-200 hover:border-[color:var(--accent)] hover:text-[color:var(--accent-strong)] disabled:opacity-50"
        :disabled="fulfillmentDownloading"
        @click="emit('download', order.order_no)"
      >{{ t('orderDetail.fulfillmentDownload') }}</button>
    </section>

    <!-- ==================== 使用说明 ==================== -->
    <section v-if="instructions.length" class="nova-card p-6">
      <h2 class="nova-eyebrow mb-5">{{ t('orderDetail.instructionsTitle') }}</h2>
      <div class="flex flex-col gap-4">
        <div v-for="(block, idx) in instructions" :key="idx">
          <div v-if="block.title" class="mb-1.5 text-[13px] font-medium text-[color:var(--ink-2)]">{{ block.title }}</div>
          <div class="nova-prose text-[14px] leading-[1.72]" v-html="block.html"></div>
        </div>
      </div>
    </section>

    <!-- ==================== 商品明细（下单流程列表展示的核心） ==================== -->
    <section class="nova-card p-6">
      <div class="mb-2 flex flex-wrap items-baseline justify-between gap-3">
        <h2 class="nova-eyebrow">{{ t('nova.order.eyebrow') }}</h2>
        <span class="text-[12.5px] tabular-nums text-[color:var(--ink-3)]">
          {{ t('nova.order.itemsCount', { count: orderItems.length }) }}
        </span>
      </div>

      <!-- 行与行之间用 1px 暖灰分隔线，而不是逐行描边卡片 -->
      <div class="flex flex-col">
        <NovaOrderItem
          v-for="(item, idx) in orderItems"
          :key="idx"
          :item="item"
          :index="idx"
          :order="order"
          :currency="order.currency"
          :class="idx > 0 ? 'border-t border-[color:var(--border)]' : ''"
        />
      </div>
    </section>

    <!-- ==================== 金额 ==================== -->
    <section class="nova-card p-6">
      <h2 class="nova-eyebrow mb-5">{{ t('orderDetail.amountTitle') }}</h2>
      <div class="flex flex-col gap-3">
        <div class="flex items-center justify-between text-[13.5px]">
          <span class="text-[color:var(--ink-3)]">{{ t('orderDetail.amountOriginal') }}</span>
          <span class="tabular-nums">{{ formatMoney(order.original_amount, order.currency) }}</span>
        </div>
        <div v-if="hasDiscountAmount(order.discount_amount)" class="flex items-center justify-between text-[13.5px]">
          <span class="text-[color:var(--ink-3)]">{{ t('orderDetail.amountDiscount') }}</span>
          <span class="tabular-nums text-[color:var(--accent-strong)]">{{ formatDiscountMoney(order.discount_amount, order.currency) }}</span>
        </div>
        <div v-if="hasDiscountAmount(order.promotion_discount_amount)" class="flex items-center justify-between text-[13.5px]">
          <span class="text-[color:var(--ink-3)]">{{ t('orderDetail.promotionDiscountLabel') }}</span>
          <span class="tabular-nums text-[color:var(--accent-strong)]">{{ formatDiscountMoney(order.promotion_discount_amount, order.currency) }}</span>
        </div>
        <div v-if="hasDiscountAmount(order.member_discount_amount)" class="flex items-center justify-between text-[13.5px]">
          <span class="text-[color:var(--ink-3)]">{{ t('orderDetail.memberDiscountLabel') }}</span>
          <span class="tabular-nums text-[color:var(--success-strong)]">{{ formatDiscountMoney(order.member_discount_amount, order.currency) }}</span>
        </div>
        <div v-if="hasDiscountAmount(order.wholesale_discount_amount)" class="flex items-center justify-between text-[13.5px]">
          <span class="text-[color:var(--ink-3)]">{{ t('orderDetail.amountWholesaleDiscount') }}</span>
          <span class="tabular-nums text-[color:var(--success-strong)]">{{ formatDiscountMoney(order.wholesale_discount_amount, order.currency) }}</span>
        </div>
        <div v-if="hasAmount(order.wallet_paid_amount)" class="flex items-center justify-between text-[13.5px]">
          <span class="text-[color:var(--ink-3)]">{{ t('orderDetail.amountWalletPaid') }}</span>
          <span class="tabular-nums">{{ formatMoney(order.wallet_paid_amount, order.currency) }}</span>
        </div>
        <div v-if="hasAmount(order.online_paid_amount)" class="flex items-center justify-between text-[13.5px]">
          <span class="text-[color:var(--ink-3)]">{{ t('orderDetail.amountOnlinePaid') }}</span>
          <span class="tabular-nums">{{ formatMoney(order.online_paid_amount, order.currency) }}</span>
        </div>
        <div class="mt-2 flex items-baseline justify-between border-t border-border pt-4">
          <span class="text-[13.5px] text-[color:var(--ink-3)]">{{ t('orderDetail.amountTotal') }}</span>
          <span class="text-[24px] font-semibold tabular-nums tracking-[-0.03em] text-foreground">{{ formatMoney(order.total_amount, order.currency) }}</span>
        </div>
      </div>
    </section>

    <!-- ==================== 子订单（拆单） ==================== -->
    <section v-if="childOrders.length" class="nova-card p-6">
      <h2 class="nova-eyebrow mb-5">{{ t('orderDetail.childOrdersTitle') }}</h2>
      <div class="flex flex-col">
        <div
          v-for="(child, idx) in childOrders"
          :key="child.order_no"
          class="py-4"
          :class="idx > 0 ? 'border-t border-[color:var(--border)]' : ''"
        >
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div class="flex flex-wrap items-center gap-2.5">
              <span class="nova-pill" :class="pillClass(resolvedChildStatus(child))">
                {{ statusLabel(resolvedChildStatus(child)) }}
              </span>
              <span class="font-mono text-[12px] text-[color:var(--ink-3)]">{{ child.order_no }}</span>
            </div>
            <span class="text-[14px] font-semibold tabular-nums">
              {{ formatMoney(child.total_amount, child.currency || order.currency) }}
            </span>
          </div>

          <!-- 子订单商品 -->
          <div v-if="Array.isArray(child.items) && child.items.length" class="mt-3 flex flex-col gap-1.5">
            <div
              v-for="(item, i) in child.items"
              :key="i"
              class="flex flex-wrap justify-between gap-3 text-[12.5px] text-[color:var(--ink-3)]"
            >
              <span>{{ getLocalizedText(item.title) }} × {{ item.quantity }}</span>
              <span class="tabular-nums">{{ formatMoney(item.total_price, child.currency || order.currency) }}</span>
            </div>
          </div>

          <!-- 子订单交付内容 -->
          <div v-if="child.fulfillment?.payload" class="mt-3">
            <div class="nova-eyebrow mb-2">{{ t('orderDetail.childFulfillmentTitle') }}</div>
            <div class="flex flex-col gap-1.5">
              <div
                v-for="(line, i) in splitPayload(child.fulfillment.payload)"
                :key="i"
                class="rounded-[var(--r-xs)] border border-[color:var(--border)] bg-[color:var(--surface-2)] px-3 py-2 font-mono text-[12.5px] break-all"
              >{{ line }}</div>
            </div>
          </div>
          <p v-else class="mt-3 text-[12.5px] text-[color:var(--ink-3)]">{{ t('orderDetail.childFulfillmentEmpty') }}</p>
        </div>
      </div>
    </section>

    <!-- ==================== 退款记录 ==================== -->
    <section v-if="showRefundRecordsCard && refundRecords.length" class="nova-card p-6">
      <h2 class="nova-eyebrow mb-5">{{ t('orderDetail.refundRecordsTitle') }}</h2>
      <div class="flex flex-col">
        <div
          v-for="(record, idx) in refundRecords"
          :key="idx"
          class="py-4"
          :class="idx > 0 ? 'border-t border-[color:var(--border)]' : ''"
        >
          <div class="flex flex-wrap items-center justify-between gap-3">
            <span class="text-[13.5px] font-medium">{{ record.type || t('orderDetail.refundRecordsTitle') }}</span>
            <span class="text-[14px] font-semibold tabular-nums text-[color:var(--accent-strong)]">
              {{ formatMoney(record.amount, record.currency || order.currency) }}
            </span>
          </div>
          <div class="mt-2 flex flex-col gap-1 text-[12.5px] text-[color:var(--ink-3)]">
            <span>{{ t('orderDetail.refundRecordTime') }}：{{ formatDate(record.created_at) }}</span>
            <span>{{ t('orderDetail.refundRecordReason') }}：{{ refundReasonText(record.remark) }}</span>
          </div>
        </div>
      </div>
    </section>

    <!-- ==================== 时间 ==================== -->
    <section v-if="showTimeCard" class="nova-card p-6">
      <h2 class="nova-eyebrow mb-5">{{ t('orderDetail.timeTitle') }}</h2>
      <div class="flex flex-col gap-2.5 text-[13.5px]">
        <div v-if="order.created_at" class="flex justify-between gap-3">
          <span class="text-[color:var(--ink-3)]">{{ t('orderDetail.createdAtLabel') }}</span>
          <span class="tabular-nums">{{ formatDate(order.created_at) }}</span>
        </div>
        <div v-if="order.paid_at" class="flex justify-between gap-3">
          <span class="text-[color:var(--ink-3)]">{{ t('orderDetail.paidAtLabel') }}</span>
          <span class="tabular-nums">{{ formatDate(order.paid_at) }}</span>
        </div>
        <div v-if="order.expires_at" class="flex justify-between gap-3">
          <span class="text-[color:var(--ink-3)]">{{ t('orderDetail.expiresAtLabel') }}</span>
          <span class="tabular-nums">{{ formatDate(order.expires_at) }}</span>
        </div>
        <div v-if="order.canceled_at" class="flex justify-between gap-3">
          <span class="text-[color:var(--ink-3)]">{{ t('orderDetail.canceledAtLabel') }}</span>
          <span class="tabular-nums">{{ formatDate(order.canceled_at) }}</span>
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
import NovaOrderItem from './NovaOrderItem.vue'

const props = withDefaults(
  defineProps<{ order: any; variant?: 'member' | 'guest'; fulfillmentDownloading?: boolean }>(),
  { variant: 'guest', fulfillmentDownloading: false },
)

const emit = defineEmits<{ download: [orderNo: string] }>()

const { t } = useI18n()

const {
  showTimeCard,
  showRefundRecordsCard,
  refundRecords,
  statusLabel,
  statusVariant,
  resolvedChildStatus,
  isFulfillmentTruncated,
  formatDate, getLocalizedText,
  formatMoney, formatDiscountMoney, hasDiscountAmount, hasAmount,
  refundReasonText,
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
    success: 'nova-pill--success',
    warning: 'nova-pill--warning',
    danger: 'nova-pill--danger',
    info: 'nova-pill--accent',
    accent: 'nova-pill--accent',
    neutral: '',
  }
  return map[statusVariant(status ?? '')] || ''
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
