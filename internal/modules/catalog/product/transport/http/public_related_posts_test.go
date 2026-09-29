package producthttp

import (
	"context"
	"errors"
	"testing"

	contentcontract "github.com/dujiao-next/internal/modules/content/contract"
)

type relatedPostReaderStub struct {
	receivedContext context.Context
	productID       uint
	limit           int
	posts           []contentcontract.RelatedPost
	err             error
}

var _ RelatedPostReader = (*relatedPostReaderStub)(nil)

func (s *relatedPostReaderStub) ListPostsForProduct(ctx context.Context, productID uint, limit int) ([]contentcontract.RelatedPost, error) {
	s.receivedContext = ctx
	s.productID = productID
	s.limit = limit
	return s.posts, s.err
}

func TestLoadRelatedPostCardsUsesConsumerOwnedReaderAndRequestContext(t *testing.T) {
	t.Parallel()

	type contextKey struct{}
	requestContext := context.WithValue(context.Background(), contextKey{}, "product-request")
	reader := &relatedPostReaderStub{posts: []contentcontract.RelatedPost{{Slug: "guide"}}}
	handler := &PublicHandler{relatedPosts: reader}

	cards, err := handler.loadRelatedPostCards(requestContext, 42)
	if err != nil {
		t.Fatalf("loadRelatedPostCards() error = %v", err)
	}
	if reader.receivedContext.Value(contextKey{}) != "product-request" {
		t.Fatal("request context was not propagated")
	}
	if reader.productID != 42 || reader.limit != publicRelatedPostsLimit {
		t.Fatalf("reader arguments = (%d, %d)", reader.productID, reader.limit)
	}
	if len(cards) != 1 || cards[0].Slug != "guide" {
		t.Fatalf("cards mismatch: %#v", cards)
	}
}

func TestLoadRelatedPostCardsReturnsReaderErrorForBestEffortCaller(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("content unavailable")
	handler := &PublicHandler{relatedPosts: &relatedPostReaderStub{err: wantErr}}
	if _, err := handler.loadRelatedPostCards(context.Background(), 42); !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}
