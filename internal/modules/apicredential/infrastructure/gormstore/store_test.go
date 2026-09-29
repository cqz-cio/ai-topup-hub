package gormstore_test

import (
	"fmt"
	"testing"
	"time"

	apicredentialcontract "github.com/dujiao-next/internal/modules/apicredential/contract"
	apicredentialdomain "github.com/dujiao-next/internal/modules/apicredential/domain"
	"github.com/dujiao-next/internal/modules/apicredential/infrastructure/gormstore"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestApiCredentialStoreSoftDeleteVisibilityAndRestorePath(t *testing.T) {
	dsn := fmt.Sprintf("file:api_credential_store_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&apicredentialdomain.ApiCredential{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	store := gormstore.New(db)
	credential := &apicredentialdomain.ApiCredential{
		UserID:   42,
		ApiKey:   "deleted-key",
		Status:   "approved",
		IsActive: true,
	}
	if err := store.Create(credential); err != nil {
		t.Fatalf("create credential: %v", err)
	}
	if err := store.Delete(credential.ID); err != nil {
		t.Fatalf("delete credential: %v", err)
	}

	if got, err := store.GetByID(credential.ID); err != nil || got != nil {
		t.Fatalf("GetByID after delete = (%v, %v), want (nil, nil)", got, err)
	}
	if got, err := store.GetByUserID(credential.UserID); err != nil || got != nil {
		t.Fatalf("GetByUserID after delete = (%v, %v), want (nil, nil)", got, err)
	}
	if got, err := store.GetByApiKey(credential.ApiKey); err != nil || got != nil {
		t.Fatalf("GetByApiKey after delete = (%v, %v), want (nil, nil)", got, err)
	}
	if got, total, err := store.List(apicredentialcontract.ListFilter{}); err != nil || total != 0 || len(got) != 0 {
		t.Fatalf("List after delete = (%v, %d, %v), want empty", got, total, err)
	}

	deleted, err := store.GetAnyByUserID(credential.UserID)
	if err != nil || deleted == nil || deleted.DeletedAt == nil {
		t.Fatalf("GetAnyByUserID after delete = (%v, %v), want deleted row", deleted, err)
	}
	deleted.DeletedAt = nil
	if err := store.UpdateAny(deleted); err != nil {
		t.Fatalf("restore credential: %v", err)
	}
	if got, err := store.GetByID(credential.ID); err != nil || got == nil {
		t.Fatalf("GetByID after restore = (%v, %v), want visible row", got, err)
	}
}
