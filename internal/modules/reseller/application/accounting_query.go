package application

import (
	"errors"
	"strings"

	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"
)

// AccountingQueryService 分销财务只读查询用例。
type AccountingQueryService struct {
	store resellercontract.AccountingQueryStore
}

func NewAccountingQueryService(store resellercontract.AccountingQueryStore) *AccountingQueryService {
	return &AccountingQueryService{store: store}
}

// RequireActiveProfile 校验分销资料处于可结算的激活状态。
func RequireActiveProfile(profile *resellerdomain.Profile) error {
	if profile == nil {
		return resellercontract.ErrNotOpened
	}
	if profile.Status != resellerdomain.ProfileStatusActive {
		return resellercontract.ErrProfileInactive
	}
	if profile.SettlementStatus != "" && profile.SettlementStatus != resellerdomain.SettlementStatusNormal {
		return resellercontract.ErrSettlementUnavailable
	}
	return nil
}

// WithdrawAvailability 返回当前资料是否允许提现及禁用原因。
func WithdrawAvailability(profile *resellerdomain.Profile) (bool, string) {
	if profile == nil {
		return false, ""
	}
	if profile.Status != resellerdomain.ProfileStatusActive {
		return false, resellercontract.WithdrawDisabledReasonProfileInactive
	}
	if profile.SettlementStatus != "" && profile.SettlementStatus != resellerdomain.SettlementStatusNormal {
		return false, resellercontract.WithdrawDisabledReasonSettlementUnavailable
	}
	return true, ""
}

func (s *AccountingQueryService) getProfileByUserID(userID uint) (*resellerdomain.Profile, error) {
	if s == nil || s.store == nil || userID == 0 {
		return nil, resellercontract.ErrNotOpened
	}
	profile, err := s.store.GetProfileByUserID(userID)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, resellercontract.ErrNotOpened
	}
	return profile, nil
}

func (s *AccountingQueryService) GetUserFinanceDashboard(userID uint) (resellercontract.UserFinanceDashboard, error) {
	profile, err := s.getProfileByUserID(userID)
	if errors.Is(err, resellercontract.ErrNotOpened) {
		return resellercontract.UserFinanceDashboard{Opened: false}, nil
	}
	if err != nil {
		return resellercontract.UserFinanceDashboard{}, err
	}
	balances, _, err := s.store.ListBalanceAccounts(resellercontract.BalanceAccountListFilter{
		Page:       1,
		PageSize:   100,
		ResellerID: profile.ID,
	})
	if err != nil {
		return resellercontract.UserFinanceDashboard{}, err
	}
	withdrawEnabled, withdrawDisabledReason := WithdrawAvailability(profile)
	return resellercontract.UserFinanceDashboard{
		Opened:                 true,
		Profile:                profile,
		Balances:               balances,
		WithdrawEnabled:        withdrawEnabled,
		WithdrawDisabledReason: withdrawDisabledReason,
	}, nil
}

func (s *AccountingQueryService) ListUserBalanceAccounts(userID uint, filter resellercontract.UserBalanceAccountListFilter) ([]resellerdomain.BalanceAccount, int64, error) {
	profile, err := s.getProfileByUserID(userID)
	if err != nil {
		return nil, 0, err
	}
	if err := RequireActiveProfile(profile); err != nil {
		return nil, 0, err
	}
	return s.store.ListBalanceAccounts(resellercontract.BalanceAccountListFilter{
		Page:       filter.Page,
		PageSize:   filter.PageSize,
		ResellerID: profile.ID,
		Currency:   strings.TrimSpace(filter.Currency),
		Status:     strings.TrimSpace(filter.Status),
	})
}

