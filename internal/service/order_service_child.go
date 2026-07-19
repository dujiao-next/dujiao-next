package service

import (
	"errors"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"

	"gorm.io/gorm"
)

// cancelOrderWithChildren 取消父订单并级联子订单
func (s *OrderService) cancelOrderWithChildren(order *models.Order, rollbackCoupon bool) error {
	if order == nil {
		return ErrOrderNotFound
	}
	now := time.Now()
	err := s.orderRepo.Transaction(func(tx *gorm.DB) error {
		orderRepo := s.orderRepo.WithTx(tx)
		productRepo := s.productRepo.WithTx(tx)
		var productSKURepo repository.ProductSKURepository
		if s.productSKURepo != nil {
			productSKURepo = s.productSKURepo.WithTx(tx)
		}
		updates := map[string]interface{}{
			"canceled_at": now,
			"updated_at":  now,
		}
		if err := orderRepo.UpdateStatus(order.ID, constants.OrderStatusCanceled, updates); err != nil {
			return ErrOrderUpdateFailed
		}
		for _, child := range order.Children {
			if err := orderRepo.UpdateStatus(child.ID, constants.OrderStatusCanceled, updates); err != nil {
				return ErrOrderUpdateFailed
			}
		}
		if s.cardSecretRepo != nil {
			secretRepo := s.cardSecretRepo.WithTx(tx)
			if len(order.Children) > 0 {
				for _, child := range order.Children {
					if _, err := secretRepo.ReleaseByOrder(child.ID); err != nil {
						return err
					}
				}
			} else {
				if _, err := secretRepo.ReleaseByOrder(order.ID); err != nil {
					return err
				}
			}
		}
		if len(order.Children) > 0 {
			for _, child := range order.Children {
				if err := releaseManualStockByItems(productRepo, productSKURepo, child.Items); err != nil {
					return err
				}
			}
		} else {
			if err := releaseManualStockByItems(productRepo, productSKURepo, order.Items); err != nil {
				return err
			}
		}

		if rollbackCoupon {
			couponRepo := s.couponRepo.WithTx(tx)
			usageRepo := s.couponUsageRepo.WithTx(tx)
			usages, err := usageRepo.ListByOrderID(order.ID)
			if err != nil {
				return err
			}
			if len(usages) > 0 {
				if err := usageRepo.DeleteByOrderID(order.ID); err != nil {
					return err
				}
				counts := make(map[uint]int)
				for _, usage := range usages {
					counts[usage.CouponID]++
				}
				for couponID, count := range counts {
					if count <= 0 {
						continue
					}
					if err := couponRepo.DecrementUsedCount(couponID, count); err != nil {
						return err
					}
				}
			}
		}
		if s.walletService != nil {
			if _, err := s.walletService.ReleaseOrderBalance(tx, order, constants.WalletTxnTypeOrderRefund, "订单取消退回余额"); err != nil {
				return err
			}
		}
		if s.paymentRepo != nil {
			orderIDs := []uint{order.ID}
			for _, child := range order.Children {
				if child.ID > 0 {
					orderIDs = append(orderIDs, child.ID)
				}
			}
			if _, err := s.paymentRepo.WithTx(tx).ExpirePendingByOrderIDs(orderIDs, now); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	order.Status = constants.OrderStatusCanceled
	order.CanceledAt = &now
	order.UpdatedAt = now
	for i := range order.Children {
		order.Children[i].Status = constants.OrderStatusCanceled
		order.Children[i].CanceledAt = &now
		order.Children[i].UpdatedAt = now
	}
	return nil
}

// CancelOrder 用户取消订单
func (s *OrderService) CancelOrder(orderID uint, userID uint) (*models.Order, error) {
	order, err := s.orderRepo.GetByIDAndUser(orderID, userID)
	if err != nil {
		return nil, ErrOrderFetchFailed
	}
	if order == nil {
		return nil, ErrOrderNotFound
	}
	if order.Status != constants.OrderStatusPendingPayment {
		return nil, ErrOrderCancelNotAllowed
	}
	if err := s.cancelOrderWithChildren(order, false); err != nil {
		return nil, ErrOrderUpdateFailed
	}
	if s.affiliateSvc != nil {
		if err := s.affiliateSvc.HandleOrderCanceled(order.ID, "order_canceled_by_user"); err != nil {
			logger.Warnw("affiliate_handle_order_canceled_failed",
				"order_id", order.ID,
				"error", err,
			)
		}
	}
	if s.queueClient != nil {
		if _, err := enqueueOrderStatusEmailTaskIfEligible(s.orderRepo, s.queueClient, s.settingService, s.defaultEmailConfig, order.ID, constants.OrderStatusCanceled); err != nil {
			logger.Warnw("order_enqueue_status_email_failed",
				"order_id", order.ID,
				"target_order_id", order.ID,
				"status", constants.OrderStatusCanceled,
				"error", err,
			)
		}
	}
	fillOrderItemsFromChildren(order)
	return order, nil
}

// UpdateOrderStatus 管理端更新订单状态
func (s *OrderService) UpdateOrderStatus(orderID uint, targetStatus string) (*models.Order, error) {
	order, err := s.orderRepo.GetByID(orderID)
	if err != nil {
		return nil, ErrOrderFetchFailed
	}
	if order == nil {
		return nil, ErrOrderNotFound
	}

	target := strings.TrimSpace(targetStatus)
	if target == "" {
		return nil, ErrOrderStatusInvalid
	}
	if order.Status == target {
		return order, nil
	}
	isParent := order.ParentID == nil && len(order.Children) > 0
	if isParent {
		switch target {
		case constants.OrderStatusCanceled:
			if err := s.cancelOrderWithChildren(order, true); err != nil {
				return nil, ErrOrderUpdateFailed
			}
			if s.affiliateSvc != nil {
				if err := s.affiliateSvc.HandleOrderCanceled(order.ID, "order_canceled_by_admin"); err != nil {
					logger.Warnw("affiliate_handle_order_canceled_failed",
						"order_id", order.ID,
						"error", err,
					)
				}
			}
			if s.queueClient != nil {
				if _, err := enqueueOrderStatusEmailTaskIfEligible(s.orderRepo, s.queueClient, s.settingService, s.defaultEmailConfig, order.ID, constants.OrderStatusCanceled); err != nil {
					logger.Warnw("order_enqueue_status_email_failed",
						"order_id", order.ID,
						"target_order_id", order.ID,
						"status", constants.OrderStatusCanceled,
						"error", err,
					)
				}
			}
			return order, nil
		case constants.OrderStatusPaid:
			if order.Status != constants.OrderStatusPendingPayment {
				return nil, ErrOrderStatusInvalid
			}
			now := time.Now()
			err = s.orderRepo.Transaction(func(tx *gorm.DB) error {
				if err := s.updateOrderToPaidInTx(tx, order.ID, nil, now); err != nil {
					return err
				}
				for _, child := range order.Children {
					if err := s.updateOrderToPaidInTx(tx, child.ID, child.Items, now); err != nil {
						return err
					}
				}
				if s.resellerAccountingSvc != nil {
					if err := s.resellerAccountingSvc.PostOrderProfitTx(tx, order, nil); err != nil {
						return err
					}
				}
				return nil
			})
			if err != nil {
				return nil, ErrOrderUpdateFailed
			}
			order.Status = constants.OrderStatusPaid
			order.PaidAt = &now
			order.UpdatedAt = now
			for i := range order.Children {
				order.Children[i].Status = constants.OrderStatusPaid
				order.Children[i].PaidAt = &now
				order.Children[i].UpdatedAt = now
			}
			if s.queueClient != nil {
				if _, err := enqueueOrderStatusEmailTaskIfEligible(s.orderRepo, s.queueClient, s.settingService, s.defaultEmailConfig, order.ID, constants.OrderStatusPaid); err != nil {
					logger.Warnw("order_enqueue_status_email_failed",
						"order_id", order.ID,
						"target_order_id", order.ID,
						"status", constants.OrderStatusPaid,
						"error", err,
					)
				}
			}
			if s.affiliateSvc != nil {
				if err := s.affiliateSvc.HandleOrderPaid(order.ID); err != nil {
					logger.Warnw("affiliate_handle_order_paid_failed",
						"order_id", order.ID,
						"error", err,
					)
				}
			}
			if s.memberLevelService != nil && order.UserID > 0 {
				if err := s.memberLevelService.OnOrderPaid(order.UserID, order.TotalAmount.Decimal); err != nil {
					logger.Warnw("member_level_order_paid_failed",
						"order_id", order.ID,
						"user_id", order.UserID,
						"amount", order.TotalAmount.Decimal.String(),
						"error", err,
					)
				}
			}
			return order, nil
		case constants.OrderStatusCompleted:
			if !canCompleteParentOrder(order) {
				return nil, ErrOrderStatusInvalid
			}
			now := time.Now()
			err = s.orderRepo.Transaction(func(tx *gorm.DB) error {
				return s.completeParentOrderInTx(tx, order, now)
			})
			if err != nil {
				if errors.Is(err, ErrOrderStatusInvalid) {
					return nil, ErrOrderStatusInvalid
				}
				return nil, ErrOrderUpdateFailed
			}
			order.Status = constants.OrderStatusCompleted
			order.UpdatedAt = now
			for i := range order.Children {
				if order.Children[i].Status == constants.OrderStatusDelivered || order.Children[i].Status == constants.OrderStatusFulfilled {
					order.Children[i].Status = constants.OrderStatusCompleted
					order.Children[i].UpdatedAt = now
				}
			}
			if s.queueClient != nil {
				if _, err := enqueueOrderStatusEmailTaskIfEligible(s.orderRepo, s.queueClient, s.settingService, s.defaultEmailConfig, order.ID, constants.OrderStatusCompleted); err != nil {
					logger.Warnw("order_enqueue_status_email_failed",
						"order_id", order.ID,
						"target_order_id", order.ID,
						"status", constants.OrderStatusCompleted,
						"error", err,
					)
				}
			}
			return order, nil
		case constants.OrderStatusPartiallyRefunded, constants.OrderStatusRefunded:
			now := time.Now()
			err = s.orderRepo.Transaction(func(tx *gorm.DB) error {
				orderRepo := s.orderRepo.WithTx(tx)
				updates := map[string]interface{}{"updated_at": now}
				if err := orderRepo.UpdateStatus(order.ID, target, updates); err != nil {
					return ErrOrderUpdateFailed
				}
				for _, child := range order.Children {
					if child.Status == target {
						continue
					}
					if !isTransitionAllowed(child.Status, target) {
						return ErrOrderStatusInvalid
					}
					if err := orderRepo.UpdateStatus(child.ID, target, updates); err != nil {
						return ErrOrderUpdateFailed
					}
				}
				return nil
			})
			if err != nil {
				if errors.Is(err, ErrOrderStatusInvalid) {
					return nil, ErrOrderStatusInvalid
				}
				return nil, ErrOrderUpdateFailed
			}
			order.Status = target
			order.UpdatedAt = now
			for i := range order.Children {
				order.Children[i].Status = target
				order.Children[i].UpdatedAt = now
			}
			if s.queueClient != nil {
				if _, err := enqueueOrderStatusEmailTaskIfEligible(s.orderRepo, s.queueClient, s.settingService, s.defaultEmailConfig, order.ID, target); err != nil {
					logger.Warnw("order_enqueue_status_email_failed",
						"order_id", order.ID,
						"target_order_id", order.ID,
						"status", target,
						"error", err,
					)
				}
			}
			return order, nil
		default:
			return nil, ErrOrderStatusInvalid
		}
	}
	if !isTransitionAllowed(order.Status, target) {
		return nil, ErrOrderStatusInvalid
	}

	now := time.Now()
	updates := map[string]interface{}{
		"updated_at": now,
	}
	switch target {
	case constants.OrderStatusPaid:
		updates["paid_at"] = now
	case constants.OrderStatusCanceled:
		updates["canceled_at"] = now
	}

	if target == constants.OrderStatusCanceled {
		err = s.orderRepo.Transaction(func(tx *gorm.DB) error {
			return s.cancelSingleOrderInTx(tx, order, target, updates)
		})
	} else if target == constants.OrderStatusPaid {
		err = s.orderRepo.Transaction(func(tx *gorm.DB) error {
			return s.updateOrderToPaidInTx(tx, order.ID, order.Items, now)
		})
	} else {
		err = s.orderRepo.UpdateStatus(order.ID, target, updates)
	}
	if err != nil {
		return nil, ErrOrderUpdateFailed
	}
	if target == constants.OrderStatusPaid && s.affiliateSvc != nil {
		if err := s.affiliateSvc.HandleOrderPaid(order.ID); err != nil {
			logger.Warnw("affiliate_handle_order_paid_failed",
				"order_id", order.ID,
				"error", err,
			)
		}
	}
	if target == constants.OrderStatusPaid && s.memberLevelService != nil && order.UserID > 0 {
		if err := s.memberLevelService.OnOrderPaid(order.UserID, order.TotalAmount.Decimal); err != nil {
			logger.Warnw("member_level_order_paid_failed",
				"order_id", order.ID,
				"user_id", order.UserID,
				"amount", order.TotalAmount.Decimal.String(),
				"error", err,
			)
		}
	}
	if target == constants.OrderStatusCanceled && s.affiliateSvc != nil {
		if err := s.affiliateSvc.HandleOrderCanceled(order.ID, "order_canceled_by_admin"); err != nil {
			logger.Warnw("affiliate_handle_order_canceled_failed",
				"order_id", order.ID,
				"error", err,
			)
		}
	}
	order.Status = target
	order.UpdatedAt = now
	if v, ok := updates["paid_at"]; ok {
		if t, ok := v.(time.Time); ok {
			order.PaidAt = &t
		}
	}
	if v, ok := updates["canceled_at"]; ok {
		if t, ok := v.(time.Time); ok {
			order.CanceledAt = &t
		}
	}
	if order.ParentID != nil {
		parentStatus, syncErr := syncParentStatus(s.orderRepo, *order.ParentID, now)
		if syncErr != nil {
			logger.Warnw("order_sync_parent_status_failed",
				"order_id", order.ID,
				"parent_order_id", *order.ParentID,
				"target_status", target,
				"error", syncErr,
			)
		} else if s.queueClient != nil {
			status := parentStatus
			if status == "" {
				status = target
			}
			if _, err := enqueueOrderStatusEmailTaskIfEligible(s.orderRepo, s.queueClient, s.settingService, s.defaultEmailConfig, *order.ParentID, status); err != nil {
				logger.Warnw("order_enqueue_status_email_failed",
					"order_id", order.ID,
					"target_order_id", *order.ParentID,
					"status", status,
					"error", err,
				)
			}
		}
	} else if s.queueClient != nil {
		if _, err := enqueueOrderStatusEmailTaskIfEligible(s.orderRepo, s.queueClient, s.settingService, s.defaultEmailConfig, order.ID, target); err != nil {
			logger.Warnw("order_enqueue_status_email_failed",
				"order_id", order.ID,
				"target_order_id", order.ID,
				"status", target,
				"error", err,
			)
		}
	}
	fillOrderItemsFromChildren(order)
	return order, nil
}

func (s *OrderService) completeParentOrderInTx(tx *gorm.DB, order *models.Order, now time.Time) error {
	if order == nil {
		return ErrOrderNotFound
	}
	orderRepo := s.orderRepo.WithTx(tx)
	updates := map[string]interface{}{"updated_at": now}
	if err := orderRepo.UpdateStatus(order.ID, constants.OrderStatusCompleted, updates); err != nil {
		return ErrOrderUpdateFailed
	}
	for _, child := range order.Children {
		if child.Status == constants.OrderStatusCompleted {
			continue
		}
		if child.Status != constants.OrderStatusDelivered && child.Status != constants.OrderStatusFulfilled {
			return ErrOrderStatusInvalid
		}
		if err := orderRepo.UpdateStatus(child.ID, constants.OrderStatusCompleted, updates); err != nil {
			return ErrOrderUpdateFailed
		}
	}
	return nil
}

func (s *OrderService) updateOrderToPaidInTx(tx *gorm.DB, orderID uint, items []models.OrderItem, now time.Time) error {
	orderRepo := s.orderRepo.WithTx(tx)
	productRepo := s.productRepo.WithTx(tx)
	var productSKURepo repository.ProductSKURepository
	if s.productSKURepo != nil {
		productSKURepo = s.productSKURepo.WithTx(tx)
	}
	updates := map[string]interface{}{
		"paid_at":    now,
		"updated_at": now,
	}
	if err := orderRepo.UpdateStatus(orderID, constants.OrderStatusPaid, updates); err != nil {
		return ErrOrderUpdateFailed
	}
	if err := consumeManualStockByItems(productRepo, productSKURepo, items); err != nil {
		return err
	}
	return nil
}

func (s *OrderService) cancelSingleOrderInTx(tx *gorm.DB, order *models.Order, target string, updates map[string]interface{}) error {
	if order == nil {
		return ErrOrderNotFound
	}
	orderRepo := s.orderRepo.WithTx(tx)
	productRepo := s.productRepo.WithTx(tx)
	var productSKURepo repository.ProductSKURepository
	if s.productSKURepo != nil {
		productSKURepo = s.productSKURepo.WithTx(tx)
	}
	if err := orderRepo.UpdateStatus(order.ID, target, updates); err != nil {
		return ErrOrderUpdateFailed
	}
	if s.cardSecretRepo != nil {
		secretRepo := s.cardSecretRepo.WithTx(tx)
		if _, err := secretRepo.ReleaseByOrder(order.ID); err != nil {
			return err
		}
	}
	if err := releaseManualStockByItems(productRepo, productSKURepo, order.Items); err != nil {
		return err
	}
	if s.walletService != nil {
		if _, err := s.walletService.ReleaseOrderBalance(tx, order, constants.WalletTxnTypeOrderRefund, "订单取消退回余额"); err != nil {
			return err
		}
	}
	if target == constants.OrderStatusCanceled && s.paymentRepo != nil {
		expiredAt := time.Now()
		if v, ok := updates["canceled_at"]; ok {
			if t, ok := v.(time.Time); ok {
				expiredAt = t
			}
		}
		if _, err := s.paymentRepo.WithTx(tx).ExpirePendingByOrderIDs([]uint{order.ID}, expiredAt); err != nil {
			return err
		}
	}
	return nil
}

// CancelExpiredOrder 超时取消订单
func (s *OrderService) CancelExpiredOrder(orderID uint) (*models.Order, error) {
	if orderID == 0 {
		return nil, ErrOrderNotFound
	}
	order, err := s.orderRepo.GetByID(orderID)
	if err != nil {
		return nil, ErrOrderFetchFailed
	}
	if order == nil {
		return nil, ErrOrderNotFound
	}
	if order.Status != constants.OrderStatusPendingPayment {
		return order, nil
	}
	if order.ExpiresAt == nil {
		return order, nil
	}
	now := time.Now()
	if order.ExpiresAt.After(now) {
		return order, nil
	}
	if err := s.cancelOrderWithChildren(order, true); err != nil {
		return nil, err
	}
	if s.affiliateSvc != nil {
		if err := s.affiliateSvc.HandleOrderCanceled(order.ID, "order_expired_canceled"); err != nil {
			logger.Warnw("affiliate_handle_order_canceled_failed",
				"order_id", order.ID,
				"error", err,
			)
		}
	}
	if s.queueClient != nil {
		if _, err := enqueueOrderStatusEmailTaskIfEligible(s.orderRepo, s.queueClient, s.settingService, s.defaultEmailConfig, order.ID, constants.OrderStatusCanceled); err != nil {
			logger.Warnw("order_enqueue_status_email_failed",
				"order_id", order.ID,
				"target_order_id", order.ID,
				"status", constants.OrderStatusCanceled,
				"error", err,
			)
		}
	}
	fillOrderItemsFromChildren(order)
	return order, nil
}

func canCompleteParentOrder(order *models.Order) bool {
	if order == nil {
		return false
	}
	if order.Status != constants.OrderStatusDelivered && order.Status != constants.OrderStatusFulfilled {
		return false
	}
	for _, child := range order.Children {
		if child.Status != constants.OrderStatusDelivered && child.Status != constants.OrderStatusFulfilled && child.Status != constants.OrderStatusCompleted {
			return false
		}
	}
	return true
}

func isTransitionAllowed(current, target string) bool {
	if current == target {
		return true
	}
	nexts, ok := allowedTransitions[current]
	if !ok {
		return false
	}
	return nexts[target]
}
