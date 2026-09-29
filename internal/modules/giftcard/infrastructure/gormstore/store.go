package gormstore

import (
	"errors"
	"strings"
	"time"

	giftcardcontract "github.com/dujiao-next/internal/modules/giftcard/contract"
	giftcarddomain "github.com/dujiao-next/internal/modules/giftcard/domain"
	"github.com/dujiao-next/internal/persistence/gormutil"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const listStatusExpired = "expired"

// Store 是礼品卡仓储端口的 GORM 实现。
type Store struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

func (r *Store) WithTx(tx *gorm.DB) *Store {
	if tx == nil {
		return r
	}
	return &Store{db: tx}
}

// WithinTransaction 为管理用例提供不暴露 GORM 的事务边界。
func (r *Store) WithinTransaction(fn func(repo giftcardcontract.Repository) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		return fn(New(tx))
	})
}

// Transaction 为兑换写路径提供可与钱包入账共享的事务。
func (r *Store) Transaction(fn func(tx *gorm.DB) error) error {
	return r.db.Transaction(fn)
}

// CreateBatch 创建礼品卡批次与卡片。
func (r *Store) CreateBatch(batch *giftcarddomain.GiftCardBatch, cards []giftcarddomain.GiftCard) error {
	if batch == nil {
		return errors.New("invalid gift card batch")
	}
	if err := r.db.Create(batch).Error; err != nil {
		return err
	}
	if len(cards) == 0 {
		return nil
	}
	for idx := range cards {
		cards[idx].BatchID = &batch.ID
	}
	return r.db.Create(&cards).Error
}

// GetByID 根据 ID 查询礼品卡。
func (r *Store) GetByID(id uint) (*giftcarddomain.GiftCard, error) {
	if id == 0 {
		return nil, nil
	}
	var card giftcarddomain.GiftCard
	if err := r.db.Where("gift_cards.deleted_at IS NULL").
		Preload("Batch", "deleted_at IS NULL").
		First(&card, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &card, nil
}

// GetByCodeForUpdate 根据卡密加锁查询礼品卡。
func (r *Store) GetByCodeForUpdate(code string) (*giftcarddomain.GiftCard, error) {
	code = strings.TrimSpace(strings.ToUpper(code))
	if code == "" {
		return nil, nil
	}
	var card giftcarddomain.GiftCard
	if err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("deleted_at IS NULL AND code = ?", code).
		First(&card).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &card, nil
}

// List 查询礼品卡列表。
func (r *Store) List(filter giftcardcontract.ListFilter) ([]giftcarddomain.GiftCard, int64, error) {
	query := r.db.Model(&giftcarddomain.GiftCard{}).
		Where("gift_cards.deleted_at IS NULL").
		Preload("Batch", "deleted_at IS NULL")
	if code := strings.TrimSpace(strings.ToUpper(filter.Code)); code != "" {
		query = query.Where("code LIKE ?", "%"+code+"%")
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		now := time.Now()
		switch status {
		case listStatusExpired:
			query = query.Where("status = ? AND expires_at IS NOT NULL AND expires_at < ?", giftcarddomain.GiftCardStatusActive, now)
		case giftcarddomain.GiftCardStatusActive:
			query = query.Where("status = ? AND (expires_at IS NULL OR expires_at >= ?)", giftcarddomain.GiftCardStatusActive, now)
		default:
			query = query.Where("status = ?", status)
		}
	}
	if batchNo := strings.TrimSpace(strings.ToUpper(filter.BatchNo)); batchNo != "" {
		query = query.Joins("LEFT JOIN gift_card_batches ON gift_card_batches.id = gift_cards.batch_id").
			Where("gift_card_batches.deleted_at IS NULL AND gift_card_batches.batch_no LIKE ?", "%"+batchNo+"%")
	}
	if filter.RedeemedUserID > 0 {
		query = query.Where("redeemed_user_id = ?", filter.RedeemedUserID)
	}
	if filter.CreatedFrom != nil {
		query = query.Where("created_at >= ?", *filter.CreatedFrom)
	}
	if filter.CreatedTo != nil {
		query = query.Where("created_at <= ?", *filter.CreatedTo)
	}
	if filter.RedeemedFrom != nil {
		query = query.Where("redeemed_at >= ?", *filter.RedeemedFrom)
	}
	if filter.RedeemedTo != nil {
		query = query.Where("redeemed_at <= ?", *filter.RedeemedTo)
	}
	if filter.ExpiresFrom != nil {
		query = query.Where("expires_at >= ?", *filter.ExpiresFrom)
	}
	if filter.ExpiresTo != nil {
		query = query.Where("expires_at <= ?", *filter.ExpiresTo)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	query = gormutil.ApplyPagination(query, filter.Page, filter.PageSize)

	var cards []giftcarddomain.GiftCard
	if err := query.Order("id desc").Find(&cards).Error; err != nil {
		return nil, 0, err
	}
	return cards, total, nil
}

// ListByIDs 按 ID 列表查询礼品卡。
func (r *Store) ListByIDs(ids []uint) ([]giftcarddomain.GiftCard, error) {
	if len(ids) == 0 {
		return []giftcarddomain.GiftCard{}, nil
	}
	var cards []giftcarddomain.GiftCard
	if err := r.db.Where("gift_cards.deleted_at IS NULL AND id IN ?", ids).
		Preload("Batch", "deleted_at IS NULL").
		Order("id asc").Find(&cards).Error; err != nil {
		return nil, err
	}
	return cards, nil
}

// Update 更新礼品卡。
func (r *Store) Update(card *giftcarddomain.GiftCard) error {
	if card == nil {
		return errors.New("invalid gift card")
	}
	return r.db.Save(card).Error
}

// Delete 删除礼品卡。
func (r *Store) Delete(id uint) error {
	if id == 0 {
		return nil
	}
	return r.db.Model(&giftcarddomain.GiftCard{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("deleted_at", time.Now()).Error
}

// BatchUpdateStatus 批量更新礼品卡状态。
func (r *Store) BatchUpdateStatus(ids []uint, status string, updatedAt time.Time) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}
	result := r.db.Model(&giftcarddomain.GiftCard{}).
		Where("deleted_at IS NULL AND id IN ? AND status <> ?", ids, giftcarddomain.GiftCardStatusRedeemed).
		Updates(map[string]interface{}{
			"status":     strings.TrimSpace(status),
			"updated_at": updatedAt,
		})
	return result.RowsAffected, result.Error
}

var _ giftcardcontract.Repository = (*Store)(nil)
