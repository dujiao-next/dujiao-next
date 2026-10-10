// Re-export client instances and ApiResponse
export { api, userApi } from './client'
export type { ApiResponse } from './client'

// Re-export all types
export type {
    UserProfileData,
    PublicMemberLevel,
    UpdateUserProfilePayload,
    UserLoginLogItem,
    SendChangeEmailCodePayload,
    ChangeEmailPayload,
    ChangeUserPasswordPayload,
    TelegramAuthPayload,
    TelegramMiniAppAuthPayload,
    TelegramBindingData,
    GoogleCredentialPayload,
    GoogleBindingData,
    WalletAccountData,
    WalletTransactionData,
    WalletRechargePayload,
    WalletRechargeOrderData,
    WalletRechargeResult,
    GiftCardRedeemResult,
    AffiliateDashboardData,
    AffiliateCommissionData,
    AffiliateWithdrawData,
    AffiliateWithdrawApplyPayload,
    ResellerProfileSummaryData,
    ResellerManagementProfileData,
    ResellerDomainData,
    ResellerManagementSnapshotData,
    ResellerApplyPayload,
    ResellerCustomDomainPayload,
    ResellerLocalizedText,
    ResellerSiteConfigPayload,
    ResellerSiteConfigData,
    ResellerSiteConfigSnapshotData,
    ResellerBalanceData,
    ResellerLedgerData,
    ResellerWithdrawData,
    ResellerDashboardData,
    ResellerOrderListParams,
    ResellerOrderStatsParams,
    ResellerOrderData,
    ResellerOrderItemData,
    ResellerOrderDetailData,
    ResellerOrderStatsData,
    ResellerWithdrawApplyPayload,
    CreatePaymentPayload,
    PaymentCreateResult,
    CaptchaPayload,
} from './types'

// Re-export domain APIs
export { productAPI, postAPI, postCategoryAPI, bannerAPI, categoryAPI, memberLevelAPI } from './product'
export { userAuthAPI, captchaAPI, configAPI } from './auth'
export { userProfileAPI } from './user'
export { userOrderAPI, guestOrderAPI, paymentAPI } from './order'
export { walletAPI, giftCardAPI } from './wallet'
export { affiliateAPI } from './affiliate'
export { resellerAPI } from './reseller'
export { apiCredentialAPI } from './credential'

// Default export for backward compatibility
export { default } from './client'
