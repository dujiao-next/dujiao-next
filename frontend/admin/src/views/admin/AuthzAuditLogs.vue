<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useDebounceFn } from '@vueuse/core'
import { adminAPI, type AdminAuthzAuditLog } from '@/api/admin'
import IdCell from '@/components/IdCell.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import TableSkeleton from '@/components/TableSkeleton.vue'
import { useListRefresh, type ListFetchOptions } from '@/composables/useListRefresh'
import { notifyError } from '@/utils/notify'
import { formatDate, toRFC3339 } from '@/utils/format'

const { locale, t } = useI18n()
const { refreshing, refreshList } = useListRefresh()

const messages = {
  'zh-CN': {
    title: '权限审计日志',
    subtitle: '查看角色与策略变更记录',
    filters: {
      operator: '操作管理员ID',
      target: '目标管理员ID',
      action: '动作',
      role: '角色',
      object: '资源',
      method: '方法',
      createdFrom: '开始时间',
      createdTo: '结束时间',
      allActions: '全部动作',
      allMethods: '全部方法',
    },
    actions: {
      refresh: '刷新',
      search: '查询',
      reset: '重置',
      prev: '上一页',
      next: '下一页',
    },
    table: {
      id: 'ID',
      operator: '操作人',
      target: '目标管理员',
      action: '动作',
      role: '角色',
      object: '资源',
      method: '方法',
      requestId: '请求ID',
      createdAt: '时间',
      empty: '暂无审计日志',
      loading: '加载中...',
    },
  },
  'zh-TW': {
    title: '權限審計日誌',
    subtitle: '查看角色與策略變更記錄',
    filters: {
      operator: '操作管理員ID',
      target: '目標管理員ID',
      action: '動作',
      role: '角色',
      object: '資源',
      method: '方法',
      createdFrom: '開始時間',
      createdTo: '結束時間',
      allActions: '全部動作',
      allMethods: '全部方法',
    },
    actions: {
      refresh: '刷新',
      search: '查詢',
      reset: '重置',
      prev: '上一頁',
      next: '下一頁',
    },
    table: {
      id: 'ID',
      operator: '操作人',
      target: '目標管理員',
      action: '動作',
      role: '角色',
      object: '資源',
      method: '方法',
      requestId: '請求ID',
      createdAt: '時間',
      empty: '暫無審計日誌',
      loading: '載入中...',
    },
  },
  'en-US': {
    title: 'Permission Audit Logs',
    subtitle: 'Review role and policy change history',
    filters: {
      operator: 'Operator Admin ID',
      target: 'Target Admin ID',
      action: 'Action',
      role: 'Role',
      object: 'Object',
      method: 'Method',
      createdFrom: 'Created From',
      createdTo: 'Created To',
      allActions: 'All Actions',
      allMethods: 'All Methods',
    },
    actions: {
      refresh: 'Refresh',
      search: 'Search',
      reset: 'Reset',
      prev: 'Prev',
      next: 'Next',
    },
    table: {
      id: 'ID',
      operator: 'Operator',
      target: 'Target Admin',
      action: 'Action',
      role: 'Role',
      object: 'Object',
      method: 'Method',
      requestId: 'Request ID',
      createdAt: 'Time',
      empty: 'No audit logs',
      loading: 'Loading...',
    },
  },
} as const

const text = computed(() => {
  if (locale.value === 'zh-TW') return messages['zh-TW']
  if (locale.value === 'en-US') return messages['en-US']
  return messages['zh-CN']
})

const loading = ref(false)
const logs = ref<AdminAuthzAuditLog[]>([])
const pagination = ref({
  page: 1,
  page_size: 20,
  total: 0,
  total_page: 1,
})

const filters = reactive({
  operator_admin_id: '',
  target_admin_id: '',
  action: '__all__',
  role: '',
  object: '',
  method: '__all__',
  created_from: '',
  created_to: '',
})

const actionOptions = [
  'role_create',
  'role_delete',
  'policy_grant',
  'policy_revoke',
  'admin_roles_update',
  'admin_api_token_create',
  'admin_api_token_revoke',
  'admin_api_token_rotate',
  'admin_api_token_request',
]

const actionLabel = (action: string) => action.startsWith('admin_api_token_') ? t(`adminApiTokens.auditActions.${action}`) : action

const methodOptions = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', '*']

