import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createHead } from '@unhead/vue/client'
import './style.css'
import App from './App.vue'
import router, { warmupCommonRoutes } from './router'
import i18n, { detectLocale, setI18nLocale, warmupLocaleMessages } from './i18n'
import { useTelegramMiniAppStore } from './stores/telegramMiniApp'
import { useAppStore } from './stores/app'
import { initTemplateOverride } from './templates/registry'

// 预览用：?template=vault 持久化激活模板（站长正式切换走站点配置）
initTemplateOverride()

const brandLog = (globalThis as any).console?.log?.bind(console)
brandLog?.(
  '%c Dujiao-Next %c Digital Commerce Platform ',
  'background:#0071e3;color:#fff;padding:4px 8px;border-radius:4px 0 0 4px;font-weight:bold;',
  'background:#1d1d1f;color:#f5f5f7;padding:4px 8px;border-radius:0 4px 4px 0;',
)
brandLog?.('%cGitHub → https://github.com/dujiao-next', 'color:#6e6e73;')

const app = createApp(App)
const head = createHead()
const pinia = createPinia()

app.use(pinia)
app.use(head)
app.use(router)
app.use(i18n)

// 站点配置必须在「挂载前」就绪 —— 这是消除「刷新时闪跳一下默认模板」的关键：
//
//   App.vue 的模板外壳是二选一结构（自带外壳的模板 vs classic 的 Navbar/Footer），
//   由 activeTemplate 决定；而 registry.getActiveTemplate() 在 config 缺失时会回退
//   DEFAULT_TEMPLATE('classic')。
//   此前 config 是在首次路由守卫里异步加载的，于是硬刷新的时序是：
//     App 挂载（config 空）→ 渲染 classic 外壳 → 守卫 await loadConfig 完成
//     → activeTemplate 变成站点实际模板（如 atlas）→ 整块外壳被替换
//   两个外壳结构差异很大（白色导航条 vs 深色两层顶栏），看起来就是「闪了一下默认模板」。
//
//   把 config 并入挂载前的并行等待后，App 挂载时 config 已就绪，不会先渲染错的外壳。
//   失败也必须照常挂载（否则首屏永远白屏），所以这里吞掉异常。
const loadConfigSafely = () =>
  useAppStore(pinia)
    .loadConfig()
    .catch(() => {
      /* 配置拉取失败不阻塞挂载；页面内仍有兜底与重试路径 */
    })

// 非默认语言的语言包为懒加载 chunk，挂载前并行加载，避免首屏文案闪现兜底语言
Promise.all([
  useTelegramMiniAppStore(pinia).init(),
  setI18nLocale(detectLocale()),
  loadConfigSafely(),
]).then(() => {
  app.mount('#app')
})

void router.isReady().then(() => {
    warmupCommonRoutes()
    warmupLocaleMessages()
})
