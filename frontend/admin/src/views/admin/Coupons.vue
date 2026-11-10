<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { adminAPI } from '@/api/admin'
import type { AdminCoupon, AdminMemberLevel, AdminProduct, AdminProductSKU } from '@/api/types'
import IdCell from '@/components/IdCell.vue'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Dialog, DialogHeader, DialogScrollContent, DialogTitle } from '@/components/ui/dialog'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import TableSkeleton from '@/components/TableSkeleton.vue'
import ListPagination from '@/components/ListPagination.vue'
import { useListRefresh, type ListFetchOptions } from '@/composables/useListRefresh'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { MultiSelect } from '@/components/ui/multi-select'
import { formatDate, getLocalizedText } from '@/utils/format'
import { formatSkuDisplayLabel } from '@/utils/sku'
import { notifyError } from '@/utils/notify'
import { confirmAction } from '@/utils/confirm'
import { useFormValidation, rules } from '@/composables/useFormValidation'

const loading = ref(true)
const { refreshing, refreshList } = useListRefresh()
const coupons = ref<AdminCoupon[]>([])
const pagination = ref({
  page: 1,
  page_size: 20,
  total: 0,
  total_page: 1,
})
const filters = reactive({
  id: '',
  code: '',
  scopeRefId: '__all__',
  isActive: '__all__',
})
const autoOpenId = ref<number | null>(null)
const normalizeFilterValue = (value: string) => (value === '__all__' ? '' : value)
const normalizeScopeFilterValue = (value: string) => (value === '__all__' ? '' : value)

const showModal = ref(false)
const submitting = ref(false)
const error = ref('')
const isEditing = ref(false)
const editingId = ref<number | null>(null)
const scopeFilterKeyword = ref('')
const productKeyword = ref('')
const productOptions = ref<AdminProduct[]>([])
const productOptionsLoading = ref(false)
const memberLevels = ref<AdminMemberLevel[]>([])
const selectedScopeIDs = ref<number[]>([])
// 选中的规格 ID；为空表示已选商品下的全部规格
const selectedSkuIDs = ref<number[]>([])
const form = reactive({
  code: '',
  type: 'percent',
  value: 0,
  min_amount: 0,
  max_discount: 0,
  usage_limit: 0,
  per_user_limit: 0,
  disabled_wholesale_price: false,
  per_item_discount: false,
  payment_roles: [] as string[],
  member_levels: [] as number[],
  starts_at: '',
  ends_at: '',
  is_active: true,
})
const { t, locale } = useI18n()
const route = useRoute()

const couponSchema = {
  code: [rules.required()],
  type: [rules.required()],
  value: [rules.required(), rules.numeric(), rules.min(0)],
}
const { errors, validate, clearErrors } = useFormValidation(couponSchema)

const applyRouteFilter = () => {
  const rawCode = route.query.code
  if (typeof rawCode === 'string' && rawCode.trim() !== '') {
    filters.code = rawCode.trim()
  } else {
    filters.code = ''
  }
  const rawId = route.query.id || route.query.coupon_id
  const parsed = Number(rawId)
  if (Number.isFinite(parsed) && parsed > 0) {
    filters.id = String(parsed)
    autoOpenId.value = parsed
  } else {
    filters.id = ''
    autoOpenId.value = null
  }

  const rawScopeRefID = Number(route.query.scope_ref_id)
  if (Number.isFinite(rawScopeRefID) && rawScopeRefID > 0) {
    filters.scopeRefId = String(Math.floor(rawScopeRefID))
  } else {
    filters.scopeRefId = '__all__'
  }
}

const discountTypeLabel = (type: string) => {
  const map: Record<string, string> = {
    percent: t('admin.common.discountTypes.percent'),
    fixed: t('admin.common.discountTypes.fixed'),
  }
  return map[type] || type
}

const paymentRoleOptions = computed(() => [
  { value: 'guest', label: t('admin.coupons.paymentRoles.guest') },
  { value: 'member', label: t('admin.coupons.paymentRoles.member') },
])

const memberLevelOptions = computed(() =>
  memberLevels.value.map((item) => ({
    value: item.id,
    label: getLocalizedText(item.name) || `#${item.id}`,
  }))
)

const resetForm = () => {
  form.code = ''
  form.type = 'percent'
  form.value = 0
  form.min_amount = 0
  form.max_discount = 0
  form.usage_limit = 0
  form.per_user_limit = 0
  form.disabled_wholesale_price = false
  form.per_item_discount = false
  form.payment_roles = []
  form.member_levels = []
  form.starts_at = ''
  form.ends_at = ''
  form.is_active = true
  selectedScopeIDs.value = []
  selectedSkuIDs.value = []
  scopeSkuCache.clear()
  editingId.value = null
  isEditing.value = false
}

const toISO = (raw: string) => {
  if (!raw) return ''
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return raw
  return date.toISOString()
}

