package application

import (
	"fmt"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	procurementcontract "github.com/dujiao-next/internal/modules/procurement/contract"
)

// HandleUpstreamCallback 处理上游回调通知
func (s *Service) HandleUpstreamCallback(procurementOrderID uint, upstreamStatus string, fulfillment *procurementcontract.Fulfillment) error {
	procOrder, err := s.procRepo.GetByID(procurementOrderID)
	if err != nil {
		return fmt.Errorf("load procurement order: %w", err)
	}
	if procOrder == nil {
		return procurementcontract.ErrNotFound
	}

	now := time.Now()
	upstreamStatus = strings.ToLower(strings.TrimSpace(upstreamStatus))

	// 状态机守卫：已进入终态的采购单不再接受交付 / 取消回调
	if !isUpstreamTransitionAllowed(procOrder.Status, upstreamStatus) {
		logger.Warnw("procurement_callback_transition_rejected",
			"procurement_order_id", procOrder.ID,
			"current_status", procOrder.Status,
			"upstream_status", upstreamStatus,
		)
		return nil
	}

	switch upstreamStatus {
	case "delivered", "completed", "fulfilled":
		// 先创建交付记录（幂等），再推进采购单状态；写入失败时采购单保持非终态以便上游重试
		if fulfillment != nil && s.orderLifecycle != nil {
			if err := s.createUpstreamFulfillment(procOrder.LocalOrderID, fulfillment, now); err != nil {
				logger.Warnw("procurement_create_fulfillment_failed",
					"procurement_order_id", procOrder.ID,
					"local_order_id", procOrder.LocalOrderID,
					"error", err,
				)
				return err
			}
		}

		// 更新采购单状态
		updates := map[string]interface{}{
			"updated_at": now,
		}
		if fulfillment != nil {
			updates["upstream_payload"] = fulfillment.Payload
		}
		fulfilled, err := s.procRepo.UpdateStatus(procOrder.ID, procOrder.Status, constants.ProcurementStatusFulfilled, updates)
		if err != nil {
			return fmt.Errorf("update procurement status: %w", err)
		}
		if !fulfilled {
			// 回调与轮询/巡检并发：已由另一路径完成交付，避免重复通知
			return nil
		}

		// 更新本地订单状态
		_ = s.orderRepo.UpdateStatus(procOrder.LocalOrderID, constants.OrderStatusDelivered, map[string]interface{}{
			"updated_at": now,
		})

		// 如果有父订单，同步父订单状态
		localOrder, _ := s.orderRepo.GetByID(procOrder.LocalOrderID)
		if localOrder != nil && localOrder.ParentID != nil && s.orderLifecycle != nil {
			if status, syncErr := s.orderLifecycle.SyncParentStatus(*localOrder.ParentID, now); syncErr != nil {
				logger.Warnw("procurement_sync_parent_status_failed",
					"procurement_order_id", procOrder.ID,
					"parent_order_id", *localOrder.ParentID,
					"error", syncErr,
				)
			} else {
				if status == "" {
					status = constants.OrderStatusDelivered
				}
				if status != constants.OrderStatusCanceled {
					_, _ = s.orderLifecycle.EnqueueStatusEmail(*localOrder.ParentID, status)
				}
			}
		} else if localOrder != nil && s.orderLifecycle != nil {
			_, _ = s.orderLifecycle.EnqueueStatusEmail(localOrder.ID, constants.OrderStatusDelivered)
		}

		// 触发下游回调（多级连跳：本站作为中间节点，通知下游交付完成）
		if s.downstreamCallback != nil {
			s.downstreamCallback.EnqueueCallback(procOrder.LocalOrderID)
			// 如果有父订单，也通知父订单的下游
			if localOrder != nil && localOrder.ParentID != nil {
				s.downstreamCallback.EnqueueCallback(*localOrder.ParentID)
			}
		}

		// 通知 Bot 订单已交付
		if s.botNotifier != nil && localOrder != nil {
			notifyOrderID := localOrder.ID
			if localOrder.ParentID != nil {
				notifyOrderID = *localOrder.ParentID
			}
			go s.botNotifier.NotifyBotOrderFulfilled(localOrder.UserID, notifyOrderID)
		}

		logger.Infow("procurement_order_fulfilled",
			"procurement_order_id", procOrder.ID,
			"local_order_id", procOrder.LocalOrderID,
		)

	case "canceled":
		updates := map[string]interface{}{
			"updated_at": now,
		}
		canceled, err := s.procRepo.UpdateStatus(procOrder.ID, procOrder.Status, constants.ProcurementStatusCanceled, updates)
		if err != nil {
			return fmt.Errorf("update procurement status: %w", err)
		}
		if !canceled {
			return nil
		}

		// 回退本地订单状态并通知管理员
		s.rollbackLocalOrderOnProcurementFailure(procOrder, "upstream canceled order")

		logger.Infow("procurement_order_canceled_by_upstream",
			"procurement_order_id", procOrder.ID,
			"local_order_id", procOrder.LocalOrderID,
		)
	case "refunded", "partially_refunded":
		updates := map[string]interface{}{
			"updated_at": now,
		}
		if fulfillment != nil {
			updates["upstream_payload"] = fulfillment.Payload
		}
		targetStatus := constants.ProcurementStatusPartiallyRefunded
		if upstreamStatus == "refunded" {
			targetStatus = constants.ProcurementStatusRefunded
		}
		if _, err := s.procRepo.UpdateStatus(procOrder.ID, procOrder.Status, targetStatus, updates); err != nil {
			return fmt.Errorf("update procurement status: %w", err)
		}
		logger.Infow("procurement_order_refunded",
			"procurement_order_id", procOrder.ID,
			"local_order_id", procOrder.LocalOrderID,
			"upstream_status", upstreamStatus,
			"local_status", targetStatus,
		)

	default:
		logger.Warnw("procurement_unknown_upstream_status",
			"procurement_order_id", procOrder.ID,
			"upstream_status", upstreamStatus,
		)
	}

	return nil
}

// createUpstreamFulfillment 在本地订单上创建上游交付记录
func (s *Service) createUpstreamFulfillment(orderID uint, uf *procurementcontract.Fulfillment, now time.Time) error {
	return s.orderLifecycle.CreateUpstreamFulfillment(orderID, uf, now)
}

// isUpstreamTransitionAllowed 上游回调允许的采购单状态转换：
// 已交付、已全额退款、已取消的采购单不再接受 delivered / canceled；
// 退款类回调不受限（部分退款可能先于交付到达）。
func isUpstreamTransitionAllowed(current, upstreamStatus string) bool {
	switch upstreamStatus {
	case "delivered", "completed", "fulfilled", "canceled":
		switch current {
		case constants.ProcurementStatusFulfilled, constants.ProcurementStatusCompleted,
			constants.ProcurementStatusRefunded, constants.ProcurementStatusCanceled:
			return false
		}
	}
	return true
}
