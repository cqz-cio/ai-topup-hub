package chainhttp

import (
	"context"
	"github.com/dujiao-next/internal/modules/payment/domain"
	"github.com/gin-gonic/gin"
	"strconv"
	"time"
)

type ChainPaymentQueries interface {
	Status(context.Context) (map[string]interface{}, error)
	Transfers(context.Context, string, uint64) ([]domain.ChainTransfer, error)
}

func RegisterBSCUSDTRoutes(admin *gin.RouterGroup, service ChainPaymentQueries) {
	admin.GET("/chain-payments/bsc-usdt/status", func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()
		result, err := service.Status(ctx)
		if err != nil {
			c.JSON(503, gin.H{"code": "chain_payment_query_unavailable"})
			return
		}
		c.JSON(200, gin.H{"data": result})
	})
	admin.GET("/chain-payments/bsc-usdt/transfers", func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		state := c.Query("state")
		if state != "" && state != "manual_review" && state != "ready" && state != "delivered" {
			c.JSON(400, gin.H{"code": "invalid_state"})
			return
		}
		var before uint64
		if raw := c.Query("before_block"); raw != "" {
			var err error
			before, err = strconv.ParseUint(raw, 10, 64)
			if err != nil {
				c.JSON(400, gin.H{"code": "invalid_block"})
				return
			}
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()
		rows, err := service.Transfers(ctx, state, before)
		if err != nil {
			c.JSON(503, gin.H{"code": "chain_payment_query_unavailable"})
			return
		}
		c.JSON(200, gin.H{"data": rows})
	})
}
