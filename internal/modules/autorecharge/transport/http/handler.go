package rechargehttp

import (
	"context"
	"errors"
	"github.com/dujiao-next/internal/modules/autorecharge/contract"
	"github.com/dujiao-next/internal/modules/autorecharge/domain"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"time"
)

type Service interface {
	LookupCode(context.Context, string) (*contract.RedemptionView, error)
	CheckAccount(context.Context, string, string) (*contract.RedemptionView, error)
	ConfirmCode(context.Context, string, string) (*contract.RedemptionView, error)
	ResolveOrder(context.Context, string, uint) (uint, error)
	Get(context.Context, uint, uint, bool) (*domain.Task, error)
	SetSession(context.Context, uint, uint, bool, string) error
	Verification(context.Context, uint) (*contract.Result, error)
	Finalize(context.Context, uint, bool) error
	CreateForOrder(uint) error
	Advance(context.Context, uint) error
}
type Handler struct{ service Service }

func New(s Service) *Handler { return &Handler{service: s} }

func (h *Handler) resolve(c *gin.Context, admin bool) (uint, bool) {
	if admin {
		return orderID(c)
	}
	id, err := h.service.ResolveOrder(c.Request.Context(), c.Param("order_no"), c.GetUint("user_id"))
	if err != nil {
		fail(c, err)
		return 0, false
	}
	return id, true
}
func orderID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		fail(c, contract.ErrInvalid)
		return 0, false
	}
	return uint(id), true
}
func fail(c *gin.Context, err error) {
	status, code := http.StatusConflict, "recharge_unavailable"
	switch {
	case errors.Is(err, contract.ErrNotFound):
		status, code = 404, "recharge_not_found"
	case errors.Is(err, contract.ErrInvalid):
		status, code = 400, "invalid_recharge_request"
	case errors.Is(err, contract.ErrBusy):
		code = "recharge_busy"
	case errors.Is(err, contract.ErrLocked):
		code = "recharge_already_started"
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(status, gin.H{"code": code})
}
func (h *Handler) status(c *gin.Context, admin bool) {
	id, ok := h.resolve(c, admin)
	if !ok {
		return
	}
	t, err := h.service.Get(c.Request.Context(), id, c.GetUint("user_id"), admin)
	if err != nil {
		fail(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	if admin {
		c.JSON(200, gin.H{"data": t, "reconciliation": gin.H{"operation_id": t.OperationID, "idempotency_key": t.IdempotencyKey, "billing_state": t.BillingState, "charged_credits": t.ChargedCredits}})
		return
	}
	c.JSON(200, gin.H{"data": t})
}
func (h *Handler) adminStatus(c *gin.Context) { h.status(c, true) }
func (h *Handler) userStatus(c *gin.Context)  { h.status(c, false) }
func (h *Handler) session(c *gin.Context, admin bool) {
	id, ok := h.resolve(c, admin)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 70000)
	var input struct {
		Session    string `json:"session"`
		Authorized bool   `json:"authorized"`
	}
	if c.ShouldBindJSON(&input) != nil || !input.Authorized {
		fail(c, contract.ErrInvalid)
		return
	}
	if err := h.service.SetSession(c.Request.Context(), id, c.GetUint("user_id"), admin, input.Session); err != nil {
		fail(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"ok": true})
}
func (h *Handler) adminSession(c *gin.Context) { h.session(c, true) }
func (h *Handler) userSession(c *gin.Context)  { h.session(c, false) }
func (h *Handler) advance(c *gin.Context) {
	id, ok := orderID(c)
	if !ok {
		return
	}
	if err := h.service.CreateForOrder(id); err != nil {
		fail(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 75*time.Second)
	defer cancel()
	if err := h.service.Advance(ctx, id); err != nil {
		fail(c, err)
		return
	}
	h.adminStatus(c)
}
func (h *Handler) verification(c *gin.Context) {
	id, ok := orderID(c)
	if !ok {
		return
	}
	r, err := h.service.Verification(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	// Only the bank challenge information is returned to the merchant administrator.
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"data": gin.H{"operation_id": r.OperationID, "stripe_publishable_key": r.PublishableKey, "client_secret": r.ClientSecret, "verification_stage": r.VerificationStage}})
}
func (h *Handler) finalize(c *gin.Context) {
	id, ok := orderID(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024)
	var input struct {
		Failed *bool `json:"authentication_failed"`
	}
	if c.ShouldBindJSON(&input) != nil || input.Failed == nil {
		fail(c, contract.ErrInvalid)
		return
	}
	if err := h.service.Finalize(c.Request.Context(), id, *input.Failed); err != nil {
		fail(c, err)
		return
	}
	h.adminStatus(c)
}
