<template>
  <div class="bp" :class="[`bp--${type}`, `bp--${variant}`]">
    <!-- 大横幅 -->
    <template v-if="type === 'hero'">
      <div class="bp-hero" :class="{ 'is-overlay': p.variant === 'overlay', 'is-light': p.theme === 'light' }">
        <img v-if="p.media?.src && variant === 'full'" :src="p.media.src" class="bp-hero-img" alt="" />
        <div class="bp-hero-copy">
          <span v-if="txt(p.eyebrow)" class="bp-eyebrow">{{ txt(p.eyebrow) }}</span>
          <span class="bp-title">{{ txt(p.title) || (variant === 'thumb' ? '' : '大标题') }}</span>
          <span v-if="txt(p.subtitle)" class="bp-line">{{ txt(p.subtitle) }}</span>
          <div v-if="p.ctas?.length" class="bp-ctas">
            <span v-for="(c, i) in p.ctas.slice(0, 2)" :key="i" class="bp-pill" :class="{ ghost: c.style === 'ghost' }">{{ txt(c.label) || '按钮' }}</span>
          </div>
        </div>
        <div v-if="!p.media?.src && variant === 'full'" class="bp-imgnote">配图区</div>
      </div>
    </template>

    <!-- 双栏推广 -->
    <template v-else-if="type === 'promo'">
      <div class="bp-promo">
        <div class="bp-promo-copy">
          <span class="bp-title sm">{{ txt(p.title) || '标题' }}</span>
          <span class="bp-line">{{ txt(p.body) || '说明文字' }}</span>
        </div>
        <div class="bp-media-box">{{ p.media?.src ? '' : (variant === 'full' ? '配图' : '') }}</div>
      </div>
    </template>

    <!-- 章节头 -->
    <template v-else-if="type === 'sectionHeader'">
      <div class="bp-section">
        <span v-if="txt(p.eyebrow)" class="bp-eyebrow">{{ txt(p.eyebrow) }}</span>
        <span class="bp-title md">{{ txt(p.title) || '章节大标题' }}</span>
        <span class="bp-line">{{ txt(p.lead) || '说明文字' }}</span>
      </div>
    </template>

    <!-- 特性网格 -->
    <template v-else-if="type === 'featureGrid'">
      <div class="bp-grid" :style="{ gridTemplateColumns: `repeat(${Number(p.columns) || 2}, 1fr)` }">
        <div v-for="(item, i) in gridItems" :key="i" class="bp-card">
          <div class="bp-media-box sm">
            <img v-if="item.media?.src && variant === 'full'" :src="item.media.src" alt="" />
            <span v-else-if="variant === 'full'">图 {{ Number(i) + 1 }}</span>
          </div>
          <span class="bp-card-title">{{ txt(item.title) || '小标题' }}</span>
          <span class="bp-card-body">{{ txt(item.body) || '说明' }}</span>
        </div>
      </div>
    </template>

    <!-- 横滑卡片 -->
    <template v-else-if="type === 'cardScroller'">
      <div class="bp-scroller">
        <div v-for="(card, i) in scrollerCards" :key="i" class="bp-scard">
          <div class="bp-media-box sm">
            <img v-if="card.media?.src && variant === 'full'" :src="card.media.src" alt="" />
            <span v-else-if="variant === 'full'">图</span>
          </div>
          <span class="bp-card-title">{{ txt(card.title) || '卡片标题' }}</span>
        </div>
      </div>
    </template>

    <!-- 图库 -->
    <template v-else-if="type === 'gallery'">
      <div class="bp-gallery">
        <div v-for="(item, i) in galleryItems" :key="i" class="bp-gitem">
          <img v-if="item.media?.src && variant === 'full'" :src="item.media.src" alt="" />
          <span v-else-if="variant === 'full'">图 {{ Number(i) + 1 }}</span>
        </div>
      </div>
    </template>

    <!-- 影片 -->
    <template v-else-if="type === 'video'">
      <div class="bp-video">
        <span v-if="p.poster?.src" class="bp-video-poster" :style="{ backgroundImage: `url(${p.poster.src})` }" />
        <span class="bp-play" />
        <span v-if="variant === 'full'" class="bp-video-note">16:9 影片</span>
      </div>
    </template>

    <!-- 参数 -->
    <template v-else-if="type === 'specs'">
      <div class="bp-specs">
        <div v-for="(row, i) in specItems" :key="i" class="bp-spec-row">
          <span>{{ txt(row.label) || '项目' }}</span>
          <span>{{ txt(row.value) || '值' }}</span>
        </div>
      </div>
    </template>

    <!-- 购买区 -->
    <template v-else-if="type === 'cta'">
      <div class="bp-cta">
        <span class="bp-title sm">{{ txt(p.title) || '购买标题' }}</span>
        <span class="bp-line">{{ txt(p.subtitle) || '说明' }}</span>
        <div class="bp-ctas">
          <span class="bp-pill">{{ txt(p.price) || '价格' }}</span>
          <span class="bp-pill primary">购买</span>
        </div>
      </div>
    </template>

    <!-- 富文本 -->
    <template v-else-if="type === 'richText'">
      <div class="bp-rich">
        <span class="bp-line w90" /><span class="bp-line w80" /><span class="bp-line w95" /><span class="bp-line w60" />
      </div>
    </template>

    <!-- 未知类型 -->
    <template v-else>
      <div class="bp-unknown">{{ type }}</div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const props = withDefaults(defineProps<{ type: string; props?: any; variant?: 'thumb' | 'full' }>(), {
  props: () => ({}),
  variant: 'thumb',
})
const p = props.props
const fill = (n: number) => Array.from({ length: n }, () => ({}))
const gridItems = computed(() => (p.items?.length ? p.items.slice(0, 4) : fill(Math.min(Number(p.columns) || 2, 4))))
const scrollerCards = computed(() => (p.cards?.length ? p.cards.slice(0, 3) : fill(3)))
const galleryItems = computed(() => (p.items?.length ? p.items.slice(0, 4) : fill(4)))
const specItems = computed(() => (p.items?.length ? p.items.slice(0, 4) : fill(3)))
const { locale } = useI18n()

