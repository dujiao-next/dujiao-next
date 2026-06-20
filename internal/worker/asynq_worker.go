package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/htmltext"
	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/provider"
	"github.com/dujiao-next/internal/queue"
	"github.com/dujiao-next/internal/service"
	"github.com/dujiao-next/internal/telegramidentity"
	"github.com/dujiao-next/internal/upstream"

	"github.com/hibiken/asynq"
)

// Consumer 异步任务消费者
type Consumer struct {
	*provider.Container
}

// NewConsumer 创建消费者
func NewConsumer(c *provider.Container) *Consumer {
	return &Consumer{
		Container: c,
	}
}

// Register 注册消费者
func (c *Consumer) Register(mux *asynq.ServeMux) {
	if c == nil || mux == nil {
		logger.Debugw("worker_register_skip_nil", "consumer_nil", c == nil, "mux_nil", mux == nil)
		return
	}
	mux.HandleFunc(queue.TaskOrderStatusEmail, withPanicRecovery(queue.TaskOrderStatusEmail, c.handleOrderStatusEmail))
	mux.HandleFunc(queue.TaskOrderAutoFulfill, withPanicRecovery(queue.TaskOrderAutoFulfill, c.handleOrderAutoFulfill))
	mux.HandleFunc(queue.TaskOrderTimeoutCancel, withPanicRecovery(queue.TaskOrderTimeoutCancel, c.handleOrderTimeoutCancel))
	mux.HandleFunc(queue.TaskWalletRechargeExpire, withPanicRecovery(queue.TaskWalletRechargeExpire, c.handleWalletRechargeExpire))
	mux.HandleFunc(queue.TaskNotificationDispatch, withPanicRecovery(queue.TaskNotificationDispatch, c.handleNotificationDispatch))
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

// handleOrderStatusEmail 处理订单状态邮件发送任务。
func (c *Consumer) handleOrderStatusEmail(_ context.Context, task *asynq.Task) error {
	if c == nil || task == nil {
		logger.Debugw("worker_order_status_email_skip_nil", "consumer_nil", c == nil, "task_nil", task == nil)
		return nil
	}
	var payload queue.OrderStatusEmailPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		logger.Warnw("worker_order_status_email_unmarshal_failed", "error", err)
		return err
	}
	if payload.OrderID == 0 {
		logger.Debugw("worker_order_status_email_skip_invalid_payload", "order_id", payload.OrderID)
		return nil
	}
	// 消费侧二次校验开关：避免 SMTP/订单通知关闭后，旧任务（含 retry/scheduled 中的任务）继续触发发送。
	if c.SettingService != nil && c.Config != nil {
		smtpSetting, smtpErr := c.SettingService.GetSMTPSetting(c.Config.Email)
		if smtpErr != nil {
			// fail-closed：读不到设置就视作关闭，不重试，避免 DB 抖动放大成邮件雪崩。
			logger.Warnw("worker_order_status_email_load_smtp_setting_failed", "order_id", payload.OrderID, "error", smtpErr)
			return nil
		}
		if !smtpSetting.Enabled || !smtpSetting.OrderNotificationEnabled {
			logger.Debugw("worker_order_status_email_skip_setting_disabled",
				"order_id", payload.OrderID,
				"smtp_enabled", smtpSetting.Enabled,
				"order_notification_enabled", smtpSetting.OrderNotificationEnabled,
			)
			return nil
		}
	}
	order, err := c.OrderRepo.GetByID(payload.OrderID)
	if err != nil {
		logger.Warnw("worker_order_status_email_fetch_order_failed", "order_id", payload.OrderID, "error", err)
		return err
	}
	if order == nil {
		logger.Debugw("worker_order_status_email_skip_order_not_found", "order_id", payload.OrderID)
		return nil
	}
	// 游客订单的"取消"通知价值极低（用户大概率已离开），但极易被滥用为退信轰炸载体：
	// 攻击者可用伪造邮箱批量下单等过期取消触发发送。统一不发，保留登录用户与其它状态。
	if order.UserID == 0 {
		emailStatus := strings.TrimSpace(payload.Status)
		if emailStatus == "" {
			emailStatus = order.Status
		}
		if emailStatus == constants.OrderStatusCanceled {
			logger.Debugw("worker_order_status_email_skip_guest_canceled",
				"order_id", order.ID,
				"order_no", order.OrderNo,
			)
			return nil
		}
	}
	var receiverEmail string
	var locale string
	if order.UserID != 0 {
		user, err := c.UserRepo.GetByID(order.UserID)
		if err != nil {
			logger.Warnw("worker_order_status_email_fetch_user_failed", "order_id", order.ID, "user_id", order.UserID, "error", err)
			return err
		}
		if user != nil {
			receiverEmail = strings.TrimSpace(user.Email)
			locale = strings.TrimSpace(user.Locale)
		}
	} else {
		receiverEmail = strings.TrimSpace(order.GuestEmail)
		locale = strings.TrimSpace(order.GuestLocale)
	}
	if receiverEmail == "" {
		logger.Debugw("worker_order_status_email_skip_empty_receiver", "order_id", order.ID, "order_no", order.OrderNo)
		return nil
	}
	if telegramidentity.IsPlaceholderEmail(receiverEmail) {
		logger.Debugw("worker_order_status_email_skip_placeholder_receiver", "order_id", order.ID, "order_no", order.OrderNo)
		return nil
	}
	if c.EmailService == nil {
		logger.Warnw("worker_order_status_email_skip_email_service_nil", "order_id", order.ID, "order_no", order.OrderNo)
		return nil
	}
	var tmplSetting *service.OrderEmailTemplateSetting
	if c.SettingService != nil {
		setting, tmplErr := c.SettingService.GetOrderEmailTemplateSetting()
		if tmplErr != nil {
			logger.Warnw("worker_order_status_email_load_template_failed", "order_id", order.ID, "error", tmplErr)
		} else {
			tmplSetting = &setting
		}
	}
	status := strings.TrimSpace(payload.Status)
	if status == "" {
		status = order.Status
	}
	payloadText := buildOrderFulfillmentEmailPayload(order)
	siteBrand := service.SiteBrand{}
	if c.SettingService != nil {
		resolvedSiteBrand, siteErr := c.SettingService.GetSiteBrand()
		if siteErr != nil {
			logger.Warnw("worker_order_status_email_load_site_brand_failed", "order_id", order.ID, "error", siteErr)
		} else {
			siteBrand = resolvedSiteBrand
		}
	}
	refundDetails, refundDetailsErr := c.OrderRefundService.ResolveOrderStatusEmailRefundDetails(order, payload.RefundRecordID)
	if refundDetailsErr != nil {
		logger.Warnw("worker_order_status_email_resolve_refund_details_failed",
			"order_id", order.ID,
			"refund_record_id", payload.RefundRecordID,
			"error", refundDetailsErr,
		)
	}
	input := service.OrderStatusEmailInput{
		OrderNo:      order.OrderNo,
		Status:       status,
		Amount:       order.TotalAmount,
		RefundAmount: refundDetails.Amount,
		RefundReason: refundDetails.Reason,
		Currency:     order.Currency,
		SiteName:     siteBrand.SiteName,
		SiteURL:      siteBrand.SiteURL,
		IsGuest:      order.UserID == 0,
	}
	if models.ShouldAttachFulfillmentPayload(payloadText) {
		// 交付内容过大，正文不放交付内容，以附件形式发送
		input.AttachmentName = fmt.Sprintf("order_%s_delivery.txt", order.OrderNo)
		input.AttachmentContent = payloadText
	} else {
		input.FulfillmentInfo = payloadText
	}
	// 使用说明只在交付类场景追加，避免被追加到退款/取消等无关邮件正文里。
	if status == constants.OrderStatusDelivered || status == constants.OrderStatusCompleted {
		input.Instructions = buildOrderInstructionsEmailText(order, locale)
	}
	if err := c.EmailService.SendOrderStatusEmailWithTemplate(receiverEmail, input, locale, tmplSetting); err != nil {
		switch {
		case errors.Is(err, service.ErrEmailServiceDisabled):
			logger.Debugw("worker_order_status_email_skip_email_disabled",
				"order_id", order.ID,
				"order_no", order.OrderNo,
				"receiver_email", receiverEmail,
				"status", status,
			)
			return nil
		case errors.Is(err, service.ErrEmailServiceNotConfigured):
			logger.Debugw("worker_order_status_email_skip_email_not_configured",
				"order_id", order.ID,
				"order_no", order.OrderNo,
				"receiver_email", receiverEmail,
				"status", status,
			)
			return nil
		case errors.Is(err, service.ErrInvalidEmail):
			logger.Debugw("worker_order_status_email_skip_invalid_email",
				"order_id", order.ID,
				"order_no", order.OrderNo,
				"receiver_email", receiverEmail,
				"status", status,
			)
			return nil
		default:
			logger.Warnw("worker_order_status_email_send_failed",
				"order_id", order.ID,
				"order_no", order.OrderNo,
				"receiver_email", receiverEmail,
				"status", status,
				"error", err,
			)
			return err
		}
	}
	return nil
}

