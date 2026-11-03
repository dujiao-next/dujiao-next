<template>
  <div class="st-root st-page nf-page">
    <div class="st-container nf-inner">
      <p class="nf-code">404</p>
      <h1 class="st-page-title nf-title">{{ t('notFoundPage.title') }}</h1>
      <p class="nf-desc">{{ t('notFoundPage.description', { site: brandSiteName }) }}</p>

      <nav class="nf-links">
        <RouterLink class="nf-link" to="/">{{ t('notFoundPage.backHome') }}</RouterLink>
        <button type="button" class="nf-link" @click="goBack">{{ t('notFoundPage.backPrevious') }}</button>
      </nav>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useNotFound } from '../../composables/useNotFound'

const { t } = useI18n()

const { brandSiteName, goBack } = useNotFound()
</script>

<style scoped>
.nf-page {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 60vh;
  min-height: max(420px, calc(100svh - 300px));
  padding-top: 56px;
}
@media (min-width: 735px) {
  .nf-page {
    padding-top: 80px;
  }
}

.nf-inner {
  text-align: center;
}

.nf-code {
  margin: 0 0 14px;
  font-size: 12px;
  font-weight: 600;
  line-height: 1.3;
  letter-spacing: 0.08em;
  color: var(--st-text-3);
}

.nf-title {
  font-size: 48px;
  line-height: 1.05;
  letter-spacing: -0.02em;
  text-wrap: balance;
}
@media (min-width: 735px) {
  .nf-title {
    font-size: 64px;
  }
}

.nf-desc {
  max-width: 460px;
  margin: 18px auto 0;
  font-size: 17px;
  line-height: 1.5;
  color: var(--st-text-2);
  text-wrap: pretty;
}
@media (min-width: 735px) {
  .nf-desc {
    font-size: 19px;
  }
}

.nf-links {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: center;
  gap: 12px 28px;
  margin-top: 36px;
}
.nf-link {
  padding: 0;
  border: 0;
  background: none;
  font: inherit;
  font-size: 17px;
  line-height: 1.4;
  color: var(--st-link);
  text-decoration: none;
  cursor: pointer;
}
.nf-link:hover {
  text-decoration: underline;
}
.nf-link:focus-visible {
  outline: 2px solid var(--st-link);
  outline-offset: 3px;
  border-radius: 2px;
}
</style>
