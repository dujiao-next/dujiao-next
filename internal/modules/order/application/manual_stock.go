package application

import (
	"strconv"
	"strings"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"

	"github.com/dujiao-next/internal/constants"
)

type manualStockSummary struct {
	BySKU           map[uint]int
	ByProductAll    map[uint]int
	ByLegacyProduct map[uint]int
}

func summarizeManualStockItems(items []orderdomain.OrderItem) manualStockSummary {
	result := manualStockSummary{
		BySKU:           make(map[uint]int),
		ByProductAll:    make(map[uint]int),
		ByLegacyProduct: make(map[uint]int),
	}
	for _, item := range items {
		if strings.TrimSpace(item.FulfillmentType) != constants.FulfillmentTypeManual {
			continue
		}
		if item.ProductID == 0 || item.Quantity <= 0 {
			continue
		}
		result.ByProductAll[item.ProductID] += item.Quantity
		if item.SKUID > 0 {
			result.BySKU[item.SKUID] += item.Quantity
			continue
		}
		result.ByLegacyProduct[item.ProductID] += item.Quantity
	}
	return result
}

func releaseManualStockByItems(productRepo productcontract.Repository, productSKURepo productcontract.SKURepository, items []orderdomain.OrderItem) error {
	var skuOp func(uint, int) (int64, error)
	if productSKURepo != nil {
		skuOp = productSKURepo.ReleaseManualStock
	}
	var productOp func(uint, int) (int64, error)
	if productRepo != nil {
		productOp = productRepo.ReleaseManualStock
	}
	return applyManualStockByItems(productRepo, productSKURepo, items, skuOp, productOp, false)
}

// reserveManualStockByItems 重新占用人工库存，库存不足时返回 ErrManualStockInsufficient。
// 用于重开已取消订单：取消时占用已释放，重开必须重新占用，否则后续消耗会扣掉其他待支付订单的占用。
func reserveManualStockByItems(productRepo productcontract.Repository, productSKURepo productcontract.SKURepository, items []orderdomain.OrderItem) error {
	var skuOp func(uint, int) (int64, error)
	if productSKURepo != nil {
		skuOp = productSKURepo.ReserveManualStock
	}
	var productOp func(uint, int) (int64, error)
	if productRepo != nil {
		productOp = productRepo.ReserveManualStock
	}
	return applyManualStockByItems(productRepo, productSKURepo, items, skuOp, productOp, true)
}

func ConsumeManualStockByItems(productRepo productcontract.Repository, productSKURepo productcontract.SKURepository, items []orderdomain.OrderItem) error {
	var skuOp func(uint, int) (int64, error)
	if productSKURepo != nil {
		skuOp = productSKURepo.ConsumeManualStock
	}
	var productOp func(uint, int) (int64, error)
	if productRepo != nil {
		productOp = productRepo.ConsumeManualStock
	}
	return applyManualStockByItems(productRepo, productSKURepo, items, skuOp, productOp, true)
}

func applyManualStockByItems(
	productRepo productcontract.Repository,
	productSKURepo productcontract.SKURepository,
	items []orderdomain.OrderItem,
	updateSKU func(uint, int) (int64, error),
	updateProduct func(uint, int) (int64, error),
	requireAffected bool,
) error {
	summary := summarizeManualStockItems(items)
	if productSKURepo != nil && updateSKU != nil {
		for skuID, quantity := range summary.BySKU {
			sku, err := productSKURepo.GetByID(skuID)
			if err != nil {
				return err
			}
			if sku == nil || sku.ManualStockTotal == constants.ManualStockUnlimited {
				continue
			}
			affected, err := updateSKU(skuID, quantity)
			if err != nil {
				return err
			}
			if requireAffected && affected != 1 {
				return ErrManualStockInsufficient
			}
		}
	}

	productSummary := summary.ByLegacyProduct
	if productSKURepo == nil {
		productSummary = summary.ByProductAll
	}
	if productRepo == nil || updateProduct == nil {
		return nil
	}
	for productID, quantity := range productSummary {
		product, err := productRepo.GetByID(strconv.FormatUint(uint64(productID), 10))
		if err != nil {
			return err
		}
		if product == nil || product.ManualStockTotal == constants.ManualStockUnlimited {
			continue
		}
		affected, err := updateProduct(productID, quantity)
		if err != nil {
			return err
		}
		if requireAffected && affected != 1 {
			return ErrManualStockInsufficient
		}
	}
	return nil
}
