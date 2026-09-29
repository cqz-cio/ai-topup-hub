package presenter

import (
	"time"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	resellermodule "github.com/dujiao-next/internal/modules/reseller/contract"
)

type AdminResellerProfileUserResp struct {
	ID          uint   `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

type AdminResellerProfileResp struct {
	ID                   uint                          `json:"id"`
	UserID               uint                          `json:"user_id"`
	Status               string                        `json:"status"`
	ApplyReason          string                        `json:"apply_reason,omitempty"`
	RejectReason         string                        `json:"reject_reason,omitempty"`
	DefaultMarkupPercent string                        `json:"default_markup_percent"`
	MaxMarkupPercent     string                        `json:"max_markup_percent"`
	SettlementStatus     string                        `json:"settlement_status"`
	ReviewedBy           *uint                         `json:"reviewed_by,omitempty"`
	ReviewedAt           *time.Time                    `json:"reviewed_at,omitempty"`
	CreatedAt            time.Time                     `json:"created_at"`
	UpdatedAt            time.Time                     `json:"updated_at"`
	User                 *AdminResellerProfileUserResp `json:"user,omitempty"`
}

type AdminResellerProductSummaryResp struct {
	ConfiguredProducts int64 `json:"configured_products"`
	HiddenProducts     int64 `json:"hidden_products"`
	SKUOverrides       int64 `json:"sku_overrides"`
	PricingOverrides   int64 `json:"pricing_overrides"`
}

type AdminResellerFinanceSummaryResp struct {
	Balances            []ResellerBalanceResp `json:"balances"`
	RecentLedgerCount   int                   `json:"recent_ledger_count"`
	RecentWithdrawCount int                   `json:"recent_withdraw_count"`
}

type AdminResellerProfileDetailResp struct {
	Profile             *AdminResellerProfileResp       `json:"profile"`
	Domains             []ResellerDomainResp            `json:"domains"`
	SiteConfig          *AdminResellerSiteConfigResp    `json:"site_config,omitempty"`
	ProductSummary      AdminResellerProductSummaryResp `json:"product_summary"`
	FinanceSummary      AdminResellerFinanceSummaryResp `json:"finance_summary"`
	RecentOrders        []ResellerOrderResp             `json:"recent_orders"`
	RecentLedgerEntries []ResellerLedgerResp            `json:"recent_ledger_entries"`
	RecentWithdraws     []ResellerWithdrawResp          `json:"recent_withdraws"`
}

func NewAdminResellerProfileResp(profile *resellerdomain.Profile) *AdminResellerProfileResp {
	if profile == nil {
		return nil
	}
	resp := &AdminResellerProfileResp{
		ID:                   profile.ID,
		UserID:               profile.UserID,
		Status:               profile.Status,
		ApplyReason:          profile.ApplyReason,
		RejectReason:         profile.RejectReason,
		DefaultMarkupPercent: profile.DefaultMarkupPercent.String(),
		MaxMarkupPercent:     profile.MaxMarkupPercent.String(),
		SettlementStatus:     profile.SettlementStatus,
		ReviewedBy:           profile.ReviewedBy,
		ReviewedAt:           profile.ReviewedAt,
		CreatedAt:            profile.CreatedAt,
		UpdatedAt:            profile.UpdatedAt,
	}
	if profile.User != nil {
		resp.User = &AdminResellerProfileUserResp{
			ID:          profile.User.ID,
			Email:       profile.User.Email,
			DisplayName: profile.User.DisplayName,
		}
	}
	return resp
}

func NewAdminResellerProfileDetailResp(
	profile *resellerdomain.Profile,
	domains []resellerdomain.Domain,
	siteConfig *resellerdomain.SiteConfig,
	productSummary resellermodule.ProductSettingSummary,
	balances []resellerdomain.BalanceAccount,
	recentOrders []resellermodule.OrderListItem,
	recentLedgerEntries []resellerdomain.LedgerEntry,
	recentWithdraws []resellerdomain.WithdrawRequest,
) AdminResellerProfileDetailResp {
	var siteConfigResp *AdminResellerSiteConfigResp
	if siteConfig != nil {
		row := NewAdminResellerSiteConfigResp(siteConfig)
		siteConfigResp = &row
	}
	return AdminResellerProfileDetailResp{
		Profile:    NewAdminResellerProfileResp(profile),
		Domains:    NewResellerDomainRespList(domains),
		SiteConfig: siteConfigResp,
		ProductSummary: AdminResellerProductSummaryResp{
			ConfiguredProducts: productSummary.ConfiguredProducts,
			HiddenProducts:     productSummary.HiddenProducts,
			SKUOverrides:       productSummary.SKUOverrides,
			PricingOverrides:   productSummary.PricingOverrides,
		},
		FinanceSummary: AdminResellerFinanceSummaryResp{
			Balances:            NewResellerBalanceRespList(balances),
			RecentLedgerCount:   len(recentLedgerEntries),
			RecentWithdrawCount: len(recentWithdraws),
		},
		RecentOrders:        NewResellerOrderRespList(recentOrders),
		RecentLedgerEntries: NewResellerLedgerRespList(recentLedgerEntries),
		RecentWithdraws:     NewResellerWithdrawRespList(recentWithdraws),
	}
}
