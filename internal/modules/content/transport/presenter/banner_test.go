package presenter

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	contentdomain "github.com/dujiao-next/internal/modules/content/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

func TestBannerRespOmitsSensitiveFields(t *testing.T) {
	now := time.Now()
	banner := &contentdomain.Banner{
		ID:           1,
		Name:         "Admin-Only-Name",
		Position:     "home_hero",
		TitleJSON:    jsonmap.JSON{"zh-CN": "标题"},
		SubtitleJSON: jsonmap.JSON{"zh-CN": "副标题"},
		Image:        "/img/banner.png",
		LinkType:     "url",
		LinkValue:    "https://example.com",
		OpenInNewTab: true,
		IsActive:     true,
		StartAt:      &now,
		EndAt:        &now,
		SortOrder:    5,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	resp := NewBannerResp(banner)
	data, _ := json.Marshal(resp)
	jsonStr := string(data)

	sensitiveFields := []string{"name", "is_active", "start_at", "end_at", "sort_order", "created_at", "updated_at"}
	for _, field := range sensitiveFields {
		if strings.Contains(jsonStr, `"`+field+`"`) {
			t.Errorf("sensitive field %q should not appear", field)
		}
	}

	if !strings.Contains(jsonStr, `"position"`) {
		t.Error("position should appear")
	}
	if !strings.Contains(jsonStr, `"image"`) {
		t.Error("image should appear")
	}
	if !strings.Contains(jsonStr, `"link_type"`) {
		t.Error("link_type should appear")
	}
}
