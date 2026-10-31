package application

import (
	"context"
	"strings"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/notification/application/format"
	"github.com/dujiao-next/internal/modules/notification/contract"
	"github.com/dujiao-next/internal/queue"
)

// Service 通知中心服务。
type Service struct {
	deliveryBatch  contract.DeliverBatch
	settingService contract.SettingsReader
	emailService   contract.EmailSender
	queueClient    contract.DispatchQueue
	dashboardSvc   contract.DashboardAlertReader
	logService     *LogService
	telegramSender contract.TelegramSender
	feishuSender   contract.FeishuSender
}

// NewService 创建通知中心服务。
func NewService(
	settingService contract.SettingsReader,
	emailService contract.EmailSender,
	queueClient contract.DispatchQueue,
	dashboardSvc contract.DashboardAlertReader,
	logService *LogService,
	telegramSender contract.TelegramSender,
	feishuSender contract.FeishuSender,
) *Service {
	return &Service{
		settingService: settingService,
		emailService:   emailService,
		queueClient:    queueClient,
		dashboardSvc:   dashboardSvc,
		logService:     logService,
		telegramSender: telegramSender,
		feishuSender:   feishuSender,
	}
}

// Enqueue 入队通知任务
func (s *Service) Enqueue(input contract.EnqueueInput) error {
	eventType := strings.ToLower(strings.TrimSpace(input.EventType))
	if !isNotificationEventSupported(eventType) {
		return contract.ErrEventInvalid
	}
	if s == nil || s.queueClient == nil {
		return nil
	}

	payload := queue.NotificationDispatchPayload{
		EventType: eventType,
		BizType:   strings.TrimSpace(input.BizType),
		BizID:     input.BizID,
		Locale:    strings.TrimSpace(input.Locale),
		Force:     input.Force,
		Data:      format.JSONToMap(input.Data),
	}
	return s.queueClient.EnqueueNotificationDispatch(payload, 5)
}

// Dispatch 处理通知分发任务
func (s *Service) Dispatch(ctx context.Context, payload queue.NotificationDispatchPayload) error {
	if s == nil {
		return nil
	}
	eventType := strings.ToLower(strings.TrimSpace(payload.EventType))
	if !isNotificationEventSupported(eventType) {
		return contract.ErrEventInvalid
	}

	setting, err := s.settingService.GetNotificationCenterSetting()
	if err != nil {
		return err
	}
	if !setting.Scenes.IsSceneEnabled(eventType) {
		return nil
	}

	if eventType == constants.NotificationEventExceptionAlertCheck {
		return s.dispatchExceptionAlertCheck(ctx, setting, payload)
	}
	return s.dispatchSingleEvent(ctx, setting, payload)
}

// DispatchDurable is owned by the payment SQL outbox. Redis dedupe taken before
// a send is not an acknowledgment and must never suppress a failed retry here.
func (s *Service) DispatchDurable(ctx context.Context, input contract.EnqueueInput, admit func(context.Context) error) error {
	if s == nil || s.settingService == nil || admit == nil {
		return contract.ErrSendFailed
	}
	if input.EventType != constants.NotificationEventOrderPaidSuccess && input.EventType != constants.NotificationEventManualFulfillmentPending {
		return contract.ErrEventInvalid
	}
	setting, err := s.settingService.GetNotificationCenterSetting()
	if err != nil {
		return err
	}
	if !setting.Scenes.IsSceneEnabled(input.EventType) {
		if s.deliveryBatch != nil {
			return s.deliveryBatch(ctx, nil)
		}
		return nil
	}
	payload := queue.NotificationDispatchPayload{EventType: input.EventType, BizType: input.BizType, BizID: input.BizID, Locale: input.Locale, Force: true, Data: format.JSONToMap(input.Data)}
	return s.dispatchSingleEvent(ctx, setting, payload, admit)
}

// DispatchRecipients delegates the full destination set before any outbound IO.
func (s *Service) DispatchRecipients(ctx context.Context, input contract.EnqueueInput, batch contract.DeliverBatch) error {
	if s == nil || batch == nil {
		return contract.ErrSendFailed
	}
	copy := *s
	copy.deliveryBatch = batch
	return copy.DispatchDurable(ctx, input, func(context.Context) error { return nil })
}
