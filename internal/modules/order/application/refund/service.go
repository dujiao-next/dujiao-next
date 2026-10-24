package refund

import (
	"strconv"
	"strings"
	"time"

	orderapp "github.com/dujiao-next/internal/modules/order/application"

	affiliatecontract "github.com/dujiao-next/internal/modules/affiliate/contract"
	usercontract "github.com/dujiao-next/internal/modules/identity/user/contract"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	ordercontract "github.com/dujiao-next/internal/modules/order/contract"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	walletapp "github.com/dujiao-next/internal/modules/wallet/application"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// AdminManualRefundInput 管理员手动退款输入（不处理钱包/支付渠道）
type AdminManualRefundInput struct {
	OrderID            uint
	Amount             money.Amount
	Remark             string
	PaymentFeeRefunded bool
}

// AdminOrderRefundListQuery 管理端退款记录列表查询条件（原始输入）。
type AdminOrderRefundListQuery struct {
	Page           int
	PageSize       int
	UserID         string
	UserKeyword    string
	OrderNo        string
	GuestEmail     string
	ProductKeyword string
	ProductName    string
	CreatedFrom    string
	CreatedTo      string
}

// AdminOrderRefundItem 管理端订单退款返回项（列表/详情统一字段）。
type AdminOrderRefundItem struct {
	orderdomain.OrderRefundRecord
	OrderNo         string                  `json:"order_no,omitempty"`
	GuestLocale     string                  `json:"guest_locale,omitempty"`
	Items           []orderdomain.OrderItem `json:"items,omitempty"`
	UserEmail       string                  `json:"user_email,omitempty"`
	UserDisplayName string                  `json:"user_display_name,omitempty"`
	RefundTypeLabel string                  `json:"refund_type_label"`
}

// Service 编排订单退款、退款记录和关联领域的逆向处理。
type Service struct {
	orderStore         ordercontract.Store
	userRepo           usercontract.Store
	affiliateRefund    affiliateRefundProcessor
	settingService     *settingsapp.Service
	resellerAccounting resellerAccountingTransactions
	wallets            *walletapp.Service
	payments           paymentFeeReader
}

type affiliateRefundProcessor interface {
	HandleOrderRefunded(
		store affiliatecontract.Store,
		order *orderdomain.Order,
		refundDelta decimal.Decimal,
		refundedBefore decimal.Decimal,
		reason string,
	) error
}

type resellerAccountingTransactions interface {
	HandleRefundDeduct(
		store resellercontract.AccountingLedgerStore,
		order *orderdomain.Order,
		refundRecord *orderdomain.OrderRefundRecord,
		refundedBefore decimal.Decimal,
	) error
}

// OrderStatusEmailRefundDetails 订单状态邮件中的退款信息
type OrderStatusEmailRefundDetails struct {
	Amount money.Amount
	Reason string
}

// ResolveOrderStatusEmailRefundDetails 解析订单状态邮件展示的退款信息。
// 优先使用 refund_record_id 对应记录；若无匹配或服务不可用，回退到订单累计退款金额。
// 当解析退款记录失败时，仍返回回退结果，并透传 error 供调用方记录日志。
func (s *Service) ResolveOrderStatusEmailRefundDetails(order *orderdomain.Order, refundRecordID uint) (OrderStatusEmailRefundDetails, error) {
	if order == nil {
		return OrderStatusEmailRefundDetails{}, nil
	}
	fallback := OrderStatusEmailRefundDetails{
		Amount: order.RefundedAmount,
		Reason: "",
	}
	if s == nil || refundRecordID == 0 {
		return fallback, nil
	}
	details, ok, err := s.ResolveStatusEmailRefundDetails(order.ID, refundRecordID)
	if err != nil {
		return fallback, err
	}
	if ok {
		return details, nil
	}
	return fallback, nil
}

// New 创建订单退款服务。
func New(
	orderStore ordercontract.Store,
	userRepo usercontract.Store,
	affiliateRefund affiliateRefundProcessor,
	settingService *settingsapp.Service,
	wallets *walletapp.Service,
	payments paymentFeeReader,
) *Service {
	return &Service{
		orderStore:      orderStore,
		userRepo:        userRepo,
		affiliateRefund: affiliateRefund,
		settingService:  settingService,
		wallets:         wallets,
		payments:        payments,
	}
}

func (s *Service) SetResellerAccounting(accounting resellerAccountingTransactions) {
	s.resellerAccounting = accounting
}

// ParseRefundAmount 解析并校验退款金额。
func (s *Service) ParseRefundAmount(raw string) (money.Amount, error) {
	parsed, err := decimal.NewFromString(strings.TrimSpace(raw))
	if err != nil {
		return money.Amount{}, walletcontract.ErrInvalidAmount
	}
	amount := parsed.Round(2)
	if amount.LessThanOrEqual(decimal.Zero) {
		return money.Amount{}, walletcontract.ErrInvalidAmount
	}
	return money.FromDecimal(amount), nil
}

// ParseAdminRefundListFilter 解析管理端退款记录列表筛选条件。
func (s *Service) ParseAdminRefundListFilter(query AdminOrderRefundListQuery) (ordercontract.RefundRecordListFilter, error) {
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}

	userID, err := parseOptionalUint(query.UserID)
	if err != nil {
		return ordercontract.RefundRecordListFilter{}, err
	}
	createdFrom, err := parseRFC3339Nullable(query.CreatedFrom)
	if err != nil {
		return ordercontract.RefundRecordListFilter{}, err
	}
	createdTo, err := parseRFC3339Nullable(query.CreatedTo)
	if err != nil {
		return ordercontract.RefundRecordListFilter{}, err
	}

	productKeyword := strings.TrimSpace(query.ProductKeyword)
	if productKeyword == "" {
		productKeyword = strings.TrimSpace(query.ProductName)
	}

	return ordercontract.RefundRecordListFilter{
		Page:           page,
		PageSize:       pageSize,
		UserID:         userID,
		UserKeyword:    strings.TrimSpace(query.UserKeyword),
		OrderNo:        strings.TrimSpace(query.OrderNo),
		GuestEmail:     strings.TrimSpace(query.GuestEmail),
		ProductKeyword: productKeyword,
		CreatedFrom:    createdFrom,
		CreatedTo:      createdTo,
	}, nil
}

