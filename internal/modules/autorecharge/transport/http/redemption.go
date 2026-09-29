package rechargehttp

import (
	"github.com/dujiao-next/internal/modules/autorecharge/contract"
	"github.com/gin-gonic/gin"
	"net/http"
)

type redemptionInput struct {
	Code       string `json:"code"`
	Session    string `json:"session"`
	Authorized bool   `json:"authorized"`
	Confirmed  bool   `json:"confirmed"`
	Token      string `json:"confirmation_token"`
}

func readRedemption(c *gin.Context, limit int64) (*redemptionInput, bool) {
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
	var input redemptionInput
	if c.ShouldBindJSON(&input) != nil || input.Code == "" || len(input.Code) > 64 {
		fail(c, contract.ErrInvalid)
		return nil, false
	}
	return &input, true
}
func (h *Handler) lookupCode(c *gin.Context) {
	i, ok := readRedemption(c, 1024)
	if !ok {
		return
	}
	v, err := h.service.LookupCode(c.Request.Context(), i.Code)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, gin.H{"data": v})
}
func (h *Handler) checkAccount(c *gin.Context) {
	i, ok := readRedemption(c, 70000)
	if !ok {
		return
	}
	if !i.Authorized {
		fail(c, contract.ErrInvalid)
		return
	}
	v, err := h.service.CheckAccount(c.Request.Context(), i.Code, i.Session)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, gin.H{"data": v})
}
func (h *Handler) confirmCode(c *gin.Context) {
	i, ok := readRedemption(c, 1024)
	if !ok {
		return
	}
	if !i.Confirmed {
		fail(c, contract.ErrInvalid)
		return
	}
	v, err := h.service.ConfirmCode(c.Request.Context(), i.Code, i.Token)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, gin.H{"data": v})
}
