import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { userAuthAPI } from '../api'

export const useUserAuthStore = defineStore('user-auth', () => {
    const router = useRouter()

    const token = ref<string>(localStorage.getItem('user_token') || '')
    const storedUser = localStorage.getItem('user_profile')
    let parsedUser = null
    try {
        if (storedUser && storedUser !== 'undefined') {
            parsedUser = JSON.parse(storedUser)
        }
    } catch (e) {
        console.error('Failed to parse user profile from localStorage', e)
        localStorage.removeItem('user_profile')
    }
    const user = ref<any>(parsedUser)
    const loading = ref(false)

    // 2FA 挑战中间状态（不持久化到 localStorage）
    const challengeToken = ref<string>('')
    const challengeExpiresAt = ref<string>('')

    const isAuthenticated = computed(() => !!token.value)

    const setToken = (newToken: string) => {
        token.value = newToken
        localStorage.setItem('user_token', newToken)
    }

    const setUser = (newUser: any) => {
        user.value = newUser
        localStorage.setItem('user_profile', JSON.stringify(newUser))
    }

    const clearAuth = () => {
        token.value = ''
        user.value = null
        localStorage.removeItem('user_token')
        localStorage.removeItem('user_profile')
    }

    const sendVerifyCode = async (payload: any) => {
        loading.value = true
        try {
            await userAuthAPI.sendVerifyCode(payload)
            return true
        } finally {
            loading.value = false
        }
    }

    const register = async (payload: any) => {
        loading.value = true
        try {
            const response = await userAuthAPI.register(payload)
            const { token: accessToken, user: userData } = response.data.data
            if (accessToken) {
                setToken(accessToken)
            }
            if (userData) {
                setUser(userData)
            }
            return true
        } finally {
            loading.value = false
        }
    }

    const clearChallenge = () => {
        challengeToken.value = ''
        challengeExpiresAt.value = ''
    }

    const login = async (payload: any) => {
        loading.value = true
        try {
            const response = await userAuthAPI.login(payload)
            return handleLoginResponse(response.data.data)
        } finally {
            loading.value = false
        }
    }

    const verify2FA = async (payload: { code?: string; recovery_code?: string }) => {
        if (!challengeToken.value) {
            throw new Error('challenge_token_missing')
        }
        loading.value = true
        try {
            const response = await userAuthAPI.verify2FA({
                challenge_token: challengeToken.value,
                ...payload,
            })
            const data = response.data.data || {}
            const { token: accessToken, user: userData } = data
            setToken(accessToken)
            setUser(userData)
            clearChallenge()
            return true
        } finally {
            loading.value = false
        }
    }

    const handleLoginResponse = (data: any): { requiresTotp: boolean } => {
        if (data?.requires_totp) {
            challengeToken.value = data.challenge_token || ''
            challengeExpiresAt.value = data.challenge_expires_at || ''
            return { requiresTotp: true }
        }
        clearChallenge()
        const { token: accessToken, user: userData } = data || {}
        setToken(accessToken)
        setUser(userData)
        return { requiresTotp: false }
    }

    const telegramLogin = async (payload: any) => {
        loading.value = true
        try {
            const response = await userAuthAPI.telegramLogin(payload)
            return handleLoginResponse(response.data.data)
        } finally {
            loading.value = false
        }
    }

    const telegramOidcLogin = async (payload: { code: string; state: string }) => {
        loading.value = true
        try {
            const response = await userAuthAPI.telegramOidcCallback(payload)
            return handleLoginResponse(response.data.data)
        } finally {
            loading.value = false
        }
    }

    const telegramMiniAppLogin = async (initData: string) => {
        loading.value = true
        try {
            const response = await userAuthAPI.telegramMiniAppLogin({ init_data: initData })
            return handleLoginResponse(response.data.data)
        } finally {
            loading.value = false
        }
    }

    const googleLogin = async (credential: string) => {
        loading.value = true
        try {
            const response = await userAuthAPI.googleLogin({ credential })
            return handleLoginResponse(response.data.data)
        } finally {
            loading.value = false
        }
    }

    const googleRedirectLogin = async () => {
        loading.value = true
        try {
            const response = await userAuthAPI.googleRedirectExchange()
            return handleLoginResponse(response.data.data)
        } finally {
            loading.value = false
        }
    }

    const forgotPassword = async (payload: any) => {
        loading.value = true
        try {
            await userAuthAPI.forgotPassword(payload)
            return true
        } finally {
            loading.value = false
        }
    }

    const syncUserProfile = (profile: {
        id?: number
        email?: string
        nickname?: string
        locale?: string
        email_verified_at?: string | null
        email_change_mode?: 'bind_only' | 'change_with_old_and_new'
        password_change_mode?: 'set_without_old' | 'change_with_old'
    }) => {
        const nextUser = { ...(user.value || {}), ...profile }
        setUser(nextUser)
    }

    const acceptOAuthLogin = (data: any) => {
        if (data?.token) setToken(String(data.token))
        if (data?.user) setUser(data.user)
        if (data?.requires_totp && data?.challenge_token) {
            challengeToken.value = String(data.challenge_token)
            challengeExpiresAt.value = String(data.challenge_expires_at || '')
        }
    }

    const logout = (redirect = '/auth/login') => {
        clearAuth()
        router.push(redirect)
    }

    return {
        token,
        user,
        loading,
        challengeToken,
        challengeExpiresAt,
        isAuthenticated,
        sendVerifyCode,
        register,
        login,
        verify2FA,
        clearChallenge,
        telegramLogin,
        telegramOidcLogin,
        telegramMiniAppLogin,
        googleLogin,
        googleRedirectLogin,
        acceptOAuthLogin,
        forgotPassword,
        syncUserProfile,
        logout,
    }
})
