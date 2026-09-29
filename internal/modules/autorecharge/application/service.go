package application

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/dujiao-next/internal/crypto"
	"github.com/dujiao-next/internal/modules/autorecharge/contract"
	"github.com/dujiao-next/internal/modules/autorecharge/domain"
	"github.com/google/uuid"
	"strings"
	"time"
)

type Service struct {
	store    contract.Store
	orders   contract.Orders
	cards    contract.Cards
	partner  contract.Partner
	key      []byte
	bindings map[uint]contract.Binding
}
type Options struct {
	Store    contract.Store
	Orders   contract.Orders
	Cards    contract.Cards
	Partner  contract.Partner
	Secret   string
	Bindings []contract.Binding
}

func New(o Options) (*Service, error) {
	if o.Store == nil || o.Orders == nil || o.Partner == nil || len(o.Secret) < 32 {
		return nil, contract.ErrInvalid
	}
	if o.Cards == nil {
		o.Cards = contract.NoCards{}
	}
	s := &Service{store: o.Store, orders: o.Orders, cards: o.Cards, partner: o.Partner, key: crypto.DeriveKey(o.Secret), bindings: map[uint]contract.Binding{}}
	for _, b := range o.Bindings {
		if b.SKUID == 0 || b.CardRule == "" || len(b.Region) != 2 || b.RegionVersion < 0 || (b.Plan != "chatgptplusplan" && b.Plan != "chatgptprolite" && b.Plan != "chatgptpro") || (b.Channel != "1" && b.Channel != "2" && b.Channel != "3") {
			return nil, contract.ErrInvalid
		}
		if _, ok := s.bindings[b.SKUID]; ok {
			return nil, contract.ErrInvalid
		}
		s.bindings[b.SKUID] = b
	}
	return s, nil
}
func (s *Service) CreateForOrder(id uint) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ids, err := s.orders.Children(ctx, id)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		ids = []uint{id}
	}
	for _, id := range ids {
		if err = s.ensure(ctx, id); err != nil {
			return err
		}
	}
	return nil
}
func (s *Service) ensure(ctx context.Context, id uint) error {
	o, err := s.orders.Get(ctx, id)
	if err != nil {
		return err
	}
	if o == nil || !o.Paid {
		return nil
	}
	if _, err := s.store.Get(ctx, id); err == nil {
		return nil
	} else if !errors.Is(err, contract.ErrNotFound) {
		return err
	}
	b, ok := s.bindings[o.SKUID]
	if !ok {
		return nil
	}
	now := time.Now().UTC()
	code, err := domain.GenerateRedemptionCode()
	if err != nil {
		return err
	}
	cipher, err := crypto.Encrypt(s.key, code)
	if err != nil {
		return err
	}
	hash := digest(code)
	t := &domain.Task{OrderID: id, State: "issuing", CodeHash: &hash, CodeCipher: cipher, Plan: b.Plan, Region: b.Region, RegionVersion: b.RegionVersion, Channel: b.Channel, CardRule: b.CardRule, CancelAfterSuccess: b.CancelAfterSuccess, IdempotencyKey: uuid.NewString(), NextRunAt: now}
	if !o.Eligible {
		t.State = "manual_review"
		t.LastCode = "requires_single_manual_item_quantity_one"
	}
	return s.store.Ensure(ctx, t)
}
func (s *Service) Get(ctx context.Context, orderID, userID uint, admin bool) (*domain.Task, error) {
	if err := s.authorize(ctx, orderID, userID, admin); err != nil {
		return nil, err
	}
	return s.store.Get(ctx, orderID)
}

func (s *Service) ResolveOrder(ctx context.Context, orderNo string, userID uint) (uint, error) {
	if userID == 0 || strings.TrimSpace(orderNo) == "" {
		return 0, contract.ErrNotFound
	}
	return s.orders.Resolve(ctx, orderNo, userID)
}
func (s *Service) authorize(ctx context.Context, id, user uint, admin bool) error {
	o, err := s.orders.Get(ctx, id)
	if err != nil {
		return err
	}
	if o == nil || (!admin && (user == 0 || o.UserID != user)) {
		return contract.ErrNotFound
	}
	return nil
}
func (s *Service) SetSession(ctx context.Context, id, user uint, admin bool, session string) error {
	if err := s.authorize(ctx, id, user, admin); err != nil {
		return err
	}
	if err := validateSession(session); err != nil {
		return err
	}
	// Legacy entry point cannot bypass code possession, account precheck or consent.
	return contract.ErrLocked
}
func terminal(state string) bool {
	return state == "completed" || state == "failed" || state == "canceled" || state == "manual_review"
}
func (s *Service) binding(t *domain.Task) contract.Binding {
	return contract.Binding{Plan: t.Plan, Region: t.Region, RegionVersion: t.RegionVersion, Channel: t.Channel, CardRule: t.CardRule}
}

