package userhttp

import (
	"strings"

	resellermodule "github.com/dujiao-next/internal/modules/reseller/application"
	dto "github.com/dujiao-next/internal/modules/reseller/transport/http/presenter"
	transportshared "github.com/dujiao-next/internal/modules/reseller/transport/http/shared"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// UserProductSettingService 是用户中心分销商品配置端点所需的最小用例接口。
type UserProductSettingService interface {
	ListUserProductSettings(userID uint, input resellermodule.ProductSettingUserListInput) ([]resellermodule.ProductSettingListRow, int64, error)
	GetUserProductSetting(userID, productID uint) (*resellermodule.ProductSettingDetail, error)
	PreviewUserProductSettings(userID, productID uint, input resellermodule.ProductSettingSaveInput) ([]resellermodule.ProductSettingPreviewItem, error)
	SaveUserProductSettings(userID, productID uint, input resellermodule.ProductSettingSaveInput) (*resellermodule.ProductSettingDetail, error)
	ResetUserProductSetting(userID, productID, skuID uint) error
}

// UserProductSettingHandler 处理用户中心分销商品配置请求。
type UserProductSettingHandler struct {
	service UserProductSettingService
}

func NewUserProductSettingHandler(service UserProductSettingService) *UserProductSettingHandler {
	if service == nil {
		panic("reseller user product setting handler: service is nil")
	}
	return &UserProductSettingHandler{service: service}
}

// ListProductSettings 查询当前用户可配置的分销商品。
func (h *UserProductSettingHandler) ListProductSettings(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	page, pageSize := ginutil.ParsePagination(c)
	categoryID, _ := ginutil.ParseQueryUint(c.Query("category_id"), false)
	rows, total, err := h.service.ListUserProductSettings(uid, resellermodule.ProductSettingUserListInput{
		Page:       page,
		PageSize:   pageSize,
		Keyword:    strings.TrimSpace(c.Query("keyword")),
		CategoryID: categoryID,
		Configured: strings.TrimSpace(c.Query("configured")),
		Listed:     strings.TrimSpace(c.Query("listed")),
	})
	if err != nil {
		respondUserProductSettingError(c, err, "error.user_fetch_failed")
		return
	}
	response.SuccessWithPage(c, dto.NewResellerProductSettingListResp(transportshared.ListDTOInput(rows)), response.BuildPagination(page, pageSize, total))
}

// GetProductSetting 获取当前用户的单个商品分销配置详情。
func (h *UserProductSettingHandler) GetProductSetting(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	productID, err := ginutil.ParseParamUint(c, "product_id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	detail, err := h.service.GetUserProductSetting(uid, productID)
	if err != nil {
		respondUserProductSettingError(c, err, "error.user_fetch_failed")
		return
	}
	response.Success(c, dto.NewResellerProductSettingDetailResp(transportshared.DetailDTOInput(detail)))
}

// UpdateProductSettings 保存当前用户的商品级或 SKU 级分销配置。
func (h *UserProductSettingHandler) UpdateProductSettings(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	productID, err := ginutil.ParseParamUint(c, "product_id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
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
	detail, err := h.service.SaveUserProductSettings(uid, productID, input)
	if err != nil {
		respondUserProductSettingError(c, err, "error.save_failed")
		return
	}
	response.Success(c, dto.NewResellerProductSettingDetailResp(transportshared.DetailDTOInput(detail)))
}

// PreviewProductSettings 计算当前用户拟用定价规则的预计生效价与校验结果（不落库）。
func (h *UserProductSettingHandler) PreviewProductSettings(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	productID, err := ginutil.ParseParamUint(c, "product_id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
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
	items, err := h.service.PreviewUserProductSettings(uid, productID, input)
	if err != nil {
		respondUserProductSettingError(c, err, "error.user_fetch_failed")
		return
	}
	response.Success(c, dto.NewResellerProductSettingPreviewResp(transportshared.PreviewDTOInput(items)))
}

// ResetProductSetting 删除当前用户的商品级或 SKU 级分销配置。
func (h *UserProductSettingHandler) ResetProductSetting(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	productID, err := ginutil.ParseParamUint(c, "product_id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	skuID, err := ginutil.ParseQueryUint(c.Query("sku_id"), false)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	if err := h.service.ResetUserProductSetting(uid, productID, skuID); err != nil {
		respondUserProductSettingError(c, err, "error.save_failed")
		return
	}
	response.Success(c, gin.H{"deleted": true})
}
