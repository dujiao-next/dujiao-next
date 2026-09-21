<template>
  <nav
    class="fixed top-0 left-0 right-0 z-50 bg-card/80 border-b backdrop-blur-md transition-all"
    :class="scrolled ? 'py-2 shadow-lg' : 'py-4'"
    :style="{ transitionDuration: 'var(--ui-duration-normal)' }">
    <div class="container mx-auto px-4 flex items-center justify-between gap-4">
      <!-- Logo -->
      <router-link to="/" class="theme-wordmark group relative gap-3" :title="brandSiteName">
        <svg class="h-8 w-8 shrink-0" viewBox="0 0 64 64" role="img" aria-label="雪糕数卡">
          <defs>
            <linearGradient id="navbar-ice-logo" x1="0" y1="0" x2="1" y2="1">
              <stop stop-color="#60a5fa" />
              <stop offset="1" stop-color="#2563eb" />
            </linearGradient>
          </defs>
          <rect x="6" y="8" width="38" height="44" rx="12" fill="url(#navbar-ice-logo)" />
          <path d="M17 8h16v26a8 8 0 0 1-16 0z" fill="#dbeafe" opacity=".92" />
          <rect x="20" y="51" width="10" height="9" rx="4" fill="#d6a768" />
          <rect x="31" y="22" width="27" height="30" rx="7" fill="#fff" stroke="#1d4ed8" stroke-width="3" />
          <circle cx="39" cy="31" r="3" fill="#2563eb" />
          <path d="M46 29h7M37 40h16M37 46h11" stroke="#60a5fa" stroke-width="3" stroke-linecap="round" />
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
