package application

import (
	"context"
	"encoding/json"

	"github.com/dujiao-next/internal/modules/adproxy/contract"
	"github.com/dujiao-next/internal/modules/adproxy/domain"
)

// Service 编排广告渲染与曝光上报用例。
type Service struct {
	gateway contract.Gateway
}

// NewService 创建广告代理应用服务。
func NewService(gateway contract.Gateway) *Service {
	if gateway == nil {
		panic("adproxy service: gateway is nil")
	}
	return &Service{gateway: gateway}
}

// RenderSlot 获取指定广告位的渲染结果。
func (s *Service) RenderSlot(ctx context.Context, slotCode string, params map[string]string) (*domain.RenderResponse, error) {
	return s.gateway.RenderSlot(ctx, slotCode, params)
}

// ReportImpression 上报广告曝光。
func (s *Service) ReportImpression(ctx context.Context, payload json.RawMessage) error {
	return s.gateway.ReportImpression(ctx, payload)
}
