package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/queue"
	"github.com/dujiao-next/internal/repository"

	"github.com/hibiken/asynq"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (s *PaymentService) HandleCallback(input PaymentCallbackInput) (*models.Payment, error) {
	if input.PaymentID == 0 {
		return nil, ErrPaymentInvalid
	}
	status := normalizePaymentStatus(input.Status)
	if !isPaymentStatusValid(status) {
		return nil, ErrPaymentStatusInvalid
	}

	log := paymentLogger(
		"payment_id", input.PaymentID,
		"target_status", status,
		"callback_channel_id", input.ChannelID,
		"callback_order_no", strings.TrimSpace(input.OrderNo),
		"callback_provider_ref", strings.TrimSpace(input.ProviderRef),
		"callback_currency", strings.ToUpper(strings.TrimSpace(input.Currency)),
		"callback_amount", input.Amount.String(),
	)
	log.Infow("payment_callback_received")

	payment, err := s.paymentRepo.GetByID(input.PaymentID)
	if err != nil {
		log.Errorw("payment_callback_payment_fetch_failed", "error", err)
		return nil, ErrPaymentUpdateFailed
	}
	if payment == nil {
		log.Warnw("payment_callback_payment_not_found")
		return nil, ErrPaymentNotFound
	}
	if payment.OrderID == 0 {
		log.Infow("payment_callback_wallet_recharge_flow")
		return s.handleWalletRechargeCallback(payment, status, input)
	}

	order, err := s.orderRepo.GetByID(payment.OrderID)
	if err != nil {
		log.Errorw("payment_callback_order_fetch_failed", "order_id", payment.OrderID, "error", err)
		return nil, ErrOrderFetchFailed
	}
	if order == nil {
		log.Warnw("payment_callback_order_not_found", "order_id", payment.OrderID)
		return nil, ErrOrderNotFound
	}

	if input.ChannelID != 0 && input.ChannelID != payment.ChannelID {
		log.Warnw("payment_callback_channel_mismatch",
			"stored_channel_id", payment.ChannelID,
			"callback_channel_id", input.ChannelID,
		)
		return nil, ErrPaymentInvalid
	}
	if input.OrderNo != "" && input.OrderNo != order.OrderNo && input.OrderNo != fmt.Sprintf("%s-%d", order.OrderNo, payment.ID) {
		log.Warnw("payment_callback_order_no_mismatch",
			"stored_order_no", order.OrderNo,
			"callback_order_no", input.OrderNo,
		)
		return nil, ErrPaymentInvalid
	}
	if input.Currency != "" && strings.ToUpper(strings.TrimSpace(input.Currency)) != strings.ToUpper(strings.TrimSpace(payment.Currency)) {
		log.Warnw("payment_callback_currency_mismatch",
			"stored_currency", payment.Currency,
			"callback_currency", input.Currency,
		)
		return nil, ErrPaymentCurrencyMismatch
	}
	if !input.Amount.Decimal.IsZero() && input.Amount.Decimal.Cmp(payment.Amount.Decimal) != 0 {
		log.Warnw("payment_callback_amount_mismatch",
			"stored_amount", payment.Amount.String(),
			"callback_amount", input.Amount.String(),
		)
		return nil, ErrPaymentAmountMismatch
	}

	// 幂等处理：已成功的不再回退状态
	if payment.Status == constants.PaymentStatusSuccess {
		log.Infow("payment_callback_idempotent_success",
			"current_status", payment.Status,
		)
		return s.updateCallbackMeta(payment, constants.PaymentStatusSuccess, input)
	}
	if payment.Status == status {
		log.Infow("payment_callback_idempotent_same_status",
			"current_status", payment.Status,
		)
		return s.updateCallbackMeta(payment, status, input)
	}

	previousStatus := payment.Status
	now := time.Now()
	updated, orderPaid, err := s.applyPaymentUpdate(payment, order, status, input, now)
	if err != nil {
		log.Errorw("payment_callback_apply_failed",
			"order_id", order.ID,
			"order_no", order.OrderNo,
			"current_status", payment.Status,
			"error", err,
		)
		return nil, err
	}
	if orderPaid {
		s.enqueueOrderPaidAsync(order, updated, log)
	}
	log.Infow("payment_callback_processed",
		"order_id", order.ID,
		"order_no", order.OrderNo,
		"previous_status", previousStatus,
		"new_status", updated.Status,
		"order_paid", orderPaid,
	)
	return updated, nil
}

