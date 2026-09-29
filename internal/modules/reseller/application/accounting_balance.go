package application

import (
	"strings"
	"time"

	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

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
	return store.UpdateBalanceAccount(account)
}
