package contract

import (
	"time"

	"github.com/dujiao-next/internal/shared/money"
)

type JobListFilter struct {
	Page         int
	PageSize     int
	ConnectionID uint
	Status       string
	Type         string
}

type RunInput struct {
	ConnectionID   uint      `json:"connection_id" binding:"required"`
	Type           string    `json:"type" binding:"required"`
	TimeRangeStart time.Time `json:"time_range_start" binding:"required"`
	TimeRangeEnd   time.Time `json:"time_range_end" binding:"required"`
}

// ProcurementOrder 是对账从采购域读取的最小快照。
type ProcurementOrder struct {
	ID              uint
	UpstreamOrderID uint
	LocalOrderNo    string
	UpstreamOrderNo string
	Status          string
	UpstreamAmount  money.Amount
}

// UpstreamOrder 是上游订单查询返回的最小快照。
type UpstreamOrder struct {
	Status string
	Amount string
}
