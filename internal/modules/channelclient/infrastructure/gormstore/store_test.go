package channelclientstore

import (
	"fmt"
	"testing"
	"time"

	channelclientdomain "github.com/dujiao-next/internal/modules/channelclient/domain"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dsn := fmt.Sprintf("file:channelclient_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&channelclientdomain.Client{}); err != nil {
		t.Fatalf("migrate channel client failed: %v", err)
	}
	return New(db)
}

func TestStorePreservesSoftDeleteAndLastUsedSemantics(t *testing.T) {
	store := newTestStore(t)
	client := &channelclientdomain.Client{
		Name:          "Telegram Bot",
		ChannelType:   "telegram_bot",
		ChannelKey:    "channel-key",
		ChannelSecret: "ciphertext",
		Status:        1,
	}
	if err := store.Create(client); err != nil {
		t.Fatalf("create client failed: %v", err)
	}

	usedAt := time.Now().UTC().Truncate(time.Millisecond)
	if err := store.UpdateLastUsed(client.ID, usedAt); err != nil {
		t.Fatalf("update last-used failed: %v", err)
	}
	active, err := store.FindActiveByChannelType("telegram_bot")
	if err != nil {
		t.Fatalf("find active client failed: %v", err)
	}
	if active == nil || active.LastUsedAt == nil || !active.LastUsedAt.Equal(usedAt) {
		t.Fatalf("unexpected active client: %#v", active)
	}

	deletedAt := time.Now().UTC()
	if err := store.Delete(client.ID, deletedAt); err != nil {
		t.Fatalf("soft delete client failed: %v", err)
	}
	byID, err := store.FindByID(client.ID)
	if err != nil {
		t.Fatalf("find soft-deleted client failed: %v", err)
	}
	if byID != nil {
		t.Fatalf("soft-deleted client must be hidden: %#v", byID)
	}
	items, err := store.FindAll()
	if err != nil {
		t.Fatalf("list clients failed: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("soft-deleted client must be absent from list: %#v", items)
	}
}
