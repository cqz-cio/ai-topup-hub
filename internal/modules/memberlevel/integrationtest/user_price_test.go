package integrationtest

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	producthttp "github.com/dujiao-next/internal/modules/catalog/product/transport/http"
	productpresenter "github.com/dujiao-next/internal/modules/catalog/product/transport/presenter"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	memberlevelapp "github.com/dujiao-next/internal/modules/memberlevel/application"
	memberlevelcontract "github.com/dujiao-next/internal/modules/memberlevel/contract"
	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"
	"github.com/dujiao-next/internal/modules/memberlevel/infrastructure/gormstore"
	memberlevelhttp "github.com/dujiao-next/internal/modules/memberlevel/transport/http"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func userPriceFixture(t *testing.T) (*memberlevelapp.Service, *gorm.DB, userdomain.User, userdomain.User, productdomain.Product) {
	t.Helper()
	_, db := newMemberLevelServiceForTest(t)
	if err := db.AutoMigrate(&productdomain.Product{}, &productdomain.ProductSKU{}, &memberleveldomain.UserPrice{}); err != nil {
		t.Fatal(err)
	}
	level := createMemberLevelFixture(t, db, "personal-price-grade", 5, "0", false)
	if err := db.Model(&level).Update("discount_rate", 80).Error; err != nil {
		t.Fatal(err)
	}
	a := createUserFixture(t, db, "price-a@example.invalid", level.ID)
	b := createUserFixture(t, db, "price-b@example.invalid", level.ID)
	product := productdomain.Product{Slug: "personal-price-product", TitleJSON: jsonmap.JSON{"zh-CN": "专属价格测试商品"}, IsActive: true, FulfillmentType: "manual", PriceAmount: money.FromDecimal(decimal.NewFromInt(100))}
	if err := db.Create(&product).Error; err != nil {
		t.Fatal(err)
	}
	product.SKUs = []productdomain.ProductSKU{
		{ProductID: product.ID, SKUCode: "first", IsActive: true, ManualStockTotal: -1, PriceAmount: money.FromDecimal(decimal.NewFromInt(100))},
		{ProductID: product.ID, SKUCode: "second", IsActive: true, ManualStockTotal: -1, PriceAmount: money.FromDecimal(decimal.NewFromInt(200))},
	}
	if err := db.Create(&product.SKUs).Error; err != nil {
		t.Fatal(err)
	}
	svc := memberlevelapp.NewService(gormstore.NewLevelStore(db), gormstore.NewPriceStore(db), gormstore.NewUserStore(db), gormstore.NewUserPriceStore(db))
	return svc, db, a, b, product
}

func personalPrice(userID, productID, skuID uint, amount string) memberleveldomain.UserPrice {
	return memberleveldomain.UserPrice{UserID: userID, ProductID: productID, SKUID: skuID, PriceAmount: money.Amount{Decimal: decimal.RequireFromString(amount)}}
}

