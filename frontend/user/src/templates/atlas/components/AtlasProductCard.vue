<template>
  <!--
    参考站商品卡（.recommend-item），结构为「上半部实景图 + 下半部价格」：
      .recommend-item        302x180; radius 8; shadow 0 4px 12px rgba(0,0,0,.05)
                             transition all .35s cubic-bezier(.25,1,.5,1); bg #fff
      .recommend-top         高 134; padding 28/26; flex; align-center; gap 15;
                             relative; overflow hidden; radius 8 8 0 0
        .btAmVe（遮罩）      absolute inset-0; bg #1a1a1a
                             img: cover; transform scale(2);
                                  blur(35px) saturate(3) brightness(.8)
        .img                 70x70; radius 12; shadow 0 4px 12px rgba(0,0,0,.2); z-index 1
        .recommend-right     flex column; align-items flex-start; z-index 1
          .title-box         高 37; flex align-center
          .title             白 18px/700; line-height 20; max-width 160; 2 行截断;
                             text-shadow 0 1px 4px rgba(0,0,0,.2)
          .desc              白 11px; bg rgba(255,255,255,.2); backdrop-blur 5px;
                             padding 0 8; radius 4; margin-top 6; 单行截断
      .recommend-bottom      padding 10; z-index 1
        .info                flex; align-end; gap 10; justify-between
          span（价格）       color var(--mainColor); font-weight bold; bottom -2px
                             small 12px（货币符号） + strong 23px（数字）
          .sale              「起」12px #aaa
    悬停时底图放大到 1.5 倍。
  -->
  <article class="atlas-pcard" @click="$emit('click', product.slug)">
    <div class="atlas-pcard__top">
      <div class="atlas-pcard__mask">
        <img v-if="cover" class="atlas-pcard__bg" :src="cover" alt="" aria-hidden="true" />
      </div>

      <div class="atlas-pcard__img">
        <img v-if="cover" :src="cover" :alt="getLocalizedText(product.title)" loading="lazy" decoding="async" />
        <ImageOff v-else class="h-6 w-6 text-white/50" />
      </div>

      <div class="atlas-pcard__right">
        <div class="atlas-pcard__titlebox">
          <div class="atlas-pcard__title">{{ getLocalizedText(product.title) }}</div>
        </div>
        <div v-if="tagsText" class="atlas-pcard__desc">{{ tagsText }}</div>
      </div>
    </div>

    <div class="atlas-pcard__bottom">
      <div class="atlas-pcard__info">
        <span class="atlas-pcard__price">
          <small>{{ currencySymbol }}</small><strong>{{ amountText }}</strong>
        </span>
        <span class="atlas-pcard__sale">{{ t('atlas.product.from') }}</span>
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
  { maxTags: 3 },
)

defineEmits<{ click: [slug: string] }>()

const { t } = useI18n()
const { getLocalizedText, siteCurrency } = useLocalized()

const cover = computed(() => {
  const url = getFirstImageUrl(props.product?.images)
  return url ? getImageUrl(url) : ''
})

/* 参考站的标签是一行纯文本（如「平台账号,CDK,手游账号」），不是多个 pill */
const tagsText = computed(() => {
  const raw = props.product?.tags
  if (!Array.isArray(raw)) return ''
  return raw
    .map((tag: any) => (typeof tag === 'string' ? tag : getLocalizedText(tag?.name ?? tag)))
    .filter((tag: string) => tag)
    .slice(0, props.maxTags)
    .join(',')
})

/* 货币符号：参考站是「小号￥ + 大号数字」，所以不能用 formatPrice（它会把币种码缀在数字后） */
const currencySymbol = computed(() => {
  const map: Record<string, string> = {
    CNY: '¥', TWD: 'NT$', USD: '$', EUR: '€', JPY: '¥', HKD: 'HK$', GBP: '£', KRW: '₩',
  }
  const cur = siteCurrency.value
  return map[cur] || cur
})

