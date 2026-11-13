<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  adminAPI,
  type AdminAuthzAdmin,
  type AdminAuthzPolicy,
  type AdminAuthzRole,
  type AdminPermissionCatalogItem,
} from '@/api/admin'
import { useAdminAuthStore } from '@/stores/auth'
import IdCell from '@/components/IdCell.vue'
import AdminAPITokens from '@/components/AdminAPITokens.vue'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { notifyError, notifySuccess } from '@/utils/notify'
import { confirmAction } from '@/utils/confirm'

const { locale } = useI18n()

const textMap = {
  'zh-CN': {
    title: '权限管理',
    subtitle: '管理角色、接口权限策略与管理员角色绑定',
    create: '创建',
    delete: '删除',
    edit: '编辑',
    loading: '加载中...',
    yes: '是',
    no: '否',
    unknown: '-',
    rolesTitle: '角色列表',
    rolePlaceholder: '输入角色名，如 operations_manager',
    rolesEmpty: '暂无角色',
    policiesTitle: '角色策略',
    selectRoleHint: '请先在左侧选择角色',
    immutableRoleHint: '系统内置角色由程序托管，仅可查看和分配，不能修改策略或删除。',
    objectPlaceholder: '资源路径，例如 /admin/orders/:id',
    addPolicy: '添加策略',
    policiesEmpty: '暂无策略',
    tableObject: '资源',
    tableAction: '动作',
    tableOperation: '操作',
    catalogTitle: '系统接口权限目录',
    catalogHint: '无需手动记 API，直接搜索并一键添加',
    catalogSearch: '搜索模块/路径/方法，例如 orders GET',
    catalogEmpty: '暂无可配置接口',
    catalogAdd: '添加',
    catalogAdded: '已添加',
    catalogExpandAll: '全部展开',
    catalogCollapseAll: '全部收起',
    adminManageTitle: '管理员列表',
    adminManageHint: '支持新增、编辑、删除管理员，并可快速选择管理员分配角色',
    adminSearchPlaceholder: '搜索管理员账号或 ID',
    adminListEmpty: '暂无管理员',
    adminTableId: 'ID',
    adminTableUsername: '账号',
    adminTableRoles: '角色',
    adminTableSuper: '超管',
    adminTableTotp: '2FA',
    adminTableCreatedAt: '创建时间',
    adminTableLastLoginAt: '最近登录',
    adminTotpEnabled: '已启用',
    adminTotpDisabled: '未启用',
    adminTotpReset: '重置 2FA',
    adminTotpResetConfirm: '确认重置 {name} 的两步验证？目标将被强制下线。',
    adminTotpResetSuccess: '已重置',
    adminTotpResetFailed: '重置失败',
    adminRolesEmpty: '未分配',
    adminSelectRoleTarget: '设为角色分配对象',
    adminRoleTargetSelected: '已选择',
    adminFormTitleCreate: '新增管理员',
    adminFormTitleEdit: '编辑管理员',
    adminUsername: '管理员账号',
    adminUsernamePlaceholder: '请输入管理员账号（3-64位，不含空格）',
    adminPassword: '登录密码',
    adminPasswordPlaceholderCreate: '请输入登录密码',
    adminPasswordPlaceholderEdit: '留空则不修改密码',
    adminIsSuper: '设为超级管理员（免权限校验）',
    adminSubmitCreate: '创建管理员',
    adminSubmitUpdate: '保存修改',
    adminFormReset: '重置',
    adminRolesTitle: '管理员角色分配',
    adminLabel: '管理员',
    selectAdmin: '请选择管理员',
    adminId: '管理员ID',
    superAdmin: '超级管理员',
    rolesLabel: '勾选该管理员拥有的角色',
    saveRoles: '保存角色分配',
    confirmDeleteRole: '确认删除角色 {role}？',
    confirmRevokePolicy: '确认移除策略 {action} {object}？',
    confirmDeleteAdmin: '确认删除管理员 {username}？',
    roleCreated: '角色创建成功',
    roleDeleted: '角色删除成功',
    policyGranted: '策略添加成功',
    policyRevoked: '策略移除成功',
    adminCreated: '管理员创建成功',
    adminUpdated: '管理员更新成功',
    adminDeleted: '管理员删除成功',
    adminRolesSaved: '管理员角色保存成功',
    fetchRolesFailed: '获取角色失败',
    fetchPoliciesFailed: '获取策略失败',
    fetchCatalogFailed: '获取权限目录失败',
    fetchAdminsFailed: '获取管理员失败',
    fetchAdminRolesFailed: '获取管理员角色失败',
    roleRequired: '角色名不能为空',
    createRoleFailed: '创建角色失败',
    deleteRoleFailed: '删除角色失败',
    selectRoleFirst: '请先选择角色',
    objectRequired: '资源路径不能为空',
    grantPolicyFailed: '添加策略失败',
    revokePolicyFailed: '移除策略失败',
    selectAdminFirst: '请先选择管理员',
    adminUsernameRequired: '管理员账号不能为空',
    adminPasswordRequired: '登录密码不能为空',
    createAdminFailed: '创建管理员失败',
    updateAdminFailed: '更新管理员失败',
    deleteAdminFailed: '删除管理员失败',
    saveAdminRolesFailed: '保存管理员角色失败',
  },
  'zh-TW': {
    title: '權限管理',
    subtitle: '管理角色、介面權限策略與管理員角色綁定',
    create: '建立',
    delete: '刪除',
    edit: '編輯',
    loading: '載入中...',
    yes: '是',
    no: '否',
    unknown: '-',
    rolesTitle: '角色列表',
    rolePlaceholder: '輸入角色名，例如 operations_manager',
    rolesEmpty: '暫無角色',
    policiesTitle: '角色策略',
    selectRoleHint: '請先在左側選擇角色',
    immutableRoleHint: '系統內建角色由程式託管，僅可檢視和分配，不能修改策略或刪除。',
    objectPlaceholder: '資源路徑，例如 /admin/orders/:id',
    addPolicy: '新增策略',
    policiesEmpty: '暫無策略',
    tableObject: '資源',
    tableAction: '動作',
    tableOperation: '操作',
    catalogTitle: '系統介面權限目錄',
    catalogHint: '無需手動記 API，直接搜尋並一鍵新增',
    catalogSearch: '搜尋模組/路徑/方法，例如 orders GET',
    catalogEmpty: '暫無可配置介面',
    catalogAdd: '新增',
    catalogAdded: '已新增',
    catalogExpandAll: '全部展開',
    catalogCollapseAll: '全部收起',
    adminManageTitle: '管理員列表',
    adminManageHint: '支援新增、編輯、刪除管理員，並可快速選擇管理員分配角色',
    adminSearchPlaceholder: '搜尋管理員帳號或 ID',
    adminListEmpty: '暫無管理員',
    adminTableId: 'ID',
    adminTableUsername: '帳號',
    adminTableRoles: '角色',
    adminTableSuper: '超管',
    adminTableTotp: '2FA',
    adminTableCreatedAt: '建立時間',
    adminTableLastLoginAt: '最近登入',
    adminTotpEnabled: '已啟用',
    adminTotpDisabled: '未啟用',
    adminTotpReset: '重設 2FA',
    adminTotpResetConfirm: '確認重設 {name} 的兩步驗證？目標將被強制登出。',
    adminTotpResetSuccess: '已重設',
    adminTotpResetFailed: '重設失敗',
    adminRolesEmpty: '未分配',
    adminSelectRoleTarget: '設為角色分配對象',
    adminRoleTargetSelected: '已選擇',
    adminFormTitleCreate: '新增管理員',
    adminFormTitleEdit: '編輯管理員',
    adminUsername: '管理員帳號',
    adminUsernamePlaceholder: '請輸入管理員帳號（3-64位，不含空格）',
    adminPassword: '登入密碼',
    adminPasswordPlaceholderCreate: '請輸入登入密碼',
    adminPasswordPlaceholderEdit: '留空則不修改密碼',
    adminIsSuper: '設為超級管理員（免權限校驗）',
    adminSubmitCreate: '建立管理員',
    adminSubmitUpdate: '儲存修改',
    adminFormReset: '重設',
    adminRolesTitle: '管理員角色分配',
    adminLabel: '管理員',
    selectAdmin: '請選擇管理員',
    adminId: '管理員ID',
    superAdmin: '超級管理員',
    rolesLabel: '勾選該管理員擁有的角色',
    saveRoles: '儲存角色分配',
    confirmDeleteRole: '確認刪除角色 {role}？',
    confirmRevokePolicy: '確認移除策略 {action} {object}？',
    confirmDeleteAdmin: '確認刪除管理員 {username}？',
    roleCreated: '角色建立成功',
    roleDeleted: '角色刪除成功',
    policyGranted: '策略新增成功',
    policyRevoked: '策略移除成功',
    adminCreated: '管理員建立成功',
    adminUpdated: '管理員更新成功',
    adminDeleted: '管理員刪除成功',
    adminRolesSaved: '管理員角色儲存成功',
    fetchRolesFailed: '取得角色失敗',
    fetchPoliciesFailed: '取得策略失敗',
    fetchCatalogFailed: '取得權限目錄失敗',
    fetchAdminsFailed: '取得管理員失敗',
    fetchAdminRolesFailed: '取得管理員角色失敗',
    roleRequired: '角色名不能為空',
    createRoleFailed: '建立角色失敗',
    deleteRoleFailed: '刪除角色失敗',
    selectRoleFirst: '請先選擇角色',
    objectRequired: '資源路徑不能為空',
    grantPolicyFailed: '新增策略失敗',
    revokePolicyFailed: '移除策略失敗',
    selectAdminFirst: '請先選擇管理員',
    adminUsernameRequired: '管理員帳號不能為空',
    adminPasswordRequired: '登入密碼不能為空',
    createAdminFailed: '建立管理員失敗',
    updateAdminFailed: '更新管理員失敗',
    deleteAdminFailed: '刪除管理員失敗',
    saveAdminRolesFailed: '儲存管理員角色失敗',
  },
  'en-US': {
    title: 'Authorization',
    subtitle: 'Manage roles, API policies, and admin role bindings',
    create: 'Create',
    delete: 'Delete',
    edit: 'Edit',
    loading: 'Loading...',
    yes: 'Yes',
    no: 'No',
    unknown: '-',
    rolesTitle: 'Roles',
    rolePlaceholder: 'Enter role name, e.g. operations_manager',
    rolesEmpty: 'No roles',
    policiesTitle: 'Role Policies',
    selectRoleHint: 'Please select a role first',
    immutableRoleHint: 'Built-in roles are system-managed. They can be viewed and assigned, but not modified or deleted.',
    objectPlaceholder: 'Resource path, e.g. /admin/orders/:id',
    addPolicy: 'Add Policy',
    policiesEmpty: 'No policies',
    tableObject: 'Object',
    tableAction: 'Action',
    tableOperation: 'Operation',
    catalogTitle: 'Permission Catalog',
    catalogHint: 'Search and add API permissions directly',
    catalogSearch: 'Search module/path/method, e.g. orders GET',
    catalogEmpty: 'No available APIs',
    catalogAdd: 'Add',
    catalogAdded: 'Added',
    catalogExpandAll: 'Expand All',
    catalogCollapseAll: 'Collapse All',
    adminManageTitle: 'Admin List',
    adminManageHint: 'Create, edit, and delete admins, then assign roles quickly',
    adminSearchPlaceholder: 'Search by username or ID',
    adminListEmpty: 'No admins',
    adminTableId: 'ID',
    adminTableUsername: 'Username',
    adminTableRoles: 'Roles',
    adminTableSuper: 'Super',
    adminTableTotp: '2FA',
    adminTableCreatedAt: 'Created At',
    adminTableLastLoginAt: 'Last Login',
    adminTotpEnabled: 'Enabled',
    adminTotpDisabled: 'Disabled',
    adminTotpReset: 'Reset 2FA',
    adminTotpResetConfirm: 'Reset 2FA for {name}? They will be forced to sign in again.',
    adminTotpResetSuccess: 'Reset',
    adminTotpResetFailed: 'Reset failed',
    adminRolesEmpty: 'Unassigned',
    adminSelectRoleTarget: 'Set as role target',
    adminRoleTargetSelected: 'Selected',
    adminFormTitleCreate: 'Create Admin',
    adminFormTitleEdit: 'Edit Admin',
    adminUsername: 'Username',
    adminUsernamePlaceholder: 'Enter username (3-64 chars, no spaces)',
    adminPassword: 'Password',
    adminPasswordPlaceholderCreate: 'Enter password',
    adminPasswordPlaceholderEdit: 'Leave empty to keep unchanged',
    adminIsSuper: 'Set as super admin (bypass RBAC checks)',
    adminSubmitCreate: 'Create Admin',
    adminSubmitUpdate: 'Save Changes',
    adminFormReset: 'Reset',
    adminRolesTitle: 'Admin Role Assignment',
    adminLabel: 'Admin',
    selectAdmin: 'Please select admin',
    adminId: 'Admin ID',
    superAdmin: 'Super Admin',
    rolesLabel: 'Check roles to assign to selected admin',
    saveRoles: 'Save Role Assignment',
    confirmDeleteRole: 'Delete role {role}?',
    confirmRevokePolicy: 'Remove policy {action} {object}?',
    confirmDeleteAdmin: 'Delete admin {username}?',
    roleCreated: 'Role created',
    roleDeleted: 'Role deleted',
    policyGranted: 'Policy added',
    policyRevoked: 'Policy removed',
    adminCreated: 'Admin created',
    adminUpdated: 'Admin updated',
    adminDeleted: 'Admin deleted',
    adminRolesSaved: 'Admin roles saved',
    fetchRolesFailed: 'Failed to fetch roles',
    fetchPoliciesFailed: 'Failed to fetch policies',
    fetchCatalogFailed: 'Failed to fetch permission catalog',
    fetchAdminsFailed: 'Failed to fetch admins',
    fetchAdminRolesFailed: 'Failed to fetch admin roles',
    roleRequired: 'Role name is required',
    createRoleFailed: 'Failed to create role',
    deleteRoleFailed: 'Failed to delete role',
    selectRoleFirst: 'Please select role first',
    objectRequired: 'Object path is required',
    grantPolicyFailed: 'Failed to add policy',
    revokePolicyFailed: 'Failed to remove policy',
    selectAdminFirst: 'Please select admin first',
    adminUsernameRequired: 'Username is required',
    adminPasswordRequired: 'Password is required',
    createAdminFailed: 'Failed to create admin',
    updateAdminFailed: 'Failed to update admin',
    deleteAdminFailed: 'Failed to delete admin',
    saveAdminRolesFailed: 'Failed to save admin roles',
  },
}

