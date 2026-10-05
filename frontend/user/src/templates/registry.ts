import { useAppStore } from '../stores/app'

/**
 * 店面模板系统（站长全局切换 · 渐进并行迁移）
 *
 * - 当前激活模板优先级：本地预览覆盖(?template=) > 站点全局配置(storefront_template) > 默认 classic
 * - classic 沿用现有 ../views/*，其余模板落在 ./<name>/*；模板缺页时自动回退 classic，
 *   因此可以一页一页地把新设计搬进模板目录，旧站全程可用。
 */

export type StorefrontTemplate = 'classic' | 'vault' | 'atlas'

export const STOREFRONT_TEMPLATES: StorefrontTemplate[] = ['classic', 'vault', 'atlas']
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
 * 预览用：URL 带 ?template=vault / ?template=classic 时持久化到 localStorage，
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

// 各非 classic 模板的页面（按需动态加载）。key 形如 './vault/Home.vue' / './atlas/Home.vue'
// 注意：import.meta.glob 的参数必须是静态字面量，因此每新增一个模板都要在此登记一行。
const templateViews: Record<
    Exclude<StorefrontTemplate, 'classic'>,
    Record<string, () => Promise<unknown>>
> = {
    vault: import.meta.glob('./vault/**/*.vue'),
    atlas: import.meta.glob('./atlas/**/*.vue'),
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
            const loader = templateViews[active]?.[`./${active}/${name}.vue`]
            if (loader) return loader()
        }
        return classicLoader()
    }
}
