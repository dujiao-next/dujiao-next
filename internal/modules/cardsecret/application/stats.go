package application

import (
	"time"

	cardsecretdomain "github.com/dujiao-next/internal/modules/cardsecret/domain"
)

// CardSecretStats 卡密统计
type CardSecretStats struct {
	Total     int64 `json:"total"`
	Available int64 `json:"available"`
	Reserved  int64 `json:"reserved"`
	Used      int64 `json:"used"`
	// LoopAvailable 可用循环卡密数量（已包含在 Available 中）
	LoopAvailable int64 `json:"loop_available"`
}

// CardSecretBatchSummary 卡密批次列表摘要
type CardSecretBatchSummary struct {
	ID             uint      `json:"id"`
	ProductID      uint      `json:"product_id"`
	SKUID          uint      `json:"sku_id"`
	Name           string    `json:"name"`
	BatchNo        string    `json:"batch_no"`
	Source         string    `json:"source"`
	Note           string    `json:"note"`
	TotalCount     int64     `json:"total_count"`
	AvailableCount int64     `json:"available_count"`
	ReservedCount  int64     `json:"reserved_count"`
	UsedCount      int64     `json:"used_count"`
	CreatedAt      time.Time `json:"created_at"`
}

// GetStats 获取库存统计
func (s *Service) GetStats(productID, skuID uint) (*CardSecretStats, error) {
	if productID == 0 {
		return nil, ErrInvalid
	}
	if skuID > 0 {
		if _, err := s.resolveCardSecretSKU(productID, skuID); err != nil {
			return nil, err
		}
	}
	total, available, used, err := s.secretRepo.CountByProduct(productID, skuID)
	if err != nil {
		return nil, ErrStatsFailed
	}
	reserved, err := s.secretRepo.CountReserved(productID, skuID)
	if err != nil {
		return nil, ErrStatsFailed
	}
	loopAvailable, err := s.secretRepo.CountAvailableLoop(productID, skuID)
	if err != nil {
		return nil, ErrStatsFailed
	}
	return &CardSecretStats{
		Total:         total,
		Available:     available,
		Reserved:      reserved,
		Used:          used,
		LoopAvailable: loopAvailable,
	}, nil
}

// ListBatches 获取批次列表
func (s *Service) ListBatches(productID, skuID uint, page, pageSize int) ([]CardSecretBatchSummary, int64, error) {
	if productID == 0 {
		return nil, 0, ErrInvalid
	}
	if skuID > 0 {
		if _, err := s.resolveCardSecretSKU(productID, skuID); err != nil {
			return nil, 0, err
		}
	}
	if s.batchRepo == nil {
		return nil, 0, ErrBatchFetchFailed
	}
	items, total, err := s.batchRepo.ListByProduct(productID, skuID, page, pageSize)
	if err != nil {
		return nil, 0, ErrBatchFetchFailed
	}
	if len(items) == 0 {
		return []CardSecretBatchSummary{}, total, nil
	}

	batchIDs := make([]uint, 0, len(items))
	for _, item := range items {
		batchIDs = append(batchIDs, item.ID)
	}
	countRows, err := s.secretRepo.CountByBatchIDs(batchIDs)
	if err != nil {
		return nil, 0, ErrBatchFetchFailed
	}

	type batchCounter struct {
		available int64
		reserved  int64
		used      int64
	}
	counterMap := make(map[uint]batchCounter, len(batchIDs))
	for _, row := range countRows {
		counter := counterMap[row.BatchID]
		switch row.Status {
		case cardsecretdomain.StatusAvailable:
			counter.available = row.Total
		case cardsecretdomain.StatusReserved:
			counter.reserved = row.Total
		case cardsecretdomain.StatusUsed:
			counter.used = row.Total
		}
		counterMap[row.BatchID] = counter
	}

	result := make([]CardSecretBatchSummary, 0, len(items))
	for _, item := range items {
		counter := counterMap[item.ID]
		result = append(result, CardSecretBatchSummary{
			ID:             item.ID,
			ProductID:      item.ProductID,
			SKUID:          item.SKUID,
			Name:           "",
			BatchNo:        item.BatchNo,
			Source:         item.Source,
			Note:           item.Note,
			TotalCount:     counter.available + counter.reserved + counter.used,
			AvailableCount: counter.available,
			ReservedCount:  counter.reserved,
			UsedCount:      counter.used,
			CreatedAt:      item.CreatedAt,
		})
	}
	return result, total, nil
}
