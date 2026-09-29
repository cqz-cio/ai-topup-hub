package compliance_test

import (
	"testing"

	settingsstore "github.com/dujiao-next/internal/modules/settings/infrastructure/gormstore"

	complianceapp "github.com/dujiao-next/internal/modules/compliance/application"
	compliancecontract "github.com/dujiao-next/internal/modules/compliance/contract"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestComplianceService(t *testing.T) (*complianceapp.Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&settingsstore.SettingRecord{}))
	repo := settingsstore.New(db)
	return complianceapp.NewService(repo), db
}

func TestComplianceService_InitialNotAcknowledged(t *testing.T) {
	svc, _ := newTestComplianceService(t)
	assert.False(t, svc.IsAcknowledged())

	status, err := svc.Status()
	require.NoError(t, err)
	assert.False(t, status.Acknowledged)
	assert.Empty(t, status.AcknowledgedByUsername)
}

func TestComplianceService_AcknowledgeSuccess(t *testing.T) {
	svc, _ := newTestComplianceService(t)

	err := svc.Acknowledge(complianceapp.AcknowledgeCommand{
		Segment1:  "我已阅读并理解上述合规声明提醒",
		Segment2:  "知悉相关法律风险",
		Segment3:  "并确认自行承担部署运营和收费行为产生的法律责任",
		AdminID:   1,
		Username:  "admin",
		ClientIP:  "127.0.0.1",
		UserAgent: "go-test",
	})
	require.NoError(t, err)

	assert.True(t, svc.IsAcknowledged())

	status, err := svc.Status()
	require.NoError(t, err)
	assert.True(t, status.Acknowledged)
	assert.Equal(t, "admin", status.AcknowledgedByUsername)
	assert.Equal(t, uint(1), status.AcknowledgedByAdminID)
	assert.Equal(t, "v1", status.Version)
}

func TestComplianceService_AcknowledgeWrongSegment(t *testing.T) {
	svc, _ := newTestComplianceService(t)

	cases := []struct {
		name string
		req  complianceapp.AcknowledgeCommand
	}{
		{
			"段 1 缺字",
			complianceapp.AcknowledgeCommand{
				Segment1: "我已阅读并理解上述合规声明",
				Segment2: "知悉相关法律风险",
				Segment3: "并确认自行承担部署运营和收费行为产生的法律责任",
			},
		},
		{
			"段 2 多字",
			complianceapp.AcknowledgeCommand{
				Segment1: "我已阅读并理解上述合规声明提醒",
				Segment2: "知悉相关法律风险啊",
				Segment3: "并确认自行承担部署运营和收费行为产生的法律责任",
			},
		},
		{
			"段 3 错字",
			complianceapp.AcknowledgeCommand{
				Segment1: "我已阅读并理解上述合规声明提醒",
				Segment2: "知悉相关法律风险",
				Segment3: "并确认自行承担部署运营和收费行為产生的法律责任",
			},
		},
		{
			"段 3 含标点（应拒）",
			complianceapp.AcknowledgeCommand{
				Segment1: "我已阅读并理解上述合规声明提醒",
				Segment2: "知悉相关法律风险",
				Segment3: "并确认自行承担部署、运营和收费行为产生的法律责任",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.req.AdminID = 1
			tc.req.Username = "admin"
			err := svc.Acknowledge(tc.req)
			assert.ErrorIs(t, err, compliancecontract.ErrTextMismatch)
			assert.False(t, svc.IsAcknowledged())
		})
	}
}

func TestComplianceService_AcknowledgeIdempotent(t *testing.T) {
	svc, _ := newTestComplianceService(t)
	req := complianceapp.AcknowledgeCommand{
		Segment1: "我已阅读并理解上述合规声明提醒",
		Segment2: "知悉相关法律风险",
		Segment3: "并确认自行承担部署运营和收费行为产生的法律责任",
		AdminID:  1, Username: "admin",
	}
	require.NoError(t, svc.Acknowledge(req))
	err := svc.Acknowledge(req)
	assert.ErrorIs(t, err, compliancecontract.ErrAlreadyAcknowledged)
	assert.True(t, svc.IsAcknowledged())
}

func TestComplianceService_LoadFromExistingSetting(t *testing.T) {
	svc, db := newTestComplianceService(t)
	require.False(t, svc.IsAcknowledged())

	// 通过仓库写入已确认状态，模拟既有部署
	repo := settingsstore.New(db)
	_, err := repo.Upsert("compliance.acknowledgement.v1", jsonmap.JSON{
		"acknowledged":             true,
		"acknowledged_at":          "2026-01-01T00:00:00Z",
		"acknowledged_by_admin_id": float64(2),
		"acknowledged_by_username": "root",
		"version":                  "v1",
	})
	require.NoError(t, err)

	// 模拟重启：重新建 service
	svc2 := complianceapp.NewService(repo)
	assert.True(t, svc2.IsAcknowledged())
	status, _ := svc2.Status()
	assert.Equal(t, "root", status.AcknowledgedByUsername)
}
