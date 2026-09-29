package categorypresenter

import (
	"encoding/json"
	"strings"
	"testing"

	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"

	"github.com/dujiao-next/internal/shared/jsonmap"
)

func TestCategoryRespOmitsSensitiveFields(t *testing.T) {
	cat := &categorydomain.Category{
		ID:        1,
		ParentID:  0,
		Slug:      "games",
		NameJSON:  jsonmap.JSON{"zh-CN": "游戏"},
		Icon:      "/icons/game.png",
		SortOrder: 10,
	}

	resp := New(cat)
	data, _ := json.Marshal(resp)
	jsonStr := string(data)

	if strings.Contains(jsonStr, `"created_at"`) {
		t.Error("created_at should not appear")
	}
	if !strings.Contains(jsonStr, `"slug"`) {
		t.Error("slug should appear")
	}
	if resp.Name["zh-CN"] != "游戏" {
		t.Error("name should be preserved")
	}
}

func TestCategoryRespListEmpty(t *testing.T) {
	result := List(nil)
	if len(result) != 0 {
		t.Errorf("expected empty list, got %d", len(result))
	}
}
