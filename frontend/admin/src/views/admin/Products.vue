<script setup lang="ts">
import { ref, reactive, computed, onMounted, watch, nextTick } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AdminProduct, AdminCategory, AdminProductSKU } from '@/api/types'
import IdCell from '@/components/IdCell.vue'
import { getFirstImageUrl } from '@/utils/image'
import { formatMoney, getLocalizedText } from '@/utils/format'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { notifyError, notifySuccess } from '@/utils/notify'
import TableSkeleton from '@/components/TableSkeleton.vue'
import ListPagination from '@/components/ListPagination.vue'
import type { ListFetchOptions } from '@/composables/useListRefresh'
import { confirmAction } from '@/utils/confirm'
import ProductEditModal from './components/ProductEditModal.vue'
import { buildAdminCategoryPath, createAdminCategoryMap, createAdminCategoryChildCountMap, flattenAdminCategories, isAdminProductCategorySelectable } from '@/utils/category'
import { formatWholesaleTierScopeLabel, wholesaleTierScopeValue } from '@/utils/wholesalePricing'

const { t, locale } = useI18n()
const loading = ref(false)
const searchQuery = ref('')
const stockStatus = ref('all')
const statusFilter = ref('all')
const wholesaleStatus = ref('all')
const categoryFilter = ref('all')
const route = useRoute()
const router = useRouter()

const showModal = ref(false)
const editingProductId = ref<number | null>(null)
const editingSortId = ref<number | null>(null)
const editingSortValue = ref('')
const editingCategoryId = ref<number | null>(null)

const products = ref<AdminProduct[]>([])
const categories = ref<AdminCategory[]>([])
const categoryMap = ref(new Map<number, AdminCategory>())
const categoryChildCountMap = ref(new Map<number, number>())
const orderedCategories = ref<AdminCategory[]>([])
const pagination = reactive({
  page: 1,
  page_size: 10,
  total: 0,
  total_page: 0,
})

// --- Batch ---
const selectedIds = ref<Set<number>>(new Set())
const batchOperating = ref(false)
const batchCategoryId = ref('')

const allSelected = computed(() => products.value.length > 0 && products.value.every((p) => selectedIds.value.has(p.id)))

const toggleSelectAll = () => {
  selectedIds.value = allSelected.value ? new Set() : new Set(products.value.map((p) => p.id))
}

const toggleSelect = (id: number) => {
  const next = new Set(selectedIds.value)
  next.has(id) ? next.delete(id) : next.add(id)
  selectedIds.value = next
}

const handleBatchStatus = async (isActive: boolean) => {
  const ids = Array.from(selectedIds.value)
  if (!ids.length) return
  batchOperating.value = true
  try {
    const res = await adminAPI.batchUpdateProductStatus(ids, isActive)
    const data = res.data.data as { success_count?: number } | null
    notifySuccess(t('admin.products.batch.statusResult', { success: data?.success_count || 0, total: ids.length }))
    selectedIds.value = new Set()
    fetchProducts()
  } catch (err: any) { notifyError(err?.response?.data?.message || err?.message) } finally { batchOperating.value = false }
}

const handleBatchCategory = async () => {
  const ids = Array.from(selectedIds.value)
  if (!ids.length || !batchCategoryId.value) return
  batchOperating.value = true
  try {
    const res = await adminAPI.batchUpdateProductCategory(ids, Number(batchCategoryId.value))
    const data = res.data.data as { success_count?: number } | null
    notifySuccess(t('admin.products.batch.categoryResult', { success: data?.success_count || 0, total: ids.length }))
    selectedIds.value = new Set()
    batchCategoryId.value = ''
    fetchProducts()
  } catch (err: any) { notifyError(err?.response?.data?.message || err?.message) } finally { batchOperating.value = false }
}

const handleBatchDelete = async () => {
  const ids = Array.from(selectedIds.value)
  if (!ids.length) return
  const confirmed = await confirmAction({ description: t('admin.products.batch.deleteConfirm', { count: ids.length }), confirmText: t('admin.common.delete'), variant: 'destructive' })
  if (!confirmed) return
  batchOperating.value = true
  try {
    const res = await adminAPI.batchDeleteProducts(ids)
    const data = res.data.data as { success_count?: number } | null
    notifySuccess(t('admin.products.batch.deleteResult', { success: data?.success_count || 0, total: ids.length }))
    selectedIds.value = new Set()
    fetchProducts()
  } catch (err: any) { notifyError(err?.response?.data?.message || err?.message) } finally { batchOperating.value = false }
}

