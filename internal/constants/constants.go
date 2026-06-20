package constants

// 订单状态常量
const (
	OrderStatusPendingPayment     = "pending_payment"
	OrderStatusPaid               = "paid"
	OrderStatusFulfilling         = "fulfilling"
	OrderStatusPartiallyDelivered = "partially_delivered"
	OrderStatusPartiallyRefunded  = "partially_refunded"
	OrderStatusDelivered          = "delivered"
	OrderStatusCompleted          = "completed"
	OrderStatusCanceled           = "canceled"
	OrderStatusRefunded           = "refunded"
)

// 订单退款常量

const (
	OrderRefundTypeManual = "manual"
	OrderRefundTypeWallet = "wallet"
)

// 交付类型与状态常量
const (
	FulfillmentTypeAuto        = "auto"
	FulfillmentTypeManual      = "manual"
	FulfillmentTypeUpstream    = "upstream"
	FulfillmentStatusPending   = "pending"
	FulfillmentStatusDelivered = "delivered"
)

// 支付状态常量
const (
	PaymentStatusInitiated = "initiated"
	PaymentStatusPending   = "pending"
	PaymentStatusSuccess   = "success"
	PaymentStatusFailed    = "failed"
	PaymentStatusExpired   = "expired"
)

// 支付提供方常量
const (
	PaymentProviderOfficial  = "official"
	PaymentProviderEpay      = "epay"
	PaymentProviderEpusdt    = "epusdt"
	PaymentProviderBepusdt   = "bepusdt"
	PaymentProviderDujiaoPay = "dujiaopay"
	PaymentProviderOkpay     = "okpay"
	PaymentProviderTokenpay  = "tokenpay"
	PaymentProviderWallet    = "wallet"
)

// 支付渠道类型常量
const (
	PaymentChannelTypeWechat    = "wechat"
	PaymentChannelTypeWxpay     = "wxpay"
	PaymentChannelTypeAlipay    = "alipay"
	PaymentChannelTypePaypal    = "paypal"
	PaymentChannelTypeStripe    = "stripe"
	PaymentChannelTypeQqpay     = "qqpay"
	PaymentChannelTypeUsdt      = "usdt"
	PaymentChannelTypeUsdtTrc20 = "usdt-trc20"
	PaymentChannelTypeUsdcTrc20 = "usdc-trc20"
	PaymentChannelTypeTrx       = "trx"
	PaymentChannelTypeBalance   = "balance"
)

// 支付渠道付款角色常量
const (
	PaymentRoleGuest  = "guest"
	PaymentRoleMember = "member"
)

// 支付付款类型常量
const (
	PaymentTypeWallet = "wallet"
	PaymentTypeOrder  = "order"
)

// 支付交互方式常量
const (
	PaymentInteractionQR       = "qr"
	PaymentInteractionRedirect = "redirect"
	PaymentInteractionWAP      = "wap"
	PaymentInteractionPage     = "page"
	PaymentInteractionBalance  = "balance"
)

// 钱包交易类型常量
const (
	WalletTxnTypeRecharge    = "recharge"
	WalletTxnTypeOrderPay    = "order_pay"
	WalletTxnTypeOrderRefund = "order_refund"
	WalletTxnTypeAdminAdjust = "admin_adjust"
	WalletTxnTypeAdminRefund = "admin_refund"
	WalletTxnTypeGiftCard    = "gift_card_redeem"
)

// 钱包交易方向常量
const (
	WalletTxnDirectionIn  = "in"
	WalletTxnDirectionOut = "out"
)

// 钱包充值状态常量
const (
	WalletRechargeStatusPending = "pending"
	WalletRechargeStatusSuccess = "success"
	WalletRechargeStatusFailed  = "failed"
	WalletRechargeStatusExpired = "expired"
)

// 推广返利状态常量
const (
	AffiliateProfileStatusActive   = "active"
	AffiliateProfileStatusDisabled = "disabled"
)

// 推广返利佣金状态常量
const (
	AffiliateCommissionStatusPendingConfirm = "pending_confirm"
	AffiliateCommissionStatusAvailable      = "available"
	AffiliateCommissionStatusRejected       = "rejected"
	AffiliateCommissionStatusWithdrawn      = "withdrawn"
)

// 推广返利佣金类型常量
const (
	AffiliateCommissionTypeOrder = "order"
)

// 推广返利提现状态常量
const (
	AffiliateWithdrawStatusPendingReview = "pending_review"
	AffiliateWithdrawStatusRejected      = "rejected"
	AffiliateWithdrawStatusPaid          = "paid"
)

// 推广返利提现审核动作常量
const (
	AffiliateWithdrawActionReject = "reject"
	AffiliateWithdrawActionPay    = "pay"
)

