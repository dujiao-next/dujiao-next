<template>
  <div id="app" class="min-h-screen bg-background text-foreground flex flex-col">
    <!-- 自带顶栏/页脚的外壳模板（vault / atlas）：包裹页面；分销控制台仍走下方分支 -->
    <component :is="templateShell" v-if="templateShell && !isResellerConsole">
      <ErrorBoundary>
        <RouterView v-slot="{ Component }">
          <Transition name="page-fade" mode="out-in">
            <component :is="Component" />
          </Transition>
        </RouterView>
      </ErrorBoundary>
    </component>

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

    <Loading :loading="appStore.loading" />
    <Toast />
    <ConfirmDialog />
    <SupportWidget v-if="!isResellerConsole" />
  </div>
</template>

<script setup lang="ts">
import { computed, defineAsyncComponent } from 'vue'
import { useRoute } from 'vue-router'
import { useAppStore } from './stores/app'
import { getActiveTemplate } from './templates/registry'
import Navbar from './components/Navbar.vue'
import Footer from './components/Footer.vue'
import Loading from './components/Loading.vue'
import Toast from './components/Toast.vue'
import ConfirmDialog from './components/ConfirmDialog.vue'
import ErrorBoundary from './components/ErrorBoundary.vue'
import BackToTop from './components/BackToTop.vue'
import MobileBottomNav from './components/MobileBottomNav.vue'
import SupportWidget from './components/SupportWidget.vue'

// 各模板外壳按需加载，classic 用户不会拉取其 chunk/样式
const VaultLayout = defineAsyncComponent(() => import('./templates/vault/layout/VaultLayout.vue'))
const AtlasLayout = defineAsyncComponent(() => import('./templates/atlas/layout/AtlasLayout.vue'))

// config 由 router.beforeEach 统一加载，无需在此重复调用
const appStore = useAppStore()
const route = useRoute()
const isResellerConsole = computed(() => route.meta.resellerConsole === true)

// getActiveTemplate 读取 appStore.config（响应式），config 加载后会重新计算
const activeTemplate = computed(() => getActiveTemplate())

/** 外壳模板：返回 null 表示该模板不自带外壳（classic），走下方分支。 */
const templateShell = computed(() => {
  if (activeTemplate.value === 'vault') return VaultLayout
  if (activeTemplate.value === 'atlas') return AtlasLayout
  return null
})
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
