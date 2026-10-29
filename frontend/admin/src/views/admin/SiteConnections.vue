<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AdminSiteConnection } from '@/api/types'
import IdCell from '@/components/IdCell.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Dialog, DialogScrollContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import TableSkeleton from '@/components/TableSkeleton.vue'
import ListPagination from '@/components/ListPagination.vue'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { getLocalizedText } from '@/utils/format'
import { confirmAction } from '@/utils/confirm'
import { notifyError, notifySuccess } from '@/utils/notify'
import { useFormValidation, rules } from '@/composables/useFormValidation'

const { t } = useI18n()
const loading = ref(true)
const connections = ref<AdminSiteConnection[]>([])
const pagination = reactive({
  page: 1,
  page_size: 20,
  total: 0,
  total_page: 1,
})
const showModal = ref(false)
const isEditing = ref(false)
const editingId = ref<number | null>(null)
const pingingId = ref<number | null>(null)

const form = reactive({
  name: '',
  base_url: '',
  api_key: '',
  api_secret: '',
  protocol: 'dujiao-next',
  callback_url: '',
  retry_max: 3,
  retry_intervals: '30,60,120',
  exchange_rate: 1,
  price_markup_percent: 0,
  price_rounding_mode: 'none',
  auto_sync_price: 'false',
})
const reapplyingId = ref<number | null>(null)

// 按上游分类加价：键为上游分类 ID，值为输入框内容（空表示沿用默认加价）
interface UpstreamCategoryItem {
  id: number
  parent_id: number
  name: Record<string, string>
  sort_order: number
}
const categoryMarkupInputs = reactive<Record<string, number | string>>({})
const upstreamCategories = ref<UpstreamCategoryItem[]>([])
const loadingUpstreamCategories = ref(false)
const upstreamCategoriesLoaded = ref(false)
const upstreamCategoriesSupported = ref(true)

const resetCategoryMarkups = (source?: Record<string, number> | null) => {
  for (const key of Object.keys(categoryMarkupInputs)) delete categoryMarkupInputs[key]
  for (const [key, value] of Object.entries(source || {})) categoryMarkupInputs[key] = value
  upstreamCategories.value = []
  upstreamCategoriesLoaded.value = false
  upstreamCategoriesSupported.value = true
}

// 按父子关系展开成带缩进层级的列表，成环或父分类缺失的分类按顶层处理
const categoryRows = computed(() => {
  const items = upstreamCategories.value
  const ids = new Set(items.map((c) => c.id))
  const childrenOf = new Map<number, UpstreamCategoryItem[]>()
  for (const c of items) {
    const parent = ids.has(c.parent_id) && c.parent_id !== c.id ? c.parent_id : 0
    const list = childrenOf.get(parent) || []
    list.push(c)
    childrenOf.set(parent, list)
  }
  const rows: { category: UpstreamCategoryItem; depth: number }[] = []
  const visited = new Set<number>()
  const walk = (parent: number, depth: number) => {
    const list = [...(childrenOf.get(parent) || [])].sort((a, b) => a.sort_order - b.sort_order || a.id - b.id)
    for (const c of list) {
      if (visited.has(c.id)) continue
      visited.add(c.id)
      rows.push({ category: c, depth })
      walk(c.id, depth + 1)
    }
  }
  walk(0, 0)
  return rows
})

const loadUpstreamCategories = async () => {
  if (!editingId.value) return
  loadingUpstreamCategories.value = true
  try {
    const res = await adminAPI.getUpstreamCategories({ connection_id: String(editingId.value) })
    const data = res.data.data as { supported?: boolean; categories?: UpstreamCategoryItem[] } | null
    upstreamCategoriesSupported.value = data?.supported ?? false
    upstreamCategories.value = data?.categories || []
    upstreamCategoriesLoaded.value = true
  } catch (err: any) {
    notifyError(err?.response?.data?.message || err?.message)
  } finally {
    loadingUpstreamCategories.value = false
  }
}

