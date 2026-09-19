import { computed, ref, type Ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '../stores/app'
import { orderStatusVariant, orderStatusLabel } from '../utils/status'
import { fulfillmentStatusLabel, fulfillmentTypeLabel } from '../utils/fulfillment'
import { amountToCents, centsToAmount } from '../utils/money'
import { buildSkuDisplayTextFromSnapshot } from '../utils/sku'
import { getImageUrl } from '../utils/image'
import { copyText } from '../utils/clipboard'
import { sanitizeRichHtml } from '../utils/richContent'

interface ManualFormSnapshotField {
  key: string
  label?: Record<string, string> | string
}

/**
 * 订单详情展示辅助逻辑（OrderDetail / GuestOrderDetail 共用）。
 * 纯展示函数 + 复制成功态，按需被 useOrderDetail / useGuestOrderDetail 复用，避免重复造轮子。
 */
export function useOrderDisplayHelpers(order: Ref<any>) {
  const appStore = useAppStore()
  const { t } = useI18n()

  const fulfillmentCopied = ref(false)
  let fulfillmentCopiedTimer: ReturnType<typeof setTimeout> | null = null

  const showTimeCard = computed(() => {
    if (!order.value) return false
    return Boolean(order.value.paid_at || order.value.expires_at || order.value.canceled_at)
  })

  const showRefundRecordsCard = computed(() => {
    const status = String(order.value?.status || '').trim()
    return status === 'refunded' || status === 'partially_refunded'
  })

  const refundRecords = computed(() => {
    const records = order.value?.refund_records
    if (!Array.isArray(records)) return []
    return records
  })

  const isFulfillmentTruncated = (fulfillment: any) => {
    return fulfillment?.payload_line_count > 100
  }

  const statusLabel = (status: string) => orderStatusLabel(t, status)
  const statusVariant = (status: string) => orderStatusVariant(status)

  // vault 模板：把 BadgeTone 映射到 vault pill 类名（classic 不使用）
  const statusPillClass = (status?: string) => {
    const map: Record<string, string> = {
      success: 'pill-done',
      warning: 'pill-low',
      danger: 'pill-sale',
      info: 'pill-stock',
      accent: 'pill-sale',
      neutral: 'pill-out',
    }
    return map[orderStatusVariant(status)] || 'pill-out'
  }
  const fulfillmentTypeLabelText = (type: string) => fulfillmentTypeLabel(t, type, 'orderDetail')
  const fulfillmentStatusLabelText = (status: string) => fulfillmentStatusLabel(t, status, 'orderDetail')

  const resolvedChildStatus = (child: any) => {
    const status = String(child?.status || '').trim()
    const refundedCents = amountToCents(child?.refunded_amount)
    if (refundedCents !== null && refundedCents > 0) {
      const totalCents = amountToCents(child?.total_amount)
      if (totalCents !== null && totalCents > 0 && refundedCents >= totalCents) {
        return 'refunded'
      }
      return 'partially_refunded'
    }
    if (order.value?.status === 'refunded' && status !== 'refunded') {
      return 'refunded'
    }
    return status
  }

  const formatDate = (raw?: string) => {
    if (!raw) return ''
    const date = new Date(raw)
    if (Number.isNaN(date.getTime())) return raw
    return date.toLocaleString()
  }

  const getLocalizedText = (jsonData: any) => {
    if (!jsonData) return ''
    const locale = appStore.locale
    return jsonData[locale] || jsonData['zh-CN'] || jsonData['en-US'] || ''
  }


  const instructionBlocks = (items: any): Array<{ title: string; html: string }> => {
    if (!Array.isArray(items)) return []
    const seen = new Set<string>()
    const blocks: Array<{ title: string; html: string }> = []
    for (const item of items) {
      const html = String(getLocalizedText(item?.instructions) || '').trim()
      if (!html) continue
      if (seen.has(html)) continue
      seen.add(html)
      blocks.push({ title: getLocalizedText(item?.title), html: sanitizeRichHtml(html) })
    }
    return blocks
  }

  const refundReasonText = (remark?: string) => {
    const value = String(remark || '').trim()
    return value || t('orderDetail.refundRecordReasonEmpty')
  }

  const orderItemImage = (item: any) => {
    const snapshot = item?.sku_snapshot
    if (!snapshot || typeof snapshot !== 'object') return ''
    const rawImage = String(snapshot.image || '').trim()
    if (!rawImage) return ''
    return getImageUrl(rawImage)
  }

  const formatMoney = (amount?: string, currency?: string) => {
    if (amount === null || amount === undefined || amount === '') return '-'
    if (currency === null || currency === undefined || currency === '') {
      return String(amount)
    }
    return `${amount} ${currency}`
  }

  const formatDiscountMoney = (amount?: string, currency?: string) => {
    return hasDiscountAmount(amount) ? `-${formatMoney(amount, currency)}` : formatMoney(amount, currency)
  }

  const hasDiscountAmount = (amount?: string) => {
    if (amount === null || amount === undefined || amount === '') return false
    const valueCents = amountToCents(amount)
    return valueCents !== null && valueCents > 0
  }

  const positiveAmountCents = (amount?: string) => {
    const valueCents = amountToCents(amount)
    return valueCents !== null && valueCents > 0 ? valueCents : 0
  }

  const itemDiscountTotalCents = (item: any) => {
    return positiveAmountCents(item?.promotion_discount_amount)
      + positiveAmountCents(item?.wholesale_discount_amount)
      + positiveAmountCents(item?.member_discount_amount)
      + positiveAmountCents(item?.coupon_discount_amount)
  }

  const hasItemDiscount = (item: any) => itemDiscountTotalCents(item) > 0

  const formatItemDiscountTotal = (item: any, currency?: string) => {
    return formatMoney(centsToAmount(itemDiscountTotalCents(item)), currency)
  }

  const itemPaidAmountCents = (item: any) => {
    const totalCents = amountToCents(item?.total_price)
    const paidCents = (totalCents !== null ? totalCents : 0) - positiveAmountCents(item?.coupon_discount_amount)
    return Math.max(0, paidCents)
  }

  const formatItemPaidAmount = (item: any, currency?: string) => {
    return formatMoney(centsToAmount(itemPaidAmountCents(item)), currency)
  }

  const hasAmount = (amount?: string) => {
    if (amount === null || amount === undefined || amount === '') return false
    const valueCents = amountToCents(amount)
    return valueCents !== null && valueCents > 0
  }

  const formatManualValue = (value: unknown) => {
    if (Array.isArray(value)) {
      return value.map((item) => String(item)).join(', ')
    }
    if (value === null || value === undefined) {
      return '-'
    }
    if (typeof value === 'object') {
      try {
        return JSON.stringify(value)
      } catch {
        return String(value)
      }
    }
    return String(value)
  }

  const normalizeManualSnapshotFields = (schemaSnapshot: any): ManualFormSnapshotField[] => {
    if (!schemaSnapshot || typeof schemaSnapshot !== 'object') return []
    const rawFields = Array.isArray(schemaSnapshot.fields) ? schemaSnapshot.fields : []
    return rawFields
      .map((field: any) => {
        const key = String(field?.key || '').trim()
        if (!key) return null
        return {
          key,
          label: field?.label,
        } as ManualFormSnapshotField
      })
      .filter(Boolean) as ManualFormSnapshotField[]
  }

  const resolveManualFieldLabel = (field: ManualFormSnapshotField) => {
    if (typeof field.label === 'string' && field.label.trim()) return field.label.trim()
    if (field.label && typeof field.label === 'object') {
      const localized = getLocalizedText(field.label)
      if (localized) return localized
    }
    return field.key
  }

  const manualSubmissionRows = (submission: any, schemaSnapshot?: any) => {
    if (!submission || typeof submission !== 'object') return []
    const entries = Object.entries(submission).filter(([key]) => String(key).trim() !== '')
    if (entries.length === 0) return []

    const valueMap = new Map(entries.map(([key, value]) => [String(key), value] as const))
    const rows: Array<{ key: string; label: string; value: string }> = []

    normalizeManualSnapshotFields(schemaSnapshot).forEach((field) => {
      if (!valueMap.has(field.key)) return
      rows.push({
        key: field.key,
        label: resolveManualFieldLabel(field),
        value: formatManualValue(valueMap.get(field.key)),
      })
      valueMap.delete(field.key)
    })

    valueMap.forEach((value, key) => {
      rows.push({
        key,
        label: key,
        value: formatManualValue(value),
      })
    })

    return rows
  }

  const orderItemSkuText = (item: any) => {
    return buildSkuDisplayTextFromSnapshot(item?.sku_snapshot, {
      locale: appStore.locale,
      fallback: t('productDetail.skuFallback'),
    })
  }

  const fulfillmentDeliveryLines = (fulfillment: any) => {
    const deliveryData = fulfillment?.delivery_data || fulfillment?.logistics
    const lines: string[] = []
    if (deliveryData && typeof deliveryData === 'object') {
      const note = String(deliveryData.note || '').trim()
      if (note) {
        lines.push(note)
      }
      const entries = Array.isArray(deliveryData.entries) ? deliveryData.entries : []
      entries.forEach((entry: any) => {
        const key = String(entry?.key || '').trim()
        const value = String(entry?.value || '').trim()
        if (!key && !value) {
          return
        }
        if (!key) {
          lines.push(value)
        } else if (!value) {
          lines.push(key)
        } else {
          lines.push(`${key}: ${value}`)
        }
      })
    }
    return lines
  }

  const handleCopyFulfillment = async (fulfillment: any) => {
    const lines = fulfillmentDeliveryLines(fulfillment)
    const text = lines.length > 0 ? lines.join('\n') : (fulfillment?.payload || '')
    if (!text) return
    try {
      await copyText(text)
      fulfillmentCopied.value = true
      if (fulfillmentCopiedTimer) clearTimeout(fulfillmentCopiedTimer)
      fulfillmentCopiedTimer = setTimeout(() => { fulfillmentCopied.value = false }, 1500)
    } catch {}
  }

  return {
    // 卡片显隐
    showTimeCard,
    showRefundRecordsCard,
    refundRecords,
    // 状态/标签
    statusLabel,
    statusVariant,
    statusPillClass,
    fulfillmentTypeLabelText,
    fulfillmentStatusLabelText,
    resolvedChildStatus,
    isFulfillmentTruncated,
    // 文本/格式化
    formatDate,
    getLocalizedText,
    refundReasonText,
    orderItemImage,
    orderItemSkuText,
    formatMoney,
    formatDiscountMoney,
    hasDiscountAmount,
    hasAmount,
    hasItemDiscount,
    formatItemDiscountTotal,
    formatItemPaidAmount,
    manualSubmissionRows,
    fulfillmentDeliveryLines,
    instructionBlocks,
    // 发货复制态
    fulfillmentCopied,
    handleCopyFulfillment,
  }
}
