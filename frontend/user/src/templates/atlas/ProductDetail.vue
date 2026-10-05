<template>
  <div>
    <!-- ==================== 合规提示条 ==================== -->
    <div class="atlas-tips">
      <div class="atlas-shell atlas-tips__row">
        <Info class="h-4 w-4" />
        <span>{{ t('atlas.detail.tips') }}</span>
      </div>
    </div>

    <div class="atlas-shell">
      <!-- ==================== 面包屑（64px 行高） ==================== -->
      <nav class="atlas-dcrumb">
        <RouterLink to="/">{{ t('nav.home') }}</RouterLink>
        <span class="atlas-dcrumb__sep">/</span>
        <RouterLink v-if="categoryName && categorySlug" :to="`/categories/${categorySlug}`">{{ categoryName }}</RouterLink>
        <span v-else-if="categoryName">{{ categoryName }}</span>
        <span class="atlas-dcrumb__sep">/</span>
        <span class="atlas-dcrumb__cur">{{ getLocalizedText(product?.title) }}</span>
      </nav>

      <!-- ==================== 加载中 ==================== -->
      <div v-if="loading" class="atlas-dhead">
        <div class="atlas-dhead__row">
          <div class="atlas-dhead__img atlas-skeleton"></div>
          <div class="atlas-dhead__info w-full">
            <div class="atlas-skeleton h-8 w-1/3"></div>
          </div>
        </div>
      </div>

      <!-- ==================== 商品头：168px 图 + 标题 + 服务徽章 ==================== -->
      <div v-else-if="product" class="atlas-dhead">
        <div class="atlas-dhead__row">
          <div class="atlas-dhead__img">
            <img v-if="headImage" :src="headImage" :alt="getLocalizedText(product.title)" />
          </div>
          <div class="atlas-dhead__info">
            <h1>{{ getLocalizedText(product.title) }}</h1>
            <div class="atlas-dhead__badges">
              <div v-for="badge in badges" :key="badge.key" class="atlas-dhead__badge">
                <component :is="badge.icon" class="atlas-dhead__badge-icon" :stroke-width="1.5" />
                <div>
                  <strong>{{ t(badge.titleKey) }}</strong>
                  <p>{{ t(badge.subKey) }}</p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- ==================== 主体两栏 ==================== -->
      <div v-if="!loading && product" class="atlas-dmain">
        <!-- 左栏 950px -->
        <div class="atlas-dleft">
          <!-- 规格选择 -->
          <div class="atlas-dbox">
            <div class="atlas-dsku">
              <h2 class="atlas-dtitle">{{ t('atlas.detail.specTitle') }}</h2>
              <div class="atlas-dsku__list">
                <button
                  v-for="sku in activeSkus"
                  :key="sku.id"
                  class="atlas-sku-item"
                  :class="[
                    normalizeSkuId(sku.id) === selectedSkuId && 'atlas-sku-item--on',
                    !isSkuPurchasable(sku) && 'atlas-sku-item--off',
                  ]"
                  type="button"
                  :disabled="!isSkuPurchasable(sku)"
                  @click="selectedSkuId = normalizeSkuId(sku.id)"
                >
                  <span class="atlas-sku-item__name">{{ skuDisplayText(sku) }}</span>
                  <span class="atlas-sku-item__stock">{{ skuStockText(sku) }}</span>
                </button>
              </div>
              <p v-if="requiresSKUSelection" class="atlas-dsku__hint">{{ t('atlas.detail.selectSku') }}</p>
            </div>
          </div>

          <!-- 商品说明：内容只取「商品详情」(content)，见下方 descHtml -->
          <div class="atlas-dbox">
            <h2 class="atlas-dtitle">{{ t('atlas.detail.desc') }}</h2>
            <div class="atlas-ddesc theme-prose">
              <div v-if="descHtml" v-html="descHtml"></div>
              <p v-else class="text-[color:var(--ink-3)]">{{ t('atlas.detail.noDesc') }}</p>
            </div>
          </div>
        </div>

        <!-- 右栏 375px（吸顶） -->
        <div class="atlas-dright">
          <div class="atlas-dbox atlas-dsticky">
            <div class="atlas-pricebox">
              <div class="atlas-pricebox__row">
                <span class="atlas-pricebox__label">{{ t('atlas.detail.price') }}</span>
                <span class="atlas-pricebox__amt">
                  <del v-if="showBasePrice">{{ formatPrice(product.price_amount, siteCurrency) }}</del>
                  <strong>{{ formatPrice(currentPriceAmount, siteCurrency) }}</strong>
                </span>
              </div>
              <p v-if="purchaseWarning" class="mt-2 text-[12px] text-[color:var(--warn)]">{{ purchaseWarning }}</p>
            </div>

            <!-- 未登录时只给一个入口，文案用 productDetail.loginToBuy（与 classic 一致），
                 避免按钮写「立即购买」却跳登录页这种误导 -->
            <button
              v-if="requiresLogin"
              class="atlas-buybtn"
              type="button"
              @click="goLogin"
            >{{ t('productDetail.loginToBuy') }}</button>
            <template v-else>
              <button class="atlas-buybtn" type="button" :disabled="!canPurchase" @click="buyNow">
                {{ t('productDetail.buyNow') }}
              </button>
              <button
                class="atlas-buybtn atlas-buybtn--ghost"
                type="button"
                :disabled="!canPurchase"
                @click="addToCart"
              >{{ t('productDetail.addToCart') }}</button>
            </template>

            <p v-if="cannotPurchaseReason" class="mt-3 text-[12px] text-[color:var(--danger)]">
              {{ cannotPurchaseReason }}
            </p>
          </div>
        </div>
      </div>

      <!-- ==================== 商品不存在 ==================== -->
      <EmptyState
        v-else-if="!loading"
        variant="soft"
        size="lg"
        icon="package"
        :title="t('products.empty')"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { BadgeCheck, Info, ShieldCheck, Zap } from 'lucide-vue-next'
