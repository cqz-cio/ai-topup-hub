package application

import (
	"context"
	"encoding/json"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/modules/procurement/contract"
	"github.com/dujiao-next/internal/modules/procurement/domain"
	"github.com/dujiao-next/internal/telegramidentity"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// BalanceRecovery only observes supplier balances and sends customer notices.
// It has no purchase, payment, wallet or order mutation port.
type BalanceRecovery struct {
	waits       contract.BalanceWaitRepository
	orders      contract.BalanceRecoveryOrderReader
	connections contract.ConnectionProvider
	queue       contract.BalanceRecoveryQueue
	mailer      contract.BalanceRecoveryMailer
}

func NewBalanceRecovery(waits contract.BalanceWaitRepository, orders contract.BalanceRecoveryOrderReader, connections contract.ConnectionProvider, queue contract.BalanceRecoveryQueue, mailer contract.BalanceRecoveryMailer) *BalanceRecovery {
	return &BalanceRecovery{waits: waits, orders: orders, connections: connections, queue: queue, mailer: mailer}
}

func (s *Service) SetBalanceRecovery(recovery *BalanceRecovery) { s.balanceRecovery = recovery }

func (s *Service) recordBalanceWait(ctx context.Context, orderID uint, needs []fundingNeed) {
	if s.balanceRecovery == nil {
		return
	}
	requirements := make([]domain.FundingRequirement, 0, len(needs))
	for _, need := range needs {
		requirements = append(requirements, domain.FundingRequirement{ConnectionID: need.connectionID, Amount: need.amount.String(), Currency: need.currency})
	}
	// A disconnected browser must not lose the durable reminder registration.
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	if err := s.balanceRecovery.Record(writeCtx, orderID, requirements); err != nil {
		logger.Warnw("balance_recovery_record_failed", "order_id", orderID, "error", err)
	}
}

func recoveryOrderEligible(order *contract.BalanceRecoveryOrder) bool {
	if order == nil || order.PaidAt != nil || order.ResellerID != nil {
		return false
	}
	if order.Status == constants.OrderStatusPendingPayment {
		return true
	}
	// Timeout cancellation still merits a return-to-store notice; an explicit
	// cancellation before expiry does not. No expired payment is resurrected.
	return order.Status == constants.OrderStatusCanceled && order.ExpiresAt != nil && order.CanceledAt != nil && !order.CanceledAt.Before(*order.ExpiresAt)
}

func recoveryRecipient(raw string) string {
	value := strings.TrimSpace(raw)
	parsed, err := mail.ParseAddress(value)
	if err != nil || parsed.Address != value || telegramidentity.IsPlaceholderEmail(value) {
		return ""
	}
	return value
}

func (s *BalanceRecovery) Record(ctx context.Context, orderID uint, requirements []domain.FundingRequirement) error {
	if s.waits == nil || s.orders == nil {
		return errors.New("balance recovery storage unavailable")
	}
	order, err := s.orders.ReadBalanceRecoveryOrder(ctx, orderID)
	if err != nil {
		return err
	}
	if !recoveryOrderEligible(order) {
		return nil
	}
	recipient := recoveryRecipient(order.Recipient)
	if recipient == "" {
		return nil
	}
	if err := validateRequirements(requirements); err != nil {
		return err
	}
	encoded, err := json.Marshal(requirements)
	if err != nil {
		return err
	}
	return s.waits.CreateBalanceWait(ctx, &domain.BalanceWait{OrderID: orderID, OrderNo: order.OrderNo, Recipient: recipient, Products: order.Products, Requirements: string(encoded), Status: domain.BalanceWaitWaiting})
}

func validateRequirements(needs []domain.FundingRequirement) error {
	if len(needs) == 0 {
		return errors.New("empty funding requirements")
	}
	seen := map[uint]bool{}
	for _, need := range needs {
		amount, err := decimal.NewFromString(need.Amount)
		if err != nil || !amount.IsPositive() || need.ConnectionID == 0 || strings.TrimSpace(need.Currency) == "" || seen[need.ConnectionID] {
			return errors.New("invalid funding requirements")
		}
		seen[need.ConnectionID] = true
	}
	return nil
}

type recoveryBalance struct {
	amount   decimal.Decimal
	currency string
	err      error
}

func (s *BalanceRecovery) readRecoveryBalance(ctx context.Context, id uint) recoveryBalance {
	if s.connections == nil {
		return recoveryBalance{err: errors.New("balance reader unavailable")}
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	connection, err := s.connections.Open(id)
	if err != nil {
		return recoveryBalance{err: err}
	}
	reader, ok := connection.(contract.BalanceReader)
	if !ok {
		return recoveryBalance{err: errors.New("balance reader unavailable")}
	}
	raw, currency, err := reader.ReadBalance(ctx)
	if err != nil {
		return recoveryBalance{err: err}
	}
	amount, err := decimal.NewFromString(strings.TrimSpace(raw))
	if err != nil || amount.IsNegative() {
		return recoveryBalance{err: errors.New("invalid supplier balance")}
	}
	return recoveryBalance{amount: amount, currency: strings.TrimSpace(currency)}
}

// Poll is scheduled every 60 seconds. Each supplier is read at most once in a
// run, including failed reads, and all requirements must pass for each order.
func (s *BalanceRecovery) Poll(ctx context.Context) error {
	if s.waits == nil || s.orders == nil || s.queue == nil {
		return errors.New("balance recovery unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	balances := map[uint]recoveryBalance{}
	var after uint
	var failures []error
	for {
		rows, err := s.waits.ListBalanceWaits(ctx, after, 100, time.Now().UTC())
		if err != nil {
			return errors.Join(append(failures, err)...)
		}
		for _, wait := range rows {
			after = wait.ID
			if err := ctx.Err(); err != nil {
				return errors.Join(append(failures, err)...)
			}
			if err := s.pollWait(ctx, wait, balances); err != nil {
				failures = append(failures, err)
			}
		}
		if len(rows) < 100 {
			break
		}
	}
	return errors.Join(failures...)
}

func (s *BalanceRecovery) pollWait(ctx context.Context, wait domain.BalanceWait, balances map[uint]recoveryBalance) error {
	order, err := s.orders.ReadBalanceRecoveryOrder(ctx, wait.OrderID)
	if err != nil {
		return err
	}
	if !recoveryOrderEligible(order) || recoveryRecipient(order.Recipient) != wait.Recipient {
		return s.waits.CloseBalanceWait(ctx, wait.ID, "order_or_recipient_changed")
	}
	var needs []domain.FundingRequirement
	if err := json.Unmarshal([]byte(wait.Requirements), &needs); err != nil {
		return s.waits.CloseBalanceWait(ctx, wait.ID, "invalid_requirements")
	}
	if err := validateRequirements(needs); err != nil {
		return s.waits.CloseBalanceWait(ctx, wait.ID, "invalid_requirements")
	}
	ready := true
	for _, need := range needs {
		balance, found := balances[need.ConnectionID]
		if !found {
			balance = s.readRecoveryBalance(ctx, need.ConnectionID)
			balances[need.ConnectionID] = balance
		}
		amount, _ := decimal.NewFromString(need.Amount)
		if balance.err != nil || !strings.EqualFold(balance.currency, need.Currency) || balance.amount.LessThan(amount) {
			ready = false
		}
	}
	if err := s.waits.SetBalanceWaitReady(ctx, wait.ID, ready, time.Now().UTC()); err != nil {
		return err
	}
	if ready {
		return s.queue.EnqueueBalanceRecoveryEmail(wait.ID)
	}
	return nil
}

func (s *BalanceRecovery) Send(ctx context.Context, id uint) error {
	if s.waits == nil || s.orders == nil || s.mailer == nil {
		return errors.New("balance recovery mail unavailable")
	}
	token := uuid.NewString()
	wait, err := s.waits.ClaimBalanceMail(ctx, id, token, time.Now().UTC())
	if err != nil || wait == nil {
		return err
	}
	order, err := s.orders.ReadBalanceRecoveryOrder(ctx, wait.OrderID)
	if err == nil && (!recoveryOrderEligible(order) || recoveryRecipient(order.Recipient) != wait.Recipient) {
		return s.waits.CloseBalanceWait(ctx, id, "order_or_recipient_changed")
	}
	if err == nil && (wait.RecoveredAt == nil || time.Since(*wait.RecoveredAt) > 90*time.Second) {
		err = errors.New("balance snapshot expired")
	}
	if err == nil {
		err = s.mailer.SendBalanceRecoveryEmail(ctx, wait)
	}
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	accepted := err == nil || errors.Is(err, contract.ErrBalanceRecoveryMailAccepted)
	return errors.Join(err, s.waits.FinishBalanceMail(writeCtx, id, token, time.Now().UTC(), accepted))
}
