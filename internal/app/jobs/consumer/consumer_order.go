package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	paymentapp "github.com/dujiao-next/internal/modules/payment/application"

	fulfillmentapp "github.com/dujiao-next/internal/modules/fulfillment/application"
	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	orderapp "github.com/dujiao-next/internal/modules/order/application"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/htmltext"
	"github.com/dujiao-next/internal/logger"
	notificationcontract "github.com/dujiao-next/internal/modules/notification/contract"
	settingsmessaging "github.com/dujiao-next/internal/modules/settings/schema/messaging"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	"github.com/dujiao-next/internal/queue"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/mailbrand"
	"github.com/dujiao-next/internal/telegramidentity"

	"github.com/hibiken/asynq"
)

// handleOrderStatusEmail 处理订单状态邮件发送任务。
func (c *Consumer) handleOrderStatusEmail(ctx context.Context, task *asynq.Task) error {
	if c == nil || task == nil {
		logger.Debugw("worker_order_status_email_skip_nil", "consumer_nil", c == nil, "task_nil", task == nil)
		return nil
	}
	var payload queue.OrderStatusEmailPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		logger.Warnw("worker_order_status_email_unmarshal_failed", "error", err)
		return err
	}
	if payload.OrderID == 0 {
		logger.Debugw("worker_order_status_email_skip_invalid_payload", "order_id", payload.OrderID)
		return nil
	}
	payloadStatus := strings.TrimSpace(payload.Status)
	// 兼容部署前已进入 Redis 的旧取消邮件任务。新代码已不再创建此类任务；
	// 这里保留墓碑分支，避免旧任务在取消模板删除后误用默认模板发送。
	if payloadStatus == constants.OrderStatusCanceled {
		logger.Debugw("worker_order_status_email_skip_canceled", "order_id", payload.OrderID)
		return nil
	}
	// 消费侧二次校验开关：避免 SMTP/订单通知关闭后，旧任务（含 retry/scheduled 中的任务）继续触发发送。
	if c.SettingService != nil && c.Config != nil {
		smtpSetting, smtpErr := c.SettingService.GetSMTPSetting(c.Config.Email)
		if smtpErr != nil {
			// fail-closed：读不到设置就视作关闭，不重试，避免 DB 抖动放大成邮件雪崩。
			logger.Warnw("worker_order_status_email_load_smtp_setting_failed", "order_id", payload.OrderID, "error", smtpErr)
			return nil
		}
		if !smtpSetting.Enabled || !smtpSetting.OrderNotificationEnabled {
			logger.Debugw("worker_order_status_email_skip_setting_disabled",
				"order_id", payload.OrderID,
				"smtp_enabled", smtpSetting.Enabled,
				"order_notification_enabled", smtpSetting.OrderNotificationEnabled,
			)
			return nil
		}
	}
	if c.orderReader == nil {
		return fmt.Errorf("order reader unavailable")
	}
	order, err := c.orderReader.GetByID(payload.OrderID)
	if err != nil {
		logger.Warnw("worker_order_status_email_fetch_order_failed", "order_id", payload.OrderID, "error", err)
		return err
	}
	if order == nil {
		logger.Debugw("worker_order_status_email_skip_order_not_found", "order_id", payload.OrderID)
		return nil
	}
	status := payloadStatus
	if status == "" {
		status = strings.TrimSpace(order.Status)
	}
	if status == constants.OrderStatusCanceled {
		logger.Debugw("worker_order_status_email_skip_canceled",
			"order_id", order.ID,
			"order_no", order.OrderNo,
		)
		return nil
	}
	var receiverEmail string
	var locale string
	if order.UserID != 0 {
		user, err := c.UserStore.GetByID(order.UserID)
		if err != nil {
			logger.Warnw("worker_order_status_email_fetch_user_failed", "order_id", order.ID, "user_id", order.UserID, "error", err)
			return err
		}
		if user != nil {
			receiverEmail = strings.TrimSpace(user.Email)
			locale = strings.TrimSpace(user.Locale)
		}
	} else {
		receiverEmail = strings.TrimSpace(order.GuestEmail)
		locale = strings.TrimSpace(order.GuestLocale)
	}
	if receiverEmail == "" {
		logger.Debugw("worker_order_status_email_skip_empty_receiver", "order_id", order.ID, "order_no", order.OrderNo)
		return nil
	}
	if telegramidentity.IsPlaceholderEmail(receiverEmail) {
		logger.Debugw("worker_order_status_email_skip_placeholder_receiver", "order_id", order.ID, "order_no", order.OrderNo)
		return nil
	}
	if c.EmailSender == nil {
		logger.Warnw("worker_order_status_email_skip_email_service_nil", "order_id", order.ID, "order_no", order.OrderNo)
		return nil
	}
	var tmplSetting *settingsmessaging.OrderEmailTemplateSetting
	if c.SettingService != nil {
		setting, tmplErr := c.SettingService.GetOrderEmailTemplateSetting()
		if tmplErr != nil {
			logger.Warnw("worker_order_status_email_load_template_failed", "order_id", order.ID, "error", tmplErr)
		} else {
			tmplSetting = &setting
		}
	}
	payloadText := buildOrderFulfillmentEmailPayload(order)
	emailBrand, brandErr := c.resolveOrderEmailBrand(ctx, order)
	if brandErr != nil {
		logger.Warnw("worker_order_status_email_load_site_brand_failed", "order_id", order.ID, "error", brandErr)
		return brandErr
	}
	refundDetails, refundDetailsErr := c.OrderRefundService.ResolveOrderStatusEmailRefundDetails(order, payload.RefundRecordID)
	if refundDetailsErr != nil {
		logger.Warnw("worker_order_status_email_resolve_refund_details_failed",
			"order_id", order.ID,
			"refund_record_id", payload.RefundRecordID,
			"error", refundDetailsErr,
		)
	}
	input := notificationcontract.OrderStatusEmailInput{
		OrderNo:      order.OrderNo,
		Status:       status,
		Amount:       order.TotalAmount,
		RefundAmount: refundDetails.Amount,
		RefundReason: refundDetails.Reason,
		Currency:     order.Currency,
		SiteName:     emailBrand.SiteName,
		SiteURL:      emailBrand.SiteURL,
		IsGuest:      order.UserID == 0,
		MailBrand:    emailBrand,
	}
	if fulfillmentdomain.ShouldAttachFulfillmentPayload(payloadText) {
		// 交付内容过大，正文不放交付内容，以附件形式发送
		input.AttachmentName = fmt.Sprintf("order_%s_delivery.txt", order.OrderNo)
		input.AttachmentContent = payloadText
	} else {
		input.FulfillmentInfo = payloadText
	}
	// 使用说明只在交付类场景追加，避免被追加到退款/取消等无关邮件正文里。
	if status == constants.OrderStatusDelivered || status == constants.OrderStatusCompleted {
		input.Instructions = buildOrderInstructionsEmailText(order, locale)
	}
	if err := c.EmailSender.SendOrderStatusEmailWithTemplate(receiverEmail, input, locale, tmplSetting); err != nil {
		switch {
		case errors.Is(err, notificationcontract.ErrEmailServiceDisabled):
			logger.Debugw("worker_order_status_email_skip_email_disabled",
				"order_id", order.ID,
				"order_no", order.OrderNo,
				"receiver_email", receiverEmail,
				"status", status,
			)
			return nil
		case errors.Is(err, notificationcontract.ErrEmailNotConfigured):
			logger.Debugw("worker_order_status_email_skip_email_not_configured",
				"order_id", order.ID,
				"order_no", order.OrderNo,
				"receiver_email", receiverEmail,
				"status", status,
			)
			return nil
		case errors.Is(err, notificationcontract.ErrInvalidEmail):
			logger.Debugw("worker_order_status_email_skip_invalid_email",
				"order_id", order.ID,
				"order_no", order.OrderNo,
				"receiver_email", receiverEmail,
				"status", status,
			)
			return nil
		default:
			logger.Warnw("worker_order_status_email_send_failed",
				"order_id", order.ID,
				"order_no", order.OrderNo,
				"receiver_email", receiverEmail,
				"status", status,
				"error", err,
			)
			return err
		}
	}
	return nil
}

