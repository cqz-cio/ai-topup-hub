package categorypresenter

import (
	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

// Category 是公开分类响应；不暴露数据库软删除和内部时间字段。
type Category struct {
	ID        uint         `json:"id"`
	ParentID  uint         `json:"parent_id"`
	Slug      string       `json:"slug"`
	Name      jsonmap.JSON `json:"name"`
	Icon      string       `json:"icon,omitempty"`
	SortOrder int          `json:"sort_order"`
}

func New(category *categorydomain.Category) Category {
	return Category{
		ID:        category.ID,
		ParentID:  category.ParentID,
		Slug:      category.Slug,
		Name:      category.NameJSON,
		Icon:      category.Icon,
		SortOrder: category.SortOrder,
	}
}

func List(categories []categorydomain.Category) []Category {
	result := make([]Category, 0, len(categories))
	for index := range categories {
		result = append(result, New(&categories[index]))
	}
	return result
}
