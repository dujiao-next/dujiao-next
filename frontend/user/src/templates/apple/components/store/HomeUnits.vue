<template>
  <div class="hu">
    <!-- 整幅推广单元：一屏一个，图占大半屏 -->
    <section
      v-for="unit in banners"
      :key="`b-${unit.id}`"
      class="hu-unit"
      :class="`hu-unit--${unit.tone || 'light'}`"
    >
      <div class="hu-copy">
        <p v-if="unit.eyebrow" class="hu-eyebrow">{{ unit.eyebrow }}</p>
        <h2 class="hu-title">{{ unit.title }}</h2>
        <p v-if="unit.subtitle" class="hu-sub">{{ unit.subtitle }}</p>
        <div class="hu-links">
          <RouterLink v-if="unit.to" class="hu-pill hu-pill--primary" :to="unit.to">
            {{ unit.linkLabel || '进一步了解' }}
          </RouterLink>
          <RouterLink v-if="unit.buyTo" class="hu-pill hu-pill--ghost" :to="unit.buyTo">
            {{ unit.buyLabel || '购买' }}
          </RouterLink>
        </div>
      </div>
      <div class="hu-media hu-media--banner">
        <picture>
          <source v-if="unit.mobileImage" :srcset="unit.mobileImage" media="(max-width: 733px)" />
          <img :src="unit.image" :alt="unit.title" loading="lazy" />
        </picture>
      </div>
    </section>

    <!-- 两列商品单元 -->
    <div v-if="products.length" class="hu-grid">
      <div
        v-for="item in products"
        :key="`p-${item.id}`"
        class="hu-unit hu-unit--half"
        :class="`hu-unit--${item.tone || 'light'}`"
      >
        <div class="hu-copy">
          <p v-if="item.eyebrow" class="hu-eyebrow" :style="item.eyebrowColor ? { color: item.eyebrowColor } : undefined">{{ item.eyebrow }}</p>
          <RouterLink class="hu-title hu-title--sm hu-title--link" :to="item.link">{{ item.title }}</RouterLink>
          <p class="hu-sub">{{ item.price }}</p>
          <div v-if="item.ctaTo || item.buyTo" class="hu-links">
            <RouterLink v-if="item.ctaTo" class="hu-pill hu-pill--primary" :to="item.ctaTo">
              {{ item.ctaLabel }}
            </RouterLink>
            <RouterLink v-if="item.buyTo" class="hu-pill hu-pill--ghost" :to="item.buyTo">
              {{ item.buyLabel }}
            </RouterLink>
          </div>
        </div>
        <RouterLink class="hu-media hu-media--product" :to="item.link" :aria-label="item.title">
          <picture>
            <source v-if="item.mobileImage" :srcset="item.mobileImage" media="(max-width: 733px)" />
            <img :src="item.image" :alt="item.title" loading="lazy" />
          </picture>
        </RouterLink>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { RouterLink } from 'vue-router'

defineProps<{
  banners: Array<{
    id: number | string
    eyebrow?: string
    title?: string
    subtitle?: string
    image: string
    mobileImage?: string
    to?: string
    buyTo?: string
    linkLabel?: string
    buyLabel?: string
    tone?: 'light' | 'dark'
  }>
  products: Array<{
    id: number | string
    slug: string
    eyebrow?: string
    eyebrowColor?: string
    title: string
    price: string
    image: string
    link: string
    mobileImage?: string
    ctaLabel?: string
    ctaTo?: string
    buyLabel?: string
    buyTo?: string
    tone?: 'light' | 'dark'
  }>
}>()
</script>

<style scoped>
.hu {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

/* ---- 整幅单元 ---- */
.hu-unit {
  position: relative;
  display: block;
  height: 620px;
  overflow: hidden;
  background: var(--st-bg-alt);
  text-decoration: none;
  color: inherit;
}
/* 深浅 = 文字颜色：深色底片用白字，浅色底片用黑字 */
.hu-unit--dark {
  background: #000;
  color: #f5f5f7;
}
.hu-unit--light {
  background: #f5f5f7;
  color: #1d1d1f;
}


.hu-copy {
  position: relative;
  z-index: 2;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  padding: 52px 24px 0;
  text-align: center;
}

.hu .hu-eyebrow {
  margin: 0;
  font-size: 14px;
  letter-spacing: -0.01em;
  color: #bf4800;
}
.hu-unit--dark .hu-eyebrow {
  color: #f56300;
}

.hu .hu-title {
  margin: 0;
  color: inherit;
  font-family: var(--st-font-display);
  font-size: clamp(40px, 4.6vw, 56px);
  line-height: 1.05;
  letter-spacing: -0.015em;
  font-weight: 600;
}
.hu .hu-title--sm {
  font-size: clamp(28px, 2.6vw, 40px);
}
.hu-title--link {
  text-decoration: none;
  color: inherit;
}
.hu-title--link:hover {
  text-decoration: underline;
}

.hu .hu-sub {
  margin: 0;
  color: inherit;
  font-size: clamp(17px, 1.6vw, 28px);
  line-height: 1.2;
  letter-spacing: 0.007em;
}

.hu-links {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 12px;
  margin-top: 18px;
  font-size: 17px;
}
.hu-pill {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 44px;
  padding: 0 22px;
  border-radius: 980px;
  font-size: 17px;
  line-height: 1.2;
  text-decoration: none;
  transition: background 240ms ease, color 240ms ease, border-color 240ms ease;
}
.hu-pill--primary {
  background: #0071e3;
  color: #fff;
  border: 1px solid #0071e3;
}
.hu-pill--primary:hover {
  background: #0077ed;
  border-color: #0077ed;
}
.hu-pill--ghost {
  background: transparent;
  color: #0071e3;
  border: 1px solid #0071e3;
}
.hu-unit--dark .hu-pill--ghost {
  color: #2997ff;
  border-color: #2997ff;
}
.hu-pill--ghost:hover {
  background: rgba(0, 113, 227, 0.08);
}
@media (max-width: 734px) {
  .hu-links {
    font-size: 17px;
    gap: 10px;
  }
  .hu-pill {
    min-height: 40px;
    padding: 0 18px;
    font-size: 15px;
  }
}

.hu-media {
  position: absolute;
  inset: 0;
  z-index: 0;
  margin: 0;
}
.hu-media img {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: cover;
  object-position: center top;
}
/* 商品海报完整展示，不裁切 */
.hu-media--product img {
  object-fit: cover;
  object-position: center;
}

/* 横幅是完整画面，用 contain 不裁切 */
.hu-media--banner img {
  object-fit: contain;
  object-position: center bottom;
}

/* ---- 两列网格 ---- */
.hu-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}
.hu-unit--half {
  height: 560px;
}


@media (max-width: 734px) {
  .hu-unit {
    height: 480px;
  }
  .hu-copy {
    padding: 36px 20px 0;
    gap: 8px;
  }
  .hu-links {
    font-size: 17px;
  }
  .hu-grid {
    grid-template-columns: 1fr;
  }
  .hu-unit--half {
    height: 420px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .hu-pill {
    transition: none;
  }
}
</style>
