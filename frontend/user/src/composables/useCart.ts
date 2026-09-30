import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useCartStore, type CartItem } from '../stores/cart'
import { useAppStore } from '../stores/app'
import { amountToCents, centsToAmount, parseInteger } from '../utils/money'
import { buildSkuDisplayText, normalizeSkuId } from '../utils/sku'
import {
  refreshCartStockSnapshots,
  cartItemPurchaseLimit as itemPurchaseLimit,
  cartItemPurchaseMin as itemPurchaseMin,
} from '../utils/cartStock'
import { getImageUrl } from '../utils/image'
import { useLocalized } from './useProduct'
import { toast } from './useToast'

/**
 * 购物车页面共享逻辑（classic + vault 双模板共用）。
 * 完整保留原 views/Cart.vue 的行为，仅抽离为 composable。
 */
export function useCart() {
  const cartStore = useCartStore()
  const appStore = useAppStore()
  const { t } = useI18n()

  const { getLocalizedText, siteCurrency, formatPrice } = useLocalized()
  const totalCurrency = computed(() => siteCurrency.value || 'CNY')

  const cartItems = computed(() => cartStore.items)
  const totalItems = computed(() => cartStore.totalItems)
  const quantityWarnings = ref<Record<string, string>>({})

  const totalAmount = computed(() => {
    const totalCents = cartItems.value.reduce((sum, item) => {
      const amountCents = amountToCents(item.priceAmount)
      const qty = parseInteger(item.quantity)
      if (amountCents === null || qty === null) return sum
      return sum + amountCents * qty
    }, 0)
    return centsToAmount(totalCents)
  })

  const removeWithUndo = (item: CartItem) => {
    const removedItem = { ...item }
    cartStore.removeItem(item.productId, item.skuId)
    toast.info(t('cart.removed'), {
      duration: 5000,
      action: {
        label: t('cart.undo'),
        onClick: () => {
          cartStore.addItem(removedItem, removedItem.quantity)
        },
      },
    })
  }

  const itemSubtotal = (item: CartItem) => {
    const amountCents = amountToCents(item.priceAmount)
    const qty = parseInteger(item.quantity)
    if (amountCents === null || qty === null) {
      return formatPrice('-', totalCurrency.value)
    }
    return formatPrice(centsToAmount(amountCents * qty), totalCurrency.value)
  }

  const updateQty = (item: CartItem, qty: number) => {
    const key = cartItemKey(item)
    quantityWarnings.value[key] = ''
    const max = itemMaxQuantity(item)
    const available = itemAvailableStock(item)
    const purchaseLimit = itemPurchaseLimit(item)
    const purchaseMin = itemPurchaseMin(item)
    if (qty < purchaseMin) {
      if (purchaseMin > 1) {
        quantityWarnings.value[key] = t('cart.minPurchaseNotMet', { count: purchaseMin })
      }
      return
    }
    if (qty > max) {
      if (max <= 0) {
        quantityWarnings.value[key] = t('cart.stockOut')
      } else if (purchaseLimit !== null && max === purchaseLimit && (available === null || purchaseLimit < available)) {
        quantityWarnings.value[key] = t('cart.maxPurchaseExceeded', { count: purchaseLimit })
      } else {
        quantityWarnings.value[key] = t('cart.stockExceeded', { count: max })
      }
      return
    }
    cartStore.updateQuantity(item.productId, qty, item.skuId)
  }

  const handleQtyChange = (item: CartItem, event: Event) => {
    const target = event.target as HTMLInputElement
    const value = parseInt(target.value, 10)
    const purchaseMin = itemPurchaseMin(item)
    if (!Number.isFinite(value) || value < purchaseMin) {
      target.value = String(item.quantity)
      return
    }
    updateQty(item, value)
    target.value = String(item.quantity)
  }

  const cartItemKey = (item: CartItem) => `${item.productId}:${normalizeSkuId(item.skuId)}`

  const itemSkuDisplay = (item: CartItem) => buildSkuDisplayText({
    skuCode: item.skuCode,
    specValues: item.skuSpecValues,
    fallback: t('productDetail.skuFallback'),
    locale: appStore.locale,
  })

  const cartItemImage = (item: CartItem) => {
    const rawImage = String(item.image || '').trim()
    if (!rawImage) return ''
    return getImageUrl(rawImage)
  }

  const normalizeStockNumber = (value: unknown) => {
    const numberValue = Number(value)
    if (!Number.isFinite(numberValue)) return 0
    return Math.max(Math.floor(numberValue), 0)
  }

  const normalizeManualStockTotal = (value: unknown) => {
    const numberValue = Number(value)
    if (!Number.isFinite(numberValue)) return 0
    const integerValue = Math.floor(numberValue)
    if (integerValue === -1) return -1
    return Math.max(integerValue, 0)
  }

  const hasItemStockSnapshot = (item: CartItem) => Boolean(String(item.skuStockSnapshotAt || '').trim())

  const shouldEnforceItemStock = (item: CartItem) => {
    if (item.skuStockQuantityHidden === true) return false
    if (item.fulfillmentType === 'auto') return true
    if (item.fulfillmentType === 'upstream') return true
    if (item.fulfillmentType !== 'manual') return false
    if (!hasItemStockSnapshot(item)) return false
    const total = normalizeManualStockTotal(item.skuManualStockTotal)
    if (total === -1) return false
    if (item.skuStockEnforced === true) return true
    if (item.skuStockEnforced === false) return false
    return true
  }

  const itemAvailableStock = (item: CartItem) => {
    if (item.skuStockQuantityHidden === true) {
      return item.skuStockStatus === 'out_of_stock' ? 0 : null
    }
    if (!shouldEnforceItemStock(item)) return null
    if (item.fulfillmentType === 'upstream') {
      const upstreamStock = Number(item.skuUpstreamStock ?? 0)
      if (upstreamStock === -1) return null
      return Math.max(upstreamStock, 0)
    }
    if (item.fulfillmentType === 'auto') {
      if (Number(item.skuAutoStockAvailable) === -1) return null
      return normalizeStockNumber(item.skuAutoStockAvailable)
    }
    const total = normalizeManualStockTotal(item.skuManualStockTotal)
    if (total === -1) return null
    return total
  }

  const itemMaxQuantity = (item: CartItem) => {
    const available = itemAvailableStock(item)
    const purchaseLimit = itemPurchaseLimit(item)
    if (available === null && purchaseLimit === null) return Number.MAX_SAFE_INTEGER
    if (available === null) return purchaseLimit || 0
    if (purchaseLimit === null) return Math.max(available, 0)
    return Math.max(Math.min(available, purchaseLimit), 0)
  }

  const itemStockHint = (item: CartItem) => {
    const available = itemAvailableStock(item)
    const purchaseLimit = itemPurchaseLimit(item)
    const maxQuantity = itemMaxQuantity(item)
    if (available === null) return ''
    if (available <= 0) return t('cart.stockOut')
    if (purchaseLimit !== null && maxQuantity === purchaseLimit && purchaseLimit < available) {
      return t('cart.maxPurchaseExceeded', { count: purchaseLimit })
    }
    return t('cart.stockRemaining', { count: available })
  }

  const quantityWarning = (item: CartItem) => quantityWarnings.value[cartItemKey(item)] || ''

  onMounted(() => {
    void refreshCartStockSnapshots(cartStore)
  })

  return {
    // helpers / formatters
    getLocalizedText,
    formatPrice,
    totalCurrency,
    // state
    cartItems,
    totalItems,
    totalAmount,
    // item display
    cartItemKey,
    cartItemImage,
    itemSkuDisplay,
    itemSubtotal,
    itemStockHint,
    quantityWarning,
    // purchase limits
    itemPurchaseMin,
    itemMaxQuantity,
    // actions
    removeWithUndo,
    updateQty,
    handleQtyChange,
  }
}
