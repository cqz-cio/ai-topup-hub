package domain

// OrderReference is the read-only projection affiliate queries need when
// preloading a commission's order.
type OrderReference struct {
	ID      uint   `gorm:"primarykey" json:"id"`
	OrderNo string `gorm:"column:order_no" json:"order_no"`
}

func (OrderReference) TableName() string {
	return "orders"
}