const siteCurrency = ref('CNY')

const formatPrice = (amount: number | string, currency: string) => {
  return formatMoney(amount, currency)
}

const formatWholesaleSummary = (product: AdminProduct) => {
  const tiers = Array.isArray(product.wholesale_prices) ? product.wholesale_prices : []
  if (!tiers.length) return ''
  return tiers
    .slice()
    .sort((a, b) => {
      const scopeA = wholesaleTierScopeValue(a)
      const scopeB = wholesaleTierScopeValue(b)
      if (scopeA !== scopeB) return scopeA.localeCompare(scopeB)
      return Number(a.min_quantity || 0) - Number(b.min_quantity || 0)
    })
    .map((tier) => {
      const scope = formatWholesaleTierScopeLabel(
        product,
        tier,
        String(locale.value || 'zh-CN'),
        t('admin.wholesalePrices.modal.skuAll'),
      )
      return `${scope} ≥${Number(tier.min_quantity || 0)} ${formatPrice(tier.unit_price, siteCurrency.value)}`
    })
    .join(' / ')
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

const formatManualStockSummary = (product: AdminProduct) => {
  const metrics = resolveManualStockMetrics(product)
  const total = metrics.total
  if (total === -1) {
    return t('admin.products.stock.unlimited')
  }
  const locked = metrics.locked
  const sold = metrics.sold
  const remaining = Math.max(total, 0)
  return t('admin.products.stock.manualSummary', { remaining, locked, sold })
}

const formatAutoStockSummary = (product: AdminProduct) => {
  if (Number(product?.auto_stock_available) === -1) {
    return `${t('admin.products.stock.unlimited')} · ${t('admin.cardSecrets.isLoopLabel')}`
  }
  const total = toSafeInt(product?.auto_stock_total)
  const locked = toSafeInt(product?.auto_stock_locked)
  const sold = toSafeInt(product?.auto_stock_sold)
  const available = toSafeInt(product?.auto_stock_available)
  return t('admin.products.stock.summary', { total, locked, sold, available })
}

const purchaseTypeBadgeClass = (product: AdminProduct) => {
  return product?.purchase_type === 'guest'
    ? 'border-sky-200 bg-sky-50 text-sky-700'
    : 'border-indigo-200 bg-indigo-50 text-indigo-700'
}

const fulfillmentTypeBadgeClass = (product: AdminProduct) => {
  return product?.fulfillment_type === 'auto'
    ? 'border-violet-200 bg-violet-50 text-violet-700'
    : 'border-amber-200 bg-amber-50 text-amber-700'
}

const manualStockBadgeClass = (product: AdminProduct) => {
  const total = resolveManualStockMetrics(product).total
  if (total === -1) {
    return 'border-zinc-200 bg-zinc-50 text-zinc-700'
  }
  if (total <= 0) {
    return 'border-rose-200 bg-rose-50 text-rose-700'
  }
  return 'border-emerald-200 bg-emerald-50 text-emerald-700'
}

const autoStockBadgeClass = (product: AdminProduct) => {
  if (Number(product?.auto_stock_available) === -1) {
    return 'border-zinc-200 bg-zinc-50 text-zinc-700'
  }
  const available = toSafeInt(product?.auto_stock_available)
  if (available <= 0) {
    return 'border-rose-200 bg-rose-50 text-rose-700'
  }
  return 'border-emerald-200 bg-emerald-50 text-emerald-700'
}

const fetchProducts = async (options: ListFetchOptions = {}) => {
  if (!options.preserveRows) loading.value = true
  selectedIds.value = new Set()
  try {
    const res = await adminAPI.getProducts({
      page: pagination.page,
      page_size: pagination.page_size,
      search: searchQuery.value,
      stock_status: stockStatus.value,
      is_active: statusQueryValue(),
      wholesale: wholesaleStatus.value,
      category_id: categoryFilter.value === 'all' ? undefined : categoryFilter.value,
    })
    products.value = res.data.data || []
    if (res.data.pagination) {
      Object.assign(pagination, res.data.pagination)
    }
  } catch (err) {
    if (!options.preserveRows) products.value = []
  } finally {
    if (!options.preserveRows) loading.value = false
  }
}

const fetchCategories = async () => {
  try {
    const res = await adminAPI.getCategories({ type: 'product' })
    categories.value = res.data.data || []
    categoryMap.value = createAdminCategoryMap(categories.value)
    categoryChildCountMap.value = createAdminCategoryChildCountMap(categories.value)
    orderedCategories.value = flattenAdminCategories(categories.value).map((item) => item.category)
  } catch (err) {
    categories.value = []
    categoryMap.value = new Map()
    categoryChildCountMap.value = new Map()
    orderedCategories.value = []
  }
}

const getProductCategoryLabel = (category?: AdminCategory) => {
  if (!category) return ''
  return buildAdminCategoryPath(category, categoryMap.value, (item) => getLocalizedText(item.name))
}

const categoryOptions = computed(() => flattenAdminCategories(categories.value).map((item) => ({
  ...item,
  selectable: isAdminProductCategorySelectable(item.category, categoryChildCountMap.value),
})))

const fetchSiteCurrency = async () => {
  try {
    const response = await adminAPI.getSettings({ key: 'site_config' })
    const data = response.data?.data as Record<string, unknown> | undefined
    const raw = String(data?.currency || 'CNY').trim().toUpperCase()
    siteCurrency.value = /^[A-Z]{3}$/.test(raw) ? raw : 'CNY'
  } catch {
    siteCurrency.value = 'CNY'
  }
}

const handleSearch = () => {
  pagination.page = 1
  fetchProducts()
}
const debouncedSearch = useDebounceFn(handleSearch, 300)

// 上架状态三态：'all' 不下发参数，'active' → is_active=1，'inactive' → is_active=0
const statusQueryValue = () => {
  if (statusFilter.value === 'active') return '1'
  if (statusFilter.value === 'inactive') return '0'
  return undefined
}

const syncWholesaleStatusFromRoute = (value: unknown) => {
  const raw = Array.isArray(value) ? value[0] : value
  const normalized = String(raw ?? '').trim().toLowerCase()
  if (['1', 'true', 'yes', 'enabled'].includes(normalized)) {
    wholesaleStatus.value = 'enabled'
  } else if (['0', 'false', 'no', 'disabled'].includes(normalized)) {
    wholesaleStatus.value = 'disabled'
  } else {
    wholesaleStatus.value = 'all'
  }
}

const wholesaleQueryValue = () => {
  if (wholesaleStatus.value === 'enabled') return '1'
  if (wholesaleStatus.value === 'disabled') return '0'
  return undefined
}

const handleWholesaleStatusChange = () => {
  pagination.page = 1
  const current = Array.isArray(route.query.wholesale) ? route.query.wholesale[0] : route.query.wholesale
  const nextWholesale = wholesaleQueryValue()
  if ((current || undefined) !== nextWholesale) {
    router.replace({ query: { ...route.query, wholesale: nextWholesale } })
    return
  }
  fetchProducts()
}

const resetFilters = () => {
  searchQuery.value = ''
  stockStatus.value = 'all'
  statusFilter.value = 'all'
  wholesaleStatus.value = 'all'
  categoryFilter.value = 'all'
  pagination.page = 1
  if (route.query.wholesale !== undefined) {
    router.replace({ query: { ...route.query, wholesale: undefined } })
  } else {
    fetchProducts({ preserveRows: true })
  }
  nextTick(() => {
    const input = document.getElementById('admin-products-search') as HTMLInputElement | null
    input?.focus()
  })
}

const changePage = (page: number) => {
  pagination.page = page
  fetchProducts()
}

const pageSizeOptions = [10, 20, 50, 100]

const changePageSize = (size: number) => {
  if (size === pagination.page_size) return
  pagination.page_size = size
  pagination.page = 1
  fetchProducts()
}

const openCreateModal = () => {
  editingProductId.value = null
  showModal.value = true
}

const openEditById = (rawId: unknown) => {
  const id = Number(rawId)
  if (!Number.isFinite(id) || id <= 0) return
  editingProductId.value = id
  showModal.value = true
}

const handleModalSuccess = () => {
  fetchProducts()
}

const handleDelete = async (product: AdminProduct) => {
  const confirmed = await confirmAction({ description: t('admin.products.confirmDelete', { name: getLocalizedText(product.title) }), confirmText: t('admin.common.delete'), variant: 'destructive' })
  if (!confirmed) return
  try {
    await adminAPI.deleteProduct(product.id)
    fetchProducts()
  } catch (err: any) {
    if (isNotifiedError(err)) return
    notifyError(t('admin.products.errors.deleteFailed', { message: err?.message || '' }))
  }
}

const toggleStatus = async (product: AdminProduct) => {
  const newStatus = !product.is_active
  try {
    product.is_active = newStatus
    await adminAPI.patchProduct(product.id, { is_active: newStatus })
    // 上架状态筛选生效时，该行已不满足筛选条件，需要重新拉取以免列表与筛选结果不符
    if (statusFilter.value !== 'all') fetchProducts({ preserveRows: true })
  } catch (err: any) {
    product.is_active = !newStatus
    if (isNotifiedError(err)) return
    notifyError(t('admin.products.errors.updateFailed', { message: err?.message || '' }))
  }
}

const startEditSort = (product: AdminProduct) => {
  editingSortId.value = product.id
  editingSortValue.value = String(product.sort_order || 0)
  nextTick(() => {
    const input = document.getElementById(`sort-input-${product.id}`) as HTMLInputElement | null
    input?.focus()
    input?.select()
  })
}

const saveSort = async (product: AdminProduct) => {
  const newValue = Math.max(Math.floor(Number(editingSortValue.value) || 0), 0)
  editingSortId.value = null
  if (newValue === (product.sort_order || 0)) return
  const oldValue = product.sort_order || 0
  try {
    product.sort_order = newValue
    await adminAPI.patchProduct(product.id, { sort_order: newValue })
  } catch (err: any) {
    product.sort_order = oldValue
    if (isNotifiedError(err)) return
    notifyError(t('admin.products.errors.updateFailed', { message: err?.message || '' }))
  }
}

const cancelEditSort = () => {
  editingSortId.value = null
}

const startEditCategory = (product: AdminProduct) => {
  editingCategoryId.value = product.id
}

const saveCategory = async (product: AdminProduct, newCategoryId: unknown) => {
  editingCategoryId.value = null
  const numId = Number(newCategoryId)
  if (!numId || numId === product.category_id) return
  const oldCategoryId = product.category_id
  const oldCategory = product.category
  try {
    product.category_id = numId
    product.category = orderedCategories.value.find((c) => c.id === numId)
    await adminAPI.patchProduct(product.id, { category_id: numId })
  } catch (err: any) {
    product.category_id = oldCategoryId
    product.category = oldCategory
    if (isNotifiedError(err)) return
    notifyError(t('admin.products.errors.updateFailed', { message: err?.message || '' }))
  }
}

onMounted(() => {
  syncWholesaleStatusFromRoute(route.query.wholesale)
  fetchProducts()
  fetchCategories()
  fetchSiteCurrency()
  if (route.query.action === 'create') {
    openCreateModal()
    router.replace({ query: { ...route.query, action: undefined } })
  }
  if (route.query.product_id) {
    openEditById(route.query.product_id)
  }
})

watch(
  () => route.query.product_id,
  (value) => {
    if (value) {
      openEditById(value)
    }
  }
)

watch(
  () => route.query.wholesale,
  (value) => {
    syncWholesaleStatusFromRoute(value)
    pagination.page = 1
    fetchProducts()
  }
)
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <h1 class="text-2xl font-semibold">{{ t('admin.products.title') }}</h1>
      <Button class="w-full sm:w-auto" @click="openCreateModal">{{ t('admin.products.create') }}</Button>
    </div>

    <div class="rounded-xl border border-border bg-card p-4 shadow-sm">
      <div class="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center">
        <div class="relative w-full md:w-80">
          <div class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">
            <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
          </div>
          <Input
            id="admin-products-search"
            v-model="searchQuery"
            class="pl-9"
            :placeholder="t('admin.products.searchPlaceholder')"
            @update:modelValue="debouncedSearch"
            @keydown.enter="handleSearch"
          />
        </div>
        <div class="w-full md:w-56">
          <Select v-model="stockStatus" @update:modelValue="handleSearch">
            <SelectTrigger class="h-9 w-full">
              <SelectValue :placeholder="t('admin.products.filters.stockStatusPlaceholder')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{{ t('admin.products.filters.stockStatusAll') }}</SelectItem>
              <SelectItem value="low">{{ t('admin.products.stockStatus.low') }}</SelectItem>
              <SelectItem value="normal">{{ t('admin.products.stockStatus.normal') }}</SelectItem>
              <SelectItem value="unlimited">{{ t('admin.products.stockStatus.unlimited') }}</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="w-full md:w-56">
          <Select v-model="categoryFilter" @update:modelValue="handleSearch">
            <SelectTrigger class="h-9 w-full">
              <SelectValue :placeholder="t('admin.products.filters.categoryPlaceholder')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{{ t('admin.products.filters.categoryAll') }}</SelectItem>
              <SelectItem
                v-for="item in flattenAdminCategories(categories).map(i => ({ ...i, selectable: isAdminProductCategorySelectable(i.category, categoryChildCountMap) }))"
                :key="item.category.id"
                :value="String(item.category.id)"
                :disabled="!item.selectable"
                :class="item.depth > 0 ? 'pl-5' : ''"
              >
                {{ buildAdminCategoryPath(item.category, categoryMap, (c) => getLocalizedText(c.name)) }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="w-full md:w-48">
          <Select v-model="statusFilter" @update:modelValue="handleSearch">
            <SelectTrigger class="h-9 w-full">
              <SelectValue :placeholder="t('admin.products.filters.statusPlaceholder')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{{ t('admin.products.filters.statusAll') }}</SelectItem>
              <SelectItem value="active">{{ t('admin.products.filters.statusActive') }}</SelectItem>
              <SelectItem value="inactive">{{ t('admin.products.filters.statusInactive') }}</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="w-full md:w-48">
          <Select v-model="wholesaleStatus" @update:modelValue="handleWholesaleStatusChange">
            <SelectTrigger class="h-9 w-full">
              <SelectValue :placeholder="t('admin.products.filters.wholesalePlaceholder')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{{ t('admin.products.filters.wholesaleAll') }}</SelectItem>
              <SelectItem value="enabled">{{ t('admin.products.filters.wholesaleEnabled') }}</SelectItem>
              <SelectItem value="disabled">{{ t('admin.products.filters.wholesaleDisabled') }}</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <Button variant="outline" size="sm" class="h-9 w-full sm:w-auto" @click="resetFilters">
          {{ t('admin.common.reset') }}
        </Button>
      </div>
    </div>

    <!-- Batch action bar -->
    <div v-if="selectedIds.size > 0" class="flex flex-wrap items-center gap-3 rounded-xl border border-primary/20 bg-primary/5 px-4 py-3">
      <span class="text-sm font-medium">{{ t('admin.products.batch.selected', { count: selectedIds.size }) }}</span>
      <div class="flex flex-wrap items-center gap-2">
        <Button size="sm" variant="outline" :disabled="batchOperating" @click="handleBatchStatus(true)">{{ t('admin.products.batch.activate') }}</Button>
        <Button size="sm" variant="outline" :disabled="batchOperating" @click="handleBatchStatus(false)">{{ t('admin.products.batch.deactivate') }}</Button>
        <div class="flex items-center gap-1">
          <Select v-model="batchCategoryId">
            <SelectTrigger class="h-8 w-[160px] text-xs"><SelectValue :placeholder="t('admin.products.batch.categoryPlaceholder')" /></SelectTrigger>
            <SelectContent>
              <SelectItem
                v-for="item in flattenAdminCategories(categories).map(i => ({ ...i, selectable: isAdminProductCategorySelectable(i.category, categoryChildCountMap) }))"
                :key="item.category.id"
                :value="String(item.category.id)"
                :disabled="!item.selectable"
                :class="item.depth > 0 ? 'pl-5' : ''"
              >
                {{ buildAdminCategoryPath(item.category, categoryMap, (c) => getLocalizedText(c.name)) }}
              </SelectItem>
            </SelectContent>
          </Select>
          <Button size="sm" variant="outline" :disabled="batchOperating || !batchCategoryId" @click="handleBatchCategory">{{ t('admin.products.batch.moveCategory') }}</Button>
        </div>
        <Button size="sm" variant="destructive" :disabled="batchOperating" @click="handleBatchDelete">{{ t('admin.products.batch.delete') }}</Button>
      </div>
      <button class="ml-auto text-xs text-muted-foreground hover:text-foreground" @click="selectedIds = new Set()">{{ t('admin.products.batch.clearSelection') }}</button>
    </div>

    <div class="rounded-xl border border-border bg-card overflow-x-auto">
      <Table class="min-w-[920px]">
        <TableHeader class="border-b border-border bg-muted/40 text-xs uppercase text-muted-foreground">
          <TableRow>
            <TableHead class="w-10 px-3 py-3">
              <Checkbox :model-value="allSelected ? true : (selectedIds.size > 0 ? 'indeterminate' : false)" @update:model-value="toggleSelectAll" />
            </TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.products.table.id') }}</TableHead>
            <TableHead class="px-6 py-3 min-w-[320px]">{{ t('admin.products.table.name') }}</TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.products.table.price') }}</TableHead>
            <TableHead class="px-6 py-3 min-w-[220px]">{{ t('admin.products.table.category') }}</TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.products.table.sort') }}</TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.products.table.status') }}</TableHead>
            <TableHead class="px-6 py-3 text-right">{{ t('admin.products.table.action') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody class="divide-y divide-border">
          <TableRow v-if="loading">
            <TableCell :colspan="8" class="p-0">
              <TableSkeleton :columns="7" :rows="5" />
            </TableCell>
          </TableRow>
          <TableRow v-else-if="products.length === 0">
            <TableCell colspan="8" class="px-6 py-8 text-center text-muted-foreground">{{ t('admin.products.empty') }}</TableCell>
          </TableRow>
          <TableRow v-for="product in products" :key="product.id" class="hover:bg-muted/30">
            <TableCell class="w-10 px-3 py-4">
              <Checkbox :model-value="selectedIds.has(product.id)" @update:model-value="() => toggleSelect(product.id)" />
            </TableCell>
            <TableCell class="px-6 py-4">
              <IdCell :value="product.id" />
            </TableCell>
            <TableCell class="px-6 py-4">
              <div class="flex min-w-[320px] items-center gap-4">
                <div class="flex h-12 w-12 shrink-0 items-center justify-center overflow-hidden rounded-lg border border-border bg-muted/40 text-xs text-muted-foreground">
                  <img v-if="getFirstImageUrl(product.images)" :src="getFirstImageUrl(product.images)" class="h-full w-full object-cover" />
                  <span v-else>{{ t('admin.common.noImage') }}</span>
                </div>
                <div class="min-w-0 flex-1">
                  <div class="font-medium text-foreground break-words">{{ getLocalizedText(product.title) }}</div>
                  <div class="text-xs text-muted-foreground font-mono break-all">{{ product.slug }}</div>
                  <div class="mt-2 flex flex-wrap gap-2">
                    <span class="rounded-full border px-2 py-0.5 text-[11px]" :class="purchaseTypeBadgeClass(product)">
                      {{ product.purchase_type === 'guest' ? t('admin.products.purchaseType.guest') : t('admin.products.purchaseType.member') }}
                    </span>
                    <span class="rounded-full border px-2 py-0.5 text-[11px]" :class="fulfillmentTypeBadgeClass(product)">
                      {{ product.fulfillment_type === 'auto' ? t('admin.products.fulfillmentType.auto') : t('admin.products.fulfillmentType.manual') }}
                    </span>
                    <span
                      v-if="product.is_mapped"
                      class="rounded-full border border-indigo-200 bg-indigo-50 px-2 py-0.5 text-[11px] text-indigo-700"
                    >
                      {{ t('admin.products.mappedProduct') }}
                    </span>
                    <span
                      class="rounded-full border px-2 py-0.5 text-[11px]"
                      :class="product.is_affiliate_enabled ? 'border-sky-200 bg-sky-50 text-sky-700' : 'border-border bg-muted/30 text-muted-foreground'"
                    >
                      {{ product.is_affiliate_enabled ? t('admin.products.affiliate.enabled') : t('admin.products.affiliate.disabled') }}
                    </span>
                    <span
                      v-if="product.fulfillment_type === 'manual'"
                      class="rounded-full border px-2 py-0.5 text-[11px]"
                      :class="manualStockBadgeClass(product)"
                    >
                      {{ formatManualStockSummary(product) }}
                    </span>
                    <span
                      v-if="product.fulfillment_type === 'auto'"
                      class="rounded-full border px-2 py-0.5 text-[11px]"
                      :class="autoStockBadgeClass(product)"
                    >
                      {{ formatAutoStockSummary(product) }}
                    </span>
                    <span
                      v-for="(tag, index) in (product.tags || []).slice(0, 3)"
                      :key="index"
                      class="rounded-full border border-border bg-muted/30 px-2 py-0.5 text-[11px] text-muted-foreground"
                    >
                      # {{ tag }}
                    </span>
                  </div>
                </div>
              </div>
            </TableCell>
            <TableCell class="px-6 py-4">
              <div class="font-mono text-foreground">{{ formatPrice(product.price_amount, siteCurrency) }}</div>
              <div v-if="formatWholesaleSummary(product)" class="mt-1 text-xs text-emerald-600 dark:text-emerald-400">
                {{ formatWholesaleSummary(product) }}
              </div>
            </TableCell>
            <TableCell class="px-6 py-4 min-w-[220px]">
              <div v-if="editingCategoryId === product.id" class="min-w-[140px]">
                <Select
                  :modelValue="String(product.category_id || '')"
                  @update:modelValue="(val) => saveCategory(product, val)"
                >
                  <SelectTrigger class="h-7 w-full text-xs">
                    <SelectValue :placeholder="t('admin.products.uncategorized')" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem
                      v-for="item in categoryOptions"
                      :key="item.category.id"
                      :value="String(item.category.id)"
                      :disabled="!item.selectable"
                      :class="item.depth > 0 ? 'pl-6' : ''"
                    >
                      {{ item.depth > 0 ? getLocalizedText(item.category.name) : getProductCategoryLabel(item.category) }}
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <span
                v-else
                class="inline-flex max-w-full cursor-pointer rounded-full border border-border px-2 py-1 text-xs leading-5 text-muted-foreground transition-colors hover:border-primary hover:text-primary break-words"
                :title="t('admin.common.clickToEdit')"
                @click="startEditCategory(product)"
              >
                {{ product.category ? getProductCategoryLabel(product.category) : t('admin.products.uncategorized') }}
              </span>
            </TableCell>
            <TableCell class="px-6 py-4">
              <div v-if="editingSortId === product.id" class="flex items-center gap-1">
                <Input
                  :id="`sort-input-${product.id}`"
                  v-model="editingSortValue"
                  type="number"
                  min="0"
                  class="h-7 w-20 text-xs"
                  @keydown.enter="saveSort(product)"
                  @keydown.escape="cancelEditSort"
                  @blur="saveSort(product)"
                />
              </div>
              <span
                v-else
                class="cursor-pointer rounded-full border border-border px-2 py-1 text-xs text-muted-foreground transition-colors hover:border-primary hover:text-primary"
                :title="t('admin.common.clickToEdit')"
                @click="startEditSort(product)"
              >
                {{ product.sort_order || 0 }}
              </span>
            </TableCell>
            <TableCell class="px-6 py-4">
              <span
                class="inline-flex cursor-pointer rounded-full border px-2.5 py-1 text-xs transition-colors"
                :class="product.is_active ? 'text-emerald-700 border-emerald-200 bg-emerald-50 hover:bg-emerald-100' : 'text-muted-foreground border-border bg-muted/30 hover:bg-muted/50'"
                :title="product.is_active ? t('admin.products.status.clickToDeactivate') : t('admin.products.status.clickToActivate')"
                @click="toggleStatus(product)"
              >
                {{ product.is_active ? t('admin.products.status.active') : t('admin.products.status.inactive') }}
              </span>
            </TableCell>
            <TableCell class="px-6 py-4 text-right">
              <div class="flex flex-wrap items-center justify-end gap-2">
                <Button size="sm" variant="outline" @click="openEditById(product.id)">{{ t('admin.products.actions.edit') }}</Button>
                <Button size="sm" variant="destructive" @click="handleDelete(product)">{{ t('admin.products.actions.delete') }}</Button>
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

    <ProductEditModal
      v-model="showModal"
      :product-id="editingProductId"
      :categories="orderedCategories"
      :site-currency="siteCurrency"
      :supported-locales="['zh-CN', 'zh-TW', 'en-US']"
      @success="handleModalSuccess"
    />
  </div>
</template>
