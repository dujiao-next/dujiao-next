package queue

import (
	"encoding/json"

	"github.com/dujiao-next/internal/constants"

	"github.com/hibiken/asynq"
)

const (
	// TaskOrderStatusEmail 订单状态邮件通知任务
	TaskOrderStatusEmail = constants.TaskOrderStatusEmail
	// TaskOrderAutoFulfill 自动交付任务
	TaskOrderAutoFulfill = constants.TaskOrderAutoFulfill
	// TaskOrderTimeoutCancel 超时取消任务
	TaskOrderTimeoutCancel = constants.TaskOrderTimeoutCancel
	// TaskWalletRechargeExpire 钱包充值超时过期任务
	TaskWalletRechargeExpire = constants.TaskWalletRechargeExpire
	// TaskNotificationDispatch 通知中心分发任务
	TaskNotificationDispatch = constants.TaskNotificationDispatch
	// TaskAffiliateConfirmCommissions 佣金到期确认任务
	TaskAffiliateConfirmCommissions = constants.TaskAffiliateConfirmCommissions
	// TaskAffiliateOrderPaid 支付后佣金创建失败时的重试任务
	TaskAffiliateOrderPaid = constants.TaskAffiliateOrderPaid
	// TaskResellerConfirmLedger 分销商账务到期确认任务
	TaskResellerConfirmLedger = constants.TaskResellerConfirmLedger
	// TaskUpstreamSyncStock 上游库存同步任务
	TaskUpstreamSyncStock = constants.TaskUpstreamSyncStock
	// TaskProcurementSubmit 采购提交任务
	TaskProcurementSubmit = constants.TaskProcurementSubmit
	// TaskProcurementPollStatus 采购状态轮询任务
	TaskProcurementPollStatus = constants.TaskProcurementPollStatus
	// TaskProcurementSyncAccepted 采购单定时巡检任务
	TaskProcurementSyncAccepted = constants.TaskProcurementSyncAccepted
	// TaskDownstreamCallback 下游回调通知任务
	TaskDownstreamCallback = constants.TaskDownstreamCallback
	// TaskReconciliationRun 对账执行任务
	TaskReconciliationRun = constants.TaskReconciliationRun
	// TaskBotNotify Bot 交付通知任务
	TaskBotNotify = constants.TaskBotNotify
	// TaskTelegramBroadcast Telegram 群发任务
	TaskTelegramBroadcast = constants.TaskTelegramBroadcast
)

// OrderStatusEmailPayload 订单状态邮件任务载荷
type OrderStatusEmailPayload struct {
	OrderID        uint   `json:"order_id"`
	RefundRecordID uint   `json:"refund_record_id,omitempty"`
	Status         string `json:"status"`
}

// OrderAutoFulfillPayload 自动交付任务载荷
type OrderAutoFulfillPayload struct {
	OrderID uint `json:"order_id"`
}

// AffiliateOrderPaidPayload 佣金创建重试任务载荷
type AffiliateOrderPaidPayload struct {
	OrderID uint `json:"order_id"`
}

// OrderTimeoutCancelPayload 超时取消任务载荷
type OrderTimeoutCancelPayload struct {
	OrderID uint `json:"order_id"`
}

// WalletRechargeExpirePayload 钱包充值超时过期任务载荷
type WalletRechargeExpirePayload struct {
	PaymentID uint `json:"payment_id"`
}

// NotificationDispatchPayload 通知中心分发任务载荷
type NotificationDispatchPayload struct {
	EventType string                 `json:"event_type"`
	BizType   string                 `json:"biz_type"`
	BizID     uint                   `json:"biz_id"`
	Locale    string                 `json:"locale"`
	Force     bool                   `json:"force"`
	Data      map[string]interface{} `json:"data"`
}

// NewOrderStatusEmailTask 创建订单状态邮件任务
func NewOrderStatusEmailTask(payload OrderStatusEmailPayload) (*asynq.Task, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TaskOrderStatusEmail, body), nil
}

// NewOrderAutoFulfillTask 创建自动交付任务
func NewOrderAutoFulfillTask(payload OrderAutoFulfillPayload) (*asynq.Task, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TaskOrderAutoFulfill, body), nil
}

// NewAffiliateOrderPaidTask 创建佣金创建重试任务
func NewAffiliateOrderPaidTask(payload AffiliateOrderPaidPayload) (*asynq.Task, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TaskAffiliateOrderPaid, body), nil
}

// NewOrderTimeoutCancelTask 创建超时取消任务
func NewOrderTimeoutCancelTask(payload OrderTimeoutCancelPayload) (*asynq.Task, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TaskOrderTimeoutCancel, body), nil
}