// handleOrderAutoFulfill 处理自动交付任务。
func (c *Consumer) handleOrderAutoFulfill(_ context.Context, task *asynq.Task) error {
	if c == nil || task == nil {
		logger.Debugw("worker_order_auto_fulfill_skip_nil", "consumer_nil", c == nil, "task_nil", task == nil)
		return nil
	}
	var payload queue.OrderAutoFulfillPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		logger.Warnw("worker_order_auto_fulfill_unmarshal_failed", "error", err)
		return err
	}
	if payload.OrderID == 0 {
		logger.Debugw("worker_order_auto_fulfill_skip_invalid_payload", "order_id", payload.OrderID)
		return nil
	}
	_, err := c.FulfillmentService.CreateAuto(payload.OrderID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrFulfillmentExists):
			logger.Debugw("worker_order_auto_fulfill_skip_exists", "order_id", payload.OrderID)
			return nil
		case errors.Is(err, service.ErrFulfillmentNotAuto):
			logger.Debugw("worker_order_auto_fulfill_skip_not_auto", "order_id", payload.OrderID)
			return nil
		case errors.Is(err, service.ErrOrderStatusInvalid):
			logger.Debugw("worker_order_auto_fulfill_skip_invalid_status", "order_id", payload.OrderID)
			return nil
		case errors.Is(err, service.ErrOrderNotFound):
			logger.Debugw("worker_order_auto_fulfill_skip_order_not_found", "order_id", payload.OrderID)
			return nil
		default:
			logger.Warnw("worker_order_auto_fulfill_failed", "order_id", payload.OrderID, "error", err)
			return err
		}
	}
	return nil
}

