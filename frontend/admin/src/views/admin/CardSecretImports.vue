<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useDebounceFn } from '@vueuse/core'
import { adminAPI } from '@/api/admin'
import type { AdminProduct, AdminProductSKU } from '@/api/types'
import { PackagePlus, Upload } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { getLocalizedText } from '@/utils/format'
import { formatCurrentLocaleSkuSpec } from '@/utils/sku'
import CardSecretBatchCreateModal from './components/CardSecretBatchCreateModal.vue'
import { adminUrl } from '@/utils/adminBase'

const { t, locale } = useI18n()

const productKeyword = ref('')
const productOptions = ref<AdminProduct[]>([])
const productOptionsLoading = ref(false)
const selectedProductValue = ref('__all__')
const productInfo = ref<AdminProduct | null>(null)
const skuFilterValue = ref('__all__')

const normalizeFilterValue = (value: string) => (value === '__all__' ? '' : value)

const parseProductId = () => {
  const normalized = normalizeFilterValue(selectedProductValue.value)
  if (!normalized) return null
  const parsed = Number(normalized)
  if (!Number.isFinite(parsed) || parsed <= 0) return null
  return Math.floor(parsed)
}

const parseSkuId = () => {
  const normalized = normalizeFilterValue(skuFilterValue.value)
  if (!normalized) return 0
  const parsed = Number(normalized)
  if (!Number.isFinite(parsed) || parsed <= 0) return 0
  return Math.floor(parsed)
}

const buildSkuLabel = (sku: AdminProductSKU | null | undefined) => {
  const skuCode = String(sku?.sku_code || '').trim()
  const specText = formatCurrentLocaleSkuSpec(sku?.spec_values, locale.value)
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

const productHint = computed(() => {
  if (!currentProductId.value) return t('admin.cardSecretImports.selectionTip')
  return t('admin.cardSecrets.productHintCurrent', { id: currentProductId.value })
})

const productInfoName = computed(() => {
  if (productInfo.value) return getLocalizedText(productInfo.value.title)
  const option = productOptions.value.find((item: AdminProduct) => Number(item?.id || 0) === currentProductId.value)
  if (!option) return ''
  return getLocalizedText(option.title || {})
})
const currentProductLabel = computed(() => {
  if (productInfo.value) return buildProductLabel(productInfo.value)
  if (currentProductId.value) return `#${currentProductId.value}`
  return '-'
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
const requireExplicitSkuSelection = computed(() => !!currentProductId.value && availableSkus.value.length > 1)
const currentSkuLabel = computed(() => {
  if (!currentSkuId.value) return t('admin.cardSecrets.skuAll')
  const matched = availableSkus.value.find((sku) => sku.id === currentSkuId.value)
  if (!matched) return `#${currentSkuId.value}`
  return matched.label
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

const handleSearchProducts = async () => {
  await loadProductOptions()
}

const debouncedSearchProducts = useDebounceFn(handleSearchProducts, 300)

const handleProductSelectionChange = async () => {
  skuFilterValue.value = '__all__'
  await loadProductInfo()
}

const handleImportSuccess = async () => {
  await loadProductInfo()
}

const productLink = (productId: number) => adminUrl(`/products?product_id=${productId}`)

onMounted(async () => {
  await loadProductOptions()
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
      <div>
        <h1 class="text-2xl font-semibold">{{ t('admin.cardSecretImports.title') }}</h1>
        <p class="mt-1 text-sm text-muted-foreground">{{ t('admin.cardSecretImports.subtitle') }}</p>
      </div>
      <Button class="w-full lg:w-auto" variant="outline" as-child>
        <RouterLink to="/card-secrets">
          <Upload class="mr-2 h-4 w-4" />
          {{ t('admin.cardSecretImports.inventoryAction') }}
        </RouterLink>
      </Button>
    </div>

    <div class="rounded-xl border border-border bg-card p-4 shadow-sm">
      <div class="mb-4">
        <h2 class="text-lg font-semibold text-foreground">{{ t('admin.cardSecretImports.selectionTitle') }}</h2>
        <p class="mt-1 text-sm text-muted-foreground">{{ t('admin.cardSecretImports.selectionDescription') }}</p>
      </div>

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

        <div class="md:col-span-4">
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

        <div class="md:col-span-4">
          <Select v-model="skuFilterValue" :disabled="skuFilterDisabled">
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
      </div>

      <div class="mt-4 space-y-1 text-xs text-muted-foreground">
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
          {{ t('admin.cardSecrets.skuLabel') }}：{{ currentSkuLabel }}
        </p>
      </div>
    </div>

    <div v-if="!currentProductId" class="rounded-xl border-2 border-dashed border-primary/30 bg-primary/5 p-8">
      <div class="mx-auto max-w-xl space-y-4 text-center">
        <div class="mx-auto flex h-16 w-16 items-center justify-center rounded-full bg-primary/10">
          <PackagePlus class="h-8 w-8 text-primary" />
        </div>
        <h2 class="text-xl font-semibold text-foreground">{{ t('admin.cardSecretImports.emptyTitle') }}</h2>
        <p class="text-sm text-muted-foreground">{{ t('admin.cardSecretImports.emptyDescription') }}</p>
      </div>
    </div>

    <div v-else class="space-y-4">
      <div class="rounded-xl border border-primary/20 bg-primary/5 p-4">
        <div class="flex flex-col gap-2 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <p class="text-sm font-medium text-foreground">{{ t('admin.cardSecretImports.targetTitle') }}</p>
            <p class="mt-1 text-sm text-muted-foreground">{{ currentProductLabel }}</p>
            <p class="mt-1 text-xs text-muted-foreground">
              {{ t('admin.cardSecrets.skuLabel') }}：{{ currentSkuLabel }}
            </p>
          </div>
          <Button size="sm" variant="outline" as-child>
            <RouterLink to="/card-secrets">
              {{ t('admin.cardSecretImports.inventoryAction') }}
            </RouterLink>
          </Button>
        </div>
        <p class="mt-3 text-xs text-muted-foreground">{{ t('admin.cardSecretImports.targetHint') }}</p>
        <p v-if="requireExplicitSkuSelection && !currentSkuId" class="mt-2 text-xs text-destructive">
          {{ t('admin.cardSecrets.errors.skuRequired') }}
        </p>
      </div>

      <div>
        <div class="mb-4">
          <h2 class="text-lg font-semibold text-foreground">{{ t('admin.cardSecretImports.readyTitle') }}</h2>
          <p class="mt-1 text-sm text-muted-foreground">{{ t('admin.cardSecretImports.readyDescription') }}</p>
        </div>
        <CardSecretBatchCreateModal
          :model-value="true"
          :product-id="currentProductId || 0"
          :sku-id="currentSkuId"
          :require-sku-selection="requireExplicitSkuSelection"
          @success="handleImportSuccess"
        />
      </div>
    </div>
  </div>
</template>
