package broadcasthttp

import (
	"context"
	"errors"
	"strings"

	broadcastapp "github.com/dujiao-next/internal/modules/telegram/broadcast/application"
	broadcastdomain "github.com/dujiao-next/internal/modules/telegram/broadcast/domain"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/shared/jsonmap"

	"github.com/gin-gonic/gin"
)

// AdminService is the transport-facing broadcast application contract.
type AdminService interface {
	ListBroadcasts(broadcastapp.ListInput) ([]broadcastdomain.Broadcast, int64, error)
	GetBroadcast(uint) (*broadcastdomain.Broadcast, error)
	CreateBroadcast(context.Context, broadcastapp.CreateInput) (*broadcastdomain.Broadcast, error)
	ListTelegramUsers(broadcastapp.UserQuery) ([]broadcastapp.UserItem, int64, error)
}

type createBroadcastRequest struct {
	Title          string       `json:"title" binding:"required"`
	RecipientType  string       `json:"recipient_type" binding:"required"`
	UserIDs        []uint       `json:"user_ids"`
	Filters        jsonmap.JSON `json:"filters"`
	AttachmentURL  string       `json:"attachment_url"`
	AttachmentName string       `json:"attachment_name"`
	MessageHTML    string       `json:"message_html" binding:"required"`
}

// AdminHandler handles administrative Telegram broadcast requests.
type AdminHandler struct {
	broadcasts AdminService
}

func NewAdminHandler(broadcasts AdminService) *AdminHandler {
	if broadcasts == nil {
		panic("telegram broadcast handler: broadcasts is nil")
	}
	return &AdminHandler{broadcasts: broadcasts}
}

// List returns Telegram broadcast jobs.
func (h *AdminHandler) List(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)

	items, total, err := h.broadcasts.ListBroadcasts(broadcastapp.ListInput{
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.bad_request", err)
		return
	}
	response.SuccessWithPage(c, items, response.BuildPagination(page, pageSize, total))
}

// Create persists and dispatches a Telegram broadcast job.
func (h *AdminHandler) Create(c *gin.Context) {
	var req createBroadcastRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	result, err := h.broadcasts.CreateBroadcast(c.Request.Context(), broadcastapp.CreateInput{
		Title:          req.Title,
		RecipientType:  req.RecipientType,
		UserIDs:        req.UserIDs,
		Filters:        req.Filters,
		AttachmentURL:  req.AttachmentURL,
		AttachmentName: req.AttachmentName,
		MessageHTML:    req.MessageHTML,
	})
	if err != nil {
		if errors.Is(err, broadcastapp.ErrInvalid) ||
			errors.Is(err, broadcastapp.ErrNoRecipients) ||
			errors.Is(err, broadcastapp.ErrTokenUnavailable) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.bad_request", err)
		return
	}
	response.Success(c, result)
}

// Get returns one Telegram broadcast job.
func (h *AdminHandler) Get(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", errors.New("invalid broadcast id"))
		return
	}
	broadcast, err := h.broadcasts.GetBroadcast(id)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.bad_request", err)
		return
	}
	if broadcast == nil {
		ginutil.RespondError(c, response.CodeNotFound, "error.not_found", broadcastapp.ErrNotFound)
		return
	}
	response.Success(c, broadcast)
}

// ListUsers returns Telegram users selectable as broadcast recipients.
func (h *AdminHandler) ListUsers(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)

	createdFrom, createdTo, err := ginutil.ParseQueryTimeRange(c, "created_from", "created_to")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	items, total, err := h.broadcasts.ListTelegramUsers(broadcastapp.UserQuery{
		Page:             page,
		PageSize:         pageSize,
		Keyword:          strings.TrimSpace(c.Query("keyword")),
		DisplayName:      strings.TrimSpace(c.Query("display_name")),
		TelegramUsername: strings.TrimSpace(c.Query("telegram_username")),
		TelegramUserID:   strings.TrimSpace(c.Query("telegram_user_id")),
		CreatedFrom:      createdFrom,
		CreatedTo:        createdTo,
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.bad_request", err)
		return
	}
	response.SuccessWithPage(c, items, response.BuildPagination(page, pageSize, total))
}
