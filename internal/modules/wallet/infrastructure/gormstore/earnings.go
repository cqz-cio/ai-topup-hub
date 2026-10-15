package gormstore

import (
	"errors"
	affiliatedomain "github.com/dujiao-next/internal/modules/affiliate/domain"
	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	"github.com/dujiao-next/internal/persistence/gormutil"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
)

func (s *Store) CheckWithdrawalAllowed(userID uint) error {
	var restricted int64
	if err := s.db.Model(&affiliatedomain.Profile{}).Where("user_id = ? AND deleted_at IS NULL AND status <> ?", userID, "active").Count(&restricted).Error; err != nil {
		return err
	}
	if restricted > 0 {
		return walletcontract.ErrWithdrawRestricted
	}
	if err := s.db.Model(&resellerdomain.Profile{}).Where("user_id = ? AND deleted_at IS NULL AND (status <> ? OR (settlement_status <> '' AND settlement_status <> ?))", userID, "active", "normal").Count(&restricted).Error; err != nil {
		return err
	}
	if restricted > 0 {
		return walletcontract.ErrWithdrawRestricted
	}
	if err := s.db.Model(&resellerdomain.BalanceAccount{}).Where("reseller_id IN (SELECT id FROM reseller_profiles WHERE user_id = ? AND deleted_at IS NULL) AND deleted_at IS NULL AND status NOT IN ?", userID, []string{"normal", ""}).Count(&restricted).Error; err != nil {
		return err
	}
	if restricted > 0 {
		return walletcontract.ErrWithdrawRestricted
	}
	return nil
}

func (s *Store) GetEarningSettlement(reference string) (*walletdomain.EarningSettlement, error) {
	var row walletdomain.EarningSettlement
	err := s.db.Where("reference = ?", reference).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &row, err
}
func (s *Store) SaveEarningSettlement(row *walletdomain.EarningSettlement) error {
	return s.db.Save(row).Error
}
func (s *Store) GetOrderFunding(id uint) (*walletdomain.OrderFunding, error) {
	var row walletdomain.OrderFunding
	err := s.db.Where("order_id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &row, err
}
func (s *Store) SaveOrderFunding(row *walletdomain.OrderFunding) error { return s.db.Save(row).Error }
func (s *Store) CreateWithdraw(row *walletdomain.WithdrawRequest) error {
	return s.db.Create(row).Error
}
func (s *Store) UpdateWithdraw(row *walletdomain.WithdrawRequest) error { return s.db.Save(row).Error }
func (s *Store) GetWithdraw(id uint, lock bool) (*walletdomain.WithdrawRequest, error) {
	var row walletdomain.WithdrawRequest
	query := s.db
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err := query.First(&row, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &row, err
}
func (s *Store) GetWithdrawByKey(userID uint, key string) (*walletdomain.WithdrawRequest, error) {
	var row walletdomain.WithdrawRequest
	err := s.db.Where("user_id = ? AND request_key = ?", userID, key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &row, err
}
func (s *Store) ListWithdraws(filter walletcontract.WithdrawListFilter) ([]walletdomain.WithdrawRequest, int64, error) {
	query := s.db.Model(&walletdomain.WithdrawRequest{})
	if filter.UserID > 0 {
		query = query.Where("user_id = ?", filter.UserID)
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []walletdomain.WithdrawRequest
	err := gormutil.ApplyPagination(query, filter.Page, filter.PageSize).Order("id DESC").Find(&rows).Error
	return rows, total, err
}
