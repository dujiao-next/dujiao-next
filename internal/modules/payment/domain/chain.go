package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// ChainExpectation comes only from the merchant's persisted, validated intent.
// Gateway observations supply a lookup hint, never an amount or destination.
type ChainExpectation struct {
	Network     string
	Contract    string
	Recipient   string
	RawAmount   string
	CreatedAtMs int64
	ExpiresAtMs int64
}

// ChainProof records agreement of configured RPC sources. It is not an event
// ownership decision, cryptographic consensus proof, or permission to fulfill.
type ChainProof struct {
	Version         int              `json:"version"`
	Network         string           `json:"network"`
	TransactionHash string           `json:"transaction_hash"`
	EventIndex      uint64           `json:"event_index"`
	BlockHash       string           `json:"block_hash"`
	BlockNumber     uint64           `json:"block_number"`
	BlockTimeMs     int64            `json:"block_time_ms"`
	Contract        string           `json:"contract"`
	Recipient       string           `json:"recipient"`
	RawAmount       string           `json:"raw_amount"`
	PolicyVersion   string           `json:"policy_version"`
	Finality        []SourceFinality `json:"finality"`
	Sources         []string         `json:"sources"`
	CheckedAtMs     int64            `json:"checked_at_ms"`
}

// EventKey deliberately excludes block hash and merchant/order IDs. A reorg or
// another order cannot create another identity for the same token event.
func (p ChainProof) EventKey() string {
	b, _ := json.Marshal([]interface{}{p.Network, p.TransactionHash, p.EventIndex})
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

type ChainCheckWork struct {
	Evidence CandidateEvidence
	Receipt  CandidateReceipt
	Payments []Payment
}

// SourceFinality records the exact trusted anchor and solid/finalized head used.
type SourceFinality struct {
	Name         string `json:"name"`
	Provider     string `json:"provider"`
	AnchorNumber uint64 `json:"anchor_number"`
	AnchorHash   string `json:"anchor_hash"`
	Number       uint64 `json:"number"`
	Hash         string `json:"hash"`
	TimeMs       int64  `json:"time_ms"`
}