// ListAdminRefundItems 管理端退款记录列表（含展示所需关联字段）。
func (s *Service) ListAdminRefundItems(query AdminOrderRefundListQuery) ([]AdminOrderRefundItem, int64, error) {
	filter, err := s.ParseAdminRefundListFilter(query)
	if err != nil {
		return nil, 0, err
	}
	records, total, err := s.ListAdminRefundRecords(filter)
	if err != nil {
		return nil, 0, err
	}

	orderMap, err := s.resolveRefundOrders(records)
	if err != nil {
		return nil, 0, ErrOrderFetchFailed
	}
	userMap, err := s.resolveRefundUsers(records, orderMap)
	if err != nil {
		return nil, 0, ErrOrderFetchFailed
	}

	items := make([]AdminOrderRefundItem, 0, len(records))
	for _, record := range records {
		order := orderMap[record.OrderID]
		items = append(items, s.buildAdminRefundItem(record, order, userMap))
	}
	return items, total, nil
}

// GetAdminRefundItem 管理端退款记录详情（含展示所需关联字段）。
func (s *Service) GetAdminRefundItem(id uint) (*AdminOrderRefundItem, error) {
	record, err := s.GetAdminRefundRecord(id)
	if err != nil {
		return nil, err
	}
	if s == nil || s.orderStore == nil {
		return nil, ErrOrderFetchFailed
	}

	order, err := s.orderStore.GetByID(record.OrderID)
	if err != nil {
		return nil, ErrOrderFetchFailed
	}
	userMap, err := s.resolveRefundUsers([]orderdomain.OrderRefundRecord{*record}, map[uint]*orderdomain.Order{
		record.OrderID: order,
	})
	if err != nil {
		return nil, ErrOrderFetchFailed
	}
	item := s.buildAdminRefundItem(*record, order, userMap)
	return &item, nil
}

