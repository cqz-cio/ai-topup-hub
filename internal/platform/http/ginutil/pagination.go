package ginutil

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// ParsePagination 从 query 中读取默认分页参数并归一化。
func ParsePagination(c *gin.Context) (int, int) {
	return ParsePaginationWithKeys(c, "page", "page_size", 20)
}

// ParsePaginationWithKeys 从自定义 query key 中读取分页参数并归一化。
func ParsePaginationWithKeys(c *gin.Context, pageKey, pageSizeKey string, defaultPageSize int) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery(pageKey, "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery(pageSizeKey, strconv.Itoa(defaultPageSize)))
	return NormalizePagination(page, pageSize)
}

// ParsePaginationWithBounds 从 query 中读取分页参数，page_size 越界时回退到默认值。
func ParsePaginationWithBounds(c *gin.Context, pageKey, pageSizeKey string, defaultPageSize, maxPageSize int) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery(pageKey, "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery(pageSizeKey, strconv.Itoa(defaultPageSize)))
	if page < 1 {
		page = 1
	}
	if defaultPageSize <= 0 {
		defaultPageSize = 20
	}
	if maxPageSize <= 0 {
		maxPageSize = 200
	}
	if pageSize < 1 || pageSize > maxPageSize {
		pageSize = defaultPageSize
	}
	return page, pageSize
}

// NormalizePagination 归一化分页参数。
func NormalizePagination(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize
}
