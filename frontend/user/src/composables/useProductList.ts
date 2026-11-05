import { computed, ref, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { productAPI, categoryAPI } from '../api'
import { buildCategoryGroups, createCategoryMap, normalizeCategoryParentId, type PublicCategory } from '../utils/category'
import { debounceAsync } from '../utils/debounce'
import { resetListScrollSignal, resolveListScrollSignal } from '../utils/listScrollSignal'

export interface UseProductListOptions {
  pageSize?: number
  homeRouteName?: string
  categoryRouteName?: string
}

export function useProductList(options: UseProductListOptions = {}) {
  const {
    pageSize: defaultPageSize = 20,
    homeRouteName = 'home',
    categoryRouteName = 'category-products',
  } = options

  const router = useRouter()
  const route = useRoute()

  const loading = ref(true)
  const products = ref<any[]>([])
  const categories = ref<PublicCategory[]>([])
  const selectedCategory = ref<number | null>(null)
  const searchQuery = ref('')
  const currentPage = ref(1)
  const pageSize = ref(defaultPageSize)
  const totalPages = ref(0)
  const showFilterDrawer = ref(false)
  const expandedParentIds = ref<number[]>([])

  const categoryGroups = computed(() => buildCategoryGroups(categories.value))
  const categoryMap = computed(() => createCategoryMap(categories.value))

  let initializing = true
  // 记录最近一次真正用于请求的页码/关键词，URL 监听据此判断是否需要重新加载。
  // 不能用 currentPage / searchQuery 比对：它们可能已被 changePage、清空搜索等提前改写过。
  let appliedPage = 1
  let appliedKeyword = ''

  const isParentExpanded = (categoryId: number) => {
    return expandedParentIds.value.includes(categoryId)
  }

  const expandParentCategory = (categoryId: number) => {
    if (!categoryId || isParentExpanded(categoryId)) return
    expandedParentIds.value = [...expandedParentIds.value, categoryId]
  }

  const toggleParentCategory = (categoryId: number) => {
    if (isParentExpanded(categoryId)) {
      expandedParentIds.value = expandedParentIds.value.filter((id) => id !== categoryId)
      return
    }
    expandParentCategory(categoryId)
  }

  const getParentToggleButtonClass = (categoryId: number) => {
    return isParentExpanded(categoryId)
      ? 'bg-primary text-primary-foreground border-transparent'
      : 'bg-card/70 text-muted-foreground hover:text-foreground'
  }

  const syncExpandedCategoryState = () => {
    if (!selectedCategory.value) return

    const matched = categoryMap.value.get(selectedCategory.value)
    if (!matched) return

    const parentId = normalizeCategoryParentId(matched.parent_id)
    if (parentId > 0) {
      expandParentCategory(parentId)
      return
    }

    const selectedGroup = categoryGroups.value.find((group) => group.id === matched.id)
    if (selectedGroup?.children.length) {
      expandParentCategory(selectedGroup.id)
    }
  }

  const selectCategory = (categoryId: number | null, closeDrawer = false) => {
    selectedCategory.value = categoryId
    if (closeDrawer) {
      showFilterDrawer.value = false
    }
  }

  /**
   * 计算需要写入 URL 的分页/搜索状态，基于最新 route.query 合并，
   * 避免多个写入点互相覆盖。
   * page=1、关键词为空时不带参数，保持 URL 简洁。
   */
  const buildQuery = (): Record<string, string> => {
    const query: Record<string, string> = { ...route.query as Record<string, string> }
    if (currentPage.value > 1) {
      query.page = String(currentPage.value)
    } else {
      delete query.page
    }
    const keyword = searchQuery.value.trim()
    if (keyword) {
      query.q = keyword
    } else {
      delete query.q
    }
    return query
  }

  /** 把分页/搜索状态写入 URL；URL 没有实际变化时返回 false，不会触发导航 */
  const syncQuery = (): boolean => {
    const query = buildQuery()
    const current = route.query as Record<string, string>
    const unchanged =
      Object.keys(query).length === Object.keys(current).length &&
      Object.entries(query).every(([key, value]) => current[key] === value)
    if (unchanged) return false
    // 连续输入时后一次导航会中止前一次，忽略这类可预期的 rejection
    router.replace({ query }).catch(() => {})
    return true
  }

  const loadProducts = async () => {
    loading.value = true
    resetListScrollSignal()
    const page = currentPage.value
    const keyword = searchQuery.value.trim()
    appliedPage = page
    appliedKeyword = keyword
    try {
      const params: any = {
        page,
        page_size: pageSize.value,
      }
      if (selectedCategory.value) {
        params.category_id = selectedCategory.value
      }
      if (keyword) {
        params.search = keyword
      }
      const response = await productAPI.list(params)
      products.value = response.data.data || []
      if (response.data.pagination) {
        totalPages.value = response.data.pagination.total_page || 0
      }
      // 页码超出范围时回落到末页并重新加载
      if (totalPages.value > 0 && page > totalPages.value) {
        currentPage.value = totalPages.value
        syncQuery()
        await loadProducts()
        return
      }
    } catch (error) {
      console.error('Failed to load products:', error)
    } finally {
      loading.value = false
      resolveListScrollSignal()
    }
  }

  const loadCategories = async () => {
    try {
      const response = await categoryAPI.list()
      categories.value = response.data.data || []
    } catch (error) {
      console.error('Failed to load categories:', error)
    }
  }

  const debouncedLoadProducts = debounceAsync(loadProducts, 300)

  const changePage = (page: number) => {
    if (page < 1 || page > totalPages.value) return
    currentPage.value = page
    // URL 更新后由 watch(route.query.page) 触发加载，避免双重请求
    syncQuery()
  }

  const clearSearch = () => {
    if (!searchQuery.value) return
    // watch(searchQuery) 负责写 URL；URL 已不含 q 时（例如刚进页面 q 为空）直接加载
    searchQuery.value = ''
    if (!syncQuery()) {
      currentPage.value = 1
      debouncedLoadProducts()
    }
  }

  const onSearch = () => {
    currentPage.value = 1
    // 关键词没变时 URL 不变，watch(route.query.q) 不会触发，需主动加载一次
    if (!syncQuery()) {
      debouncedLoadProducts()
    }
  }

  const syncSelectedCategoryFromRoute = () => {
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

    if (selectedCategory.value !== matched.id) {
      selectedCategory.value = matched.id
    }

    return true
  }

  /** 从 URL query.page 读取页码，非法值回落到 1 */
  const readPageFromQuery = (): number => {
    const raw = route.query.page
    const parsed = parseInt(Array.isArray(raw) ? (raw[0] ?? '') : (raw ?? ''), 10)
    return Number.isFinite(parsed) && parsed >= 1 ? parsed : 1
  }

  watch(selectedCategory, () => {
    if (initializing) return
    currentPage.value = 1
    syncExpandedCategoryState()
    debouncedLoadProducts()

    if (selectedCategory.value) {
      const matched = categories.value.find((category) => category.id === selectedCategory.value)
      if (matched?.slug && route.params.slug !== matched.slug) {
        // 切分类时保留关键词、丢掉页码，让 URL 简洁
        router.replace({ name: categoryRouteName, params: { slug: matched.slug }, query: buildQuery() })
      }
    } else if (route.name === categoryRouteName) {
      router.replace({ name: homeRouteName, query: buildQuery() })
    }
  })

  /**
   * 从 URL query.q 读取搜索关键词（已 trim）。
   * 输入框内容与它一致时，说明 searchQuery 的这次变化来自 URL（后退/前进/首次进入），
   * 不应该再写回 URL 或重置页码。
   */
  const readKeywordFromQuery = (): string => {
    const raw = route.query.q
    return (Array.isArray(raw) ? (raw[0] ?? '') : (raw ?? '')).trim()
  }

  watch(searchQuery, () => {
    if (initializing) return
    if (searchQuery.value.trim() === readKeywordFromQuery()) return
    currentPage.value = 1
    syncQuery()
    // 加载由 watch(route.query.q) 触发，避免一次输入发出两次请求
  })

  watch(
    () => route.params.slug,
    () => {
      if (initializing) return
      if (categories.value.length === 0) return
      syncSelectedCategoryFromRoute()
    },
  )

  // URL 里的 page / q 变化时（包括浏览器后退/前进）同步状态并加载
  watch(
    () => [route.query.page, route.query.q] as const,
    () => {
      if (initializing) return
      const page = readPageFromQuery()
      const keyword = readKeywordFromQuery()
      if (page === appliedPage && keyword === appliedKeyword) return
      const keywordChanged = keyword !== appliedKeyword
      currentPage.value = page
      // 只在 URL 与输入框真正不一致时回写（后退/前进、外部链接）。
      // 用 trim 后比较，避免把用户正在输入的尾随空格吃掉。
      if (searchQuery.value.trim() !== keyword) {
        searchQuery.value = keyword
      }
      debouncedLoadProducts()
      // 关键词变化时已回到第 1 页并重置列表，不需要额外滚动
      if (!keywordChanged) {
        window.scrollTo({ top: 0, behavior: 'smooth' })
      }
    },
  )

  const initialize = async () => {
    await loadCategories()
    if (syncSelectedCategoryFromRoute()) {
      syncExpandedCategoryState()
    }
    // 初始化时从 URL 恢复关键词和页码
    const keyword = readKeywordFromQuery()
    if (keyword) {
      searchQuery.value = keyword
    }
    currentPage.value = readPageFromQuery()
    await loadProducts()
    initializing = false
  }

  const cleanup = () => {
    debouncedLoadProducts.cancel()
  }

  return {
    loading,
    products,
    categories,
    selectedCategory,
    searchQuery,
    currentPage,
    pageSize,
    totalPages,
    showFilterDrawer,
    expandedParentIds,
    categoryGroups,
    categoryMap,
    isParentExpanded,
    toggleParentCategory,
    getParentToggleButtonClass,
    selectCategory,
    loadProducts,
    loadCategories,
    changePage,
    clearSearch,
    onSearch,
    syncSelectedCategoryFromRoute,
    syncExpandedCategoryState,
    initialize,
    cleanup,
  }
}
