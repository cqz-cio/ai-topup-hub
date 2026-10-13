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

func TestSuccessfulDeliveryReceiptIsScopedToOrderChannelAndRecipient(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:sale_receipts_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domain.NotificationLog{}); err != nil {
		t.Fatal(err)
	}
	store := gormstore.NewLogStore(db)
	for _, item := range []domain.NotificationLog{
		{EventType: "order_paid_success", BizType: "order", BizID: 10, Channel: "email", Recipient: "owner@example.com", Status: "failed"},
		{EventType: "order_paid_success", BizType: "order", BizID: 10, Channel: "email", Recipient: "owner@example.com", Status: "success", IsTest: true},
		{EventType: "order_paid_success", BizType: "order", BizID: 11, Channel: "email", Recipient: "owner@example.com", Status: "success"},
	} {
		if err := store.Create(&item); err != nil {
			t.Fatal(err)
		}
	}
	if found, err := store.HasSuccessfulDelivery("order_paid_success", "order", 10, "email", "owner@example.com"); err != nil || found {
		t.Fatalf("test/failed/other-order receipts must not suppress delivery: %v %v", found, err)
	}
	if err := store.Create(&domain.NotificationLog{EventType: "order_paid_success", BizType: "order", BizID: 10, Channel: "email", Recipient: "owner@example.com", Status: "success"}); err != nil {
		t.Fatal(err)
	}
	if found, err := store.HasSuccessfulDelivery("order_paid_success", "order", 10, "email", "owner@example.com"); err != nil || !found {
		t.Fatalf("missing successful receipt: %v %v", found, err)
	}
	if found, err := store.HasSuccessfulDelivery("order_paid_success", "order", 10, "email", "backup@example.com"); err != nil || found {
		t.Fatalf("recipient receipt leaked: %v %v", found, err)
	}
	if found, err := store.HasSuccessfulDelivery("order_paid_success", "order", 10, "telegram", "owner@example.com"); err != nil || found {
		t.Fatalf("channel receipt leaked: %v %v", found, err)
	}
}
