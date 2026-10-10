import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAppStore } from '../stores/app'
import { postAPI, postCategoryAPI } from '../api'
import { debounceAsync } from '../utils/debounce'
import { usePageSeo } from './usePageSeo'

export interface UsePostListOptions {
  /** 是否启用分类筛选；仅博客列表需要，公告不支持分类。 */
  categoryFilter?: boolean
  /** 分类页路由名，URL 形如 /blog/category/:slug。 */
  categoryRouteName?: string
  /** 列表主页路由名，清除分类时回到这里。 */
  listRouteName?: string
}

/**
 * 文章/公告列表共享逻辑（Blog/Notice，classic + vault 双模板共用）。
 * 完整保留原 views/Blog.vue 与 Notice.vue 的行为，仅抽离为 composable。
 * Blog 启用搜索；Notice 无搜索框（searchKeyword 恒为空，watch 永不触发，行为一致）。
 *
 * 启用 categoryFilter 后按商品分类的同一套做法处理分类：对外一律用分类 slug ——
 * URL（/blog/category/:slug）与请求参数（category_slug）都用 slug，选中态也直接存 slug。
 * 这样分类标识全程不依赖自增 id，URL 与接口参数都稳定可读。
 */
export function usePostList(
  type: 'blog' | 'notice',
  seo: { title: () => string; canonicalPath: () => string },
  options: UsePostListOptions = {},
) {
  const {
    categoryFilter: categoryFilterEnabled = false,
    categoryRouteName = 'blog-category',
    listRouteName = 'blog',
  } = options

  const route = useRoute()
  const router = useRouter()
  const appStore = useAppStore()

  usePageSeo({
    title: seo.title,
    canonicalPath: seo.canonicalPath,
  })

  const loading = ref(true)
  const posts = ref<any[]>([])
  const currentPage = ref(1)
  const pageSize = ref(12)
  const total = ref(0)
  const totalPages = ref(0)
  const searchKeyword = ref('')
  const categories = ref<any[]>([])
  // 分类筛选：selectedCategory 存分类 slug，请求参数与 URL 都直接用它，不经过分类 id。
  const selectedCategory = ref<string | null>(null)

  let initializing = true

  // 后端选中父分类时会连带匹配其启用子分类，因此按「父分类 -> 其子分类」排列，
  // 让层级关系在平铺的筛选项里仍然可读。
  const orderedCategories = computed(() => {
    const all = categories.value
    const roots = all.filter((item: any) => !item.parent_id)
    const result: any[] = []
    for (const root of roots) {
      result.push(root)
      result.push(...all.filter((item: any) => item.parent_id === root.id))
    }
    return result
  })

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

  const loadCategories = async () => {
    if (!categoryFilterEnabled) return
    try {
      const response = await postCategoryAPI.list()
      categories.value = response.data.data || []
    } catch (error) {
      console.error('Failed to load post categories:', error)
    }
  }

  const loadPosts = async () => {
    loading.value = true
    try {
      const params: Record<string, any> = {
        type,
        page: currentPage.value,
        page_size: pageSize.value,
      }
      const keyword = searchKeyword.value.trim()
      if (keyword) {
        params.search = keyword
      }
      if (categoryFilterEnabled && selectedCategory.value) {
        params.category_slug = selectedCategory.value
      }
      const response = await postAPI.list(params)
      posts.value = response.data.data || []
      if (response.data.pagination) {
        total.value = response.data.pagination.total || 0
        totalPages.value = response.data.pagination.total_page || 0
      }
    } catch (error) {
      console.error('Failed to load posts:', error)
    } finally {
      loading.value = false
    }
  }

  const debouncedLoadPosts = debounceAsync(loadPosts, 300)

  /** 把 URL 上的 slug 同步为选中分类；分类未加载完或 slug 不在启用分类里时返回 false。 */
  const syncSelectedCategoryFromRoute = () => {
    if (!categoryFilterEnabled) return false
    if (route.name !== categoryRouteName) {
      if (selectedCategory.value !== null) {
        selectedCategory.value = null
      }
      return false
    }

    const slugParam = route.params.slug as string | undefined
    if (!slugParam || categories.value.length === 0) return false

    const matched = categories.value.find((category) => category.slug === slugParam)
    if (!matched) return false

    if (selectedCategory.value !== matched.slug) {
      selectedCategory.value = matched.slug
    }
    return true
  }

  /**
   * URL 指向分类页但该 slug 在启用分类里不存在（分类被删或停用）。
   * 分类尚未加载完时返回 false，避免网络失败被误判成 slug 失效。
   */
  const isUnknownCategorySlug = () => {
    if (!categoryFilterEnabled || route.name !== categoryRouteName) return false
    const slugParam = route.params.slug as string | undefined
    if (!slugParam || categories.value.length === 0) return false
    return !categories.value.some((category) => category.slug === slugParam)
  }

  /** 选项点击入口：只改选中 slug，URL 由 watch 统一同步，避免两处各写一次。 */
  const selectCategory = (slug: string | null) => {
    if (!categoryFilterEnabled) return
    selectedCategory.value = slug
  }

  const clearCategory = () => selectCategory(null)

  const goToPost = (slug: string) => {
    router.push(`/blog/${slug}`)
  }

  const changePage = (page: number) => {
    if (page < 1 || page > totalPages.value) return
    currentPage.value = page
    debouncedLoadPosts()
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }

  watch(selectedCategory, () => {
    if (initializing) return
    currentPage.value = 1
    debouncedLoadPosts()

    if (selectedCategory.value) {
      if (route.params.slug !== selectedCategory.value) {
        router.replace({ name: categoryRouteName, params: { slug: selectedCategory.value } })
      }
      return
    }
    if (route.name === categoryRouteName) {
      router.replace({ name: listRouteName })
    }
  })

  watch(searchKeyword, () => {
    if (initializing) return
    currentPage.value = 1
    debouncedLoadPosts()
  })

  // 面包屑跳转、浏览器前进后退都会改 slug，这里保持与 URL 一致。
  // 上面 watch(selectedCategory) 已先更新过 slug，所以它自己触发的 replace 不会重复请求。
  watch(
    () => route.params.slug,
    () => {
      if (initializing) return
      if (categories.value.length === 0) return
      if (isUnknownCategorySlug()) {
        router.replace({ name: listRouteName })
        return
      }
      syncSelectedCategoryFromRoute()
    },
  )

  const initialize = async () => {
    await loadCategories()
    if (categoryFilterEnabled) {
      if (isUnknownCategorySlug()) {
        // slug 已失效（分类被删或停用）：回落到列表页，避免 URL 与内容不一致。
        router.replace({ name: listRouteName })
      } else {
        syncSelectedCategoryFromRoute()
      }
    }
    await loadPosts()
    initializing = false
  }

  onMounted(initialize)

  onUnmounted(() => {
    debouncedLoadPosts.cancel()
  })

  return {
    loading,
    posts,
    currentPage,
    totalPages,
    total,
    searchKeyword,
    categories: orderedCategories,
    selectedCategory,
    categoryFilterEnabled,
    hasCategoryFilter: computed(() => selectedCategory.value !== null),
    selectCategory,
    clearCategory,
    getLocalizedText,
    formatDate,
    goToPost,
    changePage,
  }
}
