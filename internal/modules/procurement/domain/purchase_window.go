package domain

import "time"

// PurchaseWindow is captured once when purchasing is resumed. IDs exclude all
// pre-existing orders, including delayed historical callbacks, while PaidAfter
// requires a newly confirmed payment. Never move this boundary on restart.
type PurchaseWindow struct {
	OrderIDFloor uint
	PaidAfter    time.Time
}
