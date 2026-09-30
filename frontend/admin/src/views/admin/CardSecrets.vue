<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useDebounceFn } from '@vueuse/core'
import { adminAPI, type AdminCardSecretQueryPayload } from '@/api/admin'
import type { AdminProduct, AdminProductSKU, AdminCardSecret, AdminCardSecretBatch } from '@/api/types'
import { Upload, PackagePlus } from 'lucide-vue-next'
import IdCell from '@/components/IdCell.vue'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { toggleArrayMember } from '@/lib/utils'
import ListPagination from '@/components/ListPagination.vue'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import TableSkeleton from '@/components/TableSkeleton.vue'
import { useListRefresh, type ListFetchOptions } from '@/composables/useListRefresh'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { formatDate, getLocalizedText } from '@/utils/format'
import { confirmAction } from '@/utils/confirm'
import CardSecretEditModal from './components/CardSecretEditModal.vue'
import { adminUrl } from '@/utils/adminBase'

const { t } = useI18n()
const { refreshing, refreshList } = useListRefresh()
const pageSizeOptions = [10, 20, 50, 100, 200]

const productKeyword = ref('')
const productOptions = ref<AdminProduct[]>([])
const productOptionsLoading = ref(false)
const selectedProductValue = ref('__all__')
const productInfo = ref<AdminProduct | null>(null)
const skuFilterValue = ref('__all__')

const stats = ref<Record<string, unknown> | null>(null)
const statsLoading = ref(false)

const batches = ref<AdminCardSecretBatch[]>([])
const batchesLoading = ref(false)
const batchPagination = ref({
  page: 1,
  page_size: 10,
  total: 0,
  total_page: 1,
})

const cardSecrets = ref<AdminCardSecret[]>([])
const cardSecretsLoading = ref(false)
const cardSecretPagination = ref({
  page: 1,
  page_size: 20,
  total: 0,
  total_page: 1,
})

const filters = reactive({
  status: '__all__',
  secret: '',
  batchNo: '',
})

const currentBatchFilter = ref<AdminCardSecretBatch | null>(null)
const selectedSecretIds = ref<number[]>([])
const operationScope = ref<'selected' | 'filtered'>('selected')
const batchStatusTarget = ref<'available' | 'reserved' | 'used'>('available')
const batchActionLoading = ref(false)
const batchActionError = ref('')
const batchActionSuccess = ref('')

const showEditModal = ref(false)
const editingCardSecret = ref<AdminCardSecret | null>(null)

const normalizeFilterValue = (value: string) => (value === '__all__' ? '' : value)

const parseProductId = () => {
  if (selectedProductValue.value === '__all__') return null
  const parsed = Number(selectedProductValue.value)
  if (!Number.isFinite(parsed) || parsed <= 0) return null
  return Math.floor(parsed)
}

const parseSkuId = () => {
  if (skuFilterValue.value === '__all__') return 0
  const parsed = Number(skuFilterValue.value)
  if (!Number.isFinite(parsed) || parsed <= 0) return 0
  return Math.floor(parsed)
}

const formatSkuSpecValues = (specValues: Record<string, string> | null | undefined) => {
  if (!specValues || typeof specValues !== 'object' || Array.isArray(specValues)) return ''
  return Object.entries(specValues as Record<string, string>)
    .map(([key, value]) => {
      const keyText = String(key || '').trim()
      const valueText = Array.isArray(value)
        ? value.map((entry) => String(entry || '').trim()).filter(Boolean).join(', ')
        : String(value ?? '').trim()
      if (!valueText) return ''
      if (!keyText) return valueText
      return `${keyText}:${valueText}`
    })
    .filter(Boolean)
    .join(' / ')
}

const buildSkuLabel = (sku: AdminProductSKU | null | undefined) => {
  const skuCode = String(sku?.sku_code || '').trim()
  const specText = formatSkuSpecValues(sku?.spec_values)
  if (skuCode && specText) return `${skuCode} · ${specText}`
  if (skuCode) return skuCode
  if (specText) return specText
  if (sku?.id) return `#${sku.id}`
  return '-'
}

const buildProductLabel = (product: AdminProduct | null | undefined) => {
  const id = Number(product?.id || 0)
  const name = getLocalizedText(product?.title || {})
  if (id > 0 && name) return `#${id} ${name}`
  if (id > 0) return `#${id}`
  return name || '-'
}

const currentProductId = computed(() => parseProductId())
const currentSkuId = computed(() => parseSkuId())
const currentBatchId = computed(() => {
  const raw = Number(currentBatchFilter.value?.id || 0)
  if (!Number.isFinite(raw) || raw <= 0) return 0
  return Math.floor(raw)
})

const productHint = computed(() => {
  if (!currentProductId.value) return t('admin.cardSecrets.productHintEmpty')
  return t('admin.cardSecrets.productHintCurrent', { id: currentProductId.value })
})

const productInfoName = computed(() => {
  if (productInfo.value) return getLocalizedText(productInfo.value.title)
  const option = productOptions.value.find((item: AdminProduct) => Number(item?.id || 0) === currentProductId.value)
  if (!option) return ''
  return getLocalizedText(option.title || {})
})

const availableSkus = computed(() => {
  const rows = Array.isArray(productInfo.value?.skus) ? productInfo.value.skus : []
  return rows
    .filter((sku: AdminProductSKU) => Boolean(sku?.is_active))
    .map((sku: AdminProductSKU) => ({
      ...sku,
      id: Number(sku.id),
      label: buildSkuLabel(sku),
    }))
    .filter((sku: AdminProductSKU & { label: string }) => Number.isFinite(sku.id) && sku.id > 0)
})