const toLocalInput = (raw?: string) => {
  if (!raw) return ''
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return ''
  const offset = date.getTimezoneOffset()
  const local = new Date(date.getTime() - offset * 60000)
  return local.toISOString().slice(0, 16)
}

const normalizeScopeIDs = (raw: unknown) => {
  if (Array.isArray(raw)) {
    return Array.from(
      new Set(
        raw
          .map((item) => Number(item))
          .filter((item) => Number.isFinite(item) && item > 0)
          .map((item) => Math.floor(item))
      )
    )
  }
  if (typeof raw === 'string') {
    const text = raw.trim()
    if (!text) return []
    try {
      const parsed = JSON.parse(text)
      if (Array.isArray(parsed)) return normalizeScopeIDs(parsed)
    } catch (_err) {
      // 兼容历史逗号分隔的输入格式
    }
    return Array.from(
      new Set(
        text
          .split(/[,，\s]+/)
          .map((item) => Number(item))
          .filter((item) => Number.isFinite(item) && item > 0)
          .map((item) => Math.floor(item))
      )
    )
  }
  return []
}

const normalizePaymentRoles = (raw: unknown) => {
  if (Array.isArray(raw)) {
    const allowed = new Set(['guest', 'member'])
    return Array.from(
      new Set(
        raw
          .map((item) => String(item || '').trim().toLowerCase())
          .filter((item) => allowed.has(item))
      )
    )
  }
  return []
}

const normalizeMemberLevels = (raw: unknown) => {
  if (Array.isArray(raw)) {
    return Array.from(
      new Set(
        raw
          .map((item) => Number(item))
          .filter((item) => Number.isFinite(item) && item > 0)
          .map((item) => Math.floor(item))
      )
    )
  }
  return []
}

const buildProductLabel = (product: AdminProduct) => {
  const id = Number(product?.id || 0)
  const name = getLocalizedText(product?.title || {})
  if (id > 0 && name) return `#${id} ${name}`
  if (id > 0) return `#${id}`
  return name || '-'
}

// 已选商品被搜索条件过滤掉时会退化成占位项。这里缓存它此前加载到的规格，
// 占位项补回规格，管理员才能继续勾选，新校验也不会卡在「可见商品没规格」上。
const scopeSkuCache = new Map<number, AdminProductSKU[]>()

const ensureScopeProductsInOptions = (rows: AdminProduct[]) => {
  if (!selectedScopeIDs.value.length) return rows
  const exists = new Set(
    rows.map((item) => Number(item?.id || 0)).filter((id: number) => Number.isFinite(id) && id > 0)
  )
  selectedScopeIDs.value.forEach((id) => {
    if (exists.has(id)) return
    rows.unshift({
      id,
      title: {
        'zh-CN': `#${id}`,
        'zh-TW': `#${id}`,
        'en-US': `#${id}`,
      },
      skus: scopeSkuCache.get(id) || [],
    } as unknown as AdminProduct)
    exists.add(id)
  })
  return rows
}

const loadProductOptions = async (keywordInput?: string) => {
  productOptionsLoading.value = true
  try {
    const keyword = String((keywordInput ?? productKeyword.value) || '').trim()
    const rows: AdminProduct[] = []
    let page = 1
    let totalPage = 1
    do {
      const response = await adminAPI.getProducts({
        page,
        page_size: 100,
        search: keyword || undefined,
      })
      const list = Array.isArray(response.data.data) ? response.data.data : []
      rows.push(...list)
      totalPage = Number(response.data?.pagination?.total_page || 1)
      page += 1
    } while (page <= totalPage && page <= 20)

    const dedup = new Map<number, AdminProduct>()
    rows.forEach((item) => {
      const id = Number(item?.id || 0)
      if (!Number.isFinite(id) || id <= 0) return
      if (!dedup.has(id)) dedup.set(id, item)
      const skus = Array.isArray(item?.skus) ? item.skus : []
      if (skus.length) scopeSkuCache.set(Math.floor(id), skus)
    })
    productOptions.value = ensureScopeProductsInOptions(Array.from(dedup.values()))
  } catch {
    productOptions.value = ensureScopeProductsInOptions([])
  } finally {
    productOptionsLoading.value = false
  }
}

const loadMemberLevels = async () => {
  try {
    const response = await adminAPI.getMemberLevels({ page: 1, page_size: 200 })
    memberLevels.value = Array.isArray(response.data.data) ? response.data.data : []
  } catch {
    memberLevels.value = []
  }
}

const handleSearchProducts = async () => {
  await loadProductOptions(productKeyword.value)
}

const handleSearchScopeProducts = async () => {
  await loadProductOptions(scopeFilterKeyword.value)
}