const fetchLogs = async (page = 1, options: ListFetchOptions = {}) => {
  if (!options.preserveRows) loading.value = true
  try {
    const response = await adminAPI.listAuthzAuditLogs({
      page,
      page_size: pagination.value.page_size,
      operator_admin_id: filters.operator_admin_id || undefined,
      target_admin_id: filters.target_admin_id || undefined,
      action: filters.action !== '__all__' ? filters.action || undefined : undefined,
      role: filters.role || undefined,
      object: filters.object || undefined,
      method: filters.method !== '__all__' ? filters.method || undefined : undefined,
      created_from: toRFC3339(filters.created_from),
      created_to: toRFC3339(filters.created_to),
    })
    logs.value = Array.isArray(response.data.data) ? response.data.data : []
    pagination.value = response.data.pagination || pagination.value
  } catch (err: any) {
    if (!options.preserveRows) logs.value = []
    notifyError(err?.message || 'Fetch audit logs failed')
  } finally {
    if (!options.preserveRows) loading.value = false
  }
}

const handleSearch = () => {
  fetchLogs(1, { preserveRows: true })
}
const debouncedSearch = useDebounceFn(handleSearch, 300)

const handleReset = () => {
  filters.operator_admin_id = ''
  filters.target_admin_id = ''
  filters.action = '__all__'
  filters.role = ''
  filters.object = ''
  filters.method = '__all__'
  filters.created_from = ''
  filters.created_to = ''
  fetchLogs(1, { preserveRows: true })
}

const refresh = () => {
  refreshList(() => fetchLogs(pagination.value.page, { preserveRows: true }))
}

const changePage = (next: number) => {
  if (next < 1 || next > pagination.value.total_page) return
  fetchLogs(next)
}

onMounted(() => {
  fetchLogs()
})
</script>

