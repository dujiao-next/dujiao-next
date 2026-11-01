package consumer

import (
	"context"
	"errors"

	pc "github.com/dujiao-next/internal/modules/payment/contract"
	"github.com/dujiao-next/internal/queue"
)

func (c *Consumer) SendPaymentStatusEmail(ctx context.Context, orderID uint, status string, admit func(context.Context) error) error {
	return c.sendOrderStatusEmail(ctx, queue.OrderStatusEmailPayload{OrderID: orderID, Status: status}, admit)
}

func (c *Consumer) SendPaymentBot(ctx context.Context, input pc.BotNotification, admit func(context.Context) error) error {
	return c.sendBotNotify(ctx, queue.BotNotifyPayload{EventType: input.EventType, OrderID: input.OrderID, TelegramUserID: input.TelegramUserID}, admit)
}

// Already queued legacy tasks cannot bypass the fixed-payment SQL owner. A nil
// payment store is tolerated in unrelated standalone worker tests, not production.
func (c *Consumer) usesFixedPaymentEffects(orderID uint) (bool, error) {
	if orderID == 0 || c == nil || c.Container == nil || c.PaymentStore == nil {
		return false, nil
	}
	reader, ok := c.PaymentStore.(interface{ UsesFixedPaymentEffects(uint) (bool, error) })
	if !ok {
		return false, errors.New("payment effect ownership reader unavailable")
	}
	return reader.UsesFixedPaymentEffects(orderID)
}
