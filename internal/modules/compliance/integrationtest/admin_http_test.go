package compliance_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	settingsstore "github.com/dujiao-next/internal/modules/settings/infrastructure/gormstore"

	complianceapp "github.com/dujiao-next/internal/modules/compliance/application"
	compliancetransport "github.com/dujiao-next/internal/modules/compliance/transport/http"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupComplianceHandler(t *testing.T) (*gin.Engine, *complianceapp.Service) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&settingsstore.SettingRecord{}))

	cs := complianceapp.NewService(settingsstore.New(db))
	h := compliancetransport.NewAdminHandler(cs)

	r := gin.New()
	r.GET("/compliance/status", h.GetComplianceStatus)
	r.POST("/compliance/acknowledge",
		func(c *gin.Context) {
			if v := c.GetHeader("X-Test-Super"); v == "1" {
				c.Set("admin_is_super", true)
			}
			c.Set("admin_id", uint(1))
			c.Set("username", "admin")
			c.Next()
		},
		h.AcknowledgeCompliance,
	)
	return r, cs
}

func TestAcknowledgeCompliance_NotSuper(t *testing.T) {
	r, cs := setupComplianceHandler(t)

	body, _ := json.Marshal(map[string]string{
		"segment1": "我已阅读并理解上述合规声明提醒",
		"segment2": "知悉相关法律风险",
		"segment3": "并确认自行承担部署运营和收费行为产生的法律责任",
	})
	req := httptest.NewRequest(http.MethodPost, "/compliance/acknowledge", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "\"status_code\":403")
	assert.False(t, cs.IsAcknowledged())
}

func TestAcknowledgeCompliance_SuperSuccess(t *testing.T) {
	r, cs := setupComplianceHandler(t)

	body, _ := json.Marshal(map[string]string{
		"segment1": "我已阅读并理解上述合规声明提醒",
		"segment2": "知悉相关法律风险",
		"segment3": "并确认自行承担部署运营和收费行为产生的法律责任",
	})
	req := httptest.NewRequest(http.MethodPost, "/compliance/acknowledge", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Super", "1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.True(t, cs.IsAcknowledged())
	assert.Contains(t, w.Body.String(), "\"already_acknowledged\":false")
}

func TestAcknowledgeCompliance_BadText(t *testing.T) {
	r, cs := setupComplianceHandler(t)

	body, _ := json.Marshal(map[string]string{
		"segment1": "我已阅读",
		"segment2": "知悉相关法律风险",
		"segment3": "并确认自行承担部署运营和收费行为产生的法律责任",
	})
	req := httptest.NewRequest(http.MethodPost, "/compliance/acknowledge", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Super", "1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "\"status_code\":400")
	assert.False(t, cs.IsAcknowledged())
}

func TestAcknowledgeCompliance_AlreadyAcked(t *testing.T) {
	r, cs := setupComplianceHandler(t)
	require.NoError(t, cs.Acknowledge(complianceapp.AcknowledgeCommand{
		Segment1: "我已阅读并理解上述合规声明提醒",
		Segment2: "知悉相关法律风险",
		Segment3: "并确认自行承担部署运营和收费行为产生的法律责任",
		AdminID:  1, Username: "admin",
	}))

	body, _ := json.Marshal(map[string]string{
		"segment1": "我已阅读并理解上述合规声明提醒",
		"segment2": "知悉相关法律风险",
		"segment3": "并确认自行承担部署运营和收费行为产生的法律责任",
	})
	req := httptest.NewRequest(http.MethodPost, "/compliance/acknowledge", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Super", "1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "\"already_acknowledged\":true")
}

func TestGetComplianceStatus(t *testing.T) {
	r, cs := setupComplianceHandler(t)

	// 未确认
	req := httptest.NewRequest(http.MethodGet, "/compliance/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "\"acknowledged\":false")

	// 确认
	require.NoError(t, cs.Acknowledge(complianceapp.AcknowledgeCommand{
		Segment1: "我已阅读并理解上述合规声明提醒",
		Segment2: "知悉相关法律风险",
		Segment3: "并确认自行承担部署运营和收费行为产生的法律责任",
		AdminID:  1, Username: "admin",
	}))

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/compliance/status", nil))
	require.Equal(t, http.StatusOK, w2.Code)
	body := w2.Body.String()
	assert.True(t, strings.Contains(body, "\"acknowledged\":true"))
	assert.True(t, strings.Contains(body, "\"acknowledged_by_username\":\"admin\""))
}
