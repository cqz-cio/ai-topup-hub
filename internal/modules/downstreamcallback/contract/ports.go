package contract

import (
	"context"
	"time"

	downstreamdomain "github.com/dujiao-next/internal/modules/downstreamcallback/domain"
)

// Repository 持久化下游订单回调引用。
type Repository interface {
	GetByID(id uint) (*downstreamdomain.OrderRef, error)
	GetByOrderID(orderID uint) (*downstreamdomain.OrderRef, error)
	GetByCredentialAndDownstreamNo(credentialID uint, downstreamOrderNo string) (*downstreamdomain.OrderRef, error)
	Create(ref *downstreamdomain.OrderRef) error
	Update(ref *downstreamdomain.OrderRef) error
	ListPendingCallbacks(limit int) ([]downstreamdomain.OrderRef, error)
	ListByCredentialID(credentialID uint, filter RefListFilter) ([]downstreamdomain.OrderRef, int64, error)
}

// OrderReader 返回回调编排所需的订单投影。
type OrderReader interface {
	GetByID(id uint) (*OrderSnapshot, error)
}

// CredentialReader 返回回调签名所需的凭证投影。
type CredentialReader interface {
	GetByID(id uint) (*Credential, error)
}

// CallbackQueue 负责立即或延迟投递回调任务。
type CallbackQueue interface {
	EnqueueCallback(refID uint, delay time.Duration) error
}

// Deliverer 执行签名后的下游 HTTP 回调。
type Deliverer interface {
	Send(ctx context.Context, request DeliveryRequest) error
}
