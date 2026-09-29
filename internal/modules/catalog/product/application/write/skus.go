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

func (s *WriteService) syncSingleProductSKU(skuRepo SKURepository, productID uint, priceAmount decimal.Decimal, costPriceAmount decimal.Decimal, manualStockTotal int, createWhenMissing bool) error {
	if skuRepo == nil || productID == 0 {
		return nil
	}
	skus, err := skuRepo.ListByProduct(productID, false)
	if err != nil {
		return err
	}
	if len(skus) == 0 {
		if !createWhenMissing {
			return nil
		}
		return skuRepo.Create(&productdomain.ProductSKU{
			ProductID:         productID,
			SKUCode:           productdomain.DefaultSKUCode,
			SpecValuesJSON:    jsonmap.JSON{},
			PriceAmount:       money.FromDecimal(priceAmount),
			CostPriceAmount:   money.FromDecimal(costPriceAmount),
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

	target := skus[targetIndex]
	target.PriceAmount = money.FromDecimal(priceAmount)
	target.CostPriceAmount = money.FromDecimal(costPriceAmount)
	target.ManualStockTotal = manualStockTotal
	target.IsActive = true
	if strings.TrimSpace(target.SKUCode) == "" {
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

func pickSingleModeTargetSKUIndex(skus []productdomain.ProductSKU) int {
	if len(skus) == 0 {
		return -1
	}
	defaultCode := strings.ToUpper(strings.TrimSpace(productdomain.DefaultSKUCode))

	for i := range skus {
		if !skus[i].IsActive {
			continue
		}
		if strings.ToUpper(strings.TrimSpace(skus[i].SKUCode)) == defaultCode {
			return i
		}
	}
	for i := range skus {
		if skus[i].IsActive {
			return i
		}
	}
	for i := range skus {
		if strings.ToUpper(strings.TrimSpace(skus[i].SKUCode)) == defaultCode {
			return i
		}
	}
	return 0
}

type normalizedProductSKU struct {
	ID               uint
	SKUCode          string
	SpecValuesJSON   jsonmap.JSON
	PriceAmount      money.Amount
	CostPriceAmount  money.Amount
	ManualStockTotal int
	IsActive         bool
	SortOrder        int
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
			ID:               input.ID,
			SKUCode:          skuCode,
			SpecValuesJSON:   specValues,
			PriceAmount:      money.FromDecimal(priceAmount),
			CostPriceAmount:  money.FromDecimal(costPriceAmount),
			ManualStockTotal: manualTotal,
			IsActive:         isActive,
			SortOrder:        input.SortOrder,
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

	nextActive := make(map[uint]bool, len(existingRows))
	kept := make(map[uint]struct{}, len(rows))
	for _, row := range rows {
		if row.ID > 0 {
			existing, ok := existingByID[row.ID]
			if !ok {
				return productcontract.ErrProductSKUInvalid
			}
			nextActive[existing.ID] = row.IsActive
			kept[existing.ID] = struct{}{}
			continue
		}

		codeKey := strings.ToLower(strings.TrimSpace(row.SKUCode))
		if existing, ok := existingByCode[codeKey]; ok {
			nextActive[existing.ID] = row.IsActive
			kept[existing.ID] = struct{}{}
		}
	}

	for _, existing := range existingRows {
		if _, ok := nextActive[existing.ID]; !ok {
			nextActive[existing.ID] = false
		}
		if _, ok := kept[existing.ID]; !ok {
			nextActive[existing.ID] = false
		}
		if !existing.IsActive || nextActive[existing.ID] {
			continue
		}
		total, available, used, err := cardSecretRepo.CountByProduct(productID, existing.ID)
		if err != nil {
			return err
		}
		outstanding := total - used
		if available > 0 || outstanding > 0 {
			return productcontract.ErrProductSKUHasCardSecretStock
		}
	}
	return nil
}
