import { api, userApi } from './client'
import type { CaptchaPayload, GoogleCredentialPayload, TelegramAuthPayload, TelegramMiniAppAuthPayload } from './types'
import { GOOGLE_REDIRECT_API_PATHS } from '../utils/googleRedirect'

export type SSOCaptchaAction = 'login' | 'register-send-code' | 'register'
export interface SSOCaptchaProof { challenge: string; answer: string }
export interface SSOCaptchaChallenge {
    required: boolean
    challenge?: string
    type?: 'image' | 'turnstile'
    image_base64?: string
    site_key?: string
    expires_in?: number
}
export interface SSOAuthProofs {
    captcha?: SSOCaptchaProof
    captcha_payload?: CaptchaPayload
}

export const userAuthAPI = {
    sendVerifyCode: (data: any) => userApi.post('/auth/send-verify-code', data),
    register: (data: any) => userApi.post('/auth/register', data),
    login: (data: any) => userApi.post('/auth/login', data),
    verify2FA: (data: { challenge_token: string; code?: string; recovery_code?: string }) =>
        userApi.post('/auth/login/verify-2fa', data),
    telegramLogin: (data: TelegramAuthPayload) => userApi.post('/auth/telegram/login', data),
    telegramMiniAppLogin: (data: TelegramMiniAppAuthPayload) =>
        userApi.post('/auth/telegram/miniapp/login', data),
    telegramOidcStart: () => userApi.get('/auth/telegram/oidc/start'),
    telegramOidcCallback: (data: { code: string; state: string }) =>
        userApi.post('/auth/telegram/oidc/callback', data),
    googleLogin: (data: GoogleCredentialPayload) => userApi.post('/auth/google/login', data),
    ssoCaptcha: (data: { action: SSOCaptchaAction; account: string }) => userApi.post('/auth/sso/captcha', data),
    ssoPasswordLogin: (data: { email: string; password: string } & SSOAuthProofs) =>
        userApi.post('/auth/sso/password-login', data),
    ssoMfa: (data: { challenge: string; mfa_type: string; passcode: string }) =>
        userApi.post('/auth/sso/mfa', data),
    ssoRegisterSendCode: (data: { email: string } & SSOAuthProofs) =>
        userApi.post('/auth/sso/register/send-code', data),
    ssoRegister: (data: { email: string; password: string; code: string; display_name?: string } & SSOAuthProofs) =>
        userApi.post('/auth/sso/register', data),
    googleRedirectIntent: () =>
        userApi.post(GOOGLE_REDIRECT_API_PATHS.loginIntent, {}, { credentials: 'include' }),
    googleRedirectExchange: () =>
        userApi.post(GOOGLE_REDIRECT_API_PATHS.loginExchange, {}, { credentials: 'include' }),
    completeInviteRegistration: (data: { invite_ticket: string; invite_code: string }) =>
        userApi.post('/auth/invite/complete', data),
    forgotPassword: (data: any) => userApi.post('/auth/forgot-password', data),
}

export const userTotpAPI = {
    status: () => userApi.get('/me/2fa/status'),
    setup: () => userApi.post('/me/2fa/setup', {}),
    enable: (data: { code: string }) => userApi.post('/me/2fa/enable', data),
    disable: (data: { code?: string; recovery_code?: string }) => userApi.post('/me/2fa/disable', data),
    regenerateRecoveryCodes: (data: { code: string }) =>
        userApi.post('/me/2fa/recovery-codes/regenerate', data),
}

export const captchaAPI = {
    image: () => api.get('/public/captcha/image'),
}

export const configAPI = {
    get: () => api.get('/public/config'),
}
