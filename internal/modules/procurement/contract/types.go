package contract

import (
	"context"
	"time"

	procurementdomain "github.com/dujiao-next/internal/modules/procurement/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

type ListFilter struct {
	ConnectionID    uint
	Status          string
	LocalOrderNo    string
	UpstreamOrderNo string
	CreatedFrom     *time.Time
	CreatedTo       *time.Time
	Page            int
	PageSize        int
}

type Fulfillment struct {
	Type         string
	Status       string
	Payload      string
	DeliveryData jsonmap.JSON
	DeliveredAt  *time.Time
}

type CreateOrderRequest struct {
	SKUID             uint
	Quantity          int
	ManualFormData    jsonmap.JSON
	DownstreamOrderNo string
	TraceID           string
	CallbackURL       string
	MaxAmount         string
	Currency          string
}

type CreateOrderResult struct {
	OK           bool
	OrderID      uint
	OrderNo      string
	Status       string
	Amount       string
	Currency     string
	ErrorCode    string
	ErrorMessage string
	Fulfillment  *Fulfillment
}

// ReferencedOrderConnection retains supplier order numbers without integer coercion.
type ReferencedOrderConnection interface {
	GetOrderByReference(ctx context.Context, orderID uint, orderNo string) (*UpstreamOrder, error)
}

type SingleSubmissionConnection interface {
	RequiresSingleSubmission() bool
}

type UpstreamOrder struct {
	OrderID        uint
	OrderNo        string
	Status         string
	Amount         string
	RefundedAmount string
	Currency       string
	Fulfillment    *Fulfillment
	RefundRecords  []jsonmap.JSON
}

type UpstreamConnection interface {
	CallbackURL() string
	RetryMax() int
	RetryIntervals() string
	CreateOrder(ctx context.Context, request CreateOrderRequest) (*CreateOrderResult, error)
	GetOrder(ctx context.Context, orderID uint) (*UpstreamOrder, error)
	CancelOrder(ctx context.Context, orderID uint) error
}

// UseCase 是支付、HTTP、上游回调与 Worker 共享的正式采购应用契约。
type UseCase interface {
	CreateForOrder(orderID uint) error
	SubmitToUpstream(procurementOrderID uint) error
	PollUpstreamStatus(procurementOrderID uint) error
	SyncAcceptedOrders()
	RecoverPaidOrders() error
	HandleUpstreamCallback(procurementOrderID uint, upstreamStatus string, fulfillment *Fulfillment) error
	GetByID(id uint) (*procurementdomain.Order, error)
	GetByLocalOrderNo(localOrderNo string) (*procurementdomain.Order, error)
	List(filter ListFilter) ([]procurementdomain.Order, int64, error)
	StatsByStatus(filter ListFilter) (map[string]int64, error)
	FillParentOrderNo(order *procurementdomain.Order)
	RetryManual(id uint) error
	CancelManual(id uint) error
}
