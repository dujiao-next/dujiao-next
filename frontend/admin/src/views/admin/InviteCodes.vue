<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Copy } from 'lucide-vue-next'
import { adminAPI, type AdminUpdateInviteCodePayload } from '@/api/admin'
import type { AdminInviteCode } from '@/api/types'
import IdCell from '@/components/IdCell.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Dialog, DialogHeader, DialogScrollContent, DialogTitle } from '@/components/ui/dialog'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import TableSkeleton from '@/components/TableSkeleton.vue'
import ListPagination from '@/components/ListPagination.vue'
import { useListRefresh, type ListFetchOptions } from '@/composables/useListRefresh'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { formatDate } from '@/utils/format'
import { confirmAction } from '@/utils/confirm'
import { copyText } from '@/utils/clipboard'
import { notifyError, notifySuccess } from '@/utils/notify'
import { userStatusClass, userStatusLabel } from '@/utils/status'

interface InviteCodeUser {
  id: number
  email: string
  display_name: string
  status: string
  created_at: string
}

const { t } = useI18n()

const loading = ref(true)
const { refreshing, refreshList } = useListRefresh()
const codes = ref<AdminInviteCode[]>([])
const pagination = ref({ page: 1, page_size: 20, total: 0, total_page: 1 })
const pageSizeOptions = [10, 20, 50, 100]
const filters = reactive({ code: '', status: '__all__' })

const showGenerateModal = ref(false)
const generateSubmitting = ref(false)
const generateError = ref('')
const generateForm = reactive({ quantity: 10, code: '', maxUses: 1, expiresAt: '', note: '' })
const generatedCodes = ref<string[]>([])
const showResultModal = ref(false)

const showEditModal = ref(false)
const editSubmitting = ref(false)
const editError = ref('')
const editingCode = ref<AdminInviteCode | null>(null)
const editForm = reactive({ maxUses: 1, expiresAt: '', note: '' })

const showUsersModal = ref(false)
const usersLoading = ref(false)
const usersCode = ref<AdminInviteCode | null>(null)
const users = ref<InviteCodeUser[]>([])
const usersPagination = ref({ page: 1, page_size: 20, total: 0, total_page: 1 })

const customCodePattern = /^[A-Za-z0-9_-]{4,32}$/

const toISO = (raw?: string) => {
  const text = String(raw || '').trim()
  if (!text) return ''
  const date = new Date(text)
  if (Number.isNaN(date.getTime())) return ''
  return date.toISOString()
}

const toLocalInput = (raw?: string | null) => {
  if (!raw) return ''
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return ''
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60000)
  return local.toISOString().slice(0, 16)
}

const isValidMaxUses = (value: number) => Number.isInteger(value) && value >= 0

const statusKey = (item: AdminInviteCode) => {
  if (item.status === 'disabled') return 'disabled'
  if (item.usable) return 'active'
  if (item.max_uses > 0 && item.used_count >= item.max_uses) return 'exhausted'
  return 'expired'
}

const statusClass = (item: AdminInviteCode) => {
  switch (statusKey(item)) {
    case 'active':
      return 'text-emerald-700 border-emerald-200 bg-emerald-50'
    case 'exhausted':
      return 'text-sky-700 border-sky-200 bg-sky-50'
    case 'expired':
      return 'text-amber-700 border-amber-200 bg-amber-50'
    default:
      return 'text-muted-foreground border-border bg-muted/30'
  }
}

const usageText = (item: AdminInviteCode) => `${item.used_count} / ${item.max_uses > 0 ? item.max_uses : '∞'}`

const handleCopy = async (text: string) => {
  try {
    await copyText(text)
    notifySuccess(t('admin.inviteCodes.alerts.copied'))
  } catch {
    notifyError(t('admin.common.copyFailed'))
  }
}