// handleOrderTimeoutCancel 处理超时未支付订单自动取消任务。
func (c *Consumer) handleOrderTimeoutCancel(_ context.Context, task *asynq.Task) error {
	if c == nil || task == nil {
		logger.Debugw("worker_order_timeout_cancel_skip_nil", "consumer_nil", c == nil, "task_nil", task == nil)
		return nil
	}
	var payload queue.OrderTimeoutCancelPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		logger.Warnw("worker_order_timeout_cancel_unmarshal_failed", "error", err)
		return err
	}
	if payload.OrderID == 0 {
		logger.Debugw("worker_order_timeout_cancel_skip_invalid_payload", "order_id", payload.OrderID)
		return nil
	}
	if c.OrderService == nil {
		logger.Warnw("worker_order_timeout_cancel_skip_order_service_nil", "order_id", payload.OrderID)
		return nil
	}
	_, err := c.OrderService.CancelExpiredOrder(payload.OrderID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrOrderNotFound):
			logger.Debugw("worker_order_timeout_cancel_skip_order_not_found", "order_id", payload.OrderID)
			return nil
		case errors.Is(err, service.ErrOrderFetchFailed):
			logger.Warnw("worker_order_timeout_cancel_fetch_failed", "order_id", payload.OrderID, "error", err)
			return nil
		case errors.Is(err, service.ErrOrderUpdateFailed):
			logger.Warnw("worker_order_timeout_cancel_update_failed", "order_id", payload.OrderID, "error", err)
			return err
		default:
			logger.Warnw("worker_order_timeout_cancel_failed", "order_id", payload.OrderID, "error", err)
			return err
		}
	}
	return nil
}

