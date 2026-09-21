package application

import (
	"errors"
	"strings"
	"time"

	ordercontract "github.com/dujiao-next/internal/modules/order/contract"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
)

// cancelOrderWithChildren 取消父订单并级联子订单
func (s *OrderService) cancelOrderWithChildren(order *orderdomain.Order, rollbackCoupon bool) error {
	if order == nil {
		return ErrOrderNotFound
	}
	now := time.Now()
	err := s.orderStore.WithinTransaction(func(tx ordercontract.Transaction) error {
		orderStore := tx.Orders()
		productRepo := tx.Products()
		productSKURepo := tx.ProductSKUs()
		// 事务内加锁复核状态，避免与并发的支付回调竞争
		locked, err := orderStore.GetByIDForUpdate(order.ID)
		if err != nil {
			return ErrOrderFetchFailed
		}
		if locked == nil {
			return ErrOrderNotFound
		}
		if locked.Status != order.Status {
			return ErrOrderCancelNotAllowed
		}
		updates := map[string]interface{}{
			"canceled_at": now,
			"updated_at":  now,
		}
		if err := orderStore.UpdateStatus(order.ID, constants.OrderStatusCanceled, updates); err != nil {
			return ErrOrderUpdateFailed
		}
		for _, child := range order.Children {
			if err := orderStore.UpdateStatus(child.ID, constants.OrderStatusCanceled, updates); err != nil {
				return ErrOrderUpdateFailed
			}
		}
		secretRepo := tx.CardSecrets()
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
			couponRepo := tx.Coupons()
			usageRepo := tx.CouponUsages()
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
			if _, err := ReleaseWalletBalance(s.walletService, tx, order, constants.WalletTxnTypeOrderRefund, "订单取消退回余额"); err != nil {
				return err
			}
		}
		orderIDs := []uint{order.ID}
		for _, child := range order.Children {
			if child.ID > 0 {
				orderIDs = append(orderIDs, child.ID)
			}
		}
		if _, err := tx.ExpirePendingPaymentsByOrderIDs(orderIDs, now); err != nil {
			return err
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
func (s *OrderService) CancelOrder(orderID uint, userID uint) (*orderdomain.Order, error) {
	order, err := s.orderStore.GetByIDAndUser(orderID, userID)
	if err != nil {
		return nil, ErrOrderFetchFailed
	}
	if order == nil {
		return nil, ErrOrderNotFound
	}
	if order.Status != constants.OrderStatusPendingPayment {
		return nil, ErrOrderCancelNotAllowed
	}
	// 未支付订单取消时同步回滚优惠券用量
	if err := s.cancelOrderWithChildren(order, true); err != nil {
		if errors.Is(err, ErrOrderCancelNotAllowed) {
			return nil, err
		}
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
	FillOrderItemsFromChildren(order)
	return order, nil
}

// CancelGuestOrder 取消已经通过游客邮箱和查询密码校验的待支付父订单。
func (s *OrderService) CancelGuestOrder(order *orderdomain.Order) (*orderdomain.Order, error) {
	if order == nil || order.ID == 0 || order.ParentID != nil {
		return nil, ErrOrderNotFound
	}
	if order.Status != constants.OrderStatusPendingPayment {
		return nil, ErrOrderCancelNotAllowed
	}
	if err := s.cancelOrderWithChildren(order, true); err != nil {
		if errors.Is(err, ErrOrderCancelNotAllowed) {
			return nil, err
		}
		return nil, ErrOrderUpdateFailed
	}
	if s.affiliateSvc != nil {
		if err := s.affiliateSvc.HandleOrderCanceled(order.ID, "order_canceled_by_guest"); err != nil {
			logger.Warnw("affiliate_handle_order_canceled_failed", "order_id", order.ID, "error", err)
		}
	}
	FillOrderItemsFromChildren(order)
	return order, nil
}

// UpdateOrderStatus 管理端更新订单状态
func (s *OrderService) UpdateOrderStatus(orderID uint, targetStatus string) (*orderdomain.Order, error) {
	order, err := s.orderStore.GetByID(orderID)
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
	// “已支付”只能由钱包原子扣款或经过验签、金额校验的支付回调驱动。
	// 管理端通用状态接口不得绕过支付事实直接触发库存、累计消费和交付副作用。
	if target == constants.OrderStatusPaid {
		return nil, ErrOrderStatusInvalid
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
			return order, nil
		case constants.OrderStatusCompleted:
			if !canCompleteParentOrder(order) {
				return nil, ErrOrderStatusInvalid
			}
			now := time.Now()
			err = s.orderStore.WithinTransaction(func(tx ordercontract.Transaction) error {
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
				if order.Children[i].Status == constants.OrderStatusDelivered {
					order.Children[i].Status = constants.OrderStatusCompleted
					order.Children[i].UpdatedAt = now
				}
			}
			if s.queueClient != nil {
				if _, err := EnqueueStatusEmailTaskIfEligible(s.orderStore, s.queueClient, s.settingService, s.defaultEmailConfig, order.ID, constants.OrderStatusCompleted); err != nil {
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
			err = s.orderStore.WithinTransaction(func(tx ordercontract.Transaction) error {
				orderStore := tx.Orders()
				updates := map[string]interface{}{"updated_at": now}
				if err := orderStore.UpdateStatus(order.ID, target, updates); err != nil {
					return ErrOrderUpdateFailed
				}
				for _, child := range order.Children {
					if child.Status == target {
						continue
					}
					if !IsTransitionAllowed(child.Status, target) {
						return ErrOrderStatusInvalid
					}
					if err := orderStore.UpdateStatus(child.ID, target, updates); err != nil {
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
				if _, err := EnqueueStatusEmailTaskIfEligible(s.orderStore, s.queueClient, s.settingService, s.defaultEmailConfig, order.ID, target); err != nil {
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
	if !IsTransitionAllowed(order.Status, target) {
		return nil, ErrOrderStatusInvalid
	}

	now := time.Now()
	updates := map[string]interface{}{
		"updated_at": now,
	}
	if target == constants.OrderStatusCanceled {
		updates["canceled_at"] = now
	}

	if target == constants.OrderStatusCanceled {
		err = s.orderStore.WithinTransaction(func(tx ordercontract.Transaction) error {
			return s.cancelSingleOrderInTx(tx, order, target, updates)
		})
	} else {
		err = s.orderStore.UpdateStatus(order.ID, target, updates)
	}
	if err != nil {
		return nil, ErrOrderUpdateFailed
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
	if v, ok := updates["canceled_at"]; ok {
		if t, ok := v.(time.Time); ok {
			order.CanceledAt = &t
		}
	}
	if order.ParentID != nil {
		parentStatus, syncErr := SyncParentStatus(s.orderStore, *order.ParentID, now)
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
			if status != constants.OrderStatusCanceled {
				if _, err := EnqueueStatusEmailTaskIfEligible(s.orderStore, s.queueClient, s.settingService, s.defaultEmailConfig, *order.ParentID, status); err != nil {
					logger.Warnw("order_enqueue_status_email_failed",
						"order_id", order.ID,
						"target_order_id", *order.ParentID,
						"status", status,
						"error", err,
					)
				}
			}
		}
	} else if s.queueClient != nil && target != constants.OrderStatusCanceled {
		if _, err := EnqueueStatusEmailTaskIfEligible(s.orderStore, s.queueClient, s.settingService, s.defaultEmailConfig, order.ID, target); err != nil {
			logger.Warnw("order_enqueue_status_email_failed",
				"order_id", order.ID,
				"target_order_id", order.ID,
				"status", target,
				"error", err,
			)
		}
	}
	FillOrderItemsFromChildren(order)
	return order, nil
}

func (s *OrderService) completeParentOrderInTx(tx ordercontract.Transaction, order *orderdomain.Order, now time.Time) error {
	if order == nil {
		return ErrOrderNotFound
	}
	orderStore := tx.Orders()
	updates := map[string]interface{}{"updated_at": now}
	if err := orderStore.UpdateStatus(order.ID, constants.OrderStatusCompleted, updates); err != nil {
		return ErrOrderUpdateFailed
	}
	for _, child := range order.Children {
		if child.Status == constants.OrderStatusCompleted {
			continue
		}
		if child.Status != constants.OrderStatusDelivered {
			return ErrOrderStatusInvalid
		}
		if err := orderStore.UpdateStatus(child.ID, constants.OrderStatusCompleted, updates); err != nil {
			return ErrOrderUpdateFailed
		}
	}
	return nil
}

func (s *OrderService) cancelSingleOrderInTx(tx ordercontract.Transaction, order *orderdomain.Order, target string, updates map[string]interface{}) error {
	if order == nil {
		return ErrOrderNotFound
	}
	orderStore := tx.Orders()
	productRepo := tx.Products()
	productSKURepo := tx.ProductSKUs()
	if err := orderStore.UpdateStatus(order.ID, target, updates); err != nil {
		return ErrOrderUpdateFailed
	}
	secretRepo := tx.CardSecrets()
	if _, err := secretRepo.ReleaseByOrder(order.ID); err != nil {
		return err
	}
	if err := releaseManualStockByItems(productRepo, productSKURepo, order.Items); err != nil {
		return err
	}
	if s.walletService != nil {
		if _, err := ReleaseWalletBalance(s.walletService, tx, order, constants.WalletTxnTypeOrderRefund, "订单取消退回余额"); err != nil {
			return err
		}
	}
	if target == constants.OrderStatusCanceled {
		expiredAt := time.Now()
		if v, ok := updates["canceled_at"]; ok {
			if t, ok := v.(time.Time); ok {
				expiredAt = t
			}
		}
		if _, err := tx.ExpirePendingPaymentsByOrderIDs([]uint{order.ID}, expiredAt); err != nil {
			return err
		}
	}
	return nil
}

// CancelExpiredOrder 超时取消订单
func (s *OrderService) CancelExpiredOrder(orderID uint) (*orderdomain.Order, error) {
	if orderID == 0 {
		return nil, ErrOrderNotFound
	}
	order, err := s.orderStore.GetByID(orderID)
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
	FillOrderItemsFromChildren(order)
	return order, nil
}

func canCompleteParentOrder(order *orderdomain.Order) bool {
	if order == nil {
		return false
	}
	if order.Status != constants.OrderStatusDelivered {
		return false
	}
	for _, child := range order.Children {
		if child.Status != constants.OrderStatusDelivered && child.Status != constants.OrderStatusCompleted {
			return false
		}
	}
	return true
}

func IsTransitionAllowed(current, target string) bool {
	if current == target {
		return true
	}
	nexts, ok := allowedTransitions[current]
	if !ok {
		return false
	}
	return nexts[target]
}
