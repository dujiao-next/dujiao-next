package application

import (
	"sort"
	"strings"
	"time"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	"github.com/dujiao-next/internal/constants"
	ordercontract "github.com/dujiao-next/internal/modules/order/contract"
	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	"github.com/dujiao-next/internal/shared/jsonmap"

	"github.com/shopspring/decimal"
)

// BuildLocalRefundRecordsForOrder 构建订单关联的本地退款记录列表。
// 仅返回本地 order_refund_records 数据，不透传更上游退款记录。
func (s *OrderService) BuildLocalRefundRecordsForOrder(order *orderdomain.Order) ([]jsonmap.JSON, error) {
	recordsJSON := make([]jsonmap.JSON, 0)
	if order == nil || s.orderStore == nil {
		return recordsJSON, nil
	}

	idSet := make(map[uint]struct{}, 4)
	idSet[order.ID] = struct{}{}
	if order.ParentID != nil && *order.ParentID > 0 {
		idSet[*order.ParentID] = struct{}{}
	}
	for i := range order.Children {
		if order.Children[i].ID > 0 {
			idSet[order.Children[i].ID] = struct{}{}
		}
	}

	orderIDs := make([]uint, 0, len(idSet))
	for id := range idSet {
		orderIDs = append(orderIDs, id)
	}

	records, err := s.orderStore.ListRefundRecordsByOrderIDs(orderIDs)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return recordsJSON, nil
	}

	sort.SliceStable(records, func(i, j int) bool {
		if records[i].CreatedAt.Equal(records[j].CreatedAt) {
			return records[i].ID < records[j].ID
		}
		return records[i].CreatedAt.Before(records[j].CreatedAt)
	})

	recordsJSON = make([]jsonmap.JSON, 0, len(records))
	for idx, record := range records {
		recordsJSON = append(recordsJSON, jsonmap.JSON{
			// 不暴露内部退款主键，统一返回列表序号。
			"id":          idx + 1,
			"user_id":     record.UserID,
			"guest_email": record.GuestEmail,
			"order_id":    record.OrderID,
			"type":        record.Type,
			"amount":      record.Amount,
			"currency":    record.Currency,
			"remark":      record.Remark,
			"created_at":  record.CreatedAt,
			"updated_at":  record.UpdatedAt,
		})
	}
	return recordsJSON, nil
}

// ensureOrderCanceledIfExpired 读取时懒同步过期订单状态
func (s *OrderService) ensureOrderCanceledIfExpired(order *orderdomain.Order) error {
	if order == nil {
		return nil
	}
	if order.Status != constants.OrderStatusPendingPayment {
		return nil
	}
	if order.ExpiresAt == nil {
		return nil
	}
	if order.ExpiresAt.After(time.Now()) {
		return nil
	}
	if err := s.cancelOrderWithChildren(order, true); err != nil {
		return err
	}
	return nil
}

// ensureOrdersCanceledIfExpired 批量懒同步过期订单状态
func (s *OrderService) ensureOrdersCanceledIfExpired(orders []orderdomain.Order) error {
	if len(orders) == 0 {
		return nil
	}
	for i := range orders {
		if err := s.ensureOrderCanceledIfExpired(&orders[i]); err != nil {
			return err
		}
	}
	return nil
}

// expectedRefundStatus 根据订单总额与已退款金额计算应处于的退款状态。
func expectedRefundStatus(order *orderdomain.Order) string {
	if order == nil {
		return ""
	}
	if strings.ToLower(strings.TrimSpace(order.Status)) == constants.OrderStatusCanceled {
		return ""
	}
	if order.PaidAt == nil {
		return ""
	}
	total := order.TotalAmount.Decimal.Round(2)
	if total.LessThanOrEqual(decimal.Zero) {
		return ""
	}
	refunded := order.RefundedAmount.Decimal.Round(2)
	if refunded.LessThanOrEqual(decimal.Zero) {
		return ""
	}
	if refunded.GreaterThanOrEqual(total) {
		return constants.OrderStatusRefunded
	}
	return constants.OrderStatusPartiallyRefunded
}

// resolvedParentStatus 计算父订单当前应同步的状态（优先退款状态）。
func resolvedParentStatus(order *orderdomain.Order) string {
	if order == nil {
		return ""
	}
	if refundStatus := expectedRefundStatus(order); refundStatus != "" {
		return refundStatus
	}
	return CalcParentStatus(order.Children, order.Status)
}

