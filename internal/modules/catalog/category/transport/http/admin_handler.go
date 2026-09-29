package categoryhttp

import (
	"errors"

	categoryapp "github.com/dujiao-next/internal/modules/catalog/category/application"
	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// CategoryService 是后台分类 HTTP 端点所需的最小用例接口。
type CategoryService interface {
	List() ([]categorydomain.Category, error)
	Create(input categoryapp.UpsertInput) (*categorydomain.Category, error)
	Update(id string, input categoryapp.UpsertInput) (*categorydomain.Category, error)
	SetActive(id string, active bool) (*categorydomain.Category, error)
	Delete(id string) error
}

// AdminCategoryHandler 处理后台商品分类管理请求。
type AdminCategoryHandler struct {
	service CategoryService
}

func NewAdminCategoryHandler(service CategoryService) *AdminCategoryHandler {
	if service == nil {
		panic("catalog admin category handler: service is nil")
	}
	return &AdminCategoryHandler{service: service}
}

// GetAdminCategories 获取分类列表 (Admin)
func (h *AdminCategoryHandler) GetAdminCategories(c *gin.Context) {
	categories, err := h.service.List()
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.category_fetch_failed", err)
		return
	}

	response.Success(c, categories)
}

// ====================  分类管理  ====================

// CreateCategoryRequest 创建分类请求
type CreateCategoryRequest struct {
	ParentID  uint                   `json:"parent_id"`
	Slug      string                 `json:"slug" binding:"required"`
	NameJSON  map[string]interface{} `json:"name" binding:"required"`
	Icon      string                 `json:"icon"`
	SortOrder int                    `json:"sort_order"`
}

// CreateCategory 创建分类
func (h *AdminCategoryHandler) CreateCategory(c *gin.Context) {
	var req CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	category, err := h.service.Create(categoryapp.UpsertInput{
		ParentID:  req.ParentID,
		Slug:      req.Slug,
		NameJSON:  req.NameJSON,
		Icon:      req.Icon,
		SortOrder: req.SortOrder,
	})
	if err != nil {
		if errors.Is(err, categoryapp.ErrSlugExists) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.slug_exists", nil)
			return
		}
		if errors.Is(err, categoryapp.ErrParentInvalid) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.category_parent_invalid", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.category_create_failed", err)
		return
	}

	response.Success(c, category)
}

// UpdateCategory 更新分类
func (h *AdminCategoryHandler) UpdateCategory(c *gin.Context) {
	id := c.Param("id")

	var req CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	category, err := h.service.Update(id, categoryapp.UpsertInput{
		ParentID:  req.ParentID,
		Slug:      req.Slug,
		NameJSON:  req.NameJSON,
		Icon:      req.Icon,
		SortOrder: req.SortOrder,
	})
	if err != nil {
		if errors.Is(err, categoryapp.ErrNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.category_not_found", nil)
			return
		}
		if errors.Is(err, categoryapp.ErrSlugExists) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.slug_used", nil)
			return
		}
		if errors.Is(err, categoryapp.ErrParentInvalid) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.category_parent_invalid", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.category_update_failed", err)
		return
	}

	response.Success(c, category)
}

// PatchCategoryActiveRequest 切换启用状态请求
type PatchCategoryActiveRequest struct {
	IsActive bool `json:"is_active"`
}

// PatchCategoryActive 切换分类启用状态
func (h *AdminCategoryHandler) PatchCategoryActive(c *gin.Context) {
	id := c.Param("id")

	var req PatchCategoryActiveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	category, err := h.service.SetActive(id, req.IsActive)
	if err != nil {
		if errors.Is(err, categoryapp.ErrNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.category_not_found", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.category_update_failed", err)
		return
	}

	response.Success(c, category)
}

// DeleteCategory 删除分类（软删除）
func (h *AdminCategoryHandler) DeleteCategory(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.Delete(id); err != nil {
		if errors.Is(err, categoryapp.ErrInUse) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.category_in_use", nil)
			return
		}
		if errors.Is(err, categoryapp.ErrNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.category_not_found", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.category_delete_failed", err)
		return
	}

	response.Success(c, nil)
}
