package adminhttp

import (
	"context"
	"errors"

	reportingdomain "github.com/dujiao-next/internal/modules/reporting/domain"
	reportinghttp "github.com/dujiao-next/internal/modules/reporting/transport/http"
	resellermodule "github.com/dujiao-next/internal/modules/reseller/application"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// AdminOperationsService 是后台分销运营看板端点所需的最小用例接口。
type AdminOperationsService interface {
	GetOverview(ctx context.Context, input reportingdomain.Query) (*resellermodule.OperationsOverviewResponse, error)
	GetFinance(ctx context.Context, input reportingdomain.Query) (*resellermodule.OperationsFinanceResponse, error)
}

// AdminOperationsHandler 处理后台分销运营看板请求。
type AdminOperationsHandler struct {
	service AdminOperationsService
}

func NewAdminOperationsHandler(service AdminOperationsService) *AdminOperationsHandler {
	if service == nil {
		panic("reseller admin operations handler: service is nil")
	}
	return &AdminOperationsHandler{service: service}
}

// GetOverview 管理端分销运营总览。
func (h *AdminOperationsHandler) GetOverview(c *gin.Context) {
	input, err := reportinghttp.ParseQuery(c)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	data, err := h.service.GetOverview(c.Request.Context(), input)
	if err != nil {
		if errors.Is(err, reportingdomain.ErrRangeInvalid) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.dashboard_fetch_failed", err)
		return
	}
	response.Success(c, data)
}

// GetFinance 管理端分销运营财务聚合。
func (h *AdminOperationsHandler) GetFinance(c *gin.Context) {
	input, err := reportinghttp.ParseQuery(c)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	data, err := h.service.GetFinance(c.Request.Context(), input)
	if err != nil {
		if errors.Is(err, reportingdomain.ErrRangeInvalid) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.dashboard_fetch_failed", err)
		return
	}
	response.Success(c, data)
}