// handleWalletRechargeExpire 处理钱包充值订单过期任务。
func (c *Consumer) handleWalletRechargeExpire(_ context.Context, task *asynq.Task) error {
	if c == nil || task == nil {
		logger.Debugw("worker_wallet_recharge_expire_skip_nil", "consumer_nil", c == nil, "task_nil", task == nil)
		return nil
	}
	var payload queue.WalletRechargeExpirePayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		logger.Warnw("worker_wallet_recharge_expire_unmarshal_failed", "error", err)
		return err
	}
	if payload.PaymentID == 0 {
		logger.Debugw("worker_wallet_recharge_expire_skip_invalid_payload", "payment_id", payload.PaymentID)
		return nil
	}
	if c.PaymentService == nil {
		logger.Warnw("worker_wallet_recharge_expire_skip_payment_service_nil", "payment_id", payload.PaymentID)
		return nil
	}
	if _, err := c.PaymentService.ExpireWalletRechargePayment(payload.PaymentID); err != nil {
		switch {
		case errors.Is(err, service.ErrPaymentNotFound):
			logger.Debugw("worker_wallet_recharge_expire_skip_payment_not_found", "payment_id", payload.PaymentID)
			return nil
		case errors.Is(err, service.ErrWalletRechargeNotFound):
			logger.Debugw("worker_wallet_recharge_expire_skip_recharge_not_found", "payment_id", payload.PaymentID)
			return nil
		case errors.Is(err, service.ErrPaymentUpdateFailed):
			logger.Warnw("worker_wallet_recharge_expire_update_failed", "payment_id", payload.PaymentID, "error", err)
			return err
		default:
			logger.Warnw("worker_wallet_recharge_expire_failed", "payment_id", payload.PaymentID, "error", err)
			return err
		}
	}
	return nil
}

// handleNotificationDispatch 处理通知中心异步分发任务。
func (c *Consumer) handleNotificationDispatch(ctx context.Context, task *asynq.Task) error {
	if c == nil || task == nil {
		logger.Debugw("worker_notification_dispatch_skip_nil", "consumer_nil", c == nil, "task_nil", task == nil)
		return nil
	}
	if c.NotificationService == nil {
		logger.Warnw("worker_notification_dispatch_skip_service_nil")
		return nil
	}
	var payload queue.NotificationDispatchPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		logger.Warnw("worker_notification_dispatch_unmarshal_failed", "error", err)
		return err
	}
	if strings.TrimSpace(payload.EventType) == "" {
		logger.Debugw("worker_notification_dispatch_skip_empty_event")
		return nil
	}
	if err := c.NotificationService.Dispatch(ctx, payload); err != nil {
		logger.Warnw("worker_notification_dispatch_failed",
			"event_type", payload.EventType,
			"biz_type", payload.BizType,
			"biz_id", payload.BizID,
			"error", err,
		)
		return err
	}
	return nil
}

// handleAffiliateConfirmCommissions 处理分销佣金确认任务。
func (c *Consumer) handleAffiliateConfirmCommissions(_ context.Context, _ *asynq.Task) error {
	if c == nil || c.AffiliateService == nil {
		logger.Debugw("worker_affiliate_confirm_skip_nil", "consumer_nil", c == nil)
		return nil
	}
	if err := c.AffiliateService.ConfirmDueCommissions(time.Now()); err != nil {
		logger.Warnw("worker_affiliate_confirm_due_failed", "error", err)
		return err
	}
	return nil
}

func (c *Consumer) handleResellerConfirmLedger(_ context.Context, _ *asynq.Task) error {
	if c == nil || c.ResellerAccountingService == nil {
		logger.Debugw("worker_reseller_confirm_ledger_skip_nil", "consumer_nil", c == nil)
		return nil
	}
	affected, err := c.ResellerAccountingService.ConfirmDueLedgerEntries(time.Now())
	if err != nil {
		logger.Warnw("worker_reseller_confirm_ledger_failed", "error", err)
		return err
	}
	logger.Debugw("worker_reseller_confirm_ledger_ok", "affected", affected)
	return nil
}

