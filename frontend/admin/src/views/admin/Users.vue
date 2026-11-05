<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AdminUser, AdminMemberLevel } from '@/api/types'
import IdCell from '@/components/IdCell.vue'
import { userStatusClass, userStatusLabel } from '@/utils/status'
import { formatDate, formatMoney, getLocalizedText, toRFC3339 } from '@/utils/format'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { toggleArrayMember } from '@/lib/utils'
import { Textarea } from '@/components/ui/textarea'
import { Dialog, DialogHeader, DialogScrollContent, DialogTitle } from '@/components/ui/dialog'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import TableSkeleton from '@/components/TableSkeleton.vue'
import ListPagination from '@/components/ListPagination.vue'
import { useListRefresh, type ListFetchOptions } from '@/composables/useListRefresh'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { confirmAction } from '@/utils/confirm'
import { useFormValidation, rules } from '@/composables/useFormValidation'
import { ArrowDown, ArrowUp, ChevronsUpDown } from 'lucide-vue-next'

const { t } = useI18n()
const loading = ref(true)
const { refreshing, refreshList } = useListRefresh()
const users = ref<AdminUser[]>([])
const selectedIds = ref<number[]>([])
const pagination = ref({
  page: 1,
  page_size: 20,
  total: 0,
  total_page: 1,
})
const filters = reactive({
  userId: '',
  keyword: '',
  status: '__all__',
  createdFrom: '',
  createdTo: '',
  lastLoginFrom: '',
  lastLoginTo: '',
})
const normalizeFilterValue = (value: string) => (value === '__all__' ? '' : value)

// 可排序列 + 三态排序状态（by 为空 = 默认 id 倒序）
type SortColumn = 'wallet_balance' | 'created_at' | 'last_login_at'
type SortDirection = '' | 'asc' | 'desc'
const sort = reactive<{ by: SortColumn | ''; order: SortDirection }>({ by: '', order: '' })

// 三态循环：无 → 降序 → 升序 → 无；点击其它列直接切到该列降序
const nextSortState = (
  current: { by: SortColumn | ''; order: SortDirection },
  column: SortColumn,
): { by: SortColumn | ''; order: SortDirection } => {
  if (current.by !== column) return { by: column, order: 'desc' }
  if (current.order === 'desc') return { by: column, order: 'asc' }
  return { by: '', order: '' }
}

const toggleSort = (column: SortColumn) => {
  const next = nextSortState(sort, column)
  sort.by = next.by
  sort.order = next.order
  fetchUsers(1)
}

const showModal = ref(false)
const submitting = ref(false)
const error = ref('')
const editingId = ref<number | null>(null)
const siteCurrency = ref('CNY')
const form = reactive({
  email: '',
  nickname: '',
  password: '',
  locale: 'zh-CN',
  email_verified: 'unverified',
  status: 'active',
  admin_note: '',
})

const { errors: formErrors, validate, clearErrors } = useFormValidation({
  email: [rules.required('This field is required'), rules.email('Invalid email')],
  nickname: [rules.required('This field is required')],
})

const fetchUsers = async (page = 1, options: ListFetchOptions = {}) => {
  if (!options.preserveRows) loading.value = true
  try {
    const response = await adminAPI.getUsers({
      page,
      page_size: pagination.value.page_size,
      user_id: filters.userId || undefined,
      keyword: filters.keyword || undefined,
      status: normalizeFilterValue(filters.status) || undefined,
      created_from: toRFC3339(filters.createdFrom),
      created_to: toRFC3339(filters.createdTo),
      last_login_from: toRFC3339(filters.lastLoginFrom),
      last_login_to: toRFC3339(filters.lastLoginTo),
      sort_by: sort.by || undefined,
      sort_order: sort.by ? sort.order : undefined,
    })
    users.value = response.data.data || []
    pagination.value = response.data.pagination || pagination.value
    selectedIds.value = []
  } catch {
    if (!options.preserveRows) users.value = []
  } finally {
    if (!options.preserveRows) loading.value = false
  }
}

const fetchSiteCurrency = async () => {
  try {
    const response = await adminAPI.getSettings({ key: 'site_config' })
    const data = response.data?.data as Record<string, unknown>
    const raw = String(data?.currency || 'CNY').trim().toUpperCase()
    siteCurrency.value = /^[A-Z]{3}$/.test(raw) ? raw : 'CNY'
  } catch {
    siteCurrency.value = 'CNY'
  }
}

