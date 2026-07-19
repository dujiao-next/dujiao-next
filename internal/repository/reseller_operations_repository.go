package repository

import (
	"database/sql"
	"sort"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// ResellerOperationsRepository provides read-only reseller operations aggregates.
type ResellerOperationsRepository interface {
	GetOverview(startAt, endAt time.Time) (ResellerOperationsOverviewRow, error)
	GetFinance(startAt, endAt time.Time) (ResellerOperationsFinanceRowSet, error)
}

type ResellerOperationsLifecycleRow struct {
	ProfilesTotal                   int64
	ProfilesPendingReview           int64
	ProfilesActive                  int64
	ProfilesRejected                int64
	ProfilesDisabled                int64
	ProfilesSettlementFrozen        int64
	DomainsTotal                    int64
	DomainsPendingReview            int64
	DomainsActive                   int64
	DomainsDisabled                 int64
	DomainsPendingVerification      int64
	DomainsVerified                 int64
	CustomDomains                   int64
	Subdomains                      int64
	SiteConfigsTotal                int64
	ActiveProfilesWithoutSiteConfig int64
}

type ResellerOperationsOrdersRow struct {
	OrdersTotal               int64
	PaidOrders                int64
	CompletedOrders           int64
	RefundedOrders            int64
	SelfDealingBlockedOrders  int64
	ActiveResellersWithOrders int64
}

type ResellerOperationsTopResellerRow struct {
	ResellerID     uint
	UserID         uint
	Email          string
	DisplayName    string
	OrdersTotal    int64
	PaidOrders     int64
	ActiveDomains  int64
	SiteConfigured bool
	LastOrderAt    *time.Time
}

type ResellerOperationsOverviewRow struct {
	Lifecycle    ResellerOperationsLifecycleRow
	Orders       ResellerOperationsOrdersRow
	TopResellers []ResellerOperationsTopResellerRow
}

type ResellerOperationsPeriodCurrencyRow struct {
	Currency       string
	OrdersTotal    int64
	PaidOrders     int64
	GMVPaid        decimal.Decimal
	ProfitEarned   decimal.Decimal
	RefundDeducted decimal.Decimal
	WithdrawPaid   decimal.Decimal
}

type ResellerOperationsCurrentCurrencyRow struct {
	Currency                string
	AvailableBalance        decimal.Decimal
	LockedBalance           decimal.Decimal
	NegativeBalance         decimal.Decimal
	PendingWithdrawCount    int64
	PendingWithdrawAmount   decimal.Decimal
	NegativeBalanceAccounts int64
	FrozenBalanceAccounts   int64
}

type ResellerOperationsFinanceRowSet struct {
	PeriodCurrencyRows  []ResellerOperationsPeriodCurrencyRow
	CurrentCurrencyRows []ResellerOperationsCurrentCurrencyRow
}

type GormResellerOperationsRepository struct {
	db *gorm.DB
}

func NewResellerOperationsRepository(db *gorm.DB) *GormResellerOperationsRepository {
	return &GormResellerOperationsRepository{db: db}
}

func resellerOperationsPaidStatuses() []string {
	return []string{
		constants.OrderStatusPaid,
		constants.OrderStatusFulfilling,
		constants.OrderStatusPartiallyDelivered,
		constants.OrderStatusPartiallyRefunded,
		constants.OrderStatusDelivered,
		constants.OrderStatusFulfilled,
		constants.OrderStatusCompleted,
	}
}

func (r *GormResellerOperationsRepository) GetOverview(startAt, endAt time.Time) (ResellerOperationsOverviewRow, error) {
	var out ResellerOperationsOverviewRow
	if r == nil || r.db == nil {
		return out, nil
	}
	if err := r.scanLifecycle(&out.Lifecycle); err != nil {
		return out, err
	}
	if err := r.scanOrderOverview(startAt, endAt, &out.Orders); err != nil {
		return out, err
	}
	top, err := r.scanTopResellers(startAt, endAt)
	if err != nil {
		return out, err
	}
	out.TopResellers = top
	return out, nil
}

func (r *GormResellerOperationsRepository) GetFinance(startAt, endAt time.Time) (ResellerOperationsFinanceRowSet, error) {
	out := ResellerOperationsFinanceRowSet{
		PeriodCurrencyRows:  []ResellerOperationsPeriodCurrencyRow{},
		CurrentCurrencyRows: []ResellerOperationsCurrentCurrencyRow{},
	}
	if r == nil || r.db == nil {
		return out, nil
	}
	period, err := r.scanPeriodCurrencyRows(startAt, endAt)
	if err != nil {
		return out, err
	}
	current, err := r.scanCurrentCurrencyRows()
	if err != nil {
		return out, err
	}
	out.PeriodCurrencyRows = period
	out.CurrentCurrencyRows = current
	return out, nil
}

func (r *GormResellerOperationsRepository) scanLifecycle(out *ResellerOperationsLifecycleRow) error {
	type profileScan struct {
		ProfilesTotal            int64
		ProfilesPendingReview    int64
		ProfilesActive           int64
		ProfilesRejected         int64
		ProfilesDisabled         int64
		ProfilesSettlementFrozen int64
	}
	var profiles profileScan
	if err := r.db.Model(&models.ResellerProfile{}).
		Select(`
			COUNT(1) AS profiles_total,
			SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS profiles_pending_review,
			SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS profiles_active,
			SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS profiles_rejected,
			SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS profiles_disabled,
			SUM(CASE WHEN settlement_status = ? THEN 1 ELSE 0 END) AS profiles_settlement_frozen
		`,
			models.ResellerProfileStatusPendingReview,
			models.ResellerProfileStatusActive,
			models.ResellerProfileStatusRejected,
			models.ResellerProfileStatusDisabled,
			models.ResellerSettlementStatusFrozen,
		).
		Scan(&profiles).Error; err != nil {
		return err
	}
	out.ProfilesTotal = profiles.ProfilesTotal
	out.ProfilesPendingReview = profiles.ProfilesPendingReview
	out.ProfilesActive = profiles.ProfilesActive
	out.ProfilesRejected = profiles.ProfilesRejected
	out.ProfilesDisabled = profiles.ProfilesDisabled
	out.ProfilesSettlementFrozen = profiles.ProfilesSettlementFrozen

	type domainScan struct {
		DomainsTotal               int64
		DomainsPendingReview       int64
		DomainsActive              int64
		DomainsDisabled            int64
		DomainsPendingVerification int64
		DomainsVerified            int64
		CustomDomains              int64
		Subdomains                 int64
	}
	var domains domainScan
	if err := r.db.Model(&models.ResellerDomain{}).
		Select(`
			COUNT(1) AS domains_total,
			SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS domains_pending_review,
			SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS domains_active,
			SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS domains_disabled,
			SUM(CASE WHEN verification_status = ? THEN 1 ELSE 0 END) AS domains_pending_verification,
			SUM(CASE WHEN verification_status = ? THEN 1 ELSE 0 END) AS domains_verified,
			SUM(CASE WHEN type = ? THEN 1 ELSE 0 END) AS custom_domains,
			SUM(CASE WHEN type = ? THEN 1 ELSE 0 END) AS subdomains
		`,
			models.ResellerDomainStatusPendingReview,
			models.ResellerDomainStatusActive,
			models.ResellerDomainStatusDisabled,
			models.ResellerDomainVerificationPending,
			models.ResellerDomainVerificationVerified,
			models.ResellerDomainTypeCustom,
			models.ResellerDomainTypeSubdomain,
		).
		Scan(&domains).Error; err != nil {
		return err
	}
	out.DomainsTotal = domains.DomainsTotal
	out.DomainsPendingReview = domains.DomainsPendingReview
	out.DomainsActive = domains.DomainsActive
	out.DomainsDisabled = domains.DomainsDisabled
	out.DomainsPendingVerification = domains.DomainsPendingVerification
	out.DomainsVerified = domains.DomainsVerified
	out.CustomDomains = domains.CustomDomains
	out.Subdomains = domains.Subdomains

	if err := r.db.Model(&models.ResellerSiteConfig{}).Count(&out.SiteConfigsTotal).Error; err != nil {
		return err
	}
	return r.db.Model(&models.ResellerProfile{}).
		Joins("LEFT JOIN reseller_site_configs ON reseller_site_configs.reseller_id = reseller_profiles.id AND reseller_site_configs.deleted_at IS NULL").
		Where("reseller_profiles.status = ?", models.ResellerProfileStatusActive).
		Where("reseller_site_configs.id IS NULL").
		Count(&out.ActiveProfilesWithoutSiteConfig).Error
}

func (r *GormResellerOperationsRepository) scanOrderOverview(startAt, endAt time.Time, out *ResellerOperationsOrdersRow) error {
	paidStatuses := resellerOperationsPaidStatuses()
	type orderScan struct {
		OrdersTotal               int64
		PaidOrders                int64
		CompletedOrders           int64
		RefundedOrders            int64
		ActiveResellersWithOrders int64
	}
	var orders orderScan
	err := r.db.Model(&models.Order{}).
		Where("orders.reseller_id IS NOT NULL AND orders.parent_id IS NULL AND orders.created_at >= ? AND orders.created_at < ?", startAt, endAt).
		Select(`
			COUNT(1) AS orders_total,
			SUM(CASE WHEN orders.status IN ? THEN 1 ELSE 0 END) AS paid_orders,
			SUM(CASE WHEN orders.status = ? THEN 1 ELSE 0 END) AS completed_orders,
			SUM(CASE WHEN orders.status = ? THEN 1 ELSE 0 END) AS refunded_orders,
			COUNT(DISTINCT CASE WHEN orders.status IN ? THEN orders.reseller_id ELSE NULL END) AS active_resellers_with_orders
		`, paidStatuses, constants.OrderStatusCompleted, constants.OrderStatusRefunded, paidStatuses).
		Scan(&orders).Error
	if err != nil {
		return err
	}
	out.OrdersTotal = orders.OrdersTotal
	out.PaidOrders = orders.PaidOrders
	out.CompletedOrders = orders.CompletedOrders
	out.RefundedOrders = orders.RefundedOrders
	out.ActiveResellersWithOrders = orders.ActiveResellersWithOrders

	return r.db.Model(&models.ResellerOrderSnapshot{}).
		Where("profit_eligible = ? AND profit_block_reason IN ? AND created_at >= ? AND created_at < ?",
			false,
			[]string{"self_dealing_owner", "self_dealing_related_account"},
			startAt,
			endAt,
		).
		Count(&out.SelfDealingBlockedOrders).Error
}

func (r *GormResellerOperationsRepository) scanTopResellers(startAt, endAt time.Time) ([]ResellerOperationsTopResellerRow, error) {
	type topScan struct {
		ResellerID          uint
		UserID              uint
		Email               string
		DisplayName         string
		OrdersTotal         int64
		PaidOrders          int64
		ActiveDomains       int64
		SiteConfiguredCount int64
		LastOrderAt         sql.NullString
	}
	rows := []topScan{}
	err := r.db.Table("reseller_profiles").
		Select(`
			reseller_profiles.id AS reseller_id,
			reseller_profiles.user_id AS user_id,
			users.email AS email,
			users.display_name AS display_name,
			COUNT(orders.id) AS orders_total,
			SUM(CASE WHEN orders.status IN ? THEN 1 ELSE 0 END) AS paid_orders,
			COALESCE(domain_counts.active_domains, 0) AS active_domains,
			COUNT(DISTINCT reseller_site_configs.id) AS site_configured_count,
			MAX(orders.created_at) AS last_order_at
		`, resellerOperationsPaidStatuses()).
		Joins("JOIN orders ON orders.reseller_id = reseller_profiles.id AND orders.parent_id IS NULL AND orders.created_at >= ? AND orders.created_at < ? AND orders.deleted_at IS NULL", startAt, endAt).
		Joins("LEFT JOIN users ON users.id = reseller_profiles.user_id AND users.deleted_at IS NULL").
		Joins("LEFT JOIN (SELECT reseller_id, COUNT(1) AS active_domains FROM reseller_domains WHERE status = ? AND deleted_at IS NULL GROUP BY reseller_id) AS domain_counts ON domain_counts.reseller_id = reseller_profiles.id", models.ResellerDomainStatusActive).
		Joins("LEFT JOIN reseller_site_configs ON reseller_site_configs.reseller_id = reseller_profiles.id AND reseller_site_configs.deleted_at IS NULL").
		Where("reseller_profiles.deleted_at IS NULL").
		Group("reseller_profiles.id, reseller_profiles.user_id, users.email, users.display_name, domain_counts.active_domains").
		Order("paid_orders DESC, orders_total DESC, reseller_profiles.id DESC").
		Limit(10).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]ResellerOperationsTopResellerRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, ResellerOperationsTopResellerRow{
			ResellerID:     row.ResellerID,
			UserID:         row.UserID,
			Email:          row.Email,
			DisplayName:    row.DisplayName,
			OrdersTotal:    row.OrdersTotal,
			PaidOrders:     row.PaidOrders,
			ActiveDomains:  row.ActiveDomains,
			SiteConfigured: row.SiteConfiguredCount > 0,
			LastOrderAt:    resellerOperationsParseDBTime(row.LastOrderAt),
		})
	}
	return out, nil
}

