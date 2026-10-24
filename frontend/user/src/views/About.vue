<template>
  <div
    class="about-page min-h-screen bg-background text-foreground pt-20 pb-16">
    <div class="container mx-auto px-4 max-w-4xl">
      <div class="text-center mb-16 mt-12">
        <h1 class="text-4xl md:text-6xl font-black mb-6 tracking-tight text-foreground">{{ heroTitle }}</h1>
        <p
          class="text-muted-foreground max-w-2xl mx-auto text-lg leading-relaxed border-b pb-8">
          {{ heroSubtitle }}
        </p>
      </div>

      <Card
        class="backdrop-blur-xl rounded-3xl p-8 md:p-12 shadow-2xl relative overflow-hidden">

        <div v-if="hasIntroduction" class="mb-12">
          <p class="text-muted-foreground text-lg leading-relaxed whitespace-pre-line">
            {{ introductionText }}
          </p>
        </div>

        <div v-if="hasServices" class="mb-12">
          <h2 v-if="servicesTitle" class="text-2xl font-bold text-foreground mb-6 flex items-center gap-3">
            <span class="w-1.5 h-8 bg-primary rounded-full"></span>
            {{ servicesTitle }}
          </h2>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div
              v-for="(service, index) in serviceItems"
              :key="`about-service-${index}`"
              class="flex items-start gap-3 p-4 bg-secondary rounded-xl border transition-colors">
              <Check class="w-6 h-6 text-primary flex-shrink-0 mt-0.5" />
              <span class="text-muted-foreground">{{ service }}</span>
            </div>
          </div>
        </div>

        <div v-if="hasContact">
          <h2 v-if="contactTitle" class="text-2xl font-bold text-foreground mb-6 flex items-center gap-3">
            <span class="w-1.5 h-8 bg-primary rounded-full"></span>
            {{ contactTitle }}
          </h2>
          <p v-if="contactText" class="text-muted-foreground mb-8 whitespace-pre-line">
            {{ contactText }}
          </p>
          <div v-if="hasContactLinks" class="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <a v-if="contactConfig?.telegram" :href="contactConfig.telegram" target="_blank" rel="noopener noreferrer"
              class="group flex items-center justify-center space-x-3 bg-secondary border text-muted-foreground px-6 py-4 rounded-xl transition-colors hover:text-foreground">
              <svg class="w-6 h-6 text-sky-500" fill="currentColor" viewBox="0 0 24 24">
                <path
                  d="M12 0C5.373 0 0 5.373 0 12s5.373 12 12 12 12-5.373 12-12S18.627 0 12 0zm5.894 8.221l-1.97 9.28c-.145.658-.537.818-1.084.508l-3-2.21-1.446 1.394c-.14.18-.357.295-.6.295-.002 0-.003 0-.005 0l.213-3.054 5.56-5.022c.24-.213-.054-.334-.373-.121l-6.869 4.326-2.96-.924c-.64-.203-.658-.64.135-.954l11.566-4.458c.538-.196 1.006.128.832.941z" />
              </svg>
              <span class="font-bold">Telegram</span>
            </a>
            <a v-if="contactConfig?.whatsapp" :href="contactConfig.whatsapp" target="_blank" rel="noopener noreferrer"
              class="group flex items-center justify-center space-x-3 bg-secondary border text-muted-foreground px-6 py-4 rounded-xl transition-colors hover:text-foreground">
              <svg class="w-6 h-6 text-emerald-500" fill="currentColor" viewBox="0 0 24 24">
                <path
                  d="M17.472 14.382c-.297-.149-1.758-.867-2.03-.967-.273-.099-.471-.148-.67.15-.197.297-.767.966-.94 1.164-.173.199-.347.223-.644.075-.297-.15-1.255-.463-2.39-1.475-.883-.788-1.48-1.761-1.653-2.059-.173-.297-.018-.458.13-.606.134-.133.298-.347.446-.52.149-.174.198-.298.298-.497.099-.198.05-.371-.025-.52-.075-.149-.669-1.612-.916-2.207-.242-.579-.487-.5-.669-.51-.173-.008-.371-.01-.57-.01-.198 0-.52.074-.792.372-.272.297-1.04 1.016-1.04 2.479 0 1.462 1.065 2.875 1.213 3.074.149.198 2.096 3.2 5.077 4.487.709.306 1.262.489 1.694.625.712.227 1.36.195 1.871.118.571-.085 1.758-.719 2.006-1.413.248-.694.248-1.289.173-1.413-.074-.124-.272-.198-.57-.347m-5.421 7.403h-.004a9.87 9.87 0 01-5.031-1.378l-.361-.214-3.741.982.998-3.648-.235-.374a9.86 9.86 0 01-1.51-5.26c.001-5.45 4.436-9.884 9.888-9.884 2.64 0 5.122 1.03 6.988 2.898a9.825 9.825 0 012.893 6.994c-.003 5.45-4.437 9.884-9.885 9.884m8.413-18.297A11.815 11.815 0 0012.05 0C5.495 0 .16 5.335.157 11.892c0 2.096.547 4.142 1.588 5.945L.057 24l6.305-1.654a11.882 11.882 0 005.683 1.448h.005c6.554 0 11.89-5.335 11.893-11.893a11.821 11.821 0 00-3.48-8.413Z" />
              </svg>
              <span class="font-bold">WhatsApp</span>
            </a>
            <a v-if="contactConfig?.email" :href="`mailto:${contactConfig.email}`"
              class="group flex items-center justify-center space-x-3 bg-secondary border text-muted-foreground px-6 py-4 rounded-xl transition-colors hover:text-foreground">
              <Mail class="w-6 h-6 text-amber-500" />
              <span class="font-bold">{{ t('footer.email') }}</span>
            </a>
            <a v-if="contactConfig?.support_url" :href="contactConfig.support_url"
              :target="contactConfig.support_url.startsWith('https://') ? '_blank' : undefined" rel="noopener noreferrer"
              class="group flex items-center justify-center space-x-3 bg-secondary border text-muted-foreground px-6 py-4 rounded-xl transition-colors hover:text-foreground">
              <Headphones class="w-6 h-6 text-violet-500" />
              <span class="font-bold">{{ t('footer.supportUrl') }}</span>
            </a>
          </div>
        </div>
      </Card>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Check, Mail, Headphones } from 'lucide-vue-next'
import { Card } from '@/components/ui/card'
import { useAbout } from '../composables/useAbout'

const { t } = useI18n()

const {
  contactConfig, heroTitle, heroSubtitle, introductionText, servicesTitle, contactTitle, contactText,
  serviceItems, hasIntroduction, hasServices, hasContactLinks, hasContact,
} = useAbout()
</script>
