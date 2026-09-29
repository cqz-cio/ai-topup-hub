package adminstore

import (
	"testing"
	"time"

	admindomain "github.com/dujiao-next/internal/modules/identity/admin/domain"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupStoreTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&admindomain.Admin{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	return db
}

func TestStoreUpdatePasswordRewritesHashAndForcesLogout(t *testing.T) {
	db := setupStoreTestDB(t)
	store := New(db)

	admin := &admindomain.Admin{
		Username:     "super",
		PasswordHash: "old-hash",
		IsSuper:      true,
		TokenVersion: 3,
	}
	if err := store.Create(admin); err != nil {
		t.Fatalf("create: %v", err)
	}

	before := time.Now().Add(-time.Second)
	if err := store.UpdatePassword(admin.ID, "new-hash"); err != nil {
		t.Fatalf("UpdatePassword: %v", err)
	}
	after := time.Now().Add(time.Second)

	got, err := store.GetByID(admin.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.PasswordHash != "new-hash" {
		t.Errorf("PasswordHash = %q, want new-hash", got.PasswordHash)
	}
	if got.TokenVersion != 4 {
		t.Errorf("TokenVersion = %d, want 4 (bumped from 3)", got.TokenVersion)
	}
	if got.TokenInvalidBefore == nil {
		t.Fatal("TokenInvalidBefore should be set to force logout")
	}
	if got.TokenInvalidBefore.Before(before) || got.TokenInvalidBefore.After(after) {
		t.Errorf("TokenInvalidBefore = %v, want between %v and %v", got.TokenInvalidBefore, before, after)
	}
}

func TestStoreUpdatePasswordRejectsZeroID(t *testing.T) {
	db := setupStoreTestDB(t)
	store := New(db)

	if err := store.UpdatePassword(0, "hash"); err == nil {
		t.Fatal("expected error for zero admin id")
	}
}

func TestStoreUpdatePasswordRejectsEmptyHash(t *testing.T) {
	db := setupStoreTestDB(t)
	store := New(db)

	admin := &admindomain.Admin{Username: "u", PasswordHash: "h"}
	if err := store.Create(admin); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := store.UpdatePassword(admin.ID, ""); err == nil {
		t.Fatal("expected error for empty hash")
	}

	got, err := store.GetByID(admin.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.PasswordHash != "h" {
		t.Errorf("PasswordHash should be unchanged, got %q", got.PasswordHash)
	}
}

func TestStoreSoftDeleteExcludesAdminFromReadsAndMutations(t *testing.T) {
	db := setupStoreTestDB(t)
	store := New(db)
	admin := &admindomain.Admin{Username: "deleted", PasswordHash: "old", TokenVersion: 2}
	if err := store.Create(admin); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := store.Delete(admin.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	byID, err := store.GetByID(admin.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if byID != nil {
		t.Fatalf("deleted admin returned by id: %#v", byID)
	}
	byUsername, err := store.GetByUsername(admin.Username)
	if err != nil {
		t.Fatalf("get by username: %v", err)
	}
	if byUsername != nil {
		t.Fatalf("deleted admin returned by username: %#v", byUsername)
	}
	admins, err := store.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(admins) != 0 {
		t.Fatalf("list contains deleted admin: %#v", admins)
	}
	count, err := store.Count()
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0", count)
	}
	if err := store.UpdatePassword(admin.ID, "new"); err != nil {
		t.Fatalf("update deleted password: %v", err)
	}

	var persisted admindomain.Admin
	if err := db.Where("id = ?", admin.ID).First(&persisted).Error; err != nil {
		t.Fatalf("load raw row: %v", err)
	}
	if persisted.PasswordHash != "old" || persisted.TokenVersion != 2 {
		t.Fatalf("deleted admin was mutated: %#v", persisted)
	}
}
