<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useDebounceFn } from '@vueuse/core'
import { adminAPI } from '@/api/admin'
import type { AdminProductMapping, AdminSiteConnection, AdminCategory, AdminProduct, AdminProductSKU } from '@/api/types'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Switch } from '@/components/ui/switch'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Dialog, DialogScrollContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { confirmAction } from '@/utils/confirm'
import { notifyError, notifySuccess } from '@/utils/notify'
import { getLocalizedText } from '@/utils/format'
import { buildAdminCategoryPath, createAdminCategoryChildCountMap, createAdminCategoryMap, flattenAdminCategories, isAdminProductCategorySelectable } from '@/utils/category'
import TableSkeleton from '@/components/TableSkeleton.vue'
import ListPagination from '@/components/ListPagination.vue'
import { useListRefresh, type ListFetchOptions } from '@/composables/useListRefresh'

const { t } = useI18n()
const loading = ref(true)
const { refreshing, refreshList } = useListRefresh()
const mappings = ref<(AdminProductMapping & { product?: AdminProduct })[]>([])
const connections = ref<AdminSiteConnection[]>([])
const categories = ref<AdminCategory[]>([])
const categoryMap = computed(() => createAdminCategoryMap(categories.value))
const categoryChildCountMap = computed(() => createAdminCategoryChildCountMap(categories.value))
const categoryOptions = computed(() => flattenAdminCategories(categories.value).map((item) => ({
  ...item,
  selectable: isAdminProductCategorySelectable(item.category, categoryChildCountMap.value),
})))
const pagination = reactive({ page: 1, page_size: 20, total: 0, total_page: 1 })
const filters = reactive({ connection_id: '__all__', upstream_status: '__all__', product_status: '__all__', search: '' })
const syncingId = ref<number | null>(null)

// Expand detail
const expandedMappingId = ref<number | null>(null)
const detailLoading = ref(false)

interface SkuMapping {
  local_sku_id: number
  upstream_sku_id: number
  upstream_price: number
  upstream_stock: number
  upstream_is_active: boolean
}

interface MappingDetail {
  mapping: AdminProductMapping
  sku_mappings: SkuMapping[]
}

interface UpstreamSku {
  id: number
  sku_code?: string
  spec_values?: Record<string, string>
  price_amount: number | string
  stock_status: string
  is_active: boolean
}

interface UpstreamProduct {
  id: number
  title: Record<string, string>
  price_amount: number | string
  currency?: string
  is_active: boolean
  category_id?: number
  skus?: UpstreamSku[]
}

interface UpstreamCategory {
  id: number
  parent_id: number
  slug: string
  name: Record<string, string>
  icon: string
  sort_order: number
}

const detailData = ref<MappingDetail | null>(null)

// Import dialog
const showImportModal = ref(false)
const importConnectionId = ref('')
const importCategoryId = ref('__none__')
const upstreamProducts = ref<UpstreamProduct[]>([])
const mappedUpstreamIds = ref<Set<number>>(new Set())
const loadingUpstream = ref(false)
const loadingMoreUpstream = ref(false)
const upstreamPage = ref(1)
const upstreamTotal = ref(0)
const upstreamPageSize = 50
const selectedProductIds = ref<Set<number>>(new Set())
const importExpandedIds = ref<Set<number>>(new Set())
const importing = ref(false)
const importProgress = ref({ done: 0, total: 0, success: 0 })

// Upstream categories (for "by category" mode)
const importViewMode = ref<'category' | 'flat'>('category')
const upstreamCategories = ref<UpstreamCategory[]>([])
const upstreamCategoriesSupported = ref(false)
const loadingUpstreamCategories = ref(false)
// 分类下的真实商品数量（遍历全部上游分页统计得出，不受管理端已加载页数影响）。
// null 表示统计尚未完成或失败，此时分类列表退回按已加载商品计数，不阻塞展示。
const categoryProductCounts = ref<Map<number, number> | null>(null)
const loadingCategoryCounts = ref(false)
const categoryCountsFailed = ref(false)
const expandedCategoryIds = ref<Set<number>>(new Set())
const autoCreateCategory = ref(false)
const categoryImporting = ref(false)

const normalizeFilterValue = (value: string) => (value === '__all__' ? '' : value)

// --- List helpers ---

const getLocalProductTitle = (mapping: AdminProductMapping & { product?: AdminProduct }) => {
  if (!mapping.product) return `#${mapping.local_product_id}`
  return getLocalizedText(mapping.product.title)
}

const getLocalPriceRange = (mapping: AdminProductMapping & { product?: AdminProduct }) => {
  const p = mapping.product
  if (!p) return '-'
  if (!p.skus || p.skus.length === 0) return p.price_amount || '-'
  const skus = p.skus as AdminProductSKU[]
  const prices = skus.map((s) => parseFloat(String(s.price_amount))).filter((v) => !isNaN(v) && v > 0)
  if (prices.length === 0) return p.price_amount || '-'
  const min = Math.min(...prices)
  const max = Math.max(...prices)
  return min === max ? `${min}` : `${min} ~ ${max}`
}

const getLocalSkuCount = (mapping: AdminProductMapping & { product?: AdminProduct }) => mapping.product?.skus?.length || 0

const formatTime = (raw?: string) => {
  if (!raw) return '-'
  const d = new Date(raw)
  return Number.isNaN(d.getTime()) ? '-' : d.toLocaleString()
}

const formatSpecValues = (specValues: Record<string, string> | undefined | null) => {
  if (!specValues || typeof specValues !== 'object') return '-'
  const entries = Object.entries(specValues)
  if (entries.length === 0) return '-'
  return entries.map(([k, v]) => `${k}: ${getLocalizedText(v)}`).join(' / ')
}

const getConnectionName = (connectionId: number) => {
  const conn = connections.value.find((c) => c.id === connectionId)
  return conn?.name || `#${connectionId}`
}

const getCategoryOptionLabel = (category: AdminCategory) => {
  return buildAdminCategoryPath(category, categoryMap.value, (item) => getLocalizedText(item.name))
}

const getCategoryOptionName = (category: AdminCategory) => {
  return getLocalizedText(category.name)
}



// --- Detail expand ---

const toggleMappingExpand = async (mapping: AdminProductMapping & { product?: AdminProduct }) => {
  if (expandedMappingId.value === mapping.id) {
    expandedMappingId.value = null
    detailData.value = null
    return
  }
  expandedMappingId.value = mapping.id
  detailLoading.value = true
  detailData.value = null
  try {
    const res = await adminAPI.getProductMapping(mapping.id)
    detailData.value = res.data.data as unknown as MappingDetail
  } catch {
    detailData.value = null
  } finally {
    detailLoading.value = false
  }
}

// Build a lookup: local_sku_id -> sku_mapping
const skuMappingByLocalId = computed(() => {
  const map: Record<number, SkuMapping> = {}
  if (detailData.value?.sku_mappings) {
    for (const sm of detailData.value.sku_mappings) {
      map[sm.local_sku_id] = sm
    }
  }
  return map
})

const getConnectionExchangeRate = (connectionId: number): number => {
  const conn = connections.value.find((c) => c.id === connectionId)
  return conn?.exchange_rate || 1
}

// 上游价格 × 汇率 = 本地币种等价
const toLocalCurrency = (upstreamPrice: number, connectionId: number): number => {
  return upstreamPrice * getConnectionExchangeRate(connectionId)
}

// --- Fetch ---

const fetchConnections = async () => {
  try {
    const res = await adminAPI.getSiteConnections({ page_size: 100 })
    connections.value = res.data.data || []
  } catch { connections.value = [] }
}

const fetchCategories = async () => {
  try {
    const res = await adminAPI.getCategories()
    categories.value = res.data.data || []
  } catch { categories.value = [] }
}

