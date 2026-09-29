package gormstore

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestResellerRepositoryPostgresActiveUniqueIndex(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("skip postgres integration test: TEST_POSTGRES_DSN is empty")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres failed: %v", err)
	}
	if err := db.AutoMigrate(&userdomain.User{}, &resellerdomain.Profile{}, &resellerdomain.Domain{}, &resellerdomain.SiteConfig{}); err != nil {
		t.Fatalf("migrate postgres failed: %v", err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_reseller_domains_active_domain ON reseller_domains(domain) WHERE deleted_at IS NULL").Error; err != nil {
		t.Fatalf("create postgres active domain index failed: %v", err)
	}
	suffix := time.Now().Format("20060102150405")
	profile := seedResellerProfile(t, db, "pg-reseller-"+suffix+"@example.com")
	repo := New(db)
	first, err := repo.UpsertDomain(resellerdomain.Domain{
		ResellerID:         profile.ID,
		Domain:             "pg-" + suffix + ".example.test",
		Type:               resellerdomain.DomainTypeCustom,
		Status:             resellerdomain.DomainStatusActive,
		VerificationStatus: resellerdomain.DomainVerificationVerified,
	})
	if err != nil {
		t.Fatalf("create postgres domain failed: %v", err)
	}
	now := time.Now()
	if err := db.Model(&resellerdomain.Domain{}).Where("id = ?", first.ID).Update("deleted_at", &now).Error; err != nil {
		t.Fatalf("soft delete postgres domain failed: %v", err)
	}
	second, err := repo.UpsertDomain(resellerdomain.Domain{
		ResellerID:         profile.ID,
		Domain:             first.Domain,
		Type:               resellerdomain.DomainTypeCustom,
		Status:             resellerdomain.DomainStatusActive,
		VerificationStatus: resellerdomain.DomainVerificationVerified,
	})
	if err != nil {
		t.Fatalf("restore postgres domain failed: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("expected restore same row id=%d got id=%d", first.ID, second.ID)
	}
}

func TestResellerRepositoryPostgresWithdrawLocksSameRows(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if strings.TrimSpace(dsn) == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres failed: %v", err)
	}
	if err := db.AutoMigrate(&userdomain.User{}, &resellerdomain.Profile{}, &resellerdomain.LedgerEntry{}, &resellerdomain.BalanceAccount{}, &resellerdomain.WithdrawRequest{}); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	profile := seedResellerProfile(t, db, "pg-reseller-withdraw-"+suffix+"@example.com")
	now := time.Now()
	entry := resellerdomain.LedgerEntry{
		ResellerID:     profile.ID,
		Type:           resellerdomain.LedgerTypeOrderProfit,
		Amount:         money.FromDecimal(decimal.NewFromInt(100)),
		Currency:       "USD",
		IdempotencyKey: "pg-order-profit-" + suffix,
		Status:         resellerdomain.LedgerStatusAvailable,
		AvailableAt:    &now,
	}
	if err := db.Create(&entry).Error; err != nil {
		t.Fatalf("create ledger failed: %v", err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		repoTx := New(tx)
		if _, err := repoTx.GetOrCreateBalanceAccountForUpdate(profile.ID, "USD"); err != nil {
			return err
		}
		rows, err := repoTx.ListAvailableLedgerEntriesForUpdate(profile.ID, "USD")
		if err != nil {
			return err
		}
		if len(rows) != 1 || rows[0].ID != entry.ID {
			t.Fatalf("expected locked ledger %d, got %+v", entry.ID, rows)
		}
		return nil
	}); err != nil {
		t.Fatalf("transaction failed: %v", err)
	}
}
