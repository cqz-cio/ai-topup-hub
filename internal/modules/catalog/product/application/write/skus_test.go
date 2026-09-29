package productwrite

import (
	"sort"
	"testing"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

func TestSyncSingleProductSKUMultipleRowsKeepsSingleActive(t *testing.T) {
	service := NewWriteService(Options{})
	repo := newSyncSingleSKURepo(t)
	productID := uint(2001)

	inactiveDefault := productdomain.ProductSKU{
		ProductID:        productID,
		SKUCode:          productdomain.DefaultSKUCode,
		PriceAmount:      money.FromDecimal(decimal.NewFromInt(10)),
		ManualStockTotal: 9,
		IsActive:         false,
		SortOrder:        0,
	}
	firstActive := productdomain.ProductSKU{
		ProductID:        productID,
		SKUCode:          "A",
		PriceAmount:      money.FromDecimal(decimal.NewFromInt(20)),
		ManualStockTotal: 2,
		IsActive:         true,
		SortOrder:        2,
	}
	secondActive := productdomain.ProductSKU{
		ProductID:        productID,
		SKUCode:          "B",
		PriceAmount:      money.FromDecimal(decimal.NewFromInt(30)),
		ManualStockTotal: 4,
		IsActive:         true,
		SortOrder:        1,
	}
	if err := repo.Create(&inactiveDefault); err != nil {
		t.Fatalf("create inactive default sku failed: %v", err)
	}
	inactiveDefault.IsActive = false
	if err := repo.Update(&inactiveDefault); err != nil {
		t.Fatalf("update inactive default sku failed: %v", err)
	}
	if err := repo.Create(&firstActive); err != nil {
		t.Fatalf("create first active sku failed: %v", err)
	}
	if err := repo.Create(&secondActive); err != nil {
		t.Fatalf("create second active sku failed: %v", err)
	}

	targetPrice := decimal.RequireFromString("88.88")
	if err := service.syncSingleProductSKU(repo, productID, targetPrice, decimal.Zero, 5, true); err != nil {
		t.Fatalf("sync single sku failed: %v", err)
	}

	skus, err := repo.ListByProduct(productID, false)
	if err != nil {
		t.Fatalf("list sku failed: %v", err)
	}
	activeCount := 0
	for _, sku := range skus {
		if !sku.IsActive {
			continue
		}
		activeCount++
		if sku.ID != firstActive.ID {
			t.Fatalf("expected higher sort_order active sku id=%d, got id=%d", firstActive.ID, sku.ID)
		}
		if !sku.PriceAmount.Equal(targetPrice) || sku.ManualStockTotal != 5 {
			t.Fatalf("unexpected synchronized SKU: %+v", sku)
		}
	}
	if activeCount != 1 {
		t.Fatalf("expected exactly one active sku, got %d", activeCount)
	}
}

func TestSyncSingleProductSKUNoActivePrefersDefaultCode(t *testing.T) {
	service := NewWriteService(Options{})
	repo := newSyncSingleSKURepo(t)
	productID := uint(2002)

	inactiveA := productdomain.ProductSKU{
		ProductID:        productID,
		SKUCode:          "A",
		PriceAmount:      money.FromDecimal(decimal.NewFromInt(10)),
		ManualStockTotal: 3,
		IsActive:         false,
		SortOrder:        1,
	}
	inactiveDefault := productdomain.ProductSKU{
		ProductID:        productID,
		SKUCode:          productdomain.DefaultSKUCode,
		PriceAmount:      money.FromDecimal(decimal.NewFromInt(20)),
		ManualStockTotal: 8,
		IsActive:         false,
		SortOrder:        0,
	}
	if err := repo.Create(&inactiveA); err != nil {
		t.Fatalf("create inactive sku A failed: %v", err)
	}
	inactiveA.IsActive = false
	if err := repo.Update(&inactiveA); err != nil {
		t.Fatalf("update inactive sku A failed: %v", err)
	}
	if err := repo.Create(&inactiveDefault); err != nil {
		t.Fatalf("create inactive default sku failed: %v", err)
	}
	inactiveDefault.IsActive = false
	if err := repo.Update(&inactiveDefault); err != nil {
		t.Fatalf("update inactive default sku failed: %v", err)
	}

	targetPrice := decimal.RequireFromString("19.90")
	if err := service.syncSingleProductSKU(repo, productID, targetPrice, decimal.Zero, 6, true); err != nil {
		t.Fatalf("sync single sku failed: %v", err)
	}

	skus, err := repo.ListByProduct(productID, false)
	if err != nil {
		t.Fatalf("list sku failed: %v", err)
	}
	activeCount := 0
	for _, sku := range skus {
		if !sku.IsActive {
			continue
		}
		activeCount++
		if sku.ID != inactiveDefault.ID {
			t.Fatalf("expected default sku id=%d to be active, got id=%d", inactiveDefault.ID, sku.ID)
		}
		if !sku.PriceAmount.Equal(targetPrice) || sku.ManualStockTotal != 6 {
			t.Fatalf("unexpected synchronized DEFAULT SKU: %+v", sku)
		}
	}
	if activeCount != 1 {
		t.Fatalf("expected exactly one active sku, got %d", activeCount)
	}
}

type memorySKURepository struct {
	rows   []productdomain.ProductSKU
	nextID uint
}

func (repo *memorySKURepository) ListByProduct(productID uint, onlyActive bool) ([]productdomain.ProductSKU, error) {
	rows := make([]productdomain.ProductSKU, 0, len(repo.rows))
	for _, row := range repo.rows {
		if row.ProductID == productID && (!onlyActive || row.IsActive) {
			rows = append(rows, row)
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].SortOrder != rows[j].SortOrder {
			return rows[i].SortOrder > rows[j].SortOrder
		}
		return rows[i].ID < rows[j].ID
	})
	return rows, nil
}

func (repo *memorySKURepository) Create(item *productdomain.ProductSKU) error {
	if item.ID == 0 {
		repo.nextID++
		item.ID = repo.nextID
	}
	repo.rows = append(repo.rows, *item)
	return nil
}

func (repo *memorySKURepository) Update(item *productdomain.ProductSKU) error {
	for index := range repo.rows {
		if repo.rows[index].ID == item.ID {
			repo.rows[index] = *item
			return nil
		}
	}
	return nil
}

func (repo *memorySKURepository) Delete(id uint) error {
	for index := range repo.rows {
		if repo.rows[index].ID == id {
			repo.rows = append(repo.rows[:index], repo.rows[index+1:]...)
			break
		}
	}
	return nil
}

func (repo *memorySKURepository) PurgeSoftDeletedByProductAndCode(uint, string) error {
	return nil
}

func newSyncSingleSKURepo(t *testing.T) SKURepository {
	t.Helper()
	return &memorySKURepository{}
}
