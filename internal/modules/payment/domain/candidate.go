package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"time"
)

// CandidateSource identifies an inbox independently of channel IDs and secrets.
// Endpoint or PID changes require draining the previous inbox before removal.
type CandidateSource struct {
	GatewayURL string
	PID        string
}

func (s CandidateSource) Key() string {
	b, _ := json.Marshal([]string{s.GatewayURL, s.PID})
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// TransferCandidate is untrusted discovery data, never proof of payment.
type TransferCandidate struct {
	ID              string  `json:"id"`
	ObservationKey  string  `json:"observation_key"`
	TradeID         string  `json:"trade_id"`
	OrderID         string  `json:"order_id"`
	Network         string  `json:"network"`
	Kind            string  `json:"kind"`
	TransactionHash string  `json:"transaction_hash"`
	EventIndex      *uint64 `json:"event_index"`
	BlockHash       string  `json:"block_hash"`
	BlockNumber     int64   `json:"block_number"`
	BlockTimeMs     int64   `json:"block_time_ms"`
	Contract        string  `json:"contract"`
	Recipient       string  `json:"recipient"`
	RawAmount       string  `json:"raw_amount"`
	Removed         bool    `json:"removed"`
	ReviewReason    string  `json:"review_reason"`
	ObservedAtMs    int64   `json:"observed_at_ms"`
}

// CandidateReceipt is durable before ACK. Conflict is sticky; ID reuse must not
// replace evidence. There is deliberately no paid/verified flag or soft delete.
type CandidateReceipt struct {
	ID          uint   `gorm:"primaryKey"`
	SourceKey   string `gorm:"size:64;not null;uniqueIndex:candidate_receipt_source"`
	CandidateID string `gorm:"size:20;not null;uniqueIndex:candidate_receipt_source"`
	FirstDigest string `gorm:"size:64;not null"`
	Conflicted  bool   `gorm:"not null;default:false"`
	ReceivedAt  time.Time
}

func (CandidateReceipt) TableName() string { return "payment_candidate_receipts" }

// CandidateEvidence retains every distinct version under a source/ID. PaymentID
// is only a local identity hint, not an amount/address/receipt verification.
type CandidateEvidence struct {
	ID            uint   `gorm:"primaryKey"`
	ReceiptID     uint   `gorm:"not null;uniqueIndex:candidate_evidence_version"`
	Digest        string `gorm:"size:64;not null;uniqueIndex:candidate_evidence_version"`
	Payload       string `gorm:"type:text;not null"`
	PaymentID     *uint  `gorm:"index"`
	MatchState    string `gorm:"size:32;not null"`
	ReceivedAt    time.Time
	CheckState    string     `gorm:"size:32;not null;default:pending;index"`
	CheckCode     string     `gorm:"size:64"`
	CheckAfter    *time.Time `gorm:"index"`
	CheckToken    string     `gorm:"size:36"`
	CheckAttempts int        `gorm:"not null;default:0"`
	ChainProof    string     `gorm:"type:text"`
	EventKey      string     `gorm:"size:64;index"`
}

func (CandidateEvidence) TableName() string { return "payment_candidate_evidence" }

// CandidateSubscription retains historical delivery credentials after a channel
// is disabled, renamed or deleted. Private data: never serialize to a browser.
type CandidateSubscription struct {
	SourceKey      string       `gorm:"primaryKey;size:64" json:"-"`
	ChannelType    string       `gorm:"size:32" json:"-"`
	ConfigJSON     jsonmap.JSON `gorm:"type:json" json:"-"`
	CreatedAt      time.Time
	LastAttemptAt  *time.Time
	LastSuccessAt  *time.Time
	ScannerHealthy bool `gorm:"not null;default:false"`
}

func (CandidateSubscription) TableName() string { return "payment_candidate_subscriptions" }
