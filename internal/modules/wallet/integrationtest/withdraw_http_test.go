package integrationtest

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	affiliatedomain "github.com/dujiao-next/internal/modules/affiliate/domain"
	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	wallethttp "github.com/dujiao-next/internal/modules/wallet/transport/http"
	"github.com/gin-gonic/gin"
)

func TestWithdrawalHTTPScopingPrecisionAndPolicy(t *testing.T) {
	svc, store, _ := earningsWallet(t)
	settle(t, store, "affiliate:1", "100")
	gin.SetMode(gin.TestMode)
	h := wallethttp.NewWithdrawalHandler(svc)
	call := func(method, path, body string, userID uint) map[string]interface{} {
		t.Helper()
		r := gin.New()
		r.Use(func(c *gin.Context) {
			if userID > 0 {
				c.Set("user_id", userID)
			}
		})
		r.POST("/apply", h.Apply)
		r.GET("/list", h.UserList)
		r.GET("/policy", h.Policy)
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		r.ServeHTTP(response, request)
		var result map[string]interface{}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	for i, amount := range []string{`"20.001"`, `20.001`, `"1e99999999"`, `"-20"`, `"NaN"`, `"0"`} {
		result := call("POST", "/apply", fmt.Sprintf(`{"UserID":2,"amount":%s,"channel":"wechat","account":"local","real_name":"Local Test","request_key":"invalid-request-%02d"}`, amount, i), 1)
		if result["status_code"] == float64(0) {
			t.Fatalf("accepted invalid amount %s", amount)
		}
	}
	body := `{"UserID":2,"user_id":2,"amount":"20.00","channel":"wechat","account":"local","real_name":"Local Test","request_key":"valid-request-001"}`
	if call("POST", "/apply", body, 0)["status_code"] == float64(0) {
		t.Fatal("anonymous withdrawal accepted")
	}
	for i := 0; i < 2; i++ {
		result := call("POST", "/apply", body, 1)
		if result["status_code"] != float64(0) {
			t.Fatalf("valid withdrawal: %v", result)
		}
		if result["data"].(map[string]interface{})["user_id"] != float64(1) {
			t.Fatal("request body changed withdrawal owner")
		}
	}
	result := call("GET", "/list?user_id=1", "", 2)
	if len(result["data"].([]interface{})) != 0 {
		t.Fatal("other user's requests leaked")
	}
	accountIs(t, svc, "80", "80", "20", "0")
}

func TestSuspendedSourceCannotWithdrawOrBePaid(t *testing.T) {
	svc, store, db := earningsWallet(t)
	settle(t, store, "reseller:1:CNY", "100")
	request, err := svc.ApplyWithdraw(applyInput("before-source-freeze-1", "20"))
	if err != nil {
		t.Fatal(err)
	}
	profile := resellerdomain.Profile{UserID: 1, Status: resellerdomain.ProfileStatusActive, SettlementStatus: resellerdomain.SettlementStatusFrozen}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatal(err)
	}
	policy, err := svc.GetUserWithdrawPolicy(1)
	if err != nil || policy.BlockedReason == "" {
		t.Fatalf("UI misses restriction: %+v %v", policy, err)
	}
	if _, err := svc.ApplyWithdraw(applyInput("after-source-freeze-01", "20")); err != walletcontract.ErrWithdrawRestricted {
		t.Fatal(err)
	}
	if _, err := svc.ReviewWithdraw(walletcontract.WithdrawReviewInput{ID: request.ID, AdminID: 1, Action: "approve"}); err != walletcontract.ErrWithdrawRestricted {
		t.Fatal(err)
	}
	if _, err := svc.ReviewWithdraw(walletcontract.WithdrawReviewInput{ID: request.ID, AdminID: 1, Action: "reject", Remark: "Local verification"}); err != nil {
		t.Fatal(err)
	}
	db.Model(&profile).Update("settlement_status", resellerdomain.SettlementStatusNormal)
	if err := db.Create(&affiliatedomain.Profile{UserID: 1, AffiliateCode: "LOCALFREEZE", Status: "disabled"}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ApplyWithdraw(applyInput("affiliate-disabled-01", "20")); err != walletcontract.ErrWithdrawRestricted {
		t.Fatal(err)
	}
	accountIs(t, svc, "100", "100", "0", "0")
}
