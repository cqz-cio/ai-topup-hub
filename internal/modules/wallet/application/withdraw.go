package application

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dujiao-next/internal/constants"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

func (s *Service) GetWithdrawPolicy() (walletcontract.WithdrawPolicy, error) {
	if s.withdrawPolicy == nil {
		return walletcontract.WithdrawPolicy{Currency: "CNY", Channels: []string{}}, nil
	}
	return s.withdrawPolicy.GetWalletWithdrawPolicy()
}

func (s *Service) GetUserWithdrawPolicy(userID uint) (walletcontract.WithdrawPolicy, error) {
	policy, err := s.GetWithdrawPolicy()
	if err != nil {
		return policy, err
	}
	if err := s.repository.CheckWithdrawalAllowed(userID); err != nil {
		if errors.Is(err, walletcontract.ErrWithdrawRestricted) {
			policy.BlockedReason = err.Error()
		} else {
			return policy, err
		}
	}
	return policy, nil
}

func (s *Service) ListWithdraws(filter walletcontract.WithdrawListFilter) ([]walletdomain.WithdrawRequest, int64, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	return s.repository.ListWithdraws(filter)
}

func (s *Service) ApplyWithdraw(input walletcontract.WithdrawInput) (*walletdomain.WithdrawRequest, error) {
	policy, err := s.GetWithdrawPolicy()
	if err != nil {
		return nil, err
	}
	if !policy.Enabled {
		return nil, walletcontract.ErrWithdrawDisabled
	}
	if policy.Currency != "CNY" {
		return nil, walletcontract.ErrEarningCurrency
	}
	input.Channel, input.Account, input.RealName = strings.TrimSpace(input.Channel), strings.TrimSpace(input.Account), strings.TrimSpace(input.RealName)
	input.RequestKey = strings.TrimSpace(input.RequestKey)
	amount := input.Amount.Decimal
	channelAllowed := false
	for _, channel := range policy.Channels {
		if channel == input.Channel && (channel == "alipay" || channel == "wechat") {
			channelAllowed = true
		}
	}
	if input.UserID == 0 || !amount.IsPositive() || !amount.Equal(amount.Round(2)) || amount.LessThan(policy.Minimum.Decimal) ||
		!channelAllowed || input.Account == "" || input.RealName == "" || utf8.RuneCountInString(input.Account) > 200 ||
		utf8.RuneCountInString(input.RealName) > 80 || len(input.RequestKey) < 16 || len(input.RequestKey) > 80 {
		return nil, walletcontract.ErrWithdrawInvalid
	}
	var result *walletdomain.WithdrawRequest
	err = s.transactions.WithinTransaction(func(tx walletcontract.Transaction) error {
		repo := tx.Wallets()
		now := time.Now()
		account, err := ensureAccountForUpdate(repo, input.UserID, now)
		if err != nil {
			return err
		}
		existing, err := repo.GetWithdrawByKey(input.UserID, input.RequestKey)
		if err != nil {
			return err
		}
		if existing != nil {
			if !existing.Amount.Decimal.Equal(amount) || existing.Channel != input.Channel || existing.Account != input.Account || existing.RealName != input.RealName {
				return walletcontract.ErrWithdrawConflict
			}
			result = existing
			return nil
		}
		if account.EarningsDebt.Decimal.IsPositive() {
			return walletcontract.ErrWithdrawDebt
		}
		if err := repo.CheckWithdrawalAllowed(input.UserID); err != nil {
			return err
		}
		if account.WithdrawableBalance.Decimal.LessThan(amount) || account.Balance.Decimal.LessThan(amount) {
			return walletcontract.ErrWithdrawInsufficient
		}
		before := account.Balance
		account.Balance = money.FromDecimal(account.Balance.Decimal.Sub(amount))
		account.WithdrawableBalance = money.FromDecimal(account.WithdrawableBalance.Decimal.Sub(amount))
		account.FrozenBalance = money.FromDecimal(account.FrozenBalance.Decimal.Add(amount))
		account.UpdatedAt = now
		if err := validateAccount(account); err != nil {
			return err
		}
		if err := repo.UpdateAccount(account); err != nil {
			return err
		}
		result = &walletdomain.WithdrawRequest{UserID: input.UserID, RequestKey: input.RequestKey, Amount: input.Amount, Currency: "CNY", Channel: input.Channel, Account: input.Account, RealName: input.RealName, Status: walletdomain.WithdrawPending, CreatedAt: now, UpdatedAt: now}
		if err := repo.CreateWithdraw(result); err != nil {
			return err
		}
		return repo.CreateTransaction(&walletdomain.Transaction{UserID: input.UserID, Type: "withdraw_freeze", Direction: constants.WalletTxnDirectionOut, Amount: input.Amount, SourceAmount: input.Amount, FrozenAmount: input.Amount, WithdrawableAmount: input.Amount, BalanceBefore: before, BalanceAfter: account.Balance, Currency: "CNY", Reference: fmt.Sprintf("withdraw:%d:freeze", result.ID), Remark: "提现申请，收益冻结待人工审核打款", CreatedAt: now, UpdatedAt: now})
	})
	return result, err
}

