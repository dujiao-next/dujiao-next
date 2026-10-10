package contract

import "errors"

var (
	// ErrNotOpened 表示用户尚未开通分销商资料。
	ErrNotOpened = errors.New("reseller not opened")
	// ErrProfileInactive 表示分销商资料未处于可用状态。
	ErrProfileInactive = errors.New("reseller profile inactive")
	// ErrApplyDisabled 表示未开启自助申请。
	ErrApplyDisabled = errors.New("reseller apply disabled")
	// ErrProfileStatusInvalid 表示资料状态流转不合法。
	ErrProfileStatusInvalid = errors.New("reseller profile status invalid")
	// ErrDomainInvalid 表示域名格式不合法。
	ErrDomainInvalid = errors.New("reseller domain invalid")
	// ErrDomainConflict 表示域名已被占用。
	ErrDomainConflict = errors.New("reseller domain conflict")
	// ErrDomainStatusInvalid 表示域名状态流转不合法。
	ErrDomainStatusInvalid = errors.New("reseller domain status invalid")
	// ErrSubdomainBaseMissing 表示系统子域基址未配置。
	ErrSubdomainBaseMissing = errors.New("reseller subdomain base missing")
	// ErrDomainMainHostNotAllowed 表示域名命中主站 Host 保留区。
	ErrDomainMainHostNotAllowed = errors.New("reseller domain main host not allowed")
	// ErrSiteConfigInvalid 表示站点配置校验失败。
	ErrSiteConfigInvalid = errors.New("reseller site config invalid")
	// ErrSiteConfigNotFound 表示站点配置不存在。
	ErrSiteConfigNotFound = errors.New("reseller site config not found")
	// ErrPriceBelowBase 表示分销价低于底价或成本价。
	ErrPriceBelowBase = errors.New("reseller price below base")
	// ErrPriceBelowMasterPrice 表示分销售价低于主站销售价格。
	ErrPriceBelowMasterPrice = errors.New("reseller price below master price")
	// ErrMarkupExceeded 表示加价超过资料上限。
	ErrMarkupExceeded = errors.New("reseller markup exceeded")
	// ErrPricingModeInvalid 表示定价模式不合法。
	ErrPricingModeInvalid = errors.New("reseller pricing mode invalid")
	// ErrSettlementUnavailable 表示结算状态不可用。
	ErrSettlementUnavailable = errors.New("reseller settlement unavailable")
	// ErrWithdrawAmountInvalid 表示提现金额不合法。
	ErrWithdrawAmountInvalid = errors.New("reseller withdraw amount invalid")
	// ErrWithdrawInsufficient 表示可提现余额不足。
	ErrWithdrawInsufficient = errors.New("reseller withdraw insufficient")
	// ErrWithdrawCurrencyUnavailable 表示提现币种不可用。
	ErrWithdrawCurrencyUnavailable = errors.New("reseller withdraw currency unavailable")
	// ErrWithdrawStatusInvalid 表示提现状态流转不合法。
	ErrWithdrawStatusInvalid = errors.New("reseller withdraw status invalid")
	// ErrBalanceAccountFrozen 表示余额账户已冻结。
	ErrBalanceAccountFrozen = errors.New("reseller balance account frozen")
	// ErrOrderNotFound 表示分销视角下订单不存在。
	ErrOrderNotFound = errors.New("order not found")
	// ErrAccountingUnavailable 表示分销账务服务不可用。
	ErrAccountingUnavailable = errors.New("reseller accounting unavailable")
	// ErrLedgerInvalidSnapshot 表示订单快照不足以生成账务流水。
	ErrLedgerInvalidSnapshot = errors.New("reseller ledger invalid snapshot")
)
