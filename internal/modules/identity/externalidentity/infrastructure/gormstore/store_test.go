package externalidentitystore

import (
	"testing"
	"time"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	externalidentitycontract "github.com/dujiao-next/internal/modules/identity/externalidentity/contract"
	externalidentitydomain "github.com/dujiao-next/internal/modules/identity/externalidentity/domain"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestListTelegramUsers(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&userdomain.User{}, &externalidentitydomain.Identity{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	user1 := &userdomain.User{
		Email:        "alice@example.com",
		PasswordHash: "hash",
		DisplayName:  "Alice",
		Status:       "active",
	}
	user2 := &userdomain.User{
		Email:        "bob@example.com",
		PasswordHash: "hash",
		DisplayName:  "Bob",
		Status:       "active",
	}
	if err := db.Create(user1).Error; err != nil {
		t.Fatalf("create user1 failed: %v", err)
	}
	if err := db.Create(user2).Error; err != nil {
		t.Fatalf("create user2 failed: %v", err)
	}

	boundAt1 := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	boundAt2 := time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)
	identity1 := &externalidentitydomain.Identity{
		UserID:         user1.ID,
		Provider:       "telegram",
		ProviderUserID: "10001",
		Username:       "alice_tg",
		CreatedAt:      boundAt1,
		UpdatedAt:      boundAt1,
	}
	identity2 := &externalidentitydomain.Identity{
		UserID:         user2.ID,
		Provider:       "telegram",
		ProviderUserID: "20002",
		Username:       "bob_tg",
		CreatedAt:      boundAt2,
		UpdatedAt:      boundAt2,
	}
	if err := db.Create(identity1).Error; err != nil {
		t.Fatalf("create identity1 failed: %v", err)
	}
	if err := db.Create(identity2).Error; err != nil {
		t.Fatalf("create identity2 failed: %v", err)
	}

	repo := New(db)

	items, total, err := repo.ListTelegramUsers(externalidentitycontract.TelegramUserFilter{
		Keyword:  "alice",
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("list telegram users failed: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
	if len(items) != 1 || items[0].TelegramUserID != "10001" {
		t.Fatalf("unexpected list result: %+v", items)
	}

	items, total, err = repo.ListTelegramUsers(externalidentitycontract.TelegramUserFilter{
		TelegramUserID: "200",
		Page:           1,
		PageSize:       10,
	})
	if err != nil {
		t.Fatalf("list by telegram id failed: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].UserID != user2.ID {
		t.Fatalf("unexpected list by telegram id: total=%d items=%+v", total, items)
	}

	items, total, err = repo.ListTelegramUsers(externalidentitycontract.TelegramUserFilter{
		CreatedFrom: ptrTime(boundAt2.Add(-time.Hour)),
		CreatedTo:   ptrTime(boundAt2.Add(time.Hour)),
		Page:        1,
		PageSize:    10,
	})
	if err != nil {
		t.Fatalf("list by created range failed: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].TelegramUsername != "bob_tg" {
		t.Fatalf("unexpected list by created range: total=%d items=%+v", total, items)
	}

	got, err := repo.GetByProviderUserID(" Telegram ", "10001")
	if err != nil {
		t.Fatalf("get normalized provider identity failed: %v", err)
	}
	if got == nil || got.ID != identity1.ID {
		t.Fatalf("unexpected normalized provider identity: %#v", got)
	}
	got.Username = "alice_updated"
	if err := repo.Update(got); err != nil {
		t.Fatalf("update identity failed: %v", err)
	}
	byUser, err := repo.GetByUserProvider(user1.ID, "telegram")
	if err != nil {
		t.Fatalf("get user provider identity failed: %v", err)
	}
	if byUser == nil || byUser.Username != "alice_updated" {
		t.Fatalf("identity update was not persisted: %#v", byUser)
	}
	if err := repo.DeleteByID(identity1.ID); err != nil {
		t.Fatalf("delete identity failed: %v", err)
	}
	deleted, err := repo.GetByProviderUserID("telegram", "10001")
	if err != nil {
		t.Fatalf("get deleted identity failed: %v", err)
	}
	if deleted != nil {
		t.Fatalf("deleted identity must be absent: %#v", deleted)
	}
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
