package presenter

import (
	"time"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	"github.com/dujiao-next/internal/shared/jsonmap"
)

type ResellerProfileSummaryResp struct {
	ID               uint      `json:"id"`
	Status           string    `json:"status"`
	SettlementStatus string    `json:"settlement_status"`
	CreatedAt        time.Time `json:"created_at"`
}

type ResellerBalanceResp struct {
	ID              uint      `json:"id"`
	Currency        string    `json:"currency"`
	Status          string    `json:"status"`
	AvailableAmount string    `json:"available_amount"`
	LockedAmount    string    `json:"locked_amount"`
	NegativeAmount  string    `json:"negative_amount"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type ResellerLedgerResp struct {
	ID                uint       `json:"id"`
	OrderID           *uint      `json:"order_id,omitempty"`
	Type              string     `json:"type"`
	Amount            string     `json:"amount"`
	Currency          string     `json:"currency"`
	Status            string     `json:"status"`
	AvailableAt       *time.Time `json:"available_at,omitempty"`
	WithdrawRequestID *uint      `json:"withdraw_request_id,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}

type ResellerWithdrawResp struct {
	ID           uint       `json:"id"`
	Amount       string     `json:"amount"`
	Currency     string     `json:"currency"`
	Channel      string     `json:"channel"`
	Account      string     `json:"account"`
	Status       string     `json:"status"`
	RejectReason string     `json:"reject_reason,omitempty"`
	ProcessedAt  *time.Time `json:"processed_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

type ResellerDashboardResp struct {
	Opened                 bool                        `json:"opened"`
	Profile                *ResellerProfileSummaryResp `json:"profile,omitempty"`
	Balances               []ResellerBalanceResp       `json:"balances,omitempty"`
	WithdrawEnabled        bool                        `json:"withdraw_enabled"`
	WithdrawDisabledReason string                      `json:"withdraw_disabled_reason,omitempty"`
}

type ResellerManagementProfileResp struct {
	ID                   uint       `json:"id"`
	Status               string     `json:"status"`
	ApplyReason          string     `json:"apply_reason,omitempty"`
	RejectReason         string     `json:"reject_reason,omitempty"`
	DefaultMarkupPercent string     `json:"default_markup_percent"`
	MaxMarkupPercent     string     `json:"max_markup_percent"`
	SettlementStatus     string     `json:"settlement_status"`
	ReviewedAt           *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type ResellerDomainResp struct {
	ID                 uint       `json:"id"`
	Domain             string     `json:"domain"`
	Type               string     `json:"type"`
	VerificationToken  string     `json:"verification_token,omitempty"`
	VerificationStatus string     `json:"verification_status"`
	Status             string     `json:"status"`
	IsPrimary          bool       `json:"is_primary"`
	VerifiedAt         *time.Time `json:"verified_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type ResellerManagementSnapshotResp struct {
	Opened   bool                           `json:"opened"`
	CanApply bool                           `json:"can_apply"`
	Profile  *ResellerManagementProfileResp `json:"profile,omitempty"`
	Domains  []ResellerDomainResp           `json:"domains"`
}

type ResellerSiteConfigResp struct {
	ID           uint          `json:"id"`
	SiteName     string        `json:"site_name"`
	Logo         string        `json:"logo"`
	Favicon      string        `json:"favicon"`
	Announcement jsonmap.JSON  `json:"announcement"`
	Support      jsonmap.JSON  `json:"support"`
	SEO          jsonmap.JSON  `json:"seo"`
	FooterLinks  []interface{} `json:"footer_links"`
	NavConfig    jsonmap.JSON  `json:"nav_config"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

type ResellerSiteConfigSnapshotResp struct {
	Opened  bool                    `json:"opened"`
	CanEdit bool                    `json:"can_edit"`
	Config  *ResellerSiteConfigResp `json:"config,omitempty"`
}

type ResellerSiteConfigOwnerUserResp struct {
	ID          uint   `json:"id"`
	Email       string `json:"email,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
}

type ResellerSiteConfigProfileRefResp struct {
	ID               uint                             `json:"id"`
	UserID           uint                             `json:"user_id"`
	Status           string                           `json:"status,omitempty"`
	SettlementStatus string                           `json:"settlement_status,omitempty"`
	User             *ResellerSiteConfigOwnerUserResp `json:"user,omitempty"`
}

type AdminResellerSiteConfigResp struct {
	ID           uint                              `json:"id"`
	ResellerID   uint                              `json:"reseller_id"`
	SiteName     string                            `json:"site_name"`
	Logo         string                            `json:"logo"`
	Favicon      string                            `json:"favicon"`
	Announcement jsonmap.JSON                      `json:"announcement"`
	Support      jsonmap.JSON                      `json:"support"`
	SEO          jsonmap.JSON                      `json:"seo"`
	FooterLinks  []interface{}                     `json:"footer_links"`
	NavConfig    jsonmap.JSON                      `json:"nav_config"`
	Profile      *ResellerSiteConfigProfileRefResp `json:"profile,omitempty"`
	CreatedAt    time.Time                         `json:"created_at"`
	UpdatedAt    time.Time                         `json:"updated_at"`
}

type ResellerProductSettingResp struct {
	ID                   uint       `json:"id"`
	ProductID            uint       `json:"product_id"`
	SKUID                uint       `json:"sku_id"`
	IsListed             bool       `json:"is_listed"`
	PricingMode          string     `json:"pricing_mode"`
	MarkupPercent        string     `json:"markup_percent"`
	FixedMarkupAmount    string     `json:"fixed_markup_amount"`
	FixedPriceAmount     string     `json:"fixed_price_amount"`
	EffectivePriceAmount string     `json:"effective_price_amount,omitempty"`
	RuleSource           string     `json:"rule_source,omitempty"`
	SortOrder            int        `json:"sort_order"`
	UpdatedAt            *time.Time `json:"updated_at,omitempty"`
}

type ResellerProductSettingProductResp struct {
	ID                uint         `json:"id"`
	Slug              string       `json:"slug"`
	Title             jsonmap.JSON `json:"title"`
	PriceAmount       string       `json:"price_amount"`
	AgencyPriceAmount string       `json:"agency_price_amount,omitempty"`
	IsActive          bool         `json:"is_active"`
}

type ResellerProductSettingSKUResp struct {
	ID                uint                        `json:"id"`
	SKUCode           string                      `json:"sku_code"`
	SpecValues        jsonmap.JSON                `json:"spec_values"`
	BasePriceAmount   string                      `json:"base_price_amount"`
	MasterPriceAmount string                      `json:"master_price_amount,omitempty"`
	IsActive          bool                        `json:"is_active"`
	Setting           *ResellerProductSettingResp `json:"setting,omitempty"`
	EffectivePrice    string                      `json:"effective_price_amount,omitempty"`
}

type ResellerProductSettingDetailResp struct {
	Product        ResellerProductSettingProductResp `json:"product"`
	ProductSetting *ResellerProductSettingResp       `json:"product_setting,omitempty"`
	SKUs           []ResellerProductSettingSKUResp   `json:"skus"`
}

type ResellerProductSettingDTOInput struct {
	Product          productdomain.Product
	Settings         []resellerdomain.ProductSetting
	EffectiveBySKUID map[uint]string
	RuleBySKUID      map[uint]string
}

type AdminResellerProductSettingUserResp struct {
	ID          uint   `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

type AdminResellerProductSettingProfileResp struct {
	ID               uint                                 `json:"id"`
	UserID           uint                                 `json:"user_id"`
	Status           string                               `json:"status"`
	SettlementStatus string                               `json:"settlement_status"`
	User             *AdminResellerProductSettingUserResp `json:"user,omitempty"`
}

type AdminResellerProductSettingProductResp struct {
	ID          uint         `json:"id"`
	Slug        string       `json:"slug"`
	Title       jsonmap.JSON `json:"title"`
	PriceAmount string       `json:"price_amount"`
	IsActive    bool         `json:"is_active"`
}

type AdminResellerProductSettingResp struct {
	ID                uint                                    `json:"id"`
	ResellerID        uint                                    `json:"reseller_id"`
	ProductID         uint                                    `json:"product_id"`
	SKUID             uint                                    `json:"sku_id"`
	IsListed          bool                                    `json:"is_listed"`
	PricingMode       string                                  `json:"pricing_mode"`
	MarkupPercent     string                                  `json:"markup_percent"`
	FixedMarkupAmount string                                  `json:"fixed_markup_amount"`
	FixedPriceAmount  string                                  `json:"fixed_price_amount"`
	SortOrder         int                                     `json:"sort_order"`
	CreatedAt         time.Time                               `json:"created_at"`
	UpdatedAt         time.Time                               `json:"updated_at"`
	Profile           *AdminResellerProductSettingProfileResp `json:"profile,omitempty"`
	Product           *AdminResellerProductSettingProductResp `json:"product,omitempty"`
}

func NewResellerProfileSummaryResp(profile *resellerdomain.Profile) *ResellerProfileSummaryResp {
	if profile == nil {
		return nil
	}
	return &ResellerProfileSummaryResp{
		ID:               profile.ID,
		Status:           profile.Status,
		SettlementStatus: profile.SettlementStatus,
		CreatedAt:        profile.CreatedAt,
	}
}

func NewResellerManagementProfileResp(profile *resellerdomain.Profile) *ResellerManagementProfileResp {
	if profile == nil {
		return nil
	}
	return &ResellerManagementProfileResp{
		ID:                   profile.ID,
		Status:               profile.Status,
		ApplyReason:          profile.ApplyReason,
		RejectReason:         profile.RejectReason,
		DefaultMarkupPercent: profile.DefaultMarkupPercent.String(),
		MaxMarkupPercent:     profile.MaxMarkupPercent.String(),
		SettlementStatus:     profile.SettlementStatus,
		ReviewedAt:           profile.ReviewedAt,
		CreatedAt:            profile.CreatedAt,
		UpdatedAt:            profile.UpdatedAt,
	}
}

func NewResellerDomainResp(row *resellerdomain.Domain) ResellerDomainResp {
	if row == nil {
		return ResellerDomainResp{}
	}
	return ResellerDomainResp{
		ID:                 row.ID,
		Domain:             row.Domain,
		Type:               row.Type,
		VerificationToken:  row.VerificationToken,
		VerificationStatus: row.VerificationStatus,
		Status:             row.Status,
		IsPrimary:          row.IsPrimary,
		VerifiedAt:         row.VerifiedAt,
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
	}
}

func NewResellerDomainRespList(rows []resellerdomain.Domain) []ResellerDomainResp {
	result := make([]ResellerDomainResp, 0, len(rows))
	for i := range rows {
		result = append(result, NewResellerDomainResp(&rows[i]))
	}
	return result
}

func NewResellerManagementSnapshotResp(profile *resellerdomain.Profile, domains []resellerdomain.Domain, canApply bool) ResellerManagementSnapshotResp {
	if profile == nil {
		return ResellerManagementSnapshotResp{Opened: false, CanApply: canApply, Domains: []ResellerDomainResp{}}
	}
	return ResellerManagementSnapshotResp{
		Opened:   true,
		CanApply: canApply,
		Profile:  NewResellerManagementProfileResp(profile),
		Domains:  NewResellerDomainRespList(domains),
	}
}

func NewResellerSiteConfigResp(row *resellerdomain.SiteConfig) *ResellerSiteConfigResp {
	if row == nil {
		return nil
	}
	return &ResellerSiteConfigResp{
		ID:           row.ID,
		SiteName:     row.SiteName,
		Logo:         row.Logo,
		Favicon:      row.Favicon,
		Announcement: row.AnnouncementJSON,
		Support:      row.SupportJSON,
		SEO:          row.SEOJSON,
		FooterLinks:  resellerFooterLinksFromEnvelope(row.FooterLinksJSON),
		NavConfig:    row.NavConfigJSON,
		UpdatedAt:    row.UpdatedAt,
	}
}

func resellerFooterLinksFromEnvelope(raw jsonmap.JSON) []interface{} {
	if raw == nil {
		return make([]interface{}, 0)
	}
	if items, ok := raw["items"].([]interface{}); ok {
		return items
	}
	if typed, ok := raw["items"].([]jsonmap.JSON); ok {
		out := make([]interface{}, 0, len(typed))
		for _, item := range typed {
			out = append(out, item)
		}
		return out
	}
	return make([]interface{}, 0)
}

func NewResellerSiteConfigSnapshotResp(profile *resellerdomain.Profile, row *resellerdomain.SiteConfig, canEdit bool) ResellerSiteConfigSnapshotResp {
	return ResellerSiteConfigSnapshotResp{
		Opened:  profile != nil,
		CanEdit: canEdit,
		Config:  NewResellerSiteConfigResp(row),
	}
}

func NewAdminResellerSiteConfigResp(row *resellerdomain.SiteConfig) AdminResellerSiteConfigResp {
	if row == nil {
		return AdminResellerSiteConfigResp{FooterLinks: make([]interface{}, 0)}
	}
	var profile *ResellerSiteConfigProfileRefResp
	if row.Profile != nil {
		profile = &ResellerSiteConfigProfileRefResp{
			ID:               row.Profile.ID,
			UserID:           row.Profile.UserID,
			Status:           row.Profile.Status,
			SettlementStatus: row.Profile.SettlementStatus,
		}
		if row.Profile.User != nil {
			profile.User = &ResellerSiteConfigOwnerUserResp{
				ID:          row.Profile.User.ID,
				Email:       row.Profile.User.Email,
				DisplayName: row.Profile.User.DisplayName,
			}
		}
	}
	return AdminResellerSiteConfigResp{
		ID:           row.ID,
		ResellerID:   row.ResellerID,
		SiteName:     row.SiteName,
		Logo:         row.Logo,
		Favicon:      row.Favicon,
		Announcement: row.AnnouncementJSON,
		Support:      row.SupportJSON,
		SEO:          row.SEOJSON,
		FooterLinks:  resellerFooterLinksFromEnvelope(row.FooterLinksJSON),
		NavConfig:    row.NavConfigJSON,
		Profile:      profile,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
}

func NewAdminResellerSiteConfigRespList(rows []resellerdomain.SiteConfig) []AdminResellerSiteConfigResp {
	result := make([]AdminResellerSiteConfigResp, 0, len(rows))
	for i := range rows {
		result = append(result, NewAdminResellerSiteConfigResp(&rows[i]))
	}
	return result
}

func NewResellerProductSettingDetailResp(input ResellerProductSettingDTOInput) ResellerProductSettingDetailResp {
	productSetting := findResellerProductSetting(input.Settings, 0)
	agencyPrice := ""
	if input.Product.AgencyPriceAmount.IsPositive() {
		agencyPrice = input.Product.AgencyPriceAmount.String()
	}
	resp := ResellerProductSettingDetailResp{
		Product: ResellerProductSettingProductResp{
			ID:                input.Product.ID,
			Slug:              input.Product.Slug,
			Title:             input.Product.TitleJSON,
			PriceAmount:       input.Product.PriceAmount.String(),
			AgencyPriceAmount: agencyPrice,
			IsActive:          input.Product.IsActive,
		},
		SKUs: make([]ResellerProductSettingSKUResp, 0, len(input.Product.SKUs)),
	}
	if productSetting != nil {
		resp.ProductSetting = newResellerProductSettingResp(*productSetting, input.EffectiveBySKUID[0], input.RuleBySKUID[0])
	}
	for i := range input.Product.SKUs {
		sku := input.Product.SKUs[i]
		setting := findResellerProductSetting(input.Settings, sku.ID)
		var settingResp *ResellerProductSettingResp
		if setting != nil {
			settingResp = newResellerProductSettingResp(*setting, input.EffectiveBySKUID[sku.ID], input.RuleBySKUID[sku.ID])
		}
		basePrice := sku.PriceAmount.String()
		if sku.AgencyPriceAmount.IsPositive() {
			basePrice = sku.AgencyPriceAmount.String()
		}
		resp.SKUs = append(resp.SKUs, ResellerProductSettingSKUResp{
			ID:                sku.ID,
			SKUCode:           sku.SKUCode,
			SpecValues:        sku.SpecValuesJSON,
			BasePriceAmount:   basePrice,
			MasterPriceAmount: sku.PriceAmount.String(),
			IsActive:          sku.IsActive,
			Setting:           settingResp,
			EffectivePrice:    input.EffectiveBySKUID[sku.ID],
		})
	}
	return resp
}

func NewResellerProductSettingListResp(rows []ResellerProductSettingDTOInput) []ResellerProductSettingDetailResp {
	out := make([]ResellerProductSettingDetailResp, 0, len(rows))
	for i := range rows {
		out = append(out, NewResellerProductSettingDetailResp(rows[i]))
	}
	return out
}

// ResellerProductSettingPreviewItemResp 单个商品级（sku_id=0）或 SKU 级规则的预览结果。
type ResellerProductSettingPreviewItemResp struct {
	SKUID                uint   `json:"sku_id"`
	IsListed             bool   `json:"is_listed"`
	BasePriceAmount      string `json:"base_price_amount"`
	EffectivePriceAmount string `json:"effective_price_amount"`
	Valid                bool   `json:"valid"`
	ErrorCode            string `json:"error_code,omitempty"`
}

type ResellerProductSettingPreviewResp struct {
	Items []ResellerProductSettingPreviewItemResp `json:"items"`
}

// ResellerProductSettingPreviewInput 由 handler 从 service 结果映射而来（金额已格式化为两位小数字符串）。
type ResellerProductSettingPreviewInput struct {
	SKUID          uint
	IsListed       bool
	BasePrice      string
	EffectivePrice string
	Valid          bool
	ErrorCode      string
}

func NewResellerProductSettingPreviewResp(items []ResellerProductSettingPreviewInput) ResellerProductSettingPreviewResp {
	out := ResellerProductSettingPreviewResp{Items: make([]ResellerProductSettingPreviewItemResp, 0, len(items))}
	for _, item := range items {
		out.Items = append(out.Items, ResellerProductSettingPreviewItemResp{
			SKUID:                item.SKUID,
			IsListed:             item.IsListed,
			BasePriceAmount:      item.BasePrice,
			EffectivePriceAmount: item.EffectivePrice,
			Valid:                item.Valid,
			ErrorCode:            item.ErrorCode,
		})
	}
	return out
}

func newResellerProductSettingResp(setting resellerdomain.ProductSetting, effectivePrice string, ruleSource string) *ResellerProductSettingResp {
	updatedAt := setting.UpdatedAt
	return &ResellerProductSettingResp{
		ID:                   setting.ID,
		ProductID:            setting.ProductID,
		SKUID:                setting.SKUID,
		IsListed:             setting.IsListed,
		PricingMode:          setting.PricingMode,
		MarkupPercent:        setting.MarkupPercent.String(),
		FixedMarkupAmount:    setting.FixedMarkupAmount.String(),
		FixedPriceAmount:     setting.FixedPriceAmount.String(),
		EffectivePriceAmount: effectivePrice,
		RuleSource:           ruleSource,
		SortOrder:            setting.SortOrder,
		UpdatedAt:            &updatedAt,
	}
}

func findResellerProductSetting(settings []resellerdomain.ProductSetting, skuID uint) *resellerdomain.ProductSetting {
	for i := range settings {
		if settings[i].SKUID == skuID {
			return &settings[i]
		}
	}
	return nil
}

func NewAdminResellerProductSettingResp(row resellerdomain.ProductSetting) AdminResellerProductSettingResp {
	resp := AdminResellerProductSettingResp{
		ID:                row.ID,
		ResellerID:        row.ResellerID,
		ProductID:         row.ProductID,
		SKUID:             row.SKUID,
		IsListed:          row.IsListed,
		PricingMode:       row.PricingMode,
		MarkupPercent:     row.MarkupPercent.String(),
		FixedMarkupAmount: row.FixedMarkupAmount.String(),
		FixedPriceAmount:  row.FixedPriceAmount.String(),
		SortOrder:         row.SortOrder,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}
	if row.Profile != nil {
		profile := &AdminResellerProductSettingProfileResp{
			ID:               row.Profile.ID,
			UserID:           row.Profile.UserID,
			Status:           row.Profile.Status,
			SettlementStatus: row.Profile.SettlementStatus,
		}
		if row.Profile.User != nil {
			profile.User = &AdminResellerProductSettingUserResp{
				ID:          row.Profile.User.ID,
				Email:       row.Profile.User.Email,
				DisplayName: row.Profile.User.DisplayName,
			}
		}
		resp.Profile = profile
	}
	if row.Product != nil {
		resp.Product = &AdminResellerProductSettingProductResp{
			ID:          row.Product.ID,
			Slug:        row.Product.Slug,
			Title:       row.Product.TitleJSON,
			PriceAmount: row.Product.PriceAmount.String(),
			IsActive:    row.Product.IsActive,
		}
	}
	return resp
}

func NewAdminResellerProductSettingRespList(rows []resellerdomain.ProductSetting) []AdminResellerProductSettingResp {
	out := make([]AdminResellerProductSettingResp, 0, len(rows))
	for i := range rows {
		out = append(out, NewAdminResellerProductSettingResp(rows[i]))
	}
	return out
}

func NewResellerBalanceResp(row *resellerdomain.BalanceAccount) ResellerBalanceResp {
	if row == nil {
		return ResellerBalanceResp{}
	}
	return ResellerBalanceResp{
		ID:              row.ID,
		Currency:        row.Currency,
		Status:          row.Status,
		AvailableAmount: row.AvailableAmountCache.String(),
		LockedAmount:    row.LockedAmountCache.String(),
		NegativeAmount:  row.NegativeAmountCache.String(),
		UpdatedAt:       row.UpdatedAt,
	}
}

func NewResellerBalanceRespList(rows []resellerdomain.BalanceAccount) []ResellerBalanceResp {
	result := make([]ResellerBalanceResp, 0, len(rows))
	for i := range rows {
		result = append(result, NewResellerBalanceResp(&rows[i]))
	}
	return result
}

func NewResellerLedgerResp(row *resellerdomain.LedgerEntry) ResellerLedgerResp {
	if row == nil {
		return ResellerLedgerResp{}
	}
	return ResellerLedgerResp{
		ID:                row.ID,
		OrderID:           row.OrderID,
		Type:              row.Type,
		Amount:            row.Amount.String(),
		Currency:          row.Currency,
		Status:            row.Status,
		AvailableAt:       row.AvailableAt,
		WithdrawRequestID: row.WithdrawRequestID,
		CreatedAt:         row.CreatedAt,
	}
}

func NewResellerLedgerRespList(rows []resellerdomain.LedgerEntry) []ResellerLedgerResp {
	result := make([]ResellerLedgerResp, 0, len(rows))
	for i := range rows {
		result = append(result, NewResellerLedgerResp(&rows[i]))
	}
	return result
}

func NewResellerWithdrawResp(row *resellerdomain.WithdrawRequest) ResellerWithdrawResp {
	if row == nil {
		return ResellerWithdrawResp{}
	}
	return ResellerWithdrawResp{
		ID:           row.ID,
		Amount:       row.Amount.String(),
		Currency:     row.Currency,
		Channel:      row.Channel,
		Account:      row.Account,
		Status:       row.Status,
		RejectReason: row.RejectReason,
		ProcessedAt:  row.ProcessedAt,
		CreatedAt:    row.CreatedAt,
	}
}

func NewResellerWithdrawRespList(rows []resellerdomain.WithdrawRequest) []ResellerWithdrawResp {
	result := make([]ResellerWithdrawResp, 0, len(rows))
	for i := range rows {
		result = append(result, NewResellerWithdrawResp(&rows[i]))
	}
	return result
}

func NewResellerDashboardResp(opened bool, profile *resellerdomain.Profile, balances []resellerdomain.BalanceAccount, withdrawEnabled bool, withdrawDisabledReason string) ResellerDashboardResp {
	if !opened {
		return ResellerDashboardResp{Opened: false}
	}
	return ResellerDashboardResp{
		Opened:                 true,
		Profile:                NewResellerProfileSummaryResp(profile),
		Balances:               NewResellerBalanceRespList(balances),
		WithdrawEnabled:        withdrawEnabled,
		WithdrawDisabledReason: withdrawDisabledReason,
	}
}
