import { createApp } from 'vue'
import { createPinia } from 'pinia'
import './style.css'
import App from './App.vue'
import router from './router'
import i18n from './i18n'
import { scheduleSessionKeepAlive } from './api/session'

const app = createApp(App)
app.use(createPinia())
app.use(router)
app.use(i18n)
app.mount('#app')

// 已有登录态时提前续期，避免编辑中断。
scheduleSessionKeepAlive()