func (r *GormResellerOperationsRepository) scanPeriodCurrencyRows(startAt, endAt time.Time) ([]ResellerOperationsPeriodCurrencyRow, error) {
	rowsByCurrency := map[string]*ResellerOperationsPeriodCurrencyRow{}
	paidStatuses := resellerOperationsPaidStatuses()

	type orderCurrencyScan struct {
		Currency    string
		OrdersTotal int64
		PaidOrders  int64
		GMVPaid     decimal.Decimal
	}
	orderRows := []orderCurrencyScan{}
	if err := r.db.Model(&models.Order{}).
		Select(`
			currency,
			COUNT(1) AS orders_total,
			SUM(CASE WHEN status IN ? THEN 1 ELSE 0 END) AS paid_orders,
			COALESCE(SUM(CASE WHEN status IN ? THEN total_amount ELSE 0 END), 0) AS gmv_paid
		`, paidStatuses, paidStatuses).
		Where("reseller_id IS NOT NULL AND parent_id IS NULL AND created_at >= ? AND created_at < ?", startAt, endAt).
		Group("currency").
		Scan(&orderRows).Error; err != nil {
		return nil, err
	}
	for _, row := range orderRows {
		target := resellerOperationsPeriodCurrencyTarget(rowsByCurrency, row.Currency)
		target.OrdersTotal = row.OrdersTotal
		target.PaidOrders = row.PaidOrders
		target.GMVPaid = row.GMVPaid.Round(2)
	}

	type ledgerCurrencyScan struct {
		Currency       string
		ProfitEarned   decimal.Decimal
		RefundDeducted decimal.Decimal
	}
	ledgerRows := []ledgerCurrencyScan{}
	if err := r.db.Model(&models.ResellerLedgerEntry{}).
		Select(`
			currency,
			COALESCE(SUM(CASE WHEN type = ? THEN amount ELSE 0 END), 0) AS profit_earned,
			ABS(COALESCE(SUM(CASE WHEN type = ? THEN amount ELSE 0 END), 0)) AS refund_deducted
		`, models.ResellerLedgerTypeOrderProfit, models.ResellerLedgerTypeRefundDeduct).
		Where("created_at >= ? AND created_at < ? AND status <> ?", startAt, endAt, models.ResellerLedgerStatusCanceled).
		Group("currency").
		Scan(&ledgerRows).Error; err != nil {
		return nil, err
	}
	for _, row := range ledgerRows {
		target := resellerOperationsPeriodCurrencyTarget(rowsByCurrency, row.Currency)
		target.ProfitEarned = row.ProfitEarned.Round(2)
		target.RefundDeducted = row.RefundDeducted.Round(2)
	}

	type withdrawCurrencyScan struct {
		Currency     string
		WithdrawPaid decimal.Decimal
	}
	withdrawRows := []withdrawCurrencyScan{}
	if err := r.db.Model(&models.ResellerWithdrawRequest{}).
		Select("currency, COALESCE(SUM(amount), 0) AS withdraw_paid").
		Where("status = ? AND processed_at IS NOT NULL AND processed_at >= ? AND processed_at < ?", models.ResellerWithdrawStatusPaid, startAt, endAt).
		Group("currency").
		Scan(&withdrawRows).Error; err != nil {
		return nil, err
	}
	for _, row := range withdrawRows {
		target := resellerOperationsPeriodCurrencyTarget(rowsByCurrency, row.Currency)
		target.WithdrawPaid = row.WithdrawPaid.Round(2)
	}

	return resellerOperationsSortedPeriodRows(rowsByCurrency), nil
}