const fetchMappings = async (page = 1, options: ListFetchOptions = {}) => {
  if (!options.preserveRows) loading.value = true
  expandedMappingId.value = null
  detailData.value = null
  selectedMappingIds.value = new Set()
  try {
    const connId = normalizeFilterValue(filters.connection_id)
    const us = normalizeFilterValue(filters.upstream_status)
    const ps = normalizeFilterValue(filters.product_status)
    const res = await adminAPI.getProductMappings({
      page, page_size: pagination.page_size,
      connection_id: connId || undefined,
      upstream_status: us || undefined,
      product_status: ps || undefined,
      search: filters.search || undefined,
    })
    mappings.value = (res.data.data as (AdminProductMapping & { product?: AdminProduct })[]) || []
    const p = res.data.pagination
    if (p) { pagination.page = p.page; pagination.page_size = p.page_size; pagination.total = p.total; pagination.total_page = p.total_page }
  } catch {
    if (!options.preserveRows) mappings.value = []
  } finally {
    if (!options.preserveRows) loading.value = false
  }
}

const refresh = () => {
  refreshList(() => fetchMappings(pagination.page, { preserveRows: true }))
}

const changePage = (page: number) => { if (page >= 1 && page <= pagination.total_page) fetchMappings(page) }

const pageSizeOptions = [10, 20, 50, 100]

const changePageSize = (size: number) => {
  if (size === pagination.page_size) return
  pagination.page_size = size
  fetchMappings(1)
}

const handleFilterChange = () => fetchMappings(1)
const debouncedSearch = useDebounceFn(() => fetchMappings(1), 300)

// --- Batch selection ---
const selectedMappingIds = ref<Set<number>>(new Set())
const batchOperating = ref(false)

const allMappingsSelected = computed(() => {
  if (mappings.value.length === 0) return false
  return mappings.value.every((m) => selectedMappingIds.value.has(m.id))
})

const toggleMappingSelect = (id: number) => {
  const next = new Set(selectedMappingIds.value)
  next.has(id) ? next.delete(id) : next.add(id)
  selectedMappingIds.value = next
}

const toggleAllMappings = () => {
  selectedMappingIds.value = allMappingsSelected.value
    ? new Set()
    : new Set(mappings.value.map((m) => m.id))
}

const handleBatchSync = async () => {
  const ids = Array.from(selectedMappingIds.value)
  if (ids.length === 0) return
  batchOperating.value = true
  try {
    const res = await adminAPI.batchSyncProductMappings(ids)
    const data = res.data.data as { success_count?: number } | null
    notifySuccess(t('productMappings.batch.syncResult', { success: data?.success_count || 0, total: ids.length }))
    selectedMappingIds.value = new Set()
    fetchMappings(pagination.page)
  } catch (err: any) { notifyError(err?.response?.data?.message || err?.message) } finally { batchOperating.value = false }
}

const handleBatchStatus = async (isActive: boolean) => {
  const ids = Array.from(selectedMappingIds.value)
  if (ids.length === 0) return
  batchOperating.value = true
  try {
    const res = await adminAPI.batchUpdateProductMappingStatus(ids, isActive)
    const data = res.data.data as { success_count?: number } | null
    notifySuccess(t('productMappings.batch.statusResult', { success: data?.success_count || 0, total: ids.length }))
    selectedMappingIds.value = new Set()
    fetchMappings(pagination.page)
  } catch (err: any) { notifyError(err?.response?.data?.message || err?.message) } finally { batchOperating.value = false }
}

const handleBatchDelete = async () => {
  const ids = Array.from(selectedMappingIds.value)
  if (ids.length === 0) return
  const confirmed = await confirmAction({
    description: t('productMappings.batch.deleteConfirm', { count: ids.length }),
    confirmText: t('admin.common.delete'),
    variant: 'destructive',
  })
  if (!confirmed) return
  batchOperating.value = true
  try {
    const res = await adminAPI.batchDeleteProductMappings(ids)
    const data = res.data.data as { success_count?: number } | null
    notifySuccess(t('productMappings.batch.deleteResult', { success: data?.success_count || 0, total: ids.length }))
    selectedMappingIds.value = new Set()
    fetchMappings(pagination.page)
  } catch (err: any) { notifyError(err?.response?.data?.message || err?.message) } finally { batchOperating.value = false }
}

// --- Actions ---

const handleSync = async (mapping: AdminProductMapping) => {
  syncingId.value = mapping.id
  try {
    await adminAPI.syncProductMapping(mapping.id)
    notifySuccess(t('productMappings.sync.success'))
    fetchMappings(pagination.page)
  } catch (err: any) {
    notifyError(t('productMappings.sync.failed') + ': ' + (err?.response?.data?.message || err?.message || ''))
  } finally { syncingId.value = null }
}

const handleToggleStatus = async (mapping: AdminProductMapping) => {
  try {
    await adminAPI.updateProductMappingStatus(mapping.id, { is_active: !mapping.is_active })
    fetchMappings(pagination.page)
    notifySuccess()
  } catch (err: any) { notifyError(err?.response?.data?.message || err?.message) }
}

const handleDelete = async (mapping: AdminProductMapping) => {
  const confirmed = await confirmAction({
    description: t('productMappings.delete.confirm', { id: mapping.id }),
    confirmText: t('admin.common.delete'),
    variant: 'destructive',
  })
  if (!confirmed) return
  try {
    await adminAPI.deleteProductMapping(mapping.id)
    fetchMappings(pagination.page)
    notifySuccess()
  } catch (err: any) { notifyError(err?.response?.data?.message || err?.message) }
}

// --- Import dialog ---

const selectableProducts = computed(() => upstreamProducts.value.filter((p) => !mappedUpstreamIds.value.has(p.id)))

const allSelected = computed(() => {
  if (selectableProducts.value.length === 0) return false
  return selectableProducts.value.every((p) => selectedProductIds.value.has(p.id))
})

const toggleSelectAll = () => {
  selectedProductIds.value = allSelected.value
    ? new Set()
    : new Set(selectableProducts.value.map((p) => p.id))
}

const toggleProduct = (id: number) => {
  if (mappedUpstreamIds.value.has(id)) return
  const next = new Set(selectedProductIds.value)
  next.has(id) ? next.delete(id) : next.add(id)
  selectedProductIds.value = next
}

const toggleImportExpand = (id: number) => {
  const next = new Set(importExpandedIds.value)
  next.has(id) ? next.delete(id) : next.add(id)
  importExpandedIds.value = next
}

const getSkuPriceRange = (product: UpstreamProduct) => {
  if (!product.skus || product.skus.length === 0) return product.price_amount || '-'
  const prices = product.skus.map((s) => parseFloat(String(s.price_amount))).filter((p: number) => !isNaN(p) && p > 0)
  if (prices.length === 0) return product.price_amount || '-'
  const min = Math.min(...prices)
  const max = Math.max(...prices)
  return min === max ? `${min}` : `${min} ~ ${max}`
}

const isSkuAvailable = (status: string) =>
  status === 'in_stock' || status === 'low_stock' || status === 'unlimited'

const getSkuStockSummary = (product: UpstreamProduct) => {
  if (!product.skus || product.skus.length === 0) return '-'
  const total = product.skus.length
  const inStock = product.skus.filter((s) => isSkuAvailable(s.stock_status)).length
  if (inStock === total) return t('productMappings.import.stockAllInStock')
  if (inStock === 0) return t('productMappings.import.stockAllOutOfStock')
  return t('productMappings.import.stockPartial', { inStock, total })
}

