<template>
  <!--
    推荐位卡片（参考站 .recommend-item 的运营位形态）。
    参考站这里放的是运营配图；本站没有配图来源，按需求「直接使用商品图片作为图标」：
    底图用商品图放大模糊铺底（给出与商品一致的主色调），
    左上角放商品图作为 70x70 圆角图标，右侧标题 + 标签 + CTA。
    这样每张推荐位都有图、不再是无图的纯渐变占位。
  -->
  <RouterLink class="atlas-promo" :class="large && 'atlas-promo--lg'" :to="to">
    <div class="atlas-promo__mask">
      <img v-if="image" :src="image" alt="" aria-hidden="true" />
    </div>

    <div class="atlas-promo__body">
      <div class="atlas-promo__img">
        <img v-if="image" :src="image" :alt="title" loading="lazy" decoding="async" />
      </div>
      <div class="atlas-promo__right">
        <div class="atlas-promo__title">{{ title }}</div>
        <div v-if="desc" class="atlas-promo__desc">{{ desc }}</div>
        <span v-if="large" class="atlas-promo__cta">{{ cta }}</span>
      </div>
    </div>
  </RouterLink>
</template>

<script setup lang="ts">
withDefaults(
  defineProps<{
    to: string
    title: string
    image?: string
    desc?: string
    cta?: string
    large?: boolean
  }>(),
  { image: '', desc: '', cta: '', large: false },
)
</script>

<style scoped>
.atlas-promo {
  position: relative;
  display: block;
  aspect-ratio: 335 / 178;
  border-radius: 12px;
  overflow: hidden;
  background: #1a1a1a;
  box-shadow: var(--shadow-card);
  transition: box-shadow var(--dur) var(--ease), transform var(--dur) var(--ease);
}

.atlas-promo:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 20px rgba(0, 0, 0, 0.12);
}

.atlas-promo--lg {
  aspect-ratio: 700 / 386;
}

/* 底图：商品图放大模糊铺底，保证每张卡都有主色调（与商品卡同一套处理） */
.atlas-promo__mask {
  position: absolute;
  inset: 0;
  z-index: 0;
  overflow: hidden;
}

.atlas-promo__mask img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  transform: scale(2);
  filter: blur(35px) saturate(3) brightness(0.8);
}

.atlas-promo__body {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
  gap: 15px;
  height: 100%;
  padding: 28px 26px;
}

.atlas-promo__img {
  display: grid;
  place-items: center;
  width: 70px;
  height: 70px;
  flex: none;
  border-radius: 12px;
  overflow: hidden;
  background: rgba(255, 255, 255, 0.12);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
}

.atlas-promo--lg .atlas-promo__img {
  width: 96px;
  height: 96px;
}

.atlas-promo__img img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.atlas-promo__right {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  min-width: 0;
}

.atlas-promo__title {
  display: -webkit-box;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  color: #fff;
  font-size: 18px;
  font-weight: 700;
  line-height: 20px;
  text-shadow: 0 1px 4px rgba(0, 0, 0, 0.2);
}

.atlas-promo--lg .atlas-promo__title {
  font-size: 26px;
  line-height: 30px;
}

.atlas-promo__desc {
  max-width: 100%;
  margin-top: 6px;
  padding: 0 8px;
  border-radius: 4px;
  background: rgba(255, 255, 255, 0.2);
  backdrop-filter: blur(5px);
  color: #fff;
  font-size: 11px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.atlas-promo__cta {
  display: inline-flex;
  align-items: center;
  height: 32px;
  margin-top: 14px;
  padding: 0 18px;
  border-radius: var(--r-pill);
  background: #fff;
  color: #1a1a1a;
  font-size: 13px;
  font-weight: 600;
}
</style>
