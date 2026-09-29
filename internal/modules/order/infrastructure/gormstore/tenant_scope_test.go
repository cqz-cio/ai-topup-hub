package gormstore

import (
	"fmt"
	"testing"
	"time"

	ordercontract "github.com/dujiao-next/internal/modules/order/contract"

	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func openOrderTenantScopeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:order_tenant_scope_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&userdomain.User{}, &orderdomain.Order{}, &orderdomain.OrderItem{}, &fulfillmentdomain.Fulfillment{}); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	return db
}

func seedScopedOrder(t *testing.T, db *gorm.DB, orderNo string, userID uint, guestEmail string, guestPassword string, status string, resellerID *uint, parentID *uint) orderdomain.Order {
	t.Helper()
	order := orderdomain.Order{
		OrderNo:          orderNo,
		ParentID:         parentID,
		UserID:           userID,
		GuestEmail:       guestEmail,
		GuestPassword:    guestPassword,
		Status:           status,
		Currency:         "USD",
		OriginalAmount:   money.FromDecimal(decimal.NewFromInt(10)),
		TotalAmount:      money.FromDecimal(decimal.NewFromInt(10)),
		OnlinePaidAmount: money.FromDecimal(decimal.NewFromInt(10)),
		ResellerID:       resellerID,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("create order %s failed: %v", orderNo, err)
	}
	return order
}

func assertOrderFound(t *testing.T, order *orderdomain.Order, wantOrderNo string) {
	t.Helper()
	if order == nil {
		t.Fatalf("expected order %s, got nil", wantOrderNo)
	}
	if order.OrderNo != wantOrderNo {
		t.Fatalf("expected order_no %s, got %s", wantOrderNo, order.OrderNo)
	}
}

func assertOrderMissing(t *testing.T, order *orderdomain.Order) {
	t.Helper()
	if order != nil {
		t.Fatalf("expected nil order, got %+v", order)
	}
}

