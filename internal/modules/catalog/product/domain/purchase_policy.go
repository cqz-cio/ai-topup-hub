package productdomain

import (
	"errors"
	"strconv"
	"strings"

	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"

	"github.com/dujiao-next/internal/constants"
)

var (
	ErrPurchaseQuantityInvalid = errors.New("invalid order item")
	ErrMaxPurchaseExceeded     = errors.New("product max purchase exceeded")
	ErrMinPurchaseNotMet       = errors.New("product min purchase not met")
	ErrCategoryInvalid         = errors.New("product category invalid")
)

// CategoryAssignmentRepository 是商品分类归属校验所需的最小端口。
type CategoryAssignmentRepository interface {
	GetByID(id string) (*categorydomain.Category, error)
	CountChildren(categoryID string) (int64, error)
}

// NormalizePurchaseType 归一化商品购买身份限制。
func NormalizePurchaseType(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "", constants.ProductPurchaseMember:
		return constants.ProductPurchaseMember
	case constants.ProductPurchaseGuest:
		return constants.ProductPurchaseGuest
	default:
		return ""
	}
}

// NormalizeFulfillmentType 归一化商品交付类型。
func NormalizeFulfillmentType(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "", constants.FulfillmentTypeManual:
		return constants.FulfillmentTypeManual
	case constants.FulfillmentTypeAuto:
		return constants.FulfillmentTypeAuto
	case constants.FulfillmentTypeUpstream:
		return constants.FulfillmentTypeUpstream
	default:
		return ""
	}
}

// NormalizeStockDisplayMode 归一化商品库存展示模式。
func NormalizeStockDisplayMode(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "", constants.ProductStockDisplayExact:
		return constants.ProductStockDisplayExact
	case constants.ProductStockDisplayStatus:
		return constants.ProductStockDisplayStatus
	case constants.ProductStockDisplayRange:
		return constants.ProductStockDisplayRange
	case constants.ProductStockDisplayHidden:
		return constants.ProductStockDisplayHidden
	default:
		return ""
	}
}

// ValidateCategoryAssignment 禁止把商品直接分配给拥有子分类的父分类。
func ValidateCategoryAssignment(categoryRepo CategoryAssignmentRepository, categoryID uint, currentCategoryID uint, invalidError error) error {
	if categoryID == 0 || categoryRepo == nil {
		return nil
	}
	if invalidError == nil {
		invalidError = ErrCategoryInvalid
	}

	categoryIDText := strconv.FormatUint(uint64(categoryID), 10)
	category, err := categoryRepo.GetByID(categoryIDText)
	if err != nil {
		return err
	}
	if category == nil {
		return invalidError
	}

	childCount, err := categoryRepo.CountChildren(categoryIDText)
	if err != nil {
		return err
	}
	if childCount > 0 && categoryID != currentCategoryID {
		return invalidError
	}
	return nil
}

// NormalizePurchaseQuantityLimit 归一化商品单次购买数量上下限；非正数表示不限制。
func NormalizePurchaseQuantityLimit(value int) int {
	if value <= 0 {
		return 0
	}
	return value
}

func productMaxPurchaseQuantity(product *Product) int {
	if product == nil {
		return 0
	}
	return NormalizePurchaseQuantityLimit(product.MaxPurchaseQuantity)
}

func productMinPurchaseQuantity(product *Product) int {
	if product == nil {
		return 0
	}
	return NormalizePurchaseQuantityLimit(product.MinPurchaseQuantity)
}

// ValidatePurchaseQuantity 校验单次购买数量是否在商品上下限内。
func ValidatePurchaseQuantity(product *Product, quantity int) error {
	if quantity <= 0 {
		return ErrPurchaseQuantityInvalid
	}
	if minLimit := productMinPurchaseQuantity(product); minLimit > 0 && quantity < minLimit {
		return ErrMinPurchaseNotMet
	}
	if maxLimit := productMaxPurchaseQuantity(product); maxLimit > 0 && quantity > maxLimit {
		return ErrMaxPurchaseExceeded
	}
	return nil
}
