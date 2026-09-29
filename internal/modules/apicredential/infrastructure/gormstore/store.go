package gormstore

import (
	"errors"
	"time"

	apicredentialcontract "github.com/dujiao-next/internal/modules/apicredential/contract"
	apicredentialdomain "github.com/dujiao-next/internal/modules/apicredential/domain"

	"gorm.io/gorm"
)

type Store struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

// GetByID 根据 ID 获取
func (r *Store) GetByID(id uint) (*apicredentialdomain.ApiCredential, error) {
	var cred apicredentialdomain.ApiCredential
	if err := r.db.Where("api_credentials.deleted_at IS NULL").First(&cred, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &cred, nil
}

// GetByUserID 根据用户 ID 获取
func (r *Store) GetByUserID(userID uint) (*apicredentialdomain.ApiCredential, error) {
	var cred apicredentialdomain.ApiCredential
	if err := r.db.Where("deleted_at IS NULL AND user_id = ?", userID).First(&cred).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &cred, nil
}

// GetAnyByUserID 根据用户 ID 获取，包含软删除记录。
func (r *Store) GetAnyByUserID(userID uint) (*apicredentialdomain.ApiCredential, error) {
	var cred apicredentialdomain.ApiCredential
	if err := r.db.Where("user_id = ?", userID).First(&cred).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &cred, nil
}

// GetByApiKey 根据 API Key 获取（预加载 User 用于状态校验）
func (r *Store) GetByApiKey(apiKey string) (*apicredentialdomain.ApiCredential, error) {
	var cred apicredentialdomain.ApiCredential
	if err := r.db.Preload("User", "deleted_at IS NULL").Where("api_credentials.deleted_at IS NULL AND api_key = ?", apiKey).First(&cred).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &cred, nil
}

// Create 创建凭证
func (r *Store) Create(cred *apicredentialdomain.ApiCredential) error {
	return r.db.Create(cred).Error
}

// Update 更新凭证
func (r *Store) Update(cred *apicredentialdomain.ApiCredential) error {
	return r.db.Save(cred).Error
}

// TouchLastUsedAt 只更新 last_used_at 单列，不覆盖其它字段。
func (r *Store) TouchLastUsedAt(id uint, at time.Time) error {
	return r.db.Model(&apicredentialdomain.ApiCredential{}).
		Where("id = ? AND deleted_at IS NULL", id).
		UpdateColumn("last_used_at", at).Error
}

// UpdateAny 更新凭证，包含软删除记录。
func (r *Store) UpdateAny(cred *apicredentialdomain.ApiCredential) error {
	return r.db.Save(cred).Error
}

// Delete 软删除凭证
func (r *Store) Delete(id uint) error {
	return r.db.Model(&apicredentialdomain.ApiCredential{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("deleted_at", time.Now()).Error
}

// List 列表查询
func (r *Store) List(filter apicredentialcontract.ListFilter) ([]apicredentialdomain.ApiCredential, int64, error) {
	var creds []apicredentialdomain.ApiCredential
	var total int64

	q := r.db.Model(&apicredentialdomain.ApiCredential{}).Where("api_credentials.deleted_at IS NULL")
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}
	if filter.UserID > 0 {
		q = q.Where("user_id = ?", filter.UserID)
	}
	if filter.Search != "" {
		// 按邮箱或昵称搜索：需要 JOIN users 表
		q = q.Joins("JOIN users ON users.id = api_credentials.user_id").
			Where("users.email LIKE ? OR users.display_name LIKE ?",
				"%"+filter.Search+"%", "%"+filter.Search+"%")
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	q = q.Order("api_credentials.created_at DESC")
	if filter.Page > 0 && filter.PageSize > 0 {
		q = q.Offset((filter.Page - 1) * filter.PageSize).Limit(filter.PageSize)
	}

	if err := q.Preload("User", "deleted_at IS NULL").Find(&creds).Error; err != nil {
		return nil, 0, err
	}

	return creds, total, nil
}

var _ apicredentialcontract.Repository = (*Store)(nil)
