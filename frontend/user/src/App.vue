<template>
  <div id="app" class="min-h-screen bg-background text-foreground flex flex-col">
    <!-- 首次访问还不知道站点用哪套模板时先不渲染外壳，等配置返回（见 shellReady） -->
    <template v-if="shellReady">
      <!-- vault 模板：自带顶栏/页脚的外壳包裹页面（控制台仍走下方分支） -->
      <component :is="TemplateLayout" v-if="hasTemplateLayout && !isResellerConsole">
        <ErrorBoundary>
          <RouterView v-slot="{ Component }">
            <div class="page-in">
              <component :is="Component" />
            </div>
          </RouterView>
        </ErrorBoundary>
      </component>

      <!-- classic 模板 / 分销控制台（保持原有结构不变） -->
      <template v-else>
        <Navbar v-if="!isResellerConsole" />
        <main class="flex-1" :class="isResellerConsole ? '' : 'pb-14 lg:pb-0'">
          <ErrorBoundary>
            <RouterView v-slot="{ Component }">
              <div class="page-in">
                <component :is="Component" />
              </div>
            </RouterView>
          </ErrorBoundary>
        </main>
        <Footer v-if="!isResellerConsole" />
        <BackToTop v-if="!isResellerConsole" />
        <MobileBottomNav v-if="!isResellerConsole" />
      </template>
    </template>

    <Loading :loading="appStore.loading || !shellReady" />
    <Toast />
    <ConfirmDialog />
  </div>
</template>

<script setup lang="ts">
import { computed, defineAsyncComponent, nextTick, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAppStore } from './stores/app'
import { getActiveTemplate, hasTemplateHint, rememberTemplate } from './templates/registry'
import Navbar from './components/Navbar.vue'
import Footer from './components/Footer.vue'
import Loading from './components/Loading.vue'
import Toast from './components/Toast.vue'
import ConfirmDialog from './components/ConfirmDialog.vue'
import ErrorBoundary from './components/ErrorBoundary.vue'
import BackToTop from './components/BackToTop.vue'
import MobileBottomNav from './components/MobileBottomNav.vue'

// 模板外壳按需加载，classic 用户不会拉取其 chunk/样式
const VaultLayout = defineAsyncComponent(() => import('./templates/vault/layout/VaultLayout.vue'))
const AppleLayout = defineAsyncComponent(() => import('./templates/apple/layout/VaultLayout.vue'))

// config 由 router.beforeEach 统一加载，无需在此重复调用
const appStore = useAppStore()
const route = useRoute()
const isResellerConsole = computed(() => route.meta.resellerConsole === true)
// getActiveTemplate 读取 appStore.config（响应式），config 加载后会重新计算
const activeTemplate = computed(() => getActiveTemplate())
const hasTemplateLayout = computed(() => activeTemplate.value !== 'classic')
const TemplateLayout = computed(() => (activeTemplate.value === 'apple' ? AppleLayout : VaultLayout))

// 模板来自异步的站点配置。已有线索（预览覆盖 / 上次访问记住的模板）时立即渲染外壳；
// 首次访问则等配置返回再渲染，避免先闪一下 classic 外壳。配置加载失败时由首个路由就绪兜底
// （守卫里的加载无论成败都会结束），不会一直停在空白。
const shellReady = ref(hasTemplateHint())
if (!shellReady.value) {
  watch(() => appStore.config, (config) => { if (config) shellReady.value = true }, { immediate: true })
  void useRouter().isReady().catch(() => {}).finally(() => { shellReady.value = true })
}
watch(() => appStore.config?.storefront_template, (value) => rememberTemplate(value), { immediate: true })

// 每次换页让内容轻微淡入。用 Web Animations 而不是 CSS 过渡/动画：
// CSS 方案在频繁重渲染时会被反复重置到 opacity:0，看起来就是整页空白。
const playPageIn = async () => {
  await nextTick()
  const el = document.querySelector<HTMLElement>('.page-in')
  if (!el || typeof el.animate !== 'function') return
  if (window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) return
  el.animate(
    [
      { opacity: 0, transform: 'translateY(6px)' },
      { opacity: 1, transform: 'none' },
    ],
    { duration: 200, easing: 'cubic-bezier(0.4, 0, 0.6, 1)', fill: 'none' },
  )
}
watch(() => route.fullPath, () => { void playPageIn() }, { immediate: true })
</script>

<style>
/* 页面进入动效由 App 里的 Web Animations 驱动，见 watch(route.path)。
   不用 CSS 动画：组件频繁重渲染会反复重置到 opacity:0，看上去就是空白页。 */
.page-in {
  will-change: opacity;
}
</style>