// ensureSingleOrderRefundStatusSynced 懒同步单条订单退款状态到最新值。
func (s *OrderService) ensureSingleOrderRefundStatusSynced(order *orderdomain.Order, now time.Time) (bool, error) {
	target := expectedRefundStatus(order)
	if target == "" || strings.EqualFold(strings.TrimSpace(order.Status), target) {
		return false, nil
	}
	if err := s.orderStore.UpdateStatus(order.ID, target, map[string]interface{}{
		"updated_at": now,
	}); err != nil {
		return false, err
	}
	order.Status = target
	order.UpdatedAt = now
	return true, nil
}

// ensureOrderRefundStatusSynced 读取时懒同步退款相关状态
func (s *OrderService) ensureOrderRefundStatusSynced(order *orderdomain.Order) error {
	if order == nil {
		return nil
	}

	now := time.Now()
	orderChanged, err := s.ensureSingleOrderRefundStatusSynced(order, now)
	if err != nil {
		return err
	}

	for i := range order.Children {
		changed, childErr := s.ensureSingleOrderRefundStatusSynced(&order.Children[i], now)
		if childErr != nil {
			return childErr
		}
		if changed {
			orderChanged = true
		}
	}

	if len(order.Children) > 0 {
		parentStatus := resolvedParentStatus(order)
		if !strings.EqualFold(strings.TrimSpace(order.Status), parentStatus) {
			if err := s.orderStore.UpdateStatus(order.ID, parentStatus, map[string]interface{}{
				"updated_at": now,
			}); err != nil {
				return err
			}
			order.Status = parentStatus
			order.UpdatedAt = now
		}
		return nil
	}

	if order.ParentID != nil && orderChanged {
		if _, err := SyncParentStatus(s.orderStore, *order.ParentID, now); err != nil {
			return err
		}
	}
	return nil
}

// ensureOrdersRefundStatusSynced 批量懒同步退款相关状态
func (s *OrderService) ensureOrdersRefundStatusSynced(orders []orderdomain.Order) error {
	if len(orders) == 0 {
		return nil
	}
	for i := range orders {
		if err := s.ensureOrderRefundStatusSynced(&orders[i]); err != nil {
			return err
		}
	}
	return nil
}

// GetOrderByUser 获取订单详情
func (s *OrderService) GetOrderByUser(orderID uint, userID uint) (*orderdomain.Order, error) {
	order, err := s.orderStore.GetByIDAndUser(orderID, userID)
	if err != nil {
		return nil, ErrOrderFetchFailed
	}
	if order == nil {
		return nil, ErrOrderNotFound
	}
	if err := s.ensureOrderCanceledIfExpired(order); err != nil {
		return nil, ErrOrderUpdateFailed
	}
	if err := s.ensureOrderRefundStatusSynced(order); err != nil {
		return nil, ErrOrderUpdateFailed
	}
	FillOrderItemsFromChildren(order)
	return order, nil
}

// GetOrderByUserForTenant 获取当前租户上下文中的用户订单详情。
func (s *OrderService) GetOrderByUserForTenant(tenant resellercontract.TenantContext, orderID uint, userID uint) (*orderdomain.Order, error) {
	order, err := s.orderStore.GetByIDAndUserScoped(orderID, userID, orderScopeFromTenant(tenant))
	if err != nil {
		return nil, ErrOrderFetchFailed
	}
	if order == nil {
		return nil, ErrOrderNotFound
	}
	if err := s.ensureOrderCanceledIfExpired(order); err != nil {
		return nil, ErrOrderUpdateFailed
	}
	if err := s.ensureOrderRefundStatusSynced(order); err != nil {
		return nil, ErrOrderUpdateFailed
	}
	FillOrderItemsFromChildren(order)
	return order, nil
}

// GetOrderByUserOrderNo 按订单号获取用户订单详情
func (s *OrderService) GetOrderByUserOrderNo(orderNo string, userID uint) (*orderdomain.Order, error) {
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" {
		return nil, ErrOrderNotFound
	}
	order, err := s.orderStore.GetByOrderNoAndUser(orderNo, userID)
	if err != nil {
		return nil, ErrOrderFetchFailed
	}
	if order == nil {
		return nil, ErrOrderNotFound
	}
	if err := s.ensureOrderCanceledIfExpired(order); err != nil {
		return nil, ErrOrderUpdateFailed
	}
	if err := s.ensureOrderRefundStatusSynced(order); err != nil {
		return nil, ErrOrderUpdateFailed
	}
	FillOrderItemsFromChildren(order)
	return order, nil
}

