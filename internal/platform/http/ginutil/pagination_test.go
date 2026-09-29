package ginutil

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestParsePagination(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		target       string
		wantPage     int
		wantPageSize int
	}{
		{
			name:         "uses query values",
			target:       "/?page=3&page_size=40",
			wantPage:     3,
			wantPageSize: 40,
		},
		{
			name:         "uses defaults when query is absent",
			target:       "/",
			wantPage:     1,
			wantPageSize: 20,
		},
		{
			name:         "normalizes invalid values",
			target:       "/?page=bad&page_size=bad",
			wantPage:     1,
			wantPageSize: 20,
		},
		{
			name:         "bounds values",
			target:       "/?page=0&page_size=300",
			wantPage:     1,
			wantPageSize: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("GET", tt.target, nil)

			gotPage, gotPageSize := ParsePagination(c)

			if gotPage != tt.wantPage || gotPageSize != tt.wantPageSize {
				t.Fatalf("ParsePagination() = (%d, %d), want (%d, %d)", gotPage, gotPageSize, tt.wantPage, tt.wantPageSize)
			}
		})
	}
}

func TestParsePaginationWithKeysUsesCustomKeysAndDefaultSize(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		target       string
		wantPage     int
		wantPageSize int
	}{
		{
			name:         "uses custom query keys",
			target:       "/?items_page=2&items_page_size=35",
			wantPage:     2,
			wantPageSize: 35,
		},
		{
			name:         "uses custom page size default",
			target:       "/",
			wantPage:     1,
			wantPageSize: 50,
		},
		{
			name:         "normalizes invalid custom values",
			target:       "/?items_page=0&items_page_size=bad",
			wantPage:     1,
			wantPageSize: 20,
		},
		{
			name:         "bounds custom page size",
			target:       "/?items_page=4&items_page_size=500",
			wantPage:     4,
			wantPageSize: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("GET", tt.target, nil)

			gotPage, gotPageSize := ParsePaginationWithKeys(c, "items_page", "items_page_size", 50)

			if gotPage != tt.wantPage || gotPageSize != tt.wantPageSize {
				t.Fatalf("ParsePaginationWithKeys() = (%d, %d), want (%d, %d)", gotPage, gotPageSize, tt.wantPage, tt.wantPageSize)
			}
		})
	}
}

func TestParsePaginationWithBoundsFallsBackWhenPageSizeOutOfRange(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		target       string
		wantPage     int
		wantPageSize int
	}{
		{
			name:         "uses values inside bounds",
			target:       "/?page=3&page_size=12",
			wantPage:     3,
			wantPageSize: 12,
		},
		{
			name:         "uses default size when absent",
			target:       "/",
			wantPage:     1,
			wantPageSize: 5,
		},
		{
			name:         "falls back to default size when too large",
			target:       "/?page=2&page_size=21",
			wantPage:     2,
			wantPageSize: 5,
		},
		{
			name:         "falls back to default size when invalid",
			target:       "/?page=0&page_size=bad",
			wantPage:     1,
			wantPageSize: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("GET", tt.target, nil)

			gotPage, gotPageSize := ParsePaginationWithBounds(c, "page", "page_size", 5, 20)

			if gotPage != tt.wantPage || gotPageSize != tt.wantPageSize {
				t.Fatalf("ParsePaginationWithBounds() = (%d, %d), want (%d, %d)", gotPage, gotPageSize, tt.wantPage, tt.wantPageSize)
			}
		})
	}
}