func TestUserPricesAreIndependentExactAndClearable(t *testing.T) {
	svc, db, a, b, product := userPriceFixture(t)
	rows := []memberleveldomain.UserPrice{personalPrice(a.ID, product.ID, 0, "95.55"), personalPrice(a.ID, product.ID, product.SKUs[1].ID, "240.00")}
	if err := svc.ReplaceUserPrices(a.ID, rows); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		user, sku uint
		want      string
	}{
		{a.ID, product.SKUs[0].ID, "95.55"}, {a.ID, product.SKUs[1].ID, "240.00"}, {a.ID, 0, "95.55"}, {b.ID, product.SKUs[0].ID, ""}, {0, product.SKUs[0].ID, ""},
	} {
		price, err := svc.ResolveUserPrice(test.user, product.ID, test.sku)
		if err != nil {
			t.Fatal(err)
		}
		if test.want == "" {
			if price != nil {
				t.Fatal("another user's price leaked")
			}
			continue
		}
		if price == nil || price.StringFixed(2) != test.want {
			t.Fatalf("price = %v, want %s", price, test.want)
		}
	}
	if err := svc.SetUserLevel(a.ID, 0); err != nil {
		t.Fatal(err)
	}
	price, err := svc.ResolveUserPrice(a.ID, product.ID, product.SKUs[0].ID)
	if err != nil || price == nil || price.StringFixed(2) != "95.55" {
		t.Fatal("fixed prices must work without a level")
	}
	var original productdomain.ProductSKU
	if err := db.First(&original, product.SKUs[0].ID).Error; err != nil || original.PriceAmount.StringFixed(2) != "100.00" {
		t.Fatal("global price was changed")
	}
	if err := svc.ReplaceUserPrices(a.ID, []memberleveldomain.UserPrice{personalPrice(a.ID, product.ID, 0, "92.33")}); err != nil {
		t.Fatal(err)
	}
	price, _ = svc.ResolveUserPrice(a.ID, product.ID, product.SKUs[1].ID)
	if price == nil || price.StringFixed(2) != "92.33" {
		t.Fatal("cleared SKU must inherit the product price")
	}
	if err := svc.ReplaceUserPrices(a.ID, nil); err != nil {
		t.Fatal(err)
	}
	price, err = svc.ResolveUserPrice(a.ID, product.ID, product.SKUs[0].ID)
	if err != nil || price != nil {
		t.Fatal("clearing must restore ordinary pricing")
	}
}

func TestInvalidUserPricesDoNotReplaceSavedPrices(t *testing.T) {
	svc, db, a, b, product := userPriceFixture(t)
	valid := personalPrice(a.ID, product.ID, 0, "77.77")
	if err := svc.ReplaceUserPrices(a.ID, []memberleveldomain.UserPrice{valid}); err != nil {
		t.Fatal(err)
	}
	other := productdomain.Product{Slug: "other-personal-product", TitleJSON: jsonmap.JSON{"zh-CN": "另一商品"}}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	invalid := [][]memberleveldomain.UserPrice{
		{personalPrice(a.ID, product.ID, 0, "0")}, {personalPrice(a.ID, product.ID, 0, "-1")},
		{personalPrice(a.ID, product.ID, 0, "1.001")}, {personalPrice(a.ID, product.ID, 0, "10000000000")},
		{valid, valid}, {personalPrice(b.ID, product.ID, 0, "9")},
		{personalPrice(a.ID, other.ID, product.SKUs[0].ID, "9")}, {personalPrice(a.ID, 999999, 0, "9")},
	}
	for _, rows := range invalid {
		if err := svc.ReplaceUserPrices(a.ID, rows); !errors.Is(err, memberlevelcontract.ErrPriceInvalid) {
			t.Fatalf("invalid prices accepted: %v", err)
		}
		price, err := svc.ResolveUserPrice(a.ID, product.ID, product.SKUs[0].ID)
		if err != nil || price == nil || price.StringFixed(2) != "77.77" {
			t.Fatal("invalid save changed the old price")
		}
	}
	if err := svc.ReplaceUserPrices(999999, nil); !errors.Is(err, memberlevelcontract.ErrUserNotFound) {
		t.Fatalf("missing user = %v", err)
	}
	// Force an insertion failure after DELETE; the transaction must restore old rows.
	if err := gormstore.NewUserPriceStore(db).ReplaceByUser(a.ID, []memberleveldomain.UserPrice{valid, valid}); err == nil {
		t.Fatal("duplicate insertion should fail")
	}
	price, _ := svc.ResolveUserPrice(a.ID, product.ID, 0)
	if price == nil || price.StringFixed(2) != "77.77" {
		t.Fatal("failed transaction lost saved prices")
	}
}

