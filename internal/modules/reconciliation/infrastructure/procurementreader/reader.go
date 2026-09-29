package procurementreader

import (
	"time"

	procurementdomain "github.com/dujiao-next/internal/modules/procurement/domain"
	reconciliationcontract "github.com/dujiao-next/internal/modules/reconciliation/contract"
)

type Source interface {
	ListByConnectionAndTimeRange(connectionID uint, start, end time.Time) ([]procurementdomain.Order, error)
}

type Reader struct {
	source Source
}

var _ reconciliationcontract.ProcurementReader = (*Reader)(nil)

func New(source Source) *Reader {
	if source == nil {
		panic("reconciliation procurement reader: source is nil")
	}
	return &Reader{source: source}
}

func (r *Reader) ListByConnectionAndTimeRange(connectionID uint, start, end time.Time) ([]reconciliationcontract.ProcurementOrder, error) {
	orders, err := r.source.ListByConnectionAndTimeRange(connectionID, start, end)
	if err != nil {
		return nil, err
	}
	result := make([]reconciliationcontract.ProcurementOrder, 0, len(orders))
	for _, order := range orders {
		result = append(result, reconciliationcontract.ProcurementOrder{
			ID: order.ID, UpstreamOrderID: order.UpstreamOrderID,
			LocalOrderNo: order.LocalOrderNo, UpstreamOrderNo: order.UpstreamOrderNo,
			Status: order.Status, UpstreamAmount: order.UpstreamAmount,
		})
	}
	return result, nil
}
