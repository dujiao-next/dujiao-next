<template>
  <div class="min-h-screen pb-16 pt-[88px] text-foreground">
    <div class="mx-auto w-full max-w-4xl px-4 sm:px-6">
      <div class="mb-7 border-b pb-5">
        <h1 class="text-[28px] font-bold tracking-[-0.03em] md:text-[38px]">{{ heroTitle }}</h1>
        <p class="mt-2 max-w-2xl text-base leading-relaxed text-muted-foreground">
          {{ heroSubtitle }}
        </p>
      </div>

      <div v-if="hasIntroduction || hasServices || hasContact" class="atelier-card space-y-9 rounded-[16px] bg-card p-6 md:p-10">
        <section v-if="hasIntroduction">
          <p class="whitespace-pre-line text-base leading-relaxed text-muted-foreground md:text-[17px]">
            {{ introductionText }}
          </p>
        </section>

        <section v-if="hasServices" class="border-t pt-8">
          <h2 v-if="servicesTitle" class="mb-4 text-base font-bold tracking-[-0.015em]">
            {{ servicesTitle }}
          </h2>
          <div class="grid grid-cols-1 gap-2.5 md:grid-cols-2">
            <div
              v-for="(service, index) in serviceItems"
              :key="`at-about-service-${index}`"
              class="flex items-start gap-3 rounded-[14px] border bg-background p-4"
            >
              <span class="mt-0.5 grid h-5 w-5 shrink-0 place-items-center rounded-full bg-success-soft text-success">
                <Check class="h-3 w-3" :stroke-width="2.8" />
              </span>
              <span class="text-sm leading-relaxed text-muted-foreground">{{ service }}</span>
            </div>
          </div>
        </section>

        <section v-if="hasContact" class="border-t pt-8">
          <h2 v-if="contactTitle" class="mb-3 text-base font-bold tracking-[-0.015em]">
            {{ contactTitle }}
          </h2>
          <p v-if="contactText" class="mb-6 whitespace-pre-line text-sm leading-relaxed text-muted-foreground">
            {{ contactText }}
          </p>
          <div v-if="hasContactLinks" class="flex flex-wrap gap-2.5">
            <a
              v-if="contactConfig?.telegram"
              :href="contactConfig.telegram"
              target="_blank"
              rel="noopener noreferrer"
              class="inline-flex h-11 items-center gap-2.5 rounded-full border bg-background px-5 text-sm font-semibold text-foreground transition-colors hover:border-primary hover:bg-primary hover:text-primary-foreground"
            >
              <Send class="h-4 w-4" />
              <span>Telegram</span>
            </a>
            <a
              v-if="contactConfig?.whatsapp"
              :href="contactConfig.whatsapp"
              target="_blank"
              rel="noopener noreferrer"
              class="inline-flex h-11 items-center gap-2.5 rounded-full border bg-background px-5 text-sm font-semibold text-foreground transition-colors hover:border-primary hover:bg-primary hover:text-primary-foreground"
            >
              <MessageCircle class="h-4 w-4" />
              <span>WhatsApp</span>
            </a>
            <a
              v-if="contactConfig?.email"
              :href="`mailto:${contactConfig.email}`"
              class="inline-flex h-11 items-center gap-2.5 rounded-full border bg-background px-5 text-sm font-semibold text-foreground transition-colors hover:border-primary hover:bg-primary hover:text-primary-foreground"
            >
              <Mail class="h-4 w-4" />
              <span>{{ t('footer.email') }}</span>
            </a>
            <a
              v-if="contactConfig?.support_url"
              :href="contactConfig.support_url"
              :target="contactConfig.support_url.startsWith('https://') ? '_blank' : undefined"
              rel="noopener noreferrer"
              class="inline-flex h-11 items-center gap-2.5 rounded-full border bg-background px-5 text-sm font-semibold text-foreground transition-colors hover:border-primary hover:bg-primary hover:text-primary-foreground"
            >
              <Headphones class="h-4 w-4" />
              <span>{{ t('footer.supportUrl') }}</span>
            </a>
          </div>
        </section>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Check, Headphones, Mail, MessageCircle, Send } from 'lucide-vue-next'
import { useAbout } from '../../composables/useAbout'

const { t } = useI18n()

const {
  contactConfig, heroTitle, heroSubtitle, introductionText, servicesTitle, contactTitle, contactText,
  serviceItems, hasIntroduction, hasServices, hasContactLinks, hasContact,
} = useAbout()
</script>
