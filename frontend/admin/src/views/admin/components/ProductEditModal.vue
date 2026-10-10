<script setup lang="ts">
import { computed, ref, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AdminProduct, AdminCategory, AdminProductSKU, AdminPaymentChannel, LocalizedText } from '@/api/types'
import MediaPicker from '@/components/admin/MediaPicker.vue'
import RichEditor from '@/components/RichEditor.vue'
// image utils removed - MediaPicker handles image display
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { Dialog, DialogScrollContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { notifyError } from '@/utils/notify'
import { getLocalizedText } from '@/utils/format'
import { buildAdminCategoryPath, createAdminCategoryChildCountMap, createAdminCategoryMap, flattenAdminCategories, isAdminProductCategorySelectable } from '@/utils/category'

const props = defineProps<{
  modelValue: boolean
  productId: number | null
  categories: AdminCategory[]
  siteCurrency: string
  supportedLocales: string[]
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  success: []
}>()

const { t, locale } = useI18n()
const submitting = ref(false)
const isEditing = ref(false)
const paymentChannels = ref<AdminPaymentChannel[]>([])
const editingIsMapped = ref(false)
const initialCategoryID = ref<number | null>(null)
const newTag = ref('')
const currentLang = ref('zh-CN')

const languages = computed(() => [
  { code: 'zh-CN', name: t('admin.common.lang.zhCN') },
  { code: 'zh-TW', name: t('admin.common.lang.zhTW') },
  { code: 'en-US', name: t('admin.common.lang.enUS') },
])

const categoryMap = computed(() => createAdminCategoryMap(props.categories))
const categoryChildCountMap = computed(() => createAdminCategoryChildCountMap(props.categories))
const categoryOptions = computed(() => flattenAdminCategories(props.categories).map((item) => ({
  ...item,
  selectable: isProductCategorySelectable(item.category.id),
})))

const getCategoryOptionLabel = (category: AdminCategory) => {
  return buildAdminCategoryPath(category, categoryMap.value, (item) => getLocalizedText(item.name))
}

const getCategoryOptionName = (category: AdminCategory) => {
  return getLocalizedText(category.name)
}

const getCategoryLeafTip = () => {
  if (locale.value === 'en-US') {
    return 'Root categories with child categories cannot receive products directly. Choose a leaf category.'
  }
  if (locale.value === 'zh-TW') {
    return '已有二級分類的一級分類不能直接掛商品，請選擇末級分類'
  }
  return '已有二级分类的一级分类不能直接挂商品，请选择末级分类'
}

const getCategoryRequiredError = () => {
  if (locale.value === 'en-US') return 'Please select a category'
  if (locale.value === 'zh-TW') return '請選擇商品分類'
  return '请选择商品分类'
}

const isProductCategorySelectable = (categoryID: number | null | undefined) => {
  const normalizedCategoryID = Number(categoryID)
  if (!Number.isFinite(normalizedCategoryID) || normalizedCategoryID <= 0) return false

  const exactCategoryID = Math.floor(normalizedCategoryID)
  const category = categoryMap.value.get(exactCategoryID)
  if (!category) return false

  return isAdminProductCategorySelectable(category, categoryChildCountMap.value) || initialCategoryID.value === exactCategoryID
}

type SKUFormItem = {
  id: number
  sku_code: string
  spec_values: Record<string, string>
  price_amount: number
  agency_price_amount: number
  cost_price_amount: number
  manual_stock_total: number
  is_active: boolean
  sort_order: number
}

type ManualFormField = {
  key: string
  type: string
  required: boolean
  label: LocalizedText
  placeholder: LocalizedText
  regex: string
  min: string
  max: string
  max_len: string
  options_text: string
}

const createEmptyLocaleText = () => ({
  'zh-CN': '',
  'zh-TW': '',
  'en-US': '',
})

// resolveEditableSKURows 把接口返回的 SKU 列表转换为表单可编辑的规格行。
//
// 单规格商品在后端固定表示为「恰好一行 DEFAULT」，它承载价格与库存，并不是一条用户可见的
// 规格。若把它填进规格列表，用户删掉它再保存时会被后端重新生成，表现为「关不掉规格配置」，
// 所以这里只隐藏「有且仅有一行且编码为 DEFAULT」的情况；多规格中恰好叫 DEFAULT 的自定义
// 编码属于真实规格，照常展示。
const resolveEditableSKURows = (product: AdminProduct): SKUFormItem[] => {
  const rows = Array.isArray(product.skus) ? product.skus : []
  const isSingleSpecMode = rows.length === 1 && String(rows[0]?.sku_code || '').trim().toUpperCase() === 'DEFAULT'
  if (isSingleSpecMode) return []
  return rows.map((item: AdminProductSKU) => createSKUFormItem(item))
}

const createSKUFormItem = (raw?: Partial<AdminProductSKU>): SKUFormItem => ({
  id: Number(raw?.id || 0),
  sku_code: String(raw?.sku_code || '').trim(),
  spec_values: {
    ...createEmptyLocaleText(),
    ...Object.keys(raw?.spec_values || {}).reduce((result: Record<string, string>, key) => {
      const text = String(raw?.spec_values?.[key] ?? '').trim()
      if (text) {
        result[key] = text
      }
      return result
    }, {}),
  },
  price_amount: Number(raw?.price_amount || 0),
  agency_price_amount: Number(raw?.agency_price_amount || 0),
  cost_price_amount: Number(raw?.cost_price_amount || 0),
  manual_stock_total: toSafeStockTotal(raw?.manual_stock_total),
  is_active: raw?.is_active ?? true,
  sort_order: Number(raw?.sort_order || 0),
})

const form = reactive({
  id: 0,
  title: { 'zh-CN': '', 'zh-TW': '', 'en-US': '' } as LocalizedText,
  slug: '',
  seo_meta: { keywords: { 'zh-CN': '', 'zh-TW': '', 'en-US': '' }, description: { 'zh-CN': '', 'zh-TW': '', 'en-US': '' } } as { keywords: LocalizedText; description: LocalizedText; [key: string]: LocalizedText },
  description: { 'zh-CN': '', 'zh-TW': '', 'en-US': '' } as LocalizedText,
  content: { 'zh-CN': '', 'zh-TW': '', 'en-US': '' } as LocalizedText,
  instructions: { 'zh-CN': '', 'zh-TW': '', 'en-US': '' } as LocalizedText,
  price_amount: 0,
  agency_price_amount: 0,
  cost_price_amount: 0,
  images: [] as string[],
  tags: [] as string[],
  purchase_type: 'member',
  min_purchase_quantity: '' as number | '',
  max_purchase_quantity: '' as number | '',
  stock_display_mode: 'exact',
  fulfillment_type: 'manual',
  manual_stock_total: 0,
  skus: [] as SKUFormItem[],
  category_id: null as number | null,
  payment_channel_ids: [] as number[],
  is_affiliate_enabled: false,
  is_active: true,
  sort_order: 0,
  manual_form_schema: { fields: [] as ManualFormField[] },
})

const loadPaymentChannels = async () => {
  try {
    const res = await adminAPI.getPaymentChannels({ page: 1, page_size: 200 })
    paymentChannels.value = (res.data?.data ?? []).filter((ch: AdminPaymentChannel) => ch.is_active)
  } catch {
    paymentChannels.value = []
  }
}

const parsePaymentChannelIDs = (raw: string | number[] | null | undefined): number[] => {
  if (!raw) return []
  if (Array.isArray(raw)) return raw.filter((id) => Number.isFinite(id) && id > 0)
  if (typeof raw === 'string') {
    const trimmed = raw.trim()
    if (!trimmed || trimmed === '[]') return []
    try {
      const parsed = JSON.parse(trimmed)
      if (Array.isArray(parsed)) return parsed.filter((id: unknown) => typeof id === 'number' && id > 0)
    } catch { /* ignore */ }
  }
  return []
}

const togglePaymentChannel = (channelId: number) => {
  const idx = form.payment_channel_ids.indexOf(channelId)
  if (idx >= 0) {
    form.payment_channel_ids.splice(idx, 1)
  } else {
    form.payment_channel_ids.push(channelId)
  }
}

const getCurrentLangName = () => {
  return languages.value.find((item) => item.code === currentLang.value)?.name || t('admin.common.lang.zhCN')
}

const emptyI18nString = () => ({ 'zh-CN': '', 'zh-TW': '', 'en-US': '' })

const normalizeSeoMeta = (raw: Record<string, LocalizedText> | null | undefined) => {
  if (!raw) return { keywords: emptyI18nString(), description: emptyI18nString() }
  const normalize = (val: string | LocalizedText | null | undefined) => {
    if (!val) return emptyI18nString()
    if (typeof val === 'string') {
      return { 'zh-CN': val, 'zh-TW': '', 'en-US': '' }
    }
    if (typeof val === 'object') {
      return { 'zh-CN': val['zh-CN'] || '', 'zh-TW': val['zh-TW'] || '', 'en-US': val['en-US'] || '' }
    }
    return emptyI18nString()
  }
  return {
    keywords: normalize(raw.keywords),
    description: normalize(raw.description),
  }
}

const toSafeInt = (value: unknown) => {
  const num = Number(value)
  if (!Number.isFinite(num)) return 0
  return Math.max(Math.floor(num), 0)
}

const toSafeStockTotal = (value: unknown) => {
  const num = Number(value)
  if (!Number.isFinite(num)) return 0
  const integer = Math.floor(num)
  if (integer === -1) return -1
  return Math.max(integer, 0)
}

const isNotifiedError = (error: unknown) => {
  return Boolean((error as { __notified?: boolean } | null)?.__notified)
}

const resolveManualStockMetrics = (product: AdminProduct) => {
  const skuRows = Array.isArray(product?.skus) ? product.skus : []
  const activeRows = skuRows.filter((item: AdminProductSKU) => Boolean(item?.is_active))
  if (!activeRows.length) {
    return {
      total: toSafeStockTotal(product?.manual_stock_total),
      locked: toSafeInt(product?.manual_stock_locked),
      sold: toSafeInt(product?.manual_stock_sold),
    }
  }

  const locked = activeRows.reduce((sum: number, item: AdminProductSKU) => sum + toSafeInt(item?.manual_stock_locked), 0)
  const sold = activeRows.reduce((sum: number, item: AdminProductSKU) => sum + toSafeInt(item?.manual_stock_sold), 0)
  if (activeRows.some((item: AdminProductSKU) => toSafeStockTotal(item?.manual_stock_total) === -1)) {
    return { total: -1, locked, sold }
  }
  const total = activeRows.reduce((sum: number, item: AdminProductSKU) => sum + toSafeStockTotal(item?.manual_stock_total), 0)
  return { total, locked, sold }
}

const createManualFormField = () => ({
  key: '',
  type: 'text',
  required: false,
  label: createEmptyLocaleText(),
  placeholder: createEmptyLocaleText(),
  regex: '',
  min: '',
  max: '',
  max_len: '',
  options_text: '',
})

const manualFieldTypeOptions = computed(() => [
  { value: 'text', label: t('admin.products.form.manualFormFieldTypes.text') },
  { value: 'textarea', label: t('admin.products.form.manualFormFieldTypes.textarea') },
  { value: 'phone', label: t('admin.products.form.manualFormFieldTypes.phone') },
  { value: 'email', label: t('admin.products.form.manualFormFieldTypes.email') },
  { value: 'number', label: t('admin.products.form.manualFormFieldTypes.number') },
  { value: 'select', label: t('admin.products.form.manualFormFieldTypes.select') },
  { value: 'radio', label: t('admin.products.form.manualFormFieldTypes.radio') },
  { value: 'checkbox', label: t('admin.products.form.manualFormFieldTypes.checkbox') },
])

const stockDisplayModeOptions = computed(() => [
  { value: 'exact', label: t('admin.products.form.stockDisplayModes.exact') },
  { value: 'status', label: t('admin.products.form.stockDisplayModes.status') },
  { value: 'range', label: t('admin.products.form.stockDisplayModes.range') },
  { value: 'hidden', label: t('admin.products.form.stockDisplayModes.hidden') },
])

const normalizeLocaleText = (value: unknown) => {
  const result: Record<string, string> = {}
  const obj = (value && typeof value === 'object' ? value : {}) as Record<string, string>
  for (const code of ['zh-CN', 'zh-TW', 'en-US']) {
    const text = String(obj[code] || '').trim()
    if (text) {
      result[code] = text
    }
  }
  return result
}

const normalizeSpecValues = (value: unknown) => {
  const result: Record<string, string> = {}
  if (!value || typeof value !== 'object') {
    return result
  }
  const obj = value as Record<string, unknown>
  Object.keys(obj).forEach((key) => {
    const text = String(obj[key] ?? '').trim()
    if (text) {
      result[key] = text
    }
  })
  return result
}

const parseManualOptions = (value: unknown) => {
  if (Array.isArray(value)) {
    return value.map((item) => String(item || '').trim()).filter(Boolean)
  }
  return []
}

const parseManualFormSchemaForEdit = (rawSchema: Record<string, unknown> | null | undefined) => {
  const rawFields = Array.isArray(rawSchema?.fields) ? (rawSchema!.fields as Record<string, unknown>[]) : []
  const fields = rawFields.map((rawField: Record<string, unknown>) => {
    const type = String(rawField?.type || 'text').trim() || 'text'
    return {
      key: String(rawField?.key || '').trim(),
      type,
      required: Boolean(rawField?.required),
      label: {
        ...createEmptyLocaleText(),
        ...((rawField?.label || {}) as Record<string, string>),
      },
      placeholder: {
        ...createEmptyLocaleText(),
        ...((rawField?.placeholder || {}) as Record<string, string>),
      },
      regex: String(rawField?.regex || '').trim(),
      min: rawField?.min == null ? '' : String(rawField.min),
      max: rawField?.max == null ? '' : String(rawField.max),
      max_len: rawField?.max_len == null ? '' : String(rawField.max_len),
      options_text: parseManualOptions(rawField?.options).join('\n'),
    }
  })
  return { fields }
}

const normalizeManualFormSchemaForSubmit = () => {
  if (form.fulfillment_type !== 'manual') {
    return { fields: [] }
  }
  const fields = (form.manual_form_schema?.fields || [])
    .map((field: ManualFormField) => {
      const key = String(field?.key || '').trim()
      const type = String(field?.type || 'text').trim() || 'text'
      if (!key) {
        return null
      }
      const normalized: Record<string, unknown> = {
        key,
        type,
        required: Boolean(field?.required),
      }
      const label = normalizeLocaleText(field?.label)
      if (Object.keys(label).length) {
        normalized.label = label
      }
      const placeholder = normalizeLocaleText(field?.placeholder)
      if (Object.keys(placeholder).length) {
        normalized.placeholder = placeholder
      }
      const regex = String(field?.regex || '').trim()
      if (regex) {
        normalized.regex = regex
      }
      const min = Number(field?.min)
      if (Number.isFinite(min) && field?.min !== '') {
        normalized.min = min
      }
      const max = Number(field?.max)
      if (Number.isFinite(max) && field?.max !== '') {
        normalized.max = max
      }
      const maxLen = Number(field?.max_len)
      if (Number.isInteger(maxLen) && maxLen > 0) {
        normalized.max_len = maxLen
      }
      if (type === 'select' || type === 'radio' || type === 'checkbox') {
        const options = String(field?.options_text || '')
          .split(/\n|,/)
          .map((item) => item.trim())
          .filter(Boolean)
        if (options.length) {
          normalized.options = Array.from(new Set(options))
        }
      }
      return normalized
    })
    .filter(Boolean)

  return { fields }
}

const addManualFormField = () => {
  form.manual_form_schema.fields.push(createManualFormField())
}

const removeManualFormField = (index: number) => {
  form.manual_form_schema.fields.splice(index, 1)
}

const addSKU = () => {
  form.skus.push(
    createSKUFormItem({
      sku_code: `SKU-${form.skus.length + 1}`,
      price_amount: Number(form.price_amount || 0),
      agency_price_amount: Number(form.agency_price_amount || 0),
      manual_stock_total: toSafeStockTotal(form.manual_stock_total),
      is_active: true,
      sort_order: form.skus.length,
    })
  )
}

const removeSKU = (index: number) => {
  form.skus.splice(index, 1)
}

const normalizeSKUsForSubmit = () => {
  if (!form.skus.length) {
    return [] as Array<Record<string, unknown>>
  }

  const seenCode = new Set<string>()
  const normalized = form.skus.map((item, index) => {
    const skuCode = String(item.sku_code || '').trim()
    if (!skuCode) {
      throw new Error(t('admin.products.errors.skuCodeRequired', { index: index + 1 }))
    }
    const codeKey = skuCode.toLowerCase()
    if (seenCode.has(codeKey)) {
      throw new Error(t('admin.products.errors.skuCodeDuplicate', { code: skuCode }))
    }
    seenCode.add(codeKey)

    const priceAmount = Number(item.price_amount)
    if (!Number.isFinite(priceAmount) || priceAmount <= 0) {
      throw new Error(t('admin.products.errors.skuPriceInvalid', { index: index + 1 }))
    }

    const isActive = Boolean(item.is_active)
    const manualStockTotal = form.fulfillment_type === 'manual' ? toSafeStockTotal(item.manual_stock_total) : 0
    const specValues = normalizeSpecValues(item.spec_values)

    return {
      id: item.id > 0 ? item.id : undefined,
      sku_code: skuCode,
      spec_values: specValues,
      price_amount: priceAmount,
      agency_price_amount: Number(item.agency_price_amount) || 0,
      cost_price_amount: Number(item.cost_price_amount) || 0,
      manual_stock_total: manualStockTotal,
      is_active: isActive,
      sort_order: Number(item.sort_order) || 0,
    }
  })

  if (!normalized.some((item) => item.is_active)) {
    throw new Error(t('admin.products.errors.skuNeedActive'))
  }
  return normalized
}

const isTextLikeField = (fieldType: string) => {
  return fieldType === 'text' || fieldType === 'textarea' || fieldType === 'phone' || fieldType === 'email'
}

const resetForm = () => {
  initialCategoryID.value = null
  Object.assign(form, {
    id: 0,
    title: { 'zh-CN': '', 'zh-TW': '', 'en-US': '' },
    slug: '',
    seo_meta: { keywords: { 'zh-CN': '', 'zh-TW': '', 'en-US': '' }, description: { 'zh-CN': '', 'zh-TW': '', 'en-US': '' } },
    description: { 'zh-CN': '', 'zh-TW': '', 'en-US': '' },
    content: { 'zh-CN': '', 'zh-TW': '', 'en-US': '' },
    instructions: { 'zh-CN': '', 'zh-TW': '', 'en-US': '' },
    price_amount: 0,
    agency_price_amount: 0,
    cost_price_amount: 0,
    images: [],
    tags: [],
    purchase_type: 'member',
    min_purchase_quantity: '',
    max_purchase_quantity: '',
    stock_display_mode: 'exact',
    fulfillment_type: 'manual',
    manual_stock_total: 0,
    skus: [],
    category_id: null,
    payment_channel_ids: [],
    is_affiliate_enabled: false,
    is_active: true,
    sort_order: 0,
    manual_form_schema: { fields: [] },
  })
}

const populateForm = (product: AdminProduct) => {
  initialCategoryID.value = Number(product.category_id || 0) || null
  let imagesList: string[] = []
  if (product.images) {
    if (Array.isArray(product.images)) {
      imagesList = product.images
    } else if ((product.images as any).images && Array.isArray((product.images as any).images)) {
      imagesList = (product.images as any).images
    }
  }

  let tagsList: string[] = []
  if (product.tags) {
    if (Array.isArray(product.tags)) {
      tagsList = product.tags
    } else if ((product.tags as any).tags && Array.isArray((product.tags as any).tags)) {
      tagsList = (product.tags as any).tags
    }
  }

  Object.assign(form, {
    id: product.id,
    title: product.title || { 'zh-CN': '', 'zh-TW': '', 'en-US': '' },
    slug: product.slug,
    seo_meta: normalizeSeoMeta(product.seo_meta),
    description: product.description || { 'zh-CN': '', 'zh-TW': '', 'en-US': '' },
    content: product.content || { 'zh-CN': '', 'zh-TW': '', 'en-US': '' },
    instructions: (product as AdminProduct & { instructions?: LocalizedText }).instructions || { 'zh-CN': '', 'zh-TW': '', 'en-US': '' },
    price_amount: Number(product.price_amount || 0),
    agency_price_amount: Number(product.agency_price_amount || 0),
    cost_price_amount: Number(product.cost_price_amount || 0),
    images: imagesList,
    tags: tagsList,
    purchase_type: product.purchase_type || 'member',
    min_purchase_quantity: Number(product.min_purchase_quantity || 0) > 0 ? Math.floor(Number(product.min_purchase_quantity || 0)) : '',
    max_purchase_quantity: Number(product.max_purchase_quantity || 0) > 0 ? Math.floor(Number(product.max_purchase_quantity || 0)) : '',
    stock_display_mode: product.stock_display_mode || 'exact',
    fulfillment_type: product.fulfillment_type || 'manual',
    manual_stock_total: resolveManualStockMetrics(product).total,
    skus: resolveEditableSKURows(product),
    category_id: Number(product.category_id || 0) || null,
    payment_channel_ids: parsePaymentChannelIDs(product.payment_channel_ids),
    is_affiliate_enabled: Boolean(product.is_affiliate_enabled),
    is_active: product.is_active ?? true,
    sort_order: Number(product.sort_order || 0),
    manual_form_schema: parseManualFormSchemaForEdit(product.manual_form_schema),
  })
}

const closeModal = () => {
  emit('update:modelValue', false)
}

const handleSubmit = async () => {
  submitting.value = true
  try {
    const normalizedCategoryID = Number(form.category_id)
    if (!Number.isFinite(normalizedCategoryID) || normalizedCategoryID <= 0) {
      throw new Error(getCategoryRequiredError())
    }
    if (!isProductCategorySelectable(normalizedCategoryID)) {
      throw new Error(getCategoryLeafTip())
    }

    const normalizedSKUs = normalizeSKUsForSubmit()
    const activeSKU = normalizedSKUs.find((item) => item.is_active)
    let effectivePrice = Number(form.price_amount)
    let effectiveAgencyPrice = Number(form.agency_price_amount || 0)
    let effectiveCostPrice = Number(form.cost_price_amount)
    if (normalizedSKUs.length > 0) {
      const priceSource = activeSKU || normalizedSKUs[0]!
      effectivePrice = Number(priceSource.price_amount)
      effectiveAgencyPrice = Number(priceSource.agency_price_amount || 0)
      effectiveCostPrice = Number(priceSource.cost_price_amount || 0)
    }
    const normalizedMinPurchaseQuantity = Number(form.min_purchase_quantity)
    const normalizedMaxPurchaseQuantity = Number(form.max_purchase_quantity)
    const minPurchaseQuantityValue = Number.isFinite(normalizedMinPurchaseQuantity) && normalizedMinPurchaseQuantity > 0
      ? Math.floor(normalizedMinPurchaseQuantity)
      : 0
    const maxPurchaseQuantityValue = Number.isFinite(normalizedMaxPurchaseQuantity) && normalizedMaxPurchaseQuantity > 0
      ? Math.floor(normalizedMaxPurchaseQuantity)
      : 0
    if (minPurchaseQuantityValue > 0 && maxPurchaseQuantityValue > 0 && minPurchaseQuantityValue > maxPurchaseQuantityValue) {
      throw new Error(t('admin.products.form.purchaseLimitInvalid'))
    }
    const effectiveManualStockTotal = normalizedSKUs.length
      ? (() => {
          const activeRows = normalizedSKUs.filter((item) => item.is_active)
          if (activeRows.some((item) => toSafeStockTotal(item.manual_stock_total) === -1)) {
            return -1
          }
          return activeRows.reduce((sum, item) => sum + toSafeStockTotal(item.manual_stock_total), 0)
        })()
      : toSafeStockTotal(form.manual_stock_total)

    const payload = {
      slug: String(form.slug || '').trim(),
      category_id: Math.floor(normalizedCategoryID),
      seo_meta: form.seo_meta,
      title: form.title,
      description: form.description,
      content: form.content,
      instructions: form.instructions,
      price_amount: effectivePrice,
      agency_price_amount: effectiveAgencyPrice,
      cost_price_amount: effectiveCostPrice,
      images: form.images,
      tags: form.tags,
      purchase_type: form.purchase_type,
      min_purchase_quantity: minPurchaseQuantityValue,
      max_purchase_quantity: maxPurchaseQuantityValue,
      stock_display_mode: form.stock_display_mode,
      fulfillment_type: form.fulfillment_type,
      manual_stock_total: effectiveManualStockTotal,
      skus: normalizedSKUs,
      payment_channel_ids: form.payment_channel_ids.length > 0 ? form.payment_channel_ids : [],
      is_affiliate_enabled: form.is_affiliate_enabled,
      is_active: form.is_active,
      sort_order: Number(form.sort_order) || 0,
      manual_form_schema: normalizeManualFormSchemaForSubmit(),
    }

    if (isEditing.value) {
      await adminAPI.updateProduct(form.id, payload as unknown as Partial<AdminProduct>)
    } else {
      await adminAPI.createProduct(payload as unknown as Partial<AdminProduct>)
    }
    closeModal()
    emit('success')
  } catch (err: any) {
    if (isNotifiedError(err)) return
    notifyError(t('admin.products.errors.operationFailed', { message: err?.message || '' }))
  } finally {
    submitting.value = false
  }
}

const addTag = () => {
  if (newTag.value.trim() && !form.tags.includes(newTag.value.trim())) {
    form.tags.push(newTag.value.trim())
    newTag.value = ''
  }
}

const removeTag = (index: number) => {
  form.tags.splice(index, 1)
}

// Watch productId to determine create vs edit mode and fetch product details
watch(
  () => props.productId,
  async (newId) => {
    if (!props.modelValue) return
    currentLang.value = 'zh-CN'
    if (newId != null && newId > 0) {
      // Edit mode
      isEditing.value = true
      try {
        const res = await adminAPI.getProduct(newId)
        const product = res.data.data
        editingIsMapped.value = Boolean(product.is_mapped)
        populateForm(product)
      } catch (err: any) {
        if (isNotifiedError(err)) return
        notifyError(t('admin.products.errors.operationFailed', { message: err?.message || '' }))
        closeModal()
      }
    } else {
      // Create mode
      isEditing.value = false
      editingIsMapped.value = false
      resetForm()
    }
  }
)

// Also handle when modal opens
watch(
  () => props.modelValue,
  (newVal) => {
    if (!newVal) return
    currentLang.value = 'zh-CN'
    if (paymentChannels.value.length === 0) {
      loadPaymentChannels()
    }
    if (props.productId != null && props.productId > 0) {
      isEditing.value = true
      adminAPI.getProduct(props.productId).then((res) => {
        const product = res.data.data
        editingIsMapped.value = Boolean(product.is_mapped)
        populateForm(product)
      }).catch((err: any) => {
        if (isNotifiedError(err)) return
        notifyError(t('admin.products.errors.operationFailed', { message: err?.message || '' }))
        closeModal()
      })
    } else {
      isEditing.value = false
      editingIsMapped.value = false
      resetForm()
    }
  }
)
</script>

<template>
  <Dialog :open="modelValue" @update:open="(value) => { if (!value) closeModal() }">
    <DialogScrollContent class="w-[calc(100vw-1rem)] max-w-5xl p-4 sm:p-6" @interact-outside="(e) => e.preventDefault()">
      <DialogHeader>
        <DialogTitle>{{ isEditing ? t('admin.products.modal.editTitle') : t('admin.products.modal.createTitle') }}</DialogTitle>
      </DialogHeader>
      <form class="space-y-6" @submit.prevent="handleSubmit">
        <div class="border-b border-border">
          <div class="flex gap-2 overflow-x-auto pb-1 sm:gap-4">
            <button
              v-for="lang in languages"
              :key="lang.code"
              type="button"
              class="shrink-0 border-b-2 px-3 py-2 text-sm font-medium sm:px-4"
              :class="currentLang === lang.code ? 'border-primary text-foreground' : 'border-transparent text-muted-foreground hover:text-foreground'"
              @click="currentLang = lang.code"
            >
              {{ lang.name }}
            </button>
          </div>
        </div>

        <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
          <div class="col-span-1 md:col-span-2">
            <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.products.form.title', { lang: getCurrentLangName() }) }}</label>
            <Input v-model="form.title[currentLang]" required :placeholder="t('admin.products.form.titlePlaceholder')" />
          </div>

          <div class="col-span-1">
            <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.products.form.slug') }}</label>
            <Input v-model="form.slug" required :placeholder="t('admin.products.form.slugPlaceholder')" />
            <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.products.form.slugTip') }}</p>
          </div>

          <div class="col-span-1">
            <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.products.form.seoMetaKeywords', { lang: getCurrentLangName() }) }}</label>
            <Input v-model="form.seo_meta.keywords[currentLang]" :placeholder="t('admin.products.form.seoMetaKeywordsPlaceholder')" />
            <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.products.form.seoMetaKeywordsTip') }}</p>
          </div>

          <div class="col-span-1 md:col-span-2">
            <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.products.form.seoMetaDescription', { lang: getCurrentLangName() }) }}</label>
            <Textarea v-model="form.seo_meta.description[currentLang]" :placeholder="t('admin.products.form.seoMetaDescriptionPlaceholder')" class="min-h-[80px]" />
            <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.products.form.seoMetaDescriptionTip') }}</p>
          </div>

          <div class="col-span-1">
            <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.products.form.category') }}</label>
            <Select
              :model-value="form.category_id ? String(form.category_id) : '__none__'"
              @update:modelValue="(value) => { form.category_id = value && value !== '__none__' ? Number(value) : null }"
            >
              <SelectTrigger class="h-9 w-full">
                <SelectValue :placeholder="t('admin.products.form.categoryPlaceholder')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="__none__">{{ t('admin.products.form.categoryPlaceholder') }}</SelectItem>
                <SelectItem
                  v-for="item in categoryOptions"
                  :key="item.category.id"
                  :value="String(item.category.id)"
                  :disabled="!item.selectable"
                  :class="item.depth > 0 ? 'pl-6' : ''"
                >
                  {{ item.depth > 0 ? getCategoryOptionName(item.category) : getCategoryOptionLabel(item.category) }}
                </SelectItem>
              </SelectContent>
            </Select>
            <p class="mt-1 text-xs text-muted-foreground">{{ getCategoryLeafTip() }}</p>
          </div>

          <div class="col-span-1">
            <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.products.form.purchaseType') }}</label>
            <Select v-model="form.purchase_type">
              <SelectTrigger class="h-9 w-full">
                <SelectValue :placeholder="t('admin.products.purchaseType.member')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="member">{{ t('admin.products.purchaseType.member') }}</SelectItem>
                <SelectItem value="guest">{{ t('admin.products.purchaseType.guest') }}</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div class="col-span-1">
            <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.products.form.minPurchaseQuantity') }}</label>
            <Input
              v-model.number="form.min_purchase_quantity"
              type="number"
              min="1"
              :placeholder="t('admin.products.form.minPurchaseQuantityPlaceholder')"
            />
            <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.products.form.minPurchaseQuantityTip') }}</p>
          </div>

          <div class="col-span-1">
            <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.products.form.maxPurchaseQuantity') }}</label>
            <Input
              v-model.number="form.max_purchase_quantity"
              type="number"
              min="1"
              :placeholder="t('admin.products.form.maxPurchaseQuantityPlaceholder')"
            />
            <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.products.form.maxPurchaseQuantityTip') }}</p>
          </div>

          <div class="col-span-1">
            <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.products.form.stockDisplayMode') }}</label>
            <Select v-model="form.stock_display_mode">
              <SelectTrigger class="h-9 w-full">
                <SelectValue :placeholder="t('admin.products.form.stockDisplayModes.exact')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="item in stockDisplayModeOptions" :key="item.value" :value="item.value">
                  {{ item.label }}
                </SelectItem>
              </SelectContent>
            </Select>
            <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.products.form.stockDisplayModeTip') }}</p>
          </div>

          <div class="col-span-1">
            <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.products.form.fulfillmentType') }}</label>
            <Select v-model="form.fulfillment_type" :disabled="editingIsMapped">
              <SelectTrigger class="h-9 w-full">
                <SelectValue :placeholder="t('admin.products.fulfillmentType.manual')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="manual">{{ t('admin.products.fulfillmentType.manual') }}</SelectItem>
                <SelectItem value="auto">{{ t('admin.products.fulfillmentType.auto') }}</SelectItem>
              </SelectContent>
            </Select>
            <p v-if="editingIsMapped" class="mt-1 text-xs text-indigo-600">{{ t('admin.products.mappedFulfillmentLocked') }}</p>
          </div>

          <div class="col-span-1">
            <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.products.form.manualStockTotal') }}</label>
            <Input
              v-model.number="form.manual_stock_total"
              type="number"
              min="-1"
              :placeholder="t('admin.products.form.manualStockTotalPlaceholder')"
              :disabled="form.skus.length > 0"
            />
            <p v-if="form.skus.length > 0" class="mt-1 text-xs text-muted-foreground">
              {{ t('admin.products.form.manualStockTotalSkuTip') }}
            </p>
            <p v-else class="mt-1 text-xs text-muted-foreground">{{ t('admin.products.form.manualStockTotalTip') }}</p>
          </div>

          <div v-if="form.fulfillment_type === 'manual' || editingIsMapped" class="col-span-1 md:col-span-2 rounded-xl border border-border bg-muted/20 p-4 space-y-4">
            <div class="flex items-center justify-between">
              <div>
                <h3 class="text-sm font-semibold text-foreground">{{ t('admin.products.form.manualFormSchemaTitle') }}</h3>
                <p class="text-xs text-muted-foreground mt-1">{{ t('admin.products.form.manualFormSchemaTip') }}</p>
                <p v-if="editingIsMapped" class="mt-1 text-xs text-indigo-600">{{ t('admin.products.mappedFormSchemaLocked') }}</p>
              </div>
              <Button v-if="!editingIsMapped" type="button" size="sm" variant="outline" @click="addManualFormField">
                {{ t('admin.products.form.manualFormAddField') }}
              </Button>
            </div>

            <div v-if="!form.manual_form_schema.fields.length" class="rounded-lg border border-dashed border-border p-4 text-xs text-muted-foreground">
              {{ t('admin.products.form.manualFormEmpty') }}
            </div>

            <div v-for="(field, index) in form.manual_form_schema.fields" :key="index" class="rounded-lg border border-border bg-background p-4 space-y-3">
              <div class="flex items-center justify-between">
                <div class="text-xs font-medium text-muted-foreground">{{ t('admin.products.form.manualFormFieldTitle', { index: index + 1 }) }}</div>
                <Button v-if="!editingIsMapped" type="button" variant="destructive" size="sm" @click="removeManualFormField(index)">
                  {{ t('admin.products.form.manualFormRemoveField') }}
                </Button>
              </div>

              <div class="grid grid-cols-1 md:grid-cols-3 gap-3">
                <div>
                  <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.products.form.manualFormFieldKey') }}</label>
                  <Input v-model="field.key" :placeholder="t('admin.products.form.manualFormFieldKeyPlaceholder')" :disabled="editingIsMapped" />
                </div>
                <div>
                  <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.products.form.manualFormFieldType') }}</label>
                  <Select v-model="field.type" :disabled="editingIsMapped">
                    <SelectTrigger class="h-9">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem v-for="typeItem in manualFieldTypeOptions" :key="typeItem.value" :value="typeItem.value">{{ typeItem.label }}</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div class="flex items-end">
                  <div class="inline-flex items-center gap-2">
                    <Switch v-model="field.required" :disabled="editingIsMapped" />
                    <Label class="text-sm text-muted-foreground">{{ t('admin.products.form.manualFormFieldRequired') }}</Label>
                  </div>
                </div>
              </div>

              <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
                <div>
                  <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.products.form.manualFormFieldLabel', { lang: getCurrentLangName() }) }}</label>
                  <Input v-model="field.label[currentLang]" :placeholder="t('admin.products.form.manualFormFieldLabelPlaceholder')" :disabled="editingIsMapped" />
                </div>
                <div>
                  <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.products.form.manualFormFieldPlaceholder', { lang: getCurrentLangName() }) }}</label>
                  <Input v-model="field.placeholder[currentLang]" :placeholder="t('admin.products.form.manualFormFieldPlaceholderPlaceholder')" :disabled="editingIsMapped" />
                </div>
              </div>

              <div v-if="isTextLikeField(field.type)" class="grid grid-cols-1 md:grid-cols-2 gap-3">
                <div>
                  <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.products.form.manualFormFieldRegex') }}</label>
                  <Input v-model="field.regex" :placeholder="t('admin.products.form.manualFormFieldRegexPlaceholder')" :disabled="editingIsMapped" />
                </div>
                <div>
                  <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.products.form.manualFormFieldMaxLength') }}</label>
                  <Input v-model="field.max_len" type="number" min="1" :placeholder="t('admin.products.form.manualFormFieldMaxLengthPlaceholder')" :disabled="editingIsMapped" />
                </div>
              </div>

              <div v-if="field.type === 'number'" class="grid grid-cols-1 md:grid-cols-2 gap-3">
                <div>
                  <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.products.form.manualFormFieldMin') }}</label>
                  <Input v-model="field.min" type="number" :placeholder="t('admin.products.form.manualFormFieldMinPlaceholder')" :disabled="editingIsMapped" />
                </div>
                <div>
                  <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.products.form.manualFormFieldMax') }}</label>
                  <Input v-model="field.max" type="number" :placeholder="t('admin.products.form.manualFormFieldMaxPlaceholder')" :disabled="editingIsMapped" />
                </div>
              </div>

              <div v-if="field.type === 'select' || field.type === 'radio' || field.type === 'checkbox'">
                <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.products.form.manualFormFieldOptions') }}</label>
                <Textarea v-model="field.options_text" rows="4" :placeholder="t('admin.products.form.manualFormFieldOptionsPlaceholder')" :disabled="editingIsMapped" />
                <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.products.form.manualFormFieldOptionsTip') }}</p>
              </div>
            </div>
          </div>

          <div class="col-span-1 md:col-span-2 rounded-xl border border-border bg-muted/20 p-4 space-y-4">
            <div class="flex items-center justify-between">
              <div>
                <h3 class="text-sm font-semibold text-foreground">{{ t('admin.products.form.skuTitle') }}</h3>
                <p class="text-xs text-muted-foreground mt-1">{{ t('admin.products.form.skuTip') }}</p>
                <p v-if="editingIsMapped" class="mt-1 text-xs text-indigo-600">{{ t('admin.products.mappedSkuLocked') }}</p>
              </div>
              <Button v-if="!editingIsMapped" type="button" size="sm" variant="outline" @click="addSKU">
                {{ t('admin.products.form.skuAdd') }}
              </Button>
            </div>

            <div v-if="!form.skus.length" class="rounded-lg border border-dashed border-border p-4 text-xs text-muted-foreground">
              {{ t('admin.products.form.skuEmpty') }}
            </div>

            <div v-for="(sku, index) in form.skus" :key="`sku-${index}-${sku.id || 0}`" class="rounded-lg border border-border bg-background p-4 space-y-3">
              <div class="flex items-center justify-between">
                <div class="text-xs font-medium text-muted-foreground">{{ t('admin.products.form.skuItemTitle', { index: index + 1 }) }}</div>
                <Button v-if="!editingIsMapped" type="button" size="sm" variant="destructive" @click="removeSKU(index)">
                  {{ t('admin.products.form.skuRemove') }}
                </Button>
              </div>

              <div class="grid grid-cols-1 md:grid-cols-7 gap-3">
                <div class="md:col-span-1">
                  <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.products.form.skuCode') }}</label>
                  <Input v-model="sku.sku_code" :placeholder="t('admin.products.form.skuCodePlaceholder')" :disabled="editingIsMapped" />
                </div>
                <div :class="form.fulfillment_type === 'manual' ? 'md:col-span-1' : 'md:col-span-2'">
                  <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.products.form.skuSpec', { lang: getCurrentLangName() }) }}</label>
                  <Input v-model="sku.spec_values[currentLang]" :placeholder="t('admin.products.form.skuSpecPlaceholder')" :disabled="editingIsMapped" />
                </div>
                <div class="md:col-span-1">
                  <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.products.form.skuPrice') }}</label>
                  <Input v-model.number="sku.price_amount" type="number" step="0.01" min="0" :placeholder="t('admin.products.form.skuPricePlaceholder')" />
                </div>
                <div class="md:col-span-1">
                  <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.products.form.skuAgencyPrice') }}</label>
                  <Input v-model.number="sku.agency_price_amount" type="number" step="0.01" min="0" :placeholder="t('admin.products.form.skuAgencyPricePlaceholder')" />
                </div>
                <div class="md:col-span-1">
                  <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.products.form.skuCostPrice') }}</label>
                  <Input v-model.number="sku.cost_price_amount" type="number" step="0.01" min="0" :placeholder="t('admin.products.form.skuCostPricePlaceholder')" :disabled="editingIsMapped" />
                  <p v-if="editingIsMapped" class="mt-1 text-xs text-muted-foreground">{{ t('admin.products.form.costPriceAutoFromUpstream') }}</p>
                </div>
                <div v-if="form.fulfillment_type === 'manual'" class="md:col-span-1">
                  <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.products.form.skuManualStock') }}</label>
                  <Input v-model.number="sku.manual_stock_total" type="number" min="-1" :placeholder="t('admin.products.form.skuManualStockPlaceholder')" />
                </div>
                <div class="md:col-span-1 grid grid-cols-1 gap-3 sm:grid-cols-2">
                  <div>
                    <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.products.form.skuSort') }}</label>
                    <Input v-model.number="sku.sort_order" type="number" :placeholder="t('admin.products.form.skuSortPlaceholder')" />
                    <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.products.form.skuSortTip') }}</p>
                  </div>
                  <div class="flex items-end">
                    <div class="inline-flex items-center gap-2">
                      <Switch v-model="sku.is_active" />
                      <Label class="text-sm text-muted-foreground">{{ t('admin.products.form.skuActive') }}</Label>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div class="col-span-1">
            <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.products.form.priceAmount') }}</label>
            <Input
              v-model.number="form.price_amount"
              type="number"
              step="0.01"
              min="0"
              required
              :placeholder="t('admin.products.form.priceAmountPlaceholder')"
              :disabled="form.skus.length > 0"
            />
            <p v-if="form.skus.length > 0" class="mt-1 text-xs text-muted-foreground">
              {{ t('admin.products.form.priceAmountSkuTip') }}
            </p>
            <p v-else class="mt-1 text-xs text-muted-foreground">{{ t('admin.products.form.priceAmountTip') }}</p>
          </div>

          <div class="col-span-1">
            <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.products.form.agencyPriceAmount') }}</label>
            <Input
              v-model.number="form.agency_price_amount"
              type="number"
              step="0.01"
              min="0"
              :placeholder="t('admin.products.form.agencyPriceAmountPlaceholder')"
              :disabled="form.skus.length > 0"
            />
            <p v-if="form.skus.length > 0" class="mt-1 text-xs text-muted-foreground">
              {{ t('admin.products.form.agencyPriceAmountSkuTip') }}
            </p>
            <p v-else class="mt-1 text-xs text-muted-foreground">{{ t('admin.products.form.agencyPriceAmountTip') }}</p>
          </div>

          <div class="col-span-1">
            <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.products.form.costPriceAmount') }}</label>
            <Input
              v-model.number="form.cost_price_amount"
              type="number"
              step="0.01"
              min="0"
              :placeholder="t('admin.products.form.costPriceAmountPlaceholder')"
              :disabled="form.skus.length > 0 || editingIsMapped"
            />
            <p v-if="form.skus.length > 0" class="mt-1 text-xs text-muted-foreground">
              {{ t('admin.products.form.costPriceAmountSkuTip') }}
            </p>
            <p v-else-if="editingIsMapped" class="mt-1 text-xs text-muted-foreground">{{ t('admin.products.form.costPriceAutoFromUpstream') }}</p>
          </div>

          <div class="col-span-1">
            <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.products.form.sortOrder') }}</label>
            <Input v-model.number="form.sort_order" type="number" placeholder="0" />
            <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.products.form.sortTip') }}</p>
          </div>

          <div class="col-span-1 md:col-span-2">
            <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.products.form.images') }}</label>
            <MediaPicker v-model="form.images" :multiple="true" scene="product" />
          </div>

          <div class="col-span-1 md:col-span-2">
            <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.products.form.description', { lang: getCurrentLangName() }) }}</label>
            <Textarea v-model="form.description[currentLang]" rows="3" :placeholder="t('admin.products.form.descriptionPlaceholder')" />
          </div>

          <div class="col-span-1 md:col-span-2">
            <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.products.form.content', { lang: getCurrentLangName() }) }}</label>
            <RichEditor :model-value="form.content[currentLang] || ''" @update:model-value="(v: string) => form.content[currentLang] = v" :placeholder="t('admin.products.form.contentPlaceholder')" />
            <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.products.form.contentTip') }}</p>
          </div>

          <div class="col-span-1 md:col-span-2">
            <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.products.form.instructions', { lang: getCurrentLangName() }) }}</label>
            <RichEditor :model-value="form.instructions[currentLang] || ''" @update:model-value="(v: string) => form.instructions[currentLang] = v" :placeholder="t('admin.products.form.instructionsPlaceholder')" />
            <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.products.form.instructionsTip') }}</p>
          </div>

          <div class="col-span-1 md:col-span-2">
            <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.products.form.tags') }}</label>
            <div class="flex flex-wrap gap-2 mb-2">
              <span v-for="(tag, index) in form.tags" :key="index" class="rounded-lg border border-border px-3 py-1 text-xs text-muted-foreground flex items-center gap-1">
                {{ tag }}
                <button type="button" class="hover:text-foreground" @click="removeTag(index)">x</button>
              </span>
            </div>
            <div class="flex flex-col gap-2 sm:flex-row">
              <Input v-model="newTag" :placeholder="t('admin.products.form.tagsPlaceholder')" @keydown.enter.prevent="addTag" />
              <Button type="button" variant="outline" class="w-full sm:w-auto" @click="addTag">{{ t('admin.products.actions.addTag') }}</Button>
            </div>
          </div>

          <div v-if="paymentChannels.length > 0" class="col-span-1 md:col-span-2">
            <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.products.form.paymentChannels') }}</label>
            <div class="flex flex-wrap gap-2">
              <div v-for="ch in paymentChannels" :key="ch.id" class="inline-flex items-center gap-1.5 rounded-lg border border-border px-3 py-1.5 text-xs cursor-pointer select-none" :class="form.payment_channel_ids.includes(ch.id) ? 'bg-primary/10 border-primary text-primary' : 'text-muted-foreground hover:border-primary/40'" @click="togglePaymentChannel(ch.id)">
                <Checkbox
                  :model-value="form.payment_channel_ids.includes(ch.id)"
                  @update:model-value="() => togglePaymentChannel(ch.id)"
                  @click.stop
                />
                {{ ch.name }}
              </div>
            </div>
            <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.products.form.paymentChannelsTip') }}</p>
          </div>

          <div class="col-span-1 md:col-span-2 flex flex-col items-start gap-4 border-t border-border pt-4 sm:flex-row sm:flex-wrap sm:items-center sm:gap-6">
            <div class="inline-flex items-center gap-2">
              <Switch id="is_affiliate_enabled" v-model="form.is_affiliate_enabled" />
              <Label for="is_affiliate_enabled" class="text-sm text-muted-foreground select-none">{{ t('admin.products.form.affiliateEnabled') }}</Label>
            </div>
            <div class="inline-flex items-center gap-2">
              <Switch id="is_active" v-model="form.is_active" />
              <Label for="is_active" class="text-sm text-muted-foreground select-none">{{ t('admin.products.form.activeNow') }}</Label>
            </div>
          </div>
        </div>

        <div class="flex flex-col-reverse gap-3 border-t border-border pt-6 sm:flex-row sm:justify-end">
          <Button type="button" variant="outline" class="w-full sm:w-auto" @click="closeModal">{{ t('admin.common.cancel') }}</Button>
          <Button type="submit" class="w-full sm:w-auto" :disabled="submitting">
            {{ submitting ? t('admin.products.actions.submitting') : isEditing ? t('admin.products.actions.saveChanges') : t('admin.products.actions.createNow') }}
          </Button>
        </div>
      </form>
    </DialogScrollContent>
  </Dialog>
</template>