func (s *Service) ReviewWithdraw(input walletcontract.WithdrawReviewInput) (*walletdomain.WithdrawRequest, error) {
	input.Action, input.Remark, input.PayoutReference = strings.TrimSpace(input.Action), strings.TrimSpace(input.Remark), strings.TrimSpace(input.PayoutReference)
	if input.ID == 0 || input.AdminID == 0 || utf8.RuneCountInString(input.Remark) > 500 || utf8.RuneCountInString(input.PayoutReference) > 120 {
		return nil, walletcontract.ErrWithdrawInvalid
	}
	if input.Action != "approve" && input.Action != "reject" && input.Action != "paid" {
		return nil, walletcontract.ErrWithdrawInvalid
	}
	if (input.Action == "reject" && input.Remark == "") || (input.Action == "paid" && input.PayoutReference == "") {
		return nil, walletcontract.ErrWithdrawInvalid
	}
	var result *walletdomain.WithdrawRequest
	err := s.transactions.WithinTransaction(func(tx walletcontract.Transaction) error {
		repo := tx.Wallets()
		request, err := repo.GetWithdraw(input.ID, false)
		if err != nil {
			return err
		}
		if request == nil {
			return walletcontract.ErrWithdrawNotFound
		}
		now := time.Now()
		// All operations lock the account before the request to avoid lock inversion.
		account, err := ensureAccountForUpdate(repo, request.UserID, now)
		if err != nil {
			return err
		}
		request, err = repo.GetWithdraw(input.ID, true)
		if err != nil {
			return err
		}
		if request == nil {
			return walletcontract.ErrWithdrawNotFound
		}
		result = request
		if request.Status == walletdomain.WithdrawPaid || request.Status == walletdomain.WithdrawRejected {
			if (request.Status == walletdomain.WithdrawPaid && input.Action == "paid" && input.PayoutReference == request.PayoutReference) || (request.Status == walletdomain.WithdrawRejected && input.Action == "reject") {
				return nil
			}
			return walletcontract.ErrWithdrawState
		}
		if input.Action == "approve" && request.Status == walletdomain.WithdrawApproved {
			return nil
		}
		if input.Action != "reject" && account.EarningsDebt.Decimal.IsPositive() {
			return walletcontract.ErrWithdrawDebt
		}
		if input.Action != "reject" {
			if err := repo.CheckWithdrawalAllowed(request.UserID); err != nil {
				return err
			}
		}
		amount := request.Amount.Decimal
		if account.FrozenBalance.Decimal.LessThan(amount) {
			return walletcontract.ErrEarningSource
		}
		if input.Action == "approve" {
			if request.Status != walletdomain.WithdrawPending {
				return walletcontract.ErrWithdrawState
			}
			request.Status, request.ApprovedBy, request.ApprovedAt = walletdomain.WithdrawApproved, &input.AdminID, &now
			request.Remark, request.UpdatedAt = input.Remark, now
			return repo.UpdateWithdraw(request)
		}
		if input.Action == "paid" && request.Status != walletdomain.WithdrawApproved {
			return walletcontract.ErrWithdrawState
		}
		before := account.Balance
		debtBefore := account.EarningsDebt.Decimal
		cash := decimal.Zero
		account.FrozenBalance = money.FromDecimal(account.FrozenBalance.Decimal.Sub(amount))
		kind, direction := "withdraw_paid", constants.WalletTxnDirectionOut
		if input.Action == "reject" {
			request.Status = walletdomain.WithdrawRejected
			kind, direction = "withdraw_release", constants.WalletTxnDirectionIn
			cash = creditEarnings(account, amount)
		} else {
			request.Status, request.PayoutReference = walletdomain.WithdrawPaid, input.PayoutReference
		}
		request.ProcessedBy, request.ProcessedAt, request.Remark, request.UpdatedAt = &input.AdminID, &now, input.Remark, now
		account.UpdatedAt = now
		if err := validateAccount(account); err != nil {
			return err
		}
		if err := repo.UpdateAccount(account); err != nil {
			return err
		}
		if err := repo.UpdateWithdraw(request); err != nil {
			return err
		}
		return repo.CreateTransaction(&walletdomain.Transaction{UserID: request.UserID, OperatorAdminID: &input.AdminID, Type: kind, Direction: direction, Amount: money.FromDecimal(account.Balance.Decimal.Sub(before.Decimal).Abs()), SourceAmount: request.Amount, FrozenAmount: money.FromDecimal(amount.Neg()), WithdrawableAmount: money.FromDecimal(cash), DebtAmount: money.FromDecimal(account.EarningsDebt.Decimal.Sub(debtBefore)), BalanceBefore: before, BalanceAfter: account.Balance, Currency: "CNY", Reference: fmt.Sprintf("withdraw:%d:%s", request.ID, input.Action), Remark: cleanRemark(input.Remark, "人工提现处理"), CreatedAt: now, UpdatedAt: now})
	})
	return result, err
}
