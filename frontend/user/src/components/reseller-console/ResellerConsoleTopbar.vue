<template>
  <header class="sticky top-0 z-40 border-b bg-background/85 backdrop-blur-md">
    <div class="mx-auto flex h-16 max-w-7xl items-center justify-between gap-3 px-4 lg:px-6">
      <!-- Left -->
      <div class="flex min-w-0 items-center gap-2">
        <Button
          type="button"
          variant="ghost"
          size="icon"
          class="lg:hidden"
          :aria-label="t('resellerConsole.nav.dashboard')"
          @click="drawerOpen = true"
        >
          <Menu class="h-5 w-5" />
        </Button>

        <RouterLink to="/" class="group relative hidden shrink-0 sm:block" :title="brandSiteName">
          <img v-if="brandLogo" :src="brandLogo" :alt="brandSiteName" class="h-7 max-w-[150px] object-contain" />
          <span v-else class="text-base font-black text-foreground">{{ brandSiteName }}</span>
        </RouterLink>

        <span class="hidden h-5 w-px shrink-0 bg-border sm:block"></span>

        <p class="min-w-0 truncate text-sm font-bold text-foreground">{{ title }}</p>
      </div>

      <!-- Right -->
      <div class="flex shrink-0 items-center gap-1 sm:gap-2">
        <Button as-child variant="ghost" size="sm" class="hidden sm:inline-flex">
          <RouterLink to="/">
            <ArrowLeft class="h-4 w-4" />
            {{ t('resellerConsole.nav.backStore') }}
          </RouterLink>
        </Button>

        <Button
          type="button"
          variant="ghost"
          size="icon"
          :aria-label="t('resellerConsole.common.toggleTheme')"
          @click="toggleTheme"
        >
          <Sun v-if="theme === 'dark'" class="h-4 w-4" />
          <Moon v-else class="h-4 w-4" />
        </Button>

        <!-- Language switcher -->
        <Popover v-model:open="langOpen">
          <PopoverTrigger as-child>
            <Button type="button" variant="ghost" size="sm" class="gap-1.5 px-2.5">
              <Languages class="h-4 w-4" />
              <span class="text-xs font-semibold uppercase tracking-wider">{{ currentLocaleLabel }}</span>
            </Button>
          </PopoverTrigger>
          <PopoverContent align="end" class="w-44 p-1.5">
            <p class="px-2 pb-1.5 pt-1 font-mono text-xs text-muted-foreground">{{ t('navbar.selectLanguage') }}</p>
            <button
              v-for="lang in languages"
              :key="lang.code"
              type="button"
              class="flex w-full items-center justify-between rounded-md px-2.5 py-2 text-left text-sm transition-colors hover:bg-accent hover:text-accent-foreground"
              :class="appStore.locale === lang.code ? 'text-primary' : 'text-foreground'"
              @click="changeLanguage(lang.code)"
            >
              {{ lang.name }}
              <Check v-if="appStore.locale === lang.code" class="h-4 w-4" />
            </button>
          </PopoverContent>
        </Popover>

        <Button
          as-child
          variant="ghost"
          size="icon"
          class="hidden sm:inline-flex"
          :aria-label="t('navbar.personalCenter')"
        >
          <RouterLink to="/me">
            <CircleUserRound class="h-5 w-5" />
          </RouterLink>
        </Button>

        <Button
          v-if="userAuthStore.isAuthenticated"
          type="button"
          variant="ghost"
          size="sm"
          class="hidden text-destructive hover:bg-destructive/10 hover:text-destructive lg:inline-flex"
          @click="userAuthStore.logout()"
        >
          <LogOut class="h-4 w-4" />
          {{ t('navbar.logout') }}
        </Button>
      </div>
    </div>
  </header>

  <!-- Mobile drawer -->
  <Teleport to="body">
    <Transition
      enter-active-class="transition duration-300 ease-out"
      enter-from-class="opacity-0"
      enter-to-class="opacity-100"
      leave-active-class="transition duration-200 ease-in"
      leave-from-class="opacity-100"
      leave-to-class="opacity-0"
    >
      <div v-if="drawerOpen" class="fixed inset-0 z-[60] bg-black/50 lg:hidden" @click="drawerOpen = false"></div>
    </Transition>

    <Transition
      enter-active-class="transition duration-300 ease-out"
      enter-from-class="-translate-x-full"
      enter-to-class="translate-x-0"
      leave-active-class="transition duration-200 ease-in"
      leave-from-class="translate-x-0"
      leave-to-class="-translate-x-full"
    >
      <aside
        v-if="drawerOpen"
        class="fixed bottom-0 left-0 top-0 z-[70] w-72 max-w-[82vw] overflow-y-auto border-r bg-card lg:hidden"
      >
        <div class="space-y-1 p-4">
          <div class="mb-3 flex items-center justify-between">
            <span class="text-sm font-black text-foreground">{{ title }}</span>
            <Button type="button" variant="ghost" size="icon-sm" @click="drawerOpen = false">
              <X class="h-5 w-5" />
            </Button>
          </div>

          <div v-for="group in navGroups" :key="group.key" class="space-y-1">
            <p class="px-3 pb-1 pt-2 text-[11px] font-semibold uppercase tracking-[0.14em] text-muted-foreground">
              {{ group.title }}
            </p>
            <RouterLink
              v-for="item in group.items"
              :key="item.to"
              :to="item.to"
              class="flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-semibold transition-colors"
              :class="isActive(item.to) ? 'bg-accent text-foreground' : 'text-muted-foreground hover:bg-accent hover:text-foreground'"
              @click="drawerOpen = false"
            >
              <component :is="item.icon" class="h-5 w-5 shrink-0" />
              <span class="truncate">{{ item.label }}</span>
            </RouterLink>
          </div>

          <div class="mt-3 space-y-1 border-t pt-3">
            <RouterLink
              to="/"
              class="flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-semibold text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
              @click="drawerOpen = false"
            >
              <ArrowLeft class="h-5 w-5 shrink-0" />
              {{ t('resellerConsole.nav.backStore') }}
            </RouterLink>
            <RouterLink
              to="/me"
              class="flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-semibold text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
              @click="drawerOpen = false"
            >
              <CircleUserRound class="h-5 w-5 shrink-0" />
              {{ t('navbar.personalCenter') }}
            </RouterLink>
            <button
              v-if="userAuthStore.isAuthenticated"
              type="button"
              class="flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-left text-sm font-semibold text-destructive transition-colors hover:bg-destructive/10"
              @click="userAuthStore.logout(); drawerOpen = false"
            >
              <LogOut class="h-5 w-5 shrink-0" />
              {{ t('navbar.logout') }}
            </button>
          </div>
        </div>
      </aside>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, ref, watch, type Component } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import {
  ArrowLeft,
  Check,
  CircleUserRound,
  Languages,
  LogOut,
  Menu,
  Moon,
  Sun,
  X,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { useAppStore } from '../../stores/app'
import { useUserAuthStore } from '../../stores/userAuth'
import { useTheme } from '../../utils/theme'
import { getImageUrl } from '../../utils/image'

defineProps<{
  title: string
  navGroups: Array<{ key: string; title: string; items: Array<{ to: string; label: string; icon: Component }> }>
}>()

const { t } = useI18n()
const route = useRoute()
const appStore = useAppStore()
const userAuthStore = useUserAuthStore()
const { theme, toggleTheme } = useTheme()

const drawerOpen = ref(false)
const langOpen = ref(false)

const languages = [
  { code: 'zh-CN', name: '简体中文' },
  { code: 'zh-TW', name: '繁體中文' },
  { code: 'en-US', name: 'English' },
]

const currentLocaleLabel = computed(() => {
  if (appStore.locale === 'en-US') return 'EN'
  if (appStore.locale === 'zh-TW') return '繁'
  return '简'
})

const brandSiteName = computed(() => {
  const text = String(appStore.config?.brand?.site_name || '').trim()
  return text !== '' ? text : 'Dovelora'
})

const brandLogo = computed(() => {
  const raw = String(appStore.config?.brand?.site_logo || '').trim()
  return raw ? getImageUrl(raw) : ''
})

const isActive = (path: string) => {
  if (path === '/reseller') return route.path === path
  return route.path === path || route.path.startsWith(`${path}/`)
}

const changeLanguage = (code: string) => {
  appStore.setLocale(code)
  langOpen.value = false
}

watch(
  () => route.path,
  () => {
    drawerOpen.value = false
  },
)
</script>
