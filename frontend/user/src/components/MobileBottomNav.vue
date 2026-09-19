<template>
  <nav class="lg:hidden fixed bottom-0 left-0 right-0 z-40 bg-card/90 backdrop-blur-xl border-t theme-safe-bottom">
    <div class="flex items-stretch h-14">
      <router-link
        v-for="item in navItems"
        :key="item.path"
        :to="item.path"
        class="flex-1 flex flex-col items-center justify-center gap-0.5 text-xs transition-colors min-h-[44px]"
        :class="isActive(item.path) ? 'text-primary font-semibold' : 'text-muted-foreground'"
      >
        <!-- Home -->
        <Home v-if="item.icon === 'home'" class="w-5 h-5" />
        <ClipboardList v-else-if="item.icon === 'orders'" class="w-5 h-5" />
        <!-- Me -->
        <User v-else-if="item.icon === 'me'" class="w-5 h-5" />
        <span>{{ t(item.label) }}</span>
      </router-link>
    </div>
  </nav>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ClipboardList, Home, User } from 'lucide-vue-next'
import { useUserAuthStore } from '../stores/userAuth'


const route = useRoute()
const { t } = useI18n()

const userAuthStore = useUserAuthStore()
const navItems = computed(() => {
  const items = [
    { path: '/', icon: 'home', label: 'bottomNav.home' },
    { path: '/guest/orders', icon: 'orders', label: 'bottomNav.orders' },
    { path: userAuthStore.isAuthenticated ? '/me' : '/auth/login', icon: 'me', label: 'bottomNav.me' },
  ]
  return items
})

const isActive = (path: string) => {
  if (path === '/') return route.path === '/'
  return route.path.startsWith(path)
}
</script>
