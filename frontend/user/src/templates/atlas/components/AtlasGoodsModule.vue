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
      <!-- 标题区可选：筛选态下标题与侧栏选中项重复，直接不渲染，让商品内容顶上来 -->
      <div v-if="title || desc" class="atlas-module__descrow">
        <div>
          <h2 v-if="title" class="atlas-module__title">{{ title }}</h2>
          <p v-if="desc" class="atlas-module__desc">{{ desc }}</p>
        </div>
        <slot name="more" />
      </div>

      <!-- 骨架卡必须与真实卡等高，否则 loading→加载完成时主列高度跳变（实测 530→517，
           侧栏底与分割线跟着抖 13px）。所以这里用固定高度块，而不是按比例撑开的占位。 -->
      <div v-if="loading" class="atlas-module__list">
        <div v-for="i in skeletonCount" :key="i" class="atlas-skeleton atlas-module__ph"></div>
      </div>

      <div v-else-if="products.length" class="atlas-module__list">
        <AtlasProductCard
          v-for="product in products"
          :key="product.id"
          :product="product"
          :compact="compact"
          @click="$emit('select', $event)"
        />
      </div>

      <EmptyState v-else variant="soft" size="md" icon="package" :title="emptyTitle" />

      <nav v-if="loading || products.length" class="atlas-pager">
        <template v-if="totalPages > 1">
          <button class="atlas-pager__btn" type="button" :disabled="page <= 1" @click="$emit('change-page', page - 1)">
            {{ t('products.previous') }}
          </button>
          <span class="atlas-pager__info">{{ page }} / {{ totalPages }}</span>
          <button class="atlas-pager__btn" type="button" :disabled="page >= totalPages" @click="$emit('change-page', page + 1)">
            {{ t('products.next') }}
          </button>
        </template>
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
    products: any[]
    loading: boolean
    page: number
    totalPages: number
    /** 标题/副标题可选：筛选态不传，避免与侧栏选中项重复显示 */
    title?: string
    desc?: string
    compact?: boolean
    emptyTitle?: string
  }>(),
  { compact: false, title: '', desc: '' },
)

defineEmits<{ select: [slug: string]; 'change-page': [page: number] }>()

const { t } = useI18n()
const emptyTitle = computed(() => props.emptyTitle || t('products.empty'))
const skeletonCount = computed(() => 12)
</script>

<style scoped>
/* 骨架块：高度必须等于真实卡（整宽 180 / 紧凑 159），数量等于一页 12 张，
   这样 loading 期的网格高与加载完成后完全一致（3 行 = 517px），不产生跳动。 */
.atlas-module__ph {
  height: 180px;
  border-radius: 8px;
}

.atlas-module--compact .atlas-module__ph {
  height: 159px;
}

.atlas-pager {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  margin-top: 28px;
  /*
    恒定占位：只有 1 页时也保留这条 34px 的高度。
    否则分页整条消失 → 主列变矮 62px（28 边距 + 34 高）→ 侧栏底（stretch 跟随主列）
    跟着上移，就违反「分类底部位置不随商品数量变化」。
  */
  min-height: 34px;
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
  紧凑态（收进主列时）：容器填满主列宽度，网格按参考站分类页的 .list.long ——
  repeat(4,1fr) + gap 20px 13px，卡片同步切到 .item.long 的紧凑尺寸（260x159）。
  主列 1080 下算下来正好 (1080 - 3*13) / 4 = 260px，与参考站一致。
*/
.atlas-module--compact {
  /* 必须为 0：margin 会被算进主列高度，而侧栏是 stretch 跟随主列 ——
     留 40px 会把侧栏底再推低 40px，破坏「与分页底部线齐平」。
     分页下方的留白改由 .atlas-home-main 的 padding-bottom 提供（在行外，不参与 stretch）。 */
  margin-bottom: 0;
}

.atlas-module--compact .atlas-module__content {
  max-width: none;
  /*
    紧凑态内边距必须为 0，否则两端各差一截（实测）：
      padding-top:24 + 列表 margin-top:20 = 商品顶比分类顶低 44px
      padding-bottom:24              = 分类底比分页底低 24px
    归零后：商品顶与分类顶齐平，分类底与分页底齐平。
    左右同理：留 20px 会把卡片挤成 250px，与参考站的 260px 差 10px。
  */
  padding: 0;
}

.atlas-module--compact .atlas-module__list {
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 20px 13px;
  /* 归零：标题区已被 v-if 去掉，这里再留 20px 会让商品顶比分类顶低 44px */
  margin-top: 0;
  /*
    恒定占 3 行高度：3 × 159 + 2 × 20 = 517px。
    商品不足 12 条时也要占满 —— 否则网格变矮 → 分页上移 → 侧栏底（stretch 跟随主列）
    跟着上移。用户要的是「无论所选分类商品多少，分类底部位置都不变」，
    所以这里必须预留固定高度，而不是按内容自适应。
  */
  min-height: 517px;
  align-content: start;
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
