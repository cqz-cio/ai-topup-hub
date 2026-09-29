package application

import (
	"testing"

	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"
)

type accountingQueryStoreStub struct {
	profile  *resellerdomain.Profile
	balances []resellerdomain.BalanceAccount
	err      error
}

func (s accountingQueryStoreStub) GetProfileByUserID(userID uint) (*resellerdomain.Profile, error) {
	return s.profile, s.err
}

func (s accountingQueryStoreStub) ListBalanceAccounts(filter resellercontract.BalanceAccountListFilter) ([]resellerdomain.BalanceAccount, int64, error) {
	return s.balances, int64(len(s.balances)), s.err
}

func (s accountingQueryStoreStub) ListLedgerEntries(filter resellercontract.LedgerListFilter) ([]resellerdomain.LedgerEntry, int64, error) {
	return nil, 0, s.err
}

func (s accountingQueryStoreStub) ListWithdrawRequests(filter resellercontract.WithdrawListFilter) ([]resellerdomain.WithdrawRequest, int64, error) {
	return nil, 0, s.err
}

func (s accountingQueryStoreStub) ListAdminResellerLedgerEntries(filter resellercontract.AdminLedgerListFilter) ([]resellerdomain.LedgerEntry, int64, error) {
	return nil, 0, s.err
}

func (s accountingQueryStoreStub) ListAdminResellerBalanceAccounts(filter resellercontract.AdminBalanceAccountListFilter) ([]resellerdomain.BalanceAccount, int64, error) {
	return nil, 0, s.err
}

func (s accountingQueryStoreStub) ListAdminResellerWithdrawRequests(filter resellercontract.AdminWithdrawListFilter) ([]resellerdomain.WithdrawRequest, int64, error) {
	return nil, 0, s.err
}

func TestAccountingQueryServiceDashboardNotOpened(t *testing.T) {
	svc := NewAccountingQueryService(accountingQueryStoreStub{})
	got, err := svc.GetUserFinanceDashboard(1)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.Opened {
		t.Fatalf("expected not opened")
	}
}

func TestRequireActiveProfileSettlementFrozen(t *testing.T) {
	err := RequireActiveProfile(&resellerdomain.Profile{
		Status:           resellerdomain.ProfileStatusActive,
		SettlementStatus: resellerdomain.SettlementStatusFrozen,
	})
	if err != resellercontract.ErrSettlementUnavailable {
		t.Fatalf("expected settlement unavailable, got %v", err)
	}
}

func TestWithdrawAvailabilityProfileInactive(t *testing.T) {
	ok, reason := WithdrawAvailability(&resellerdomain.Profile{Status: resellerdomain.ProfileStatusDisabled})
	if ok || reason != resellercontract.WithdrawDisabledReasonProfileInactive {
		t.Fatalf("unexpected availability: ok=%v reason=%s", ok, reason)
	}
}