const toggleScopeProduct = (rawProductID: number | string, v: boolean | 'indeterminate') => {
  const productID = Number(rawProductID)
  if (!Number.isFinite(productID) || productID <= 0) return
  const normalizedID = Math.floor(productID)
  if (v === true) {
    selectedScopeIDs.value = Array.from(new Set([...selectedScopeIDs.value, normalizedID])).sort((a, b) => a - b)
  } else {
    selectedScopeIDs.value = selectedScopeIDs.value.filter((id) => id !== normalizedID)
    const product = productOptions.value.find((item) => Number(item?.id || 0) === normalizedID)
    const removed = new Set((product?.skus || []).map((sku) => Number(sku.id)))
    selectedSkuIDs.value = selectedSkuIDs.value.filter((id) => !removed.has(id))
  }
}

const scopeSkuChecked = (rawSkuID: number | string) => selectedSkuIDs.value.includes(Math.floor(Number(rawSkuID)))

const toggleScopeSku = (rawSkuID: number | string, v: boolean | 'indeterminate') => {
  const skuID = Math.floor(Number(rawSkuID))
  if (!Number.isFinite(skuID) || skuID <= 0) return
  if (v === true) {
    selectedSkuIDs.value = Array.from(new Set([...selectedSkuIDs.value, skuID])).sort((a, b) => a - b)
  } else {
    selectedSkuIDs.value = selectedSkuIDs.value.filter((id) => id !== skuID)
  }
}

const buildSkuLabel = (sku: AdminProductSKU) =>
  `#${sku.id} ${formatSkuDisplayLabel(sku, locale.value) || t('admin.orders.itemSkuDefaultCode')}`

// 单规格商品只在券已限定规格时才需要展开去勾它唯一的规格，否则每个已选商品都会多出一块
const showScopeSkus = (product: AdminProduct) => {
  const count = product?.skus?.length || 0
  return scopeProductChecked(product.id) && (count > 1 || (count === 1 && selectedSkuIDs.value.length > 0))
}

// 已选规格里不属于任何「已选且可见」商品的 ID：可能是商品删掉了该规格，
// 或搜索后商品变成了没有 skus 的占位项。界面必须能取消它们，否则券无法保存。
const orphanScopeSkuIDs = computed(() => {
  const visible = new Set<number>()
  productOptions.value.forEach((product) => {
    const productID = Math.floor(Number(product?.id || 0))
    if (!selectedScopeIDs.value.includes(productID)) return
    ;(product?.skus || []).forEach((sku) => visible.add(Math.floor(Number(sku.id))))
  })
  return selectedSkuIDs.value.filter((id) => !visible.has(id))
})

// 传了规格就必须保证每个所选商品都被覆盖，否则后端会返回 ErrScopeInvalid。
const uncoveredScopeProductIDs = computed(() => {
  if (!selectedSkuIDs.value.length) return []
  const covered = new Set<number>()
  productOptions.value.forEach((product) => {
    const productID = Math.floor(Number(product?.id || 0))
    if (!selectedScopeIDs.value.includes(productID)) return
    ;(product?.skus || []).forEach((sku) => {
      if (selectedSkuIDs.value.includes(Math.floor(Number(sku.id)))) covered.add(productID)
    })
  })
  // 规格 ID 已失联（商品删了该规格 / 从未加载过）时，无法判断它覆盖哪个商品，
  // 交给后端判定，避免这里误报。
  const known = new Set<number>()
  productOptions.value.forEach((product) => {
    ;(product?.skus || []).forEach((sku) => known.add(Math.floor(Number(sku.id))))
  })
  if (selectedSkuIDs.value.some((id) => !known.has(id))) return []
  return selectedScopeIDs.value.filter((id) => !covered.has(id))
})

const removeScopeSku = (rawSkuID: number | string) => {
  const skuID = Math.floor(Number(rawSkuID))
  if (!Number.isFinite(skuID)) return
  selectedSkuIDs.value = selectedSkuIDs.value.filter((id) => id !== skuID)
}

const scopeProductChecked = (rawProductID: number | string) => {
  const productID = Number(rawProductID)
  if (!Number.isFinite(productID) || productID <= 0) return false
  return selectedScopeIDs.value.includes(Math.floor(productID))
}

const selectAllScopeProducts = () => {
  const ids = productOptions.value
    .map((item) => Number(item?.id || 0))
    .filter((id: number) => Number.isFinite(id) && id > 0)
    .map((id: number) => Math.floor(id))
  selectedScopeIDs.value = Array.from(new Set([...selectedScopeIDs.value, ...ids])).sort((a, b) => a - b)
}

const clearScopeProducts = () => {
  selectedScopeIDs.value = []
  selectedSkuIDs.value = []
}

const resolveProductNameByID = (rawProductID: number | string) => {
  const productID = Number(rawProductID)
  if (!Number.isFinite(productID) || productID <= 0) return ''
  const target = productOptions.value.find((item) => Number(item?.id || 0) === Math.floor(productID))
  if (!target) return ''
  return getLocalizedText(target?.title || {})
}

