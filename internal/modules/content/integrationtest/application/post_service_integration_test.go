package application_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	contentapp "github.com/dujiao-next/internal/modules/content/application"
	contentcontract "github.com/dujiao-next/internal/modules/content/contract"
	contentdomain "github.com/dujiao-next/internal/modules/content/domain"
	"github.com/dujiao-next/internal/modules/content/infrastructure/gormstore"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newPostServiceForTest(t *testing.T) (*contentapp.PostService, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:post_service_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&contentdomain.PostCategory{}, &contentdomain.Post{}, &contentdomain.PostProduct{}); err != nil {
		t.Fatalf("auto migrate post tables failed: %v", err)
	}

	postStore := gormstore.NewPostStore(db)
	return contentapp.NewPostService(
		postStore,
		postStore,
		gormstore.NewPostCategoryStore(db),
		contentapp.SystemClock{},
	), db
}

func createPostCategoryFixture(t *testing.T, db *gorm.DB, slug string, parentID *uint) contentdomain.PostCategory {
	t.Helper()

	category := contentdomain.PostCategory{
		ParentID: parentID,
		Slug:     slug,
		NameJSON: jsonmap.JSON{
			"zh-CN": slug,
		},
		IsActive: true,
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create post category fixture failed: %v", err)
	}
	return category
}

func createPostFixture(t *testing.T, db *gorm.DB, slug string, postType string, categoryID *uint) contentdomain.Post {
	t.Helper()

	post := contentdomain.Post{
		Slug:       slug,
		Type:       postType,
		TitleJSON:  jsonmap.JSON{"zh-CN": slug},
		CategoryID: categoryID,
	}
	if err := db.Create(&post).Error; err != nil {
		t.Fatalf("create post fixture failed: %v", err)
	}
	return post
}

func TestPostServiceCreateRejectsNoticeCategory(t *testing.T) {
	svc, db := newPostServiceForTest(t)
	leaf := createPostCategoryFixture(t, db, "announcements", nil)

	_, err := svc.Create(context.Background(), contentapp.CreatePostInput{
		Slug:       "notice-with-category",
		Type:       constants.PostTypeNotice,
		TitleJSON:  map[string]interface{}{"zh-CN": "notice-with-category"},
		CategoryID: &leaf.ID,
	})
	if err != contentcontract.ErrPostNoticeCategoryUnsupported {
		t.Fatalf("expected domaincontent.ErrPostNoticeCategoryUnsupported, got %v", err)
	}
}

func TestPostServiceCreateRejectsMissingOrParentCategory(t *testing.T) {
	svc, db := newPostServiceForTest(t)
	parent := createPostCategoryFixture(t, db, "blog", nil)
	_ = createPostCategoryFixture(t, db, "backend", &parent.ID)

	missingID := uint(9999)
	_, err := svc.Create(context.Background(), contentapp.CreatePostInput{
		Slug:       "blog-missing-category",
		Type:       constants.PostTypeBlog,
		TitleJSON:  map[string]interface{}{"zh-CN": "blog-missing-category"},
		CategoryID: &missingID,
	})
	if err != contentcontract.ErrPostCategoryInvalid {
		t.Fatalf("expected domaincontent.ErrPostCategoryInvalid for missing category, got %v", err)
	}

	_, err = svc.Create(context.Background(), contentapp.CreatePostInput{
		Slug:       "blog-parent-category",
		Type:       constants.PostTypeBlog,
		TitleJSON:  map[string]interface{}{"zh-CN": "blog-parent-category"},
		CategoryID: &parent.ID,
	})
	if err != contentcontract.ErrPostCategoryInvalid {
		t.Fatalf("expected domaincontent.ErrPostCategoryInvalid for parent category, got %v", err)
	}
}

func TestPostServiceUpdateRejectsUnsupportedOrInvalidCategoryAssignment(t *testing.T) {
	svc, db := newPostServiceForTest(t)
	parent := createPostCategoryFixture(t, db, "blog", nil)
	leaf := createPostCategoryFixture(t, db, "backend", &parent.ID)
	post := createPostFixture(t, db, "service-post", constants.PostTypeBlog, &leaf.ID)

	_, err := svc.Update(context.Background(), fmt.Sprintf("%d", post.ID), contentapp.CreatePostInput{
		Slug:       post.Slug,
		Type:       constants.PostTypeNotice,
		TitleJSON:  map[string]interface{}{"zh-CN": post.Slug},
		CategoryID: &leaf.ID,
	})
	if err != contentcontract.ErrPostNoticeCategoryUnsupported {
		t.Fatalf("expected domaincontent.ErrPostNoticeCategoryUnsupported on notice update, got %v", err)
	}

	_, err = svc.Update(context.Background(), fmt.Sprintf("%d", post.ID), contentapp.CreatePostInput{
		Slug:       post.Slug,
		Type:       constants.PostTypeBlog,
		TitleJSON:  map[string]interface{}{"zh-CN": post.Slug},
		CategoryID: &parent.ID,
	})
	if err != contentcontract.ErrPostCategoryInvalid {
		t.Fatalf("expected domaincontent.ErrPostCategoryInvalid on parent category update, got %v", err)
	}
}

