package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dujiao-next/internal/cache"
	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	procurementcontract "github.com/dujiao-next/internal/modules/procurement/contract"
	procurementdomain "github.com/dujiao-next/internal/modules/procurement/domain"
)

// pollIntervals 短期轮询间隔：捕获自动交付等快速场景（共约30分钟后停止）
// 超时后不标记失败，交由回调和定时巡检接管
var pollIntervals = []time.Duration{
	30 * time.Second, 30 * time.Second,
	1 * time.Minute, 1 * time.Minute,
	2 * time.Minute, 2 * time.Minute,
	5 * time.Minute, 5 * time.Minute,
	10 * time.Minute,
}

// PollUpstreamStatus Worker 调用：轮询上游订单状态
func (s *Service) PollUpstreamStatus(procurementOrderID uint) error {
	procOrder, err := s.procRepo.GetByID(procurementOrderID)
	if err != nil {
		return fmt.Errorf("load procurement order: %w", err)
	}
	if procOrder == nil {
		return procurementcontract.ErrNotFound
	}

	// 只轮询 accepted 状态的订单
	if procOrder.Status != "accepted" {
		return nil
	}

	connection, err := s.connections.Open(procOrder.ConnectionID)
	if err != nil {
		return fmt.Errorf("load connection: %w", err)
	}
	if connection == nil {
		return procurementcontract.ErrConnectionNotFound
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	detail, err := connection.GetOrder(ctx, procOrder.UpstreamOrderID)
	if err != nil {
		logger.Warnw("procurement_poll_status_error",
			"procurement_order_id", procOrder.ID,
			"upstream_order_id", procOrder.UpstreamOrderID,
			"error", err,
		)
		// 轮询失败，重新入队
		return s.requeuePoll(procOrder)
	}

	mappedStatus := mapProcurementUpstreamStatus(detail.Status)
	switch mappedStatus {
	case "delivered":
		return s.HandleUpstreamCallback(procOrder.ID, mappedStatus, detail.Fulfillment)
	case "canceled":
		return s.HandleUpstreamCallback(procOrder.ID, mappedStatus, nil)
	case "refunded", "partially_refunded":
		return s.HandleUpstreamCallback(procOrder.ID, mappedStatus, detail.Fulfillment)
	default:
		// 状态未变，继续轮询
		return s.requeuePoll(procOrder)
	}
}

// requeuePoll 重新入队轮询任务
func (s *Service) requeuePoll(procOrder *procurementdomain.Order) error {
	if s.queue == nil {
		return nil
	}

	idx := procOrder.RetryCount
	if idx >= len(pollIntervals) {
		// 短期轮询结束，后续由定时巡检和回调接管，不标记失败
		logger.Infow("procurement_poll_handoff_to_periodic_sync",
			"procurement_order_id", procOrder.ID,
			"retry_count", procOrder.RetryCount,
		)
		return nil
	}

	delay := pollIntervals[idx]

	// 递增轮询计数
	now := time.Now()
	stillAccepted, err := s.procRepo.UpdateStatus(procOrder.ID, procOrder.Status, procOrder.Status, map[string]interface{}{
		"retry_count": procOrder.RetryCount + 1,
		"updated_at":  now,
	})
	if err != nil || !stillAccepted {
		// 期间已被回调推进（或写入失败），不再继续轮询
		return err
	}

	return s.queue.EnqueuePoll(procOrder.ID, delay)
}

// SyncAcceptedOrders 定时巡检：检查所有 accepted 状态的采购单，向上游查询最新状态
// 由 worker 定时任务调用（每30分钟）。每个 worker 进程都注册了该定时任务，
// 用周期锁保证同一周期只有一个进程执行；锁不主动释放，到期即进入下一周期。
func (s *Service) SyncAcceptedOrders() {
	ctx := context.Background()
	if claimed, err := cache.SetNX(ctx, "procurement:sync_accepted_period", "1", 25*time.Minute); err == nil && !claimed {
		return
	}
	s.requeueStalledSubmissions()

	const pageSize = 200
	for page := 1; ; page++ {
		orders, total, err := s.procRepo.List(procurementcontract.ListFilter{
			Status:   constants.ProcurementStatusAccepted,
			Page:     page,
			PageSize: pageSize,
		})
		if err != nil {
			logger.Warnw("procurement_sync_accepted_list_failed", "error", err)
			return
		}
		if page == 1 && len(orders) > 0 {
			logger.Infow("procurement_sync_accepted_start", "count", total)
		}
		for i := range orders {
			s.syncAcceptedOrder(ctx, &orders[i])
		}
		// 本轮被推进出 accepted 的单会让后续分页前移、少看几条，由下一周期补上
		if len(orders) < pageSize || int64(page*pageSize) >= total {
			return
		}
	}
}

// stalledSubmissionGrace 提交任务应执行时间之后多久仍未推进，视为任务丢失（入队失败、Redis 重启等）。
const stalledSubmissionGrace = 15 * time.Minute

// requeueStalledSubmissions 为停滞的 pending / failed 采购单补投提交任务，否则对应订单会永远不发货。
func (s *Service) requeueStalledSubmissions() {
	if s.queue == nil {
		return
	}
	cutoff := time.Now().Add(-stalledSubmissionGrace)
	const pageSize = 200
	for _, status := range []string{constants.ProcurementStatusPending, constants.ProcurementStatusFailed} {
		for page := 1; ; page++ {
			orders, total, err := s.procRepo.List(procurementcontract.ListFilter{Status: status, Page: page, PageSize: pageSize})
			if err != nil {
				logger.Warnw("procurement_requeue_stalled_list_failed", "status", status, "error", err)
				break
			}
			for i := range orders {
				dueAt := orders[i].UpdatedAt
				if orders[i].NextRetryAt != nil {
					dueAt = *orders[i].NextRetryAt
				}
				if dueAt.After(cutoff) {
					continue
				}
				if err := s.queue.EnqueueSubmit(orders[i].ID, 0); err != nil {
					logger.Warnw("procurement_requeue_stalled_failed", "procurement_order_id", orders[i].ID, "error", err)
					continue
				}
				logger.Infow("procurement_requeue_stalled", "procurement_order_id", orders[i].ID, "status", status)
			}
			if len(orders) < pageSize || int64(page*pageSize) >= total {
				break
			}
		}
	}
}

func (s *Service) syncAcceptedOrder(ctx context.Context, procOrder *procurementdomain.Order) {
	if procOrder.UpstreamOrderID == 0 {
		return
	}
	connection, err := s.connections.Open(procOrder.ConnectionID)
	if err != nil || connection == nil {
		return
	}
	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	detail, err := connection.GetOrder(reqCtx, procOrder.UpstreamOrderID)
	cancel()
	if err != nil {
		logger.Warnw("procurement_sync_accepted_poll_error",
			"procurement_order_id", procOrder.ID,
			"upstream_order_id", procOrder.UpstreamOrderID,
			"error", err,
		)
		return
	}

	mappedStatus := mapProcurementUpstreamStatus(detail.Status)
	switch mappedStatus {
	case "delivered", "canceled", "refunded", "partially_refunded":
		fulfillment := detail.Fulfillment
		if mappedStatus == "canceled" {
			fulfillment = nil
		}
		if cbErr := s.HandleUpstreamCallback(procOrder.ID, mappedStatus, fulfillment); cbErr != nil {
			logger.Warnw("procurement_sync_accepted_apply_failed",
				"procurement_order_id", procOrder.ID,
				"upstream_status", mappedStatus,
				"error", cbErr,
			)
			return
		}
		logger.Infow("procurement_sync_accepted_applied",
			"procurement_order_id", procOrder.ID,
			"upstream_status", mappedStatus,
		)
	default:
		// 超过 24 小时仍在 accepted：每单每 24 小时最多告警一次
		acceptedDuration := time.Since(procOrder.UpdatedAt)
		if acceptedDuration <= 24*time.Hour {
			return
		}
		alertKey := fmt.Sprintf("procurement:accepted_timeout_alert:%d", procOrder.ID)
		if first, err := cache.SetNX(ctx, alertKey, "1", 24*time.Hour); err == nil && !first {
			return
		}
		logger.Warnw("procurement_accepted_timeout",
			"procurement_order_id", procOrder.ID,
			"upstream_order_id", procOrder.UpstreamOrderID,
			"accepted_duration", acceptedDuration.String(),
		)
		s.notifyProcurementFailure(procOrder, fmt.Sprintf(
			"procurement order stuck in accepted for %s, upstream status: %s",
			acceptedDuration.Round(time.Hour), detail.Status))
	}
}

// mapProcurementUpstreamStatus 统一映射上游状态别名，便于回调与轮询使用同一分支逻辑。
func mapProcurementUpstreamStatus(status string) string {
	normalized := strings.ToLower(strings.TrimSpace(status))
	switch normalized {
	case "delivered", "completed", "fulfilled":
		return "delivered"
	case "canceled", "cancelled":
		return "canceled"
	case "refunded", "partially_refunded":
		return normalized
	default:
		return normalized
	}
}