// GetOrderByUserOrderNoForTenant 按订单号获取当前租户上下文中的用户订单详情。
func (s *OrderService) GetOrderByUserOrderNoForTenant(tenant resellercontract.TenantContext, orderNo string, userID uint) (*orderdomain.Order, error) {
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" {
		return nil, ErrOrderNotFound
	}
	order, err := s.orderStore.GetByOrderNoAndUserScoped(orderNo, userID, orderScopeFromTenant(tenant))
	if err != nil {
		return nil, ErrOrderFetchFailed
	}
	if order == nil {
		return nil, ErrOrderNotFound
	}
	if err := s.ensureOrderCanceledIfExpired(order); err != nil {
		return nil, ErrOrderUpdateFailed
	}
	if err := s.ensureOrderRefundStatusSynced(order); err != nil {
		return nil, ErrOrderUpdateFailed
	}
	FillOrderItemsFromChildren(order)
	return order, nil
}

// GetAnyOrderByUserOrderNoForTenant 支持父订单或子订单号获取当前租户上下文中的用户订单。
func (s *OrderService) GetAnyOrderByUserOrderNoForTenant(tenant resellercontract.TenantContext, orderNo string, userID uint) (*orderdomain.Order, error) {
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" {
		return nil, ErrOrderNotFound
	}
	order, err := s.orderStore.GetAnyByOrderNoAndUserScoped(orderNo, userID, orderScopeFromTenant(tenant))
	if err != nil {
		return nil, ErrOrderFetchFailed
	}
	if order == nil {
		return nil, ErrOrderNotFound
	}
	if err := s.ensureOrderCanceledIfExpired(order); err != nil {
		return nil, ErrOrderUpdateFailed
	}
	if err := s.ensureOrderRefundStatusSynced(order); err != nil {
		return nil, ErrOrderUpdateFailed
	}
	FillOrderItemsFromChildren(order)
	return order, nil
}

// GetOrderByGuest 获取游客订单详情
func (s *OrderService) GetOrderByGuest(orderID uint, email, password string) (*orderdomain.Order, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	order, err := s.orderStore.GetByIDAndGuest(orderID, email, password)
	if err != nil {
		return nil, ErrOrderFetchFailed
	}
	if order == nil {
		return nil, ErrGuestOrderNotFound
	}
	if err := s.ensureOrderCanceledIfExpired(order); err != nil {
		return nil, ErrOrderUpdateFailed
	}
	if err := s.ensureOrderRefundStatusSynced(order); err != nil {
		return nil, ErrOrderUpdateFailed
	}
	FillOrderItemsFromChildren(order)
	return order, nil
}

// GetOrderByGuestForTenant 获取当前租户上下文中的游客订单详情。
func (s *OrderService) GetOrderByGuestForTenant(tenant resellercontract.TenantContext, orderID uint, email, password string) (*orderdomain.Order, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	order, err := s.orderStore.GetByIDAndGuestScoped(orderID, email, password, orderScopeFromTenant(tenant))
	if err != nil {
		return nil, ErrOrderFetchFailed
	}
	if order == nil {
		return nil, ErrGuestOrderNotFound
	}
	if err := s.ensureOrderCanceledIfExpired(order); err != nil {
		return nil, ErrOrderUpdateFailed
	}
	if err := s.ensureOrderRefundStatusSynced(order); err != nil {
		return nil, ErrOrderUpdateFailed
	}
	FillOrderItemsFromChildren(order)
	return order, nil
}

// GetOrderByGuestOrderNo 获取游客订单详情（按订单号）
func (s *OrderService) GetOrderByGuestOrderNo(orderNo, email, password string) (*orderdomain.Order, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	order, err := s.orderStore.GetByOrderNoAndGuest(orderNo, email, password)
	if err != nil {
		return nil, ErrOrderFetchFailed
	}
	if order == nil {
		return nil, ErrGuestOrderNotFound
	}
	if err := s.ensureOrderCanceledIfExpired(order); err != nil {
		return nil, ErrOrderUpdateFailed
	}
	if err := s.ensureOrderRefundStatusSynced(order); err != nil {
		return nil, ErrOrderUpdateFailed
	}
	FillOrderItemsFromChildren(order)
	return order, nil
}

