import { api } from './client'
import type {
  AdminCategory,
  AdminProduct,
  AdminFulfillment,
  AdminCardSecret,
  AdminCoupon,
  AdminMemberLevel,
  AdminMemberLevelPrice,
  AdminPromotion,
  AdminWholesalePrice,
  AdminBanner,
  AdminPost,
  AdminPaymentChannel,
  AdminUser,
  AdminSiteConnection,
  AdminResellerProfileApprovePayload,
  AdminResellerProfileUpdatePayload,
  AdminResellerProductSettingUpdatePayload,
  AdminResellerReasonPayload,
  AdminResellerSiteConfigPayload,
  AdminResellerSystemDomainPayload,
} from './types'

export interface CaptchaPayload {
  captcha_id?: string
  captcha_code?: string
  turnstile_token?: string
}

export interface AdminLoginRequest {
  username: string
  password: string
  captcha_payload?: CaptchaPayload
}

export interface AdminLoginPasswordResponse {
  requires_totp: false
  token: string
  user: {
    id: number
    username: string
  }
  expires_at: string
}

export interface AdminLoginChallengeResponse {
  requires_totp: true
  challenge_token: string
  challenge_expires_at: string
}

export type AdminLoginResponse = AdminLoginPasswordResponse | AdminLoginChallengeResponse

export interface TwoFAStatus {
  enabled: boolean
  enabled_at?: string
  recovery_codes_remaining: number
  recovery_codes_total: number
}

export interface SetupTwoFAResponse {
  secret: string
  otpauth_url: string
  expires_at: string
}

export interface EnableTwoFAResponse {
  enabled_at: string
  recovery_codes: string[]
}

export interface Verify2FAPayload {
  challenge_token: string
  code?: string
  recovery_code?: string
}

export interface AdminAuthzPolicy {
  subject: string
  object: string
  action: string
}

export interface AdminAuthzRole {
  role: string
  immutable: boolean
}

export interface AdminAuthzMeResponse {
  admin_id: number
  is_super: boolean
  roles: string[]
  policies: AdminAuthzPolicy[]
}

export interface AdminAuthzAdmin {
  id: number
  username: string
  is_super: boolean
  last_login_at?: string
  created_at?: string
  roles?: string[]
  totp_enabled?: boolean
  totp_enabled_at?: string
}

export interface AuthzCreateAdminRequest {
  username: string
  password: string
  is_super?: boolean
}

export interface AuthzUpdateAdminRequest {
  username?: string
  password?: string
  is_super?: boolean
}

export interface AdminAuthzAuditLog {
  id: number
  operator_admin_id: number
  operator_username: string
  target_admin_id?: number
  target_username: string
  action: string
  role: string
  object: string
  method: string
  request_id: string
  detail: Record<string, unknown>
  created_at: string
}

export interface AdminPermissionCatalogItem {
  module: string
  method: string
  object: string
  permission: string
}

export interface AdminWalletAccount {
  id: number
  user_id: number
  balance: string
  created_at: string
  updated_at: string
}

export interface AdminWalletTransaction {
  id: number
  user_id: number
  order_id?: number | null
  type: string
  direction: string
  amount: string
  balance_before: string
  balance_after: string
  currency: string
  reference: string
  remark: string
  created_at: string
  updated_at: string
}

export interface AdminWalletRechargeUser {
  id: number
  email: string
  display_name: string
}

export interface AdminWalletRecharge {
  id: number
  recharge_no: string
  user_id: number
  payment_id: number
  channel_id: number
  provider_type: string
  channel_type: string
  interaction_mode: string
  amount: string
  payable_amount: string
  fee_rate: string
  fee_amount: string
  currency: string
  status: string
  remark?: string
  paid_at?: string
  created_at: string
  updated_at: string
  channel_name?: string
  payment_status?: string
  user?: AdminWalletRechargeUser
}

export interface AdminAdjustWalletPayload {
  amount: string
  operation?: 'add' | 'subtract'
  currency?: string
  remark: string
}

export interface AdminRefundToWalletPayload {
  amount: string
  remark?: string
}

export interface AdminManualRefundPayload {
  amount: string
  remark?: string
  payment_fee_refunded?: boolean
}

export interface AdminUpdateRefundPaymentFeePayload {
  payment_fee_refunded: boolean
}

export interface AdminBatchCardSecretStatusPayload {
  ids?: number[]
  batch_id?: number
  filter?: AdminCardSecretQueryPayload
  status: 'available' | 'reserved' | 'used'
}

