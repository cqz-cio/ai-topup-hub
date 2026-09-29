package adminhttp

import (
	"errors"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"

	resellermodule "github.com/dujiao-next/internal/modules/reseller/contract"
	dto "github.com/dujiao-next/internal/modules/reseller/transport/http/presenter"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// ProfileDetailDirectory 是管理端分销商运营详情所需的只读目录端口。
type ProfileDetailDirectory interface {
	GetProfileByID(id uint) (*resellerdomain.Profile, error)
	ListDomainsByResellerID(resellerID uint) ([]resellerdomain.Domain, error)
	GetSiteConfigByResellerID(resellerID uint) (*resellerdomain.SiteConfig, error)
}

// ProductSettingSummarizer 是管理端商品配置汇总端口。
type ProductSettingSummarizer interface {
	SummarizeAdminSettings(resellerID uint) (resellermodule.ProductSettingSummary, error)
}

// OrderAdminLister 是管理端分销订单只读列表端口。
type OrderAdminLister interface {
	ListAdminOrders(resellerID uint, input resellermodule.OrderListInput) ([]resellermodule.OrderListItem, int64, error)
}

// AdminProfileDetailHandler 处理后台分销商运营详情聚合。
type AdminProfileDetailHandler struct {
	directory ProfileDetailDirectory
	products  ProductSettingSummarizer
	finance   AdminFinanceQueryService
	orders    OrderAdminLister
}

func NewAdminProfileDetailHandler(
	directory ProfileDetailDirectory,
	products ProductSettingSummarizer,
	finance AdminFinanceQueryService,
	orders OrderAdminLister,
) *AdminProfileDetailHandler {
	if directory == nil {
		panic("reseller admin profile detail handler: directory is nil")
	}
	return &AdminProfileDetailHandler{
		directory: directory,
		products:  products,
		finance:   finance,
		orders:    orders,
	}
}

// GetProfileDetail 管理端分销商运营详情。
func (h *AdminProfileDetailHandler) GetProfileDetail(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	profile, err := h.directory.GetProfileByID(id)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	if profile == nil {
		ginutil.RespondError(c, response.CodeNotFound, "error.bad_request", nil)
		return
	}
	domains, err := h.directory.ListDomainsByResellerID(id)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	siteConfig, err := h.directory.GetSiteConfigByResellerID(id)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	productSummary := resellermodule.ProductSettingSummary{}
	if h.products != nil {
		productSummary, err = h.products.SummarizeAdminSettings(id)
		if err != nil {
			ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
			return
		}
	}
	balances := make([]resellerdomain.BalanceAccount, 0)
	recentLedgerEntries := make([]resellerdomain.LedgerEntry, 0)
	recentWithdraws := make([]resellerdomain.WithdrawRequest, 0)
	if h.finance != nil {
		balances, _, err = h.finance.ListAdminBalanceAccounts(resellermodule.AdminBalanceAccountListFilter{
			Page:       1,
			PageSize:   20,
			ResellerID: id,
		})
		if err != nil {
			ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
			return
		}
		recentLedgerEntries, _, err = h.finance.ListAdminLedgerEntries(resellermodule.AdminLedgerListFilter{
			Page:       1,
			PageSize:   10,
			ResellerID: id,
		})
		if err != nil {
			ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
			return
		}
		recentWithdraws, _, err = h.finance.ListAdminWithdrawRequests(resellermodule.AdminWithdrawListFilter{
			Page:       1,
			PageSize:   10,
			ResellerID: id,
		})
		if err != nil {
			ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
			return
		}
	}
	recentOrders := make([]resellermodule.OrderListItem, 0)
	if h.orders != nil {
		recentOrders, _, err = h.orders.ListAdminOrders(id, resellermodule.OrderListInput{
			Page:     1,
			PageSize: 10,
		})
		if err != nil {
			respondAdminProfileDetailError(c, err)
			return
		}
	}
	response.Success(c, dto.NewAdminResellerProfileDetailResp(profile, domains, siteConfig, productSummary, balances, recentOrders, recentLedgerEntries, recentWithdraws))
}

func respondAdminProfileDetailError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, productcontract.ErrNotFound):
		ginutil.RespondError(c, response.CodeNotFound, "error.bad_request", nil)
	default:
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
	}
}
