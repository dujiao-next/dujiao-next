import { createI18n } from 'vue-i18n'
import zhCN from './locales/zh-CN.json'
import zhTW from './locales/zh-TW.json'
import enUS from './locales/en-US.json'

const supportedLocales = ['zh-CN', 'zh-TW', 'en-US']

export function detectLocale(): string {
    const saved = localStorage.getItem('locale')
    if (saved && supportedLocales.includes(saved)) return saved

    const browserLang = navigator.language || ''
    if (supportedLocales.includes(browserLang)) return browserLang

    const langPrefix = browserLang.split('-')[0]
    if (langPrefix === 'zh') {
        if (browserLang.includes('TW') || browserLang.includes('HK') || browserLang.includes('Hant')) {
            return 'zh-TW'
        }
        return 'zh-CN'
    }
    if (langPrefix === 'en') return 'en-US'

    return 'zh-CN'
}

// All supported messages are available before Vue mounts. This keeps immediate
// mounting while preventing the fallback locale from being painted first.
const initialLocale = detectLocale()

const i18n = createI18n({
    legacy: false,
    locale: initialLocale,
    fallbackLocale: 'zh-CN',
    messages: {
        'zh-CN': zhCN,
        'zh-TW': zhTW,
        'en-US': enUS,
    } as Record<string, typeof zhCN>,
})

// 记录最新一次切换请求，防止快速连续切换时慢加载的旧请求覆盖新选择
let pendingLocale = ''

export async function setI18nLocale(locale: string): Promise<void> {
    if (!supportedLocales.includes(locale)) return
    pendingLocale = locale
    if (pendingLocale !== locale) return
    i18n.global.locale.value = locale
}

// Kept as a compatibility no-op for boot code and callers from older bundles.
export function warmupLocaleMessages(): void {
    // Messages are bundled eagerly so the detected locale is ready at first paint.
}

export default i18n