// handleUpstreamSyncStock 处理上游库存同步任务。
func (c *Consumer) handleUpstreamSyncStock(_ context.Context, _ *asynq.Task) error {
	if c == nil || c.ProductMappingService == nil {
		logger.Debugw("worker_upstream_sync_stock_skip_nil", "consumer_nil", c == nil)
		return nil
	}
	cfg, _ := c.SettingService.GetUpstreamSyncConfig("5m")
	if err := c.ProductMappingService.SyncAllStock(cfg); err != nil {
		logger.Warnw("worker_upstream_sync_stock_failed", "error", err)
		return err
	}
	return nil
}

// handleProcurementSubmit 处理采购单提交上游任务。
func (c *Consumer) handleProcurementSubmit(_ context.Context, task *asynq.Task) error {
	if c == nil || task == nil || c.ProcurementOrderService == nil {
		logger.Debugw("worker_procurement_submit_skip_nil")
		return nil
	}
	var payload queue.ProcurementSubmitPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		logger.Warnw("worker_procurement_submit_unmarshal_failed", "error", err)
		return err
	}
	if payload.ProcurementOrderID == 0 {
		return nil
	}
	if err := c.ProcurementOrderService.SubmitToUpstream(payload.ProcurementOrderID); err != nil {
		logger.Warnw("worker_procurement_submit_failed",
			"procurement_order_id", payload.ProcurementOrderID,
			"error", err,
		)
		return err
	}
	return nil
}

// handleProcurementPollStatus 处理采购单轮询上游状态任务。
func (c *Consumer) handleProcurementPollStatus(_ context.Context, task *asynq.Task) error {
	if c == nil || task == nil || c.ProcurementOrderService == nil {
		logger.Debugw("worker_procurement_poll_skip_nil")
		return nil
	}
	var payload queue.ProcurementPollStatusPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		logger.Warnw("worker_procurement_poll_unmarshal_failed", "error", err)
		return err
	}
	if payload.ProcurementOrderID == 0 {
		return nil
	}
	if err := c.ProcurementOrderService.PollUpstreamStatus(payload.ProcurementOrderID); err != nil {
		logger.Warnw("worker_procurement_poll_failed",
			"procurement_order_id", payload.ProcurementOrderID,
			"error", err,
		)
		return err
	}
	return nil
}

// handleProcurementSyncAccepted 处理 accepted 采购单的定时巡检任务。
func (c *Consumer) handleProcurementSyncAccepted(_ context.Context, _ *asynq.Task) error {
	if c == nil || c.ProcurementOrderService == nil {
		logger.Debugw("worker_procurement_sync_accepted_skip_nil")
		return nil
	}
	c.ProcurementOrderService.SyncAcceptedOrders()
	return nil
}

// handleDownstreamCallback 处理下游回调发送任务。
func (c *Consumer) handleDownstreamCallback(_ context.Context, task *asynq.Task) error {
	if c == nil || task == nil || c.DownstreamCallbackService == nil {
		logger.Debugw("worker_downstream_callback_skip_nil")
		return nil
	}
	var payload queue.DownstreamCallbackPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		logger.Warnw("worker_downstream_callback_unmarshal_failed", "error", err)
		return err
	}
	if payload.DownstreamOrderRefID == 0 {
		return nil
	}
	if err := c.DownstreamCallbackService.SendCallback(payload.DownstreamOrderRefID); err != nil {
		logger.Warnw("worker_downstream_callback_failed",
			"ref_id", payload.DownstreamOrderRefID,
			"error", err,
		)
		return err
	}
	return nil
}

