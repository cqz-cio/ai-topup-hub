package dashboardhttp

import (
	"context"
	"errors"

	dashboardapp "github.com/dujiao-next/internal/modules/dashboard/application"
	dashboardcontract "github.com/dujiao-next/internal/modules/dashboard/contract"
	reportingdomain "github.com/dujiao-next/internal/modules/reporting/domain"
	reportinghttp "github.com/dujiao-next/internal/modules/reporting/transport/http"
	settingsstorefront "github.com/dujiao-next/internal/modules/settings/schema/storefront"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// Reader is the minimal dashboard use-case surface consumed by HTTP.
type Reader interface {
	GetOverview(ctx context.Context, input reportingdomain.Query) (*dashboardapp.OverviewResponse, error)
	GetTrends(ctx context.Context, input reportingdomain.Query) (*dashboardapp.TrendResponse, error)
	GetRankings(ctx context.Context, input reportingdomain.Query) (*dashboardapp.RankingsResponse, error)
	LoadDashboardAlertSetting() settingsstorefront.DashboardAlertSetting
	GetInventoryAlertItems(ctx context.Context, lowStockThreshold int64) ([]dashboardcontract.InventoryAlertRow, error)
}

type AdminHandler struct {
	reader Reader
}

func NewAdminHandler(reader Reader) *AdminHandler {
	return &AdminHandler{reader: reader}
}

func (h *AdminHandler) GetOverview(c *gin.Context) {
	input, ok := parseQuery(c)
	if !ok {
		return
	}
	if h == nil || h.reader == nil {
		respondFetchError(c, nil)
		return
	}
	data, err := h.reader.GetOverview(c.Request.Context(), input)
	if err != nil {
		respondFetchError(c, err)
		return
	}
	response.Success(c, data)
}

func (h *AdminHandler) GetTrends(c *gin.Context) {
	input, ok := parseQuery(c)
	if !ok {
		return
	}
	if h == nil || h.reader == nil {
		respondFetchError(c, nil)
		return
	}
	data, err := h.reader.GetTrends(c.Request.Context(), input)
	if err != nil {
		respondFetchError(c, err)
		return
	}
	response.Success(c, data)
}

func (h *AdminHandler) GetRankings(c *gin.Context) {
	input, ok := parseQuery(c)
	if !ok {
		return
	}
	if h == nil || h.reader == nil {
		respondFetchError(c, nil)
		return
	}
	data, err := h.reader.GetRankings(c.Request.Context(), input)
	if err != nil {
		respondFetchError(c, err)
		return
	}
	response.Success(c, data)
}

func (h *AdminHandler) GetInventoryAlerts(c *gin.Context) {
	if h == nil || h.reader == nil {
		respondFetchError(c, nil)
		return
	}
	setting := h.reader.LoadDashboardAlertSetting()
	items, err := h.reader.GetInventoryAlertItems(c.Request.Context(), setting.LowStockThreshold)
	if err != nil {
		respondFetchError(c, err)
		return
	}
	response.Success(c, mapInventoryAlerts(items))
}

func parseQuery(c *gin.Context) (reportingdomain.Query, bool) {
	input, err := reportinghttp.ParseQuery(c)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return reportingdomain.Query{}, false
	}
	return input, true
}

func respondFetchError(c *gin.Context, err error) {
	if errors.Is(err, reportingdomain.ErrRangeInvalid) {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	ginutil.RespondError(c, response.CodeInternal, "error.dashboard_fetch_failed", err)
}