func (s *PaymentService) handleWalletRechargeCallback(payment *models.Payment, status string, input PaymentCallbackInput) (*models.Payment, error) {
	log := paymentLogger(
		"payment_id", payment.ID,
		"recharge_no", strings.TrimSpace(input.OrderNo),
		"target_status", status,
		"callback_channel_id", input.ChannelID,
		"callback_provider_ref", strings.TrimSpace(input.ProviderRef),
		"callback_currency", strings.ToUpper(strings.TrimSpace(input.Currency)),
		"callback_amount", input.Amount.String(),
	)
	if s.walletRepo == nil {
		log.Errorw("wallet_recharge_callback_wallet_repo_nil")
		return nil, ErrPaymentUpdateFailed
	}
	recharge, err := s.walletRepo.GetRechargeOrderByPaymentID(payment.ID)
	if err != nil {
		log.Errorw("wallet_recharge_callback_recharge_fetch_failed", "error", err)
		return nil, ErrPaymentUpdateFailed
	}
	if recharge == nil {
		log.Warnw("wallet_recharge_callback_recharge_not_found")
		return nil, ErrWalletRechargeNotFound
	}

	if input.ChannelID != 0 && input.ChannelID != payment.ChannelID {
		log.Warnw("wallet_recharge_callback_channel_mismatch",
			"stored_channel_id", payment.ChannelID,
			"callback_channel_id", input.ChannelID,
		)
		return nil, ErrPaymentInvalid
	}
	uniqueOrderNo := fmt.Sprintf("%s-%d", recharge.RechargeNo, payment.ID)
	if input.OrderNo != "" && input.OrderNo != recharge.RechargeNo && input.OrderNo != uniqueOrderNo {
		log.Warnw("wallet_recharge_callback_order_no_mismatch",
			"stored_recharge_no", recharge.RechargeNo,
			"callback_order_no", input.OrderNo,
		)
		return nil, ErrPaymentInvalid
	}
	if input.Currency != "" && strings.ToUpper(strings.TrimSpace(input.Currency)) != strings.ToUpper(strings.TrimSpace(payment.Currency)) {
		log.Warnw("wallet_recharge_callback_currency_mismatch",
			"stored_currency", payment.Currency,
			"callback_currency", input.Currency,
		)
		return nil, ErrPaymentCurrencyMismatch
	}
	if !input.Amount.Decimal.IsZero() && input.Amount.Decimal.Cmp(payment.Amount.Decimal) != 0 {
		log.Warnw("wallet_recharge_callback_amount_mismatch",
			"stored_amount", payment.Amount.String(),
			"callback_amount", input.Amount.String(),
		)
		return nil, ErrPaymentAmountMismatch
	}

	// 幂等处理：已成功状态仅更新回调元信息。
	if payment.Status == constants.PaymentStatusSuccess {
		log.Infow("wallet_recharge_callback_idempotent_success",
			"current_status", payment.Status,
		)
		return s.updateCallbackMeta(payment, constants.PaymentStatusSuccess, input)
	}
	if payment.Status == status {
		log.Infow("wallet_recharge_callback_idempotent_same_status",
			"current_status", payment.Status,
		)
		return s.updateCallbackMeta(payment, status, input)
	}
	if !canApplyWalletRechargeCallback(payment.Status, recharge.Status, status) {
		log.Infow("wallet_recharge_callback_ignored_terminal_transition",
			"current_payment_status", payment.Status,
			"current_recharge_status", recharge.Status,
			"target_status", status,
		)
		return s.updateCallbackMeta(payment, payment.Status, input)
	}

	now := time.Now()
	updated, err := s.applyWalletRechargePaymentUpdate(payment, status, input, now)
	if err != nil {
		log.Errorw("wallet_recharge_callback_apply_failed", "error", err)
		return nil, err
	}
	log.Infow("wallet_recharge_callback_processed",
		"new_status", updated.Status,
	)
	if updated.Status == constants.PaymentStatusSuccess {
		s.enqueueWalletRechargeSuccessAsync(recharge, updated, log)
	}
	s.enqueueExceptionAlertCheckAsync("wallet_recharge_callback_processed", log)
	return updated, nil
}

