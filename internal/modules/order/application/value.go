package application

import (
	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"

	"github.com/dujiao-next/internal/constants"
)

func resolveSiteCurrency(settingService *settingsapp.Service) string {
	if settingService == nil {
		return constants.SiteCurrencyDefault
	}
	currency, err := settingService.GetSiteCurrency(constants.SiteCurrencyDefault)
	if err != nil {
		return constants.SiteCurrencyDefault
	}
	return currency
}

// ResolvePaymentExpireMinutes 解析订单与支付共同使用的付款超时时间。
func ResolvePaymentExpireMinutes(settingService *settingsapp.Service, defaultMinutes int) int {
	if defaultMinutes <= 0 {
		defaultMinutes = 15
	}
	if settingService == nil {
		return defaultMinutes
	}
	minutes, err := settingService.GetOrderPaymentExpireMinutes(defaultMinutes)
	if err != nil || minutes <= 0 {
		return defaultMinutes
	}
	return minutes
}

func resolveProductOrderSKU(productSKURepo productcontract.SKURepository, product *productdomain.Product, rawSKUID uint) (*productdomain.ProductSKU, error) {
	if product == nil || product.ID == 0 {
		return nil, ErrProductNotAvailable
	}
	if productSKURepo == nil {
		return nil, ErrProductSKUInvalid
	}
	if rawSKUID > 0 {
		sku, err := productSKURepo.GetByID(rawSKUID)
		if err != nil {
			return nil, err
		}
		if sku == nil || sku.ProductID != product.ID || !sku.IsActive {
			return nil, ErrProductSKUInvalid
		}
		return sku, nil
	}

	activeSKUs, err := productSKURepo.ListByProduct(product.ID, true)
	if err != nil {
		return nil, err
	}
	if len(activeSKUs) == 1 {
		return &activeSKUs[0], nil
	}
	if len(activeSKUs) == 0 {
		return nil, ErrProductSKUInvalid
	}
	return nil, ErrProductSKURequired
}