const getSkuStockClass = (product: UpstreamProduct) => {
  if (!product.skus || product.skus.length === 0) return 'text-muted-foreground'
  const total = product.skus.length
  const inStock = product.skus.filter((s) => isSkuAvailable(s.stock_status)).length
  if (inStock === total) return 'text-emerald-600'
  if (inStock === 0) return 'text-red-500'
  return 'text-amber-600'
}

const openImportModal = () => {
  importConnectionId.value = ''
  importCategoryId.value = '__none__'
  upstreamProducts.value = []
  upstreamTotal.value = 0
  upstreamPage.value = 1
  mappedUpstreamIds.value = new Set()
  selectedProductIds.value = new Set()
  importExpandedIds.value = new Set()
  categoryProductCounts.value = null
  categoryCountsFailed.value = false
  showImportModal.value = true
}

const closeImportModal = () => { showImportModal.value = false }

const hasMoreUpstream = computed(() => upstreamProducts.value.length < upstreamTotal.value)

const parseUpstreamResult = (res: any): { items: UpstreamProduct[]; total: number; mappedIds?: number[] } => {
  const data = res.data.data
  if (Array.isArray(data)) return { items: data, total: data.length }
  return { items: data?.items || [], total: data?.total || 0, mappedIds: data?.mapped_ids }
}

const fetchUpstreamProducts = async (connectionId: string) => {
  if (!connectionId) { upstreamProducts.value = []; upstreamTotal.value = 0; mappedUpstreamIds.value = new Set(); return }
  loadingUpstream.value = true
  upstreamPage.value = 1
  try {
    const res = await adminAPI.getUpstreamProducts({ connection_id: connectionId, page: 1, page_size: upstreamPageSize })
    const { items, total, mappedIds } = parseUpstreamResult(res)
    upstreamProducts.value = items
    upstreamTotal.value = total
    if (mappedIds) mappedUpstreamIds.value = new Set(mappedIds)
  } catch { upstreamProducts.value = []; upstreamTotal.value = 0; mappedUpstreamIds.value = new Set() } finally { loadingUpstream.value = false }
}

const loadMoreUpstreamProducts = async () => {
  if (!importConnectionId.value || loadingMoreUpstream.value || !hasMoreUpstream.value) return
  loadingMoreUpstream.value = true
  try {
    const nextPage = upstreamPage.value + 1
    const res = await adminAPI.getUpstreamProducts({ connection_id: importConnectionId.value, page: nextPage, page_size: upstreamPageSize })
    const { items, total } = parseUpstreamResult(res)
    upstreamProducts.value.push(...items)
    upstreamTotal.value = total
    upstreamPage.value = nextPage
  } catch { /* ignore */ } finally { loadingMoreUpstream.value = false }
}

const fetchUpstreamCategories = async (connectionId: string) => {
  if (!connectionId) { upstreamCategories.value = []; upstreamCategoriesSupported.value = false; return }
  loadingUpstreamCategories.value = true
  try {
    const res = await adminAPI.getUpstreamCategories({ connection_id: connectionId })
    const data = res.data.data as { supported?: boolean; categories?: UpstreamCategory[] } | null
    upstreamCategories.value = data?.categories || []
    upstreamCategoriesSupported.value = data?.supported ?? false
    // If upstream doesn't support categories, fall back to flat mode
    if (!upstreamCategoriesSupported.value) importViewMode.value = 'flat'
    else importViewMode.value = 'category'
  } catch {
    upstreamCategories.value = []
    upstreamCategoriesSupported.value = false
    importViewMode.value = 'flat'
  } finally { loadingUpstreamCategories.value = false }
}

// 拉取分类下的真实商品数量（后端遍历全部上游分页统计），避免商品数量较多时
// 部分分类的商品仍在未加载的分页中，导致分类列表里看不到、误以为“对接不全”
// 统计要遍历上游全部分页，商品多时较慢，所以不参与列表的加载态。
const fetchUpstreamCategoryCounts = async (connectionId: string) => {
  if (!connectionId) return
  loadingCategoryCounts.value = true
  categoryCountsFailed.value = false
  try {
    const res = await adminAPI.getUpstreamCategoryCounts({ connection_id: connectionId })
    if (importConnectionId.value !== connectionId) return  // 已切换连接，丢弃过期结果
    const counts = (res.data.data?.counts || {}) as Record<string, number>
    categoryProductCounts.value = new Map(Object.entries(counts).map(([k, v]) => [Number(k), v]))
  } catch {
    if (importConnectionId.value !== connectionId) return
    categoryProductCounts.value = null
    categoryCountsFailed.value = true
  } finally {
    if (importConnectionId.value === connectionId) loadingCategoryCounts.value = false
  }
}

// Group upstream products by category_id
const productsByCategory = computed(() => {
  const map = new Map<number, UpstreamProduct[]>()
  for (const p of upstreamProducts.value) {
    const catId = p.category_id || 0
    if (!map.has(catId)) map.set(catId, [])
    map.get(catId)!.push(p)
  }
  return map
})

// Build display list: upstream categories with product counts.
// productCount 优先取 categoryProductCounts（后端统计的真实全量数量），而不是
// upstreamProducts（管理端当前已加载的分页），否则商品较多时未加载分页里的
// 分类会被误判为空、从列表里消失。统计未就绪时退回已加载数量。
const categoryDisplayList = computed(() => {
  // Build parent map for breadcrumb
  const catMap = new Map(upstreamCategories.value.map(c => [c.id, c]))
  const counts = categoryProductCounts.value
  const countOf = (catId: number) => counts ? (counts.get(catId) || 0) : (productsByCategory.value.get(catId) || []).length

  const result: { category: UpstreamCategory; productCount: number; path: string; nonMappedCount: number; fullyLoaded: boolean }[] = []

  for (const cat of upstreamCategories.value) {
    const productCount = countOf(cat.id)
    if (productCount === 0) continue  // Skip empty categories

    const loadedProducts = productsByCategory.value.get(cat.id) || []
    const parentCat = cat.parent_id > 0 ? catMap.get(cat.parent_id) : undefined
    const path = parentCat
      ? `${getLocalizedText(parentCat.name)} / ${getLocalizedText(cat.name)}`
      : getLocalizedText(cat.name)
    // 未映射数量只能基于已加载商品估算；未全部加载时不能当作准确值展示/用于禁用按钮
    const fullyLoaded = loadedProducts.length >= productCount
    const loadedNonMapped = loadedProducts.filter(p => !mappedUpstreamIds.value.has(p.id)).length
    // 未全部加载时，未映射数至少等于“已加载且未映射”的数量，避免因分页未加载全就误判为 0 导致按钮被禁用
    const nonMappedCount = fullyLoaded ? loadedNonMapped : Math.max(loadedNonMapped, 1)

    result.push({ category: cat, productCount, path, nonMappedCount, fullyLoaded })
  }

  // Also include uncategorized (category_id = 0) if any
  const uncategorizedCount = countOf(0)
  if (uncategorizedCount > 0) {
    const loadedUncategorized = productsByCategory.value.get(0) || []
    const fullyLoaded = loadedUncategorized.length >= uncategorizedCount
    const loadedNonMapped = loadedUncategorized.filter(p => !mappedUpstreamIds.value.has(p.id)).length
    const nonMappedCount = fullyLoaded ? loadedNonMapped : Math.max(loadedNonMapped, 1)
    result.push({
      category: { id: 0, parent_id: 0, slug: '', name: {}, icon: '', sort_order: -1 },
      productCount: uncategorizedCount,
      path: t('productMappings.import.uncategorized'),
      nonMappedCount,
      fullyLoaded,
    })
  }

  return result
})

const toggleCategoryExpand = (catId: number) => {
  const s = new Set(expandedCategoryIds.value)
  if (s.has(catId)) s.delete(catId); else s.add(catId)
  expandedCategoryIds.value = s
}

