package channelclienthttp

import (
	"errors"

	channelclientapp "github.com/dujiao-next/internal/modules/channelclient/application"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// AdminService 是后台渠道客户端端口。
type AdminService interface {
	ListChannelClientDetails() ([]channelclientapp.ClientDetail, error)
	CreateChannelClient(name, channelType, description, botToken, callbackURL string) (*channelclientapp.ClientDetail, error)
	GetChannelClientDetail(id uint) (*channelclientapp.ClientDetail, error)
	UpdateChannelClientStatus(id uint, status int) error
	UpdateChannelClient(id uint, name, description string, botToken *string, callbackURL *string) (*channelclientapp.ClientDetail, error)
	ResetChannelClientSecret(id uint) (*channelclientapp.ClientDetail, error)
	DeleteChannelClient(id uint) error
}

type createRequest struct {
	Name        string `json:"name" binding:"required"`
	ChannelType string `json:"channel_type" binding:"required"`
	Description string `json:"description"`
	BotToken    string `json:"bot_token"`
	CallbackURL string `json:"callback_url"`
}

type updateStatusRequest struct {
	Status int `json:"status" binding:"oneof=0 1"`
}

type updateRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	BotToken    *string `json:"bot_token"`    // nil = 不修改, "" = 清空, "xxx" = 设置
	CallbackURL *string `json:"callback_url"` // nil = 不修改, "" = 清空
}

// AdminHandler 处理后台渠道客户端请求。
type AdminHandler struct {
	clients AdminService
}

func NewAdminHandler(clients AdminService) *AdminHandler {
	if clients == nil {
		panic("channelclient admin handler: clients is nil")
	}
	return &AdminHandler{clients: clients}
}

// ListChannelClients 获取渠道客户端列表（含解密 secret）
func (h *AdminHandler) ListChannelClients(c *gin.Context) {
	clients, err := h.clients.ListChannelClientDetails()
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.channel_clients_fetch_failed", err)
		return
	}
	response.Success(c, clients)
}

// CreateChannelClient 创建渠道客户端
func (h *AdminHandler) CreateChannelClient(c *gin.Context) {
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	result, err := h.clients.CreateChannelClient(req.Name, req.ChannelType, req.Description, req.BotToken, req.CallbackURL)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.channel_client_create_failed", err)
		return
	}

	response.Success(c, result)
}

// GetChannelClient 获取渠道客户端详情（含解密 secret）
func (h *AdminHandler) GetChannelClient(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}

	detail, err := h.clients.GetChannelClientDetail(id)
	if err != nil {
		if errors.Is(err, channelclientapp.ErrNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.not_found", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.channel_client_fetch_failed", err)
		return
	}
	response.Success(c, detail)
}

// UpdateChannelClientStatus 更新渠道客户端状态
func (h *AdminHandler) UpdateChannelClientStatus(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}

	var req updateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	if err := h.clients.UpdateChannelClientStatus(id, req.Status); err != nil {
		if errors.Is(err, channelclientapp.ErrNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.not_found", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.channel_client_update_failed", err)
		return
	}

	response.Success(c, nil)
}

// UpdateChannelClient 更新渠道客户端信息
func (h *AdminHandler) UpdateChannelClient(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}

	var req updateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	result, err := h.clients.UpdateChannelClient(id, req.Name, req.Description, req.BotToken, req.CallbackURL)
	if err != nil {
		if errors.Is(err, channelclientapp.ErrNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.not_found", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.channel_client_update_failed", err)
		return
	}

	response.Success(c, result)
}

// ResetChannelClientSecret 重置渠道客户端 Secret
func (h *AdminHandler) ResetChannelClientSecret(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}

	result, err := h.clients.ResetChannelClientSecret(id)
	if err != nil {
		if errors.Is(err, channelclientapp.ErrNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.not_found", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.channel_client_reset_secret_failed", err)
		return
	}

	response.Success(c, result)
}

// DeleteChannelClient 删除渠道客户端
func (h *AdminHandler) DeleteChannelClient(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}

	if err := h.clients.DeleteChannelClient(id); err != nil {
		if errors.Is(err, channelclientapp.ErrNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.not_found", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.channel_client_delete_failed", err)
		return
	}

	response.Success(c, nil)
}
