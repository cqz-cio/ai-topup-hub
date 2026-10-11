package integrationtest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	producthttp "github.com/dujiao-next/internal/modules/catalog/product/transport/http"
	productpresenter "github.com/dujiao-next/internal/modules/catalog/product/transport/presenter"
	contentcontract "github.com/dujiao-next/internal/modules/content/contract"
	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"
	promotiondomain "github.com/dujiao-next/internal/modules/promotion/domain"
	reseller "github.com/dujiao-next/internal/modules/reseller/contract"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type pricingCatalog struct{ product productdomain.Product }

func (p *pricingCatalog) ListPublicForTenant(reseller.TenantContext, string, string, int, int) ([]productdomain.Product, int64, error) {
	return []productdomain.Product{p.product}, 1, nil
}
func (p *pricingCatalog) GetPublicBySlugForTenant(reseller.TenantContext, string) (*productdomain.Product, error) {
	copy := p.product
	return &copy, nil
}
func (*pricingCatalog) ApplyAutoStockCounts([]productdomain.Product) error { return nil }

type pricingPosts struct{}

func (pricingPosts) ListPostsForProduct(context.Context, uint, int) ([]contentcontract.RelatedPost, error) {
	return nil, nil
}

type pricingPromotion struct{ amount int64 }

func (pricingPromotion) GetProductPromotions(uint) ([]promotiondomain.Promotion, error) {
	return nil, nil
}
func (p *pricingPromotion) ApplyPromotion(*productdomain.Product, int) (*promotiondomain.Promotion, money.Amount, error) {
	if p.amount == 0 {
		return nil, money.Amount{}, nil
	}
	return &promotiondomain.Promotion{ID: 7}, money.FromDecimal(decimal.NewFromInt(p.amount)), nil
}

func TestCatalogPricesFollowCurrentUserGradeAndPreserveOtherVaryHeaders(t *testing.T) {
	svc, db := newMemberLevelServiceForTest(t)
	retail := createMemberLevelFixture(t, db, "catalog-retail", 0, "0", true)
	agent := createMemberLevelFixture(t, db, "catalog-agent", 10, "0", false)
	if err := db.Model(&agent).Update("discount_rate", 80).Error; err != nil {
		t.Fatal(err)
	}
	user := createUserFixture(t, db, "catalog-pricing@example.test", retail.ID)
	prices := []memberleveldomain.MemberLevelPrice{
		{MemberLevelID: agent.ID, ProductID: 42, SKUID: 0, PriceAmount: money.FromDecimal(decimal.NewFromInt(75))},
		{MemberLevelID: agent.ID, ProductID: 42, SKUID: 11, PriceAmount: money.FromDecimal(decimal.NewFromInt(70))},
		{MemberLevelID: agent.ID, ProductID: 42, SKUID: 12, PriceAmount: money.FromDecimal(decimal.NewFromInt(130))},
		{MemberLevelID: retail.ID, ProductID: 900, PriceAmount: money.FromDecimal(decimal.NewFromInt(1))},
	}
	if err := db.Create(&prices).Error; err != nil {
		t.Fatal(err)
	}
	catalog := &pricingCatalog{product: productdomain.Product{
		ID: 42, Slug: "member-pricing", PriceAmount: money.FromDecimal(decimal.NewFromInt(100)),
		FulfillmentType: "manual",
		SKUs: []productdomain.ProductSKU{
			{ID: 11, ProductID: 42, IsActive: true, SortOrder: 100, ManualStockTotal: -1, PriceAmount: money.FromDecimal(decimal.NewFromInt(100))},
			{ID: 12, ProductID: 42, IsActive: true, SortOrder: 50, ManualStockTotal: -1, PriceAmount: money.FromDecimal(decimal.NewFromInt(200))},
		},
	}}
	promotion := &pricingPromotion{}
	handler := producthttp.NewPublicHandler(catalog, nil, promotion, svc, nil, nil, pricingPosts{})
	router := gin.New()
	var authenticatedUser uint
	router.Use(func(c *gin.Context) {
		c.Set("user_id", authenticatedUser)
		c.Writer.Header().Add("Vary", "Origin")
		c.Next()
	})
	router.GET("/products", handler.GetProducts)
	router.GET("/products/:slug", handler.GetProductBySlug)

	check := func(wantAmount int64, wantLevel uint, wantPrices int) {
		t.Helper()
		for _, path := range []string{"/products", "/products/member-pricing?member_level_id=" + strconv.Itoa(int(agent.ID))} {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
			var envelope struct {
				Code int             `json:"status_code"`
				Data json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil || envelope.Code != 0 {
				t.Fatalf("%s failed: %s", path, recorder.Body.String())
			}
			var product productpresenter.Product
			if path == "/products" {
				var products []productpresenter.Product
				if err := json.Unmarshal(envelope.Data, &products); err != nil || len(products) != 1 {
					t.Fatalf("invalid product list: %s", envelope.Data)
				}
				product = products[0]
			} else if err := json.Unmarshal(envelope.Data, &product); err != nil {
				t.Fatal(err)
			}
			if product.EffectivePriceAmount == nil || !product.EffectivePriceAmount.Decimal.Equal(decimal.NewFromInt(wantAmount)) || product.PricingMemberLevelID != wantLevel || len(product.MemberPrices) != wantPrices {
				t.Fatalf("unexpected price for %s: %+v", path, product)
			}
			if product.SKUs[0].EffectivePriceAmount == nil || !product.SKUs[0].EffectivePriceAmount.Decimal.Equal(decimal.NewFromInt(wantAmount)) {
				t.Fatalf("SKU price differs from list display: %+v", product.SKUs)
			}
			for _, row := range product.MemberPrices {
				if row.MemberLevelID != wantLevel {
					t.Fatal("another grade's prices leaked")
				}
			}
			vary := strings.Join(recorder.Header().Values("Vary"), ",")
			if !strings.Contains(vary, "Origin") || !strings.Contains(vary, "Authorization") || recorder.Header().Get("Cache-Control") != "private, no-store" {
				t.Fatalf("unsafe personalized caching headers: %+v", recorder.Header())
			}
		}
	}
	check(100, 0, 0)
	authenticatedUser = user.ID
	check(100, retail.ID, 0)
	if err := svc.SetUserLevel(user.ID, agent.ID); err != nil {
		t.Fatal(err)
	}
	check(70, agent.ID, 3)
	promotion.amount = 60
	check(60, agent.ID, 3)
	catalog.product.WholesalePrices = productdomain.WholesalePriceTiers{{MinQuantity: 1, UnitPrice: money.FromDecimal(decimal.NewFromInt(55))}}
	check(55, agent.ID, 3)
	catalog.product.WholesalePrices = nil
	promotion.amount = 0
	if err := db.Model(&agent).Update("is_active", false).Error; err != nil {
		t.Fatal(err)
	}
	check(100, 0, 0)
	if err := db.Model(&agent).Update("is_active", true).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("member_level_id = ?", agent.ID).Delete(&memberleveldomain.MemberLevelPrice{}).Error; err != nil {
		t.Fatal(err)
	}
	check(80, agent.ID, 0)
	if err := svc.SetUserLevel(user.ID, 0); err != nil {
		t.Fatal(err)
	}
	check(100, 0, 0)
}
