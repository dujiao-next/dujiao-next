import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { memberLevelAPI, userOrderAPI, userProfileAPI } from '../api'
import type {
    ChangeEmailPayload,
    ChangeUserPasswordPayload,
    GoogleBindingData,
    PublicMemberLevel,
    SendChangeEmailCodePayload,
    TelegramAuthPayload,
    TelegramBindingData,
    UpdateUserProfilePayload,
    UserLoginLogItem,
    UserProfileData,
} from '../api'
import { useUserAuthStore } from './userAuth'

interface PersonalOrderSummary {
    id: number
    order_no: string
    total_amount?: string
    currency?: string
    status?: string
    created_at?: string
}

const normalizeErrorMessage = (error: unknown, fallback: string) => {
    if (error instanceof Error && error.message.trim() !== '') {
        return error.message
    }
    return fallback
}

export const useUserProfileStore = defineStore('user-profile', () => {
    const userAuthStore = useUserAuthStore()

    const profile = ref<UserProfileData | null>(null)
    const recentOrders = ref<PersonalOrderSummary[]>([])
    const ordersTotal = ref(0)
    const recentLoginLogs = ref<UserLoginLogItem[]>([])
    const telegramBinding = ref<TelegramBindingData | null>(null)
    const googleBinding = ref<GoogleBindingData | null>(null)
    const memberLevels = ref<PublicMemberLevel[]>([])

    const loadingProfile = ref(false)
    const savingProfile = ref(false)
    const loadingOrders = ref(false)
    const loadingLoginLogs = ref(false)
    const loadingTelegramBinding = ref(false)
    const bindingTelegram = ref(false)
    const unbindingTelegram = ref(false)
    const loadingGoogleBinding = ref(false)
    const bindingGoogle = ref(false)
    const unbindingGoogle = ref(false)
    const sendingCode = ref(false)
    const changingEmail = ref(false)
    const changingPassword = ref(false)

    const profileError = ref('')
    const securityError = ref('')

    const displayName = computed(() => {
        if (profile.value?.nickname && profile.value.nickname.trim() !== '') {
            return profile.value.nickname
        }
        return profile.value?.email || '-'
    })

    const currentLevel = computed(() => {
        const levelId = profile.value?.member_level_id
        if (!levelId || memberLevels.value.length === 0) return null
        return memberLevels.value.find((l) => l.id === levelId) || null
    })

    const nextLevel = computed(() => {
        const sorted = [...memberLevels.value].sort((a, b) => a.sort_order - b.sort_order)
        if (!currentLevel.value) {
            return sorted.length > 0 ? sorted[0] : null
        }
        const idx = sorted.findIndex((l) => l.id === currentLevel.value!.id)
        if (idx < 0 || idx >= sorted.length - 1) return null
        return sorted[idx + 1]
    })

    const upgradeProgress = computed(() => {
        const next = nextLevel.value
        if (!next) return null
        const recharged = Number(profile.value?.total_recharged || 0)
        const spent = Number(profile.value?.total_spent || 0)
        const rechargeThreshold = next.recharge_threshold
        const spendThreshold = next.spend_threshold
        return {
            rechargePercent: rechargeThreshold > 0 ? Math.min(100, (recharged / rechargeThreshold) * 100) : null,
            spendPercent: spendThreshold > 0 ? Math.min(100, (spent / spendThreshold) * 100) : null,
            recharged,
            spent,
            rechargeThreshold,
            spendThreshold,
        }
    })

    // 当前用户的专属一口价（口径与下单一致：命中后不再叠加批发价、活动价与等级价）
    // product_* / sku_* 仅用于个人中心展示，商品已删除时这些字段缺省。
    const myProductPrices = ref<Array<{
        product_id: number
        sku_id: number
        price_amount: string | number
        product_slug?: string
        product_title?: Record<string, string>
        product_active?: boolean
        sku_code?: string
        sku_spec_values?: Record<string, unknown>
    }>>([])
    let myProductPricesLoadedFor = 0

    const loadMyProductPrices = async (force = false) => {
        const userId = Number(userAuthStore.user?.id || 0)
        if (!userAuthStore.isAuthenticated || userId <= 0) {
            myProductPrices.value = []
            myProductPricesLoadedFor = 0
            return
        }
        if (!force && myProductPricesLoadedFor === userId) return
        if (myProductPricesLoadedFor !== userId) myProductPrices.value = []
        myProductPricesLoadedFor = userId
        try {
            const response = await userProfileAPI.productPrices()
            myProductPrices.value = Array.isArray(response.data.data) ? response.data.data : []
        } catch {
            myProductPrices.value = []
            myProductPricesLoadedFor = 0
        }
    }

    const loadMemberLevels = async () => {
        try {
            const response = await memberLevelAPI.list()
            memberLevels.value = Array.isArray(response.data.data) ? response.data.data : []
        } catch {
            memberLevels.value = []
        }
    }

    const clearProfileError = () => {
        profileError.value = ''
    }

    const clearSecurityError = () => {
        securityError.value = ''
    }

    const loadProfile = async () => {
        loadingProfile.value = true
        clearProfileError()
        try {
            const response = await userProfileAPI.current()
            const data = response.data.data
            profile.value = data
            userAuthStore.syncUserProfile(data)
            return true
        } catch (error) {
            profile.value = null
            profileError.value = normalizeErrorMessage(error, '加载个人资料失败')
            return false
        } finally {
            loadingProfile.value = false
        }
    }

    const saveProfile = async (payload: UpdateUserProfilePayload) => {
        savingProfile.value = true
        clearProfileError()
        try {
            const response = await userProfileAPI.updateProfile(payload)
            const data = response.data.data
            profile.value = data
            userAuthStore.syncUserProfile(data)
            return true
        } catch (error) {
            profileError.value = normalizeErrorMessage(error, '保存个人资料失败')
            return false
        } finally {
            savingProfile.value = false
        }
    }

    const sendChangeEmailCode = async (payload: SendChangeEmailCodePayload) => {
        sendingCode.value = true
        clearSecurityError()
        try {
            await userProfileAPI.sendChangeEmailCode(payload)
            return true
        } catch (error) {
            securityError.value = normalizeErrorMessage(error, '发送验证码失败')
            return false
        } finally {
            sendingCode.value = false
        }
    }

    const changeEmail = async (payload: ChangeEmailPayload) => {
        changingEmail.value = true
        clearSecurityError()
        try {
            const response = await userProfileAPI.changeEmail(payload)
            const data = response.data.data
            profile.value = data
            userAuthStore.syncUserProfile(data)
            return true
        } catch (error) {
            securityError.value = normalizeErrorMessage(error, '更换邮箱失败')
            return false
        } finally {
            changingEmail.value = false
        }
    }

    const changePassword = async (payload: ChangeUserPasswordPayload) => {
        changingPassword.value = true
        clearSecurityError()
        try {
            await userProfileAPI.changePassword(payload)
            return true
        } catch (error) {
            securityError.value = normalizeErrorMessage(error, '修改密码失败')
            return false
        } finally {
            changingPassword.value = false
        }
    }

    const loadRecentOrders = async (limit = 5) => {
        loadingOrders.value = true
        try {
            const response = await userOrderAPI.list({ page: 1, page_size: limit })
            const data = response.data.data
            recentOrders.value = Array.isArray(data) ? (data as PersonalOrderSummary[]) : []
            ordersTotal.value = response.data.pagination?.total ?? recentOrders.value.length
            return true
        } catch {
            recentOrders.value = []
            ordersTotal.value = 0
            return false
        } finally {
            loadingOrders.value = false
        }
    }


    const loadRecentLoginLogs = async (limit = 5) => {
        loadingLoginLogs.value = true
        try {
            const response = await userProfileAPI.loginLogs({ page: 1, page_size: limit })
            const data = response.data.data
            recentLoginLogs.value = Array.isArray(data) ? (data as UserLoginLogItem[]) : []
            return true
        } catch {
            recentLoginLogs.value = []
            return false
        } finally {
            loadingLoginLogs.value = false
        }
    }

    const loadTelegramBinding = async () => {
        loadingTelegramBinding.value = true
        clearSecurityError()
        try {
            const response = await userProfileAPI.getTelegramBinding()
            telegramBinding.value = response.data.data || { bound: false }
            return true
        } catch (error) {
            telegramBinding.value = null
            securityError.value = normalizeErrorMessage(error, '加载 Telegram 绑定信息失败')
            return false
        } finally {
            loadingTelegramBinding.value = false
        }
    }

    const bindTelegram = async (payload: TelegramAuthPayload) => {
        bindingTelegram.value = true
        clearSecurityError()
        try {
            const response = await userProfileAPI.bindTelegram(payload)
            telegramBinding.value = response.data.data || { bound: true }
            return true
        } catch (error) {
            securityError.value = normalizeErrorMessage(error, '绑定 Telegram 失败')
            return false
        } finally {
            bindingTelegram.value = false
        }
    }

    const bindTelegramMiniApp = async (initData: string) => {
        bindingTelegram.value = true
        clearSecurityError()
        try {
            const response = await userProfileAPI.bindTelegramMiniApp({ init_data: initData })
            telegramBinding.value = response.data.data || { bound: true }
            return true
        } catch (error) {
            securityError.value = normalizeErrorMessage(error, '绑定 Telegram 失败')
            return false
        } finally {
            bindingTelegram.value = false
        }
    }

    const unbindTelegram = async () => {
        unbindingTelegram.value = true
        clearSecurityError()
        try {
            await userProfileAPI.unbindTelegram()
            telegramBinding.value = { bound: false }
            return true
        } catch (error) {
            securityError.value = normalizeErrorMessage(error, '解绑 Telegram 失败')
            return false
        } finally {
            unbindingTelegram.value = false
        }
    }

    const loadGoogleBinding = async () => {
        loadingGoogleBinding.value = true
        clearSecurityError()
        try {
            const response = await userProfileAPI.getGoogleBinding()
            googleBinding.value = response.data.data || { bound: false }
            return true
        } catch (error) {
            googleBinding.value = null
            securityError.value = normalizeErrorMessage(error, '加载 Google 绑定信息失败')
            return false
        } finally {
            loadingGoogleBinding.value = false
        }
    }

    const bindGoogle = async (credential: string) => {
        bindingGoogle.value = true
        clearSecurityError()
        try {
            const response = await userProfileAPI.bindGoogle({ credential })
            googleBinding.value = response.data.data || { bound: true }
            return true
        } catch (error) {
            securityError.value = normalizeErrorMessage(error, '绑定 Google 失败')
            return false
        } finally {
            bindingGoogle.value = false
        }
    }

    const exchangeGoogleRedirectBind = async () => {
        bindingGoogle.value = true
        clearSecurityError()
        try {
            const response = await userProfileAPI.googleRedirectBindExchange()
            googleBinding.value = response.data.data || { bound: true }
            return true
        } catch (error) {
            securityError.value = normalizeErrorMessage(error, '绑定 Google 失败')
            return false
        } finally {
            bindingGoogle.value = false
        }
    }

    const unbindGoogle = async () => {
        unbindingGoogle.value = true
        clearSecurityError()
        try {
            await userProfileAPI.unbindGoogle()
            googleBinding.value = { bound: false }
            return true
        } catch (error) {
            securityError.value = normalizeErrorMessage(error, '解绑 Google 失败')
            return false
        } finally {
            unbindingGoogle.value = false
        }
    }

    return {
        profile,
        recentOrders,
        ordersTotal,
        recentLoginLogs,
        telegramBinding,
        googleBinding,
        memberLevels,
        currentLevel,
        nextLevel,
        upgradeProgress,
        loadingProfile,
        savingProfile,
        loadingOrders,
        loadingLoginLogs,
        loadingTelegramBinding,
        bindingTelegram,
        unbindingTelegram,
        loadingGoogleBinding,
        bindingGoogle,
        unbindingGoogle,
        sendingCode,
        changingEmail,
        changingPassword,
        profileError,
        securityError,
        displayName,
        clearProfileError,
        clearSecurityError,
        loadProfile,
        saveProfile,
        sendChangeEmailCode,
        changeEmail,
        changePassword,
        loadRecentOrders,
        loadMemberLevels,
        myProductPrices,
        loadMyProductPrices,
        loadRecentLoginLogs,
        loadTelegramBinding,
        bindTelegram,
        bindTelegramMiniApp,
        unbindTelegram,
        loadGoogleBinding,
        bindGoogle,
        exchangeGoogleRedirectBind,
        unbindGoogle,
    }
})