// handleReconciliationRun 处理对账任务执行。
func (c *Consumer) handleReconciliationRun(ctx context.Context, task *asynq.Task) error {
	if c == nil || task == nil || c.ReconciliationService == nil {
		logger.Debugw("worker_reconciliation_run_skip_nil")
		return nil
	}
	var payload queue.ReconciliationRunPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		logger.Warnw("worker_reconciliation_run_unmarshal_failed", "error", err)
		return err
	}
	if payload.JobID == 0 {
		return nil
	}
	if err := c.ReconciliationService.Execute(ctx, payload.JobID); err != nil {
		logger.Warnw("worker_reconciliation_run_failed",
			"job_id", payload.JobID,
			"error", err,
		)
		return err
	}
	return nil
}

// buildOrderInstructionsEmailText 收集订单项的交付使用说明（多语言选取 + HTML 去标签 + 去重）。
func buildOrderInstructionsEmailText(order *models.Order, locale string) string {
	if order == nil {
		return ""
	}
	seen := make(map[string]struct{})
	var parts []string
	add := func(raw models.JSON) {
		text := localizedInstructionsText(raw, locale)
		if text == "" {
			return
		}
		plain := htmltext.StripToPlainText(text)
		if plain == "" {
			return
		}
		if _, ok := seen[plain]; ok {
			return
		}
		seen[plain] = struct{}{}
		parts = append(parts, plain)
	}
	for _, item := range order.Items {
		add(item.InstructionsJSON)
	}
	for _, child := range order.Children {
		for _, item := range child.Items {
			add(item.InstructionsJSON)
		}
	}
	return strings.Join(parts, "\n\n")
}

func localizedInstructionsText(raw models.JSON, locale string) string {
	if len(raw) == 0 {
		return ""
	}
	candidates := []string{strings.TrimSpace(locale), "zh-CN", "en-US", "zh-TW"}
	for _, key := range candidates {
		if key == "" {
			continue
		}
		if value, ok := raw[key]; ok {
			if s, ok := value.(string); ok {
				if trimmed := strings.TrimSpace(s); trimmed != "" {
					return trimmed
				}
			}
		}
	}
	return ""
}

// buildOrderFulfillmentEmailPayload 组装订单状态邮件中的交付内容文本。
func buildOrderFulfillmentEmailPayload(order *models.Order) string {
	if order == nil {
		return ""
	}
	if order.Fulfillment != nil {
		payload := strings.TrimSpace(order.Fulfillment.Payload)
		if payload != "" {
			return payload
		}
	}
	if len(order.Children) == 0 {
		return ""
	}
	parts := make([]string, 0, len(order.Children))
	for _, child := range order.Children {
		if child.Fulfillment == nil {
			continue
		}
		content := strings.TrimSpace(child.Fulfillment.Payload)
		if content == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("[%s]\n%s", strings.TrimSpace(child.OrderNo), content))
	}
	return strings.Join(parts, "\n\n")
}

