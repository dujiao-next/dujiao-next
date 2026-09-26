// 从 vitest/config 引入而不是 vite：只有它的 defineConfig 认识下面的 test 字段，
// 用 vite 的那个会让 vue-tsc 报 TS2769
import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

const cfAsyncModuleScriptPlugin = () => ({
  name: 'cfasync-module-script',
  transformIndexHtml(html: string) {
    return html.replace(
      /<script\s+type="module"(?![^>]*data-cfasync)/g,
      '<script data-cfasync="false" type="module"',
    )
  },
})

const adminBaseInjector = () => ({
  name: 'inject-admin-base-placeholder',
  transformIndexHtml(html: string) {
    // 在 <head> 开标签后立即插入 <base>。占位符 __DJ_ADMIN_BASE__
    // 由后端 internal/web 包在启动时一次性替换为实际 admin path。
    return html.replace(
      /<head[^>]*>/,
      (m) => `${m}\n    <base href="__DJ_ADMIN_BASE__/">`,
    )
  },
})

const isFullstack = process.env.VITE_FULLSTACK === '1'

// https://vite.dev/config/
export default defineConfig({
  base: isFullstack ? './' : '/',
  plugins: [
    vue(),
    cfAsyncModuleScriptPlugin(),
    ...(isFullstack ? [adminBaseInjector()] : []),
  ],
  server: {
    host: '0.0.0.0',
    port: 5174,
    strictPort: true,
    // 同 frontend/user：Docker Desktop 的 bind mount 不转发 inotify，
    // 必须开启轮询才能感知宿主上的文件改动。原生 Linux 可设 VITE_USE_POLLING=0 关闭。
    watch: {
      usePolling: process.env.VITE_USE_POLLING !== '0',
      interval: Number(process.env.VITE_POLL_INTERVAL || 300),
      ignored: ['**/node_modules/**', '**/.git/**', '**/dist/**', '**/coverage/**'],
    },
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/uploads': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          'vendor-vue': ['vue', 'vue-router', 'pinia', 'vue-i18n'],
          'vendor-ui': ['reka-ui', 'class-variance-authority', 'clsx', 'tailwind-merge', 'lucide-vue-next'],
          'vendor-tiptap': [
            '@tiptap/vue-3',
            '@tiptap/starter-kit',
            '@tiptap/extension-image',
            '@tiptap/extension-link',
            '@tiptap/extension-placeholder',
            '@tiptap/extension-text-align',
            '@tiptap/extension-color',
            '@tiptap/extension-text-style',
            '@tiptap/extension-table',
            '@tiptap/extension-table-cell',
            '@tiptap/extension-table-header',
            '@tiptap/extension-table-row',
          ],
        },
      },
    },
    chunkSizeWarningLimit: 600,
  },
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  // DOMPurify 只能在有 DOM 的环境里跑，因此 sanitizer 的单测需要 jsdom
  test: {
    environment: 'jsdom',
    include: ['src/**/*.test.ts'],
  },
})
