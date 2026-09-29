package catalog

import (
	"strings"

	"github.com/dujiao-next/internal/constants"
)

const (
	StorefrontLowStockThreshold int64 = 5
	UpstreamLowStockThreshold   int64 = 20
)

// StockPolicy 描述特定消费上下文如何把可用数量映射为库存状态。
// Public 与 Channel 属于店面消费上下文；Upstream API 保留更宽的低库存预警窗口。
type StockPolicy struct {
	LowStockThreshold int64
}

// StorefrontStockPolicy 返回 Public/Channel 使用的库存策略值。
func StorefrontStockPolicy() StockPolicy {
	return StockPolicy{LowStockThreshold: StorefrontLowStockThreshold}
}

// UpstreamStockPolicy 返回 Upstream API 使用的库存策略值。
func UpstreamStockPolicy() StockPolicy {
	return StockPolicy{LowStockThreshold: UpstreamLowStockThreshold}
}

// Status 根据可用数量计算库存状态。负数表示无限库存。
func (policy StockPolicy) Status(quantity int64) string {
	threshold := policy.LowStockThreshold
	if threshold < 0 {
		threshold = 0
	}
	switch {
	case quantity < 0:
		return constants.ProductStockStatusUnlimited
	case quantity == 0:
		return constants.ProductStockStatusOutOfStock
	case quantity <= threshold:
		return constants.ProductStockStatusLowStock
	default:
		return constants.ProductStockStatusInStock
	}
}

// NormalizeStatus 优先保留调用方已经解析出的有效状态；负库存始终归一为无限库存。
func (policy StockPolicy) NormalizeStatus(status string, quantity int64) string {
	if quantity < 0 {
		return constants.ProductStockStatusUnlimited
	}
	switch strings.TrimSpace(status) {
	case constants.ProductStockStatusUnlimited:
		return constants.ProductStockStatusUnlimited
	case constants.ProductStockStatusOutOfStock:
		return constants.ProductStockStatusOutOfStock
	case constants.ProductStockStatusLowStock:
		return constants.ProductStockStatusLowStock
	case constants.ProductStockStatusInStock:
		return constants.ProductStockStatusInStock
	default:
		return policy.Status(quantity)
	}
}

// StockDisplay 是跨 Public/Channel 共用的库存展示决策。
type StockDisplay struct {
	Mode           string
	Status         string
	Display        string
	RangeMin       *int
	RangeMax       *int
	QuantityHidden bool
}

// Display 根据展示模式、库存状态和真实数量构建展示元数据。
func (policy StockPolicy) Display(mode, status string, quantity int64) StockDisplay {
	normalizedMode := NormalizeStockDisplayMode(mode)
	normalizedStatus := policy.NormalizeStatus(status, quantity)
	view := StockDisplay{
		Mode:           normalizedMode,
		Status:         normalizedStatus,
		Display:        normalizedStatus,
		QuantityHidden: normalizedMode != constants.ProductStockDisplayExact,
	}

	switch normalizedMode {
	case constants.ProductStockDisplayRange:
		if normalizedStatus == constants.ProductStockStatusInStock || normalizedStatus == constants.ProductStockStatusLowStock {
			view.Display, view.RangeMin, view.RangeMax = StockRange(quantity)
		}
	case constants.ProductStockDisplayHidden:
		if normalizedStatus == constants.ProductStockStatusInStock || normalizedStatus == constants.ProductStockStatusLowStock {
			view.Display = constants.ProductStockDisplayHidden
		}
	case constants.ProductStockDisplayExact:
		view.Display = constants.ProductStockDisplayExact
	}
	return view
}

// NormalizeStockDisplayMode 把未知展示模式回退为精确库存。
func NormalizeStockDisplayMode(raw string) string {
	switch strings.TrimSpace(raw) {
	case constants.ProductStockDisplayStatus:
		return constants.ProductStockDisplayStatus
	case constants.ProductStockDisplayRange:
		return constants.ProductStockDisplayRange
	case constants.ProductStockDisplayHidden:
		return constants.ProductStockDisplayHidden
	default:
		return constants.ProductStockDisplayExact
	}
}

// StockRange 把正库存映射到稳定的公开区间。
func StockRange(quantity int64) (string, *int, *int) {
	switch {
	case quantity <= 5:
		min, max := 1, 5
		return constants.ProductStockDisplayRange1To5, &min, &max
	case quantity <= 20:
		min, max := 6, 20
		return constants.ProductStockDisplayRange6To20, &min, &max
	case quantity <= 50:
		min, max := 21, 50
		return constants.ProductStockDisplayRange21To50, &min, &max
	case quantity <= 100:
		min, max := 51, 100
		return constants.ProductStockDisplayRange51To100, &min, &max
	default:
		min := 100
		return constants.ProductStockDisplayRange100Plus, &min, nil
	}
}

// StockQuantity 按交付类型选择已经计算完成的可用库存。
func StockQuantity(fulfillmentType string, autoStockAvailable int64, manualStockAvailable int) int64 {
	if strings.TrimSpace(fulfillmentType) == constants.FulfillmentTypeAuto {
		return autoStockAvailable
	}
	return int64(manualStockAvailable)
}

// MaskStockInt 在非精确展示模式中隐藏真实库存数量，同时保留售罄/有货/无限语义。
func MaskStockInt(mode string, value int) int {
	if NormalizeStockDisplayMode(mode) == constants.ProductStockDisplayExact {
		return value
	}
	switch {
	case value == constants.ManualStockUnlimited:
		return constants.ManualStockUnlimited
	case value <= 0:
		return 0
	default:
		return 1
	}
}

// MaskStockInt64 是 int64 库存的隐藏版本。
func MaskStockInt64(mode string, value int64) int64 {
	if NormalizeStockDisplayMode(mode) == constants.ProductStockDisplayExact {
		return value
	}
	switch {
	case value == int64(constants.ManualStockUnlimited):
		return int64(constants.ManualStockUnlimited)
	case value <= 0:
		return 0
	default:
		return 1
	}
}

// MaskSoldCount 避免非精确库存模式通过已售数量反推出真实库存。
func MaskSoldCount(mode string, value int) int {
	if NormalizeStockDisplayMode(mode) == constants.ProductStockDisplayExact {
		return value
	}
	return 0
}