// GetOrderByGuestOrderNoForTenant 获取当前租户上下文中的游客订单详情（按订单号）。
func (s *OrderService) GetOrderByGuestOrderNoForTenant(tenant resellercontract.TenantContext, orderNo, email, password string) (*orderdomain.Order, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	order, err := s.orderStore.GetByOrderNoAndGuestScoped(orderNo, email, password, orderScopeFromTenant(tenant))
	if err != nil {
		return nil, ErrOrderFetchFailed
	}
	if order == nil {
		return nil, ErrGuestOrderNotFound
	}
	if err := s.ensureOrderCanceledIfExpired(order); err != nil {
		return nil, ErrOrderUpdateFailed
	}
	if err := s.ensureOrderRefundStatusSynced(order); err != nil {
		return nil, ErrOrderUpdateFailed
	}
	FillOrderItemsFromChildren(order)
	return order, nil
}

// GetAnyOrderByGuestOrderNoForTenant 支持父订单或子订单号获取当前租户上下文中的游客订单。
func (s *OrderService) GetAnyOrderByGuestOrderNoForTenant(tenant resellercontract.TenantContext, orderNo, email, password string) (*orderdomain.Order, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	order, err := s.orderStore.GetAnyByOrderNoAndGuestScoped(orderNo, email, password, orderScopeFromTenant(tenant))
	if err != nil {
		return nil, ErrOrderFetchFailed
	}
	if order == nil {
		return nil, ErrGuestOrderNotFound
	}
	if err := s.ensureOrderCanceledIfExpired(order); err != nil {
		return nil, ErrOrderUpdateFailed
	}
	if err := s.ensureOrderRefundStatusSynced(order); err != nil {
		return nil, ErrOrderUpdateFailed
	}
	FillOrderItemsFromChildren(order)
	return order, nil
}

// ListOrdersByUser 获取订单列表
func (s *OrderService) ListOrdersByUser(filter ordercontract.ListFilter) ([]orderdomain.Order, int64, error) {
	if filter.UserID == 0 {
		return nil, 0, ErrOrderFetchFailed
	}
	orders, total, err := s.orderStore.ListByUser(filter)
	if err != nil {
		return nil, 0, ErrOrderFetchFailed
	}
	if err := s.ensureOrdersCanceledIfExpired(orders); err != nil {
		return nil, 0, ErrOrderUpdateFailed
	}
	if err := s.ensureOrdersRefundStatusSynced(orders); err != nil {
		return nil, 0, ErrOrderUpdateFailed
	}
	FillOrdersItemsFromChildren(orders)
	return orders, total, nil
}

// ListOrdersByUserForTenant 获取当前租户上下文中的用户订单列表。
func (s *OrderService) ListOrdersByUserForTenant(tenant resellercontract.TenantContext, filter ordercontract.ListFilter) ([]orderdomain.Order, int64, error) {
	if filter.UserID == 0 {
		return nil, 0, ErrOrderFetchFailed
	}
	orders, total, err := s.orderStore.ListByUserScoped(filter, orderScopeFromTenant(tenant))
	if err != nil {
		return nil, 0, ErrOrderFetchFailed
	}
	if err := s.ensureOrdersCanceledIfExpired(orders); err != nil {
		return nil, 0, ErrOrderUpdateFailed
	}
	if err := s.ensureOrdersRefundStatusSynced(orders); err != nil {
		return nil, 0, ErrOrderUpdateFailed
	}
	FillOrdersItemsFromChildren(orders)
	return orders, total, nil
}

// StatsOrdersByUser 按状态聚合用户订单数量（基于全量数据，仅复用关键词筛选）
func (s *OrderService) StatsOrdersByUser(filter ordercontract.ListFilter) (map[string]int64, error) {
	if filter.UserID == 0 {
		return nil, ErrOrderFetchFailed
	}
	stats, err := s.orderStore.StatsByUser(filter)
	if err != nil {
		return nil, ErrOrderFetchFailed
	}
	return stats, nil
}

// StatsOrdersByUserForTenant 按状态聚合当前租户上下文中的用户订单数量。
func (s *OrderService) StatsOrdersByUserForTenant(tenant resellercontract.TenantContext, filter ordercontract.ListFilter) (map[string]int64, error) {
	if filter.UserID == 0 {
		return nil, ErrOrderFetchFailed
	}
	stats, err := s.orderStore.StatsByUserScoped(filter, orderScopeFromTenant(tenant))
	if err != nil {
		return nil, ErrOrderFetchFailed
	}
	return stats, nil
}

