package contenthttp

import (
	"testing"
	"time"
)

func TestBuildBannerInputFromRequestMapsFieldsAndParsesTimes(t *testing.T) {
	openInNewTab := true
	isActive := false
	req := BannerUpsertRequest{
		Name:         "Home hero",
		Position:     "home_hero",
		TitleJSON:    map[string]interface{}{"zh-CN": "标题"},
		SubtitleJSON: map[string]interface{}{"zh-CN": "副标题"},
		Image:        "/uploads/banner.png",
		MobileImage:  "/uploads/banner-mobile.png",
		LinkType:     "internal",
		LinkValue:    "/products",
		OpenInNewTab: &openInNewTab,
		IsActive:     &isActive,
		StartAt:      "2026-06-01T10:00:00Z",
		EndAt:        "2026-06-02T10:00:00Z",
		SortOrder:    7,
	}

	input, err := buildBannerInputFromRequest(req)
	if err != nil {
		t.Fatalf("buildBannerInputFromRequest: %v", err)
	}
	if input.Name != req.Name || input.Position != req.Position || input.Image != req.Image {
		t.Fatalf("basic fields mismatch: %#v", input)
	}
	if input.TitleJSON["zh-CN"] != "标题" || input.SubtitleJSON["zh-CN"] != "副标题" {
		t.Fatalf("localized fields mismatch: %#v", input)
	}
	if input.MobileImage != req.MobileImage || input.LinkType != req.LinkType || input.LinkValue != req.LinkValue {
		t.Fatalf("media/link fields mismatch: %#v", input)
	}
	if input.OpenInNewTab == nil || *input.OpenInNewTab != openInNewTab {
		t.Fatalf("open_in_new_tab mismatch: %#v", input.OpenInNewTab)
	}
	if input.IsActive == nil || *input.IsActive != isActive {
		t.Fatalf("is_active mismatch: %#v", input.IsActive)
	}
	if input.StartAt == nil || input.StartAt.Format(time.RFC3339) != req.StartAt {
		t.Fatalf("start_at mismatch: %#v", input.StartAt)
	}
	if input.EndAt == nil || input.EndAt.Format(time.RFC3339) != req.EndAt {
		t.Fatalf("end_at mismatch: %#v", input.EndAt)
	}
	if input.SortOrder != req.SortOrder {
		t.Fatalf("sort_order mismatch: %d", input.SortOrder)
	}
}

func TestBuildBannerInputFromRequestRejectsInvalidTime(t *testing.T) {
	_, err := buildBannerInputFromRequest(BannerUpsertRequest{
		Name:    "Home hero",
		Image:   "/uploads/banner.png",
		StartAt: "not-a-time",
	})
	if err == nil {
		t.Fatalf("expected invalid time error")
	}
}