/** 多语言字段取当前语言，缺失回退简中 */
const txt = (value: any): string => {
  if (!value) return ''
  if (typeof value === 'string') return value
  return value[locale.value] || value['zh-CN'] || value['en-US'] || ''
}
</script>

<style scoped>
.bp { width: 100%; }
.bp--thumb { font-size: 0; }

.bp-hero {
  position: relative;
  display: flex;
  align-items: center;
  min-height: 128px;
  padding: 16px;
  border-radius: 10px;
  background: #1d1d1f;
  overflow: hidden;
}
.bp--thumb .bp-hero { min-height: 44px; padding: 8px; border-radius: 6px; }
.bp-hero.is-light { background: #f5f5f7; }
.bp-hero-img { position: absolute; inset: 0; width: 100%; height: 100%; object-fit: cover; opacity: 0.5; }
.bp-hero-copy { position: relative; display: flex; flex-direction: column; gap: 6px; min-width: 0; }
.bp-hero.is-overlay { align-items: flex-end; }
.bp-imgnote {
  position: absolute; right: 12px; top: 12px;
  padding: 4px 8px; border-radius: 6px; background: rgba(255,255,255,.16);
  color: #fff; font-size: 11px;
}

.bp-eyebrow { font-size: 12px; color: #86868b; }
.bp--thumb .bp-eyebrow { display: none; }
.bp-title { font-size: 22px; font-weight: 600; color: #fff; line-height: 1.2; }
.bp-hero.is-light .bp-title { color: #1d1d1f; }
.bp-title.md { font-size: 18px; }
.bp-title.sm { font-size: 15px; }
.bp--thumb .bp-title { font-size: 9px; color: #1d1d1f; }
.bp-line { font-size: 13px; color: #a1a1a6; }
.bp--thumb .bp-line { display: none; }
.bp-ctas { display: flex; gap: 8px; margin-top: 4px; }
.bp--thumb .bp-ctas { display: none; }
.bp-pill {
  padding: 4px 12px; border-radius: 980px; background: #0071e3; color: #fff; font-size: 12px;
}
.bp-pill.ghost { background: transparent; border: 1px solid rgba(255,255,255,.4); }
.bp-pill.primary { background: #0071e3; }

.bp-promo { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; align-items: center; }
.bp-promo-copy { display: flex; flex-direction: column; gap: 6px; }
.bp-promo-copy .bp-title { color: #1d1d1f; }
.bp-media-box {
  display: flex; align-items: center; justify-content: center;
  min-height: 72px; border-radius: 8px; border: 1px dashed #d2d2d7;
  background: #f5f5f7; color: #86868b; font-size: 11px; overflow: hidden;
}
.bp-media-box.sm { min-height: 52px; }
.bp-media-box img { width: 100%; height: 100%; object-fit: cover; }

.bp-section { display: flex; flex-direction: column; align-items: center; gap: 6px; text-align: center; padding: 8px 0; }
.bp-section .bp-title { color: #1d1d1f; }
.bp--thumb .bp-section .bp-title { font-size: 10px; }
.bp--thumb .bp-section { padding: 4px 0; }

.bp-grid { display: grid; gap: 10px; }
.bp-card { display: flex; flex-direction: column; gap: 4px; }
.bp-card-title { font-size: 13px; font-weight: 600; color: #1d1d1f; }
.bp--thumb .bp-card-title { font-size: 9px; }
.bp-card-body { font-size: 12px; color: #6e6e73; }
.bp--thumb .bp-card-body { display: none; }
.bp--thumb .bp-grid { gap: 6px; }

.bp-scroller { display: flex; gap: 10px; overflow: hidden; }
.bp-scard { flex: 1; display: flex; flex-direction: column; gap: 4px; min-width: 0; }

.bp-gallery { display: grid; grid-template-columns: repeat(4, 1fr); gap: 8px; }
.bp-gitem {
  aspect-ratio: 4 / 5; display: flex; align-items: center; justify-content: center;
  border-radius: 8px; border: 1px dashed #d2d2d7; background: #f5f5f7;
  color: #86868b; font-size: 11px; overflow: hidden;
}
.bp--thumb .bp-gitem { aspect-ratio: 1; border-radius: 4px; }
.bp-gitem img { width: 100%; height: 100%; object-fit: cover; }

.bp-video {
  position: relative; aspect-ratio: 16 / 9; border-radius: 10px; background: #1d1d1f;
  display: flex; align-items: center; justify-content: center; overflow: hidden;
}
.bp-video-poster { position: absolute; inset: 0; background-size: cover; background-position: center; opacity: .6; }
.bp-play {
  position: relative; width: 34px; height: 34px; border-radius: 50%; background: rgba(255,255,255,.9);
}
.bp-play::after {
  content: ''; position: absolute; left: 13px; top: 10px;
  border-left: 11px solid #1d1d1f; border-top: 7px solid transparent; border-bottom: 7px solid transparent;
}
.bp--thumb .bp-play { width: 18px; height: 18px; }
.bp--thumb .bp-play::after { left: 7px; top: 5px; border-left-width: 6px; border-top-width: 4px; border-bottom-width: 4px; }
.bp-video-note { position: absolute; right: 10px; bottom: 8px; font-size: 11px; color: #a1a1a6; }

.bp-specs { display: flex; flex-direction: column; }
.bp-spec-row {
  display: grid; grid-template-columns: 1fr 1fr; gap: 8px; padding: 7px 0;
  border-top: 1px solid #e8e8ed; font-size: 12px; color: #1d1d1f;
}
.bp-spec-row span:last-child { color: #6e6e73; text-align: right; }
.bp--thumb .bp-spec-row { padding: 3px 0; font-size: 9px; }

.bp-cta { display: flex; flex-direction: column; align-items: center; gap: 6px; padding: 10px 0; text-align: center; }
.bp-cta .bp-title { color: #1d1d1f; }

.bp-rich { display: flex; flex-direction: column; gap: 7px; padding: 4px 0; }
.bp-rich .bp-line { display: block; height: 8px; border-radius: 4px; background: #e8e8ed; }
.bp-rich .w90 { width: 90%; } .bp-rich .w80 { width: 80%; } .bp-rich .w95 { width: 95%; } .bp-rich .w60 { width: 60%; }

.bp-unknown { padding: 12px; color: #86868b; font-size: 12px; }
</style>
