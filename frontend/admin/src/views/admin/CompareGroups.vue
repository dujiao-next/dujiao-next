<template>
  <div class="cg">
    <header class="cg-head">
      <div>
        <h1 class="cg-title">{{ t('admin.compareGroups.title') }}</h1>
        <p class="cg-desc">{{ t('admin.compareGroups.desc') }}</p>
      </div>
    </header>
    <div class="cg-body">
      <CompareGroupsPanel lang="zh-CN" :categories="categories" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AdminCategory } from '@/api/types'
import CompareGroupsPanel from '@/components/admin/blocks/CompareGroupsPanel.vue'

const { t } = useI18n()
const categories = ref<AdminCategory[]>([])

onMounted(async () => {
  try {
    const res = await adminAPI.getCategories({ page: 1, page_size: 200 })
    categories.value = (res.data?.data || []) as AdminCategory[]
  } catch {
    categories.value = []
  }
})
</script>

<style scoped>
.cg {
  padding: 4px 2px 48px;
}
.cg-head {
  margin-bottom: 16px;
}
.cg-title {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  letter-spacing: -0.01em;
}
.cg-desc {
  margin: 4px 0 0;
  font-size: 13px;
  color: var(--muted-foreground, #6b7280);
}
.cg-body {
  padding: 16px;
  border: 1px solid var(--border, #e5e7eb);
  border-radius: 12px;
  background: var(--background, #fff);
}
</style>
