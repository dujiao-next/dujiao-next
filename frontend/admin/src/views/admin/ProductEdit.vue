<template>
  <div class="pep">
    <header class="pep-bar">
      <div class="pep-bar-left">
        <button type="button" class="pep-back" :title="t('admin.common.back')" @click="goBack">
          <ArrowLeft class="h-4 w-4" />
        </button>
        <nav class="pep-crumbs">
          <RouterLink to="/products">{{ t('admin.nav.products') }}</RouterLink>
          <span class="pep-sep">›</span>
          <span>{{ isEditing ? t('admin.products.modal.editTitle') : t('admin.products.modal.createTitle') }}</span>
        </nav>
      </div>
      <div class="pep-bar-right">
        <a
          v-if="isEditing"
          class="pep-preview"
          :href="previewUrl"
          target="_blank"
          rel="noopener"
        >{{ t('admin.products.previewPage') }}</a>
      </div>
    </header>

    <main class="pep-main">
      <ProductEditor
        :product-id="productId"
        :categories="categories"
        :site-currency="siteCurrency"
        :supported-locales="['zh-CN', 'zh-TW', 'en-US']"
        @cancel="goBack"
        @saved="goBack"
      />
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ArrowLeft } from 'lucide-vue-next'
import { adminAPI } from '@/api/admin'
import type { AdminCategory } from '@/api/types'
import ProductEditor from './components/ProductEditor.vue'
import { notifyError } from '@/utils/notify'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

const categories = ref<AdminCategory[]>([])
const siteCurrency = ref('CNY')
const slug = ref('')

const productId = computed<number | null>(() => {
  const raw = route.params.id
  const value = Number(Array.isArray(raw) ? raw[0] : raw)
  return Number.isFinite(value) && value > 0 ? value : null
})
const isEditing = computed(() => productId.value !== null)
const previewUrl = computed(() => (slug.value ? `/products/${slug.value}` : '#'))

const loadMeta = async () => {
  try {
    const res = await adminAPI.getCategories({ page: 1, page_size: 200 })
    categories.value = (res.data?.data || []) as AdminCategory[]
  } catch (err) {
    notifyError((err as Error).message)
  }
  try {
    const settings = await adminAPI.getSettings()
    const currency = (settings.data?.data as any)?.site_currency
    if (currency) siteCurrency.value = String(currency)
  } catch {
    /* 拿不到就用默认币种 */
  }
  if (productId.value) {
    try {
      const res = await adminAPI.getProduct(productId.value)
      slug.value = (res.data?.data as any)?.slug || ''
    } catch {
      /* 编辑器自己会再取一次 */
    }
  }
}

const goBack = () => {
  router.push('/products')
}

onMounted(loadMeta)
</script>

<style scoped>
.pep {
  display: flex;
  min-height: calc(100vh - 64px);
  flex-direction: column;
  background: var(--background, #fff);
}
.pep-bar {
  position: sticky;
  top: 0;
  z-index: 20;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  height: 52px;
  padding: 0 20px;
  border-bottom: 1px solid var(--border, #e5e7eb);
  background: color-mix(in srgb, var(--background, #fff) 88%, transparent);
  backdrop-filter: blur(12px);
}
.pep-bar-left {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}
.pep-back {
  display: grid;
  place-items: center;
  width: 30px;
  height: 30px;
  border: 1px solid var(--border, #e5e7eb);
  border-radius: 8px;
  background: none;
  cursor: pointer;
}
.pep-crumbs {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--muted-foreground, #6b7280);
}
.pep-crumbs a {
  color: inherit;
  text-decoration: none;
}
.pep-crumbs a:hover {
  color: var(--foreground, #111827);
}
.pep-sep {
  opacity: 0.5;
}
.pep-preview {
  font-size: 13px;
  color: var(--primary, #2563eb);
  text-decoration: none;
}
.pep-main {
  width: 100%;
  max-width: 1180px;
  margin: 0 auto;
  padding: 20px 20px 64px;
}
</style>