func (c *Consumer) resolveOrderEmailBrand(ctx context.Context, order *orderdomain.Order) (mailbrand.Brand, error) {
	if order == nil {
		return mailbrand.Brand{}, nil
	}
	scope := mailbrand.Scope{}
	if order.ResellerID != nil {
		scope.ResellerID = order.ResellerID
		scope.Host = order.ResellerDomain
	}
	if c != nil && c.Container != nil && c.EmailBrandResolver != nil {
		return c.EmailBrandResolver.ResolveEmailBrand(ctx, scope)
	}
	if order.ResellerID != nil {
		// Fail closed for white-label identity: a missing resolver may degrade to
		// the order's own domain, but must never expose the global main brand.
		return mailbrand.ResellerFallback(order.ResellerDomain), nil
	}
	if c != nil && c.Container != nil && c.SettingService != nil {
		brand, err := c.SettingService.GetSiteBrand()
		if err != nil {
			return mailbrand.Brand{}, err
		}
		return mailbrand.Brand{
			SiteName: strings.TrimSpace(brand.SiteName),
			SiteURL:  strings.TrimRight(strings.TrimSpace(brand.SiteURL), "/"),
		}, nil
	}
	return mailbrand.Brand{}, nil
}

// handleOrderAutoFulfill 处理自动交付任务。
func (c *Consumer) handleOrderAutoFulfill(_ context.Context, task *asynq.Task) error {
	if c == nil || task == nil {
		logger.Debugw("worker_order_auto_fulfill_skip_nil", "consumer_nil", c == nil, "task_nil", task == nil)
		return nil
	}
	var payload queue.OrderAutoFulfillPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		logger.Warnw("worker_order_auto_fulfill_unmarshal_failed", "error", err)
		return err
	}
	if payload.OrderID == 0 {
		logger.Debugw("worker_order_auto_fulfill_skip_invalid_payload", "order_id", payload.OrderID)
		return nil
	}
	_, err := c.FulfillmentService.CreateAuto(payload.OrderID)
	if err != nil {
		switch {
		case errors.Is(err, fulfillmentapp.ErrFulfillmentExists):
			logger.Debugw("worker_order_auto_fulfill_skip_exists", "order_id", payload.OrderID)
			return nil
		case errors.Is(err, fulfillmentapp.ErrFulfillmentNotAuto):
			logger.Debugw("worker_order_auto_fulfill_skip_not_auto", "order_id", payload.OrderID)
			return nil
		case errors.Is(err, orderapp.ErrOrderStatusInvalid):
			logger.Debugw("worker_order_auto_fulfill_skip_invalid_status", "order_id", payload.OrderID)
			return nil
		case errors.Is(err, orderapp.ErrOrderNotFound):
			logger.Debugw("worker_order_auto_fulfill_skip_order_not_found", "order_id", payload.OrderID)
			return nil
		default:
			logger.Warnw("worker_order_auto_fulfill_failed", "order_id", payload.OrderID, "error", err)
			return err
		}
	}
	return nil
}

