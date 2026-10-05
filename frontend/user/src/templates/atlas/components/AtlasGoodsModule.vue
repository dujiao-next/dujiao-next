<template>
  <!--
    商品分区（参考站 .module-content）。
    抽成组件的原因：它要在两个位置出现，且同一时刻只渲染一处 ——
      · 未筛选：整宽 1340（参考站版式），4 列
      · 已筛选：收进主列 1080，3 列（compact）
    如果把列表留在三栏行之外，筛选时主列塌缩、行高仍由侧栏撑开，
    列表会被推到页面下方（实测空隙 438px），所以已筛选态必须放进主列内部。
  -->
  <div class="atlas-module" :class="compact && 'atlas-module--compact'">
    <div class="atlas-module__content">
      <div class="atlas-module__descrow">
        <div>
          <h2 class="atlas-module__title">{{ title }}</h2>
          <p class="atlas-module__desc">{{ desc }}</p>
        </div>
        <slot name="more" />
      </div>

      <div v-if="loading" class="atlas-module__list">
        <div v-for="i in skeletonCount" :key="i" class="atlas-card overflow-hidden">
          <div class="atlas-skeleton aspect-[4/3] rounded-none"></div>
          <div class="space-y-2 p-3">
            <div class="atlas-skeleton h-3 w-2/3"></div>
            <div class="atlas-skeleton h-4 w-1/3"></div>
          </div>
        </div>
      </div>

      <div v-else-if="products.length" class="atlas-module__list">
        <AtlasProductCard
          v-for="product in products"
          :key="product.id"
          :product="product"
          @click="$emit('select', $event)"
        />
      </div>

      <EmptyState v-else variant="soft" size="md" icon="package" :title="emptyTitle" />

      <nav v-if="totalPages > 1" class="atlas-pager">
        <button class="atlas-pager__btn" type="button" :disabled="page <= 1" @click="$emit('change-page', page - 1)">
          {{ t('products.previous') }}
        </button>
        <span class="atlas-pager__info">{{ page }} / {{ totalPages }}</span>
        <button class="atlas-pager__btn" type="button" :disabled="page >= totalPages" @click="$emit('change-page', page + 1)">
          {{ t('products.next') }}
        </button>
      </nav>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import AtlasProductCard from './AtlasProductCard.vue'
import EmptyState from '../../../components/EmptyState.vue'

const props = withDefaults(
  defineProps<{
    title: string
    desc: string
    products: any[]
    loading: boolean
    page: number
    totalPages: number
    compact?: boolean
    emptyTitle?: string
  }>(),
  { compact: false },
)

defineEmits<{ select: [slug: string]; 'change-page': [page: number] }>()

const { t } = useI18n()
const emptyTitle = computed(() => props.emptyTitle || t('products.empty'))
const skeletonCount = computed(() => (props.compact ? 6 : 8))
</script>

<style scoped>
.atlas-pager {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  margin-top: 28px;
}

.atlas-pager__btn {
  min-height: 34px;
  padding: 0 14px;
  border: 1px solid var(--border-strong);
  border-radius: var(--r-sm);
  background: var(--surface);
  font-size: 13px;
  transition: border-color var(--dur) var(--ease);
}

.atlas-pager__btn:hover:not(:disabled) { border-color: var(--ink); }
.atlas-pager__btn:disabled { opacity: 0.4; cursor: not-allowed; }

.atlas-pager__info { color: var(--ink-3); font-size: 13px; }

/*
  紧凑态（收进主列时）：容器不再撑到 1340，改为填满主列宽度，网格降为 3 列。
  为什么是 3 列而不是 4：卡片内部最窄需要「70px 图标 + 15 gap + 160 标题 +
  52 内边距 ≈ 297px」，主列 1080 下 4 列只有 ~250px，会把标题挤变形；
  3 列约 342px，卡片内部结构完整。
*/
.atlas-module--compact {
  margin-bottom: 0;
}

.atlas-module--compact .atlas-module__content {
  max-width: none;
  padding: 24px 20px;
}

.atlas-module--compact .atlas-module__list {
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 20px;
}

.atlas-module--compact .atlas-module__title {
  font-size: 20px;
  height: 28px;
  line-height: 28px;
}

@media (max-width: 1023px) {
  .atlas-module--compact .atlas-module__list {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 16px;
  }
}

@media (max-width: 640px) {
  .atlas-module--compact .atlas-module__list {
    grid-template-columns: 1fr;
  }
}
</style>
