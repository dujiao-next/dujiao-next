<template>
  <div class="st-root st-page">
    <div class="st-container">
      <div class="about-shell">
        <header class="st-page-head about-head">
          <h1 class="st-page-title">{{ heroTitle || t('nav.about') }}</h1>
          <p v-if="heroSubtitle" class="st-page-lead">{{ heroSubtitle }}</p>
        </header>

        <div class="about-body">
          <div v-if="bodyIsEmpty" class="about-intro">
            <p class="about-copy">{{ fallbackText }}</p>
          </div>
          <div v-if="hasIntroduction" class="about-intro">
            <p class="about-copy">{{ introductionText }}</p>
          </div>

          <section v-if="hasServices" class="about-section">
            <h2 v-if="servicesTitle" class="st-section-title">{{ servicesTitle }}</h2>
            <ul class="about-services">
              <li v-for="(service, index) in serviceItems" :key="`about-service-${index}`" class="about-service">
                <Check class="about-service-icon" aria-hidden="true" />
                <span>{{ service }}</span>
              </li>
            </ul>
          </section>

          <section v-if="hasContact" class="about-section">
            <h2 v-if="contactTitle" class="st-section-title">{{ contactTitle }}</h2>
            <p v-if="contactText" class="about-copy">{{ contactText }}</p>
            <div v-if="hasContactLinks" class="about-contact-list">
              <a
                v-if="contactConfig?.telegram"
                :href="contactConfig.telegram"
                target="_blank"
                rel="noopener noreferrer"
                class="about-contact-link"
              >
                <svg viewBox="0 0 24 24" fill="currentColor" class="about-contact-icon" aria-hidden="true"><path d="M12 0C5.373 0 0 5.373 0 12s5.373 12 12 12 12-5.373 12-12S18.627 0 12 0zm5.894 8.221l-1.97 9.28c-.145.658-.537.818-1.084.508l-3-2.21-1.446 1.394c-.14.18-.357.295-.6.295-.002 0-.003 0-.005 0l.213-3.054 5.56-5.022c.24-.213-.054-.334-.373-.121l-6.869 4.326-2.96-.924c-.64-.203-.658-.64.135-.954l11.566-4.458c.538-.196 1.006.128.832.941z" /></svg>
                <span>Telegram</span>
              </a>
              <a
                v-if="contactConfig?.whatsapp"
                :href="contactConfig.whatsapp"
                target="_blank"
                rel="noopener noreferrer"
                class="about-contact-link"
              >
                <svg viewBox="0 0 24 24" fill="currentColor" class="about-contact-icon" aria-hidden="true"><path d="M17.472 14.382c-.297-.149-1.758-.867-2.03-.967-.273-.099-.471-.148-.67.15-.197.297-.767.966-.94 1.164-.173.199-.347.223-.644.075-.297-.15-1.255-.463-2.39-1.475-.883-.788-1.48-1.761-1.653-2.059-.173-.297-.018-.458.13-.606.134-.133.298-.347.446-.52.149-.174.198-.298.298-.497.099-.198.05-.371-.025-.52-.075-.149-.669-1.612-.916-2.207-.242-.579-.487-.5-.669-.51-.173-.008-.371-.01-.57-.01-.198 0-.52.074-.792.372-.272.297-1.04 1.016-1.04 2.479 0 1.462 1.065 2.875 1.213 3.074.149.198 2.096 3.2 5.077 4.487.709.306 1.262.489 1.694.625.712.227 1.36.195 1.871.118.571-.085 1.758-.719 2.006-1.413.248-.694.248-1.289.173-1.413-.074-.124-.272-.198-.57-.347m-5.421 7.403h-.004a9.87 9.87 0 01-5.031-1.378l-.361-.214-3.741.982.998-3.648-.235-.374a9.86 9.86 0 01-1.51-5.26c.001-5.45 4.436-9.884 9.888-9.884 2.64 0 5.122 1.03 6.988 2.898a9.825 9.825 0 012.893 6.994c-.003 5.45-4.437 9.884-9.885 9.884m8.413-18.297A11.815 11.815 0 0012.05 0C5.495 0 .16 5.335.157 11.892c0 2.096.547 4.142 1.588 5.945L.057 24l6.305-1.654a11.882 11.882 0 005.683 1.448h.005c6.554 0 11.89-5.335 11.893-11.893a11.821 11.821 0 00-3.48-8.413Z" /></svg>
                <span>WhatsApp</span>
              </a>
              <a v-if="contactConfig?.email" :href="`mailto:${contactConfig.email}`" class="about-contact-link">
                <Mail class="about-contact-icon" aria-hidden="true" />
                <span>{{ t('footer.email') }}</span>
              </a>
              <a
                v-if="contactConfig?.support_url"
                :href="contactConfig.support_url"
                :target="contactConfig.support_url.startsWith('https://') ? '_blank' : undefined"
                rel="noopener noreferrer"
                class="about-contact-link"
              >
                <Headphones class="about-contact-icon" aria-hidden="true" />
                <span>{{ t('footer.supportUrl') }}</span>
              </a>
            </div>
          </section>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Check, Mail, Headphones } from 'lucide-vue-next'
