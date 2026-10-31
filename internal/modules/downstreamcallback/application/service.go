package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	downstreamcontract "github.com/dujiao-next/internal/modules/downstreamcallback/contract"
	downstreamdomain "github.com/dujiao-next/internal/modules/downstreamcallback/domain"
)

const maxCallbackRetries = 5

var callbackRetryDelays = []time.Duration{
	30 * time.Second,
	60 * time.Second,
	120 * time.Second,
	300 * time.Second,
}

// Options 声明下游回调应用服务的全部端口。
type Options struct {
	PaymentEffects PaymentEffectOwner
	References     downstreamcontract.Repository
	Orders         downstreamcontract.OrderReader
	Credentials    downstreamcontract.CredentialReader
	Queue          downstreamcontract.CallbackQueue
	Deliverer      downstreamcontract.Deliverer
	Now            func() time.Time
}

// Service 编排 B 侧下游回调引用、投递和重试状态。
type Service struct {
	paymentEffects PaymentEffectOwner
	references     downstreamcontract.Repository
	orders         downstreamcontract.OrderReader
	credentials    downstreamcontract.CredentialReader
	queue          downstreamcontract.CallbackQueue
	deliverer      downstreamcontract.Deliverer
	now            func() time.Time
}

// NewService 创建下游回调应用服务。
func NewService(options Options) *Service {
	if options.References == nil {
		panic("downstream callback service: references are nil")
	}
	if options.Orders == nil {
		panic("downstream callback service: orders are nil")
	}
	if options.Credentials == nil {
		panic("downstream callback service: credentials are nil")
	}
	if options.Deliverer == nil {
		panic("downstream callback service: deliverer is nil")
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	return &Service{
		paymentEffects: options.PaymentEffects,
		references:     options.References,
		orders:         options.Orders,
		credentials:    options.Credentials,
		queue:          options.Queue,
		deliverer:      options.Deliverer,
		now:            options.Now,
	}
}

// CreateRef 创建下游订单引用并强制初始化为待回调状态。
func (s *Service) CreateRef(ref *downstreamdomain.OrderRef) error {
	if ref == nil || ref.OrderID == 0 {
		return downstreamcontract.ErrInvalidRef
	}
	ref.CallbackStatus = downstreamdomain.StatusPending
	return s.references.Create(ref)
}

// GetByOrderID 根据订单 ID 查询下游引用。
func (s *Service) GetByOrderID(orderID uint) (*downstreamdomain.OrderRef, error) {
	return s.references.GetByOrderID(orderID)
}

// EnqueueCallback 在 B 侧订单状态变更后解析引用并投递回调任务。
func (s *Service) EnqueueCallback(orderID uint) {
	if err := s.EnqueueCallbackReliable(orderID); err != nil {
		logger.Warnw("downstream_enqueue_callback_failed", "order_id", orderID, "error", err)
	}
}

// EnqueueCallbackReliable exposes handoff failures to the durable payment outbox.
func (s *Service) EnqueueCallbackReliable(orderID uint) error {
	if s.paymentEffects != nil {
		if fixed, err := s.usesFixedPaymentEffects(orderID); err != nil {
			return err
		} else if fixed {
			return nil
		}
	}
	if s.queue == nil {
		return fmt.Errorf("downstream queue unavailable")
	}
	ref, err := s.references.GetByOrderID(orderID)
	if err != nil {
		return err
	}
	if ref == nil {
		order, err := s.orders.GetByID(orderID)
		if err != nil {
			return err
		}
		if order == nil || order.ParentID == nil {
			return nil
		}
		ref, err = s.references.GetByOrderID(*order.ParentID)
		if err != nil {
			return err
		}
	}
	if ref == nil || strings.TrimSpace(ref.CallbackURL) == "" {
		return nil
	}
	if ref.CallbackStatus != downstreamdomain.StatusPending {
		ref.CallbackStatus = downstreamdomain.StatusPending
		ref.CallbackRetryCount = 0
		if err := s.references.Update(ref); err != nil {
			return err
		}
	}
	return s.queue.EnqueueCallback(ref.ID, 0)
}

// SendCallback 读取最新订单状态并执行一次下游回调。
func (s *Service) SendCallback(ctx context.Context, refID uint) error {
	return s.sendCallback(ctx, refID, nil)
}

func (s *Service) sendCallback(ctx context.Context, refID uint, admit func(context.Context) error) error {
	ref, err := s.references.GetByID(refID)
	if err != nil {
		return err
	}
	if ref == nil {
		return downstreamcontract.ErrRefNotFound
	}
	if admit == nil && s.paymentEffects != nil {
		if fixed, err := s.usesFixedPaymentEffects(ref.OrderID); err != nil {
			return err
		} else if fixed {
			return nil
		}
	}
	if strings.TrimSpace(ref.CallbackURL) == "" {
		return nil
	}

	order, err := s.orders.GetByID(ref.OrderID)
	if err != nil || order == nil {
		logger.Warnw("downstream_callback_order_not_found", "ref_id", ref.ID, "order_id", ref.OrderID)
		if err == nil {
			err = fmt.Errorf("callback order not found")
		}
		return err
	}
	credential, err := s.credentials.GetByID(ref.ApiCredentialID)
	if err != nil || credential == nil {
		logger.Warnw("downstream_callback_credential_not_found", "ref_id", ref.ID, "credential_id", ref.ApiCredentialID)
		return fmt.Errorf("credential not found for ref %d", ref.ID)
	}

	event := "order.status_changed"
	if order.Status == constants.OrderStatusDelivered || order.Status == constants.OrderStatusCompleted {
		event = "order.fulfilled"
	}
	fulfillment := deliveredFulfillment(order)
	now := s.now()
	payload := downstreamcontract.CallbackPayload{
		Event:             event,
		OrderID:           order.ID,
		OrderNo:           order.OrderNo,
		DownstreamOrderNo: ref.DownstreamOrderNo,
		Status:            order.Status,
		Fulfillment:       fulfillment,
		Timestamp:         now.Unix(),
	}

	logger.Infow("downstream_callback_sending",
		"ref_id", ref.ID,
		"order_id", order.ID,
		"callback_url", ref.CallbackURL,
		"event", event,
		"status", order.Status,
		"has_fulfillment", fulfillment != nil,
	)
	if admit != nil {
		if err := admit(ctx); err != nil {
			return err
		}
	}
	if err := s.deliverer.Send(ctx, downstreamcontract.DeliveryRequest{
		URL:       ref.CallbackURL,
		APIKey:    credential.APIKey,
		APISecret: credential.APISecret,
		Payload:   payload,
	}); err != nil {
		logger.Warnw("downstream_callback_http_error", "ref_id", ref.ID, "callback_url", ref.CallbackURL, "error", err)
		if admit != nil {
			// SQL outbox owns retry. Never enqueue a second Redis retry or swallow
			// HTTP failure merely because persisting the status succeeded.
			ref.CallbackRetryCount++
			ref.CallbackStatus = downstreamdomain.StatusFailed
			ref.LastCallbackAt = &now
			return errors.Join(err, s.references.Update(ref))
		}
		return s.handleCallbackFailure(ref, now, err)
	}

	ref.CallbackStatus = downstreamdomain.StatusSent
	ref.LastCallbackAt = &now
	return s.references.Update(ref)
}

func deliveredFulfillment(order *downstreamcontract.OrderSnapshot) *downstreamcontract.Fulfillment {
	if order == nil {
		return nil
	}
	source := order.Fulfillment
	if source == nil {
		for i := range order.Children {
			if order.Children[i].Fulfillment != nil {
				source = order.Children[i].Fulfillment
				break
			}
		}
	}
	if source == nil || source.Status != constants.FulfillmentStatusDelivered {
		return nil
	}
	copy := *source
	return &copy
}

func (s *Service) handleCallbackFailure(ref *downstreamdomain.OrderRef, now time.Time, callbackErr error) error {
	ref.CallbackRetryCount++
	ref.LastCallbackAt = &now

	if ref.CallbackRetryCount >= maxCallbackRetries {
		ref.CallbackStatus = downstreamdomain.StatusFailed
		logger.Warnw("downstream_callback_max_retries",
			"ref_id", ref.ID,
			"order_id", ref.OrderID,
			"retry_count", ref.CallbackRetryCount,
			"error", callbackErr,
		)
	} else if s.queue != nil {
		index := ref.CallbackRetryCount - 1
		if index >= len(callbackRetryDelays) {
			index = len(callbackRetryDelays) - 1
		}
		if err := s.queue.EnqueueCallback(ref.ID, callbackRetryDelays[index]); err != nil {
			logger.Warnw("downstream_callback_requeue_failed", "ref_id", ref.ID, "error", err)
		}
	}

	return s.references.Update(ref)
}

type PaymentEffectOwner interface{ UsesFixedPaymentEffects(uint) (bool, error) }

// Payment owns success/delivery callbacks. Refund and cancellation transitions
// still use their existing queue until they gain their own durable effects.
func (s *Service) usesFixedPaymentEffects(orderID uint) (bool, error) {
	fixed, err := s.paymentEffects.UsesFixedPaymentEffects(orderID)
	if err != nil || !fixed {
		return fixed, err
	}
	order, err := s.orders.GetByID(orderID)
	if err != nil {
		return false, err
	}
	if order == nil {
		return false, fmt.Errorf("callback order missing")
	}
	switch order.Status {
	case constants.OrderStatusCanceled, constants.OrderStatusRefunded, constants.OrderStatusPartiallyRefunded:
		return false, nil
	default:
		return true, nil
	}
}

// SendCallbackDurable returns only after the HTTP protocol acknowledgment and
// reference write succeed. Payment owns retry/recovery through its SQL outbox.
func (s *Service) SendCallbackDurable(ctx context.Context, orderID uint, admit func(context.Context) error) error {
	if admit == nil {
		return fmt.Errorf("callback admission unavailable")
	}
	ref, err := s.references.GetByOrderID(orderID)
	if err != nil {
		return err
	}
	if ref == nil {
		order, err := s.orders.GetByID(orderID)
		if err != nil {
			return err
		}
		if order == nil {
			return fmt.Errorf("callback order missing")
		}
		if order.ParentID == nil {
			return nil
		}
		ref, err = s.references.GetByOrderID(*order.ParentID)
		if err != nil {
			return err
		}
	}
	if ref == nil || strings.TrimSpace(ref.CallbackURL) == "" {
		return nil
	}
	return s.sendCallback(ctx, ref.ID, admit)
}
