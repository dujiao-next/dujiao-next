package consumer

import (
	"github.com/dujiao-next/internal/app/container"
	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/queue"
	"github.com/hibiken/asynq"

	notificationcontract "github.com/dujiao-next/internal/modules/notification/contract"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	settingsmessaging "github.com/dujiao-next/internal/modules/settings/schema/messaging"
)

// Consumer 异步任务消费者
type Consumer struct {
	*container.Container
	orderReader orderReader
	emailSender orderStatusEmailSender
}

// orderStatusEmailSender 为订单及充值退款通知提供状态邮件发送能力。
type orderStatusEmailSender interface {
	SendOrderStatusEmailWithTemplate(string, notificationcontract.OrderStatusEmailInput, string, *settingsmessaging.OrderEmailTemplateSetting) error
}

type orderReader interface {
	GetByID(id uint) (*orderdomain.Order, error)
}

// resolveStatusEmailSender 优先使用注入的发送器，否则使用应用装配的邮件服务。
func (c *Consumer) resolveStatusEmailSender() orderStatusEmailSender {
	if c == nil {
		return nil
	}
	if c.emailSender != nil {
		return c.emailSender
	}
	if c.Container == nil || c.EmailSender == nil {
		return nil
	}
	return c.EmailSender
}

// New 创建消费者
func New(c *container.Container) *Consumer {
	var orders orderReader
	if c != nil {
		orders = c.OrderStore
	}
	return &Consumer{
		Container:   c,
		orderReader: orders,
	}
}

// Register 注册消费者
func (c *Consumer) Register(mux *asynq.ServeMux) {
	if c == nil || mux == nil {
		logger.Debugw("worker_register_skip_nil", "consumer_nil", c == nil, "mux_nil", mux == nil)
		return
	}
	mux.HandleFunc(queue.TaskOrderStatusEmail, withPanicRecovery(queue.TaskOrderStatusEmail, c.handleOrderStatusEmail))
	mux.HandleFunc(queue.TaskWalletRechargeRefundEmail, withPanicRecovery(queue.TaskWalletRechargeRefundEmail, c.handleWalletRechargeRefundEmail))
	mux.HandleFunc(queue.TaskOrderAutoFulfill, withPanicRecovery(queue.TaskOrderAutoFulfill, c.handleOrderAutoFulfill))
	mux.HandleFunc(queue.TaskOrderTimeoutCancel, withPanicRecovery(queue.TaskOrderTimeoutCancel, c.handleOrderTimeoutCancel))
	mux.HandleFunc(queue.TaskWalletRechargeExpire, withPanicRecovery(queue.TaskWalletRechargeExpire, c.handleWalletRechargeExpire))
	mux.HandleFunc(queue.TaskNotificationDispatch, withPanicRecovery(queue.TaskNotificationDispatch, c.handleNotificationDispatch))
	mux.HandleFunc(queue.TaskAffiliateOrderPaid, withPanicRecovery(queue.TaskAffiliateOrderPaid, c.handleAffiliateOrderPaid))
	mux.HandleFunc(queue.TaskAffiliateConfirmCommissions, withPanicRecovery(queue.TaskAffiliateConfirmCommissions, c.handleAffiliateConfirmCommissions))
	mux.HandleFunc(queue.TaskResellerConfirmLedger, withPanicRecovery(queue.TaskResellerConfirmLedger, c.handleResellerConfirmLedger))
	mux.HandleFunc(queue.TaskUpstreamSyncStock, withPanicRecovery(queue.TaskUpstreamSyncStock, c.handleUpstreamSyncStock))
	mux.HandleFunc(queue.TaskProcurementSubmit, withPanicRecovery(queue.TaskProcurementSubmit, c.handleProcurementSubmit))
	mux.HandleFunc(queue.TaskProcurementPollStatus, withPanicRecovery(queue.TaskProcurementPollStatus, c.handleProcurementPollStatus))
	mux.HandleFunc(queue.TaskProcurementSyncAccepted, withPanicRecovery(queue.TaskProcurementSyncAccepted, c.handleProcurementSyncAccepted))
	mux.HandleFunc(queue.TaskDownstreamCallback, withPanicRecovery(queue.TaskDownstreamCallback, c.handleDownstreamCallback))
	mux.HandleFunc(queue.TaskReconciliationRun, withPanicRecovery(queue.TaskReconciliationRun, c.handleReconciliationRun))
	mux.HandleFunc(queue.TaskBotNotify, withPanicRecovery(queue.TaskBotNotify, c.handleBotNotify))
	mux.HandleFunc(queue.TaskTelegramBroadcast, withPanicRecovery(queue.TaskTelegramBroadcast, c.handleTelegramBroadcast))
}