import { useAbout } from '../../composables/useAbout'
import { useAppStore } from '../../stores/app'

const { t } = useI18n()
const appStore = useAppStore()

const {
  contactConfig, heroTitle, heroSubtitle, introductionText, servicesTitle, contactTitle, contactText,
  serviceItems, hasIntroduction, hasServices, hasContactLinks, hasContact,
} = useAbout()

/** 后台没填任何关于内容时，给一段兜底，避免整页空白 */
const bodyIsEmpty = computed(
  () => !hasIntroduction.value && !hasServices.value && !hasContact.value && !hasContactLinks.value,
)
const fallbackText = computed(() => {
  const cfg = appStore.config as any
  const desc = cfg?.brand?.site_description
  const text = typeof desc === 'object' ? (desc[appStore.locale] || desc['zh-CN'] || '') : desc
  return (typeof text === 'string' && text.trim()) || t('about.fallback')
})
</script>

<style scoped>
.about-shell {
  --about-hair: color-mix(in srgb, var(--st-text) 12%, transparent);
  max-width: 720px;
  margin-inline: auto;
}

.about-head {
  text-align: center;
}

.about-body {
  padding-top: 40px;
}
.about-body > * + * {
  margin-top: 48px;
  padding-top: 48px;
  border-top: 1px solid var(--about-hair);
}

.about-copy {
  margin: 0;
  font-size: 17px;
  line-height: 1.6;
  color: var(--st-text);
  white-space: pre-line;
  text-wrap: pretty;
}

/* ---------- 服务 ---------- */
.about-services {
  display: grid;
  grid-template-columns: 1fr;
  gap: 14px 32px;
  margin: 0;
  padding: 0;
  list-style: none;
}
@media (min-width: 735px) {
  .about-services {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 16px 40px;
  }
}
.about-service {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  font-size: 17px;
  line-height: 1.5;
  color: var(--st-text);
}
.about-service-icon {
  width: 18px;
  height: 18px;
  margin-top: 2px;
  flex: none;
  color: var(--st-text-2);
  stroke-width: 2;
}

/* ---------- 联系方式 ---------- */
.about-contact-list {
  margin-top: 24px;
  border-top: 1px solid var(--about-hair);
}
.about-contact-link {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 16px 0;
  border-bottom: 1px solid var(--about-hair);
  font-size: 17px;
  line-height: 1.4;
  color: var(--st-link);
  text-decoration: none;
}
.about-contact-link:hover span {
  text-decoration: underline;
}
.about-contact-link:focus-visible {
  outline: 2px solid var(--st-link);
  outline-offset: 2px;
}
.about-contact-icon {
  width: 20px;
  height: 20px;
  flex: none;
  color: var(--st-text-2);
}
</style>
