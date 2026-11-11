package application

import (
	"fmt"
	"strings"
	"time"

	settingscontract "github.com/dujiao-next/internal/modules/settings/contract"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
)

const defaultCurrency = "CNY"

type Options struct {
	Repository   walletcontract.Repository
	Transactions walletcontract.UnitOfWork
	RefundConfig settingscontract.OrderRefundConfigReader
}

type Service struct {
	repository   walletcontract.Repository
	transactions walletcontract.UnitOfWork
	refundConfig settingscontract.OrderRefundConfigReader
}

var _ walletcontract.UseCase = (*Service)(nil)

func NewService(options Options) *Service {
	return &Service{repository: options.Repository, transactions: options.Transactions, refundConfig: options.RefundConfig}
}

// getRechargeRefundConfig 在进入钱包事务前读取退款期限配置。
func (s *Service) getRechargeRefundConfig() (settingscontract.OrderRefundConfig, error) {
	if s.refundConfig == nil {
		return settingscontract.DefaultOrderRefundConfig(), nil
	}
	return s.refundConfig.GetOrderRefundConfig()
}

func normalizeCurrency(currency string) string {
	normalized := strings.ToUpper(strings.TrimSpace(currency))
	if normalized == "" {
		return defaultCurrency
	}
	return normalized
}

func cleanRemark(raw, fallback string) string {
	remark := strings.TrimSpace(raw)
	if remark == "" {
		return fallback
	}
	return remark
}

// orderAllocationReference 生成订单余额分配的轮次幂等键。
//
// 第 1 轮沿用历史格式（order:<id>:<action>），升级前已经扣款的在途订单不会被重复扣款；
// 订单在"用余额 → 改在线（退回）→ 再用余额"之间来回切换时，后续轮次追加序号，
// 否则上一轮早已被退回的流水会把新一轮的扣款判成重复操作而跳过——
// 订单会被标记为已用余额，钱包却一分钱都没扣。
//
// 同一轮内的重复调用由订单侧的 wallet_paid_amount 与 Release 的原子占位拦截，
// 并发写入还有 wallet_transactions.reference 唯一索引兜底。
func orderAllocationReference(repository walletcontract.Repository, orderID uint, action string) (string, error) {
	base := orderReference(orderID, action)
	if repository == nil || orderID == 0 {
		return base, nil
	}
	count, err := repository.CountOrderTransactionsByType(orderID, action)
	if err != nil {
		return "", err
	}
	if count <= 0 {
		return base, nil
	}
	return fmt.Sprintf("%s:%d", base, count+1), nil
}

func orderReference(orderID uint, action string) string {
	action = strings.TrimSpace(action)
	if action == "" {
		action = "wallet"
	}
	return fmt.Sprintf("order:%d:%s", orderID, action)
}

func uniqueReference(prefix string, id uint) string {
	normalized := strings.TrimSpace(prefix)
	if normalized == "" {
		normalized = "wallet"
	}
	return fmt.Sprintf("%s:%d:%d", normalized, id, time.Now().UnixNano())
}
