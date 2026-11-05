package contract

import "errors"

var (
	ErrNotFound           = errors.New("procurement order not found")
	ErrExists             = errors.New("procurement order already exists")
	ErrStatusInvalid      = errors.New("procurement order status invalid")
	ErrOrderNotFound      = errors.New("order not found")
	ErrConnectionNotFound = errors.New("site connection not found")
	// ErrUpstreamCancelRefused 上游拒绝取消（订单已在上游付款），需到上游申请退款
	ErrUpstreamCancelRefused = errors.New("upstream refused to cancel order")
)