const siteConnectionSchema = {
  name: [rules.required()],
  base_url: [rules.required(), rules.url()],
  api_key: [rules.required()],
}
const { errors, validate, clearErrors } = useFormValidation(siteConnectionSchema)

const fetchConnections = async (page = 1) => {
  loading.value = true
  try {
    const res = await adminAPI.getSiteConnections({
      page,
      page_size: pagination.page_size,
    })
    connections.value = res.data.data || []
    const p = res.data.pagination
    if (p) {
      pagination.page = p.page
      pagination.page_size = p.page_size
      pagination.total = p.total
      pagination.total_page = p.total_page
    }
  } catch {
    connections.value = []
  } finally {
    loading.value = false
  }
}

const changePage = (page: number) => {
  if (page < 1 || page > pagination.total_page) return
  fetchConnections(page)
}

const pageSizeOptions = [10, 20, 50, 100]

const changePageSize = (size: number) => {
  if (size === pagination.page_size) return
  pagination.page_size = size
  fetchConnections(1)
}

const resetForm = () => {
  Object.assign(form, {
    name: '',
    base_url: '',
    api_key: '',
    api_secret: '',
    protocol: 'dujiao-next',
    callback_url: '',
    retry_max: 3,
    retry_intervals: '30,60,120',
    exchange_rate: 1,
    price_markup_percent: 0,
    price_rounding_mode: 'none',
    auto_sync_price: 'false',
  })
  resetCategoryMarkups()
}

const openCreateModal = () => {
  isEditing.value = false
  editingId.value = null
  clearErrors()
  resetForm()
  showModal.value = true
}

const openEditModal = (conn: AdminSiteConnection) => {
  isEditing.value = true
  editingId.value = conn.id
  Object.assign(form, {
    name: conn.name || '',
    base_url: conn.base_url || '',
    api_key: conn.api_key || '',
    api_secret: conn.api_secret || '',
    protocol: conn.protocol || 'dujiao-next',
    callback_url: conn.callback_url || '',
    retry_max: conn.retry_max ?? 3,
    retry_intervals: (() => {
      const ri = conn.retry_intervals
      if (Array.isArray(ri)) return ri.join(',')
      if (typeof ri === 'string') {
        try {
          const parsed = JSON.parse(ri)
          if (Array.isArray(parsed)) return parsed.join(',')
        } catch { /* ignore */ }
        return ri
      }
      return '30,60,120'
    })(),
    exchange_rate: conn.exchange_rate ?? 1,
    price_markup_percent: conn.price_markup_percent ?? 0,
    price_rounding_mode: conn.price_rounding_mode || 'none',
    auto_sync_price: conn.auto_sync_price ? 'true' : 'false',
  })
  resetCategoryMarkups(conn.category_markups)
  showModal.value = true
}

const closeModal = () => {
  showModal.value = false
  clearErrors()
}

const buildPayload = () => {
  const intervals = form.retry_intervals
    .split(',')
    .map((s: string) => Number(s.trim()))
    .filter((n: number) => !Number.isNaN(n) && n > 0)
  return {
    name: form.name,
    base_url: form.base_url,
    api_key: form.api_key,
    api_secret: form.api_secret,
    protocol: form.protocol,
    callback_url: form.callback_url,
    retry_max: Number(form.retry_max) || 3,
    retry_intervals: JSON.stringify(intervals),
    exchange_rate: Number(form.exchange_rate) || 1,
    price_markup_percent: Number(form.price_markup_percent) || 0,
    price_rounding_mode: form.price_rounding_mode,
    auto_sync_price: form.auto_sync_price === 'true',
    category_markups: buildCategoryMarkups(),
  }
}

