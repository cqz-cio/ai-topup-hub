package http_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	contentapp "github.com/dujiao-next/internal/modules/content/application"
	contentdomain "github.com/dujiao-next/internal/modules/content/domain"
	"github.com/dujiao-next/internal/modules/content/infrastructure/gormstore"
	contenttransport "github.com/dujiao-next/internal/modules/content/transport/http"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupPublicContentHandlerTest(t *testing.T) (
	*contenttransport.PublicHandler,
	*contentapp.PostService,
	*contentapp.PostCategoryService,
	*contentapp.BannerService,
	*gorm.DB,
) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dsn := fmt.Sprintf("file:public_content_handler_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&contentdomain.PostCategory{},
		&contentdomain.Post{},
		&contentdomain.PostProduct{},
		&contentdomain.Banner{},
	); err != nil {
		t.Fatalf("auto migrate public content tables: %v", err)
	}

	postStore := gormstore.NewPostStore(db)
	postService := contentapp.NewPostService(
		postStore,
		postStore,
		gormstore.NewPostCategoryStore(db),
		contentapp.SystemClock{},
	)
	postCategoryService := contentapp.NewPostCategoryService(gormstore.NewPostCategoryStore(db))
	bannerService := contentapp.NewBannerService(gormstore.NewBannerStore(db), contentapp.SystemClock{})
	return contenttransport.NewPublicHandler(postService, postCategoryService, bannerService),
		postService,
		postCategoryService,
		bannerService,
		db
}

func publicContentTestRouter(handler *contenttransport.PublicHandler) *gin.Engine {
	router := gin.New()
	public := router.Group("/api/v1/public")
	contenttransport.RegisterPublicRoutes(public, handler)
	return router
}

func requestPublicContent(t *testing.T, router http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 for %s, got %d body=%s", path, recorder.Code, recorder.Body.String())
	}
	return recorder
}

