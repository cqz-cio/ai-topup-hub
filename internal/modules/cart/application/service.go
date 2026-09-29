package application

import (
	"strconv"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/cart/contract"
	"github.com/dujiao-next/internal/modules/cart/domain"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	promotionapp "github.com/dujiao-next/internal/modules/promotion/application"
	promotioncontract "github.com/dujiao-next/internal/modules/promotion/contract"
)

// Service 购物车服务。
type Service struct {
	cartRepo       contract.Repository
	productRepo    contract.ProductReader
	productSKURepo contract.SKUReader
	promotionRepo  promotioncontract.Repository
	currencyReader contract.CurrencyReader
}

// NewService 创建购物车服务。
func NewService(cartRepo contract.Repository, productRepo contract.ProductReader, productSKURepo contract.SKUReader, promotionRepo promotioncontract.Repository, currencyReader contract.CurrencyReader) *Service {
	return &Service{
		cartRepo:       cartRepo,
		productRepo:    productRepo,
		productSKURepo: productSKURepo,
		promotionRepo:  promotionRepo,
		currencyReader: currencyReader,
	}
}

// ListByUser 获取用户购物车
func (s *Service) ListByUser(userID uint) ([]ItemDetail, error) {
	if userID == 0 {
		return nil, contract.ErrInvalidItem
	}
	items, err := s.cartRepo.ListByUser(userID)
	if err != nil {
		return nil, err
	}
	currency := s.siteCurrency()
	details := make([]ItemDetail, 0, len(items))
	promotionService := promotionapp.NewService(s.promotionRepo)
	for _, item := range items {
		product := item.Product
		if product == nil || product.ID == 0 {
			p, err := s.productRepo.GetByID(strconv.FormatUint(uint64(item.ProductID), 10))
			if err != nil {
				return nil, err
			}
			product = p
		}
		if product == nil || !product.IsActive {
			_ = s.cartRepo.DeleteByUserProductSKU(userID, item.ProductID, item.SKUID)
			continue
		}

		sku := item.SKU
		if sku == nil || sku.ID == 0 {
			resolvedSKU, resolveErr := resolveProductSKU(s.productSKURepo, product, item.SKUID)
			if resolveErr != nil {
				_ = s.cartRepo.DeleteByUserProductSKU(userID, item.ProductID, item.SKUID)
				continue
			}
			sku = resolvedSKU
		}

		if sku == nil || !sku.IsActive {
			_ = s.cartRepo.DeleteByUserProductSKU(userID, item.ProductID, item.SKUID)
			continue
		}
		if strings.TrimSpace(product.FulfillmentType) == constants.FulfillmentTypeManual &&
			productdomain.ShouldEnforceManualSKUStock(product, sku) &&
			productdomain.ManualSKUAvailable(sku) <= 0 {
			_ = s.cartRepo.DeleteByUserProductSKU(userID, item.ProductID, item.SKUID)
			continue
		}

		priceCarrier := *product
		priceCarrier.PriceAmount = sku.PriceAmount
		unitPrice := sku.PriceAmount
		if promotionService != nil {
			_, discounted, err := promotionService.ApplyPromotion(&priceCarrier, item.Quantity)
			if err != nil {
				return nil, err
			}
			unitPrice = discounted
		}

		fulfillmentType := strings.TrimSpace(product.FulfillmentType)
		if fulfillmentType == "" {
			fulfillmentType = constants.FulfillmentTypeManual
		}

		details = append(details, ItemDetail{
			ProductID:       item.ProductID,
			SKUID:           sku.ID,
			Quantity:        item.Quantity,
			FulfillmentType: fulfillmentType,
			UnitPrice:       unitPrice,
			OriginalPrice:   sku.PriceAmount,
			Currency:        currency,
			Product:         product,
			SKU:             sku,
		})
	}
	return details, nil
}

// UpsertItem 添加或更新购物车项
func (s *Service) UpsertItem(input UpsertItemInput) error {
	if input.UserID == 0 || input.ProductID == 0 || input.Quantity <= 0 {
		return contract.ErrInvalidItem
	}
	product, err := s.productRepo.GetByID(strconv.FormatUint(uint64(input.ProductID), 10))
	if err != nil {
		return err
	}
	if product == nil || !product.IsActive {
		return contract.ErrProductUnavailable
	}
	if err := productdomain.ValidatePurchaseQuantity(product, input.Quantity); err != nil {
		return err
	}
	sku, err := resolveProductSKU(s.productSKURepo, product, input.SKUID)
	if err != nil {
		return err
	}

	fulfillmentType := strings.TrimSpace(product.FulfillmentType)
	if fulfillmentType == "" {
		fulfillmentType = constants.FulfillmentTypeManual
	}
	if fulfillmentType != constants.FulfillmentTypeManual && fulfillmentType != constants.FulfillmentTypeAuto {
		return contract.ErrFulfillmentInvalid
	}
	if fulfillmentType == constants.FulfillmentTypeManual &&
		productdomain.ShouldEnforceManualSKUStock(product, sku) &&
		productdomain.ManualSKUAvailable(sku) < input.Quantity {
		return contract.ErrManualStockInsufficient
	}

	now := time.Now()
	item := &domain.Item{
		UserID:          input.UserID,
		ProductID:       input.ProductID,
		SKUID:           sku.ID,
		Quantity:        input.Quantity,
		FulfillmentType: fulfillmentType,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	return s.cartRepo.Upsert(item)
}

// RemoveItem 删除购物车项
func (s *Service) RemoveItem(userID, productID, skuID uint) error {
	if userID == 0 || productID == 0 {
		return contract.ErrInvalidItem
	}
	return s.cartRepo.DeleteByUserProductSKU(userID, productID, skuID)
}

func (s *Service) siteCurrency() string {
	if s == nil || s.currencyReader == nil {
		return constants.SiteCurrencyDefault
	}
	currency, err := s.currencyReader.GetSiteCurrency(constants.SiteCurrencyDefault)
	if err != nil || strings.TrimSpace(currency) == "" {
		return constants.SiteCurrencyDefault
	}
	return currency
}

func resolveProductSKU(repo contract.SKUReader, product *productdomain.Product, rawSKUID uint) (*productdomain.ProductSKU, error) {
	if product == nil || product.ID == 0 {
		return nil, contract.ErrProductUnavailable
	}
	if repo == nil {
		return nil, contract.ErrSKUInvalid
	}
	if rawSKUID > 0 {
		sku, err := repo.GetByID(rawSKUID)
		if err != nil {
			return nil, err
		}
		if sku == nil || sku.ProductID != product.ID || !sku.IsActive {
			return nil, contract.ErrSKUInvalid
		}
		return sku, nil
	}
	activeSKUs, err := repo.ListByProduct(product.ID, true)
	if err != nil {
		return nil, err
	}
	if len(activeSKUs) == 1 {
		return &activeSKUs[0], nil
	}
	if len(activeSKUs) == 0 {
		return nil, contract.ErrSKUInvalid
	}
	return nil, contract.ErrSKURequired
}
