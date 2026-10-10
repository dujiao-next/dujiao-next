import { ref, onMounted, onUnmounted, computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '../stores/app'
import { postAPI } from '../api'
import { debounceAsync } from '../utils/debounce'
import { useLocalized } from './useProduct'
import { usePageSeo } from './usePageSeo'

/**
 * 文章/公告详情共享逻辑（classic + vault 双模板共用）。
 * 完整保留原 views/BlogDetail.vue 的行为，仅抽离为 composable。
 *
 * 层级文案按 post.type 分派（公告与博客各有短名与完整返回文案），
 * 判断只服务于本文件，因此就地内联，不再单独抽 utils 模块。
 */
export function useBlogDetail() {
  const route = useRoute()
  const { t } = useI18n()
  const appStore = useAppStore()
  const { formatPrice } = useLocalized()

  const loading = ref(true)
  const post = ref<any>(null)
  const relatedProducts = computed<any[]>(() => post.value?.related_products || [])

  // 公告与博客在面包屑层级、返回文案、返回目标上都不同；详情未加载完时按博客处理，
  // 避免面包屑出现空白层级。
  const isNotice = computed(() => post.value?.type === 'notice')

  const getLocalizedText = (jsonData: any) => {
    if (!jsonData) return ''
    const locale = appStore.locale
    return jsonData[locale] || jsonData['zh-CN'] || jsonData['en-US'] || ''
  }

  const formatDate = (dateString: string) => {
    if (!dateString) return ''
    const date = new Date(dateString)
    return date.toLocaleDateString(appStore.locale, {
      year: 'numeric',
      month: 'long',
      day: 'numeric',
    })
  }

  usePageSeo({
    title: () => post.value ? getLocalizedText(post.value.title) : '',
    description: () => post.value ? getLocalizedText(post.value.summary) : '',
    image: () => post.value?.thumbnail || '',
    canonicalPath: () => `/blog/${(route.params.slug as string) || ''}`,
    type: () => 'article',
  })

  const backLink = computed(() => (isNotice.value ? '/notice' : '/blog'))

  // 底部返回按钮用完整文案（"返回博客列表"）。
  const backText = computed(() =>
    t(isNotice.value ? 'blogDetail.backToNotice' : 'blogDetail.backToBlog'),
  )

  // 面包屑里的层级标签用短名（"博客"/"公告"），与顶部导航保持一致；
  // "返回博客列表"在窄屏会和分类、标题挤成一团，故面包屑不复用 backText。
  const backLabel = computed(() => t(isNotice.value ? 'nav.notice' : 'nav.blog'))

  // 分类标识与名称随详情一起返回（都是后台设置的分类数据），公告没有分类因此恒为空。
  const categorySlug = computed(() => {
    const slug = post.value?.category_slug
    return typeof slug === 'string' && slug ? slug : ''
  })

  const categoryName = computed(() => getLocalizedText(post.value?.category_name))

  // 分类链接指向该分类的博客列表，用分类 slug 而非自增 id（对齐商品的 /categories/:slug）。
  const categoryLink = computed(() => {
    if (!categorySlug.value) return null
    return { name: 'blog-category', params: { slug: categorySlug.value } }
  })

  const loadPost = async () => {
    loading.value = true
    try {
      const slug = route.params.slug as string
      const response = await postAPI.detail(slug)
      post.value = response.data.data || null
    } catch (error) {
      console.error('Failed to load post:', error)
      post.value = null
    } finally {
      loading.value = false
    }
  }

  const debouncedLoadPost = debounceAsync(loadPost, 300)

  onMounted(() => {
    loadPost()
  })

  onUnmounted(() => {
    debouncedLoadPost.cancel()
  })

  return {
    loading,
    post,
    relatedProducts,
    getLocalizedText,
    formatDate,
    formatPrice,
    backLink,
    backText,
    backLabel,
    categoryName,
    categoryLink,
  }
}
