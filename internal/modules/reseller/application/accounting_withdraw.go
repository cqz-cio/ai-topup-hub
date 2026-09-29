package application

import (
	"fmt"
	"strings"
	"time"

	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"

	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

// AccountingWithdrawService 分销提现申请与审核用例。
type AccountingWithdrawService struct {
	store resellercontract.AccountingWithdrawStore
}

func NewAccountingWithdrawService(store resellercontract.AccountingWithdrawStore) *AccountingWithdrawService {
	return &AccountingWithdrawService{store: store}
}

func (s *AccountingWithdrawService) ApplyUserWithdraw(userID uint, input resellercontract.WithdrawApplyInput) (*resellerdomain.WithdrawRequest, error) {
	if s == nil || s.store == nil || userID == 0 {
		return nil, resellercontract.ErrAccountingUnavailable
	}
	profile, err := s.store.GetProfileByUserID(userID)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, resellercontract.ErrNotOpened
	}
	if err := RequireActiveProfile(profile); err != nil {
		return nil, err
	}
	return s.ApplyWithdraw(profile.ID, input)
}

func (s *AccountingWithdrawService) ApplyWithdraw(resellerID uint, input resellercontract.WithdrawApplyInput) (*resellerdomain.WithdrawRequest, error) {
	if s == nil || s.store == nil || resellerID == 0 {
		return nil, resellercontract.ErrAccountingUnavailable
	}
	amount := input.Amount.Round(2)
	currency := strings.TrimSpace(input.Currency)
	channel := strings.TrimSpace(input.Channel)
	account := strings.TrimSpace(input.Account)
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, resellercontract.ErrWithdrawAmountInvalid
	}
	if currency == "" {
		return nil, resellercontract.ErrWithdrawCurrencyUnavailable
	}
	if channel == "" || account == "" {
		return nil, resellercontract.ErrWithdrawAmountInvalid
	}
	var createdID uint
	err := s.store.WithinWithdrawTransaction(func(store resellercontract.AccountingWithdrawStore) error {
		balance, err := store.GetOrCreateBalanceAccountForUpdate(resellerID, currency)
		if err != nil {
			return err
		}
		if balance.Status == resellerdomain.BalanceStatusNegativeBalance ||
			balance.Status == resellerdomain.BalanceStatusFrozenReview ||
			balance.Status == resellerdomain.BalanceStatusDisabled {
			return resellercontract.ErrBalanceAccountFrozen
		}
		// 可提现额必须以「净可用余额」为准（含退款扣减等负数流水），
		// 防止仅凭正数流水之和超额提现，导致账户被提成负余额、造成平台资损。
		availableSums, err := store.SumLedgerAmountGroupedByStatus(resellerID, currency, []string{resellerdomain.LedgerStatusAvailable})
		if err != nil {
			return err
		}
		if amount.GreaterThan(availableSums[resellerdomain.LedgerStatusAvailable].Round(2)) {
			return resellercontract.ErrWithdrawInsufficient
		}
		ledgers, err := store.ListAvailableLedgerEntriesForUpdate(resellerID, currency)
		if err != nil {
			return err
		}
		remaining := amount
		selectedIDs := make([]uint, 0)
		now := time.Now()
		for i := range ledgers {
			if remaining.LessThanOrEqual(decimal.Zero) {
				break
			}
			row := ledgers[i]
			rowAmount := row.Amount.Decimal.Round(2)
			if rowAmount.LessThanOrEqual(decimal.Zero) {
				continue
			}
			if rowAmount.LessThanOrEqual(remaining) {
				selectedIDs = append(selectedIDs, row.ID)
				remaining = remaining.Sub(rowAmount).Round(2)
				continue
			}
			lockAmount := remaining.Round(2)
			remainAmount := rowAmount.Sub(lockAmount).Round(2)
			row.Amount = money.FromDecimal(lockAmount)
			row.UpdatedAt = now
			if err := store.UpdateLedgerEntry(&row); err != nil {
				return err
			}
			remainRow := row
			remainRow.ID = 0
			remainRow.Amount = money.FromDecimal(remainAmount)
			remainRow.Status = resellerdomain.LedgerStatusAvailable
			remainRow.WithdrawRequestID = nil
			remainRow.IdempotencyKey = fmt.Sprintf("split:%d:%d", row.ID, now.UnixNano())
			remainRow.CreatedAt = now
			remainRow.UpdatedAt = now
			if _, err := store.CreateLedgerEntryIfNotExists(&remainRow); err != nil {
				return err
			}
			selectedIDs = append(selectedIDs, row.ID)
			remaining = decimal.Zero
			break
		}
		if remaining.GreaterThan(decimal.Zero) {
			return resellercontract.ErrWithdrawInsufficient
		}
		req := &resellerdomain.WithdrawRequest{
			ResellerID: resellerID,
			Amount:     money.FromDecimal(amount),
			Currency:   currency,
			Channel:    channel,
			Account:    account,
			Status:     resellerdomain.WithdrawStatusPending,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		if err := store.CreateWithdrawRequest(req); err != nil {
			return err
		}
		if err := store.BatchUpdateLedgerEntries(selectedIDs, map[string]interface{}{
			"status":              resellerdomain.LedgerStatusLocked,
			"withdraw_request_id": req.ID,
		}); err != nil {
			return err
		}
		createdID = req.ID
		return RefreshBalanceAccount(store, resellerID, currency, now)
	})
	if err != nil {
		return nil, err
	}
	return s.store.GetWithdrawRequestByID(createdID)
}

func (s *AccountingWithdrawService) ReviewWithdraw(adminID uint, withdrawID uint, action string, rejectReason string) (*resellerdomain.WithdrawRequest, error) {
	if s == nil || s.store == nil || withdrawID == 0 {
		return nil, productcontract.ErrNotFound
	}
	act := strings.ToLower(strings.TrimSpace(action))
	if act != resellercontract.WithdrawActionReject && act != resellercontract.WithdrawActionPay {
		return nil, resellercontract.ErrWithdrawStatusInvalid
	}
	err := s.store.WithinWithdrawTransaction(func(store resellercontract.AccountingWithdrawStore) error {
		req, err := store.GetWithdrawRequestByIDForUpdate(withdrawID)
		if err != nil {
			return err
		}
		if req == nil {
			return productcontract.ErrNotFound
		}
		if req.Status != resellerdomain.WithdrawStatusPending {
			return resellercontract.ErrWithdrawStatusInvalid
		}
		now := time.Now()
		req.ProcessedBy = &adminID
		req.ProcessedAt = &now
		req.UpdatedAt = now
		if act == resellercontract.WithdrawActionReject {
			req.Status = resellerdomain.WithdrawStatusRejected
			req.RejectReason = strings.TrimSpace(rejectReason)
			if err := store.BatchUpdateLedgerEntriesByWithdrawID(withdrawID, map[string]interface{}{
				"status":              resellerdomain.LedgerStatusAvailable,
				"withdraw_request_id": nil,
			}); err != nil {
				return err
			}
		} else {
			req.Status = resellerdomain.WithdrawStatusPaid
			req.RejectReason = ""
			if err := store.BatchUpdateLedgerEntriesByWithdrawID(withdrawID, map[string]interface{}{
				"status": resellerdomain.LedgerStatusWithdrawn,
			}); err != nil {
				return err
			}
		}
		if err := store.UpdateWithdrawRequest(req); err != nil {
			return err
		}
		return RefreshBalanceAccount(store, req.ResellerID, req.Currency, now)
	})
	if err != nil {
		return nil, err
	}
	return s.store.GetWithdrawRequestByID(withdrawID)
}
