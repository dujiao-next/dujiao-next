import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useUserAuthStore } from '../stores/userAuth'
import { useUserProfileStore } from '../stores/userProfile'
import {
  consumeGoogleRedirectIntent,
  createGoogleRedirectIntent,
  getGoogleRedirectSessionStorage,
  parseGoogleRedirectCallbackQuery,
} from '../utils/googleRedirect'

/**
 * Google GIS redirect callback shared by classic and vault templates.
 *
 * The URL is deliberately treated as non-sensitive routing metadata. Verified
 * identity data is exchanged exclusively through the backend's one-time
 * HttpOnly handoff cookie.
 */
export function useGoogleRedirectCallback() {
  const { t } = useI18n()
  const route = useRoute()
  const router = useRouter()
  const userAuthStore = useUserAuthStore()
  const userProfileStore = useUserProfileStore()
  const loading = ref(true)
  const errMsg = ref('')
  const retryPath = ref('/auth/login')
  let returnPath = '/me/orders'

  // 登录成功（含邀请码补填后）的后续跳转；需要邀请码时停在本页由 InviteCodeDialog 接管
  const continueAfterLogin = async (result: { requiresTotp: boolean; inviteRequired?: boolean }) => {
    if (result.inviteRequired) return
    if (result.requiresTotp) {
      await router.replace({
        path: '/auth/login',
        query: {
          google2fa: '1',
          redirect: returnPath,
        },
      })
      return
    }
    await router.replace(returnPath)
  }

  const cancelInvite = () => router.replace('/auth/login')

  const fail = (message?: string) => {
    errMsg.value = message || t('auth.googleCallback.failed')
    loading.value = false
  }

  onMounted(async () => {
    const callback = parseGoogleRedirectCallbackQuery(
      route.query as Record<string, unknown>,
    )
    const storedIntent = consumeGoogleRedirectIntent(
      getGoogleRedirectSessionStorage(),
    )

    if (!callback) {
      fail()
      return
    }

    retryPath.value = callback.flow === 'bind' && userAuthStore.isAuthenticated
      ? '/me/security'
      : '/auth/login'

    if (callback.error) {
      fail()
      return
    }

    // sessionStorage only restores local UX state. Missing, expired, or
    // mismatched data falls back safely; the server-side handoff is authoritative.
    const intent = storedIntent?.flow === callback.flow
      ? storedIntent
      : createGoogleRedirectIntent(callback.flow, undefined)

    try {
      if (callback.flow === 'bind') {
        const bound = await userProfileStore.exchangeGoogleRedirectBind()
        if (!bound) {
          fail(userProfileStore.securityError || t('auth.googleCallback.failed'))
          return
        }
        await router.replace({
          path: '/me/security',
          query: { googleBound: '1' },
        })
        return
      }

      returnPath = intent.returnPath
      const result = await userAuthStore.googleRedirectLogin()
      await continueAfterLogin(result)
    } catch (error: any) {
      fail(error?.message || t('auth.googleCallback.failed'))
    }
  })

  return {
    loading,
    errMsg,
    retryPath,
    continueAfterLogin,
    cancelInvite,
  }
}
