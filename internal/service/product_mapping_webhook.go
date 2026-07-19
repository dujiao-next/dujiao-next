package service

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"

	"gorm.io/gorm"
)

// BindGenericWebhookProduct binds an existing local product to a generic webhook connection.
func (s *ProductMappingService) BindGenericWebhookProduct(connectionID, localProductID uint) (*models.ProductMapping, error) {
	conn, err := s.connService.GetByID(connectionID)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, ErrConnectionNotFound
	}
	if conn.Protocol != constants.ConnectionProtocolGenericWebhook {
		return nil, ErrProtocolCapabilityUnsupported
	}
	if existing, err := s.mappingRepo.GetByLocalProductID(localProductID); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, ErrMappingAlreadyExists
	}

	product, err := s.productRepo.GetAdminByID(strconv.FormatUint(uint64(localProductID), 10))
	if err != nil {
		return nil, err
	}
	if product == nil {
		return nil, ErrNotFound
	}
	activeSKUs, err := s.productSKURepo.ListByProduct(localProductID, true)
	if err != nil {
		return nil, err
	}
	if len(activeSKUs) == 0 {
		return nil, ErrProductNoActiveSKU
	}

	originalFulfillmentType := strings.TrimSpace(product.FulfillmentType)
	if originalFulfillmentType != constants.FulfillmentTypeAuto {
		originalFulfillmentType = constants.FulfillmentTypeManual
	}

	var mapping *models.ProductMapping
	err = s.productRepo.Transaction(func(tx *gorm.DB) error {
		productRepo := s.productRepo.WithTx(tx)
		mappingRepo := s.mappingRepo.WithTx(tx)
		skuMappingRepo := s.skuMappingRepo.WithTx(tx)

		now := time.Now()
		mapping = &models.ProductMapping{
			ConnectionID:            connectionID,
			LocalProductID:          localProductID,
			UpstreamProductID:       localProductID,
			UpstreamFulfillmentType: originalFulfillmentType,
			UpstreamStatus:          models.UpstreamStatusActive,
			IsActive:                true,
			LastSyncedAt:            &now,
		}
		if err := mappingRepo.Create(mapping); err != nil {
			return fmt.Errorf("create generic webhook product mapping: %w", err)
		}
		for i := range activeSKUs {
			sku := activeSKUs[i]
			skuMapping := &models.SKUMapping{
				ProductMappingID: mapping.ID,
				LocalSKUID:       sku.ID,
				UpstreamSKUID:    sku.ID,
				UpstreamPrice:    sku.CostPriceAmount,
				UpstreamStock:    -1,
				UpstreamIsActive: true,
				StockSyncedAt:    &now,
			}
			if err := skuMappingRepo.Create(skuMapping); err != nil {
				return fmt.Errorf("create generic webhook sku mapping: %w", err)
			}
		}

		product.FulfillmentType = constants.FulfillmentTypeUpstream
		product.IsMapped = true
		product.IsActive = false
		if err := productRepo.Update(product); err != nil {
			return fmt.Errorf("update generic webhook product: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return mapping, nil
}
