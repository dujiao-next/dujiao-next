package application

import (
	"context"
	"fmt"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	procurementcontract "github.com/dujiao-next/internal/modules/procurement/contract"
)

// RetryManual 手动重试失败的采购单
func (s *Service) RetryManual(id uint) error {
	procOrder, err := s.procRepo.GetByID(id)
	if err != nil {
		return fmt.Errorf("load procurement order: %w", err)
	}
	if procOrder == nil {
		return procurementcontract.ErrNotFound
	}

	if procOrder.Status != "failed" && procOrder.Status != "rejected" {
		return procurementcontract.ErrStatusInvalid
	}

	now := time.Now()
	updates := map[string]interface{}{
		"retry_count":   0,
		"next_retry_at": nil,
		"error_message": "",
		"updated_at":    now,
	}
	reset, err := s.procRepo.UpdateStatus(procOrder.ID, procOrder.Status, constants.ProcurementStatusPending, updates)
	if err != nil {
		return fmt.Errorf("reset procurement status: %w", err)
	}
	if !reset {
		return procurementcontract.ErrStatusInvalid
	}

	logger.Infow("procurement_manual_retry",
		"procurement_order_id", procOrder.ID,
	)

	if s.queue != nil {
		return s.queue.EnqueueSubmit(procOrder.ID, 0)
	}
	return nil
}

// CancelManual 手动取消采购单
func (s *Service) CancelManual(id uint) error {
	procOrder, err := s.procRepo.GetByID(id)
	if err != nil {
		return fmt.Errorf("load procurement order: %w", err)
	}
	if procOrder == nil {
		return procurementcontract.ErrNotFound
	}

	switch procOrder.Status {
	case constants.ProcurementStatusPending, constants.ProcurementStatusFailed, constants.ProcurementStatusRejected:
		// 尚未在上游成单，可直接本地取消
	case constants.ProcurementStatusAccepted:
		// 上游已成单（通常已从上游钱包扣款）：必须上游取消成功才能标记取消，
		// 否则上游照常发货而本地已是终态，回调被丢弃、本地订单永久卡在 fulfilling。
		connection, err := s.connections.Open(procOrder.ConnectionID)
		if err != nil {
			return fmt.Errorf("open connection: %w", err)
		}
		if connection == nil {
			return procurementcontract.ErrConnectionNotFound
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := connection.CancelOrder(ctx, procOrder.UpstreamOrderID); err != nil {
			logger.Warnw("procurement_cancel_upstream_failed",
				"procurement_order_id", procOrder.ID,
				"upstream_order_id", procOrder.UpstreamOrderID,
				"error", err,
			)
			return fmt.Errorf("cancel upstream order: %w", err)
		}
	default:
		return procurementcontract.ErrStatusInvalid
	}

	now := time.Now()
	updates := map[string]interface{}{
		"error_message": "manually canceled",
		"updated_at":    now,
	}
	canceled, err := s.procRepo.UpdateStatus(procOrder.ID, procOrder.Status, constants.ProcurementStatusCanceled, updates)
	if err != nil {
		return fmt.Errorf("update procurement status: %w", err)
	}
	if !canceled {
		return procurementcontract.ErrStatusInvalid
	}

	logger.Infow("procurement_manual_cancel",
		"procurement_order_id", procOrder.ID,
	)
	return nil
}