const memberLevelsMap = ref<Map<number, AdminMemberLevel>>(new Map())

const fetchMemberLevels = async () => {
  try {
    const response = await adminAPI.getMemberLevels({ page: 1, page_size: 100 })
    const list = response.data.data || []
    const map = new Map<number, AdminMemberLevel>()
    list.forEach((l: AdminMemberLevel) => map.set(l.id, l))
    memberLevelsMap.value = map
  } catch {
    // ignore
  }
}

const getMemberLevelLabel = (user: AdminUser) => {
  const levelId = Number(user.member_level_id || 0)
  if (!levelId) return '-'
  const level = memberLevelsMap.value.get(levelId)
  if (!level) return `#${levelId}`
  const icon = level.icon ? level.icon + ' ' : ''
  return icon + getLocalizedText(level.name)
}

const handleSearch = () => {
  fetchUsers(1)
}
const debouncedSearch = useDebounceFn(handleSearch, 300)

const refresh = () => {
  refreshList(() => fetchUsers(pagination.value.page, { preserveRows: true }))
}

const resetFilters = () => {
  filters.userId = ''
  filters.keyword = ''
  filters.status = '__all__'
  filters.createdFrom = ''
  filters.createdTo = ''
  filters.lastLoginFrom = ''
  filters.lastLoginTo = ''
  fetchUsers(1)
}

// router-link 的 :to 走 vue-router，base 前缀由 router 自己带上，这里不能再拼
const userDetailLink = (userId: number) => `/users/${userId}`

const allSelected = computed(() => {
  if (users.value.length === 0) return false
  return users.value.every((item) => selectedIds.value.includes(item.id))
})

const toggleSelectAll = () => {
  if (allSelected.value) {
    selectedIds.value = []
    return
  }
  selectedIds.value = users.value.map((item) => item.id)
}

const toggleUserSelected = (id: number, v: boolean | 'indeterminate') => {
  toggleArrayMember(selectedIds, id, v)
}

const batchUpdateStatus = async (status: string) => {
  if (selectedIds.value.length === 0) return
  const confirmed = await confirmAction(t('admin.users.batch.confirm', { count: selectedIds.value.length }))
  if (!confirmed) return
  try {
    await adminAPI.batchUpdateUserStatus({
      user_ids: selectedIds.value,
      status,
    })
    fetchUsers(pagination.value.page)
  } catch (err: any) {
    error.value = err?.message || t('admin.users.errors.updateFailed')
  }
}

const changePage = (page: number) => {
  if (page < 1 || page > pagination.value.total_page) return
  fetchUsers(page)
}

const pageSizeOptions = [10, 20, 50, 100]

const changePageSize = (size: number) => {
  if (size === pagination.value.page_size) return
  pagination.value.page_size = size
  fetchUsers(1)
}

const openEditModal = (user: AdminUser) => {
  editingId.value = user.id
  form.email = user.email || ''
  form.nickname = user.display_name || ''
  form.password = ''
  form.locale = user.locale || 'zh-CN'
  form.email_verified = user.email_verified_at ? 'verified' : 'unverified'
  form.status = user.status || 'active'
  form.admin_note = (user.admin_note as string) || ''
  error.value = ''
  clearErrors()
  showModal.value = true
}

const closeModal = () => {
  showModal.value = false
  editingId.value = null
  error.value = ''
  clearErrors()
}

const handleSubmit = async () => {
  if (!editingId.value) return
  if (!validate({ email: form.email, nickname: form.nickname } as Record<string, unknown>)) return
  submitting.value = true
  try {
    await adminAPI.updateUser(editingId.value, {
      email: form.email,
      nickname: form.nickname,
      password: form.password || undefined,
      locale: form.locale,
      email_verified: form.email_verified === 'verified',
      status: form.status,
      admin_note: form.admin_note,
    })
    closeModal()
    fetchUsers(pagination.value.page)
  } catch (err: any) {
    error.value = err?.message || t('admin.users.errors.updateFailed')
  } finally {
    submitting.value = false
  }
}

const statusClass = (status: string) => userStatusClass(status)
const statusLabel = (status: string) => userStatusLabel(t, status)

