package application

import (
	"context"
	"errors"
	"net/http"
	"strings"

	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	"github.com/dujiao-next/internal/cache"
	"github.com/dujiao-next/internal/config"
)

const (
	// DomainUnavailableNotFound 表示域名未找到或分销站不可用。
	DomainUnavailableNotFound = "not_found"
)

// DomainResolver 将请求 Host 解析为主站或分销站上下文。
type DomainResolver struct {
	repo      resellercontract.DomainLookupRepository
	cfg       config.ResellerConfig
	mainHosts map[string]struct{}
}

// NewDomainResolver 创建域名解析器。
func NewDomainResolver(repo resellercontract.DomainLookupRepository, cfg config.ResellerConfig) *DomainResolver {
	mainHosts := make(map[string]struct{}, len(cfg.MainHosts))
	for _, host := range cfg.MainHosts {
		normalized := resellercontract.NormalizeHost(host)
		if normalized != "" {
			mainHosts[normalized] = struct{}{}
		}
	}
	return &DomainResolver{repo: repo, cfg: cfg, mainHosts: mainHosts}
}

// ResolveRequest 解析 HTTP 请求的租户上下文。
func (r *DomainResolver) ResolveRequest(ctx context.Context, req *http.Request) (resellercontract.TenantContext, error) {
	if r == nil {
		return resellercontract.MainTenantContext(""), nil
	}
	return r.ResolveHost(ctx, resellercontract.ResolveRequestHost(req, r.cfg))
}

// ResolveHost 按原始 Host 解析租户上下文。
func (r *DomainResolver) ResolveHost(ctx context.Context, rawHost string) (resellercontract.TenantContext, error) {
	host := resellercontract.NormalizeHost(rawHost)
	if r == nil || !r.cfg.Enabled {
		return resellercontract.MainTenantContext(host), nil
	}
	if host == "" {
		return resellercontract.MainTenantContext(host), nil
	}
	if _, ok := r.mainHosts[host]; ok {
		return resellercontract.MainTenantContext(host), nil
	}
	var cached cache.ResellerDomainCacheValue
	if hit, err := cache.GetResellerDomain(ctx, host, &cached); err == nil && hit {
		return resellercontract.ResellerTenantContext(host, cached.ResellerID, cached.ResellerUserID, cached.PrimaryDomain), nil
	}
	if hit, err := cache.GetResellerDomainNotFound(ctx, host); err == nil && hit {
		return resellercontract.UnavailableTenantContext(host, DomainUnavailableNotFound), nil
	}
	if r.repo == nil {
		return resellercontract.TenantContext{}, errors.New("reseller domain repository is nil")
	}
	domain, err := r.repo.FindActiveVerifiedDomain(host)
	if err != nil {
		return resellercontract.TenantContext{}, err
	}
	if domain == nil || domain.Profile == nil || domain.Profile.Status != resellerdomain.ProfileStatusActive {
		_ = cache.SetResellerDomainNotFound(ctx, host)
		return resellercontract.UnavailableTenantContext(host, DomainUnavailableNotFound), nil
	}
	primaryDomain := strings.TrimSpace(domain.Domain)
	value := cache.ResellerDomainCacheValue{
		ResellerID:         domain.ResellerID,
		ResellerUserID:     domain.Profile.UserID,
		Domain:             domain.Domain,
		PrimaryDomain:      primaryDomain,
		Status:             domain.Status,
		VerificationStatus: domain.VerificationStatus,
	}
	_ = cache.SetResellerDomain(ctx, host, value)
	return resellercontract.ResellerTenantContext(host, domain.ResellerID, domain.Profile.UserID, primaryDomain), nil
}
