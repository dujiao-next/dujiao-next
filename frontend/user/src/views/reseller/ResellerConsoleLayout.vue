<template>
  <div class="reseller-console flex min-h-screen flex-col bg-background">
    <ResellerConsoleTopbar :title="currentTitle" :nav-groups="navGroups" />

    <div class="mx-auto flex w-full max-w-7xl flex-1 gap-6 px-0 lg:px-6 lg:py-6">
      <aside class="hidden w-60 shrink-0 lg:block">
        <div class="theme-float sticky top-20 rounded-[28px] bg-card p-2">
          <nav class="space-y-2">
            <div v-for="group in navGroups" :key="group.key">
              <p class="px-4 pb-1 pt-2 text-xs font-medium text-muted-foreground">
                {{ group.title }}
              </p>
              <div class="space-y-1">
                <RouterLink
                  v-for="item in group.items"
                  :key="item.to"
                  :to="item.to"
                  class="flex h-10 items-center gap-2.5 rounded-[14px] px-4 text-sm font-medium transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-foreground"
                  :class="isActive(item.to)
                    ? 'bg-foreground font-semibold text-background'
                    : 'text-muted-foreground hover:bg-secondary hover:text-foreground'"
                  :aria-current="isActive(item.to) ? 'page' : undefined"
                >
                  <component :is="item.icon" class="h-5 w-5 shrink-0" />
                  <span class="truncate">{{ item.label }}</span>
                </RouterLink>
              </div>
            </div>
          </nav>
        </div>
      </aside>

      <main class="min-w-0 flex-1 px-4 py-6 lg:px-0 lg:py-0">
        <ResellerPageState v-if="!profileReady || profileLoading" loading :title="t('resellerConsole.common.loading')" />
        <RouterView v-else-if="canRenderCurrentModule" />
        <Card v-else class="p-6 sm:p-8">
          <div class="mx-auto max-w-xl text-center">
            <span class="mx-auto flex h-12 w-12 items-center justify-center rounded-xl bg-primary/10 text-primary">
              <ClipboardCheck class="h-6 w-6" />
            </span>
            <h2 class="mt-4 text-lg font-bold text-foreground">{{ t('resellerConsole.dashboard.inactiveTitle') }}</h2>
            <p class="mt-2 text-sm text-muted-foreground">{{ t('resellerConsole.dashboard.inactiveDescription') }}</p>
            <Button as-child class="mt-5">
              <RouterLink to="/reseller/apply">{{ t('resellerConsole.nav.apply') }}</RouterLink>
            </Button>
          </div>
        </Card>
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, type Component } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import {
  Banknote,
  ClipboardCheck,
  FileText,
  Globe,
  LayoutGrid,
  Settings,
  ShoppingBag,
  Tag,
  Upload,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import ResellerConsoleTopbar from '../../components/reseller-console/ResellerConsoleTopbar.vue'
import ResellerPageState from '../../components/reseller-console/ResellerPageState.vue'
import { useResellerProfile } from '../../composables/reseller/useResellerProfile'
import { canRenderResellerConsoleModule } from '../../utils/resellerConsole'

const { t } = useI18n()
const route = useRoute()
const { loading: profileLoading, state: profileState, load: loadProfile } = useResellerProfile()
const profileReady = ref(false)

type NavDef = { to: string; label: string; icon: Component }

const groupDefs: Array<{ key: string; titleKey: string; items: NavDef[] }> = [
  {
    key: 'operations',
    titleKey: 'resellerConsole.navGroups.operations',
    items: [
      { to: '/reseller', label: 'resellerConsole.nav.dashboard', icon: LayoutGrid },
      { to: '/reseller/orders', label: 'resellerConsole.nav.orders', icon: ShoppingBag },
      { to: '/reseller/finance', label: 'resellerConsole.nav.finance', icon: Banknote },
      { to: '/reseller/ledger', label: 'resellerConsole.nav.ledger', icon: FileText },
      { to: '/reseller/withdraws', label: 'resellerConsole.nav.withdraws', icon: Upload },
    ],
  },
  {
    key: 'config',
    titleKey: 'resellerConsole.navGroups.config',
    items: [
      { to: '/reseller/domains', label: 'resellerConsole.nav.domains', icon: Globe },
      { to: '/reseller/site', label: 'resellerConsole.nav.site', icon: Settings },
      { to: '/reseller/products', label: 'resellerConsole.nav.products', icon: Tag },
    ],
  },
  {
    key: 'account',
    titleKey: 'resellerConsole.navGroups.account',
    items: [{ to: '/reseller/apply', label: 'resellerConsole.nav.apply', icon: ClipboardCheck }],
  },
]

const navGroups = computed(() =>
  groupDefs.map((group) => ({
    key: group.key,
    title: t(group.titleKey),
    items: group.items.map((item) => ({ ...item, label: t(item.label) })),
  })),
)

const isActive = (path: string) => {
  if (path === '/reseller') return route.path === path
  return route.path === path || route.path.startsWith(`${path}/`)
}

const currentTitle = computed(() => {
  for (const group of navGroups.value) {
    const hit = group.items.find((item) => isActive(item.to))
    if (hit) return hit.label
  }
  return t('resellerConsole.title')
})

const canRenderCurrentModule = computed(() => canRenderResellerConsoleModule(route.path, profileState.value))

const initializeProfile = async () => {
  await loadProfile()
  profileReady.value = true
}

onMounted(initializeProfile)
</script>

<style scoped>
/* 控制台各页沿用自己的 Card / 表格 / 筛选条，这里从外壳统一成默认模板的浮起卡片与胶囊按钮；
   主色改墨色的变量覆盖在 style.css（与个人中心面板、弹窗共用一份）。
   utilities 层用 !important 把 rounded-xl 钉在 --ui-radius-md 上，所以圆角只能改变量 */
.reseller-console :deep(.rounded-xl.border.bg-card:not(.border-dashed)) {
  --ui-radius-md: 22px;
  border-width: 0;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.04), 0 10px 28px -14px rgba(0, 0, 0, 0.14);
}
.reseller-console :deep(.rounded-xl.border.bg-card > *) {
  --ui-radius-md: 0.75rem;
}
:global(.dark) .reseller-console :deep(.rounded-xl.border.bg-card:not(.border-dashed)) {
  box-shadow: 0 0 0 1px rgba(255, 255, 255, 0.06);
}
/* 卡片里再套的卡片不再浮起，退成灰底块 */
.reseller-console :deep(.rounded-xl.border.bg-card .rounded-xl.border.bg-card:not(.border-dashed)) {
  --ui-radius-md: 1rem;
  background-color: color-mix(in srgb, var(--ui-bg-soft) 60%, transparent);
  box-shadow: none;
}
.reseller-console :deep([data-slot='button']) {
  --ui-radius-sm: 9999px;
  --ui-radius-md: 9999px;
  border-radius: 9999px;
}
</style>