// 易支付回调常量
const (
	EpayTradeStatusSuccess = "TRADE_SUCCESS"
	EpayCallbackSuccess    = "success"
	EpayCallbackFail       = "fail"
	EpayPayTypeQRCode      = "qrcode"
)

// 支付宝回调常量
const (
	AlipayTradeStatusSuccess      = "TRADE_SUCCESS"
	AlipayTradeStatusFinished     = "TRADE_FINISHED"
	AlipayTradeStatusClosed       = "TRADE_CLOSED"
	AlipayTradeStatusWaitBuyerPay = "WAIT_BUYER_PAY"
	AlipayCallbackSuccess         = "success"
	AlipayCallbackFail            = "fail"
)

// BEpusdt 回调常量
const (
	BepusdtCallbackSuccess = "success"
	BepusdtCallbackFail    = "fail"
)

// epusdt（GMPay）回调常量
const (
	EpusdtCallbackSuccess = "ok"
	EpusdtCallbackFail    = "fail"
)

// OKPAY 回调常量
const (
	OkpayCallbackSuccess = `{"status":"success"}`
	OkpayCallbackFail    = `{"status":"fail"}`
)

// TokenPay 回调常量
const (
	TokenPayCallbackSuccess = "ok"
	TokenPayCallbackFail    = "fail"
)

// 文章类型常量
const (
	PostTypeBlog   = "blog"
	PostTypeNotice = "notice"
)

// 商品购买身份常量
const (
	ProductPurchaseGuest  = "guest"
	ProductPurchaseMember = "member"
)

// 商品库存状态常量
const (
	ProductStockStatusUnlimited  = "unlimited"
	ProductStockStatusInStock    = "in_stock"
	ProductStockStatusLowStock   = "low_stock"
	ProductStockStatusOutOfStock = "out_of_stock"
)

// 公开库存展示模式与档位常量
const (
	ProductStockDisplayExact  = "exact"
	ProductStockDisplayStatus = "status"
	ProductStockDisplayRange  = "range"
	ProductStockDisplayHidden = "hidden"

	ProductStockDisplayRange1To5    = "range_1_5"
	ProductStockDisplayRange6To20   = "range_6_20"
	ProductStockDisplayRange21To50  = "range_21_50"
	ProductStockDisplayRange51To100 = "range_51_100"
	ProductStockDisplayRange100Plus = "range_100_plus"
)

// 手动库存常量
const (
	ManualStockUnlimited = -1
)

// 优惠券类型常量
const (
	CouponTypeFixed   = "fixed"
	CouponTypePercent = "percent"
)

// 活动价类型常量
const (
	PromotionTypeFixed        = "fixed"
	PromotionTypePercent      = "percent"
	PromotionTypeSpecialPrice = "special_price"
)

// 适用范围常量
const (
	ScopeTypeProduct = "product"
)

// 用户状态常量
const (
	UserStatusActive   = "active"
	UserStatusDisabled = "disabled"
)

// 第三方登录提供方常量
const (
	UserOAuthProviderTelegram = "telegram"
)

// 登录日志状态常量
const (
	LoginLogStatusSuccess = "success"
	LoginLogStatusFailed  = "failed"
)

// 登录日志失败原因常量
const (
	LoginLogFailReasonBadRequest           = "bad_request"
	LoginLogFailReasonCaptchaRequired      = "captcha_required"
	LoginLogFailReasonCaptchaInvalid       = "captcha_invalid"
	LoginLogFailReasonCaptchaConfigInvalid = "captcha_config_invalid"
	LoginLogFailReasonCaptchaVerifyFailed  = "captcha_verify_failed"
	LoginLogFailReasonInvalidEmail         = "invalid_email"
	LoginLogFailReasonInvalidCredentials   = "invalid_credentials"
	LoginLogFailReasonEmailNotVerified     = "email_not_verified"
	LoginLogFailReasonUserDisabled         = "user_disabled"
	LoginLogFailReasonTelegramInvalid      = "telegram_invalid"
	LoginLogFailReasonTelegramExpired      = "telegram_expired"
	LoginLogFailReasonTelegramReplayed     = "telegram_replayed"
	LoginLogFailReasonTelegramConfig       = "telegram_config_invalid"
	LoginLogFailReasonInternalError        = "internal_error"
	LoginLogFailReasonInvalidTOTPCode      = "invalid_totp_code"
	LoginLogFailReasonInvalidRecoveryCode  = "invalid_recovery_code"
	LoginLogFailReasonChallengeInvalid     = "challenge_invalid"
	LoginLogFailReasonTooManyAttempts      = "too_many_attempts"
	// LoginLogPasswordOK2FAPending 第一步密码通过，等待 TOTP 验证
	LoginLogPasswordOK2FAPending = "password_ok_2fa_pending"
)

