package resellerhttp_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	auditlogapp "github.com/dujiao-next/internal/modules/auditlog/application"
	resellermodule "github.com/dujiao-next/internal/modules/reseller/application"
	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	adminhttp "github.com/dujiao-next/internal/modules/reseller/transport/http/admin"
	userhttp "github.com/dujiao-next/internal/modules/reseller/transport/http/user"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"

	"github.com/gin-gonic/gin"
)

type productSettingStub struct {
	detail *resellermodule.ProductSettingDetail
	rows   []resellerdomain.ProductSetting
	total  int64
	err    error
	saved  bool
}

func (s *productSettingStub) ListUserProductSettings(userID uint, input resellermodule.ProductSettingUserListInput) ([]resellermodule.ProductSettingListRow, int64, error) {
	return nil, 0, s.err
}

func (s *productSettingStub) GetUserProductSetting(userID, productID uint) (*resellermodule.ProductSettingDetail, error) {
	return s.detail, s.err
}

func (s *productSettingStub) PreviewUserProductSettings(userID, productID uint, input resellermodule.ProductSettingSaveInput) ([]resellermodule.ProductSettingPreviewItem, error) {
	return nil, s.err
}

func (s *productSettingStub) SaveUserProductSettings(userID, productID uint, input resellermodule.ProductSettingSaveInput) (*resellermodule.ProductSettingDetail, error) {
	s.saved = true
	return s.detail, s.err
}

func (s *productSettingStub) ResetUserProductSetting(userID, productID, skuID uint) error {
	return s.err
}

func (s *productSettingStub) ListAdminSettings(input resellermodule.ProductSettingAdminListInput) ([]resellerdomain.ProductSetting, int64, error) {
	return s.rows, s.total, s.err
}

func (s *productSettingStub) GetAdminProductSetting(resellerID, productID uint) (*resellermodule.ProductSettingDetail, error) {
	return s.detail, s.err
}

func (s *productSettingStub) PreviewAdminProductSettings(resellerID, productID uint, input resellermodule.ProductSettingSaveInput) ([]resellermodule.ProductSettingPreviewItem, error) {
	return nil, s.err
}

func (s *productSettingStub) SaveAdminProductSettings(resellerID, productID uint, input resellermodule.ProductSettingSaveInput) (*resellermodule.ProductSettingDetail, error) {
	s.saved = true
	return s.detail, s.err
}

func (s *productSettingStub) ResetAdminProductSetting(resellerID, productID, skuID uint) error {
	return s.err
}

type auditStub struct {
	actions []string
}

func (a *auditStub) Record(input auditlogapp.AuthzRecord) error {
	a.actions = append(a.actions, input.Action)
	return nil
}

func newProductSettingHandlerContext(method, path string, body []byte, userID uint) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	if userID > 0 {
		c.Set("user_id", userID)
	}
	c.Set("admin_id", uint(1))
	c.Set("username", "admin")
	return c, recorder
}

func TestUserProductSettingHandlerMapsPricingError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &productSettingStub{err: resellercontract.ErrPriceBelowBase}
	h := userhttp.NewUserProductSettingHandler(stub)
	c, recorder := newProductSettingHandlerContext(http.MethodPut, "/api/v1/reseller/product-settings/1", []byte(`{"settings":[]}`), 9)
	c.Params = gin.Params{{Key: "product_id", Value: "1"}}

	h.UpdateProductSettings(c)

	var resp struct {
		StatusCode int `json:"status_code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if resp.StatusCode != response.CodeBadRequest {
		t.Fatalf("expected bad request, got body=%s", recorder.Body.String())
	}
}

func TestUserProductSettingHandlerMapsInactiveProfile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &productSettingStub{err: resellercontract.ErrProfileInactive}
	h := userhttp.NewUserProductSettingHandler(stub)
	c, recorder := newProductSettingHandlerContext(http.MethodGet, "/api/v1/reseller/product-settings/1", nil, 9)
	c.Params = gin.Params{{Key: "product_id", Value: "1"}}

	h.GetProductSetting(c)

	if !strings.Contains(recorder.Body.String(), fmt.Sprintf(`"status_code":%d`, response.CodeBadRequest)) {
		t.Fatalf("expected inactive profile mapped to bad request, body=%s", recorder.Body.String())
	}
}

func TestAdminProductSettingHandlerRecordsAuditOnSave(t *testing.T) {
	gin.SetMode(gin.TestMode)
	detail := &resellermodule.ProductSettingDetail{
		Product:  productdomain.Product{ID: 3},
		Settings: []resellerdomain.ProductSetting{{SKUID: 7, PricingMode: resellerdomain.PricingModeFixedPrice, FixedPriceAmount: money.FromDecimal(decimal.RequireFromString("130.00"))}},
	}
	stub := &productSettingStub{detail: detail}
	audit := &auditStub{}
	h := adminhttp.NewAdminProductSettingHandler(stub, audit)
	body := `{"settings":[{"sku_id":7,"is_listed":true,"pricing_mode":"fixed_price","fixed_price_amount":"130.00"}]}`
	c, recorder := newProductSettingHandlerContext(http.MethodPut, "/admin/resellers/product-settings/2/3", []byte(body), 0)
	c.Params = gin.Params{{Key: "reseller_id", Value: "2"}, {Key: "product_id", Value: "3"}}

	h.UpdateProductSettings(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !stub.saved {
		t.Fatal("expected save to be called")
	}
	if len(audit.actions) != 1 || audit.actions[0] != "reseller_product_setting_save" {
		t.Fatalf("unexpected audit actions: %v", audit.actions)
	}
}

func TestAdminProductSettingHandlerMapsNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &productSettingStub{err: productcontract.ErrNotFound}
	h := adminhttp.NewAdminProductSettingHandler(stub, &auditStub{})
	c, recorder := newProductSettingHandlerContext(http.MethodGet, "/admin/resellers/product-settings/2/3", nil, 0)
	c.Params = gin.Params{{Key: "reseller_id", Value: "2"}, {Key: "product_id", Value: "3"}}

	h.GetProductSetting(c)

	if !strings.Contains(recorder.Body.String(), fmt.Sprintf(`"status_code":%d`, response.CodeNotFound)) {
		t.Fatalf("expected not found, body=%s", recorder.Body.String())
	}
}