// 空输入表示沿用默认加价，不写入；始终提交对象（可为空）以便清空已有配置
const buildCategoryMarkups = () => {
  const result: Record<string, number> = {}
  for (const [key, value] of Object.entries(categoryMarkupInputs)) {
    if (value === '' || value === null || value === undefined) continue
    const n = Number(value)
    if (Number.isFinite(n) && n >= 0) result[key] = n
  }
  return result
}

const handleSubmit = async () => {
  if (!validate({ ...form } as Record<string, unknown>)) return
  try {
    const payload = buildPayload()
    if (isEditing.value && editingId.value) {
      await adminAPI.updateSiteConnection(editingId.value, payload)
    } else {
      await adminAPI.createSiteConnection(payload)
    }
    closeModal()
    fetchConnections(isEditing.value ? pagination.page : 1)
    notifySuccess()
  } catch (err: any) {
    notifyError(err?.response?.data?.message || err?.message)
  }
}

const handlePing = async (conn: AdminSiteConnection) => {
  pingingId.value = conn.id
  try {
    await adminAPI.pingSiteConnection(conn.id)
    notifySuccess(t('siteConnections.ping.success'))
    fetchConnections(pagination.page)
  } catch (err: any) {
    notifyError(t('siteConnections.ping.failed') + ': ' + (err?.response?.data?.message || err?.message || ''))
  } finally {
    pingingId.value = null
  }
}

const handleToggleStatus = async (conn: AdminSiteConnection) => {
  const nextStatus = conn.status === 'active' ? 'disabled' : 'active'
  try {
    await adminAPI.updateSiteConnectionStatus(conn.id, { status: nextStatus })
    fetchConnections(pagination.page)
    notifySuccess()
  } catch (err: any) {
    notifyError(err?.response?.data?.message || err?.message)
  }
}

const handleDelete = async (conn: AdminSiteConnection) => {
  const confirmed = await confirmAction({
    description: t('siteConnections.delete.confirm', { name: conn.name || '#' + conn.id }),
    confirmText: t('admin.common.delete'),
    variant: 'destructive',
  })
  if (!confirmed) return
  try {
    await adminAPI.deleteSiteConnection(conn.id)
    fetchConnections(pagination.page)
    notifySuccess()
  } catch (err: any) {
    notifyError(err?.response?.data?.message || err?.message)
  }
}

const handleReapplyMarkup = async (conn: AdminSiteConnection) => {
  const confirmed = await confirmAction({
    description: t('siteConnections.reapplyMarkup.confirm', { name: conn.name || '#' + conn.id }),
    confirmText: t('siteConnections.actions.reapplyMarkup'),
  })
  if (!confirmed) return
  reapplyingId.value = conn.id
  try {
    const res = await adminAPI.reapplyConnectionMarkup(conn.id)
    const count = (res.data?.data as Record<string, number>)?.updated_products ?? 0
    notifySuccess(t('siteConnections.reapplyMarkup.success', { count }))
  } catch (err: any) {
    notifyError(err?.response?.data?.message || err?.message)
  } finally {
    reapplyingId.value = null
  }
}

const statusBadgeClass = (status?: string) => {
  switch (status) {
    case 'active':
      return 'text-emerald-700 border-emerald-200 bg-emerald-50'
    case 'pending':
      return 'text-yellow-700 border-yellow-200 bg-yellow-50'
    default:
      return 'text-muted-foreground border-border bg-muted/30'
  }
}

const statusLabel = (status?: string) => {
  const key = `siteConnections.status.${status}`
  const translated = t(key)
  return translated !== key ? translated : status
}

const formatTime = (raw?: string) => {
  if (!raw) return '-'
  const d = new Date(raw)
  if (Number.isNaN(d.getTime())) return '-'
  return d.toLocaleString()
}

