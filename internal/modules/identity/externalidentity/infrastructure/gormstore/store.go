package externalidentitystore

import (
	"errors"
	"strings"

	externalidentitycontract "github.com/dujiao-next/internal/modules/identity/externalidentity/contract"
	externalidentitydomain "github.com/dujiao-next/internal/modules/identity/externalidentity/domain"

	"gorm.io/gorm"
)

type Store struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

// GetByProviderUserID 按提供方用户ID查询绑定
func (r *Store) GetByProviderUserID(provider, providerUserID string) (*externalidentitydomain.Identity, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	providerUserID = strings.TrimSpace(providerUserID)
	if provider == "" || providerUserID == "" {
		return nil, nil
	}
	var identity externalidentitydomain.Identity
	if err := r.db.Where("provider = ? AND provider_user_id = ?", provider, providerUserID).First(&identity).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &identity, nil
}

// GetByUserProvider 按用户查询某个提供方绑定
func (r *Store) GetByUserProvider(userID uint, provider string) (*externalidentitydomain.Identity, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if userID == 0 || provider == "" {
		return nil, nil
	}
	var identity externalidentitydomain.Identity
	if err := r.db.Where("user_id = ? AND provider = ?", userID, provider).First(&identity).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &identity, nil
}

// ListByUserID 查询用户全部第三方绑定。
func (r *Store) ListByUserID(userID uint) ([]externalidentitydomain.Identity, error) {
	if userID == 0 {
		return []externalidentitydomain.Identity{}, nil
	}
	var identities []externalidentitydomain.Identity
	if err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&identities).Error; err != nil {
		return nil, err
	}
	return identities, nil
}

// ListTelegramUsers 查询 Telegram 用户候选列表。
func (r *Store) ListTelegramUsers(filter externalidentitycontract.TelegramUserFilter) ([]externalidentitycontract.TelegramUser, int64, error) {
	query := r.db.Table("user_oauth_identities").
		Select(""+
			"users.id AS user_id, "+
			"users.display_name AS display_name, "+
			"users.email AS user_email, "+
			"user_oauth_identities.username AS telegram_username, "+
			"user_oauth_identities.provider_user_id AS telegram_user_id, "+
			"user_oauth_identities.created_at AS bound_at, "+
			"users.created_at AS user_created_at").
		Joins("JOIN users ON users.id = user_oauth_identities.user_id").
		Where("user_oauth_identities.provider = ?", "telegram").
		Where("users.deleted_at IS NULL")

	if len(filter.UserIDs) > 0 {
		query = query.Where("users.id IN ?", filter.UserIDs)
	}

	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where(
			"users.display_name LIKE ? OR user_oauth_identities.username LIKE ? OR user_oauth_identities.provider_user_id LIKE ?",
			like, like, like,
		)
	}
	if value := strings.TrimSpace(filter.DisplayName); value != "" {
		query = query.Where("users.display_name LIKE ?", "%"+value+"%")
	}
	if value := strings.TrimSpace(filter.TelegramUsername); value != "" {
		query = query.Where("user_oauth_identities.username LIKE ?", "%"+value+"%")
	}
	if value := strings.TrimSpace(filter.TelegramUserID); value != "" {
		query = query.Where("user_oauth_identities.provider_user_id LIKE ?", "%"+value+"%")
	}
	if filter.CreatedFrom != nil {
		query = query.Where("user_oauth_identities.created_at >= ?", *filter.CreatedFrom)
	}
	if filter.CreatedTo != nil {
		query = query.Where("user_oauth_identities.created_at <= ?", *filter.CreatedTo)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if filter.PageSize > 0 {
		page := filter.Page
		if page < 1 {
			page = 1
		}
		query = query.Offset((page - 1) * filter.PageSize).Limit(filter.PageSize)
	}

	var items []externalidentitycontract.TelegramUser
	if err := query.Order("user_oauth_identities.created_at DESC").Scan(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Create 创建绑定
func (r *Store) Create(identity *externalidentitydomain.Identity) error {
	if identity == nil {
		return nil
	}
	return r.db.Create(identity).Error
}

// Update 更新绑定
func (r *Store) Update(identity *externalidentitydomain.Identity) error {
	if identity == nil {
		return nil
	}
	return r.db.Save(identity).Error
}

// DeleteByID 删除绑定
func (r *Store) DeleteByID(id uint) error {
	if id == 0 {
		return nil
	}
	return r.db.Delete(&externalidentitydomain.Identity{}, id).Error
}