// RunOnce recovers missed payment notifications and advances durable due tasks.
func (s *Service) RunOnce(ctx context.Context) error {
	skus := make([]uint, 0, len(s.bindings))
	for id := range s.bindings {
		skus = append(skus, id)
	}
	if len(skus) > 0 {
		ids, err := s.orders.Unscheduled(ctx, skus, 50)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if err = s.ensure(ctx, id); err != nil {
				return err
			}
		}
	}
	ids, err := s.store.Due(ctx, time.Now().UTC(), 20)
	if err != nil {
		return err
	}
	var first error
	for _, id := range ids {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err = s.Advance(ctx, id); err != nil && !errors.Is(err, contract.ErrBusy) && first == nil {
			first = err
		}
	}
	return first
}
func (s *Service) Advance(ctx context.Context, id uint) error {
	ctx, cancelStep := context.WithTimeout(ctx, 75*time.Second)
	defer cancelStep()
	t, err := s.store.Claim(ctx, id, time.Now().UTC())
	if err != nil {
		return err
	}
	t.NextRunAt = time.Now().UTC().Add(30 * time.Second)
	err = s.step(ctx, t)
	if terminal(t.State) {
		t.SessionCipher = ""
		t.VerificationCipher = ""
		t.AccountCipher = ""
		if t.State == "completed" || t.State == "failed" || t.State == "canceled" || t.OperationID == "" && t.State != "unknown" {
			t.ActiveAccountHash = nil
		}
	}
	// Use a fresh bounded context so graceful cancellation can still release the lease.
	saveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if saveErr := s.store.Save(saveCtx, t); saveErr != nil {
		return saveErr
	}
	return err
}
func (s *Service) step(ctx context.Context, t *domain.Task) error {
	if terminal(t.State) {
		return nil
	}
	o, err := s.orders.Get(ctx, t.OrderID)
	if err != nil {
		return err
	}
	if o == nil || !o.Paid || !o.Eligible {
		// A local cancellation/refund cannot cancel an already accepted remote
		// payment. Keep reconciling it, but never deliver or initiate another pay.
		if t.OperationID != "" {
			r, pollErr := s.partner.Poll(ctx, t.OperationID)
			if pollErr == nil && r != nil && r.OperationID == t.OperationID && (r.State == "succeeded" || r.State == "failed") {
				t.BillingState, t.ChargedCredits = r.BillingState, r.ChargedCredits
				t.State, t.LastCode = "manual_review", "local_order_closed_after_"+r.State
			} else {
				t.State, t.LastCode = "unknown", "local_order_closed_reconciling"
			}
			return nil
		}
		if t.State == "submitting" || t.State == "unknown" {
			t.State, t.LastCode = "unknown", "local_order_closed_submission_unknown"
			return nil
		}
		t.State = "manual_review"
		t.LastCode = "local_order_not_fulfillable"
		return nil
	}
	if t.State == "issuing" {
		return s.issueCode(ctx, t)
	}
	if t.ConfirmedAt == nil {
		if t.CodeHash == nil {
			t.State, t.LastCode = "manual_review", "legacy_task_requires_reconciliation"
			return nil
		}
		if t.OperationID != "" || t.State == "submitting" || t.State == "unknown" {
			t.State, t.LastCode = "manual_review", "legacy_task_requires_reconciliation"
			return nil
		}
		if t.State == "awaiting_confirmation" && (t.CheckedUntil == nil || !time.Now().UTC().Before(*t.CheckedUntil)) {
			clearCheck(t)
			t.State, t.LastCode = "issued", "account_check_expired"
		}
		return nil
	}
	if t.State == "succeeded" {
		return s.complete(ctx, t)
	}
	if t.State == "confirming_subscription" {
		session, err := crypto.Decrypt(s.key, t.SessionCipher)
		if err != nil {
			return err
		}
		active, err := s.partner.SubscriptionActive(ctx, session, t.Plan)
		if err != nil {
			t.LastCode = "subscription_query_pending"
			return nil
		}
		if active {
			t.State = "succeeded"
			return s.complete(ctx, t)
		}
		return nil
	}
	if t.OperationID != "" {
		r, err := s.partner.Poll(ctx, t.OperationID)
		if err != nil {
			t.LastCode = "partner_query_pending"
			return nil
		}
		return s.apply(ctx, t, r)
	}
	if t.State == "submitting" || t.State == "unknown" {
		t.State = "unknown"
		t.LastCode = "submission_requires_reconciliation"
		return nil
	}
	if _, missing := s.cards.(contract.NoCards); missing {
		t.State = "waiting_card"
		t.LastCode = "card_provider_not_configured"
		return nil
	}
	if t.SessionCipher == "" {
		t.State = "waiting_session"
		t.LastCode = "session_required"
		return nil
	}
	if !s.partner.Ready() {
		t.State = "waiting_configuration"
		t.LastCode = "api_key_not_configured"
		return nil
	}
	if err = s.partner.Check(ctx, s.binding(t)); err != nil {
		t.State = "waiting_configuration"
		t.LastCode = "quota_or_region_check_failed"
		return nil
	}
	session, err := crypto.Decrypt(s.key, t.SessionCipher)
	if err != nil {
		return err
	}
	if err = s.recheckAccount(ctx, t, session); err != nil {
		t.State, t.LastCode = "manual_review", "account_recheck_failed"
		return nil
	}
	if t.CardRef == "" {
		t.CardRef, err = s.cards.Reserve(ctx, t.IdempotencyKey, t.CardRule)
		if err != nil || t.CardRef == "" {
			t.State = "waiting_card"
			t.LastCode = "card_unavailable"
			return nil
		}
	}
	// Persist the reservation before requesting ephemeral credentials.
	if err = s.store.Checkpoint(ctx, t); err != nil {
		return err
	}
	card, err := s.cards.Credentials(ctx, t.CardRef)
	if err != nil {
		t.State = "waiting_card"
		t.LastCode = "card_credentials_unavailable"
		return nil
	}
	if card.Number == "" || card.ExpMonth == "" || card.ExpYear == "" || card.CVC == "" {
		t.State = "waiting_card"
		t.LastCode = "card_credentials_invalid"
		return nil
	}
	t.State = "submitting"
	t.LastCode = ""
	if err = s.store.Checkpoint(ctx, t); err != nil {
		return err
	}
	r, err := s.partner.Pay(ctx, t.IdempotencyKey, contract.PayRequest{Session: session, Plan: t.Plan, Region: t.Region, RegionVersion: t.RegionVersion, Channel: t.Channel, CancelAfterSuccess: t.CancelAfterSuccess, Number: card.Number, ExpMonth: card.ExpMonth, ExpYear: card.ExpYear, CVC: card.CVC})
	card = contract.Card{}
	if err != nil {
		t.State = "unknown"
		t.LastCode = "submission_requires_reconciliation"
		return nil
	}
	return s.apply(ctx, t, r)
}
func (s *Service) apply(ctx context.Context, t *domain.Task, r *contract.Result) error {
	if r == nil || r.OperationID == "" || (t.OperationID != "" && t.OperationID != r.OperationID) {
		t.State = "unknown"
		t.LastCode = "invalid_partner_task"
		return nil
	}
	t.OperationID = r.OperationID
	t.BillingState = r.BillingState
	t.ChargedCredits = r.ChargedCredits
	t.LastCode = ""
	switch r.State {
	case "queued", "running", "stopping", "unknown":
		t.State = r.State
	case "action_required":
		t.State = r.State
		raw, err := json.Marshal(r)
		if err != nil {
			return err
		}
		t.VerificationCipher, err = crypto.Encrypt(s.key, string(raw))
		if err != nil {
			return err
		}
	case "failed":
		t.State = "failed"
		t.LastCode = "partner_task_failed"
	case "succeeded":
		t.VerificationCipher = ""
		if r.SubscriptionSynced != nil && *r.SubscriptionSynced {
			t.State = "succeeded"
			return s.complete(ctx, t)
		}
		t.State = "confirming_subscription"
	default:
		t.State = "unknown"
		t.LastCode = "unrecognized_partner_state"
	}
	return nil
}
func (s *Service) complete(ctx context.Context, t *domain.Task) error {
	// Persist payment success before local delivery, so a crash can only retry
	// the idempotent local delivery, never the external payment.
	if err := s.store.Checkpoint(ctx, t); err != nil {
		return err
	}
	// The storefront order was fulfilled when the code was delivered. Consuming
	// this entitlement must never replace that delivery or send a second code.
	now := time.Now().UTC()
	t.RedeemedAt = &now
	t.State = "completed"
	t.LastCode = ""
	return nil
}