const fetchCoupons = async (page = 1, options: ListFetchOptions = {}) => {
  if (!options.preserveRows) loading.value = true
  try {
    const normalizedIsActive = normalizeFilterValue(filters.isActive)
    const normalizedScopeRefID = normalizeScopeFilterValue(filters.scopeRefId)
    const isActiveValue = normalizedIsActive === '' ? undefined : normalizedIsActive === 'true'
    const response = await adminAPI.getCoupons({
      page,
      page_size: pagination.value.page_size,
      id: filters.id || undefined,
      code: filters.code || undefined,
      scope_ref_id: normalizedScopeRefID || undefined,
      is_active: isActiveValue,
    })
    coupons.value = response.data.data || []
    pagination.value = response.data.pagination || pagination.value
    if (autoOpenId.value) {
      const target = coupons.value.find((item) => item.id === autoOpenId.value)
      if (target) {
        openEditModal(target)
      }
      autoOpenId.value = null
    }
  } catch {
    if (!options.preserveRows) coupons.value = []
  } finally {
    if (!options.preserveRows) loading.value = false
  }
}

const handleSearch = () => {
  fetchCoupons(1)
}
const debouncedSearch = useDebounceFn(handleSearch, 300)

const handleScopeFilterChange = () => {
  fetchCoupons(1)
}

const refresh = () => {
  refreshList(() => fetchCoupons(pagination.value.page, { preserveRows: true }))
}

const changePage = (page: number) => {
  if (page < 1 || page > pagination.value.total_page) return
  fetchCoupons(page)
}

const pageSizeOptions = [10, 20, 50, 100]

const changePageSize = (size: number) => {
  if (size === pagination.value.page_size) return
  pagination.value.page_size = size
  fetchCoupons(1)
}

const openCreateModal = () => {
  error.value = ''
  clearErrors()
  resetForm()
  showModal.value = true
  void loadProductOptions()
}

const openEditModal = (coupon: AdminCoupon) => {
  error.value = ''
  isEditing.value = true
  editingId.value = coupon.id
  form.code = coupon.code || ''
  form.type = coupon.type || 'percent'
  form.value = coupon.value || 0
  selectedScopeIDs.value = normalizeScopeIDs(coupon.scope_ref_ids)
  selectedSkuIDs.value = normalizeScopeIDs(coupon.scope_sku_ids)
  form.min_amount = coupon.min_amount || 0
  form.max_discount = coupon.max_discount || 0
  form.usage_limit = coupon.usage_limit || 0
  form.per_user_limit = coupon.per_user_limit || 0
  form.disabled_wholesale_price = Boolean(coupon.disabled_wholesale_price)
  form.per_item_discount = form.type === 'fixed' && Boolean(coupon.per_item_discount)
  form.payment_roles = normalizePaymentRoles(coupon.payment_roles)
  form.member_levels = normalizeMemberLevels(coupon.member_levels)
  form.starts_at = toLocalInput(coupon.starts_at)
  form.ends_at = toLocalInput(coupon.ends_at)
  form.is_active = Boolean(coupon.is_active)
  showModal.value = true
  void loadProductOptions()
}

const closeModal = () => {
  showModal.value = false
  isEditing.value = false
  editingId.value = null
  clearErrors()
}

const handleSubmit = async () => {
  error.value = ''
  if (!validate({ ...form } as Record<string, unknown>)) return
  const scopeIDs = normalizeScopeIDs(selectedScopeIDs.value)
  if (!scopeIDs.length) {
    error.value = t('admin.coupons.errors.scopeRequired')
    return
  }
  if (uncoveredScopeProductIDs.value.length) {
    error.value = t('admin.coupons.errors.scopeSkuUncovered', {
      ids: uncoveredScopeProductIDs.value.map((id) => `#${id}`).join(', '),
    })
    return
  }
  submitting.value = true
  try {
    const payload = {
      code: form.code.trim(),
      type: form.type,
      value: Number(form.value),
      scope_ref_ids: scopeIDs,
      scope_sku_ids: normalizeScopeIDs(selectedSkuIDs.value),
      min_amount: Number(form.min_amount || 0),
      max_discount: Number(form.max_discount || 0),
      usage_limit: Number(form.usage_limit || 0),
      per_user_limit: Number(form.per_user_limit || 0),
      disabled_wholesale_price: Boolean(form.disabled_wholesale_price),
      per_item_discount: form.type === 'fixed' ? Boolean(form.per_item_discount) : false,
      payment_roles: normalizePaymentRoles(form.payment_roles),
      member_levels: normalizeMemberLevels(form.member_levels),
      starts_at: form.starts_at ? toISO(form.starts_at) : '',
      ends_at: form.ends_at ? toISO(form.ends_at) : '',
      is_active: form.is_active,
    }
    if (isEditing.value && editingId.value) {
      await adminAPI.updateCoupon(editingId.value, payload)
    } else {
      await adminAPI.createCoupon(payload)
    }
    closeModal()
    fetchCoupons(1)
  } catch (err: any) {
    error.value =
      err.message ||
      (isEditing.value ? t('admin.coupons.errors.updateFailed') : t('admin.coupons.errors.createFailed'))
  } finally {
    submitting.value = false
  }
}

