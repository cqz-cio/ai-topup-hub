package domain

import (
	"time"

	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
)

// Job 是对账任务聚合根。
type Job struct {
	ID              uint       `gorm:"primarykey" json:"id"`
	ConnectionID    uint       `gorm:"index;not null" json:"connection_id"`
	Type            string     `gorm:"type:varchar(20);not null" json:"type"`
	Status          string     `gorm:"type:varchar(20);not null;default:'pending'" json:"status"`
	TimeRangeStart  time.Time  `json:"time_range_start"`
	TimeRangeEnd    time.Time  `json:"time_range_end"`
	TotalCount      int        `gorm:"not null;default:0" json:"total_count"`
	MatchedCount    int        `gorm:"not null;default:0" json:"matched_count"`
	MismatchedCount int        `gorm:"not null;default:0" json:"mismatched_count"`
	ResultJSON      string     `gorm:"type:text" json:"result_json,omitempty"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
	CreatedAt       time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"index" json:"updated_at"`

	Connection *siteconnectiondomain.Connection `gorm:"foreignKey:ConnectionID" json:"connection,omitempty"`
}

func (Job) TableName() string { return "reconciliation_jobs" }
