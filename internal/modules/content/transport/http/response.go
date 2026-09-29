package contenthttp

import (
	contentcontract "github.com/dujiao-next/internal/modules/content/contract"
	contentdomain "github.com/dujiao-next/internal/modules/content/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

// AdminPostProductRef 是后台文章编辑回填使用的关联商品精简结构。
type AdminPostProductRef struct {
	ID    uint         `json:"id"`
	Slug  string       `json:"slug"`
	Title jsonmap.JSON `json:"title"`
	Image string       `json:"image,omitempty"`
}

func newAdminPostProductRefs(products []contentcontract.RelatedProduct) []AdminPostProductRef {
	refs := make([]AdminPostProductRef, 0, len(products))
	for index := range products {
		product := &products[index]
		ref := AdminPostProductRef{
			ID:    product.ID,
			Slug:  product.Slug,
			Title: product.Title,
		}
		if len(product.Images) > 0 {
			ref.Image = product.Images[0]
		}
		refs = append(refs, ref)
	}
	return refs
}

// PostCategoryDTO 是公开接口返回的文章分类，对齐商品分类响应形状。
type PostCategoryDTO struct {
	ID        uint                   `json:"id"`
	ParentID  uint                   `json:"parent_id"`
	Slug      string                 `json:"slug"`
	Name      map[string]interface{} `json:"name"`
	Icon      string                 `json:"icon"`
	SortOrder int                    `json:"sort_order"`
}

func newPostCategoryDTOs(categories []contentdomain.PostCategory) []PostCategoryDTO {
	result := make([]PostCategoryDTO, 0, len(categories))
	for _, category := range categories {
		result = append(result, PostCategoryDTO{
			ID:        category.ID,
			ParentID:  optionalUintOrZero(category.ParentID),
			Slug:      category.Slug,
			Name:      category.NameJSON,
			Icon:      category.Icon,
			SortOrder: category.SortOrder,
		})
	}
	return result
}

func optionalUintOrZero(value *uint) uint {
	if value == nil {
		return 0
	}
	return *value
}
