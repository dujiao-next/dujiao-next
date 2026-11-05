package consumer

import (
	"context"
	"errors"
	"github.com/dujiao-next/internal/app/container"
	"github.com/dujiao-next/internal/logger"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	"github.com/dujiao-next/internal/queue"

	"github.com/hibiken/asynq"
)

// Consumer 异步任务消费者
type Consumer struct {
	*container.Container
	orderReader orderReader
}

type orderReader interface {
	GetByID(id uint) (*orderdomain.Order, error)
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
	mux.HandleFunc(queue.TaskPaymentChainSettle, withPanicRecovery(queue.TaskPaymentChainSettle, c.handlePaymentChainSettle))
	mux.HandleFunc(queue.TaskPaymentSecurityAlerts, withPanicRecovery(queue.TaskPaymentSecurityAlerts, c.handlePaymentSecurityAlerts))
	mux.HandleFunc(queue.TaskPaymentEffects, withPanicRecovery(queue.TaskPaymentEffects, c.handlePaymentEffects))
	mux.HandleFunc(queue.TaskPaymentChainVerify, withPanicRecovery(queue.TaskPaymentChainVerify, c.handlePaymentChainVerify))
	mux.HandleFunc(queue.TaskPaymentCandidateSync, withPanicRecovery(queue.TaskPaymentCandidateSync, c.handlePaymentCandidateSync))
	mux.HandleFunc(queue.TaskOrderStatusEmail, withPanicRecovery(queue.TaskOrderStatusEmail, c.handleOrderStatusEmail))
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

func (c *Consumer) handlePaymentCandidateSync(ctx context.Context, _ *asynq.Task) error {
	if c.PaymentCandidateService == nil {
		return errors.New("payment candidate service unavailable")
	}
	return c.PaymentCandidateService.Run(ctx)
}

func (c *Consumer) handlePaymentChainVerify(ctx context.Context, _ *asynq.Task) error {
	if c.PaymentChainVerificationService == nil {
		return errors.New("payment chain verification unavailable")
	}
	return c.PaymentChainVerificationService.Run(ctx)
}

func (c *Consumer) handlePaymentChainSettle(ctx context.Context, _ *asynq.Task) error {
	if c.PaymentSettlementService == nil {
		return errors.New("payment settlement unavailable")
	}
	return c.PaymentSettlementService.Run(ctx)
}
func (c *Consumer) handlePaymentEffects(ctx context.Context, _ *asynq.Task) error {
	if c.PaymentEffectsService == nil {
		return errors.New("payment effects unavailable")
	}
	runner := *c.PaymentEffectsService
	if c.PaymentService == nil {
		return errors.New("payment effects handler unavailable")
	}
	runner.Handler = c.PaymentService.WithEffectDelivery(c)
	return runner.Run(ctx)
}

func (c *Consumer) handlePaymentSecurityAlerts(ctx context.Context, _ *asynq.Task) error {
	if c.PaymentSecurityAlertsService == nil {
		return errors.New("payment security alerts unavailable")
	}
	return c.PaymentSecurityAlertsService.Run(ctx)
}
