package adminstore

import (
	"errors"
	"time"

	admindomain "github.com/dujiao-next/internal/modules/identity/admin/domain"

	"gorm.io/gorm"
)

// Store 持久化管理员数据。
type Store struct {
	db *gorm.DB
}

// New 创建管理员存储。
func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

// GetByUsername 根据用户名获取管理员
func (r *Store) GetByUsername(username string) (*admindomain.Admin, error) {
	var admin admindomain.Admin
	if err := r.db.Where("username = ? AND deleted_at IS NULL", username).First(&admin).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &admin, nil
}

// GetByID 根据 ID 获取管理员
func (r *Store) GetByID(id uint) (*admindomain.Admin, error) {
	var admin admindomain.Admin
	if err := r.db.Where("id = ? AND deleted_at IS NULL", id).First(&admin).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &admin, nil
}

// List 获取管理员列表
func (r *Store) List() ([]admindomain.Admin, error) {
	admins := make([]admindomain.Admin, 0)
	err := r.db.
		Select("id", "username", "is_super", "last_login_at", "totp_enabled_at", "created_at").
		Where("deleted_at IS NULL").
		Order("id ASC").
		Find(&admins).Error
	if err != nil {
		return nil, err
	}
	return admins, nil
}

// Count 统计管理员数量
func (r *Store) Count() (int64, error) {
	var count int64
	if err := r.db.Model(&admindomain.Admin{}).Where("deleted_at IS NULL").Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// Create 创建管理员
func (r *Store) Create(admin *admindomain.Admin) error {
	return r.db.Create(admin).Error
}

// Update 更新管理员
func (r *Store) Update(admin *admindomain.Admin) error {
	return r.db.Save(admin).Error
}

// Delete 删除管理员（软删除）
func (r *Store) Delete(id uint) error {
	if id == 0 {
		return nil
	}
	return r.db.Model(&admindomain.Admin{}).Where("id = ? AND deleted_at IS NULL", id).Update("deleted_at", time.Now()).Error
}

// UpdateTOTPPending 写入待绑定 secret 与过期时间
func (r *Store) UpdateTOTPPending(adminID uint, encSecret string, expiresAt time.Time) error {
	if adminID == 0 {
		return errors.New("invalid admin id")
	}
	return r.db.Model(&admindomain.Admin{}).Where("id = ? AND deleted_at IS NULL", adminID).Updates(map[string]interface{}{
		"totp_pending_secret":     encSecret,
		"totp_pending_expires_at": expiresAt,
	}).Error
}

// UpdateTOTPEnabled 完成绑定：迁移 pending → 正式 secret，写入恢复码，清空 pending
func (r *Store) UpdateTOTPEnabled(adminID uint, encSecret string, enabledAt time.Time, recoveryCodesJSON string) error {
	if adminID == 0 {
		return errors.New("invalid admin id")
	}
	return r.db.Model(&admindomain.Admin{}).Where("id = ? AND deleted_at IS NULL", adminID).Updates(map[string]interface{}{
		"totp_secret":             encSecret,
		"totp_enabled_at":         enabledAt,
		"totp_pending_secret":     "",
		"totp_pending_expires_at": nil,
		"recovery_codes":          recoveryCodesJSON,
	}).Error
}

// UpdateRecoveryCodes 替换恢复码 JSON（用于消耗一个码或重新生成）
func (r *Store) UpdateRecoveryCodes(adminID uint, recoveryCodesJSON string) error {
	if adminID == 0 {
		return errors.New("invalid admin id")
	}
	return r.db.Model(&admindomain.Admin{}).Where("id = ? AND deleted_at IS NULL", adminID).Update("recovery_codes", recoveryCodesJSON).Error
}

// ClearTOTP 清空所有 TOTP 字段，TokenVersion++ 强制下线
func (r *Store) ClearTOTP(adminID uint) error {
	if adminID == 0 {
		return errors.New("invalid admin id")
	}
	now := time.Now()
	return r.db.Model(&admindomain.Admin{}).Where("id = ? AND deleted_at IS NULL", adminID).Updates(map[string]interface{}{
		"totp_secret":             "",
		"totp_enabled_at":         nil,
		"totp_pending_secret":     "",
		"totp_pending_expires_at": nil,
		"recovery_codes":          "",
		"token_version":           gorm.Expr("token_version + 1"),
		"token_invalid_before":    now,
	}).Error
}

// UpdatePassword 更新管理员密码哈希，TokenVersion++ 强制旧 token 失效
// 用于 admin-tool CLI 重置密码（超管忘记密码恢复路径）。
func (r *Store) UpdatePassword(adminID uint, passwordHash string) error {
	if adminID == 0 {
		return errors.New("invalid admin id")
	}
	if passwordHash == "" {
		return errors.New("password hash is empty")
	}
	now := time.Now()
	return r.db.Model(&admindomain.Admin{}).Where("id = ? AND deleted_at IS NULL", adminID).Updates(map[string]interface{}{
		"password_hash":        passwordHash,
		"token_version":        gorm.Expr("token_version + 1"),
		"token_invalid_before": now,
	}).Error
}
