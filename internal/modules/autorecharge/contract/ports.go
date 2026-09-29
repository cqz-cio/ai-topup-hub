package contract

import (
	"context"
	"errors"
	"github.com/dujiao-next/internal/modules/autorecharge/domain"
	"time"
)

var (
	ErrUnavailable = errors.New("provider_not_configured")
	ErrInvalid     = errors.New("invalid_recharge_request")
	ErrBusy        = errors.New("recharge_busy")
	ErrNotFound    = errors.New("recharge_not_found")
	ErrLocked      = errors.New("recharge_already_started")
)

type Binding struct {
	SKUID              uint   `mapstructure:"sku_id"`
	Plan               string `mapstructure:"plan"`
	Region             string `mapstructure:"region"`
	RegionVersion      int    `mapstructure:"region_version"`
	Channel            string `mapstructure:"channel"`
	CardRule           string `mapstructure:"card_rule"`
	CancelAfterSuccess bool   `mapstructure:"cancel_after_success"`
}

type Order struct {
	ID, UserID     uint
	Paid, Eligible bool
	SKUID          uint
}
type Orders interface {
	Resolve(context.Context, string, uint) (uint, error)
	Get(context.Context, uint) (*Order, error)
	Children(context.Context, uint) ([]uint, error)
	Unscheduled(context.Context, []uint, int) ([]uint, error)
	DeliverCode(context.Context, uint, string) error
}
type Store interface {
	Ensure(context.Context, *domain.Task) error
	Get(context.Context, uint) (*domain.Task, error) // local order ID
	FindCode(context.Context, string) (*domain.Task, error)
	Due(context.Context, time.Time, int) ([]uint, error)
	Claim(context.Context, uint, time.Time) (*domain.Task, error)
	Checkpoint(context.Context, *domain.Task) error
	Save(context.Context, *domain.Task) error
}

// Reserve must be idempotent by taskKey and permanently pin the same card to it.
// Credentials must be supplied just in time, never written to the task or logs.
// A real card platform adapter is intentionally not supplied yet.
type Cards interface {
	Reserve(ctx context.Context, taskKey, rule string) (reference string, err error)
	Credentials(ctx context.Context, reference string) (Card, error)
}
type Card struct{ Number, ExpMonth, ExpYear, CVC string }
type NoCards struct{}

func (NoCards) Reserve(context.Context, string, string) (string, error) { return "", ErrUnavailable }
func (NoCards) Credentials(context.Context, string) (Card, error)       { return Card{}, ErrUnavailable }

type PayRequest struct {
	Session            string `json:"session"`
	Plan               string `json:"plan"`
	Region             string `json:"region"`
	RegionVersion      int    `json:"region_version"`
	Channel            string `json:"protocol_channel"`
	CancelAfterSuccess bool   `json:"cancel_after_success"`
	Number             string `json:"card_number"`
	ExpMonth           string `json:"exp_month"`
	ExpYear            string `json:"exp_year"`
	CVC                string `json:"cvc"`
}

// Result deliberately excludes provider error messages and arbitrary payloads.
type Result struct {
	OperationID        string `json:"operation_id"`
	State              string `json:"state"`
	Code               string `json:"code"`
	BillingState       string `json:"billing_state"`
	ChargedCredits     int    `json:"charged_credits"`
	SubscriptionSynced *bool  `json:"subscription_synced"`
	AccountID          string `json:"account_id"`
	ClientSecret       string `json:"client_secret"`
	PublishableKey     string `json:"stripe_publishable_key"`
	CheckoutID         string `json:"checkout_id"`
	ProcessorEntity    string `json:"processor_entity"`
	SetupIntentID      string `json:"setup_intent_id"`
	VerificationStage  string `json:"verification_stage"`
	Plan               string `json:"plan"`
	PaymentIntent      struct {
		ID string `json:"id"`
	} `json:"payment_intent"`
}
type FinalizeRequest struct {
	Session              string `json:"session"`
	OperationID          string `json:"operation_id"`
	AccountID            string `json:"account_id"`
	ClientSecret         string `json:"client_secret"`
	Plan                 string `json:"plan"`
	PaymentIntentID      string `json:"payment_intent_id,omitempty"`
	SetupIntentID        string `json:"setup_intent_id,omitempty"`
	CheckoutID           string `json:"checkout_id,omitempty"`
	ProcessorEntity      string `json:"processor_entity,omitempty"`
	AuthenticationFailed bool   `json:"authentication_failed"`
}
type Partner interface {
	InspectAccount(context.Context, string) (*Account, error)
	Ready() bool
	Check(context.Context, Binding) error
	Pay(context.Context, string, PayRequest) (*Result, error)
	Poll(context.Context, string) (*Result, error)
	Finalize(context.Context, FinalizeRequest) (*Result, error)
	SubscriptionActive(context.Context, string, string) (bool, error)
}

// Account contains only fields verified by the provider, never claims extracted
// locally from customer-supplied Session JSON.
type Account struct {
	ID   string `json:"account_id"`
	Plan string `json:"current_plan"`
}

type RedemptionView struct {
	State             string     `json:"state"`
	Plan              string     `json:"plan"`
	Reason            string     `json:"reason,omitempty"`
	Account           *Account   `json:"account,omitempty"`
	ConfirmationToken string     `json:"confirmation_token,omitempty"`
	CheckedUntil      *time.Time `json:"checked_until,omitempty"`
}
