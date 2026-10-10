<template>
  <Card :class="embedded ? 'p-5 sm:p-6' : 'p-6 sm:p-7'">
    <Alert
      v-if="panelAlert"
      class="mb-5"
      :variant="panelAlert.level === 'error' ? 'destructive' : 'default'"
      :class="panelAlert.level === 'success' ? 'border-success/40 text-success' : ''"
    >
      <AlertDescription>{{ panelAlert.message }}</AlertDescription>
    </Alert>

    <div class="mb-6 flex flex-col gap-4 md:flex-row md:items-center" :class="embedded ? 'md:justify-end' : 'md:justify-between'">
      <div v-if="!embedded">
        <h2 class="text-xl font-bold text-foreground">{{ t('personalCenter.reseller.productSettings.title') }}</h2>
        <p class="mt-1 text-sm text-muted-foreground">{{ t('personalCenter.reseller.productSettings.subtitle') }}</p>
      </div>
      <Button type="button" variant="outline" size="sm" :disabled="loading" @click="loadRows(pagination.page)">
        {{ t('orders.filters.refresh') }}
      </Button>
    </div>

    <div class="mb-4 grid grid-cols-1 gap-3 md:grid-cols-[1fr_auto]">
      <Input
        v-model.trim="filters.keyword"
        type="text"
        :placeholder="t('personalCenter.reseller.productSettings.searchPlaceholder')"
        @keyup.enter="searchRows"
      />
      <Button type="button" :disabled="loading" @click="searchRows">
        {{ t('orders.filters.search') }}
      </Button>
    </div>

    <div v-if="loading" class="space-y-3">
      <div v-for="idx in 3" :key="idx" class="h-20 animate-pulse rounded-xl border bg-muted"></div>
    </div>

    <div v-else-if="rows.length === 0" class="rounded-xl border border-dashed px-4 py-8 text-sm text-muted-foreground">
      {{ t('personalCenter.reseller.productSettings.empty') }}
    </div>

    <div v-else class="space-y-3">
      <div
        v-for="row in rows"
        :id="`reseller-product-row-${row.product.id}`"
        :key="row.product.id"
        class="rounded-xl border bg-card p-4 shadow-sm"
      >
        <div class="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <div class="truncate font-bold text-foreground">{{ getProductTitle(row.product.title) }}</div>
              <Badge :variant="countListedSkus(row) > 0 ? 'success' : 'neutral'" size="xs">
                {{ countListedSkus(row) > 0 ? t('personalCenter.reseller.productSettings.displayed') : t('personalCenter.reseller.productSettings.hidden') }}
              </Badge>
            </div>
            <div class="mt-1 break-all text-xs text-muted-foreground">#{{ row.product.id }} / {{ row.product.slug }}</div>
          </div>
          <Button
            type="button"
            size="sm"
            :disabled="detailLoading || saving"
            @click="openEditor(row.product.id)"
          >
            {{ t('personalCenter.reseller.productSettings.edit') }}
          </Button>
        </div>
        <div class="mt-4 grid gap-3 sm:grid-cols-3">
          <div class="rounded-lg border bg-muted/30 px-3 py-2">
            <div class="text-[11px] font-semibold uppercase tracking-[0.12em] text-muted-foreground">{{ t('personalCenter.reseller.productSettings.basePrice') }}</div>
            <div class="mt-1 font-mono text-sm font-bold text-foreground">{{ summarizeProductBasePrice(row) }}</div>
          </div>
          <div class="rounded-lg border border-primary/20 bg-primary/5 px-3 py-2">
            <div class="text-[11px] font-semibold uppercase tracking-[0.12em] text-primary">{{ t('personalCenter.reseller.productSettings.effectivePrice') }}</div>
            <div class="mt-1 font-mono text-sm font-bold text-primary">{{ summarizeProductEffectivePrice(row) }}</div>
          </div>
          <div class="rounded-lg border bg-muted/30 px-3 py-2">
            <div class="text-[11px] font-semibold uppercase tracking-[0.12em] text-muted-foreground">{{ t('personalCenter.reseller.productSettings.listedSkuCount') }}</div>
            <div class="mt-1 font-mono text-sm font-bold text-foreground">{{ countListedSkus(row) }} / {{ countActiveSkus(row) }}</div>
          </div>
        </div>

        <div
          v-if="editing?.product.id === row.product.id"
          data-testid="reseller-product-editor"
          class="mt-4 rounded-xl border bg-muted/30 p-4"
        >
          <div class="mb-4 flex items-center justify-between gap-3">
            <div class="min-w-0">
              <h3 class="truncate text-base font-bold text-foreground">{{ t('personalCenter.reseller.productSettings.editorTitle') }}</h3>
              <div class="mt-1 flex flex-wrap items-center gap-2">
                <span class="truncate text-xs text-muted-foreground">{{ getProductTitle(editing.product.title) }}</span>
                <Badge variant="accent" size="xs">{{ t('personalCenter.reseller.productSettings.effectivePrice') }} {{ summarizeProductEffectivePrice(editing) }}</Badge>
              </div>
            </div>
            <Button type="button" variant="outline" size="sm" @click="closeEditor">
              {{ t('common.cancel') }}
            </Button>
          </div>

          <div v-if="detailLoading" class="h-24 animate-pulse rounded-xl border bg-muted"></div>
          <div v-else class="space-y-4">
            <ResellerProductRuleEditor
              v-model="productForm"
              :label="t('personalCenter.reseller.productSettings.productLevelRule')"
              :base-price="editing.product.agency_price_amount || editing.product.price_amount"
              :master-price="editing.product.agency_price_amount ? editing.product.price_amount : undefined"
              :effective-price="previewEffectiveFor(0, summarizeEffectivePrice(editing.product_setting))"
              :invalid="previewInvalidFor(0)"
              :error-code="previewErrorFor(0)"
            />
            <div v-for="sku in editing.skus" :key="sku.id" class="rounded-xl border bg-card p-3">
              <ResellerProductRuleEditor
                :model-value="skuFormFor(sku.id)"
                :label="buildSkuLabel(sku)"
                :base-price="sku.base_price_amount"
                :master-price="sku.master_price_amount"
                :effective-price="previewEffectiveFor(sku.id, summarizeSkuEffectivePrice(sku))"
                :invalid="previewInvalidFor(sku.id)"
                :error-code="previewErrorFor(sku.id)"
                @update:model-value="updateSkuForm(sku.id, $event)"
              />
            </div>
            <div class="flex flex-wrap gap-3">
              <Button type="button" :disabled="saving" @click="saveEditing">
                {{ saving ? t('personalCenter.reseller.productSettings.saving') : t('personalCenter.reseller.productSettings.save') }}
              </Button>
              <Button type="button" variant="outline" :disabled="saving" @click="resetProductRule">
                {{ t('personalCenter.reseller.productSettings.resetProductRule') }}
              </Button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div v-if="!loading && pagination.total_page > 1" class="mt-5 flex flex-wrap items-center justify-center gap-3">
      <Button type="button" variant="outline" size="sm" :disabled="pagination.page <= 1" @click="goPage(pagination.page - 1)">
        {{ t('orders.prevPage') }}
      </Button>
      <span class="rounded-full border bg-card px-4 py-1.5 text-sm text-muted-foreground">
        {{ t('orders.pageInfo', { page: pagination.page, total: pagination.total_page }) }}
      </span>
      <Button type="button" variant="outline" size="sm" :disabled="pagination.page >= pagination.total_page" @click="goPage(pagination.page + 1)">
        {{ t('orders.nextPage') }}
      </Button>
    </div>

  </Card>