const formatLocale = (raw?: string) => {
  if (!raw) return '-'
  const map: Record<string, string> = {
    'zh-CN': t('admin.common.lang.zhCN'),
    'zh-TW': t('admin.common.lang.zhTW'),
    'en-US': t('admin.common.lang.enUS'),
  }
  return map[raw] || raw
}

onMounted(() => {
  fetchSiteCurrency()
  fetchUsers()
  fetchMemberLevels()
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <h1 class="text-2xl font-semibold">{{ t('admin.users.title') }}</h1>
    </div>

    <div class="rounded-xl border border-border bg-card p-4 shadow-sm">
      <div class="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center">
        <div class="w-full md:w-32">
          <Input v-model="filters.userId" :placeholder="t('admin.users.filterUserId')" @update:modelValue="debouncedSearch" />
        </div>
        <div class="w-full md:w-64">
          <Input v-model="filters.keyword" :placeholder="t('admin.users.filterKeyword')" @update:modelValue="debouncedSearch" />
        </div>
        <div class="w-full md:w-40">
          <Select v-model="filters.status" @update:modelValue="handleSearch">
            <SelectTrigger class="h-9 w-full">
            <SelectValue :placeholder="t('admin.users.filterStatusAll')" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">{{ t('admin.users.filterStatusAll') }}</SelectItem>
            <SelectItem value="active">{{ t('admin.users.status.active') }}</SelectItem>
            <SelectItem value="disabled">{{ t('admin.users.status.disabled') }}</SelectItem>
          </SelectContent>
        </Select>
        </div>
        <div class="flex w-full flex-col gap-2 md:w-auto md:flex-row md:flex-wrap md:items-center">
          <span class="text-xs text-muted-foreground whitespace-nowrap">{{ t('admin.users.filterCreatedRange') }}</span>
          <Input
            v-model="filters.createdFrom"
            type="datetime-local"
            class="h-9 w-full md:w-auto"
            :placeholder="t('admin.users.filterCreatedFrom')"
            @update:modelValue="handleSearch"
          />
          <span class="hidden text-muted-foreground md:inline">-</span>
          <Input
            v-model="filters.createdTo"
            type="datetime-local"
            class="h-9 w-full md:w-auto"
            :placeholder="t('admin.users.filterCreatedTo')"
            @update:modelValue="handleSearch"
          />
        </div>
        <div class="flex w-full flex-col gap-2 md:w-auto md:flex-row md:flex-wrap md:items-center">
          <span class="text-xs text-muted-foreground whitespace-nowrap">{{ t('admin.users.filterLastLoginRange') }}</span>
          <Input
            v-model="filters.lastLoginFrom"
            type="datetime-local"
            class="h-9 w-full md:w-auto"
            :placeholder="t('admin.users.filterLastLoginFrom')"
            @update:modelValue="handleSearch"
          />
          <span class="hidden text-muted-foreground md:inline">-</span>
          <Input
            v-model="filters.lastLoginTo"
            type="datetime-local"
            class="h-9 w-full md:w-auto"
            :placeholder="t('admin.users.filterLastLoginTo')"
            @update:modelValue="handleSearch"
          />
        </div>
        <div class="hidden flex-1 sm:block"></div>
        <div class="flex w-full flex-col gap-2 sm:w-auto sm:flex-row">
          <Button size="sm" variant="outline" class="w-full sm:w-auto" @click="resetFilters">{{ t('admin.common.reset') }}</Button>
          <Button size="sm" class="w-full sm:w-auto" :disabled="refreshing" @click="refresh">{{ t('admin.common.refresh') }}</Button>
        </div>
      </div>
    </div>

    <div class="rounded-xl border border-border bg-card overflow-x-auto">
      <Table class="min-w-[1020px]">
        <TableHeader class="border-b border-border bg-muted/40 text-xs uppercase text-muted-foreground">
          <TableRow>
            <TableHead class="px-6 py-3">
              <Checkbox :model-value="allSelected" @update:model-value="toggleSelectAll" />
            </TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.users.table.id') }}</TableHead>
            <TableHead class="px-6 py-3 min-w-[140px]">{{ t('admin.users.table.email') }}</TableHead>
            <TableHead class="px-6 py-3 min-w-[160px]">{{ t('admin.users.table.nickname') }}</TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.users.table.status') }}</TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.users.table.locale') }}</TableHead>
            <TableHead
              class="px-6 py-3"
              :aria-sort="sort.by === 'wallet_balance' ? (sort.order === 'asc' ? 'ascending' : 'descending') : 'none'"
            >
              <button
                type="button"
                class="inline-flex items-center gap-1 uppercase transition-colors hover:text-foreground"
                @click="toggleSort('wallet_balance')"
              >
                {{ t('admin.users.table.walletBalance') }}
                <ArrowUp v-if="sort.by === 'wallet_balance' && sort.order === 'asc'" class="h-3.5 w-3.5" />
                <ArrowDown v-else-if="sort.by === 'wallet_balance' && sort.order === 'desc'" class="h-3.5 w-3.5" />
                <ChevronsUpDown v-else class="h-3.5 w-3.5 opacity-40" />
              </button>
            </TableHead>
            <TableHead class="px-6 py-3 min-w-[140px]">{{ t('admin.users.table.memberLevel') }}</TableHead>
            <TableHead
              class="px-6 py-3"
              :aria-sort="sort.by === 'created_at' ? (sort.order === 'asc' ? 'ascending' : 'descending') : 'none'"
            >
              <button
                type="button"
                class="inline-flex items-center gap-1 uppercase transition-colors hover:text-foreground"
                @click="toggleSort('created_at')"
              >
                {{ t('admin.users.table.createdAt') }}
                <ArrowUp v-if="sort.by === 'created_at' && sort.order === 'asc'" class="h-3.5 w-3.5" />
                <ArrowDown v-else-if="sort.by === 'created_at' && sort.order === 'desc'" class="h-3.5 w-3.5" />
                <ChevronsUpDown v-else class="h-3.5 w-3.5 opacity-40" />
              </button>
            </TableHead>
            <TableHead
              class="px-6 py-3"
              :aria-sort="sort.by === 'last_login_at' ? (sort.order === 'asc' ? 'ascending' : 'descending') : 'none'"
            >
              <button
                type="button"
                class="inline-flex items-center gap-1 uppercase transition-colors hover:text-foreground"
                @click="toggleSort('last_login_at')"
              >
                {{ t('admin.users.table.lastLoginAt') }}
                <ArrowUp v-if="sort.by === 'last_login_at' && sort.order === 'asc'" class="h-3.5 w-3.5" />
                <ArrowDown v-else-if="sort.by === 'last_login_at' && sort.order === 'desc'" class="h-3.5 w-3.5" />
                <ChevronsUpDown v-else class="h-3.5 w-3.5 opacity-40" />
              </button>
            </TableHead>
            <TableHead class="px-6 py-3 min-w-[120px]">{{ t('admin.users.table.adminNote') }}</TableHead>
            <TableHead class="px-6 py-3 min-w-[140px] text-right">{{ t('admin.users.table.action') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody class="divide-y divide-border">
          <TableRow v-if="loading">
            <TableCell :colspan="12" class="p-0">
              <TableSkeleton :columns="10" :rows="5" />
            </TableCell>
          </TableRow>
          <TableRow v-else-if="users.length === 0">
            <TableCell colspan="12" class="px-6 py-8 text-center text-muted-foreground">{{ t('admin.users.empty') }}</TableCell>
          </TableRow>
          <TableRow v-for="user in users" :key="user.id" class="hover:bg-muted/30">
            <TableCell class="px-6 py-4">
              <Checkbox :model-value="selectedIds.includes(user.id)" @update:model-value="(v) => toggleUserSelected(user.id, v)" />
            </TableCell>
            <TableCell class="px-6 py-4">
              <IdCell :value="user.id" />
            </TableCell>
            <TableCell class="min-w-[140px] px-6 py-4 text-foreground break-all">{{ user.email }}</TableCell>
            <TableCell class="min-w-[160px] px-6 py-4 text-muted-foreground break-words">{{ user.display_name || '-' }}</TableCell>
            <TableCell class="px-6 py-4 text-xs">
              <span class="inline-flex rounded-full border px-2.5 py-1 text-xs" :class="statusClass(user.status)">
                {{ statusLabel(user.status) }}
              </span>
            </TableCell>
            <TableCell class="px-6 py-4 text-xs text-muted-foreground">{{ formatLocale(user.locale) }}</TableCell>
            <TableCell class="px-6 py-4 text-xs font-mono text-foreground">{{ formatMoney(user.wallet_balance, siteCurrency) }}</TableCell>
            <TableCell class="min-w-[140px] px-6 py-4 text-xs text-foreground break-words">{{ getMemberLevelLabel(user) }}</TableCell>
            <TableCell class="px-6 py-4 text-xs text-muted-foreground">{{ formatDate(user.created_at) }}</TableCell>
            <TableCell class="px-6 py-4 text-xs text-muted-foreground">{{ formatDate(user.last_login_at) }}</TableCell>
            <TableCell class="min-w-[120px] px-6 py-4 text-xs text-muted-foreground truncate max-w-[200px]" :title="(user.admin_note as string) || ''">{{ (user.admin_note as string) || '-' }}</TableCell>
            <TableCell class="min-w-[140px] px-6 py-4 text-right">
              <div class="flex flex-wrap items-center justify-end gap-2">
                <Button as-child size="sm" variant="outline">
                  <router-link :to="userDetailLink(user.id)">{{ t('admin.users.actions.detail') }}</router-link>
                </Button>
                <Button size="sm" variant="outline" @click="openEditModal(user)">
                  {{ t('admin.users.actions.edit') }}
                </Button>
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
      >
        <template #actions>
          <div v-if="selectedIds.length > 0" class="flex items-center gap-2">
            <Button
              size="sm"
              variant="outline"
              class="border-emerald-200 text-emerald-700 hover:bg-emerald-50"
              @click="batchUpdateStatus('active')"
            >
              {{ t('admin.users.batch.enable') }}
            </Button>
            <Button
              size="sm"
              variant="outline"
              class="border-destructive/40 text-destructive hover:bg-destructive/10"
              @click="batchUpdateStatus('disabled')"
            >
              {{ t('admin.users.batch.disable') }}
            </Button>
          </div>
        </template>
      </ListPagination>
    </div>

    <Dialog v-model:open="showModal" @update:open="(value) => { if (!value) closeModal() }">
      <DialogScrollContent class="w-[calc(100vw-1rem)] max-w-xl p-4 sm:p-6">
        <DialogHeader>
          <DialogTitle>{{ t('admin.users.modal.editTitle') }}</DialogTitle>
        </DialogHeader>
        <form class="space-y-4" @submit.prevent="handleSubmit">
          <div class="grid grid-cols-1 gap-4">
            <div>
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.users.form.email') }}</label>
              <Input v-model="form.email" :placeholder="t('admin.users.form.emailPlaceholder')" />
              <p v-if="formErrors.email" class="text-xs text-destructive mt-1">{{ formErrors.email }}</p>
            </div>
            <div>
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.users.form.nickname') }}</label>
              <Input v-model="form.nickname" :placeholder="t('admin.users.form.nicknamePlaceholder')" maxlength="32" />
              <p v-if="formErrors.nickname" class="text-xs text-destructive mt-1">{{ formErrors.nickname }}</p>
            </div>
            <div>
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.users.form.password') }}</label>
              <Input v-model="form.password" type="password" :placeholder="t('admin.users.form.passwordPlaceholder')" />
              <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.users.form.passwordTip') }}</p>
            </div>
            <div>
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.users.form.locale') }}</label>
              <Select v-model="form.locale">
                <SelectTrigger class="h-9 w-full">
                  <SelectValue :placeholder="t('admin.common.lang.zhCN')" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="zh-CN">{{ t('admin.common.lang.zhCN') }}</SelectItem>
                  <SelectItem value="zh-TW">{{ t('admin.common.lang.zhTW') }}</SelectItem>
                  <SelectItem value="en-US">{{ t('admin.common.lang.enUS') }}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div>
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.users.form.emailVerifiedStatus') }}</label>
              <Select v-model="form.email_verified">
                <SelectTrigger class="h-9 w-full">
                  <SelectValue :placeholder="t('admin.users.emailVerification.verified')" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="verified">{{ t('admin.users.emailVerification.verified') }}</SelectItem>
                  <SelectItem value="unverified">{{ t('admin.users.emailVerification.unverified') }}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div>
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.users.form.status') }}</label>
              <Select v-model="form.status">
                <SelectTrigger class="h-9 w-full">
                  <SelectValue :placeholder="t('admin.users.status.active')" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="active">{{ t('admin.users.status.active') }}</SelectItem>
                  <SelectItem value="disabled">{{ t('admin.users.status.disabled') }}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div>
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.users.form.adminNote') }}</label>
              <Textarea v-model="form.admin_note" :placeholder="t('admin.users.form.adminNotePlaceholder')" rows="3" />
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
