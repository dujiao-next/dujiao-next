import { ref, watch } from 'vue'

const TEA_THEME_KEY = 'dujiao_theme'

type Theme = 'light' | 'dark'

const getSystemTheme = (): Theme => {
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

const getSavedTheme = (): Theme | null => {
    return localStorage.getItem(TEA_THEME_KEY) as Theme | null
}

const theme = ref<Theme>(getSavedTheme() || getSystemTheme())

const applyTheme = (newTheme: Theme) => {
    const root = document.documentElement
    if (newTheme === 'dark') {
        root.classList.add('dark')
    } else {
        root.classList.remove('dark')
    }
    // 关键：声明「配色由站点自己处理」。
    // 不声明的话，安卓上的小米浏览器 / 微信 / UC 的「强制深色」会把浅色页面整体反色，
    // 表现为原本黑字的地方变白。声明后浏览器不再强制反色。
    root.style.colorScheme = newTheme
    localStorage.setItem(TEA_THEME_KEY, newTheme)
}

// Watch for changes and apply
watch(theme, (newVal) => {
    applyTheme(newVal)
}, { immediate: true })

export const useTheme = () => {
    const toggleTheme = () => {
        theme.value = theme.value === 'dark' ? 'light' : 'dark'
    }

    return {
        theme,
        toggleTheme
    }
}
