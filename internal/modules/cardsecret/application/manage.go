package application

import (
	"strings"
	"time"

	cardsecretcontract "github.com/dujiao-next/internal/modules/cardsecret/contract"
	cardsecretdomain "github.com/dujiao-next/internal/modules/cardsecret/domain"
)

// ListCardSecretInput 卡密列表输入
type ListCardSecretInput struct {
	ProductID uint
	SKUID     uint
	BatchID   uint
	Status    string
	Secret    string
	BatchNo   string
	Page      int
	PageSize  int
}

// ListCardSecrets 获取卡密列表
func (s *Service) ListCardSecrets(input ListCardSecretInput) ([]cardsecretdomain.Secret, int64, error) {
	if input.SKUID > 0 && input.ProductID == 0 {
		return nil, 0, ErrInvalid
	}
	if input.ProductID > 0 && input.SKUID > 0 {
		if _, err := s.resolveCardSecretSKU(input.ProductID, input.SKUID); err != nil {
			return nil, 0, err
		}
	}

	items, total, err := s.secretRepo.List(cardsecretcontract.ListFilter{
		ProductID: input.ProductID,
		SKUID:     input.SKUID,
		BatchID:   input.BatchID,
		Status:    strings.TrimSpace(input.Status),
		Secret:    strings.TrimSpace(input.Secret),
		BatchNo:   strings.TrimSpace(input.BatchNo),
		Page:      input.Page,
		PageSize:  input.PageSize,
	})
	if err != nil {
		return nil, 0, ErrFetchFailed
	}
	return items, total, nil
}

func (s *Service) buildRepositoryFilter(input ListCardSecretInput) cardsecretcontract.ListFilter {
	return cardsecretcontract.ListFilter{
		ProductID: input.ProductID,
		SKUID:     input.SKUID,
		BatchID:   input.BatchID,
		Status:    strings.TrimSpace(input.Status),
		Secret:    strings.TrimSpace(input.Secret),
		BatchNo:   strings.TrimSpace(input.BatchNo),
		Page:      input.Page,
		PageSize:  input.PageSize,
	}
}

func (s *Service) hasListFilter(input ListCardSecretInput) bool {
	filter := s.buildRepositoryFilter(input)
	return filter.ProductID > 0 ||
		filter.SKUID > 0 ||
		filter.BatchID > 0 ||
		filter.Status != "" ||
		filter.Secret != "" ||
		filter.BatchNo != ""
}

// BatchUpdateCardSecretStatus 批量更新卡密状态
func (s *Service) BatchUpdateCardSecretStatus(ids []uint, batchID uint, filter ListCardSecretInput, status string) (int64, error) {
	normalizedStatus := strings.TrimSpace(status)
	switch normalizedStatus {
	case cardsecretdomain.StatusAvailable, cardsecretdomain.StatusReserved, cardsecretdomain.StatusUsed:
	default:
		return 0, ErrInvalid
	}
	normalizedIDs, err := s.resolveBatchTargetCardSecretIDs(ids, batchID, filter)
	if err != nil {
		return 0, err
	}
	rows, err := s.secretRepo.BatchUpdateStatus(normalizedIDs, normalizedStatus, time.Now())
	if err != nil {
		return 0, ErrUpdateFailed
	}
	return rows, nil
}

// BatchDeleteCardSecrets 批量删除卡密
func (s *Service) BatchDeleteCardSecrets(ids []uint, batchID uint, filter ListCardSecretInput) (int64, error) {
	normalizedIDs, err := s.resolveBatchTargetCardSecretIDs(ids, batchID, filter)
	if err != nil {
		return 0, err
	}
	rows, err := s.secretRepo.BatchDeleteByIDs(normalizedIDs)
	if err != nil {
		return 0, ErrDeleteFailed
	}
	return rows, nil
}

// UpdateCardSecret 更新卡密
func (s *Service) UpdateCardSecret(id uint, secret, status string, isLoop *bool) (*cardsecretdomain.Secret, error) {
	if id == 0 {
		return nil, ErrInvalid
	}
	item, err := s.secretRepo.GetByID(id)
	if err != nil {
		return nil, ErrFetchFailed
	}
	if item == nil {
		return nil, ErrNotFound
	}
	trimmedSecret := strings.TrimSpace(secret)
	if trimmedSecret != "" {
		item.Secret = trimmedSecret
	}
	trimmedStatus := strings.TrimSpace(status)
	if trimmedStatus != "" {
		switch trimmedStatus {
		case cardsecretdomain.StatusAvailable, cardsecretdomain.StatusReserved, cardsecretdomain.StatusUsed:
			item.Status = trimmedStatus
		default:
			return nil, ErrInvalid
		}
	}
	if isLoop != nil && *isLoop != item.IsLoop {
		// 已被订单占用的卡密不允许切换循环状态，避免影响待支付订单的发货
		if item.Status == cardsecretdomain.StatusReserved {
			return nil, ErrInvalid
		}
		item.IsLoop = *isLoop
	}
	item.UpdatedAt = time.Now()
	if err := s.secretRepo.Update(item); err != nil {
		return nil, ErrUpdateFailed
	}
	return item, nil
}