func (s *PaymentService) applyWalletRechargePaymentUpdate(payment *models.Payment, status string, input PaymentCallbackInput, now time.Time) (*models.Payment, error) {
	paymentVal := payment

	switch status {
	case constants.PaymentStatusSuccess:
		paidAt := now
		if input.PaidAt != nil {
			paidAt = *input.PaidAt
		}
		payment.PaidAt = &paidAt
	case constants.PaymentStatusExpired:
		payment.ExpiredAt = &now
	}

	payment.Status = status
	payment.CallbackAt = &now
	payment.UpdatedAt = now
	if input.ProviderRef != "" {
		payment.ProviderRef = input.ProviderRef
	}
	if input.Payload != nil {
		payment.ProviderPayload = input.Payload
	}

	err := s.paymentRepo.Transaction(func(tx *gorm.DB) error {
		paymentRepo := s.paymentRepo.WithTx(tx)
		rechargeRepo := s.walletRepo.WithTx(tx)

		if err := paymentRepo.Update(payment); err != nil {
			return ErrPaymentUpdateFailed
		}
		recharge, err := rechargeRepo.GetRechargeOrderByPaymentIDForUpdate(payment.ID)
		if err != nil {
			return ErrPaymentUpdateFailed
		}
		if recharge == nil {
			return ErrWalletRechargeNotFound
		}
		if recharge.Status == constants.WalletRechargeStatusSuccess {
			return nil
		}

		switch status {
		case constants.PaymentStatusSuccess:
			if s.walletSvc == nil {
				return ErrWalletAccountNotFound
			}
			if _, err := s.walletSvc.ApplyRechargePayment(tx, recharge); err != nil {
				return err
			}
			recharge.Status = constants.WalletRechargeStatusSuccess
			paidAt := now
			if payment.PaidAt != nil {
				paidAt = *payment.PaidAt
			}
			recharge.PaidAt = &paidAt
		case constants.PaymentStatusFailed:
			recharge.Status = constants.WalletRechargeStatusFailed
		case constants.PaymentStatusExpired:
			recharge.Status = constants.WalletRechargeStatusExpired
		default:
			recharge.Status = constants.WalletRechargeStatusPending
		}
		recharge.UpdatedAt = now
		if err := rechargeRepo.UpdateRechargeOrder(recharge); err != nil {
			return ErrPaymentUpdateFailed
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return paymentVal, nil
}

func canApplyWalletRechargeCallback(paymentStatus string, rechargeStatus string, targetStatus string) bool {
	// 成功回调允许覆盖终态（支付网关存在延迟成功通知场景）。
	if targetStatus == constants.PaymentStatusSuccess {
		return true
	}
	// 非成功回调不允许改变任何终态，避免 expired/failed/success 被回调串扰重开。
	if paymentStatus == constants.PaymentStatusSuccess || rechargeStatus == constants.WalletRechargeStatusSuccess {
		return false
	}
	if paymentStatus == constants.PaymentStatusFailed || rechargeStatus == constants.WalletRechargeStatusFailed {
		return false
	}
	if paymentStatus == constants.PaymentStatusExpired || rechargeStatus == constants.WalletRechargeStatusExpired {
		return false
	}
	return true
}

func (s *PaymentService) updateCallbackMeta(payment *models.Payment, status string, input PaymentCallbackInput) (*models.Payment, error) {
	updated := false
	if input.ProviderRef != "" && payment.ProviderRef == "" {
		payment.ProviderRef = input.ProviderRef
		updated = true
	}
	if input.Payload != nil {
		payment.ProviderPayload = input.Payload
		updated = true
	}
	if status != "" && payment.Status != status {
		payment.Status = status
		updated = true
	}
	if payment.Status == constants.PaymentStatusSuccess && payment.PaidAt == nil && input.PaidAt != nil {
		payment.PaidAt = input.PaidAt
		updated = true
	}
	if updated {
		now := time.Now()
		payment.CallbackAt = &now
		payment.UpdatedAt = now
		if err := s.paymentRepo.Update(payment); err != nil {
			return nil, ErrPaymentUpdateFailed
		}
	}
	return payment, nil
}

func (s *PaymentService) applyPaymentUpdate(payment *models.Payment, order *models.Order, status string, input PaymentCallbackInput, now time.Time) (*models.Payment, bool, error) {
	returnVal := payment
	orderPaid := false

	switch status {
	case constants.PaymentStatusSuccess:
		paidAt := now
		if input.PaidAt != nil {
			paidAt = *input.PaidAt
		}
		payment.PaidAt = &paidAt
	case constants.PaymentStatusExpired:
		payment.ExpiredAt = &now
	}

	payment.Status = status
	payment.CallbackAt = &now
	payment.UpdatedAt = now
	if input.ProviderRef != "" {
		payment.ProviderRef = input.ProviderRef
	}
	if input.Payload != nil {
		payment.ProviderPayload = input.Payload
	}

	err := s.paymentRepo.Transaction(func(tx *gorm.DB) error {
		paymentRepo := s.paymentRepo.WithTx(tx)

		if err := paymentRepo.Update(payment); err != nil {
			return ErrPaymentUpdateFailed
		}

		if status == constants.PaymentStatusSuccess && order.Status != constants.OrderStatusPaid {
			if err := s.markOrderPaid(tx, order, now); err != nil {
				return err
			}
			orderPaid = true
		}
		if (status == constants.PaymentStatusFailed || status == constants.PaymentStatusExpired) && order.Status == constants.OrderStatusPendingPayment && s.walletSvc != nil {
			if _, err := s.walletSvc.ReleaseOrderBalance(tx, order, constants.WalletTxnTypeOrderRefund, "在线支付失败，退回余额"); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return returnVal, orderPaid, nil
}

// markOrderPaid 在事务内将订单更新为已支付并处理库存
func (s *PaymentService) markOrderPaid(tx *gorm.DB, order *models.Order, now time.Time) error {
	if order == nil {
		return ErrOrderNotFound
	}
	if !isTransitionAllowed(order.Status, constants.OrderStatusPaid) {
		return ErrOrderStatusInvalid
	}
	orderRepo := s.orderRepo.WithTx(tx)
	productRepo := s.productRepo.WithTx(tx)
	var productSKURepo repository.ProductSKURepository
	if s.productSKURepo != nil {
		productSKURepo = s.productSKURepo.WithTx(tx)
	}

	onlineAmount := normalizeOrderAmount(order.TotalAmount.Decimal.Sub(order.WalletPaidAmount.Decimal))
	orderUpdates := map[string]interface{}{
		"paid_at":            now,
		"online_paid_amount": models.NewMoneyFromDecimal(onlineAmount),
		"updated_at":         now,
	}
	if err := orderRepo.UpdateStatus(order.ID, constants.OrderStatusPaid, orderUpdates); err != nil {
		return ErrOrderUpdateFailed
	}
	order.Status = constants.OrderStatusPaid
	order.PaidAt = &now
	order.OnlinePaidAmount = models.NewMoneyFromDecimal(onlineAmount)
	order.UpdatedAt = now

	if len(order.Children) > 0 {
		for idx := range order.Children {
			child := &order.Children[idx]
			childStatus := constants.OrderStatusPaid
			if shouldMarkFulfilling(child) {
				childStatus = constants.OrderStatusFulfilling
			}
			if err := orderRepo.UpdateStatus(child.ID, childStatus, map[string]interface{}{
				"paid_at":    now,
				"updated_at": now,
			}); err != nil {
				return ErrOrderUpdateFailed
			}
			if err := consumeManualStockByItems(productRepo, productSKURepo, child.Items); err != nil {
				return err
			}
			child.Status = childStatus
			child.PaidAt = &now
			child.UpdatedAt = now
		}
		parentStatus := calcParentStatus(order.Children, constants.OrderStatusPaid)
		if parentStatus != "" && parentStatus != constants.OrderStatusPaid {
			if err := orderRepo.UpdateStatus(order.ID, parentStatus, map[string]interface{}{
				"online_paid_amount": models.NewMoneyFromDecimal(onlineAmount),
				"updated_at":         now,
			}); err != nil {
				return ErrOrderUpdateFailed
			}
			order.Status = parentStatus
		}
		return nil
	}

	if err := consumeManualStockByItems(productRepo, productSKURepo, order.Items); err != nil {
		return err
	}
	return nil
}

func (s *PaymentService) enqueueOrderPaidAsync(order *models.Order, payment *models.Payment, log *zap.SugaredLogger) {
	if order == nil {
		return
	}
	if s.affiliateSvc != nil {
		if err := s.affiliateSvc.HandleOrderPaid(order.ID); err != nil {
			log.Warnw("affiliate_handle_order_paid_failed",
				"order_id", order.ID,
				"order_no", order.OrderNo,
				"error", err,
			)
		}
	}
	if s.queueClient != nil {
		if _, err := enqueueOrderStatusEmailTaskIfEligible(s.orderRepo, s.queueClient, order.ID, constants.OrderStatusPaid); err != nil {
			log.Warnw("payment_enqueue_status_email_failed",
				"order_id", order.ID,
				"order_no", order.OrderNo,
				"status", constants.OrderStatusPaid,
				"error", err,
			)
		}
	}
	s.enqueueOrderPaidNotificationAsync(order, payment, log)
	s.enqueueExceptionAlertCheckAsync("order_paid", log)

	if s.queueClient == nil {
		return
	}
	if len(order.Children) > 0 {
		for _, child := range order.Children {
			if child.Status == constants.OrderStatusFulfilling {
				s.enqueueManualFulfillmentPendingAsync(&child, order, log)
			}
			if shouldAutoFulfill(&child) {
				if err := s.queueClient.EnqueueOrderAutoFulfill(queue.OrderAutoFulfillPayload{
					OrderID: child.ID,
				}, asynq.MaxRetry(3)); err != nil {
					log.Warnw("payment_enqueue_auto_fulfill_failed",
						"order_id", order.ID,
						"child_order_id", child.ID,
						"order_no", order.OrderNo,
						"error", err,
					)
				}
			}
		}
		// 上游采购：为包含上游交付类型的订单创建采购单
		s.enqueueProcurementAsync(order, log)
		// B 侧：订单支付成功后检查是否需要回调下游
		s.enqueueDownstreamCallbackAsync(order, log)
		return
	}
	if order.Status == constants.OrderStatusFulfilling {
		s.enqueueManualFulfillmentPendingAsync(order, nil, log)
	}
	if shouldAutoFulfill(order) {
		if err := s.queueClient.EnqueueOrderAutoFulfill(queue.OrderAutoFulfillPayload{
			OrderID: order.ID,
		}, asynq.MaxRetry(3)); err != nil {
			log.Warnw("payment_enqueue_auto_fulfill_failed",
				"order_id", order.ID,
				"order_no", order.OrderNo,
				"error", err,
			)
		}
	}
	// 上游采购：为包含上游交付类型的订单创建采购单
	s.enqueueProcurementAsync(order, log)
	// B 侧：订单支付成功后检查是否需要回调下游
	s.enqueueDownstreamCallbackAsync(order, log)
}

// enqueueProcurementAsync 如果订单包含上游交付类型商品，创建采购单
func (s *PaymentService) enqueueProcurementAsync(order *models.Order, log *zap.SugaredLogger) {
	if s.procurementSvc == nil || order == nil {
		return
	}
	if err := s.procurementSvc.CreateForOrder(order.ID); err != nil {
		if !errors.Is(err, ErrProcurementExists) {
			log.Warnw("payment_enqueue_procurement_failed",
				"order_id", order.ID,
				"order_no", order.OrderNo,
				"error", err,
			)
		}
	}
}

// enqueueDownstreamCallbackAsync B 侧：通知下游 A 站点订单已支付
func (s *PaymentService) enqueueDownstreamCallbackAsync(order *models.Order, log *zap.SugaredLogger) {
	if s.downstreamCallbackSvc == nil || order == nil {
		return
	}
	s.downstreamCallbackSvc.EnqueueCallback(order.ID)
}

func (s *PaymentService) enqueueOrderPaidNotificationAsync(order *models.Order, payment *models.Payment, log *zap.SugaredLogger) {
	if s.notificationSvc == nil || order == nil {
		return
	}
	providerType := ""
	channelType := ""
	payload := models.JSON{
		"order_id":     fmt.Sprintf("%d", order.ID),
		"order_no":     strings.TrimSpace(order.OrderNo),
		"user_id":      fmt.Sprintf("%d", order.UserID),
		"guest_email":  strings.TrimSpace(order.GuestEmail),
		"amount":       order.TotalAmount.String(),
		"currency":     strings.ToUpper(strings.TrimSpace(order.Currency)),
		"order_status": strings.TrimSpace(order.Status),
	}
	if payment != nil {
		payload["payment_id"] = fmt.Sprintf("%d", payment.ID)
		providerType = strings.TrimSpace(payment.ProviderType)
		channelType = strings.TrimSpace(payment.ChannelType)
	}
	// 钱包全额支付不会生成在线支付单，这里补充可读渠道标识，避免模板渲染为空。
	if providerType == "" && order.WalletPaidAmount.Decimal.GreaterThan(decimal.Zero) {
		providerType = constants.PaymentProviderWallet
		channelType = constants.PaymentChannelTypeBalance
	}
	if providerType != "" {
		payload["provider_type"] = providerType
	}
	if channelType != "" {
		payload["channel_type"] = channelType
	}
	if err := s.notificationSvc.Enqueue(NotificationEnqueueInput{
		EventType: constants.NotificationEventOrderPaidSuccess,
		BizType:   constants.NotificationBizTypeOrder,
		BizID:     order.ID,
		Locale:    strings.TrimSpace(order.GuestLocale),
		Data:      payload,
	}); err != nil {
		log.Warnw("notification_enqueue_order_paid_failed",
			"order_id", order.ID,
			"order_no", order.OrderNo,
			"error", err,
		)
	}
}

func (s *PaymentService) enqueueWalletRechargeSuccessAsync(recharge *models.WalletRechargeOrder, payment *models.Payment, log *zap.SugaredLogger) {
	if s.notificationSvc == nil || recharge == nil {
		return
	}
	payload := models.JSON{
		"user_id":       fmt.Sprintf("%d", recharge.UserID),
		"recharge_id":   fmt.Sprintf("%d", recharge.ID),
		"recharge_no":   strings.TrimSpace(recharge.RechargeNo),
		"amount":        recharge.Amount.String(),
		"currency":      strings.ToUpper(strings.TrimSpace(recharge.Currency)),
		"provider_type": strings.TrimSpace(recharge.ProviderType),
		"channel_type":  strings.TrimSpace(recharge.ChannelType),
	}
	if payment != nil {
		payload["payment_id"] = fmt.Sprintf("%d", payment.ID)
	}
	if err := s.notificationSvc.Enqueue(NotificationEnqueueInput{
		EventType: constants.NotificationEventWalletRechargeSuccess,
		BizType:   constants.NotificationBizTypeWalletRecharge,
		BizID:     recharge.ID,
		Data:      payload,
	}); err != nil {
		log.Warnw("notification_enqueue_wallet_recharge_failed",
			"recharge_id", recharge.ID,
			"recharge_no", recharge.RechargeNo,
			"error", err,
		)
	}
}

func (s *PaymentService) enqueueManualFulfillmentPendingAsync(order *models.Order, parent *models.Order, log *zap.SugaredLogger) {
	if s.notificationSvc == nil || order == nil {
		return
	}
	payload := models.JSON{
		"order_id":     fmt.Sprintf("%d", order.ID),
		"order_no":     strings.TrimSpace(order.OrderNo),
		"user_id":      fmt.Sprintf("%d", order.UserID),
		"guest_email":  strings.TrimSpace(order.GuestEmail),
		"order_status": strings.TrimSpace(order.Status),
	}
	if parent != nil {
		payload["parent_order_id"] = fmt.Sprintf("%d", parent.ID)
		payload["parent_order_no"] = strings.TrimSpace(parent.OrderNo)
	}
	if err := s.notificationSvc.Enqueue(NotificationEnqueueInput{
		EventType: constants.NotificationEventManualFulfillmentPending,
		BizType:   constants.NotificationBizTypeOrder,
		BizID:     order.ID,
		Locale:    strings.TrimSpace(order.GuestLocale),
		Data:      payload,
	}); err != nil {
		log.Warnw("notification_enqueue_manual_pending_failed",
			"order_id", order.ID,
			"order_no", order.OrderNo,
			"error", err,
		)
	}
}

func (s *PaymentService) enqueueExceptionAlertCheckAsync(reason string, log *zap.SugaredLogger) {
	if s.notificationSvc == nil {
		return
	}
	payload := models.JSON{
		"message": strings.TrimSpace(reason),
	}
	if err := s.notificationSvc.Enqueue(NotificationEnqueueInput{
		EventType: constants.NotificationEventExceptionAlertCheck,
		BizType:   constants.NotificationBizTypeDashboardAlert,
		BizID:     0,
		Data:      payload,
	}); err != nil {
		log.Warnw("notification_enqueue_exception_check_failed",
			"reason", reason,
			"error", err,
		)
	}
}