func TestPostServiceCategoryAssignmentRespectsInactive(t *testing.T) {
	svc, db := newPostServiceForTest(t)
	inactive := createPostCategoryFixture(t, db, "archived", nil)
	if err := db.Model(&contentdomain.PostCategory{}).Where("id = ?", inactive.ID).Update("is_active", false).Error; err != nil {
		t.Fatalf("deactivate category fixture failed: %v", err)
	}

	_, err := svc.Create(context.Background(), contentapp.CreatePostInput{
		Slug:       "blog-inactive-category",
		Type:       constants.PostTypeBlog,
		TitleJSON:  map[string]interface{}{"zh-CN": "blog-inactive-category"},
		CategoryID: &inactive.ID,
	})
	if err != contentcontract.ErrPostCategoryInvalid {
		t.Fatalf("expected domaincontent.ErrPostCategoryInvalid for inactive category, got %v", err)
	}

	// 文章已有分类后来被禁用：保存时保留原分类应放行
	post := createPostFixture(t, db, "blog-keeps-category", constants.PostTypeBlog, &inactive.ID)
	updated, err := svc.Update(context.Background(), fmt.Sprintf("%d", post.ID), contentapp.CreatePostInput{
		Slug:       post.Slug,
		Type:       constants.PostTypeBlog,
		TitleJSON:  map[string]interface{}{"zh-CN": post.Slug},
		CategoryID: &inactive.ID,
	})
	if err != nil {
		t.Fatalf("expected keeping now-inactive category to succeed, got %v", err)
	}
	if updated.CategoryID == nil || *updated.CategoryID != inactive.ID {
		t.Fatalf("expected category to be kept, got %v", updated.CategoryID)
	}
}

