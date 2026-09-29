package gormstore

import (
	"errors"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	affiliatecontract "github.com/dujiao-next/internal/modules/affiliate/contract"
	affiliatedomain "github.com/dujiao-next/internal/modules/affiliate/domain"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Store GORM 推广返利仓储
type Store struct {
	db *gorm.DB
}

// New 创建推广返利仓储
func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

func (r *Store) WithinTransaction(fn func(affiliatecontract.Store) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		return fn(New(tx))
	})
}

// GetProfileByID 按ID获取推广档案
func (r *Store) GetProfileByID(id uint) (*affiliatedomain.Profile, error) {
	if id == 0 {
		return nil, nil
	}
	var profile affiliatedomain.Profile
	if err := r.db.Where("affiliate_profiles.deleted_at IS NULL").Preload("User", "deleted_at IS NULL").First(&profile, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &profile, nil
}

// UpdateProfileStatus 更新推广档案状态
func (r *Store) UpdateProfileStatus(id uint, status string, updatedAt time.Time) error {
	if id == 0 {
		return nil
	}
	return r.db.Model(&affiliatedomain.Profile{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]interface{}{
			"status":     strings.TrimSpace(status),
			"updated_at": updatedAt,
		}).Error
}

// BatchUpdateProfileStatus 批量更新推广档案状态
func (r *Store) BatchUpdateProfileStatus(ids []uint, status string, updatedAt time.Time) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	result := r.db.Model(&affiliatedomain.Profile{}).
		Where("id IN ? AND deleted_at IS NULL", ids).
		Updates(map[string]interface{}{
			"status":     strings.TrimSpace(status),
			"updated_at": updatedAt,
		})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

// GetProfileByUserID 按用户ID获取推广档案
func (r *Store) GetProfileByUserID(userID uint) (*affiliatedomain.Profile, error) {
	if userID == 0 {
		return nil, nil
	}
	var profile affiliatedomain.Profile
	if err := r.db.Preload("User", "deleted_at IS NULL").Where("user_id = ? AND affiliate_profiles.deleted_at IS NULL", userID).First(&profile).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &profile, nil
}

// GetProfileByCode 按联盟ID获取推广档案
func (r *Store) GetProfileByCode(code string) (*affiliatedomain.Profile, error) {
	normalized := strings.ToUpper(strings.TrimSpace(code))
	if normalized == "" {
		return nil, nil
	}
	var profile affiliatedomain.Profile
	if err := r.db.Preload("User", "deleted_at IS NULL").Where("affiliate_code = ? AND affiliate_profiles.deleted_at IS NULL", normalized).First(&profile).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &profile, nil
}

// CreateProfile 创建推广档案
func (r *Store) CreateProfile(profile *affiliatedomain.Profile) error {
	return r.db.Create(profile).Error
}

// ListProfiles 查询推广档案列表
func (r *Store) ListProfiles(filter affiliatecontract.ProfileListFilter) ([]affiliatedomain.Profile, int64, error) {
	query := r.db.Model(&affiliatedomain.Profile{}).Where("affiliate_profiles.deleted_at IS NULL").Preload("User", "deleted_at IS NULL")
	if filter.UserID != 0 {
		query = query.Where("affiliate_profiles.user_id = ?", filter.UserID)
	}
	if code := strings.TrimSpace(filter.Code); code != "" {
		query = query.Where("affiliate_profiles.affiliate_code = ?", strings.ToUpper(code))
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		query = query.Where("affiliate_profiles.status = ?", status)
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.
			Joins("LEFT JOIN users ON users.id = affiliate_profiles.user_id").
			Where("(users.email LIKE ? OR users.display_name LIKE ? OR affiliate_profiles.affiliate_code LIKE ?)", like, like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	query = applyPagination(query, filter.Page, filter.PageSize)

	var rows []affiliatedomain.Profile
	if err := query.Order("affiliate_profiles.id desc").Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// CreateClick 创建推广点击记录
func (r *Store) CreateClick(click *affiliatedomain.Click) error {
	return r.db.Create(click).Error
}

// HasRecentClick 查询是否存在近期重复点击记录
func (r *Store) HasRecentClick(profileID uint, visitorKey, landingPath string, since time.Time) (bool, error) {
	if profileID == 0 || strings.TrimSpace(visitorKey) == "" {
		return false, nil
	}
	query := r.db.Model(&affiliatedomain.Click{}).
		Where("affiliate_profile_id = ? AND visitor_key = ? AND created_at >= ?",
			profileID,
			strings.TrimSpace(visitorKey),
			since,
		)
	if path := strings.TrimSpace(landingPath); path != "" {
		query = query.Where("landing_path = ?", path)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return false, err
	}
	return total > 0, nil
}

// GetLatestActiveProfileByVisitorKey 查询访客最近一次有效点击对应的推广用户
func (r *Store) GetLatestActiveProfileByVisitorKey(visitorKey string, since time.Time) (*affiliatedomain.Profile, error) {
	key := strings.TrimSpace(visitorKey)
	if key == "" {
		return nil, nil
	}

	var profile affiliatedomain.Profile
	err := r.db.Model(&affiliatedomain.Profile{}).
		Joins("JOIN affiliate_clicks ac ON ac.affiliate_profile_id = affiliate_profiles.id").
		Where("ac.visitor_key = ? AND ac.created_at >= ? AND affiliate_profiles.status = ? AND affiliate_profiles.deleted_at IS NULL",
			key,
			since,
			constants.AffiliateProfileStatusActive,
		).
		Order("ac.created_at DESC, ac.id DESC").
		Limit(1).
		Preload("User", "deleted_at IS NULL").
		First(&profile).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &profile, nil
}

// CountClicksByProfile 统计推广点击数
func (r *Store) CountClicksByProfile(profileID uint) (int64, error) {
	if profileID == 0 {
		return 0, nil
	}
	var total int64
	if err := r.db.Model(&affiliatedomain.Click{}).Where("affiliate_profile_id = ?", profileID).Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

// GetCommissionByOrderAndProfile 按订单和推广人查询佣金
func (r *Store) GetCommissionByOrderAndProfile(orderID, profileID uint, commissionType string) (*affiliatedomain.Commission, error) {
	if orderID == 0 || profileID == 0 {
		return nil, nil
	}
	ctype := strings.TrimSpace(commissionType)
	if ctype == "" {
		ctype = constants.AffiliateCommissionTypeOrder
	}
	var commission affiliatedomain.Commission
	if err := r.db.Where("order_id = ? AND affiliate_profile_id = ? AND commission_type = ? AND deleted_at IS NULL", orderID, profileID, ctype).
		First(&commission).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &commission, nil
}

// CreateCommission 创建佣金记录
func (r *Store) CreateCommission(commission *affiliatedomain.Commission) error {
	return r.db.Create(commission).Error
}

// UpdateCommission 更新佣金记录
func (r *Store) UpdateCommission(commission *affiliatedomain.Commission) error {
	if commission == nil || commission.ID == 0 {
		return nil
	}
	return r.db.Model(&affiliatedomain.Commission{}).
		Where("id = ? AND deleted_at IS NULL", commission.ID).
		Select("*").
		Updates(commission).Error
}

// ListCommissions 查询佣金记录
func (r *Store) ListCommissions(filter affiliatecontract.CommissionListFilter) ([]affiliatedomain.Commission, int64, error) {
	query := r.db.Model(&affiliatedomain.Commission{}).
		Where("affiliate_commissions.deleted_at IS NULL").
		Preload("AffiliateProfile", "deleted_at IS NULL").
		Preload("AffiliateProfile.User", "deleted_at IS NULL").
		Preload("Order", "deleted_at IS NULL")
	if filter.AffiliateProfileID != 0 {
		query = query.Where("affiliate_commissions.affiliate_profile_id = ?", filter.AffiliateProfileID)
	}
	if filter.OrderID != 0 {
		query = query.Where("affiliate_commissions.order_id = ?", filter.OrderID)
	}
	if orderNo := strings.TrimSpace(filter.OrderNo); orderNo != "" {
		query = query.Joins("LEFT JOIN orders ON orders.id = affiliate_commissions.order_id AND orders.deleted_at IS NULL").
			Where("orders.order_no LIKE ?", "%"+orderNo+"%")
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		query = query.Where("affiliate_commissions.status = ?", status)
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.
			Joins("LEFT JOIN affiliate_profiles ap ON ap.id = affiliate_commissions.affiliate_profile_id").
			Joins("LEFT JOIN users u ON u.id = ap.user_id").
			Where("(u.email LIKE ? OR u.display_name LIKE ? OR ap.affiliate_code LIKE ?)", like, like, like)
	}
	if filter.CreatedFrom != nil {
		query = query.Where("affiliate_commissions.created_at >= ?", *filter.CreatedFrom)
	}
	if filter.CreatedTo != nil {
		query = query.Where("affiliate_commissions.created_at <= ?", *filter.CreatedTo)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	query = applyPagination(query, filter.Page, filter.PageSize)

	var rows []affiliatedomain.Commission
	if err := query.Order("affiliate_commissions.id desc").Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// ListCommissionsByOrder 按订单查询佣金记录
func (r *Store) ListCommissionsByOrder(orderID uint, statuses []string) ([]affiliatedomain.Commission, error) {
	if orderID == 0 {
		return []affiliatedomain.Commission{}, nil
	}
	query := r.db.Model(&affiliatedomain.Commission{}).Where("order_id = ? AND deleted_at IS NULL", orderID)
	if len(statuses) > 0 {
		query = query.Where("status IN ?", statuses)
	}
	var rows []affiliatedomain.Commission
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListCommissionsByOrderForUpdate 按订单查询佣金并加锁
func (r *Store) ListCommissionsByOrderForUpdate(orderID uint, statuses []string) ([]affiliatedomain.Commission, error) {
	if orderID == 0 {
		return []affiliatedomain.Commission{}, nil
	}
	query := r.db.Model(&affiliatedomain.Commission{}).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("order_id = ? AND deleted_at IS NULL", orderID)
	if len(statuses) > 0 {
		query = query.Where("status IN ?", statuses)
	}
	var rows []affiliatedomain.Commission
	if err := query.Order("id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListCommissionsByWithdrawIDForUpdate 按提现单查询并锁定佣金记录
func (r *Store) ListCommissionsByWithdrawIDForUpdate(withdrawID uint) ([]affiliatedomain.Commission, error) {
	if withdrawID == 0 {
		return []affiliatedomain.Commission{}, nil
	}
	var rows []affiliatedomain.Commission
	if err := r.db.Model(&affiliatedomain.Commission{}).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("withdraw_request_id = ? AND deleted_at IS NULL", withdrawID).
		Order("id asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// MarkPendingCommissionsAvailable 批量将待确认佣金转可提现
func (r *Store) MarkPendingCommissionsAvailable(before, now time.Time) (int64, error) {
	result := r.db.Model(&affiliatedomain.Commission{}).
		Where("status = ? AND confirm_at IS NOT NULL AND confirm_at <= ? AND withdraw_request_id IS NULL AND deleted_at IS NULL",
			constants.AffiliateCommissionStatusPendingConfirm, before).
		Updates(map[string]interface{}{
			"status":       constants.AffiliateCommissionStatusAvailable,
			"available_at": now,
			"updated_at":   now,
		})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

// CountValidOrdersByProfile 统计有效订单数
func (r *Store) CountValidOrdersByProfile(profileID uint) (int64, error) {
	if profileID == 0 {
		return 0, nil
	}
	var total int64
	if err := r.db.Model(&affiliatedomain.Commission{}).
		Where("affiliate_profile_id = ? AND status <> ? AND deleted_at IS NULL", profileID, constants.AffiliateCommissionStatusRejected).
		Distinct("order_id").
		Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

// SumCommissionByProfile 汇总指定状态佣金金额
func (r *Store) SumCommissionByProfile(profileID uint, statuses []string, unboundOnly bool) (decimal.Decimal, error) {
	if profileID == 0 || len(statuses) == 0 {
		return decimal.Zero, nil
	}
	query := r.db.Model(&affiliatedomain.Commission{}).
		Where("affiliate_profile_id = ? AND status IN ? AND deleted_at IS NULL", profileID, statuses)
	if unboundOnly {
		query = query.Where("withdraw_request_id IS NULL")
	}

	var row struct {
		Total decimal.Decimal `gorm:"column:total"`
	}
	if err := query.Select("COALESCE(SUM(commission_amount), 0) AS total").Scan(&row).Error; err != nil {
		return decimal.Zero, err
	}
	return row.Total.Round(2), nil
}

// ListAvailableCommissionsForUpdate 查询并锁定可提现佣金
func (r *Store) ListAvailableCommissionsForUpdate(profileID uint) ([]affiliatedomain.Commission, error) {
	if profileID == 0 {
		return []affiliatedomain.Commission{}, nil
	}
	var rows []affiliatedomain.Commission
	if err := r.db.Model(&affiliatedomain.Commission{}).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("affiliate_profile_id = ? AND status = ? AND withdraw_request_id IS NULL AND deleted_at IS NULL",
			profileID, constants.AffiliateCommissionStatusAvailable).
		Order("id asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// BatchUpdateCommissions 批量更新佣金记录
func (r *Store) BatchUpdateCommissions(ids []uint, updates map[string]interface{}) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.Model(&affiliatedomain.Commission{}).Where("id IN ? AND deleted_at IS NULL", ids).Updates(updates).Error
}

// CreateWithdraw 创建提现申请
func (r *Store) CreateWithdraw(req *affiliatedomain.WithdrawRequest) error {
	return r.db.Create(req).Error
}

// UpdateWithdraw 更新提现申请
func (r *Store) UpdateWithdraw(req *affiliatedomain.WithdrawRequest) error {
	if req == nil || req.ID == 0 {
		return nil
	}
	return r.db.Model(&affiliatedomain.WithdrawRequest{}).
		Where("id = ? AND deleted_at IS NULL", req.ID).
		Select("*").
		Updates(req).Error
}

// GetWithdrawByID 按ID查询提现申请
func (r *Store) GetWithdrawByID(id uint) (*affiliatedomain.WithdrawRequest, error) {
	if id == 0 {
		return nil, nil
	}
	var row affiliatedomain.WithdrawRequest
	if err := r.db.Where("affiliate_withdraw_requests.deleted_at IS NULL").Preload("AffiliateProfile", "deleted_at IS NULL").Preload("AffiliateProfile.User", "deleted_at IS NULL").Preload("Processor", "deleted_at IS NULL").First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// GetWithdrawByIDForUpdate 按ID锁定查询提现申请
func (r *Store) GetWithdrawByIDForUpdate(id uint) (*affiliatedomain.WithdrawRequest, error) {
	if id == 0 {
		return nil, nil
	}
	var row affiliatedomain.WithdrawRequest
	if err := r.db.Where("affiliate_withdraw_requests.deleted_at IS NULL").Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// ListWithdraws 查询提现申请列表
func (r *Store) ListWithdraws(filter affiliatecontract.WithdrawListFilter) ([]affiliatedomain.WithdrawRequest, int64, error) {
	query := r.db.Model(&affiliatedomain.WithdrawRequest{}).
		Where("affiliate_withdraw_requests.deleted_at IS NULL").
		Preload("AffiliateProfile", "deleted_at IS NULL").
		Preload("AffiliateProfile.User", "deleted_at IS NULL").
		Preload("Processor", "deleted_at IS NULL")

	if filter.AffiliateProfileID != 0 {
		query = query.Where("affiliate_withdraw_requests.affiliate_profile_id = ?", filter.AffiliateProfileID)
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		query = query.Where("affiliate_withdraw_requests.status = ?", status)
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.
			Joins("LEFT JOIN affiliate_profiles ap ON ap.id = affiliate_withdraw_requests.affiliate_profile_id").
			Joins("LEFT JOIN users u ON u.id = ap.user_id").
			Where("(u.email LIKE ? OR u.display_name LIKE ? OR ap.affiliate_code LIKE ? OR affiliate_withdraw_requests.account LIKE ?)",
				like, like, like, like)
	}
	if filter.CreatedFrom != nil {
		query = query.Where("affiliate_withdraw_requests.created_at >= ?", *filter.CreatedFrom)
	}
	if filter.CreatedTo != nil {
		query = query.Where("affiliate_withdraw_requests.created_at <= ?", *filter.CreatedTo)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	query = applyPagination(query, filter.Page, filter.PageSize)

	var rows []affiliatedomain.WithdrawRequest
	if err := query.Order("affiliate_withdraw_requests.id desc").Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// GetProfileStatsBatch 批量汇总推广用户统计信息
func (r *Store) GetProfileStatsBatch(profileIDs []uint) (map[uint]affiliatecontract.ProfileStatsAggregate, error) {
	result := make(map[uint]affiliatecontract.ProfileStatsAggregate, len(profileIDs))
	if len(profileIDs) == 0 {
		return result, nil
	}

	for _, id := range profileIDs {
		if id == 0 {
			continue
		}
		result[id] = affiliatecontract.ProfileStatsAggregate{
			PendingCommission:   decimal.Zero,
			AvailableCommission: decimal.Zero,
			WithdrawnCommission: decimal.Zero,
		}
	}

	var clickRows []struct {
		AffiliateProfileID uint  `gorm:"column:affiliate_profile_id"`
		Total              int64 `gorm:"column:total"`
	}
	if err := r.db.Model(&affiliatedomain.Click{}).
		Select("affiliate_profile_id, COUNT(*) AS total").
		Where("affiliate_profile_id IN ?", profileIDs).
		Group("affiliate_profile_id").
		Scan(&clickRows).Error; err != nil {
		return nil, err
	}
	for _, row := range clickRows {
		item := result[row.AffiliateProfileID]
		item.ClickCount = row.Total
		result[row.AffiliateProfileID] = item
	}

	var validRows []struct {
		AffiliateProfileID uint  `gorm:"column:affiliate_profile_id"`
		Total              int64 `gorm:"column:total"`
	}
	if err := r.db.Model(&affiliatedomain.Commission{}).
		Select("affiliate_profile_id, COUNT(DISTINCT order_id) AS total").
		Where("affiliate_profile_id IN ? AND status <> ? AND deleted_at IS NULL", profileIDs, constants.AffiliateCommissionStatusRejected).
		Group("affiliate_profile_id").
		Scan(&validRows).Error; err != nil {
		return nil, err
	}
	for _, row := range validRows {
		item := result[row.AffiliateProfileID]
		item.ValidOrderCount = row.Total
		result[row.AffiliateProfileID] = item
	}

	var pendingRows []struct {
		AffiliateProfileID uint            `gorm:"column:affiliate_profile_id"`
		Total              decimal.Decimal `gorm:"column:total"`
	}
	if err := r.db.Model(&affiliatedomain.Commission{}).
		Select("affiliate_profile_id, COALESCE(SUM(commission_amount), 0) AS total").
		Where("affiliate_profile_id IN ? AND status = ? AND deleted_at IS NULL", profileIDs, constants.AffiliateCommissionStatusPendingConfirm).
		Group("affiliate_profile_id").
		Scan(&pendingRows).Error; err != nil {
		return nil, err
	}
	for _, row := range pendingRows {
		item := result[row.AffiliateProfileID]
		item.PendingCommission = row.Total.Round(2)
		result[row.AffiliateProfileID] = item
	}

	var availableRows []struct {
		AffiliateProfileID uint            `gorm:"column:affiliate_profile_id"`
		Total              decimal.Decimal `gorm:"column:total"`
	}
	if err := r.db.Model(&affiliatedomain.Commission{}).
		Select("affiliate_profile_id, COALESCE(SUM(commission_amount), 0) AS total").
		Where("affiliate_profile_id IN ? AND status = ? AND withdraw_request_id IS NULL AND deleted_at IS NULL",
			profileIDs,
			constants.AffiliateCommissionStatusAvailable,
		).
		Group("affiliate_profile_id").
		Scan(&availableRows).Error; err != nil {
		return nil, err
	}
	for _, row := range availableRows {
		item := result[row.AffiliateProfileID]
		item.AvailableCommission = row.Total.Round(2)
		result[row.AffiliateProfileID] = item
	}

	var withdrawnRows []struct {
		AffiliateProfileID uint            `gorm:"column:affiliate_profile_id"`
		Total              decimal.Decimal `gorm:"column:total"`
	}
	if err := r.db.Model(&affiliatedomain.Commission{}).
		Select("affiliate_profile_id, COALESCE(SUM(commission_amount), 0) AS total").
		Where("affiliate_profile_id IN ? AND status = ? AND deleted_at IS NULL", profileIDs, constants.AffiliateCommissionStatusWithdrawn).
		Group("affiliate_profile_id").
		Scan(&withdrawnRows).Error; err != nil {
		return nil, err
	}
	for _, row := range withdrawnRows {
		item := result[row.AffiliateProfileID]
		item.WithdrawnCommission = row.Total.Round(2)
		result[row.AffiliateProfileID] = item
	}

	return result, nil
}

func applyPagination(query *gorm.DB, page, pageSize int) *gorm.DB {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	return query.Offset((page - 1) * pageSize).Limit(pageSize)
}

var _ affiliatecontract.Store = (*Store)(nil)
