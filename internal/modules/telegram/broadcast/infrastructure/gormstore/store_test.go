package broadcaststore

import (
	"fmt"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	broadcastcontract "github.com/dujiao-next/internal/modules/telegram/broadcast/contract"
	broadcastdomain "github.com/dujiao-next/internal/modules/telegram/broadcast/domain"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupStoreTest(t *testing.T) *Store {
	t.Helper()
	dsn := fmt.Sprintf("file:telegram_broadcast_repo_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&broadcastdomain.Broadcast{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	return New(db)
}

func TestStoreListPagination(t *testing.T) {
	repo := setupStoreTest(t)
	now := time.Now().UTC().Truncate(time.Second)
	items := []broadcastdomain.Broadcast{
		{
			Title:         "Spring Promo",
			RecipientType: constants.TelegramBroadcastRecipientTypeAll,
			Status:        constants.TelegramBroadcastStatusCompleted,
			MessageHTML:   "<b>1</b>",
			CreatedAt:     now.Add(-2 * time.Hour),
			UpdatedAt:     now.Add(-2 * time.Hour),
		},
		{
			Title:         "VIP Users",
			RecipientType: constants.TelegramBroadcastRecipientTypeSpecific,
			Status:        constants.TelegramBroadcastStatusFailed,
			MessageHTML:   "<b>2</b>",
			CreatedAt:     now,
			UpdatedAt:     now,
		},
	}
	for i := range items {
		if err := repo.Create(&items[i]); err != nil {
			t.Fatalf("create broadcast failed: %v", err)
		}
	}
	deletedAt := now.Add(time.Hour)
	deleted := broadcastdomain.Broadcast{
		Title:         "Deleted",
		RecipientType: constants.TelegramBroadcastRecipientTypeAll,
		Status:        constants.TelegramBroadcastStatusCompleted,
		MessageHTML:   "<b>deleted</b>",
		DeletedAt:     &deletedAt,
	}
	if err := repo.Create(&deleted); err != nil {
		t.Fatalf("create deleted broadcast fixture failed: %v", err)
	}

	rows, total, err := repo.List(broadcastcontract.ListFilter{
		Page:     1,
		PageSize: 1,
	})
	if err != nil {
		t.Fatalf("list broadcasts failed: %v", err)
	}
	if total != 2 || len(rows) != 1 {
		t.Fatalf("unexpected filter result: total=%d rows=%d", total, len(rows))
	}
	if rows[0].Title != "VIP Users" {
		t.Fatalf("unexpected broadcast title: %s", rows[0].Title)
	}
	deletedRow, err := repo.GetByID(deleted.ID)
	if err != nil {
		t.Fatalf("get deleted broadcast failed: %v", err)
	}
	if deletedRow != nil {
		t.Fatalf("soft-deleted broadcast must be excluded: %#v", deletedRow)
	}
}
