import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAppStore } from '../stores/app'

/**
 * 404 页共享逻辑（classic + vault 双模板共用）。
 * 完整保留原 views/NotFound.vue 的行为，仅抽离为 composable。
 */
export function useNotFound() {
  const router = useRouter()
  const appStore = useAppStore()

  const brandSiteName = computed(() => {
    const siteName = String(appStore.config?.brand?.site_name || '').trim()
    return siteName !== '' ? siteName : 'Dovelora'
  })

  const goBack = () => {
    if (window.history.length > 1) {
      router.back()
      return
    }
    router.push('/')
  }

  return {
    brandSiteName,
    goBack,
  }
}