const fetchCodes = async (page = 1, options: ListFetchOptions = {}) => {
  if (!options.preserveRows) loading.value = true
  try {
    const response = await adminAPI.getInviteCodes({
      page,
      page_size: pagination.value.page_size,
      code: filters.code.trim() || undefined,
      status: filters.status === '__all__' ? undefined : filters.status,
    })
    codes.value = (response.data.data as AdminInviteCode[]) || []
    pagination.value = response.data.pagination || pagination.value
  } catch {
    if (!options.preserveRows) codes.value = []
  } finally {
    if (!options.preserveRows) loading.value = false
  }
}

const handleSearch = () => fetchCodes(1, { preserveRows: true })

const refresh = () => {
  refreshList(() => fetchCodes(pagination.value.page, { preserveRows: true }))
}

const changePage = (page: number) => {
  if (page < 1 || page > pagination.value.total_page) return
  fetchCodes(page)
}

const changePageSize = (size: number) => {
  if (size === pagination.value.page_size) return
  pagination.value.page_size = size
  fetchCodes(1)
}

const openGenerateModal = () => {
  Object.assign(generateForm, { quantity: 10, code: '', maxUses: 1, expiresAt: '', note: '' })
  generateError.value = ''
  showGenerateModal.value = true
}

const submitGenerate = async () => {
  generateError.value = ''
  const code = generateForm.code.trim()
  const quantity = code ? 1 : Number(generateForm.quantity)
  if (!Number.isInteger(quantity) || quantity < 1 || quantity > 1000) {
    generateError.value = t('admin.inviteCodes.errors.invalidQuantity')
    return
  }
  if (code && !customCodePattern.test(code)) {
    generateError.value = t('admin.inviteCodes.errors.invalidCode')
    return
  }
  const maxUses = Number(generateForm.maxUses)
  if (!isValidMaxUses(maxUses)) {
    generateError.value = t('admin.inviteCodes.errors.invalidMaxUses')
    return
  }

  generateSubmitting.value = true
  try {
    const response = await adminAPI.generateInviteCodes({
      quantity,
      code: code || undefined,
      max_uses: maxUses,
      expires_at: toISO(generateForm.expiresAt) || undefined,
      note: generateForm.note.trim() || undefined,
    })
    const created = (response?.data?.data?.codes as AdminInviteCode[]) || []
    generatedCodes.value = created.map((item) => item.code)
    showGenerateModal.value = false
    showResultModal.value = true
    await fetchCodes(1)
  } catch (err: any) {
    generateError.value = err?.message || t('admin.inviteCodes.errors.generateFailed')
  } finally {
    generateSubmitting.value = false
  }
}

const openEditModal = (item: AdminInviteCode) => {
  editingCode.value = item
  editForm.maxUses = item.max_uses
  editForm.expiresAt = toLocalInput(item.expires_at)
  editForm.note = item.note || ''
  editError.value = ''
  showEditModal.value = true
}

const submitEdit = async () => {
  if (!editingCode.value) return
  editError.value = ''
  const maxUses = Number(editForm.maxUses)
  if (!isValidMaxUses(maxUses)) {
    editError.value = t('admin.inviteCodes.errors.invalidMaxUses')
    return
  }
  const payload: AdminUpdateInviteCodePayload = { max_uses: maxUses, note: editForm.note.trim() }
  if (editForm.expiresAt !== toLocalInput(editingCode.value.expires_at)) {
    payload.expires_at = toISO(editForm.expiresAt)
  }

  editSubmitting.value = true
  try {
    await adminAPI.updateInviteCode(editingCode.value.id, payload)
    notifySuccess(t('admin.inviteCodes.alerts.updated'))
    showEditModal.value = false
    await fetchCodes(pagination.value.page)
  } catch (err: any) {
    editError.value = err?.message || t('admin.inviteCodes.errors.updateFailed')
  } finally {
    editSubmitting.value = false
  }
}