onMounted(() => {
  fetchConnections()
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <h1 class="text-2xl font-semibold">{{ t('siteConnections.title') }}</h1>
      <Button class="w-full sm:w-auto" @click="openCreateModal">{{ t('siteConnections.createButton') }}</Button>
    </div>

    <div class="rounded-xl border border-border bg-card overflow-x-auto">
      <Table class="min-w-[900px]">
        <TableHeader class="border-b border-border bg-muted/40 text-xs uppercase text-muted-foreground">
          <TableRow>
            <TableHead class="px-6 py-3">{{ t('siteConnections.columns.id') }}</TableHead>
            <TableHead class="min-w-[160px] px-6 py-3">{{ t('siteConnections.columns.name') }}</TableHead>
            <TableHead class="min-w-[200px] px-6 py-3">{{ t('siteConnections.columns.baseUrl') }}</TableHead>
            <TableHead class="min-w-[80px] px-6 py-3">{{ t('siteConnections.columns.protocol') }}</TableHead>
            <TableHead class="min-w-[80px] px-6 py-3">{{ t('siteConnections.columns.markup') }}</TableHead>
            <TableHead class="min-w-[90px] px-6 py-3">{{ t('siteConnections.columns.status') }}</TableHead>
            <TableHead class="min-w-[80px] px-6 py-3">{{ t('siteConnections.columns.lastPing') }}</TableHead>
            <TableHead class="min-w-[200px] px-6 py-3 text-right">{{ t('siteConnections.columns.actions') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody class="divide-y divide-border">
          <TableRow v-if="loading">
            <TableCell :colspan="8" class="p-0">
              <TableSkeleton :columns="8" :rows="5" />
            </TableCell>
          </TableRow>
          <TableRow v-else-if="connections.length === 0">
            <TableCell colspan="8" class="px-6 py-8 text-center text-muted-foreground">{{ t('siteConnections.empty') }}</TableCell>
          </TableRow>
          <TableRow v-for="conn in connections" :key="conn.id" class="hover:bg-muted/30">
            <TableCell class="px-6 py-4">
              <IdCell :value="conn.id" />
            </TableCell>
            <TableCell class="min-w-[160px] px-6 py-4 font-medium text-foreground break-words">{{ conn.name }}</TableCell>
            <TableCell class="min-w-[200px] px-6 py-4 text-xs text-muted-foreground font-mono break-all">{{ conn.base_url }}</TableCell>
            <TableCell class="min-w-[80px] px-6 py-4 text-xs text-muted-foreground break-words">{{ conn.protocol }}</TableCell>
            <TableCell class="min-w-[80px] px-6 py-4 text-xs text-muted-foreground">
              <span v-if="conn.price_markup_percent && Number(conn.price_markup_percent) !== 0" class="inline-flex rounded-full border border-blue-200 bg-blue-50 px-2 py-0.5 text-xs text-blue-700">
                +{{ conn.price_markup_percent }}%
              </span>
              <span v-else class="text-muted-foreground">-</span>
            </TableCell>
            <TableCell class="min-w-[90px] px-6 py-4">
              <span
                class="inline-flex rounded-full border px-2.5 py-1 text-xs"
                :class="statusBadgeClass(conn.status)"
              >
                {{ statusLabel(conn.status) }}
              </span>
            </TableCell>
            <TableCell class="min-w-[80px] px-6 py-4 text-xs text-muted-foreground">{{ formatTime(conn.last_ping_at) }}</TableCell>
            <TableCell class="min-w-[160px] px-6 py-4 text-right">
              <div class="flex flex-wrap items-center justify-end gap-2">
                <Button size="sm" variant="outline" @click="openEditModal(conn)">{{ t('admin.common.edit') }}</Button>
                <Button
                  size="sm"
                  variant="outline"
                  :disabled="pingingId === conn.id"
                  @click="handlePing(conn)"
                >
                  {{ pingingId === conn.id ? t('siteConnections.ping.loading') : 'Ping' }}
                </Button>
                <Button
                  v-if="(conn.price_markup_percent && Number(conn.price_markup_percent) !== 0) || (conn.exchange_rate != null && Number(conn.exchange_rate) !== 1)"
                  size="sm"
                  variant="outline"
                  :disabled="reapplyingId === conn.id"
                  @click="handleReapplyMarkup(conn)"
                >
                  {{ reapplyingId === conn.id ? '...' : t('siteConnections.actions.reapplyMarkup') }}
                </Button>
                <Button size="sm" variant="outline" @click="handleToggleStatus(conn)">
                  {{ conn.status === 'active' ? t('siteConnections.actions.disable') : t('siteConnections.actions.enable') }}
                </Button>
                <Button size="sm" variant="destructive" @click="handleDelete(conn)">{{ t('admin.common.delete') }}</Button>
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

    <Dialog v-model:open="showModal" @update:open="(value: boolean) => { if (!value) closeModal() }">
      <DialogScrollContent class="w-[calc(100vw-1rem)] max-w-2xl p-4 sm:p-6" @interact-outside="(e: Event) => e.preventDefault()">
        <DialogHeader>
          <DialogTitle>{{ isEditing ? t('siteConnections.editTitle') : t('siteConnections.createTitle') }}</DialogTitle>
        </DialogHeader>

        <form class="space-y-6" @submit.prevent="handleSubmit">
          <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
            <div class="md:col-span-2">
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('siteConnections.form.name') }}</label>
              <Input v-model="form.name" required :placeholder="t('siteConnections.form.namePlaceholder')" />
              <p v-if="errors.name" class="text-xs text-destructive mt-1">{{ errors.name }}</p>
            </div>
            <div class="md:col-span-2">
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('siteConnections.form.baseUrl') }}</label>
              <Input v-model="form.base_url" required :placeholder="t('siteConnections.form.baseUrlPlaceholder')" />
              <p v-if="errors.base_url" class="text-xs text-destructive mt-1">{{ errors.base_url }}</p>
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('siteConnections.form.apiKey') }}</label>
              <Input v-model="form.api_key" :placeholder="t('siteConnections.form.apiKeyPlaceholder')" />
              <p v-if="errors.api_key" class="text-xs text-destructive mt-1">{{ errors.api_key }}</p>
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('siteConnections.form.apiSecret') }}</label>
              <Input v-model="form.api_secret" type="password" :placeholder="t('siteConnections.form.apiSecretPlaceholder')" />
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('siteConnections.form.protocol') }}</label>
              <Select v-model="form.protocol">
                <SelectTrigger class="h-9 w-full">
                  <SelectValue :placeholder="t('siteConnections.form.protocolPlaceholder')" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="dujiao-next">dujiao-next</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('siteConnections.form.callbackUrl') }}</label>
              <Input v-model="form.callback_url" :placeholder="t('siteConnections.form.callbackUrlPlaceholder')" />
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('siteConnections.form.retryMax') }}</label>
              <Input v-model.number="form.retry_max" type="number" min="0" placeholder="3" />
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('siteConnections.form.retryIntervals') }}</label>
              <Input v-model="form.retry_intervals" placeholder="30,60,120" />
            </div>
          </div>

          <div class="border-t border-border pt-4">
            <h3 class="mb-3 text-sm font-medium text-foreground">{{ t('siteConnections.form.markupSection') }}</h3>
            <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
              <div>
                <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('siteConnections.form.exchangeRate') }}</label>
                <Input v-model.number="form.exchange_rate" type="number" step="0.000001" min="0" placeholder="1" />
                <p class="mt-1 text-xs text-muted-foreground">{{ t('siteConnections.form.exchangeRateHint') }}</p>
              </div>
              <div>
                <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('siteConnections.form.priceMarkupPercent') }}</label>
                <div class="relative">
                  <Input v-model.number="form.price_markup_percent" type="number" step="0.01" min="0" placeholder="0" />
                  <span class="absolute right-3 top-1/2 -translate-y-1/2 text-xs text-muted-foreground">%</span>
                </div>
                <p class="mt-1 text-xs text-muted-foreground">{{ t('siteConnections.form.priceMarkupPercentHint') }}</p>
              </div>
              <div>
                <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('siteConnections.form.priceRoundingMode') }}</label>
                <Select v-model="form.price_rounding_mode">
                  <SelectTrigger class="h-9 w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="none">{{ t('siteConnections.form.roundingNone') }}</SelectItem>
                    <SelectItem value="ceil_int">{{ t('siteConnections.form.roundingCeilInt') }}</SelectItem>
                    <SelectItem value="ceil_tenth">{{ t('siteConnections.form.roundingCeilTenth') }}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div>
                <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('siteConnections.form.autoSyncPrice') }}</label>
                <Select v-model="form.auto_sync_price">
                  <SelectTrigger class="h-9 w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="false">{{ t('admin.common.no') }}</SelectItem>
                    <SelectItem value="true">{{ t('admin.common.yes') }}</SelectItem>
                  </SelectContent>
                </Select>
                <p class="mt-1 text-xs text-muted-foreground">{{ t('siteConnections.form.autoSyncPriceHint') }}</p>
              </div>
            </div>

            <div class="mt-4">
              <div class="flex items-center justify-between gap-2">
                <label class="block text-xs font-medium text-muted-foreground">{{ t('siteConnections.form.categoryMarkups') }}</label>
                <Button
                  v-if="isEditing"
                  type="button"
                  variant="outline"
                  size="sm"
                  class="h-7 text-xs"
                  :disabled="loadingUpstreamCategories"
                  @click="loadUpstreamCategories"
                >{{ upstreamCategoriesLoaded ? t('siteConnections.form.categoryMarkupsReload') : t('siteConnections.form.categoryMarkupsLoad') }}</Button>
              </div>
              <p class="mt-1 text-xs text-muted-foreground">{{ t('siteConnections.form.categoryMarkupsHint') }}</p>
              <p v-if="!isEditing" class="mt-2 text-xs text-amber-600">{{ t('siteConnections.form.categoryMarkupsSaveFirst') }}</p>
              <p v-else-if="upstreamCategoriesLoaded && !upstreamCategoriesSupported" class="mt-2 text-xs text-amber-600">{{ t('siteConnections.form.categoryMarkupsUnsupported') }}</p>
              <p v-else-if="upstreamCategoriesLoaded && categoryRows.length === 0" class="mt-2 text-xs text-muted-foreground">{{ t('siteConnections.form.categoryMarkupsEmpty') }}</p>
              <div v-else-if="categoryRows.length > 0" class="mt-2 max-h-64 divide-y divide-border overflow-y-auto rounded-lg border border-border">
                <div v-for="row in categoryRows" :key="row.category.id" class="flex items-center gap-3 px-3 py-2">
                  <span class="min-w-0 flex-1 truncate text-sm text-foreground" :style="{ paddingLeft: `${row.depth * 16}px` }">{{ getLocalizedText(row.category.name) }}</span>
                  <div class="relative w-32 shrink-0">
                    <Input
                      v-model.number="categoryMarkupInputs[String(row.category.id)]"
                      type="number"
                      step="0.01"
                      min="0"
                      :placeholder="t('siteConnections.form.categoryMarkupsDefaultPlaceholder')"
                    />
                    <span class="absolute right-3 top-1/2 -translate-y-1/2 text-xs text-muted-foreground">%</span>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div class="flex flex-col-reverse gap-3 border-t border-border pt-6 sm:flex-row sm:justify-end">
            <Button type="button" variant="outline" class="w-full sm:w-auto" @click="closeModal">{{ t('admin.common.cancel') }}</Button>
            <Button type="submit" class="w-full sm:w-auto">{{ isEditing ? t('admin.common.save') : t('admin.common.create') }}</Button>
          </div>
        </form>
      </DialogScrollContent>
    </Dialog>
  </div>
</template>
