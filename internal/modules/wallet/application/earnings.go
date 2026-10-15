package application

import (
	"fmt"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

// SyncEarningInTransaction transfers a cumulative source ledger into the
// wallet in the source's own transaction. Account locking serializes different
// sources, spending and withdrawal, while the checkpoint makes retries safe.
func SyncEarningInTransaction(tx walletcontract.Transaction, input walletcontract.EarningInput) error {
	if tx == nil || tx.Wallets() == nil {
		return walletcontract.ErrTransactionRequired
	}
	if input.UserID == 0 || strings.TrimSpace(input.Reference) == "" {
		return walletcontract.ErrEarningSource
	}
	if normalizeCurrency(input.Currency) != "CNY" {
		return walletcontract.ErrEarningCurrency
	}
	repo := tx.Wallets()
	now := time.Now()
	account, err := ensureAccountForUpdate(repo, input.UserID, now)
	if err != nil {
		return err
	}
	checkpoint, err := repo.GetEarningSettlement(input.Reference)
	if err != nil {
		return err
	}
	if checkpoint == nil {
		checkpoint = &walletdomain.EarningSettlement{Reference: input.Reference, UserID: input.UserID, Currency: "CNY", CreatedAt: now}
	}
	if checkpoint.UserID != input.UserID || checkpoint.Currency != "CNY" {
		return walletcontract.ErrEarningSource
	}
	target := input.Amount.Decimal.Round(2)
	delta := target.Sub(checkpoint.Amount.Decimal).Round(2)
	if delta.IsZero() {
		return nil
	}
	before := account.Balance
	debtBefore := account.EarningsDebt.Decimal
	cashBefore := account.WithdrawableBalance.Decimal
	direction := constants.WalletTxnDirectionIn
	if delta.IsPositive() {
		creditEarnings(account, delta)
	} else {
		direction = constants.WalletTxnDirectionOut
		take := decimal.Min(delta.Abs(), account.WithdrawableBalance.Decimal)
		account.Balance = money.FromDecimal(account.Balance.Decimal.Sub(take))
		account.WithdrawableBalance = money.FromDecimal(account.WithdrawableBalance.Decimal.Sub(take))
		account.EarningsDebt = money.FromDecimal(account.EarningsDebt.Decimal.Add(delta.Abs().Sub(take)))
	}
	account.UpdatedAt = now
	if err := validateAccount(account); err != nil {
		return err
	}
	if err := repo.UpdateAccount(account); err != nil {
		return err
	}
	checkpoint.Amount = money.FromDecimal(target)
	checkpoint.Revision++
	checkpoint.UpdatedAt = now
	if err := repo.SaveEarningSettlement(checkpoint); err != nil {
		return err
	}
	kind := input.Type
	if delta.IsNegative() {
		kind = "earning_reversal"
	}
	return repo.CreateTransaction(&walletdomain.Transaction{
		UserID: input.UserID, Type: kind, Direction: direction, Amount: money.FromDecimal(account.Balance.Decimal.Sub(before.Decimal).Abs()), SourceAmount: money.FromDecimal(delta.Abs()),
		WithdrawableAmount: money.FromDecimal(account.WithdrawableBalance.Decimal.Sub(cashBefore).Abs()),
		DebtAmount:         money.FromDecimal(account.EarningsDebt.Decimal.Sub(debtBefore)),
		BalanceBefore:      before, BalanceAfter: account.Balance, Currency: "CNY",
		Reference: fmt.Sprintf("earning:%s:%d", input.Reference, checkpoint.Revision),
		Remark:    cleanRemark(input.Remark, "已结算收益转入钱包"), CreatedAt: now, UpdatedAt: now,
	})
}

// Newly received earnings first repay any earlier refund clawback. Recharge
// principal and gifts are never consumed by this operation or made cashable.
func creditEarnings(account *walletdomain.Account, amount decimal.Decimal) decimal.Decimal {
	repaid := decimal.Min(amount, account.EarningsDebt.Decimal)
	cash := amount.Sub(repaid).Round(2)
	account.EarningsDebt = money.FromDecimal(account.EarningsDebt.Decimal.Sub(repaid))
	account.Balance = money.FromDecimal(account.Balance.Decimal.Add(cash))
	account.WithdrawableBalance = money.FromDecimal(account.WithdrawableBalance.Decimal.Add(cash))
	return cash
}

// Spending uses non-withdrawable funds first and records the exact earnings
// component, so a payment cancellation cannot turn recharge money into cash.
func debitSpendable(account *walletdomain.Account, amount decimal.Decimal) decimal.Decimal {
	nonCash := account.Balance.Decimal.Sub(account.WithdrawableBalance.Decimal)
	cash := decimal.Max(decimal.Zero, amount.Sub(nonCash))
	account.Balance = money.FromDecimal(account.Balance.Decimal.Sub(amount))
	account.WithdrawableBalance = money.FromDecimal(account.WithdrawableBalance.Decimal.Sub(cash))
	return cash
}

func validateAccount(a *walletdomain.Account) error {
	if a.Balance.Decimal.IsNegative() || a.WithdrawableBalance.Decimal.IsNegative() ||
		a.FrozenBalance.Decimal.IsNegative() || a.EarningsDebt.Decimal.IsNegative() ||
		a.WithdrawableBalance.Decimal.GreaterThan(a.Balance.Decimal) ||
		(a.EarningsDebt.Decimal.IsPositive() && a.WithdrawableBalance.Decimal.IsPositive()) {
		return walletcontract.ErrEarningSource
	}
	return nil
}

func refundEarnings(repo walletcontract.Repository, userID uint, refund *walletcontract.RefundFunding) (decimal.Decimal, error) {
	if refund == nil {
		return decimal.Zero, nil
	}
	if refund.RootOrderID == 0 || !refund.RootTotal.Decimal.IsPositive() {
		return decimal.Zero, walletcontract.ErrEarningSource
	}
	funding, err := repo.GetOrderFunding(refund.RootOrderID)
	if err != nil || funding == nil {
		return decimal.Zero, err
	} // historical principal stays non-withdrawable
	if funding.UserID != userID {
		return decimal.Zero, walletcontract.ErrEarningSource
	}
	total := refund.RootTotal.Decimal
	after := decimal.Min(total, refund.RefundedAfter.Decimal)
	before := decimal.Max(decimal.Zero, refund.RefundedBefore.Decimal)
	if after.LessThan(before) {
		return decimal.Zero, walletcontract.ErrEarningSource
	}
	// Cumulative rounding leaves the final cent for the final refund and cannot
	// be exploited by dividing a refund into many small requests.
	target := funding.WithdrawableAmount.Decimal.Mul(after).Div(total).RoundFloor(2)
	cash := decimal.Max(decimal.Zero, target.Sub(funding.RefundedWithdrawable.Decimal))
	cash = decimal.Min(cash, after.Sub(before))
	funding.RefundedWithdrawable = money.FromDecimal(funding.RefundedWithdrawable.Decimal.Add(cash))
	funding.UpdatedAt = time.Now()
	if err := repo.SaveOrderFunding(funding); err != nil {
		return decimal.Zero, err
	}
	return cash, nil
}
