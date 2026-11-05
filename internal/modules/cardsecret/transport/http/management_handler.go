package cardsecrethttp

import (
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/modules/cardsecret/contract"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/gin-gonic/gin"
)

var cardIdentity = regexp.MustCompile(`^[A-Za-z0-9_-]{16,80}$`)

func (h *AdminHandler) manageCards(c *gin.Context, operation string) {
	ginutil.PrivateNoStore(c)
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	if h.management == nil {
		response.Error(c, response.CodeBadRequest, "PAIPLUS 卡密管理尚未配置，请联系站点管理员。")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16384)
	var input contract.ManagementInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, response.CodeBadRequest, "卡密管理请求格式不正确。")
		return
	}
	if len(input.Query) > 256 || len(input.Reason) > 2000 || len(input.Status) > 30 || (operation != "cards" && !cardIdentity.MatchString(input.CardID)) {
		response.Error(c, response.CodeBadRequest, "卡密管理参数不正确。")
		return
	}
	if operation == "revoke" && (!input.Confirmed || strings.TrimSpace(input.Reason) == "" || !cardIdentity.MatchString(input.RequestID)) {
		response.Error(c, response.CodeBadRequest, "请填写销毁原因，并确认永久停用这张本站卡密。")
		return
	}
	result, err := h.management.Call(c.Request.Context(), operation, input, adminID)
	if operation != "cards" {
		logger.Infow("admin_paiplus_card_access", "admin_id", adminID, "card_id", input.CardID, "action", operation, "success", err == nil)
	}
	if err != nil {
		var known *contract.ManagementError
		if errors.As(err, &known) {
			response.Error(c, response.CodeBadRequest, known.Message)
			return
		}
		response.Error(c, response.CodeBadRequest, "卡密管理服务暂不可用，操作未确认成功，请刷新详情核对。")
		return
	}
	response.Success(c, result)
}

func (h *AdminHandler) QueryPAIPLUSCards(c *gin.Context) { h.manageCards(c, "cards") }
func (h *AdminHandler) RevealPAIPLUSCard(c *gin.Context) { h.manageCards(c, "detail") }
func (h *AdminHandler) CheckPAIPLUSCard(c *gin.Context)  { h.manageCards(c, "check") }
func (h *AdminHandler) RevokePAIPLUSCard(c *gin.Context) { h.manageCards(c, "revoke") }
