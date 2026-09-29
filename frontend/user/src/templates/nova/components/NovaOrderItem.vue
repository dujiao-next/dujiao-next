<template>
  <!--
    订单商品行 —— 下单流程列表展示的核心单元
    设计意图：参考站点的列表是「序号 / 主信息 / 右对齐数值」三段式，靠 1px 暖灰
    分隔线而不是卡片边框来分组。金额一律 tabular-nums 右对齐，保证多行数字对得齐。
    数据与计算全部来自 useOrderDisplayHelpers，本组件只负责呈现。
  -->
  <div class="grid grid-cols-[auto_minmax(0,1fr)_auto] items-start gap-4 py-4">
    <!-- 封面（有 SKU 快照图时用图，否则退回序号） -->
    <RouterLink
      v-if="thumb"
      :to="item.slug ? `/products/${item.slug}` : ''"
      class="mt-0.5 grid h-12 w-12 flex-none place-items-center overflow-hidden rounded-[10px] bg-[color:var(--surface-2)]"
    >
      <img :src="thumb" :alt="getLocalizedText(item.title)" loading="lazy" class="h-full w-full object-cover" />
    </RouterLink>
    <span
      v-else
      class="mt-0.5 grid h-7 w-7 flex-none place-items-center rounded-[9px] bg-[color:var(--surface-2)] text-[11.5px] font-medium tabular-nums text-[color:var(--ink-3)]"
    >{{ index + 1 }}</span>

    <!-- 主体 -->
    <div class="min-w-0">
      <div class="flex flex-wrap items-center gap-2">
        <span class="min-w-0 break-words text-[14.5px] font-medium leading-snug tracking-[-0.012em] text-foreground">
          {{ getLocalizedText(item.title) }}
        </span>
        <span v-if="soldOutHint" class="nova-pill nova-pill--danger">{{ soldOutHint }}</span>
      </div>

      <!-- 单价 × 数量：下单清单里最需要一眼看清的一行 -->
      <div class="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-[12.5px] text-[color:var(--ink-3)]">
        <span class="tabular-nums">
          {{ t('nova.order.unitPriceTimes', { price: unitPriceText, qty: item.quantity }) }}
        </span>
        <template v-if="fulfillmentLabel">
          <span class="text-[color:var(--ink-4)]">·</span>
          <span>{{ t('orderDetail.itemFulfillmentLabel') }}：{{ fulfillmentLabel }}</span>
        </template>
      </div>

      <div v-if="skuText" class="mt-1 text-[12px] text-[color:var(--ink-3)]">
        {{ t('orderDetail.itemSkuLabel') }}：{{ skuText }}
      </div>

      <div v-if="hasItemDiscount(item)" class="mt-1 text-[12px] text-[color:var(--accent-strong)]">
        {{ t('orderDetail.itemDiscountTotalLabel') }}：-{{ formatItemDiscountTotal(item, currency) }}
      </div>

      <!-- 手动发货表单快照 -->
      <dl v-if="manualRows.length" class="mt-3 flex flex-col gap-1.5 rounded-[var(--r-sm)] bg-[color:var(--surface-2)] p-3">
        <div v-for="row in manualRows" :key="row.key" class="flex flex-wrap gap-2 text-[12px]">
          <dt class="text-[color:var(--ink-3)]">{{ row.label }}</dt>
          <dd class="text-foreground">{{ row.value }}</dd>
        </div>
      </dl>
    </div>

    <!-- 金额 -->
    <div class="flex flex-col items-end gap-0.5 text-right">
      <span class="text-[15px] font-semibold tabular-nums tracking-[-0.02em] text-foreground">
        {{ formatItemPaidAmount(item, currency) }}
      </span>
      <span v-if="hasItemDiscount(item)" class="text-[11.5px] tabular-nums text-[color:var(--ink-4)] line-through">
        {{ formatMoney(item.total_price, currency) }}
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useOrderDisplayHelpers } from '../../../composables/useOrderDisplayHelpers'
import { useLocalized } from '../../../composables/useProduct'
import { toRef } from 'vue'

const props = withDefaults(defineProps<{
  item: any
  index?: number
  currency?: string
  order?: any
}>(), { index: 0 })

const { t } = useI18n()

// 注意：useOrderDisplayHelpers 里的 formatMoney 只是把「后端已格式化好的金额字符串」
// 拼上币种（`${amount} ${currency}`），不能用于计算派生金额。
// 单价是算出来的，必须走 useLocalized().formatPrice（接受数值）。
const { formatPrice } = useLocalized()

// useOrderDisplayHelpers 需要一个 order ref；本组件只用到其中与单行商品相关的几个，
// 传一个最小可用的对象即可（这些 helper 只读 item 字段）。
const orderRef = toRef(props, 'order')
const {
  getLocalizedText,
  orderItemImage,
  fulfillmentTypeLabelText,
  orderItemSkuText,
  formatMoney, formatItemDiscountTotal, formatItemPaidAmount,
  hasItemDiscount,
  manualSubmissionRows,
} = useOrderDisplayHelpers(orderRef as any)

const thumb = computed(() => {
  try {
    return orderItemImage(props.item) || ''
  } catch {
    return ''
  }
})

const currency = computed(() => props.currency || props.order?.currency || 'CNY')

const fulfillmentLabel = computed(() => {
  const type = props.item?.fulfillment_type
  return type ? fulfillmentTypeLabelText(type) : ''
})

const skuText = computed(() => orderItemSkuText(props.item))

/** 单价：优先用实付金额，回退到 total_price，再按数量均摊 */
const unitPriceText = computed(() => {
  const item = props.item || {}
  const qty = Number(item.quantity) || 1
  const paid = Number(item.paid_amount ?? item.total_price ?? 0)
  const unit = qty > 0 ? paid / qty : paid
  return formatPrice(unit, currency.value)
})

const manualRows = computed(() => {
  try {
    return manualSubmissionRows(props.item?.manual_form_submission, props.item?.manual_form_schema_snapshot) || []
  } catch {
    return []
  }
})

/** 缺货提示（订单项上的库存字段，缺失时不显示） */
const soldOutHint = computed(() => {
  const item = props.item || {}
  return item.stock_status === 'out_of_stock' ? t('products.stockStatus.outOfStock') : ''
})
</script>
