package consumer

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/queue"

	"github.com/hibiken/asynq"
)

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

// handleAffiliateOrderPaid 重试支付后失败的佣金创建；返回错误交由队列按退避重试。
func (c *Consumer) handleAffiliateOrderPaid(_ context.Context, task *asynq.Task) error {
	if c == nil || task == nil || c.AffiliateService == nil {
		return nil
	}
	var payload queue.AffiliateOrderPaidPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil || payload.OrderID == 0 {
		return nil
	}
	return c.AffiliateService.HandleOrderPaid(payload.OrderID)
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
	if c == nil || c.ResellerAccountingLedger == nil {
		logger.Debugw("worker_reseller_confirm_ledger_skip_nil", "consumer_nil", c == nil)
		return nil
	}
	affected, err := c.ResellerAccountingLedger.ConfirmDueLedgerEntries(time.Now())
	if err != nil {
		logger.Warnw("worker_reseller_confirm_ledger_failed", "error", err)
		return err
	}
	logger.Debugw("worker_reseller_confirm_ledger_ok", "affected", affected)
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
