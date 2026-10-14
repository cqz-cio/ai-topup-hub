package upstreamreader

import (
	"context"
	"fmt"
	"strings"

	reconciliationcontract "github.com/dujiao-next/internal/modules/reconciliation/contract"
	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
	"github.com/dujiao-next/internal/upstream"
)

type ConnectionProvider interface {
	GetByID(id uint) (*siteconnectiondomain.Connection, error)
	GetAdapter(connection *siteconnectiondomain.Connection) (upstream.Adapter, error)
}

type Reader struct {
	connections ConnectionProvider
}

var _ reconciliationcontract.UpstreamOrderProvider = (*Reader)(nil)

type session struct {
	adapter upstream.Adapter
}

var _ reconciliationcontract.UpstreamOrderReader = (*session)(nil)

func New(connections ConnectionProvider) *Reader {
	if connections == nil {
		panic("reconciliation upstream reader: connections is nil")
	}
	return &Reader{connections: connections}
}

func (r *Reader) Open(connectionID uint) (reconciliationcontract.UpstreamOrderReader, error) {
	connection, err := r.connections.GetByID(connectionID)
	if err != nil {
		return nil, fmt.Errorf("get connection: %w", err)
	}
	adapter, err := r.connections.GetAdapter(connection)
	if err != nil {
		return nil, fmt.Errorf("get adapter: %w", err)
	}
	return &session{adapter: adapter}, nil
}

func (s *session) Get(ctx context.Context, upstreamOrderID uint, upstreamOrderNo string) (*reconciliationcontract.UpstreamOrder, error) {
	var detail *upstream.UpstreamOrderDetail
	var err error
	if referenced, ok := s.adapter.(upstream.ReferenceOrderAdapter); ok && strings.TrimSpace(upstreamOrderNo) != "" {
		detail, err = referenced.GetOrderByReference(ctx, upstreamOrderNo)
	} else if upstreamOrderID > 0 {
		detail, err = s.adapter.GetOrder(ctx, upstreamOrderID)
	} else {
		return nil, fmt.Errorf("supplier does not support string order references")
	}
	if err != nil {
		return nil, err
	}
	if detail == nil {
		return nil, fmt.Errorf("supplier returned no order detail")
	}
	return &reconciliationcontract.UpstreamOrder{Status: detail.Status, Amount: detail.Amount}, nil
}
