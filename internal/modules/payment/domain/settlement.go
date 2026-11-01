package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

var ErrAutomationFrozen = errors.New("payment automation frozen or generation changed")

// AutomationControl is a singleton SQL fence, closed until operator acceptance.
// It must be backed up with payment/fulfillment history, never kept only in Redis.
type AutomationControl struct {
	ID        uint   `gorm:"primaryKey;autoIncrement:false"`
	Frozen    bool   `gorm:"not null;default:true"`
	Revision  uint64 `gorm:"not null;default:1"`
	Reason    string `gorm:"size:160;not null"`
	UpdatedAt time.Time
}

func (AutomationControl) TableName() string { return "payment_automation_control" }

// Allocation is an append-only merchant-owned address/amount reservation. No
// expiry or soft delete releases a slot. Restoring it requires coordinated replay.
type Allocation struct {
	ID        uint   `gorm:"primaryKey"`
	SlotKey   string `gorm:"size:64;not null;uniqueIndex"`
	PaymentID uint   `gorm:"not null;uniqueIndex"`
	OrderID   uint   `gorm:"not null;index"`
	Network   string `gorm:"size:16;not null"`
	Contract  string `gorm:"size:64;not null"`
	Recipient string `gorm:"size:64;not null"`
	RawAmount string `gorm:"size:80;not null"`
	Snapshot  string `gorm:"size:64;not null"`
	CreatedAt time.Time
}

func (Allocation) TableName() string { return "payment_allocations" }

type EventConsumption struct {
	ID           uint   `gorm:"primaryKey"`
	EventKey     string `gorm:"size:64;not null;uniqueIndex"`
	PaymentID    uint   `gorm:"not null;uniqueIndex"`
	OrderID      uint   `gorm:"not null;uniqueIndex"`
	EvidenceID   uint   `gorm:"not null;uniqueIndex"`
	AllocationID uint   `gorm:"not null;uniqueIndex"`
	Proof        string `gorm:"type:text;not null"`
	CreatedAt    time.Time
}

func (EventConsumption) TableName() string { return "payment_event_consumptions" }

// PaymentEffect is a durable, independently retryable post-settlement task.
// Financial effects belong to the settlement transaction, never this dispatcher.
type PaymentEffect struct {
	RecipientPlan      string     `gorm:"size:64"` // Frozen destination-set digest; no addresses or content.
	DispatchedAt       *time.Time // Last durable admission; not proof of external delivery.
	AutomationRevision uint64     `gorm:"-" json:"-"`
	ID                 uint       `gorm:"primaryKey"`
	PaymentID          uint       `gorm:"not null;uniqueIndex:payment_effect_identity"`
	OrderID            uint       `gorm:"not null;uniqueIndex:payment_effect_identity"`
	Kind               string     `gorm:"size:32;not null;uniqueIndex:payment_effect_identity"`
	State              string     `gorm:"size:16;not null;index"`
	Token              string     `gorm:"size:36"`
	After              *time.Time `gorm:"index"`
	Attempts           int        `gorm:"not null;default:0"`
	Code               string     `gorm:"size:64"`
	CreatedAt          time.Time
	CompletedAt        *time.Time
}

func (PaymentEffect) TableName() string { return "payment_effects" }

func digest(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func (e ChainExpectation) SlotKey() string {
	return digest([]string{e.Network, e.Contract, e.Recipient, e.RawAmount})
}

// OrderBasis freezes the merchant's economic and delivery obligations before
// sending a create request. Timestamps/statuses are checked separately.
func OrderBasis(p Payment, o orderdomain.Order) string {
	orders := append([]orderdomain.Order{o}, o.Children...)
	sort.Slice(orders, func(i, j int) bool { return orders[i].ID < orders[j].ID })
	rows := make([]interface{}, 0, len(orders))
	for _, order := range orders {
		items := append([]orderdomain.OrderItem(nil), order.Items...)
		sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
		for i := range items {
			// Database timestamps are bookkeeping, not payment obligations.
			// PostgreSQL rounds them to microseconds; transient display labels and
			// nil/empty JSON must not change authorization after a SQL round trip.
			items[i].CreatedAt, items[i].UpdatedAt = time.Time{}, time.Time{}
			items[i].PromotionName = ""
			for _, object := range []*jsonmap.JSON{&items[i].TitleJSON, &items[i].SKUSnapshotJSON, &items[i].ManualFormSchemaSnapshotJSON, &items[i].ManualFormSubmissionJSON, &items[i].InstructionsJSON} {
				if len(*object) == 0 {
					*object = nil
				}
			}
			if len(items[i].Tags) == 0 {
				items[i].Tags = nil
			}
		}
		rows = append(rows, []interface{}{order.ID, order.ParentID, order.UserID, order.OrderNo, order.Currency, order.TotalAmount.String(), order.WalletPaidAmount.String(), order.ResellerID, order.ResellerProfitAmount.String(), order.AffiliateProfileID, order.AffiliateCode, items})
	}
	return digest([]interface{}{p.ID, p.OrderID, p.ChannelID, p.ProviderType, p.ChannelType, p.Amount.String(), p.Currency, p.FeePolicy, p.FeeAmount.String(), p.FeeRate.String(), p.FixedFee.String(), rows})
}

func SettlementSnapshot(p Payment, o orderdomain.Order) string {
	return digest([]interface{}{OrderBasis(p, o), p.SettlementBasis, p.PaymentIntent, p.GatewayOrderNo, p.ProviderRef})
}

// OperatorAudit is an append-only operational trail, not a payment override.
type OperatorAudit struct {
	ID               uint   `gorm:"primaryKey"`
	Operator         string `gorm:"size:120;not null"`
	Action           string `gorm:"size:24;not null"`
	PreviousRevision uint64
	Revision         uint64
	PaymentID        uint
	Reason           string `gorm:"size:160;not null"`
	CreatedAt        time.Time
}

func (OperatorAudit) TableName() string { return "payment_operator_audit" }
