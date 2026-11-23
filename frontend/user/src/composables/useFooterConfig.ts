import { computed, type Component } from 'vue'
import { Bell, Home, Info, LayoutGrid, Newspaper } from 'lucide-vue-next'
import { useAppStore } from '../stores/app'
import { useNavConfig } from './useNavConfig'
import { getImageUrl } from '../utils/image'
import { getLocalizedText } from '../utils/resellerSiteConfig'

/**
 * 站点页脚与品牌共享配置（classic Footer 与 atelier Layout 共用）。
 *
 * 统一从 appStore.config 解析品牌名称/Logo/简介、快捷导航、联系方式与自定义页脚链接，
 * 避免各模板各自复制解析逻辑导致新增配置项时漏改。
 */

export interface FooterQuickLink {
  path: string
  label: string
  icon: Component
}

export interface FooterContact {
  telegram?: string
  whatsapp?: string
  email?: string
  support_url?: string
}

export interface FooterCustomLink {
  name: string
  url: string
}

export interface UseFooterConfigOptions {
  /** 是否在快捷链接中包含公告入口（默认 false，保持 classic 既有展示） */
  includeNotice?: boolean
}

export const useFooterConfig = (options: UseFooterConfigOptions = {}) => {
  const appStore = useAppStore()
  const { isListMode, blogEnabled, noticeEnabled, aboutEnabled } = useNavConfig()

  const config = computed(() => appStore.config)
  const currentYear = new Date().getFullYear()

  const brandSiteName = computed(() => {
    const siteName = config.value?.brand?.site_name
    return typeof siteName === 'string' && siteName.trim() ? siteName.trim() : 'Dujiao-Next'
  })

  const brandInitial = computed(() => Array.from(brandSiteName.value)[0]?.toUpperCase() || 'D')

  const brandLogo = computed(() => {
    const raw = String(config.value?.brand?.site_logo || '').trim()
    return raw ? getImageUrl(raw) : ''
  })

  const brandDescription = computed(() => {
    const desc = config.value?.brand?.site_description
    if (desc && typeof desc === 'object') {
      const val = (desc as Record<string, string>)[appStore.locale] || (desc as Record<string, string>)['zh-CN'] || ''
      return typeof val === 'string' ? val.trim() : ''
    }
    return ''
  })

  const contact = computed(() => config.value?.contact as FooterContact | undefined)

  const hasContact = computed(() => Boolean(
    contact.value?.telegram || contact.value?.whatsapp || contact.value?.email || contact.value?.support_url
  ))

  const quickLinks = computed<FooterQuickLink[]>(() => {
    const items: FooterQuickLink[] = [{ path: '/', label: 'nav.home', icon: Home }]
    if (!isListMode.value) {
      items.push({ path: '/products', label: 'nav.products', icon: LayoutGrid })
    }
    if (blogEnabled.value) {
      items.push({ path: '/blog', label: 'nav.blog', icon: Newspaper })
    }
    if (options.includeNotice && noticeEnabled.value) {
      items.push({ path: '/notice', label: 'nav.notice', icon: Bell })
    }
    if (aboutEnabled.value) {
      items.push({ path: '/about', label: 'nav.about', icon: Info })
    }
    return items
  })

  const footerLinks = computed<FooterCustomLink[]>(() => {
    const links = config.value?.footer_links
    if (!Array.isArray(links)) return []
    return links
      .map((item: { name?: unknown; url?: unknown }) => ({
        name: typeof item?.name === 'string'
          ? item.name.trim()
          : getLocalizedText(item?.name as Record<string, string>, appStore.locale),
        url: String(item?.url || '').trim(),
      }))
      .filter((item) => item.name)
  })

  return {
    config,
    currentYear,
    brandSiteName,
    brandInitial,
    brandLogo,
    brandDescription,
    contact,
    hasContact,
    quickLinks,
    footerLinks,
  }
}
