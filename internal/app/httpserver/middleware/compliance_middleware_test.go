package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	settingsstore "github.com/dujiao-next/internal/modules/settings/infrastructure/gormstore"

	complianceapp "github.com/dujiao-next/internal/modules/compliance/application"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupComplianceMW(t *testing.T) (*gin.Engine, *complianceapp.Service) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&settingsstore.SettingRecord{}))
	cs := complianceapp.NewService(settingsstore.New(db))

	r := gin.New()
	r.GET("/proto",
		func(c *gin.Context) {
			if c.GetHeader("X-Test-Super") == "1" {
				c.Set("admin_is_super", true)
			}
			c.Next()
		},
		PaymentComplianceRequired(cs),
		func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) },
	)
	return r, cs
}

func TestPaymentComplianceRequired_PassWhenAcked(t *testing.T) {
	r, cs := setupComplianceMW(t)
	require.NoError(t, cs.Acknowledge(complianceapp.AcknowledgeCommand{
		Segment1: "我已阅读并理解上述合规声明提醒",
		Segment2: "知悉相关法律风险",
		Segment3: "并确认自行承担部署运营和收费行为产生的法律责任",
		AdminID:  1, Username: "admin",
	}))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/proto", nil))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "\"ok\":true")
}

func TestPaymentComplianceRequired_SuperNotAcked(t *testing.T) {
	r, _ := setupComplianceMW(t)
	req := httptest.NewRequest(http.MethodGet, "/proto", nil)
	req.Header.Set("X-Test-Super", "1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "\"status_code\":403")
	assert.Contains(t, body, "compliance_required")
	assert.NotContains(t, body, "compliance_required_by_super_admin")
	assert.NotContains(t, body, "\"ok\":true")
}

func TestPaymentComplianceRequired_NonSuperNotAcked(t *testing.T) {
	r, _ := setupComplianceMW(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/proto", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "\"status_code\":403")
	assert.Contains(t, body, "compliance_required_by_super_admin")
	assert.NotContains(t, body, "\"ok\":true")
}

func TestPaymentComplianceRequired_NilService(t *testing.T) {
	// 防御性：服务为 nil 时不放行（更安全的默认），但目前实现走 fallback 非超管分支
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/proto",
		PaymentComplianceRequired(nil),
		func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) },
	)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/proto", nil))
	// nil service 视为未确认；执行非超管分支
	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "compliance_required_by_super_admin")
	assert.NotContains(t, body, "\"ok\":true")
}
