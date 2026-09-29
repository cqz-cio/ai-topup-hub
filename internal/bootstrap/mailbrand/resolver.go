package mailbrandwiring

import (
	"context"
	"net/mail"
	"strings"

	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	"github.com/dujiao-next/internal/shared/mailbrand"
)

type mainBrandReader interface {
	GetSiteBrand() (settingsapp.SiteBrand, error)
}

type resellerSiteConfigReader interface {
	GetSiteConfigByResellerID(resellerID uint) (*resellerdomain.SiteConfig, error)
}

// Resolver converts a request/order tenant scope into a customer-visible mail
// brand. Reseller scopes never fall back to the global main-site settings.
type Resolver struct {
	main      mainBrandReader
	resellers resellerSiteConfigReader
}

func New(main mainBrandReader, resellers resellerSiteConfigReader) *Resolver {
	return &Resolver{main: main, resellers: resellers}
}

func (r *Resolver) ResolveEmailBrand(ctx context.Context, scope mailbrand.Scope) (mailbrand.Brand, error) {
	scope = scopeFromContext(ctx, scope)
	if scope.ResellerID != nil {
		return r.resolveReseller(scope)
	}
	if r == nil || r.main == nil {
		return mailbrand.Brand{}, nil
	}
	brand, err := r.main.GetSiteBrand()
	if err != nil {
		return mailbrand.Brand{}, err
	}
	return mailbrand.Brand{
		SiteName: strings.TrimSpace(brand.SiteName),
		SiteURL:  strings.TrimRight(strings.TrimSpace(brand.SiteURL), "/"),
	}, nil
}

func scopeFromContext(ctx context.Context, scope mailbrand.Scope) mailbrand.Scope {
	if scope.ResellerID != nil || strings.TrimSpace(scope.Host) != "" {
		return scope
	}
	tenant, ok := resellercontract.TenantFromContext(ctx)
	if !ok || !tenant.IsReseller() {
		return scope
	}
	host := tenant.Host
	if strings.TrimSpace(host) == "" {
		host = tenant.PrimaryDomain
	}
	return mailbrand.Scope{
		ResellerID: tenant.ResellerID,
		Host:       host,
	}
}

func (r *Resolver) resolveReseller(scope mailbrand.Scope) (mailbrand.Brand, error) {
	host := resellercontract.NormalizeHost(scope.Host)
	brand := mailbrand.ResellerFallback(host)
	if r == nil || r.resellers == nil || scope.ResellerID == nil || *scope.ResellerID == 0 {
		return brand, nil
	}
	cfg, err := r.resellers.GetSiteConfigByResellerID(*scope.ResellerID)
	if err != nil {
		return brand, err
	}
	if cfg == nil {
		return brand, nil
	}
	if siteName := strings.TrimSpace(cfg.SiteName); siteName != "" {
		brand.SiteName = siteName
		brand.FromName = siteName
	}
	if raw, ok := cfg.SupportJSON["email"].(string); ok {
		brand.ReplyTo = normalizeReplyTo(raw)
	}
	return brand, nil
}

func normalizeReplyTo(raw string) string {
	addr, err := mail.ParseAddress(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return addr.Address
}
