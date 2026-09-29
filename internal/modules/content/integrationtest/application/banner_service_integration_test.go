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
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newBannerServiceForTest(t *testing.T) (*contentapp.BannerService, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:banner_service_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&contentdomain.Banner{}); err != nil {
		t.Fatalf("auto migrate banners: %v", err)
	}

	return contentapp.NewBannerService(gormstore.NewBannerStore(db), contentapp.SystemClock{}), db
}

func validBannerInput() contentapp.BannerInput {
	return contentapp.BannerInput{
		Name:     "Home hero",
		Image:    "/uploads/banner.png",
		LinkType: constants.BannerLinkTypeNone,
	}
}

func TestBannerServiceCreateNormalizesDefaultsAndLocalizedText(t *testing.T) {
	svc, _ := newBannerServiceForTest(t)

	banner, err := svc.Create(context.Background(), contentapp.BannerInput{
		Name:         "  Home hero  ",
		Position:     "unsupported-position",
		TitleJSON:    map[string]interface{}{"zh-CN": "  标题  ", "unknown": "ignored"},
		SubtitleJSON: map[string]interface{}{"en-US": 123},
		Image:        "  /uploads/banner.png  ",
		MobileImage:  "  /uploads/banner-mobile.png  ",
		LinkValue:    "ignored-for-none",
	})
	if err != nil {
		t.Fatalf("create banner: %v", err)
	}

	if banner.Name != "Home hero" || banner.Image != "/uploads/banner.png" || banner.MobileImage != "/uploads/banner-mobile.png" {
		t.Fatalf("trimmed fields mismatch: %#v", banner)
	}
	if banner.Position != constants.BannerPositionHomeHero {
		t.Fatalf("position should normalize to home hero, got %q", banner.Position)
	}
	if banner.LinkType != constants.BannerLinkTypeNone || banner.LinkValue != "" {
		t.Fatalf("empty link type should normalize to none and clear value: %#v", banner)
	}
	if !banner.IsActive {
		t.Fatal("new banner should be active by default")
	}
	if banner.TitleJSON[constants.LocaleZhCN] != "标题" || banner.TitleJSON[constants.LocaleZhTW] != "" || banner.TitleJSON[constants.LocaleEnUS] != "" {
		t.Fatalf("localized title normalization mismatch: %#v", banner.TitleJSON)
	}
	if _, exists := banner.TitleJSON["unknown"]; exists {
		t.Fatalf("unsupported locale should not be persisted: %#v", banner.TitleJSON)
	}
	if banner.SubtitleJSON[constants.LocaleEnUS] != "" {
		t.Fatalf("non-string localized values should normalize to empty: %#v", banner.SubtitleJSON)
	}
}

func TestBannerServiceCreatePersistsExplicitInactive(t *testing.T) {
	svc, db := newBannerServiceForTest(t)
	inactive := false

	banner, err := svc.Create(context.Background(), contentapp.BannerInput{
		Name:     "Initially inactive",
		Image:    "/uploads/inactive.png",
		LinkType: constants.BannerLinkTypeNone,
		IsActive: &inactive,
	})
	if err != nil {
		t.Fatalf("create inactive banner: %v", err)
	}
	if banner.IsActive {
		t.Fatalf("created banner should remain inactive in returned entity: %#v", banner)
	}

	var reloaded contentdomain.Banner
	if err := db.First(&reloaded, banner.ID).Error; err != nil {
		t.Fatalf("reload inactive banner: %v", err)
	}
	if reloaded.IsActive {
		t.Fatalf("explicit is_active=false should persist as false: %#v", reloaded)
	}
}

func TestBannerServiceCreateRejectsInvalidInput(t *testing.T) {
	now := time.Now()
	earlier := now.Add(-time.Hour)

	tests := []struct {
		name   string
		mutate func(*contentapp.BannerInput)
	}{
		{
			name: "missing name",
			mutate: func(input *contentapp.BannerInput) {
				input.Name = " "
			},
		},
		{
			name: "missing image",
			mutate: func(input *contentapp.BannerInput) {
				input.Image = " "
			},
		},
		{
			name: "invalid link type",
			mutate: func(input *contentapp.BannerInput) {
				input.LinkType = "javascript"
			},
		},
		{
			name: "missing link value",
			mutate: func(input *contentapp.BannerInput) {
				input.LinkType = constants.BannerLinkTypeInternal
				input.LinkValue = " "
			},
		},
		{
			name: "end before start",
			mutate: func(input *contentapp.BannerInput) {
				input.StartAt = &now
				input.EndAt = &earlier
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			svc, _ := newBannerServiceForTest(t)
			input := validBannerInput()
			test.mutate(&input)

			if _, err := svc.Create(context.Background(), input); err != contentcontract.ErrInvalidBanner {
				t.Fatalf("expected domaincontent.ErrInvalidBanner, got %v", err)
			}
		})
	}
}

