<template>
  <div id="app" class="min-h-screen bg-background text-foreground flex flex-col">
    <!-- 首次访问还不知道站点用哪套模板时先不渲染外壳，等配置返回（见 shellReady） -->
    <template v-if="shellReady">
      <!-- vault 模板：自带顶栏/页脚的外壳包裹页面（控制台仍走下方分支） -->
      <VaultLayout v-if="isVault && !isResellerConsole">
        <ErrorBoundary>
          <RouterView v-slot="{ Component }">
            <Transition name="page-fade" mode="out-in">
              <component :is="Component" />
            </Transition>
          </RouterView>
        </ErrorBoundary>
      </VaultLayout>

      <!-- classic 模板 / 分销控制台（保持原有结构不变） -->
      <template v-else>
        <Navbar v-if="!isResellerConsole" />
        <main class="flex-1" :class="isResellerConsole ? '' : 'pb-14 lg:pb-0'">
          <ErrorBoundary>
            <RouterView v-slot="{ Component }">
              <Transition name="page-fade" mode="out-in">
                <component :is="Component" />
              </Transition>
            </RouterView>
          </ErrorBoundary>
        </main>
        <Footer v-if="!isResellerConsole" />
        <BackToTop v-if="!isResellerConsole" />
        <MobileBottomNav v-if="!isResellerConsole" />
      </template>
    </template>

    <Loading :loading="appStore.loading" />
    <Toast />
    <ConfirmDialog />
  </div>
</template>

<script setup lang="ts">
import { computed, defineAsyncComponent, ref, watch } from 'vue'
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

// vault 外壳按需加载，classic 用户不会拉取其 chunk/样式
const VaultLayout = defineAsyncComponent(() => import('./templates/vault/layout/VaultLayout.vue'))

// config 由 router.beforeEach 统一加载，无需在此重复调用
const appStore = useAppStore()
const route = useRoute()
const isResellerConsole = computed(() => route.meta.resellerConsole === true)
// getActiveTemplate 读取 appStore.config（响应式），config 加载后会重新计算
const isVault = computed(() => getActiveTemplate() === 'vault')

// 模板来自异步的站点配置。已有线索（预览覆盖 / 上次访问记住的模板）时立即渲染外壳；
// 首次访问则等配置返回再渲染，避免先闪一下 classic 外壳。配置加载失败时由首个路由就绪兜底
// （守卫里的加载无论成败都会结束），不会一直停在空白。
const shellReady = ref(hasTemplateHint())
if (!shellReady.value) {
  watch(() => appStore.config, (config) => { if (config) shellReady.value = true }, { immediate: true })
  void useRouter().isReady().catch(() => {}).finally(() => { shellReady.value = true })
}
watch(() => appStore.config?.storefront_template, (value) => rememberTemplate(value), { immediate: true })
</script>

<style>
.page-fade-enter-active,
.page-fade-leave-active {
  transition: opacity 200ms ease;
}

.page-fade-enter-from,
.page-fade-leave-to {
  opacity: 0;
}
</style>
