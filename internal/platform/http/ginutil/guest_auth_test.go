package ginutil

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGetGuestCredentialsReadsGuestAuthorizationHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/guest/orders", nil)
	token := base64.RawURLEncoding.EncodeToString([]byte("guest@example.com\ncorrect horse battery staple"))
	c.Request.Header.Set("Authorization", "Guest "+token)

	email, password, ok := GetGuestCredentials(c)
	if !ok || email != "guest@example.com" || password != "correct horse battery staple" {
		t.Fatalf("unexpected guest credentials email=%q password=%q ok=%v", email, password, ok)
	}
}

func TestGetGuestCredentialsRejectsMalformedHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/guest/orders?email=guest@example.com&order_password=leaked", nil)

	if _, _, ok := GetGuestCredentials(c); ok {
		t.Fatalf("query credentials must not authenticate without Guest authorization header")
	}
}