const skuFilterDisabled = computed(() => !currentProductId.value || availableSkus.value.length === 0)
const currentBatchFilterText = computed(() => {
  if (!currentBatchFilter.value) return t('admin.cardSecrets.batchFilterAll')
  return t('admin.cardSecrets.batchFilterCurrent', {
    id: currentBatchFilter.value.id,
    batchNo: currentBatchFilter.value.batch_no || '-',
  })
})

const allCurrentPageSelected = computed(() => {
  if (cardSecrets.value.length === 0) return false
  return cardSecrets.value.every((item: AdminCardSecret) => selectedSecretIds.value.includes(Number(item?.id || 0)))
})

const syncSkuSelection = () => {
  if (!currentProductId.value || availableSkus.value.length === 0) {
    skuFilterValue.value = '__all__'
    return
  }
  if (availableSkus.value.length === 1) {
    skuFilterValue.value = String(availableSkus.value[0]!.id)
    return
  }
  const matched = availableSkus.value.some((sku) => sku.id === currentSkuId.value)
  if (!matched) {
    skuFilterValue.value = '__all__'
  }
}

const resolveSkuLabelById = (skuID: number) => {
  if (!skuID) return '-'
  const target = availableSkus.value.find((sku) => sku.id === skuID)
  if (!target) return `#${skuID}`
  return target.label
}

const resolveProductName = (productId: number) => {
  if (!productId) return ''
  if (productInfo.value && Number(productInfo.value.id || 0) === productId) {
    return getLocalizedText(productInfo.value.title)
  }
  const option = productOptions.value.find((item: AdminProduct) => Number(item?.id || 0) === productId)
  if (!option) return ''
  return getLocalizedText(option.title || {})
}

const buildBatchLabel = (batch: Partial<AdminCardSecretBatch> | null | undefined) => {
  const id = Number(batch?.id || 0)
  const batchNo = String(batch?.batch_no || '').trim()
  if (id > 0 && batchNo) return `#${id} ${batchNo}`
  if (id > 0) return `#${id}`
  return batchNo || '-'
}

const resolveSecretBatchLabel = (secret: AdminCardSecret) => {
  if (secret.batch) return buildBatchLabel(secret.batch)
  if (secret.batch_id) return `#${secret.batch_id}`
  return '-'
}

const productLink = (productId: number) => adminUrl(`/products?product_id=${productId}`)
const orderLink = (orderId: number) => adminUrl(`/orders?order_id=${orderId}`)

const clearBatchActionMessages = () => {
  batchActionError.value = ''
  batchActionSuccess.value = ''
}

const normalizeSelectedSecretIDs = () => {
  return Array.from(
    new Set(
      selectedSecretIds.value
        .map((item) => Number(item))
        .filter((item) => Number.isFinite(item) && item > 0)
        .map((item) => Math.floor(item))
    )
  )
}

const selectedSecretCount = computed(() => normalizeSelectedSecretIDs().length)
const filteredSecretCount = computed(() => Number(cardSecretPagination.value.total || 0))
const currentScopeCount = computed(() => (
  operationScope.value === 'selected' ? selectedSecretCount.value : filteredSecretCount.value
))
const currentScopeLabel = computed(() => (
  operationScope.value === 'selected'
    ? t('admin.cardSecrets.batch.scopeSelected')
    : t('admin.cardSecrets.batch.scopeFiltered')
))
const hasActionableScope = computed(() => currentScopeCount.value > 0)

const currentQueryFilter = computed<AdminCardSecretQueryPayload>(() => {
  const payload: AdminCardSecretQueryPayload = {
    status: normalizeFilterValue(filters.status) || undefined,
    secret: String(filters.secret || '').trim() || undefined,
    batch_no: String(filters.batchNo || '').trim() || undefined,
  }
  if (currentProductId.value) {
    payload.product_id = currentProductId.value
    payload.sku_id = currentSkuId.value || undefined
  }
  if (currentBatchId.value) {
    payload.batch_id = currentBatchId.value
  }
  return payload
})

const scopeHint = computed(() => (
  operationScope.value === 'selected'
    ? t('admin.cardSecrets.batch.scopeHintSelected', { count: selectedSecretCount.value })
    : t('admin.cardSecrets.batch.scopeHintFiltered', { count: filteredSecretCount.value })
))

const toggleSelectAllSecrets = () => {
  if (allCurrentPageSelected.value) {
    selectedSecretIds.value = []
    return
  }
  selectedSecretIds.value = cardSecrets.value
    .map((item: AdminCardSecret) => Number(item?.id || 0))
    .filter((item: number) => Number.isFinite(item) && item > 0)
}

const toggleSecretSelected = (id: number, v: boolean | 'indeterminate') => {
  toggleArrayMember(selectedSecretIds, id, v)
}

const clearBatchFilter = () => {
  currentBatchFilter.value = null
}

const clearBatchFilterAndRefresh = async () => {
  clearBatchFilter()
  await fetchCardSecrets(1)
}

const buildPlaceholderBatch = (batchID: number): AdminCardSecretBatch => ({
  id: batchID,
  product_id: currentProductId.value || 0,
  sku_id: currentSkuId.value || 0,
  name: '',
  batch_no: '',
  source: '',
  note: '',
  total_count: 0,
  available_count: 0,
  reserved_count: 0,
  used_count: 0,
  created_at: '',
})

const findBatchById = (batchID: number) => {
  if (!Number.isFinite(batchID) || batchID <= 0) return null
  if (currentBatchFilter.value && Number(currentBatchFilter.value.id || 0) === batchID) {
    return currentBatchFilter.value
  }
  return batches.value.find((batch: AdminCardSecretBatch) => Number(batch?.id || 0) === batchID) || null
}

