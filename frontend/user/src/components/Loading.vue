<template>
  <div
    v-if="visible"
    class="st-topbar-loader"
    role="progressbar"
    aria-label="loading"
    aria-valuetext="loading"
  >
    <span class="st-topbar-loader-bar" />
  </div>
</template>

<script setup lang="ts">
import { onUnmounted, ref, watch } from 'vue'

/**
 * 全局加载指示：顶部 2px 进度条（Safari / 苹果站点的做法）。
 * 不用全屏遮罩——苹果的页面加载是「内容直接出现」，慢的地方用骨架屏，
 * 全屏转圈既挡操作又显得旧。
 */
const props = defineProps<{ loading: boolean }>()

const visible = ref(false)
let timer: ReturnType<typeof setTimeout> | null = null

watch(
  () => props.loading,
  (loading) => {
    if (timer) {
      clearTimeout(timer)
      timer = null
    }
    if (loading) {
      // 120ms 内结束的请求不显示，避免闪一下
      timer = setTimeout(() => { visible.value = true }, 120)
    } else {
      visible.value = false
    }
  },
  // 首次渲染时 loading 可能已经是 true（配置还没回来），必须立即求值一次
  { immediate: true },
)

onUnmounted(() => {
  if (timer) clearTimeout(timer)
})
</script>

<style scoped>
.st-topbar-loader {
  position: fixed;
  inset: 0 0 auto 0;
  z-index: 100;
  height: 2px;
  overflow: hidden;
  pointer-events: none;
}

.st-topbar-loader-bar {
  position: absolute;
  inset-block: 0;
  width: 40%;
  border-radius: 2px;
  background: #0071e3;
  animation: st-topbar-slide 1s cubic-bezier(0.4, 0, 0.2, 1) infinite;
}

@keyframes st-topbar-slide {
  0% {
    left: -40%;
  }
  100% {
    left: 100%;
  }
}

@media (prefers-reduced-motion: reduce) {
  .st-topbar-loader-bar {
    animation: none;
    width: 100%;
    opacity: 0.6;
  }
}
</style>
