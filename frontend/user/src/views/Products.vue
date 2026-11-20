<template>
  <div class="products-page min-h-screen bg-background pb-16 pt-24 text-foreground">
    <h1 class="sr-only">{{ t('nav.products') }}</h1>
    <ProductCatalog home-route-name="products" @category-name="seoCategoryName = $event" />
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { usePageSeo } from '../composables/usePageSeo'
import ProductCatalog from '../components/ProductCatalog.vue'

const { t } = useI18n()
const route = useRoute()

const seoCategoryName = ref('')
usePageSeo({
  canonicalPath: () => route.path,
  title: () => {
    if (route.name === 'category-products') {
      return seoCategoryName.value || t('nav.products')
    }
    return t('nav.products')
  },
})
</script>