const text = computed(() => textMap[locale.value as keyof typeof textMap] || textMap['zh-CN'])

const formatText = (template: string, payload: Record<string, string | number>) => {
  return template.replace(/\{(\w+)\}/g, (_, key: string) => String(payload[key] ?? ''))
}

const formatDateTime = (value?: string) => {
  if (!value) {
    return text.value.unknown
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  return date.toLocaleString()
}

const loadingRoles = ref(false)
const loadingPolicies = ref(false)
const loadingCatalog = ref(false)
const loadingAdmins = ref(false)
const savingAdminRoles = ref(false)
const savingAdminForm = ref(false)

const roles = ref<string[]>([])
const selectedRole = ref('')
const newRole = ref('')
const policies = ref<AdminAuthzPolicy[]>([])

const catalogKeyword = ref('')
const permissionCatalog = ref<AdminPermissionCatalogItem[]>([])
const collapsedModules = ref<string[]>([])

const admins = ref<AdminAuthzAdmin[]>([])
const authStore = useAdminAuthStore()
const selectedAdminId = ref<number>(0)
const selectedAdminRoles = ref<string[]>([])
const adminKeyword = ref('')

const adminFormMode = ref<'create' | 'edit'>('create')
const adminForm = reactive({
  id: 0,
  username: '',
  password: '',
  isSuper: false,
})

const policyForm = reactive({
  object: '/admin/',
  action: 'GET',
})

const actionOptions = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', '*']
const immutableRoles = ref<Set<string>>(new Set())
const stripRolePrefix = (role: string) => role.replace(/^role:/, '')
const isRoleImmutable = (role: string) => immutableRoles.value.has(role)

const normalizedRoleOptions = computed(() => roles.value.map((item) => item.trim()).filter((item) => !!item))
const selectedRoleImmutable = computed(() => isRoleImmutable(selectedRole.value))
const selectedAdmin = computed(() => admins.value.find((item) => item.id === selectedAdminId.value) || null)
const adminFormTitle = computed(() => (adminFormMode.value === 'create' ? text.value.adminFormTitleCreate : text.value.adminFormTitleEdit))
const adminSubmitLabel = computed(() => (adminFormMode.value === 'create' ? text.value.adminSubmitCreate : text.value.adminSubmitUpdate))

const filteredAdmins = computed(() => {
  const keyword = adminKeyword.value.trim().toLowerCase()
  if (!keyword) {
    return admins.value
  }
  return admins.value.filter((item) => {
    const haystack = `${item.id} ${item.username}`.toLowerCase()
    return haystack.includes(keyword)
  })
})

const filteredCatalog = computed(() => {
  const keyword = catalogKeyword.value.trim().toLowerCase()
  if (!keyword) {
    return permissionCatalog.value
  }
  return permissionCatalog.value.filter((item) => {
    const haystack = `${item.module} ${item.method} ${item.object} ${item.permission}`.toLowerCase()
    return haystack.includes(keyword)
  })
})

const groupedCatalog = computed(() => {
  const groupMap = new Map<string, AdminPermissionCatalogItem[]>()
  filteredCatalog.value.forEach((item) => {
    const key = (item.module || 'other').trim() || 'other'
    const list = groupMap.get(key) || []
    list.push(item)
    groupMap.set(key, list)
  })
  return Array.from(groupMap.entries())
    .sort((a, b) => a[0].localeCompare(b[0]))
    .map(([module, items]) => ({ module, items }))
})

const isModuleCollapsed = (module: string) => collapsedModules.value.includes(module)

const toggleModule = (module: string) => {
  if (isModuleCollapsed(module)) {
    collapsedModules.value = collapsedModules.value.filter((item) => item !== module)
    return
  }
  collapsedModules.value = [...collapsedModules.value, module]
}

const expandAllModules = () => {
  collapsedModules.value = []
}

const collapseAllModules = () => {
  collapsedModules.value = groupedCatalog.value.map((item) => item.module)
}

const resetAdminForm = () => {
  adminFormMode.value = 'create'
  adminForm.id = 0
  adminForm.username = ''
  adminForm.password = ''
  adminForm.isSuper = false
}

const openAdminEditForm = (admin: AdminAuthzAdmin) => {
  adminFormMode.value = 'edit'
  adminForm.id = admin.id
  adminForm.username = admin.username
  adminForm.password = ''
  adminForm.isSuper = Boolean(admin.is_super)
}

const pickAdminForRoles = (admin: AdminAuthzAdmin) => {
  selectedAdminId.value = admin.id
}

const fetchRoles = async () => {
  loadingRoles.value = true
  try {
    const res = await adminAPI.listAuthzRoles()
    const items: AdminAuthzRole[] = Array.isArray(res.data.data) ? res.data.data : []
    roles.value = items.map((item) => item.role.trim()).filter((item) => !!item)
    immutableRoles.value = new Set(
      items
        .filter((item) => item.immutable)
        .map((item) => item.role.trim())
        .filter((item) => !!item),
    )
    if (selectedRole.value && !roles.value.includes(selectedRole.value)) {
      selectedRole.value = ''
      policies.value = []
    }
    if (!selectedRole.value && roles.value.length) {
      selectedRole.value = roles.value[0] ?? ''
    }
  } catch (err: any) {
    notifyError(err?.message || text.value.fetchRolesFailed)
  } finally {
    loadingRoles.value = false
  }
}

const fetchRolePolicies = async () => {
  if (!selectedRole.value) {
    policies.value = []
    return
  }
  loadingPolicies.value = true
  try {
    const res = await adminAPI.getAuthzRolePolicies(selectedRole.value)
    policies.value = Array.isArray(res.data.data) ? res.data.data : []
  } catch (err: any) {
    policies.value = []
    notifyError(err?.message || text.value.fetchPoliciesFailed)
  } finally {
    loadingPolicies.value = false
  }
}

const fetchPermissionCatalog = async () => {
  loadingCatalog.value = true
  try {
    const res = await adminAPI.listAuthzPermissionCatalog()
    permissionCatalog.value = Array.isArray(res.data.data) ? res.data.data : []
  } catch (err: any) {
    permissionCatalog.value = []
    notifyError(err?.message || text.value.fetchCatalogFailed)
  } finally {
    loadingCatalog.value = false
  }
}

const fetchAdmins = async () => {
  loadingAdmins.value = true
  try {
    const res = await adminAPI.listAuthzAdmins()
    admins.value = Array.isArray(res.data.data) ? res.data.data : []

    if (selectedAdminId.value && !admins.value.some((item) => item.id === selectedAdminId.value)) {
      selectedAdminId.value = 0
    }
    if (!selectedAdminId.value && admins.value.length) {
      selectedAdminId.value = admins.value[0]?.id ?? 0
    }

    if (adminFormMode.value === 'edit' && adminForm.id && !admins.value.some((item) => item.id === adminForm.id)) {
      resetAdminForm()
    }
  } catch (err: any) {
    notifyError(err?.message || text.value.fetchAdminsFailed)
  } finally {
    loadingAdmins.value = false
  }
}

const fetchSelectedAdminRoles = async () => {
  if (!selectedAdminId.value) {
    selectedAdminRoles.value = []
    return
  }
  try {
    const res = await adminAPI.getAuthzAdminRoles(selectedAdminId.value)
    selectedAdminRoles.value = Array.isArray(res.data.data) ? res.data.data : []
  } catch (err: any) {
    selectedAdminRoles.value = []
    notifyError(err?.message || text.value.fetchAdminRolesFailed)
  }
}

const isRoleChecked = (role: string) => selectedAdminRoles.value.includes(role)

const toggleAdminRole = (role: string, v: boolean | 'indeterminate') => {
  const next = new Set(selectedAdminRoles.value)
  if (v === true) {
    next.add(role)
  } else {
    next.delete(role)
  }
  selectedAdminRoles.value = Array.from(next)
}

const hasCoveredPolicy = (item: AdminPermissionCatalogItem) => {
  return policies.value.some((policy) => {
    if (policy.object !== item.object) {
      return false
    }
    if (item.method === '*') {
      return policy.action === '*'
    }
    return policy.action === item.method || policy.action === '*'
  })
}

const handleCreateRole = async () => {
  const role = newRole.value.trim()
  if (!role) {
    notifyError(text.value.roleRequired)
    return
  }
  try {
    await adminAPI.createAuthzRole({ role })
    newRole.value = ''
    await fetchRoles()
    selectedRole.value = `role:${role}`
    await fetchRolePolicies()
    notifySuccess(text.value.roleCreated)
  } catch (err: any) {
    notifyError(err?.message || text.value.createRoleFailed)
  }
}

const handleDeleteRole = async (role: string) => {
  if (isRoleImmutable(role)) {
    notifyError(text.value.immutableRoleHint)
    return
  }
  const confirmed = await confirmAction({
    description: formatText(text.value.confirmDeleteRole, { role: stripRolePrefix(role) }),
    confirmText: text.value.delete,
    variant: 'destructive',
  })
  if (!confirmed) {
    return
  }
  try {
    await adminAPI.deleteAuthzRole(role)
    if (selectedRole.value === role) {
      selectedRole.value = ''
    }
    await fetchRoles()
    await fetchRolePolicies()
    await fetchSelectedAdminRoles()
    notifySuccess(text.value.roleDeleted)
  } catch (err: any) {
    notifyError(err?.message || text.value.deleteRoleFailed)
  }
}

const handleGrantPolicy = async () => {
  if (!selectedRole.value) {
    notifyError(text.value.selectRoleFirst)
    return
  }
  if (selectedRoleImmutable.value) {
    notifyError(text.value.immutableRoleHint)
    return
  }
  const object = policyForm.object.trim()
  if (!object) {
    notifyError(text.value.objectRequired)
    return
  }
  try {
    await adminAPI.grantAuthzPolicy({
      role: selectedRole.value,
      object,
      action: policyForm.action,
    })
    await fetchRolePolicies()
    notifySuccess(text.value.policyGranted)
  } catch (err: any) {
    notifyError(err?.message || text.value.grantPolicyFailed)
  }
}

const handleGrantCatalogPolicy = async (item: AdminPermissionCatalogItem) => {
  if (!selectedRole.value) {
    notifyError(text.value.selectRoleFirst)
    return
  }
  if (selectedRoleImmutable.value) {
    notifyError(text.value.immutableRoleHint)
    return
  }
  if (hasCoveredPolicy(item)) {
    return
  }
  try {
    await adminAPI.grantAuthzPolicy({
      role: selectedRole.value,
      object: item.object,
      action: item.method,
    })
    policyForm.object = item.object
    policyForm.action = item.method
    await fetchRolePolicies()
    notifySuccess(text.value.policyGranted)
  } catch (err: any) {
    notifyError(err?.message || text.value.grantPolicyFailed)
  }
}

const handleRevokePolicy = async (item: AdminAuthzPolicy) => {
  if (selectedRoleImmutable.value) {
    notifyError(text.value.immutableRoleHint)
    return
  }
  const confirmed = await confirmAction({
    description: formatText(text.value.confirmRevokePolicy, { object: item.object, action: item.action }),
    confirmText: text.value.delete,
    variant: 'destructive',
  })
  if (!confirmed) {
    return
  }
  try {
    await adminAPI.revokeAuthzPolicy({
      role: selectedRole.value,
      object: item.object,
      action: item.action,
    })
    await fetchRolePolicies()
    notifySuccess(text.value.policyRevoked)
  } catch (err: any) {
    notifyError(err?.message || text.value.revokePolicyFailed)
  }
}

const handleSubmitAdminForm = async () => {
  const username = adminForm.username.trim()
  if (!username) {
    notifyError(text.value.adminUsernameRequired)
    return
  }

  if (adminFormMode.value === 'create') {
    const password = adminForm.password.trim()
    if (!password) {
      notifyError(text.value.adminPasswordRequired)
      return
    }

    savingAdminForm.value = true
    try {
      const res = await adminAPI.createAuthzAdmin({
        username,
        password,
        is_super: adminForm.isSuper,
      })
      const createdAdminID = Number(res.data?.data?.id || 0)
      await fetchAdmins()
      if (createdAdminID > 0) {
        selectedAdminId.value = createdAdminID
      }
      await fetchSelectedAdminRoles()
      resetAdminForm()
      notifySuccess(text.value.adminCreated)
    } catch (err: any) {
      notifyError(err?.message || text.value.createAdminFailed)
    } finally {
      savingAdminForm.value = false
    }
    return
  }

  if (!adminForm.id) {
    notifyError(text.value.selectAdminFirst)
    return
  }

  savingAdminForm.value = true
  try {
    const payload: { username?: string; password?: string; is_super?: boolean } = {
      username,
      is_super: adminForm.isSuper,
    }
    const password = adminForm.password.trim()
    if (password) {
      payload.password = password
    }

    await adminAPI.updateAuthzAdmin(adminForm.id, payload)
    await fetchAdmins()
    if (selectedAdminId.value === adminForm.id) {
      await fetchSelectedAdminRoles()
    }
    resetAdminForm()
    notifySuccess(text.value.adminUpdated)
  } catch (err: any) {
    notifyError(err?.message || text.value.updateAdminFailed)
  } finally {
    savingAdminForm.value = false
  }
}

const handleDeleteAdmin = async (admin: AdminAuthzAdmin) => {
  const confirmed = await confirmAction({
    description: formatText(text.value.confirmDeleteAdmin, { username: admin.username }),
    confirmText: text.value.delete,
    variant: 'destructive',
  })
  if (!confirmed) {
    return
  }

  try {
    await adminAPI.deleteAuthzAdmin(admin.id)
    if (selectedAdminId.value === admin.id) {
      selectedAdminId.value = 0
      selectedAdminRoles.value = []
    }
    if (adminFormMode.value === 'edit' && adminForm.id === admin.id) {
      resetAdminForm()
    }
    await fetchAdmins()
    if (selectedAdminId.value) {
      await fetchSelectedAdminRoles()
    }
    notifySuccess(text.value.adminDeleted)
  } catch (err: any) {
    notifyError(err?.message || text.value.deleteAdminFailed)
  }
}

const resetAdmin2FA = async (admin: AdminAuthzAdmin) => {
  const confirmed = await confirmAction({
    description: formatText(text.value.adminTotpResetConfirm, { name: admin.username }),
    variant: 'destructive',
  })
  if (!confirmed) return
  try {
    await adminAPI.resetAdmin2FA(admin.id)
    notifySuccess(text.value.adminTotpResetSuccess)
    await fetchAdmins()
  } catch (err: any) {
    if (!err?.__notified) {
      notifyError(text.value.adminTotpResetFailed)
    }
  }
}

const handleSaveAdminRoles = async () => {
  if (!selectedAdminId.value) {
    notifyError(text.value.selectAdminFirst)
    return
  }
  savingAdminRoles.value = true
  try {
    await adminAPI.setAuthzAdminRoles(selectedAdminId.value, { roles: selectedAdminRoles.value })
    await fetchSelectedAdminRoles()
    await fetchAdmins()
    notifySuccess(text.value.adminRolesSaved)
  } catch (err: any) {
    notifyError(err?.message || text.value.saveAdminRolesFailed)
  } finally {
    savingAdminRoles.value = false
  }
}

watch(selectedRole, () => {
  fetchRolePolicies()
})

watch(selectedAdminId, () => {
  fetchSelectedAdminRoles()
})

watch(catalogKeyword, () => {
  expandAllModules()
})

onMounted(async () => {
  await Promise.all([fetchRoles(), fetchAdmins(), fetchPermissionCatalog()])
  await Promise.all([fetchRolePolicies(), fetchSelectedAdminRoles()])
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <h1 class="text-2xl font-semibold">{{ text.title }}</h1>
        <p class="text-sm text-muted-foreground mt-1">{{ text.subtitle }}</p>
      </div>
    </div>

    <section class="rounded-xl border border-border bg-card p-5 space-y-4">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 class="text-base font-medium">{{ text.adminManageTitle }}</h2>
          <p class="text-xs text-muted-foreground mt-1">{{ text.adminManageHint }}</p>
        </div>
        <span class="text-xs text-muted-foreground">{{ admins.length }}</span>
      </div>

      <div class="grid grid-cols-1 xl:grid-cols-[1fr_360px] gap-4">
        <div class="min-w-0 space-y-3">
          <Input v-model="adminKeyword" :placeholder="text.adminSearchPlaceholder" class="h-9" />
          <div class="rounded-lg border border-border overflow-x-auto">
            <Table class="min-w-[900px]">
              <TableHeader class="border-b border-border bg-muted/40 text-xs uppercase text-muted-foreground">
                <TableRow>
                  <TableHead class="px-4 py-3">{{ text.adminTableId }}</TableHead>
                  <TableHead class="min-w-[140px] px-4 py-3">{{ text.adminTableUsername }}</TableHead>
                  <TableHead class="min-w-[160px] px-4 py-3">{{ text.adminTableRoles }}</TableHead>
                  <TableHead class="min-w-[90px] px-4 py-3">{{ text.adminTableSuper }}</TableHead>
                  <TableHead class="min-w-[90px] px-4 py-3">{{ text.adminTableTotp }}</TableHead>
                  <TableHead class="min-w-[140px] px-4 py-3">{{ text.adminTableCreatedAt }}</TableHead>
                  <TableHead class="min-w-[140px] px-4 py-3">{{ text.adminTableLastLoginAt }}</TableHead>
                  <TableHead class="min-w-[160px] px-4 py-3 text-right">{{ text.tableOperation }}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class="divide-y divide-border">
                <TableRow v-if="loadingAdmins">
                  <TableCell colspan="7" class="px-4 py-6 text-center text-muted-foreground">{{ text.loading }}</TableCell>
                </TableRow>
                <TableRow v-else-if="filteredAdmins.length === 0">
                  <TableCell colspan="7" class="px-4 py-6 text-center text-muted-foreground">{{ text.adminListEmpty }}</TableCell>
                </TableRow>
                <TableRow
                  v-for="item in filteredAdmins"
                  :key="item.id"
                  class="hover:bg-muted/30"
                  :class="selectedAdminId === item.id ? 'bg-primary/5' : ''"
                >
                  <TableCell class="px-4 py-3">
                    <IdCell :value="item.id" />
                  </TableCell>
                  <TableCell class="min-w-[140px] px-4 py-3 font-medium break-words">{{ item.username }}</TableCell>
                  <TableCell class="min-w-[160px] px-4 py-3 text-xs text-muted-foreground break-words">
                    <span v-if="item.roles && item.roles.length">{{ item.roles.map((role) => stripRolePrefix(role)).join(', ') }}</span>
                    <span v-else>{{ text.adminRolesEmpty }}</span>
                  </TableCell>
                  <TableCell class="min-w-[90px] px-4 py-3 text-xs">
                    <span :class="item.is_super ? 'text-emerald-600' : 'text-muted-foreground'">{{ item.is_super ? text.yes : text.no }}</span>
                  </TableCell>
                  <TableCell class="min-w-[90px] px-4 py-3 text-xs">
                    <span :class="item.totp_enabled ? 'text-emerald-600' : 'text-muted-foreground'">{{ item.totp_enabled ? text.adminTotpEnabled : text.adminTotpDisabled }}</span>
                  </TableCell>
                  <TableCell class="min-w-[140px] px-4 py-3 text-xs text-muted-foreground">{{ formatDateTime(item.created_at) }}</TableCell>
                  <TableCell class="min-w-[140px] px-4 py-3 text-xs text-muted-foreground">{{ formatDateTime(item.last_login_at) }}</TableCell>
                  <TableCell class="min-w-[160px] px-4 py-3">
                    <div class="flex flex-wrap items-center justify-end gap-2">
                      <Button size="sm" variant="outline" @click="pickAdminForRoles(item)">
                        {{ selectedAdminId === item.id ? text.adminRoleTargetSelected : text.adminSelectRoleTarget }}
                      </Button>
                      <Button size="sm" variant="outline" @click="openAdminEditForm(item)">
                        {{ text.edit }}
                      </Button>
                      <Button
                        v-if="authStore.isSuper && item.totp_enabled"
                        size="sm"
                        variant="outline"
                        @click="resetAdmin2FA(item)"
                      >
                        {{ text.adminTotpReset }}
                      </Button>
                      <Button size="sm" variant="outline" class="text-destructive" @click="handleDeleteAdmin(item)">
                        {{ text.delete }}
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              </TableBody>
            </Table>
          </div>
        </div>

        <div class="rounded-lg border border-border p-4 space-y-3">
          <div class="flex items-center justify-between gap-2">
            <h3 class="text-sm font-medium">{{ adminFormTitle }}</h3>
            <Button size="sm" variant="ghost" @click="resetAdminForm">{{ text.adminFormReset }}</Button>
          </div>
          <div class="space-y-2">
            <label class="text-xs text-muted-foreground">{{ text.adminUsername }}</label>
            <Input v-model="adminForm.username" :placeholder="text.adminUsernamePlaceholder" />
          </div>
          <div class="space-y-2">
            <label class="text-xs text-muted-foreground">{{ text.adminPassword }}</label>
            <Input
              v-model="adminForm.password"
              type="password"
              :placeholder="adminFormMode === 'create' ? text.adminPasswordPlaceholderCreate : text.adminPasswordPlaceholderEdit"
            />
          </div>
          <Label class="flex items-center gap-2 rounded-lg border border-border px-3 py-2 text-sm cursor-pointer">
            <Switch v-model="adminForm.isSuper" />
            <span>{{ text.adminIsSuper }}</span>
          </Label>
          <Button class="w-full" :disabled="savingAdminForm" @click="handleSubmitAdminForm">
            {{ savingAdminForm ? text.loading : adminSubmitLabel }}
          </Button>
        </div>
      </div>
    </section>

    <div class="grid grid-cols-1 xl:grid-cols-3 gap-6">
      <section class="rounded-xl border border-border bg-card p-5 space-y-4">
        <div class="flex items-center justify-between">
          <h2 class="text-base font-medium">{{ text.rolesTitle }}</h2>
          <span class="text-xs text-muted-foreground">{{ roles.length }}</span>
        </div>
        <div class="flex gap-2">
          <Input v-model="newRole" :placeholder="text.rolePlaceholder" />
          <Button @click="handleCreateRole">{{ text.create }}</Button>
        </div>
        <div class="space-y-2 max-h-[420px] overflow-auto pr-1">
          <div v-if="loadingRoles" class="text-sm text-muted-foreground">{{ text.loading }}</div>
          <div v-else-if="roles.length === 0" class="text-sm text-muted-foreground">{{ text.rolesEmpty }}</div>
          <button
            v-for="role in roles"
            :key="role"
            type="button"
            class="w-full rounded-lg border px-3 py-2 text-left text-sm transition-colors"
            :class="selectedRole === role ? 'border-primary bg-primary/5 text-foreground' : 'border-border hover:bg-muted/40 text-muted-foreground'"
            @click="selectedRole = role"
          >
            <div class="flex items-center justify-between gap-2">
              <span class="font-medium">{{ stripRolePrefix(role) }}</span>
              <Button
                size="sm"
                variant="ghost"
                class="h-7 px-2 text-destructive"
                :disabled="isRoleImmutable(role)"
                @click.stop="handleDeleteRole(role)"
              >
                {{ text.delete }}
              </Button>
            </div>
          </button>
        </div>
      </section>

      <section class="min-w-0 rounded-xl border border-border bg-card p-5 space-y-4 xl:col-span-2">
        <div class="flex items-center justify-between">
          <h2 class="text-base font-medium">{{ text.policiesTitle }}</h2>
          <span class="text-xs text-muted-foreground" v-if="selectedRole">{{ stripRolePrefix(selectedRole) }}</span>
        </div>

        <div v-if="!selectedRole" class="text-sm text-muted-foreground">{{ text.selectRoleHint }}</div>

        <template v-else>
          <div v-if="selectedRoleImmutable" class="rounded-lg border border-border bg-muted/40 px-3 py-2 text-sm text-muted-foreground">
            {{ text.immutableRoleHint }}
          </div>
          <div class="grid grid-cols-1 md:grid-cols-[1fr_140px_auto] gap-2">
            <Input v-model="policyForm.object" :placeholder="text.objectPlaceholder" :disabled="selectedRoleImmutable" />
            <Select v-model="policyForm.action" :disabled="selectedRoleImmutable">
              <SelectTrigger class="h-10">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="action in actionOptions" :key="action" :value="action">{{ action }}</SelectItem>
              </SelectContent>
            </Select>
            <Button :disabled="selectedRoleImmutable" @click="handleGrantPolicy">{{ text.addPolicy }}</Button>
          </div>

          <div class="rounded-lg border border-border p-3 space-y-3">
            <div class="flex items-center justify-between gap-2">
              <div>
                <div class="text-sm font-medium">{{ text.catalogTitle }}</div>
                <div class="text-xs text-muted-foreground">{{ text.catalogHint }}</div>
              </div>
              <div class="text-xs text-muted-foreground">{{ filteredCatalog.length }}/{{ permissionCatalog.length }}</div>
            </div>
            <div class="grid grid-cols-1 md:grid-cols-[1fr_auto_auto] gap-2">
              <Input v-model="catalogKeyword" :placeholder="text.catalogSearch" class="h-9" />
              <Button size="sm" variant="outline" @click="expandAllModules">{{ text.catalogExpandAll }}</Button>
              <Button size="sm" variant="outline" @click="collapseAllModules">{{ text.catalogCollapseAll }}</Button>
            </div>
            <div class="max-h-56 overflow-auto space-y-2 pr-1">
              <div v-if="loadingCatalog" class="text-xs text-muted-foreground">{{ text.loading }}</div>
              <div v-else-if="groupedCatalog.length === 0" class="text-xs text-muted-foreground">{{ text.catalogEmpty }}</div>
              <div v-for="group in groupedCatalog" :key="group.module" class="space-y-2">
                <button
                  type="button"
                  class="w-full flex items-center justify-between rounded-lg border border-border bg-muted/30 px-3 py-2 text-left"
                  @click="toggleModule(group.module)"
                >
                  <span class="text-xs font-semibold uppercase tracking-wide">{{ group.module }}</span>
                  <span class="text-xs text-muted-foreground">{{ group.items.length }} · {{ isModuleCollapsed(group.module) ? '+' : '-' }}</span>
                </button>
                <div v-if="!isModuleCollapsed(group.module)" class="space-y-2 pl-1">
                  <div
                    v-for="item in group.items"
                    :key="item.permission"
                    class="flex items-center justify-between rounded-lg border border-border px-3 py-2"
                  >
                    <div class="min-w-0">
                      <div class="text-[11px] text-muted-foreground uppercase tracking-wide">{{ item.module }}</div>
                      <div class="break-all text-xs font-mono">{{ item.method }} {{ item.object }}</div>
                    </div>
                    <Button
                      size="sm"
                      :variant="hasCoveredPolicy(item) ? 'outline' : 'default'"
                      :disabled="selectedRoleImmutable || hasCoveredPolicy(item)"
                      @click="handleGrantCatalogPolicy(item)"
                    >
                      {{ hasCoveredPolicy(item) ? text.catalogAdded : text.catalogAdd }}
                    </Button>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div class="rounded-lg border border-border overflow-x-auto">
            <Table class="min-w-[720px]">
              <TableHeader class="border-b border-border bg-muted/40 text-xs uppercase text-muted-foreground">
                <TableRow>
                  <TableHead class="min-w-[320px] px-4 py-3">{{ text.tableObject }}</TableHead>
                  <TableHead class="min-w-[140px] px-4 py-3">{{ text.tableAction }}</TableHead>
                  <TableHead class="min-w-[140px] px-4 py-3 text-right">{{ text.tableOperation }}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class="divide-y divide-border">
                <TableRow v-if="loadingPolicies">
                  <TableCell colspan="3" class="px-4 py-6 text-center text-muted-foreground">{{ text.loading }}</TableCell>
                </TableRow>
                <TableRow v-else-if="policies.length === 0">
                  <TableCell colspan="3" class="px-4 py-6 text-center text-muted-foreground">{{ text.policiesEmpty }}</TableCell>
                </TableRow>
                <TableRow v-for="item in policies" :key="`${item.object}:${item.action}`" class="hover:bg-muted/30">
                  <TableCell class="min-w-[320px] px-4 py-3 font-mono text-xs text-muted-foreground break-all">{{ item.object }}</TableCell>
                  <TableCell class="min-w-[140px] px-4 py-3 font-medium break-words">{{ item.action }}</TableCell>
                  <TableCell class="min-w-[140px] px-4 py-3 text-right">
                    <Button size="sm" variant="outline" :disabled="selectedRoleImmutable" @click="handleRevokePolicy(item)">
                      {{ text.delete }}
                    </Button>
                  </TableCell>
                </TableRow>
              </TableBody>
            </Table>
          </div>
        </template>
      </section>
    </div>

    <section class="rounded-xl border border-border bg-card p-5 space-y-4">
      <div class="flex items-center justify-between">
        <h2 class="text-base font-medium">{{ text.adminRolesTitle }}</h2>
      </div>

      <div class="grid grid-cols-1 lg:grid-cols-[320px_1fr] gap-4">
        <div class="space-y-2">
          <label class="text-sm text-muted-foreground">{{ text.adminLabel }}</label>
          <Select :model-value="String(selectedAdminId)" @update:model-value="selectedAdminId = Number($event)">
            <SelectTrigger class="h-10 w-full">
              <SelectValue :placeholder="text.selectAdmin" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="0">{{ text.selectAdmin }}</SelectItem>
              <SelectItem v-for="admin in admins" :key="admin.id" :value="String(admin.id)">
                {{ admin.username }}{{ admin.is_super ? ' (super)' : '' }}
              </SelectItem>
            </SelectContent>
          </Select>
          <div v-if="loadingAdmins" class="text-xs text-muted-foreground">{{ text.loading }}</div>
          <div v-else-if="selectedAdmin" class="rounded-lg border border-border px-3 py-2 text-xs text-muted-foreground space-y-1">
            <div class="flex items-center justify-between">
              <span>{{ text.adminId }}</span>
              <IdCell :value="selectedAdmin.id" />
            </div>
            <div class="flex items-center justify-between">
              <span>{{ text.superAdmin }}</span>
              <span>{{ selectedAdmin.is_super ? text.yes : text.no }}</span>
            </div>
          </div>
        </div>

        <div class="min-w-0 space-y-3">
          <div class="text-sm text-muted-foreground">{{ text.rolesLabel }}</div>
          <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-2">
            <Label
              v-for="role in normalizedRoleOptions"
              :key="`admin-role-${role}`"
              class="flex items-center gap-2 rounded-lg border border-border px-3 py-2 text-sm cursor-pointer"
            >
              <Checkbox
                :model-value="isRoleChecked(role)"
                @update:model-value="(v) => toggleAdminRole(role, v)"
              />
              <span>{{ stripRolePrefix(role) }}</span>
            </Label>
          </div>
          <div class="flex items-center justify-end">
            <Button :disabled="savingAdminRoles" @click="handleSaveAdminRoles">
              {{ savingAdminRoles ? text.loading : text.saveRoles }}
            </Button>
          </div>
        </div>
      </div>
    </section>
    <AdminAPITokens
      v-if="selectedAdmin && authStore.hasPermission(`GET:/admin/authz/admins/${selectedAdmin.id}/api-tokens`)"
      :key="selectedAdmin.id"
      :admin="selectedAdmin"
    />
  </div>
</template>
