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

// CreateWithdrawRequest 创建分销提现申请。
func (r *Store) CreateWithdrawRequest(req *resellerdomain.WithdrawRequest) error {
	if req == nil {
		return errors.New("reseller withdraw request is nil")
	}
	now := time.Now()
	if req.CreatedAt.IsZero() {
		req.CreatedAt = now
	}
	req.UpdatedAt = now
	return r.db.Create(req).Error
}

// GetWithdrawRequestByID 按 ID 获取分销提现申请。
func (r *Store) GetWithdrawRequestByID(id uint) (*resellerdomain.WithdrawRequest, error) {
	if id == 0 {
		return nil, nil
	}
	var row resellerdomain.WithdrawRequest
	if err := r.db.Where("id = ? AND deleted_at IS NULL", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// GetWithdrawRequestByIDForUpdate 按 ID 获取并锁定分销提现申请。
func (r *Store) GetWithdrawRequestByIDForUpdate(id uint) (*resellerdomain.WithdrawRequest, error) {
	if id == 0 {
		return nil, nil
	}
	var row resellerdomain.WithdrawRequest
	if err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// UpdateWithdrawRequest 更新分销提现申请。
func (r *Store) UpdateWithdrawRequest(req *resellerdomain.WithdrawRequest) error {
	if req == nil {
		return errors.New("reseller withdraw request is nil")
	}
	req.UpdatedAt = time.Now()
	return r.db.Model(&resellerdomain.WithdrawRequest{}).
		Where("id = ? AND deleted_at IS NULL", req.ID).
		Select("*").
		Updates(req).Error
}

// ListWithdrawRequests 分页列出分销提现申请。
func (r *Store) ListWithdrawRequests(filter resellercontract.WithdrawListFilter) ([]resellerdomain.WithdrawRequest, int64, error) {
	rows := make([]resellerdomain.WithdrawRequest, 0)
	query := r.db.Model(&resellerdomain.WithdrawRequest{}).Where("deleted_at IS NULL")
	if filter.ResellerID != 0 {
		query = query.Where("reseller_id = ?", filter.ResellerID)
	}
	if currency := strings.TrimSpace(filter.Currency); currency != "" {
		query = query.Where("currency = ?", currency)
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := applyPagination(query.Session(&gorm.Session{}), filter.Page, filter.PageSize).
		Order("id DESC").
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}
