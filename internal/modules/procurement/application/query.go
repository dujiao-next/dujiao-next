package application

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	procurementcontract "github.com/dujiao-next/internal/modules/procurement/contract"
	procurementdomain "github.com/dujiao-next/internal/modules/procurement/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// GetByID 根据 ID 获取采购单
func (s *Service) GetByID(id uint) (*procurementdomain.Order, error) {
	procOrder, err := s.procRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if procOrder == nil {
		return nil, procurementcontract.ErrNotFound
	}
	s.fillUpstreamRefundRecordsForProcurementOrder(context.Background(), procOrder)
	return procOrder, nil
}

// GetByLocalOrderNo 根据本地订单号获取采购单
func (s *Service) GetByLocalOrderNo(localOrderNo string) (*procurementdomain.Order, error) {
	return s.procRepo.GetByLocalOrderNo(localOrderNo)
}

// List 列表查询采购单
func (s *Service) List(filter procurementcontract.ListFilter) ([]procurementdomain.Order, int64, error) {
	orders, total, err := s.procRepo.List(filter)
	if err != nil {
		return nil, 0, err
	}
	s.fillParentOrderNos(orders)
	s.fillUpstreamRefundRecordsForProcurementOrders(orders)
	return orders, total, nil
}

// StatsByStatus 按状态聚合采购单数量（基于全量数据）
func (s *Service) StatsByStatus(filter procurementcontract.ListFilter) (map[string]int64, error) {
	return s.procRepo.StatsByStatus(filter)
}

// FillParentOrderNo 为单个采购单填充父订单号
func (s *Service) FillParentOrderNo(order *procurementdomain.Order) {
	if order == nil || order.LocalOrder == nil || order.LocalOrder.ParentID == nil {
		return
	}
	parentOrder, err := s.orderRepo.GetByID(*order.LocalOrder.ParentID)
	if err == nil && parentOrder != nil {
		order.ParentOrderNo = parentOrder.OrderNo
		applyProcurementLocalRefundedAmountFallback(order.LocalOrder, parentOrder)
	}
}

// fillParentOrderNos 为采购单批量填充父订单号
func (s *Service) fillParentOrderNos(orders []procurementdomain.Order) {
	// 收集需要查询的父订单 ID
	parentIDs := make(map[uint]bool)
	for i := range orders {
		if orders[i].LocalOrder != nil && orders[i].LocalOrder.ParentID != nil {
			parentIDs[*orders[i].LocalOrder.ParentID] = true
		}
	}
	if len(parentIDs) == 0 {
		return
	}

	ids := make([]uint, 0, len(parentIDs))
	for id := range parentIDs {
		ids = append(ids, id)
	}

	parentOrders, err := s.orderRepo.GetByIDs(ids)
	if err != nil {
		return
	}
	parentMap := make(map[uint]*procurementdomain.LocalOrder, len(parentOrders))
	for _, o := range parentOrders {
		order := o
		parentMap[o.ID] = &order
	}

	for i := range orders {
		if orders[i].LocalOrder != nil && orders[i].LocalOrder.ParentID != nil {
			if parent := parentMap[*orders[i].LocalOrder.ParentID]; parent != nil {
				orders[i].ParentOrderNo = parent.OrderNo
				applyProcurementLocalRefundedAmountFallback(orders[i].LocalOrder, parent)
			}
		}
	}
}

// applyProcurementLocalRefundedAmountFallback 在子订单退款金额为空时回填父订单退款金额，便于采购单视图展示。
func applyProcurementLocalRefundedAmountFallback(localOrder *procurementdomain.LocalOrder, parentOrder *procurementdomain.LocalOrder) {
	if localOrder == nil || parentOrder == nil {
		return
	}
	localRefunded := localOrder.RefundedAmount.Decimal.Round(2)
	if localRefunded.GreaterThan(decimal.Zero) {
		return
	}
	parentRefunded := parentOrder.RefundedAmount.Decimal.Round(2)
	if parentRefunded.LessThanOrEqual(decimal.Zero) {
		return
	}
	localOrder.RefundedAmount = money.FromDecimal(parentRefunded)
}

// shouldSyncUpstreamRefundStatus 判断当前采购单状态是否需要从上游拉取退款信息。
func shouldSyncUpstreamRefundStatus(localStatus string) bool {
	switch strings.ToLower(strings.TrimSpace(localStatus)) {
	case constants.ProcurementStatusFulfilled,
		constants.ProcurementStatusCompleted,
		constants.ProcurementStatusPartiallyRefunded,
		constants.ProcurementStatusRefunded:
		return true
	default:
		return false
	}
}

// normalizeProcurementUpstreamStatus 规范化上游状态字符串（去空白+小写）。
func normalizeProcurementUpstreamStatus(status string) string {
	return strings.ToLower(strings.TrimSpace(status))
}

