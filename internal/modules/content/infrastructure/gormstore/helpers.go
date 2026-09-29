package gormstore

import (
	"context"
	"fmt"
	"strings"

	"github.com/dujiao-next/internal/constants"
	"gorm.io/gorm"
)

var localizedJSONSearchKeys = append([]string(nil), constants.SupportedLocales...)

func withContext(db *gorm.DB, ctx context.Context) *gorm.DB {
	if ctx == nil {
		return db
	}
	return db.WithContext(ctx)
}

func applyPagination(query *gorm.DB, page, pageSize int) *gorm.DB {
	if query == nil || pageSize <= 0 {
		return query
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}
	return query.Limit(pageSize).Offset(offset)
}

func dbDialectName(db *gorm.DB) string {
	if db == nil || db.Dialector == nil {
		return "sqlite"
	}
	name := strings.ToLower(strings.TrimSpace(db.Dialector.Name()))
	if name == "" {
		return "sqlite"
	}
	return name
}

func buildLocalizedLikeCondition(db *gorm.DB, plainColumns, jsonColumns []string) (string, int) {
	return buildLocalizedLikeConditionByDialect(dbDialectName(db), plainColumns, jsonColumns)
}

func buildLocalizedLikeConditionByDialect(dialect string, plainColumns, jsonColumns []string) (string, int) {
	parts := make([]string, 0, len(plainColumns)+len(jsonColumns)*len(localizedJSONSearchKeys))
	argCount := 0
	operator := likeOperatorByDialect(dialect)

	for _, column := range plainColumns {
		trimmed := strings.TrimSpace(column)
		if trimmed == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %s ?", trimmed, operator))
		argCount++
	}

	for _, column := range jsonColumns {
		trimmed := strings.TrimSpace(column)
		if trimmed == "" {
			continue
		}
		for _, key := range localizedJSONSearchKeys {
			parts = append(parts, fmt.Sprintf("%s %s ?", jsonTextExprByDialect(dialect, trimmed, key), operator))
			argCount++
		}
	}

	return strings.Join(parts, " OR "), argCount
}

func jsonTextExprByDialect(dialect, column, key string) string {
	switch strings.ToLower(strings.TrimSpace(dialect)) {
	case "postgres", "postgresql":
		return fmt.Sprintf("(%s::jsonb ->> '%s')", column, key)
	default:
		return fmt.Sprintf("json_extract(%s, '$.\"%s\"')", column, key)
	}
}

func likeOperatorByDialect(dialect string) string {
	switch strings.ToLower(strings.TrimSpace(dialect)) {
	case "postgres", "postgresql":
		return "ILIKE"
	default:
		return "LIKE"
	}
}

func repeatLikeArgs(like string, count int) []interface{} {
	args := make([]interface{}, 0, count)
	for i := 0; i < count; i++ {
		args = append(args, like)
	}
	return args
}