func (r *GormResellerOperationsRepository) scanCurrentCurrencyRows() ([]ResellerOperationsCurrentCurrencyRow, error) {
	rowsByCurrency := map[string]*ResellerOperationsCurrentCurrencyRow{}

	type balanceCurrencyScan struct {
		Currency                string
		AvailableBalance        decimal.Decimal
		LockedBalance           decimal.Decimal
		NegativeBalance         decimal.Decimal
		NegativeBalanceAccounts int64
		FrozenBalanceAccounts   int64
	}
	balanceRows := []balanceCurrencyScan{}
	if err := r.db.Model(&models.ResellerBalanceAccount{}).
		Select(`
			currency,
			COALESCE(SUM(available_amount_cache), 0) AS available_balance,
			COALESCE(SUM(locked_amount_cache), 0) AS locked_balance,
			COALESCE(SUM(negative_amount_cache), 0) AS negative_balance,
			SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS negative_balance_accounts,
			SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS frozen_balance_accounts
		`, models.ResellerBalanceStatusNegativeBalance, models.ResellerBalanceStatusFrozenReview).
		Group("currency").
		Scan(&balanceRows).Error; err != nil {
		return nil, err
	}
	for _, row := range balanceRows {
		target := resellerOperationsCurrentCurrencyTarget(rowsByCurrency, row.Currency)
		target.AvailableBalance = row.AvailableBalance.Round(2)
		target.LockedBalance = row.LockedBalance.Round(2)
		target.NegativeBalance = row.NegativeBalance.Round(2)
		target.NegativeBalanceAccounts = row.NegativeBalanceAccounts
		target.FrozenBalanceAccounts = row.FrozenBalanceAccounts
	}

	type pendingWithdrawScan struct {
		Currency              string
		PendingWithdrawCount  int64
		PendingWithdrawAmount decimal.Decimal
	}
	withdrawRows := []pendingWithdrawScan{}
	if err := r.db.Model(&models.ResellerWithdrawRequest{}).
		Select("currency, COUNT(1) AS pending_withdraw_count, COALESCE(SUM(amount), 0) AS pending_withdraw_amount").
		Where("status = ?", models.ResellerWithdrawStatusPending).
		Group("currency").
		Scan(&withdrawRows).Error; err != nil {
		return nil, err
	}
	for _, row := range withdrawRows {
		target := resellerOperationsCurrentCurrencyTarget(rowsByCurrency, row.Currency)
		target.PendingWithdrawCount = row.PendingWithdrawCount
		target.PendingWithdrawAmount = row.PendingWithdrawAmount.Round(2)
	}

	return resellerOperationsSortedCurrentRows(rowsByCurrency), nil
}

