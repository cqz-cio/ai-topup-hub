package application

import (
	"fmt"
	"strings"
	"time"

	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	walletapp "github.com/dujiao-next/internal/modules/wallet/application"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

// RefreshBalanceAccount 按账务流水重算余额账户缓存。
func RefreshBalanceAccount(store resellercontract.BalanceAccountStore, resellerID uint, currency string, now time.Time) error {
	currency = strings.TrimSpace(currency)
	if store == nil || resellerID == 0 || currency == "" {
		return nil
	}
	account, err := store.GetOrCreateBalanceAccountForUpdate(resellerID, currency)
	if err != nil {
		return err
	}
	sums, err := store.SumLedgerAmountGroupedByStatus(resellerID, currency, []string{
		resellerdomain.LedgerStatusAvailable,
		resellerdomain.LedgerStatusLocked,
	})
	if err != nil {
		return err
	}
	available := sums[resellerdomain.LedgerStatusAvailable]
	locked := sums[resellerdomain.LedgerStatusLocked]
	net := available.Round(2)
	negative := decimal.Zero
	if net.LessThan(decimal.Zero) {
		negative = net.Abs().Round(2)
		account.Status = resellerdomain.BalanceStatusNegativeBalance
	} else if account.Status == resellerdomain.BalanceStatusNegativeBalance {
		account.Status = resellerdomain.BalanceStatusNormal
	}
	account.AvailableAmountCache = money.FromDecimal(net)
	account.LockedAmountCache = money.FromDecimal(locked.Round(2))
	account.NegativeAmountCache = money.FromDecimal(negative)
	account.UpdatedAt = now
	if err := store.UpdateBalanceAccount(account); err != nil {
		return err
	}
	// Preserve historical foreign-currency ledgers without silently converting
	// them to CNY. Only CNY earnings enter the current single-currency wallet.
	if strings.ToUpper(currency) != "CNY" {
		return nil
	}
	// The source ledger remains intact for reconciliation. Its cumulative net
	// settled income is transferred once; spending never mutates this history.
	if net.IsZero() { // still sync zero when an earlier credit has been reversed
		checkpoint, err := store.Wallets().GetEarningSettlement(fmt.Sprintf("reseller:%d:%s", resellerID, currency))
		if err != nil {
			return err
		}
		if checkpoint == nil {
			return nil
		}
	}
	profile, err := store.GetProfileByID(resellerID)
	if err != nil {
		return err
	}
	if profile == nil {
		return walletcontract.ErrEarningSource
	}
	return walletapp.SyncEarningInTransaction(store, walletcontract.EarningInput{UserID: profile.UserID, Reference: fmt.Sprintf("reseller:%d:%s", resellerID, currency), Currency: currency, Amount: money.FromDecimal(net), Type: "reseller_earning", Remark: "分销收益结算 / 退款冲回"})
}
