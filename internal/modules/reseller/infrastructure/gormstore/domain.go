package gormstore

import (
	"errors"
	"strings"
	"time"

	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UpsertDomain 创建域名，或恢复同域名的软删除记录。
func (r *Store) UpsertDomain(input resellerdomain.Domain) (*resellerdomain.Domain, error) {
	input.Domain = normalizeDomainForRepository(input.Domain)
	if input.ResellerID == 0 || input.Domain == "" {
		return nil, errors.New("invalid reseller domain")
	}
	now := time.Now()
	var existing resellerdomain.Domain
	err := r.db.Unscoped().Where("domain = ?", input.Domain).First(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		input.CreatedAt = now
		input.UpdatedAt = now
		if err := r.db.Create(&input).Error; err != nil {
			return nil, err
		}
		return &input, nil
	}
	if existing.DeletedAt == nil {
		return nil, errors.New("reseller domain already exists")
	}
	existing.ResellerID = input.ResellerID
	existing.Type = input.Type
	existing.VerificationToken = input.VerificationToken
	existing.VerificationStatus = input.VerificationStatus
	existing.Status = input.Status
	existing.IsPrimary = input.IsPrimary
	existing.VerifiedAt = input.VerifiedAt
	existing.DeletedAt = nil
	existing.UpdatedAt = now
	if err := r.db.Unscoped().Save(&existing).Error; err != nil {
		return nil, err
	}
	return &existing, nil
}

// GetDomainByID 按 ID 获取域名。
func (r *Store) GetDomainByID(id uint) (*resellerdomain.Domain, error) {
	if id == 0 {
		return nil, nil
	}
	var row resellerdomain.Domain
	if err := r.db.Preload("Profile", "deleted_at IS NULL").
		Preload("Profile.User", "deleted_at IS NULL").
		Where("reseller_domains.id = ? AND reseller_domains.deleted_at IS NULL", id).
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// GetDomainByIDForUpdate 按 ID 获取并锁定域名。
func (r *Store) GetDomainByIDForUpdate(id uint) (*resellerdomain.Domain, error) {
	if id == 0 {
		return nil, nil
	}
	var row resellerdomain.Domain
	if err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Preload("Profile", "deleted_at IS NULL").
		Preload("Profile.User", "deleted_at IS NULL").
		Where("reseller_domains.id = ? AND reseller_domains.deleted_at IS NULL", id).
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// UpdateDomain 更新域名。
func (r *Store) UpdateDomain(domain *resellerdomain.Domain) error {
	if domain == nil || domain.ID == 0 {
		return errors.New("invalid reseller domain")
	}
	domain.Domain = normalizeDomainForRepository(domain.Domain)
	return r.db.Model(&resellerdomain.Domain{}).
		Where("id = ? AND deleted_at IS NULL", domain.ID).
		Select("*").
		Updates(domain).Error
}

// FindDomainByHost 按域名获取未删除域名记录。
func (r *Store) FindDomainByHost(host string) (*resellerdomain.Domain, error) {
	domain := normalizeDomainForRepository(host)
	if domain == "" {
		return nil, nil
	}
	var row resellerdomain.Domain
	err := r.db.Preload("Profile", "deleted_at IS NULL").
		Where("domain = ? AND reseller_domains.deleted_at IS NULL", domain).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// FindActiveVerifiedDomain 按域名获取已验证且启用的分销域名。
func (r *Store) FindActiveVerifiedDomain(host string) (*resellerdomain.Domain, error) {
	domain := normalizeDomainForRepository(host)
	if domain == "" {
		return nil, nil
	}
	var row resellerdomain.Domain
	err := r.db.Preload("Profile", "deleted_at IS NULL").
		Joins("JOIN reseller_profiles ON reseller_profiles.id = reseller_domains.reseller_id AND reseller_profiles.deleted_at IS NULL").
		Where("reseller_domains.domain = ? AND reseller_domains.status = ? AND reseller_domains.verification_status = ? AND reseller_domains.deleted_at IS NULL", domain, resellerdomain.DomainStatusActive, resellerdomain.DomainVerificationVerified).
		Where("reseller_profiles.status = ?", resellerdomain.ProfileStatusActive).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// ListDomainsByResellerID 列出分销商名下所有未删除域名。
func (r *Store) ListDomainsByResellerID(resellerID uint) ([]resellerdomain.Domain, error) {
	rows := make([]resellerdomain.Domain, 0)
	if resellerID == 0 {
		return rows, nil
	}
	if err := r.db.Where("reseller_id = ? AND deleted_at IS NULL", resellerID).Order("is_primary DESC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListDomains 分页列出分销商域名。
func (r *Store) ListDomains(filter resellercontract.DomainListFilter) ([]resellerdomain.Domain, int64, error) {
	rows := make([]resellerdomain.Domain, 0)
	query := r.db.Model(&resellerdomain.Domain{}).
		Preload("Profile", "deleted_at IS NULL").
		Preload("Profile.User", "deleted_at IS NULL").
		Where("reseller_domains.deleted_at IS NULL")
	if filter.ResellerID > 0 {
		query = query.Where("reseller_domains.reseller_id = ?", filter.ResellerID)
	}
	if filter.UserID > 0 {
		query = query.Joins("JOIN reseller_profiles rp_user_filter ON rp_user_filter.id = reseller_domains.reseller_id").
			Where("rp_user_filter.user_id = ?", filter.UserID)
	}
	if domain := strings.TrimSpace(filter.Domain); domain != "" {
		query = query.Where("reseller_domains.domain = ?", normalizeDomainForRepository(domain))
	}
	if typ := strings.TrimSpace(filter.Type); typ != "" {
		query = query.Where("reseller_domains.type = ?", typ)
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		query = query.Where("reseller_domains.status = ?", status)
	}
	if verification := strings.TrimSpace(filter.VerificationStatus); verification != "" {
		query = query.Where("reseller_domains.verification_status = ?", verification)
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + strings.ToLower(keyword) + "%"
		query = query.Joins("LEFT JOIN reseller_profiles rp_keyword ON rp_keyword.id = reseller_domains.reseller_id").
			Joins("LEFT JOIN users ON users.id = rp_keyword.user_id").
			Where("LOWER(reseller_domains.domain) LIKE ? OR LOWER(users.email) LIKE ? OR LOWER(users.display_name) LIKE ? OR CAST(reseller_domains.reseller_id AS TEXT) = ?", like, like, like, keyword)
	}
	if filter.CreatedFrom != nil {
		query = query.Where("reseller_domains.created_at >= ?", *filter.CreatedFrom)
	}
	if filter.CreatedTo != nil {
		query = query.Where("reseller_domains.created_at <= ?", *filter.CreatedTo)
	}
	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := applyPagination(query.Session(&gorm.Session{}), filter.Page, filter.PageSize).
		Order("reseller_domains.id DESC").
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func normalizeDomainForRepository(raw string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(raw)), ".")
}
