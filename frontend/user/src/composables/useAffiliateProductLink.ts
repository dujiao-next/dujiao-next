import { computed, ref, watch, type Ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { affiliateAPI } from '../api'
import { useAppStore } from '../stores/app'
import { useUserAuthStore } from '../stores/userAuth'
import { toast } from './useToast'

/**
 * 商品详情页的「复制推广链接」能力：已登录且已开通推广的用户，一键复制带自己推广码的单品直达链接。
 *
 * 链接形如 `/products/<slug>?aff=<推广码>`。归因由全局路由守卫（captureAffiliateFromRoute）统一处理，
 * 任何带 `aff` 参数的页面都会记录点击并写入 30 天归因，所以这里只负责生成链接，不新增任何归因路径。
 */

// 推广码按登录态缓存：dashboard 接口每次都会统计点击与佣金，不应随每次进入商品页重复查询。
let cachedToken = ''
let cachedCode: string | null = null

const loadAffiliateCode = async (token: string, isStale: () => boolean): Promise<string> => {
  if (cachedCode !== null && cachedToken === token) return cachedCode
  try {
    const response = await affiliateAPI.dashboard()
    const data = response.data?.data
    // 已被后台停用的推广账号不会再产生佣金，不提供分享入口，避免推广员白推。
    const code = data?.opened && data.status !== 'disabled' ? String(data.affiliate_code || '').trim() : ''
    // 请求期间用户可能已退出或切换账号，过期结果既不缓存也不使用。
    if (isStale()) return ''
    cachedToken = token
    cachedCode = code
    return code
  } catch {
    // 失败不缓存，下次进入页面会重试；入口按钮此时直接不展示。
    return ''
  }
}

export function useAffiliateProductLink(slug: Ref<string>) {
  const router = useRouter()
  const { t } = useI18n()
  const appStore = useAppStore()
  const userAuthStore = useUserAuthStore()

  const code = ref('')
  const enabled = computed(() => Boolean(appStore.config?.affiliate?.enabled))

  const refreshCode = async () => {
    if (!enabled.value || !userAuthStore.isAuthenticated) {
      code.value = ''
      return
    }
    const token = userAuthStore.token
    code.value = await loadAffiliateCode(token, () => userAuthStore.token !== token)
  }

  // 站点配置与登录态都是异步就绪的，任一变化都要重新判断。
  watch([enabled, () => userAuthStore.token], () => void refreshCode(), { immediate: true })

  const canShareAffiliateLink = computed(() => Boolean(code.value && slug.value))

  const affiliateProductLink = computed(() => {
    if (!canShareAffiliateLink.value) return ''
    const resolved = router.resolve({
      name: 'product-detail',
      params: { slug: slug.value },
      query: { aff: code.value },
    })
    return new URL(resolved.href, window.location.origin).toString()
  })

  const copyAffiliateLink = async () => {
    if (!affiliateProductLink.value) return
    try {
      await navigator.clipboard.writeText(affiliateProductLink.value)
      toast.success(t('productDetail.copyAffiliateLinkSuccess'))
    } catch {
      toast.error(t('productDetail.copyAffiliateLinkFailed'))
    }
  }

  return { canShareAffiliateLink, copyAffiliateLink }
}
