import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useUserAuthStore } from '../stores/userAuth'
import { userAuthAPI } from '../api'
import { useI18n } from 'vue-i18n'
import { useFeedback } from './useFeedback'
import { debounceAsync } from '../utils/debounce'
import { useAppStore } from '../stores/app'
import type { CaptchaPayload } from '../api'
import ImageCaptcha from '../components/captcha/ImageCaptcha.vue'
import TurnstileCaptcha from '../components/captcha/TurnstileCaptcha.vue'
import { useFormValidation, getPasswordStrength } from './useFormValidation'
import { useSSOCredentials } from '../utils/unifiedAuth'
import { useSSOCaptcha } from './useSSOCaptcha'
import type { SSOCaptchaAction } from '../api/auth'

/**
 * 用户注册页共享逻辑（classic + vault 双模板共用）。
 * 完整保留原 views/auth/Register.vue 的行为，仅抽离为 composable。
 */
export function useRegister() {
  const router = useRouter()
  const route = useRoute()
  const userAuthStore = useUserAuthStore()
  const appStore = useAppStore()
  const { t } = useI18n()
  const { success: notifySuccess } = useFeedback()

  const brandSiteName = computed(() => {
    const siteName = String(appStore.config?.brand?.site_name || '').trim()
    return siteName !== '' ? siteName : 'Dujiao-Next'
  })

  const email = ref('')
  const emailLocalPart = ref('')
  const selectedEmailDomain = ref('')
  const password = ref('')
  const showPassword = ref(false)
  const code = ref('')
  const agreed = ref(false)
  // 支持站长分发 /auth/register?invite=CODE 预填邀请码
  const inviteCode = ref(typeof route.query.invite === 'string' ? route.query.invite.trim() : '')

  const passwordStrength = computed(() => getPasswordStrength(password.value))
  const error = ref('')
  const ssoCaptcha = useSSOCaptcha()
  const ssoCaptchaChallenge = ssoCaptcha.challenge
  const ssoCaptchaAnswer = ssoCaptcha.answer
  let ssoCaptchaAction: SSOCaptchaAction = 'register-send-code'
  const refreshSSOCaptcha = async () => {
    try { await ssoCaptcha.refresh(ssoCaptchaAction, registrationEmail.value) } catch (err: any) { error.value = err?.message || t('auth.register.errors.registerFailed') }
  }
  const sending = ref(false)
  const countdown = ref(0)
  const captchaPayload = ref<CaptchaPayload>({})
  const turnstileToken = ref('')
  const imageCaptchaRef = ref<InstanceType<typeof ImageCaptcha> | null>(null)
  const turnstileRef = ref<InstanceType<typeof TurnstileCaptcha> | null>(null)
  let timer: number | undefined

  const captchaConfig = computed(() => appStore.config?.captcha || null)
  const captchaProvider = computed(() => String(captchaConfig.value?.provider || 'none'))
  const sendCodeCaptchaEnabled = computed(() => !!captchaConfig.value?.scenes?.register_send_code && captchaProvider.value !== 'none')
  const turnstileSiteKey = computed(() => String(captchaConfig.value?.turnstile?.site_key || ''))
  const ssoOnlyMode = computed(() => useSSOCredentials(appStore.config?.sso_auth, undefined))
  const registrationEnabled = computed(() => ssoOnlyMode.value || appStore.config?.registration_enabled !== false)
  const emailVerificationEnabled = computed(() => ssoOnlyMode.value || appStore.config?.email_verification_enabled !== false)
  const inviteOnlyEnabled = computed(() => appStore.config?.invite_only_enabled === true)
  const loginLink = computed(() => ({
    path: '/auth/login',
    query: inviteCode.value ? { invite: inviteCode.value } : {},
  }))
  const emailDomainAllowlistEnabled = computed(() => appStore.config?.email_domain_allowlist_enabled === true)
  const allowedEmailDomains = computed(() => {
    const raw = appStore.config?.allowed_email_domains
    if (!Array.isArray(raw)) return []

    const seen = new Set<string>()
    const domains: string[] = []
    raw
      .map((item) => String(item || '').trim().replace(/^@+/, '').toLowerCase())
      .filter(Boolean)
      .forEach((domain) => {
        if (seen.has(domain)) return
        seen.add(domain)
        domains.push(domain)
      })
    return domains
  })
  const allowedEmailDomainsText = computed(() => allowedEmailDomains.value.join(', '))
  const emailDomainSelectionRequired = computed(() => !ssoOnlyMode.value && emailDomainAllowlistEnabled.value && allowedEmailDomains.value.length > 0)

  watch(allowedEmailDomains, (domains) => {
    if (domains.length === 0) {
      selectedEmailDomain.value = ''
      return
    }
    if (!domains.includes(selectedEmailDomain.value)) {
      selectedEmailDomain.value = domains[0] || ''
    }
  }, { immediate: true })

  const registrationEmail = computed(() => {
    if (!emailDomainSelectionRequired.value) return email.value.trim()
    const localPart = emailLocalPart.value.trim()
    const domain = selectedEmailDomain.value.trim()
    if (!localPart || !domain) return ''
    return `${localPart}@${domain}`
  })

  const getEmailDomain = (value: string): string => {
    const normalized = value.trim().toLowerCase()
    const at = normalized.lastIndexOf('@')
    if (at <= 0 || at === normalized.length - 1) return ''
    return normalized.slice(at + 1)
  }

  const emailDomainRule = (value: string): string | null => {
    // SSO 模式下注册策略以 IdP 为准,本站域名白名单不适用
    if (ssoOnlyMode.value) return null
    if (!emailDomainAllowlistEnabled.value) return null
    const domain = getEmailDomain(value)
    if (!domain) return null
    if (allowedEmailDomains.value.length === 0) {
      return t('auth.register.errors.emailDomainUnavailable')
    }
    if (allowedEmailDomains.value.includes(domain)) return null
    return t('auth.register.errors.emailDomainNotAllowed', { domains: allowedEmailDomainsText.value })
  }

  const touchRegistrationEmail = () => {
    formValidation.touchField('email', registrationEmail.value)
  }

  const formValidation = useFormValidation(['email', 'password'])
  formValidation.addRule('email', formValidation.requiredRule())
  formValidation.addRule('email', formValidation.emailRule())
  formValidation.addRule('email', emailDomainRule)
  formValidation.addRule('password', formValidation.requiredRule())
  formValidation.addRule('password', formValidation.minLengthRule(6))

  const startCountdown = () => {
    countdown.value = 60
    timer = window.setInterval(() => {
      countdown.value -= 1
      if (countdown.value <= 0 && timer) {
        clearInterval(timer)
        timer = undefined
      }
    }, 1000)
  }

  const getCaptchaPayload = (): CaptchaPayload | undefined => {
    if (!sendCodeCaptchaEnabled.value) return undefined
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

  const inviteCodeMissing = (): boolean => {
    if (!inviteOnlyEnabled.value || inviteCode.value.trim() !== '') return false
    error.value = t('auth.register.errors.inviteCodeRequired')
    return true
  }

  const performSendCode = async () => {
    error.value = ''
    const currentEmail = registrationEmail.value
    if (!currentEmail) {
      error.value = t('auth.register.errors.emailRequired')
      return
    }
    if (inviteCodeMissing()) return
    touchRegistrationEmail()
    if (formValidation.hasError('email')) return
    if (countdown.value > 0) return

    if (sendCodeCaptchaEnabled.value && captchaProvider.value === 'image') {
      if (!captchaPayload.value.captcha_id || !captchaPayload.value.captcha_code) {
        error.value = t('auth.common.captchaRequired')
        return
      }
    }
    if (sendCodeCaptchaEnabled.value && captchaProvider.value === 'turnstile') {
      if (!turnstileToken.value) {
        error.value = t('auth.common.captchaRequired')
        return
      }
    }

    sending.value = true
    try {
      if (ssoOnlyMode.value) {
        ssoCaptchaAction = 'register-send-code'
        if (!await ssoCaptcha.ensure(ssoCaptchaAction, currentEmail)) return
        await userAuthAPI.ssoRegisterSendCode({ email: currentEmail, captcha: ssoCaptcha.proof(), captcha_payload: getCaptchaPayload() })
        ssoCaptcha.invalidate()
      } else {
        await userAuthStore.sendVerifyCode({
          email: currentEmail,
          purpose: 'register',
          invite_code: inviteOnlyEnabled.value ? inviteCode.value.trim() : undefined,
          captcha_payload: getCaptchaPayload(),
        })
      }
      startCountdown()
      notifySuccess(t('auth.common.codeSent'))
    } catch (err: any) {
      error.value = err.message || t('auth.register.errors.sendCodeFailed')
      if (ssoOnlyMode.value) await refreshSSOCaptcha()
      if (captchaProvider.value === 'image') {
        imageCaptchaRef.value?.refresh()
      }
      if (captchaProvider.value === 'turnstile') {
        turnstileRef.value?.reset()
        turnstileToken.value = ''
      }
    } finally {
      sending.value = false
    }
  }

  const performRegister = async () => {
    error.value = ''
    const currentEmail = registrationEmail.value
    if (!formValidation.validateAll({ email: currentEmail, password: password.value })) return
    if (emailVerificationEnabled.value && !code.value) return
    if (inviteCodeMissing()) return
    if (!agreed.value) {
      error.value = t('auth.register.errors.agreementRequired')
      return
    }
    try {
      if (ssoOnlyMode.value) {
        ssoCaptchaAction = 'register'
        if (!await ssoCaptcha.ensure(ssoCaptchaAction, currentEmail)) return
        await userAuthStore.ssoRegister({
          email: currentEmail,
          password: password.value,
          code: code.value,
          captcha: ssoCaptcha.proof(),
        })
        ssoCaptcha.invalidate()
      } else {
        await userAuthStore.register({
          email: currentEmail,
          password: password.value,
          code: emailVerificationEnabled.value ? code.value : '',
          agreement_accepted: agreed.value,
          invite_code: inviteOnlyEnabled.value ? inviteCode.value.trim() : undefined,
        })
      }
      router.push('/me/orders')
    } catch (err: any) {
      error.value = err.message || t('auth.register.errors.registerFailed')
      if (ssoOnlyMode.value) await refreshSSOCaptcha()
    }
  }

  const handleSendCode = debounceAsync(performSendCode, 200)
  const handleRegister = debounceAsync(performRegister, 200)

  onMounted(async () => {
    await appStore.loadConfig(true)
  })

  return {
    ssoOnlyMode,
    ssoCaptchaChallenge, ssoCaptchaAnswer, refreshSSOCaptcha,
    userAuthStore,
    brandSiteName,
    email,
    emailLocalPart,
    selectedEmailDomain,
    password,
    showPassword,
    code,
    agreed,
    inviteCode,
    inviteOnlyEnabled,
    loginLink,
    passwordStrength,
    error,
    sending,
    countdown,
    captchaPayload,
    turnstileToken,
    imageCaptchaRef,
    turnstileRef,
    captchaProvider,
    sendCodeCaptchaEnabled,
    turnstileSiteKey,
    registrationEnabled,
    emailVerificationEnabled,
    emailDomainAllowlistEnabled,
    allowedEmailDomains,
    allowedEmailDomainsText,
    emailDomainSelectionRequired,
    touchRegistrationEmail,
    formValidation,
    handleCaptchaConfigStale,
    handleSendCode,
    handleRegister,
  }
}
