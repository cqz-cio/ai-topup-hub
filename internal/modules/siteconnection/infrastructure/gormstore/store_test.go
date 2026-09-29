package gormstore

import (
	"fmt"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	siteconnectioncontract "github.com/dujiao-next/internal/modules/siteconnection/contract"
	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func setupSiteConnectionStoreTest(t *testing.T) (*gorm.DB, *Store) {
	t.Helper()
	dsn := fmt.Sprintf("file:siteconnection_store_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&siteconnectiondomain.Connection{}); err != nil {
		t.Fatalf("migrate site connection: %v", err)
	}
	return db, New(db)
}

func TestStoreDeleteHidesConnectionFromEveryReadPath(t *testing.T) {
	db, store := setupSiteConnectionStoreTest(t)
	connection := &siteconnectiondomain.Connection{
		Name:               "primary upstream",
		BaseURL:            "https://upstream.example.com",
		ApiKey:             "upstream-key",
		ApiSecret:          "encrypted-secret",
		Protocol:           constants.ConnectionProtocolDujiaoNext,
		Status:             constants.ConnectionStatusActive,
		RetryMax:           5,
		RetryIntervals:     "[30,60,300]",
		ExchangeRate:       decimal.NewFromInt(1),
		PriceMarkupPercent: decimal.Zero,
		PriceRoundingMode:  "none",
	}
	if err := store.Create(connection); err != nil {
		t.Fatalf("create connection: %v", err)
	}

	if err := store.Delete(connection.ID); err != nil {
		t.Fatalf("delete connection: %v", err)
	}

	if got, err := store.GetByID(connection.ID); err != nil || got != nil {
		t.Fatalf("GetByID after delete = %#v, %v; want nil, nil", got, err)
	}
	if got, err := store.GetByApiKey(connection.ApiKey); err != nil || got != nil {
		t.Fatalf("GetByApiKey after delete = %#v, %v; want nil, nil", got, err)
	}
	listed, total, err := store.List(siteconnectioncontract.ListFilter{})
	if err != nil {
		t.Fatalf("list connections: %v", err)
	}
	if total != 0 || len(listed) != 0 {
		t.Fatalf("list after delete = total %d rows %d; want empty", total, len(listed))
	}
	active, err := store.ListActive()
	if err != nil {
		t.Fatalf("list active connections: %v", err)
	}
	if len(active) != 0 {
		t.Fatalf("active list after delete has %d rows; want empty", len(active))
	}

	var persisted siteconnectiondomain.Connection
	if err := db.Where("id = ?", connection.ID).First(&persisted).Error; err != nil {
		t.Fatalf("load persisted deleted connection: %v", err)
	}
	if persisted.DeletedAt == nil {
		t.Fatal("delete must persist deleted_at instead of physically removing the row")
	}
}
