import { useAppStore } from '../stores/app'

/**
 * 店面模板系统（站长全局切换 · 渐进并行迁移）
 *
 * - 当前激活模板优先级：本地预览覆盖(?template=) > 站点全局配置(storefront_template) > 默认 classic
 * - classic 沿用现有 ../views/*，其余模板落在 ./<name>/*（vault、luna）；模板缺页时自动回退 classic，
 *   因此可以一页一页地把新设计搬进模板目录，旧站全程可用。
 */

export type StorefrontTemplate = 'classic' | 'vault' | 'luna'

export const STOREFRONT_TEMPLATES: StorefrontTemplate[] = ['classic', 'vault', 'luna']
export const DEFAULT_TEMPLATE: StorefrontTemplate = 'classic'

const OVERRIDE_KEY = 'dj-storefront-template'

const isTemplate = (value: unknown): value is StorefrontTemplate =>
    typeof value === 'string' && (STOREFRONT_TEMPLATES as string[]).includes(value)

const readOverride = (): StorefrontTemplate | null => {
    try {
        const value = localStorage.getItem(OVERRIDE_KEY)
        return isTemplate(value) ? value : null
    } catch {
        return null
    }
}

/**
 * 预览用：URL 带 ?template=vault / ?template=luna / ?template=classic 时持久化到 localStorage，
 * ?template=reset 清除覆盖。站长正式切换走站点配置，不依赖此入口。
 * 在 app 挂载前调用一次即可。
 */
export const initTemplateOverride = (): void => {
    if (typeof window === 'undefined') return
    try {
        const param = new URLSearchParams(window.location.search).get('template')
        if (param === 'reset') {
            localStorage.removeItem(OVERRIDE_KEY)
        } else if (isTemplate(param)) {
            localStorage.setItem(OVERRIDE_KEY, param)
        }
    } catch {
        /* localStorage 不可用时忽略 */
    }
}

/** 当前激活的店面模板。 */
export const getActiveTemplate = (): StorefrontTemplate => {
    const override = readOverride()
    if (override) return override
    try {
        const appStore = useAppStore()
        const fromConfig = appStore.config?.storefront_template
        if (isTemplate(fromConfig)) return fromConfig
    } catch {
        /* pinia 尚未就绪时退回默认 */
    }
    return DEFAULT_TEMPLATE
}

// 非 classic 模板的页面（按需动态加载）。key 形如 './vault/Home.vue'
const templateViews = import.meta.glob(['./vault/**/*.vue', './luna/**/*.vue'])

/**
 * 模板内的页面别名：该模板下 key 页面直接复用 value 页面（同一个组件，切换路由不会重新挂载）。
 * luna 没有「全部商品」页，/products 与 /categories/:slug 都是首页那套「选分类 → 选商品」。
 */
const templateViewAliases: Partial<Record<StorefrontTemplate, Record<string, string>>> = {
    luna: { Products: 'Home' },
}

type ViewLoader = () => Promise<unknown>

/**
 * 路由 view 解析器：当前模板下若存在同名页面则用模板版，否则回退传入的 classic loader。
 * 用法：`component: templateView('Home', () => import('../views/Home.vue'))`
 */
export const templateView = (name: string, classicLoader: ViewLoader): ViewLoader => {
    return () => {
        const active = getActiveTemplate()
        if (active !== 'classic') {
            const viewName = templateViewAliases[active]?.[name] ?? name
            const loader = templateViews[`./${active}/${viewName}.vue`]
            if (loader) return loader()
        }
        return classicLoader()
    }
}