func TestUserPriceAdminHTTPValidatesAmountsAndEmptyList(t *testing.T) {
	svc, _, a, _, product := userPriceFixture(t)
	router := gin.New()
	memberlevelhttp.RegisterUserPriceAdminRoutes(router.Group("/admin"), memberlevelhttp.NewUserPriceAdminHandler(svc))
	path := "/admin/users/" + strconv.Itoa(int(a.ID)) + "/prices"
	for _, body := range []string{`{}`, `{"prices":null}`, `{"prices":[{"product_id":1,"price_amount":10}]}`, `{"prices":[{"product_id":1,"price_amount":"1e2"}]}`, `{"prices":[{"product_id":1,"price_amount":"1.001"}]}`} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		var failure struct {
			Code int `json:"status_code"`
		}
		if json.Unmarshal(w.Body.Bytes(), &failure) != nil || failure.Code != 400 {
			t.Fatalf("invalid body %s = %d: %s", body, w.Code, w.Body.String())
		}
	}
	for _, body := range []string{`{"prices":[{"product_id":` + strconv.Itoa(int(product.ID)) + `,"price_amount":"39.90"}]}`, `{"prices":[]}`} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("valid body = %d: %s", w.Code, w.Body.String())
		}
	}
	prices, err := svc.GetUserPrices(a.ID)
	if err != nil || len(prices) != 0 {
		t.Fatal("empty list must clear prices")
	}
}

func TestPublicCatalogPersonalPricesDoNotLeakOrStackDiscounts(t *testing.T) {
	svc, _, a, b, product := userPriceFixture(t)
	product.WholesalePrices = productdomain.WholesalePriceTiers{{MinQuantity: 1, UnitPrice: money.FromDecimal(decimal.NewFromInt(20))}}
	rows := []memberleveldomain.UserPrice{personalPrice(a.ID, product.ID, 0, "95.55"), personalPrice(a.ID, product.ID, product.SKUs[1].ID, "240")}
	if err := svc.ReplaceUserPrices(a.ID, rows); err != nil {
		t.Fatal(err)
	}
	handler := producthttp.NewPublicHandler(&pricingCatalog{product: product}, nil, &pricingPromotion{amount: 50}, svc, nil, nil, pricingPosts{})
	router := gin.New()
	var userID uint
	router.Use(func(c *gin.Context) { c.Set("user_id", userID); c.Next() })
	router.GET("/products", handler.GetProducts)
	router.GET("/products/:slug", handler.GetProductBySlug)
	check := func(id uint, amount string, personalized bool) {
		t.Helper()
		userID = id
		for _, path := range []string{"/products", "/products/" + product.Slug + "?user_id=" + strconv.Itoa(int(a.ID))} {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
			var env struct {
				Data json.RawMessage `json:"data"`
			}
			if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &env) != nil {
				t.Fatal(w.Body.String())
			}
			var result productpresenter.Product
			if path == "/products" {
				var rows []productpresenter.Product
				if err := json.Unmarshal(env.Data, &rows); err != nil || len(rows) != 1 {
					t.Fatal(string(env.Data))
				}
				result = rows[0]
			} else if err := json.Unmarshal(env.Data, &result); err != nil {
				t.Fatal(err)
			}
			if result.EffectivePriceAmount == nil || result.EffectivePriceAmount.StringFixed(2) != amount {
				t.Fatalf("user %d price = %s", id, env.Data)
			}
			if (result.UserPriceAmount != nil) != personalized {
				t.Fatal("personal price leaked")
			}
			if personalized && (result.PriceAmount.StringFixed(2) != "95.55" || result.SKUs[1].PriceAmount.StringFixed(2) != "240.00" || result.SKUs[1].RetailPriceAmount.StringFixed(2) != "200.00" || result.SKUs[0].PromotionPriceAmount != nil || len(result.WholesalePrices) != 0) {
				t.Fatal("personal price was capped or discounted")
			}
			if w.Header().Get("Cache-Control") != "private, no-store" {
				t.Fatal("personal prices must not be cached publicly")
			}
		}
	}
	check(a.ID, "95.55", true)
	check(b.ID, "16.00", false)
	check(0, "20.00", false)
	if err := svc.ReplaceUserPrices(a.ID, nil); err != nil {
		t.Fatal(err)
	}
	check(a.ID, "16.00", false)
}