// NewWalletRechargeExpireTask 创建钱包充值超时过期任务
func NewWalletRechargeExpireTask(payload WalletRechargeExpirePayload) (*asynq.Task, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TaskWalletRechargeExpire, body), nil
}

// NewNotificationDispatchTask 创建通知中心分发任务
func NewNotificationDispatchTask(payload NotificationDispatchPayload) (*asynq.Task, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TaskNotificationDispatch, body), nil
}

// NewNotificationInventoryAlertCheckTask 创建库存告警巡检任务
func NewNotificationInventoryAlertCheckTask() (*asynq.Task, error) {
	return NewNotificationDispatchTask(NotificationDispatchPayload{
		EventType: constants.NotificationEventExceptionAlertCheck,
		BizType:   constants.NotificationBizTypeDashboardAlert,
		BizID:     0,
		Data: map[string]interface{}{
			"message": "scheduled_inventory_alert_check",
		},
	})
}

// NewAffiliateConfirmCommissionsTask 创建佣金到期确认任务
func NewAffiliateConfirmCommissionsTask() *asynq.Task {
	return asynq.NewTask(TaskAffiliateConfirmCommissions, nil)
}

// NewResellerConfirmLedgerTask 创建分销商账务到期确认任务。
func NewResellerConfirmLedgerTask() *asynq.Task {
	return asynq.NewTask(TaskResellerConfirmLedger, nil)
}

// NewUpstreamSyncStockTask 创建上游库存同步任务
func NewUpstreamSyncStockTask() *asynq.Task {
	return asynq.NewTask(TaskUpstreamSyncStock, nil)
}

// NewProcurementSyncAcceptedTask 创建采购单定时巡检任务
func NewProcurementSyncAcceptedTask() *asynq.Task {
	return asynq.NewTask(TaskProcurementSyncAccepted, nil)
}

// ProcurementSubmitPayload 采购提交任务载荷
type ProcurementSubmitPayload struct {
	ProcurementOrderID uint `json:"procurement_order_id"`
}

// NewProcurementSubmitTask 创建采购提交任务
func NewProcurementSubmitTask(payload ProcurementSubmitPayload) (*asynq.Task, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TaskProcurementSubmit, body), nil
}

// ProcurementPollStatusPayload 采购状态轮询任务载荷
type ProcurementPollStatusPayload struct {
	ProcurementOrderID uint `json:"procurement_order_id"`
}

// NewProcurementPollStatusTask 创建采购状态轮询任务
func NewProcurementPollStatusTask(payload ProcurementPollStatusPayload) (*asynq.Task, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TaskProcurementPollStatus, body), nil
}

// ReconciliationRunPayload 对账执行任务载荷
type ReconciliationRunPayload struct {
	JobID uint `json:"job_id"`
}

// NewReconciliationRunTask 创建对账执行任务
func NewReconciliationRunTask(payload ReconciliationRunPayload) (*asynq.Task, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TaskReconciliationRun, body), nil
}

// DownstreamCallbackPayload 下游回调通知任务载荷
type DownstreamCallbackPayload struct {
	DownstreamOrderRefID uint `json:"downstream_order_ref_id"`
}

// NewDownstreamCallbackTask 创建下游回调通知任务
func NewDownstreamCallbackTask(payload DownstreamCallbackPayload) (*asynq.Task, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TaskDownstreamCallback, body), nil
}

// BotNotifyPayload Bot 交付通知任务载荷
type BotNotifyPayload struct {
	EventType      string `json:"event_type,omitempty"`
	OrderID        uint   `json:"order_id"`
	TelegramUserID string `json:"telegram_user_id"`
	RechargeNo     string `json:"recharge_no,omitempty"`
	Amount         string `json:"amount,omitempty"`
	Currency       string `json:"currency,omitempty"`
}

const (
	// BotNotifyEventOrderPaid 订单支付成功通知事件。
	BotNotifyEventOrderPaid = "order_paid"
	// BotNotifyEventOrderFulfilled 订单交付通知事件。
	BotNotifyEventOrderFulfilled = "order_fulfilled"
	// BotNotifyEventWalletRechargeSucceeded 钱包充值成功通知事件。
	BotNotifyEventWalletRechargeSucceeded = "wallet_recharge_succeeded"
)

// NewBotNotifyTask 创建 Bot 交付通知任务
func NewBotNotifyTask(payload BotNotifyPayload) (*asynq.Task, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TaskBotNotify, body), nil
}

// TelegramBroadcastPayload Telegram 群发任务载荷。
type TelegramBroadcastPayload struct {
	BroadcastID uint `json:"broadcast_id"`
}

// NewTelegramBroadcastTask 创建 Telegram 群发任务。
func NewTelegramBroadcastTask(payload TelegramBroadcastPayload) (*asynq.Task, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TaskTelegramBroadcast, body), nil
}
