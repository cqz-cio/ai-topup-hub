package fulfillmentwiring

import (
	"errors"
	"fmt"

	fulfillmentapp "github.com/dujiao-next/internal/modules/fulfillment/application"
	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	orderapp "github.com/dujiao-next/internal/modules/order/application"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	fulfillmenttransport "github.com/dujiao-next/internal/modules/fulfillment/transport/http"
)

type fulfillmentManualCreatorAdapter struct {
	svc *fulfillmentapp.Service
}

func (a fulfillmentManualCreatorAdapter) CreateManual(input fulfillmenttransport.CreateManualInput) (*fulfillmentdomain.Fulfillment, error) {
	res, err := a.svc.CreateManual(fulfillmentapp.CreateManualInput{
		OrderID:      input.OrderID,
		AdminID:      input.AdminID,
		Payload:      input.Payload,
		DeliveryData: input.DeliveryData,
	})
	return res, mapFulfillmentTransportError(err)
}

type fulfillmentAdminOrderAdapter struct {
	orders *orderapp.OrderService
}

func (a fulfillmentAdminOrderAdapter) GetOrderForAdmin(orderID uint) (*orderdomain.Order, error) {
	order, err := a.orders.GetOrderForAdmin(orderID)
	return order, mapFulfillmentTransportError(err)
}

func mapFulfillmentTransportError(err error) error {
	if err == nil {
		return nil
	}
	for _, mapping := range []struct {
		source error
		target error
	}{
		{fulfillmentapp.ErrFulfillmentExists, fulfillmenttransport.ErrFulfillmentExists},
		{fulfillmentapp.ErrFulfillmentInvalid, fulfillmenttransport.ErrFulfillmentInvalid},
		{fulfillmentapp.ErrOrderStatusInvalid, fulfillmenttransport.ErrOrderStatusInvalid},
		{fulfillmentapp.ErrOrderNotFound, fulfillmenttransport.ErrOrderNotFound},
	} {
		if errors.Is(err, mapping.source) {
			return fmt.Errorf("%w: %v", mapping.target, err)
		}
	}
	return err
}