const toggleStatus = async (item: AdminInviteCode) => {
  try {
    await adminAPI.updateInviteCode(item.id, { status: item.status === 'active' ? 'disabled' : 'active' })
    notifySuccess(t('admin.inviteCodes.alerts.updated'))
    await fetchCodes(pagination.value.page, { preserveRows: true })
  } catch {
    // 错误提示由 API client 统一弹出
  }
}

const handleDelete = async (item: AdminInviteCode) => {
  const confirmed = await confirmAction({
    description: t('admin.inviteCodes.confirmDelete', { code: item.code }),
    confirmText: t('admin.common.delete'),
    variant: 'destructive',
  })
  if (!confirmed) return
  try {
    await adminAPI.deleteInviteCode(item.id)
    notifySuccess(t('admin.inviteCodes.alerts.deleted'))
    const targetPage = codes.value.length === 1 && pagination.value.page > 1 ? pagination.value.page - 1 : pagination.value.page
    await fetchCodes(targetPage)
  } catch {
    // 错误提示由 API client 统一弹出
  }
}

const fetchUsers = async (page = 1) => {
  if (!usersCode.value) return
  usersLoading.value = true
  try {
    const response = await adminAPI.getInviteCodeUsers(usersCode.value.id, {
      page,
      page_size: usersPagination.value.page_size,
    })
    users.value = (response.data.data as InviteCodeUser[]) || []
    usersPagination.value = response.data.pagination || usersPagination.value
  } catch {
    users.value = []
  } finally {
    usersLoading.value = false
  }
}

const openUsersModal = (item: AdminInviteCode) => {
  usersCode.value = item
  users.value = []
  usersPagination.value.page = 1
  showUsersModal.value = true
  fetchUsers(1)
}

const changeUsersPage = (page: number) => {
  if (page < 1 || page > usersPagination.value.total_page) return
  fetchUsers(page)
}

