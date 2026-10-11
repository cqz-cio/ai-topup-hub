package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dujiao-next/internal/modules/identity/jwttoken"
	userauthapp "github.com/dujiao-next/internal/modules/identity/userauth/application"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func TestOptionalUserJWTKeepsGuestsPublicAndAuthenticatesSuppliedTokens(t *testing.T) {
	repo, user := setupUserMiddlewareTestRepo(t)
	token := signToken(t, userauthapp.UserJWTClaims{
		UserID: user.ID, Email: user.Email, TokenVersion: user.TokenVersion, Typ: jwttoken.TypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)), IssuedAt: jwt.NewNumericDate(time.Now())},
	})
	r := gin.New()
	r.Use(OptionalUserJWTAuthMiddleware(middlewareTestSecret, repo))
	r.GET("/catalog", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"user_id": c.GetUint("user_id")}) })
	for _, tc := range []struct {
		header  string
		userID  uint
		allowed bool
	}{
		{"", 0, true}, {"Bearer " + token, user.ID, true}, {"Bearer invalid", 0, false}, {"Basic invalid", 0, false},
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/catalog", nil)
		req.Header.Set("Authorization", tc.header)
		r.ServeHTTP(w, req)
		var result map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if tc.allowed {
			if id, ok := result["user_id"].(float64); !ok || uint(id) != tc.userID {
				t.Fatalf("unexpected identity: %+v", result)
			}
		} else if result["status_code"] != float64(401) {
			t.Fatalf("invalid token granted catalog identity: %+v", result)
		}
	}
}
