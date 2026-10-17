<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import { CircleHelp, Search, X } from 'lucide-vue-next'
import { adminAPI } from '@/api/admin'
import type { AdminProduct, AdminUser, AdminUserProductPrice } from '@/api/types'
import IdCell from '@/components/IdCell.vue'
import ListPagination from '@/components/ListPagination.vue'
import TableSkeleton from '@/components/TableSkeleton.vue'
import { Button } from '@/components/ui/button'
import { Dialog, DialogHeader, DialogScrollContent, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { formatMoney, getLocalizedText } from '@/utils/format'
import { notifyError, notifySuccess } from '@/utils/notify'
import { confirmAction } from '@/utils/confirm'
import { formatWholesaleSkuLabel, listActiveWholesaleSkus } from '@/utils/wholesalePricing'

const { t, locale } = useI18n()

const siteCurrency = ref('CNY')
const formatPrice = (amount: number | string) => formatMoney(amount, siteCurrency.value)
const productName = (product?: AdminProduct | null) =>
  product ? getLocalizedText(product.title || {}) || `#${product.id}` : '-'

// --- 用户选择 ---
const userKeyword = ref('')
const userResults = ref<AdminUser[]>([])
const userSearching = ref(false)
const selectedUser = ref<AdminUser | null>(null)

const searchUsers = async () => {
  const keyword = userKeyword.value.trim()
  if (!keyword) {
    userResults.value = []
    return
  }
  userSearching.value = true
  try {
    const res = await adminAPI.getUsers({ page: 1, page_size: 8, keyword })
    userResults.value = res.data.data || []
  } catch {
    userResults.value = []
  } finally {
    userSearching.value = false
  }
}
const debouncedSearchUsers = useDebounceFn(searchUsers, 300)

const userLabel = (user: AdminUser) => `#${user.id} ${user.email}${user.display_name ? ` (${user.display_name})` : ''}`

const pickUser = (user: AdminUser) => {
  selectedUser.value = user
  userKeyword.value = ''
  userResults.value = []
  pagination.page = 1
  fetchPrices()
}

const clearUser = () => {
  selectedUser.value = null
  prices.value = []
  pagination.total = 0
}

// --- 专属价列表 ---
const loading = ref(false)
const prices = ref<AdminUserProductPrice[]>([])
const productCache = reactive<Record<number, AdminProduct | null>>({})
const pagination = reactive({ page: 1, page_size: 20, total: 0, total_page: 0 })
const pageSizeOptions = [10, 20, 50, 100]

const loadProducts = async (ids: number[]) => {
  const missing = Array.from(new Set(ids)).filter((id) => !(id in productCache))
  await Promise.all(
    missing.map(async (id) => {
      try {
        const res = await adminAPI.getProduct(id)
        productCache[id] = (res.data.data as AdminProduct) || null
      } catch {
        productCache[id] = null
      }
    }),
  )
}

const fetchPrices = async () => {
  if (!selectedUser.value) return
  loading.value = true
  try {
    const res = await adminAPI.getUserProductPrices({
      user_id: selectedUser.value.id,
      page: pagination.page,
      page_size: pagination.page_size,
    })
    prices.value = res.data.data || []
    if (res.data.pagination) Object.assign(pagination, res.data.pagination)
    await loadProducts(prices.value.map((p) => p.product_id))
  } catch {
    prices.value = []
  } finally {
    loading.value = false
  }
}

const changePage = (page: number) => {
  if (page < 1 || page > pagination.total_page) return
  pagination.page = page
  fetchPrices()
}

const changePageSize = (size: number) => {
  if (size === pagination.page_size) return
  pagination.page_size = size
  pagination.page = 1
  fetchPrices()
}

const skuLabelOf = (row: AdminUserProductPrice) => {
  if (!row.sku_id) return t('admin.userPrices.skuAll')
  const sku = productCache[row.product_id]?.skus?.find((s) => s.id === row.sku_id)
  return sku ? formatWholesaleSkuLabel(sku, String(locale.value || 'zh-CN')) : `#${row.sku_id}`
}

const basePriceOf = (row: AdminUserProductPrice) => {
  const product = productCache[row.product_id]
  if (!product) return '-'
  if (row.sku_id) {
    const sku = product.skus?.find((s) => s.id === row.sku_id)
    if (sku) return formatPrice(sku.price_amount)
  }
  return formatPrice(product.price_amount)
}

const removePrice = async (row: AdminUserProductPrice) => {
  const confirmed = await confirmAction({
    description: t('admin.userPrices.confirmDelete'),
    confirmText: t('admin.common.delete'),
    variant: 'destructive',
  })
  if (!confirmed) return
  try {
    await adminAPI.deleteUserProductPrice(row.id)
    notifySuccess(t('admin.userPrices.deleteSuccess'))
    await fetchPrices()
  } catch (err: any) {
    notifyError(err?.response?.data?.message || err?.message)
  }
}

// --- 新增 / 修改弹窗 ---
const showModal = ref(false)
const submitting = ref(false)
const productKeyword = ref('')
const productResults = ref<AdminProduct[]>([])
const productSearching = ref(false)
const editingProduct = ref<AdminProduct | null>(null)
const form = reactive({ sku_id: '0', price: '' as number | string })

const activeSkus = computed(() => (editingProduct.value ? listActiveWholesaleSkus(editingProduct.value) : []))

const searchProducts = async () => {
  const keyword = productKeyword.value.trim()
  if (!keyword) {
    productResults.value = []
    return
  }
  productSearching.value = true
  try {
    const res = await adminAPI.getProducts({ page: 1, page_size: 8, search: keyword })
    productResults.value = res.data.data || []
  } catch {
    productResults.value = []
  } finally {
    productSearching.value = false
  }
}
const debouncedSearchProducts = useDebounceFn(searchProducts, 300)

const openModal = () => {
  productKeyword.value = ''
  productResults.value = []
  editingProduct.value = null
  form.sku_id = '0'
  form.price = ''
  showModal.value = true
}

const openEdit = async (row: AdminUserProductPrice) => {
  await loadProducts([row.product_id])
  const product = productCache[row.product_id]
  if (!product) return
  productKeyword.value = ''
  productResults.value = []
  editingProduct.value = product
  form.sku_id = String(row.sku_id || 0)
  form.price = Number(row.price_amount)
  showModal.value = true
}

const pickProduct = (product: AdminProduct) => {
  editingProduct.value = product
  productCache[product.id] = product
  productResults.value = []
  productKeyword.value = ''
  form.sku_id = '0'
}

const selectedBasePrice = computed(() => {
  const product = editingProduct.value
  if (!product) return null
  if (form.sku_id !== '0') {
    const sku = product.skus?.find((s) => String(s.id) === form.sku_id)
    if (sku) return sku.price_amount
  }
  return product.price_amount
})

const savePrice = async () => {
  if (!selectedUser.value || !editingProduct.value) return
  const price = Number(form.price)
  if (!Number.isFinite(price) || price <= 0) {
    notifyError(t('admin.userPrices.errors.invalidPrice'))
    return
  }
  submitting.value = true
  try {
    await adminAPI.batchUpsertUserProductPrices({
      prices: [
        {
          user_id: selectedUser.value.id,
          product_id: editingProduct.value.id,
          sku_id: Number(form.sku_id) || 0,
          price_amount: price,
        },
      ],
    })
    notifySuccess(t('admin.userPrices.saveSuccess'))
    showModal.value = false
    await fetchPrices()
  } catch (err: any) {
    notifyError(err?.response?.data?.message || err?.message)
  } finally {
    submitting.value = false
  }
}

const fetchSiteCurrency = async () => {
  try {
    const res = await adminAPI.getSettings({ key: 'site_config' })
    const raw = String((res.data?.data as Record<string, unknown> | undefined)?.currency || 'CNY').trim().toUpperCase()
    siteCurrency.value = /^[A-Z]{3}$/.test(raw) ? raw : 'CNY'
  } catch {
    siteCurrency.value = 'CNY'
  }
}
fetchSiteCurrency()

const ruleKeys = ['admin.userPrices.ruleExclusive', 'admin.userPrices.ruleLowerOnly', 'admin.userPrices.ruleSkuFirst']
</script>

<template>
  <div class="space-y-6">
    <div>
      <h1 class="text-2xl font-semibold">{{ t('admin.userPrices.title') }}</h1>
      <p class="mt-1 text-sm text-muted-foreground">{{ t('admin.userPrices.subtitle') }}</p>
    </div>

    <div class="rounded-xl border border-border bg-card p-4 shadow-sm space-y-3">
      <div class="text-sm font-medium">{{ t('admin.userPrices.selectUser') }}</div>
      <div v-if="selectedUser" class="flex items-center justify-between rounded-lg border border-border bg-muted/30 px-4 py-3">
        <span class="text-sm">{{ userLabel(selectedUser) }}</span>
        <Button size="sm" variant="outline" @click="clearUser">
          <X class="mr-1 h-3.5 w-3.5" />{{ t('admin.userPrices.changeUser') }}
        </Button>
      </div>
      <div v-else class="relative max-w-md">
        <Search class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          v-model="userKeyword"
          class="pl-9"
          :placeholder="t('admin.userPrices.userSearchPlaceholder')"
          @input="debouncedSearchUsers"
          @keyup.enter="searchUsers"
        />
        <div v-if="userResults.length" class="absolute z-10 mt-1 w-full overflow-hidden rounded-lg border border-border bg-popover shadow-md">
          <button
            v-for="user in userResults"
            :key="user.id"
            type="button"
            class="block w-full px-4 py-2 text-left text-sm hover:bg-muted"
            @click="pickUser(user)"
          >
            {{ userLabel(user) }}
          </button>
        </div>
        <div v-else-if="userKeyword.trim() && !userSearching" class="mt-1 text-xs text-muted-foreground">
          {{ t('admin.userPrices.userNotFound') }}
        </div>
      </div>
    </div>

    <template v-if="selectedUser">
      <div class="rounded-lg border border-sky-200 bg-sky-50/70 p-4 text-sky-950 dark:border-sky-900/60 dark:bg-sky-950/25 dark:text-sky-100">
        <div class="flex items-start gap-3">
          <CircleHelp class="mt-0.5 h-4 w-4 shrink-0 text-sky-600 dark:text-sky-300" />
          <div class="min-w-0">
            <div class="text-sm font-medium">{{ t('admin.userPrices.ruleTitle') }}</div>
            <ul class="mt-2 space-y-1 text-xs leading-5 text-sky-800 dark:text-sky-200/90">
              <li v-for="key in ruleKeys" :key="key">{{ t(key) }}</li>
            </ul>
          </div>
        </div>
      </div>

      <div class="flex justify-end">
        <Button @click="openModal">{{ t('admin.userPrices.add') }}</Button>
      </div>

      <div class="rounded-xl border border-border bg-card overflow-x-auto">
        <Table class="min-w-[800px]">
          <TableHeader class="border-b border-border bg-muted/40 text-xs uppercase text-muted-foreground">
            <TableRow>
              <TableHead class="px-6 py-3">{{ t('admin.userPrices.table.id') }}</TableHead>
              <TableHead class="min-w-[260px] px-6 py-3">{{ t('admin.userPrices.table.product') }}</TableHead>
              <TableHead class="px-6 py-3">{{ t('admin.userPrices.table.sku') }}</TableHead>
              <TableHead class="px-6 py-3">{{ t('admin.userPrices.table.basePrice') }}</TableHead>
              <TableHead class="px-6 py-3">{{ t('admin.userPrices.table.userPrice') }}</TableHead>
              <TableHead class="px-6 py-3 text-right">{{ t('admin.userPrices.table.action') }}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody class="divide-y divide-border">
            <TableRow v-if="loading">
              <TableCell :colspan="6" class="p-0"><TableSkeleton :columns="6" :rows="4" /></TableCell>
            </TableRow>
            <TableRow v-else-if="prices.length === 0">
              <TableCell colspan="6" class="px-6 py-8 text-center text-muted-foreground">{{ t('admin.userPrices.empty') }}</TableCell>
            </TableRow>
            <TableRow v-for="row in prices" v-else :key="row.id" class="hover:bg-muted/30">
              <TableCell class="px-6 py-4"><IdCell :value="row.id" /></TableCell>
              <TableCell class="px-6 py-4">
                <div class="font-medium">{{ productName(productCache[row.product_id]) }}</div>
                <div class="font-mono text-xs text-muted-foreground">#{{ row.product_id }}</div>
              </TableCell>
              <TableCell class="px-6 py-4">{{ skuLabelOf(row) }}</TableCell>
              <TableCell class="px-6 py-4 font-mono text-muted-foreground">{{ basePriceOf(row) }}</TableCell>
              <TableCell class="px-6 py-4 font-mono font-medium text-emerald-600 dark:text-emerald-400">{{ formatPrice(row.price_amount) }}</TableCell>
              <TableCell class="px-6 py-4 text-right">
                <div class="flex justify-end gap-2">
                  <Button size="sm" variant="outline" @click="openEdit(row)">{{ t('admin.userPrices.edit') }}</Button>
                  <Button size="sm" variant="destructive" @click="removePrice(row)">{{ t('admin.common.delete') }}</Button>
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
    </template>

    <Dialog v-model:open="showModal">
      <DialogScrollContent class="max-w-xl">
        <DialogHeader>
          <DialogTitle>{{ t('admin.userPrices.modalTitle') }}</DialogTitle>
        </DialogHeader>
        <div class="space-y-4">
          <div v-if="!editingProduct" class="relative">
            <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('admin.userPrices.searchProduct') }}</label>
            <Input
              v-model="productKeyword"
              :placeholder="t('admin.userPrices.productSearchPlaceholder')"
              @input="debouncedSearchProducts"
              @keyup.enter="searchProducts"
            />
            <div v-if="productResults.length" class="mt-1 overflow-hidden rounded-lg border border-border">
              <button
                v-for="product in productResults"
                :key="product.id"
                type="button"
                class="flex w-full items-center justify-between px-4 py-2 text-left text-sm hover:bg-muted"
                @click="pickProduct(product)"
              >
                <span class="truncate">#{{ product.id }} {{ productName(product) }}</span>
                <span class="ml-3 shrink-0 font-mono text-xs text-muted-foreground">{{ formatPrice(product.price_amount) }}</span>
              </button>
            </div>
            <div v-else-if="productKeyword.trim() && !productSearching" class="mt-1 text-xs text-muted-foreground">
              {{ t('admin.userPrices.productNotFound') }}
            </div>
          </div>

          <template v-else>
            <div class="flex items-center justify-between rounded-lg border border-border bg-muted/30 px-4 py-3 text-sm">
              <span class="truncate">#{{ editingProduct.id }} {{ productName(editingProduct) }}</span>
              <Button size="sm" variant="outline" @click="editingProduct = null">{{ t('admin.userPrices.changeProduct') }}</Button>
            </div>

            <div>
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('admin.userPrices.skuScope') }}</label>
              <Select v-model="form.sku_id">
                <SelectTrigger class="h-9 w-full"><SelectValue :placeholder="t('admin.userPrices.skuAll')" /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="0">{{ t('admin.userPrices.skuAll') }}</SelectItem>
                  <SelectItem v-for="sku in activeSkus" :key="sku.id" :value="String(sku.id)">
                    {{ formatWholesaleSkuLabel(sku, String(locale || 'zh-CN')) }}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div>
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">
                {{ t('admin.userPrices.userPrice') }}
                <span v-if="selectedBasePrice !== null" class="ml-2 font-normal">
                  {{ t('admin.userPrices.currentBase', { price: formatPrice(selectedBasePrice) }) }}
                </span>
              </label>
              <Input v-model.number="form.price" type="number" min="0" step="0.01" :placeholder="t('admin.userPrices.pricePlaceholder')" />
            </div>
          </template>

          <div class="flex justify-end gap-2">
            <Button type="button" variant="outline" @click="showModal = false">{{ t('admin.common.cancel') }}</Button>
            <Button type="button" :disabled="submitting || !editingProduct" @click="savePrice">
              {{ submitting ? t('admin.userPrices.saving') : t('admin.userPrices.save') }}
            </Button>
          </div>
        </div>
      </DialogScrollContent>
    </Dialog>
  </div>
</template>
