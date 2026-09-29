package mailbrandwiring

import (
	"context"
	"errors"
	"testing"

	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/mailbrand"
)

type mainBrandReaderStub struct {
	brand settingsapp.SiteBrand
	err   error
	calls int
}

func (s *mainBrandReaderStub) GetSiteBrand() (settingsapp.SiteBrand, error) {
	s.calls++
	return s.brand, s.err
}

type resellerSiteConfigReaderStub struct {
	config *resellerdomain.SiteConfig
	err    error
	calls  int
}

func (s *resellerSiteConfigReaderStub) GetSiteConfigByResellerID(_ uint) (*resellerdomain.SiteConfig, error) {
	s.calls++
	return s.config, s.err
}

func TestResolverUsesRequestTenantWithoutReadingMainBrand(t *testing.T) {
	main := &mainBrandReaderStub{brand: settingsapp.SiteBrand{
		SiteName: "Main Store",
		SiteURL:  "https://main.example.test",
	}}
	resellers := &resellerSiteConfigReaderStub{config: &resellerdomain.SiteConfig{
		SiteName:    "White Label Store",
		SupportJSON: jsonmap.JSON{"email": "support@shop.example.test"},
	}}
	resolver := New(main, resellers)
	tenant := resellercontract.ResellerTenantContext("shop.example.test", 7, 70, "primary.example.test")
	ctx := resellercontract.WithTenantContext(context.Background(), tenant)

	got, err := resolver.ResolveEmailBrand(ctx, mailbrand.Scope{})
	if err != nil {
		t.Fatalf("resolve reseller email brand failed: %v", err)
	}
	if main.calls != 0 {
		t.Fatalf("reseller resolution must not read main brand, calls=%d", main.calls)
	}
	if got.SiteName != "White Label Store" || got.SiteURL != "https://shop.example.test" {
		t.Fatalf("unexpected reseller brand: %+v", got)
	}
	if got.FromName != "White Label Store" || got.ReplyTo != "support@shop.example.test" {
		t.Fatalf("unexpected reseller headers: %+v", got)
	}
}

func TestResolverReturnsSafeResellerFallbackWhenConfigMissing(t *testing.T) {
	main := &mainBrandReaderStub{brand: settingsapp.SiteBrand{
		SiteName: "Main Store",
		SiteURL:  "https://main.example.test",
	}}
	resolver := New(main, &resellerSiteConfigReaderStub{})
	resellerID := uint(9)

	got, err := resolver.ResolveEmailBrand(context.Background(), mailbrand.Scope{
		ResellerID: &resellerID,
		Host:       "fallback.example.test",
	})
	if err != nil {
		t.Fatalf("resolve fallback failed: %v", err)
	}
	if main.calls != 0 {
		t.Fatalf("fallback must not read main brand, calls=%d", main.calls)
	}
	if got.SiteName != "fallback.example.test" || got.SiteURL != "https://fallback.example.test" || got.FromName != "fallback.example.test" {
		t.Fatalf("unexpected safe fallback: %+v", got)
	}
}

func TestResolverFailsClosedOnResellerConfigReadError(t *testing.T) {
	main := &mainBrandReaderStub{brand: settingsapp.SiteBrand{
		SiteName: "Main Store",
		SiteURL:  "https://main.example.test",
	}}
	wantErr := errors.New("database unavailable")
	resolver := New(main, &resellerSiteConfigReaderStub{err: wantErr})
	resellerID := uint(11)

	got, err := resolver.ResolveEmailBrand(context.Background(), mailbrand.Scope{
		ResellerID: &resellerID,
		Host:       "safe.example.test",
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected reseller read error, got brand=%+v err=%v", got, err)
	}
	if main.calls != 0 {
		t.Fatalf("failed reseller resolution must not read main brand, calls=%d", main.calls)
	}
	if got.SiteName != "safe.example.test" || got.SiteURL != "https://safe.example.test" {
		t.Fatalf("error result should remain tenant-safe: %+v", got)
	}
}
