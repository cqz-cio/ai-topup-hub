package gormstore

import (
	"fmt"
	"testing"
	"time"

	downstreamcontract "github.com/dujiao-next/internal/modules/downstreamcallback/contract"
	downstreamdomain "github.com/dujiao-next/internal/modules/downstreamcallback/domain"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dsn := fmt.Sprintf("file:downstream_callback_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&downstreamdomain.OrderRef{}); err != nil {
		t.Fatalf("migrate order refs: %v", err)
	}
	return New(db)
}

func TestStorePersistsAndFindsOrderReference(t *testing.T) {
	store := openTestStore(t)
	ref := &downstreamdomain.OrderRef{
		OrderID:           8,
		ApiCredentialID:   5,
		DownstreamOrderNo: "remote-8",
		CallbackURL:       "https://callback.example.test",
		CallbackStatus:    downstreamdomain.StatusPending,
	}
	if err := store.Create(ref); err != nil {
		t.Fatalf("create ref: %v", err)
	}

	byOrder, err := store.GetByOrderID(ref.OrderID)
	if err != nil || byOrder == nil || byOrder.ID != ref.ID {
		t.Fatalf("get by order: ref=%#v err=%v", byOrder, err)
	}
	byRemote, err := store.GetByCredentialAndDownstreamNo(ref.ApiCredentialID, ref.DownstreamOrderNo)
	if err != nil || byRemote == nil || byRemote.ID != ref.ID {
		t.Fatalf("get by credential and downstream no: ref=%#v err=%v", byRemote, err)
	}
}

func TestStoreFiltersPendingAndCredentialLists(t *testing.T) {
	store := openTestStore(t)
	refs := []*downstreamdomain.OrderRef{
		{OrderID: 1, ApiCredentialID: 5, CallbackURL: "https://one.example", CallbackStatus: downstreamdomain.StatusPending},
		{OrderID: 2, ApiCredentialID: 5, CallbackURL: "", CallbackStatus: downstreamdomain.StatusPending},
		{OrderID: 3, ApiCredentialID: 5, CallbackURL: "https://three.example", CallbackStatus: downstreamdomain.StatusSent},
		{OrderID: 4, ApiCredentialID: 6, CallbackURL: "https://four.example", CallbackStatus: downstreamdomain.StatusPending},
	}
	for _, ref := range refs {
		if err := store.Create(ref); err != nil {
			t.Fatalf("create ref for order %d: %v", ref.OrderID, err)
		}
	}

	pending, err := store.ListPendingCallbacks(10)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if len(pending) != 2 || pending[0].OrderID != 1 || pending[1].OrderID != 4 {
		t.Fatalf("pending refs mismatch: %#v", pending)
	}

	listed, total, err := store.ListByCredentialID(5, downstreamcontract.RefListFilter{
		CallbackStatus: downstreamdomain.StatusPending,
		Page:           1,
		PageSize:       1,
	})
	if err != nil {
		t.Fatalf("list by credential: %v", err)
	}
	if total != 2 || len(listed) != 1 || listed[0].OrderID != 2 {
		t.Fatalf("credential list mismatch: total=%d refs=%#v", total, listed)
	}
}