export interface AdminBatchCardSecretDeletePayload {
  ids?: number[]
  batch_id?: number
  filter?: AdminCardSecretQueryPayload
}

export interface AdminExportCardSecretsPayload {
  ids?: number[]
  batch_id?: number
  filter?: AdminCardSecretQueryPayload
  format: 'txt' | 'csv'
}

export interface AdminExportAvailableCardSecretsPayload {
  product_id: number
  sku_id?: number
  batch_id?: number
  limit: number
  format: 'txt' | 'csv'
  delete_after_export?: boolean
}

export interface AdminCardSecretQueryPayload {
  product_id?: number
  sku_id?: number
  batch_id?: number
  status?: string
  secret?: string
  batch_no?: string
}

export type AdminGiftCardStatus = 'active' | 'redeemed' | 'disabled'

export interface AdminGenerateGiftCardsPayload {
  name: string
  quantity: number
  amount: string
  expires_at?: string
}

export interface AdminUpdateGiftCardPayload {
  name?: string
  status?: Exclude<AdminGiftCardStatus, 'redeemed'>
  expires_at?: string
}

export interface AdminBatchGiftCardStatusPayload {
  ids: number[]
  status: 'active' | 'disabled'
}

export interface AdminExportGiftCardsPayload {
  ids: number[]
  format: 'txt' | 'csv'
}

export interface AdminAffiliateSetting {
  enabled: boolean
  commission_rate: number
  confirm_days: number
  min_withdraw_amount: number
  withdraw_channels: string[]
}

export interface ComplianceStatus {
  acknowledged: boolean
  acknowledged_at?: string
  acknowledged_by_admin_id?: number
  acknowledged_by_username?: string
  version?: string
}

export interface ComplianceAcknowledgePayload {
  segment1: string
  segment2: string
  segment3: string
}

