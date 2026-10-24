package queueadapter

import (
	"strings"
	"time"

	ordercontract "github.com/dujiao-next/internal/modules/order/contract"
	orderqueue "github.com/dujiao-next/internal/modules/order/infrastructure/queueadapter"
	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"
	"github.com/dujiao-next/internal/queue"

	"github.com/hibiken/asynq"
)

type Queue struct {
	client *queue.Client
	orders ordercontract.Queue
}

var _ paymentcontract.Queue = (*Queue)(nil)

func New(client *queue.Client) *Queue {
	return &Queue{client: client, orders: orderqueue.New(client)}
}

func (q *Queue) Enabled() bool {
	return q != nil && q.client != nil && q.client.Enabled()
}

func (q *Queue) EnqueueTimeoutCancel(orderID uint, delay time.Duration) error {
	return q.orders.EnqueueTimeoutCancel(orderID, delay)
}

func (q *Queue) EnqueueStatusEmail(orderID uint, status string) error {
	return q.orders.EnqueueStatusEmail(orderID, status)
}

func (q *Queue) EnqueueOrderAutoFulfill(orderID uint) error {
	if q == nil || q.client == nil {
		return nil
	}
	return q.client.EnqueueOrderAutoFulfill(queue.OrderAutoFulfillPayload{OrderID: orderID}, asynq.MaxRetry(3))
}

func (q *Queue) EnqueueAffiliateOrderPaid(orderID uint) error {
	if q == nil || q.client == nil {
		return nil
	}
	return q.client.EnqueueAffiliateOrderPaid(queue.AffiliateOrderPaidPayload{OrderID: orderID}, asynq.ProcessIn(time.Minute), asynq.MaxRetry(5))
}

func (q *Queue) EnqueueBotNotification(input paymentcontract.BotNotification) error {
	if q == nil || q.client == nil {
		return nil
	}
	return q.client.EnqueueBotNotify(queue.BotNotifyPayload{
		EventType:      strings.TrimSpace(input.EventType),
		OrderID:        input.OrderID,
		TelegramUserID: strings.TrimSpace(input.TelegramUserID),
		RechargeNo:     strings.TrimSpace(input.RechargeNo),
		Amount:         strings.TrimSpace(input.Amount),
		Currency:       strings.TrimSpace(input.Currency),
	})
}

func (q *Queue) EnqueueWalletRechargeExpire(paymentID uint, delay time.Duration) error {
	if q == nil || q.client == nil {
		return nil
	}
	return q.client.EnqueueWalletRechargeExpire(queue.WalletRechargeExpirePayload{PaymentID: paymentID}, delay)
}