const handleDelete = async (coupon: AdminCoupon) => {
  const confirmed = await confirmAction({ description: t('admin.coupons.confirmDelete', { code: coupon.code }), confirmText: t('admin.common.delete'), variant: 'destructive' })
  if (!confirmed) return
  try {
    await adminAPI.deleteCoupon(coupon.id)
    fetchCoupons(pagination.value.page)
  } catch (err: any) {
    const message = err.message || t('admin.coupons.errors.deleteFailed')
    notifyError(message)
  }
}

const formatScope = (scope?: unknown) => {
  const ids = normalizeScopeIDs(scope)
  if (!ids.length) return '-'
  return ids
    .map((id) => {
      const productName = resolveProductNameByID(id)
      if (productName) return `#${id} ${productName}`
      return `#${id}`
    })
    .join(', ')
}

const formatScopeSKUs = (raw?: unknown) => {
  const ids = normalizeScopeIDs(raw)
  if (!ids.length) return ''
  return ids.map((id) => `#${id}`).join(', ')
}

const formatPaymentRoles = (raw: unknown) => {
  const roles = normalizePaymentRoles(raw)
  if (!roles.length) return '-'
  const labels: Record<string, string> = {
    guest: t('admin.coupons.paymentRoles.guest'),
    member: t('admin.coupons.paymentRoles.member'),
  }
  return roles.map((role) => labels[role] || role).join(', ')
}

const formatMemberLevels = (raw: unknown) => {
  const levelIDs = normalizeMemberLevels(raw)
  if (!levelIDs.length) return '-'
  return levelIDs
    .map((levelID) => {
      const target = memberLevels.value.find((item) => Number(item.id) === levelID)
      if (!target) return `#${levelID}`
      return getLocalizedText(target.name) || `#${levelID}`
    })
    .join(', ')
}

onMounted(async () => {
  applyRouteFilter()
  await Promise.all([loadProductOptions(), loadMemberLevels()])
  fetchCoupons()
})

watch(
  () => route.query.id,
  () => {
    applyRouteFilter()
    fetchCoupons(1)
  }
)

watch(
  () => route.query.coupon_id,
  () => {
    applyRouteFilter()
    fetchCoupons(1)
  }
)

watch(
  () => form.type,
  (type) => {
    if (type !== 'fixed') {
      form.per_item_discount = false
    }
  }
)

watch(
  () => route.query.code,
  () => {
    applyRouteFilter()
    fetchCoupons(1)
  }
)