// handleOrderTimeoutCancel 处理超时未支付订单自动取消任务。
func (c *Consumer) handleOrderTimeoutCancel(_ context.Context, task *asynq.Task) error {
	if c == nil || task == nil {
		logger.Debugw("worker_order_timeout_cancel_skip_nil", "consumer_nil", c == nil, "task_nil", task == nil)
		return nil
	}
	var payload queue.OrderTimeoutCancelPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		logger.Warnw("worker_order_timeout_cancel_unmarshal_failed", "error", err)
		return err
	}
	if payload.OrderID == 0 {
		logger.Debugw("worker_order_timeout_cancel_skip_invalid_payload", "order_id", payload.OrderID)
		return nil
	}
	if c.OrderService == nil {
		logger.Warnw("worker_order_timeout_cancel_skip_order_service_nil", "order_id", payload.OrderID)
		return nil
	}
	_, err := c.OrderService.CancelExpiredOrder(payload.OrderID)
	if err != nil {
		switch {
		case errors.Is(err, orderapp.ErrOrderNotFound):
			logger.Debugw("worker_order_timeout_cancel_skip_order_not_found", "order_id", payload.OrderID)
			return nil
		case errors.Is(err, orderapp.ErrOrderFetchFailed):
			// 读库瞬时失败（如 SQLite database is locked）必须交给队列重试，
			// 否则订单永远停在待支付并占着预占库存
			logger.Warnw("worker_order_timeout_cancel_fetch_failed", "order_id", payload.OrderID, "error", err)
			return err
		case errors.Is(err, orderapp.ErrOrderUpdateFailed):
			logger.Warnw("worker_order_timeout_cancel_update_failed", "order_id", payload.OrderID, "error", err)
			return err
		default:
			logger.Warnw("worker_order_timeout_cancel_failed", "order_id", payload.OrderID, "error", err)
			return err
		}
	}
	return nil
}

