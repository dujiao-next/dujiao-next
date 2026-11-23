<template>
  <div class="min-h-screen pb-16 pt-[88px] text-foreground">
    <AtelierCatalog
      home-route-name="products"
      compact-hero
      @category-name="seoCategoryName = $event"
    />
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { usePageSeo } from '../../composables/usePageSeo'
import AtelierCatalog from './components/AtelierCatalog.vue'

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