const selectAllInCategory = (catId: number, v: boolean | 'indeterminate') => {
  const products = productsByCategory.value.get(catId) || []
  const s = new Set(selectedProductIds.value)
  const nonMapped = products.filter(p => !mappedUpstreamIds.value.has(p.id))
  if (v === true) {
    for (const p of nonMapped) s.add(p.id)
  } else {
    for (const p of nonMapped) s.delete(p.id)
  }
  selectedProductIds.value = s
}

const isCategoryAllSelected = (catId: number) => {
  const products = productsByCategory.value.get(catId) || []
  const nonMapped = products.filter(p => !mappedUpstreamIds.value.has(p.id))
  return nonMapped.length > 0 && nonMapped.every(p => selectedProductIds.value.has(p.id))
}

const handleImportCategory = async (catId: number) => {
  if (!importConnectionId.value) return
  // Validate: must have autoCreateCategory or a selected local category
  const localCatId = importCategoryId.value !== '__none__' ? Number(importCategoryId.value) : 0
  if (!autoCreateCategory.value && localCatId === 0) {
    notifyError(t('productMappings.import.selectCategoryFirst'))
    return
  }
  categoryImporting.value = true
  try {
    const res = await adminAPI.batchImportByCategory({
      connection_id: Number(importConnectionId.value),
      upstream_category_id: catId,
      auto_create_category: autoCreateCategory.value,
      local_category_id: localCatId || undefined,
    })
    const data = res.data.data as { total: number; success_count: number; category_name?: string } | null
    const total = data?.total || 0
    const success = data?.success_count || 0
    if (success === total) {
      notifySuccess(t('productMappings.import.categoryImportSuccess', { success, total }))
    } else {
      notifySuccess(t('productMappings.import.categoryImportPartial', { success, total }))
    }
    // Refresh mapped IDs and list
    fetchUpstreamProducts(importConnectionId.value)
    fetchUpstreamCategoryCounts(importConnectionId.value)
    fetchMappings(1)
    fetchCategories()
  } catch (err: any) {
    notifyError(err?.response?.data?.message || err?.message)
  } finally { categoryImporting.value = false }
}

watch(importConnectionId, (value) => {
  selectedProductIds.value = new Set()
  importExpandedIds.value = new Set()
  expandedCategoryIds.value = new Set()
  autoCreateCategory.value = false
  categoryProductCounts.value = null
  categoryCountsFailed.value = false
  loadingCategoryCounts.value = false
  fetchUpstreamProducts(value)
  fetchUpstreamCategories(value)
  fetchUpstreamCategoryCounts(value)
})

const BATCH_SIZE = 3

const handleBatchImport = async () => {
  const ids = Array.from(selectedProductIds.value)
  if (ids.length === 0) return
  importing.value = true
  importProgress.value = { done: 0, total: ids.length, success: 0 }
  const categoryId = importCategoryId.value !== '__none__' ? Number(importCategoryId.value) : 0
  const allResults: { upstream_product_id: number; success: boolean; error?: string }[] = []
  let successCount = 0

  try {
    // 分批导入，每批 BATCH_SIZE 条
    for (let i = 0; i < ids.length; i += BATCH_SIZE) {
      const batchIds = ids.slice(i, i + BATCH_SIZE)
      try {
        const res = await adminAPI.batchImportUpstreamProducts({
          connection_id: Number(importConnectionId.value),
          upstream_product_ids: batchIds,
          category_id: categoryId || undefined,
        })
        const result = res.data.data as { results?: typeof allResults; success_count?: number } | null
        const batchResults = result?.results || []
        allResults.push(...batchResults)
        successCount += result?.success_count || 0
      } catch (batchErr: any) {
        if (batchErr?.response?.status === 404) {
          // 后端不支持批量接口，逐条导入
          for (const id of batchIds) {
            try {
              await adminAPI.importUpstreamProduct({ connection_id: Number(importConnectionId.value), upstream_product_id: id, category_id: categoryId || undefined })
              allResults.push({ upstream_product_id: id, success: true }); successCount++
            } catch (singleErr: any) {
              allResults.push({ upstream_product_id: id, success: false, error: singleErr?.response?.data?.message || singleErr?.message })
            }
          }
        } else {
          // 整批失败
          for (const id of batchIds) {
            allResults.push({ upstream_product_id: id, success: false, error: batchErr?.response?.data?.message || batchErr?.message })
          }
        }
      }
      importProgress.value = { done: Math.min(i + BATCH_SIZE, ids.length), total: ids.length, success: successCount }
    }

    if (successCount === ids.length) {
      notifySuccess(t('productMappings.import.batchSuccess', { count: successCount }))
      closeImportModal()
    } else {
      const failed = allResults.filter((r) => !r.success)
      const failedDetails = failed.map((r) => {
        const prod = upstreamProducts.value.find((p) => p.id === r.upstream_product_id)
        const name = prod ? getLocalizedText(prod.title) : `#${r.upstream_product_id}`
        return `${name}: ${r.error || t('productMappings.import.unknownError')}`
      }).join('\n')
      if (successCount > 0) notifySuccess(t('productMappings.import.batchPartial', { success: successCount, total: ids.length }))
      notifyError(failedDetails)
      const successIds = new Set(allResults.filter((r) => r.success).map((r) => r.upstream_product_id))
      selectedProductIds.value = new Set([...selectedProductIds.value].filter(id => !successIds.has(id)))
    }
    // 更新已映射标识
    const newMapped = new Set(mappedUpstreamIds.value)
    for (const r of allResults) { if (r.success) newMapped.add(r.upstream_product_id) }
    mappedUpstreamIds.value = newMapped
    fetchMappings(1)
  } catch (err: any) { notifyError(err?.response?.data?.message || err?.message) } finally { importing.value = false }
}

