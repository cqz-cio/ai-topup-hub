package domain

import (
	"time"

	"github.com/dujiao-next/internal/shared/jsonmap"
)

// PostCategory 文章分类（支持两级）。
type PostCategory struct {
	ID        uint         `gorm:"primarykey" json:"id"`
	ParentID  *uint        `gorm:"index" json:"parent_id"`
	Slug      string       `gorm:"uniqueIndex;not null" json:"slug"`
	NameJSON  jsonmap.JSON `gorm:"type:json;not null" json:"name"`
	Icon      string       `gorm:"type:varchar(500)" json:"icon"`
	IsActive  bool         `gorm:"not null;default:true;index" json:"is_active"`
	SortOrder int          `gorm:"default:0;index" json:"sort_order"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
	DeletedAt *time.Time   `gorm:"index" json:"-"`

	Children []PostCategory `gorm:"foreignkey:ParentID" json:"children,omitempty"`
}

func (PostCategory) TableName() string {
	return "post_categories"
}
