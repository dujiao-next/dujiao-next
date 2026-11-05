<template>
  <div class="st-root cp">
    <div class="st-container">
      <nav class="cp-crumbs">
        <RouterLink to="/">{{ t('nav.home') }}</RouterLink>
        <span class="cp-sep">›</span>
        <RouterLink to="/products">{{ t('nav.products') }}</RouterLink>
        <span class="cp-sep">›</span>
        <span>{{ heading }}</span>
      </nav>

      <h1 class="cp-title">{{ heading }}</h1>
      <p v-if="data && data.items?.length" class="cp-lead">{{ t('comparePage.lead', { count: data.items.length }) }}</p>

      <div v-if="loading" class="cp-skeleton" />
      <p v-else-if="!data || !data.items?.length" class="cp-empty">
        {{ t('comparePage.empty') }}
        <RouterLink to="/products" class="cp-link">{{ t('comparePage.pick') }}</RouterLink>
      </p>

      <CompareSection v-else :data="data" removable @remove="remove" :price-suffix="t('productDetail.priceFrom')" />

      <div class="cp-foot">
        <RouterLink to="/products" class="cp-link">{{ t('comparePage.keepShopping') }}</RouterLink>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { api } from '../../api/client'
import { useLocalized } from '../../composables/useProduct'
import CompareSection from './components/CompareSection.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const { getLocalizedText } = useLocalized()

const data = ref<any>(null)
const loading = ref(false)

const heading = computed(() => {
  const name = data.value?.group_name
  return (name && getLocalizedText(name)) || t('comparePage.title')
})

const ids = computed<string[]>(() => {
  const raw = route.query.ids
  const value = Array.isArray(raw) ? raw.join(',') : String(raw || '')
  return value.split(',').map((item) => item.trim()).filter(Boolean)
})

const load = async () => {
  if (ids.value.length < 2) {
    data.value = null
    return
  }
  loading.value = true
  try {
    const res = await api.get('/public/compare', { params: { ids: ids.value.join(',') } })
    data.value = res.data?.data || null
  } catch {
    data.value = null
  } finally {
    loading.value = false
  }
}

const remove = (id: number) => {
  const next = ids.value.filter((item) => item !== String(id))
  router.replace({ path: '/compare', query: { ids: next.join(',') } })
}

watch(ids, load)
onMounted(load)
</script>

<style scoped>
.cp {
  padding-block: 24px 80px;
}
.cp-crumbs {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--st-text-2);
}
.cp-crumbs a {
  color: inherit;
  text-decoration: none;
}
.cp-crumbs a:hover {
  color: var(--st-link, #06c);
}
.cp-sep {
  opacity: 0.5;
}
.cp-title {
  margin: 18px 0 6px;
  font-family: var(--st-font-display);
  font-size: 40px;
  font-weight: 600;
  letter-spacing: -0.01em;
}
.cp-lead {
  margin: 0 0 24px;
  color: var(--st-text-2);
  font-size: 15px;
}
.cp-scroll {
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
}
.cp-table {
  width: 100%;
  min-width: 720px;
  border-collapse: collapse;
}
.cp-table th,
.cp-table td {
  padding: 16px 18px;
  text-align: center;
  font-weight: 400;
  font-size: 15px;
}
.cp-col {
  vertical-align: top;
  border-bottom: 1px solid color-mix(in srgb, var(--st-text) 10%, transparent);
  padding-bottom: 18px;
}
.cp-col-link {
  display: block;
  color: inherit;
  text-decoration: none;
}
.cp-col-link img {
  display: block;
  width: 110px;
  height: 110px;
  margin: 0 auto 10px;
  object-fit: contain;
}
.cp-name {
  display: block;
  font-size: 17px;
  font-weight: 600;
}
.cp-sub {
  display: block;
  margin-top: 2px;
  font-size: 12px;
  color: var(--st-text-2);
}
.cp-price {
  display: block;
  margin-top: 6px;
  font-size: 13px;
  color: var(--st-text-2);
}
.cp-actions {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  margin-top: 10px;
}
.cp-buy {
  display: inline-block;
  padding: 5px 16px;
  border-radius: 980px;
  background: var(--st-blue, #0071e3);
  color: #fff;
  font-size: 12px;
  text-decoration: none;
}
.cp-remove {
  border: 0;
  background: none;
  color: var(--st-text-3);
  font-size: 12px;
  cursor: pointer;
}
.cp-remove:hover {
  color: #d70015;
}
.cp-label {
  text-align: left;
  font-weight: 500;
  border-top: 1px solid color-mix(in srgb, var(--st-text) 8%, transparent);
  white-space: nowrap;
}
.cp-icon {
  display: inline-flex;
  width: 16px;
  height: 16px;
  margin-right: 6px;
  vertical-align: -3px;
  color: var(--st-text-2);
}
.cp-icon :deep(svg) {
  width: 16px;
  height: 16px;
}
.cp-table td {
  border-top: 1px solid color-mix(in srgb, var(--st-text) 8%, transparent);
  color: var(--st-text-2);
}
.cp-empty {
  padding: 48px 0;
  color: var(--st-text-2);
}
.cp-link {
  color: var(--st-link, #06c);
  text-decoration: none;
}
.cp-foot {
  margin-top: 28px;
  text-align: center;
}
.cp-skeleton {
  height: 260px;
  border-radius: 16px;
  background: var(--st-bg-alt);
}
</style>