const loadProductOptions = async () => {
  productOptionsLoading.value = true
  try {
    const keyword = String(productKeyword.value || '').trim()
    const rows: AdminProduct[] = []
    let page = 1
    let totalPage = 1
    do {
      const response = await adminAPI.getProducts({
        page,
        page_size: 100,
        search: keyword || undefined,
        fulfillment_type: 'auto',
      })
      const list = Array.isArray(response.data.data) ? response.data.data : []
      rows.push(...list.filter((item: AdminProduct) => String(item?.fulfillment_type || '').trim() === 'auto'))
      totalPage = Number(response.data?.pagination?.total_page || 1)
      page += 1
    } while (page <= totalPage && page <= 20)

    const dedup = new Map<number, AdminProduct>()
    rows.forEach((item: AdminProduct) => {
      const id = Number(item?.id || 0)
      if (!Number.isFinite(id) || id <= 0) return
      if (!dedup.has(id)) dedup.set(id, item)
    })

    const options = Array.from(dedup.values())
    if (
      currentProductId.value &&
      !options.some((item: AdminProduct) => Number(item?.id || 0) === currentProductId.value)
    ) {
      if (productInfo.value && Number(productInfo.value.id || 0) === currentProductId.value) {
        options.unshift(productInfo.value)
      } else {
        options.unshift({
          id: currentProductId.value,
          title: {
            'zh-CN': `#${currentProductId.value}`,
            'zh-TW': `#${currentProductId.value}`,
            'en-US': `#${currentProductId.value}`,
          },
          fulfillment_type: 'auto',
        } as unknown as AdminProduct)
      }
    }

    productOptions.value = options
  } catch {
    productOptions.value = []
  } finally {
    productOptionsLoading.value = false
  }
}

const loadProductInfo = async () => {
  const productId = parseProductId()
  if (!productId) {
    productInfo.value = null
    skuFilterValue.value = '__all__'
    return
  }
  try {
    const response = await adminAPI.getProduct(productId)
    productInfo.value = response.data.data
    if (!productOptions.value.some((item: AdminProduct) => Number(item?.id || 0) === productId)) {
      productOptions.value.unshift(response.data.data)
    }
    syncSkuSelection()
  } catch {
    productInfo.value = null
    skuFilterValue.value = '__all__'
  }
}

const refreshStats = async () => {
  const productId = parseProductId()
  if (!productId) {
    stats.value = null
    return
  }
  statsLoading.value = true
  try {
    const response = await adminAPI.getCardSecretStats({
      product_id: productId,
      sku_id: currentSkuId.value || undefined,
    })
    stats.value = response.data.data
  } catch {
    stats.value = null
  } finally {
    statsLoading.value = false
  }
}

const fetchBatches = async (page = 1, options: ListFetchOptions = {}) => {
  const productId = parseProductId()
  if (!productId) {
    batches.value = []
    batchPagination.value = {
      page: 1,
      page_size: 10,
      total: 0,
      total_page: 1,
    }
    return
  }
  if (!options.preserveRows) batchesLoading.value = true
  try {
    const response = await adminAPI.getCardSecretBatches({
      product_id: productId,
      sku_id: currentSkuId.value || undefined,
      page,
      page_size: batchPagination.value.page_size,
    })
    batches.value = response.data.data || []
    batchPagination.value = response.data.pagination || batchPagination.value
  } catch {
    if (!options.preserveRows) batches.value = []
  } finally {
    if (!options.preserveRows) batchesLoading.value = false
  }
}

const fetchCardSecrets = async (page = 1, options: ListFetchOptions = {}) => {
  if (!options.preserveRows) cardSecretsLoading.value = true
  try {
    const response = await adminAPI.getCardSecrets({
      ...currentQueryFilter.value,
      page,
      page_size: cardSecretPagination.value.page_size,
    })
    const nextRows = response.data.data || []
    const nextPagination = response.data.pagination || cardSecretPagination.value
    if (page > Number(nextPagination.total_page || 1) && Number(nextPagination.total_page || 1) > 0) {
      await fetchCardSecrets(Number(nextPagination.total_page || 1), options)
      return
    }
    cardSecrets.value = nextRows
    cardSecretPagination.value = nextPagination
    selectedSecretIds.value = []
  } catch {
    if (!options.preserveRows) {
      cardSecrets.value = []
      selectedSecretIds.value = []
    }
  } finally {
    if (!options.preserveRows) cardSecretsLoading.value = false
  }
}

const refreshAll = async (options: ListFetchOptions = {}) => {
  const productId = parseProductId()
  clearBatchActionMessages()
  if (productId) {
    await loadProductInfo()
    await Promise.all([refreshStats(), fetchBatches(1, options), fetchCardSecrets(1, options)])
    return
  }

  productInfo.value = null
  stats.value = null
  clearBatchFilter()
  batches.value = []
  batchPagination.value = {
    page: 1,
    page_size: 10,
    total: 0,
    total_page: 1,
  }
  skuFilterValue.value = '__all__'
  await fetchCardSecrets(1, options)
}

const refreshAfterMutations = async () => {
  await fetchCardSecrets(cardSecretPagination.value.page)
  if (currentProductId.value) {
    await Promise.all([refreshStats(), fetchBatches(batchPagination.value.page)])
  }
}

const handleSearchProducts = async () => {
  await loadProductOptions()
}

const debouncedSearchProducts = useDebounceFn(handleSearchProducts, 300)

const handleProductSelectionChange = async () => {
  skuFilterValue.value = '__all__'
  clearBatchFilter()
  await refreshAll()
}

const handleSkuSelectionChange = async () => {
  clearBatchFilter()
  await refreshAll()
}

const applyListFilters = async () => {
  clearBatchActionMessages()
  await fetchCardSecrets(1, { preserveRows: true })
}

const resetListFilters = async () => {
  filters.status = '__all__'
  filters.secret = ''
  filters.batchNo = ''
  clearBatchFilter()
  clearBatchActionMessages()
  await fetchCardSecrets(1, { preserveRows: true })
}

