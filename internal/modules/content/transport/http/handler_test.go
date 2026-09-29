package contenthttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	contentapp "github.com/dujiao-next/internal/modules/content/application"
	contentcontract "github.com/dujiao-next/internal/modules/content/contract"
	contentdomain "github.com/dujiao-next/internal/modules/content/domain"
	"github.com/gin-gonic/gin"
)

type requestContextKey struct{}

func TestPublicHandlerPassesRequestContextToUseCase(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	posts := &publicPostQueriesStub{}
	handler := NewPublicHandler(posts, nil, nil)
	router := gin.New()
	router.GET("/posts", handler.GetPosts)

	requestContext, cancel := context.WithCancel(context.WithValue(context.Background(), requestContextKey{}, "request-value"))
	cancel()
	request := httptest.NewRequest(http.MethodGet, "/posts?type=blog&page=2&page_size=5", nil).WithContext(requestContext)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if posts.receivedContext == nil {
		t.Fatal("use case did not receive a context")
	}
	if got := posts.receivedContext.Value(requestContextKey{}); got != "request-value" {
		t.Fatalf("context value = %v, want request-value", got)
	}
	if posts.receivedContext.Err() != context.Canceled {
		t.Fatalf("context error = %v, want context.Canceled", posts.receivedContext.Err())
	}
	if posts.receivedQuery.Type != "blog" || posts.receivedQuery.Page != 2 || posts.receivedQuery.PageSize != 5 {
		t.Fatalf("query mapping mismatch: %#v", posts.receivedQuery)
	}
}

func TestAdminHandlerPassesRequestContextToUseCase(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	media := &adminMediaUseCasesStub{}
	handler := NewAdminHandler(nil, nil, nil, media)
	router := gin.New()
	router.GET("/media", handler.GetAdminMedia)

	requestContext := context.WithValue(context.Background(), requestContextKey{}, "admin-request")
	request := httptest.NewRequest(http.MethodGet, "/media?scene=post&page=3&page_size=15", nil).WithContext(requestContext)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if media.receivedContext == nil || media.receivedContext.Value(requestContextKey{}) != "admin-request" {
		t.Fatalf("admin use case received wrong context: %v", media.receivedContext)
	}
	if media.receivedQuery.Scene != "post" || media.receivedQuery.Page != 3 || media.receivedQuery.PageSize != 15 {
		t.Fatalf("media query mapping mismatch: %#v", media.receivedQuery)
	}
}

type publicPostQueriesStub struct {
	receivedContext context.Context
	receivedQuery   contentapp.PublicPostQuery
}

var _ PublicPostQueries = (*publicPostQueriesStub)(nil)

func (s *publicPostQueriesStub) ListPublic(ctx context.Context, query contentapp.PublicPostQuery) ([]contentdomain.Post, int64, error) {
	s.receivedContext = ctx
	s.receivedQuery = query
	return []contentdomain.Post{}, 0, nil
}

func (s *publicPostQueriesStub) GetPublicBySlug(context.Context, string) (*contentdomain.Post, error) {
	return nil, contentcontract.ErrNotFound
}

func (s *publicPostQueriesStub) ListRelatedProducts(context.Context, uint) ([]contentcontract.RelatedProduct, error) {
	return nil, nil
}

type adminMediaUseCasesStub struct {
	receivedContext context.Context
	receivedQuery   contentapp.MediaListQuery
}

var _ AdminMediaUseCases = (*adminMediaUseCasesStub)(nil)

func (s *adminMediaUseCasesStub) List(ctx context.Context, query contentapp.MediaListQuery) ([]contentdomain.Media, int64, error) {
	s.receivedContext = ctx
	s.receivedQuery = query
	return []contentdomain.Media{}, 0, nil
}

func (s *adminMediaUseCasesStub) Rename(context.Context, uint, string) error { return nil }
func (s *adminMediaUseCasesStub) Delete(context.Context, uint) error         { return nil }
func (s *adminMediaUseCasesStub) BatchDelete(context.Context, []uint) (int, []uint) {
	return 0, []uint{}
}
