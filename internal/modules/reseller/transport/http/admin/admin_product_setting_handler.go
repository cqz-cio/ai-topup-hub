package adminhttp

import (
	"strings"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	auditlogapp "github.com/dujiao-next/internal/modules/auditlog/application"
	resellermodule "github.com/dujiao-next/internal/modules/reseller/application"
	dto "github.com/dujiao-next/internal/modules/reseller/transport/http/presenter"
	transportshared "github.com/dujiao-next/internal/modules/reseller/transport/http/shared"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/shared/jsonmap"

	"github.com/gin-gonic/gin"
)

// AdminProductSettingService 是后台分销商品配置端点所需的最小用例接口。
type AdminProductSettingService interface {
	ListAdminSettings(input resellermodule.ProductSettingAdminListInput) ([]resellerdomain.ProductSetting, int64, error)
	GetAdminProductSetting(resellerID, productID uint) (*resellermodule.ProductSettingDetail, error)
	PreviewAdminProductSettings(resellerID, productID uint, input resellermodule.ProductSettingSaveInput) ([]resellermodule.ProductSettingPreviewItem, error)
	SaveAdminProductSettings(resellerID, productID uint, input resellermodule.ProductSettingSaveInput) (*resellermodule.ProductSettingDetail, error)
	ResetAdminProductSetting(resellerID, productID, skuID uint) error
}

// AdminProductSettingHandler 处理后台分销商品配置请求。
type AdminProductSettingHandler struct {
	service AdminProductSettingService
	audit   AuditRecorder
}

func NewAdminProductSettingHandler(service AdminProductSettingService, audit AuditRecorder) *AdminProductSettingHandler {
	if service == nil {
		panic("reseller admin product setting handler: service is nil")
	}
	return &AdminProductSettingHandler{service: service, audit: audit}
}

// ListProductSettings 管理端分销商品配置列表。
func (h *AdminProductSettingHandler) ListProductSettings(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)
	resellerID, _ := ginutil.ParseQueryUint(c.Query("reseller_id"), false)
	userID, _ := ginutil.ParseQueryUint(c.Query("user_id"), false)
	productID, _ := ginutil.ParseQueryUint(c.Query("product_id"), false)
	rows, total, err := h.service.ListAdminSettings(resellermodule.ProductSettingAdminListInput{
		Page:        page,
		PageSize:    pageSize,
		ResellerID:  resellerID,
		UserID:      userID,
		ProductID:   productID,
		Keyword:     strings.TrimSpace(c.Query("keyword")),
		PricingMode: strings.TrimSpace(c.Query("pricing_mode")),
		Configured:  strings.TrimSpace(c.Query("configured")),
		Listed:      strings.TrimSpace(c.Query("listed")),
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.SuccessWithPage(c, dto.NewAdminResellerProductSettingRespList(rows), response.BuildPagination(page, pageSize, total))
}

// GetProductSetting 管理端查看某分销商的商品配置详情。
func (h *AdminProductSettingHandler) GetProductSetting(c *gin.Context) {
	resellerID, productID, ok := parseAdminProductSettingParams(c)
	if !ok {
		return
	}
	detail, err := h.service.GetAdminProductSetting(resellerID, productID)
	if err != nil {
		respondAdminProductSettingError(c, err)
		return
	}
	response.Success(c, dto.NewResellerProductSettingDetailResp(transportshared.DetailDTOInput(detail)))
}

// UpdateProductSettings 管理端保存某分销商的商品配置。
func (h *AdminProductSettingHandler) UpdateProductSettings(c *gin.Context) {
	resellerID, productID, ok := parseAdminProductSettingParams(c)
	if !ok {
		return
	}
	var req transportshared.ProductSettingsUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	input, err := req.ToInput()
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	detail, err := h.service.SaveAdminProductSettings(resellerID, productID, input)
	if err != nil {
		respondAdminProductSettingError(c, err)
		return
	}
	h.recordAudit(c, "reseller_product_setting_save", "/admin/resellers/product-settings/:reseller_id/:product_id", "PUT", jsonmap.JSON{
		"reseller_id":    resellerID,
		"product_id":     productID,
		"sku_ids":        collectProductSettingRequestSKUIDs(req.Settings),
		"settings_count": len(req.Settings),
	})
	response.Success(c, dto.NewResellerProductSettingDetailResp(transportshared.DetailDTOInput(detail)))
}

// PreviewProductSettings 管理端计算某分销商拟用定价规则的预计生效价与校验结果（不落库）。
func (h *AdminProductSettingHandler) PreviewProductSettings(c *gin.Context) {
	resellerID, productID, ok := parseAdminProductSettingParams(c)
	if !ok {
		return
	}
	var req transportshared.ProductSettingsUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	input, err := req.ToInput()
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	items, err := h.service.PreviewAdminProductSettings(resellerID, productID, input)
	if err != nil {
		respondAdminProductSettingError(c, err)
		return
	}
	response.Success(c, dto.NewResellerProductSettingPreviewResp(transportshared.PreviewDTOInput(items)))
}

// ResetProductSetting 管理端删除某分销商的商品级或 SKU 级配置。
func (h *AdminProductSettingHandler) ResetProductSetting(c *gin.Context) {
	resellerID, productID, ok := parseAdminProductSettingParams(c)
	if !ok {
		return
	}
	skuID, err := ginutil.ParseQueryUint(c.Query("sku_id"), false)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	if err := h.service.ResetAdminProductSetting(resellerID, productID, skuID); err != nil {
		respondAdminProductSettingError(c, err)
		return
	}
	h.recordAudit(c, "reseller_product_setting_reset", "/admin/resellers/product-settings/:reseller_id/:product_id", "DELETE", jsonmap.JSON{
		"reseller_id": resellerID,
		"product_id":  productID,
		"sku_ids":     []uint{skuID},
	})
	response.Success(c, gin.H{"deleted": true})
}

func parseAdminProductSettingParams(c *gin.Context) (uint, uint, bool) {
	resellerID, err := ginutil.ParseParamUint(c, "reseller_id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return 0, 0, false
	}
	productID, err := ginutil.ParseParamUint(c, "product_id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return 0, 0, false
	}
	return resellerID, productID, true
}

func collectProductSettingRequestSKUIDs(settings []transportshared.ProductSettingRequest) []uint {
	out := make([]uint, 0, len(settings))
	for _, setting := range settings {
		out = append(out, setting.SKUID)
	}
	return out
}

func (h *AdminProductSettingHandler) recordAudit(c *gin.Context, action, object, method string, detail jsonmap.JSON) {
	if h == nil || h.audit == nil {
		return
	}
	_ = h.audit.Record(auditlogapp.AuthzRecord{
		OperatorAdminID:  c.GetUint("admin_id"),
		OperatorUsername: c.GetString("username"),
		Action:           action,
		Object:           object,
		Method:           method,
		RequestID:        strings.TrimSpace(c.GetString("request_id")),
		Detail:           detail,
	})
}
