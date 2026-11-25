package application

import (
	"math"
	"strings"

	affiliatecontract "github.com/dujiao-next/internal/modules/affiliate/contract"
	affiliatedomain "github.com/dujiao-next/internal/modules/affiliate/domain"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// GetUserDashboard 获取用户返利中心数据
func (s *Service) GetUserDashboard(userID uint) (Dashboard, error) {
	dashboard := Dashboard{
		Opened:              false,
		PendingCommission:   money.FromDecimal(decimal.Zero),
		AvailableCommission: money.FromDecimal(decimal.Zero),
		WithdrawnCommission: money.FromDecimal(decimal.Zero),
	}
	if userID == 0 || s.repo == nil {
		return dashboard, nil
	}
	profile, err := s.repo.GetProfileByUserID(userID)
	if err != nil {
		return dashboard, err
	}
	if profile == nil {
		return dashboard, nil
	}

	stats, err := s.buildProfileStats(profile.ID)
	if err != nil {
		return dashboard, err
	}
	dashboard.Opened = true
	dashboard.AffiliateCode = profile.AffiliateCode
	dashboard.Status = profile.Status
	dashboard.PromotionPath = "/?aff=" + profile.AffiliateCode
	dashboard.ClickCount = stats.ClickCount
	dashboard.ValidOrderCount = stats.ValidOrderCount
	dashboard.ConversionRate = stats.ConversionRate
	dashboard.PendingCommission = stats.PendingCommission
	dashboard.AvailableCommission = stats.AvailableCommission
	dashboard.WithdrawnCommission = stats.WithdrawnCommission
	return dashboard, nil
}

// GetProductRates 返回当前推广员对某个商品（及其各规格）生效的返利比例。
//
// 比例与结算共用 ResolveAffiliateRate，保证“页面展示的比例”与“实际发放的比例”不会出现两套口径。
// 只对已开通且未停用的推广员开放：比例反映站点的毛利结构，不能暴露给普通访客。
func (s *Service) GetProductRates(userID, productID uint) (ProductRates, error) {
	result := ProductRates{SKUs: []SKURate{}}
	if userID == 0 || productID == 0 || s.repo == nil || s.productRepo == nil {
		return result, ErrNotOpened
	}
	setting, err := s.settings.GetAffiliateSetting()
	if err != nil {
		return result, err
	}
	if !setting.Enabled {
		return result, ErrDisabled
	}
	profile, err := s.repo.GetProfileByUserID(userID)
	if err != nil {
		return result, err
	}
	if profile == nil || strings.TrimSpace(profile.Status) != constants.AffiliateProfileStatusActive {
		return result, ErrNotOpened
	}
	products, err := s.productRepo.ListByIDs([]uint{productID})
	if err != nil {
		return result, err
	}
	if len(products) == 0 || !products[0].IsActive {
		return result, ErrNotFound
	}
	product := products[0]
	globalRate := decimal.NewFromFloat(setting.CommissionRate).Round(2)

	productRate := ResolveAffiliateRate(nil, product.AffiliateRatePercent, globalRate)
	result.RatePercent = money.FromDecimal(productRate)
	if s.skuRepo == nil {
		return result, nil
	}
	skus, err := s.skuRepo.ListByProduct(productID, true)
	if err != nil {
		return result, err
	}
	for _, sku := range skus {
		rate := ResolveAffiliateRate(sku.AffiliateRatePercent, product.AffiliateRatePercent, globalRate)
		result.SKUs = append(result.SKUs, SKURate{SKUID: sku.ID, RatePercent: money.FromDecimal(rate)})
	}
	return result, nil
}

// ListUserCommissions 查询用户佣金记录
func (s *Service) ListUserCommissions(userID uint, page, pageSize int, status string) ([]affiliatedomain.Commission, int64, error) {
	if userID == 0 || s.repo == nil {
		return []affiliatedomain.Commission{}, 0, nil
	}
	profile, err := s.repo.GetProfileByUserID(userID)
	if err != nil {
		return nil, 0, err
	}
	if profile == nil {
		return []affiliatedomain.Commission{}, 0, nil
	}
	return s.repo.ListCommissions(affiliatecontract.CommissionListFilter{
		Page:               page,
		PageSize:           pageSize,
		AffiliateProfileID: profile.ID,
		Status:             strings.TrimSpace(status),
	})
}

// ListUserWithdraws 查询用户提现记录
func (s *Service) ListUserWithdraws(userID uint, page, pageSize int, status string) ([]affiliatedomain.WithdrawRequest, int64, error) {
	if userID == 0 || s.repo == nil {
		return []affiliatedomain.WithdrawRequest{}, 0, nil
	}
	profile, err := s.repo.GetProfileByUserID(userID)
	if err != nil {
		return nil, 0, err
	}
	if profile == nil {
		return []affiliatedomain.WithdrawRequest{}, 0, nil
	}
	return s.repo.ListWithdraws(affiliatecontract.WithdrawListFilter{
		Page:               page,
		PageSize:           pageSize,
		AffiliateProfileID: profile.ID,
		Status:             strings.TrimSpace(status),
	})
}

// ListAdminUsers 后台查询推广用户列表
func (s *Service) ListAdminUsers(filter AdminProfileListFilter) ([]AdminUserItem, int64, error) {
	if s.repo == nil {
		return []AdminUserItem{}, 0, nil
	}
	rows, total, err := s.repo.ListProfiles(affiliatecontract.ProfileListFilter{
		Page:     filter.Page,
		PageSize: filter.PageSize,
		UserID:   filter.UserID,
		Status:   strings.TrimSpace(filter.Status),
		Code:     strings.TrimSpace(filter.Code),
		Keyword:  strings.TrimSpace(filter.Keyword),
	})
	if err != nil {
		return nil, 0, err
	}
	profileIDs := make([]uint, 0, len(rows))
	for _, row := range rows {
		if row.ID == 0 {
			continue
		}
		profileIDs = append(profileIDs, row.ID)
	}
	statsMap, err := s.repo.GetProfileStatsBatch(profileIDs)
	if err != nil {
		return nil, 0, err
	}
	result := make([]AdminUserItem, 0, len(rows))
	for _, row := range rows {
		agg := statsMap[row.ID]
		stats := Stats{
			ClickCount:          agg.ClickCount,
			ValidOrderCount:     agg.ValidOrderCount,
			ConversionRate:      calcAffiliateConversion(agg.ValidOrderCount, agg.ClickCount),
			PendingCommission:   money.FromDecimal(agg.PendingCommission.Round(2)),
			AvailableCommission: money.FromDecimal(agg.AvailableCommission.Round(2)),
			WithdrawnCommission: money.FromDecimal(agg.WithdrawnCommission.Round(2)),
		}
		result = append(result, AdminUserItem{
			Profile: row,
			Stats:   stats,
		})
	}
	return result, total, nil
}

// ListAdminCommissions 后台查询佣金记录
func (s *Service) ListAdminCommissions(filter AdminCommissionListFilter) ([]affiliatedomain.Commission, int64, error) {
	if s.repo == nil {
		return []affiliatedomain.Commission{}, 0, nil
	}
	return s.repo.ListCommissions(affiliatecontract.CommissionListFilter{
		Page:               filter.Page,
		PageSize:           filter.PageSize,
		AffiliateProfileID: filter.AffiliateProfileID,
		OrderNo:            strings.TrimSpace(filter.OrderNo),
		Status:             strings.TrimSpace(filter.Status),
		Keyword:            strings.TrimSpace(filter.Keyword),
	})
}

// ListAdminWithdraws 后台查询提现申请
func (s *Service) ListAdminWithdraws(filter AdminWithdrawListFilter) ([]affiliatedomain.WithdrawRequest, int64, error) {
	if s.repo == nil {
		return []affiliatedomain.WithdrawRequest{}, 0, nil
	}
	return s.repo.ListWithdraws(affiliatecontract.WithdrawListFilter{
		Page:               filter.Page,
		PageSize:           filter.PageSize,
		AffiliateProfileID: filter.AffiliateProfileID,
		Status:             strings.TrimSpace(filter.Status),
		Keyword:            strings.TrimSpace(filter.Keyword),
	})
}

func (s *Service) buildProfileStats(profileID uint) (Stats, error) {
	stats := Stats{
		PendingCommission:   money.FromDecimal(decimal.Zero),
		AvailableCommission: money.FromDecimal(decimal.Zero),
		WithdrawnCommission: money.FromDecimal(decimal.Zero),
	}
	if profileID == 0 || s.repo == nil {
		return stats, nil
	}
	clickCount, err := s.repo.CountClicksByProfile(profileID)
	if err != nil {
		return stats, err
	}
	validOrders, err := s.repo.CountValidOrdersByProfile(profileID)
	if err != nil {
		return stats, err
	}
	pendingAmount, err := s.repo.SumCommissionByProfile(profileID, []string{
		constants.AffiliateCommissionStatusPendingConfirm,
	}, false)
	if err != nil {
		return stats, err
	}
	availableAmount, err := s.repo.SumCommissionByProfile(profileID, []string{
		constants.AffiliateCommissionStatusAvailable,
	}, true)
	if err != nil {
		return stats, err
	}
	withdrawnAmount, err := s.repo.SumCommissionByProfile(profileID, []string{
		constants.AffiliateCommissionStatusWithdrawn,
	}, false)
	if err != nil {
		return stats, err
	}

	stats.ClickCount = clickCount
	stats.ValidOrderCount = validOrders
	stats.ConversionRate = calcAffiliateConversion(validOrders, clickCount)
	stats.PendingCommission = money.FromDecimal(pendingAmount)
	stats.AvailableCommission = money.FromDecimal(availableAmount)
	stats.WithdrawnCommission = money.FromDecimal(withdrawnAmount)
	return stats, nil
}

func calcAffiliateConversion(validOrders, clicks int64) float64 {
	if clicks <= 0 || validOrders <= 0 {
		return 0
	}
	value := (float64(validOrders) / float64(clicks)) * 100
	return math.Round(value*100) / 100
}
