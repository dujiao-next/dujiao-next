<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AdminBanner, LocalizedText } from '@/api/types'
import IdCell from '@/components/IdCell.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Dialog, DialogHeader, DialogScrollContent, DialogTitle } from '@/components/ui/dialog'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import TableSkeleton from '@/components/TableSkeleton.vue'
import ListPagination from '@/components/ListPagination.vue'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import MediaPicker from '@/components/admin/MediaPicker.vue'
import { getImageUrl } from '@/utils/image'
import { notifyError } from '@/utils/notify'
import { confirmAction } from '@/utils/confirm'
import { useFormValidation, rules } from '@/composables/useFormValidation'

const { t } = useI18n()
const route = useRoute()
const loading = ref(false)
const submitting = ref(false)
const showModal = ref(false)
const isEditing = ref(false)
const currentLang = ref('zh-CN')

const languages = computed(() => [
  { code: 'zh-CN', name: t('admin.common.lang.zhCN') },
  { code: 'zh-TW', name: t('admin.common.lang.zhTW') },
  { code: 'en-US', name: t('admin.common.lang.enUS') },
])

const linkTypes = computed(() => [
  { label: t('admin.banners.linkTypes.none'), value: 'none' },
  { label: t('admin.banners.linkTypes.internal'), value: 'internal' },
  { label: t('admin.banners.linkTypes.external'), value: 'external' },
])

const activeFilterOptions = computed(() => [
  { label: t('admin.banners.filters.statusAll'), value: '__all__' },
  { label: t('admin.common.enabled'), value: 'true' },
  { label: t('admin.common.disabled'), value: 'false' },
])

const positionOptions = computed(() => [
  { label: t('admin.banners.positions.homeHero'), value: 'home_hero' },
])

const banners = ref<AdminBanner[]>([])
const pagination = reactive({
  page: 1,
  page_size: 10,
  total: 0,
  total_page: 0,
})
const filters = reactive({
  search: '',
  position: 'home_hero',
  isActive: '__all__',
})

const form = reactive({
  id: 0,
  name: '',
  position: 'home_hero',
  title: { 'zh-CN': '', 'zh-TW': '', 'en-US': '' } as LocalizedText,
  subtitle: { 'zh-CN': '', 'zh-TW': '', 'en-US': '' } as LocalizedText,
  cta_text: { 'zh-CN': '', 'zh-TW': '', 'en-US': '' } as LocalizedText,
  buy_text: { 'zh-CN': '', 'zh-TW': '', 'en-US': '' } as LocalizedText,
  buy_link: '',
  tone: 'auto' as 'auto' | 'dark' | 'light',
  image: '',
  mobile_image: '',
  link_type: 'none',
  link_value: '',
  open_in_new_tab: false,
  is_active: true,
  start_at: '',
  end_at: '',
  sort_order: 0,
})

const { errors, validate, clearErrors } = useFormValidation({
  name: [rules.required('This field is required')],
  position: [rules.required('This field is required')],
  image: [rules.required('This field is required')],
})

const getLocalizedText = (jsonData: LocalizedText | null | undefined) => {
  if (!jsonData) return ''
  return jsonData[currentLang.value] || jsonData['zh-CN'] || jsonData['en-US'] || ''
}

const getCurrentLangName = () => {
  return languages.value.find((item) => item.code === currentLang.value)?.name || t('admin.common.lang.zhCN')
}

const getPositionLabel = (value: string) => {
  return positionOptions.value.find((item) => item.value === value)?.label || value
}

const getLinkTypeLabel = (value: string) => {
  return linkTypes.value.find((item) => item.value === value)?.label || value
}