func (s *AccountingQueryService) ListUserLedgerEntries(userID uint, filter resellercontract.UserLedgerListFilter) ([]resellerdomain.LedgerEntry, int64, error) {
	profile, err := s.getProfileByUserID(userID)
	if err != nil {
		return nil, 0, err
	}
	if err := RequireActiveProfile(profile); err != nil {
		return nil, 0, err
	}
	return s.store.ListLedgerEntries(resellercontract.LedgerListFilter{
		Page:       filter.Page,
		PageSize:   filter.PageSize,
		ResellerID: profile.ID,
		Currency:   strings.TrimSpace(filter.Currency),
		Type:       strings.TrimSpace(filter.Type),
		Status:     strings.TrimSpace(filter.Status),
		OrderID:    filter.OrderID,
	})
}

func (s *AccountingQueryService) ListUserWithdrawRequests(userID uint, filter resellercontract.UserWithdrawListFilter) ([]resellerdomain.WithdrawRequest, int64, error) {
	profile, err := s.getProfileByUserID(userID)
	if err != nil {
		return nil, 0, err
	}
	if err := RequireActiveProfile(profile); err != nil {
		return nil, 0, err
	}
	return s.store.ListWithdrawRequests(resellercontract.WithdrawListFilter{
		Page:       filter.Page,
		PageSize:   filter.PageSize,
		ResellerID: profile.ID,
		Currency:   strings.TrimSpace(filter.Currency),
		Status:     strings.TrimSpace(filter.Status),
	})
}

func (s *AccountingQueryService) ListAdminLedgerEntries(filter resellercontract.AdminLedgerListFilter) ([]resellerdomain.LedgerEntry, int64, error) {
	if s == nil || s.store == nil {
		return []resellerdomain.LedgerEntry{}, 0, nil
	}
	return s.store.ListAdminResellerLedgerEntries(resellercontract.AdminLedgerListFilter{
		Page:        filter.Page,
		PageSize:    filter.PageSize,
		ResellerID:  filter.ResellerID,
		UserID:      filter.UserID,
		Keyword:     strings.TrimSpace(filter.Keyword),
		Currency:    strings.TrimSpace(filter.Currency),
		Type:        strings.TrimSpace(filter.Type),
		Status:      strings.TrimSpace(filter.Status),
		OrderID:     filter.OrderID,
		OrderNo:     strings.TrimSpace(filter.OrderNo),
		CreatedFrom: filter.CreatedFrom,
		CreatedTo:   filter.CreatedTo,
	})
}

func (s *AccountingQueryService) ListAdminBalanceAccounts(filter resellercontract.AdminBalanceAccountListFilter) ([]resellerdomain.BalanceAccount, int64, error) {
	if s == nil || s.store == nil {
		return []resellerdomain.BalanceAccount{}, 0, nil
	}
	return s.store.ListAdminResellerBalanceAccounts(resellercontract.AdminBalanceAccountListFilter{
		Page:       filter.Page,
		PageSize:   filter.PageSize,
		ResellerID: filter.ResellerID,
		UserID:     filter.UserID,
		Keyword:    strings.TrimSpace(filter.Keyword),
		Currency:   strings.TrimSpace(filter.Currency),
		Status:     strings.TrimSpace(filter.Status),
	})
}

func (s *AccountingQueryService) ListAdminWithdrawRequests(filter resellercontract.AdminWithdrawListFilter) ([]resellerdomain.WithdrawRequest, int64, error) {
	if s == nil || s.store == nil {
		return []resellerdomain.WithdrawRequest{}, 0, nil
	}
	return s.store.ListAdminResellerWithdrawRequests(resellercontract.AdminWithdrawListFilter{
		Page:        filter.Page,
		PageSize:    filter.PageSize,
		ResellerID:  filter.ResellerID,
		UserID:      filter.UserID,
		Keyword:     strings.TrimSpace(filter.Keyword),
		Currency:    strings.TrimSpace(filter.Currency),
		Status:      strings.TrimSpace(filter.Status),
		CreatedFrom: filter.CreatedFrom,
		CreatedTo:   filter.CreatedTo,
	})
}
