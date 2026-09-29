package gormstore

import (
	"errors"

	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"
	"gorm.io/gorm"
)

// Store is the single GORM implementation of all reseller persistence ports.
type Store struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

// Migrate creates the tables and partial unique indexes owned by the reseller module.
func Migrate(db *gorm.DB) error {
	if db == nil {
		return errors.New("reseller migration database is nil")
	}
	if err := db.AutoMigrate(
		&resellerdomain.Profile{},
		&resellerdomain.Domain{},
		&resellerdomain.SiteConfig{},
		&resellerdomain.ProductSetting{},
		&resellerdomain.OrderSnapshot{},
		&resellerdomain.LedgerEntry{},
		&resellerdomain.WithdrawRequest{},
		&resellerdomain.BalanceAccount{},
		&resellerdomain.RelatedAccount{},
	); err != nil {
		return err
	}
	statements := []string{
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_reseller_domains_active_domain ON reseller_domains(domain) WHERE deleted_at IS NULL",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_reseller_site_configs_active_reseller ON reseller_site_configs(reseller_id) WHERE deleted_at IS NULL",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_reseller_product_settings_active_scope ON reseller_product_settings(reseller_id, product_id, sku_id) WHERE deleted_at IS NULL",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_reseller_balance_accounts_active_currency ON reseller_balance_accounts(reseller_id, currency) WHERE deleted_at IS NULL",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_reseller_related_accounts_active_user ON reseller_related_accounts(reseller_id, user_id) WHERE deleted_at IS NULL",
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}

// BindTx returns the same reseller store surface bound to a caller-owned transaction.
func (s *Store) BindTx(tx *gorm.DB) *Store {
	if tx == nil {
		return s
	}
	return New(tx)
}

func (s *Store) transaction(run func(*Store) error) error {
	if s == nil || s.db == nil || run == nil {
		return nil
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		return run(New(tx))
	})
}

func (s *Store) WithinManagementTransaction(run func(resellercontract.ManagementStore) error) error {
	return s.transaction(func(tx *Store) error { return run(tx) })
}

func (s *Store) WithinProductSettingTransaction(run func(resellercontract.ProductSettingStore) error) error {
	return s.transaction(func(tx *Store) error { return run(tx) })
}

func (s *Store) WithinWithdrawTransaction(run func(resellercontract.AccountingWithdrawStore) error) error {
	return s.transaction(func(tx *Store) error { return run(tx) })
}

func (s *Store) WithinLedgerTransaction(run func(resellercontract.AccountingLedgerStore) error) error {
	return s.transaction(func(tx *Store) error { return run(tx) })
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
