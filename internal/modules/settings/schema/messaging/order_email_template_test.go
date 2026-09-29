package settingsmessaging

import "testing"

func TestOrderEmailTemplateSettingDoesNotExposeCanceledScene(t *testing.T) {
	encoded := EncodeOrderEmailTemplateSetting(DefaultOrderEmailTemplateSetting())
	templates, ok := encoded["templates"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected encoded templates map, got %T", encoded["templates"])
	}
	if _, exists := templates["canceled"]; exists {
		t.Fatalf("canceled email scene must not be exposed")
	}

	// 旧版本保存的 canceled 字段应被安全忽略，并在下一次序列化时自然清除。
	templates["canceled"] = map[string]interface{}{
		"zh-CN": map[string]interface{}{"subject": "legacy", "body": "legacy"},
	}
	roundTrip := EncodeOrderEmailTemplateSetting(DecodeOrderEmailTemplateSetting(encoded, DefaultOrderEmailTemplateSetting()))
	roundTripTemplates, ok := roundTrip["templates"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected round-trip templates map, got %T", roundTrip["templates"])
	}
	if _, exists := roundTripTemplates["canceled"]; exists {
		t.Fatalf("legacy canceled email scene must be discarded")
	}
}