// handleWalletRechargeExpire 处理钱包充值订单过期任务。
func (c *Consumer) handleWalletRechargeExpire(_ context.Context, task *asynq.Task) error {
	if c == nil || task == nil {
		logger.Debugw("worker_wallet_recharge_expire_skip_nil", "consumer_nil", c == nil, "task_nil", task == nil)
		return nil
	}
	var payload queue.WalletRechargeExpirePayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		logger.Warnw("worker_wallet_recharge_expire_unmarshal_failed", "error", err)
		return err
	}
	if payload.PaymentID == 0 {
		logger.Debugw("worker_wallet_recharge_expire_skip_invalid_payload", "payment_id", payload.PaymentID)
		return nil
	}
	if c.PaymentService == nil {
		logger.Warnw("worker_wallet_recharge_expire_skip_payment_service_nil", "payment_id", payload.PaymentID)
		return nil
	}
	if _, err := c.PaymentService.ExpireWalletRechargePayment(payload.PaymentID); err != nil {
		switch {
		case errors.Is(err, paymentapp.ErrPaymentNotFound):
			logger.Debugw("worker_wallet_recharge_expire_skip_payment_not_found", "payment_id", payload.PaymentID)
			return nil
		case errors.Is(err, walletcontract.ErrRechargeNotFound):
			logger.Debugw("worker_wallet_recharge_expire_skip_recharge_not_found", "payment_id", payload.PaymentID)
			return nil
		case errors.Is(err, paymentapp.ErrPaymentUpdateFailed):
			logger.Warnw("worker_wallet_recharge_expire_update_failed", "payment_id", payload.PaymentID, "error", err)
			return err
		default:
			logger.Warnw("worker_wallet_recharge_expire_failed", "payment_id", payload.PaymentID, "error", err)
			return err
		}
	}
	return nil
}

// buildOrderInstructionsEmailText 收集订单项的交付使用说明（多语言选取 + HTML 去标签 + 去重）。
func buildOrderInstructionsEmailText(order *orderdomain.Order, locale string) string {
	if order == nil {
		return ""
	}
	seen := make(map[string]struct{})
	var parts []string
	add := func(raw jsonmap.JSON) {
		text := localizedInstructionsText(raw, locale)
		if text == "" {
			return
		}
		plain := htmltext.StripToPlainText(text)
		if plain == "" {
			return
		}
		if _, ok := seen[plain]; ok {
			return
		}
		seen[plain] = struct{}{}
		parts = append(parts, plain)
	}
	for _, item := range order.Items {
		add(item.InstructionsJSON)
	}
	for _, child := range order.Children {
		for _, item := range child.Items {
			add(item.InstructionsJSON)
		}
	}
	return strings.Join(parts, "\n\n")
}

func localizedInstructionsText(raw jsonmap.JSON, locale string) string {
	if len(raw) == 0 {
		return ""
	}
	candidates := []string{strings.TrimSpace(locale), "zh-CN", "en-US", "zh-TW"}
	for _, key := range candidates {
		if key == "" {
			continue
		}
		if value, ok := raw[key]; ok {
			if s, ok := value.(string); ok {
				if trimmed := strings.TrimSpace(s); trimmed != "" {
					return trimmed
				}
			}
		}
	}
	return ""
}

// buildOrderFulfillmentEmailPayload 组装订单状态邮件中的交付内容文本。
func buildOrderFulfillmentEmailPayload(order *orderdomain.Order) string {
	if order == nil {
		return ""
	}
	if order.Fulfillment != nil {
		payload := strings.TrimSpace(order.Fulfillment.Payload)
		if payload != "" {
			return payload
		}
	}
	if len(order.Children) == 0 {
		return ""
	}
	parts := make([]string, 0, len(order.Children))
	for _, child := range order.Children {
		if child.Fulfillment == nil {
			continue
		}
		content := strings.TrimSpace(child.Fulfillment.Payload)
		if content == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("[%s]\n%s", strings.TrimSpace(child.OrderNo), content))
	}
	return strings.Join(parts, "\n\n")
}
