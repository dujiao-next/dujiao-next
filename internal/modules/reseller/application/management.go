package application

import (
	"context"
	"regexp"
	"strings"
	"time"

	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"

	"github.com/dujiao-next/internal/cache"
	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

type ManagementService struct {
	store resellercontract.ManagementStore
	cfg   config.ResellerConfig
}

type ResellerApplyInput struct {
	Reason string
}

type ResellerApproveInput struct {
	DefaultMarkupPercent decimal.Decimal
	MaxMarkupPercent     decimal.Decimal
}

type ResellerProfileUpdateInput struct {
	DefaultMarkupPercent decimal.Decimal
	MaxMarkupPercent     decimal.Decimal
	SettlementStatus     string
	Reason               string
}

type ResellerSystemDomainInput struct {
	Subdomain string
}

type ResellerApproveResult struct {
	Profile      *resellerdomain.Profile
	SystemDomain *resellerdomain.Domain
}

func NewManagementService(store resellercontract.ManagementStore, cfg config.ResellerConfig) *ManagementService {
	return &ManagementService{store: store, cfg: cfg}
}

func (s *ManagementService) GetUserManagementSnapshot(userID uint) (*resellerdomain.Profile, []resellerdomain.Domain, bool, error) {
	if s == nil || s.store == nil || userID == 0 {
		return nil, []resellerdomain.Domain{}, false, nil
	}
	profile, err := s.store.GetProfileByUserID(userID)
	if err != nil {
		return nil, nil, false, err
	}
	if profile == nil {
		return nil, []resellerdomain.Domain{}, s.cfg.Enabled && s.cfg.SelfApplyEnabled, nil
	}
	domains, err := s.store.ListDomainsByResellerID(profile.ID)
	if err != nil {
		return nil, nil, false, err
	}
	canApply := profile.Status == resellerdomain.ProfileStatusRejected && s.cfg.Enabled && s.cfg.SelfApplyEnabled
	return profile, domains, canApply, nil
}

// Matches the precision of the decimal(10,2) profile column.
var defaultMarkupFormat = regexp.MustCompile(`^[0-9]{1,8}(\.[0-9]{1,2})?$`)

func (s *ManagementService) UpdateUserDefaultMarkup(userID uint, value string) (*resellerdomain.Profile, error) {
	if s == nil || s.store == nil || !s.cfg.Enabled || userID == 0 {
		return nil, resellercontract.ErrNotOpened
	}
	value = strings.TrimSpace(value)
	if !defaultMarkupFormat.MatchString(value) {
		return nil, resellercontract.ErrDefaultMarkupInvalid
	}
	percent, err := decimal.NewFromString(value)
	if err != nil {
		return nil, resellercontract.ErrDefaultMarkupInvalid
	}

	var profile *resellerdomain.Profile
	err = s.store.WithinManagementTransaction(func(store resellercontract.ManagementStore) error {
		updated, err := store.UpdateActiveProfileDefaultMarkup(userID, money.FromDecimal(percent))
		if err != nil {
			return err
		}
		profile, err = store.GetProfileByUserID(userID)
		if err != nil {
			return err
		}
		if profile == nil {
			return resellercontract.ErrNotOpened
		}
		if profile.Status != resellerdomain.ProfileStatusActive {
			return resellercontract.ErrProfileInactive
		}
		if !updated {
			return resellercontract.ErrMarkupExceeded
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return profile, nil
}

func (s *ManagementService) ApplyUserReseller(userID uint, input ResellerApplyInput) (*resellerdomain.Profile, error) {
	if s == nil || s.store == nil || userID == 0 {
		return nil, productcontract.ErrNotFound
	}
	if !s.cfg.Enabled || !s.cfg.SelfApplyEnabled {
		return nil, resellercontract.ErrApplyDisabled
	}
	reason := strings.TrimSpace(input.Reason)
	existing, err := s.store.GetProfileByUserID(userID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		profile := &resellerdomain.Profile{
			UserID:           userID,
			Status:           resellerdomain.ProfileStatusPendingReview,
			ApplyReason:      reason,
			SettlementStatus: resellerdomain.SettlementStatusNormal,
		}
		if err := s.store.CreateProfile(profile); err != nil {
			return nil, err
		}
		return s.store.GetProfileByID(profile.ID)
	}
	switch existing.Status {
	case resellerdomain.ProfileStatusRejected:
		existing.Status = resellerdomain.ProfileStatusPendingReview
		existing.ApplyReason = reason
		existing.RejectReason = ""
		existing.ReviewedBy = nil
		existing.ReviewedAt = nil
		if err := s.store.UpdateProfile(existing); err != nil {
			return nil, err
		}
		return s.store.GetProfileByID(existing.ID)
	case resellerdomain.ProfileStatusPendingReview, resellerdomain.ProfileStatusActive:
		return existing, nil
	case resellerdomain.ProfileStatusDisabled:
		return nil, resellercontract.ErrProfileInactive
	default:
		return nil, resellercontract.ErrProfileStatusInvalid
	}
}

func (s *ManagementService) ApproveProfile(ctx context.Context, adminID, profileID uint, input ResellerApproveInput) (*ResellerApproveResult, error) {
	if s == nil || s.store == nil || adminID == 0 || profileID == 0 {
		return nil, productcontract.ErrNotFound
	}
	if err := validateResellerProfileMarkup(input.DefaultMarkupPercent, input.MaxMarkupPercent); err != nil {
		return nil, err
	}
	var result *ResellerApproveResult
	err := s.store.WithinManagementTransaction(func(repoTx resellercontract.ManagementStore) error {
		profile, err := repoTx.GetProfileByID(profileID)
		if err != nil {
			return err
		}
		if profile == nil {
			return productcontract.ErrNotFound
		}
		if profile.Status != resellerdomain.ProfileStatusPendingReview && profile.Status != resellerdomain.ProfileStatusRejected {
			return resellercontract.ErrProfileStatusInvalid
		}
		now := time.Now()
		profile.Status = resellerdomain.ProfileStatusActive
		profile.RejectReason = ""
		profile.DefaultMarkupPercent = money.FromDecimal(input.DefaultMarkupPercent)
		profile.MaxMarkupPercent = money.FromDecimal(input.MaxMarkupPercent)
		profile.SettlementStatus = resellerdomain.SettlementStatusNormal
		profile.ReviewedBy = &adminID
		profile.ReviewedAt = &now
		if err := repoTx.UpdateProfile(profile); err != nil {
			return err
		}
		result = &ResellerApproveResult{Profile: profile}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if result != nil && result.SystemDomain != nil {
		_ = cache.DelResellerDomain(ctx, result.SystemDomain.Domain)
	}
	return result, nil
}

func (s *ManagementService) AssignSystemSubdomain(ctx context.Context, adminID, profileID uint, input ResellerSystemDomainInput) (*resellerdomain.Domain, error) {
	if s == nil || s.store == nil || adminID == 0 || profileID == 0 {
		return nil, productcontract.ErrNotFound
	}
	base := resellercontract.NormalizeHost(s.cfg.SubdomainBase)
	if base == "" {
		return nil, resellercontract.ErrSubdomainBaseMissing
	}
	nextDomain, err := normalizeAndValidateSystemSubdomain(input.Subdomain, base, s.cfg)
	if err != nil {
		return nil, err
	}
	cacheDomains := make([]string, 0, 3)
	var updatedID uint
	err = s.store.WithinManagementTransaction(func(repoTx resellercontract.ManagementStore) error {
		profile, err := repoTx.GetProfileByID(profileID)
		if err != nil {
			return err
		}
		if profile == nil {
			return productcontract.ErrNotFound
		}
		existingHost, err := repoTx.FindDomainByHost(nextDomain)
		if err != nil {
			return err
		}
		domains, err := repoTx.ListDomainsByResellerID(profile.ID)
		if err != nil {
			return err
		}
		var systemDomain *resellerdomain.Domain
		hasPrimary := false
		for i := range domains {
			domain := domains[i]
			if domain.IsPrimary {
				hasPrimary = true
			}
			if domain.Type == resellerdomain.DomainTypeSubdomain && systemDomain == nil {
				copyDomain := domain
				systemDomain = &copyDomain
			}
		}
		if existingHost != nil && existingHost.ResellerID != profile.ID {
			return resellercontract.ErrDomainConflict
		}
		if existingHost != nil && systemDomain != nil && existingHost.ID != systemDomain.ID {
			return resellercontract.ErrDomainConflict
		}
		now := time.Now()
		shouldBePrimary := !hasPrimary || (systemDomain != nil && systemDomain.IsPrimary)
		if systemDomain == nil {
			row, err := repoTx.UpsertDomain(resellerdomain.Domain{
				ResellerID:         profile.ID,
				Domain:             nextDomain,
				Type:               resellerdomain.DomainTypeSubdomain,
				VerificationStatus: resellerdomain.DomainVerificationVerified,
				Status:             resellerdomain.DomainStatusActive,
				IsPrimary:          shouldBePrimary,
				VerifiedAt:         &now,
			})
			if err != nil {
				if strings.Contains(strings.ToLower(err.Error()), "already exists") ||
					strings.Contains(strings.ToLower(err.Error()), "unique") ||
					strings.Contains(strings.ToLower(err.Error()), "duplicate") {
					return resellercontract.ErrDomainConflict
				}
				return err
			}
			updatedID = row.ID
			cacheDomains = append(cacheDomains, row.Domain)
			return nil
		}
		if systemDomain.Domain != "" {
			cacheDomains = append(cacheDomains, systemDomain.Domain)
		}
		systemDomain.Domain = nextDomain
		systemDomain.Type = resellerdomain.DomainTypeSubdomain
		systemDomain.VerificationToken = ""
		systemDomain.VerificationStatus = resellerdomain.DomainVerificationVerified
		systemDomain.Status = resellerdomain.DomainStatusActive
		systemDomain.IsPrimary = shouldBePrimary
		systemDomain.VerifiedAt = &now
		if err := repoTx.UpdateDomain(systemDomain); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "already exists") ||
				strings.Contains(strings.ToLower(err.Error()), "unique") ||
				strings.Contains(strings.ToLower(err.Error()), "duplicate") {
				return resellercontract.ErrDomainConflict
			}
			return err
		}
		updatedID = systemDomain.ID
		cacheDomains = append(cacheDomains, systemDomain.Domain)
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, domain := range cacheDomains {
		if domain != "" {
			_ = cache.DelResellerDomain(ctx, domain)
		}
	}
	return s.store.GetDomainByID(updatedID)
}

func (s *ManagementService) RejectProfile(adminID, profileID uint, reason string) (*resellerdomain.Profile, error) {
	return s.updateProfileReviewStatus(adminID, profileID, resellerdomain.ProfileStatusRejected, strings.TrimSpace(reason))
}

func (s *ManagementService) DisableProfile(adminID, profileID uint, reason string) (*resellerdomain.Profile, error) {
	return s.updateProfileReviewStatus(adminID, profileID, resellerdomain.ProfileStatusDisabled, strings.TrimSpace(reason))
}

func (s *ManagementService) RestoreProfile(adminID, profileID uint) (*resellerdomain.Profile, error) {
	return s.updateProfileReviewStatus(adminID, profileID, resellerdomain.ProfileStatusActive, "")
}

func (s *ManagementService) UpdateProfileOperationalConfig(adminID, profileID uint, input ResellerProfileUpdateInput) (*resellerdomain.Profile, error) {
	if s == nil || s.store == nil || adminID == 0 || profileID == 0 {
		return nil, productcontract.ErrNotFound
	}
	if err := validateResellerProfileMarkup(input.DefaultMarkupPercent, input.MaxMarkupPercent); err != nil {
		return nil, err
	}
	settlementStatus := strings.TrimSpace(input.SettlementStatus)
	if settlementStatus != "" && settlementStatus != resellerdomain.SettlementStatusNormal && settlementStatus != resellerdomain.SettlementStatusFrozen {
		return nil, resellercontract.ErrProfileStatusInvalid
	}
	var updated *resellerdomain.Profile
	err := s.store.WithinManagementTransaction(func(repoTx resellercontract.ManagementStore) error {
		profile, err := repoTx.GetProfileByID(profileID)
		if err != nil {
			return err
		}
		if profile == nil {
			return productcontract.ErrNotFound
		}
		now := time.Now()
		profile.DefaultMarkupPercent = money.FromDecimal(input.DefaultMarkupPercent)
		profile.MaxMarkupPercent = money.FromDecimal(input.MaxMarkupPercent)
		if settlementStatus != "" {
			profile.SettlementStatus = settlementStatus
		}
		profile.ReviewedBy = &adminID
		profile.ReviewedAt = &now
		if err := repoTx.UpdateProfile(profile); err != nil {
			return err
		}
		updated = profile
		return nil
	})
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, productcontract.ErrNotFound
	}
	return s.store.GetProfileByID(profileID)
}

func (s *ManagementService) SubmitUserCustomDomain(userID uint, rawDomain string) (*resellerdomain.Domain, error) {
	if s == nil || s.store == nil || userID == 0 {
		return nil, productcontract.ErrNotFound
	}
	profile, err := s.store.GetProfileByUserID(userID)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, resellercontract.ErrNotOpened
	}
	if profile.Status != resellerdomain.ProfileStatusActive {
		return nil, resellercontract.ErrProfileInactive
	}
	domain, err := normalizeAndValidateCustomDomain(rawDomain, s.cfg)
	if err != nil {
		return nil, err
	}
	// 暂不启用 DNS 自动校验，自定义域名走后台人工审核。
	row, err := s.store.UpsertDomain(resellerdomain.Domain{
		ResellerID:         profile.ID,
		Domain:             domain,
		Type:               resellerdomain.DomainTypeCustom,
		VerificationStatus: resellerdomain.DomainVerificationPending,
		Status:             resellerdomain.DomainStatusPendingReview,
		IsPrimary:          false,
	})
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "already exists") {
			return nil, resellercontract.ErrDomainConflict
		}
		return nil, err
	}
	return row, nil
}

func (s *ManagementService) ApproveDomain(ctx context.Context, adminID, domainID uint) (*resellerdomain.Domain, error) {
	return s.updateDomainStatus(ctx, adminID, domainID, resellerdomain.DomainStatusActive)
}

func (s *ManagementService) DisableDomain(ctx context.Context, adminID, domainID uint) (*resellerdomain.Domain, error) {
	return s.updateDomainStatus(ctx, adminID, domainID, resellerdomain.DomainStatusDisabled)
}

func (s *ManagementService) SetPrimaryDomain(ctx context.Context, adminID, domainID uint) (*resellerdomain.Domain, error) {
	if s == nil || s.store == nil || adminID == 0 || domainID == 0 {
		return nil, productcontract.ErrNotFound
	}
	cacheDomains := make([]string, 0, 4)
	err := s.store.WithinManagementTransaction(func(repoTx resellercontract.ManagementStore) error {
		target, err := repoTx.GetDomainByIDForUpdate(domainID)
		if err != nil {
			return err
		}
		if target == nil {
			return productcontract.ErrNotFound
		}
		if target.Status != resellerdomain.DomainStatusActive || target.VerificationStatus != resellerdomain.DomainVerificationVerified {
			return resellercontract.ErrDomainStatusInvalid
		}
		domains, err := repoTx.ListDomainsByResellerID(target.ResellerID)
		if err != nil {
			return err
		}
		for i := range domains {
			domain := domains[i]
			nextPrimary := domain.ID == target.ID
			if domain.IsPrimary == nextPrimary {
				if domain.Domain != "" {
					cacheDomains = append(cacheDomains, domain.Domain)
				}
				continue
			}
			domain.IsPrimary = nextPrimary
			if err := repoTx.UpdateDomain(&domain); err != nil {
				return err
			}
			if domain.Domain != "" {
				cacheDomains = append(cacheDomains, domain.Domain)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, domain := range cacheDomains {
		_ = cache.DelResellerDomain(ctx, domain)
	}
	return s.store.GetDomainByID(domainID)
}

func (s *ManagementService) updateProfileReviewStatus(adminID, profileID uint, targetStatus, reason string) (*resellerdomain.Profile, error) {
	if s == nil || s.store == nil || adminID == 0 || profileID == 0 {
		return nil, productcontract.ErrNotFound
	}
	cacheDomains := make([]string, 0, 4)
	err := s.store.WithinManagementTransaction(func(repoTx resellercontract.ManagementStore) error {
		profile, err := repoTx.GetProfileByID(profileID)
		if err != nil {
			return err
		}
		if profile == nil {
			return productcontract.ErrNotFound
		}
		switch targetStatus {
		case resellerdomain.ProfileStatusRejected:
			if profile.Status != resellerdomain.ProfileStatusPendingReview {
				return resellercontract.ErrProfileStatusInvalid
			}
		case resellerdomain.ProfileStatusDisabled:
			if profile.Status != resellerdomain.ProfileStatusPendingReview && profile.Status != resellerdomain.ProfileStatusActive && profile.Status != resellerdomain.ProfileStatusRejected {
				return resellercontract.ErrProfileStatusInvalid
			}
		case resellerdomain.ProfileStatusActive:
			if profile.Status != resellerdomain.ProfileStatusDisabled {
				return resellercontract.ErrProfileStatusInvalid
			}
		default:
			return resellercontract.ErrProfileStatusInvalid
		}
		domains, err := repoTx.ListDomainsByResellerID(profile.ID)
		if err != nil {
			return err
		}
		for i := range domains {
			if domains[i].Domain != "" {
				cacheDomains = append(cacheDomains, domains[i].Domain)
			}
		}
		now := time.Now()
		profile.Status = targetStatus
		if targetStatus == resellerdomain.ProfileStatusRejected || targetStatus == resellerdomain.ProfileStatusDisabled {
			profile.RejectReason = strings.TrimSpace(reason)
		} else {
			profile.RejectReason = ""
		}
		profile.ReviewedBy = &adminID
		profile.ReviewedAt = &now
		return repoTx.UpdateProfile(profile)
	})
	if err != nil {
		return nil, err
	}
	for _, domain := range cacheDomains {
		_ = cache.DelResellerDomain(context.Background(), domain)
	}
	return s.store.GetProfileByID(profileID)
}

func (s *ManagementService) updateDomainStatus(ctx context.Context, adminID, domainID uint, targetStatus string) (*resellerdomain.Domain, error) {
	if s == nil || s.store == nil || adminID == 0 || domainID == 0 {
		return nil, productcontract.ErrNotFound
	}
	cacheDomains := make([]string, 0, 4)
	err := s.store.WithinManagementTransaction(func(repoTx resellercontract.ManagementStore) error {
		domain, err := repoTx.GetDomainByIDForUpdate(domainID)
		if err != nil {
			return err
		}
		if domain == nil {
			return productcontract.ErrNotFound
		}
		domains, err := repoTx.ListDomainsByResellerID(domain.ResellerID)
		if err != nil {
			return err
		}
		for i := range domains {
			if domains[i].Domain != "" {
				cacheDomains = append(cacheDomains, domains[i].Domain)
			}
		}
		switch targetStatus {
		case resellerdomain.DomainStatusActive:
			if domain.Status != resellerdomain.DomainStatusPendingReview && domain.Status != resellerdomain.DomainStatusDisabled {
				return resellercontract.ErrDomainStatusInvalid
			}
			now := time.Now()
			domain.Status = resellerdomain.DomainStatusActive
			domain.VerificationStatus = resellerdomain.DomainVerificationVerified
			domain.VerifiedAt = &now
			if !hasActiveVerifiedPrimary(domains, domain.ID) {
				domain.IsPrimary = true
			}
		case resellerdomain.DomainStatusDisabled:
			if domain.Status != resellerdomain.DomainStatusPendingReview && domain.Status != resellerdomain.DomainStatusActive {
				return resellercontract.ErrDomainStatusInvalid
			}
			domain.Status = resellerdomain.DomainStatusDisabled
			wasPrimary := domain.IsPrimary
			domain.IsPrimary = false
			if wasPrimary {
				for i := range domains {
					candidate := domains[i]
					if candidate.ID == domain.ID || candidate.Status != resellerdomain.DomainStatusActive || candidate.VerificationStatus != resellerdomain.DomainVerificationVerified {
						continue
					}
					if !candidate.IsPrimary {
						candidate.IsPrimary = true
						if err := repoTx.UpdateDomain(&candidate); err != nil {
							return err
						}
					}
					break
				}
			}
		default:
			return resellercontract.ErrDomainStatusInvalid
		}
		if err := repoTx.UpdateDomain(domain); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, domain := range cacheDomains {
		_ = cache.DelResellerDomain(ctx, domain)
	}
	return s.store.GetDomainByID(domainID)
}

func validateResellerProfileMarkup(defaultMarkup, maxMarkup decimal.Decimal) error {
	if defaultMarkup.LessThan(decimal.Zero) || maxMarkup.LessThan(decimal.Zero) {
		return resellercontract.ErrProfileStatusInvalid
	}
	if maxMarkup.GreaterThan(decimal.Zero) && defaultMarkup.GreaterThan(maxMarkup) {
		return resellercontract.ErrProfileStatusInvalid
	}
	return nil
}

func hasActiveVerifiedPrimary(domains []resellerdomain.Domain, excludeID uint) bool {
	for i := range domains {
		domain := domains[i]
		if domain.ID == excludeID {
			continue
		}
		if domain.IsPrimary && domain.Status == resellerdomain.DomainStatusActive && domain.VerificationStatus == resellerdomain.DomainVerificationVerified {
			return true
		}
	}
	return false
}

func normalizeAndValidateCustomDomain(raw string, cfg config.ResellerConfig) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", resellercontract.ErrDomainInvalid
	}
	if strings.Contains(trimmed, "://") || strings.ContainsAny(trimmed, "/?#") {
		return "", resellercontract.ErrDomainInvalid
	}
	domain := resellercontract.NormalizeHost(trimmed)
	if domain == "" {
		return "", resellercontract.ErrDomainInvalid
	}
	for _, mainHost := range cfg.MainHosts {
		if domain == resellercontract.NormalizeHost(mainHost) {
			return "", resellercontract.ErrDomainMainHostNotAllowed
		}
	}
	base := resellercontract.NormalizeHost(cfg.SubdomainBase)
	if base != "" && (domain == base || strings.HasSuffix(domain, "."+base)) {
		return "", resellercontract.ErrDomainMainHostNotAllowed
	}
	return domain, nil
}

func normalizeAndValidateSystemSubdomain(raw string, base string, cfg config.ResellerConfig) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", resellercontract.ErrDomainInvalid
	}
	if strings.Contains(trimmed, "://") || strings.Contains(trimmed, ":") || strings.ContainsAny(trimmed, "/?#") || strings.ContainsAny(trimmed, " \t\r\n") {
		return "", resellercontract.ErrDomainInvalid
	}
	base = resellercontract.NormalizeHost(base)
	if base == "" {
		return "", resellercontract.ErrSubdomainBaseMissing
	}
	normalized := resellercontract.NormalizeHost(trimmed)
	if normalized == "" {
		return "", resellercontract.ErrDomainInvalid
	}
	domain := normalized
	if !strings.Contains(normalized, ".") {
		if !isValidResellerSubdomainLabel(normalized) {
			return "", resellercontract.ErrDomainInvalid
		}
		domain = normalized + "." + base
	}
	if domain == base || !strings.HasSuffix(domain, "."+base) {
		return "", resellercontract.ErrDomainInvalid
	}
	label := strings.TrimSuffix(domain, "."+base)
	if strings.Contains(label, ".") || !isValidResellerSubdomainLabel(label) {
		return "", resellercontract.ErrDomainInvalid
	}
	for _, mainHost := range cfg.MainHosts {
		if domain == resellercontract.NormalizeHost(mainHost) {
			return "", resellercontract.ErrDomainMainHostNotAllowed
		}
	}
	return domain, nil
}

func isValidResellerSubdomainLabel(label string) bool {
	if label == "" || len(label) > 63 {
		return false
	}
	if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
		return false
	}
	for _, ch := range label {
		switch {
		case ch >= 'a' && ch <= 'z':
		case ch >= '0' && ch <= '9':
		case ch == '-':
		default:
			return false
		}
	}
	return true
}
