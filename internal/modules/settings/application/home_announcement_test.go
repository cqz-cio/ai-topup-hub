package settingsapp

import (
	"testing"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

func TestHomeAnnouncementActiveDisabled(t *testing.T) {
	repo := newMockSettingRepo()
	svc := NewService(repo)
	repo.store[constants.SettingKeyHomeAnnouncement] = jsonmap.JSON{
		"enabled": false,
		"type":    "info",
		"content": map[string]interface{}{"zh-CN": "<p>hi</p>"},
	}
	if _, ok := svc.GetActiveHomeAnnouncement(); ok {
		t.Fatalf("expected disabled announcement to be inactive")
	}
}

func TestHomeAnnouncementActiveEmptyContent(t *testing.T) {
	repo := newMockSettingRepo()
	svc := NewService(repo)
	repo.store[constants.SettingKeyHomeAnnouncement] = jsonmap.JSON{
		"enabled": true,
		"type":    "info",
		"content": map[string]interface{}{"zh-CN": "   "},
	}
	if _, ok := svc.GetActiveHomeAnnouncement(); ok {
		t.Fatalf("expected empty-content announcement to be inactive")
	}
}

func TestHomeAnnouncementActiveOK(t *testing.T) {
	repo := newMockSettingRepo()
	svc := NewService(repo)
	repo.store[constants.SettingKeyHomeAnnouncement] = jsonmap.JSON{
		"enabled": true,
		"type":    "warning",
		"title":   map[string]interface{}{"zh-CN": "维护通知"},
		"content": map[string]interface{}{"zh-CN": "<p>正文</p>"},
	}
	result, ok := svc.GetActiveHomeAnnouncement()
	if !ok {
		t.Fatalf("expected announcement to be active")
	}
	if result["type"] != "warning" {
		t.Fatalf("expected type warning, got %v", result["type"])
	}
	version, _ := result["version"].(string)
	if len(version) != 8 {
		t.Fatalf("expected 8-char version, got %q", version)
	}
}
