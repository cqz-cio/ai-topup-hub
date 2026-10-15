package application

import (
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	"strings"
	"time"

	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
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
	return nil, walletcontract.ErrUnifiedWallet
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
