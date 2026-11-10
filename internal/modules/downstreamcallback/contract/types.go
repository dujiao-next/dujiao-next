package contract

import (
	"time"

	"github.com/dujiao-next/internal/shared/jsonmap"
)

// RefListFilter 描述凭证维度的回调引用分页筛选。
type RefListFilter struct {
	CallbackStatus string
	Page           int
	PageSize       int
}

// Fulfillment 是回调协议需要的交付快照。
type Fulfillment struct {
	Type         string       `json:"type"`
	Status       string       `json:"status"`
	Payload      string       `json:"payload"`
	DeliveryData jsonmap.JSON `json:"delivery_data"`
	DeliveredAt  *time.Time   `json:"delivered_at,omitempty"`
}

// OrderSnapshot 是回调应用读取订单所需的最小投影。
type OrderSnapshot struct {
	ID          uint
	OrderNo     string
	ParentID    *uint
	Status      string
	Fulfillment *Fulfillment
	Children    []OrderSnapshot
}

// Credential 是签名回调所需的最小凭证投影。
type Credential struct {
	ID        uint
	APIKey    string
	APISecret string
	// Active 与 API 鉴权同一标准：已审批、已启用且所属用户未禁用。
	Active bool
}

// CallbackPayload 是发送给下游站点的稳定 JSON 合同。
type CallbackPayload struct {
	Event             string       `json:"event"`
	OrderID           uint         `json:"order_id"`
	OrderNo           string       `json:"order_no"`
	DownstreamOrderNo string       `json:"downstream_order_no"`
	Status            string       `json:"status"`
	Fulfillment       *Fulfillment `json:"fulfillment,omitempty"`
	Timestamp         int64        `json:"timestamp"`
}

// DeliveryRequest 包含完成一次签名 HTTP 回调所需的数据。
type DeliveryRequest struct {
	URL       string
	APIKey    string
	APISecret string
	Payload   CallbackPayload
}
