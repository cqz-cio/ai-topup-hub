package contract

import "errors"

var (
	ErrNotFound              = errors.New("procurement order not found")
	ErrExists                = errors.New("procurement order already exists")
	ErrStatusInvalid         = errors.New("procurement order status invalid")
	ErrOrderNotFound         = errors.New("order not found")
	ErrConnectionNotFound    = errors.New("site connection not found")
	ErrOutsidePurchaseWindow = errors.New("historical orders cannot be retried for upstream purchase")
)
