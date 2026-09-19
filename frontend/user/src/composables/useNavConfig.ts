import { computed, type Component } from 'vue'
import { useI18n } from 'vue-i18n'
import {
    Home, Bell,
    Link2, FileText, Globe, Star, Heart, MessageCircle, Gift, Zap, Shield,
    BookOpen, Code, Phone, MapPin, Music, Camera,
} from 'lucide-vue-next'
import { useAppStore } from '../stores/app'
import { getLocalizedText } from '../utils/resellerSiteConfig'

/**
 * 站点导航配置（后台「设置 → 导航」写入的 nav_config）。
 *
 * classic 与 vault 两套模板都从这里取导航项 —— 之前各写一份，vault 那份漏了
 * custom_items，导致后台配的自定义导航在 vault 模板下不显示。新模板接入导航时
 * 一律用这个 composable，不要再抄一遍解析逻辑。
 */

export interface NavItem {
    key: string
    path: string
    /** 已本地化的显示文本，模板直接渲染即可 */
    label: string
    icon: Component
    /** route 走 <router-link>，link 走原生 <a> */
    type: 'route' | 'link'
    target: string
}

interface CustomNavItemRaw {
    id?: number
    /** 后台存 title；分销站配置面板历史上用 name，两者都兼容 */
    title?: Record<string, string>
    name?: Record<string, string>
    link_type?: string
    url?: string
    target?: string
    sort_order?: number
    enabled?: boolean
    icon?: string
}

interface NavConfigRaw {
    builtin?: Record<string, boolean>
    custom_items?: CustomNavItemRaw[]
}

const builtinNavDefs: Record<string, { path: string; label: string; icon: Component }> = {
    notice: { path: '/notice', label: 'nav.notice', icon: Bell },
}

/** 后台自定义导航项可选的图标，key 与 SettingsNavigationTab.vue 的 presetIcons 一一对应 */
const presetIcons: Record<string, Component> = {
    link: Link2,
    document: FileText,
    globe: Globe,
    star: Star,
    heart: Heart,
    chat: MessageCircle,
    gift: Gift,
    lightning: Zap,
    shield: Shield,
    book: BookOpen,
    code: Code,
    phone: Phone,
    map: MapPin,
    music: Music,
    camera: Camera,
}

/** 绝对 URL / mailto / tel 一律当外链：塞进 <router-link :to> 会解析失败 */
const isExternalUrl = (url: string): boolean =>
    /^([a-z][a-z0-9+.-]*:)?\/\//i.test(url) || /^(mailto|tel):/i.test(url)

export const useNavConfig = () => {
    const { t, locale } = useI18n()
    const appStore = useAppStore()

    const navConfig = computed(() => appStore.config?.nav_config as NavConfigRaw | undefined)

    /** 列表模式下首页即商品列表，/products 与首页同页，导航里不再单独列一项 */
    const isListMode = computed(() => appStore.config?.template_mode === 'list')

    const blogEnabled = computed(() => navConfig.value?.builtin?.blog !== false)
    const noticeEnabled = computed(() => navConfig.value?.builtin?.notice !== false)
    const aboutEnabled = computed(() => navConfig.value?.builtin?.about !== false)

    /** 内置导航项（博客 / 公告 / 关于），受后台开关控制 */
    const builtinNavItems = computed<NavItem[]>(() => {
        const builtin = navConfig.value?.builtin
        const result: NavItem[] = []
        for (const [key, def] of Object.entries(builtinNavDefs)) {
            if (builtin && builtin[key] === false) continue
            result.push({ key, path: def.path, label: t(def.label), icon: def.icon, type: 'route', target: '_self' })
        }
        return result
    })

    /** 后台自定义导航项，按 sort_order 升序，过滤禁用及缺标题/缺链接的脏数据 */
    const customNavItems = computed<NavItem[]>(() => {
        const items = navConfig.value?.custom_items
        if (!Array.isArray(items)) return []
        return items
            .filter((item) => item.enabled !== false)
            .slice()
            .sort((a, b) => (a.sort_order || 0) - (b.sort_order || 0))
            .map((item, index) => {
                const url = String(item.url || '').trim()
                // 分销站配置面板写入的 custom_items 没有 link_type，只能按 URL 形态判断
                const external = item.link_type === 'external' || isExternalUrl(url)
                return {
                    key: `custom-${item.id ?? index}`,
                    // 站内路径缺前导斜杠会被 router 当相对路径解析，这里补齐
                    path: external || url.startsWith('/') ? url : `/${url}`,
                    label: getLocalizedText(item.title || item.name, locale.value),
                    icon: presetIcons[String(item.icon)] || Link2,
                    type: external ? ('link' as const) : ('route' as const),
                    target: item.target === '_blank' ? '_blank' : '_self',
                }
            })
            .filter((item) => item.label && item.path)
    })

    /** 首页 + 商品 + 内置 + 自定义，供顶栏主导航使用 */
    const primaryNavItems = computed<NavItem[]>(() => {
        const items: NavItem[] = [
            { key: 'home', path: '/', label: t('nav.home'), icon: Home, type: 'route', target: '_self' },
        ]
        items.push(...builtinNavItems.value, ...customNavItems.value)
        return items
    })

    /** 内置 + 自定义（不含首页/商品，供移动端抽屉等已在别处露出首页入口的场景使用） */
    const secondaryNavItems = computed<NavItem[]>(() => [...builtinNavItems.value, ...customNavItems.value])

    return {
        navConfig,
        isListMode,
        blogEnabled,
        noticeEnabled,
        aboutEnabled,
        builtinNavItems,
        customNavItems,
        primaryNavItems,
        secondaryNavItems,
    }
}