func TestOrderRepositoryTenantScopePointQueriesAndLists(t *testing.T) {
	db := openOrderTenantScopeTestDB(t)
	repo := New(db, "test-guest-credential-secret-with-32-bytes")
	user := userdomain.User{Email: "scope-user@example.com", PasswordHash: "hash"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	resellerOne := uint(101)
	resellerTwo := uint(202)
	mainScope := ordercontract.TenantScope{}
	resellerOneScope := ordercontract.TenantScope{ResellerID: &resellerOne}
	resellerTwoScope := ordercontract.TenantScope{ResellerID: &resellerTwo}

	mainOrder := seedScopedOrder(t, db, "SCOPE-MAIN", user.ID, "", "", constants.OrderStatusPendingPayment, nil, nil)
	resellerOrder := seedScopedOrder(t, db, "SCOPE-R1", user.ID, "", "", constants.OrderStatusPaid, &resellerOne, nil)
	resellerTwoOrder := seedScopedOrder(t, db, "SCOPE-R2", user.ID, "", "", constants.OrderStatusPaid, &resellerTwo, nil)
	child := seedScopedOrder(t, db, "SCOPE-R1-CHILD", user.ID, "", "", constants.OrderStatusPaid, &resellerOne, &resellerOrder.ID)
	guestMain := seedScopedOrder(t, db, "GUEST-MAIN", 0, "guest@example.com", "code", constants.OrderStatusPendingPayment, nil, nil)
	guestReseller := seedScopedOrder(t, db, "GUEST-R1", 0, "guest@example.com", "code", constants.OrderStatusPaid, &resellerOne, nil)
	_ = resellerTwoOrder
	_ = child
	_ = guestMain
	_ = guestReseller
	if _, err := repo.BackfillGuestCredentialHashes(); err != nil {
		t.Fatalf("hash seeded guest credentials failed: %v", err)
	}

	got, err := repo.GetByOrderNoAndUserScoped(mainOrder.OrderNo, user.ID, mainScope)
	if err != nil {
		t.Fatalf("GetByOrderNoAndUserScoped main failed: %v", err)
	}
	assertOrderFound(t, got, mainOrder.OrderNo)
	got, err = repo.GetByOrderNoAndUserScoped(mainOrder.OrderNo, user.ID, resellerOneScope)
	if err != nil {
		t.Fatalf("GetByOrderNoAndUserScoped main from reseller failed: %v", err)
	}
	assertOrderMissing(t, got)

	got, err = repo.GetByIDAndUserScoped(resellerOrder.ID, user.ID, resellerOneScope)
	if err != nil {
		t.Fatalf("GetByIDAndUserScoped reseller failed: %v", err)
	}
	assertOrderFound(t, got, resellerOrder.OrderNo)
	got, err = repo.GetByIDAndUserScoped(resellerOrder.ID, user.ID, mainScope)
	if err != nil {
		t.Fatalf("GetByIDAndUserScoped reseller from main failed: %v", err)
	}
	assertOrderMissing(t, got)
	got, err = repo.GetByIDAndUserScoped(resellerOrder.ID, user.ID, resellerTwoScope)
	if err != nil {
		t.Fatalf("GetByIDAndUserScoped reseller from other reseller failed: %v", err)
	}
	assertOrderMissing(t, got)

	got, err = repo.GetAnyByOrderNoAndUserScoped("SCOPE-R1-CHILD", user.ID, resellerOneScope)
	if err != nil {
		t.Fatalf("GetAnyByOrderNoAndUserScoped child failed: %v", err)
	}
	assertOrderFound(t, got, "SCOPE-R1-CHILD")
	got, err = repo.GetAnyByOrderNoAndUserScoped("SCOPE-R1-CHILD", user.ID, mainScope)
	if err != nil {
		t.Fatalf("GetAnyByOrderNoAndUserScoped child from main failed: %v", err)
	}
	assertOrderMissing(t, got)

	guestGot, err := repo.GetByOrderNoAndGuestScoped("GUEST-MAIN", "guest@example.com", "code", mainScope)
	if err != nil {
		t.Fatalf("GetByOrderNoAndGuestScoped main failed: %v", err)
	}
	assertOrderFound(t, guestGot, "GUEST-MAIN")
	guestGot, err = repo.GetByOrderNoAndGuestScoped("GUEST-MAIN", "guest@example.com", "code", resellerOneScope)
	if err != nil {
		t.Fatalf("GetByOrderNoAndGuestScoped main from reseller failed: %v", err)
	}
	assertOrderMissing(t, guestGot)
	guestGot, err = repo.GetByIDAndGuestScoped(guestReseller.ID, "guest@example.com", "code", resellerOneScope)
	if err != nil {
		t.Fatalf("GetByIDAndGuestScoped reseller failed: %v", err)
	}
	assertOrderFound(t, guestGot, "GUEST-R1")
	guestGot, err = repo.GetByIDAndGuestScoped(guestReseller.ID, "guest@example.com", "code", resellerTwoScope)
	if err != nil {
		t.Fatalf("GetByIDAndGuestScoped reseller from other reseller failed: %v", err)
	}
	assertOrderMissing(t, guestGot)

	mainRows, mainTotal, err := repo.ListByUserScoped(ordercontract.ListFilter{UserID: user.ID, Page: 1, PageSize: 20}, mainScope)
	if err != nil {
		t.Fatalf("ListByUserScoped main failed: %v", err)
	}
	if mainTotal != 1 || len(mainRows) != 1 || mainRows[0].OrderNo != "SCOPE-MAIN" {
		t.Fatalf("main list mismatch total=%d rows=%+v", mainTotal, mainRows)
	}
	resellerRows, resellerTotal, err := repo.ListByUserScoped(ordercontract.ListFilter{UserID: user.ID, Page: 1, PageSize: 20}, resellerOneScope)
	if err != nil {
		t.Fatalf("ListByUserScoped reseller failed: %v", err)
	}
	if resellerTotal != 1 || len(resellerRows) != 1 || resellerRows[0].OrderNo != "SCOPE-R1" {
		t.Fatalf("reseller list mismatch total=%d rows=%+v", resellerTotal, resellerRows)
	}

	mainStats, err := repo.StatsByUserScoped(ordercontract.ListFilter{UserID: user.ID}, mainScope)
	if err != nil {
		t.Fatalf("StatsByUserScoped main failed: %v", err)
	}
	if mainStats[constants.OrderStatusPendingPayment] != 1 || len(mainStats) != 1 {
		t.Fatalf("main stats mismatch: %+v", mainStats)
	}
	resellerStats, err := repo.StatsByUserScoped(ordercontract.ListFilter{UserID: user.ID}, resellerOneScope)
	if err != nil {
		t.Fatalf("StatsByUserScoped reseller failed: %v", err)
	}
	if resellerStats[constants.OrderStatusPaid] != 1 || len(resellerStats) != 1 {
		t.Fatalf("reseller stats mismatch: %+v", resellerStats)
	}

	guestRows, guestTotal, err := repo.ListByGuestScoped("guest@example.com", "code", 1, 20, resellerOneScope)
	if err != nil {
		t.Fatalf("ListByGuestScoped reseller failed: %v", err)
	}
	if guestTotal != 1 || len(guestRows) != 1 || guestRows[0].OrderNo != "GUEST-R1" {
		t.Fatalf("guest reseller list mismatch total=%d rows=%+v", guestTotal, guestRows)
	}
}

func TestOrderStoreExcludesSoftDeletedAggregatesAndAssociations(t *testing.T) {
	db := openOrderTenantScopeTestDB(t)
	store := New(db, "test-guest-credential-secret-with-32-bytes")
	now := time.Now().UTC().Truncate(time.Second)

	deletedRoot := seedScopedOrder(t, db, "SOFT-DELETED-ROOT", 1, "", "", constants.OrderStatusPaid, nil, nil)
	if err := db.Model(&orderdomain.Order{}).Where("id = ?", deletedRoot.ID).Update("deleted_at", now).Error; err != nil {
		t.Fatalf("soft delete root order failed: %v", err)
	}
	got, err := store.GetByID(deletedRoot.ID)
	if err != nil {
		t.Fatalf("get deleted root failed: %v", err)
	}
	assertOrderMissing(t, got)

	parent := seedScopedOrder(t, db, "SOFT-ACTIVE-PARENT", 1, "", "", constants.OrderStatusPaid, nil, nil)
	activeChild := seedScopedOrder(t, db, "SOFT-ACTIVE-CHILD", 1, "", "", constants.OrderStatusPaid, nil, &parent.ID)
	deletedChild := seedScopedOrder(t, db, "SOFT-DELETED-CHILD", 1, "", "", constants.OrderStatusPaid, nil, &parent.ID)
	if err := db.Model(&orderdomain.Order{}).Where("id = ?", deletedChild.ID).Update("deleted_at", now).Error; err != nil {
		t.Fatalf("soft delete child failed: %v", err)
	}

	activeItem := orderdomain.OrderItem{
		OrderID: parent.ID, ProductID: 1, Quantity: 1, TitleJSON: jsonmap.JSON{"zh-CN": "active"},
		UnitPrice: money.FromDecimal(decimal.NewFromInt(10)), TotalPrice: money.FromDecimal(decimal.NewFromInt(10)),
		CreatedAt: now, UpdatedAt: now,
	}
	deletedItem := activeItem
	deletedItem.ProductID = 2
	if err := db.Create(&activeItem).Error; err != nil {
		t.Fatalf("create active item failed: %v", err)
	}
	if err := db.Create(&deletedItem).Error; err != nil {
		t.Fatalf("create deleted item failed: %v", err)
	}
	if err := db.Model(&orderdomain.OrderItem{}).Where("id = ?", deletedItem.ID).Update("deleted_at", now).Error; err != nil {
		t.Fatalf("soft delete item failed: %v", err)
	}

	activeChildItem := activeItem
	activeChildItem.ID = 0
	activeChildItem.OrderID = activeChild.ID
	activeChildItem.ProductID = 3
	if err := db.Create(&activeChildItem).Error; err != nil {
		t.Fatalf("create active child item failed: %v", err)
	}

	deletedFulfillment := fulfillmentdomain.Fulfillment{
		OrderID: parent.ID, Type: constants.FulfillmentTypeManual,
		Status: constants.FulfillmentStatusDelivered, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&deletedFulfillment).Error; err != nil {
		t.Fatalf("create fulfillment failed: %v", err)
	}
	if err := db.Model(&fulfillmentdomain.Fulfillment{}).Where("id = ?", deletedFulfillment.ID).Update("deleted_at", now).Error; err != nil {
		t.Fatalf("soft delete fulfillment failed: %v", err)
	}

	got, err = store.GetByID(parent.ID)
	if err != nil {
		t.Fatalf("get active parent failed: %v", err)
	}
	if got == nil {
		t.Fatal("active parent should remain visible")
	}
	if len(got.Items) != 1 || got.Items[0].ID != activeItem.ID {
		t.Fatalf("expected only active parent item, got %+v", got.Items)
	}
	if len(got.Children) != 1 || got.Children[0].ID != activeChild.ID {
		t.Fatalf("expected only active child, got %+v", got.Children)
	}
	if len(got.Children[0].Items) != 1 || got.Children[0].Items[0].ID != activeChildItem.ID {
		t.Fatalf("expected active child items only, got %+v", got.Children[0].Items)
	}
	if got.Fulfillment != nil {
		t.Fatalf("soft-deleted fulfillment must not preload, got %+v", got.Fulfillment)
	}
}
