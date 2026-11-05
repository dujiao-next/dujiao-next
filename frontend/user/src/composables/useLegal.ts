import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '../stores/app'
import { usePageSeo } from './usePageSeo'
import { processHtmlForDisplay } from '../utils/content'

/**
 * 条款/隐私页共享逻辑（classic + vault 双模板共用）。
 * 完整保留原 views/Legal.vue 的行为，仅抽离为 composable。
 * type 以 getter 传入以保持响应性（来源于组件 props）。
 */
export function useLegal(type: () => 'terms' | 'privacy') {
  const { t } = useI18n()
  const appStore = useAppStore()

  usePageSeo({
    title: () => type() === 'terms' ? t('footer.terms') : t('footer.privacy'),
    canonicalPath: () => type() === 'terms' ? '/terms' : '/privacy',
  })

  const loading = computed(() => appStore.loading)
  const locale = computed(() => appStore.locale)

  const title = computed(() => {
    return type() === 'terms' ? t('footer.terms') : t('footer.privacy')
  })

  const content = computed(() => {
    const config = appStore.config
    if (!config?.legal) return ''

    const legal = config.legal
    const lang = locale.value

    if (type() === 'terms' && legal.terms) {
      return processHtmlForDisplay(legal.terms[lang] || '')
    } else if (type() === 'privacy' && legal.privacy) {
      return processHtmlForDisplay(legal.privacy[lang] || '')
    }
    return ''
  })

  return {
    loading,
    title,
    content,
  }
}
