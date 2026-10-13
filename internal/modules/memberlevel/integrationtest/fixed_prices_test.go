package integrationtest

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"
	memberlevelhttp "github.com/dujiao-next/internal/modules/memberlevel/transport/http"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

func TestLevelPriceHTTPScopesClearsAndRejectsInvalidReplacements(t *testing.T) {
	svc, db, user, _, product := userPriceFixture(t)
	other := createMemberLevelFixture(t, db, "another-fixed-grade", 20, "0", false)
	router := gin.New()
	memberlevelhttp.RegisterAdminRoutes(router.Group("/admin"), memberlevelhttp.NewAdminHandler(svc))
	request := func(method, path, body string, wantCode int) json.RawMessage {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		var envelope struct {
			Code int             `json:"status_code"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil || envelope.Code != wantCode {
			t.Fatalf("%s %s: %s, want code %d", method, path, w.Body.String(), wantCode)
		}
		return envelope.Data
	}
	row := func(level, sku uint, amount string) string {
		return fmt.Sprintf(`{"member_level_id":%d,"product_id":%d,"sku_id":%d,"price_amount":%s}`, level, product.ID, sku, amount)
	}
	save := func(level uint, rows string, want int) {
		request("POST", "/admin/member-level-prices/batch", fmt.Sprintf(`{"member_level_id":%d,"prices":[%s]}`, level, rows), want)
	}
	save(other.ID, row(other.ID, 0, "60"), 0)
	save(user.MemberLevelID, row(user.MemberLevelID, 0, "75")+","+row(user.MemberLevelID, product.SKUs[0].ID, "70.01"), 0)
	for _, query := range []string{"", "?member_level_id=bad", fmt.Sprintf("?member_level_id=%d&product_id=%d", user.MemberLevelID, product.ID)} {
		request("GET", "/admin/member-level-prices"+query, "", 400)
	}
	request("GET", "/admin/member-level-prices?member_level_id=999999", "", 404)
	data := request("GET", fmt.Sprintf("/admin/member-level-prices?member_level_id=%d", user.MemberLevelID), "", 0)
	var prices []memberleveldomain.MemberLevelPrice
	if err := json.Unmarshal(data, &prices); err != nil || len(prices) != 2 {
		t.Fatalf("scoped prices = %s", data)
	}
	for _, price := range prices {
		if price.MemberLevelID != user.MemberLevelID {
			t.Fatal("another level leaked")
		}
	}
	for _, invalid := range []string{row(other.ID, 0, "10"), row(user.MemberLevelID, 0, "0"), row(user.MemberLevelID, 0, "1.001"), row(user.MemberLevelID, 999999, "10"), row(user.MemberLevelID, 0, "10") + "," + row(user.MemberLevelID, 0, "20")} {
		save(user.MemberLevelID, invalid, 400)
		amount, _ := svc.ResolveMemberPrice(user.MemberLevelID, product.ID, product.SKUs[0].ID, decimal.NewFromInt(100))
		if amount.StringFixed(2) != "70.01" {
			t.Fatal("invalid replacement changed the saved price")
		}
	}
	request("POST", "/admin/member-level-prices/batch", fmt.Sprintf(`{"member_level_id":%d,"product_id":%d,"prices":[]}`, user.MemberLevelID, product.ID), 400)
	save(user.MemberLevelID, "", 0)
	empty := request("GET", fmt.Sprintf("/admin/member-level-prices?member_level_id=%d", user.MemberLevelID), "", 0)
	if string(empty) != "[]" {
		t.Fatalf("empty editor must receive a list: %s", empty)
	}
	amount, discount := svc.ResolveMemberPrice(user.MemberLevelID, product.ID, product.SKUs[0].ID, decimal.NewFromInt(100))
	if amount.StringFixed(2) != "100.00" || !discount.IsZero() {
		t.Fatal("clearing fixed prices restored a historical percentage")
	}
	amount, _ = svc.ResolveMemberPrice(other.ID, product.ID, 0, decimal.NewFromInt(100))
	if amount.StringFixed(2) != "60.00" {
		t.Fatal("clearing one level changed another")
	}
}

func TestLevelWritesNeutralizeHistoricalRateAndRetainUpgradeThresholds(t *testing.T) {
	svc, db, user, _, _ := userPriceFixture(t)
	level, err := svc.GetByID(user.MemberLevelID)
	if err != nil {
		t.Fatal(err)
	}
	level.RechargeThreshold = money.FromDecimal(decimal.NewFromInt(500))
	level.SpendThreshold = money.FromDecimal(decimal.NewFromInt(300))
	if err := svc.UpdateLevel(level); err != nil {
		t.Fatal(err)
	}
	var stored memberleveldomain.MemberLevel
	if err := db.First(&stored, level.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.DiscountRate.StringFixed(2) != "100.00" || stored.RechargeThreshold.StringFixed(2) != "500.00" || stored.SpendThreshold.StringFixed(2) != "300.00" {
		t.Fatalf("unexpected level: %+v", stored)
	}
	stored.ID = 0
	stored.Slug = "new-fixed-grade"
	stored.SortOrder = 30
	stored.DiscountRate = money.FromDecimal(decimal.NewFromInt(50))
	if err := svc.CreateLevel(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.DiscountRate.StringFixed(2) != "100.00" {
		t.Fatal("new levels must not reintroduce percentage discounts")
	}
}