const toISO = (raw: string) => {
  if (!raw) return ''
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return ''
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

const normalizeFilterValue = (value: string) => (value === '__all__' ? '' : value)

const fetchBanners = async (page = 1) => {
  loading.value = true
  try {
    const normalizedIsActive = normalizeFilterValue(filters.isActive)
    const isActiveValue = normalizedIsActive === '' ? undefined : normalizedIsActive === 'true'
    const res = await adminAPI.getBanners({
      page,
      page_size: pagination.page_size,
      search: filters.search || undefined,
      position: filters.position || undefined,
      is_active: isActiveValue,
    })
    banners.value = res.data.data || []
    const p = res.data.pagination
    if (p) {
      pagination.page = p.page
      pagination.page_size = p.page_size
      pagination.total = p.total
      pagination.total_page = p.total_page
    }
  } catch {
    banners.value = []
  } finally {
    loading.value = false
  }
}

const handleSearch = () => {
  fetchBanners(1)
}
const debouncedSearch = useDebounceFn(handleSearch, 300)

const changePage = (page: number) => {
  if (page < 1 || page > pagination.total_page) return
  fetchBanners(page)
}

const pageSizeOptions = [10, 20, 50, 100]

const changePageSize = (size: number) => {
  if (size === pagination.page_size) return
  pagination.page_size = size
  fetchBanners(1)
}

const resetForm = () => {
  currentLang.value = 'zh-CN'
  Object.assign(form, {
    id: 0,
    name: '',
    position: 'home_hero',
    title: { 'zh-CN': '', 'zh-TW': '', 'en-US': '' },
    subtitle: { 'zh-CN': '', 'zh-TW': '', 'en-US': '' },
    cta_text: { 'zh-CN': '', 'zh-TW': '', 'en-US': '' },
    buy_text: { 'zh-CN': '', 'zh-TW': '', 'en-US': '' },
    buy_link: '',
    tone: 'auto',
    image: '',
    mobile_image: '',
    link_type: 'none',
    link_value: '',
    open_in_new_tab: false,
    is_active: true,
    start_at: '',
    end_at: '',
    sort_order: 0,
  })
}

const openCreateModal = () => {
  isEditing.value = false
  resetForm()
  clearErrors()
  showModal.value = true
}

const openEditModal = (banner: AdminBanner) => {
  isEditing.value = true
  currentLang.value = 'zh-CN'
  Object.assign(form, {
    id: banner.id,
    name: banner.name || '',
    position: banner.position || 'home_hero',
    title: {
      'zh-CN': banner.title?.['zh-CN'] || '',
      'zh-TW': banner.title?.['zh-TW'] || '',
      'en-US': banner.title?.['en-US'] || '',
    },
    subtitle: {
      'zh-CN': banner.subtitle?.['zh-CN'] || '',
      'zh-TW': banner.subtitle?.['zh-TW'] || '',
      'en-US': banner.subtitle?.['en-US'] || '',
    },
    cta_text: {
      'zh-CN': (banner as any).cta_text?.['zh-CN'] || '',
      'zh-TW': (banner as any).cta_text?.['zh-TW'] || '',
      'en-US': (banner as any).cta_text?.['en-US'] || '',
    },
    buy_text: {
      'zh-CN': (banner as any).buy_text?.['zh-CN'] || '',
      'zh-TW': (banner as any).buy_text?.['zh-TW'] || '',
      'en-US': (banner as any).buy_text?.['en-US'] || '',
    },
    buy_link: (banner as any).buy_link || '',
    tone: ((banner as any).tone === 'dark' || (banner as any).tone === 'light' ? (banner as any).tone : 'auto') as 'auto' | 'dark' | 'light',
    image: banner.image || '',
    mobile_image: banner.mobile_image || '',
    link_type: banner.link_type || 'none',
    link_value: banner.link_value || '',
    open_in_new_tab: Boolean(banner.open_in_new_tab),
    is_active: Boolean(banner.is_active),
    start_at: toLocalInput(banner.start_at),
    end_at: toLocalInput(banner.end_at),
    sort_order: banner.sort_order || 0,
  })
  showModal.value = true
}

const closeModal = () => {
  showModal.value = false
  clearErrors()
}

const buildPayload = () => ({
  name: form.name,
  position: form.position,
  title: form.title,
  subtitle: form.subtitle,
  cta_text: form.cta_text,
  buy_text: form.buy_text,
  buy_link: form.buy_link,
  tone: form.tone === 'auto' ? '' : form.tone,
  image: form.image,
  mobile_image: form.mobile_image,
  link_type: form.link_type,
  link_value: form.link_type === 'none' ? '' : form.link_value,
  open_in_new_tab: form.open_in_new_tab,
  is_active: form.is_active,
  start_at: form.start_at ? toISO(form.start_at) : '',
  end_at: form.end_at ? toISO(form.end_at) : '',
  sort_order: Number(form.sort_order || 0),
})

const handleSubmit = async () => {
  if (!validate({ name: form.name, position: form.position, image: form.image } as Record<string, unknown>)) return
  submitting.value = true
  try {
    const payload = buildPayload()
    if (isEditing.value && form.id) {
      await adminAPI.updateBanner(form.id, payload)
    } else {
      await adminAPI.createBanner(payload)
    }
    closeModal()
    fetchBanners(1)
  } catch (err: any) {
    notifyError(t('admin.banners.errors.operationFailed', { message: err?.message || '' }))
  } finally {
    submitting.value = false
  }
}

const handleDelete = async (banner: AdminBanner) => {
  const confirmed = await confirmAction({
    description: t('admin.banners.confirmDelete', { name: banner.name || '#' + banner.id }),
    confirmText: t('admin.common.delete'),
    variant: 'destructive',
  })
  if (!confirmed) return
  try {
    await adminAPI.deleteBanner(banner.id)
    fetchBanners(pagination.page)
  } catch (err: any) {
    notifyError(t('admin.banners.errors.deleteFailed', { message: err?.message || '' }))
  }
}

const openEditById = async (rawId: unknown) => {
  const id = Number(rawId)
  if (!Number.isFinite(id) || id <= 0) return
  try {
    const res = await adminAPI.getBanner(id)
    openEditModal(res.data.data)
  } catch (err: any) {
    notifyError(t('admin.banners.errors.operationFailed', { message: err?.message || '' }))
  }
}

onMounted(() => {
  fetchBanners()
  if (route.query.banner_id) {
    openEditById(route.query.banner_id)
  }
})

watch(
  () => route.query.banner_id,
  (value) => {
    if (value) {
      openEditById(value)
    }
  }
)
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <h1 class="text-2xl font-semibold">{{ t('admin.banners.title') }}</h1>
      <Button class="w-full sm:w-auto" @click="openCreateModal">{{ t('admin.banners.create') }}</Button>
    </div>

    <div class="rounded-xl border border-border bg-card p-4 shadow-sm">
      <div class="flex flex-wrap items-center gap-3">
        <div class="w-full md:w-72">
          <Input v-model="filters.search" :placeholder="t('admin.banners.searchPlaceholder')" @update:modelValue="debouncedSearch" />
        </div>
        <Select v-model="filters.position" @update:modelValue="handleSearch">
          <SelectTrigger class="h-9 w-full md:w-[220px]">
            <SelectValue :placeholder="t('admin.banners.filters.positionPlaceholder')" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem v-for="item in positionOptions" :key="item.value" :value="item.value">{{ item.label }}</SelectItem>
          </SelectContent>
        </Select>
        <Select v-model="filters.isActive" @update:modelValue="handleSearch">
          <SelectTrigger class="h-9 w-full md:w-[180px]">
            <SelectValue :placeholder="t('admin.banners.filters.statusPlaceholder')" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem v-for="item in activeFilterOptions" :key="item.value" :value="item.value">{{ item.label }}</SelectItem>
          </SelectContent>
        </Select>
      </div>
    </div>

    <div class="rounded-xl border border-border bg-card overflow-x-auto">
      <Table class="min-w-[900px]">
        <TableHeader class="border-b border-border bg-muted/40 text-xs uppercase text-muted-foreground">
          <TableRow>
            <TableHead class="px-6 py-3">{{ t('admin.banners.table.id') }}</TableHead>
            <TableHead class="min-w-[90px] px-6 py-3">{{ t('admin.banners.table.image') }}</TableHead>
            <TableHead class="min-w-[140px] px-6 py-3">{{ t('admin.banners.table.name') }}</TableHead>
            <TableHead class="min-w-[90px] px-6 py-3">{{ t('admin.banners.table.position') }}</TableHead>
            <TableHead class="min-w-[90px] px-6 py-3">{{ t('admin.banners.table.linkType') }}</TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.banners.table.sort') }}</TableHead>
            <TableHead class="min-w-[90px] px-6 py-3">{{ t('admin.banners.table.status') }}</TableHead>
            <TableHead class="min-w-[140px] px-6 py-3 text-right">{{ t('admin.banners.table.action') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody class="divide-y divide-border">
          <TableRow v-if="loading">
            <TableCell :colspan="8" class="p-0">
              <TableSkeleton :columns="8" :rows="5" />
            </TableCell>
          </TableRow>
          <TableRow v-else-if="banners.length === 0">
            <TableCell colspan="8" class="px-6 py-8 text-center text-muted-foreground">{{ t('admin.banners.empty') }}</TableCell>
          </TableRow>
          <TableRow v-for="banner in banners" :key="banner.id" class="hover:bg-muted/30">
            <TableCell class="px-6 py-4">
              <IdCell :value="banner.id" />
            </TableCell>
            <TableCell class="min-w-[90px] px-6 py-4">
              <img v-if="banner.image" :src="getImageUrl(banner.image)" class="h-14 w-28 shrink-0 rounded object-cover" />
            </TableCell>
            <TableCell class="min-w-[140px] px-6 py-4">
              <div class="break-words font-medium text-foreground">{{ banner.name }}</div>
              <div class="break-words text-xs text-muted-foreground">{{ getLocalizedText(banner.title) }}</div>
            </TableCell>
            <TableCell class="min-w-[90px] px-6 py-4 text-xs text-muted-foreground break-words">{{ getPositionLabel(banner.position) }}</TableCell>
            <TableCell class="min-w-[90px] px-6 py-4 text-xs text-muted-foreground break-words">{{ getLinkTypeLabel(banner.link_type) }}</TableCell>
            <TableCell class="px-6 py-4 text-xs text-muted-foreground">{{ banner.sort_order || 0 }}</TableCell>
            <TableCell class="min-w-[90px] px-6 py-4">
              <span class="inline-flex rounded-full border px-2.5 py-1 text-xs" :class="banner.is_active ? 'text-emerald-700 border-emerald-200 bg-emerald-50' : 'text-muted-foreground border-border bg-muted/30'">
                {{ banner.is_active ? t('admin.common.enabled') : t('admin.common.disabled') }}
              </span>
            </TableCell>
            <TableCell class="min-w-[140px] px-6 py-4 text-right">
              <div class="flex flex-wrap items-center justify-end gap-2">
                <Button size="sm" variant="outline" @click="openEditModal(banner)">{{ t('admin.banners.actions.edit') }}</Button>
                <Button size="sm" variant="destructive" @click="handleDelete(banner)">{{ t('admin.banners.actions.delete') }}</Button>
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
      <DialogScrollContent class="w-[calc(100vw-1rem)] max-w-4xl p-4 sm:p-6" @interact-outside="(e) => e.preventDefault()">
        <DialogHeader>
          <DialogTitle>{{ isEditing ? t('admin.banners.modal.editTitle') : t('admin.banners.modal.createTitle') }}</DialogTitle>
        </DialogHeader>

        <form class="space-y-6" @submit.prevent="handleSubmit">
          <div class="border-b border-border">
            <div class="flex gap-4 overflow-x-auto pb-1">
              <button
                v-for="lang in languages"
                :key="lang.code"
                type="button"
                class="shrink-0 border-b-2 px-4 py-2 text-sm font-medium"
                :class="currentLang === lang.code ? 'border-primary text-foreground' : 'border-transparent text-muted-foreground hover:text-foreground'"
                @click="currentLang = lang.code"
              >
                {{ lang.name }}
              </button>
            </div>
          </div>

          <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
            <div>
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('admin.banners.form.name') }}</label>
              <Input v-model="form.name" :placeholder="t('admin.banners.form.namePlaceholder')" />
              <p v-if="errors.name" class="text-xs text-destructive mt-1">{{ errors.name }}</p>
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('admin.banners.form.position') }}</label>
              <Select v-model="form.position">
                <SelectTrigger class="h-9 w-full">
                  <SelectValue :placeholder="t('admin.banners.form.positionPlaceholder')" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="item in positionOptions" :key="item.value" :value="item.value">{{ item.label }}</SelectItem>
                </SelectContent>
              </Select>
              <p v-if="errors.position" class="text-xs text-destructive mt-1">{{ errors.position }}</p>
            </div>

            <div class="md:col-span-2">
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('admin.banners.form.title', { lang: getCurrentLangName() }) }}</label>
              <Input v-model="form.title[currentLang]" :placeholder="t('admin.banners.form.titlePlaceholder')" />
            </div>

            <div class="md:col-span-2">
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('admin.banners.form.subtitle', { lang: getCurrentLangName() }) }}</label>
              <Input v-model="form.subtitle[currentLang]" :placeholder="t('admin.banners.form.subtitlePlaceholder')" />
            </div>

            <div>
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('admin.banners.form.ctaText', { lang: getCurrentLangName() }) }}</label>
              <Input v-model="form.cta_text[currentLang]" :placeholder="t('admin.banners.form.ctaTextPlaceholder')" />
            </div>

            <div>
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('admin.banners.form.buyText', { lang: getCurrentLangName() }) }}</label>
              <Input v-model="form.buy_text[currentLang]" :placeholder="t('admin.banners.form.buyTextPlaceholder')" />
            </div>

            <div>
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('admin.banners.form.buyLink') }}</label>
              <Input v-model="form.buy_link" :placeholder="t('admin.banners.form.buyLinkPlaceholder')" />
            </div>

            <div>
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('admin.banners.form.tone') }}</label>
              <Select v-model="form.tone">
                <SelectTrigger class="h-9 w-full"><SelectValue :placeholder="t('admin.banners.form.toneAuto')" /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="auto">{{ t('admin.banners.form.toneAuto') }}</SelectItem>
                  <SelectItem value="dark">{{ t('admin.banners.form.toneDark') }}</SelectItem>
                  <SelectItem value="light">{{ t('admin.banners.form.toneLight') }}</SelectItem>
                </SelectContent>
              </Select>
              <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.banners.form.toneHint') }}</p>
            </div>

            <div class="md:col-span-2">
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('admin.banners.form.image') }}</label>
              <MediaPicker v-model="form.image" scene="banner" />
              <p v-if="errors.image" class="text-xs text-destructive mt-1">{{ errors.image }}</p>
            </div>

            <div class="md:col-span-2">
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('admin.banners.form.mobileImage') }}</label>
              <MediaPicker v-model="form.mobile_image" scene="banner" />
              <Input v-model="form.mobile_image" class="mt-2" :placeholder="t('admin.banners.form.mobileImagePlaceholder')" />
            </div>

            <div>
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('admin.banners.form.linkType') }}</label>
              <Select v-model="form.link_type">
                <SelectTrigger class="h-9 w-full">
                  <SelectValue :placeholder="t('admin.banners.form.linkTypePlaceholder')" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="item in linkTypes" :key="item.value" :value="item.value">{{ item.label }}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('admin.banners.form.linkValue') }}</label>
              <Input v-model="form.link_value" :disabled="form.link_type === 'none'" :placeholder="t('admin.banners.form.linkValuePlaceholder')" />
            </div>

            <div>
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('admin.banners.form.startAt') }}</label>
              <Input v-model="form.start_at" type="datetime-local" />
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('admin.banners.form.endAt') }}</label>
              <Input v-model="form.end_at" type="datetime-local" />
            </div>

            <div>
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('admin.banners.form.sortOrder') }}</label>
              <Input v-model.number="form.sort_order" type="number" placeholder="0" />
            </div>

            <div class="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center sm:gap-6">
              <Label class="inline-flex items-center gap-2 text-sm text-muted-foreground cursor-pointer">
                <Switch v-model="form.is_active" />
                {{ t('admin.banners.form.activeNow') }}
              </Label>
              <Label class="inline-flex items-center gap-2 text-sm text-muted-foreground cursor-pointer">
                <Switch v-model="form.open_in_new_tab" />
                {{ t('admin.banners.form.openInNewTab') }}
              </Label>
            </div>
          </div>

          <div class="flex flex-col-reverse gap-3 border-t border-border pt-6 sm:flex-row sm:justify-end">
            <Button type="button" variant="outline" class="w-full sm:w-auto" @click="closeModal">{{ t('admin.common.cancel') }}</Button>
            <Button type="submit" class="w-full sm:w-auto" :disabled="submitting">{{ isEditing ? t('admin.banners.actions.saveChanges') : t('admin.banners.actions.createNow') }}</Button>
          </div>
        </form>
      </DialogScrollContent>
    </Dialog>
  </div>
</template>
