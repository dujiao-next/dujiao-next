import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useUserAuthStore } from '../stores/userAuth'
import { useI18n } from 'vue-i18n'
import { debounceAsync } from '../utils/debounce'
import { useAppStore } from '../stores/app'
import type { CaptchaPayload } from '../api'
import ImageCaptcha from '../components/captcha/ImageCaptcha.vue'
import TurnstileCaptcha from '../components/captcha/TurnstileCaptcha.vue'

/**
 * 找回密码页共享逻辑（classic + vault 双模板共用）。
 * 完整保留原 views/auth/Forgot.vue 的行为，仅抽离为 composable。
 */
export function useForgot() {
  const router = useRouter()
  const userAuthStore = useUserAuthStore()
  const appStore = useAppStore()
  const { t } = useI18n()

  const brandSiteName = computed(() => {
    const siteName = String(appStore.config?.brand?.site_name || '').trim()
    return siteName !== '' ? siteName : 'Dovelora'
  })

  const emailVerificationEnabled = computed(() => appStore.config?.email_verification_enabled !== false)

  const email = ref('')
  const code = ref('')
  const newPassword = ref('')
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
  const sendCodeCaptchaEnabled = computed(() => !!captchaConfig.value?.scenes?.reset_send_code && captchaProvider.value !== 'none')
  const turnstileSiteKey = computed(() => String(captchaConfig.value?.turnstile?.site_key || ''))

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
    if (!email.value) {
      error.value = t('auth.forgot.errors.emailRequired')
      return
    }
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
        email: email.value,
        purpose: 'reset',
        captcha_payload: getCaptchaPayload(),
      })
      startCountdown()
    } catch (err: any) {
      error.value = err.message || t('auth.forgot.errors.sendCodeFailed')
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

  const performReset = async () => {
    error.value = ''
    if (!email.value || !code.value || !newPassword.value) return
    try {
      await userAuthStore.forgotPassword({
        email: email.value,
        code: code.value,
        new_password: newPassword.value
      })
      router.push('/auth/login')
    } catch (err: any) {
      error.value = err.message || t('auth.forgot.errors.resetFailed')
    }
  }

  const handleSendCode = debounceAsync(performSendCode, 200)
  const handleReset = debounceAsync(performReset, 200)

  onMounted(async () => {
    await appStore.loadConfig(true)
  })

  return {
    userAuthStore,
    brandSiteName,
    emailVerificationEnabled,
    email,
    code,
    newPassword,
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
    handleCaptchaConfigStale,
    handleSendCode,
    handleReset,
  }
}