func resellerOperationsPeriodCurrencyTarget(rows map[string]*ResellerOperationsPeriodCurrencyRow, currency string) *ResellerOperationsPeriodCurrencyRow {
	currency = resellerOperationsNormalizeCurrency(currency)
	if currency == "" {
		currency = "UNKNOWN"
	}
	if row, ok := rows[currency]; ok {
		return row
	}
	row := &ResellerOperationsPeriodCurrencyRow{
		Currency:       currency,
		GMVPaid:        decimal.Zero,
		ProfitEarned:   decimal.Zero,
		RefundDeducted: decimal.Zero,
		WithdrawPaid:   decimal.Zero,
	}
	rows[currency] = row
	return row
}

func resellerOperationsCurrentCurrencyTarget(rows map[string]*ResellerOperationsCurrentCurrencyRow, currency string) *ResellerOperationsCurrentCurrencyRow {
	currency = resellerOperationsNormalizeCurrency(currency)
	if currency == "" {
		currency = "UNKNOWN"
	}
	if row, ok := rows[currency]; ok {
		return row
	}
	row := &ResellerOperationsCurrentCurrencyRow{
		Currency:              currency,
		AvailableBalance:      decimal.Zero,
		LockedBalance:         decimal.Zero,
		NegativeBalance:       decimal.Zero,
		PendingWithdrawAmount: decimal.Zero,
	}
	rows[currency] = row
	return row
}