func TestPostServiceListPublicFiltersDraftsAndOrdersByPublishedAt(t *testing.T) {
	svc, db := newPostServiceForTest(t)
	now := time.Now().UTC()
	olderPublishedAt := now.Add(-2 * time.Hour)
	newerPublishedAt := now.Add(-time.Hour)

	fixtures := []contentdomain.Post{
		{
			Slug:        "older-published",
			Type:        constants.PostTypeBlog,
			TitleJSON:   jsonmap.JSON{"zh-CN": "older"},
			IsPublished: true,
			PublishedAt: &olderPublishedAt,
		},
		{
			Slug:        "newer-published",
			Type:        constants.PostTypeBlog,
			TitleJSON:   jsonmap.JSON{"zh-CN": "newer"},
			IsPublished: true,
			PublishedAt: &newerPublishedAt,
		},
		{
			Slug:        "draft",
			Type:        constants.PostTypeBlog,
			TitleJSON:   jsonmap.JSON{"zh-CN": "draft"},
			IsPublished: false,
		},
	}
	for i := range fixtures {
		if err := db.Create(&fixtures[i]).Error; err != nil {
			t.Fatalf("create post fixture %q: %v", fixtures[i].Slug, err)
		}
	}

	posts, total, err := svc.ListPublic(context.Background(), contentapp.PublicPostQuery{
		Type:     constants.PostTypeBlog,
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("list public posts: %v", err)
	}
	if total != 2 || len(posts) != 2 {
		t.Fatalf("expected two published posts, total=%d posts=%#v", total, posts)
	}
	if posts[0].Slug != "newer-published" || posts[1].Slug != "older-published" {
		t.Fatalf("unexpected public post order: %v, %v", posts[0].Slug, posts[1].Slug)
	}
}

func TestPostServiceGetPublicBySlugHidesDraftsAndMissingPosts(t *testing.T) {
	svc, db := newPostServiceForTest(t)
	_ = createPostFixture(t, db, "draft-post", constants.PostTypeBlog, nil)

	if _, err := svc.GetPublicBySlug(context.Background(), "draft-post"); err != contentcontract.ErrNotFound {
		t.Fatalf("expected draft to be hidden as not found, got %v", err)
	}
	if _, err := svc.GetPublicBySlug(context.Background(), "missing-post"); err != contentcontract.ErrNotFound {
		t.Fatalf("expected missing post to return domaincontent.ErrNotFound, got %v", err)
	}
}

func TestPostServiceCreateRejectsDuplicateSlug(t *testing.T) {
	svc, db := newPostServiceForTest(t)
	_ = createPostFixture(t, db, "duplicate-slug", constants.PostTypeBlog, nil)

	_, err := svc.Create(context.Background(), contentapp.CreatePostInput{
		Slug:      "duplicate-slug",
		Type:      constants.PostTypeBlog,
		TitleJSON: map[string]interface{}{"zh-CN": "duplicate"},
	})
	if err != contentcontract.ErrSlugExists {
		t.Fatalf("expected domaincontent.ErrSlugExists, got %v", err)
	}
}

func TestPostServicePublishedAtIsSetOnlyOnFirstPublish(t *testing.T) {
	svc, _ := newPostServiceForTest(t)
	draft, err := svc.Create(context.Background(), contentapp.CreatePostInput{
		Slug:      "publish-transition",
		Type:      constants.PostTypeBlog,
		TitleJSON: map[string]interface{}{"zh-CN": "publish-transition"},
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	if draft.PublishedAt != nil {
		t.Fatalf("draft should not have published_at, got %v", draft.PublishedAt)
	}

	published := true
	firstPublish, err := svc.Update(context.Background(), fmt.Sprintf("%d", draft.ID), contentapp.CreatePostInput{
		Slug:        draft.Slug,
		Type:        constants.PostTypeBlog,
		TitleJSON:   map[string]interface{}{"zh-CN": "first publish"},
		IsPublished: &published,
	})
	if err != nil {
		t.Fatalf("publish draft: %v", err)
	}
	if firstPublish.PublishedAt == nil {
		t.Fatal("first publish should set published_at")
	}
	firstPublishedAt := *firstPublish.PublishedAt

	published = false
	if _, err := svc.Update(context.Background(), fmt.Sprintf("%d", draft.ID), contentapp.CreatePostInput{
		Slug:        draft.Slug,
		Type:        constants.PostTypeBlog,
		TitleJSON:   map[string]interface{}{"zh-CN": "unpublished"},
		IsPublished: &published,
	}); err != nil {
		t.Fatalf("unpublish post: %v", err)
	}

	published = true
	republished, err := svc.Update(context.Background(), fmt.Sprintf("%d", draft.ID), contentapp.CreatePostInput{
		Slug:        draft.Slug,
		Type:        constants.PostTypeBlog,
		TitleJSON:   map[string]interface{}{"zh-CN": "republished"},
		IsPublished: &published,
	})
	if err != nil {
		t.Fatalf("republish post: %v", err)
	}
	if republished.PublishedAt == nil || !republished.PublishedAt.Equal(firstPublishedAt) {
		t.Fatalf("republish must preserve first published_at, first=%v got=%v", firstPublishedAt, republished.PublishedAt)
	}
}

func TestPostServiceRelatedProductIDsPreserveOrderAndDeduplicate(t *testing.T) {
	svc, _ := newPostServiceForTest(t)
	productIDs := []uint{7, 0, 3, 7, 5}
	post, err := svc.Create(context.Background(), contentapp.CreatePostInput{
		Slug:       "related-products",
		Type:       constants.PostTypeBlog,
		TitleJSON:  map[string]interface{}{"zh-CN": "related-products"},
		ProductIDs: &productIDs,
	})
	if err != nil {
		t.Fatalf("create post with related products: %v", err)
	}

	got, err := svc.GetRelatedProductIDs(context.Background(), post.ID)
	if err != nil {
		t.Fatalf("get related product IDs: %v", err)
	}
	want := []uint{7, 3, 5}
	if len(got) != len(want) {
		t.Fatalf("related product IDs want %v got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("related product IDs want %v got %v", want, got)
		}
	}
}

func TestPostServiceCreateRollsBackPostWhenRelatedProductsFail(t *testing.T) {
	svc, db := newPostServiceForTest(t)
	installFailingPostProductTrigger(t, db, 42)
	productIDs := []uint{42}

	_, err := svc.Create(context.Background(), contentapp.CreatePostInput{
		Slug:       "atomic-create",
		Type:       constants.PostTypeBlog,
		TitleJSON:  map[string]interface{}{"zh-CN": "atomic-create"},
		ProductIDs: &productIDs,
	})
	if err == nil {
		t.Fatal("expected related product insert failure")
	}

	var postCount int64
	if countErr := db.Model(&contentdomain.Post{}).Where("slug = ? AND deleted_at IS NULL", "atomic-create").Count(&postCount).Error; countErr != nil {
		t.Fatalf("count posts after failed create: %v", countErr)
	}
	if postCount != 0 {
		t.Fatalf("failed aggregate create must roll back post, count=%d", postCount)
	}
}

func TestPostServiceUpdateRollsBackPostAndRelationsWhenRelatedProductsFail(t *testing.T) {
	svc, db := newPostServiceForTest(t)
	post := createPostFixture(t, db, "atomic-update-before", constants.PostTypeBlog, nil)
	if err := db.Create(&contentdomain.PostProduct{PostID: post.ID, ProductID: 7}).Error; err != nil {
		t.Fatalf("create existing post relation: %v", err)
	}
	installFailingPostProductTrigger(t, db, 42)
	productIDs := []uint{42}

	_, err := svc.Update(context.Background(), fmt.Sprintf("%d", post.ID), contentapp.CreatePostInput{
		Slug:       "atomic-update-after",
		Type:       constants.PostTypeBlog,
		TitleJSON:  map[string]interface{}{"zh-CN": "atomic-update-after"},
		ProductIDs: &productIDs,
	})
	if err == nil {
		t.Fatal("expected related product insert failure")
	}

	var reloaded contentdomain.Post
	if reloadErr := db.First(&reloaded, post.ID).Error; reloadErr != nil {
		t.Fatalf("reload post after failed update: %v", reloadErr)
	}
	if reloaded.Slug != "atomic-update-before" {
		t.Fatalf("failed aggregate update must roll back post, slug=%q", reloaded.Slug)
	}

	var relatedProductIDs []uint
	if relationErr := db.Model(&contentdomain.PostProduct{}).
		Where("post_id = ?", post.ID).
		Order("sort ASC, id ASC").
		Pluck("product_id", &relatedProductIDs).Error; relationErr != nil {
		t.Fatalf("reload relations after failed update: %v", relationErr)
	}
	if len(relatedProductIDs) != 1 || relatedProductIDs[0] != 7 {
		t.Fatalf("failed aggregate update must preserve old relations, got %v", relatedProductIDs)
	}
}

func installFailingPostProductTrigger(t *testing.T, db *gorm.DB, productID uint) {
	t.Helper()
	statement := fmt.Sprintf(`
		CREATE TRIGGER fail_post_product_insert
		BEFORE INSERT ON post_products
		WHEN NEW.product_id = %d
		BEGIN
			SELECT RAISE(ABORT, 'forced relation failure');
		END
	`, productID)
	if err := db.Exec(statement).Error; err != nil {
		t.Fatalf("install failing post product trigger: %v", err)
	}
}

func TestPostServiceListPostsForProductFiltersTypeAndPublication(t *testing.T) {
	svc, _ := newPostServiceForTest(t)
	productIDs := []uint{42}
	published := true
	draft := false

	create := func(slug, postType string, isPublished *bool) {
		t.Helper()
		if _, err := svc.Create(context.Background(), contentapp.CreatePostInput{
			Slug:        slug,
			Type:        postType,
			TitleJSON:   map[string]interface{}{"zh-CN": slug},
			IsPublished: isPublished,
			ProductIDs:  &productIDs,
		}); err != nil {
			t.Fatalf("create related post %q: %v", slug, err)
		}
	}

	create("first-blog", constants.PostTypeBlog, &published)
	create("notice", constants.PostTypeNotice, &published)
	create("draft-blog", constants.PostTypeBlog, &draft)
	create("second-blog", constants.PostTypeBlog, &published)

	posts, err := svc.ListPostsForProduct(context.Background(), 42, 10)
	if err != nil {
		t.Fatalf("list posts for product: %v", err)
	}
	if len(posts) != 2 || posts[0].Slug != "first-blog" || posts[1].Slug != "second-blog" {
		t.Fatalf("expected only published blogs in relation order, got %#v", posts)
	}

	limited, err := svc.ListPostsForProduct(context.Background(), 42, 1)
	if err != nil {
		t.Fatalf("list limited posts for product: %v", err)
	}
	if len(limited) != 1 || limited[0].Slug != "first-blog" {
		t.Fatalf("expected first related published blog, got %#v", limited)
	}
}