watch(
  () => route.query.scope_ref_id,
  () => {
    applyRouteFilter()
    fetchCoupons(1)
  }
)
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <h1 class="text-2xl font-semibold">{{ t('admin.coupons.title') }}</h1>
      <Button size="sm" class="w-full gap-2 sm:w-auto" @click="openCreateModal">
        <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
        </svg>
        {{ t('admin.coupons.create') }}
      </Button>
    </div>

    <div class="rounded-xl border border-border bg-card p-4 shadow-sm">
      <div class="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center">
        <div class="w-full md:w-48">
          <Input v-model="filters.code" :placeholder="t('admin.coupons.filterCode')" @update:modelValue="debouncedSearch" />
        </div>
        <div class="flex w-full flex-col gap-2 md:w-72 sm:flex-row sm:items-center">
          <Input
            v-model="scopeFilterKeyword"
            :placeholder="t('admin.coupons.filterScopeSearchPlaceholder')"
            @keyup.enter="handleSearchScopeProducts"
          />
          <Button
            size="sm"
            class="w-full sm:w-auto"
            variant="outline"
            :disabled="productOptionsLoading"
            @click="handleSearchScopeProducts"
          >
            {{ productOptionsLoading ? t('admin.common.loading') : t('admin.coupons.filterScopeSearchAction') }}
          </Button>
        </div>
        <div class="w-full md:w-60">
          <Select v-model="filters.scopeRefId" @update:modelValue="handleScopeFilterChange">
            <SelectTrigger class="h-9 w-full">
              <SelectValue :placeholder="t('admin.coupons.filterScope')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="__all__">{{ t('admin.coupons.filterScopeAll') }}</SelectItem>
              <SelectItem
                v-for="product in productOptions"
                :key="`coupon-filter-product-${product.id}`"
                :value="String(product.id)"
              >
                {{ buildProductLabel(product) }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="w-full md:w-48">
          <Select v-model="filters.isActive" @update:modelValue="handleSearch">
            <SelectTrigger class="h-9 w-full">
            <SelectValue :placeholder="t('admin.coupons.filterStatusAll')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="__all__">{{ t('admin.coupons.filterStatusAll') }}</SelectItem>
              <SelectItem value="true">{{ t('admin.common.enabled') }}</SelectItem>
              <SelectItem value="false">{{ t('admin.common.disabled') }}</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="hidden flex-1 sm:block"></div>
        <Button size="sm" class="w-full sm:w-auto" :disabled="refreshing" @click="refresh">{{ t('admin.common.refresh') }}</Button>
      </div>
    </div>

    <div class="rounded-xl border border-border bg-card overflow-x-auto">
      <Table class="min-w-[920px]">
        <TableHeader class="border-b border-border bg-muted/40 text-xs uppercase text-muted-foreground">
          <TableRow>
            <TableHead class="px-6 py-3">{{ t('admin.coupons.table.id') }}</TableHead>
            <TableHead class="min-w-[90px] px-6 py-3">{{ t('admin.coupons.table.code') }}</TableHead>
            <TableHead class="min-w-[90px] px-6 py-3">{{ t('admin.coupons.table.type') }}</TableHead>
            <TableHead class="min-w-[90px] px-6 py-3">{{ t('admin.coupons.table.value') }}</TableHead>
            <TableHead class="min-w-[90px] px-6 py-3">{{ t('admin.coupons.table.scope') }}</TableHead>
            <TableHead class="min-w-[90px] px-6 py-3">{{ t('admin.coupons.table.limits') }}</TableHead>
            <TableHead class="min-w-[90px] px-6 py-3">{{ t('admin.coupons.table.period') }}</TableHead>
            <TableHead class="min-w-[90px] px-6 py-3">{{ t('admin.coupons.table.status') }}</TableHead>
            <TableHead class="min-w-[90px] px-6 py-3 text-right">{{ t('admin.coupons.table.action') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody class="divide-y divide-border">
          <TableRow v-if="loading">
            <TableCell :colspan="9" class="p-0">
              <TableSkeleton :columns="9" :rows="5" />
            </TableCell>
          </TableRow>
          <TableRow v-else-if="coupons.length === 0">
            <TableCell colspan="9" class="px-6 py-8 text-center text-muted-foreground">{{ t('admin.coupons.empty') }}</TableCell>
          </TableRow>
          <TableRow v-for="coupon in coupons" :key="coupon.id" class="hover:bg-muted/30">
            <TableCell class="px-6 py-4">
              <IdCell :value="coupon.id" />
            </TableCell>
            <TableCell class="min-w-[90px] px-6 py-4">
              <div class="break-all font-medium text-foreground">{{ coupon.code }}</div>
            </TableCell>
            <TableCell class="min-w-[90px] px-6 py-4 text-xs text-muted-foreground">{{ discountTypeLabel(coupon.type) }}</TableCell>
            <TableCell class="min-w-[90px] px-6 py-4 text-foreground font-mono">{{ coupon.value }}</TableCell>
            <TableCell class="min-w-[90px] px-6 py-4 text-xs text-muted-foreground">
              <div class="break-words">{{ formatScope(coupon.scope_ref_ids) }}</div>
              <div v-if="formatScopeSKUs(coupon.scope_sku_ids)" class="break-words">
                {{ t('admin.coupons.modal.scopeSkuList') }}：{{ formatScopeSKUs(coupon.scope_sku_ids) }}
              </div>
            </TableCell>
            <TableCell class="min-w-[90px] px-6 py-4 text-xs text-muted-foreground">
              <div class="break-words">{{ t('admin.coupons.limit.minAmount') }}：{{ coupon.min_amount || '-' }}</div>
              <div class="break-words">{{ t('admin.coupons.limit.maxDiscount') }}：{{ coupon.max_discount || '-' }}</div>
              <div class="break-words">{{ t('admin.coupons.limit.usageLimit') }}：{{ coupon.usage_limit || '-' }}</div>
              <div class="break-words">{{ t('admin.coupons.limit.perUserLimit') }}：{{ coupon.per_user_limit || '-' }}</div>
              <div class="break-words">{{ t('admin.coupons.limit.paymentRoles') }}：{{ formatPaymentRoles(coupon.payment_roles) }}</div>
              <div class="break-words">{{ t('admin.coupons.limit.memberLevels') }}：{{ formatMemberLevels(coupon.member_levels) }}</div>
            </TableCell>
            <TableCell class="min-w-[90px] px-6 py-4 text-xs text-muted-foreground">
              <div class="break-words">{{ t('admin.coupons.period.startsAt') }}：{{ formatDate(coupon.starts_at) || '-' }}</div>
              <div class="break-words">{{ t('admin.coupons.period.endsAt') }}：{{ formatDate(coupon.ends_at) || '-' }}</div>
            </TableCell>
            <TableCell class="min-w-[90px] px-6 py-4">
              <span
                class="inline-flex rounded-full border px-2.5 py-1 text-xs"
                :class="coupon.is_active ? 'text-emerald-700 border-emerald-200 bg-emerald-50' : 'text-muted-foreground border-border bg-muted/30'"
              >
                {{ coupon.is_active ? t('admin.common.enabled') : t('admin.common.disabled') }}
              </span>
            </TableCell>
            <TableCell class="min-w-[90px] px-6 py-4 text-right">
              <div class="flex items-center justify-end gap-2">
                <Button size="sm" variant="outline" @click="openEditModal(coupon)">{{ t('admin.coupons.actions.edit') }}</Button>
                <Button size="sm" variant="destructive" @click="handleDelete(coupon)">{{ t('admin.coupons.actions.delete') }}</Button>
              </div>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>

      <ListPagination
        :page="pagination.page"
        :total-page="pagination.total_page"
        :total="pagination.total"
        :page-size="pagination.page_size"
        :page-size-options="pageSizeOptions"
        @change-page="changePage"
        @change-page-size="changePageSize"
      />
    </div>

    <Dialog v-model:open="showModal" @update:open="(value) => { if (!value) closeModal() }">
      <DialogScrollContent class="w-[calc(100vw-1rem)] max-w-3xl p-4 sm:p-6" @interact-outside="(e) => e.preventDefault()">
        <DialogHeader>
          <DialogTitle>{{ isEditing ? t('admin.coupons.modal.editTitle') : t('admin.coupons.modal.title') }}</DialogTitle>
        </DialogHeader>
        <form class="space-y-4" @submit.prevent="handleSubmit">
          <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
            <div>
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('admin.coupons.modal.code') }} *</label>
              <Input v-model="form.code" required placeholder="PROMO2026" />
              <p v-if="errors.code" class="text-xs text-destructive mt-1">{{ errors.code }}</p>
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('admin.coupons.modal.type') }} *</label>
              <Select v-model="form.type">
                <SelectTrigger class="h-9 w-full">
                  <SelectValue :placeholder="t('admin.common.discountTypes.percent')" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="percent">{{ t('admin.common.discountTypes.percent') }}</SelectItem>
                  <SelectItem value="fixed">{{ t('admin.common.discountTypes.fixed') }}</SelectItem>
                </SelectContent>
              </Select>
              <p v-if="errors.type" class="text-xs text-destructive mt-1">{{ errors.type }}</p>
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('admin.coupons.modal.value') }} *</label>
              <Input v-model.number="form.value" type="number" step="0.01" required placeholder="20" />
              <p v-if="errors.value" class="text-xs text-destructive mt-1">{{ errors.value }}</p>
              <div v-if="form.type === 'fixed'" class="mt-3 flex items-center gap-2">
                <Switch v-model="form.per_item_discount" />
                <span class="text-xs text-muted-foreground">{{ t('admin.coupons.modal.perItemDiscount') }}</span>
              </div>
            </div>
            <div class="md:col-span-2">
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('admin.coupons.modal.scope') }} *</label>
              <div class="space-y-2">
                <div class="flex flex-col gap-2 sm:flex-row sm:items-center">
                  <Input
                    v-model="productKeyword"
                    :placeholder="t('admin.coupons.modal.scopeSearchPlaceholder')"
                    @keyup.enter="handleSearchProducts"
                  />
                  <Button
                    class="w-full sm:w-auto"
                    type="button"
                    size="sm"
                    variant="outline"
                    :disabled="productOptionsLoading"
                    @click="handleSearchProducts"
                  >
                    {{ productOptionsLoading ? t('admin.common.loading') : t('admin.coupons.modal.scopeSearchAction') }}
                  </Button>
                </div>
                <div class="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                  <span>{{ t('admin.coupons.modal.scopeSelectedCount', { count: selectedScopeIDs.length }) }}</span>
                  <Button type="button" size="sm" variant="ghost" class="h-7 px-2" @click="selectAllScopeProducts">
                    {{ t('admin.coupons.modal.scopeSelectAll') }}
                  </Button>
                  <Button type="button" size="sm" variant="ghost" class="h-7 px-2" @click="clearScopeProducts">
                    {{ t('admin.coupons.modal.scopeClear') }}
                  </Button>
                </div>
                <p v-if="productOptions.some(showScopeSkus)" class="text-xs text-muted-foreground">
                  {{ t('admin.coupons.modal.scopeSkuHint') }}
                </p>
                <div class="max-h-56 overflow-auto rounded-lg border border-border bg-background">
                  <div v-if="productOptionsLoading" class="px-3 py-3 text-xs text-muted-foreground">
                    {{ t('admin.common.loading') }}
                  </div>
                  <div v-else-if="productOptions.length === 0" class="px-3 py-3 text-xs text-muted-foreground">
                    {{ t('admin.coupons.modal.scopeEmpty') }}
                  </div>
                  <div
                    v-for="product in productOptions"
                    :key="`scope-product-${product.id}`"
                    class="border-b border-border/60 last:border-b-0"
                  >
                  <Label
                    class="flex cursor-pointer items-center gap-2 px-3 py-2 text-sm hover:bg-muted/30"
                  >
                    <Checkbox
                      :model-value="scopeProductChecked(product.id)"
                      @update:model-value="(v) => toggleScopeProduct(product.id, v)"
                    />
                    <span class="truncate">{{ buildProductLabel(product) }}</span>
                  </Label>
                  <div
                    v-if="showScopeSkus(product)"
                    class="space-y-1 bg-muted/20 py-2 pl-9 pr-3"
                  >
                    <Label
                      v-for="sku in product.skus"
                      :key="`scope-sku-${sku.id}`"
                      class="flex cursor-pointer items-center gap-2 text-xs"
                    >
                      <Checkbox
                        :model-value="scopeSkuChecked(sku.id)"
                        @update:model-value="(v) => toggleScopeSku(sku.id, v)"
                      />
                      <span class="truncate">{{ buildSkuLabel(sku) }}</span>
                    </Label>
                  </div>
                  </div>
                </div>
                <div
                  v-if="orphanScopeSkuIDs.length"
                  class="flex flex-wrap items-center gap-2 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800"
                >
                  <span>{{ t('admin.coupons.modal.scopeSkuOrphan') }}：</span>
                  <span
                    v-for="skuID in orphanScopeSkuIDs"
                    :key="`scope-orphan-sku-${skuID}`"
                    class="inline-flex items-center gap-1 rounded-full border border-amber-300 bg-background px-2 py-0.5"
                  >
                    #{{ skuID }}
                    <button
                      type="button"
                      class="text-amber-700 hover:text-amber-900"
                      :title="t('admin.coupons.modal.scopeSkuRemove')"
                      :aria-label="t('admin.coupons.modal.scopeSkuRemove')"
                      @click="removeScopeSku(skuID)"
                    >✕</button>
                  </span>
                </div>
              </div>
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('admin.coupons.modal.minAmount') }}</label>
              <Input v-model.number="form.min_amount" type="number" step="0.01" placeholder="0" />
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('admin.coupons.modal.maxDiscount') }}</label>
              <Input v-model.number="form.max_discount" type="number" step="0.01" placeholder="0" />
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('admin.coupons.modal.usageLimit') }}</label>
              <Input v-model.number="form.usage_limit" type="number" placeholder="0" />
              <div class="mt-3 flex items-center gap-2">
                <Switch v-model="form.disabled_wholesale_price" />
                <span class="text-xs text-muted-foreground">{{ t('admin.coupons.modal.disabledWholesalePrice') }}</span>
              </div>
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('admin.coupons.modal.perUserLimit') }}</label>
              <Input v-model.number="form.per_user_limit" type="number" placeholder="0" />
            </div>
            <div class="md:col-span-2">
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('admin.coupons.modal.paymentRoles') }}</label>
              <MultiSelect
                v-model="form.payment_roles"
                :options="paymentRoleOptions"
                :placeholder="t('admin.coupons.modal.paymentRolesPlaceholder')"
              />
            </div>
            <div class="md:col-span-2">
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('admin.coupons.modal.memberLevels') }}</label>
              <MultiSelect
                v-model="form.member_levels"
                :options="memberLevelOptions"
                :placeholder="t('admin.coupons.modal.memberLevelsPlaceholder')"
                :disabled="memberLevelOptions.length === 0"
              />
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('admin.coupons.modal.startsAt') }}</label>
              <Input v-model="form.starts_at" type="datetime-local" />
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('admin.coupons.modal.endsAt') }}</label>
              <Input v-model="form.ends_at" type="datetime-local" />
            </div>
            <div class="flex flex-col gap-2 md:col-span-2 sm:flex-row sm:items-center">
              <Switch v-model="form.is_active" />
              <span class="text-xs text-muted-foreground">{{ t('admin.common.enabled') }}</span>
            </div>
          </div>

          <div v-if="error" class="rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">
            {{ error }}
          </div>

          <div class="flex flex-col-reverse gap-3 sm:flex-row sm:justify-end">
            <Button class="w-full sm:w-auto" type="button" variant="outline" @click="closeModal">{{ t('admin.common.cancel') }}</Button>
            <Button class="w-full sm:w-auto" type="submit" :disabled="submitting">{{ t('admin.common.save') }}</Button>
          </div>
        </form>
      </DialogScrollContent>
    </Dialog>
  </div>
</template>
