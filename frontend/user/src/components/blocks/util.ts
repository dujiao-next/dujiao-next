import { useAppStore } from '../../stores/app'

/** 多语言文本取值：缺省回退中文，再回退英文 */
export function useBlockText() {
  const appStore = useAppStore()
  const txt = (value: any): string => {
    if (!value) return ''
    if (typeof value === 'string') return value
    const locale = appStore.locale || 'zh-CN'
    return value[locale] || value['zh-CN'] || value['en-US'] || value['zh-TW'] || ''
  }
  return { txt, locale: () => appStore.locale }
}