func TestPublicContentHandlersReturnOnlyPublicData(t *testing.T) {
	handler, posts, categories, banners, _ := setupPublicContentHandlerTest(t)
	router := publicContentTestRouter(handler)

	published := true
	if _, err := posts.Create(context.Background(), contentapp.CreatePostInput{
		Slug:        "published-notice",
		Type:        constants.PostTypeNotice,
		TitleJSON:   map[string]interface{}{"zh-CN": "published"},
		IsPublished: &published,
	}); err != nil {
		t.Fatalf("create published notice: %v", err)
	}
	if _, err := posts.Create(context.Background(), contentapp.CreatePostInput{
		Slug:      "draft-notice",
		Type:      constants.PostTypeNotice,
		TitleJSON: map[string]interface{}{"zh-CN": "draft"},
	}); err != nil {
		t.Fatalf("create draft notice: %v", err)
	}

	activeCategory, err := categories.Create(context.Background(), contentapp.CreatePostCategoryInput{
		NameJSON: jsonmap.JSON{"zh-CN": "active"},
		Slug:     "active",
	})
	if err != nil {
		t.Fatalf("create active category: %v", err)
	}
	disabledCategory, err := categories.Create(context.Background(), contentapp.CreatePostCategoryInput{
		NameJSON: jsonmap.JSON{"zh-CN": "disabled"},
		Slug:     "disabled",
	})
	if err != nil {
		t.Fatalf("create disabled category: %v", err)
	}
	if _, err := categories.SetActive(context.Background(), disabledCategory.ID, false); err != nil {
		t.Fatalf("disable category: %v", err)
	}

	if _, err := banners.Create(context.Background(), contentapp.BannerInput{
		Name:     "public banner",
		Image:    "/uploads/banner.png",
		LinkType: constants.BannerLinkTypeNone,
	}); err != nil {
		t.Fatalf("create public banner: %v", err)
	}

	postsRecorder := requestPublicContent(t, router, "/api/v1/public/posts?type=notice")
	var postsResponse struct {
		StatusCode int `json:"status_code"`
		Data       []struct {
			Slug string `json:"slug"`
		} `json:"data"`
		Pagination response.Pagination `json:"pagination"`
	}
	if err := json.Unmarshal(postsRecorder.Body.Bytes(), &postsResponse); err != nil {
		t.Fatalf("decode posts response: %v body=%s", err, postsRecorder.Body.String())
	}
	if postsResponse.StatusCode != response.CodeOK || len(postsResponse.Data) != 1 || postsResponse.Data[0].Slug != "published-notice" {
		t.Fatalf("public list must hide drafts: %#v", postsResponse)
	}
	if postsResponse.Pagination.Total != 1 {
		t.Fatalf("public post pagination total want 1 got %#v", postsResponse.Pagination)
	}

	detailRecorder := requestPublicContent(t, router, "/api/v1/public/posts/published-notice")
	var detailResponse struct {
		StatusCode int `json:"status_code"`
		Data       struct {
			Slug string `json:"slug"`
		} `json:"data"`
	}
	if err := json.Unmarshal(detailRecorder.Body.Bytes(), &detailResponse); err != nil {
		t.Fatalf("decode post detail response: %v", err)
	}
	if detailResponse.StatusCode != response.CodeOK || detailResponse.Data.Slug != "published-notice" {
		t.Fatalf("published detail mismatch: %#v", detailResponse)
	}

	draftRecorder := requestPublicContent(t, router, "/api/v1/public/posts/draft-notice")
	var draftResponse response.Response
	if err := json.Unmarshal(draftRecorder.Body.Bytes(), &draftResponse); err != nil {
		t.Fatalf("decode hidden draft response: %v", err)
	}
	if draftResponse.StatusCode != response.CodeNotFound {
		t.Fatalf("draft detail should be not found, got %#v", draftResponse)
	}

	categoriesRecorder := requestPublicContent(t, router, "/api/v1/public/post-categories")
	var categoriesResponse struct {
		StatusCode int `json:"status_code"`
		Data       []struct {
			ID       uint `json:"id"`
			ParentID uint `json:"parent_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(categoriesRecorder.Body.Bytes(), &categoriesResponse); err != nil {
		t.Fatalf("decode categories response: %v", err)
	}
	if categoriesResponse.StatusCode != response.CodeOK || len(categoriesResponse.Data) != 1 || categoriesResponse.Data[0].ID != activeCategory.ID || categoriesResponse.Data[0].ParentID != 0 {
		t.Fatalf("public categories must be active and flatten root parent: %#v", categoriesResponse)
	}

	bannersRecorder := requestPublicContent(t, router, "/api/v1/public/banners?limit=1")
	var bannersResponse struct {
		StatusCode int `json:"status_code"`
		Data       []struct {
			Image string `json:"image"`
		} `json:"data"`
	}
	if err := json.Unmarshal(bannersRecorder.Body.Bytes(), &bannersResponse); err != nil {
		t.Fatalf("decode banners response: %v", err)
	}
	if bannersResponse.StatusCode != response.CodeOK || len(bannersResponse.Data) != 1 || bannersResponse.Data[0].Image != "/uploads/banner.png" {
		t.Fatalf("public banners response mismatch: %#v", bannersResponse)
	}
}

func TestPublicContentHandlersPreserveRepositoryErrorMapping(t *testing.T) {
	handler, _, _, _, db := setupPublicContentHandlerTest(t)
	router := publicContentTestRouter(handler)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("resolve sql database: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close sql database: %v", err)
	}

	for _, path := range []string{
		"/api/v1/public/posts",
		"/api/v1/public/banners",
	} {
		recorder := requestPublicContent(t, router, path)
		var got response.Response
		if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode error response for %s: %v", path, err)
		}
		if got.StatusCode != response.CodeInternal {
			t.Fatalf("expected internal error for %s, got %#v", path, got)
		}
	}

	recorder := requestPublicContent(t, router, "/api/v1/public/post-categories")
	var categories response.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &categories); err != nil {
		t.Fatalf("decode category error response: %v", err)
	}
	if categories.StatusCode != response.CodeInternal {
		t.Fatalf("category repository error should be observable as internal error, got %#v", categories)
	}
}