export const adminAPI = {
  login: (data: AdminLoginRequest) => api.post('/admin/login', data),
  verify2FA: (data: Verify2FAPayload) => api.post('/admin/login/verify-2fa', data),
  // 合规声明
  getComplianceStatus: () => api.get('/admin/compliance/status'),
  acknowledgeCompliance: (data: ComplianceAcknowledgePayload) =>
    api.post('/admin/compliance/acknowledge', data),
  get2FAStatus: () => api.get('/admin/2fa/status'),
  setup2FA: () => api.post('/admin/2fa/setup', {}),
  enable2FA: (data: { code: string }) => api.post('/admin/2fa/enable', data),
  disable2FA: (data: { code?: string; recovery_code?: string }) => api.post('/admin/2fa/disable', data),
  regenerateRecoveryCodes: (data: { code: string }) => api.post('/admin/2fa/recovery-codes/regenerate', data),
  resetAdmin2FA: (id: number) => api.post(`/admin/authz/admins/${id}/2fa/reset`, {}),
  getAuthzMe: () => api.get('/admin/authz/me'),
  listAuthzRoles: () => api.get('/admin/authz/roles', { params: { include_metadata: true } }),
  listAuthzAdmins: () => api.get("/admin/authz/admins"),
  createAuthzAdmin: (data: AuthzCreateAdminRequest) => api.post("/admin/authz/admins", data),
  updateAuthzAdmin: (id: number, data: AuthzUpdateAdminRequest) => api.put(`/admin/authz/admins/${id}`, data),
  deleteAuthzAdmin: (id: number) => api.delete(`/admin/authz/admins/${id}`),
  listAuthzAuditLogs: (params?: Record<string, unknown>) => api.get("/admin/authz/audit-logs", { params }),
  listAuthzPermissionCatalog: () => api.get("/admin/authz/permissions/catalog"),
  createAuthzRole: (data: { role: string }) => api.post('/admin/authz/roles', data),
  deleteAuthzRole: (role: string) => api.delete(`/admin/authz/roles/${encodeURIComponent(role)}`),
  getAuthzRolePolicies: (role: string) => api.get(`/admin/authz/roles/${encodeURIComponent(role)}/policies`),
  grantAuthzPolicy: (data: { role: string; object: string; action: string }) => api.post('/admin/authz/policies', data),
  revokeAuthzPolicy: (data: { role: string; object: string; action: string }) => api.delete('/admin/authz/policies', { data }),
  getAuthzAdminRoles: (id: number) => api.get(`/admin/authz/admins/${id}/roles`),
  setAuthzAdminRoles: (id: number, data: { roles: string[] }) => api.put(`/admin/authz/admins/${id}/roles`, data),
  upload: (formData: FormData, scene = 'common') => {
    const payload = new FormData()
    formData.forEach((value, key) => {
      payload.append(key, value)
    })
    payload.append('scene', scene)
    return api.post('/admin/upload', payload)
  },
  // 素材管理
  getMedia: (params?: Record<string, unknown>) => api.get('/admin/media', { params }),
  updateMedia: (id: number, data: { name: string }) => api.put(`/admin/media/${id}`, data),
  deleteMedia: (id: number) => api.delete(`/admin/media/${id}`),
  batchDeleteMedia: (ids: number[]) => api.post('/admin/media/batch-delete', { ids }),

  getProducts: (params?: Record<string, unknown>) => api.get('/admin/products', { params }),
  getProduct: (id: number) => api.get(`/admin/products/${id}`),
  createProduct: (data: Partial<AdminProduct>) => api.post('/admin/products', data),
  updateProduct: (id: number, data: Partial<AdminProduct>) => api.put(`/admin/products/${id}`, data),
  patchProduct: (id: number, data: { is_active?: boolean; sort_order?: number; category_id?: number }) => api.patch(`/admin/products/${id}`, data),
  updateProductWholesalePrices: (id: number, data: { wholesale_prices: AdminWholesalePrice[] }) =>
    api.patch(`/admin/products/${id}/wholesale-prices`, data),
  deleteProduct: (id: number) => api.delete(`/admin/products/${id}`),
  batchUpdateProductStatus: (ids: number[], isActive: boolean) => api.post('/admin/products/batch-status', { ids, is_active: isActive }),
  batchUpdateProductCategory: (ids: number[], categoryId: number) => api.post('/admin/products/batch-category', { ids, category_id: categoryId }),
  batchDeleteProducts: (ids: number[]) => api.post('/admin/products/batch-delete', { ids }),
  getCategories: (params?: Record<string, unknown>) => api.get('/admin/categories', { params }),
  createCategory: (data: Partial<AdminCategory>) => api.post('/admin/categories', data),
  updateCategory: (id: number, data: Partial<AdminCategory>) => api.put(`/admin/categories/${id}`, data),
  patchCategoryActive: (id: number, isActive: boolean) => api.patch(`/admin/categories/${id}/active`, { is_active: isActive }),
  deleteCategory: (id: number) => api.delete(`/admin/categories/${id}`),
  getPosts: (params?: Record<string, unknown>) => api.get('/admin/posts', { params }),
  getPost: (id: number) => api.get(`/admin/posts/${id}`),
  createPost: (data: Partial<AdminPost> & { product_ids?: number[] }) => api.post('/admin/posts', data),
  updatePost: (id: number, data: Partial<AdminPost> & { product_ids?: number[] }) => api.put(`/admin/posts/${id}`, data),
  deletePost: (id: number) => api.delete(`/admin/posts/${id}`),
  getPostRelatedProducts: (id: number) => api.get(`/admin/posts/${id}/products`),
  getBanners: (params?: Record<string, unknown>) => api.get('/admin/banners', { params }),
  getBanner: (id: number) => api.get(`/admin/banners/${id}`),
  createBanner: (data: Partial<AdminBanner>) => api.post('/admin/banners', data),
  updateBanner: (id: number, data: Partial<AdminBanner>) => api.put(`/admin/banners/${id}`, data),
  deleteBanner: (id: number) => api.delete(`/admin/banners/${id}`),
  getSettings: (params?: Record<string, unknown>) => api.get('/admin/settings', { params }),
  updateSettings: (data: Record<string, unknown>) => api.put('/admin/settings', data),
  getHomeAnnouncement: () => api.get('/admin/settings', { params: { key: 'home_announcement' } }),
  updateHomeAnnouncement: (value: Record<string, unknown>) => api.put('/admin/settings', { key: 'home_announcement', value }),
  getSMTPSettings: () => api.get('/admin/settings/smtp'),
  updateSMTPSettings: (data: Record<string, unknown>) => api.put('/admin/settings/smtp', data),
  testSMTPSettings: (data: Record<string, unknown>) => api.post('/admin/settings/smtp/test', data),
  getCaptchaSettings: () => api.get('/admin/settings/captcha'),
  updateCaptchaSettings: (data: Record<string, unknown>) => api.put('/admin/settings/captcha', data),
  getTelegramAuthSettings: () => api.get('/admin/settings/telegram-auth'),
  updateTelegramAuthSettings: (data: Record<string, unknown>) => api.put('/admin/settings/telegram-auth', data),
  getGoogleAuthSettings: () => api.get('/admin/settings/google-auth'),
  updateGoogleAuthSettings: (data: Record<string, unknown>) => api.put('/admin/settings/google-auth', data),
  getOrderEmailTemplateSettings: () => api.get('/admin/settings/order-email-template'),
  updateOrderEmailTemplateSettings: (data: Record<string, unknown>) => api.put('/admin/settings/order-email-template', data),
  resetOrderEmailTemplateSettings: () => api.post('/admin/settings/order-email-template/reset'),
  getNotificationCenterSettings: () => api.get('/admin/settings/notification-center'),
  updateNotificationCenterSettings: (data: Record<string, unknown>) => api.put('/admin/settings/notification-center', data),
  listNotificationLogs: (params?: Record<string, unknown>) => api.get('/admin/settings/notification-center/logs', { params }),
  testNotificationCenterSettings: (data: Record<string, unknown>) => api.post('/admin/settings/notification-center/test', data),
  getAffiliateSettings: () => api.get('/admin/settings/affiliate'),
  updateAffiliateSettings: (data: AdminAffiliateSetting) => api.put('/admin/settings/affiliate', data),
  getPublicConfig: () => api.get('/public/config'),
  getImageCaptcha: () => api.get('/public/captcha/image'),
  getSystemVersion: () => api.get('/admin/system/version'),
  checkSystemUpdate: (params?: { owner?: string; repo?: string }) =>
    api.get('/admin/system/version/check', { params }),
  getUpdateCapability: () => api.get('/admin/system/update/capability'),
  getUpdateStatus: () => api.get('/admin/system/update/status'),
  startSystemUpdate: () => api.post('/admin/system/update/start'),
  // force：迁移已开始或元数据不可信时，确认风险后强制回滚
  rollbackSystemUpdate: (data?: { force?: boolean }) =>
    api.post('/admin/system/update/rollback', data ?? {}),
  restartSystemService: () => api.post('/admin/system/restart'),
  getDashboardOverview: (params?: Record<string, unknown>) => api.get('/admin/dashboard/overview', { params }),
  getDashboardTrends: (params?: Record<string, unknown>) => api.get('/admin/dashboard/trends', { params }),
  getDashboardRankings: (params?: Record<string, unknown>) => api.get('/admin/dashboard/rankings', { params }),
  getDashboardInventoryAlerts: () => api.get('/admin/dashboard/inventory-alerts'),
  updatePassword: (data: { old_password: string; new_password: string }) => api.put('/admin/password', data),
  getOrders: (params?: Record<string, unknown>) => api.get('/admin/orders', { params }),
  getOrder: (id: number) => api.get(`/admin/orders/${id}`),
  updateOrderStatus: (id: number, data: { status: string }) => api.patch(`/admin/orders/${id}`, data),
  createFulfillment: (data: Partial<AdminFulfillment>) => api.post('/admin/fulfillments', data),
  downloadFulfillment: (orderId: number) => api.get(`/admin/orders/${orderId}/fulfillment/download`, { blob: true }),
  getPayments: (params?: Record<string, unknown>) => api.get('/admin/payments', { params }),
  getPayment: (id: number) => api.get(`/admin/payments/${id}`),
  exportPayments: (params?: Record<string, unknown>) => api.get('/admin/payments/export', { params, blob: true }),
  createPaymentChannel: (data: Partial<AdminPaymentChannel>) => api.post('/admin/payment-channels', data),
  getPaymentChannels: (params?: Record<string, unknown>) => api.get('/admin/payment-channels', { params }),
  getPaymentChannel: (id: number) => api.get(`/admin/payment-channels/${id}`),
  testWechatPayPublicKey: (id: number) => api.post(`/admin/payment-channels/${id}/wechatpay-public-key-test`, {}),
  updatePaymentChannel: (id: number, data: Partial<AdminPaymentChannel>) => api.put(`/admin/payment-channels/${id}`, data),
  deletePaymentChannel: (id: number) => api.delete(`/admin/payment-channels/${id}`),
  getUsers: (params?: Record<string, unknown>) => api.get('/admin/users', { params }),
  getUserLoginLogs: (params?: Record<string, unknown>) => api.get('/admin/user-login-logs', { params }),
  getUser: (id: number) => api.get(`/admin/users/${id}`),
  getUserWallet: (id: number) => api.get(`/admin/users/${id}/wallet`),
  getUserWalletTransactions: (id: number, params?: Record<string, unknown>) =>
    api.get(`/admin/users/${id}/wallet/transactions`, { params }),
  getWalletRecharges: (params?: Record<string, unknown>) =>
    api.get('/admin/wallet/recharges', { params }),
  adjustUserWallet: (id: number, data: AdminAdjustWalletPayload) =>
    api.post(`/admin/users/${id}/wallet/adjust`, data),
  updateUser: (id: number, data: Partial<AdminUser>) => api.put(`/admin/users/${id}`, data),
  unbindUserTelegram: (id: number) => api.delete(`/admin/users/${id}/oauth/telegram`),
  unbindUserGoogle: (id: number) => api.delete(`/admin/users/${id}/oauth/google`),
  resetUser2FA: (id: number) => api.delete(`/admin/users/${id}/2fa`),
  batchUpdateUserStatus: (data: { user_ids: number[]; status: string }) => api.put('/admin/users/batch-status', data),
  getUserCouponUsages: (id: number, params?: Record<string, unknown>) => api.get(`/admin/users/${id}/coupon-usages`, { params }),
  getAffiliateUsers: (params?: Record<string, unknown>) => api.get('/admin/affiliates/users', { params }),
  updateAffiliateUserStatus: (id: number, data: { status: string }) =>
    api.patch(`/admin/affiliates/users/${id}/status`, data),
  batchUpdateAffiliateUserStatus: (data: { profile_ids: number[]; status: string }) =>
    api.patch('/admin/affiliates/users/batch-status', data),
  getAffiliateCommissions: (params?: Record<string, unknown>) => api.get('/admin/affiliates/commissions', { params }),
  getAffiliateWithdraws: (params?: Record<string, unknown>) => api.get('/admin/affiliates/withdraws', { params }),
  rejectAffiliateWithdraw: (id: number, data: { reason?: string }) => api.post(`/admin/affiliates/withdraws/${id}/reject`, data),
  payAffiliateWithdraw: (id: number) => api.post(`/admin/affiliates/withdraws/${id}/pay`, {}),
  getResellerOperationsOverview: (params?: Record<string, unknown>) =>
    api.get('/admin/resellers/operations/overview', { params }),
  getResellerOperationsFinance: (params?: Record<string, unknown>) =>
    api.get('/admin/resellers/operations/finance', { params }),
  getResellerLedgerEntries: (params?: Record<string, unknown>) =>
    api.get('/admin/resellers/ledger-entries', { params }),
  getResellerBalanceAccounts: (params?: Record<string, unknown>) =>
    api.get('/admin/resellers/balance-accounts', { params }),
  getResellerWithdraws: (params?: Record<string, unknown>) =>
    api.get('/admin/resellers/withdraws', { params }),
  rejectResellerWithdraw: (id: number, data: { reason?: string }) =>
    api.post(`/admin/resellers/withdraws/${id}/reject`, data),
  payResellerWithdraw: (id: number) =>
    api.post(`/admin/resellers/withdraws/${id}/pay`, {}),
  getResellerProfiles: (params?: Record<string, unknown>) =>
    api.get('/admin/resellers/profiles', { params }),
  getResellerProfile: (id: number) =>
    api.get(`/admin/resellers/profiles/${id}`),
  updateResellerProfile: (id: number, data: AdminResellerProfileUpdatePayload) =>
    api.put(`/admin/resellers/profiles/${id}`, data),
  assignResellerSystemDomain: (id: number, data: AdminResellerSystemDomainPayload) =>
    api.put(`/admin/resellers/profiles/${id}/system-domain`, data),
  approveResellerProfile: (id: number, data: AdminResellerProfileApprovePayload) =>
    api.post(`/admin/resellers/profiles/${id}/approve`, data),
  rejectResellerProfile: (id: number, data: AdminResellerReasonPayload) =>
    api.post(`/admin/resellers/profiles/${id}/reject`, data),
  disableResellerProfile: (id: number, data: AdminResellerReasonPayload) =>
    api.post(`/admin/resellers/profiles/${id}/disable`, data),
  restoreResellerProfile: (id: number) =>
    api.post(`/admin/resellers/profiles/${id}/restore`, {}),
  getResellerDomains: (params?: Record<string, unknown>) =>
    api.get('/admin/resellers/domains', { params }),
  approveResellerDomain: (id: number) =>
    api.post(`/admin/resellers/domains/${id}/approve`, {}),
  disableResellerDomain: (id: number) =>
    api.post(`/admin/resellers/domains/${id}/disable`, {}),
  setPrimaryResellerDomain: (id: number) =>
    api.post(`/admin/resellers/domains/${id}/set-primary`, {}),
  getResellerSiteConfigs: (params?: Record<string, unknown>) =>
    api.get('/admin/resellers/site-configs', { params }),
  getResellerSiteConfig: (resellerId: number) =>
    api.get(`/admin/resellers/site-configs/${resellerId}`),
  updateResellerSiteConfig: (resellerId: number, data: AdminResellerSiteConfigPayload) =>
    api.put(`/admin/resellers/site-configs/${resellerId}`, data),
  resetResellerSiteConfig: (resellerId: number) =>
    api.post(`/admin/resellers/site-configs/${resellerId}/reset`, {}),
  getResellerProductSettings: (params?: Record<string, unknown>) =>
    api.get('/admin/resellers/product-settings', { params }),
  getResellerProductSetting: (resellerId: number, productId: number) =>
    api.get(`/admin/resellers/product-settings/${resellerId}/${productId}`),
  updateResellerProductSettings: (
    resellerId: number,
    productId: number,
    data: AdminResellerProductSettingUpdatePayload,
  ) => api.put(`/admin/resellers/product-settings/${resellerId}/${productId}`, data),
  previewResellerProductSettings: (
    resellerId: number,
    productId: number,
    data: AdminResellerProductSettingUpdatePayload,
  ) => api.post(`/admin/resellers/product-settings/${resellerId}/${productId}/preview`, data),
  resetResellerProductSetting: (resellerId: number, productId: number, skuId = 0) =>
    api.delete(`/admin/resellers/product-settings/${resellerId}/${productId}`, { params: { sku_id: skuId } }),
  refundOrderToWallet: (id: number, data: AdminRefundToWalletPayload) =>
    api.post(`/admin/orders/${id}/refund-to-wallet`, data),
  manualRefundOrder: (id: number, data: AdminManualRefundPayload) =>
    api.post(`/admin/orders/${id}/manual-refund`, data),
  getOrderRefunds: (params?: Record<string, unknown>) => api.get('/admin/order-refunds', { params }),
  getOrderRefund: (id: number) => api.get(`/admin/order-refunds/${id}`),
  updateOrderRefundPaymentFee: (id: number, data: AdminUpdateRefundPaymentFeePayload) =>
    api.patch(`/admin/order-refunds/${id}/payment-fee`, data),
  createCoupon: (data: Partial<AdminCoupon>) => api.post('/admin/coupons', data),
  getCoupons: (params?: Record<string, unknown>) => api.get('/admin/coupons', { params }),
  updateCoupon: (id: number, data: Partial<AdminCoupon>) => api.put(`/admin/coupons/${id}`, data),
  deleteCoupon: (id: number) => api.delete(`/admin/coupons/${id}`),
  generateGiftCards: (data: AdminGenerateGiftCardsPayload) =>
    api.post('/admin/gift-cards/generate', data),
  getGiftCards: (params?: Record<string, unknown>) => api.get('/admin/gift-cards', { params }),
  updateGiftCard: (id: number, data: AdminUpdateGiftCardPayload) => api.put(`/admin/gift-cards/${id}`, data),
  deleteGiftCard: (id: number) => api.delete(`/admin/gift-cards/${id}`),
  batchUpdateGiftCardStatus: (data: AdminBatchGiftCardStatusPayload) =>
    api.patch('/admin/gift-cards/batch-status', data),
  exportGiftCards: (data: AdminExportGiftCardsPayload) =>
    api.post('/admin/gift-cards/export', data, { blob: true }),
  createPromotion: (data: Partial<AdminPromotion>) => api.post('/admin/promotions', data),
  getPromotions: (params?: Record<string, unknown>) => api.get('/admin/promotions', { params }),
  updatePromotion: (id: number, data: Partial<AdminPromotion>) => api.put(`/admin/promotions/${id}`, data),
  deletePromotion: (id: number) => api.delete(`/admin/promotions/${id}`),

  // 会员等级
  getMemberLevels: (params?: Record<string, unknown>) => api.get('/admin/member-levels', { params }),
  createMemberLevel: (data: Partial<AdminMemberLevel>) => api.post('/admin/member-levels', data),
  updateMemberLevel: (id: number, data: Partial<AdminMemberLevel>) => api.put(`/admin/member-levels/${id}`, data),
  deleteMemberLevel: (id: number) => api.delete(`/admin/member-levels/${id}`),
  getMemberLevelPrices: (productId: number) => api.get('/admin/member-level-prices', { params: { product_id: productId } }),
  batchUpsertMemberLevelPrices: (data: { prices: Partial<AdminMemberLevelPrice>[] }) => api.post('/admin/member-level-prices/batch', data),
  deleteMemberLevelPrice: (id: number) => api.delete(`/admin/member-level-prices/${id}`),
  setUserMemberLevel: (userId: number, memberLevelId: number) => api.put(`/admin/users/${userId}/member-level`, { member_level_id: memberLevelId }),
  backfillMemberLevels: () => api.post('/admin/member-levels/backfill'),
  createCardSecretBatch: (data: { product_id: number; sku_id?: number; name?: string; secrets: string[]; batch_no?: string; note?: string; deduplicate?: boolean; is_loop?: boolean }) => api.post('/admin/card-secrets/batch', data),
  importCardSecretCSV: (formData: FormData) =>
    api.post('/admin/card-secrets/import', formData),
  getCardSecrets: (params?: Record<string, unknown>) => api.get('/admin/card-secrets', { params }),
  updateCardSecret: (id: number, data: Partial<AdminCardSecret>) => api.put(`/admin/card-secrets/${id}`, data),
  batchUpdateCardSecretStatus: (data: AdminBatchCardSecretStatusPayload) =>
    api.patch('/admin/card-secrets/batch-status', data),
  batchDeleteCardSecrets: (data: AdminBatchCardSecretDeletePayload) =>
    api.post('/admin/card-secrets/batch-delete', data),
  exportCardSecrets: (data: AdminExportCardSecretsPayload) =>
    api.post('/admin/card-secrets/export', data, { blob: true }),
  exportAvailableCardSecrets: (data: AdminExportAvailableCardSecretsPayload) =>
    api.post('/admin/card-secrets/export-available', data, { blob: true }),
  getCardSecretStats: (params?: Record<string, unknown>) => api.get('/admin/card-secrets/stats', { params }),
  getCardSecretBatches: (params?: Record<string, unknown>) => api.get('/admin/card-secrets/batches', { params }),
  getCardSecretTemplate: () => api.get('/admin/card-secrets/template'),
  // Site Connections
  getSiteConnections: (params?: Record<string, unknown>) => api.get('/admin/site-connections', { params }),
  getSiteConnection: (id: number) => api.get(`/admin/site-connections/${id}`),
  createSiteConnection: (data: Partial<AdminSiteConnection>) => api.post('/admin/site-connections', data),
  updateSiteConnection: (id: number, data: Partial<AdminSiteConnection>) => api.put(`/admin/site-connections/${id}`, data),
  deleteSiteConnection: (id: number) => api.delete(`/admin/site-connections/${id}`),
  pingSiteConnection: (id: number) => api.post(`/admin/site-connections/${id}/ping`),
  updateSiteConnectionStatus: (id: number, data: { is_active?: boolean; status?: string }) => api.put(`/admin/site-connections/${id}/status`, data),
  reapplyConnectionMarkup: (id: number) => api.post(`/admin/site-connections/${id}/reapply-markup`),
  // Product Mappings
  getProductMappings: (params?: Record<string, unknown>) => api.get('/admin/product-mappings', { params }),
  getProductMapping: (id: number) => api.get(`/admin/product-mappings/${id}`),
  importUpstreamProduct: (data: Record<string, unknown>) => api.post('/admin/product-mappings/import', data),
  batchImportUpstreamProducts: (data: Record<string, unknown>) => api.post('/admin/product-mappings/batch-import', data),
  syncProductMapping: (id: number) => api.post(`/admin/product-mappings/${id}/sync`),
  updateProductMappingStatus: (id: number, data: { is_active: boolean }) => api.put(`/admin/product-mappings/${id}/status`, data),
  deleteProductMapping: (id: number) => api.delete(`/admin/product-mappings/${id}`),
  batchSyncProductMappings: (ids: number[]) => api.post('/admin/product-mappings/batch-sync', { ids }),
  batchUpdateProductMappingStatus: (ids: number[], isActive: boolean) => api.post('/admin/product-mappings/batch-status', { ids, is_active: isActive }),
  batchDeleteProductMappings: (ids: number[]) => api.post('/admin/product-mappings/batch-delete', { ids }),
  getUpstreamProducts: (params?: Record<string, unknown>) => api.get('/admin/upstream-products', { params }),
  getUpstreamCategories: (params: { connection_id: string }) => api.get('/admin/upstream-categories', { params }),
  batchImportByCategory: (data: Record<string, unknown>) => api.post('/admin/product-mappings/batch-import-by-category', data),
  // Procurement Orders
  getProcurementOrders: (params?: Record<string, unknown>) => api.get('/admin/procurement-orders', { params }),
  getProcurementOrderStats: (params?: Record<string, unknown>) => api.get('/admin/procurement-orders/stats', { params }),
  getProcurementOrder: (id: number) => api.get(`/admin/procurement-orders/${id}`),
  downloadProcurementUpstreamPayload: (id: number) => api.get(`/admin/procurement-orders/${id}/upstream-payload/download`, { blob: true }),
  retryProcurementOrder: (id: number) => api.post(`/admin/procurement-orders/${id}/retry`),
  cancelProcurementOrder: (id: number) => api.post(`/admin/procurement-orders/${id}/cancel`),

  // 对账管理
  runReconciliation: (data: { connection_id: number; type: string; time_range_start: string; time_range_end: string }) => api.post('/admin/reconciliation/run', data),
  getReconciliationJobs: (params?: Record<string, unknown>) => api.get('/admin/reconciliation/jobs', { params }),
  getReconciliationJob: (id: number, params?: Record<string, unknown>) => api.get(`/admin/reconciliation/jobs/${id}`, { params }),
  resolveReconciliationItem: (id: number, data: { resolution?: string; remark?: string }) => api.put(`/admin/reconciliation/items/${id}/resolve`, data),

  // API 凭证管理
  getApiCredentials: (params?: Record<string, unknown>) => api.get('/admin/api-credentials', { params }),
  getApiCredential: (id: number) => api.get(`/admin/api-credentials/${id}`),
  approveApiCredential: (id: number) => api.post(`/admin/api-credentials/${id}/approve`),
  rejectApiCredential: (id: number, data: { reason: string }) => api.post(`/admin/api-credentials/${id}/reject`, data),
  updateApiCredentialStatus: (id: number, data: { is_active: boolean }) => api.put(`/admin/api-credentials/${id}/status`, data),
  deleteApiCredential: (id: number) => api.delete(`/admin/api-credentials/${id}`),

  // Channel Clients (Bot Clients)
  getChannelClients: () => api.get('/admin/channel-clients'),
  createChannelClient: (data: { name: string; channel_type: string; description?: string; bot_token?: string; callback_url?: string }) => api.post('/admin/channel-clients', data),
  getChannelClient: (id: number) => api.get(`/admin/channel-clients/${id}`),
  updateChannelClient: (id: number, data: { name?: string; description?: string; bot_token?: string; callback_url?: string }) => api.put(`/admin/channel-clients/${id}`, data),
  updateChannelClientStatus: (id: number, data: { status: number }) => api.put(`/admin/channel-clients/${id}/status`, data),
  resetChannelClientSecret: (id: number) => api.post(`/admin/channel-clients/${id}/reset-secret`),
  deleteChannelClient: (id: number) => api.delete(`/admin/channel-clients/${id}`),

  // Telegram Bot Settings
  getTelegramBotSettings: () => api.get('/admin/settings/telegram-bot'),
  updateTelegramBotSettings: (data: Record<string, unknown>) => api.put('/admin/settings/telegram-bot', data),
  getTelegramBotRuntimeStatus: () => api.get('/admin/settings/telegram-bot/runtime-status'),
  getTelegramBroadcasts: (params?: Record<string, unknown>) =>
    api.get('/admin/telegram-bot/broadcasts', { params }),
  getTelegramBroadcast: (id: number) =>
    api.get(`/admin/telegram-bot/broadcasts/${id}`),
  createTelegramBroadcast: (data: {
    title: string
    recipient_type: string
    user_ids?: number[]
    filters?: Record<string, unknown>
    attachment_url?: string
    attachment_name?: string
    message_html: string
  }) => api.post('/admin/telegram-bot/broadcasts', data),
  deleteTelegramBroadcast: (id: number) => api.delete(`/admin/telegram-bot/broadcasts/${id}`),
  getTelegramBroadcastUsers: (params?: Record<string, unknown>) =>
    api.get('/admin/telegram-bot/users', { params }),

  // 广告代理
  renderAdSlot: (slotCode: string, params?: Record<string, string>) =>
    api.get(`/admin/ads/render/${slotCode}`, { params }),
  reportAdImpression: (data: { tenant: string; client: string; slot_code: string; items: { ad_id: number; impression_token: string }[] }) =>
    api.post('/admin/ads/impression', data),

  // 文章分类
  getPostCategories: (params?: Record<string, unknown>) => api.get('/admin/post-categories', { params }),
  createPostCategory: (data: any) => api.post('/admin/post-categories', data),
  updatePostCategory: (id: number, data: any) => api.put(`/admin/post-categories/${id}`, data),
  deletePostCategory: (id: number) => api.delete(`/admin/post-categories/${id}`),
  patchPostCategoryStatus: (id: number, is_active: boolean) => api.patch(`/admin/post-categories/${id}/status`, { is_active }),
}