</template>

<script setup lang="ts">
import { nextTick, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { resellerAPI } from '../../api/reseller'
import type {
  ResellerProductSettingData,
  ResellerProductSettingDetailData,
  ResellerProductSettingPayloadItem,
  ResellerProductSettingPreviewData,
  ResellerProductSettingProductData,
  ResellerProductSettingSKUData,
} from '../../api/types'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { type PageAlert } from '../../utils/alerts'
import { getLocalizedText } from '../../utils/resellerSiteConfig'
import {
  buildResellerProductSettingPayload,
  countActiveSkus,
  countListedSkus,
  isResellerProductSettingDetail,
  normalizeResellerProductSettingsPagination,
  normalizeResellerProductSettingForm,
  summarizeEffectivePrice,
  summarizeProductBasePrice,
  summarizeProductEffectivePrice,
} from '../../utils/resellerProductSettings'
import { formatSkuSpecValues } from '../../utils/sku'
import ResellerProductRuleEditor from './ResellerProductRuleEditor.vue'

withDefaults(defineProps<{ embedded?: boolean }>(), { embedded: false })

const { t, locale } = useI18n()

const loading = ref(false)
const detailLoading = ref(false)
const saving = ref(false)
const rows = ref<ResellerProductSettingDetailData[]>([])
const editing = ref<ResellerProductSettingDetailData | null>(null)
const panelAlert = ref<PageAlert | null>(null)
const productForm = ref<ResellerProductSettingPayloadItem>(normalizeResellerProductSettingForm({ sku_id: 0 }))
const skuForms = reactive<Record<number, ResellerProductSettingPayloadItem>>({})
const pagination = reactive({ page: 1, page_size: 20, total: 0, total_page: 1 })

type PreviewEntry = { effective: string; valid: boolean; errorCode: string }
// key 0 = 商品级规则，其余为 sku_id。生效价随编辑实时由后端预览接口刷新，与保存/下单口径一致。
const previewByKey = reactive<Record<number, PreviewEntry>>({})
let previewTimer: ReturnType<typeof setTimeout> | null = null
let previewSeq = 0

const filters = reactive({
  keyword: '',
})

const unwrapData = <T,>(response: any): T | null => response?.data?.data || null

const clearSkuForms = () => {
  Object.keys(skuForms).forEach((key) => {
    delete skuForms[Number(key)]
  })
}

const formFromSetting = (
  setting: ResellerProductSettingData | undefined,
  skuID: number,
): ResellerProductSettingPayloadItem =>
  normalizeResellerProductSettingForm({
    sku_id: skuID,
    is_listed: setting?.is_listed,
    pricing_mode: setting?.pricing_mode,
    markup_percent: setting?.markup_percent,
    fixed_markup_amount: setting?.fixed_markup_amount,
    fixed_price_amount: setting?.fixed_price_amount,
    sort_order: setting?.sort_order,
  })

const clearPreview = () => {
  Object.keys(previewByKey).forEach((key) => delete previewByKey[Number(key)])
}

// 用已保存的生效价先行填充预览，避免打开编辑器瞬间出现空值，随后由实时预览刷新。
const seedPreviewFromDetail = (detail: ResellerProductSettingDetailData) => {
  clearPreview()
  if (detail.product_setting?.effective_price_amount) {
    previewByKey[0] = { effective: detail.product_setting.effective_price_amount, valid: true, errorCode: '' }
  }
  detail.skus.forEach((sku) => {
    const effective = sku.effective_price_amount || sku.setting?.effective_price_amount
    if (effective) previewByKey[sku.id] = { effective, valid: true, errorCode: '' }
  })
}

const applyDetailToEditor = (detail: ResellerProductSettingDetailData) => {
  productForm.value = formFromSetting(detail.product_setting, 0)
  clearSkuForms()
  detail.skus.forEach((sku) => {
    skuForms[sku.id] = formFromSetting(sku.setting, sku.id)
  })
  editing.value = detail
  seedPreviewFromDetail(detail)
  schedulePreview()
}

const runPreview = async () => {
  const current = editing.value
  if (!current) return
  const productID = current.product.id
  const seq = ++previewSeq
  try {
    const payload = buildResellerProductSettingPayload([
      productForm.value,
      ...current.skus.map((sku) => skuFormFor(sku.id)),
    ])
    const response = await resellerAPI.previewProductSettings(productID, payload)
    const data = unwrapData<ResellerProductSettingPreviewData>(response)
    // 丢弃过期结果（编辑器已切换商品或有更新的请求在途）。
    if (seq !== previewSeq || editing.value?.product.id !== productID) return
    const next: Record<number, PreviewEntry> = {}
    ;(data?.items || []).forEach((item) => {
      next[item.sku_id] = {
        effective: item.effective_price_amount,
        valid: item.valid,
        errorCode: item.error_code || '',
      }
    })
    clearPreview()
    Object.assign(previewByKey, next)
  } catch {
    // 预览失败静默：保留上次值；保存时后端仍会做权威校验。
  }
}

const schedulePreview = () => {
  if (previewTimer) clearTimeout(previewTimer)
  previewTimer = setTimeout(runPreview, 350)
}

const previewEffectiveFor = (key: number, fallback: string) => previewByKey[key]?.effective || fallback
const previewInvalidFor = (key: number) => previewByKey[key]?.valid === false
const previewErrorFor = (key: number) => previewByKey[key]?.errorCode || ''

watch(
  () => [productForm.value, skuForms],
  () => {
    if (editing.value) schedulePreview()
  },
  { deep: true },
)

const scrollProductRowIntoView = async (productID: number) => {
  await nextTick()
  document.getElementById(`reseller-product-row-${productID}`)?.scrollIntoView({
    behavior: 'smooth',
    block: 'nearest',
  })
}

const skuFormFor = (skuID: number): ResellerProductSettingPayloadItem => {
  if (!skuForms[skuID]) {
    skuForms[skuID] = formFromSetting(undefined, skuID)
  }
  return skuForms[skuID]
}

const updateSkuForm = (skuID: number, value: ResellerProductSettingPayloadItem) => {
  skuForms[skuID] = value
}

const showAlert = (level: PageAlert['level'], message: string) => {
  panelAlert.value = { level, message }
}

const getProductTitle = (title: ResellerProductSettingProductData['title']) =>
  getLocalizedText(title, String(locale.value))

const loadRows = async (page = pagination.page) => {
  loading.value = true
  panelAlert.value = null
  try {
    const params: Record<string, string | number> = { page, page_size: pagination.page_size }
    if (filters.keyword.trim()) params.keyword = filters.keyword.trim()
    const response = await resellerAPI.productSettings(params)
    rows.value = unwrapData<ResellerProductSettingDetailData[]>(response) || []
    Object.assign(pagination, normalizeResellerProductSettingsPagination(response?.data?.pagination, pagination))
  } catch (err: any) {
    rows.value = []
    showAlert('error', err?.message || t('personalCenter.reseller.productSettings.loadFailed'))
  } finally {
    loading.value = false
  }
}

const searchRows = () => loadRows(1)
const goPage = (page: number) => loadRows(page)

const closeEditor = () => {
  if (previewTimer) {
    clearTimeout(previewTimer)
    previewTimer = null
  }
  previewSeq++
  clearPreview()
  editing.value = null
}

const openEditor = async (productID: number) => {
  detailLoading.value = true
  panelAlert.value = null
  const existing = rows.value.find((row) => row.product.id === productID)
  if (existing) {
    applyDetailToEditor(existing)
    await scrollProductRowIntoView(productID)
  }
  try {
    const response = await resellerAPI.productSettingDetail(productID)
    const detail = unwrapData<ResellerProductSettingDetailData>(response)
    if (detail) {
      applyDetailToEditor(detail)
      await scrollProductRowIntoView(productID)
    }
  } catch (err: any) {
    showAlert('error', err?.message || t('personalCenter.reseller.productSettings.loadFailed'))
  } finally {
    detailLoading.value = false
  }
}

const buildSkuLabel = (sku: ResellerProductSettingSKUData) => {
  const spec = formatSkuSpecValues(sku.spec_values, String(locale.value))
  const fallback = sku.sku_code || `#${sku.id}`
  return `${t('personalCenter.reseller.productSettings.skuLevelRule')} · ${spec || fallback}`
}

const summarizeSkuEffectivePrice = (sku: ResellerProductSettingSKUData) => {
  return String(sku.effective_price_amount || sku.setting?.effective_price_amount || '').trim() || '-'
}

const saveEditing = async () => {
  if (!editing.value) return
  saving.value = true
  panelAlert.value = null
  try {
    const payload = buildResellerProductSettingPayload([
      productForm.value,
      ...editing.value.skus.map((sku) => skuFormFor(sku.id)),
    ])
    const response = await resellerAPI.updateProductSettings(editing.value.product.id, payload)
    const detail = unwrapData<ResellerProductSettingDetailData>(response)
    if (detail) {
      applyDetailToEditor(detail)
    }
    await loadRows(pagination.page)
    showAlert('success', t('personalCenter.reseller.productSettings.saveSuccess'))
  } catch (err: any) {
    showAlert('error', err?.message || t('personalCenter.reseller.productSettings.saveFailed'))
  } finally {
    saving.value = false
  }
}

const resetProductRule = async () => {
  if (!editing.value) return
  saving.value = true
  panelAlert.value = null
  try {
    const productID = editing.value.product.id
    const response = await resellerAPI.resetProductSetting(productID, 0)
    const detail = unwrapData<ResellerProductSettingDetailData>(response)
    if (isResellerProductSettingDetail(detail)) {
      applyDetailToEditor(detail)
    } else {
      await openEditor(productID)
    }
    await loadRows(pagination.page)
    showAlert('success', t('personalCenter.reseller.productSettings.resetSuccess'))
  } catch (err: any) {
    showAlert('error', err?.message || t('personalCenter.reseller.productSettings.resetFailed'))
  } finally {
    saving.value = false
  }
}

onMounted(() => loadRows(1))
</script>
