<template>
  <div class="bp" :class="{ 'bp--dark': dark, dark }">
    <div class="vault-scope">
      <BlockRenderer v-if="blocks.length" :blocks="blocks" />
      <div v-else class="bp-empty">
        <p class="bp-empty-title">预览待命</p>
        <p class="bp-empty-hint">后台编辑区块后，这里会实时渲染真实前台效果</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
/**
 * 后台积木编辑器专用预览页。
 *
 * 后台通过 iframe 加载本页，用 postMessage 把 content_blocks 推过来，
 * 这里用**跟线上完全同一套** BlockRenderer 渲染 —— 不再维护第二套 mock 预览。
 *
 * 协议（parent -> here）：
 *   { type: 'dj:preview', blocks, locale, dark }
 * 协议（here -> parent）：
 *   { type: 'dj:preview-ready' }
 *   { type: 'dj:preview-select', id }   // 点画布里的某个区块
 */
import { onMounted, onUnmounted, ref, nextTick } from 'vue'
import BlockRenderer from '../components/blocks/BlockRenderer.vue'
import { useAppStore } from '../stores/app'
import '../templates/apple/styles/store.css'

const appStore = useAppStore()
const blocks = ref<any[]>([])
const dark = ref(false)
const selected = ref<string | null>(null)

const post = (payload: Record<string, any>) => {
  // 预览页可能被嵌在任意 admin 源下，这里只回发给 parent
  window.parent?.postMessage(payload, '*')
}

const apply = async (payload: any) => {
  if (!payload) return
  if (Array.isArray(payload.blocks)) {
    blocks.value = payload.blocks
    await nextTick()
  }
  if (payload.locale && payload.locale !== appStore.locale) {
    appStore.locale = payload.locale
  }
  if (typeof payload.dark === 'boolean') {
    dark.value = payload.dark
  }
  selected.value = payload.selected ? String(payload.selected) : null
  await nextTick()
  paintSelection()
  markEmptyBlocks()
  window.setTimeout(markEmptyBlocks, 320)
  // 从右侧面板点进来时，把该区块滚到视野中间
  if (payload.reveal) {
    document.querySelector(`[data-block-id="${payload.reveal}"]`)?.scrollIntoView({ block: 'center', behavior: 'smooth' })
  }
}

/** 空格子标一下：否则“没填内容”和“没渲染出来”在画布上看起来一样 */
const markEmptyBlocks = () => {
  document.querySelectorAll<HTMLElement>('[data-block-id]').forEach((el) => {
    const hasMedia = !!el.querySelector('img, video, picture, canvas')
    const hasText = (el.innerText || '').trim().length > 0
    el.classList.toggle('is-empty-block', !hasMedia && !hasText)
  })
}

/** 选中描边：直接操作 DOM，避免为预览再加一层包装组件 */
const paintSelection = () => {
  document.querySelectorAll<HTMLElement>('[data-block-id]').forEach((el) => {
    el.classList.toggle('is-preview-selected', el.dataset.blockId === selected.value)
  })
}

const onMessage = (event: MessageEvent) => {
  const data = event.data
  if (!data || typeof data !== 'object') return
  if (data.type === 'dj:preview') {
    apply(data)
  }
}

/** 点画布里的区块 → 告诉后台选中它 */
const onClick = (event: MouseEvent) => {
  const target = event.target as HTMLElement | null
  const holder = target?.closest('[data-block-id]') as HTMLElement | null
  if (!holder) return
  post({ type: 'dj:preview-select', id: holder.dataset.blockId })
}

onMounted(() => {
  window.addEventListener('message', onMessage)
  document.addEventListener('click', onClick, true)
  post({ type: 'dj:preview-ready' })
})

onUnmounted(() => {
  window.removeEventListener('message', onMessage)
  document.removeEventListener('click', onClick, true)
})
</script>

<style>
html,
body {
  margin: 0;
  background: #fff;
}
.bp--dark,
.bp--dark body {
  background: #000;
}
.bp {
  min-height: 100vh;
  background: #fff;
}
.bp--dark {
  background: #000;
}
.bp-empty {
  display: flex;
  min-height: 60vh;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: #86868b;
  font-family: -apple-system, BlinkMacSystemFont, 'PingFang SC', 'Helvetica Neue', Arial, sans-serif;
}
.bp [data-block-id].is-empty-block {
  position: relative;
  min-height: 88px;
  outline: 1px dashed rgba(0, 0, 0, 0.18);
  outline-offset: -10px;
  border-radius: 12px;
}
.bp [data-block-id].is-empty-block::after {
  content: '空区块';
  position: absolute;
  top: 12px;
  left: 50%;
  transform: translateX(-50%);
  font-size: 11px;
  color: #9ca3af;
  letter-spacing: 0.04em;
}
.bp--dark [data-block-id].is-empty-block {
  outline-color: rgba(255, 255, 255, 0.28);
}
.bp :deep(.is-preview-selected) {
  outline: 2px solid #0071e3;
  outline-offset: -2px;
}
.bp-empty-title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
}
.bp-empty-hint {
  margin: 0;
  font-size: 12px;
}
</style>