onMounted(() => {
  fetchCodes()
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <h1 class="text-2xl font-semibold">{{ t('admin.inviteCodes.title') }}</h1>
        <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.inviteCodes.subtitle') }}</p>
      </div>
      <Button size="sm" class="w-full gap-2 sm:w-auto" @click="openGenerateModal">
        <svg class="h-4 w-4 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
        </svg>
        {{ t('admin.inviteCodes.generate') }}
      </Button>
    </div>

    <div class="rounded-xl border border-border bg-card p-4 shadow-sm">
      <div class="grid grid-cols-1 gap-3 md:grid-cols-12">
        <div class="md:col-span-4">
          <Input v-model="filters.code" :placeholder="t('admin.inviteCodes.filterCode')" @keyup.enter="handleSearch" />
        </div>
        <div class="md:col-span-3">
          <Select v-model="filters.status" @update:modelValue="handleSearch">
            <SelectTrigger class="h-9 w-full">
              <SelectValue :placeholder="t('admin.inviteCodes.filterStatusAll')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="__all__">{{ t('admin.inviteCodes.filterStatusAll') }}</SelectItem>
              <SelectItem value="active">{{ t('admin.inviteCodes.status.active') }}</SelectItem>
              <SelectItem value="disabled">{{ t('admin.inviteCodes.status.disabled') }}</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="md:col-span-5 flex items-center justify-end">
          <Button size="sm" class="w-full md:w-auto" @click="handleSearch">{{ t('admin.common.search') }}</Button>
        </div>
      </div>
    </div>

    <div class="rounded-xl border border-border bg-card overflow-x-auto">
      <Table class="min-w-[1060px]">
        <TableHeader class="border-b border-border bg-muted/40 text-xs uppercase text-muted-foreground">
          <TableRow>
            <TableHead class="min-w-[80px] px-4 py-3">{{ t('admin.inviteCodes.table.id') }}</TableHead>
            <TableHead class="min-w-[180px] px-4 py-3">{{ t('admin.inviteCodes.table.code') }}</TableHead>
            <TableHead class="min-w-[90px] px-4 py-3">{{ t('admin.inviteCodes.table.usage') }}</TableHead>
            <TableHead class="min-w-[90px] px-4 py-3">{{ t('admin.inviteCodes.table.status') }}</TableHead>
            <TableHead class="min-w-[120px] px-4 py-3">{{ t('admin.inviteCodes.table.expiresAt') }}</TableHead>
            <TableHead class="min-w-[140px] px-4 py-3">{{ t('admin.inviteCodes.table.note') }}</TableHead>
            <TableHead class="min-w-[120px] px-4 py-3">{{ t('admin.inviteCodes.table.createdAt') }}</TableHead>
            <TableHead class="min-w-[260px] px-4 py-3 text-right">{{ t('admin.inviteCodes.table.action') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody class="divide-y divide-border">
          <TableRow v-if="loading">
            <TableCell :colspan="8" class="p-0">
              <TableSkeleton :columns="8" :rows="5" />
            </TableCell>
          </TableRow>
          <TableRow v-else-if="codes.length === 0">
            <TableCell colspan="8" class="px-4 py-8 text-center text-muted-foreground">{{ t('admin.inviteCodes.empty') }}</TableCell>
          </TableRow>
          <TableRow v-for="item in codes" :key="item.id" class="hover:bg-muted/30">
            <TableCell class="px-4 py-3"><IdCell :value="item.id" /></TableCell>
            <TableCell class="px-4 py-3">
              <div class="flex items-center gap-2">
                <span class="font-mono text-xs text-foreground break-all">{{ item.code }}</span>
                <button
                  type="button"
                  class="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-md border border-border/60 text-muted-foreground hover:text-foreground hover:border-border"
                  :title="t('admin.common.copy')"
                  @click="handleCopy(item.code)"
                >
                  <Copy class="h-3.5 w-3.5" />
                </button>
              </div>
            </TableCell>
            <TableCell class="px-4 py-3 font-mono text-xs text-foreground">{{ usageText(item) }}</TableCell>
            <TableCell class="px-4 py-3">
              <span class="inline-flex rounded-full border px-2.5 py-1 text-xs" :class="statusClass(item)">
                {{ t(`admin.inviteCodes.status.${statusKey(item)}`) }}
              </span>
            </TableCell>
            <TableCell class="px-4 py-3 text-xs text-muted-foreground">{{ item.expires_at ? formatDate(item.expires_at) : t('admin.inviteCodes.neverExpire') }}</TableCell>
            <TableCell class="px-4 py-3 text-xs text-muted-foreground break-words">{{ item.note || '-' }}</TableCell>
            <TableCell class="px-4 py-3 text-xs text-muted-foreground">{{ formatDate(item.created_at) || '-' }}</TableCell>
            <TableCell class="px-4 py-3 text-right">
              <div class="flex flex-wrap items-center justify-end gap-2">
                <Button size="sm" variant="outline" @click="openUsersModal(item)">{{ t('admin.inviteCodes.actions.users') }}</Button>
                <Button size="sm" variant="outline" @click="toggleStatus(item)">
                  {{ item.status === 'active' ? t('admin.inviteCodes.actions.disable') : t('admin.inviteCodes.actions.enable') }}
                </Button>
                <Button size="sm" variant="outline" @click="openEditModal(item)">{{ t('admin.common.edit') }}</Button>
                <Button size="sm" variant="destructive" @click="handleDelete(item)">{{ t('admin.common.delete') }}</Button>
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
          <Button size="sm" variant="outline" class="h-8" :disabled="refreshing" @click="refresh">{{ t('admin.common.refresh') }}</Button>
        </template>
      </ListPagination>
    </div>

    <Dialog v-model:open="showGenerateModal">
      <DialogScrollContent class="w-[calc(100vw-1rem)] max-w-xl p-4 sm:p-6" @interact-outside="(e) => e.preventDefault()">
        <DialogHeader>
          <DialogTitle>{{ t('admin.inviteCodes.modal.generateTitle') }}</DialogTitle>
        </DialogHeader>
        <form class="space-y-4" @submit.prevent="submitGenerate">
          <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
            <div class="space-y-2 md:col-span-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.inviteCodes.form.code') }}</label>
              <Input v-model="generateForm.code" maxlength="32" class="font-mono" />
              <p class="text-xs text-muted-foreground">{{ t('admin.inviteCodes.form.codeHint') }}</p>
            </div>
            <div class="space-y-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.inviteCodes.form.quantity') }}</label>
              <Input v-model.number="generateForm.quantity" type="number" min="1" max="1000" :disabled="!!generateForm.code.trim()" />
            </div>
            <div class="space-y-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.inviteCodes.form.maxUses') }}</label>
              <Input v-model.number="generateForm.maxUses" type="number" min="0" />
              <p class="text-xs text-muted-foreground">{{ t('admin.inviteCodes.form.maxUsesHint') }}</p>
            </div>
            <div class="space-y-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.inviteCodes.form.expiresAt') }}</label>
              <Input v-model="generateForm.expiresAt" type="datetime-local" />
              <p class="text-xs text-muted-foreground">{{ t('admin.inviteCodes.form.expiresHint') }}</p>
            </div>
            <div class="space-y-2 md:col-span-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.inviteCodes.form.note') }}</label>
              <Input v-model="generateForm.note" maxlength="255" />
            </div>
          </div>

          <div v-if="generateError" class="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {{ generateError }}
          </div>
          <div class="flex flex-col-reverse gap-3 sm:flex-row sm:justify-end">
            <Button type="button" variant="outline" class="w-full sm:w-auto" @click="showGenerateModal = false">{{ t('admin.common.cancel') }}</Button>
            <Button type="submit" class="w-full sm:w-auto" :disabled="generateSubmitting">
              {{ generateSubmitting ? t('admin.common.submitting') : t('admin.common.confirm') }}
            </Button>
          </div>
        </form>
      </DialogScrollContent>
    </Dialog>

    <Dialog v-model:open="showResultModal">
      <DialogScrollContent class="w-[calc(100vw-1rem)] max-w-xl p-4 sm:p-6" @interact-outside="(e) => e.preventDefault()">
        <DialogHeader>
          <DialogTitle>{{ t('admin.inviteCodes.modal.resultTitle', { count: generatedCodes.length }) }}</DialogTitle>
        </DialogHeader>
        <div class="space-y-4">
          <Textarea :model-value="generatedCodes.join('\n')" readonly rows="10" class="font-mono text-xs" />
          <div class="flex flex-col-reverse gap-3 sm:flex-row sm:justify-end">
            <Button type="button" variant="outline" class="w-full sm:w-auto" @click="showResultModal = false">{{ t('admin.common.close') }}</Button>
            <Button type="button" class="w-full gap-2 sm:w-auto" @click="handleCopy(generatedCodes.join('\n'))">
              <Copy class="h-4 w-4" />
              {{ t('admin.inviteCodes.actions.copyAll') }}
            </Button>
          </div>
        </div>
      </DialogScrollContent>
    </Dialog>

    <Dialog v-model:open="showEditModal">
      <DialogScrollContent class="w-[calc(100vw-1rem)] max-w-xl p-4 sm:p-6" @interact-outside="(e) => e.preventDefault()">
        <DialogHeader>
          <DialogTitle>{{ t('admin.inviteCodes.modal.editTitle', { code: editingCode?.code || '' }) }}</DialogTitle>
        </DialogHeader>
        <form class="space-y-4" @submit.prevent="submitEdit">
          <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
            <div class="space-y-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.inviteCodes.form.maxUses') }}</label>
              <Input v-model.number="editForm.maxUses" type="number" min="0" />
              <p class="text-xs text-muted-foreground">{{ t('admin.inviteCodes.form.maxUsesHint') }}</p>
            </div>
            <div class="space-y-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.inviteCodes.form.expiresAt') }}</label>
              <Input v-model="editForm.expiresAt" type="datetime-local" />
              <p class="text-xs text-muted-foreground">{{ t('admin.inviteCodes.form.editExpiresHint') }}</p>
            </div>
            <div class="space-y-2 md:col-span-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.inviteCodes.form.note') }}</label>
              <Input v-model="editForm.note" maxlength="255" />
            </div>
          </div>

          <div v-if="editError" class="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {{ editError }}
          </div>
          <div class="flex flex-col-reverse gap-3 sm:flex-row sm:justify-end">
            <Button type="button" variant="outline" class="w-full sm:w-auto" @click="showEditModal = false">{{ t('admin.common.cancel') }}</Button>
            <Button type="submit" class="w-full sm:w-auto" :disabled="editSubmitting">
              {{ editSubmitting ? t('admin.common.submitting') : t('admin.common.confirm') }}
            </Button>
          </div>
        </form>
      </DialogScrollContent>
    </Dialog>

    <Dialog v-model:open="showUsersModal">
      <DialogScrollContent class="w-[calc(100vw-1rem)] max-w-3xl p-4 sm:p-6">
        <DialogHeader>
          <DialogTitle>{{ t('admin.inviteCodes.modal.usersTitle', { code: usersCode?.code || '' }) }}</DialogTitle>
        </DialogHeader>
        <div class="rounded-xl border border-border overflow-x-auto">
          <Table class="min-w-[560px]">
            <TableHeader class="border-b border-border bg-muted/40 text-xs uppercase text-muted-foreground">
              <TableRow>
                <TableHead class="px-4 py-3">{{ t('admin.inviteCodes.usersTable.id') }}</TableHead>
                <TableHead class="px-4 py-3">{{ t('admin.inviteCodes.usersTable.email') }}</TableHead>
                <TableHead class="px-4 py-3">{{ t('admin.inviteCodes.usersTable.displayName') }}</TableHead>
                <TableHead class="px-4 py-3">{{ t('admin.inviteCodes.usersTable.status') }}</TableHead>
                <TableHead class="px-4 py-3">{{ t('admin.inviteCodes.usersTable.createdAt') }}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody class="divide-y divide-border">
              <TableRow v-if="usersLoading">
                <TableCell :colspan="5" class="p-0">
                  <TableSkeleton :columns="5" :rows="3" />
                </TableCell>
              </TableRow>
              <TableRow v-else-if="users.length === 0">
                <TableCell colspan="5" class="px-4 py-8 text-center text-muted-foreground">{{ t('admin.inviteCodes.usersEmpty') }}</TableCell>
              </TableRow>
              <template v-else>
              <TableRow v-for="user in users" :key="user.id">
                <TableCell class="px-4 py-3"><IdCell :value="user.id" /></TableCell>
                <TableCell class="px-4 py-3 text-xs text-foreground break-all">{{ user.email || '-' }}</TableCell>
                <TableCell class="px-4 py-3 text-xs text-foreground break-words">{{ user.display_name || '-' }}</TableCell>
                <TableCell class="px-4 py-3">
                  <span class="inline-flex rounded-full border px-2.5 py-1 text-xs" :class="userStatusClass(user.status)">
                    {{ userStatusLabel(t, user.status) }}
                  </span>
                </TableCell>
                <TableCell class="px-4 py-3 text-xs text-muted-foreground">{{ formatDate(user.created_at) || '-' }}</TableCell>
              </TableRow>
              </template>
            </TableBody>
          </Table>
          <ListPagination
            :page="usersPagination.page"
            :total-page="usersPagination.total_page"
            :total="usersPagination.total"
            @change-page="changeUsersPage"
          />
        </div>
      </DialogScrollContent>
    </Dialog>
  </div>
</template>
