package service

import (
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"
)

// syncParentStatus 汇总父订单状态并写入
func syncParentStatus(orderRepo repository.OrderRepository, parentID uint, now time.Time) (string, error) {
	if parentID == 0 {
		return "", nil
	}
	parent, err := orderRepo.GetByID(parentID)
	if err != nil {
		return "", err
	}
	if parent == nil || parent.ParentID != nil {
		return "", nil
	}
	if parent.Status == constants.OrderStatusCanceled {
		return parent.Status, nil
	}
	newStatus := calcParentStatus(parent.Children, parent.Status)
	if newStatus == "" || newStatus == parent.Status {
		return parent.Status, nil
	}
	updates := map[string]interface{}{
		"updated_at": now,
	}
	if err := orderRepo.UpdateStatus(parent.ID, newStatus, updates); err != nil {
		return "", err
	}
	return newStatus, nil
}

func calcParentStatus(children []models.Order, currentStatus string) string {
	if len(children) == 0 {
		return currentStatus
	}
	var deliveredCount int
	var fulfilledCount int
	var completedCount int
	var canceledCount int
	var refundedCount int
	var partiallyRefundedCount int
	var paidCount int
	var pendingCount int
	var fulfillingCount int
	for _, child := range children {
		switch strings.ToLower(strings.TrimSpace(child.Status)) {
		case constants.OrderStatusCanceled:
			canceledCount++
		case constants.OrderStatusRefunded:
			refundedCount++
		case constants.OrderStatusPartiallyRefunded:
			partiallyRefundedCount++
		case constants.OrderStatusCompleted:
			completedCount++
		case constants.OrderStatusFulfilled:
			fulfilledCount++
		case constants.OrderStatusDelivered:
			deliveredCount++
		case constants.OrderStatusPaid:
			paidCount++
		case constants.OrderStatusFulfilling:
			fulfillingCount++
		case constants.OrderStatusPendingPayment:
			pendingCount++
		}
	}
	if canceledCount == len(children) {
		return constants.OrderStatusCanceled
	}
	if refundedCount == len(children) {
		return constants.OrderStatusRefunded
	}
	if refundedCount > 0 || partiallyRefundedCount > 0 {
		return constants.OrderStatusPartiallyRefunded
	}
	if completedCount == len(children) {
		return constants.OrderStatusCompleted
	}
	if fulfilledCount+completedCount == len(children) {
		return constants.OrderStatusFulfilled
	}
	if deliveredCount+fulfilledCount+completedCount == len(children) {
		return constants.OrderStatusDelivered
	}
	if deliveredCount+fulfilledCount+completedCount > 0 {
		return constants.OrderStatusPartiallyDelivered
	}
	if fulfillingCount > 0 {
		return constants.OrderStatusFulfilling
	}
	if paidCount > 0 {
		return constants.OrderStatusPaid
	}
	if pendingCount > 0 {
		return constants.OrderStatusPendingPayment
	}
	return currentStatus
}
