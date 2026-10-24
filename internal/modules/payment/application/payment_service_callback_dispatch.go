package application

import (
	"errors"
	"strings"

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
			// 支付已提交，佣金写入失败交给队列重试（佣金表唯一索引保证重复执行安全）
			if s.queue != nil && s.queue.Enabled() {
				if enqueueErr := s.queue.EnqueueAffiliateOrderPaid(order.ID); enqueueErr != nil {
					log.Warnw("affiliate_order_paid_enqueue_retry_failed", "order_id", order.ID, "error", enqueueErr)
				}
			}
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
