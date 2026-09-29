package domain

import (
	"time"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
)

// OrderSnapshot 订单分销定价快照。
type OrderSnapshot struct {
	ID                  uint         `gorm:"primarykey" json:"id"`
	OrderID             uint         `gorm:"not null;uniqueIndex" json:"order_id"`
	ResellerID          uint         `gorm:"not null;index" json:"reseller_id"`
	Domain              string       `gorm:"type:varchar(255);not null;index" json:"domain"`
	Currency            string       `gorm:"type:varchar(16);not null" json:"currency"`
	ResellerUserID      uint         `gorm:"not null;index" json:"reseller_user_id"`
	BuyerUserID         uint         `gorm:"not null;default:0;index" json:"buyer_user_id"`
	BaseAmount          money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"base_amount"`
	ResellerAmount      money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"reseller_amount"`
	ProfitAmount        money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"profit_amount"`
	ProfitEligible      bool         `gorm:"not null;default:true;index" json:"profit_eligible"`
	ProfitBlockReason   string       `gorm:"type:varchar(64);index" json:"profit_block_reason"`
	PricingSnapshotJSON jsonmap.JSON `gorm:"type:json" json:"pricing_snapshot_json"`
	RiskSnapshotJSON    jsonmap.JSON `gorm:"type:json" json:"risk_snapshot_json"`
	CreatedAt           time.Time    `gorm:"index" json:"created_at"`
	UpdatedAt           time.Time    `gorm:"index" json:"updated_at"`
	DeletedAt           *time.Time   `gorm:"index" json:"-"`

	Order orderdomain.Order `gorm:"foreignKey:OrderID" json:"-"`
}

func (OrderSnapshot) TableName() string { return "reseller_order_snapshots" }
