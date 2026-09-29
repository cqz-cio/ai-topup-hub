package gormstore

import (
	"errors"
	"time"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	"github.com/dujiao-next/internal/persistence/gormutil"

	"gorm.io/gorm"
)

// ChannelStore 是支付渠道的 GORM 实现。
type ChannelStore struct {
	db *gorm.DB
}

// NewChannelStore 创建支付渠道仓库
func NewChannelStore(db *gorm.DB) *ChannelStore {
	return &ChannelStore{db: db}
}

// Create 创建支付渠道
func (r *ChannelStore) Create(channel *paymentdomain.PaymentChannel) error {
	return r.db.Create(channel).Error
}

// Update 更新支付渠道
func (r *ChannelStore) Update(channel *paymentdomain.PaymentChannel) error {
	return r.db.Save(channel).Error
}

// Delete 删除支付渠道
func (r *ChannelStore) Delete(id uint) error {
	if id == 0 {
		return nil
	}
	now := time.Now()
	return r.db.Model(&paymentdomain.PaymentChannel{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]interface{}{"deleted_at": now, "updated_at": now}).Error
}

// GetByID 根据 ID 获取支付渠道
func (r *ChannelStore) GetByID(id uint) (*paymentdomain.PaymentChannel, error) {
	var channel paymentdomain.PaymentChannel
	if err := r.db.Where("deleted_at IS NULL").First(&channel, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &channel, nil
}

// ListByIDs 根据 ID 列表获取支付渠道
func (r *ChannelStore) ListByIDs(ids []uint) ([]paymentdomain.PaymentChannel, error) {
	if len(ids) == 0 {
		return []paymentdomain.PaymentChannel{}, nil
	}
	var channels []paymentdomain.PaymentChannel
	if err := r.db.Where("id IN ? AND deleted_at IS NULL", ids).Find(&channels).Error; err != nil {
		return nil, err
	}
	return channels, nil
}

// List 支付渠道列表
func (r *ChannelStore) List(filter paymentcontract.ChannelListFilter) ([]paymentdomain.PaymentChannel, int64, error) {
	query := r.db.Model(&paymentdomain.PaymentChannel{}).Where("deleted_at IS NULL")

	if filter.ProviderType != "" {
		query = query.Where("provider_type = ?", filter.ProviderType)
	}
	if filter.ChannelType != "" {
		query = query.Where("channel_type = ?", filter.ChannelType)
	}
	if filter.ActiveOnly {
		query = query.Where("is_active = ?", true)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	query = gormutil.ApplyPagination(query, filter.Page, filter.PageSize)

	var channels []paymentdomain.PaymentChannel
	if err := query.Order("sort_order DESC, id ASC").Find(&channels).Error; err != nil {
		return nil, 0, err
	}
	return channels, total, nil
}
