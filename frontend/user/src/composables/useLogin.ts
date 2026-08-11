import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useUserAuthStore } from '../stores/userAuth'
import { useI18n } from 'vue-i18n'
import { debounceAsync } from '../utils/debounce'
import { useAppStore } from '../stores/app'
import { useTelegramMiniAppStore } from '../stores/telegramMiniApp'
import { buildTelegramMiniAppEntryLink, isTelegramUrlEnvironment, openTelegramCompatibleLink } from '../utils/telegramMiniApp'
import { userAuthAPI } from '../api'
import type { CaptchaPayload, TelegramAuthPayload } from '../api'
import {
  canShowGoogleIdentityButton,
  detectGoogleIdentityUXMode,
  hasThirdPartyLoginOption,
} from '../utils/googleIdentity'
import {
  createGoogleRedirectIntent,
  createGoogleRedirectPreparedIntent,
  getGoogleRedirectSessionStorage,
  shouldResumeGoogleRedirect2FA,
  storeGoogleRedirectIntent,
  tryBuildGoogleRedirectCredentialCallbackURL,
} from '../utils/googleRedirect'
import ImageCaptcha from '../components/captcha/ImageCaptcha.vue'
import TurnstileCaptcha from '../components/captcha/TurnstileCaptcha.vue'
import { useFormValidation } from './useFormValidation'

/**
 * 用户登录页共享逻辑（classic + vault 双模板共用）。
 * 完整保留原 views/auth/Login.vue 的行为，仅抽离为 composable。
 */
