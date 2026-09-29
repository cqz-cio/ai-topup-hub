package application

import (
	"context"

	"github.com/dujiao-next/internal/modules/content/contract"
	"github.com/dujiao-next/internal/modules/content/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

// CreatePostCategoryInput 描述文章分类创建和更新所需字段。
type CreatePostCategoryInput struct {
	NameJSON  jsonmap.JSON
	Slug      string
	ParentID  *uint
	SortOrder int
	Icon      string
}

// PostCategoryService 实现文章分类用例。
type PostCategoryService struct {
	store contract.PostCategoryStore
}

// NewPostCategoryService 创建文章分类用例服务。
func NewPostCategoryService(store contract.PostCategoryStore) *PostCategoryService {
	return &PostCategoryService{store: store}
}

// ListAll 获取所有文章分类。
func (s *PostCategoryService) ListAll(ctx context.Context, parentID *uint) ([]domain.PostCategory, error) {
	return s.store.ListAll(ctx, parentID)
}

// ListTree 获取全部分类树，包含禁用分类。
func (s *PostCategoryService) ListTree(ctx context.Context) ([]domain.PostCategory, error) {
	return s.store.ListTree(ctx)
}

// GetByID 获取单个分类。
func (s *PostCategoryService) GetByID(ctx context.Context, id uint) (*domain.PostCategory, error) {
	category, err := s.store.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if category == nil {
		return nil, contract.ErrNotFound
	}
	return category, nil
}

// Create 创建文章分类。
func (s *PostCategoryService) Create(ctx context.Context, input CreatePostCategoryInput) (*domain.PostCategory, error) {
	parentID := normalizeParentID(input.ParentID)
	if err := s.validateParent(ctx, nil, parentID); err != nil {
		return nil, err
	}

	count, err := s.store.CountBySlug(ctx, input.Slug, nil)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, contract.ErrSlugExists
	}

	category := &domain.PostCategory{
		NameJSON:  input.NameJSON,
		Slug:      input.Slug,
		ParentID:  parentID,
		SortOrder: input.SortOrder,
		Icon:      input.Icon,
		IsActive:  true,
	}
	if err := s.store.Create(ctx, category); err != nil {
		return nil, err
	}
	return category, nil
}

// Update 更新文章分类。
func (s *PostCategoryService) Update(ctx context.Context, id uint, input CreatePostCategoryInput) (*domain.PostCategory, error) {
	category, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	parentID := normalizeParentID(input.ParentID)
	if err := s.validateParent(ctx, category, parentID); err != nil {
		return nil, err
	}

	count, err := s.store.CountBySlug(ctx, input.Slug, &id)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, contract.ErrSlugExists
	}

	if input.NameJSON != nil {
		category.NameJSON = input.NameJSON
	}
	if input.Slug != "" {
		category.Slug = input.Slug
	}
	category.ParentID = parentID
	category.SortOrder = input.SortOrder
	category.Icon = input.Icon

	if err := s.store.Update(ctx, category); err != nil {
		return nil, err
	}
	return category, nil
}

// Delete 软删除文章分类；存在子分类或文章时拒绝删除。
func (s *PostCategoryService) Delete(ctx context.Context, id uint) error {
	if _, err := s.GetByID(ctx, id); err != nil {
		return err
	}

	childCount, err := s.store.CountChildren(ctx, id)
	if err != nil {
		return err
	}
	if childCount > 0 {
		return contract.ErrCategoryInUse
	}

	postCount, err := s.store.CountPostsByCategory(ctx, id)
	if err != nil {
		return err
	}
	if postCount > 0 {
		return contract.ErrCategoryInUse
	}

	return s.store.Delete(ctx, id)
}

// ListActive 获取所有激活分类。
func (s *PostCategoryService) ListActive(ctx context.Context) ([]domain.PostCategory, error) {
	return s.store.ListActive(ctx)
}

// SetActive 切换分类启用状态。
func (s *PostCategoryService) SetActive(ctx context.Context, id uint, active bool) (*domain.PostCategory, error) {
	category, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if category.IsActive == active {
		return category, nil
	}
	if err := s.store.UpdateActive(ctx, id, active); err != nil {
		return nil, err
	}
	category.IsActive = active
	return category, nil
}

func (s *PostCategoryService) validateParent(ctx context.Context, category *domain.PostCategory, parentID *uint) error {
	if parentID == nil || *parentID == 0 {
		return nil
	}
	if category != nil && category.ID == *parentID {
		return contract.ErrCategoryParentInvalid
	}

	parent, err := s.store.GetByID(ctx, *parentID)
	if err != nil {
		return err
	}
	if parent == nil || (parent.ParentID != nil && *parent.ParentID != 0) {
		return contract.ErrCategoryParentInvalid
	}

	if category != nil && (category.ParentID == nil || *category.ParentID == 0) {
		childCount, err := s.store.CountChildren(ctx, category.ID)
		if err != nil {
			return err
		}
		if childCount > 0 {
			return contract.ErrCategoryParentInvalid
		}
	}
	return nil
}

func normalizeParentID(parentID *uint) *uint {
	if parentID != nil && *parentID == 0 {
		return nil
	}
	return parentID
}
