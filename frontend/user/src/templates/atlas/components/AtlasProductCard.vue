<template>
  <!-- 参考站的商品卡：白底、小圆角、极轻阴影，图片区用纯色底而非骨架屏。
       信息层级固定为「标签 → 标题 → 价格」，价格右对齐并带红色「起」字。 -->
  <article class="atlas-pcard" @click="$emit('click', product.slug)">
    <div class="atlas-thumb atlas-pcard__media">
      <img
        v-if="cover"
        :src="cover"
        :alt="getLocalizedText(product.title)"
        loading="lazy"
        decoding="async"
      />
      <ImageOff v-else class="h-8 w-8 text-[color:var(--ink-4)]" />
    </div>

    <div class="atlas-pcard__body">
      <div v-if="tags.length" class="flex flex-wrap gap-1">
        <span v-for="(tag, i) in tags" :key="i" class="atlas-tag">{{ tag }}</span>
      </div>

      <h3 class="atlas-pcard__title">{{ getLocalizedText(product.title) }}</h3>

      <div v-if="categoryName" class="atlas-pcard__cat">{{ categoryName }}</div>

      <div class="atlas-pcard__foot">
        <span class="atlas-price atlas-pcard__price">{{ formatPrice(product.price_amount, siteCurrency) }}</span>
        <span class="atlas-price-from">{{ t('atlas.product.from') }}</span>
      </div>
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ImageOff } from 'lucide-vue-next'
import { getFirstImageUrl, getImageUrl } from '../../../utils/image'
import { useLocalized } from '../../../composables/useProduct'

const props = withDefaults(
  defineProps<{
    product: any
    maxTags?: number
  }>(),
  { maxTags: 2 },
)

defineEmits<{ click: [slug: string] }>()

const { t } = useI18n()
const { getLocalizedText, siteCurrency, formatPrice } = useLocalized()

const cover = computed(() => {
  const url = getFirstImageUrl(props.product?.images)
  return url ? getImageUrl(url) : ''
})

const tags = computed(() => {
  const raw = props.product?.tags
  if (!Array.isArray(raw)) return []
  return raw
    .map((tag: any) => (typeof tag === 'string' ? tag : getLocalizedText(tag?.name ?? tag)))
    .filter((tag: string) => tag)
    .slice(0, props.maxTags)
})

const categoryName = computed(() => {
  const name = props.product?.category?.name
  return name ? getLocalizedText(name) : ''
})
</script>

<style scoped>
.atlas-pcard {
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border-radius: var(--r-lg);
  background: var(--surface);
  box-shadow: var(--shadow-sm);
  cursor: pointer;
  transition: box-shadow var(--dur) var(--ease), transform var(--dur) var(--ease);
}

.atlas-pcard:hover {
  box-shadow: var(--shadow-md);
  transform: translateY(-2px);
}

.atlas-pcard__media {
  display: grid;
  place-items: center;
  aspect-ratio: 4 / 3;
  border-radius: 0;
}

.atlas-pcard__body {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 6px;
  padding: 12px;
}

.atlas-pcard__title {
  display: -webkit-box;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  font-size: 14px;
  font-weight: 600;
  line-height: 1.35;
}

.atlas-pcard__cat {
  color: var(--ink-4);
  font-size: 11px;
}

.atlas-pcard__foot {
  display: flex;
  align-items: baseline;
  gap: 4px;
  margin-top: auto;
  padding-top: 8px;
}

.atlas-pcard__price {
  font-size: 17px;
}
</style>
