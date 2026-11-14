import { userApi } from './client'
import type {
    UpdateUserProfilePayload,
    SendChangeEmailCodePayload,
    ChangeEmailPayload,
    ChangeUserPasswordPayload,
    GoogleCredentialPayload,
    TelegramAuthPayload,
    TelegramMiniAppAuthPayload,
} from './types'
import { GOOGLE_REDIRECT_API_PATHS } from '../utils/googleRedirect'

export const userProfileAPI = {
    current: () => userApi.get('/me'),
    productPrices: () => userApi.get('/me/product-prices'),
    loginLogs: (params?: any) => userApi.get('/me/login-logs', { params }),
    updateProfile: (data: UpdateUserProfilePayload) => userApi.put('/me/profile', data),
    sendChangeEmailCode: (data: SendChangeEmailCodePayload) => userApi.post('/me/email/send-verify-code', data),
    changeEmail: (data: ChangeEmailPayload) => userApi.post('/me/email/change', data),
    changePassword: (data: ChangeUserPasswordPayload) => userApi.put('/me/password', data),
    getTelegramBinding: () => userApi.get('/me/telegram'),
    bindTelegram: (data: TelegramAuthPayload) => userApi.post('/me/telegram/bind', data),
    bindTelegramMiniApp: (data: TelegramMiniAppAuthPayload) =>
        userApi.post('/me/telegram/miniapp/bind', data),
    telegramOidcBindStart: () => userApi.get('/me/telegram/oidc/start'),
    telegramOidcBindCallback: (data: { code: string; state: string }) =>
        userApi.post('/me/telegram/oidc/callback', data),
    unbindTelegram: () => userApi.delete('/me/telegram/unbind'),
    getGoogleBinding: () => userApi.get('/me/google'),
    bindGoogle: (data: GoogleCredentialPayload) => userApi.post('/me/google/bind', data),
    googleRedirectBindIntent: () =>
        userApi.post(GOOGLE_REDIRECT_API_PATHS.bindIntent, {}, { credentials: 'include' }),
    googleRedirectBindExchange: () =>
        userApi.post(GOOGLE_REDIRECT_API_PATHS.bindExchange, {}, { credentials: 'include' }),
    unbindGoogle: () => userApi.delete('/me/google/unbind'),
}