// 登录日志来源常量
const (
	LoginLogSourceWeb      = "web"
	LoginLogSourceTelegram = "telegram"
)

// 验证码用途常量
const (
	VerifyPurposeRegister       = "register"
	VerifyPurposeReset          = "reset"
	VerifyPurposeTelegramBind   = "telegram_bind"
	VerifyPurposeChangeEmailOld = "change_email_old"
	VerifyPurposeChangeEmailNew = "change_email_new"
)

// 验证码提供方常量
const (
	CaptchaProviderNone      = "none"
	CaptchaProviderImage     = "image"
	CaptchaProviderTurnstile = "turnstile"
)

// 验证码校验场景常量
const (
	CaptchaSceneLogin            = "login"
	CaptchaSceneRegisterSendCode = "register_send_code"
	CaptchaSceneResetSendCode    = "reset_send_code"
	CaptchaSceneGuestCreateOrder = "guest_create_order"
	CaptchaSceneGiftCardRedeem   = "gift_card_redeem"
)

// 通知中心事件常量
const (
	NotificationEventWalletRechargeSuccess    = "wallet_recharge_success"
	NotificationEventOrderPaidSuccess         = "order_paid_success"
	NotificationEventManualFulfillmentPending = "manual_fulfillment_pending"
	NotificationEventExceptionAlert           = "exception_alert"
	NotificationEventExceptionAlertCheck      = "exception_alert_check"
)

// 通知中心异常阈值类型常量
const (
	NotificationAlertTypeOutOfStockProducts = "out_of_stock_products"
	NotificationAlertTypeLowStockProducts   = "low_stock_products"
	NotificationAlertTypePendingOrders      = "pending_payment_orders"
	NotificationAlertTypePaymentsFailed     = "payments_failed"
)

// 队列常量
const (
	QueueDefault                    = "default"
	TaskOrderStatusEmail            = "order:status_email"
	TaskOrderAutoFulfill            = "order:auto_fulfill"
	TaskOrderTimeoutCancel          = "order:timeout_cancel"
	TaskWalletRechargeExpire        = "wallet_recharge:timeout_expire"
	TaskNotificationDispatch        = "notification:dispatch"
	TaskAffiliateConfirmCommissions = "affiliate:confirm_commissions"
	TaskResellerConfirmLedger       = "reseller:confirm_ledger"
	TaskProcurementSubmit           = "procurement:submit"
	TaskProcurementPollStatus       = "procurement:poll_status"
	TaskProcurementSyncAccepted     = "procurement:sync_accepted"
	TaskUpstreamSyncProducts        = "upstream:sync_products"
	TaskUpstreamSyncStock           = "upstream:sync_stock"
	TaskReconciliationRun           = "reconciliation:run"
	TaskDownstreamCallback          = "downstream:callback"
	TaskBotNotify                   = "bot:notify"
	TaskTelegramBroadcast           = "telegram:broadcast"
)

// Telegram Bot 群发常量
const (
	TelegramBroadcastRecipientTypeAll      = "all"
	TelegramBroadcastRecipientTypeSpecific = "specific"
	TelegramBroadcastStatusPending         = "pending"
	TelegramBroadcastStatusRunning         = "running"
	TelegramBroadcastStatusCompleted       = "completed"
	TelegramBroadcastStatusFailed          = "failed"
)

// 采购单状态常量
const (
	ProcurementStatusPending           = "pending"
	ProcurementStatusSubmitted         = "submitted"
	ProcurementStatusAccepted          = "accepted"
	ProcurementStatusRejected          = "rejected"
	ProcurementStatusFailed            = "failed"
	ProcurementStatusPartiallyRefunded = "partially_refunded"
	ProcurementStatusFulfilled         = "fulfilled"
	ProcurementStatusCompleted         = "completed"
	ProcurementStatusRefunded          = "refunded"
	ProcurementStatusCanceled          = "canceled"
)

// 对接连接状态常量
const (
	ConnectionStatusPending  = "pending"
	ConnectionStatusActive   = "active"
	ConnectionStatusDisabled = "disabled"
)

// 对接协议类型常量
const (
	ConnectionProtocolDujiaoNext = "dujiao-next"
)

// API 凭证状态常量
const (
	ApiCredentialStatusPendingReview = "pending_review"
	ApiCredentialStatusApproved      = "approved"
	ApiCredentialStatusRejected      = "rejected"
	ApiCredentialStatusDisabled      = "disabled"
)

// 下游回调状态常量
const (
	CallbackStatusPending = "pending"
	CallbackStatusSent    = "sent"
	CallbackStatusFailed  = "failed"
)

// 对账类型常量
const (
	ReconciliationTypeStatus = "status"
	ReconciliationTypeAmount = "amount"
	ReconciliationTypeFull   = "full"
)

