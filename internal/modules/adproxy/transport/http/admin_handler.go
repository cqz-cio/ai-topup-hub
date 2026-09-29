package adproxyhttp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	adproxydomain "github.com/dujiao-next/internal/modules/adproxy/domain"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// AdminService 是后台广告代理端口。
type AdminService interface {
	RenderSlot(ctx context.Context, slotCode string, params map[string]string) (*adproxydomain.RenderResponse, error)
	ReportImpression(ctx context.Context, payload json.RawMessage) error
}

// AdminHandler 处理后台广告代理请求。
type AdminHandler struct {
	ads AdminService
}

func NewAdminHandler(ads AdminService) *AdminHandler {
	if ads == nil {
		panic("adproxy admin handler: ads is nil")
	}
	return &AdminHandler{ads: ads}
}

// GetAdRender 代理广告位渲染请求到 ad-system
func (h *AdminHandler) GetAdRender(c *gin.Context) {
	slotCode := c.Param("slotCode")
	if slotCode == "" {
		response.Error(c, http.StatusBadRequest, "slot_code is required")
		return
	}

	params := make(map[string]string)
	for _, key := range []string{"tenant", "client", "locale"} {
		if v := c.Query(key); v != "" {
			params[key] = v
		}
	}

	data, err := h.ads.RenderSlot(c.Request.Context(), slotCode, params)
	if err != nil {
		// 广告请求失败时静默返回空数据，不影响主业务
		response.Success(c, nil)
		return
	}

	response.Success(c, data)
}

// PostAdImpression 代理广告曝光上报到 ad-system
func (h *AdminHandler) PostAdImpression(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.ads.ReportImpression(c.Request.Context(), json.RawMessage(body)); err != nil {
		// 曝光上报失败不影响主业务
		response.Success(c, nil)
		return
	}

	response.Success(c, nil)
}