export function useLogin() {
  const router = useRouter()
  const route = useRoute()
  const userAuthStore = useUserAuthStore()
  const appStore = useAppStore()
  const telegramMiniAppStore = useTelegramMiniAppStore()
  const { t } = useI18n()

  const brandSiteName = computed(() => {
    const siteName = String(appStore.config?.brand?.site_name || '').trim()
    return siteName !== '' ? siteName : 'Dovelora'
  })

  const email = ref('')
  const password = ref('')
  const showPassword = ref(false)
  const rememberMe = ref(true)

  const resumeGoogleRedirect2FAOnMount = shouldResumeGoogleRedirect2FA(
    route.query.google2fa,
    userAuthStore.challengeToken,
  )
  const step = ref<'password' | 'totp'>(
    resumeGoogleRedirect2FAOnMount ? 'totp' : 'password',
  )
  const totpMode = ref<'code' | 'recovery'>('code')
  const totpCode = ref('')
  const recoveryCode = ref('')
  const challengeRemainingSeconds = ref(0)
  let challengeTimer: ReturnType<typeof setInterval> | null = null

  const formValidation = useFormValidation(['email', 'password'])
  formValidation.addRule('email', formValidation.requiredRule())
  formValidation.addRule('email', formValidation.emailRule())
  formValidation.addRule('password', formValidation.requiredRule())
  const error = ref('')
  const info = ref('')
  const captchaPayload = ref<CaptchaPayload>({})
  const turnstileToken = ref('')
  const imageCaptchaRef = ref<InstanceType<typeof ImageCaptcha> | null>(null)
  const turnstileRef = ref<InstanceType<typeof TurnstileCaptcha> | null>(null)
  const telegramWidgetRef = ref<HTMLDivElement | null>(null)

  const captchaConfig = computed(() => appStore.config?.captcha || null)
  const captchaProvider = computed(() => String(captchaConfig.value?.provider || 'none'))
  const loginCaptchaEnabled = computed(() => !!captchaConfig.value?.scenes?.login && captchaProvider.value !== 'none')
  const turnstileSiteKey = computed(() => String(captchaConfig.value?.turnstile?.site_key || ''))
  const telegramConfig = computed(() => appStore.config?.telegram_auth || null)
  const telegramBotUsername = computed(() => String(telegramConfig.value?.bot_username || '').trim())
  const telegramMiniAppURL = computed(() => String(telegramConfig.value?.mini_app_url || '').trim())
  const telegramEnabled = computed(() => !!telegramConfig.value?.enabled && telegramBotUsername.value !== '')
  const telegramLoginMode = computed(() => String(telegramConfig.value?.mode || '').trim())
  const isWidgetMode = computed(() => telegramLoginMode.value === 'widget' || (telegramLoginMode.value === '' && telegramEnabled.value))
  const googleConfig = computed(() => appStore.config?.google_auth || null)
  const googleClientID = computed(() => String(googleConfig.value?.client_id || '').trim())
  const googleEnabled = computed(() => !!googleConfig.value?.enabled && googleClientID.value !== '')
  const googleButtonLocale = computed(() => String(appStore.locale || '').trim())
  const googleIdentityUXMode = detectGoogleIdentityUXMode()
  const googleRedirectLoginURI = googleIdentityUXMode === 'redirect'
    ? tryBuildGoogleRedirectCredentialCallbackURL()
    : ''
  const googleRedirectAvailable = googleIdentityUXMode === 'popup' || googleRedirectLoginURI !== ''
  const registrationEnabled = computed(() => appStore.config?.registration_enabled !== false)
  const emailVerificationEnabled = computed(() => appStore.config?.email_verification_enabled !== false)
  const isTelegramUrlEnv = isTelegramUrlEnvironment()
  const isTelegramMiniApp = computed(() => (telegramMiniAppStore.isMiniApp && telegramMiniAppStore.isReady) || isTelegramUrlEnv)
  const miniAppInitData = computed(() => String(telegramMiniAppStore.initData || '').trim())
  const showTelegramWidget = computed(() => isWidgetMode.value && telegramEnabled.value && !isTelegramMiniApp.value)
  const showTelegramOidc = computed(() => telegramLoginMode.value === 'oidc' && telegramEnabled.value && !isTelegramMiniApp.value)
  const showMiniAppLoginHint = computed(() => isTelegramMiniApp.value)
  const showGoogleLogin = computed(() => canShowGoogleIdentityButton(
    googleEnabled.value,
    googleClientID.value,
    isTelegramMiniApp.value,
  ) && googleRedirectAvailable)
  const showThirdPartyLogin = computed(() => hasThirdPartyLoginOption(
    showTelegramWidget.value,
    showTelegramOidc.value,
    showMiniAppLoginHint.value,
    showGoogleLogin.value,
  ))
  const telegramMiniAppEntryLink = computed(() => buildTelegramMiniAppEntryLink(telegramBotUsername.value, telegramMiniAppURL.value))
  const showTelegramMiniAppEntry = computed(() => !isTelegramMiniApp.value && telegramMiniAppEntryLink.value !== '')
  const telegramCallbackName = '__dujiaoUserTelegramLogin'
  const miniAppLoginAttempted = ref(false)
  const attemptingMiniAppLogin = ref(false)

  const getCaptchaPayload = (): CaptchaPayload | undefined => {
    if (!loginCaptchaEnabled.value) return undefined
    if (captchaProvider.value === 'image') {
      return {
        captcha_id: captchaPayload.value.captcha_id || '',
        captcha_code: captchaPayload.value.captcha_code || '',
      }
    }
    if (captchaProvider.value === 'turnstile') {
      return {
        turnstile_token: turnstileToken.value,
      }
    }
    return undefined
  }

  const handleCaptchaConfigStale = async () => {
    await appStore.loadConfig(true)
    captchaPayload.value = {}
    turnstileToken.value = ''
  }

  const redirectAfterLogin = () => {
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/me/orders'
    return router.push(redirect)
  }

  const openTelegramMiniAppEntry = () => {
    if (telegramMiniAppEntryLink.value === '') return
    openTelegramCompatibleLink(telegramMiniAppEntryLink.value)
  }

  const performLogin = async () => {
    error.value = ''
    if (!formValidation.validateAll({ email: email.value, password: password.value })) return

    if (loginCaptchaEnabled.value && captchaProvider.value === 'image') {
      if (!captchaPayload.value.captcha_id || !captchaPayload.value.captcha_code) {
        error.value = t('auth.common.captchaRequired')
        return
      }
    }

    if (loginCaptchaEnabled.value && captchaProvider.value === 'turnstile') {
      if (!turnstileToken.value) {
        error.value = t('auth.common.captchaRequired')
        return
      }
    }

    try {
      const result = await userAuthStore.login({
        email: email.value,
        password: password.value,
        remember_me: rememberMe.value,
        captcha_payload: getCaptchaPayload(),
      })
      if (result && result.requiresTotp) {
        enter2FAStep()
        return
      }
      await redirectAfterLogin()
    } catch (err: any) {
      error.value = err.message || t('auth.login.error')
      if (captchaProvider.value === 'image') {
        imageCaptchaRef.value?.refresh()
      }
      if (captchaProvider.value === 'turnstile') {
        turnstileRef.value?.reset()
        turnstileToken.value = ''
      }
    }
  }

  const handleLogin = debounceAsync(performLogin, 200)

  const stopChallengeCountdown = () => {
    if (challengeTimer) {
      clearInterval(challengeTimer)
      challengeTimer = null
    }
  }

  const startChallengeCountdown = () => {
    stopChallengeCountdown()
    const tick = () => {
      const expiresAt = userAuthStore.challengeExpiresAt
      if (!expiresAt) {
        challengeRemainingSeconds.value = 0
        stopChallengeCountdown()
        return
      }
      const diff = Math.max(0, Math.floor((new Date(expiresAt).getTime() - Date.now()) / 1000))
      challengeRemainingSeconds.value = diff
      if (diff <= 0) {
        stopChallengeCountdown()
        cancel2FA()
        error.value = t('auth.login.totp.expired')
      }
    }
    tick()
    challengeTimer = setInterval(tick, 1000)
  }

  const cancel2FA = () => {
    stopChallengeCountdown()
    userAuthStore.clearChallenge()
    step.value = 'password'
    totpCode.value = ''
    recoveryCode.value = ''
    challengeRemainingSeconds.value = 0
  }

  const performVerify2FA = async () => {
    error.value = ''
    if (totpMode.value === 'code') {
      const code = totpCode.value.trim()
      if (code === '') {
        error.value = t('auth.login.totp.codeRequired')
        return
      }
      try {
        await userAuthStore.verify2FA({ code })
        stopChallengeCountdown()
        await redirectAfterLogin()
      } catch (err: any) {
        error.value = err.message || t('auth.login.totp.verifyFailed')
        totpCode.value = ''
      }
      return
    }
    const rc = recoveryCode.value.trim()
    if (rc === '') {
      error.value = t('auth.login.totp.recoveryRequired')
      return
    }
    try {
      await userAuthStore.verify2FA({ recovery_code: rc })
      stopChallengeCountdown()
      await redirectAfterLogin()
    } catch (err: any) {
      error.value = err.message || t('auth.login.totp.verifyFailed')
      recoveryCode.value = ''
    }
  }

  const handleVerify2FA = debounceAsync(performVerify2FA, 200)

  const buildTelegramPayload = (raw: any): TelegramAuthPayload | null => {
    const id = Number(raw?.id)
    const authDate = Number(raw?.auth_date)
    const hash = String(raw?.hash || '').trim()
    if (!Number.isFinite(id) || id <= 0 || !Number.isFinite(authDate) || authDate <= 0 || hash === '') {
      return null
    }
    return {
      id,
      first_name: String(raw?.first_name || '').trim(),
      last_name: String(raw?.last_name || '').trim(),
      username: String(raw?.username || '').trim(),
      photo_url: String(raw?.photo_url || '').trim(),
      auth_date: authDate,
      hash,
    }
  }

  const enter2FAStep = () => {
    step.value = 'totp'
    totpMode.value = 'code'
    totpCode.value = ''
    recoveryCode.value = ''
    startChallengeCountdown()
  }

  const handleTelegramAuth = async (raw: any) => {
    error.value = ''
    const payload = buildTelegramPayload(raw)
    if (!payload) {
      error.value = t('auth.login.telegramInvalidPayload')
      return
    }
    try {
      const result = await userAuthStore.telegramLogin(payload)
      if (result && result.requiresTotp) {
        enter2FAStep()
        return
      }
      await redirectAfterLogin()
    } catch (err: any) {
      error.value = err.message || t('auth.login.telegramLoginFailed')
    }
  }

  const handleGoogleCredential = async (credential: string) => {
    if (userAuthStore.loading) return
    error.value = ''
    const normalizedCredential = String(credential || '').trim()
    if (normalizedCredential === '') {
      error.value = t('auth.login.googleInvalidCredential')
      return
    }

    try {
      const result = await userAuthStore.googleLogin(normalizedCredential)
      if (result?.requiresTotp) {
        enter2FAStep()
        return
      }
      await redirectAfterLogin()
    } catch (err: any) {
      error.value = err?.message || t('auth.login.googleLoginFailed')
    }
  }

  const handleGoogleScriptError = () => {
    error.value = t('auth.login.googleWidgetLoadFailed')
  }

  const prepareGoogleRedirectLogin = async () => {
    const response = await userAuthAPI.googleRedirectIntent()
    const preparedIntent = createGoogleRedirectPreparedIntent(response.data.data)
    if (!preparedIntent) {
      throw new Error('Google redirect state is invalid')
    }
    const intent = createGoogleRedirectIntent(
      'login',
      route.query.redirect,
      preparedIntent.issuedAt,
    )
    storeGoogleRedirectIntent(getGoogleRedirectSessionStorage(), intent)
    return preparedIntent
  }

  const tryTelegramMiniAppLogin = async () => {
    if (!isTelegramMiniApp.value || miniAppInitData.value === '' || miniAppLoginAttempted.value || attemptingMiniAppLogin.value) {
      return
    }

    miniAppLoginAttempted.value = true
    attemptingMiniAppLogin.value = true
    error.value = ''

    try {
      const result = await userAuthStore.telegramMiniAppLogin(miniAppInitData.value)
      if (result && result.requiresTotp) {
        enter2FAStep()
        return
      }
      await redirectAfterLogin()
    } catch (err: any) {
      error.value = err.message || t('auth.login.telegramLoginFailed')
    } finally {
      attemptingMiniAppLogin.value = false
    }
  }

  const clearTelegramWidget = () => {
    if (telegramWidgetRef.value) {
      telegramWidgetRef.value.innerHTML = ''
    }
  }

  const startTelegramOidc = async () => {
    error.value = ''
    try {
      sessionStorage.removeItem('tg_oidc_intent')
      const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : ''
      if (redirect) {
        sessionStorage.setItem('tg_oidc_redirect', redirect)
      } else {
        sessionStorage.removeItem('tg_oidc_redirect')
      }
      const res = await userAuthAPI.telegramOidcStart()
      const url = String(res?.data?.data?.auth_url || '')
      if (!url) {
        error.value = t('auth.login.telegramLoginFailed')
        return
      }
      window.location.href = url
    } catch (err: any) {
      error.value = err?.message || t('auth.login.telegramLoginFailed')
    }
  }

  const renderTelegramWidget = () => {
    if (telegramLoginMode.value === 'oidc') {
      clearTelegramWidget()
      return
    }
    if (!showTelegramWidget.value || !telegramWidgetRef.value) {
      clearTelegramWidget()
      return
    }
    clearTelegramWidget()
    const script = document.createElement('script')
    script.async = true
    script.src = 'https://telegram.org/js/telegram-widget.js?22'
    script.setAttribute('data-telegram-login', telegramBotUsername.value)
    script.setAttribute('data-size', 'large')
    script.setAttribute('data-userpic', 'false')
    script.setAttribute('data-request-access', 'write')
    script.setAttribute('data-onauth', `${telegramCallbackName}(user)`)
    script.onerror = () => {
      error.value = t('auth.login.telegramWidgetLoadFailed')
    }
    telegramWidgetRef.value.appendChild(script)
  }

  onMounted(async () => {
    await appStore.loadConfig(true)
    const win = window as Window & Record<string, any>
    win[telegramCallbackName] = handleTelegramAuth
    renderTelegramWidget()

    const resumeTelegram2FA = route.query.tg2fa === '1' && userAuthStore.challengeToken
    const resumeGoogle2FA = shouldResumeGoogleRedirect2FA(
      route.query.google2fa,
      userAuthStore.challengeToken,
    )
    if (resumeTelegram2FA || resumeGoogle2FA) {
      enter2FAStep()
      const nextQuery = { ...route.query }
      delete nextQuery.tg2fa
      delete nextQuery.google2fa
      router.replace({ path: route.path, query: nextQuery })
    }

    const reason = typeof route.query.reason === 'string' ? route.query.reason : ''
    if (reason === 'password_changed') {
      info.value = t('auth.login.passwordChangedTip')
      const nextQuery = { ...route.query }
      delete nextQuery.reason
      router.replace({ path: route.path, query: nextQuery })
    }

    await tryTelegramMiniAppLogin()
  })

  watch([showTelegramWidget, telegramBotUsername], () => {
    renderTelegramWidget()
  })

  watch([isTelegramMiniApp, miniAppInitData], () => {
    void tryTelegramMiniAppLogin()
  })

  onUnmounted(() => {
    const win = window as Window & Record<string, any>
    delete win[telegramCallbackName]
    clearTelegramWidget()
    stopChallengeCountdown()
  })

  return {
    // store
    userAuthStore,
    // brand
    brandSiteName,
    // credentials
    email,
    password,
    showPassword,
    rememberMe,
    // 2FA
    step,
    totpMode,
    totpCode,
    recoveryCode,
    challengeRemainingSeconds,
    handleVerify2FA,
    cancel2FA,
    // messages
    error,
    info,
    // validation
    formValidation,
    // captcha
    loginCaptchaEnabled,
    captchaProvider,
    captchaPayload,
    turnstileToken,
    turnstileSiteKey,
    imageCaptchaRef,
    turnstileRef,
    handleCaptchaConfigStale,
    // flags
    registrationEnabled,
    emailVerificationEnabled,
    // telegram
    showTelegramWidget,
    telegramWidgetRef,
    showTelegramOidc,
    startTelegramOidc,
    showMiniAppLoginHint,
    attemptingMiniAppLogin,
    showTelegramMiniAppEntry,
    openTelegramMiniAppEntry,
    // google
    googleClientID,
    googleButtonLocale,
    googleIdentityUXMode,
    googleRedirectLoginURI,
    prepareGoogleRedirectLogin,
    showGoogleLogin,
    showThirdPartyLogin,
    handleGoogleCredential,
    handleGoogleScriptError,
    // actions
    handleLogin,
  }
}
