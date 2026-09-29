package fulfillmentwiring

import (
	"github.com/dujiao-next/internal/app/container"
	fulfillmenttransport "github.com/dujiao-next/internal/modules/fulfillment/transport/http"
)

func NewAdminHandler(c *container.Container) *fulfillmenttransport.AdminHandler {
	return fulfillmenttransport.NewAdminHandler(
		fulfillmentManualCreatorAdapter{svc: c.FulfillmentService},
		fulfillmentAdminOrderAdapter{orders: c.OrderService},
	)
}
