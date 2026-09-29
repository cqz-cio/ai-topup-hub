package application

import "errors"

var (
	ErrNotFound           = errors.New("not found")
	ErrInsufficient       = errors.New("card secret insufficient")
	ErrInvalid            = errors.New("card secret invalid")
	ErrCreateFailed       = errors.New("card secret create failed")
	ErrFetchFailed        = errors.New("card secret fetch failed")
	ErrUpdateFailed       = errors.New("card secret update failed")
	ErrDeleteFailed       = errors.New("card secret delete failed")
	ErrBatchCreateFailed  = errors.New("card secret batch create failed")
	ErrBatchFetchFailed   = errors.New("card secret batch fetch failed")
	ErrImportFailed       = errors.New("card secret import failed")
	ErrStatsFailed        = errors.New("card secret stats failed")
	ErrProductFetchFailed = errors.New("product fetch failed")
	ErrProductNotFound    = errors.New("product not found")
	ErrProductSKURequired = errors.New("product sku required")
	ErrProductSKUInvalid  = errors.New("product sku invalid")
)