// handleBotNotify 处理 Telegram Bot 事件回调任务。
func (c *Consumer) handleBotNotify(_ context.Context, task *asynq.Task) error {
	if c == nil || task == nil {
		return nil
	}
	var payload queue.BotNotifyPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		logger.Warnw("worker_bot_notify_unmarshal_failed", "error", err)
		return fmt.Errorf("unmarshal bot notify payload: %w", err)
	}
	if strings.TrimSpace(payload.TelegramUserID) == "" {
		logger.Debugw("worker_bot_notify_skip_invalid", "event_type", payload.EventType, "order_id", payload.OrderID, "telegram_user_id", payload.TelegramUserID, "recharge_no", payload.RechargeNo)
		return nil
	}

	// 从 DB 读取 telegram_bot 类型的活跃 ChannelClient
	channelClient, err := c.ChannelClientRepo.FindActiveByChannelType("telegram_bot")
	if err != nil {
		logger.Warnw("worker_bot_notify_find_channel_client_failed", "error", err)
		return fmt.Errorf("find telegram bot channel client: %w", err)
	}
	if channelClient == nil || channelClient.CallbackURL == "" {
		logger.Debugw("worker_bot_notify_skip_no_channel_client")
		return nil
	}

	// 解密 ChannelSecret
	plainSecret, err := c.ChannelClientService.DecryptChannelSecret(channelClient)
	if err != nil {
		logger.Warnw("worker_bot_notify_decrypt_secret_failed", "error", err)
		return fmt.Errorf("decrypt channel secret: %w", err)
	}

	timestamp := time.Now().Unix()
	path := "/internal/order-fulfilled"
	requestBody := map[string]interface{}{
		"order_id":         payload.OrderID,
		"telegram_user_id": payload.TelegramUserID,
	}
	switch payload.EventType {
	case queue.BotNotifyEventOrderPaid:
		if payload.OrderID == 0 {
			logger.Debugw("worker_bot_notify_skip_invalid", "event_type", payload.EventType, "order_id", payload.OrderID, "telegram_user_id", payload.TelegramUserID)
			return nil
		}
		path = "/internal/order-paid"
	case "", queue.BotNotifyEventOrderFulfilled:
		if payload.OrderID == 0 {
			logger.Debugw("worker_bot_notify_skip_invalid", "event_type", payload.EventType, "order_id", payload.OrderID, "telegram_user_id", payload.TelegramUserID)
			return nil
		}
	case queue.BotNotifyEventWalletRechargeSucceeded:
		if strings.TrimSpace(payload.RechargeNo) == "" {
			logger.Debugw("worker_bot_notify_skip_invalid", "event_type", payload.EventType, "telegram_user_id", payload.TelegramUserID, "recharge_no", payload.RechargeNo)
			return nil
		}
		path = "/internal/wallet-recharge-succeeded"
		requestBody = map[string]interface{}{
			"recharge_no":      payload.RechargeNo,
			"telegram_user_id": payload.TelegramUserID,
			"amount":           payload.Amount,
			"currency":         payload.Currency,
		}
	default:
		logger.Debugw("worker_bot_notify_skip_unknown_event", "event_type", payload.EventType)
		return nil
	}
	body, _ := json.Marshal(requestBody)
	signature := upstream.Sign(plainSecret, "POST", path, timestamp, body)

	requestURL, err := buildBotNotifyRequestURL(channelClient.CallbackURL, path)
	if err != nil {
		return fmt.Errorf("build bot notify request url: %w", err)
	}

	req, err := http.NewRequest("POST", requestURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create bot notify request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Dujiao-Next-Channel-Key", channelClient.ChannelKey)
	req.Header.Set("Dujiao-Next-Channel-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("Dujiao-Next-Channel-Signature", signature)

	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		logger.Warnw("worker_bot_notify_request_failed",
			"order_id", payload.OrderID, "telegram_user_id", payload.TelegramUserID, "error", err)
		return fmt.Errorf("bot notify request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		logger.Debugw("worker_bot_notify_sent",
			"order_id", payload.OrderID, "telegram_user_id", payload.TelegramUserID)
		return nil
	}

	// 4xx 客户端错误不重试
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		logger.Warnw("worker_bot_notify_client_error",
			"order_id", payload.OrderID, "status", resp.StatusCode)
		return nil
	}

	// 5xx 返回 error 触发 asynq 重试
	return fmt.Errorf("bot notify unexpected status: %d", resp.StatusCode)
}

// buildBotNotifyRequestURL 构建 Bot 回调请求URL并重置查询参数。
func buildBotNotifyRequestURL(rawURL string, path string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid callback url: %s", rawURL)
	}

	parsed.Path = path
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""

	return parsed.String(), nil
}

// handleTelegramBroadcast 处理 Telegram 群发任务。
func (c *Consumer) handleTelegramBroadcast(ctx context.Context, task *asynq.Task) error {
	if c == nil || task == nil {
		return nil
	}
	var payload queue.TelegramBroadcastPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		logger.Warnw("worker_telegram_broadcast_unmarshal_failed", "error", err)
		return err
	}
	if payload.BroadcastID == 0 {
		logger.Debugw("worker_telegram_broadcast_skip_invalid")
		return nil
	}
	if c.TelegramBroadcastService == nil {
		return nil
	}
	return c.TelegramBroadcastService.ProcessBroadcast(ctx, payload.BroadcastID)
}