// buildUpstreamRefundRecords 标准化上游退款记录并按 created_at 升序排序，随后重排顺序ID。
func buildUpstreamRefundRecords(records []jsonmap.JSON) []jsonmap.JSON {
	if len(records) == 0 {
		return make([]jsonmap.JSON, 0)
	}
	normalized := make([]jsonmap.JSON, 0, len(records))
	for i := range records {
		record := make(jsonmap.JSON, len(records[i]))
		for k, v := range records[i] {
			record[k] = v
		}
		normalized = append(normalized, record)
	}
	sort.SliceStable(normalized, func(i, j int) bool {
		ti, okI := parseUpstreamRefundRecordCreatedAt(normalized[i]["created_at"])
		tj, okJ := parseUpstreamRefundRecordCreatedAt(normalized[j]["created_at"])
		switch {
		case okI && okJ:
			if ti.Equal(tj) {
				return false
			}
			return ti.Before(tj)
		case okI:
			return true
		case okJ:
			return false
		default:
			return false
		}
	})
	for i := range normalized {
		// 不透传上游退款记录主键，统一使用列表自增序号（按排序后序号）。
		normalized[i]["id"] = i + 1
	}
	return normalized
}

// parseUpstreamRefundRecordCreatedAt 解析上游退款记录中的 created_at 字段并返回可排序时间值。
func parseUpstreamRefundRecordCreatedAt(v interface{}) (time.Time, bool) {
	switch value := v.(type) {
	case time.Time:
		return value, true
	case *time.Time:
		if value == nil {
			return time.Time{}, false
		}
		return *value, true
	case string:
		s := strings.TrimSpace(value)
		if s == "" {
			return time.Time{}, false
		}
		formats := []string{
			time.RFC3339Nano,
			time.RFC3339,
			"2006-01-02 15:04:05.999999999",
			"2006-01-02 15:04:05",
		}
		for _, layout := range formats {
			if parsed, err := time.Parse(layout, s); err == nil {
				return parsed, true
			}
		}
		return time.Time{}, false
	case int64:
		return time.Unix(value, 0), true
	case int:
		return time.Unix(int64(value), 0), true
	case float64:
		return time.Unix(int64(value), 0), true
	default:
		return time.Time{}, false
	}
}

// fillUpstreamRefundRecordsForProcurementOrder 为单条采购单补充上游退款记录与退款金额，并同步退款状态。
func (s *Service) fillUpstreamRefundRecordsForProcurementOrder(parent context.Context, order *procurementdomain.Order) {
	if order == nil {
		return
	}
	order.UpstreamRefundRecords = nil
	order.UpstreamRefundedAmount = ""
	if s.connections == nil || order.UpstreamOrderID == 0 || !shouldSyncUpstreamRefundStatus(order.Status) {
		return
	}
	connection, err := s.connections.Open(order.ConnectionID)
	if err != nil || connection == nil {
		return
	}
	ctx, cancel := context.WithTimeout(parent, 8*time.Second)
	defer cancel()
	detail, err := connection.GetOrder(ctx, order.UpstreamOrderID)
	if err != nil || detail == nil {
		return
	}
	upstreamRefundRecords := buildUpstreamRefundRecords(detail.RefundRecords)
	upstreamRefundedAmount := strings.TrimSpace(detail.RefundedAmount)
	hasRefundRecords := len(upstreamRefundRecords) > 0
	hasRefundedAmount := isPositiveUpstreamRefundAmount(upstreamRefundedAmount)
	if hasRefundRecords {
		order.UpstreamRefundRecords = upstreamRefundRecords
	}
	if hasRefundedAmount {
		order.UpstreamRefundedAmount = upstreamRefundedAmount
	}

	upstreamStatus := normalizeProcurementUpstreamStatus(detail.Status)
	if upstreamStatus != "refunded" && upstreamStatus != "partially_refunded" {
		return
	}
	targetStatus := constants.ProcurementStatusPartiallyRefunded
	if upstreamStatus == "refunded" {
		targetStatus = constants.ProcurementStatusRefunded
	}
	if strings.EqualFold(strings.TrimSpace(order.Status), targetStatus) {
		order.Status = targetStatus
		return
	}
	if _, err := s.procRepo.UpdateStatus(order.ID, order.Status, targetStatus, map[string]interface{}{"updated_at": time.Now()}); err != nil {
		logger.Warnw("procurement_sync_refund_status_failed",
			"procurement_order_id", order.ID,
			"upstream_order_id", order.UpstreamOrderID,
			"upstream_status", upstreamStatus,
			"error", err,
		)
		return
	}
	order.Status = targetStatus
}

// isPositiveUpstreamRefundAmount 判断上游退款金额字符串是否为正数。
func isPositiveUpstreamRefundAmount(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}
	amount, err := decimal.NewFromString(trimmed)
	if err != nil {
		return false
	}
	return amount.Round(2).GreaterThan(decimal.Zero)
}

// fillUpstreamRefundRecordsForProcurementOrders 批量为采购单补充上游退款记录与退款金额。
// 列表页并发查询并设整页总超时：上游变慢或宕机时，超时的行只是缺少上游退款数据，页面不会被拖到超时。
func (s *Service) fillUpstreamRefundRecordsForProcurementOrders(orders []procurementdomain.Order) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for i := range orders {
		wg.Add(1)
		sem <- struct{}{}
		go func(order *procurementdomain.Order) {
			defer wg.Done()
			defer func() { <-sem }()
			s.fillUpstreamRefundRecordsForProcurementOrder(ctx, order)
		}(&orders[i])
	}
	wg.Wait()
}
