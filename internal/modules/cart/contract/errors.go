package contract

import "errors"

var (
	ErrInvalidItem             = errors.New("invalid cart item")
	ErrProductUnavailable      = errors.New("product not available")
	ErrFulfillmentInvalid      = errors.New("fulfillment invalid")
	ErrSKURequired             = errors.New("product sku required")
	ErrSKUInvalid              = errors.New("product sku invalid")
	ErrManualStockInsufficient = errors.New("manual stock insufficient")
)
