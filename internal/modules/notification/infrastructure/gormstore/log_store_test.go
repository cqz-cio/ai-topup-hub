package gormstore_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/dujiao-next/internal/modules/notification/contract"
	"github.com/dujiao-next/internal/modules/notification/domain"
	"github.com/dujiao-next/internal/modules/notification/infrastructure/gormstore"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestLogStoreListAdminFiltersStatusChannelAndTestFlag(t *testing.T) {
	dsn := fmt.Sprintf("file:notification_log_store_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&domain.NotificationLog{}); err != nil {
		t.Fatalf("migrate notification log: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	items := []domain.NotificationLog{
		{EventType: "order_paid_success", Channel: "email", Recipient: "failed@example.com", Status: "failed", IsTest: false, CreatedAt: now},
		{EventType: "order_paid_success", Channel: "telegram", Recipient: "-100100", Status: "success", IsTest: true, CreatedAt: now.Add(time.Second)},
	}
	if err := db.Create(&items).Error; err != nil {
		t.Fatalf("seed notification logs: %v", err)
	}

	isTest := false
	rows, total, err := gormstore.NewLogStore(db).ListAdmin(contract.LogListFilter{
		Page: 1, PageSize: 10, Channel: "email", Status: "failed", IsTest: &isTest,
	})
	if err != nil {
		t.Fatalf("list notification logs: %v", err)
	}
	if total != 1 || len(rows) != 1 || rows[0].Recipient != "failed@example.com" {
		t.Fatalf("unexpected result total=%d rows=%#v", total, rows)
	}
}