// AdminManualRefund 管理端手动退款（不处理钱包/支付渠道，仅写 order_refund_records）
func (s *Service) AdminManualRefund(input AdminManualRefundInput) (*orderdomain.Order, *orderdomain.OrderRefundRecord, error) {
	if input.OrderID == 0 {
		return nil, nil, ErrOrderNotFound
	}
	amount := input.Amount.Decimal.Round(2)
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, nil, walletcontract.ErrInvalidAmount
	}
	recordRemark := strings.TrimSpace(input.Remark)
	var createdRecord *orderdomain.OrderRefundRecord
	if s == nil || s.orderStore == nil {
		return nil, nil, ErrOrderFetchFailed
	}

	cfg := settingsapp.DefaultOrderRefundConfig()
	if s.settingService != nil {
		cfgLoaded, cfgErr := s.settingService.GetOrderRefundConfig()
		if cfgErr != nil {
			return nil, nil, cfgErr
		}
		cfg = cfgLoaded
	}
	initialOrder, err := s.orderStore.GetByID(input.OrderID)
	if err != nil {
		return nil, nil, ErrOrderFetchFailed
	}
	if initialOrder == nil {
		return nil, nil, ErrOrderNotFound
	}
	feeSnapshot := paymentFeeRefundSnapshot{rootOrderID: paymentFeeRefundRootOrderID(initialOrder)}
	if input.PaymentFeeRefunded {
		feeSnapshot, err = s.loadPaymentFeeRefundSnapshot(initialOrder)
		if err != nil {
			return nil, nil, err
		}
	}

	if err := s.orderStore.WithinTransaction(func(tx ordercontract.Transaction) error {
		orders := tx.Orders()
		locked, err := orders.GetByIDForUpdate(input.OrderID)
		if err != nil {
			return err
		}
		if locked == nil {
			return ErrOrderNotFound
		}
		order := *locked
		if order.PaidAt == nil {
			return ErrOrderStatusInvalid
		}
		if settingsapp.IsOrderRefundWindowExpired(order.CreatedAt, order.PaidAt, cfg.MaxRefundDays, time.Now()) {
			return ErrOrderRefundExpired
		}
		if order.TotalAmount.Decimal.LessThanOrEqual(decimal.Zero) {
			return ErrOrderStatusInvalid
		}
		refundedBefore := order.RefundedAmount.Decimal.Round(2)
		refundable := order.TotalAmount.Decimal.Sub(refundedBefore).Round(2)
		if amount.GreaterThan(refundable) {
			return walletcontract.ErrRefundExceeded
		}
		now := time.Now()
		if err := reserveParentRefund(orders, &order, amount, now); err != nil {
			return err
		}

		newRefunded := refundedBefore.Add(amount).Round(2)
		updates := map[string]interface{}{
			"refunded_amount": money.FromDecimal(newRefunded),
			"updated_at":      now,
		}
		markRefunded := newRefunded.GreaterThanOrEqual(order.TotalAmount.Decimal.Round(2))
		if markRefunded {
			updates["status"] = constants.OrderStatusRefunded
		} else {
			updates["status"] = constants.OrderStatusPartiallyRefunded
		}
		if err := orders.UpdateFields(order.ID, updates); err != nil {
			return ErrOrderUpdateFailed
		}
		if order.ParentID == nil {
			targetStatus := constants.OrderStatusPartiallyRefunded
			if markRefunded {
				targetStatus = constants.OrderStatusRefunded
			}
			if err := applyParentRefundChildStatusUpdates(orders, order.ID, targetStatus, now); err != nil {
				return ErrOrderUpdateFailed
			}
		}
		if order.ParentID != nil {
			if _, err := orderapp.SyncParentStatus(orders, *order.ParentID, now); err != nil {
				return ErrOrderUpdateFailed
			}
		}
		feeRefundedAmount := money.FromDecimal(decimal.Zero)
		if input.PaymentFeeRefunded {
			if paymentFeeRefundRootOrderID(&order) != feeSnapshot.rootOrderID {
				return ErrOrderFetchFailed
			}
			feeRefundedAmount, err = resolvePaymentFeeRefundAmount(orders, feeSnapshot, amount, 0)
			if err != nil {
				return err
			}
		}
		record, err := s.createRefundRecordTx(
			orders,
			&order,
			constants.OrderRefundTypeManual,
			amount,
			recordRemark,
			input.PaymentFeeRefunded,
			feeRefundedAmount,
			now,
		)
		if err != nil {
			return err
		}
		createdRecord = record
		if s.affiliateRefund != nil && order.UserID > 0 {
			if err := s.affiliateRefund.HandleOrderRefunded(
				tx.Affiliates(),
				&order,
				amount,
				refundedBefore,
				"order_refunded_manual",
			); err != nil {
				return err
			}
		}
		if s.resellerAccounting != nil {
			if err := s.resellerAccounting.HandleRefundDeduct(tx.ResellerAccounting(), &order, record, refundedBefore); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, nil, err
	}

	order, err := s.orderStore.GetByID(input.OrderID)
	if err != nil {
		return nil, nil, ErrOrderFetchFailed
	}
	if order == nil {
		return nil, nil, ErrOrderNotFound
	}
	return order, createdRecord, nil
}

// ListAdminRefundRecords 管理端退款记录列表
func (s *Service) ListAdminRefundRecords(filter ordercontract.RefundRecordListFilter) ([]orderdomain.OrderRefundRecord, int64, error) {
	if s == nil || s.orderStore == nil {
		return nil, 0, ErrOrderFetchFailed
	}
	records, total, err := s.orderStore.ListRefundRecordsAdmin(filter)
	if err != nil {
		return nil, 0, ErrOrderFetchFailed
	}
	return records, total, nil
}

// GetAdminRefundRecord 管理端退款记录详情
func (s *Service) GetAdminRefundRecord(id uint) (*orderdomain.OrderRefundRecord, error) {
	if s == nil || s.orderStore == nil {
		return nil, ErrOrderFetchFailed
	}
	if id == 0 {
		return nil, ErrOrderNotFound
	}
	record, err := s.orderStore.GetRefundRecordByID(id)
	if err != nil {
		return nil, ErrOrderFetchFailed
	}
	if record == nil {
		return nil, ErrOrderNotFound
	}
	return record, nil
}

// ResolveStatusEmailRefundDetails 按退款记录解析订单状态邮件所需的退款金额与原因。
// 仅当退款记录与目标订单有关联时返回 true（同一订单，或退款记录来自该父订单的子订单）。
func (s *Service) ResolveStatusEmailRefundDetails(orderID uint, refundRecordID uint) (OrderStatusEmailRefundDetails, bool, error) {
	if s == nil || s.orderStore == nil || refundRecordID == 0 {
		return OrderStatusEmailRefundDetails{}, false, nil
	}

	record, err := s.orderStore.GetRefundRecordByID(refundRecordID)
	if err != nil {
		return OrderStatusEmailRefundDetails{}, false, ErrOrderFetchFailed
	}
	if record == nil {
		return OrderStatusEmailRefundDetails{}, false, nil
	}
	if orderID == 0 {
		return OrderStatusEmailRefundDetails{
			Amount: record.Amount,
			Reason: strings.TrimSpace(record.Remark),
		}, true, nil
	}
	if record.OrderID == orderID {
		return OrderStatusEmailRefundDetails{
			Amount: record.Amount,
			Reason: strings.TrimSpace(record.Remark),
		}, true, nil
	}
	if s.orderStore == nil {
		return OrderStatusEmailRefundDetails{}, false, nil
	}

	recordOrder, err := s.orderStore.GetByID(record.OrderID)
	if err != nil {
		return OrderStatusEmailRefundDetails{}, false, ErrOrderFetchFailed
	}
	if recordOrder != nil && recordOrder.ParentID != nil && *recordOrder.ParentID == orderID {
		return OrderStatusEmailRefundDetails{
			Amount: record.Amount,
			Reason: strings.TrimSpace(record.Remark),
		}, true, nil
	}
	return OrderStatusEmailRefundDetails{}, false, nil
}

// createRefundRecordTx 在事务内写入退款记录（order_refund_records）。
func (s *Service) createRefundRecordTx(
	orders ordercontract.Store,
	order *orderdomain.Order,
	refundType string,
	amount decimal.Decimal,
	remark string,
	paymentFeeRefunded bool,
	paymentFeeRefundedAmount money.Amount,
	now time.Time,
) (*orderdomain.OrderRefundRecord, error) {
	if orders == nil || order == nil {
		return nil, ErrRefundRecordCreateFailed
	}
	currency := strings.ToUpper(strings.TrimSpace(order.Currency))
	if currency == "" {
		currency = "CNY"
	}
	record := &orderdomain.OrderRefundRecord{
		UserID:                   order.UserID,
		GuestEmail:               order.GuestEmail,
		OrderID:                  order.ID,
		Type:                     strings.TrimSpace(refundType),
		Amount:                   money.FromDecimal(amount.Round(2)),
		PaymentFeeRefunded:       paymentFeeRefunded,
		PaymentFeeRefundedAmount: paymentFeeRefundedAmount,
		Currency:                 currency,
		Remark:                   remark,
		CreatedAt:                now,
		UpdatedAt:                now,
	}
	if err := orders.CreateRefundRecord(record); err != nil {
		return nil, ErrRefundRecordCreateFailed
	}
	return record, nil
}

// parseOptionalUint 解析可选正整数查询参数，空串返回 0。
func parseOptionalUint(raw string) (uint, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseUint(trimmed, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(parsed), nil
}

// parseRFC3339Nullable 解析可空 RFC3339 时间，空串返回 nil。
func parseRFC3339Nullable(raw string) (*time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

// resolveRefundOrders 批量加载退款记录关联订单，避免重复查询。
func (s *Service) resolveRefundOrders(records []orderdomain.OrderRefundRecord) (map[uint]*orderdomain.Order, error) {
	result := make(map[uint]*orderdomain.Order, len(records))
	if s == nil || s.orderStore == nil {
		return result, ErrOrderFetchFailed
	}
	seen := make(map[uint]struct{}, len(records))
	for _, record := range records {
		if record.OrderID == 0 {
			continue
		}
		if _, ok := seen[record.OrderID]; ok {
			continue
		}
		seen[record.OrderID] = struct{}{}
		order, err := s.orderStore.GetByID(record.OrderID)
		if err != nil {
			return nil, err
		}
		result[record.OrderID] = order
	}
	return result, nil
}

// resolveRefundUsers 批量加载退款记录关联用户（游客退款不加载用户）。
func (s *Service) resolveRefundUsers(records []orderdomain.OrderRefundRecord, orderMap map[uint]*orderdomain.Order) (map[uint]userdomain.User, error) {
	result := make(map[uint]userdomain.User)
	if s == nil || s.userRepo == nil {
		return result, nil
	}

	userIDs := make([]uint, 0, len(records))
	seen := make(map[uint]struct{}, len(records))
	for _, record := range records {
		userID := resolveRefundUserID(record, orderMap[record.OrderID])
		if userID == 0 {
			continue
		}
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		userIDs = append(userIDs, userID)
	}
	if len(userIDs) == 0 {
		return result, nil
	}

	users, err := s.userRepo.ListByIDs(userIDs)
	if err != nil {
		return nil, err
	}
	for _, user := range users {
		result[user.ID] = user
	}
	return result, nil
}

// buildAdminRefundItem 组装管理端退款记录展示项。
func (s *Service) buildAdminRefundItem(record orderdomain.OrderRefundRecord, order *orderdomain.Order, userMap map[uint]userdomain.User) AdminOrderRefundItem {
	recordGuestEmail := resolveRefundGuestEmail(record, order)
	recordGuestLocale := resolveRefundGuestLocale(order)
	recordForResponse := record
	recordForResponse.GuestEmail = recordGuestEmail

	user := userMap[resolveRefundUserID(recordForResponse, order)]
	userEmail := strings.TrimSpace(user.Email)
	userDisplayName := strings.TrimSpace(user.DisplayName)
	if recordGuestEmail != "" {
		userEmail = ""
		userDisplayName = ""
	}

	return AdminOrderRefundItem{
		OrderRefundRecord: recordForResponse,
		OrderNo:           resolveRefundOrderNo(order),
		GuestLocale:       recordGuestLocale,
		Items:             collectRefundItems(order),
		UserEmail:         userEmail,
		UserDisplayName:   userDisplayName,
		RefundTypeLabel:   recordForResponse.Type,
	}
}

// resolveRefundUserID 解析退款记录对应的用户ID（游客订单返回0）。
func resolveRefundUserID(record orderdomain.OrderRefundRecord, order *orderdomain.Order) uint {
	if resolveRefundGuestEmail(record, order) != "" {
		return 0
	}
	if record.UserID > 0 {
		return record.UserID
	}
	if order != nil && order.UserID > 0 {
		return order.UserID
	}
	return 0
}

// resolveRefundGuestEmail 解析退款记录对应的游客邮箱（优先记录字段，其次订单字段）。
func resolveRefundGuestEmail(record orderdomain.OrderRefundRecord, order *orderdomain.Order) string {
	if guestEmail := strings.TrimSpace(record.GuestEmail); guestEmail != "" {
		return guestEmail
	}
	if order != nil {
		return strings.TrimSpace(order.GuestEmail)
	}
	return ""
}

// resolveRefundGuestLocale 解析退款订单的游客语言设置。
func resolveRefundGuestLocale(order *orderdomain.Order) string {
	if order == nil {
		return ""
	}
	return strings.TrimSpace(order.GuestLocale)
}

// resolveRefundOrderNo 解析退款记录关联订单号。
func resolveRefundOrderNo(order *orderdomain.Order) string {
	if order == nil {
		return ""
	}
	return strings.TrimSpace(order.OrderNo)
}

// collectRefundItems 收集退款记录对应订单及其子订单的商品项快照。
func collectRefundItems(order *orderdomain.Order) []orderdomain.OrderItem {
	if order == nil {
		return nil
	}
	result := make([]orderdomain.OrderItem, 0, len(order.Items))
	result = append(result, order.Items...)
	for i := range order.Children {
		result = append(result, order.Children[i].Items...)
	}
	return result
}
