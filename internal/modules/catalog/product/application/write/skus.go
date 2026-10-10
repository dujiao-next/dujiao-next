package productwrite

import (
	"strings"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	"github.com/dujiao-next/internal/constants"
	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// syncSingleProductSKU 把商品收敛为单规格模式：恰好保留一行 DEFAULT SKU，删除其余规格行。
//
// DEFAULT 行是单规格约定的内部表示（见 productdomain.DefaultSKUCode），后台不把它当作
// 一条"规格"展示，因此用户在界面上删除全部规格后保存，得到的就是这个状态。
//
// 删除会连带让被删 SKU 的卡密库存失去归属，因此复用多规格路径的库存守卫：
// 待删 SKU 只要还有可用或未核销的卡密库存，就拒绝而不是静默删除。
func (s *WriteService) syncSingleProductSKU(
	skuRepo SKURepository,
	cardSecretRepo CardSecretStockRepository,
	productID uint,
	fulfillmentType string,
	priceAmount decimal.Decimal,
	costPriceAmount decimal.Decimal,
	agencyPriceAmount decimal.Decimal,
	manualStockTotal int,
) error {
	if skuRepo == nil || productID == 0 {
		return nil
	}
	skus, err := skuRepo.ListByProduct(productID, false)
	if err != nil {
		return err
	}
	if len(skus) == 0 {
		return skuRepo.Create(&productdomain.ProductSKU{
			ProductID:         productID,
			SKUCode:           productdomain.DefaultSKUCode,
			SpecValuesJSON:    jsonmap.JSON{},
			PriceAmount:       money.FromDecimal(priceAmount),
			CostPriceAmount:   money.FromDecimal(costPriceAmount),
			AgencyPriceAmount: money.FromDecimal(agencyPriceAmount),
			ManualStockTotal:  manualStockTotal,
			ManualStockLocked: 0,
			ManualStockSold:   0,
			IsActive:          true,
			SortOrder:         0,
		})
	}
	targetIndex := pickSingleModeTargetSKUIndex(skus)
	if targetIndex < 0 || targetIndex >= len(skus) {
		return productcontract.ErrProductSKUInvalid
	}

	// 先确认待删除的行可以安全删除，避免删到一半才发现有卡密库存。
	for i := range skus {
		if i == targetIndex {
			continue
		}
		if err := ensureSKURemovable(cardSecretRepo, productID, fulfillmentType, skus[i]); err != nil {
			return err
		}
	}

	target := skus[targetIndex]
	target.PriceAmount = money.FromDecimal(priceAmount)
	target.CostPriceAmount = money.FromDecimal(costPriceAmount)
	target.AgencyPriceAmount = money.FromDecimal(agencyPriceAmount)
	target.ManualStockTotal = manualStockTotal
	target.IsActive = true
	target.SpecValuesJSON = jsonmap.JSON{}
	if strings.ToUpper(strings.TrimSpace(target.SKUCode)) != strings.ToUpper(productdomain.DefaultSKUCode) {
		// 改名为 DEFAULT 前先清理同编码的软删除残留，否则会撞唯一索引。
		if err := skuRepo.PurgeSoftDeletedByProductAndCode(productID, productdomain.DefaultSKUCode); err != nil {
			return err
		}
		target.SKUCode = productdomain.DefaultSKUCode
	}
	if err := skuRepo.Update(&target); err != nil {
		return err
	}

	for i := range skus {
		if i == targetIndex {
			continue
		}
		if err := skuRepo.Delete(skus[i].ID); err != nil {
			return err
		}
	}
	return nil
}

// ensureSKURemovable 在自动发货场景下校验该 SKU 是否可以安全删除（无卡密库存占用）。
// 非自动发货或未注入卡密仓储时直接放行。
func ensureSKURemovable(cardSecretRepo CardSecretStockRepository, productID uint, fulfillmentType string, sku productdomain.ProductSKU) error {
	if cardSecretRepo == nil || productID == 0 || strings.TrimSpace(fulfillmentType) != constants.FulfillmentTypeAuto {
		return nil
	}
	total, available, used, err := cardSecretRepo.CountByProduct(productID, sku.ID)
	if err != nil {
		return err
	}
	if available > 0 || total-used > 0 {
		return productcontract.ErrProductSKUHasCardSecretStock
	}
	return nil
}

// pickSingleModeTargetSKUIndex 选择单规格模式下存续的那一行。
// 单规格模式的唯一存续行必须是 DEFAULT，因此优先复用已存在的 DEFAULT 行（无论当前是否启用），
// 避免每次保存都新建一行；只有确实没有 DEFAULT 行时才退化为挑一个现有行改名。
func pickSingleModeTargetSKUIndex(skus []productdomain.ProductSKU) int {
	if len(skus) == 0 {
		return -1
	}
	defaultCode := strings.ToUpper(strings.TrimSpace(productdomain.DefaultSKUCode))

	for i := range skus {
		if strings.ToUpper(strings.TrimSpace(skus[i].SKUCode)) == defaultCode {
			return i
		}
	}
	for i := range skus {
		if skus[i].IsActive {
			return i
		}
	}
	return 0
}

type normalizedProductSKU struct {
	ID                uint
	SKUCode           string
	SpecValuesJSON    jsonmap.JSON
	PriceAmount       money.Amount
	CostPriceAmount   money.Amount
	AgencyPriceAmount money.Amount
	ManualStockTotal  int
	IsActive          bool
	SortOrder         int
}

func (s *WriteService) normalizeProductSKUInputs(inputs []ProductSKUInput, fulfillmentType string, existingSKUMap map[uint]productdomain.ProductSKU) ([]normalizedProductSKU, decimal.Decimal, int, error) {
	if len(inputs) == 0 {
		return nil, decimal.Zero, 0, productcontract.ErrProductSKUInvalid
	}
	seenCode := make(map[string]struct{}, len(inputs))
	normalized := make([]normalizedProductSKU, 0, len(inputs))
	hasActive := false
	minActivePrice := decimal.Zero
	manualStockTotal := 0
	hasUnlimitedStock := false

	for _, input := range inputs {
		skuCode := strings.TrimSpace(input.SKUCode)
		if skuCode == "" {
			return nil, decimal.Zero, 0, productcontract.ErrProductSKUInvalid
		}
		codeKey := strings.ToLower(skuCode)
		if _, exists := seenCode[codeKey]; exists {
			return nil, decimal.Zero, 0, productcontract.ErrProductSKUInvalid
		}
		seenCode[codeKey] = struct{}{}

		priceAmount := input.PriceAmount.Round(2)
		if priceAmount.LessThanOrEqual(decimal.Zero) {
			return nil, decimal.Zero, 0, productcontract.ErrProductPriceInvalid
		}
		costPriceAmount := input.CostPriceAmount.Round(2)
		if costPriceAmount.LessThan(decimal.Zero) {
			return nil, decimal.Zero, 0, productcontract.ErrProductPriceInvalid
		}
		agencyPriceAmount := input.AgencyPriceAmount.Round(2)
		if agencyPriceAmount.LessThan(decimal.Zero) {
			return nil, decimal.Zero, 0, productcontract.ErrProductPriceInvalid
		}

		manualTotal := input.ManualStockTotal
		if manualTotal < constants.ManualStockUnlimited {
			return nil, decimal.Zero, 0, productcontract.ErrManualStockInvalid
		}
		if fulfillmentType != constants.FulfillmentTypeManual {
			manualTotal = 0
		}
		if existingSKUMap != nil && input.ID > 0 {
			_, ok := existingSKUMap[input.ID]
			if !ok {
				return nil, decimal.Zero, 0, productcontract.ErrProductSKUInvalid
			}
		}

		isActive := true
		if input.IsActive != nil {
			isActive = *input.IsActive
		}
		specValues := jsonmap.JSON{}
		if input.SpecValuesJSON != nil {
			specValues = jsonmap.JSON(input.SpecValuesJSON)
		}

		normalized = append(normalized, normalizedProductSKU{
			ID:                input.ID,
			SKUCode:           skuCode,
			SpecValuesJSON:    specValues,
			PriceAmount:       money.FromDecimal(priceAmount),
			CostPriceAmount:   money.FromDecimal(costPriceAmount),
			AgencyPriceAmount: money.FromDecimal(agencyPriceAmount),
			ManualStockTotal:  manualTotal,
			IsActive:          isActive,
			SortOrder:         input.SortOrder,
		})

		if isActive {
			if !hasActive || priceAmount.LessThan(minActivePrice) {
				minActivePrice = priceAmount
			}
			hasActive = true
			if fulfillmentType == constants.FulfillmentTypeManual {
				if manualTotal == constants.ManualStockUnlimited {
					hasUnlimitedStock = true
				} else {
					manualStockTotal += manualTotal
				}
			}
		}
	}

	if !hasActive {
		return nil, decimal.Zero, 0, productcontract.ErrProductSKUInvalid
	}
	if fulfillmentType != constants.FulfillmentTypeManual {
		manualStockTotal = 0
	} else if hasUnlimitedStock {
		manualStockTotal = constants.ManualStockUnlimited
	}
	return normalized, minActivePrice, manualStockTotal, nil
}

// minActiveCostPrice 从已标准化的 SKU 列表中取最低活跃 SKU 的成本价
func minActiveCostPrice(skus []normalizedProductSKU) decimal.Decimal {
	first := true
	min := decimal.Zero
	for _, s := range skus {
		if !s.IsActive {
			continue
		}
		d := s.CostPriceAmount.Decimal
		if first || d.LessThan(min) {
			min = d
			first = false
		}
	}
	return min
}

// minActiveAgencyPrice 从已标准化的 SKU 列表中取最低活跃 SKU 的代理价
func minActiveAgencyPrice(skus []normalizedProductSKU) decimal.Decimal {
	first := true
	min := decimal.Zero
	for _, s := range skus {
		if !s.IsActive {
			continue
		}
		d := s.AgencyPriceAmount.Decimal
		if first || d.LessThan(min) {
			min = d
			first = false
		}
	}
	return min
}

func (s *WriteService) applyProductSKUsWithStockGuard(
	skuRepo SKURepository,
	cardSecretRepo CardSecretStockRepository,
	productID uint,
	fulfillmentType string,
	rows []normalizedProductSKU,
) error {
	if skuRepo == nil || productID == 0 || len(rows) == 0 {
		return productcontract.ErrProductSKUInvalid
	}
	existingRows, err := skuRepo.ListByProduct(productID, false)
	if err != nil {
		return err
	}
	existingByID := make(map[uint]productdomain.ProductSKU, len(existingRows))
	existingByCode := make(map[string]productdomain.ProductSKU, len(existingRows))
	for _, row := range existingRows {
		existingByID[row.ID] = row
		existingByCode[strings.ToLower(strings.TrimSpace(row.SKUCode))] = row
	}
	if err := s.ensureAutoSKUCardSecretStockSafe(cardSecretRepo, productID, fulfillmentType, existingRows, rows, existingByID, existingByCode); err != nil {
		return err
	}

	kept := make(map[uint]struct{}, len(rows))
	for _, row := range rows {
		if row.ID > 0 {
			existing, ok := existingByID[row.ID]
			if !ok {
				return productcontract.ErrProductSKUInvalid
			}
			existing.SKUCode = row.SKUCode
			existing.SpecValuesJSON = row.SpecValuesJSON
			existing.PriceAmount = row.PriceAmount
			existing.CostPriceAmount = row.CostPriceAmount
			existing.AgencyPriceAmount = row.AgencyPriceAmount
			existing.ManualStockTotal = row.ManualStockTotal
			existing.IsActive = row.IsActive
			existing.SortOrder = row.SortOrder
			if err := skuRepo.Update(&existing); err != nil {
				return err
			}
			kept[existing.ID] = struct{}{}
			existingByCode[strings.ToLower(strings.TrimSpace(existing.SKUCode))] = existing
			continue
		}

		codeKey := strings.ToLower(strings.TrimSpace(row.SKUCode))
		if existing, ok := existingByCode[codeKey]; ok {
			existing.SpecValuesJSON = row.SpecValuesJSON
			existing.PriceAmount = row.PriceAmount
			existing.CostPriceAmount = row.CostPriceAmount
			existing.AgencyPriceAmount = row.AgencyPriceAmount
			existing.ManualStockTotal = row.ManualStockTotal
			existing.IsActive = row.IsActive
			existing.SortOrder = row.SortOrder
			if err := skuRepo.Update(&existing); err != nil {
				return err
			}
			kept[existing.ID] = struct{}{}
			continue
		}

		// 清理同 sku_code 的软删除残留，避免唯一索引冲突
		if err := skuRepo.PurgeSoftDeletedByProductAndCode(productID, row.SKUCode); err != nil {
			return err
		}
		item := productdomain.ProductSKU{
			ProductID:         productID,
			SKUCode:           row.SKUCode,
			SpecValuesJSON:    row.SpecValuesJSON,
			PriceAmount:       row.PriceAmount,
			CostPriceAmount:   row.CostPriceAmount,
			AgencyPriceAmount: row.AgencyPriceAmount,
			ManualStockTotal:  row.ManualStockTotal,
			ManualStockLocked: 0,
			ManualStockSold:   0,
			IsActive:          row.IsActive,
			SortOrder:         row.SortOrder,
		}
		if err := skuRepo.Create(&item); err != nil {
			return err
		}
		kept[item.ID] = struct{}{}
	}

	for _, existing := range existingRows {
		if _, ok := kept[existing.ID]; ok {
			continue
		}
		if err := skuRepo.Delete(existing.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *WriteService) ensureAutoSKUCardSecretStockSafe(
	cardSecretRepo CardSecretStockRepository,
	productID uint,
	fulfillmentType string,
	existingRows []productdomain.ProductSKU,
	rows []normalizedProductSKU,
	existingByID map[uint]productdomain.ProductSKU,
	existingByCode map[string]productdomain.ProductSKU,
) error {
	if cardSecretRepo == nil || productID == 0 || strings.TrimSpace(fulfillmentType) != constants.FulfillmentTypeAuto {
		return nil
	}

	// nextActive 只收录本次保留的行；不在其中的行将被删除。
	nextActive := make(map[uint]bool, len(existingRows))
	for _, row := range rows {
		if row.ID > 0 {
			existing, ok := existingByID[row.ID]
			if !ok {
				return productcontract.ErrProductSKUInvalid
			}
			nextActive[existing.ID] = row.IsActive
			continue
		}

		codeKey := strings.ToLower(strings.TrimSpace(row.SKUCode))
		if existing, ok := existingByCode[codeKey]; ok {
			nextActive[existing.ID] = row.IsActive
		}
	}

	for _, existing := range existingRows {
		// 删除的行一律校验（与单规格收敛路径一致）；保留的行仅在由启用变为停用时校验。
		if active, kept := nextActive[existing.ID]; kept && (!existing.IsActive || active) {
			continue
		}
		if err := ensureSKURemovable(cardSecretRepo, productID, fulfillmentType, existing); err != nil {
			return err
		}
	}
	return nil
}
