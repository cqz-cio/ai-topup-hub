package application

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dujiao-next/internal/modules/adproxy/domain"
)

type gatewayStub struct {
	renderSlotCode string
	renderParams   map[string]string
	impression     json.RawMessage
}

func (g *gatewayStub) RenderSlot(_ context.Context, slotCode string, params map[string]string) (*domain.RenderResponse, error) {
	g.renderSlotCode = slotCode
	g.renderParams = params
	return &domain.RenderResponse{Slot: domain.RenderSlot{Code: slotCode}}, nil
}

func (g *gatewayStub) ReportImpression(_ context.Context, payload json.RawMessage) error {
	g.impression = append(json.RawMessage(nil), payload...)
	return nil
}

func TestServiceDelegatesAdUseCasesToGateway(t *testing.T) {
	gateway := &gatewayStub{}
	service := NewService(gateway)
	params := map[string]string{"locale": "zh-CN"}

	response, err := service.RenderSlot(context.Background(), "dashboard", params)
	if err != nil {
		t.Fatalf("render slot failed: %v", err)
	}
	if response == nil || response.Slot.Code != "dashboard" {
		t.Fatalf("unexpected render response: %#v", response)
	}
	if gateway.renderSlotCode != "dashboard" || gateway.renderParams["locale"] != "zh-CN" {
		t.Fatalf("render input was not delegated: code=%q params=%v", gateway.renderSlotCode, gateway.renderParams)
	}

	payload := json.RawMessage(`{"token":"impression-1"}`)
	if err := service.ReportImpression(context.Background(), payload); err != nil {
		t.Fatalf("report impression failed: %v", err)
	}
	if string(gateway.impression) != string(payload) {
		t.Fatalf("impression payload mismatch: got %s want %s", gateway.impression, payload)
	}
}
