package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/dujiao-next/internal/crypto"
	"github.com/dujiao-next/internal/modules/autorecharge/contract"
	"github.com/dujiao-next/internal/modules/autorecharge/domain"
	"github.com/google/uuid"
)

func digest(value string) string {
	h := sha256.Sum256([]byte(value))
	return hex.EncodeToString(h[:])
}

func validateSession(session string) error {
	if len(session) > 65536 {
		return contract.ErrInvalid
	}
	var v struct {
		AccessToken  string `json:"accessToken"`
		SessionToken string `json:"sessionToken"`
	}
	if json.Unmarshal([]byte(session), &v) != nil || strings.TrimSpace(v.AccessToken) == "" || strings.TrimSpace(v.SessionToken) == "" {
		return contract.ErrInvalid
	}
	return nil
}

func (s *Service) codeTask(ctx context.Context, code string) (*domain.Task, error) {
	canonical, err := domain.NormalizeRedemptionCode(code)
	if err != nil {
		return nil, contract.ErrNotFound
	}
	t, err := s.store.FindCode(ctx, digest(canonical))
	if err != nil {
		return nil, err
	}
	o, err := s.orders.Get(ctx, t.OrderID)
	if err != nil {
		return nil, err
	}
	if o == nil || !o.Paid || !o.Eligible {
		return nil, contract.ErrNotFound
	}
	return t, nil
}

func redemptionView(t *domain.Task) *contract.RedemptionView {
	return &contract.RedemptionView{State: t.State, Plan: t.Plan, Reason: t.LastCode}
}

// LookupCode authenticates possession without disclosing order IDs or credentials.
func (s *Service) LookupCode(ctx context.Context, code string) (*contract.RedemptionView, error) {
	t, err := s.codeTask(ctx, code)
	if err != nil {
		return nil, err
	}
	return redemptionView(t), nil
}

func (s *Service) saveRedemption(t *domain.Task) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.store.Save(ctx, t)
}

func clearCheck(t *domain.Task) {
	t.SessionCipher, t.AccountCipher, t.ConfirmationHash = "", "", ""
	t.CheckedUntil = nil
}

// CheckAccount invalidates every earlier confirmation before contacting the
// provider, including when the replacement Session is invalid or unavailable.
func (s *Service) CheckAccount(ctx context.Context, code, session string) (view *contract.RedemptionView, err error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	t, err := s.codeTask(ctx, code)
	if err != nil {
		return nil, err
	}
	t, err = s.store.Claim(ctx, t.OrderID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	defer func() {
		if e := s.saveRedemption(t); e != nil {
			view, err = nil, e
		}
	}()
	if t.ConfirmedAt != nil || terminal(t.State) || (t.State != "issued" && t.State != "awaiting_confirmation") {
		return nil, contract.ErrLocked
	}
	clearCheck(t)
	t.State, t.LastCode = "issued", ""
	if err = s.store.Checkpoint(ctx, t); err != nil {
		return nil, err
	}
	if err = validateSession(session); err != nil {
		return nil, err
	}
	a, err := s.partner.InspectAccount(ctx, session)
	if err != nil || a == nil || a.ID == "" || a.Plan == "" {
		return nil, contract.ErrUnavailable
	}
	// This product is a new subscription, not an upgrade/renewal product.
	if a.Plan != "free" {
		v := redemptionView(t)
		v.Account, v.Reason = a, "existing_subscription_not_supported"
		return v, nil
	}
	t.SessionCipher, err = crypto.Encrypt(s.key, session)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	t.AccountCipher, err = crypto.Encrypt(s.key, string(raw))
	if err != nil {
		return nil, err
	}
	token := uuid.NewString()
	t.ConfirmationHash = digest(token)
	until := time.Now().UTC().Add(10 * time.Minute)
	t.CheckedUntil, t.NextRunAt = &until, until
	t.State = "awaiting_confirmation"
	v := redemptionView(t)
	v.Account, v.ConfirmationToken, v.CheckedUntil = a, token, &until
	return v, nil
}

// ConfirmCode only persists explicit customer consent. The worker, never the
// HTTP request, starts the payment. Replays return the same task state.
func (s *Service) ConfirmCode(ctx context.Context, code, token string) (view *contract.RedemptionView, err error) {
	t, err := s.codeTask(ctx, code)
	if err != nil {
		return nil, err
	}
	t, err = s.store.Claim(ctx, t.OrderID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	defer func() {
		if e := s.saveRedemption(t); e != nil {
			view, err = nil, e
		}
	}()
	if token == "" || t.ConfirmationHash != digest(token) {
		return nil, contract.ErrInvalid
	}
	if t.ConfirmedAt != nil {
		return redemptionView(t), nil
	}
	now := time.Now().UTC()
	if t.State != "awaiting_confirmation" || t.CheckedUntil == nil || !now.Before(*t.CheckedUntil) || t.SessionCipher == "" || t.AccountCipher == "" {
		return nil, contract.ErrInvalid
	}
	raw, err := crypto.Decrypt(s.key, t.AccountCipher)
	if err != nil {
		return nil, err
	}
	var account contract.Account
	if json.Unmarshal([]byte(raw), &account) != nil || account.ID == "" {
		return nil, contract.ErrInvalid
	}
	hash := digest(account.ID)
	t.ActiveAccountHash = &hash
	t.ConfirmedAt, t.NextRunAt = &now, now
	t.State, t.LastCode = "waiting_card", ""
	return redemptionView(t), nil
}

func (s *Service) issueCode(ctx context.Context, t *domain.Task) error {
	code, err := crypto.Decrypt(s.key, t.CodeCipher)
	if err != nil {
		return err
	}
	if err = s.orders.DeliverCode(ctx, t.OrderID, code); err != nil {
		return err
	}
	now := time.Now().UTC()
	t.IssuedAt, t.State, t.LastCode = &now, "issued", ""
	return nil
}

func (s *Service) recheckAccount(ctx context.Context, t *domain.Task, session string) error {
	raw, err := crypto.Decrypt(s.key, t.AccountCipher)
	if err != nil {
		return err
	}
	var expected contract.Account
	if json.Unmarshal([]byte(raw), &expected) != nil || expected.ID == "" {
		return contract.ErrInvalid
	}
	a, err := s.partner.InspectAccount(ctx, session)
	if err != nil {
		return err
	}
	if a == nil || a.ID != expected.ID || a.Plan != "free" {
		return contract.ErrInvalid
	}
	return nil
}
