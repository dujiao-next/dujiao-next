import { api, userApi } from './client'
import type { GoogleCredentialPayload, TelegramAuthPayload, TelegramMiniAppAuthPayload } from './types'
import { GOOGLE_REDIRECT_API_PATHS } from '../utils/googleRedirect'

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