func resellerOperationsSortedPeriodRows(rows map[string]*ResellerOperationsPeriodCurrencyRow) []ResellerOperationsPeriodCurrencyRow {
	keys := make([]string, 0, len(rows))
	for key := range rows {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]ResellerOperationsPeriodCurrencyRow, 0, len(keys))
	for _, key := range keys {
		out = append(out, *rows[key])
	}
	return out
}

func resellerOperationsSortedCurrentRows(rows map[string]*ResellerOperationsCurrentCurrencyRow) []ResellerOperationsCurrentCurrencyRow {
	keys := make([]string, 0, len(rows))
	for key := range rows {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]ResellerOperationsCurrentCurrencyRow, 0, len(keys))
	for _, key := range keys {
		out = append(out, *rows[key])
	}
	return out
}

func resellerOperationsNormalizeCurrency(currency string) string {
	return strings.ToUpper(strings.TrimSpace(currency))
}

func resellerOperationsParseDBTime(value sql.NullString) *time.Time {
	if !value.Valid {
		return nil
	}
	raw := strings.TrimSpace(value.String)
	if raw == "" {
		return nil
	}
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return &parsed
		}
	}
	if parsed, err := time.ParseInLocation("2006-01-02 15:04:05.999999999", raw, time.UTC); err == nil {
		return &parsed
	}
	return nil
}