const refreshCurrentList = () => {
  refreshList(() => refreshAll({ preserveRows: true }))
}

const refreshBatches = () => {
  refreshList(() => fetchBatches(1, { preserveRows: true }))
}

const changeBatchPage = (page: number) => {
  if (page < 1 || page > batchPagination.value.total_page) return
  fetchBatches(page)
}

const changeSecretPage = (page: number) => {
  if (page < 1 || page > cardSecretPagination.value.total_page) return
  fetchCardSecrets(page)
}

const changeBatchPageSize = (pageSize: number) => {
  if (pageSize === batchPagination.value.page_size) return
  batchPagination.value.page_size = pageSize
  fetchBatches(1)
}

const changeCardSecretPageSize = (pageSize: number) => {
  if (pageSize === cardSecretPagination.value.page_size) return
  cardSecretPagination.value.page_size = pageSize
  fetchCardSecrets(1)
}

const buildActionPayload = () => {
  if (operationScope.value === 'selected') {
    const ids = normalizeSelectedSecretIDs()
    if (ids.length === 0) {
      batchActionError.value = t('admin.cardSecrets.errors.selectRequired')
      return null
    }
    return { ids }
  }
  if (filteredSecretCount.value <= 0) {
    batchActionError.value = t('admin.cardSecrets.errors.scopeEmpty')
    return null
  }
  return { filter: currentQueryFilter.value }
}

const buildDangerDescription = (suffix: string) => {
  return [
    { text: t('admin.cardSecrets.batch.confirmCountPrefix') + ' ' },
    { text: String(currentScopeCount.value), tone: 'danger' as const, strong: true },
    { text: ` ${suffix}` },
  ]
}

const applyBatchStatus = async () => {
  clearBatchActionMessages()
  const payload = buildActionPayload()
  if (!payload) return

  const statusLabel = cardSecretStatusLabel(batchStatusTarget.value)
  const confirmed = await confirmAction({
    description: buildDangerDescription(
      t('admin.cardSecrets.batch.confirmStatusSuffix', {
        scope: currentScopeLabel.value,
        status: statusLabel,
      })
    ),
  })
  if (!confirmed) return

  batchActionLoading.value = true
  try {
    const response = await adminAPI.batchUpdateCardSecretStatus({
      ...payload,
      status: batchStatusTarget.value,
    })
    const fallbackCount = currentScopeCount.value
    const affected = Number(response?.data?.data?.affected || fallbackCount)
    batchActionSuccess.value = t('admin.cardSecrets.success.batchStatusUpdated', { count: affected })
    await refreshAfterMutations()
  } catch (error: any) {
    batchActionError.value = error?.message || t('admin.cardSecrets.errors.batchStatusFailed')
  } finally {
    batchActionLoading.value = false
  }
}

const deleteSecretsInScope = async () => {
  clearBatchActionMessages()
  const payload = buildActionPayload()
  if (!payload) return

  const confirmed = await confirmAction({
    description: buildDangerDescription(
      t('admin.cardSecrets.batch.confirmDeleteSuffix', { scope: currentScopeLabel.value })
    ),
    confirmText: t('admin.common.delete'),
    variant: 'destructive',
  })
  if (!confirmed) return

  batchActionLoading.value = true
  try {
    const response = await adminAPI.batchDeleteCardSecrets(payload)
    const fallbackCount = currentScopeCount.value
    const affected = Number(response?.data?.data?.affected || fallbackCount)
    batchActionSuccess.value = t('admin.cardSecrets.success.batchDeleted', { count: affected })
    if (operationScope.value === 'filtered' && currentBatchId.value && affected >= filteredSecretCount.value) {
      clearBatchFilter()
    }
    await refreshAfterMutations()
  } catch (error: any) {
    batchActionError.value = error?.message || t('admin.cardSecrets.errors.batchDeleteFailed')
  } finally {
    batchActionLoading.value = false
  }
}

const exportSecretsInScope = async (format: 'txt' | 'csv') => {
  clearBatchActionMessages()
  const payload = buildActionPayload()
  if (!payload) return

  batchActionLoading.value = true
  try {
    const response = await adminAPI.exportCardSecrets({
      ...payload,
      format,
    })
    downloadExportFile(response, format)
    batchActionSuccess.value = t('admin.cardSecrets.success.batchExported', {
      count: currentScopeCount.value,
      format: format.toUpperCase(),
    })
  } catch (error: any) {
    batchActionError.value = error?.message || t('admin.cardSecrets.errors.batchExportFailed')
  } finally {
    batchActionLoading.value = false
  }
}

