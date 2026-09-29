package categorydomain

import (
	"time"

	"github.com/dujiao-next/internal/shared/jsonmap"
)

// Category 分类表
type Category struct {
	ID        uint         `gorm:"primarykey" json:"id"`                         // 主键
	ParentID  uint         `gorm:"not null;default:0;index" json:"parent_id"`    // 父分类ID，0 表示一级分类
	Slug      string       `gorm:"uniqueIndex;not null" json:"slug"`             // 唯一标识
	NameJSON  jsonmap.JSON `gorm:"type:json;not null" json:"name"`               // 多语言名称
	Icon      string       `gorm:"type:varchar(500)" json:"icon"`                // 分类图标（图片路径）
	SortOrder int          `gorm:"default:0;index" json:"sort_order"`            // 排序权重
	IsActive  bool         `gorm:"not null;default:true;index" json:"is_active"` // 是否启用，停用后前台不展示
	CreatedAt time.Time    `gorm:"index" json:"created_at"`                      // 创建时间
	DeletedAt *time.Time   `gorm:"index" json:"-"`                               // 软删除时间
}

// TableName 指定表名
func (Category) TableName() string {
	return "categories"
}
