package mailbrand

import (
	"context"
	"strings"
)

// Brand describes the customer-visible identity used by one email.
type Brand struct {
	SiteName string
	SiteURL  string
	FromName string
	ReplyTo  string
}

// Scope identifies the storefront whose brand must be used.
// A nil ResellerID represents the main storefront.
type Scope struct {
	ResellerID *uint
	Host       string
}

// Resolver resolves a safe email brand for a storefront scope.
type Resolver interface {
	ResolveEmailBrand(ctx context.Context, scope Scope) (Brand, error)
}

// ResellerFallback builds a tenant-safe brand without consulting global
// settings. It is used when a reseller has not configured a site name yet.
func ResellerFallback(host string) Brand {
	normalized := strings.TrimSpace(host)
	siteURL := ""
	if normalized != "" {
		siteURL = "https://" + normalized
	}
	return Brand{
		SiteName: normalized,
		SiteURL:  siteURL,
		FromName: normalized,
	}
}
