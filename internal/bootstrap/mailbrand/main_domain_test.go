package mailbrandwiring

import (
	"context"
	"testing"

	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	"github.com/dujiao-next/internal/shared/mailbrand"
)

func TestMainMailBrandUsesPublicDomainInsteadOfLegacyAddress(t *testing.T) {
	for _, legacyURL := range []string{"http://43.134.141.10", "http://127.0.0.1:5173", ""} {
		t.Run(legacyURL, func(t *testing.T) {
			source := &mainBrandReaderStub{brand: settingsapp.SiteBrand{
				SiteName: "π星卡社", SiteURL: legacyURL,
			}}
			brand, err := New(source, nil).ResolveEmailBrand(context.Background(), mailbrand.Scope{})
			if err != nil {
				t.Fatal(err)
			}
			if brand.SiteURL != "https://www.qzweb.cloud" || brand.SiteName != "π星卡社" {
				t.Fatalf("unexpected main-store email identity: %+v", brand)
			}
		})
	}
}