// 对账任务状态常量
const (
	ReconciliationJobStatusPending   = "pending"
	ReconciliationJobStatusRunning   = "running"
	ReconciliationJobStatusCompleted = "completed"
	ReconciliationJobStatusFailed    = "failed"
)

// 缓存默认配置常量
const (
	RedisPrefixDefault = "dj"
)

// 设置键常量
const (
	SettingKeySiteConfig               = "site_config"
	SettingKeyOrderConfig              = "order_config"
	SettingKeySMTPConfig               = "smtp_config"
	SettingKeyCaptchaConfig            = "captcha_config"
	SettingKeyTelegramAuthConfig       = "telegram_auth_config"
	SettingKeyDashboardConfig          = "dashboard_config"
	SettingKeyNotificationCenterConfig = "notification_center_config"
	SettingKeyAffiliateConfig          = "affiliate_config"
	SettingKeyTelegramBotConfig        = "telegram_bot_config"
	SettingKeyTelegramBotRuntimeStatus = "telegram_bot_runtime_status"
	SettingKeyOrderEmailTemplateConfig = "order_email_template_config"
	SettingFieldSiteCurrency           = "currency"
	SettingFieldPaymentExpireMinutes   = "payment_expire_minutes"

	SettingKeyNavConfig = "nav_config"

	SettingKeyWalletConfig        = "wallet_config"
	SettingFieldWalletOnlyPayment = "wallet_only_payment"

	SettingKeyRegistrationConfig            = "registration_config"
	SettingFieldRegistrationEnabled         = "registration_enabled"
	SettingFieldEmailVerificationEnabled    = "email_verification_enabled"
	SettingFieldEmailDomainAllowlistEnabled = "email_domain_allowlist_enabled"
	SettingFieldAllowedEmailDomains         = "allowed_email_domains"

	SettingKeyOrderRiskControlConfig = "order_risk_control_config"

	SettingKeyUpstreamSyncConfig        = "upstream_sync_config"
	SettingFieldUpstreamSyncIntervalMin = "interval_minutes"
	SettingFieldUpstreamPreOrderCheck   = "pre_order_stock_check_enabled"
	SettingFieldUpstreamSyncPageSize    = "sync_page_size"
	SettingFieldUpstreamSyncMaxPages    = "sync_max_pages"
	SettingFieldUpstreamSyncConcurrency = "sync_conn_concurrency"

	SettingKeyCallbackRoutesConfig = "callback_routes_config"

	SettingKeyHomeAnnouncement   = "home_announcement"
	SettingFieldPaymentCallback  = "payment_callback"
	SettingFieldDujiaoPayWebhook = "dujiaopay_webhook"
	SettingFieldPaypalWebhook    = "paypal_webhook"
	SettingFieldStripeWebhook    = "stripe_webhook"
	SettingFieldUpstreamCallback = "upstream_callback"

	// 默认回调路由路径
	DefaultPaymentCallbackPath  = "/api/v1/payments/callback"
	DefaultDujiaoPayWebhookPath = "/api/v1/payments/webhook/dujiaopay"
	DefaultPaypalWebhookPath    = "/api/v1/payments/webhook/paypal"
	DefaultStripeWebhookPath    = "/api/v1/payments/webhook/stripe"
	DefaultUpstreamCallbackPath = "/api/v1/upstream/callback"
)

// 币种常量
const (
	SiteCurrencyDefault = "CNY"
)

// 站点语言常量
const (
	LocaleZhCN = "zh-CN"
	LocaleZhTW = "zh-TW"
	LocaleEnUS = "en-US"
)

// 支持的站点语言顺序（含回退顺序）
var SupportedLocales = []string{LocaleZhCN, LocaleZhTW, LocaleEnUS}

// 通知业务类型常量
const (
	NotificationBizTypeOrder           = "order"
	NotificationBizTypeWalletRecharge  = "wallet_recharge"
	NotificationBizTypeDashboardAlert  = "dashboard_alert"
	NotificationBizTypePaymentCallback = "payment_callback"
	NotificationBizTypeProcurement     = "procurement"
	NotificationBizTypeReconciliation  = "reconciliation"
)

// 对账差异类型常量
const (
	MismatchTypeStatus = "status"
	MismatchTypeAmount = "amount"
	MismatchTypeBoth   = "both"
)

// 卡密批次来源常量
const (
	CardSecretSourceManual = "manual"
	CardSecretSourceCSV    = "csv"
)

// 导出格式常量
const (
	ExportFormatCSV = "csv"
	ExportFormatTXT = "txt"
)

// Banner 位置常量
const (
	BannerPositionHomeHero = "home_hero"
)

// Banner 跳转类型常量
const (
	BannerLinkTypeNone     = "none"
	BannerLinkTypeInternal = "internal"
	BannerLinkTypeExternal = "external"
)