/* 数字：去掉多余的小数尾零（参考站显示 26.8 / 20，而不是 26.80 / 20.00） */
const amountText = computed(() => {
  const raw = props.product?.price_amount
  if (raw === null || raw === undefined || raw === '') return '-'
  const num = Number(raw)
  if (!Number.isFinite(num)) return String(raw)
  return String(Number(num.toFixed(2)))
})
</script>

<style scoped>
/* 卡片本体：302x180 定尺，网格给到 ~303 宽，所以用 width:100% + 固定高 */
.atlas-pcard {
  position: relative;
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 180px;
  border-radius: 8px;
  background: #ffffff;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
  transition: all 0.35s cubic-bezier(0.25, 1, 0.5, 1);
  cursor: pointer;
  overflow: hidden;
}

.atlas-pcard:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 20px rgba(0, 0, 0, 0.1);
}

.atlas-pcard:hover .atlas-pcard__bg {
  transform: scale(1.5);
}

/* 上半部：134px，底图 + 70x70 图标 + 右侧文字 */
.atlas-pcard__top {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: 15px;
  height: 134px;
  padding: 28px 26px;
  overflow: hidden;
  border-radius: 8px 8px 0 0;
}

/* 底图遮罩：用商品图自身放大模糊做背景，避免依赖运营配图 */
.atlas-pcard__mask {
  position: absolute;
  left: 0;
  top: 0;
  z-index: 0;
  display: flex;
  align-items: center;
  width: 100%;
  height: 134px;
  overflow: hidden;
  background: #1a1a1a;
}

.atlas-pcard__bg {
  position: absolute;
  top: 0;
  left: 0;
  z-index: 1;
  width: 100%;
  height: 100%;
  object-fit: cover;
  transform: scale(2);
  filter: blur(35px) saturate(3) brightness(0.8);
  transition: transform 0.35s cubic-bezier(0.25, 1, 0.5, 1);
}

.atlas-pcard__img {
  position: relative;
  z-index: 1;
  display: grid;
  place-items: center;
  width: 70px;
  height: 70px;
  flex-shrink: 0;
  border-radius: 12px;
  overflow: hidden;
  background: rgba(255, 255, 255, 0.12);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
}

.atlas-pcard__img img {
  width: 100%;
  height: 100%;
  border-radius: 12px;
  object-fit: cover;
}

.atlas-pcard__right {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  justify-content: flex-start;
  min-width: 0;
}

.atlas-pcard__titlebox {
  display: flex;
  align-items: center;
  height: 37px;
}

.atlas-pcard__title {
  display: -webkit-box;
  max-width: 160px;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  margin: 0;
  color: #ffffff;
  font-size: 18px;
  font-weight: 700;
  line-height: 20px;
  text-shadow: 0 1px 4px rgba(0, 0, 0, 0.2);
}

.atlas-pcard__desc {
  display: inline-block;
  max-width: 160px;
  margin-top: 6px;
  padding: 0 8px;
  border-radius: 4px;
  background: rgba(255, 255, 255, 0.2);
  backdrop-filter: blur(5px);
  color: #ffffff;
  font-size: 11px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* 下半部：价格行 */
.atlas-pcard__bottom {
  position: relative;
  z-index: 1;
  padding: 10px;
}

.atlas-pcard__info {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 10px;
}

.atlas-pcard__price {
  position: relative;
  bottom: -2px;
  display: flex;
  align-items: flex-end;
  gap: 2px;
  color: var(--accent);
  font-weight: bold;
  font-size: 12px;
}

.atlas-pcard__price small {
  font-size: 12px;
}

.atlas-pcard__price strong {
  font-size: 23px;
  font-weight: bold;
  font-variant-numeric: tabular-nums;
  line-height: 1;
}

.atlas-pcard__sale {
  flex: none;
  color: #aaaaaa;
  font-size: 12px;
}
</style>