onMounted(() => { fetchConnections(); fetchCategories(); fetchMappings() })
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <h1 class="text-2xl font-semibold">{{ t('productMappings.title') }}</h1>
      <div class="flex w-full gap-2 sm:w-auto">
        <Button variant="outline" class="w-full sm:w-auto" :disabled="refreshing" @click="refresh">{{ t('productMappings.refresh') }}</Button>
        <Button class="w-full sm:w-auto" @click="openImportModal">{{ t('productMappings.importButton') }}</Button>
      </div>
    </div>

    <!-- Filter -->
    <div class="rounded-xl border border-border bg-card p-4 shadow-sm">
      <div class="flex flex-wrap items-center gap-3">
        <Select v-model="filters.connection_id" @update:modelValue="handleFilterChange">
          <SelectTrigger class="h-9 w-full sm:w-[220px]">
            <SelectValue :placeholder="t('productMappings.filter.connectionPlaceholder')" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">{{ t('productMappings.filter.allConnections') }}</SelectItem>
            <SelectItem v-for="conn in connections" :key="conn.id" :value="String(conn.id)">{{ conn.name }}</SelectItem>
          </SelectContent>
        </Select>
        <div class="relative w-full sm:w-[220px]">
          <div class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">
            <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
          </div>
          <Input
            v-model="filters.search"
            :placeholder="t('productMappings.filter.searchPlaceholder')"
            class="pl-9 h-9"
            @update:modelValue="debouncedSearch"
          />
        </div>
        <Select v-model="filters.upstream_status" @update:modelValue="handleFilterChange">
          <SelectTrigger class="h-9 w-full sm:w-[180px]">
            <SelectValue :placeholder="t('productMappings.filter.upstreamStatusPlaceholder')" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">{{ t('productMappings.filter.allUpstreamStatus') }}</SelectItem>
            <SelectItem value="inactive">{{ t('productMappings.filter.upstreamInactive') }}</SelectItem>
            <SelectItem value="deleted">{{ t('productMappings.filter.upstreamDeleted') }}</SelectItem>
          </SelectContent>
        </Select>
        <Select v-model="filters.product_status" @update:modelValue="handleFilterChange">
          <SelectTrigger class="h-9 w-full sm:w-[180px]">
            <SelectValue :placeholder="t('productMappings.filter.productStatusPlaceholder')" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">{{ t('productMappings.filter.allProductStatus') }}</SelectItem>
            <SelectItem value="active">{{ t('productMappings.filter.productActive') }}</SelectItem>
            <SelectItem value="inactive">{{ t('productMappings.filter.productInactive') }}</SelectItem>
          </SelectContent>
        </Select>
      </div>
    </div>

    <!-- Batch action bar -->
    <div v-if="selectedMappingIds.size > 0" class="flex flex-wrap items-center gap-3 rounded-xl border border-primary/20 bg-primary/5 px-4 py-3">
      <span class="text-sm font-medium">{{ t('productMappings.batch.selected', { count: selectedMappingIds.size }) }}</span>
      <div class="flex flex-wrap gap-2">
        <Button size="sm" variant="outline" :disabled="batchOperating" @click="handleBatchSync">{{ t('productMappings.batch.sync') }}</Button>
        <Button size="sm" variant="outline" :disabled="batchOperating" @click="handleBatchStatus(true)">{{ t('productMappings.batch.enable') }}</Button>
        <Button size="sm" variant="outline" :disabled="batchOperating" @click="handleBatchStatus(false)">{{ t('productMappings.batch.disable') }}</Button>
        <Button size="sm" variant="destructive" :disabled="batchOperating" @click="handleBatchDelete">{{ t('productMappings.batch.delete') }}</Button>
      </div>
      <button class="ml-auto text-xs text-muted-foreground hover:text-foreground" @click="selectedMappingIds = new Set()">{{ t('productMappings.batch.clearSelection') }}</button>
    </div>

    <!-- Mapping list -->
    <div class="space-y-3">
      <div v-if="loading" class="rounded-xl border border-border bg-card overflow-hidden">
        <TableSkeleton :columns="5" :rows="5" />
      </div>
      <div v-else-if="mappings.length === 0" class="rounded-xl border border-border bg-card px-6 py-12 text-center text-muted-foreground">
        {{ t('productMappings.empty') }}
      </div>
      <div v-if="!loading && mappings.length > 0" class="flex items-center gap-2 px-1">
        <Checkbox :model-value="allMappingsSelected ? true : (selectedMappingIds.size > 0 ? 'indeterminate' : false)" @update:model-value="toggleAllMappings" />
        <span class="text-xs text-muted-foreground">{{ t('productMappings.batch.selectAll') }}</span>
      </div>

      <div
        v-for="mapping in mappings"
        :key="mapping.id"
        class="rounded-xl border border-border bg-card overflow-hidden transition-shadow"
        :class="expandedMappingId === mapping.id ? 'shadow-md ring-1 ring-primary/20' : 'shadow-sm'"
      >
        <!-- Collapsed row -->
        <div
          class="flex flex-col gap-3 px-5 py-4 cursor-pointer transition-colors hover:bg-muted/20 sm:flex-row sm:items-center sm:gap-4"
          @click="toggleMappingExpand(mapping)"
        >
          <!-- Checkbox -->
          <div @click.stop>
            <Checkbox :model-value="selectedMappingIds.has(mapping.id)" @update:model-value="() => toggleMappingSelect(mapping.id)" />
          </div>
          <!-- Expand arrow -->
          <svg
            class="h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200"
            :class="expandedMappingId === mapping.id ? 'rotate-90' : ''"
            xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"
          >
            <path fill-rule="evenodd" d="M7.21 14.77a.75.75 0 01.02-1.06L11.168 10 7.23 6.29a.75.75 0 111.04-1.08l4.5 4.25a.75.75 0 010 1.08l-4.5 4.25a.75.75 0 01-1.06-.02z" clip-rule="evenodd" />
          </svg>

          <!-- Product info -->
          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center gap-2">
              <span class="break-words text-sm font-medium text-foreground sm:truncate">{{ getLocalProductTitle(mapping) }}</span>
              <span class="shrink-0 text-[10px] font-mono text-muted-foreground">#{{ mapping.local_product_id }}</span>
              <span
                class="shrink-0 inline-flex rounded-full border px-2 py-0.5 text-[10px]"
                :class="mapping.is_active ? 'text-emerald-700 border-emerald-200 bg-emerald-50' : 'text-muted-foreground border-border bg-muted/30'"
              >
                {{ mapping.is_active ? t('productMappings.status.active') : t('productMappings.status.inactive') }}
              </span>
              <span
                v-if="mapping.upstream_status === 'inactive'"
                class="shrink-0 inline-flex rounded-full border border-amber-200 bg-amber-50 px-2 py-0.5 text-[10px] text-amber-700"
              >
                {{ t('productMappings.upstreamStatus.inactive') }}
              </span>
              <span
                v-else-if="mapping.upstream_status === 'deleted'"
                class="shrink-0 inline-flex rounded-full border border-red-200 bg-red-50 px-2 py-0.5 text-[10px] text-red-700"
              >
                {{ t('productMappings.upstreamStatus.deleted') }}
              </span>
            </div>
            <div class="mt-1 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
              <span>{{ t('productMappings.columns.connection') }}: <span class="text-foreground">{{ getConnectionName(mapping.connection_id) }}</span></span>
              <span>{{ t('productMappings.detail.upstreamId') }}: <span class="font-mono text-foreground">{{ mapping.upstream_product_id }}</span></span>
              <span>{{ t('productMappings.detail.localPrice') }}: <span class="font-mono text-foreground">{{ getLocalPriceRange(mapping) }}</span></span>
              <span>SKU: <span class="text-foreground">{{ getLocalSkuCount(mapping) }}</span></span>
              <span>{{ t('productMappings.columns.lastSynced') }}: {{ formatTime((mapping.last_synced_at ?? mapping.last_sync_at) as string | undefined) }}</span>
            </div>
          </div>

          <!-- Actions -->
          <div class="flex w-full flex-col gap-2 sm:w-auto sm:flex-row sm:items-center sm:gap-2 shrink-0" @click.stop>
            <Button size="sm" variant="outline" class="w-full sm:w-auto" :disabled="syncingId === mapping.id" @click="handleSync(mapping)">
              {{ syncingId === mapping.id ? t('productMappings.actions.syncing') : t('productMappings.actions.sync') }}
            </Button>
            <Button size="sm" variant="outline" class="w-full sm:w-auto" @click="handleToggleStatus(mapping)">
              {{ mapping.is_active ? t('productMappings.actions.disable') : t('productMappings.actions.enable') }}
            </Button>
            <Button size="sm" variant="destructive" class="w-full sm:w-auto" @click="handleDelete(mapping)">{{ t('admin.common.delete') }}</Button>
          </div>
        </div>

        <!-- Expanded detail -->
        <div v-if="expandedMappingId === mapping.id" class="border-t border-border bg-muted/10">
          <div v-if="detailLoading" class="px-6 py-8 text-center text-sm text-muted-foreground">
            {{ t('admin.common.loading') }}
          </div>
          <div v-else-if="!detailData" class="px-6 py-8 text-center text-sm text-muted-foreground">
            {{ t('productMappings.detail.loadFailed') }}
          </div>
          <div v-else class="px-5 py-4">
            <!-- SKU comparison table -->
            <h4 class="mb-3 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              {{ t('productMappings.detail.skuComparison') }}
            </h4>
            <div class="overflow-x-auto rounded-lg border border-border">
              <table class="min-w-[860px] w-full text-xs">
                <thead>
                  <tr class="bg-muted/50 text-muted-foreground">
                    <th class="min-w-[160px] px-3 py-2.5 text-left font-medium">{{ t('productMappings.detail.skuCode') }}</th>
                    <th class="min-w-[220px] px-3 py-2.5 text-left font-medium">{{ t('productMappings.import.skuSpec') }}</th>
                    <th class="min-w-[120px] px-3 py-2.5 text-right font-medium">{{ t('productMappings.detail.localPrice') }}</th>
                    <th class="min-w-[120px] px-3 py-2.5 text-right font-medium">{{ t('productMappings.detail.upstreamPrice') }}</th>
                    <th class="min-w-[120px] px-3 py-2.5 text-right font-medium">{{ t('productMappings.detail.costPrice') }}</th>
                    <th class="min-w-[120px] px-3 py-2.5 text-center font-medium">{{ t('productMappings.detail.priceDiff') }}</th>
                    <th class="min-w-[120px] px-3 py-2.5 text-center font-medium">{{ t('productMappings.detail.upstreamStock') }}</th>
                    <th class="min-w-[120px] px-3 py-2.5 text-center font-medium">{{ t('productMappings.detail.upstreamActive') }}</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-border">
                  <tr v-if="!mapping.product?.skus?.length" >
                    <td colspan="8" class="px-3 py-6 text-center text-muted-foreground">{{ t('productMappings.detail.noSkus') }}</td>
                  </tr>
                  <tr
                    v-for="sku in (mapping.product?.skus as AdminProductSKU[] | undefined) || []"
                    :key="sku.id"
                    class="hover:bg-muted/20"
                  >
                    <td class="min-w-[160px] px-3 py-2.5 font-mono text-muted-foreground break-all">{{ sku.sku_code }}</td>
                    <td class="min-w-[220px] px-3 py-2.5 text-foreground break-words">{{ formatSpecValues(sku.spec_values) }}</td>
                    <td class="min-w-[120px] px-3 py-2.5 text-right font-mono text-foreground">{{ sku.price_amount }}</td>
                    <td class="min-w-[120px] px-3 py-2.5 text-right font-mono">
                      <template v-if="skuMappingByLocalId[sku.id]">
                        <span class="text-foreground">{{ skuMappingByLocalId[sku.id]?.upstream_price }}</span>
                        <span v-if="getConnectionExchangeRate(mapping.connection_id) !== 1" class="ml-0.5 text-[10px] text-muted-foreground">&times;{{ getConnectionExchangeRate(mapping.connection_id) }}</span>
                      </template>
                      <span v-else class="text-muted-foreground">-</span>
                    </td>
                    <td class="min-w-[120px] px-3 py-2.5 text-right font-mono">
                      <template v-if="skuMappingByLocalId[sku.id]">
                        <span class="text-foreground">{{ toLocalCurrency(Number(skuMappingByLocalId[sku.id]?.upstream_price), mapping.connection_id).toFixed(2) }}</span>
                      </template>
                      <span v-else class="text-muted-foreground">-</span>
                    </td>
                    <td class="px-3 py-2.5 text-center">
                      <template v-if="skuMappingByLocalId[sku.id]">
                        <span
                          v-if="Number(sku.price_amount) !== toLocalCurrency(Number(skuMappingByLocalId[sku.id]?.upstream_price), mapping.connection_id)"
                          class="inline-flex rounded-full px-1.5 py-0.5 text-[10px] font-medium border"
                          :class="Number(sku.price_amount) > toLocalCurrency(Number(skuMappingByLocalId[sku.id]?.upstream_price), mapping.connection_id) ? 'text-emerald-700 bg-emerald-50 border-emerald-200' : 'text-red-700 bg-red-50 border-red-200'"
                        >
                          {{ Number(sku.price_amount) > toLocalCurrency(Number(skuMappingByLocalId[sku.id]?.upstream_price), mapping.connection_id) ? '+' : '' }}{{ (Number(sku.price_amount) - toLocalCurrency(Number(skuMappingByLocalId[sku.id]?.upstream_price), mapping.connection_id)).toFixed(2) }}
                        </span>
                        <span v-else class="text-emerald-600">-</span>
                      </template>
                      <span v-else class="text-muted-foreground">-</span>
                    </td>
                    <td class="px-3 py-2.5 text-center">
                      <template v-if="skuMappingByLocalId[sku.id]">
                        <span
                          class="inline-flex rounded-full px-1.5 py-0.5 text-[10px]"
                          :class="(skuMappingByLocalId[sku.id]?.upstream_stock ?? 0) !== 0
                            ? 'text-emerald-700 bg-emerald-50'
                            : 'text-red-600 bg-red-50'"
                        >
                          {{ (skuMappingByLocalId[sku.id]?.upstream_stock ?? 0) !== 0 ? ((skuMappingByLocalId[sku.id]?.upstream_stock ?? 0) < 0 ? t('productMappings.import.unlimited') : t('productMappings.import.inStock')) : t('productMappings.import.outOfStock') }}
                        </span>
                      </template>
                      <span v-else class="text-muted-foreground">-</span>
                    </td>
                    <td class="px-3 py-2.5 text-center">
                      <template v-if="skuMappingByLocalId[sku.id]">
                        <span
                          class="inline-block h-2 w-2 rounded-full"
                          :class="skuMappingByLocalId[sku.id]?.upstream_is_active ? 'bg-emerald-500' : 'bg-gray-300'"
                        />
                      </template>
                      <span v-else class="text-muted-foreground">-</span>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>

            <!-- Mapping metadata -->
            <div class="mt-4 flex flex-wrap gap-x-6 gap-y-1 text-xs text-muted-foreground">
              <span>{{ t('productMappings.detail.mappingId') }}: <span class="font-mono text-foreground">{{ mapping.id }}</span></span>
              <span>{{ t('productMappings.detail.createdAt') }}: {{ formatTime(mapping.created_at) }}</span>
              <span>{{ t('productMappings.detail.updatedAt') }}: {{ formatTime(mapping.updated_at) }}</span>
              <span v-if="mapping.product?.fulfillment_type">
                {{ t('productMappings.detail.fulfillmentType') }}:
                <span class="text-foreground">{{ mapping.product.fulfillment_type }}</span>
              </span>
            </div>
          </div>
        </div>
      </div>

      <!-- Pagination -->
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

    <!-- Import Dialog -->
    <Dialog v-model:open="showImportModal" @update:open="(value: boolean) => { if (!value) closeImportModal() }">
      <DialogScrollContent class="w-[calc(100vw-1rem)] max-w-5xl max-h-[85vh] p-4 sm:p-6" @interact-outside="(e: Event) => e.preventDefault()">
        <DialogHeader>
          <DialogTitle>{{ t('productMappings.importTitle') }}</DialogTitle>
        </DialogHeader>
        <div class="space-y-5">
          <!-- Connection selector + view mode toggle -->
          <div class="flex flex-col gap-4 lg:flex-row lg:items-end">
            <div class="min-w-[200px] flex-1">
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('productMappings.import.selectConnection') }}</label>
              <Select v-model="importConnectionId">
                <SelectTrigger class="h-9 w-full"><SelectValue :placeholder="t('productMappings.import.selectConnectionPlaceholder')" /></SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="conn in connections" :key="conn.id" :value="String(conn.id)">{{ conn.name }}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <!-- View mode tabs (only show when categories are supported) -->
            <div v-if="importConnectionId && upstreamCategoriesSupported" class="flex items-center gap-1 rounded-lg border border-border p-0.5">
              <button
                class="rounded-md px-3 py-1.5 text-xs font-medium transition-colors"
                :class="importViewMode === 'category' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:text-foreground'"
                @click="importViewMode = 'category'"
              >{{ t('productMappings.import.byCategory') }}</button>
              <button
                class="rounded-md px-3 py-1.5 text-xs font-medium transition-colors"
                :class="importViewMode === 'flat' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:text-foreground'"
                @click="importViewMode = 'flat'"
              >{{ t('productMappings.import.flatList') }}</button>
            </div>
          </div>

          <!-- Category options: auto-create + local category selector -->
          <div v-if="importConnectionId" class="flex flex-col gap-4 lg:flex-row lg:items-end">
            <div v-if="importViewMode === 'category' && upstreamCategoriesSupported" class="flex items-center gap-2">
              <Switch id="auto-create-cat" v-model="autoCreateCategory" />
              <Label for="auto-create-cat" class="text-xs font-medium text-muted-foreground cursor-pointer">{{ t('productMappings.import.autoCreateCategory') }}</Label>
            </div>
            <div v-if="!autoCreateCategory || importViewMode === 'flat'" class="min-w-[200px] flex-1">
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('productMappings.import.category') }}</label>
              <Select v-model="importCategoryId">
                <SelectTrigger class="h-9 w-full"><SelectValue :placeholder="t('productMappings.import.categoryPlaceholder')" /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="__none__">{{ t('productMappings.import.noCategory') }}</SelectItem>
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
            </div>
          </div>

          <!-- ===== Category View Mode ===== -->
          <div v-if="importViewMode === 'category' && upstreamCategoriesSupported" class="rounded-lg border border-border overflow-hidden">
            <div v-if="!importConnectionId" class="px-6 py-12 text-center text-sm text-muted-foreground">{{ t('productMappings.import.selectConnectionFirst') }}</div>
            <div v-else-if="loadingUpstream || loadingUpstreamCategories" class="px-6 py-12 text-center text-sm text-muted-foreground">{{ t('productMappings.import.upstreamProductLoading') }}</div>
            <div v-else-if="categoryDisplayList.length === 0" class="px-6 py-12 text-center text-sm text-muted-foreground">{{ t('productMappings.import.noUpstreamProducts') }}</div>
            <div v-else class="divide-y divide-border max-h-[55vh] overflow-y-auto">
              <div v-if="loadingCategoryCounts || categoryCountsFailed" class="px-4 py-2 text-xs text-muted-foreground">
                {{ t(loadingCategoryCounts ? 'productMappings.import.categoryCountsLoading' : 'productMappings.import.categoryCountsFailed') }}
              </div>
              <div v-for="catItem in categoryDisplayList" :key="catItem.category.id">
                <!-- Category header -->
                <div class="flex items-center gap-3 px-4 py-3 bg-muted/30 cursor-pointer hover:bg-muted/50 transition-colors" @click="toggleCategoryExpand(catItem.category.id)">
                  <svg class="h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200" :class="expandedCategoryIds.has(catItem.category.id) ? 'rotate-90' : ''" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor">
                    <path fill-rule="evenodd" d="M7.21 14.77a.75.75 0 01.02-1.06L11.168 10 7.23 6.29a.75.75 0 111.04-1.08l4.5 4.25a.75.75 0 010 1.08l-4.5 4.25a.75.75 0 01-1.06-.02z" clip-rule="evenodd" />
                  </svg>
                  <div class="min-w-0 flex-1">
                    <span class="text-sm font-medium text-foreground">{{ catItem.path }}</span>
                    <span class="ml-2 text-xs text-muted-foreground">{{ t('productMappings.import.productsCount', { count: catItem.productCount }) }}</span>
                    <span v-if="catItem.fullyLoaded && catItem.nonMappedCount < catItem.productCount" class="ml-1 text-xs text-amber-600">({{ catItem.productCount - catItem.nonMappedCount }} {{ t('productMappings.import.alreadyMapped') }})</span>
                  </div>
                  <div class="flex items-center gap-2 shrink-0" @click.stop>
                    <Checkbox
                      :model-value="isCategoryAllSelected(catItem.category.id)"
                      @update:model-value="(v) => selectAllInCategory(catItem.category.id, v)"
                    />
                    <Button
                      size="sm"
                      variant="outline"
                      class="h-7 text-xs"
                      :disabled="categoryImporting || catItem.nonMappedCount === 0"
                      @click="handleImportCategory(catItem.category.id)"
                    >{{ t('productMappings.import.importCategory') }}</Button>
                  </div>
                </div>

                <!-- Expanded product list under this category -->
                <div v-if="expandedCategoryIds.has(catItem.category.id)">
                  <div
                    v-for="product in (productsByCategory.get(catItem.category.id) || [])"
                    :key="product.id"
                    class="transition-colors border-t border-border/30"
                    :class="[selectedProductIds.has(product.id) ? 'bg-primary/5' : '', mappedUpstreamIds.has(product.id) ? 'opacity-50' : '']"
                  >
                    <div class="flex items-center gap-3 px-4 pl-11 py-2.5" :class="mappedUpstreamIds.has(product.id) ? 'cursor-not-allowed' : 'cursor-pointer hover:bg-muted/20'" @click="toggleProduct(product.id)">
                      <div @click.stop>
                        <Checkbox :model-value="selectedProductIds.has(product.id)" :disabled="mappedUpstreamIds.has(product.id)" @update:model-value="() => toggleProduct(product.id)" />
                      </div>
                      <div class="min-w-0 flex-1">
                        <div class="flex flex-wrap items-center gap-2">
                          <span class="break-words text-sm text-foreground sm:truncate">{{ getLocalizedText(product.title) }}</span>
                          <span class="shrink-0 text-[10px] font-mono text-muted-foreground">#{{ product.id }}</span>
                          <span v-if="mappedUpstreamIds.has(product.id)" class="shrink-0 inline-flex rounded-full border border-amber-200 bg-amber-50 px-2 py-0.5 text-[10px] text-amber-700">
                            {{ t('productMappings.import.alreadyMapped') }}
                          </span>
                        </div>
                        <div class="mt-0.5 flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
                          <span>{{ getSkuPriceRange(product) }}<span v-if="product.currency" class="ml-0.5">{{ product.currency }}</span></span>
                          <span>SKU: {{ product.skus?.length || 0 }}</span>
                          <span :class="getSkuStockClass(product)">{{ getSkuStockSummary(product) }}</span>
                        </div>
                      </div>
                    </div>
                  </div>
                  <div v-if="!catItem.fullyLoaded" class="px-4 pl-11 py-2 text-xs text-muted-foreground border-t border-border/30">
                    {{ t('productMappings.import.categoryPartiallyLoaded', { loaded: (productsByCategory.get(catItem.category.id) || []).length, total: catItem.productCount }) }}
                  </div>
                </div>
              </div>

              <!-- Load more for category mode -->
              <div v-if="hasMoreUpstream" class="px-4 py-3 text-center border-t border-border/50">
                <button
                  class="inline-flex items-center gap-1.5 rounded-md px-4 py-1.5 text-xs font-medium text-primary hover:bg-primary/5 transition-colors disabled:opacity-50"
                  :disabled="loadingMoreUpstream"
                  @click="loadMoreUpstreamProducts"
                >
                  {{ loadingMoreUpstream ? t('productMappings.import.loadingMore') : t('productMappings.import.loadMore', { remaining: upstreamTotal - upstreamProducts.length }) }}
                </button>
              </div>
            </div>
          </div>

          <!-- ===== Flat List View Mode ===== -->
          <div v-if="importViewMode === 'flat' || !upstreamCategoriesSupported" class="rounded-lg border border-border overflow-hidden">
            <div v-if="!importConnectionId" class="px-6 py-12 text-center text-sm text-muted-foreground">{{ t('productMappings.import.selectConnectionFirst') }}</div>
            <div v-else-if="loadingUpstream" class="px-6 py-12 text-center text-sm text-muted-foreground">{{ t('productMappings.import.upstreamProductLoading') }}</div>
            <div v-else-if="upstreamProducts.length === 0" class="px-6 py-12 text-center text-sm text-muted-foreground">{{ t('productMappings.import.noUpstreamProducts') }}</div>
            <div v-else>
              <div class="flex items-center gap-3 border-b border-border bg-muted/40 px-4 py-2.5">
                <Checkbox :model-value="allSelected ? true : (selectedProductIds.size > 0 ? 'indeterminate' : false)" @update:model-value="toggleSelectAll" />
                <span class="text-xs font-medium text-muted-foreground">{{ t('productMappings.import.selectAll') }} ({{ upstreamProducts.length }}/{{ upstreamTotal }})</span>
              </div>
              <div class="divide-y divide-border max-h-[50vh] overflow-y-auto">
                <div v-for="product in upstreamProducts" :key="product.id" class="transition-colors" :class="[selectedProductIds.has(product.id) ? 'bg-primary/5' : '', mappedUpstreamIds.has(product.id) ? 'opacity-50' : '']">
                  <div class="flex items-center gap-3 px-4 py-3" :class="mappedUpstreamIds.has(product.id) ? 'cursor-not-allowed' : 'cursor-pointer hover:bg-muted/20'" @click="toggleProduct(product.id)">
                    <div @click.stop>
                      <Checkbox :model-value="selectedProductIds.has(product.id)" :disabled="mappedUpstreamIds.has(product.id)" @update:model-value="() => toggleProduct(product.id)" />
                    </div>
                    <div class="min-w-0 flex-1">
                      <div class="flex flex-wrap items-center gap-2">
                        <span class="break-words text-sm font-medium text-foreground sm:truncate">{{ getLocalizedText(product.title) }}</span>
                        <span class="shrink-0 text-[10px] font-mono text-muted-foreground">#{{ product.id }}</span>
                        <span class="shrink-0 inline-flex rounded-full border px-2 py-0.5 text-[10px]" :class="product.is_active ? 'text-emerald-700 border-emerald-200 bg-emerald-50' : 'text-muted-foreground border-border bg-muted/30'">
                          {{ product.is_active ? t('productMappings.status.active') : t('productMappings.status.inactive') }}
                        </span>
                        <span v-if="mappedUpstreamIds.has(product.id)" class="shrink-0 inline-flex rounded-full border border-amber-200 bg-amber-50 px-2 py-0.5 text-[10px] text-amber-700">
                          {{ t('productMappings.import.alreadyMapped') }}
                        </span>
                      </div>
                      <div class="mt-1 flex flex-wrap items-center gap-4 text-xs text-muted-foreground">
                        <span>{{ t('productMappings.import.colPrice') }}: <span class="font-mono text-foreground">{{ getSkuPriceRange(product) }}</span><span v-if="product.currency" class="ml-0.5">{{ product.currency }}</span></span>
                        <span>SKU: <span class="text-foreground">{{ product.skus?.length || 0 }}</span></span>
                        <span :class="getSkuStockClass(product)">{{ getSkuStockSummary(product) }}</span>
                      </div>
                    </div>
                    <button v-if="product.skus && product.skus.length > 0" class="shrink-0 rounded p-1 text-muted-foreground hover:bg-muted hover:text-foreground transition-colors" @click.stop="toggleImportExpand(product.id)" :title="t('productMappings.import.toggleSkuDetails')">
                      <svg class="h-4 w-4 transition-transform duration-200" :class="importExpandedIds.has(product.id) ? 'rotate-180' : ''" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor">
                        <path fill-rule="evenodd" d="M5.23 7.21a.75.75 0 011.06.02L10 11.168l3.71-3.938a.75.75 0 111.08 1.04l-4.25 4.5a.75.75 0 01-1.08 0l-4.25-4.5a.75.75 0 01.02-1.06z" clip-rule="evenodd" />
                      </svg>
                    </button>
                  </div>
                  <div v-if="importExpandedIds.has(product.id) && product.skus && product.skus.length > 0" class="border-t border-border/50 bg-muted/20 px-4 py-2">
                    <div class="ml-7 overflow-x-auto">
                      <div class="grid min-w-[640px] grid-cols-[1fr_auto_auto_auto] gap-x-4 text-xs">
                        <div class="font-medium text-muted-foreground py-1.5 border-b border-border/30">{{ t('productMappings.import.skuSpec') }}</div>
                        <div class="font-medium text-muted-foreground py-1.5 border-b border-border/30 text-right">{{ t('productMappings.import.skuPrice') }}</div>
                        <div class="font-medium text-muted-foreground py-1.5 border-b border-border/30 text-center">{{ t('productMappings.import.skuStock') }}</div>
                        <div class="font-medium text-muted-foreground py-1.5 border-b border-border/30 text-center">{{ t('productMappings.import.skuActive') }}</div>
                        <template v-for="sku in product.skus" :key="sku.id">
                          <div class="py-1.5 text-foreground">
                            <span v-if="sku.sku_code" class="mr-2 break-all font-mono text-muted-foreground">{{ sku.sku_code }}</span>
                            <span>{{ formatSpecValues(sku.spec_values) }}</span>
                          </div>
                          <div class="py-1.5 text-right font-mono text-foreground">{{ sku.price_amount }}<span v-if="product.currency" class="text-muted-foreground ml-0.5">{{ product.currency }}</span></div>
                          <div class="py-1.5 text-center">
                            <span class="inline-flex rounded-full px-1.5 py-0.5 text-[10px]" :class="isSkuAvailable(sku.stock_status) ? 'text-emerald-700 bg-emerald-50' : 'text-red-600 bg-red-50'">
                              {{ isSkuAvailable(sku.stock_status) ? t('productMappings.import.inStock') : t('productMappings.import.outOfStock') }}
                            </span>
                          </div>
                          <div class="py-1.5 text-center"><span class="inline-block h-2 w-2 rounded-full" :class="sku.is_active ? 'bg-emerald-500' : 'bg-gray-300'" /></div>
                        </template>
                      </div>
                    </div>
                  </div>
                </div>
                <div v-if="hasMoreUpstream" class="px-4 py-3 text-center border-t border-border/50">
                  <button
                    class="inline-flex items-center gap-1.5 rounded-md px-4 py-1.5 text-xs font-medium text-primary hover:bg-primary/5 transition-colors disabled:opacity-50"
                    :disabled="loadingMoreUpstream"
                    @click="loadMoreUpstreamProducts"
                  >
                    {{ loadingMoreUpstream ? t('productMappings.import.loadingMore') : t('productMappings.import.loadMore', { remaining: upstreamTotal - upstreamProducts.length }) }}
                  </button>
                </div>
              </div>
            </div>
          </div>

          <!-- Footer: selected count + action buttons -->
          <div class="flex flex-col gap-3 border-t border-border pt-4 sm:flex-row sm:items-center sm:justify-between">
            <span v-if="selectedProductIds.size > 0" class="text-sm text-muted-foreground">{{ t('productMappings.import.selectedCount', { count: selectedProductIds.size }) }}</span>
            <span v-else />
            <div class="flex flex-col-reverse gap-3 sm:flex-row">
              <Button variant="outline" class="w-full sm:w-auto" @click="closeImportModal">{{ t('admin.common.cancel') }}</Button>
              <Button class="w-full sm:w-auto" :disabled="selectedProductIds.size === 0 || !importConnectionId || importing" @click="handleBatchImport">
                {{ importing ? t('productMappings.import.importingProgress', { done: importProgress.done, total: importProgress.total, success: importProgress.success }) : t('productMappings.import.submitBatch', { count: selectedProductIds.size }) }}
              </Button>
            </div>
          </div>
        </div>
      </DialogScrollContent>
    </Dialog>
  </div>
</template>