// ListOrdersByGuest 获取游客订单列表
func (s *OrderService) ListOrdersByGuest(email, password string, page, pageSize int) ([]orderdomain.Order, int64, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	orders, total, err := s.orderStore.ListByGuest(email, password, page, pageSize)
	if err != nil {
		return nil, 0, ErrOrderFetchFailed
	}
	if err := s.ensureOrdersCanceledIfExpired(orders); err != nil {
		return nil, 0, ErrOrderUpdateFailed
	}
	if err := s.ensureOrdersRefundStatusSynced(orders); err != nil {
		return nil, 0, ErrOrderUpdateFailed
	}
	FillOrdersItemsFromChildren(orders)
	return orders, total, nil
}

// ListOrdersByGuestForTenant 获取当前租户上下文中的游客订单列表。
func (s *OrderService) ListOrdersByGuestForTenant(tenant resellercontract.TenantContext, email, password string, page, pageSize int) ([]orderdomain.Order, int64, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	orders, total, err := s.orderStore.ListByGuestScoped(email, password, page, pageSize, orderScopeFromTenant(tenant))
	if err != nil {
		return nil, 0, ErrOrderFetchFailed
	}
	if err := s.ensureOrdersCanceledIfExpired(orders); err != nil {
		return nil, 0, ErrOrderUpdateFailed
	}
	if err := s.ensureOrdersRefundStatusSynced(orders); err != nil {
		return nil, 0, ErrOrderUpdateFailed
	}
	FillOrdersItemsFromChildren(orders)
	return orders, total, nil
}

// ListOrdersByBrowserTokenForTenant lists guest summaries bound to one browser token.
func (s *OrderService) ListOrdersByBrowserTokenForTenant(tenant resellercontract.TenantContext, tokenHash string, page, pageSize int) ([]orderdomain.Order, int64, error) {
	orders, total, err := s.orderStore.ListByBrowserTokenScoped(tokenHash, page, pageSize, orderScopeFromTenant(tenant))
	if err != nil {
		return nil, 0, ErrOrderFetchFailed
	}
	if err := s.ensureOrdersCanceledIfExpired(orders); err != nil {
		return nil, 0, ErrOrderUpdateFailed
	}
	if err := s.ensureOrdersRefundStatusSynced(orders); err != nil {
		return nil, 0, ErrOrderUpdateFailed
	}
	FillOrdersItemsFromChildren(orders)
	return orders, total, nil
}

func orderScopeFromTenant(tenant resellercontract.TenantContext) ordercontract.TenantScope {
	if isResellerOrderContext(tenant) && tenant.ResellerID != nil {
		return ordercontract.TenantScope{ResellerID: tenant.ResellerID}
	}
	return ordercontract.TenantScope{}
}

// ListOrdersForAdmin 管理端订单列表
func (s *OrderService) ListOrdersForAdmin(filter ordercontract.ListFilter) ([]orderdomain.Order, int64, error) {
	orders, total, err := s.orderStore.ListAdmin(filter)
	if err != nil {
		return nil, 0, ErrOrderFetchFailed
	}
	if err := s.ensureOrdersCanceledIfExpired(orders); err != nil {
		return nil, 0, ErrOrderUpdateFailed
	}
	if err := s.ensureOrdersRefundStatusSynced(orders); err != nil {
		return nil, 0, ErrOrderUpdateFailed
	}
	FillOrdersItemsFromChildren(orders)
	return orders, total, nil
}

// GetOrderForAdmin 管理端订单详情
func (s *OrderService) GetOrderForAdmin(orderID uint) (*orderdomain.Order, error) {
	if orderID == 0 {
		return nil, ErrOrderNotFound
	}
	order, err := s.orderStore.GetByID(orderID)
	if err != nil {
		return nil, ErrOrderFetchFailed
	}
	if order == nil {
		return nil, ErrOrderNotFound
	}
	if err := s.ensureOrderCanceledIfExpired(order); err != nil {
		return nil, ErrOrderUpdateFailed
	}
	if err := s.ensureOrderRefundStatusSynced(order); err != nil {
		return nil, ErrOrderUpdateFailed
	}
	FillOrderItemsFromChildren(order)
	return order, nil
}
