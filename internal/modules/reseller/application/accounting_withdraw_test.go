package application

import (
	"testing"

	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	"github.com/shopspring/decimal"
)

func TestAccountingWithdrawServiceRejectsInvalidAmount(t *testing.T) {
	svc := NewAccountingWithdrawService(nil)
	_, err := svc.ApplyWithdraw(1, resellercontract.WithdrawApplyInput{
		Amount:   decimal.Zero,
		Currency: "USD",
		Channel:  "usdt",
		Account:  "T",
	})
	if err != resellercontract.ErrAccountingUnavailable {
		t.Fatalf("expected unavailable with nil store, got %v", err)
	}

	svc = NewAccountingWithdrawService(accountingWithdrawStoreStub{})
	_, err = svc.ApplyWithdraw(1, resellercontract.WithdrawApplyInput{
		Amount:   decimal.Zero,
		Currency: "USD",
		Channel:  "usdt",
		Account:  "T",
	})
	if err != resellercontract.ErrWithdrawAmountInvalid {
		t.Fatalf("expected amount invalid, got %v", err)
	}
}

type accountingWithdrawStoreStub struct{}

func (accountingWithdrawStoreStub) WithinWithdrawTransaction(fn func(store resellercontract.AccountingWithdrawStore) error) error {
	return fn(accountingWithdrawStoreStub{})
}
func (accountingWithdrawStoreStub) GetProfileByUserID(userID uint) (*resellerdomain.Profile, error) {
	return nil, nil
}
func (accountingWithdrawStoreStub) GetOrCreateBalanceAccountForUpdate(resellerID uint, currency string) (*resellerdomain.BalanceAccount, error) {
	return nil, nil
}
func (accountingWithdrawStoreStub) SumLedgerAmountGroupedByStatus(resellerID uint, currency string, statuses []string) (map[string]decimal.Decimal, error) {
	return map[string]decimal.Decimal{}, nil
}
func (accountingWithdrawStoreStub) UpdateBalanceAccount(account *resellerdomain.BalanceAccount) error {
	return nil
}
func (accountingWithdrawStoreStub) ListAvailableLedgerEntriesForUpdate(resellerID uint, currency string) ([]resellerdomain.LedgerEntry, error) {
	return nil, nil
}
func (accountingWithdrawStoreStub) UpdateLedgerEntry(entry *resellerdomain.LedgerEntry) error {
	return nil
}
func (accountingWithdrawStoreStub) CreateLedgerEntryIfNotExists(entry *resellerdomain.LedgerEntry) (bool, error) {
	return false, nil
}
func (accountingWithdrawStoreStub) BatchUpdateLedgerEntries(ids []uint, updates map[string]interface{}) error {
	return nil
}
func (accountingWithdrawStoreStub) BatchUpdateLedgerEntriesByWithdrawID(withdrawID uint, updates map[string]interface{}) error {
	return nil
}
func (accountingWithdrawStoreStub) CreateWithdrawRequest(req *resellerdomain.WithdrawRequest) error {
	return nil
}
func (accountingWithdrawStoreStub) GetWithdrawRequestByID(id uint) (*resellerdomain.WithdrawRequest, error) {
	return nil, nil
}
func (accountingWithdrawStoreStub) GetWithdrawRequestByIDForUpdate(id uint) (*resellerdomain.WithdrawRequest, error) {
	return nil, nil
}
func (accountingWithdrawStoreStub) UpdateWithdrawRequest(req *resellerdomain.WithdrawRequest) error {
	return nil
}
