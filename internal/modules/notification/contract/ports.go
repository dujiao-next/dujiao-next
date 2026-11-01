package contract

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	dashboardcontract "github.com/dujiao-next/internal/modules/dashboard/contract"
	"github.com/dujiao-next/internal/modules/notification/domain"
	settingsmessaging "github.com/dujiao-next/internal/modules/settings/schema/messaging"
	settingsstorefront "github.com/dujiao-next/internal/modules/settings/schema/storefront"
	"github.com/dujiao-next/internal/queue"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/mailbrand"
	"github.com/dujiao-next/internal/shared/money"
)

type SettingsReader interface {
	GetNotificationCenterSetting() (settingsmessaging.NotificationCenterSetting, error)
	GetDashboardSetting() (settingsstorefront.DashboardSetting, error)
}

type EmailSender interface {
	SendCustomEmail(toEmail, subject, body string) error
}

// OrderStatusEmailInput carries the order facts required to render an email.
type OrderStatusEmailInput struct {
	OrderNo           string
	Status            string
	Amount            money.Amount
	RefundAmount      money.Amount
	RefundReason      string
	Currency          string
	SiteName          string
	SiteURL           string
	FulfillmentInfo   string
	Instructions      string
	IsGuest           bool
	AttachmentName    string
	AttachmentContent string
	MailBrand         mailbrand.Brand
}

type DispatchQueue interface {
	EnqueueNotificationDispatch(payload queue.NotificationDispatchPayload, maxRetry int) error
}

type DashboardAlertReader interface {
	LoadDashboardAlertSetting() settingsstorefront.DashboardAlertSetting
	GetInventoryAlertItems(ctx context.Context, lowStockThreshold int64) ([]dashboardcontract.InventoryAlertRow, error)
	GetPaymentOrderAlertCounts(ctx context.Context, startAt, endAt time.Time) (dashboardcontract.PaymentOrderAlertCountsRow, error)
}

type TelegramSender interface {
	SendMessage(ctx context.Context, chatID, message string) error
}

type FeishuSender interface {
	SendMessage(ctx context.Context, appID, appSecret, receiveIDType, receiveID, message string) error
}

type LogRepository interface {
	Create(log *domain.NotificationLog) error
	ListAdmin(filter LogListFilter) ([]domain.NotificationLog, int64, error)
}

type LogListFilter struct {
	Page        int
	PageSize    int
	Channel     string
	Status      string
	EventType   string
	IsTest      *bool
	CreatedFrom *time.Time
	CreatedTo   *time.Time
}

// EnqueueInput 描述一个待投递的业务通知事件。
type EnqueueInput struct {
	EventType string
	BizType   string
	BizID     uint
	Locale    string
	Force     bool
	Data      jsonmap.JSON
}

type NotificationEnqueuer interface {
	Enqueue(input EnqueueInput) error
}

// TestSendInput 描述后台通知中心的一次测试发送。
type TestSendInput struct {
	Channel   string
	Target    string
	Scene     string
	Locale    string
	Variables map[string]interface{}
}

type TestSender interface {
	SendTest(context.Context, TestSendInput) error
}

// RecipientDelivery identifies a destination without persisting its address or
// message content in the payment receipt ledger. Send must return actual IO status.
type RecipientDelivery struct {
	Key  string
	Send func(context.Context) error
}
type DeliverBatch func(context.Context, []RecipientDelivery) error

func RecipientKey(parts ...string) string {
	b, _ := json.Marshal(parts)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