<template>
  <div class="space-y-6">
    <div>
      <h1 class="text-2xl font-semibold">{{ text.title }}</h1>
      <p class="mt-1 text-sm text-muted-foreground">{{ text.subtitle }}</p>
    </div>

    <section class="rounded-xl border border-border bg-card p-4 space-y-3">
      <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <Input v-model="filters.operator_admin_id" type="number" min="1" :placeholder="text.filters.operator" class="h-9" @update:modelValue="debouncedSearch" />
        <Input v-model="filters.target_admin_id" type="number" min="1" :placeholder="text.filters.target" class="h-9" @update:modelValue="debouncedSearch" />
        <Select v-model="filters.action">
          <SelectTrigger class="h-9">
            <SelectValue :placeholder="text.filters.allActions" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">{{ text.filters.allActions }}</SelectItem>
            <SelectItem v-for="item in actionOptions" :key="item" :value="item">{{ actionLabel(item) }}</SelectItem>
          </SelectContent>
        </Select>
        <Input v-model="filters.role" type="text" :placeholder="text.filters.role" class="h-9" @update:modelValue="debouncedSearch" />
      </div>

      <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <Input v-model="filters.object" type="text" :placeholder="text.filters.object" class="h-9" @update:modelValue="debouncedSearch" />
        <Select v-model="filters.method">
          <SelectTrigger class="h-9">
            <SelectValue :placeholder="text.filters.allMethods" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">{{ text.filters.allMethods }}</SelectItem>
            <SelectItem v-for="item in methodOptions" :key="item" :value="item">{{ item }}</SelectItem>
          </SelectContent>
        </Select>
        <Input v-model="filters.created_from" type="datetime-local" class="h-9" :placeholder="text.filters.createdFrom" />
        <Input v-model="filters.created_to" type="datetime-local" class="h-9" :placeholder="text.filters.createdTo" />
      </div>

      <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-end">
        <Button size="sm" variant="outline" class="w-full sm:w-auto" @click="handleReset">{{ text.actions.reset }}</Button>
        <Button size="sm" variant="outline" class="w-full sm:w-auto" :disabled="refreshing" @click="refresh">{{ text.actions.refresh }}</Button>
        <Button size="sm" class="w-full sm:w-auto" @click="handleSearch">{{ text.actions.search }}</Button>
      </div>
    </section>

    <p class="text-sm text-muted-foreground lg:hidden">{{ t('adminApiTokens.scrollHint') }}</p>
    <section class="rounded-xl border border-border bg-card overflow-hidden overflow-x-auto">
      <Table class="min-w-[1000px]">
        <TableHeader class="border-b border-border bg-muted/40 text-xs uppercase text-muted-foreground">
          <TableRow>
            <TableHead class="px-4 py-3">{{ text.table.id }}</TableHead>
            <TableHead>{{ t('adminApiTokens.auditDetail') }}</TableHead>
            <TableHead class="min-w-[100px] px-4 py-3">{{ text.table.operator }}</TableHead>
            <TableHead class="min-w-[100px] px-4 py-3">{{ text.table.target }}</TableHead>
            <TableHead class="min-w-[100px] px-4 py-3">{{ text.table.action }}</TableHead>
            <TableHead class="min-w-[90px] px-4 py-3">{{ text.table.role }}</TableHead>
            <TableHead class="min-w-[100px] px-4 py-3">{{ text.table.object }}</TableHead>
            <TableHead class="min-w-[90px] px-4 py-3">{{ text.table.method }}</TableHead>
            <TableHead class="min-w-[90px] px-4 py-3">{{ text.table.requestId }}</TableHead>
            <TableHead class="min-w-[100px] px-4 py-3">{{ text.table.createdAt }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody class="divide-y divide-border">
          <TableRow v-if="loading">
            <TableCell :colspan="10" class="p-0">
              <TableSkeleton :columns="10" :rows="5" />
            </TableCell>
          </TableRow>
          <TableRow v-else-if="logs.length === 0">
            <TableCell colspan="10" class="px-4 py-6 text-center text-muted-foreground">{{ text.table.empty }}</TableCell>
          </TableRow>
          <TableRow v-for="item in logs" :key="item.id" class="hover:bg-muted/30">
            <TableCell class="px-4 py-3"><IdCell :value="item.id" /></TableCell>
            <TableCell class="min-w-40 max-w-56 break-words text-xs">
              <template v-if="item.detail?.token_id">
                <div>#{{ item.detail.token_id }}</div>
                <div v-if="item.detail.http_status" class="whitespace-nowrap text-muted-foreground">
                  {{ t('adminApiTokens.auditResult', { http: item.detail.http_status, code: item.detail.status_code }) }}
                </div>
                <div v-if="item.detail.client_ip" class="font-mono">{{ item.detail.client_ip }}</div>
                <div v-if="item.detail.denial_reason === 'ip_not_allowed'" class="font-medium text-foreground">{{ t('adminApiTokens.ipRejected') }}</div>
              </template>
              <span v-else>-</span>
            </TableCell>
            <TableCell class="min-w-[100px] px-4 py-3 text-xs">
              <div class="break-words">#{{ item.operator_admin_id }} {{ item.operator_username || '-' }}</div>
            </TableCell>
            <TableCell class="min-w-[100px] px-4 py-3 text-xs">
              <span v-if="item.target_admin_id" class="break-words">#{{ item.target_admin_id }} {{ item.target_username || '-' }}</span>
              <span v-else>-</span>
            </TableCell>
            <TableCell class="min-w-[100px] px-4 py-3 font-medium break-words">{{ actionLabel(item.action) }}</TableCell>
            <TableCell class="min-w-[90px] px-4 py-3 text-xs break-words">{{ item.role || '-' }}</TableCell>
            <TableCell class="min-w-[100px] px-4 py-3 font-mono text-xs text-muted-foreground break-all">{{ item.object || '-' }}</TableCell>
            <TableCell class="min-w-[90px] px-4 py-3">{{ item.method || '-' }}</TableCell>
            <TableCell class="min-w-[90px] px-4 py-3 font-mono text-xs text-muted-foreground break-all">{{ item.request_id || '-' }}</TableCell>
            <TableCell class="min-w-[100px] px-4 py-3 text-xs text-muted-foreground">{{ formatDate(item.created_at) }}</TableCell>
          </TableRow>
        </TableBody>
      </Table>

      <div class="flex flex-wrap items-center justify-between gap-3 border-t border-border px-4 py-3 text-sm text-muted-foreground">
        <span>{{ pagination.page }} / {{ pagination.total_page }} · {{ pagination.total }}</span>
        <div class="flex items-center gap-2">
          <Button size="sm" variant="outline" :disabled="pagination.page <= 1" @click="changePage(pagination.page - 1)">{{ text.actions.prev }}</Button>
          <Button size="sm" variant="outline" :disabled="pagination.page >= pagination.total_page" @click="changePage(pagination.page + 1)">{{ text.actions.next }}</Button>
        </div>
      </div>
    </section>
  </div>
</template>
