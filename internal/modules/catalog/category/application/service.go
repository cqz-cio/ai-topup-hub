package categoryapp

import (
	"errors"
	"strconv"

	categorycontract "github.com/dujiao-next/internal/modules/catalog/category/contract"
	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	catalogslug "github.com/dujiao-next/internal/shared/slug"
)

var (
	ErrNotFound      = errors.New("category not found")
	ErrSlugExists    = categorycontract.ErrSlugExists
	ErrParentInvalid = errors.New("category parent invalid")
	ErrInUse         = errors.New("category in use")
)

// Service 承载分类查询、创建、更新、启停和删除用例。
type Service struct {
	repository categorycontract.Repository
}

func NewService(repository categorycontract.Repository) *Service {
	return &Service{repository: repository}
}

// UpsertInput 是创建与更新分类的应用层输入。
type UpsertInput struct {
	ParentID    uint
	Slug        string
	NameJSON    map[string]interface{}
	Icon        string
	SortOrder   int
	SeoMetaJSON map[string]interface{}
}

func (service *Service) SuggestSlug(title string, excludeID *string) (string, error) {
	return catalogslug.Unique(catalogslug.Generate(title), func(value string) (bool, error) {
		count, err := service.repository.CountBySlug(value, excludeID)
		return count > 0, err
	})
}

func (service *Service) ResolvePublicSlug(value string) (string, error) {
	if aliases, ok := service.repository.(interface{ ResolvePublicSlug(string) (string, error) }); ok {
		return aliases.ResolvePublicSlug(value)
	}
	return value, nil
}

func (service *Service) List() ([]categorydomain.Category, error) {
	return service.repository.List()
}

func (service *Service) ListActive() ([]categorydomain.Category, error) {
	return service.repository.ListActive()
}

func (service *Service) SetActive(id string, active bool) (*categorydomain.Category, error) {
	category, err := service.repository.GetByID(id)
	if err != nil {
		return nil, err
	}
	if category == nil {
		return nil, ErrNotFound
	}
	if category.IsActive == active {
		return category, nil
	}
	if err := service.repository.UpdateActive(id, active); err != nil {
		return nil, err
	}
	category.IsActive = active
	return category, nil
}

func (service *Service) Create(input UpsertInput) (*categorydomain.Category, error) {
	if err := catalogslug.Validate(input.Slug); err != nil {
		return nil, err
	}
	if err := service.validateParent(nil, input.ParentID); err != nil {
		return nil, err
	}
	count, err := service.repository.CountBySlug(input.Slug, nil)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrSlugExists
	}
	category := categorydomain.Category{
		ParentID:    input.ParentID,
		Slug:        input.Slug,
		NameJSON:    jsonmap.JSON(input.NameJSON),
		Icon:        input.Icon,
		SortOrder:   input.SortOrder,
		IsActive:    true,
		SeoMetaJSON: jsonmap.JSON(input.SeoMetaJSON),
	}
	if err := service.repository.Create(&category); err != nil {
		return nil, err
	}
	return &category, nil
}

func (service *Service) Update(id string, input UpsertInput) (*categorydomain.Category, error) {
	if err := catalogslug.Validate(input.Slug); err != nil {
		return nil, err
	}
	category, err := service.repository.GetByID(id)
	if err != nil {
		return nil, err
	}
	if category == nil {
		return nil, ErrNotFound
	}
	if err := service.validateParent(category, input.ParentID); err != nil {
		return nil, err
	}
	count, err := service.repository.CountBySlug(input.Slug, &id)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrSlugExists
	}
	category.ParentID = input.ParentID
	category.Slug = input.Slug
	category.NameJSON = jsonmap.JSON(input.NameJSON)
	category.Icon = input.Icon
	category.SortOrder = input.SortOrder
	if input.SeoMetaJSON != nil {
		category.SeoMetaJSON = jsonmap.JSON(input.SeoMetaJSON)
	}
	if err := service.repository.Update(category); err != nil {
		return nil, err
	}
	return category, nil
}

func (service *Service) Delete(id string) error {
	category, err := service.repository.GetByID(id)
	if err != nil {
		return err
	}
	if category == nil {
		return ErrNotFound
	}
	childCount, err := service.repository.CountChildren(id)
	if err != nil {
		return err
	}
	if childCount > 0 {
		return ErrInUse
	}
	count, err := service.repository.CountProducts(id)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrInUse
	}
	return service.repository.Delete(id)
}

func (service *Service) validateParent(category *categorydomain.Category, parentID uint) error {
	if parentID == 0 {
		return nil
	}
	if category != nil && category.ID == parentID {
		return ErrParentInvalid
	}
	parent, err := service.repository.GetByID(strconv.FormatUint(uint64(parentID), 10))
	if err != nil {
		return err
	}
	if parent == nil || parent.ParentID != 0 {
		return ErrParentInvalid
	}
	if category != nil && category.ParentID == 0 {
		childCount, err := service.repository.CountChildren(strconv.FormatUint(uint64(category.ID), 10))
		if err != nil {
			return err
		}
		if childCount > 0 {
			return ErrParentInvalid
		}
	}
	return nil
}
