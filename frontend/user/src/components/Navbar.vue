<template>
  <nav
    class="fixed top-0 left-0 right-0 z-50 bg-card/80 border-b backdrop-blur-md transition-all"
    :class="scrolled ? 'py-2 shadow-lg' : 'py-4'"
    :style="{ transitionDuration: 'var(--ui-duration-normal)' }">
    <div class="container mx-auto px-4 flex items-center justify-between gap-4">
      <!-- Logo -->
      <router-link to="/" class="theme-wordmark group relative gap-3" :title="brandSiteName">
        <svg class="h-8 w-8 shrink-0" viewBox="0 0 32 32" role="img" aria-label="雪糕数卡" shape-rendering="geometricPrecision">
          <rect x="6" y="3" width="16" height="23" rx="5" fill="#2563eb" />
          <rect x="11" y="26" width="6" height="4" rx="2" fill="#c58b45" />
          <path d="M14 3h4v12a2 2 0 0 1-4 0V3Z" fill="#bfdbfe" />
          <rect x="17" y="11" width="12" height="15" rx="3" fill="#fff" stroke="#1d4ed8" stroke-width="2" />
          <path d="M21 16h4M21 20h4" stroke="#2563eb" stroke-width="2" stroke-linecap="round" />
        </svg>
        <span class="theme-wordmark-text">{{ brandSiteName }}</span>
      </router-link>

      <!-- Desktop Menu -->
      <div class="hidden lg:flex items-center space-x-1 min-w-0 overflow-x-auto scrollbar-hide">
        <template v-for="item in menuItems" :key="item.key">
          <Button v-if="item.type === 'route'" as-child variant="ghost" size="sm"
            class="gap-1.5 text-muted-foreground whitespace-nowrap shrink-0">
            <router-link :to="item.path" active-class="!text-primary !bg-primary/10">
              <component :is="item.icon" class="w-4 h-4 shrink-0 opacity-70" />
              <span>{{ item.label }}</span>
            </router-link>
          </Button>
          <Button v-else as-child variant="ghost" size="sm"
            class="gap-1.5 text-muted-foreground whitespace-nowrap shrink-0">
            <a :href="item.path" :target="item.target" rel="noopener noreferrer">
              <component :is="item.icon" class="w-4 h-4 shrink-0 opacity-70" />
              <span>{{ item.label }}</span>
            </a>
          </Button>
        </template>
      </div>

      <!-- Right Side Actions -->
      <div class="flex items-center shrink-0 space-x-2 lg:space-x-4">

        <Button v-if="!userAuthStore.isAuthenticated" as-child variant="ghost" size="sm"
          class="hidden lg:inline-flex gap-1.5 text-muted-foreground whitespace-nowrap">
          <router-link to="/guest/orders">
            <ClipboardList class="w-4 h-4 shrink-0 opacity-70" />
            {{ t('navbar.guestOrders') }}
          </router-link>
        </Button>
        <Button v-if="!userAuthStore.isAuthenticated" as-child variant="ghost" size="sm"
          class="hidden lg:inline-flex gap-1.5 text-muted-foreground whitespace-nowrap">
          <router-link to="/auth/login">
            <LogIn class="w-4 h-4 shrink-0 opacity-70" />
            {{ t('navbar.login') }}
          </router-link>
        </Button>
        <Button v-if="userAuthStore.isAuthenticated" as-child variant="ghost" size="sm"
          class="hidden lg:inline-flex gap-1.5 text-muted-foreground whitespace-nowrap">
          <router-link to="/me">
            <User class="w-4 h-4 shrink-0 opacity-70" />
            {{ t('navbar.personalCenter') }}
          </router-link>
        </Button>
        <Button v-if="userAuthStore.isAuthenticated" variant="ghost" size="sm"
          class="hidden lg:inline-flex gap-1.5 whitespace-nowrap text-destructive hover:text-destructive hover:bg-destructive/10"
          @click="userAuthStore.logout()">
          <LogOut class="w-4 h-4 shrink-0 opacity-70" />
          {{ t('navbar.logout') }}
        </Button>
        <!-- Theme Switcher -->
        <Button variant="ghost" size="icon" class="text-muted-foreground" @click="toggleTheme">
          <Sun v-if="theme === 'dark'" class="w-4 h-4" />
          <Moon v-else class="w-4 h-4" />
        </Button>

        <!-- Language Switcher -->
        <Popover v-model:open="langOpen">
          <PopoverTrigger as-child>
            <Button variant="ghost" size="sm" class="inline-flex gap-1.5 px-2 text-muted-foreground">
              <Languages class="w-4 h-4" />
              <span class="text-xs font-medium">{{ currentLocale }}</span>
            </Button>
          </PopoverTrigger>
          <PopoverContent align="end" class="w-40 p-2">
            <button v-for="lang in languages" :key="lang.code" @click="changeLanguage(lang.code)"
              class="flex min-h-10 w-full items-center justify-between rounded-md px-3 py-2 text-left text-sm transition-colors hover:bg-accent hover:text-accent-foreground"
              :class="{ 'text-primary font-semibold': appStore.locale === lang.code }">
              {{ lang.name }}
              <span v-if="appStore.locale === lang.code" class="w-1.5 h-1.5 rounded-full bg-primary"></span>
            </button>
          </PopoverContent>
        </Popover>
      </div>
    </div>

  </nav>

</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '../stores/app'

import { useUserAuthStore } from '../stores/userAuth'
import { useTheme } from '../utils/theme'
import { useNavConfig } from '../composables/useNavConfig'
import {
  Sun, Moon, ClipboardList, LogIn, User, LogOut, Languages,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'

const { t } = useI18n()
const appStore = useAppStore()

const userAuthStore = useUserAuthStore()
const { theme, toggleTheme } = useTheme()
const { primaryNavItems } = useNavConfig()

const langOpen = ref(false)
const scrolled = ref(false)


const menuItems = computed(() => primaryNavItems.value.filter((item) => item.path !== '/'))

const languages = [
  { code: 'zh-CN', name: '简体中文' },
  { code: 'zh-TW', name: '繁體中文' },
  { code: 'en-US', name: 'English' },
]

const currentLocale = computed(() => {
  const lang = languages.find(l => l.code === appStore.locale)
  if (!lang) return 'CN'
  return lang.code === 'en-US' ? 'EN' : (lang.code === 'zh-CN' ? '简' : '繁')
})


const brandSiteName = computed(() => {
  const text = String(appStore.config?.brand?.site_name || '').trim()
  return text !== '' ? text : '雪糕数卡'
})

const changeLanguage = (langCode: string) => {
  appStore.setLocale(langCode)
  langOpen.value = false
}

const handleScroll = () => {
  scrolled.value = window.scrollY > 20
}


onMounted(() => {
  window.addEventListener('scroll', handleScroll, { passive: true })
})

onUnmounted(() => {
  window.removeEventListener('scroll', handleScroll)
})
</script>

<style scoped>
.scrollbar-hide {
  -ms-overflow-style: none;
  scrollbar-width: none;
}
.scrollbar-hide::-webkit-scrollbar {
  display: none;
}
</style>
