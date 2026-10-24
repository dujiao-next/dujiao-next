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
import { useSSOCaptcha } from './useSSOCaptcha'
import { isUnifiedAuthOnly, safeIdentityURL, useSSOCredentials } from '../utils/unifiedAuth'

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
    return siteName !== '' ? siteName : 'Dujiao-Next'
  })

  const email = ref('')
  const password = ref('')
  const showPassword = ref(false)
  const rememberMe = ref(true)

  const resumeGoogleRedirect2FAOnMount = shouldResumeGoogleRedirect2FA(
    route.query.google2fa,
    userAuthStore.challengeToken,
  )
  const step = ref<'password' | 'totp' | 'ssoMfa'>(
    resumeGoogleRedirect2FAOnMount ? 'totp' : 'password',
  )
  const ssoMfaChallenge = ref<{ token: string; props: Array<{ mfa_type: string }> } | null>(null)
  const ssoMfaType = ref('')
  const ssoMfaCode = ref('')
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
  const ssoCaptcha = useSSOCaptcha()
  const ssoCaptchaChallenge = ssoCaptcha.challenge
  const ssoCaptchaAnswer = ssoCaptcha.answer
  const refreshSSOCaptcha = async () => {
    try { await ssoCaptcha.refresh('login', email.value) } catch (err: any) { error.value = err?.message || t('auth.login.error') }
  }
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
  const telegramEnabled = computed(() => !unifiedAuthOnly.value && !!telegramConfig.value?.enabled && telegramBotUsername.value !== '')
  const telegramLoginMode = computed(() => String(telegramConfig.value?.mode || '').trim())
  const isWidgetMode = computed(() => telegramLoginMode.value === 'widget' || (telegramLoginMode.value === '' && telegramEnabled.value))
  const ssoConfig = computed(() => appStore.config?.sso_auth || null)
  const unifiedAuthOnly = computed(() => isUnifiedAuthOnly(ssoConfig.value))
  const passwordResetURL = computed(() => safeIdentityURL(ssoConfig.value?.password_reset_url))
  const googleConfig = computed(() => appStore.config?.google_auth || null)
  const googleClientID = computed(() => String(googleConfig.value?.client_id || '').trim())
  const googleEnabled = computed(() => !unifiedAuthOnly.value && !!googleConfig.value?.enabled && googleClientID.value !== '')
  const googleButtonLocale = computed(() => String(appStore.locale || '').trim())
  const googleIdentityUXMode = detectGoogleIdentityUXMode()
  const googleRedirectLoginURI = googleIdentityUXMode === 'redirect'
    ? tryBuildGoogleRedirectCredentialCallbackURL()
    : ''
  const googleRedirectAvailable = googleIdentityUXMode === 'popup' || googleRedirectLoginURI !== ''
  const registrationEnabled = computed(() => ssoOnlyMode.value || appStore.config?.registration_enabled !== false)
  const inviteOnlyEnabled = computed(() => appStore.config?.invite_only_enabled === true)
  // 透传 ?invite=，方便站长分发的邀请链接在登录/注册间切换时不丢失
  const registerLink = computed(() => ({
    path: '/auth/register',
    query: typeof route.query.invite === 'string' ? { invite: route.query.invite } : {},
  }))
  const emailVerificationEnabled = computed(() => appStore.config?.email_verification_enabled !== false)
  const isTelegramUrlEnv = isTelegramUrlEnvironment()
  const isTelegramMiniApp = computed(() => (telegramMiniAppStore.isMiniApp && telegramMiniAppStore.isReady) || isTelegramUrlEnv)
  const miniAppInitData = computed(() => String(telegramMiniAppStore.initData || '').trim())
  const showTelegramWidget = computed(() => isWidgetMode.value && telegramEnabled.value && !isTelegramMiniApp.value)
  const showTelegramOidc = computed(() => telegramLoginMode.value === 'oidc' && telegramEnabled.value && !isTelegramMiniApp.value)
  const showMiniAppLoginHint = computed(() => !unifiedAuthOnly.value && isTelegramMiniApp.value)
  const showGoogleLogin = computed(() => canShowGoogleIdentityButton(
    googleEnabled.value,
    googleClientID.value,
    isTelegramMiniApp.value,
  ) && googleRedirectAvailable)
  // Local display fallback applies only when unified authentication is optional.
  const isLocalEscape = computed(() => !unifiedAuthOnly.value && route.query.local === '1')
  const ssoOnlyMode = computed(() => useSSOCredentials(ssoConfig.value, route.query.local))
  const showThirdPartyLogin = computed(() => hasThirdPartyLoginOption(
    showTelegramWidget.value,
    showTelegramOidc.value,
    showMiniAppLoginHint.value,
    showGoogleLogin.value,
  ))
  const telegramMiniAppEntryLink = computed(() => buildTelegramMiniAppEntryLink(telegramBotUsername.value, telegramMiniAppURL.value))
  const showTelegramMiniAppEntry = computed(() => !unifiedAuthOnly.value && !isTelegramMiniApp.value && telegramMiniAppEntryLink.value !== '')
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
      if (ssoOnlyMode.value) {
        if (!await ssoCaptcha.ensure('login', email.value)) return
        const data = await userAuthStore.ssoPasswordLogin({
          email: email.value,
          password: password.value,
          captcha: ssoCaptcha.proof(),
          captcha_payload: getCaptchaPayload(),
        })
        ssoCaptcha.invalidate()
        if (data?.requires_mfa && data?.mfa_challenge) {
          ssoMfaChallenge.value = data.mfa_challenge
          ssoMfaType.value = data.mfa_challenge.props?.[0]?.mfa_type || 'otp'
          ssoMfaCode.value = ''
          step.value = 'ssoMfa'
          return
        }
        if (data?.requires_totp) {
          enter2FAStep()
          return
        }
        await redirectAfterLogin()
        return
      }
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
      if (ssoOnlyMode.value) await refreshSSOCaptcha()
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

  // 第三方登录 / 邀请码补填后的统一后续：需邀请码时由 InviteCodeDialog 接管（store.inviteTicket 已置位）
  const continueAfterLogin = async (result: { requiresTotp: boolean; inviteRequired?: boolean }) => {
    if (result.inviteRequired) return
    if (result.requiresTotp) {
      enter2FAStep()
      return
    }
    await redirectAfterLogin()
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
      await continueAfterLogin(result)
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
      await continueAfterLogin(result)
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
    if (unifiedAuthOnly.value || !isTelegramMiniApp.value || miniAppInitData.value === '' || miniAppLoginAttempted.value || attemptingMiniAppLogin.value) {
      return
    }

    miniAppLoginAttempted.value = true
    attemptingMiniAppLogin.value = true
    error.value = ''

    try {
      // 邀请制新用户：同一 initData 不能重放，只能走 InviteCodeDialog 的票据流程
      const result = await userAuthStore.telegramMiniAppLogin(miniAppInitData.value)
      await continueAfterLogin(result)
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

  const performSsoMfa = async () => {
    if (!ssoMfaChallenge.value) return
    error.value = ''
    if (!ssoMfaCode.value.trim()) {
      error.value = t('auth.login.ssoMfa.codeRequired')
      return
    }
    try {
      await userAuthStore.ssoMfaLogin({
        challenge: ssoMfaChallenge.value.token,
        mfa_type: ssoMfaType.value,
        passcode: ssoMfaCode.value.trim(),
      })
      await redirectAfterLogin()
    } catch (err: any) {
      error.value = err?.message || t('auth.login.ssoMfa.failed')
    }
  }

  const cancelSsoMfa = () => {
    step.value = 'password'
    ssoMfaChallenge.value = null
    ssoMfaCode.value = ''
    error.value = ''
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


    const resumeTelegram2FA = !unifiedAuthOnly.value && route.query.tg2fa === '1' && userAuthStore.challengeToken
    const resumeGoogle2FA = !unifiedAuthOnly.value && shouldResumeGoogleRedirect2FA(
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
    inviteOnlyEnabled,
    registerLink,
    continueAfterLogin,
    // telegram
    showTelegramWidget,
    telegramWidgetRef,
    showTelegramOidc,
    startTelegramOidc,
    ssoOnlyMode, isLocalEscape, passwordResetURL,
    ssoCaptchaChallenge, ssoCaptchaAnswer, refreshSSOCaptcha,
    ssoMfaChallenge, ssoMfaType, ssoMfaCode, performSsoMfa, cancelSsoMfa,
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
