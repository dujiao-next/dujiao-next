package contract

import (
	"time"

	ordercontract "github.com/dujiao-next/internal/modules/order/contract"
)

const (
	BotNotificationOrderPaid               = "order_paid"
	BotNotificationWalletRechargeSucceeded = "wallet_recharge_succeeded"
)

// BotNotification 是支付成功后发送给 Telegram Bot 的最小任务载荷。
type BotNotification struct {
	EventType      string
	OrderID        uint
	TelegramUserID string
	RechargeNo     string
	Amount         string
	Currency       string
}

// Queue 聚合支付流程需要的异步任务能力，并复用订单状态邮件端口。
type Queue interface {
	ordercontract.Queue
	EnqueueOrderAutoFulfill(orderID uint) error
	EnqueueAffiliateOrderPaid(orderID uint) error
	EnqueueBotNotification(input BotNotification) error
	EnqueueWalletRechargeExpire(paymentID uint, delay time.Duration) error
}