const downloadExportFile = (response: any, format: 'txt' | 'csv') => {
  const contentDisposition = String(response?.headers?.['content-disposition'] || '')
  const filenameMatch = contentDisposition.match(/filename=\"?([^\";]+)\"?/i)
  const fallbackName = `card-secrets-${new Date().toISOString().replace(/[:.]/g, '-')}.${format}`
  const filename = filenameMatch?.[1] || fallbackName

  const contentType = format === 'csv' ? 'text/csv;charset=utf-8' : 'text/plain;charset=utf-8'
  const blob = new Blob([response.data], { type: contentType })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(url)
}

const filterByBatch = async (batch: AdminCardSecretBatch | null) => {
  currentBatchFilter.value = batch
  clearBatchActionMessages()
  await fetchCardSecrets(1)
}

const filterByBatchId = async (batchID: number) => {
  if (!Number.isFinite(batchID) || batchID <= 0) return
  const matchedBatch = findBatchById(Math.floor(batchID))
  await filterByBatch(matchedBatch || buildPlaceholderBatch(Math.floor(batchID)))
}

const openEditSecret = (secret: AdminCardSecret) => {
  editingCardSecret.value = secret
  showEditModal.value = true
}

const handleEditSuccess = async () => {
  await refreshAfterMutations()
}

const cardSecretStatusLabel = (status: string) => {
  const map: Record<string, string> = {
    available: t('admin.cardSecrets.status.available'),
    reserved: t('admin.cardSecrets.status.reserved'),
    used: t('admin.cardSecrets.status.used'),
  }
  return map[status] || status
}

const cardSecretStatusClass = (status: string) => {
  switch (status) {
    case 'available':
      return 'text-emerald-700 border-emerald-200 bg-emerald-50'
    case 'reserved':
      return 'text-amber-700 border-amber-200 bg-amber-50'
    case 'used':
      return 'text-muted-foreground border-border bg-muted/30'
    default:
      return 'text-muted-foreground border-border bg-muted/30'
  }
}

const batchSkuLabel = (batch: AdminCardSecretBatch) => resolveSkuLabelById(Number(batch?.sku_id || 0))
const secretSkuLabel = (secret: AdminCardSecret) => resolveSkuLabelById(Number(secret?.sku_id || 0))

onMounted(async () => {
  await loadProductOptions()
  await fetchCardSecrets(1)
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <h1 class="text-2xl font-semibold">{{ t('admin.cardSecrets.title') }}</h1>
      <Button class="w-full sm:w-auto" as-child>
        <RouterLink to="/card-secret-imports">
          <Upload class="mr-2 h-4 w-4" />
          {{ t('admin.cardSecrets.importAction') }}
        </RouterLink>
      </Button>
    </div>

    <div v-if="!currentProductId" class="rounded-xl border-2 border-dashed border-primary/30 bg-primary/5 p-8">
      <div class="mx-auto max-w-lg space-y-4 text-center">
        <div class="mx-auto flex h-16 w-16 items-center justify-center rounded-full bg-primary/10">
          <PackagePlus class="h-8 w-8 text-primary" />
        </div>
        <h2 class="text-xl font-semibold text-foreground">{{ t('admin.cardSecrets.guide.title') }}</h2>
        <p class="text-sm text-muted-foreground">{{ t('admin.cardSecrets.guide.description') }}</p>
        <div class="flex flex-col gap-4 pt-2 text-left sm:flex-row sm:items-start sm:justify-center sm:gap-6">
          <div class="flex items-start gap-3">
            <div class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-primary text-xs font-bold text-primary-foreground">1</div>
            <div>
              <p class="text-sm font-medium text-foreground">{{ t('admin.cardSecrets.guide.step1Title') }}</p>
              <p class="text-xs text-muted-foreground">{{ t('admin.cardSecrets.guide.step1Desc') }}</p>
            </div>
          </div>
          <div class="flex items-start gap-3">
            <div class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-primary text-xs font-bold text-primary-foreground">2</div>
            <div>
              <p class="text-sm font-medium text-foreground">{{ t('admin.cardSecrets.guide.step2Title') }}</p>
              <p class="text-xs text-muted-foreground">{{ t('admin.cardSecrets.guide.step2Desc') }}</p>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="rounded-xl border border-border bg-card p-4 shadow-sm">
      <div class="grid grid-cols-1 gap-3 md:grid-cols-12">
        <div class="flex flex-col gap-2 md:col-span-4 sm:flex-row sm:items-center">
          <Input
            v-model="productKeyword"
            :placeholder="t('admin.cardSecrets.productSearchPlaceholder')"
            @update:modelValue="debouncedSearchProducts"
            @keyup.enter="handleSearchProducts"
          />
          <Button
            size="sm"
            variant="outline"
            class="h-9 w-full shrink-0 sm:w-auto"
            :disabled="productOptionsLoading"
            @click="handleSearchProducts"
          >
            {{ productOptionsLoading ? t('admin.common.loading') : t('admin.cardSecrets.searchProducts') }}
          </Button>
        </div>

        <div class="md:col-span-3">
          <Select v-model="selectedProductValue" @update:modelValue="handleProductSelectionChange">
            <SelectTrigger class="h-10">
              <SelectValue :placeholder="t('admin.cardSecrets.productSelectPlaceholder')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="__all__">{{ t('admin.cardSecrets.productAll') }}</SelectItem>
              <SelectItem v-for="product in productOptions" :key="product.id" :value="String(product.id)">
                {{ buildProductLabel(product) }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div class="md:col-span-3">
          <Select v-model="skuFilterValue" :disabled="skuFilterDisabled" @update:modelValue="handleSkuSelectionChange">
            <SelectTrigger class="h-10">
              <SelectValue :placeholder="t('admin.cardSecrets.skuPlaceholder')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="__all__">{{ t('admin.cardSecrets.skuAll') }}</SelectItem>
              <SelectItem v-for="sku in availableSkus" :key="sku.id" :value="String(sku.id)">
                {{ sku.label }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div class="md:col-span-2">
          <Button class="w-full" variant="outline" :disabled="refreshing" @click="refreshCurrentList">{{ t('admin.common.refresh') }}</Button>
        </div>
      </div>

      <div class="mt-3 space-y-1 text-xs text-muted-foreground">
        <p>{{ productHint }}</p>
        <p v-if="productInfoName">
          {{ t('admin.cardSecrets.productNameLabel') }}：
          <a
            v-if="currentProductId"
            :href="productLink(currentProductId)"
            target="_blank"
            rel="noopener"
            class="text-primary underline-offset-4 hover:underline"
          >
            {{ productInfoName }}
          </a>
          <span v-else>{{ productInfoName }}</span>
        </p>
        <p v-if="currentProductId">
          {{ t('admin.cardSecrets.skuLabel') }}：{{ currentSkuId ? resolveSkuLabelById(currentSkuId) : t('admin.cardSecrets.skuAll') }}
        </p>
        <p
          v-if="currentProductId && (statsLoading || stats)"
          class="text-sm xl:whitespace-nowrap"
        >
          <template v-if="statsLoading">{{ t('admin.common.loading') }}</template>
          <template v-else-if="stats">
            <strong class="font-semibold text-foreground">{{ t('admin.cardSecrets.statsTitle') }}：</strong>
            {{ t('admin.cardSecrets.stats.available') }}
            (<span class="font-mono text-emerald-700">{{ stats.available }}</span>) /
            {{ t('admin.cardSecrets.stats.reserved') }}
            (<span class="font-mono text-amber-700">{{ stats.reserved }}</span>) /
            {{ t('admin.cardSecrets.stats.used') }}
            (<span class="font-mono text-foreground">{{ stats.used }}</span>) /
            {{ t('admin.cardSecrets.stats.total') }}
            (<span class="font-mono text-foreground">{{ stats.total }}</span>)
          </template>
        </p>
        <p v-if="currentProductId">
          {{ currentBatchFilterText }}
          <button
            v-if="currentBatchId"
            type="button"
            class="ml-2 text-primary underline-offset-4 hover:underline"
            @click="clearBatchFilterAndRefresh"
          >
            {{ t('admin.cardSecrets.batchFilterClear') }}
          </button>
        </p>
      </div>
    </div>

    <div class="rounded-xl border border-border bg-card p-5">
      <div class="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h2 class="text-lg font-semibold text-foreground">{{ t('admin.cardSecrets.batchesTitle') }}</h2>
          <p class="text-xs text-muted-foreground">{{ t('admin.cardSecrets.batch.navigatorHint') }}</p>
        </div>
        <Button size="sm" variant="outline" :disabled="refreshing" @click="refreshBatches">{{ t('admin.common.refresh') }}</Button>
      </div>

      <div class="space-y-3">
        <button
          type="button"
          class="w-full rounded-xl border px-4 py-3 text-left transition hover:border-primary/40 hover:bg-primary/5"
          :class="!currentBatchId ? 'border-primary bg-primary/5 shadow-sm' : 'border-border bg-background'"
          @click="filterByBatch(null)"
        >
          <div class="flex items-center justify-between gap-3">
            <span class="text-sm font-medium text-foreground">{{ t('admin.cardSecrets.batch.navigatorAll') }}</span>
            <span class="text-xs text-muted-foreground">{{ batchPagination.total }}</span>
          </div>
          <p class="mt-2 text-xs text-muted-foreground">{{ t('admin.cardSecrets.batch.navigatorAllDesc') }}</p>
        </button>

        <div class="max-h-[20rem] space-y-3 overflow-y-auto pr-1">
          <div v-if="batchesLoading" class="space-y-3">
            <div v-for="index in 4" :key="index" class="animate-pulse rounded-xl border border-border bg-muted/30 p-4">
              <div class="h-4 w-1/2 rounded bg-muted" />
              <div class="mt-3 h-3 w-3/4 rounded bg-muted" />
              <div class="mt-2 h-3 w-2/3 rounded bg-muted" />
            </div>
          </div>
          <div v-else-if="batches.length === 0" class="rounded-xl border border-dashed border-border px-4 py-10 text-center text-sm text-muted-foreground">
            {{ t('admin.cardSecrets.emptyBatches') }}
          </div>
          <button
            v-for="batch in batches"
            :key="batch.id"
            type="button"
            class="w-full rounded-xl border px-4 py-3 text-left transition hover:border-primary/40 hover:bg-primary/5"
            :class="currentBatchId === Number(batch.id || 0) ? 'border-primary bg-primary/5 shadow-sm' : 'border-border bg-background'"
            @click="filterByBatch(batch)"
          >
            <div class="flex items-start justify-between gap-3">
              <div class="space-y-1">
                <p class="text-sm font-medium text-foreground">{{ batch.batch_no || `#${batch.id}` }}</p>
                <p class="text-xs text-muted-foreground">#{{ batch.id }} · {{ batchSkuLabel(batch) }}</p>
              </div>
              <span class="rounded-full bg-muted px-2 py-1 text-[11px] text-muted-foreground">
                {{ batch.total_count }}
              </span>
            </div>
            <div class="mt-3 flex flex-wrap gap-2 text-[11px]">
              <span class="rounded-full border border-emerald-200 bg-emerald-50 px-2 py-1 text-emerald-700">
                {{ t('admin.cardSecrets.stats.available') }} {{ batch.available_count }}
              </span>
              <span class="rounded-full border border-amber-200 bg-amber-50 px-2 py-1 text-amber-700">
                {{ t('admin.cardSecrets.stats.reserved') }} {{ batch.reserved_count }}
              </span>
              <span class="rounded-full border border-border bg-muted/40 px-2 py-1 text-muted-foreground">
                {{ t('admin.cardSecrets.stats.used') }} {{ batch.used_count }}
              </span>
            </div>
            <p v-if="batch.note" class="mt-3 line-clamp-2 text-xs text-muted-foreground">{{ batch.note }}</p>
            <p class="mt-3 text-[11px] text-muted-foreground">{{ formatDate(batch.created_at) }}</p>
          </button>
        </div>
      </div>

      <ListPagination
        :page="batchPagination.page"
        :total-page="batchPagination.total_page"
        :total="batchPagination.total"
        :page-size="batchPagination.page_size"
        :page-size-options="pageSizeOptions"
        @change-page="changeBatchPage"
        @change-page-size="changeBatchPageSize"
      />
    </div>

    <div class="rounded-xl border border-border bg-card p-4 md:p-6">
        <div class="space-y-4">
          <div class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
            <div>
              <h2 class="text-lg font-semibold text-foreground">{{ t('admin.cardSecrets.listTitle') }}</h2>
              <p class="text-xs text-muted-foreground">{{ currentBatchFilterText }}</p>
            </div>
            <div class="grid w-full grid-cols-1 gap-3 md:grid-cols-4 lg:max-w-4xl">
              <Input
                v-model="filters.secret"
                :placeholder="t('admin.cardSecrets.filters.secretPlaceholder')"
                @keyup.enter="applyListFilters"
              />
              <Input
                v-model="filters.batchNo"
                :placeholder="t('admin.cardSecrets.filters.batchNoPlaceholder')"
                @keyup.enter="applyListFilters"
              />
              <Select v-model="filters.status">
                <SelectTrigger class="h-10">
                  <SelectValue :placeholder="t('admin.cardSecrets.statusAll')" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="__all__">{{ t('admin.cardSecrets.statusAll') }}</SelectItem>
                  <SelectItem value="available">{{ t('admin.cardSecrets.status.available') }}</SelectItem>
                  <SelectItem value="reserved">{{ t('admin.cardSecrets.status.reserved') }}</SelectItem>
                  <SelectItem value="used">{{ t('admin.cardSecrets.status.used') }}</SelectItem>
                </SelectContent>
              </Select>
              <div class="flex gap-2">
                <Button class="flex-1" variant="outline" @click="applyListFilters">{{ t('admin.common.search') }}</Button>
                <Button class="flex-1" variant="outline" @click="resetListFilters">{{ t('admin.common.reset') }}</Button>
              </div>
            </div>
          </div>

          <div v-if="currentBatchId" class="rounded-lg border border-primary/20 bg-primary/5 px-3 py-2 text-xs text-muted-foreground">
            <span>{{ currentBatchFilterText }}</span>
            <button
              type="button"
              class="ml-2 text-primary underline-offset-4 hover:underline"
              @click="clearBatchFilterAndRefresh"
            >
              {{ t('admin.cardSecrets.batchFilterClear') }}
            </button>
          </div>

          <div class="sticky top-3 z-10 rounded-xl border border-border bg-card/95 p-3 shadow-sm backdrop-blur">
            <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
              <div class="space-y-1">
                <p class="text-sm font-medium text-foreground">{{ t('admin.cardSecrets.batch.operationTitle') }}</p>
                <p class="text-xs text-muted-foreground">{{ scopeHint }}</p>
              </div>
              <div class="flex flex-col gap-2 xl:flex-row xl:items-center">
                <Select v-model="operationScope">
                  <SelectTrigger class="h-9 w-full text-xs xl:w-[180px]">
                    <SelectValue :placeholder="t('admin.cardSecrets.batch.scopePlaceholder')" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="selected">{{ t('admin.cardSecrets.batch.scopeSelected') }}</SelectItem>
                    <SelectItem value="filtered">{{ t('admin.cardSecrets.batch.scopeFiltered') }}</SelectItem>
                  </SelectContent>
                </Select>

                <Select v-model="batchStatusTarget">
                  <SelectTrigger class="h-9 w-full text-xs xl:w-[180px]">
                    <SelectValue :placeholder="t('admin.cardSecrets.batch.statusPlaceholder')" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="available">{{ t('admin.cardSecrets.status.available') }}</SelectItem>
                    <SelectItem value="reserved">{{ t('admin.cardSecrets.status.reserved') }}</SelectItem>
                    <SelectItem value="used">{{ t('admin.cardSecrets.status.used') }}</SelectItem>
                  </SelectContent>
                </Select>

                <div class="flex flex-wrap gap-2">
                  <Button size="sm" variant="outline" :disabled="batchActionLoading || !hasActionableScope" @click="applyBatchStatus">
                    {{ t('admin.cardSecrets.batch.applyStatus') }}
                  </Button>
                  <Button size="sm" variant="outline" :disabled="batchActionLoading || !hasActionableScope" @click="exportSecretsInScope('txt')">
                    {{ t('admin.cardSecrets.batch.exportTxt') }}
                  </Button>
                  <Button size="sm" variant="outline" :disabled="batchActionLoading || !hasActionableScope" @click="exportSecretsInScope('csv')">
                    {{ t('admin.cardSecrets.batch.exportCsv') }}
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    class="border-destructive/40 text-destructive hover:bg-destructive/10"
                    :disabled="batchActionLoading || !hasActionableScope"
                    @click="deleteSecretsInScope"
                  >
                    {{ t('admin.cardSecrets.batch.deleteSelected') }}
                  </Button>
                </div>
              </div>
            </div>

            <div class="mt-3 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
              <span class="rounded-full bg-muted px-2.5 py-1">
                {{ t('admin.cardSecrets.batch.scopeCount', { scope: currentScopeLabel, count: currentScopeCount }) }}
              </span>
              <span
                v-if="operationScope === 'selected' && selectedSecretCount > 0"
                class="rounded-full border border-primary/20 bg-primary/5 px-2.5 py-1 text-primary"
              >
                {{ t('admin.cardSecrets.batch.selectedCount', { count: selectedSecretCount }) }}
              </span>
            </div>

            <div v-if="batchActionError" class="mt-3 rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">
              {{ batchActionError }}
            </div>
            <div v-if="batchActionSuccess" class="mt-3 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700">
              {{ batchActionSuccess }}
            </div>
          </div>
        </div>

        <div class="mt-4 overflow-x-auto">
          <Table class="min-w-[1020px]">
            <TableHeader class="border-b border-border bg-muted/40 text-xs uppercase text-muted-foreground">
              <TableRow>
                <TableHead class="px-4 py-3">
                  <Checkbox :model-value="allCurrentPageSelected" @update:model-value="toggleSelectAllSecrets" />
                </TableHead>
                <TableHead class="px-4 py-3">{{ t('admin.cardSecrets.listTable.id') }}</TableHead>
                <TableHead class="min-w-[100px] px-4 py-3">{{ t('admin.cardSecrets.listTable.secret') }}</TableHead>
                <TableHead class="min-w-[100px] px-4 py-3">{{ t('admin.cardSecrets.listTable.product') }}</TableHead>
                <TableHead class="min-w-[100px] px-4 py-3">{{ t('admin.cardSecrets.listTable.sku') }}</TableHead>
                <TableHead class="min-w-[90px] px-4 py-3">{{ t('admin.cardSecrets.listTable.status') }}</TableHead>
                <TableHead class="min-w-[90px] px-4 py-3">{{ t('admin.cardSecrets.listTable.orderId') }}</TableHead>
                <TableHead class="min-w-[90px] px-4 py-3">{{ t('admin.cardSecrets.listTable.batchId') }}</TableHead>
                <TableHead class="min-w-[100px] px-4 py-3">{{ t('admin.cardSecrets.listTable.createdAt') }}</TableHead>
                <TableHead class="min-w-[100px] px-4 py-3 text-right">{{ t('admin.cardSecrets.listTable.action') }}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody class="divide-y divide-border">
              <TableRow v-if="cardSecretsLoading">
                <TableCell :colspan="10" class="p-0">
                  <TableSkeleton :columns="10" :rows="5" />
                </TableCell>
              </TableRow>
              <TableRow v-else-if="cardSecrets.length === 0">
                <TableCell colspan="10" class="px-4 py-6 text-center text-muted-foreground">{{ t('admin.cardSecrets.emptyList') }}</TableCell>
              </TableRow>
              <TableRow v-for="secret in cardSecrets" :key="secret.id" class="hover:bg-muted/30">
                <TableCell class="px-4 py-3">
                  <Checkbox :model-value="selectedSecretIds.includes(secret.id)" @update:model-value="(v) => toggleSecretSelected(secret.id, v)" />
                </TableCell>
                <TableCell class="px-4 py-3">
                  <IdCell :value="secret.id" />
                </TableCell>
                <TableCell class="min-w-[100px] break-all px-4 py-3 font-mono text-xs text-muted-foreground">{{ secret.secret }}</TableCell>
                <TableCell class="min-w-[100px] px-4 py-3 text-xs text-muted-foreground">
                  <a
                    v-if="secret.product_id"
                    :href="productLink(secret.product_id)"
                    target="_blank"
                    rel="noopener"
                    class="text-primary underline-offset-4 hover:underline"
                  >
                    <span class="break-words">#{{ secret.product_id }} {{ resolveProductName(secret.product_id) }}</span>
                  </a>
                  <span v-else class="text-muted-foreground">-</span>
                </TableCell>
                <TableCell class="min-w-[100px] px-4 py-3 text-xs text-muted-foreground">
                  <div class="break-words">{{ secretSkuLabel(secret) }}</div>
                </TableCell>
                <TableCell class="min-w-[90px] px-4 py-3 text-xs">
                  <div class="flex flex-wrap items-center gap-1">
                    <span class="inline-flex rounded-full border px-2.5 py-1 text-xs" :class="cardSecretStatusClass(secret.status)">
                      {{ cardSecretStatusLabel(secret.status) }}
                    </span>
                    <span
                      v-if="secret.is_loop"
                      class="inline-flex rounded-full border border-sky-200 bg-sky-50 px-2.5 py-1 text-xs text-sky-700"
                      :title="t('admin.cardSecrets.isLoopHint')"
                    >
                      {{ t('admin.cardSecrets.isLoopLabel') }}
                    </span>
                  </div>
                </TableCell>
                <TableCell class="min-w-[90px] px-4 py-3 text-xs">
                  <div class="flex flex-col gap-1">
                    <a
                      v-if="secret.order_id"
                      :href="orderLink(secret.order_id)"
                      target="_blank"
                      rel="noopener"
                      class="text-primary underline-offset-4 hover:underline"
                    >
                      #{{ secret.order_id }}
                    </a>
                    <span v-else class="text-muted-foreground">-</span>
                    <span v-if="secret.status === 'used'" class="text-[11px] text-muted-foreground">
                      {{ t('admin.cardSecrets.listTable.usedOrderHint') }}
                    </span>
                  </div>
                </TableCell>
                <TableCell class="min-w-[90px] px-4 py-3 text-xs text-muted-foreground">
                  <button
                    v-if="secret.batch_id"
                    type="button"
                    class="text-left text-primary underline-offset-4 hover:underline"
                    @click="secret.batch ? filterByBatch(secret.batch) : filterByBatchId(Number(secret.batch_id || 0))"
                  >
                    {{ resolveSecretBatchLabel(secret) }}
                  </button>
                  <span v-else>-</span>
                </TableCell>
                <TableCell class="min-w-[100px] px-4 py-3 text-xs text-muted-foreground">{{ formatDate(secret.created_at) }}</TableCell>
                <TableCell class="min-w-[100px] px-4 py-3 text-right">
                  <Button size="sm" variant="outline" @click="openEditSecret(secret)">{{ t('admin.cardSecrets.actions.edit') }}</Button>
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </div>

        <ListPagination
          :page="cardSecretPagination.page"
          :total-page="cardSecretPagination.total_page"
          :total="cardSecretPagination.total"
          :page-size="cardSecretPagination.page_size"
          :page-size-options="pageSizeOptions"
          @change-page="changeSecretPage"
          @change-page-size="changeCardSecretPageSize"
        />
      </div>

    <CardSecretEditModal
      v-model="showEditModal"
      :card-secret="editingCardSecret"
      @success="handleEditSuccess"
    />
  </div>
</template>
