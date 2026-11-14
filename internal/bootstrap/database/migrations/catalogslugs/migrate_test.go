package catalogslugs

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	productstore "github.com/dujiao-next/internal/modules/catalog/product/store/gormstore"
	settingsstore "github.com/dujiao-next/internal/modules/settings/infrastructure/gormstore"
	"github.com/dujiao-next/internal/platform/publiccontent"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestMigrationPreservesIDsSnapshotsAndReservedURLs(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&categorydomain.Category{}, &categorydomain.CategorySlugAlias{}, &productdomain.Product{}, &productdomain.ProductSlugAlias{}, &settingsstore.SettingRecord{}); err != nil {
		t.Fatal(err)
	}
	seed := []productdomain.Product{
		{ID: 1, Slug: "qzweb-5", CategoryID: 1, TitleJSON: jsonmap.JSON{"zh-CN": "ChatGPT Plus 兑换码"}, IsActive: true},
		{ID: 2, Slug: "chatgpt-plus-code", CategoryID: 1, TitleJSON: jsonmap.JSON{"zh-CN": "另一个商品"}, IsActive: true},
		{ID: 3, Slug: "ellaix-grok-plus-month", CategoryID: 1, TitleJSON: jsonmap.JSON{"zh-CN": "Grok Plus"}, IsActive: true},
		{ID: 4, Slug: "qzweb-73", CategoryID: 1, TitleJSON: jsonmap.JSON{"zh-CN": "Codex 250 点数"}, IsActive: true},
		{ID: 5, Slug: "86gamestore-88", CategoryID: 1, TitleJSON: jsonmap.JSON{"zh-CN": "ChatGPT Plus 人工充值"}, IsActive: true},
		{ID: 6, Slug: "chatgpt-plus-manual", CategoryID: 2, TitleJSON: jsonmap.JSON{"zh-CN": "Another owner"}, IsActive: true},
	}
	if err = db.Create(&categorydomain.Category{ID: 1, Slug: "qzweb-category-4", NameJSON: jsonmap.JSON{"zh-CN": "GPT"}}).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&categorydomain.Category{ID: 2, Slug: "gpt-recharge", NameJSON: jsonmap.JSON{"zh-CN": "Other category"}}).Error; err != nil {
		t.Fatal(err)
	}
	for i := range seed {
		if err = db.Create(&seed[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err = db.Exec("CREATE TABLE historical_snapshot (product_id integer, sku_id integer, price text, payload text)").Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Exec("INSERT INTO historical_snapshot VALUES (1, 7, '88.00', 'original title and delivery snapshot')").Error; err != nil {
		t.Fatal(err)
	}
	snapshot := func(table string, stripSlug bool) string {
		var rows []map[string]interface{}
		if err := db.Table(table).Order("1").Find(&rows).Error; err != nil {
			t.Fatal(err)
		}
		if stripSlug {
			for _, row := range rows {
				delete(row, "slug")
			}
		}
		encoded, _ := json.Marshal(rows)
		return string(encoded)
	}
	beforeProduct, beforeCategory, beforeOrder := snapshot("products", true), snapshot("categories", true), snapshot("historical_snapshot", false)
	if err = Run(db); err != nil {
		t.Fatal(err)
	}
	if snapshot("products", true) != beforeProduct || snapshot("categories", true) != beforeCategory || snapshot("historical_snapshot", false) != beforeOrder {
		t.Fatal("non-URL data changed")
	}
	var product productdomain.Product
	db.First(&product, 1)
	if product.Slug != "chatgpt-plus-code-2" {
		t.Fatal(product.Slug)
	}
	repo := productstore.NewProductStore(db)
	links, err := repo.PublicCatalogLinks(context.Background(), publiccontent.GPTRecharge().CatalogIdentities)
	if err != nil {
		t.Fatal(err)
	}
	guide := publiccontent.GPTRecharge().Resolve(links)
	if guide.Path != "/categories/gpt-recharge-2" || guide.CategorySlug != "gpt-recharge-2" || links["/products/chatgpt-plus-code"] != "/products/chatgpt-plus-code-2" || links["/products/chatgpt-plus-manual"] != "/products/chatgpt-plus-manual-2" {
		t.Fatal("wrong editorial owner", links)
	}
	var occupied categorydomain.Category
	db.First(&occupied, 2)
	if occupied.Slug != "gpt-recharge" {
		t.Fatal("occupied canonical stolen")
	}
	db.Model(&categorydomain.Category{}).Where("id = ?", 1).UpdateColumn("slug", "later-category")
	renamed, err := repo.PublicCatalogLinks(context.Background(), publiccontent.GPTRecharge().CatalogIdentities)
	if err != nil || renamed["/categories/gpt-recharge"] != "/categories/later-category" {
		t.Fatal(renamed, err)
	}
	db.Model(&categorydomain.Category{}).Where("id = ?", 1).UpdateColumn("slug", "gpt-recharge-2")
	db.Model(&productdomain.Product{}).Where("id = ?", 5).UpdateColumn("is_active", false)
	hidden, err := repo.PublicCatalogLinks(context.Background(), publiccontent.GPTRecharge().CatalogIdentities)
	if err != nil || hidden["/products/chatgpt-plus-manual"] != "" {
		t.Fatal("hidden identity linked", hidden, err)
	}
	db.Model(&productdomain.Product{}).Where("id = ?", 5).UpdateColumn("is_active", true)
	var aliases []productdomain.ProductSlugAlias
	db.Order("slug").Find(&aliases)
	if err = Run(db); err != nil {
		t.Fatal(err)
	}
	var again []productdomain.ProductSlugAlias
	db.Order("slug").Find(&again)
	if !reflect.DeepEqual(aliases, again) {
		t.Fatal("migration not idempotent")
	}
	var unchanged productdomain.Product
	db.First(&unchanged, 3)
	if unchanged.Slug != seed[2].Slug {
		t.Fatal("readable slug changed")
	}
	var codex productdomain.Product
	db.First(&codex, 4)
	var oldCodex productdomain.ProductSlugAlias
	if codex.Slug == "qzweb-73" || db.First(&oldCodex, "slug = ?", "qzweb-73").Error != nil || oldCodex.ProductID != 4 {
		t.Fatal("additional legacy URL lost identity")
	}
	db.Model(&product).UpdateColumn("slug", "qzweb-5")
	if err = Run(db); err != nil {
		t.Fatal(err)
	}
	db.First(&product, 1)
	if product.Slug != "qzweb-5" {
		t.Fatal("operator rename undone")
	}
}