func TestBannerServiceListPublicFiltersTimeAndOrdersBySort(t *testing.T) {
	svc, db := newBannerServiceForTest(t)
	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	fixtures := []contentdomain.Banner{
		{Name: "high", Position: constants.BannerPositionHomeHero, Image: "/high.png", LinkType: constants.BannerLinkTypeNone, IsActive: true, StartAt: &past, EndAt: &future, SortOrder: 20},
		{Name: "low", Position: constants.BannerPositionHomeHero, Image: "/low.png", LinkType: constants.BannerLinkTypeNone, IsActive: true, SortOrder: 10},
		{Name: "inactive", Position: constants.BannerPositionHomeHero, Image: "/inactive.png", LinkType: constants.BannerLinkTypeNone, IsActive: false, SortOrder: 50},
		{Name: "future", Position: constants.BannerPositionHomeHero, Image: "/future.png", LinkType: constants.BannerLinkTypeNone, IsActive: true, StartAt: &future, SortOrder: 40},
		{Name: "expired", Position: constants.BannerPositionHomeHero, Image: "/expired.png", LinkType: constants.BannerLinkTypeNone, IsActive: true, EndAt: &past, SortOrder: 30},
		{Name: "other-position", Position: "other", Image: "/other.png", LinkType: constants.BannerLinkTypeNone, IsActive: true, SortOrder: 100},
	}
	for i := range fixtures {
		if err := db.Create(&fixtures[i]).Error; err != nil {
			t.Fatalf("create banner fixture %q: %v", fixtures[i].Name, err)
		}
	}
	if err := db.Model(&contentdomain.Banner{}).Where("id = ?", fixtures[2].ID).Update("is_active", false).Error; err != nil {
		t.Fatalf("deactivate banner fixture: %v", err)
	}

	banners, err := svc.ListPublic(context.Background(), contentapp.PublicBannerQuery{Limit: 10})
	if err != nil {
		t.Fatalf("list public banners: %v", err)
	}
	if len(banners) != 2 || banners[0].Name != "high" || banners[1].Name != "low" {
		t.Fatalf("unexpected public banners: %#v", banners)
	}

	limited, err := svc.ListPublic(context.Background(), contentapp.PublicBannerQuery{
		Position: "unsupported-position",
		Limit:    1,
	})
	if err != nil {
		t.Fatalf("list limited public banners: %v", err)
	}
	if len(limited) != 1 || limited[0].Name != "high" {
		t.Fatalf("expected normalized position and limit, got %#v", limited)
	}
}

func TestBannerServiceUpdatePreservesOptionalBooleans(t *testing.T) {
	svc, db := newBannerServiceForTest(t)
	existing := contentdomain.Banner{
		Name:         "existing",
		Position:     constants.BannerPositionHomeHero,
		Image:        "/existing.png",
		LinkType:     constants.BannerLinkTypeNone,
		OpenInNewTab: true,
		IsActive:     false,
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("create existing banner: %v", err)
	}
	if err := db.Model(&contentdomain.Banner{}).Where("id = ?", existing.ID).Update("is_active", false).Error; err != nil {
		t.Fatalf("deactivate existing banner: %v", err)
	}

	updated, err := svc.Update(context.Background(), fmt.Sprintf("%d", existing.ID), contentapp.BannerInput{
		Name:     "updated",
		Image:    "/updated.png",
		LinkType: constants.BannerLinkTypeNone,
	})
	if err != nil {
		t.Fatalf("update banner: %v", err)
	}
	if !updated.OpenInNewTab || updated.IsActive {
		t.Fatalf("nil optional booleans should preserve existing values: %#v", updated)
	}
}

func TestBannerServiceMissingBannerReturnsNotFound(t *testing.T) {
	svc, _ := newBannerServiceForTest(t)

	if _, err := svc.GetByID(context.Background(), "9999"); err != contentcontract.ErrNotFound {
		t.Fatalf("get missing banner should return domaincontent.ErrNotFound, got %v", err)
	}
	if err := svc.Delete(context.Background(), "9999"); err != contentcontract.ErrNotFound {
		t.Fatalf("delete missing banner should return domaincontent.ErrNotFound, got %v", err)
	}
}
