package gormutil

import (
	"strings"
	"testing"
)

func TestJSONTextExprByDialectSQLite(t *testing.T) {
	got := JSONTextExprByDialect("sqlite", "title_json", "zh-CN")
	want := "json_extract(title_json, '$.\"zh-CN\"')"
	if got != want {
		t.Fatalf("sqlite json expr mismatch, want %s got %s", want, got)
	}
}

func TestJSONTextExprByDialectPostgres(t *testing.T) {
	got := JSONTextExprByDialect("postgres", "title_json", "zh-CN")
	want := "(title_json::jsonb ->> 'zh-CN')"
	if got != want {
		t.Fatalf("postgres json expr mismatch, want %s got %s", want, got)
	}
}

func TestJSONArrayLengthExprByDialect(t *testing.T) {
	if got, want := JSONArrayLengthExprByDialect("sqlite", "wholesale_prices"), "json_array_length(COALESCE(wholesale_prices, '[]'))"; got != want {
		t.Fatalf("sqlite json array length expr mismatch, want %s got %s", want, got)
	}
	if got, want := JSONArrayLengthExprByDialect("postgres", "wholesale_prices"), "jsonb_array_length(COALESCE(wholesale_prices::jsonb, '[]'::jsonb))"; got != want {
		t.Fatalf("postgres json array length expr mismatch, want %s got %s", want, got)
	}
}

func TestBuildLocalizedLikeCondition(t *testing.T) {
	condition, argCount := BuildLocalizedLikeCondition(nil, []string{"slug"}, []string{"title_json", "description_json"})
	if argCount != 7 {
		t.Fatalf("arg count want 7 got %d", argCount)
	}
	if !strings.Contains(condition, "slug LIKE ?") {
		t.Fatalf("condition should contain slug LIKE, got %s", condition)
	}
	if !strings.Contains(condition, "json_extract(title_json, '$.\"zh-CN\"') LIKE ?") {
		t.Fatalf("condition should contain title zh-CN LIKE, got %s", condition)
	}
	if !strings.Contains(condition, "json_extract(description_json, '$.\"en-US\"') LIKE ?") {
		t.Fatalf("condition should contain description en-US LIKE, got %s", condition)
	}
}

func TestBuildLocalizedLikeConditionByDialectPostgres(t *testing.T) {
	condition, argCount := BuildLocalizedLikeConditionByDialect("postgres", []string{"slug"}, []string{"title_json"})
	if argCount != 4 {
		t.Fatalf("arg count want 4 got %d", argCount)
	}
	if !strings.Contains(condition, "slug ILIKE ?") {
		t.Fatalf("condition should contain slug ILIKE, got %s", condition)
	}
	if !strings.Contains(condition, "(title_json::jsonb ->> 'zh-CN') ILIKE ?") {
		t.Fatalf("condition should contain postgres zh-CN ILIKE, got %s", condition)
	}
}

func TestRepeatLikeArgs(t *testing.T) {
	args := RepeatLikeArgs("%test%", 3)
	if len(args) != 3 {
		t.Fatalf("args len want 3 got %d", len(args))
	}
	for idx, arg := range args {
		if arg != "%test%" {
			t.Fatalf("args[%d] want %%test%% got %v", idx, arg)
		}
	}
}
