package application

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dujiao-next/internal/modules/compliance/contract"
	"github.com/dujiao-next/internal/modules/compliance/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

const (
	complianceSettingKey = "compliance.acknowledgement.v1"
	complianceVersion    = "v1"

	expectedSegment1 = "我已阅读并理解上述合规声明提醒"
	expectedSegment2 = "知悉相关法律风险"
	expectedSegment3 = "并确认自行承担部署运营和收费行为产生的法律责任"

	expectedFullText = "我已阅读并理解上述合规声明提醒，知悉相关法律风险，并确认自行承担部署、运营和收费行为产生的法律责任"
)

// Service 合规声明确认服务。
type Service struct {
	settingRepo contract.SettingsStore
	acked       atomic.Bool
	writeMu     sync.Mutex // 保护 Acknowledge 的 check-then-act
}

// NewService 创建服务并从持久化状态恢复确认标记。
func NewService(repo contract.SettingsStore) *Service {
	s := &Service{settingRepo: repo}
	if status, err := s.Status(); err == nil && status.Acknowledged {
		s.acked.Store(true)
	}
	return s
}

// IsAcknowledged 中间件级快速判定，不查库
func (s *Service) IsAcknowledged() bool {
	return s.acked.Load()
}

// Status 读取当前状态（管理面 UI 展示用）
func (s *Service) Status() (*domain.Status, error) {
	value, found, err := s.settingRepo.GetByKey(complianceSettingKey)
	if err != nil {
		return nil, fmt.Errorf("compliance: read setting: %w", err)
	}
	if !found {
		return &domain.Status{Acknowledged: false}, nil
	}
	v := value
	status := &domain.Status{
		Acknowledged: complianceJSONBool(v, "acknowledged"),
		Version:      complianceJSONString(v, "version"),
	}
	status.AcknowledgedAt = complianceJSONString(v, "acknowledged_at")
	if id, ok := v["acknowledged_by_admin_id"].(float64); ok {
		status.AcknowledgedByAdminID = uint(id)
	}
	status.AcknowledgedByUsername = complianceJSONString(v, "acknowledged_by_username")
	return status, nil
}

// Acknowledge 写入确认；幂等保护：已确认返回 ErrAlreadyAcknowledged
func (s *Service) Acknowledge(req AcknowledgeCommand) error {
	if req.Segment1 != expectedSegment1 ||
		req.Segment2 != expectedSegment2 ||
		req.Segment3 != expectedSegment3 {
		return contract.ErrTextMismatch
	}
	// 拼接二次校验：从调用方实际传入的 segments 重建完整文本（防御深度）
	// segment3 不含 、，故拆出 "部署"/"运营" 边界后插入
	const seg3SplitPrefix = "并确认自行承担部署"
	if !strings.HasPrefix(req.Segment3, seg3SplitPrefix) {
		return contract.ErrTextMismatch
	}
	full := req.Segment1 + "，" + req.Segment2 + "，" + seg3SplitPrefix + "、" + req.Segment3[len(seg3SplitPrefix):]
	if full != expectedFullText {
		return contract.ErrTextMismatch
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if s.acked.Load() {
		return contract.ErrAlreadyAcknowledged
	}

	value := jsonmap.JSON{
		"acknowledged":             true,
		"acknowledged_at":          time.Now().UTC().Format(time.RFC3339),
		"acknowledged_by_admin_id": req.AdminID,
		"acknowledged_by_username": req.Username,
		"acknowledged_text":        expectedFullText,
		"version":                  complianceVersion,
		"client_ip":                req.ClientIP,
		"user_agent":               req.UserAgent,
	}
	if _, err := s.settingRepo.Upsert(complianceSettingKey, value); err != nil {
		return fmt.Errorf("compliance: upsert setting: %w", err)
	}
	s.acked.Store(true)
	return nil
}

func complianceJSONBool(j jsonmap.JSON, key string) bool {
	v, ok := j[key].(bool)
	return ok && v
}

func complianceJSONString(j jsonmap.JSON, key string) string {
	v, _ := j[key].(string)
	return v
}