import { useProductDetail } from '../../composables/useProductDetail'
import { useLocalized } from '../../composables/useProduct'
import { usePageSeo } from '../../composables/usePageSeo'
import { getImageUrl } from '../../utils/image'
import { processHtmlForDisplay } from '../../utils/content'
import EmptyState from '../../components/EmptyState.vue'

const { t } = useI18n()
const { getLocalizedText } = useLocalized()

const {
  formatPrice, siteCurrency, normalizeSkuId,
  loading, product, currentImage, categoryName, selectedSkuId,
  activeSkus, selectedSku,
  isSkuPurchasable, skuDisplayText, skuStockText,
  requiresLogin, requiresSKUSelection, canPurchase, cannotPurchaseReason,
  purchaseWarning,
  addToCart, buyNow, goLogin,
} = useProductDetail()

usePageSeo({
  canonicalPath: () => `/products/${product.value?.slug || ''}`,
  title: () => getLocalizedText(product.value?.title) || t('nav.products'),
})

/* 参考站的三个服务徽章（32px 图标 + 14px 主文案 + 12px 副文案） */
const badges = [
  { key: 'official', icon: BadgeCheck, titleKey: 'atlas.detail.badge.official', subKey: 'atlas.detail.badge.officialSub' },
  { key: 'instant', icon: Zap, titleKey: 'atlas.detail.badge.instant', subKey: 'atlas.detail.badge.instantSub' },
  { key: 'after', icon: ShieldCheck, titleKey: 'atlas.detail.badge.after', subKey: 'atlas.detail.badge.afterSub' },
]

const headImage = computed(() => {
  const raw = currentImage.value || (Array.isArray(product.value?.images) ? product.value.images[0] : '')
  return raw ? getImageUrl(raw) : ''
})

/* 分类 slug 用于面包屑跳转（category 对象里带 slug） */
const categorySlug = computed(() => String(product.value?.category?.slug || '').trim())

/* 价格：优先选中规格的价，否则商品基础价 */
const currentPriceAmount = computed(() => {
  const skuPrice = selectedSku.value?.price_amount
  return skuPrice ?? product.value?.price_amount
})

/* 选中规格价与商品基础价不一致时，把基础价作为划线原价展示 */
const showBasePrice = computed(() => {
  const base = product.value?.price_amount
  const cur = currentPriceAmount.value
  if (base === null || base === undefined || cur === null || cur === undefined) return false
  return Number(base) > Number(cur)
})

/**
 * 「商品说明」区块的内容 —— 只取「商品详情」(content)，不做回退。
 * 商品详情为空时直接显示「该商品暂无说明」。
 *
 * 为什么不能直接 v-html="product.content"：接口返回的 content 是**按语言的对象**
 * （{ 'zh-CN': '<p>…</p>', en: '…' }），必须先过 getLocalizedText 取出当前语言的
 * 字符串；正文里的 /uploads/xxx 图片路径还要过 processHtmlForDisplay 补成绝对地址。
 * 原实现这两步都没做，只是因为它排在 description 之后、而线上商品的 description
 * 恒非空，该分支从未执行才没暴露。
 */
const descHtml = computed(() => {
  const raw = getLocalizedText(product.value?.content)
  return raw ? processHtmlForDisplay(raw) : ''
})
</script>

<style scoped>
/* 面包屑：参考站 detail__StyledCrumb —— 64px 行高，悬停变红 */
.atlas-dcrumb {
  display: flex;
  align-items: center;
  height: 64px;
  color: var(--ink);
  font-size: 14px;
}

.atlas-dcrumb a { color: var(--ink); }
.atlas-dcrumb a:hover { color: var(--accent); }
.atlas-dcrumb__sep { margin: 0 4px; font-size: 10px; }
.atlas-dcrumb__cur { color: var(--ink); }

.atlas-dsku__hint {
  margin-top: 10px;
  color: var(--warn);
  font-size: 12px;
}

/* 右栏吸顶：参考站 top:114px */
.atlas-dsticky {
  position: sticky;
  top: 114px;
}

.atlas-ddesc {
  font-size: 14px;
  line-height: 1.7;
  color: var(--ink-2);
  word-break: break-word;
}

@media (max-width: 1023px) {
  .atlas-dcrumb { height: 44px; }
  .atlas-dsticky { position: static; }
}
</style>