// Verification is admin-only at the HTTP boundary: the merchant owns the card.
func (s *Service) Verification(ctx context.Context, id uint) (*contract.Result, error) {
	t, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if t.State != "action_required" || t.VerificationCipher == "" {
		return nil, contract.ErrInvalid
	}
	raw, err := crypto.Decrypt(s.key, t.VerificationCipher)
	if err != nil {
		return nil, err
	}
	var r contract.Result
	err = json.Unmarshal([]byte(raw), &r)
	return &r, err
}
func (s *Service) Finalize(ctx context.Context, id uint, authenticationFailed bool) error {
	ctx, cancelStep := context.WithTimeout(ctx, 75*time.Second)
	defer cancelStep()
	o, err := s.orders.Get(ctx, id)
	if err != nil {
		return err
	}
	if o == nil || !o.Paid || !o.Eligible {
		return contract.ErrLocked
	}
	t, err := s.store.Claim(ctx, id, time.Now().UTC())
	if err != nil {
		return err
	}
	finish := func(e error) error {
		t.NextRunAt = time.Now().UTC().Add(10 * time.Second)
		if terminal(t.State) {
			t.SessionCipher = ""
			t.VerificationCipher = ""
			t.AccountCipher = ""
			if t.State == "completed" || t.State == "failed" || t.State == "canceled" {
				t.ActiveAccountHash = nil
			}
		}
		saveCtx, cancelSave := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelSave()
		saveErr := s.store.Save(saveCtx, t)
		if saveErr != nil {
			return saveErr
		}
		return e
	}
	if t.ConfirmedAt == nil {
		return finish(contract.ErrLocked)
	}
	if (t.State != "action_required" && t.State != "unknown") || t.VerificationCipher == "" {
		return finish(contract.ErrInvalid)
	}
	raw, err := crypto.Decrypt(s.key, t.VerificationCipher)
	if err != nil {
		return finish(err)
	}
	var v contract.Result
	if json.Unmarshal([]byte(raw), &v) != nil {
		return finish(contract.ErrInvalid)
	}
	if v.OperationID != t.OperationID || v.AccountID == "" || v.ClientSecret == "" || v.Plan != t.Plan {
		return finish(contract.ErrInvalid)
	}
	session, err := crypto.Decrypt(s.key, t.SessionCipher)
	if err != nil {
		return finish(err)
	}
	req := contract.FinalizeRequest{Session: session, OperationID: t.OperationID, AccountID: v.AccountID, ClientSecret: v.ClientSecret, Plan: t.Plan, AuthenticationFailed: authenticationFailed}
	if v.VerificationStage == "bind" {
		if v.SetupIntentID == "" {
			return finish(contract.ErrInvalid)
		}
		req.SetupIntentID = v.SetupIntentID
	} else {
		if v.PaymentIntent.ID == "" || v.CheckoutID == "" {
			return finish(contract.ErrInvalid)
		}
		req.PaymentIntentID = v.PaymentIntent.ID
		req.CheckoutID = v.CheckoutID
		req.ProcessorEntity = v.ProcessorEntity
	}
	r, err := s.partner.Finalize(ctx, req)
	if err != nil {
		t.State = "unknown"
		t.LastCode = "verification_query_pending"
		return finish(nil)
	}
	return finish(s.apply(ctx, t, r))
}
