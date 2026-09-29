package application

import (
	"time"

	"github.com/dujiao-next/internal/constants"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// ApplyOrderBalance debits wallet funds inside the caller's transaction.
// Updating the order allocation remains an order-context responsibility.
func (s *Service) ApplyOrderBalance(tx walletcontract.Transaction, input walletcontract.OrderBalanceInput) (money.Amount, error) {
	if tx == nil {
		return money.Amount{}, walletcontract.ErrTransactionRequired
	}
	if !input.UseBalance {
		return input.WalletPaidAmount, nil
	}
	if input.UserID == 0 {
		return money.Amount{}, walletcontract.ErrNotSupportedForGuest
	}
	existingPaid := input.WalletPaidAmount.Decimal.Round(2)
	if existingPaid.GreaterThan(decimal.Zero) {
		return money.FromDecimal(existingPaid), nil
	}
	total := input.TotalAmount.Decimal.Round(2)
	if total.LessThanOrEqual(decimal.Zero) {
		return money.FromDecimal(decimal.Zero), nil
	}

	repository := tx.Wallets()
	now := time.Now()
	account, err := ensureAccountForUpdate(repository, input.UserID, now)
	if err != nil {
		return money.Amount{}, err
	}
	available := account.Balance.Decimal.Round(2)
	if available.LessThanOrEqual(decimal.Zero) {
		return money.FromDecimal(decimal.Zero), nil
	}
	deduct := available
	if deduct.GreaterThan(total) {
		deduct = total
	}
	reference, err := orderAllocationReference(repository, input.OrderID, constants.WalletTxnTypeOrderPay)
	if err != nil {
		return money.Amount{}, err
	}
	existing, err := repository.GetTransactionByReference(reference)
	if err != nil {
		return money.Amount{}, err
	}
	if existing != nil {
		return existing.Amount, nil
	}

	before := account.Balance.Decimal.Round(2)
	after := before.Sub(deduct).Round(2)
	if after.LessThan(decimal.Zero) {
		return money.Amount{}, walletcontract.ErrInsufficientBalance
	}
	account.Balance = money.FromDecimal(after)
	account.UpdatedAt = now
	if err := repository.UpdateAccount(account); err != nil {
		return money.Amount{}, walletcontract.ErrAccountUpdateFailed
	}
	orderID := input.OrderID
	transaction := &walletdomain.Transaction{
		UserID: input.UserID, OrderID: &orderID,
		Type: constants.WalletTxnTypeOrderPay, Direction: constants.WalletTxnDirectionOut,
		Amount: money.FromDecimal(deduct), BalanceBefore: money.FromDecimal(before),
		BalanceAfter: money.FromDecimal(after), Currency: normalizeCurrency(input.Currency),
		Reference: reference, Remark: "订单余额支付", CreatedAt: now, UpdatedAt: now,
	}
	if err := repository.CreateTransaction(transaction); err != nil {
		return money.Amount{}, walletcontract.ErrTransactionCreateFailed
	}
	return transaction.Amount, nil
}

// ReleaseOrderBalance credits a previously allocated order balance. claim must
// atomically clear the order allocation before the credit is persisted.
func (s *Service) ReleaseOrderBalance(
	tx walletcontract.Transaction,
	input walletcontract.OrderReleaseInput,
	claim walletcontract.ReleaseClaim,
) (money.Amount, error) {
	if tx == nil {
		return money.Amount{}, walletcontract.ErrTransactionRequired
	}
	if input.UserID == 0 {
		return money.FromDecimal(decimal.Zero), nil
	}
	amount := input.WalletPaidAmount.Decimal.Round(2)
	if amount.LessThanOrEqual(decimal.Zero) {
		return money.FromDecimal(decimal.Zero), nil
	}
	repository := tx.Wallets()
	reference, err := orderAllocationReference(repository, input.OrderID, input.TransactionType)
	if err != nil {
		return money.Amount{}, err
	}
	existing, err := repository.GetTransactionByReference(reference)
	if err != nil {
		return money.Amount{}, err
	}
	if existing != nil {
		return existing.Amount, nil
	}

	now := time.Now()
	if claim != nil {
		claimed, err := claim(now)
		if err != nil {
			return money.Amount{}, err
		}
		if !claimed {
			return money.FromDecimal(decimal.Zero), nil
		}
	}
	account, err := ensureAccountForUpdate(repository, input.UserID, now)
	if err != nil {
		return money.Amount{}, err
	}
	before := account.Balance.Decimal.Round(2)
	after := before.Add(amount).Round(2)
	account.Balance = money.FromDecimal(after)
	account.UpdatedAt = now
	if err := repository.UpdateAccount(account); err != nil {
		return money.Amount{}, walletcontract.ErrAccountUpdateFailed
	}
	orderID := input.OrderID
	transaction := &walletdomain.Transaction{
		UserID: input.UserID, OrderID: &orderID,
		Type: input.TransactionType, Direction: constants.WalletTxnDirectionIn,
		Amount: money.FromDecimal(amount), BalanceBefore: money.FromDecimal(before),
		BalanceAfter: money.FromDecimal(after), Currency: normalizeCurrency(input.Currency),
		Reference: reference, Remark: cleanRemark(input.Remark, "订单余额退回"),
		CreatedAt: now, UpdatedAt: now,
	}
	if err := repository.CreateTransaction(transaction); err != nil {
		return money.Amount{}, walletcontract.ErrTransactionCreateFailed
	}
	return transaction.Amount, nil
}
