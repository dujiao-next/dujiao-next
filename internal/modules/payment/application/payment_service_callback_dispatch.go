package application

import (
	"context"
	"errors"
	memberlevelcontract "github.com/dujiao-next/internal/modules/memberlevel/contract"
	ordercontract "github.com/dujiao-next/internal/modules/order/contract"
	"github.com/dujiao-next/internal/modules/payment/application/settlement"
	"strings"
	"time"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	orderapp "github.com/dujiao-next/internal/modules/order/application"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"

	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"

	"github.com/dujiao-next/internal/constants"
	notificationformat "github.com/dujiao-next/internal/modules/notification/application/format"
	notificationcontract "github.com/dujiao-next/internal/modules/notification/contract"
	procurementcontract "github.com/dujiao-next/internal/modules/procurement/contract"
	"go.uber.org/zap"
)

func (s *PaymentService) enqueueOrderPaidAsync(order *orderdomain.Order, payment *paymentdomain.Payment, log *zap.SugaredLogger) {
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
	if s.queue != nil && s.queue.Enabled() && !isOrderFullyAutoFulfill(order) {
		// 完全自动交付的订单会紧接着发送含卡密内容的"已完成"邮件，跳过"已支付"邮件避免重复打扰
		if _, err := orderapp.EnqueueStatusEmailTaskIfEligible(s.orderRepo, s.queue, s.settingService, s.defaultEmailConfig, order.ID, constants.OrderStatusPaid); err != nil {
			log.Warnw("payment_enqueue_status_email_failed",
				"order_id", order.ID,
				"order_no", order.OrderNo,
				"status", constants.OrderStatusPaid,
				"error", err,
			)
		}
	}
	s.enqueueOrderPaidNotificationAsync(order, payment, log)
	s.enqueueOrderPaidBotNotifyAsync(order, log)

	// 订单支付成功后触发会员等级升级检查
	if s.memberLevelSvc != nil && order.UserID > 0 {
		if err := s.memberLevelSvc.OnOrderPaid(order.UserID, order.TotalAmount.Decimal); err != nil {
			log.Warnw("member_level_order_paid_failed",
				"order_id", order.ID,
				"user_id", order.UserID,
				"amount", order.TotalAmount.Decimal.String(),
				"error", err,
			)
		}
	}

	if s.queue == nil || !s.queue.Enabled() {
		return
	}
	if len(order.Children) > 0 {
		for _, child := range order.Children {
			if child.Status == constants.OrderStatusFulfilling && hasManualFulfillmentItems(&child) {
				s.enqueueManualFulfillmentPendingAsync(&child, order, log)
			}
			if shouldAutoFulfill(&child) {
				if err := s.queue.EnqueueOrderAutoFulfill(child.ID); err != nil {
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
	if order.Status == constants.OrderStatusFulfilling && hasManualFulfillmentItems(order) {
		s.enqueueManualFulfillmentPendingAsync(order, nil, log)
	}
	if shouldAutoFulfill(order) {
		if err := s.queue.EnqueueOrderAutoFulfill(order.ID); err != nil {
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
func (s *PaymentService) enqueueProcurementAsync(order *orderdomain.Order, log *zap.SugaredLogger) {
	if s.procurementSvc == nil || order == nil {
		return
	}
	if err := s.procurementSvc.CreateForOrder(order.ID); err != nil {
		if !errors.Is(err, procurementcontract.ErrExists) {
			log.Warnw("payment_enqueue_procurement_failed",
				"order_id", order.ID,
				"order_no", order.OrderNo,
				"error", err,
			)
		}
	}
}

// enqueueDownstreamCallbackAsync B 侧：通知下游 A 站点订单已支付
func (s *PaymentService) enqueueDownstreamCallbackAsync(order *orderdomain.Order, log *zap.SugaredLogger) {
	if s.downstreamCallbackSvc == nil || order == nil {
		return
	}
	s.downstreamCallbackSvc.EnqueueCallback(order.ID)
}

func (s *PaymentService) enqueueOrderPaidNotificationAsync(order *orderdomain.Order, payment *paymentdomain.Payment, log *zap.SugaredLogger) {
	if s.notificationSvc == nil || order == nil {
		return
	}
	payload := s.buildOrderNotificationPayload(order, payment)
	if err := s.notificationSvc.Enqueue(notificationcontract.EnqueueInput{
		EventType: constants.NotificationEventOrderPaidSuccess,
		BizType:   constants.NotificationBizTypeOrder,
		BizID:     order.ID,
		Data:      payload,
	}); err != nil {
		log.Warnw("notification_enqueue_order_paid_failed",
			"order_id", order.ID,
			"order_no", order.OrderNo,
			"error", err,
		)
	}
}

func (s *PaymentService) enqueueWalletRechargeSuccessAsync(recharge *walletdomain.RechargeOrder, payment *paymentdomain.Payment, log *zap.SugaredLogger) {
	if s.notificationSvc == nil || recharge == nil {
		return
	}
	payload := s.buildWalletRechargeNotificationPayload(recharge, payment)
	if err := s.notificationSvc.Enqueue(notificationcontract.EnqueueInput{
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

func (s *PaymentService) enqueueOrderPaidBotNotifyAsync(order *orderdomain.Order, log *zap.SugaredLogger) {
	if s.queue == nil || !s.queue.Enabled() || order == nil || order.UserID == 0 || s.userOAuthIdentityRepo == nil {
		return
	}

	identity, err := s.userOAuthIdentityRepo.GetByUserProvider(order.UserID, constants.UserOAuthProviderTelegram)
	if err != nil {
		log.Warnw("order_paid_notify_bot_fetch_identity_failed",
			"order_id", order.ID,
			"user_id", order.UserID,
			"error", err,
		)
		return
	}
	if identity == nil || strings.TrimSpace(identity.ProviderUserID) == "" {
		return
	}

	if err := s.queue.EnqueueBotNotification(paymentcontract.BotNotification{
		EventType:      paymentcontract.BotNotificationOrderPaid,
		OrderID:        order.ID,
		TelegramUserID: strings.TrimSpace(identity.ProviderUserID),
	}); err != nil {
		log.Warnw("order_paid_notify_bot_enqueue_failed",
			"order_id", order.ID,
			"order_no", order.OrderNo,
			"user_id", order.UserID,
			"error", err,
		)
	}
}

func (s *PaymentService) enqueueWalletRechargeBotNotifyAsync(recharge *walletdomain.RechargeOrder, log *zap.SugaredLogger) {
	if s.queue == nil || !s.queue.Enabled() || recharge == nil || recharge.UserID == 0 || s.userOAuthIdentityRepo == nil {
		return
	}

	identity, err := s.userOAuthIdentityRepo.GetByUserProvider(recharge.UserID, constants.UserOAuthProviderTelegram)
	if err != nil {
		log.Warnw("wallet_recharge_notify_bot_fetch_identity_failed",
			"recharge_id", recharge.ID,
			"user_id", recharge.UserID,
			"error", err,
		)
		return
	}
	if identity == nil || strings.TrimSpace(identity.ProviderUserID) == "" {
		return
	}

	if err := s.queue.EnqueueBotNotification(paymentcontract.BotNotification{
		EventType:      paymentcontract.BotNotificationWalletRechargeSucceeded,
		TelegramUserID: strings.TrimSpace(identity.ProviderUserID),
		RechargeNo:     strings.TrimSpace(recharge.RechargeNo),
		Amount:         recharge.Amount.String(),
		Currency:       strings.ToUpper(strings.TrimSpace(recharge.Currency)),
	}); err != nil {
		log.Warnw("wallet_recharge_notify_bot_enqueue_failed",
			"recharge_id", recharge.ID,
			"recharge_no", recharge.RechargeNo,
			"user_id", recharge.UserID,
			"error", err,
		)
	}
}

// hasManualFulfillmentItems 判断订单是否包含需要人工交付的商品项。
// upstream 类型由采购流程自动交付，不触发待人工交付提醒。
func hasManualFulfillmentItems(order *orderdomain.Order) bool {
	if order == nil {
		return false
	}
	for _, item := range order.Items {
		if notificationformat.NormalizeFulfillmentType(item.FulfillmentType) == constants.FulfillmentTypeManual {
			return true
		}
	}
	return false
}

func (s *PaymentService) enqueueManualFulfillmentPendingAsync(order *orderdomain.Order, parent *orderdomain.Order, log *zap.SugaredLogger) {
	if s.notificationSvc == nil || order == nil {
		return
	}
	payload := s.buildManualFulfillmentNotificationPayload(order, parent)
	if err := s.notificationSvc.Enqueue(notificationcontract.EnqueueInput{
		EventType: constants.NotificationEventManualFulfillmentPending,
		BizType:   constants.NotificationBizTypeOrder,
		BizID:     order.ID,
		Data:      payload,
	}); err != nil {
		log.Warnw("notification_enqueue_manual_pending_failed",
			"order_id", order.ID,
			"order_no", order.OrderNo,
			"error", err,
		)
	}
}

// HandlePaymentEffect retries one responsibility, never the legacy combined
// dispatcher. Monetary increments must not be invoked from this method.
func (s *PaymentService) HandlePaymentEffect(ctx context.Context, effect paymentdomain.PaymentEffect) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	control, ok := s.paymentRepo.(interface {
		OpenAutomationRevision(context.Context) (uint64, error)
	})
	if !ok {
		return false, paymentdomain.ErrAutomationFrozen
	}
	revision, err := control.OpenAutomationRevision(ctx)
	if err != nil {
		return false, err
	}
	if effect.AutomationRevision != 0 && effect.AutomationRevision != revision {
		return false, paymentdomain.ErrAutomationFrozen
	}
	order, err := s.orderRepo.GetByID(effect.OrderID)
	if err != nil {
		return false, err
	}
	if order == nil {
		return false, ErrPaymentInvalid
	}
	payment, err := s.paymentRepo.GetByID(effect.PaymentID)
	if err != nil {
		return false, err
	}
	if payment == nil || !payment.RequiresChainVerification || payment.Status != constants.PaymentStatusSuccess || order.PaidAt == nil {
		return false, ErrPaymentInvalid
	}
	if strings.HasPrefix(effect.Kind, "paid_") || strings.HasPrefix(effect.Kind, "delivery_") || effect.Kind == "manual_notice" || effect.Kind == "downstream" {
		if !order.RefundedAmount.Decimal.IsZero() || order.Status == constants.OrderStatusCanceled || order.Status == constants.OrderStatusRefunded || order.Status == constants.OrderStatusPartiallyRefunded {
			// A delayed success/delivery message cannot describe a newer refund.
			// Refund notifications remain owned by the existing refund workflow.
			return false, paymentcontract.ErrEffectPermanent
		}
	}
	if strings.HasPrefix(effect.Kind, "delivery_") {
		leaves := order.Children
		if len(leaves) == 0 {
			leaves = []orderdomain.Order{*order}
		}
		for _, leaf := range leaves {
			if (leaf.Status != constants.OrderStatusCompleted && leaf.Status != constants.OrderStatusDelivered) || leaf.Fulfillment == nil || leaf.Fulfillment.Status != constants.FulfillmentStatusDelivered {
				return false, nil
			}
		}
		if len(order.Children) > 0 {
			if _, err := orderapp.SyncParentStatus(s.orderRepo, order.ID, time.Now()); err != nil {
				return false, err
			}
		}
	}
	admit := func(ctx context.Context) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		store, ok := s.paymentRepo.(interface {
			AdmitPaymentEffect(context.Context, paymentdomain.PaymentEffect) error
		})
		if !ok {
			return ErrPaymentInvalid
		}
		return store.AdmitPaymentEffect(ctx, effect)
	}
	switch effect.Kind {
	case "member_level":
		if s.memberLevelSvc == nil {
			return true, nil
		}
		checker, ok := s.memberLevelSvc.(interface {
			PrepareUpgrade(uint) (func(memberlevelcontract.UserRepository) error, error)
		})
		if !ok {
			return false, ErrPaymentInvalid
		}
		apply, err := checker.PrepareUpgrade(order.UserID)
		if err != nil {
			return false, err
		}
		err = s.paymentRepo.WithinTransaction(func(base paymentcontract.Transaction) error {
			tx, ok := base.(paymentcontract.SettlementTransaction)
			if !ok {
				return ErrPaymentInvalid
			}
			if err := tx.LockAutomation(revision); err != nil {
				return err
			}
			locked, err := tx.Orders().GetByIDForUpdateWithChildren(order.ID)
			if err != nil {
				return err
			}
			if locked == nil || locked.UserID != order.UserID || locked.PaidAt == nil || locked.ParentID != nil {
				return ErrPaymentInvalid
			}
			p, err := tx.Payments().GetByIDForUpdate(payment.ID)
			if err != nil {
				return err
			}
			if p == nil || p.OrderID != locked.ID || !p.RequiresChainVerification || p.Status != constants.PaymentStatusSuccess {
				return ErrPaymentInvalid
			}
			a, err := tx.Allocation(p.ID)
			if err != nil {
				return err
			}
			if a == nil || a.OrderID != locked.ID || a.Snapshot != paymentdomain.SettlementSnapshot(*p, *locked) {
				return ErrPaymentInvalid
			}
			claimed, err := tx.CompletePaymentEffect(effect)
			if err != nil {
				return err
			}
			if !claimed {
				return ErrPaymentInvalid
			}
			if !locked.RefundedAmount.Decimal.IsZero() || locked.Status == constants.OrderStatusCanceled || locked.Status == constants.OrderStatusRefunded || locked.Status == constants.OrderStatusPartiallyRefunded {
				return nil
			}
			return apply(tx.Users())
		})
		return true, err
	case "affiliate":
		if s.affiliateSvc == nil {
			return true, nil
		}
		preparer, ok := s.affiliateSvc.(interface {
			PrepareOrderPaid() (func(ordercontract.Transaction, *orderdomain.Order) error, error)
		})
		if !ok {
			return false, ErrPaymentInvalid
		}
		apply, err := preparer.PrepareOrderPaid()
		if err != nil {
			return false, err
		}
		err = s.paymentRepo.WithinTransaction(func(tx paymentcontract.Transaction) error {
			t, ok := tx.(paymentcontract.SettlementTransaction)
			if !ok {
				return ErrPaymentInvalid
			}
			if err := t.LockAutomation(revision); err != nil {
				return err
			}
			locked, err := tx.Orders().GetByIDForUpdateWithChildren(order.ID)
			if err != nil {
				return err
			}
			if locked == nil {
				return ErrPaymentInvalid
			}
			claimed, err := t.CompletePaymentEffect(effect)
			if err != nil || !claimed {
				return err
			}
			if locked.PaidAt == nil || !locked.RefundedAmount.Decimal.IsZero() || locked.Status == constants.OrderStatusCanceled {
				return nil
			}
			a, err := t.Allocation(payment.ID)
			if err != nil {
				return err
			}
			if a == nil || a.Snapshot != paymentdomain.SettlementSnapshot(*payment, *locked) {
				return ErrPaymentInvalid
			}
			return apply(tx, locked)
		})
		return true, err
	case "auto_fulfill":
		if order.Status == constants.OrderStatusCompleted && order.Fulfillment != nil && order.Fulfillment.Status == constants.FulfillmentStatusDelivered {
			if order.ParentID != nil {
				_, err := orderapp.SyncParentStatus(s.orderRepo, *order.ParentID, time.Now())
				return true, err
			}
			return true, nil
		}
		if order.Status != constants.OrderStatusPaid || !order.RefundedAmount.Decimal.IsZero() {
			return false, ErrPaymentInvalid
		}
		if s.queue == nil || !s.queue.Enabled() {
			return false, ErrQueueUnavailable
		}
		// SQL remains pending even if Redis accepts the task. Actual delivery
		// is the acknowledgment; Redis loss and exhausted retries are recoverable.
		queue, ok := s.queue.(interface{ EnqueueVerifiedOrderAutoFulfill(uint) error })
		if !ok {
			return false, ErrQueueUnavailable
		}
		return false, queue.EnqueueVerifiedOrderAutoFulfill(order.ID)
	case "downstream", "delivery_downstream":
		if s.downstreamCallbackSvc == nil {
			return true, nil
		}
		dispatcher, ok := s.downstreamCallbackSvc.(interface {
			SendCallbackDurable(context.Context, uint, func(context.Context) error) error
		})
		if !ok {
			return false, ErrPaymentInvalid
		}
		return true, dispatcher.SendCallbackDurable(ctx, order.ID, admit)
	case "paid_email", "delivery_email":
		if effect.Kind == "paid_email" && isOrderFullyAutoFulfill(order) {
			return true, nil
		}
		if s.effectDelivery == nil {
			return false, ErrQueueUnavailable
		}
		status := constants.OrderStatusPaid
		if effect.Kind == "delivery_email" {
			status = constants.OrderStatusCompleted
		}
		return true, s.effectDelivery.SendPaymentStatusEmail(ctx, order.ID, status, admit)
	case "paid_notification", "manual_notice":
		if s.notificationSvc == nil {
			return true, nil
		}
		payload := s.buildOrderNotificationPayload(order, payment)
		event := constants.NotificationEventOrderPaidSuccess
		if effect.Kind == "manual_notice" {
			event = constants.NotificationEventManualFulfillmentPending
			payload = s.buildManualFulfillmentNotificationPayload(order, nil)
		}
		sender, ok := s.notificationSvc.(interface {
			DispatchRecipients(context.Context, notificationcontract.EnqueueInput, notificationcontract.DeliverBatch) error
		})
		if !ok {
			return false, ErrQueueUnavailable
		}
		store, ok := s.paymentRepo.(settlement.RecipientStore)
		if !ok {
			return false, ErrPaymentInvalid
		}
		return true, sender.DispatchRecipients(ctx, notificationcontract.EnqueueInput{EventType: event, BizType: constants.NotificationBizTypeOrder, BizID: order.ID, Data: payload}, settlement.RecipientBatch(store, effect))
	case "paid_bot", "delivery_bot":
		if s.userOAuthIdentityRepo == nil {
			return true, nil
		}
		identity, err := s.userOAuthIdentityRepo.GetByUserProvider(order.UserID, constants.UserOAuthProviderTelegram)
		if err != nil {
			return false, err
		}
		if identity == nil || strings.TrimSpace(identity.ProviderUserID) == "" {
			return true, nil
		}
		if s.effectDelivery == nil {
			return false, ErrQueueUnavailable
		}
		event := paymentcontract.BotNotificationOrderPaid
		if effect.Kind == "delivery_bot" {
			event = paymentcontract.BotNotificationOrderFulfilled
		}
		return true, s.effectDelivery.SendPaymentBot(ctx, paymentcontract.BotNotification{EventType: event, OrderID: order.ID, TelegramUserID: identity.ProviderUserID}, admit)
	default:
		return false, ErrPaymentInvalid
	}
}
