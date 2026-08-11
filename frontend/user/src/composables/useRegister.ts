import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useUserAuthStore } from '../stores/userAuth'
import { useI18n } from 'vue-i18n'
import { debounceAsync } from '../utils/debounce'
import { useAppStore } from '../stores/app'
import type { CaptchaPayload } from '../api'
import ImageCaptcha from '../components/captcha/ImageCaptcha.vue'
import TurnstileCaptcha from '../components/captcha/TurnstileCaptcha.vue'
import { useFormValidation, getPasswordStrength } from './useFormValidation'

/**
 * 用户注册页共享逻辑（classic + vault 双模板共用）。
 * 完整保留原 views/auth/Register.vue 的行为，仅抽离为 composable。
 */
export function useRegister() {
  const router = useRouter()
  const userAuthStore = useUserAuthStore()
  const appStore = useAppStore()
  const { t } = useI18n()

  const brandSiteName = computed(() => {
    const siteName = String(appStore.config?.brand?.site_name || '').trim()
    return siteName !== '' ? siteName : 'Dovelora'
  })

  const email = ref('')
  const emailLocalPart = ref('')
  const selectedEmailDomain = ref('')
  const password = ref('')
  const showPassword = ref(false)
  const code = ref('')
  const agreed = ref(false)

  const passwordStrength = computed(() => getPasswordStrength(password.value))
  const error = ref('')
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
  const registrationEnabled = computed(() => appStore.config?.registration_enabled !== false)
  const emailVerificationEnabled = computed(() => appStore.config?.email_verification_enabled !== false)
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
  const emailDomainSelectionRequired = computed(() => emailDomainAllowlistEnabled.value && allowedEmailDomains.value.length > 0)

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

  const performSendCode = async () => {
    error.value = ''
    const currentEmail = registrationEmail.value
    if (!currentEmail) {
      error.value = t('auth.register.errors.emailRequired')
      return
    }
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
      await userAuthStore.sendVerifyCode({
        email: currentEmail,
        purpose: 'register',
        captcha_payload: getCaptchaPayload(),
      })
      startCountdown()
    } catch (err: any) {
      error.value = err.message || t('auth.register.errors.sendCodeFailed')
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
    if (!agreed.value) {
      error.value = t('auth.register.errors.agreementRequired')
      return
    }
    try {
      await userAuthStore.register({
        email: currentEmail,
        password: password.value,
        code: emailVerificationEnabled.value ? code.value : '',
        agreement_accepted: agreed.value,
      })
      router.push('/me/orders')
    } catch (err: any) {
      error.value = err.message || t('auth.register.errors.registerFailed')
    }
  }

  const handleSendCode = debounceAsync(performSendCode, 200)
  const handleRegister = debounceAsync(performRegister, 200)

  onMounted(async () => {
    await appStore.loadConfig(true)
  })

  return {
    userAuthStore,
    brandSiteName,
    email,
    emailLocalPart,
    selectedEmailDomain,
    password,
    showPassword,
    code,
    agreed,
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
